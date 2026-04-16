// Package cleanup runs a daily background goroutine that removes source map files
// from MinIO that satisfy BOTH retention conditions:
//   - created_at  < NOW() - 3 days  (not a fresh build)
//   - last_used_at < NOW() - 7 days  (not actively resolving stack traces)
//
// This protects hotfix-night builds (< 3 days old) while reclaiming space for
// old, unused maps.
package cleanup

import (
	"context"
	"log"
	"time"

	"github.com/Aspors/errhub-backend/internal/storage/s3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Start launches the cleanup goroutine. It exits when ctx is cancelled (graceful shutdown).
func Start(ctx context.Context, db *pgxpool.Pool, storage *s3.Storage) {
	go func() {
		// Run once at startup (after a short delay to let the server warm up),
		// then every 24 hours.
		select {
		case <-time.After(5 * time.Minute):
		case <-ctx.Done():
			return
		}
		run(ctx, db, storage)

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				run(ctx, db, storage)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func run(parentCtx context.Context, db *pgxpool.Pool, storage *s3.Storage) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Minute)
	defer cancel()

	rows, err := db.Query(ctx, `
		SELECT id, object_key
		FROM sourcemap_files
		WHERE created_at  < NOW() - INTERVAL '3 days'
		  AND last_used_at < NOW() - INTERVAL '7 days'
		  AND id NOT IN (
		      SELECT DISTINCT ON (project_id) id
		      FROM sourcemap_files
		      ORDER BY project_id, created_at DESC
		  )
		LIMIT 1000`)
	if err != nil {
		log.Printf("cleanup: query failed: %v", err)
		return
	}

	var ids []string
	var keys []string
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			continue
		}
		ids = append(ids, id)
		keys = append(keys, key)
	}
	rows.Close()

	if len(ids) == 0 {
		return
	}

	// Batch delete from MinIO in a single request.
	// On any error we skip the DB cleanup so records stay for the next cycle.
	if err := storage.DeleteObjects(ctx, keys); err != nil {
		log.Printf("cleanup: batch delete failed, will retry next cycle: %v", err)
		return
	}

	if _, err := db.Exec(ctx, `DELETE FROM sourcemap_files WHERE id = ANY($1)`, ids); err != nil {
		log.Printf("cleanup: failed to remove records from DB: %v", err)
		return
	}

	log.Printf("cleanup: removed %d stale source map(s)", len(ids))
}
