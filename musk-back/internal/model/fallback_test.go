package model

import (
	"context"
	"errors"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/observability"
	"autonomous-agent/internal/resilience"
)

type seqModel struct {
	replies []*schema.Message
	errs    []error
	i       int
	calls   int
}

func (s *seqModel) Generate(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	s.calls++
	i := s.i
	s.i++
	if i < len(s.errs) && s.errs[i] != nil {
		return nil, s.errs[i]
	}
	if i < len(s.replies) {
		return s.replies[i], nil
	}
	if len(s.replies) > 0 {
		return s.replies[len(s.replies)-1], nil
	}
	return schema.AssistantMessage("ok", nil), nil
}

func (s *seqModel) Stream(ctx context.Context, in []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := s.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	sr, sw := schema.Pipe[*schema.Message](1)
	sw.Send(msg, nil)
	sw.Close()
	return sr, nil
}

func TestFallbackUsesSecondaryWhenPrimaryFails(t *testing.T) {
	primary := &seqModel{errs: []error{errors.New("invalid json")}} // non-retryable
	secondary := &seqModel{replies: []*schema.Message{schema.AssistantMessage("from-fallback", nil)}}
	m := NewInstrumentedChatModel(primary, secondary, "primary", "secondary", resilience.RetryPolicy{MaxAttempts: 1}, nil, nil, nil, logger.New("error", "json", nil))
	msg, err := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "from-fallback" {
		t.Fatalf("content = %q", msg.Content)
	}
	if primary.calls != 1 || secondary.calls != 1 {
		t.Fatalf("calls primary=%d secondary=%d", primary.calls, secondary.calls)
	}
}

func TestFallbackFailsWhenBothFail(t *testing.T) {
	primary := &seqModel{errs: []error{errors.New("primary down")}}
	secondary := &seqModel{errs: []error{errors.New("secondary down")}}
	m := NewInstrumentedChatModel(primary, secondary, "p", "s", resilience.RetryPolicy{MaxAttempts: 1}, nil, nil, nil, logger.New("error", "json", nil))
	_, err := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInstrumentedRecordsTokens(t *testing.T) {
	tel, err := observability.Setup(context.Background(), observability.Config{ServiceName: "test"}, logger.New("error", "json", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer tel.Shutdown(context.Background())
	reply := schema.AssistantMessage("ok", nil)
	reply.ResponseMeta = &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}}
	primary := &seqModel{replies: []*schema.Message{reply}}
	m := NewInstrumentedChatModel(primary, nil, "stub", "", resilience.RetryPolicy{MaxAttempts: 1}, nil, nil, tel, logger.New("error", "json", nil))
	ctx := observability.WithRunID(context.Background(), "run-tokens")
	if _, err := m.Generate(ctx, []*schema.Message{schema.UserMessage("hi")}); err != nil {
		t.Fatal(err)
	}
	snap := tel.Tracker.Snapshot("run-tokens")
	if snap.TokenUsage.TotalTokens != 15 || snap.LLMCalls != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
}
