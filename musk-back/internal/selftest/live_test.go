//go:build live

package selftest

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLiveAgentRunAgainstSiliconFlow(t *testing.T) {
	env, err := LoadLLMEnv()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("loaded LLM config from %s model=%s host=%s", env.Path, env.Model, env.BaseURL)

	healthClient := NewHTTPClient(10 * time.Second)
	if err := CheckHealth(healthClient); err != nil {
		t.Fatal(err)
	}

	probeClient := NewHTTPClient(30 * time.Second)
	if err := ProbeSiliconFlow(probeClient, env); err != nil {
		t.Fatal(err)
	}

	reqID := uuid.NewString()
	type postResult struct {
		status int
		body   runPostBody
		raw    []byte
		err    error
	}
	postCh := make(chan postResult, 1)
	go func() {
		status, body, raw, err := PostAgentRun(NewHTTPClient(12*time.Minute), reqID, GoalMessage)
		postCh <- postResult{status: status, body: body, raw: raw, err: err}
	}()

	pollClient := NewHTTPClient(20 * time.Second)
	snap, pollErr := PollRun(pollClient, reqID, 10*time.Minute)

	var post postResult
	select {
	case post = <-postCh:
	case <-time.After(15 * time.Second):
		// POST still in flight; snapshot from GET is enough.
	}

	runID := reqID
	if snap.ID != "" {
		runID = snap.ID
	} else if post.body.RunID != "" {
		runID = post.body.RunID
	}

	status := snap.Status
	errText := snap.Error
	if status == "" {
		status = post.body.Status
	}
	if errText == "" {
		errText = post.body.Error
	}
	if pollErr != nil && status == "" {
		if post.err != nil {
			t.Fatalf("POST /api/v1/agent/run: %v; GET: %v", post.err, pollErr)
		}
		t.Fatalf("GET /api/v1/runs/:id: %v (POST HTTP %d body %s)", pollErr, post.status, truncate(post.raw, 400))
	}

	_ = WriteArtifact(LiveArtifact{
		RunID:  runID,
		Status: status,
		Error:  errText,
		Goal:   GoalMessage,
	})

	if post.status == 200 && len(bytesTrim(post.raw)) == 0 {
		t.Fatal("POST /api/v1/agent/run returned empty 200")
	}

	switch status {
	case "FAILED", "ABORTED":
		if strings.TrimSpace(errText) == "" {
			errText = "agent run failed"
		}
		t.Fatalf("agent run %s: %s", status, errText)
	case "COMPLETED", "WAITING_APPROVAL":
	case "":
		t.Fatalf("no terminal status; POST HTTP %d error=%s", post.status, errText)
	default:
		t.Fatalf("unexpected status %q HTTP %d error=%s", status, post.status, errText)
	}

	if runID == "" || runID == "run-ads-01" {
		t.Fatalf("expected musk-back uuid run_id, got %q", runID)
	}
	tokens := TokensUsed(snap.TokenUsage)
	if tokens == 0 && len(snap.Decisions) == 0 {
		t.Fatalf("LLM was not invoked: token_usage=0 decisions=0 (model should be %s)", ExpectedModel)
	}
	t.Logf("run_id=%s status=%s decisions=%d tokens=%d", runID, status, len(snap.Decisions), tokens)
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
