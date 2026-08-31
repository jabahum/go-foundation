package policy

import "github.com/jabahum/go-foundation/internal/transport/grpc/interceptor"

func Policies() interceptor.AuthorizationPolicy {
	return interceptor.AuthorizationPolicy{
		"/audit.v1.AuditService/ListAuditEvents":        "audit:read",
		"/user.v1.UserService/CreateUser":               "users:create",
		"/user.v1.UserService/GetUser":                  "users:read",
		"/user.v1.UserService/ListUsers":                "users:read",
		"/rbac.v1.RBACService/ListRoles":                "roles:read",
		"/rbac.v1.RBACService/ListPermissions":          "roles:read",
		"/rbac.v1.RBACService/AssignRole":               "roles:assign",
		"/rbac.v1.RBACService/RemoveRole":               "roles:assign",
		"/rbac.v1.RBACService/AssignPermissionToRole":   "roles:manage",
		"/rbac.v1.RBACService/RemovePermissionFromRole": "roles:manage",
	}
}
