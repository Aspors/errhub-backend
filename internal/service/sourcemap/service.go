// Package sourcemap resolves minified stack trace locations to original source
// positions using source maps stored in MinIO. Maps are cached in an LRU cache
// to avoid repeated downloads for hot builds.
package sourcemap

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/Aspors/errhub-backend/internal/storage/s3"
	"github.com/go-sourcemap/sourcemap"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Matches optional http(s)://host/path/ prefix, then filename.js:line:col
var stackRegex = regexp.MustCompile(`(?:https?://[^\s)]*?/)?([^/\s()]+\.js):(\d+):(\d+)`)

type Service struct {
	s3    *s3.Storage
	db    *pgxpool.Pool
	cache *lru.Cache[string, *sourcemap.Consumer]
}

func New(storage *s3.Storage, db *pgxpool.Pool) *Service {
	c, _ := lru.New[string, *sourcemap.Consumer](100)
	return &Service{s3: storage, db: db, cache: c}
}

// ProcessStack replaces minified locations in a stack trace with their original
// source positions. release is the build version string from the EventPayload;
// if empty, falls back to the legacy flat path layout.
func (s *Service) ProcessStack(ctx context.Context, projectID, release, stack string) string {
	resolved := stackRegex.ReplaceAllStringFunc(stack, func(match string) string {
		parts := stackRegex.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}

		filename := parts[1]
		line, _ := strconv.Atoi(parts[2])
		col, _ := strconv.Atoi(parts[3])

		mapFilename := filename + ".map"

		// Try release-scoped path first ({projectId}/{release}/{file}.map),
		// fall back to legacy flat path ({projectId}/{file}.map).
		var objectKey string
		if release != "" {
			objectKey = fmt.Sprintf("%s/%s/%s", projectID, release, mapFilename)
		} else {
			objectKey = fmt.Sprintf("%s/%s", projectID, mapFilename)
		}

		res, err := s.resolve(ctx, objectKey, projectID, release, mapFilename, line, col)
		if err != nil && release != "" {
			// Fallback to legacy path
			legacyKey := fmt.Sprintf("%s/%s", projectID, mapFilename)
			res, err = s.resolve(ctx, legacyKey, projectID, "", mapFilename, line, col)
		}
		if err != nil {
			log.Printf("sourcemap: failed to resolve %s (project=%s release=%q): %v", match, projectID, release, err)
			return match
		}
		return res
	})

	return resolved
}

func (s *Service) resolve(ctx context.Context, objectKey, projectID, release, mapFile string, line, col int) (string, error) {
	var consumer *sourcemap.Consumer

	if val, ok := s.cache.Get(objectKey); ok {
		consumer = val
	} else {
		// Download using the full object key path.
		var data []byte
		var err error
		if release != "" {
			data, err = s.s3.Download(ctx, projectID+"/"+release, mapFile)
		} else {
			data, err = s.s3.Download(ctx, projectID, mapFile)
		}
		if err != nil {
			return "", err
		}

		consumer, err = sourcemap.Parse("", data)
		if err != nil {
			return "", err
		}
		s.cache.Add(objectKey, consumer)
	}

	file, fn, l, c, ok := consumer.Source(line, col)
	if !ok {
		return "", fmt.Errorf("coordinates not found in source map")
	}

	// Update last_used_at so the cleanup worker knows this map is still active.
	go s.touchLastUsed(objectKey)

	// Return just the source location so it slots cleanly into the surrounding
	// "at functionName (<here>)" frame that already exists in the stack string.
	if fn != "" {
		return fmt.Sprintf("%s:%d:%d [%s]", file, l, c, fn), nil
	}
	return fmt.Sprintf("%s:%d:%d", file, l, c), nil
}

func (s *Service) touchLastUsed(objectKey string) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(context.Background(),
		`UPDATE sourcemap_files SET last_used_at = NOW() WHERE object_key = $1`,
		objectKey)
	if err != nil {
		log.Printf("sourcemap: failed to update last_used_at [key=%s]: %v", objectKey, err)
	}
}
