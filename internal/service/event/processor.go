// Package event provides an async worker pool for processing incoming error events.
// The HTTP handler validates the payload and enqueues a job immediately, so the
// response is returned to the SDK without waiting for DB/Redis writes. Workers
// drain the queue concurrently and persist each event.
package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Aspors/errhub-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Job holds everything a worker needs to persist one event.
type Job struct {
	Payload     models.EventPayload
	Fingerprint string
}

// Processor is a fixed-size worker pool connected to a buffered job channel.
// Call Start to launch workers, Enqueue to submit jobs, Stop to drain and exit.
type Processor struct {
	queue chan Job
	db    *pgxpool.Pool
	rdb   *redis.Client
	wg    sync.WaitGroup
}

func NewProcessor(db *pgxpool.Pool, rdb *redis.Client, bufSize int) *Processor {
	return &Processor{
		queue: make(chan Job, bufSize),
		db:    db,
		rdb:   rdb,
	}
}

// Start launches n worker goroutines. Call once at application startup.
func (p *Processor) Start(n int) {
	for i := 0; i < n; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for job := range p.queue {
				p.process(job)
			}
		}()
	}
}

// Stop closes the job channel and waits for all workers to finish draining it.
// Call during graceful shutdown to avoid losing queued events.
func (p *Processor) Stop() {
	close(p.queue)
	p.wg.Wait()
}

// Enqueue submits a job. Returns false and logs a warning when the buffer is full
// (back-pressure: caller can decide to drop or respond with 503).
func (p *Processor) Enqueue(job Job) bool {
	select {
	case p.queue <- job:
		return true
	default:
		log.Printf("event processor queue full (cap=%d), dropping event for project %s",
			cap(p.queue), job.Payload.ProjectID)
		return false
	}
}

// QueueLen returns the current number of pending jobs (useful for metrics/health).
func (p *Processor) QueueLen() int { return len(p.queue) }

func (p *Processor) process(job Job) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	upsertQuery := `
		INSERT INTO issues (project_id, fingerprint, level, error_type, error_message)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (project_id, fingerprint)
		DO UPDATE SET
			occurrences = issues.occurrences + 1,
			last_seen   = NOW()
		RETURNING id, occurrences`

	var issueID string
	var occurrences int
	err := p.db.QueryRow(ctx, upsertQuery,
		job.Payload.ProjectID,
		job.Fingerprint,
		job.Payload.Level,
		job.Payload.Error.Type,
		job.Payload.Error.Message,
	).Scan(&issueID, &occurrences)
	if err != nil {
		log.Printf("processor: upsert issue failed [project=%s hash=%s]: %v",
			job.Payload.ProjectID, job.Fingerprint, err)
		return
	}

	payloadBytes, _ := json.Marshal(job.Payload)
	insertEventQuery := `INSERT INTO events (project_id, issue_id, payload) VALUES ($1, $2, $3)`
	if _, err := p.db.Exec(ctx, insertEventQuery, job.Payload.ProjectID, issueID, payloadBytes); err != nil {
		log.Printf("processor: insert event failed [issue=%s]: %v", issueID, err)
	}

	// Sync Redis counter with DB value (best-effort).
	redisKey := fmt.Sprintf("issue:%s:count", job.Fingerprint)
	pipe := p.rdb.Pipeline()
	pipe.Set(ctx, redisKey, occurrences, 30*24*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("processor: redis sync failed [key=%s]: %v", redisKey, err)
	}

	if occurrences == 1 {
		log.Printf("new issue [project=%s type=%s hash=%.8s]",
			job.Payload.ProjectID, job.Payload.Error.Type, job.Fingerprint)
	}
}
