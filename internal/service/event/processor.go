// Package event provides an async worker pool for processing incoming error events.
// The HTTP handler validates the payload and enqueues a job immediately, so the
// response is returned to the SDK without waiting for DB/Redis writes. Workers
// accumulate jobs into a local batch and flush to DB when the batch is full or
// a time interval elapses — whichever comes first.
package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Aspors/errhub-backend/internal/models"
	"github.com/Aspors/errhub-backend/internal/service/sourcemap"
	"github.com/jackc/pgx/v5"
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
	queue  chan Job
	db     *pgxpool.Pool
	rdb    *redis.Client
	srcSvc *sourcemap.Service
	wg     sync.WaitGroup
}

func NewProcessor(db *pgxpool.Pool, rdb *redis.Client, srcSvc *sourcemap.Service, bufSize int) *Processor {
	return &Processor{
		queue:  make(chan Job, bufSize),
		db:     db,
		rdb:    rdb,
		srcSvc: srcSvc,
	}
}

// Start launches workersCount goroutines. Each worker accumulates jobs into a
// local batch and flushes to DB either when the batch reaches batchSize or when
// flushInterval elapses — whichever comes first.
func (p *Processor) Start(workersCount, batchSize int, flushInterval time.Duration) {
	for i := 0; i < workersCount; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()

			batch := make([]Job, 0, batchSize)
			ticker := time.NewTicker(flushInterval)
			defer ticker.Stop()

			flush := func() {
				if len(batch) > 0 {
					p.flushBatch(batch)
					batch = batch[:0]
				}
			}

			for {
				select {
				case job, ok := <-p.queue:
					if !ok {
						// Channel closed (graceful shutdown) — flush remainder and exit.
						flush()
						return
					}
					batch = append(batch, job)
					if len(batch) >= batchSize {
						flush()
						ticker.Reset(flushInterval)
					}
				case <-ticker.C:
					flush()
				}
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

func (p *Processor) flushBatch(jobs []Job) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tx, err := p.db.Begin(ctx)
	if err != nil {
		log.Printf("processor: failed to begin tx: %v", err)
		return
	}
	defer tx.Rollback(ctx)

	// Step 1: batch upsert issues — one round-trip for all jobs.
	issueBatch := &pgx.Batch{}
	for _, job := range jobs {
		issueBatch.Queue(`
			INSERT INTO issues (project_id, fingerprint, level, error_type, error_message)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (project_id, fingerprint) DO UPDATE SET
				occurrences = issues.occurrences + 1,
				last_seen   = NOW(),
				status      = CASE WHEN issues.status = 'resolved' THEN 'open' ELSE issues.status END
			RETURNING id`,
			job.Payload.ProjectID, job.Fingerprint,
			job.Payload.Level, job.Payload.Error.Type, job.Payload.Error.Message,
		)
	}

	brIssues := tx.SendBatch(ctx, issueBatch)
	issueIDs := make([]string, len(jobs))
	for i := range jobs {
		if err := brIssues.QueryRow().Scan(&issueIDs[i]); err != nil {
			log.Printf("processor: batch upsert issue failed at index %d: %v", i, err)
			brIssues.Close()
			return
		}
	}
	brIssues.Close()

	// Step 2: batch insert events.
	// Sourcemap resolution is intentionally skipped here — a slow S3 read would
	// stall the entire batch. resolved_stack is filled retroactively when a
	// sourcemap is uploaded (see sourcemap.Service.ResolveEventsForRelease).
	eventBatch := &pgx.Batch{}
	for i, job := range jobs {
		payloadBytes, err := json.Marshal(job.Payload)
		if err != nil {
			log.Printf("processor: json marshal failed at index %d: %v", i, err)
			return
		}
		eventBatch.Queue(
			`INSERT INTO events (project_id, issue_id, payload) VALUES ($1, $2, $3)`,
			job.Payload.ProjectID, issueIDs[i], payloadBytes,
		)
	}

	brEvents := tx.SendBatch(ctx, eventBatch)
	for i := range jobs {
		if _, err := brEvents.Exec(); err != nil {
			log.Printf("processor: batch insert event failed at index %d: %v", i, err)
			brEvents.Close()
			return
		}
	}
	brEvents.Close()

	if err := tx.Commit(ctx); err != nil {
		log.Printf("processor: batch commit failed: %v", err)
		return
	}

	// Step 3: Redis pipeline after successful commit — best-effort, non-critical.
	pipe := p.rdb.Pipeline()
	for _, job := range jobs {
		pipe.Incr(ctx, fmt.Sprintf("issue:%s:count", job.Fingerprint))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("processor: redis pipeline failed: %v", err)
	}

	log.Printf("processor: flushed batch of %d event(s)", len(jobs))
}
