package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	authv1 "github.com/jabahum/go-foundation/gen/proto/auth/v1"
	rbacv1 "github.com/jabahum/go-foundation/gen/proto/rbac/v1"
	userv1 "github.com/jabahum/go-foundation/gen/proto/user/v1"
	appauth "github.com/jabahum/go-foundation/internal/application/auth"
	apprbac "github.com/jabahum/go-foundation/internal/application/rbac"
	appuser "github.com/jabahum/go-foundation/internal/application/user"
	auth "github.com/jabahum/go-foundation/internal/domain/auth"
	"github.com/jabahum/go-foundation/internal/infrastructure/auth/local"
	oidcauth "github.com/jabahum/go-foundation/internal/infrastructure/auth/oidc"
	"github.com/jabahum/go-foundation/internal/infrastructure/bootstrap"
	"github.com/jabahum/go-foundation/internal/infrastructure/config"
	"github.com/jabahum/go-foundation/internal/infrastructure/database"
	"github.com/jabahum/go-foundation/internal/infrastructure/observability"
	postgresrepo "github.com/jabahum/go-foundation/internal/infrastructure/persistence/postgres"
	transportgrpc "github.com/jabahum/go-foundation/internal/transport/grpc"
	"github.com/jabahum/go-foundation/internal/transport/grpc/interceptor"
	"github.com/jabahum/go-foundation/internal/transport/grpc/policy"
	httptransport "github.com/jabahum/go-foundation/internal/transport/http"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer db.Close()

	hasher := local.NewPasswordHasher()
	if err := bootstrap.EnsureAdmin(ctx, db, hasher, os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"), os.Getenv("BOOTSTRAP_ADMIN_NAME")); err != nil {
		return err
	}

	userRepo := postgresrepo.NewUserRepository(db)
	rbacRepo := postgresrepo.NewRBACRepository(db)
	rbacService := apprbac.NewService(rbacRepo)
	userService := appuser.NewService(userRepo, hasher)

	var verifiers []auth.TokenVerifier
	var localJWT *local.JWTProvider
	if cfg.Auth.LocalEnabled {
		localJWT, err = local.NewJWTProvider(cfg.Auth.PrivateKeyFile, cfg.Auth.PublicKeyFile, cfg.Auth.Issuer, cfg.Auth.Audience, cfg.Auth.TokenTTL)
		if err != nil {
			return fmt.Errorf("local jwt: %w", err)
		}
		verifiers = append(verifiers, localJWT)
	}
	if cfg.OIDC.Enabled {
		v, err := oidcauth.New(ctx, cfg.OIDC.IssuerURL, cfg.OIDC.ClientID)
		if err != nil {
			return fmt.Errorf("oidc verifier: %w", err)
		}
		verifiers = append(verifiers, v)
	}
	if len(verifiers) == 0 {
		return fmt.Errorf("at least one authentication provider must be enabled")
	}

	authn := appauth.NewAuthenticationService(userRepo, verifiers...)

	metrics := observability.NewMetrics()
	metricsServer := observability.StartMetricsServer(cfg.GRPC.MetricsAddr)
	defer shutdownHTTP(metricsServer, cfg.App.ShutdownTimeout)

	var telemetry *observability.Telemetry
	if cfg.Observability.OTelEnabled {
		telemetry, err = observability.NewTelemetry(ctx, cfg.App.Name, cfg.Observability.OTLPEndpoint)
		if err != nil {
			return err
		}
		defer func() {
			c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = telemetry.Shutdown(c)
		}()
	}

	public := map[string]struct{}{
		"/grpc.health.v1.Health/Check": {},
		"/grpc.health.v1.Health/Watch": {},
	}
	public["/auth.v1.AuthService/Login"] = struct{}{}

	unary := []grpc.UnaryServerInterceptor{
		interceptor.RequestIDUnary(),
		interceptor.ErrorDetailsUnary(),
		interceptor.AuthenticationUnary(authn, public),
		interceptor.ValidationUnary(),
		interceptor.AuthorizationUnary(rbacService, policy.Policies(), func(p string) { metrics.Denied.WithLabelValues(p).Inc() }),
		interceptor.MetricsUnary(metrics),
		interceptor.LoggingUnary(logger),
		interceptor.RecoveryUnary(logger),
	}

	serverOptions := []grpc.ServerOption{grpc.ChainUnaryInterceptor(unary...)}
	if cfg.Observability.OTelEnabled {
		serverOptions = append(serverOptions, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}
	grpcServer := grpc.NewServer(serverOptions...)

	userv1.RegisterUserServiceServer(grpcServer, transportgrpc.NewUserHandler(userService))
	rbacv1.RegisterRBACServiceServer(grpcServer, transportgrpc.NewRBACHandler(rbacService))
	var loginService *appauth.Service
	if cfg.Auth.LocalEnabled {
		loginService = appauth.NewService(userRepo, hasher, localJWT)
	}
	authv1.RegisterAuthServiceServer(grpcServer, transportgrpc.NewAuthHandler(loginService, userRepo, rbacService))

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	if cfg.App.Environment == "development" {
		reflection.Register(grpcServer)
	}

	address := fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	gatewayCtx, cancelGateway := context.WithCancel(ctx)
	defer cancelGateway()
	httpAddress := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	grpcEndpoint := gatewayEndpoint(cfg.GRPC.Host, cfg.GRPC.Port)
	gatewayServer, err := httptransport.NewGateway(gatewayCtx, httpAddress, grpcEndpoint, cfg.HTTP.DocsEnabled)
	if err != nil {
		return fmt.Errorf("create HTTP gateway: %w", err)
	}

	serverErr := make(chan error, 2)
	go func() {
		logger.Info("grpc server started", "address", address, "metrics", cfg.GRPC.MetricsAddr)
		if err := grpcServer.Serve(listener); err != nil {
			serverErr <- fmt.Errorf("serve gRPC: %w", err)
		}
	}()
	go func() {
		logger.Info("HTTP gateway started", "address", httpAddress, "grpc_endpoint", grpcEndpoint, "docs_enabled", cfg.HTTP.DocsEnabled)
		if err := gatewayServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("serve HTTP gateway: %w", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	var serveErr error
	select {
	case err := <-serverErr:
		serveErr = err
	case sig := <-sigCh:
		logger.Info("shutdown signal received", "signal", sig.String())
	}

	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	shutdownHTTP(gatewayServer, cfg.App.ShutdownTimeout)
	cancelGateway()
	gracefulStop(grpcServer, cfg.App.ShutdownTimeout)
	return serveErr
}

func gatewayEndpoint(host string, port int) string {
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func gracefulStop(s *grpc.Server, timeout time.Duration) {
	done := make(chan struct{})
	go func() { s.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
		s.Stop()
	}
}
func shutdownHTTP(s *http.Server, timeout time.Duration) {
	if s == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
	}
}
