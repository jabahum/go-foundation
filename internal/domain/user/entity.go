package user

import (
	"errors"
	"strings"
	"time"
)

type User struct {
	ID              string
	Name            string
	Email           string
	PasswordHash    string
	Enabled         bool
	AuthProvider    string
	ExternalSubject string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func New(id, name, email string) (*User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" {
		return nil, errors.New("name is required")
	}
	if email == "" {
		return nil, errors.New("email is required")
	}
	now := time.Now().UTC()
	return &User{ID: id, Name: name, Email: email, Enabled: true, AuthProvider: "local", CreatedAt: now, UpdatedAt: now}, nil
}
