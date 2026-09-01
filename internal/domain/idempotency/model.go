package idempotency

import (
	"context"
	"errors"
	"time"
)

var ErrReservationLost = errors.New("idempotency reservation lost")

type State string

const (
	StateInProgress State = "in_progress"
	StateCompleted  State = "completed"
)

type Disposition int

const (
	DispositionAcquired Disposition = iota
	DispositionReplay
	DispositionConflict
	DispositionInProgress
)

type Record struct {
	ID              string
	ActorID         string
	RPCMethod       string
	KeyHash         []byte
	RequestHash     []byte
	State           State
	OwnerToken      string
	ResponseType    string
	ResponsePayload []byte
	StatusPayload   []byte
	LockedUntil     time.Time
	ExpiresAt       time.Time
}

type AcquireParams struct {
	ID          string
	ActorID     string
	RPCMethod   string
	KeyHash     []byte
	RequestHash []byte
	OwnerToken  string
	Now         time.Time
	LockedUntil time.Time
	ExpiresAt   time.Time
}

type Store interface {
	Acquire(context.Context, AcquireParams) (*Record, Disposition, error)
	Complete(context.Context, string, string, string, []byte, []byte, time.Time) error
	Release(context.Context, string, string) error
}
