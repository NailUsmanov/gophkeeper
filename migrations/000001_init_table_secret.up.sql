CREATE TABLE secrets (
    id UUID PRIMARY KEY,
    owner_id TEXT NOT NULL,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    data JSONB NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_secrets_owner_updated
    ON secrets (owner_id, updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_secrets_owner_type
    ON secrets (owner_id, type)
    WHERE deleted_at IS NULL;