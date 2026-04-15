CREATE TABLE IF NOT EXISTS sourcemap_files (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    release      VARCHAR(128) NOT NULL,
    object_key   TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_used_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(project_id, object_key)
);

CREATE INDEX IF NOT EXISTS idx_sourcemap_files_project_release ON sourcemap_files(project_id, release);
CREATE INDEX IF NOT EXISTS idx_sourcemap_files_last_used ON sourcemap_files(last_used_at);
