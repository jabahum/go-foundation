package observability

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Telemetry struct{ provider *sdktrace.TracerProvider }

func NewTelemetry(ctx context.Context, serviceName, endpoint string) (*Telemetry, error) {
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes("", attribute.String("service.name", serviceName)))
	if err != nil {
		return nil, err
	}
	p := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(p)
	return &Telemetry{provider: p}, nil
}
func (t *Telemetry) Shutdown(ctx context.Context) error { return t.provider.Shutdown(ctx) }
