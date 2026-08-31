package auth

import (
	"context"
	"time"
)

type Identity struct {
	UserID    string
	Username  string
	Email     string
	Provider  string
	SessionID string
}

type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

type TokenIssuer interface {
	Issue(context.Context, Identity) (*Token, error)
}
type TokenVerifier interface {
	Verify(context.Context, string) (*Identity, error)
}

type PasswordHasher interface {
	Hash(string) (string, error)
	Verify(string, string) (bool, error)
}
