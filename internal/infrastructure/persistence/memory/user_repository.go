package memory

import (
	"context"
	domainuser "github.com/jabahum/go-foundation/internal/domain/user"
	"strings"
	"sync"
)

type UserRepository struct {
	mu    sync.RWMutex
	users map[string]*domainuser.User
}

var _ domainuser.Repository = (*UserRepository)(nil)

func NewUserRepository() *UserRepository {
	return &UserRepository{users: map[string]*domainuser.User{}}
}
func clone(u *domainuser.User) *domainuser.User { c := *u; return &c }
func (r *UserRepository) Create(ctx context.Context, u *domainuser.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.users {
		if strings.EqualFold(x.Email, u.Email) {
			return domainuser.ErrEmailExists
		}
	}
	r.users[u.ID] = clone(u)
	return nil
}
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domainuser.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return nil, domainuser.ErrNotFound
	}
	return clone(u), nil
}
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domainuser.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			return clone(u), nil
		}
	}
	return nil, domainuser.ErrNotFound
}
func (r *UserRepository) GetByExternalIdentity(ctx context.Context, provider, subject string) (*domainuser.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if u.AuthProvider == provider && u.ExternalSubject == subject {
			return clone(u), nil
		}
	}
	return nil, domainuser.ErrNotFound
}
func (r *UserRepository) List(ctx context.Context, f domainuser.ListFilter) ([]*domainuser.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []*domainuser.User{}
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
