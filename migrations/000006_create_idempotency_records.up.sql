CREATE TABLE idempotency_records (
    id UUID PRIMARY KEY,
    actor_id UUID NOT NULL,
    rpc_method VARCHAR(255) NOT NULL,
    key_hash BYTEA NOT NULL,
    request_hash BYTEA NOT NULL,
    state VARCHAR(20) NOT NULL,
    owner_token UUID NOT NULL,
    response_type VARCHAR(255),
    response_payload BYTEA,
    status_payload BYTEA,
    locked_until TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT idempotency_records_scope_unique UNIQUE (actor_id, rpc_method, key_hash),
    CONSTRAINT idempotency_records_state_check CHECK (state IN ('in_progress', 'completed'))
);

CREATE INDEX idx_idempotency_records_expiry ON idempotency_records (expires_at);
