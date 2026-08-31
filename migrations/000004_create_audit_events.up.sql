CREATE TABLE audit_events (
    id UUID PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL,
    actor_id UUID,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255),
    request_id VARCHAR(255) NOT NULL,
    rpc_method VARCHAR(255) NOT NULL,
    grpc_code VARCHAR(32) NOT NULL,
    error_reason VARCHAR(150),
    client_ip VARCHAR(255),
    user_agent TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_audit_events_occurred_at ON audit_events (occurred_at DESC, id DESC);
CREATE INDEX idx_audit_events_actor_id ON audit_events (actor_id, occurred_at DESC);
CREATE INDEX idx_audit_events_resource ON audit_events (resource_type, resource_id, occurred_at DESC);
CREATE INDEX idx_audit_events_action ON audit_events (action, occurred_at DESC);

INSERT INTO permissions (name, resource, action, description)
VALUES ('audit:read', 'audit', 'read', 'View the audit trail');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name = 'audit:read'
WHERE r.name = 'super_admin';
