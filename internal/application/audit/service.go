package audit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	audit "github.com/jabahum/go-foundation/internal/domain/audit"
)

var ErrInvalidPageToken = errors.New("invalid page token")

type Service struct{ repo audit.Repository }

type ListInput struct {
	PageSize                 int
	PageToken                string
	ActorID, Action          string
	ResourceType, ResourceID string
}

type ListResult struct {
	Events        []*audit.Event
	NextPageToken string
}

type cursor struct {
	OccurredAt time.Time `json:"occurred_at"`
	ID         string    `json:"id"`
}

func NewService(repo audit.Repository) *Service { return &Service{repo: repo} }

func (s *Service) List(ctx context.Context, in ListInput) (*ListResult, error) {
	limit := in.PageSize
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	filter := audit.ListFilter{
		Limit:        limit + 1,
		ActorID:      strings.TrimSpace(in.ActorID),
		Action:       strings.TrimSpace(in.Action),
		ResourceType: strings.TrimSpace(in.ResourceType),
		ResourceID:   strings.TrimSpace(in.ResourceID),
	}
	if in.PageToken != "" {
		page, err := decodeCursor(in.PageToken)
		if err != nil {
			return nil, ErrInvalidPageToken
		}
		filter.BeforeOccurred = &page.OccurredAt
		filter.BeforeID = page.ID
	}
	events, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := &ListResult{Events: events}
	if len(events) > limit {
		last := events[limit-1]
		result.Events = events[:limit]
		result.NextPageToken = encodeCursor(cursor{OccurredAt: last.OccurredAt, ID: last.ID})
	}
	return result, nil
}

func encodeCursor(value cursor) string {
	data, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(value string) (cursor, error) {
	var page cursor
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return page, err
	}
	if err := json.Unmarshal(data, &page); err != nil || page.ID == "" || page.OccurredAt.IsZero() {
		return page, ErrInvalidPageToken
	}
	return page, nil
}
