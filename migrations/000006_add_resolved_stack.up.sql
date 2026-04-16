ALTER TABLE events ADD COLUMN IF NOT EXISTS resolved_stack TEXT;

-- Partial index: covers only unresolved events to keep it small.
-- Used by retroactive resolution query (project_id + release + NULL check).
CREATE INDEX IF NOT EXISTS idx_events_unresolved
    ON events(project_id, (payload->>'release'))
    WHERE resolved_stack IS NULL;
