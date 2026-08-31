package auth

import (
	"context"
	"errors"
	"time"
)

var ErrSessionNotFound = errors.New("session not found")

type Session struct {
	ID               string
	UserID           string
	RefreshTokenHash []byte
	UserAgent        string
	ClientIP         string
	CreatedAt        time.Time
	LastUsedAt       *time.Time
	ExpiresAt        time.Time
	RevokedAt        *time.Time
}

type SessionRepository interface {
	Create(context.Context, *Session) error
	FindActiveByTokenHash(context.Context, []byte, time.Time) (*Session, error)
	RotateToken(context.Context, string, []byte, []byte, time.Time) error
	RevokeByTokenHash(context.Context, []byte, time.Time) error
	Revoke(context.Context, string, string, time.Time) error
	ListActiveByUser(context.Context, string, time.Time) ([]*Session, error)
}
