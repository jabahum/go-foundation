package rbac

import (
	"context"
	domainrbac "example.com/grpc-clean-starter/internal/domain/rbac"
)

type Service struct{ repo domainrbac.Repository }

func NewService(r domainrbac.Repository) *Service { return &Service{repo: r} }
func (s *Service) HasPermission(ctx context.Context, userID, permission string) (bool, error) {
	ps, err := s.repo.UserPermissions(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, p := range ps {
		if p.Name == permission {
			return true, nil
		}
	}
	return false, nil
}
func (s *Service) Roles(ctx context.Context, userID string) ([]domainrbac.Role, error) {
	return s.repo.UserRoles(ctx, userID)
}
func (s *Service) Permissions(ctx context.Context, userID string) ([]domainrbac.Permission, error) {
	return s.repo.UserPermissions(ctx, userID)
}
func (s *Service) ListRoles(ctx context.Context) ([]domainrbac.Role, error) {
	return s.repo.ListRoles(ctx)
}
func (s *Service) ListPermissions(ctx context.Context) ([]domainrbac.Permission, error) {
	return s.repo.ListPermissions(ctx)
}
func (s *Service) AssignRole(ctx context.Context, userID, roleID string) error {
	return s.repo.AssignRole(ctx, userID, roleID)
}
func (s *Service) RemoveRole(ctx context.Context, userID, roleID string) error {
	return s.repo.RemoveRole(ctx, userID, roleID)
}
func (s *Service) AssignPermissionToRole(ctx context.Context, roleID, permissionID string) error {
	return s.repo.AssignPermissionToRole(ctx, roleID, permissionID)
}
func (s *Service) RemovePermissionFromRole(ctx context.Context, roleID, permissionID string) error {
	return s.repo.RemovePermissionFromRole(ctx, roleID, permissionID)
}
