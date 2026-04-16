package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

type StatsHandler struct {
	db *pgxpool.Pool
}

func NewStatsHandler(db *pgxpool.Pool) *StatsHandler {
	return &StatsHandler{db: db}
}

// DateCount is a single data point for the time-series chart.
type DateCount struct {
	Date  string `json:"date"  example:"2024-01-15"`
	Count int64  `json:"count" example:"42"`
}

// StatsResponse is the full analytics payload for a project.
type StatsResponse struct {
	ByDate           []DateCount      `json:"by_date"`
	ByLevel          map[string]int64 `json:"by_level"`
	TotalIssues      int64            `json:"total_issues"      example:"15"`
	TotalOccurrences int64            `json:"total_occurrences" example:"59"`
}

// Get godoc
//
//	@Summary      Project stats
//	@Description  Returns aggregated error statistics for a project. Used to render charts on the dashboard. Three queries (by_date, by_level, totals) run in parallel goroutines. Default range: last 30 days.
//	@Tags         stats
//	@Produce      json
//	@Security     BearerAuth
//	@Param        projectId path     string true  "Project UUID"
//	@Param        from      query    string false "Start date inclusive (YYYY-MM-DD). Default: 30 days ago"
//	@Param        to        query    string false "End date inclusive (YYYY-MM-DD). Default: today"
//	@Success      200       {object} StatsResponse
//	@Failure      400       {object} ErrorResponse "invalid date format"
//	@Failure      401       {object} ErrorResponse
//	@Failure      500       {object} ErrorResponse
//	@Router       /api/projects/{projectId}/stats [get]
func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	from, to, err := parseDateRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date format; use YYYY-MM-DD")
		return
	}

	var (
		byDate           []DateCount
		byLevel          = make(map[string]int64)
		totalIssues      int64
		totalOccurrences int64
	)

	// errgroup cancels the shared context when any goroutine returns an error,
	// stopping the remaining DB queries immediately.
	g, ctx := errgroup.WithContext(r.Context())

	// 1. Events per day (time-series chart).
	g.Go(func() error {
		rows, err := h.db.Query(ctx, `
			SELECT DATE(created_at)::text, COUNT(*)
			FROM events
			WHERE project_id = $1 AND created_at >= $2 AND created_at < $3
			GROUP BY 1 ORDER BY 1`,
			projectID, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var d DateCount
			if err := rows.Scan(&d.Date, &d.Count); err != nil {
				continue
			}
			byDate = append(byDate, d)
		}
		return rows.Err()
	})

	// 2. Occurrences by severity level.
	g.Go(func() error {
		rows, err := h.db.Query(ctx, `
			SELECT level, COALESCE(SUM(occurrences), 0)
			FROM issues
			WHERE project_id = $1 AND last_seen >= $2 AND first_seen < $3
			GROUP BY level`,
			projectID, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var level string
			var count int64
			if err := rows.Scan(&level, &count); err != nil {
				continue
			}
			byLevel[level] = count
		}
		return rows.Err()
	})

	// 3. Total issues and occurrences for the period.
	g.Go(func() error {
		return h.db.QueryRow(ctx, `
			SELECT COUNT(*), COALESCE(SUM(occurrences), 0)
			FROM issues
			WHERE project_id = $1 AND last_seen >= $2 AND first_seen < $3`,
			projectID, from, to).Scan(&totalIssues, &totalOccurrences)
	})

	if err := g.Wait(); err != nil {
		log.Printf("failed to fetch stats [project=%s]: %v", projectID, err)
		writeError(w, http.StatusInternalServerError, "failed to fetch stats")
		return
	}

	if byDate == nil {
		byDate = []DateCount{}
	}

	writeJSON(w, http.StatusOK, StatsResponse{
		ByDate:           byDate,
		ByLevel:          byLevel,
		TotalIssues:      totalIssues,
		TotalOccurrences: totalOccurrences,
	})
}

func parseDateRange(r *http.Request) (from, to time.Time, err error) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" {
		from = time.Now().UTC().AddDate(0, 0, -30)
	} else {
		from, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			return
		}
	}

	if toStr == "" {
		to = time.Now().UTC().AddDate(0, 0, 1)
	} else {
		to, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			return
		}
		to = to.AddDate(0, 0, 1)
	}
	return
}
