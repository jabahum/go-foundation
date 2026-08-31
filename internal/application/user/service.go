package user

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	auth "github.com/jabahum/go-foundation/internal/domain/auth"
	user "github.com/jabahum/go-foundation/internal/domain/user"
)

type Service struct {
	repo   user.Repository
	hasher auth.PasswordHasher
}

var ErrInvalidPageToken = errors.New("invalid page token")

type CreateInput struct{ Name, Email, Password string }
type ListInput struct {
	PageSize          int
	PageToken, Search string
}
type ListResult struct {
	Users         []*user.User
	NextPageToken string
}
type cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func NewService(r user.Repository, h auth.PasswordHasher) *Service {
	return &Service{repo: r, hasher: h}
}
func (s *Service) Create(ctx context.Context, in CreateInput) (*user.User, error) {
	if strings.TrimSpace(in.Password) == "" {
		return nil, fmt.Errorf("password is required")
	}
	if len(in.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}
	u, err := user.New(uuid.NewString(), in.Name, in.Email)
	if err != nil {
		return nil, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	u.PasswordHash = hash
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
func (s *Service) Get(ctx context.Context, id string) (*user.User, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, user.ErrInvalidUserID
	}
	return s.repo.GetByID(ctx, id)
}
func (s *Service) List(ctx context.Context, in ListInput) (*ListResult, error) {
	limit := in.PageSize
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	f := user.ListFilter{Limit: limit + 1, Search: strings.TrimSpace(in.Search)}
	if in.PageToken != "" {
		c, err := decodeCursor(in.PageToken)
		if err != nil {
			return nil, ErrInvalidPageToken
		}
		f.AfterCreatedAt = &c.CreatedAt
		f.AfterID = c.ID
	}
	users, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	res := &ListResult{Users: users}
	if len(users) > limit {
		last := users[limit-1]
		res.Users = users[:limit]
		res.NextPageToken = encodeCursor(cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return res, nil
}
func encodeCursor(c cursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}
func decodeCursor(v string) (cursor, error) {
	var c cursor
	b, e := base64.RawURLEncoding.DecodeString(v)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e != nil || c.ID == "" || c.CreatedAt.IsZero() {
		return c, errors.New("bad cursor")
	}
	return c, nil
}
