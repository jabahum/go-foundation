package httptransport

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authv1 "github.com/jabahum/go-foundation/gen/proto/auth/v1"
	grpcinterceptor "github.com/jabahum/go-foundation/internal/transport/grpc/interceptor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

type testAuthServer struct {
	authv1.UnimplementedAuthServiceServer
	t *testing.T
}

func (s *testAuthServer) Login(ctx context.Context, _ *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer test-token" {
		s.t.Errorf("authorization metadata = %v", got)
	}
	return &authv1.LoginResponse{AccessToken: "gateway-token"}, nil
}

func TestGatewayTranscodesHTTPToGRPC(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpcinterceptor.RequestIDUnary(),
		grpcinterceptor.ErrorDetailsUnary(),
		grpcinterceptor.ValidationUnary(),
	))
	authv1.RegisterAuthServiceServer(grpcServer, &testAuthServer{t: t})
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	}
	gateway, err := newGateway(ctx, "127.0.0.1:0", "passthrough:///gateway-test", true, dialOptions)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"password"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	gateway.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.AccessToken != "gateway-token" {
		t.Fatalf("accessToken = %q", body.AccessToken)
	}

	invalidRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"invalid","password":""}`))
	invalidRequest.Header.Set("Content-Type", "application/json")
	invalidResponse := httptest.NewRecorder()
	gateway.Handler.ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", invalidResponse.Code, invalidResponse.Body.String())
	}
	invalidBody := invalidResponse.Body.String()
	for _, expected := range []string{"REQUEST_VALIDATION_FAILED", "google.rpc.BadRequest", "email", "password"} {
		if !strings.Contains(invalidBody, expected) {
			t.Errorf("invalid response is missing %q: %s", expected, invalidBody)
		}
	}
}

func TestEmbeddedDocumentation(t *testing.T) {
	handler, err := withDocumentation(http.NotFoundHandler(), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("redirect", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs", nil))
		if response.Code != http.StatusPermanentRedirect || response.Header().Get("Location") != "/docs/" {
			t.Fatalf("status = %d, location = %q", response.Code, response.Header().Get("Location"))
		}
	})

	t.Run("UI", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs/", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
		if body := response.Body.String(); !strings.Contains(body, "SwaggerUIBundle") || strings.Contains(body, "https://") {
			t.Fatal("documentation UI is missing its local Swagger bundle or contains a remote asset")
		}
	})

	t.Run("OpenAPI", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
		if body := response.Body.String(); !strings.Contains(body, `"title": "go-foundation API"`) || !strings.Contains(body, `"BearerAuth"`) {
			t.Fatal("OpenAPI document is missing API metadata or bearer authentication")
		}
	})
}

func TestDocumentationCanBeDisabled(t *testing.T) {
	handler, err := withDocumentation(http.NotFoundHandler(), false)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs/", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}
