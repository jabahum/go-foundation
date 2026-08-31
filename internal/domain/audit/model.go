package audit

import (
	"context"
	"time"
)

// Event is an immutable record of a security-sensitive operation.
type Event struct {
	ID           string
	OccurredAt   time.Time
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	RequestID    string
	RPCMethod    string
	GRPCCode     string
	ErrorReason  string
	ClientIP     string
	UserAgent    string
	Metadata     map[string]string
}

type ListFilter struct {
	Limit          int
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     string
	BeforeOccurred *time.Time
	BeforeID       string
}

type Repository interface {
	Record(context.Context, *Event) error
	List(context.Context, ListFilter) ([]*Event, error)
}
