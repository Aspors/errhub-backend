CREATE TABLE IF NOT EXISTS issues (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    fingerprint   CHAR(64) NOT NULL,
    level         VARCHAR(20) NOT NULL,
    error_type    VARCHAR(255) NOT NULL,
    error_message TEXT NOT NULL,
    occurrences   INTEGER NOT NULL DEFAULT 1,
    first_seen    TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_seen     TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    status        VARCHAR(20) NOT NULL DEFAULT 'open',
    UNIQUE(project_id, fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_issues_project_id ON issues(project_id);
CREATE INDEX IF NOT EXISTS idx_issues_last_seen ON issues(last_seen DESC);

ALTER TABLE events ADD COLUMN IF NOT EXISTS issue_id UUID REFERENCES issues(id) ON DELETE CASCADE;
