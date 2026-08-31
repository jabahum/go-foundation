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
	grpcServer := grpc.NewServer()
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
	gateway, err := newGateway(ctx, "127.0.0.1:0", "passthrough:///gateway-test", dialOptions)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{}`))
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
}
