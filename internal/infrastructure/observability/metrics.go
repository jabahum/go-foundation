package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

type Metrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
	Denied   *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "grpc_server_requests_total", Help: "Total unary gRPC requests"}, []string{"method", "code"}),
		Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "grpc_server_request_duration_seconds", Help: "Unary gRPC request duration"}, []string{"method"}),
		Denied:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "grpc_authorization_denied_total", Help: "Authorization denials"}, []string{"permission"}),
	}
	prometheus.MustRegister(m.Requests, m.Duration, m.Denied)
	return m
}
func StartMetricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.ListenAndServe() }()
	return srv
}
