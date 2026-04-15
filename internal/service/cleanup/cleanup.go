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
		run(db, storage)

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				run(db, storage)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func run(db *pgxpool.Pool, storage *s3.Storage) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	rows, err := db.Query(ctx, `
		DELETE FROM sourcemap_files
		WHERE created_at  < NOW() - INTERVAL '3 days'
		  AND last_used_at < NOW() - INTERVAL '7 days'
		RETURNING object_key`)
	if err != nil {
		log.Printf("cleanup: query failed: %v", err)
		return
	}
	defer rows.Close()

	var deleted int
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			continue
		}
		if err := storage.Delete(ctx, key); err != nil {
			log.Printf("cleanup: failed to delete object [key=%s]: %v", key, err)
			continue
		}
		deleted++
	}

	if deleted > 0 {
		log.Printf("cleanup: removed %d stale source map(s)", deleted)
	}
}
