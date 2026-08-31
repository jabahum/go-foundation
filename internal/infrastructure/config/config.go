package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	App           AppConfig
	GRPC          GRPCConfig
	HTTP          HTTPConfig
	Database      DatabaseConfig
	Auth          AuthConfig
	OIDC          OIDCConfig
	Observability ObservabilityConfig
}
type AppConfig struct {
	Name, Environment string
	ShutdownTimeout   time.Duration
}
type GRPCConfig struct {
	Host        string
	Port        int
	MetricsAddr string
}
type HTTPConfig struct {
	Host        string
	Port        int
	DocsEnabled bool
}
type DatabaseConfig struct {
	URL                            string
	MaxConnections, MinConnections int32
}
type AuthConfig struct {
	LocalEnabled                                    bool
	Issuer, Audience, PrivateKeyFile, PublicKeyFile string
	TokenTTL                                        time.Duration
}
type OIDCConfig struct {
	Enabled             bool
	IssuerURL, ClientID string
}
type ObservabilityConfig struct {
	OTelEnabled  bool
	OTLPEndpoint string
}

func Load() (*Config, error) {
	port, err := intEnv("GRPC_PORT", 50051)
	if err != nil {
		return nil, err
	}
	httpPort, err := intEnv("HTTP_PORT", 8080)
	if err != nil {
		return nil, err
	}
	docsEnabled, err := boolEnv("DOCS_ENABLED", true)
	if err != nil {
		return nil, err
	}
	maxc, err := intEnv("DATABASE_MAX_CONNECTIONS", 20)
	if err != nil {
		return nil, err
	}
	minc, err := intEnv("DATABASE_MIN_CONNECTIONS", 2)
	if err != nil {
		return nil, err
	}
	shutdown, err := durationEnv("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return nil, err
	}
	ttl, err := durationEnv("JWT_ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return nil, err
	}
	local, err := boolEnv("AUTH_LOCAL_ENABLED", true)
	if err != nil {
		return nil, err
	}
	oidcEnabled, err := boolEnv("OIDC_ENABLED", false)
	if err != nil {
		return nil, err
	}
	otelEnabled, err := boolEnv("OTEL_ENABLED", false)
	if err != nil {
		return nil, err
	}
	db := os.Getenv("DATABASE_URL")
	if db == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	issuer := strEnv("JWT_ISSUER", "go-foundation")
	aud := strEnv("JWT_AUDIENCE", "grpc-api")
	priv := strEnv("JWT_PRIVATE_KEY_FILE", "./secrets/jwt_private.pem")
	pub := strEnv("JWT_PUBLIC_KEY_FILE", "./secrets/jwt_public.pem")
	if local && (priv == "" || pub == "") {
		return nil, fmt.Errorf("JWT key files are required when local auth is enabled")
	}
	if oidcEnabled && os.Getenv("OIDC_ISSUER_URL") == "" {
		return nil, fmt.Errorf("OIDC_ISSUER_URL is required when OIDC is enabled")
	}
	return &Config{
		App:           AppConfig{Name: strEnv("APP_NAME", "go-foundation"), Environment: strEnv("APP_ENV", "development"), ShutdownTimeout: shutdown},
		GRPC:          GRPCConfig{Host: strEnv("GRPC_HOST", "0.0.0.0"), Port: port, MetricsAddr: strEnv("METRICS_ADDR", ":9090")},
		HTTP:          HTTPConfig{Host: strEnv("HTTP_HOST", "0.0.0.0"), Port: httpPort, DocsEnabled: docsEnabled},
		Database:      DatabaseConfig{URL: db, MaxConnections: int32(maxc), MinConnections: int32(minc)},
		Auth:          AuthConfig{LocalEnabled: local, Issuer: issuer, Audience: aud, PrivateKeyFile: priv, PublicKeyFile: pub, TokenTTL: ttl},
		OIDC:          OIDCConfig{Enabled: oidcEnabled, IssuerURL: os.Getenv("OIDC_ISSUER_URL"), ClientID: os.Getenv("OIDC_CLIENT_ID")},
		Observability: ObservabilityConfig{OTelEnabled: otelEnabled, OTLPEndpoint: strEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")},
	}, nil
}
func strEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func intEnv(k string, d int) (int, error) {
	v := os.Getenv(k)
	if v == "" {
		return d, nil
	}
	x, e := strconv.Atoi(v)
	if e != nil {
		return 0, fmt.Errorf("invalid %s: %w", k, e)
	}
	return x, nil
}
func boolEnv(k string, d bool) (bool, error) {
	v := os.Getenv(k)
	if v == "" {
		return d, nil
	}
	x, e := strconv.ParseBool(v)
	if e != nil {
		return false, fmt.Errorf("invalid %s: %w", k, e)
	}
	return x, nil
}
func durationEnv(k string, d time.Duration) (time.Duration, error) {
	v := os.Getenv(k)
	if v == "" {
		return d, nil
	}
	x, e := time.ParseDuration(v)
	if e != nil {
		return 0, fmt.Errorf("invalid %s: %w", k, e)
	}
	return x, nil
}
