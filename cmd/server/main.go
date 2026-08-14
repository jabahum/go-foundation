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
	"syscall"
	"time"

	authv1 "example.com/grpc-clean-starter/gen/proto/auth/v1"
	rbacv1 "example.com/grpc-clean-starter/gen/proto/rbac/v1"
	userv1 "example.com/grpc-clean-starter/gen/proto/user/v1"
	appauth "example.com/grpc-clean-starter/internal/application/auth"
	apprbac "example.com/grpc-clean-starter/internal/application/rbac"
	appuser "example.com/grpc-clean-starter/internal/application/user"
	domainauth "example.com/grpc-clean-starter/internal/domain/auth"
	"example.com/grpc-clean-starter/internal/infrastructure/auth/local"
	oidcauth "example.com/grpc-clean-starter/internal/infrastructure/auth/oidc"
	"example.com/grpc-clean-starter/internal/infrastructure/bootstrap"
	"example.com/grpc-clean-starter/internal/infrastructure/config"
	"example.com/grpc-clean-starter/internal/infrastructure/database"
	"example.com/grpc-clean-starter/internal/infrastructure/observability"
	postgresrepo "example.com/grpc-clean-starter/internal/infrastructure/persistence/postgres"
	transportgrpc "example.com/grpc-clean-starter/internal/transport/grpc"
	"example.com/grpc-clean-starter/internal/transport/grpc/interceptor"
	"example.com/grpc-clean-starter/internal/transport/grpc/policy"

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

	var verifiers []domainauth.TokenVerifier
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
		interceptor.AuthenticationUnary(authn, public),
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
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("grpc server started", "address", address, "metrics", cfg.GRPC.MetricsAddr)
		serverErr <- grpcServer.Serve(listener)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	select {
	case err := <-serverErr:
		if err != nil {
			return err
		}
	case sig := <-sigCh:
		logger.Info("shutdown signal received", "signal", sig.String())
	}

	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	gracefulStop(grpcServer, cfg.App.ShutdownTimeout)
	return nil
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
