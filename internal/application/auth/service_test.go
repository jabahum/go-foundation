package auth

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domainauth "github.com/jabahum/go-foundation/internal/domain/auth"
	user "github.com/jabahum/go-foundation/internal/domain/user"
	"github.com/jabahum/go-foundation/internal/infrastructure/persistence/memory"
)

type testHasher struct{}

func (testHasher) Hash(value string) (string, error)       { return value, nil }
func (testHasher) Verify(value, hash string) (bool, error) { return value == hash, nil }

type testIssuer struct{ identities []domainauth.Identity }

func (i *testIssuer) Issue(_ context.Context, identity domainauth.Identity) (*domainauth.Token, error) {
	i.identities = append(i.identities, identity)
	return &domainauth.Token{AccessToken: "access-" + identity.SessionID, ExpiresAt: time.Now().Add(time.Minute)}, nil
}

type testSessionRepository struct {
	mu       sync.Mutex
	sessions map[string]*domainauth.Session
}

func newTestSessionRepository() *testSessionRepository {
	return &testSessionRepository{sessions: make(map[string]*domainauth.Session)}
}

func (r *testSessionRepository) Create(_ context.Context, session *domainauth.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
	return nil
}

func (r *testSessionRepository) FindActiveByTokenHash(_ context.Context, hash []byte, now time.Time) (*domainauth.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, session := range r.sessions {
		if bytes.Equal(session.RefreshTokenHash, hash) && session.RevokedAt == nil && session.ExpiresAt.After(now) {
			return session, nil
		}
	}
	return nil, domainauth.ErrSessionNotFound
}

func (r *testSessionRepository) RotateToken(_ context.Context, id string, oldHash, newHash []byte, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[id]
	if !ok || !bytes.Equal(session.RefreshTokenHash, oldHash) || session.RevokedAt != nil || !session.ExpiresAt.After(now) {
		return domainauth.ErrSessionNotFound
	}
	session.RefreshTokenHash = newHash
	session.LastUsedAt = &now
	return nil
}

func (r *testSessionRepository) RevokeByTokenHash(_ context.Context, hash []byte, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, session := range r.sessions {
		if bytes.Equal(session.RefreshTokenHash, hash) && session.RevokedAt == nil {
			session.RevokedAt = &now
		}
	}
	return nil
}

func (r *testSessionRepository) Revoke(_ context.Context, id, userID string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[id]
	if !ok || session.UserID != userID || session.RevokedAt != nil {
		return domainauth.ErrSessionNotFound
	}
	session.RevokedAt = &now
	return nil
}

func (r *testSessionRepository) ListActiveByUser(_ context.Context, userID string, now time.Time) ([]*domainauth.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]*domainauth.Session, 0)
	for _, session := range r.sessions {
		if session.UserID == userID && session.RevokedAt == nil && session.ExpiresAt.After(now) {
			result = append(result, session)
		}
	}
	return result, nil
}

func newAuthService(t *testing.T) (*Service, *testSessionRepository, *testIssuer, *user.User) {
	t.Helper()
	users := memory.NewUserRepository()
	u, err := user.New("53ad221e-91f9-452b-8851-e3e7765745d8", "Ada", "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	u.PasswordHash = "correct-password"
	if err := users.Create(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	sessions := newTestSessionRepository()
	issuer := &testIssuer{}
	return NewService(users, testHasher{}, issuer, sessions, 30*24*time.Hour), sessions, issuer, u
}

func TestLoginCreatesHashedRefreshSession(t *testing.T) {
	service, sessions, issuer, u := newAuthService(t)
	result, err := service.Login(context.Background(), u.Email, "correct-password", ClientInfo{UserAgent: "test-agent", ClientIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RefreshToken) != 64 || len(result.Session.RefreshTokenHash) != sha256Size {
		t.Fatalf("unexpected refresh token or hash length: %d, %d", len(result.RefreshToken), len(result.Session.RefreshTokenHash))
	}
	if _, exists := sessions.sessions[result.Session.ID]; !exists {
		t.Fatal("session was not persisted")
	}
	if issuer.identities[0].SessionID != result.Session.ID || result.Session.UserAgent != "test-agent" {
		t.Fatalf("session metadata was not propagated: %+v", result.Session)
	}
}

func TestRefreshRotatesTokenAndRejectsReplay(t *testing.T) {
	service, _, _, u := newAuthService(t)
	login, err := service.Login(context.Background(), u.Email, "correct-password", ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := service.Refresh(context.Background(), login.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.RefreshToken == login.RefreshToken || refreshed.Session.ID != login.Session.ID {
		t.Fatal("refresh token was not rotated within the same session")
	}
	if _, err := service.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}

func TestLogoutRevokesRefreshToken(t *testing.T) {
	service, _, _, u := newAuthService(t)
	login, err := service.Login(context.Background(), u.Email, "correct-password", ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(context.Background(), login.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(context.Background(), login.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected revoked token rejection, got %v", err)
	}
}

const sha256Size = 32
