# go-foundation

A production-oriented Go foundation using gRPC, Protocol Buffers, Clean Architecture, PostgreSQL, local RBAC, audit trails, local JWT authentication, optional OIDC/Keycloak authentication, Prometheus, OpenTelemetry, Jaeger, Docker Compose, migrations, pagination, health checks, graceful shutdown, structured logging, and tests.

## Architecture

```text
Client
  -> gRPC / Protobuf or HTTP / JSON gateway
  -> request ID
  -> authentication (local JWT or optional OIDC)
  -> local RBAC permission lookup
  -> audit capture for mutations
  -> metrics / logging / recovery
  -> transport handler
  -> application use case
  -> domain
  -> repository port
  -> PostgreSQL
```

Authorization is always local. Enabling Keycloak/OIDC only changes how identity is established; roles and permissions remain in the starter database.

## Requirements

- Go 1.23+
- Docker + Docker Compose
- OpenSSL
- Optional local tooling: Buf, grpcurl, golang-migrate

Install developer tools:

```bash
make tools
```

## Quick start

The Compose configuration contains development credentials only. Change them before using this outside local development.

```bash
make docker-up
make logs
```

Services:

- gRPC: `localhost:50051`
- HTTP/JSON gateway: `localhost:8080`
- interactive API documentation: `localhost:8080/docs/`
- application metrics: `localhost:9090/metrics`
- Prometheus: `localhost:9091`
- Jaeger: `localhost:16686`
- Grafana: `localhost:3000` (`admin` / `admin`)
- PostgreSQL: `localhost:5432`

The development bootstrap administrator is:

```text
admin@example.com
ChangeMe123!
```

## Login

```bash
grpcurl -plaintext \
  -d '{"email":"admin@example.com","password":"ChangeMe123!"}' \
  localhost:50051 \
  auth.v1.AuthService/Login
```

Copy the returned token:

```bash
export ACCESS_TOKEN='...'
```

The same API is available over HTTP/JSON:

```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"ChangeMe123!"}'
```

Inspect the current identity and effective RBAC:

```bash
grpcurl -plaintext \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  localhost:50051 \
  auth.v1.AuthService/Me
```

## Create a user

```bash
grpcurl -plaintext \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -d '{"name":"Jeremy","email":"jeremy@example.com","password":"StrongPass123!"}' \
  localhost:50051 \
  user.v1.UserService/CreateUser
```

## List users with pagination

```bash
grpcurl -plaintext \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -d '{"pageSize":20,"search":"jeremy"}' \
  localhost:50051 \
  user.v1.UserService/ListUsers
```

Pass the returned `nextPageToken` into the next request.

## Manage RBAC

List roles:

```bash
grpcurl -plaintext \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  localhost:50051 \
  rbac.v1.RBACService/ListRoles
```

List permissions:

```bash
grpcurl -plaintext \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  localhost:50051 \
  rbac.v1.RBACService/ListPermissions
```

Assign a role using IDs returned from the APIs:

```bash
grpcurl -plaintext \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -d '{"userId":"USER_UUID","roleId":"ROLE_UUID"}' \
  localhost:50051 \
  rbac.v1.RBACService/AssignRole
```

The gRPC authorization policy checks permissions, never hard-coded role names.

## Audit trail

Authorized user creation and RBAC mutations are recorded in the append-only `audit_events` table. Each event contains the actor, action, target, request ID, RPC method, gRPC outcome, structured error reason, client address, user agent, timestamp, and explicitly allow-listed metadata. Complete request bodies are never serialized, so passwords and access tokens cannot enter the audit trail.

Only callers with `audit:read` can query the trail; the default `super_admin` role receives that permission. Results use cursor pagination and can be filtered by actor, action, resource type, or resource ID:

```bash
grpcurl -plaintext \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -d '{"pageSize":50,"action":"role.assign"}' \
  localhost:50051 \
  audit.v1.AuditService/ListAuditEvents
```

The equivalent HTTP endpoint is `GET /v1/audit-events`. Audit persistence failures are logged with the request ID but do not rewrite an already-completed business operation into a client-visible failure.

## Default permissions

- `users:read`
- `users:create`
- `users:update`
- `users:disable`
- `users:delete`
- `roles:read`
- `roles:manage`
- `roles:assign`
- `audit:read`

Default roles are `super_admin`, `user_admin`, and `viewer`.

## Optional OIDC / Keycloak

Local authentication is enabled by default. To additionally accept OIDC access tokens:

```dotenv
OIDC_ENABLED=true
OIDC_ISSUER_URL=https://auth.example.org/realms/example
OIDC_CLIENT_ID=grpc-api
```

External identities must be linked to a local user so local RBAC remains authoritative. The relevant fields are `users.auth_provider='oidc'` and `users.external_subject=<OIDC sub>`.

Example linking an existing user:

```sql
UPDATE users
SET auth_provider = 'oidc',
    external_subject = 'OIDC-SUBJECT-HERE',
    password_hash = NULL
WHERE email = 'person@example.com';
```

If you want both local and external login for the same human, model identities in a separate `user_identities` table instead of replacing these fields; that is intentionally left as the first extension point for applications that need account linking.

## Protobuf

```bash
make proto
```

Contracts are versioned under:

```text
api/proto/auth/v1
api/proto/audit/v1
api/proto/user/v1
api/proto/rbac/v1
```

Generated Go and gRPC-Gateway code goes under `gen/proto`. The generated OpenAPI document is embedded into the server from `internal/transport/http/docs/go-foundation.swagger.json`; generated files are ignored by Git.

HTTP routes are declared with `google.api.http` annotations in the protobuf service definitions. Requests sent to the gateway on port `8080` pass through the existing gRPC authentication and authorization interceptors. For example:

```bash
curl http://localhost:8080/v1/users \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Swagger UI and its pinned assets are embedded in the application binary and served at `/docs/`; the underlying specification is available at `/openapi.json`. Documentation metadata and security declarations live in `api/openapi.yaml`, keeping presentation-only OpenAPI options out of the protobuf contracts. Set `DOCS_ENABLED=false` to disable both documentation endpoints, such as in a production deployment where the API surface should not be published.

## Request validation and errors

Request validation rules are declared alongside fields in the protobuf contracts using Protovalidate. A unary interceptor applies them consistently to native gRPC and HTTP/JSON requests before authorization and handler execution.

Errors use `google.rpc.Status` with structured details:

- `google.rpc.ErrorInfo` provides a stable reason, the `go-foundation` domain, and request ID metadata.
- `google.rpc.BadRequest` identifies every invalid field and its validation failure.
- `google.rpc.ResourceInfo` identifies resources involved in not-found and conflict errors.

The gRPC-Gateway renders the same details as JSON, so clients can branch on `ErrorInfo.reason` instead of matching human-readable messages. Unexpected Go errors are converted to a generic internal error and are never returned verbatim.

## Database migrations

```bash
make migrate-up
make migrate-down
make migrate-create name=add_something
```

## Testing

```bash
make test
```

Unit tests use the in-memory repository where appropriate. PostgreSQL integration tests can be added under `tests/integration` without changing application code.

## Foundry

The versioned foundation archive is available through this repository's Foundry registry:

```bash
foundry template update \
  --registry https://raw.githubusercontent.com/jabahum/go-foundation/main/registry.json

foundry new my-service \
  --template go-grpc-clean \
  --version 1.0.0 \
  --module github.com/example/my-service
```

Foundry verifies the release archive against the SHA-256 digest in `registry.json` before caching it.

## Health and operations

Check standard gRPC health:

```bash
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
```

HTTP liveness for the operations listener:

```bash
curl http://localhost:9090/livez
```

The server marks gRPC health `NOT_SERVING` before graceful shutdown.

## Security notes

- Passwords use Argon2id.
- Local JWTs use RS256.
- Development RSA keys are generated by `make keys` and are not committed.
- Never commit production private keys.
- Tokens do not contain RBAC permissions; effective permissions are read from the database so revocation takes effect immediately.
- Do not log access tokens, passwords, authorization headers, or private keys.
- The bootstrap admin environment variables are for initialization and local development; use secret management in production.

## Suggested production extensions

This starter intentionally gives you the core service platform. Depending on the application, typical next extensions are refresh-token/session rotation, a separate `user_identities` table for multi-provider account linking, Redis permission caching with invalidation, rate limiting, database integration tests via Testcontainers, TLS/mTLS, and generated API documentation.
