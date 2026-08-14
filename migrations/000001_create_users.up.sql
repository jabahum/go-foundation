CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(320) NOT NULL UNIQUE,
    password_hash TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    auth_provider VARCHAR(50) NOT NULL DEFAULT 'local',
    external_subject VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_users_external_identity
ON users(auth_provider, external_subject)
WHERE external_subject IS NOT NULL;
