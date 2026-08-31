package grpc

import (
	"context"

	rbacv1 "github.com/jabahum/go-foundation/gen/proto/rbac/v1"
	apprbac "github.com/jabahum/go-foundation/internal/application/rbac"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RBACHandler struct {
	rbacv1.UnimplementedRBACServiceServer
	svc *apprbac.Service
}

func NewRBACHandler(s *apprbac.Service) *RBACHandler { return &RBACHandler{svc: s} }
func (h *RBACHandler) ListRoles(ctx context.Context, _ *rbacv1.ListRolesRequest) (*rbacv1.ListRolesResponse, error) {
	xs, e := h.svc.ListRoles(ctx)
	if e != nil {
		return nil, status.Error(codes.Internal, "list roles failed")
	}
	out := make([]*rbacv1.Role, 0, len(xs))
	for _, x := range xs {
		out = append(out, &rbacv1.Role{Id: x.ID, Name: x.Name, Description: x.Description})
	}
	return &rbacv1.ListRolesResponse{Roles: out}, nil
}
func (h *RBACHandler) ListPermissions(ctx context.Context, _ *rbacv1.ListPermissionsRequest) (*rbacv1.ListPermissionsResponse, error) {
	xs, e := h.svc.ListPermissions(ctx)
	if e != nil {
		return nil, status.Error(codes.Internal, "list permissions failed")
	}
	out := make([]*rbacv1.Permission, 0, len(xs))
	for _, x := range xs {
		out = append(out, &rbacv1.Permission{Id: x.ID, Name: x.Name, Resource: x.Resource, Action: x.Action, Description: x.Description})
	}
	return &rbacv1.ListPermissionsResponse{Permissions: out}, nil
}
func (h *RBACHandler) AssignRole(ctx context.Context, r *rbacv1.AssignRoleRequest) (*rbacv1.AssignRoleResponse, error) {
	if e := h.svc.AssignRole(ctx, r.GetUserId(), r.GetRoleId()); e != nil {
		return nil, status.Error(codes.Internal, "assign role failed")
	}
	return &rbacv1.AssignRoleResponse{}, nil
}
func (h *RBACHandler) RemoveRole(ctx context.Context, r *rbacv1.RemoveRoleRequest) (*rbacv1.RemoveRoleResponse, error) {
	if e := h.svc.RemoveRole(ctx, r.GetUserId(), r.GetRoleId()); e != nil {
		return nil, status.Error(codes.Internal, "remove role failed")
	}
	return &rbacv1.RemoveRoleResponse{}, nil
}
func (h *RBACHandler) AssignPermissionToRole(ctx context.Context, r *rbacv1.AssignPermissionToRoleRequest) (*rbacv1.AssignPermissionToRoleResponse, error) {
	if e := h.svc.AssignPermissionToRole(ctx, r.GetRoleId(), r.GetPermissionId()); e != nil {
		return nil, status.Error(codes.Internal, "assign permission failed")
	}
	return &rbacv1.AssignPermissionToRoleResponse{}, nil
}
func (h *RBACHandler) RemovePermissionFromRole(ctx context.Context, r *rbacv1.RemovePermissionFromRoleRequest) (*rbacv1.RemovePermissionFromRoleResponse, error) {
	if e := h.svc.RemovePermissionFromRole(ctx, r.GetRoleId(), r.GetPermissionId()); e != nil {
		return nil, status.Error(codes.Internal, "remove permission failed")
	}
	return &rbacv1.RemovePermissionFromRoleResponse{}, nil
}
