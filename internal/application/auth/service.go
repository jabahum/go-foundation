package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	auth "github.com/jabahum/go-foundation/internal/domain/auth"
	user "github.com/jabahum/go-foundation/internal/domain/user"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

type Service struct {
	users      user.Repository
	hasher     auth.PasswordHasher
	issuer     auth.TokenIssuer
	sessions   auth.SessionRepository
	refreshTTL time.Duration
	now        func() time.Time
}

type ClientInfo struct {
	UserAgent string
	ClientIP  string
}

type LoginResult struct {
	User         *user.User
	Token        *auth.Token
	RefreshToken string
	Session      *auth.Session
}

type RefreshResult struct {
	Token        *auth.Token
	RefreshToken string
	Session      *auth.Session
}

func NewService(users user.Repository, hasher auth.PasswordHasher, issuer auth.TokenIssuer, sessions auth.SessionRepository, refreshTTL time.Duration) *Service {
	return &Service{users: users, hasher: hasher, issuer: issuer, sessions: sessions, refreshTTL: refreshTTL, now: time.Now}
}

func (s *Service) Login(ctx context.Context, email, password string, client ClientInfo) (*LoginResult, error) {
	u, err := s.users.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if !u.Enabled {
		return nil, user.ErrDisabled
	}
	if u.AuthProvider != "local" || u.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}
	ok, err := s.hasher.Verify(password, u.PasswordHash)
	if err != nil || !ok {
		return nil, ErrInvalidCredentials
	}

	now := s.now().UTC()
	rawRefresh, refreshHash, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	session := &auth.Session{
		ID:               uuid.NewString(),
		UserID:           u.ID,
		RefreshTokenHash: refreshHash,
		UserAgent:        client.UserAgent,
		ClientIP:         client.ClientIP,
		CreatedAt:        now,
		ExpiresAt:        now.Add(s.refreshTTL),
	}
	token, err := s.issuer.Issue(ctx, identityFor(u, session.ID))
	if err != nil {
		return nil, err
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, err
	}
	return &LoginResult{User: u, Token: token, RefreshToken: rawRefresh, Session: session}, nil
}

func (s *Service) Refresh(ctx context.Context, rawRefresh string) (*RefreshResult, error) {
	oldHash := hashRefreshToken(rawRefresh)
	now := s.now().UTC()
	session, err := s.sessions.FindActiveByTokenHash(ctx, oldHash, now)
	if err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}
	u, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		if !errors.Is(err, user.ErrNotFound) {
			return nil, err
		}
		_ = s.sessions.RevokeByTokenHash(ctx, oldHash, now)
		return nil, ErrInvalidRefreshToken
	}
	if !u.Enabled || u.AuthProvider != "local" {
		_ = s.sessions.RevokeByTokenHash(ctx, oldHash, now)
		return nil, ErrInvalidRefreshToken
	}

	rawNext, nextHash, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	token, err := s.issuer.Issue(ctx, identityFor(u, session.ID))
	if err != nil {
		return nil, err
	}
	if err := s.sessions.RotateToken(ctx, session.ID, oldHash, nextHash, now); err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}
	session.RefreshTokenHash = nextHash
	session.LastUsedAt = &now
	return &RefreshResult{Token: token, RefreshToken: rawNext, Session: session}, nil
}

// Logout is deliberately idempotent so callers cannot use it to determine
// whether a supplied refresh token was valid.
func (s *Service) Logout(ctx context.Context, rawRefresh string) error {
	return s.sessions.RevokeByTokenHash(ctx, hashRefreshToken(rawRefresh), s.now().UTC())
}

func (s *Service) ListSessions(ctx context.Context, userID string) ([]*auth.Session, error) {
	return s.sessions.ListActiveByUser(ctx, userID, s.now().UTC())
}

func (s *Service) RevokeSession(ctx context.Context, userID, sessionID string) error {
	return s.sessions.Revoke(ctx, sessionID, userID, s.now().UTC())
}

func identityFor(u *user.User, sessionID string) auth.Identity {
	return auth.Identity{UserID: u.ID, Username: u.Name, Email: u.Email, Provider: "local", SessionID: sessionID}
}

func newRefreshToken() (string, []byte, error) {
	data := make([]byte, 48)
	if _, err := rand.Read(data); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(data)
	return raw, hashRefreshToken(raw), nil
}

func hashRefreshToken(raw string) []byte {
	hash := sha256.Sum256([]byte(raw))
	return hash[:]
}
