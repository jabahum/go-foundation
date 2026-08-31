CREATE TABLE auth_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash BYTEA NOT NULL UNIQUE,
    user_agent TEXT,
    client_ip VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_auth_sessions_user_active
ON auth_sessions (user_id, created_at DESC)
WHERE revoked_at IS NULL;

CREATE INDEX idx_auth_sessions_expiry
ON auth_sessions (expires_at)
WHERE revoked_at IS NULL;
