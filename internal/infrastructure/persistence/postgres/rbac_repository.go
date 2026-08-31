package postgres

import (
	"context"
	domainrbac "github.com/jabahum/go-foundation/internal/domain/rbac"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RBACRepository struct{ db *pgxpool.Pool }

var _ domainrbac.Repository = (*RBACRepository)(nil)

func NewRBACRepository(db *pgxpool.Pool) *RBACRepository { return &RBACRepository{db: db} }
func (r *RBACRepository) UserPermissions(ctx context.Context, userID string) ([]domainrbac.Permission, error) {
	rows, err := r.db.Query(ctx, `SELECT DISTINCT p.id,p.name,p.resource,p.action,COALESCE(p.description,'') FROM user_roles ur JOIN role_permissions rp ON rp.role_id=ur.role_id JOIN permissions p ON p.id=rp.permission_id WHERE ur.user_id=$1 ORDER BY p.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domainrbac.Permission
	for rows.Next() {
		var p domainrbac.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.Description); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *RBACRepository) UserRoles(ctx context.Context, userID string) ([]domainrbac.Role, error) {
	rows, err := r.db.Query(ctx, `SELECT r.id,r.name,COALESCE(r.description,'') FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE ur.user_id=$1 ORDER BY r.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domainrbac.Role
	for rows.Next() {
		var x domainrbac.Role
		if err := rows.Scan(&x.ID, &x.Name, &x.Description); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *RBACRepository) AssignRole(ctx context.Context, u, role string) error {
	_, e := r.db.Exec(ctx, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, u, role)
	return e
}
func (r *RBACRepository) RemoveRole(ctx context.Context, u, role string) error {
	_, e := r.db.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1 AND role_id=$2`, u, role)
	return e
}
func (r *RBACRepository) AssignPermissionToRole(ctx context.Context, role, p string) error {
	_, e := r.db.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, role, p)
	return e
}
func (r *RBACRepository) RemovePermissionFromRole(ctx context.Context, role, p string) error {
	_, e := r.db.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id=$2`, role, p)
	return e
}
func (r *RBACRepository) ListRoles(ctx context.Context) ([]domainrbac.Role, error) {
	rows, err := r.db.Query(ctx, `SELECT id,name,COALESCE(description,'') FROM roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domainrbac.Role
	for rows.Next() {
		var x domainrbac.Role
		if err := rows.Scan(&x.ID, &x.Name, &x.Description); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *RBACRepository) ListPermissions(ctx context.Context) ([]domainrbac.Permission, error) {
	rows, err := r.db.Query(ctx, `SELECT id,name,resource,action,COALESCE(description,'') FROM permissions ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domainrbac.Permission
	for rows.Next() {
		var x domainrbac.Permission
		if err := rows.Scan(&x.ID, &x.Name, &x.Resource, &x.Action, &x.Description); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
