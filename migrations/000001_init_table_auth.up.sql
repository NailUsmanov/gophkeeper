CREATE TABLE IF NOT EXISTS users(
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    password_hash TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_users_email ON users(email);
