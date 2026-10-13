package observability

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"autonomous-agent/internal/logger"
)

const (
	instrumentationName = "autonomous-agent"
	attrRunID           = "agent.run_id"
	attrStepID          = "agent.step_id"
	attrToolCallID      = "agent.tool_call_id"
	attrTool            = "agent.tool"
	attrDecision        = "agent.decision"
)

// Config controls OpenTelemetry exporters and the local tracker.
type Config struct {
	ServiceName   string
	OTLPEndpoint  string
	Traces        bool
	Metrics       bool
	Cost          CostModel
	ShutdownWait  time.Duration
}

// Telemetry is the process-wide observability surface: traces, metrics, cost tracker.
type Telemetry struct {
	Tracer  trace.Tracer
	Meter   metric.Meter
	Tracker *Tracker
	logger  logger.Logger

	tp     *sdktrace.TracerProvider
	mp     *sdkmetric.MeterProvider
	runLat metric.Float64Histogram
	tokens metric.Int64Counter
	cost   metric.Float64Counter
	errs   metric.Int64Counter
	tools  metric.Int64Counter
}

var (
	globalMu sync.Mutex
	global   *Telemetry
)

// Global returns the last Setup telemetry, or a no-exporter fallback.
func Global() *Telemetry {
	globalMu.Lock()
	defer globalMu.Unlock()
	if global != nil {
		return global
	}
	t, _ := Setup(context.Background(), Config{ServiceName: instrumentationName}, logger.New("info", "json", os.Stdout))
	global = t
	return t
}

// Setup installs a TracerProvider and MeterProvider. Empty OTLP endpoint keeps spans in-process.
func Setup(ctx context.Context, cfg Config, log logger.Logger) (*Telemetry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = logger.New("info", "json", os.Stdout)
	}
	if strings.TrimSpace(cfg.ServiceName) == "" {
		cfg.ServiceName = instrumentationName
	}
	if cfg.ShutdownWait <= 0 {
		cfg.ShutdownWait = 5 * time.Second
	}
	if !cfg.Traces && !cfg.Metrics {
		cfg.Traces = true
		cfg.Metrics = true
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", "12"),
		),
	)
	if err != nil {
		return nil, err
	}

	t := &Telemetry{
		Tracker: NewTracker(cfg.Cost),
		logger:  log,
	}

	endpoint := strings.TrimSpace(cfg.OTLPEndpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}

	if cfg.Traces {
		opts := []sdktrace.TracerProviderOption{
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
		}
		if endpoint != "" {
			exp, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(normalizeOTLP(endpoint)), otlptracehttp.WithInsecure())
			if err != nil {
				log.WarnContext(ctx, "otlp trace exporter failed; using in-process traces", "error", err)
			} else {
				opts = append(opts, sdktrace.WithBatcher(exp))
			}
		}
		t.tp = sdktrace.NewTracerProvider(opts...)
		otel.SetTracerProvider(t.tp)
	}

	if cfg.Metrics {
		mopts := []sdkmetric.Option{sdkmetric.WithResource(res)}
		if endpoint != "" {
			exp, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(normalizeOTLP(endpoint)), otlpmetrichttp.WithInsecure())
			if err != nil {
				log.WarnContext(ctx, "otlp metric exporter failed; using in-process metrics", "error", err)
			} else {
				mopts = append(mopts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)))
			}
		}
		t.mp = sdkmetric.NewMeterProvider(mopts...)
		otel.SetMeterProvider(t.mp)
	}

	t.Tracer = otel.Tracer(instrumentationName)
	t.Meter = otel.Meter(instrumentationName)
	t.initInstruments()

	globalMu.Lock()
	global = t
	globalMu.Unlock()
	return t, nil
}

func normalizeOTLP(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	return "http://" + endpoint
}

func (t *Telemetry) initInstruments() {
	if t == nil || t.Meter == nil {
		return
	}
	t.runLat, _ = t.Meter.Float64Histogram("agent.run.latency_ms", metric.WithUnit("ms"))
	t.tokens, _ = t.Meter.Int64Counter("agent.llm.tokens")
	t.cost, _ = t.Meter.Float64Counter("agent.llm.cost_usd")
	t.errs, _ = t.Meter.Int64Counter("agent.errors")
	t.tools, _ = t.Meter.Int64Counter("agent.tool.calls")
}

// Shutdown flushes exporters.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var first error
	if t.tp != nil {
		if err := t.tp.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	}
	if t.mp != nil {
		if err := t.mp.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// StartSpan starts a trace span tagged with run/step/tool-call IDs from ctx.
func (t *Telemetry) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	if t == nil || t.Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	attrs = append(correlationAttrs(ctx), attrs...)
	return t.Tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func correlationAttrs(ctx context.Context) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, 4)
	if id := RunID(ctx); id != "" {
		out = append(out, attribute.String(attrRunID, id))
	}
	if id := StepID(ctx); id != "" {
		out = append(out, attribute.String(attrStepID, id))
	}
	if id := ToolCallID(ctx); id != "" {
		out = append(out, attribute.String(attrToolCallID, id))
	}
	return out
}

// RecordLLM records token usage, cost, and a metric datapoint.
func (t *Telemetry) RecordLLM(ctx context.Context, usage TokenUsage, model string) {
	if t == nil {
		return
	}
	runID := RunID(ctx)
	delta := 0.0
	if t.Tracker != nil {
		delta = t.Tracker.RecordTokens(runID, usage)
	}
	if t.tokens != nil {
		t.tokens.Add(ctx, int64(usage.TotalTokens), metric.WithAttributes(
			attribute.String(attrRunID, runID),
			attribute.String("llm.model", model),
		))
	}
	if t.cost != nil && delta > 0 {
		t.cost.Add(ctx, delta, metric.WithAttributes(attribute.String(attrRunID, runID)))
	}
	if t.logger != nil {
		args := append(Attrs(ctx),
			"prompt_tokens", usage.PromptTokens,
			"completion_tokens", usage.CompletionTokens,
			"total_tokens", usage.TotalTokens,
			"cost_usd", delta,
			"model", model,
		)
		t.logger.InfoContext(ctx, "llm token usage", args...)
	}
}

// RecordTool increments tool-call metrics.
func (t *Telemetry) RecordTool(ctx context.Context, tool string, success bool, latency time.Duration) {
	if t == nil {
		return
	}
	runID := RunID(ctx)
	if t.Tracker != nil {
		t.Tracker.RecordToolCall(runID)
	}
	if t.tools != nil {
		t.tools.Add(ctx, 1, metric.WithAttributes(
			attribute.String(attrRunID, runID),
			attribute.String(attrTool, tool),
			attribute.Bool("success", success),
		))
	}
	if t.logger != nil {
		args := append(Attrs(ctx),
			"tool", tool,
			"success", success,
			"latency_ms", latency.Milliseconds(),
		)
		t.logger.InfoContext(ctx, "tool call telemetry", args...)
	}
}

// RecordError records an error on the current span, metrics, and tracker.
func (t *Telemetry) RecordError(ctx context.Context, err error) {
	if t == nil || err == nil {
		return
	}
	runID := RunID(ctx)
	if t.Tracker != nil {
		t.Tracker.RecordError(runID, err.Error())
	}
	if t.errs != nil {
		t.errs.Add(ctx, 1, metric.WithAttributes(attribute.String(attrRunID, runID)))
	}
	if span := trace.SpanFromContext(ctx); span != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	if t.logger != nil {
		args := append(Attrs(ctx), "error", err)
		t.logger.ErrorContext(ctx, "error tracking", args...)
	}
}

// RecordRunLatency records end-to-end run duration.
func (t *Telemetry) RecordRunLatency(ctx context.Context, d time.Duration, status string) {
	if t == nil || t.runLat == nil {
		return
	}
	t.runLat.Record(ctx, float64(d.Milliseconds()), metric.WithAttributes(
		attribute.String(attrRunID, RunID(ctx)),
		attribute.String("agent.status", status),
	))
}
