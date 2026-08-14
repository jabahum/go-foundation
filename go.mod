module example.com/grpc-clean-starter

go 1.23

require (
	github.com/coreos/go-oidc/v3 v3.12.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.2
	github.com/prometheus/client_golang v1.20.5
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.57.0
	go.opentelemetry.io/otel v1.32.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.32.0
	go.opentelemetry.io/otel/sdk v1.32.0
	golang.org/x/crypto v0.31.0
	google.golang.org/grpc v1.69.2
	google.golang.org/protobuf v1.36.1
)
