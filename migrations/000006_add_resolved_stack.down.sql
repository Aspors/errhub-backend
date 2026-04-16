DROP INDEX IF EXISTS idx_events_unresolved;
ALTER TABLE events DROP COLUMN IF EXISTS resolved_stack;
