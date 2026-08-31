package bootstrap

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	domainauth "github.com/jabahum/go-foundation/internal/domain/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

func EnsureAdmin(ctx context.Context, db *pgxpool.Pool, hasher domainauth.PasswordHasher, email, password, name string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil
	}
	if name == "" {
		name = "Starter Admin"
	}
	hash, err := hasher.Hash(password)
	if err != nil {
		return err
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err = db.Exec(ctx, `INSERT INTO users(id,name,email,password_hash,enabled,auth_provider,created_at,updated_at) VALUES($1,$2,$3,$4,true,'local',$5,$5) ON CONFLICT(email) DO NOTHING`, id, name, email, hash, now)
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	_, err = db.Exec(ctx, `INSERT INTO user_roles(user_id,role_id) SELECT u.id,r.id FROM users u CROSS JOIN roles r WHERE u.email=$1 AND r.name='super_admin' ON CONFLICT DO NOTHING`, email)
	if err != nil {
		return fmt.Errorf("bootstrap admin role: %w", err)
	}
	return nil
}
