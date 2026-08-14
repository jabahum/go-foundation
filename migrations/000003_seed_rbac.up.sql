INSERT INTO permissions (name, resource, action, description) VALUES
('users:read', 'users', 'read', 'View users'),
('users:create', 'users', 'create', 'Create users'),
('users:update', 'users', 'update', 'Update users'),
('users:disable', 'users', 'disable', 'Enable or disable users'),
('users:delete', 'users', 'delete', 'Delete users'),
('roles:read', 'roles', 'read', 'View roles'),
('roles:manage', 'roles', 'manage', 'Create and update roles'),
('roles:assign', 'roles', 'assign', 'Assign roles to users');

INSERT INTO roles (name, description) VALUES
('super_admin', 'All starter permissions'),
('user_admin', 'User administration permissions'),
('viewer', 'Read-only user access');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.name = 'super_admin';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r JOIN permissions p ON p.name IN ('users:read','users:create','users:update','users:disable','roles:read')
WHERE r.name = 'user_admin';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.name = 'users:read' WHERE r.name = 'viewer';
