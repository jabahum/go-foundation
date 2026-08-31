package postgres

import (
	"context"
	"errors"
	"fmt"
	domainuser "github.com/jabahum/go-foundation/internal/domain/user"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct{ db *pgxpool.Pool }

var _ domainuser.Repository = (*UserRepository)(nil)

func NewUserRepository(db *pgxpool.Pool) *UserRepository { return &UserRepository{db: db} }
func (r *UserRepository) Create(ctx context.Context, u *domainuser.User) error {
	_, err := r.db.Exec(ctx, `INSERT INTO users(id,name,email,password_hash,enabled,auth_provider,external_subject,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9)`, u.ID, u.Name, u.Email, u.PasswordHash, u.Enabled, u.AuthProvider, u.ExternalSubject, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return domainuser.ErrEmailExists
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}
func scanUser(row pgx.Row) (*domainuser.User, error) {
	u := new(domainuser.User)
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Enabled, &u.AuthProvider, &u.ExternalSubject, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainuser.ErrNotFound
	}
	return u, err
}
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domainuser.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT id,name,email,COALESCE(password_hash,''),enabled,auth_provider,COALESCE(external_subject,''),created_at,updated_at FROM users WHERE id=$1`, id))
}
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domainuser.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT id,name,email,COALESCE(password_hash,''),enabled,auth_provider,COALESCE(external_subject,''),created_at,updated_at FROM users WHERE email=$1`, email))
}
func (r *UserRepository) GetByExternalIdentity(ctx context.Context, provider, subject string) (*domainuser.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT id,name,email,COALESCE(password_hash,''),enabled,auth_provider,COALESCE(external_subject,''),created_at,updated_at FROM users WHERE auth_provider=$1 AND external_subject=$2`, provider, subject))
}
func (r *UserRepository) List(ctx context.Context, f domainuser.ListFilter) ([]*domainuser.User, error) {
	q := `SELECT id,name,email,COALESCE(password_hash,''),enabled,auth_provider,COALESCE(external_subject,''),created_at,updated_at FROM users WHERE ($1='' OR name ILIKE '%'||$1||'%' OR email ILIKE '%'||$1||'%') AND ($2::timestamptz IS NULL OR (created_at,id) < ($2,$3::uuid)) ORDER BY created_at DESC,id DESC LIMIT $4`
	rows, err := r.db.Query(ctx, q, f.Search, f.AfterCreatedAt, nullID(f.AfterID), f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domainuser.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func nullID(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}
