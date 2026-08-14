package user

import "errors"

var (
	ErrNotFound      = errors.New("user not found")
	ErrEmailExists   = errors.New("email already exists")
	ErrInvalidUserID = errors.New("invalid user id")
	ErrDisabled      = errors.New("user disabled")
)
