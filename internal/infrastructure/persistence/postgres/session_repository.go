package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	auth "github.com/jabahum/go-foundation/internal/domain/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepository struct{ db *pgxpool.Pool }

var _ auth.SessionRepository = (*SessionRepository)(nil)

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository { return &SessionRepository{db: db} }

func (r *SessionRepository) Create(ctx context.Context, session *auth.Session) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth_sessions (
			id, user_id, refresh_token_hash, user_agent, client_ip, created_at, expires_at
		) VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,$7)`,
		session.ID, session.UserID, session.RefreshTokenHash, session.UserAgent, session.ClientIP,
		session.CreatedAt, session.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert auth session: %w", err)
	}
	return nil
}

func (r *SessionRepository) FindActiveByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (*auth.Session, error) {
	session := &auth.Session{}
	err := r.db.QueryRow(ctx, `
		SELECT id::text, user_id::text, refresh_token_hash, COALESCE(user_agent,''), COALESCE(client_ip,''),
			created_at, last_used_at, expires_at, revoked_at
		FROM auth_sessions
		WHERE refresh_token_hash=$1 AND revoked_at IS NULL AND expires_at>$2`, tokenHash, now,
	).Scan(
		&session.ID, &session.UserID, &session.RefreshTokenHash, &session.UserAgent, &session.ClientIP,
		&session.CreatedAt, &session.LastUsedAt, &session.ExpiresAt, &session.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find auth session: %w", err)
	}
	return session, nil
}

func (r *SessionRepository) RotateToken(ctx context.Context, sessionID string, oldHash, newHash []byte, now time.Time) error {
	command, err := r.db.Exec(ctx, `
		UPDATE auth_sessions
		SET refresh_token_hash=$1, last_used_at=$2
		WHERE id=$3 AND refresh_token_hash=$4 AND revoked_at IS NULL AND expires_at>$2`,
		newHash, now, sessionID, oldHash,
	)
	if err != nil {
		return fmt.Errorf("rotate refresh token: %w", err)
	}
	if command.RowsAffected() != 1 {
		return auth.ErrSessionNotFound
	}
	return nil
}

func (r *SessionRepository) RevokeByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE auth_sessions SET revoked_at=$1 WHERE refresh_token_hash=$2 AND revoked_at IS NULL`, now, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke auth session by token: %w", err)
	}
	return nil
}

func (r *SessionRepository) Revoke(ctx context.Context, sessionID, userID string, now time.Time) error {
	command, err := r.db.Exec(ctx, `UPDATE auth_sessions SET revoked_at=$1 WHERE id=$2 AND user_id=$3 AND revoked_at IS NULL`, now, sessionID, userID)
	if err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}
	if command.RowsAffected() != 1 {
		return auth.ErrSessionNotFound
	}
	return nil
}

func (r *SessionRepository) ListActiveByUser(ctx context.Context, userID string, now time.Time) ([]*auth.Session, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, user_id::text, COALESCE(user_agent,''), COALESCE(client_ip,''),
			created_at, last_used_at, expires_at, revoked_at
		FROM auth_sessions
		WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>$2
		ORDER BY created_at DESC`, userID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("list auth sessions: %w", err)
	}
	defer rows.Close()
	sessions := make([]*auth.Session, 0)
	for rows.Next() {
		session := &auth.Session{}
		if err := rows.Scan(
			&session.ID, &session.UserID, &session.UserAgent, &session.ClientIP,
			&session.CreatedAt, &session.LastUsedAt, &session.ExpiresAt, &session.RevokedAt,
		); err != nil {
			return nil, fmt.Errorf("scan auth session: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate auth sessions: %w", err)
	}
	return sessions, nil
}
