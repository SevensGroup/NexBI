package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/trace"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/observability"
	"autonomous-agent/internal/resilience"
)

// InstrumentedChatModel wraps a chat model with retry, circuit breaker, fallback, and token tracking.
type InstrumentedChatModel struct {
	primary   einomodel.BaseChatModel
	secondary einomodel.BaseChatModel
	name      string
	fallback  string
	retry     resilience.RetryPolicy
	breaker   *resilience.CircuitBreaker
	secBreak  *resilience.CircuitBreaker
	telemetry *observability.Telemetry
	logger    logger.Logger
	tools     []*schema.ToolInfo
}

var (
	_ einomodel.BaseChatModel        = (*InstrumentedChatModel)(nil)
	_ einomodel.ToolCallingChatModel = (*InstrumentedChatModel)(nil)
)

// NewInstrumentedChatModel wraps primary (and optional secondary) with production policies.
func NewInstrumentedChatModel(primary, secondary einomodel.BaseChatModel, primaryName, secondaryName string, retry resilience.RetryPolicy, breaker, secBreaker *resilience.CircuitBreaker, tel *observability.Telemetry, log logger.Logger) *InstrumentedChatModel {
	if retry.MaxAttempts < 1 {
		retry = resilience.DefaultRetry()
	}
	if log == nil {
		log = logger.New("info", "json", io.Discard)
	}
	return &InstrumentedChatModel{
		primary:   primary,
		secondary: secondary,
		name:      strings.TrimSpace(primaryName),
		fallback:  strings.TrimSpace(secondaryName),
		retry:     retry,
		breaker:   breaker,
		secBreak:  secBreaker,
		telemetry: tel,
		logger:    log,
	}
}

// WithTools returns a copy bound to the given tools (safe for concurrent use).
func (m *InstrumentedChatModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	if m == nil {
		return nil, errors.New("chat model is required")
	}
	clone := *m
	clone.tools = append([]*schema.ToolInfo(nil), tools...)
	return &clone, nil
}

// Generate calls the primary model, then the secondary on failure. Fail if both fail.
func (m *InstrumentedChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	if m == nil {
		return nil, errors.New("chat model is required")
	}
	var span trace.Span
	if m.telemetry != nil {
		ctx, span = m.telemetry.StartSpan(ctx, "llm.generate")
		defer span.End()
	}

	msg, err := m.call(ctx, m.primary, m.breaker, m.name, input, opts)
	if err == nil {
		m.recordUsage(ctx, msg, m.name)
		return msg, nil
	}
	if m.telemetry != nil {
		m.telemetry.RecordError(ctx, err)
	}
	if m.secondary == nil {
		return nil, err
	}
	if m.logger != nil {
		m.logger.WarnContext(ctx, "llm primary failed; trying fallback",
			append(observability.Attrs(ctx), "primary", m.name, "fallback", m.fallback, "error", err)...,
		)
	}
	msg, err2 := m.call(ctx, m.secondary, m.secBreak, m.fallback, input, opts)
	if err2 == nil {
		m.recordUsage(ctx, msg, m.fallback)
		return msg, nil
	}
	if m.telemetry != nil {
		m.telemetry.RecordError(ctx, err2)
	}
	return nil, fmt.Errorf("llm primary and fallback failed: primary=%w; fallback=%v", err, err2)
}

// Stream is implemented via Generate (the agent loop uses Generate).
func (m *InstrumentedChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	sr, sw := schema.Pipe[*schema.Message](1)
	sw.Send(msg, nil)
	sw.Close()
	return sr, nil
}

func (m *InstrumentedChatModel) call(ctx context.Context, cm einomodel.BaseChatModel, breaker *resilience.CircuitBreaker, name string, input []*schema.Message, opts []einomodel.Option) (*schema.Message, error) {
	if cm == nil {
		return nil, errors.New("chat model is not configured")
	}
	if breaker != nil && !breaker.Allow() {
		return nil, fmt.Errorf("%w: %s", resilience.ErrOpen, name)
	}

	bound := cm
	if len(m.tools) > 0 {
		if tm, ok := cm.(einomodel.ToolCallingChatModel); ok {
			next, err := tm.WithTools(m.tools)
			if err != nil {
				return nil, err
			}
			bound = next
		}
	}

	var out *schema.Message
	err := m.retry.Do(ctx, func(int) error {
		msg, genErr := bound.Generate(ctx, input, opts...)
		if genErr != nil {
			return genErr
		}
		out = msg
		return nil
	})
	if err != nil {
		if breaker != nil {
			breaker.Fail()
		}
		return nil, err
	}
	if breaker != nil {
		breaker.Succeed()
	}
	return out, nil
}

func (m *InstrumentedChatModel) recordUsage(ctx context.Context, msg *schema.Message, label string) {
	usage := usageFrom(msg)
	if m.telemetry != nil {
		m.telemetry.RecordLLM(ctx, usage, label)
	}
	if m.logger != nil {
		m.logger.InfoContext(ctx, "llm generate completed",
			append(observability.Attrs(ctx), "model", label, "prompt_tokens", usage.PromptTokens, "completion_tokens", usage.CompletionTokens)...,
		)
	}
}

func usageFrom(msg *schema.Message) observability.TokenUsage {
	if msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return observability.TokenUsage{}
	}
	u := msg.ResponseMeta.Usage
	total := u.TotalTokens
	if total == 0 {
		total = u.PromptTokens + u.CompletionTokens
	}
	return observability.TokenUsage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      total,
	}
}

// FallbackProvider builds a primary model and optional secondary, then instruments both.
type FallbackProvider struct {
	primary   Provider
	secondary Provider
	primaryN  string
	secondN   string
	retry     resilience.RetryPolicy
	breaker   *resilience.CircuitBreaker
	secBreak  *resilience.CircuitBreaker
	telemetry *observability.Telemetry
	logger    logger.Logger
}

var _ Provider = (*FallbackProvider)(nil)

// NewChatModel creates the instrumented Primary → Secondary → Fail stack.
func (p *FallbackProvider) NewChatModel(ctx context.Context) (einomodel.BaseChatModel, error) {
	if p == nil || p.primary == nil {
		return nil, errors.New("primary model provider is required")
	}
	pri, err := p.primary.NewChatModel(ctx)
	if err != nil {
		return nil, err
	}
	var sec einomodel.BaseChatModel
	if p.secondary != nil {
		sec, err = p.secondary.NewChatModel(ctx)
		if err != nil {
			if p.logger != nil {
				p.logger.WarnContext(ctx, "llm fallback model failed to initialize", "error", err)
			}
			sec = nil
		}
	}
	return NewInstrumentedChatModel(pri, sec, p.primaryN, p.secondN, p.retry, p.breaker, p.secBreak, p.telemetry, p.logger), nil
}

// NewProviderFromConfig builds the production model stack from LLM + resilience config.
func NewProviderFromConfig(cfg config.LLMConfig, resCfg config.ResilienceConfig, tel *observability.Telemetry, log logger.Logger) Provider {
	primary := NewOpenAIProvider(cfg)
	var secondary Provider
	if cfg.Fallback.Enabled() {
		secondary = NewOpenAIProvider(cfg.Fallback.AsLLM())
	}
	retry := resilience.RetryPolicy{
		MaxAttempts: resCfg.Retry.MaxAttempts,
		Backoff:     resCfg.Retry.Backoff.Duration(),
	}
	if retry.MaxAttempts < 1 {
		retry = resilience.DefaultRetry()
	}
	reset := resCfg.CircuitBreaker.ResetTimeout.Duration()
	if reset <= 0 {
		reset = 30 * time.Second
	}
	threshold := resCfg.CircuitBreaker.FailureThreshold
	if threshold < 1 {
		threshold = 5
	}
	breaker := resilience.NewCircuitBreaker(threshold, reset)
	var secBreak *resilience.CircuitBreaker
	if secondary != nil {
		secBreak = resilience.NewCircuitBreaker(threshold, reset)
	}
	return &FallbackProvider{
		primary:   primary,
		secondary: secondary,
		primaryN:  cfg.Model,
		secondN:   cfg.Fallback.Model,
		retry:     retry,
		breaker:   breaker,
		secBreak:  secBreak,
		telemetry: tel,
		logger:    log,
	}
}
