package audit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	appaudit "github.com/jabahum/go-foundation/internal/application/audit"
	audit "github.com/jabahum/go-foundation/internal/domain/audit"
)

type repository struct {
	events     []*audit.Event
	lastFilter audit.ListFilter
}

func (r *repository) Record(context.Context, *audit.Event) error { return nil }
func (r *repository) List(_ context.Context, filter audit.ListFilter) ([]*audit.Event, error) {
	r.lastFilter = filter
	return r.events, nil
}

func TestListUsesCursorPagination(t *testing.T) {
	now := time.Now().UTC()
	repo := &repository{events: []*audit.Event{
		{ID: "1", OccurredAt: now},
		{ID: "2", OccurredAt: now.Add(-time.Second)},
		{ID: "3", OccurredAt: now.Add(-2 * time.Second)},
	}}
	service := appaudit.NewService(repo)

	first, err := service.List(context.Background(), appaudit.ListInput{PageSize: 2, Action: " role.assign "})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Events) != 2 || first.NextPageToken == "" || repo.lastFilter.Action != "role.assign" || repo.lastFilter.Limit != 3 {
		t.Fatalf("unexpected first page: %+v, filter: %+v", first, repo.lastFilter)
	}

	_, err = service.List(context.Background(), appaudit.ListInput{PageSize: 2, PageToken: first.NextPageToken})
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastFilter.BeforeOccurred == nil || repo.lastFilter.BeforeID != "2" {
		t.Fatalf("cursor filter: %+v", repo.lastFilter)
	}
}

func TestListRejectsInvalidPageToken(t *testing.T) {
	service := appaudit.NewService(&repository{})
	_, err := service.List(context.Background(), appaudit.ListInput{PageToken: "not-a-cursor"})
	if !errors.Is(err, appaudit.ErrInvalidPageToken) {
		t.Fatalf("expected invalid page token, got %v", err)
	}
}
