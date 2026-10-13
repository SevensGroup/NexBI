package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOverlaysLLMEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	content := []byte(`
server:
  addr: ":9090"
  shutdown_timeout: 5s
llm:
  base_url: "http://yaml-base"
  api_key: "yaml-key"
  model: "yaml-model"
  timeout: 10s
agent:
  name: test-agent
  instruction: "test instruction"
  max_steps: 3
  max_tasks: 4
log:
  level: debug
  format: text
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LLM_BASE_URL", "http://env-base")
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("LLM_MODEL", "env-model")
	t.Setenv("AGENT_MAX_STEPS", "5")
	t.Setenv("AGENT_MAX_TASKS", "9")
	t.Setenv("TOOL_TIMEOUT", "7s")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Server.Addr != ":9090" {
		t.Fatalf("addr = %q", cfg.Server.Addr)
	}
	if cfg.Server.ShutdownTimeout.Duration() != 5*time.Second {
		t.Fatalf("shutdown timeout = %s", cfg.Server.ShutdownTimeout.Duration())
	}
	if cfg.LLM.BaseURL != "http://env-base" {
		t.Fatalf("base url = %q, want env overlay", cfg.LLM.BaseURL)
	}
	if cfg.LLM.APIKey != "env-key" {
		t.Fatalf("api key = %q, want env overlay", cfg.LLM.APIKey)
	}
	if cfg.LLM.Model != "env-model" {
		t.Fatalf("model = %q, want env overlay", cfg.LLM.Model)
	}
	if cfg.Agent.Name != "test-agent" {
		t.Fatalf("agent name = %q", cfg.Agent.Name)
	}
	if cfg.Agent.MaxSteps != 5 {
		t.Fatalf("max steps = %d, want env overlay 5", cfg.Agent.MaxSteps)
	}
	if cfg.Agent.MaxTasks != 9 {
		t.Fatalf("max tasks = %d, want env overlay 9", cfg.Agent.MaxTasks)
	}
	if cfg.Tools.Timeout.Duration() != 7*time.Second {
		t.Fatalf("tool timeout = %s, want env overlay 7s", cfg.Tools.Timeout.Duration())
	}
}

func TestLoadElasticsearchYAMLAndEnv(t *testing.T) {
	dir := t.TempDir()
	toolsDir := filepath.Join(dir, "tools")
	if err := os.MkdirAll(toolsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	esPath := filepath.Join(toolsDir, "elasticsearch.yaml")
	esYAML := []byte(`
mcp_url: "http://yaml-es:8080/mcp"
timeout: 8s
retry:
  max_attempts: 4
  backoff: 50ms
max_size: 25
max_aggregation_buckets: 15
read_only: true
allowed_tools:
  - search
  - get
  - count
  - aggregate
denied_tools:
  - delete
  - update
  - index
  - bulk
`)
	if err := os.WriteFile(esPath, esYAML, 0o600); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "agent.yaml")
	content := []byte(`
server:
  addr: ":9090"
llm:
  base_url: "http://yaml-base"
  api_key: "yaml-key"
  model: "yaml-model"
agent:
  name: test-agent
  instruction: "test instruction"
tools:
  timeout: 3s
  elasticsearch:
    config: tools/elasticsearch.yaml
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LLM_BASE_URL", "http://env-base")
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("LLM_MODEL", "env-model")
	t.Setenv("ELASTICSEARCH_MCP_URL", "http://env-es:9200/mcp")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Tools.Elasticsearch.MCPURL != "http://env-es:9200/mcp" {
		t.Fatalf("mcp url = %q, want env overlay", cfg.Tools.Elasticsearch.MCPURL)
	}
	if cfg.Tools.Elasticsearch.MaxSize != 25 {
		t.Fatalf("max_size = %d", cfg.Tools.Elasticsearch.MaxSize)
	}
	if cfg.Tools.Elasticsearch.MaxAggregationBuckets != 15 {
		t.Fatalf("max_aggregation_buckets = %d", cfg.Tools.Elasticsearch.MaxAggregationBuckets)
	}
	if cfg.Tools.Elasticsearch.Retry.MaxAttempts != 4 {
		t.Fatalf("retry = %d", cfg.Tools.Elasticsearch.Retry.MaxAttempts)
	}
	if cfg.Tools.Elasticsearch.Timeout.Duration() != 8*time.Second {
		t.Fatalf("es timeout = %s", cfg.Tools.Elasticsearch.Timeout.Duration())
	}
}

func TestLoadSemanticYAMLAndEnv(t *testing.T) {
	dir := t.TempDir()
	semDir := filepath.Join(dir, "semantic")
	if err := os.MkdirAll(semDir, 0o700); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "agent.yaml")
	content := []byte(`
server:
  addr: ":9090"
llm:
  base_url: "http://yaml-base"
  api_key: "yaml-key"
  model: "yaml-model"
agent:
  name: test-agent
  instruction: "test instruction"
semantic:
  dir: semantic
  max_time_range: 30d
  max_result_size: 20
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LLM_BASE_URL", "http://env-base")
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("LLM_MODEL", "env-model")
	t.Setenv("SEMANTIC_DIR", semDir)
	t.Setenv("SEMANTIC_MAX_TIME_RANGE", "14d")
	t.Setenv("SEMANTIC_MAX_RESULT_SIZE", "7")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Semantic.Dir != semDir {
		t.Fatalf("dir = %q, want env overlay", cfg.Semantic.Dir)
	}
	if cfg.Semantic.MaxTimeRange.Duration() != 14*24*time.Hour {
		t.Fatalf("max_time_range = %s", cfg.Semantic.MaxTimeRange.Duration())
	}
	if cfg.Semantic.MaxResultSize != 7 {
		t.Fatalf("max_result_size = %d", cfg.Semantic.MaxResultSize)
	}
}

func TestLoadMemoryYAMLAndEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	content := []byte(`
server:
  addr: ":9090"
llm:
  base_url: "http://yaml-base"
  api_key: "yaml-key"
  model: "yaml-model"
agent:
  name: test-agent
  instruction: "test instruction"
memory:
  postgres_url: "postgres://yaml:yaml@localhost:5432/yaml"
  short_term_ttl: 2h
  task_ttl: 3h
  long_term_ttl: 30d
  search_limit: 11
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LLM_BASE_URL", "http://env-base")
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("LLM_MODEL", "env-model")
	t.Setenv("MEMORY_POSTGRES_URL", "postgres://env:env@db:5432/agent")
	t.Setenv("MEMORY_SHORT_TERM_TTL", "12h")
	t.Setenv("MEMORY_TASK_TTL", "48h")
	t.Setenv("MEMORY_LONG_TERM_TTL", "14d")
	t.Setenv("MEMORY_SEARCH_LIMIT", "9")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Memory.PostgresURL != "postgres://env:env@db:5432/agent" {
		t.Fatalf("postgres_url = %q, want env overlay", cfg.Memory.PostgresURL)
	}
	if cfg.Memory.ShortTermTTL.Duration() != 12*time.Hour {
		t.Fatalf("short_term_ttl = %s", cfg.Memory.ShortTermTTL.Duration())
	}
	if cfg.Memory.TaskTTL.Duration() != 48*time.Hour {
		t.Fatalf("task_ttl = %s", cfg.Memory.TaskTTL.Duration())
	}
	if cfg.Memory.LongTermTTL.Duration() != 14*24*time.Hour {
		t.Fatalf("long_term_ttl = %s", cfg.Memory.LongTermTTL.Duration())
	}
	if cfg.Memory.SearchLimit != 9 {
		t.Fatalf("search_limit = %d", cfg.Memory.SearchLimit)
	}
}

func TestLoadGuardrailYAMLAndEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	content := []byte(`
server:
  addr: ":9090"
llm:
  base_url: "http://yaml-base"
  api_key: "yaml-key"
  model: "yaml-model"
agent:
  name: test-agent
  instruction: "test instruction"
guardrail:
  medium_auto: true
  approval_ttl: 10m
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LLM_BASE_URL", "http://env-base")
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("LLM_MODEL", "env-model")
	t.Setenv("GUARDRAIL_MEDIUM_AUTO", "false")
	t.Setenv("GUARDRAIL_APPROVAL_TTL", "20m")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Guardrail.MediumAutoExecute() {
		t.Fatal("medium_auto should be false from env")
	}
	if cfg.Guardrail.ApprovalTTL.Duration() != 20*time.Minute {
		t.Fatalf("approval_ttl = %s", cfg.Guardrail.ApprovalTTL.Duration())
	}
}

func TestLoadBudgetFallbackObservabilityResilienceEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	content := []byte(`
server:
  addr: ":9090"
llm:
  base_url: "http://yaml-base"
  api_key: "yaml-key"
  model: "yaml-model"
agent:
  name: test-agent
  instruction: "test instruction"
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LLM_BASE_URL", "http://env-base")
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("LLM_MODEL", "env-model")
	t.Setenv("LLM_FALLBACK_BASE_URL", "http://fallback")
	t.Setenv("LLM_FALLBACK_API_KEY", "fallback-key")
	t.Setenv("LLM_FALLBACK_MODEL", "fallback-model")
	t.Setenv("AGENT_MAX_TOOL_CALLS", "7")
	t.Setenv("AGENT_MAX_RUNTIME", "3m")
	t.Setenv("AGENT_MAX_LLM_COST", "0.5")
	t.Setenv("OTEL_SERVICE_NAME", "soda-test")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel:4318")
	t.Setenv("RATE_LIMIT_RPS", "11")
	t.Setenv("CONCURRENCY_MAX", "4")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.LLM.Fallback.Enabled() || cfg.LLM.Fallback.Model != "fallback-model" {
		t.Fatalf("fallback = %+v", cfg.LLM.Fallback)
	}
	if cfg.Agent.Budget.MaxToolCalls != 7 {
		t.Fatalf("max_tool_calls = %d", cfg.Agent.Budget.MaxToolCalls)
	}
	if cfg.Agent.Budget.MaxRuntime.Duration() != 3*time.Minute {
		t.Fatalf("max_runtime = %s", cfg.Agent.Budget.MaxRuntime.Duration())
	}
	if cfg.Agent.Budget.MaxLLMCost != 0.5 {
		t.Fatalf("max_llm_cost = %v", cfg.Agent.Budget.MaxLLMCost)
	}
	if cfg.Observability.ServiceName != "soda-test" {
		t.Fatalf("service = %q", cfg.Observability.ServiceName)
	}
	if cfg.Observability.OTLPEndpoint != "http://otel:4318" {
		t.Fatalf("otlp = %q", cfg.Observability.OTLPEndpoint)
	}
	if cfg.Resilience.RateLimit.RPS != 11 {
		t.Fatalf("rps = %v", cfg.Resilience.RateLimit.RPS)
	}
	if cfg.Resilience.Concurrency.Max != 4 {
		t.Fatalf("concurrency = %d", cfg.Resilience.Concurrency.Max)
	}
}

func TestResolvePath(t *testing.T) {
	t.Setenv("CONFIG_ENV", "prod")
	if got := ResolvePath("configs/agent.yaml"); got != "configs/prod/agent.yaml" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("CONFIG_ENV", "dev")
	if got := ResolvePath("configs/agent.yaml"); got != "configs/dev/agent.yaml" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadRequiresLLMSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	if err := os.WriteFile(path, []byte("server:\n  addr: \":8080\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")

	if _, err := Load(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLLMConfigLooksPlaceholder(t *testing.T) {
	if !(LLMConfig{APIKey: "sk-your-key"}).LooksPlaceholder() {
		t.Fatal("expected placeholder detection for sk-your-key")
	}
	if (LLMConfig{APIKey: "env-key"}).LooksPlaceholder() {
		t.Fatal("did not expect placeholder detection for env-key")
	}
}
