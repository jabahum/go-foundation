package rbac

import "context"

type Repository interface {
	ListRoles(context.Context) ([]Role, error)
	ListPermissions(context.Context) ([]Permission, error)
	UserPermissions(context.Context, string) ([]Permission, error)
	UserRoles(context.Context, string) ([]Role, error)
	AssignRole(context.Context, string, string) error
	RemoveRole(context.Context, string, string) error
	AssignPermissionToRole(context.Context, string, string) error
	RemovePermissionFromRole(context.Context, string, string) error
}
