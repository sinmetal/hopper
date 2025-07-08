package trace

import (
	"context"
	"fmt"
	"log"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// batchWriteSpansName is the name of the span to be filtered.
const batchWriteSpansName = "google.devtools.cloudtrace.v2.TraceService/BatchWriteSpans"

// filteringSampler is a sampler that filters spans based on their name.
type filteringSampler struct {
	delegate sdktrace.Sampler
}

// ShouldSample implements sdktrace.Sampler.
func (s filteringSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	// if the span name is batchWriteSpansName, drop it.
	if p.Name == batchWriteSpansName {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.Drop,
			Tracestate: trace.SpanContextFromContext(p.ParentContext).TraceState(),
		}
	}
	// For other spans, use the delegate sampler.
	return s.delegate.ShouldSample(p)
}

// Description implements sdktrace.Sampler.
func (s filteringSampler) Description() string {
	return fmt.Sprintf("filteringSampler{%s}", s.delegate.Description())
}

func InitTracer(projectID string) (func(), error) {
	exporter, err := texporter.New(texporter.WithProjectID(projectID))
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	sampler := filteringSampler{
		// Use ParentBased sampler with AlwaysSample as the root sampler.
		// This means that if there is a parent span, the child span will inherit the sampling decision.
		// If there is no parent span, the child span will always be sampled.
		delegate: sdktrace.ParentBased(sdktrace.AlwaysSample()),
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)

	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}, nil
}
