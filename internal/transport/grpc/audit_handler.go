package grpc

import (
	"context"
	"errors"

	auditv1 "github.com/jabahum/go-foundation/gen/proto/audit/v1"
	appaudit "github.com/jabahum/go-foundation/internal/application/audit"
	"github.com/jabahum/go-foundation/internal/transport/grpc/apierror"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AuditHandler struct {
	auditv1.UnimplementedAuditServiceServer
	service *appaudit.Service
}

func NewAuditHandler(service *appaudit.Service) *AuditHandler { return &AuditHandler{service: service} }

func (h *AuditHandler) ListAuditEvents(ctx context.Context, request *auditv1.ListAuditEventsRequest) (*auditv1.ListAuditEventsResponse, error) {
	result, err := h.service.List(ctx, appaudit.ListInput{
		PageSize:     int(request.GetPageSize()),
		PageToken:    request.GetPageToken(),
		ActorID:      request.GetActorId(),
		Action:       request.GetAction(),
		ResourceType: request.GetResourceType(),
		ResourceID:   request.GetResourceId(),
	})
	if err != nil {
		if errors.Is(err, appaudit.ErrInvalidPageToken) {
			return nil, apierror.InvalidArgument("PAGE_TOKEN_INVALID", "invalid page token", apierror.FieldViolation{Field: "page_token", Description: "must be a valid page token"})
		}
		return nil, apierror.New(codes.Internal, "AUDIT_LIST_FAILED", "list audit events failed")
	}

	response := &auditv1.ListAuditEventsResponse{NextPageToken: result.NextPageToken}
	response.Events = make([]*auditv1.AuditEvent, 0, len(result.Events))
	for _, event := range result.Events {
		response.Events = append(response.Events, &auditv1.AuditEvent{
			Id:           event.ID,
			OccurredAt:   timestamppb.New(event.OccurredAt),
			ActorId:      event.ActorID,
			Action:       event.Action,
			ResourceType: event.ResourceType,
			ResourceId:   event.ResourceID,
			RequestId:    event.RequestID,
			RpcMethod:    event.RPCMethod,
			GrpcCode:     event.GRPCCode,
			ErrorReason:  event.ErrorReason,
			ClientIp:     event.ClientIP,
			UserAgent:    event.UserAgent,
			Metadata:     event.Metadata,
		})
	}
	return response, nil
}
