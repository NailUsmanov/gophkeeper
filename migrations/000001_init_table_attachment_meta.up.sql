CREATE TABLE IF NOT EXISTS attachments_meta(
    id UUID PRIMARY KEY,
    file_name TEXT NOT NULL,
    secret_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    size BIGINT      NOT NULL DEFAULT 0,
    content_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')

    CHECK (size >= 0),
    CHECK (char_length(file_name)    BETWEEN 1 AND 255),
    CHECK (char_length(content_type) BETWEEN 1 AND 255)
);

CREATE INDEX IF NOT EXISTS idx_attachments_meta_secret_date
    ON attachments_meta (secret_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_attachments_meta_owner_date
    ON attachments_meta (owner_id, created_at DESC);