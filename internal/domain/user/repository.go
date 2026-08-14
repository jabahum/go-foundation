package user

import (
	"context"
	"time"
)

type ListFilter struct {
	Limit          int
	Search         string
	AfterCreatedAt *time.Time
	AfterID        string
}

type Repository interface {
	Create(context.Context, *User) error
	GetByID(context.Context, string) (*User, error)
	GetByEmail(context.Context, string) (*User, error)
	GetByExternalIdentity(context.Context, string, string) (*User, error)
	List(context.Context, ListFilter) ([]*User, error)
}
