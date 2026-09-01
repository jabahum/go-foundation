package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	idempotency "github.com/jabahum/go-foundation/internal/domain/idempotency"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IdempotencyStore struct{ db *pgxpool.Pool }

var _ idempotency.Store = (*IdempotencyStore)(nil)

func NewIdempotencyStore(db *pgxpool.Pool) *IdempotencyStore { return &IdempotencyStore{db: db} }

func (s *IdempotencyStore) Acquire(ctx context.Context, params idempotency.AcquireParams) (*idempotency.Record, idempotency.Disposition, error) {
	s.cleanupExpired(ctx, params.Now)
	record := &idempotency.Record{}
	err := s.db.QueryRow(ctx, `
		INSERT INTO idempotency_records (
			id, actor_id, rpc_method, key_hash, request_hash, state, owner_token,
			locked_until, expires_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,'in_progress',$6,$7,$8,$9,$9)
		ON CONFLICT (actor_id, rpc_method, key_hash) DO NOTHING
		RETURNING id::text, actor_id::text, rpc_method, key_hash, request_hash, state,
			owner_token::text, COALESCE(response_type,''), response_payload, status_payload,
			locked_until, expires_at`,
		params.ID, params.ActorID, params.RPCMethod, params.KeyHash, params.RequestHash,
		params.OwnerToken, params.LockedUntil, params.ExpiresAt, params.Now,
	).Scan(recordFields(record)...)
	if err == nil {
		return record, idempotency.DispositionAcquired, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, fmt.Errorf("reserve idempotency key: %w", err)
	}

	record, err = s.find(ctx, params.ActorID, params.RPCMethod, params.KeyHash)
	if err != nil {
		return nil, 0, err
	}
	if !record.ExpiresAt.After(params.Now) || (record.State == idempotency.StateInProgress && !record.LockedUntil.After(params.Now)) {
		replaced := &idempotency.Record{}
		err = s.db.QueryRow(ctx, `
			UPDATE idempotency_records
			SET request_hash=$1, state='in_progress', owner_token=$2, response_type=NULL,
				response_payload=NULL, status_payload=NULL, locked_until=$3, expires_at=$4, updated_at=$5
			WHERE id=$6 AND (expires_at<=$5 OR (state='in_progress' AND locked_until<=$5))
			RETURNING id::text, actor_id::text, rpc_method, key_hash, request_hash, state,
				owner_token::text, COALESCE(response_type,''), response_payload, status_payload,
				locked_until, expires_at`,
			params.RequestHash, params.OwnerToken, params.LockedUntil, params.ExpiresAt, params.Now, record.ID,
		).Scan(recordFields(replaced)...)
		if err == nil {
			return replaced, idempotency.DispositionAcquired, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, fmt.Errorf("take over idempotency reservation: %w", err)
		}
		record, err = s.find(ctx, params.ActorID, params.RPCMethod, params.KeyHash)
		if err != nil {
			return nil, 0, err
		}
	}

	if !bytes.Equal(record.RequestHash, params.RequestHash) {
		return record, idempotency.DispositionConflict, nil
	}
	if record.State == idempotency.StateCompleted {
		return record, idempotency.DispositionReplay, nil
	}
	return record, idempotency.DispositionInProgress, nil
}

func (s *IdempotencyStore) Complete(ctx context.Context, id, ownerToken, responseType string, responsePayload, statusPayload []byte, now time.Time) error {
	command, err := s.db.Exec(ctx, `
		UPDATE idempotency_records
		SET state='completed', response_type=NULLIF($1,''), response_payload=$2,
			status_payload=$3, locked_until=NULL, updated_at=$4
		WHERE id=$5 AND owner_token=$6 AND state='in_progress'`,
		responseType, responsePayload, statusPayload, now, id, ownerToken,
	)
	if err != nil {
		return fmt.Errorf("complete idempotency record: %w", err)
	}
	if command.RowsAffected() != 1 {
		return idempotency.ErrReservationLost
	}
	return nil
}

func (s *IdempotencyStore) Release(ctx context.Context, id, ownerToken string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM idempotency_records WHERE id=$1 AND owner_token=$2 AND state='in_progress'`, id, ownerToken)
	if err != nil {
		return fmt.Errorf("release idempotency record: %w", err)
	}
	return nil
}

func (s *IdempotencyStore) find(ctx context.Context, actorID, method string, keyHash []byte) (*idempotency.Record, error) {
	record := &idempotency.Record{}
	err := s.db.QueryRow(ctx, `
		SELECT id::text, actor_id::text, rpc_method, key_hash, request_hash, state,
			owner_token::text, COALESCE(response_type,''), response_payload, status_payload,
			COALESCE(locked_until, expires_at), expires_at
		FROM idempotency_records
		WHERE actor_id=$1 AND rpc_method=$2 AND key_hash=$3`, actorID, method, keyHash,
	).Scan(recordFields(record)...)
	if err != nil {
		return nil, fmt.Errorf("load idempotency record: %w", err)
	}
	return record, nil
}

func (s *IdempotencyStore) cleanupExpired(ctx context.Context, now time.Time) {
	_, _ = s.db.Exec(ctx, `
		DELETE FROM idempotency_records
		WHERE id IN (
			SELECT id FROM idempotency_records WHERE expires_at<=$1 ORDER BY expires_at LIMIT 100
		)`, now)
}

func recordFields(record *idempotency.Record) []any {
	return []any{
		&record.ID, &record.ActorID, &record.RPCMethod, &record.KeyHash, &record.RequestHash,
		&record.State, &record.OwnerToken, &record.ResponseType, &record.ResponsePayload,
		&record.StatusPayload, &record.LockedUntil, &record.ExpiresAt,
	}
}
