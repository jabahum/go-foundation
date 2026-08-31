package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	domainaudit "github.com/jabahum/go-foundation/internal/domain/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct{ db *pgxpool.Pool }

var _ domainaudit.Repository = (*AuditRepository)(nil)

func NewAuditRepository(db *pgxpool.Pool) *AuditRepository { return &AuditRepository{db: db} }

func (r *AuditRepository) Record(ctx context.Context, event *domainaudit.Event) error {
	metadataValue := event.Metadata
	if metadataValue == nil {
		metadataValue = map[string]string{}
	}
	metadata, err := json.Marshal(metadataValue)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO audit_events (
			id, occurred_at, actor_id, action, resource_type, resource_id,
			request_id, rpc_method, grpc_code, error_reason, client_ip, user_agent, metadata
		) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,NULLIF($6,''),$7,$8,$9,NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),$13::jsonb)`,
		event.ID, event.OccurredAt, event.ActorID, event.Action, event.ResourceType, event.ResourceID,
		event.RequestID, event.RPCMethod, event.GRPCCode, event.ErrorReason, event.ClientIP, event.UserAgent, string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

func (r *AuditRepository) List(ctx context.Context, filter domainaudit.ListFilter) ([]*domainaudit.Event, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, occurred_at, COALESCE(actor_id::text,''), action, resource_type,
			COALESCE(resource_id,''), request_id, rpc_method, grpc_code, COALESCE(error_reason,''),
			COALESCE(client_ip,''), COALESCE(user_agent,''), metadata
		FROM audit_events
		WHERE ($1='' OR actor_id::text=$1)
			AND ($2='' OR action=$2)
			AND ($3='' OR resource_type=$3)
			AND ($4='' OR resource_id=$4)
			AND ($5::timestamptz IS NULL OR (occurred_at,id) < ($5,$6::uuid))
		ORDER BY occurred_at DESC, id DESC
		LIMIT $7`,
		filter.ActorID, filter.Action, filter.ResourceType, filter.ResourceID,
		filter.BeforeOccurred, nullID(filter.BeforeID), filter.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	events := make([]*domainaudit.Event, 0)
	for rows.Next() {
		event := &domainaudit.Event{}
		var metadata []byte
		if err := rows.Scan(
			&event.ID, &event.OccurredAt, &event.ActorID, &event.Action, &event.ResourceType,
			&event.ResourceID, &event.RequestID, &event.RPCMethod, &event.GRPCCode, &event.ErrorReason,
			&event.ClientIP, &event.UserAgent, &metadata,
		); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		if err := json.Unmarshal(metadata, &event.Metadata); err != nil {
			return nil, fmt.Errorf("decode audit metadata: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}
	return events, nil
}
