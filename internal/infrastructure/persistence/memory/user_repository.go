package memory

import (
	"context"
	"strings"
	"sync"

	user "github.com/jabahum/go-foundation/internal/domain/user"
)

type UserRepository struct {
	mu    sync.RWMutex
	users map[string]*user.User
}

var _ user.Repository = (*UserRepository)(nil)

func NewUserRepository() *UserRepository {
	return &UserRepository{users: map[string]*user.User{}}
}
func clone(u *user.User) *user.User { c := *u; return &c }
func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.users {
		if strings.EqualFold(x.Email, u.Email) {
			return user.ErrEmailExists
		}
	}
	r.users[u.ID] = clone(u)
	return nil
}
func (r *UserRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return nil, user.ErrNotFound
	}
	return clone(u), nil
}
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			return clone(u), nil
		}
	}
	return nil, user.ErrNotFound
}
func (r *UserRepository) GetByExternalIdentity(ctx context.Context, provider, subject string) (*user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if u.AuthProvider == provider && u.ExternalSubject == subject {
			return clone(u), nil
		}
	}
	return nil, user.ErrNotFound
}
func (r *UserRepository) List(ctx context.Context, f user.ListFilter) ([]*user.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []*user.User{}
	for _, u := range r.users {
		if f.Search != "" && !strings.Contains(strings.ToLower(u.Name+" "+u.Email), strings.ToLower(f.Search)) {
			continue
		}
		out = append(out, clone(u))
		if len(out) >= f.Limit {
			break
		}
	}
	return out, nil
}
