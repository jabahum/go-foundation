package user_test

import (
	"context"
	"errors"
	"testing"

	appuser "github.com/jabahum/go-foundation/internal/application/user"
	user "github.com/jabahum/go-foundation/internal/domain/user"
	"github.com/jabahum/go-foundation/internal/infrastructure/auth/local"
	"github.com/jabahum/go-foundation/internal/infrastructure/persistence/memory"
)

func TestCreateUser(t *testing.T) {
	s := appuser.NewService(memory.NewUserRepository(), local.NewPasswordHasher())
	u, err := s.Create(context.Background(), appuser.CreateInput{Name: "Jeremy", Email: "jeremy@example.com", Password: "StrongPass123!"})
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == "" || u.PasswordHash == "" {
		t.Fatal("expected id and password hash")
	}
}

func TestDuplicateUser(t *testing.T) {
	s := appuser.NewService(memory.NewUserRepository(), local.NewPasswordHasher())
	in := appuser.CreateInput{Name: "Jeremy", Email: "jeremy@example.com", Password: "StrongPass123!"}
	if _, err := s.Create(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	_, err := s.Create(context.Background(), in)
	if !errors.Is(err, user.ErrEmailExists) {
		t.Fatalf("expected ErrEmailExists, got %v", err)
	}
}
