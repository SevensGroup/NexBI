package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	envLLMBaseURL             = "LLM_BASE_URL"
	envLLMAPIKey              = "LLM_API_KEY"
	envLLMModel               = "LLM_MODEL"
	envAgentMaxSteps          = "AGENT_MAX_STEPS"
	envAgentMaxTasks          = "AGENT_MAX_TASKS"
	envToolTimeout            = "TOOL_TIMEOUT"
	envElasticsearchMCPURL    = "ELASTICSEARCH_MCP_URL"
	envSemanticDir            = "SEMANTIC_DIR"
	envSemanticMaxTimeRange   = "SEMANTIC_MAX_TIME_RANGE"
	envSemanticMaxResultSize  = "SEMANTIC_MAX_RESULT_SIZE"
	envMemoryPostgresURL      = "MEMORY_POSTGRES_URL"
	envMemoryShortTermTTL     = "MEMORY_SHORT_TERM_TTL"
	envMemoryTaskTTL          = "MEMORY_TASK_TTL"
	envMemoryLongTermTTL      = "MEMORY_LONG_TERM_TTL"
	envMemorySearchLimit      = "MEMORY_SEARCH_LIMIT"
	envGuardrailMediumAuto    = "GUARDRAIL_MEDIUM_AUTO"
	envGuardrailApprovalTTL   = "GUARDRAIL_APPROVAL_TTL"
	envLLMFallbackBaseURL     = "LLM_FALLBACK_BASE_URL"
	envLLMFallbackAPIKey      = "LLM_FALLBACK_API_KEY"
	envLLMFallbackModel       = "LLM_FALLBACK_MODEL"
	envAgentMaxToolCalls      = "AGENT_MAX_TOOL_CALLS"
	envAgentMaxRuntime        = "AGENT_MAX_RUNTIME"
	envAgentMaxLLMCost        = "AGENT_MAX_LLM_COST"
	envMaxStepsAlias          = "MAX_STEPS"
	envMaxToolCallsAlias      = "MAX_TOOL_CALLS"
	envMaxRuntimeAlias        = "MAX_RUNTIME"
	envMaxLLMCostAlias        = "MAX_LLM_COST"
	envOTELEndpoint           = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envOTELServiceName        = "OTEL_SERVICE_NAME"
	envRateLimitRPS           = "RATE_LIMIT_RPS"
	envRateLimitBurst         = "RATE_LIMIT_BURST"
	envConcurrencyMax         = "CONCURRENCY_MAX"
	envLLMRetryMaxAttempts    = "LLM_RETRY_MAX_ATTEMPTS"
	envBreakerFailures        = "CIRCUIT_BREAKER_FAILURE_THRESHOLD"
	envBreakerReset           = "CIRCUIT_BREAKER_RESET_TIMEOUT"
	envConfigEnv              = "CONFIG_ENV"
	defaultMaxSteps           = 8
	defaultMaxTasks           = 12
	defaultMaxToolCalls       = 50
	defaultMaxRuntime         = 10 * time.Minute
	defaultMaxLLMCost         = 1.0
	defaultToolTimeout        = 10 * time.Second
	defaultApprovalTTL        = 15 * time.Minute
	defaultRateLimitRPS       = 20.0
	defaultRateLimitBurst     = 40
	defaultConcurrencyMax     = 8
	defaultBreakerFailures    = 5
	defaultBreakerReset       = 30 * time.Second
	defaultLLMRetryAttempts   = 3
	defaultSemanticMaxRange   = 90 * 24 * time.Hour
	defaultSemanticMaxResults = 100
	defaultMemoryShortTTL     = 24 * time.Hour
	defaultMemoryTaskTTL      = 7 * 24 * time.Hour
	defaultMemorySearchLimit  = 20
	defaultESTimeout          = 10 * time.Second
	defaultESMaxSize          = 100
	defaultESMaxAggBuckets    = 100
	defaultESRetryAttempts    = 3
	defaultESRetryBackoff     = 200 * time.Millisecond
)

// Config is the application configuration loaded from YAML and environment variables.
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	LLM           LLMConfig           `yaml:"llm"`
	Agent         AgentConfig         `yaml:"agent"`
	Tools         ToolsConfig         `yaml:"tools"`
	Semantic      SemanticConfig      `yaml:"semantic"`
	Memory        MemoryConfig        `yaml:"memory"`
	Guardrail     GuardrailConfig     `yaml:"guardrail"`
	Observability ObservabilityConfig `yaml:"observability"`
	Resilience    ResilienceConfig    `yaml:"resilience"`
	Log           LogConfig           `yaml:"log"`
}

// ServerConfig controls the HTTP server.
type ServerConfig struct {
	Addr            string   `yaml:"addr"`
	ShutdownTimeout Duration `yaml:"shutdown_timeout"`
	ReadTimeout     Duration `yaml:"read_timeout"`
	WriteTimeout    Duration `yaml:"write_timeout"`
	IdleTimeout     Duration `yaml:"idle_timeout"`
	// HandlerTimeout bounds POST /api/v1/agent/run and /resume. Empty uses agent.budget.max_runtime.
	HandlerTimeout Duration `yaml:"handler_timeout"`
}

// LLMConfig is the OpenAI-compatible model endpoint configuration.
type LLMConfig struct {
	BaseURL  string            `yaml:"base_url"`
	APIKey   string            `yaml:"api_key"`
	Model    string            `yaml:"model"`
	Timeout  Duration          `yaml:"timeout"`
	Fallback FallbackLLMConfig `yaml:"fallback"`
}

// FallbackLLMConfig is the secondary model used when the primary fails.
type FallbackLLMConfig struct {
	BaseURL string   `yaml:"base_url"`
	APIKey  string   `yaml:"api_key"`
	Model   string   `yaml:"model"`
	Timeout Duration `yaml:"timeout"`
}

// LooksPlaceholder reports whether the API key is still an example value
// from .env.example (for example sk-your-key). The process can start so
// /health works; /api/v1/agent/run will fail until a real key is set.
func (c LLMConfig) LooksPlaceholder() bool {
	key := strings.ToLower(strings.TrimSpace(c.APIKey))
	switch key {
	case "sk-your-key", "your-api-key", "your_api_key", "changeme", "placeholder", "xxx", "sk-xxx":
		return true
	}
	return strings.Contains(key, "your-key") || strings.Contains(key, "your_key")
}

// Enabled reports whether a secondary LLM is configured.
func (f FallbackLLMConfig) Enabled() bool {
	return strings.TrimSpace(f.BaseURL) != "" && strings.TrimSpace(f.Model) != ""
}

// AsLLM copies fallback settings into an LLMConfig for the OpenAI provider.
func (f FallbackLLMConfig) AsLLM() LLMConfig {
	timeout := f.Timeout
	return LLMConfig{
		BaseURL: f.BaseURL,
		APIKey:  f.APIKey,
		Model:   f.Model,
		Timeout: timeout,
	}
}

// AgentConfig describes the Eino agent identity, system prompt, and loop limits.
type AgentConfig struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Instruction string       `yaml:"instruction"`
	MaxSteps    int          `yaml:"max_steps"`
	MaxTasks    int          `yaml:"max_tasks"`
	Budget      BudgetConfig `yaml:"budget"`
}

// BudgetConfig caps steps, tool calls, wall time, and LLM spend for one run.
type BudgetConfig struct {
	MaxToolCalls   int      `yaml:"max_tool_calls"`
	MaxRuntime     Duration `yaml:"max_runtime"`
	MaxLLMCost     float64  `yaml:"max_llm_cost"`
	InputUSDPer1K  float64  `yaml:"input_usd_per_1k"`
	OutputUSDPer1K float64  `yaml:"output_usd_per_1k"`
}

// ObservabilityConfig controls OpenTelemetry traces, metrics, and cost tracking.
type ObservabilityConfig struct {
	ServiceName  string `yaml:"service_name"`
	OTLPEndpoint string `yaml:"otlp_endpoint"`
	Traces       *bool  `yaml:"traces"`
	Metrics      *bool  `yaml:"metrics"`
}

// TracesEnabled defaults to true.
func (c ObservabilityConfig) TracesEnabled() bool {
	if c.Traces == nil {
		return true
	}
	return *c.Traces
}

// MetricsEnabled defaults to true.
func (c ObservabilityConfig) MetricsEnabled() bool {
	if c.Metrics == nil {
		return true
	}
	return *c.Metrics
}

// ResilienceConfig is retry, circuit breaker, rate limiter, and concurrency limiter.
type ResilienceConfig struct {
	Retry          RetryConfig          `yaml:"retry"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
	Concurrency    ConcurrencyConfig    `yaml:"concurrency"`
}

// CircuitBreakerConfig trips after consecutive failures and recovers after reset_timeout.
type CircuitBreakerConfig struct {
	FailureThreshold int      `yaml:"failure_threshold"`
	ResetTimeout     Duration `yaml:"reset_timeout"`
}

// RateLimitConfig is a token-bucket limiter for inbound HTTP.
type RateLimitConfig struct {
	RPS   float64 `yaml:"rps"`
	Burst int     `yaml:"burst"`
}

// ConcurrencyConfig caps in-flight HTTP / agent runs.
type ConcurrencyConfig struct {
	Max int `yaml:"max"`
}

// ToolsConfig controls the unified Tool Registry and Elasticsearch MCP.
type ToolsConfig struct {
	Timeout       Duration            `yaml:"timeout"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
}

// ElasticsearchConfig controls the read-only Elasticsearch MCP integration.
type ElasticsearchConfig struct {
	ConfigPath            string      `yaml:"config"`
	MCPURL                string      `yaml:"mcp_url"`
	Timeout               Duration    `yaml:"timeout"`
	Retry                 RetryConfig `yaml:"retry"`
	MaxSize               int         `yaml:"max_size"`
	MaxAggregationBuckets int         `yaml:"max_aggregation_buckets"`
	AllowedTools          []string    `yaml:"allowed_tools"`
	DeniedTools           []string    `yaml:"denied_tools"`
	ReadOnly              *bool       `yaml:"read_only"`
}

// RetryConfig controls MCP client retries.
type RetryConfig struct {
	MaxAttempts int      `yaml:"max_attempts"`
	Backoff     Duration `yaml:"backoff"`
}

// Enabled reports whether an Elasticsearch MCP endpoint is configured.
func (c ElasticsearchConfig) Enabled() bool {
	return strings.TrimSpace(c.MCPURL) != ""
}

// SemanticConfig controls the advertising Semantic Layer catalog and query limits.
type SemanticConfig struct {
	Dir           string   `yaml:"dir"`
	MaxTimeRange  Duration `yaml:"max_time_range"`
	MaxResultSize int      `yaml:"max_result_size"`
}

// MemoryConfig controls the three-layer Agent Memory store (PostgreSQL in phase 1).
type MemoryConfig struct {
	PostgresURL  string   `yaml:"postgres_url"`
	ShortTermTTL Duration `yaml:"short_term_ttl"`
	TaskTTL      Duration `yaml:"task_ttl"`
	LongTermTTL  Duration `yaml:"long_term_ttl"`
	SearchLimit  int      `yaml:"search_limit"`
}

// GuardrailConfig controls the Risk Engine and Human-in-the-Loop approvals.
type GuardrailConfig struct {
	// MediumAuto: MEDIUM actions auto-execute when true (default). When false they require approval.
	MediumAuto *bool `yaml:"medium_auto"`
	// ApprovalTTL is how long a pending HIGH/CRITICAL (or gated MEDIUM) request stays valid.
	ApprovalTTL Duration `yaml:"approval_ttl"`
}

// MediumAutoExecute reports whether MEDIUM risk actions run without approval.
func (c GuardrailConfig) MediumAutoExecute() bool {
	if c.MediumAuto == nil {
		return true
	}
	return *c.MediumAuto
}

// LogConfig controls structured logging.
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Duration is a YAML-friendly wrapper around time.Duration.
type Duration time.Duration

// UnmarshalYAML parses values such as "15s".
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return err
	}
	parsed, err := parseFlexibleDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", raw, err)
	}
	*d = Duration(parsed)
	return nil
}

func parseFlexibleDuration(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(raw, "d") {
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(raw, "d")))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", raw)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(raw)
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Load reads YAML from path, then overlays LLM_* environment variables.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := defaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := loadElasticsearchFile(path, cfg); err != nil {
		return nil, err
	}
	resolveSemanticDir(path, cfg)
	if err := applyEnv(cfg); err != nil {
		return nil, err
	}
	applyDefaults(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks required runtime settings after YAML and env overlay.
func (c *Config) Validate() error {
	if c.Server.Addr == "" {
		return fmt.Errorf("server.addr is required")
	}
	if c.LLM.BaseURL == "" {
		return fmt.Errorf("llm.base_url is required (set LLM_BASE_URL)")
	}
	if c.LLM.APIKey == "" {
		return fmt.Errorf("llm.api_key is required (set LLM_API_KEY)")
	}
	if c.LLM.Model == "" {
		return fmt.Errorf("llm.model is required (set LLM_MODEL)")
	}
	if c.Agent.Name == "" {
		return fmt.Errorf("agent.name is required")
	}
	if c.Agent.Instruction == "" {
		return fmt.Errorf("agent.instruction is required")
	}
	if c.Agent.MaxSteps < 1 {
		return fmt.Errorf("agent.max_steps must be >= 1 (set AGENT_MAX_STEPS)")
	}
	if c.Agent.MaxTasks < 1 {
		return fmt.Errorf("agent.max_tasks must be >= 1 (set AGENT_MAX_TASKS)")
	}
	if c.Tools.Timeout.Duration() <= 0 {
		return fmt.Errorf("tools.timeout must be > 0 (set TOOL_TIMEOUT)")
	}
	if err := c.Tools.Elasticsearch.validate(); err != nil {
		return err
	}
	if err := c.Semantic.validate(); err != nil {
		return err
	}
	if err := c.Memory.validate(); err != nil {
		return err
	}
	if err := c.Guardrail.validate(); err != nil {
		return err
	}
	if err := c.Agent.Budget.validate(); err != nil {
		return err
	}
	if err := c.Resilience.validate(); err != nil {
		return err
	}
	if c.LLM.Fallback.Enabled() {
		if strings.TrimSpace(c.LLM.Fallback.APIKey) == "" {
			c.LLM.Fallback.APIKey = c.LLM.APIKey
		}
		if c.LLM.Fallback.Timeout.Duration() <= 0 {
			c.LLM.Fallback.Timeout = c.LLM.Timeout
		}
	}
	return nil
}

func (c GuardrailConfig) validate() error {
	if c.ApprovalTTL.Duration() <= 0 {
		return fmt.Errorf("guardrail.approval_ttl must be > 0 (set GUARDRAIL_APPROVAL_TTL)")
	}
	return nil
}

func (c BudgetConfig) validate() error {
	if c.MaxToolCalls < 1 {
		return fmt.Errorf("agent.budget.max_tool_calls must be >= 1 (set AGENT_MAX_TOOL_CALLS)")
	}
	if c.MaxRuntime.Duration() <= 0 {
		return fmt.Errorf("agent.budget.max_runtime must be > 0 (set AGENT_MAX_RUNTIME)")
	}
	if c.MaxLLMCost < 0 {
		return fmt.Errorf("agent.budget.max_llm_cost must be >= 0 (set AGENT_MAX_LLM_COST)")
	}
	return nil
}

func (c ResilienceConfig) validate() error {
	if c.Retry.MaxAttempts < 1 {
		return fmt.Errorf("resilience.retry.max_attempts must be >= 1")
	}
	if c.CircuitBreaker.FailureThreshold < 1 {
		return fmt.Errorf("resilience.circuit_breaker.failure_threshold must be >= 1")
	}
	if c.CircuitBreaker.ResetTimeout.Duration() <= 0 {
		return fmt.Errorf("resilience.circuit_breaker.reset_timeout must be > 0")
	}
	if c.RateLimit.RPS <= 0 {
		return fmt.Errorf("resilience.rate_limit.rps must be > 0")
	}
	if c.RateLimit.Burst < 1 {
		return fmt.Errorf("resilience.rate_limit.burst must be >= 1")
	}
	if c.Concurrency.Max < 1 {
		return fmt.Errorf("resilience.concurrency.max must be >= 1")
	}
	return nil
}

func (c SemanticConfig) validate() error {
	if strings.TrimSpace(c.Dir) == "" {
		return nil
	}
	if c.MaxTimeRange.Duration() <= 0 {
		return fmt.Errorf("semantic.max_time_range must be > 0 (set SEMANTIC_MAX_TIME_RANGE)")
	}
	if c.MaxResultSize < 1 {
		return fmt.Errorf("semantic.max_result_size must be >= 1 (set SEMANTIC_MAX_RESULT_SIZE)")
	}
	return nil
}

func (c MemoryConfig) validate() error {
	if dsn := strings.TrimSpace(c.PostgresURL); dsn != "" {
		u, err := url.Parse(dsn)
		if err != nil || u.Host == "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
			return fmt.Errorf("memory.postgres_url must be a postgres URL (set MEMORY_POSTGRES_URL)")
		}
	}
	if c.ShortTermTTL.Duration() < 0 {
		return fmt.Errorf("memory.short_term_ttl must be >= 0 (set MEMORY_SHORT_TERM_TTL)")
	}
	if c.TaskTTL.Duration() < 0 {
		return fmt.Errorf("memory.task_ttl must be >= 0 (set MEMORY_TASK_TTL)")
	}
	if c.LongTermTTL.Duration() < 0 {
		return fmt.Errorf("memory.long_term_ttl must be >= 0 (set MEMORY_LONG_TERM_TTL)")
	}
	if c.SearchLimit < 1 {
		return fmt.Errorf("memory.search_limit must be >= 1 (set MEMORY_SEARCH_LIMIT)")
	}
	return nil
}

func (c ElasticsearchConfig) validate() error {
	if !c.Enabled() {
		return nil
	}
	u, err := url.Parse(strings.TrimSpace(c.MCPURL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("elasticsearch.mcp_url must be an http(s) URL (set ELASTICSEARCH_MCP_URL)")
	}
	if c.Timeout.Duration() <= 0 {
		return fmt.Errorf("elasticsearch.timeout must be > 0")
	}
	if c.MaxSize < 1 {
		return fmt.Errorf("elasticsearch.max_size must be >= 1")
	}
	if c.MaxAggregationBuckets < 1 {
		return fmt.Errorf("elasticsearch.max_aggregation_buckets must be >= 1")
	}
	if c.Retry.MaxAttempts < 1 {
		return fmt.Errorf("elasticsearch.retry.max_attempts must be >= 1")
	}
	return nil
}

func defaultConfig() *Config {
	readOnly := true
	return &Config{
		Server: ServerConfig{
			Addr:            ":8080",
			ShutdownTimeout: Duration(15 * time.Second),
			ReadTimeout:     Duration(15 * time.Second),
			WriteTimeout:    Duration(120 * time.Second),
		},
		LLM: LLMConfig{
			Timeout: Duration(60 * time.Second),
		},
		Agent: AgentConfig{
			Name:        "autonomous-agent",
			Description: "A minimal Eino autonomous agent",
			Instruction: "You are a helpful assistant.",
			MaxSteps:    defaultMaxSteps,
			MaxTasks:    defaultMaxTasks,
			Budget: BudgetConfig{
				MaxToolCalls: defaultMaxToolCalls,
				MaxRuntime:   Duration(defaultMaxRuntime),
				MaxLLMCost:   defaultMaxLLMCost,
			},
		},
		Tools: ToolsConfig{
			Timeout: Duration(defaultToolTimeout),
			Elasticsearch: ElasticsearchConfig{
				ConfigPath:            "configs/tools/elasticsearch.yaml",
				Timeout:               Duration(defaultESTimeout),
				Retry:                 RetryConfig{MaxAttempts: defaultESRetryAttempts, Backoff: Duration(defaultESRetryBackoff)},
				MaxSize:               defaultESMaxSize,
				MaxAggregationBuckets: defaultESMaxAggBuckets,
				AllowedTools:          []string{"search", "get", "count", "aggregate"},
				DeniedTools:           []string{"delete", "update", "index", "bulk"},
				ReadOnly:              &readOnly,
			},
		},
		Semantic: SemanticConfig{
			Dir:           "configs/semantic",
			MaxTimeRange:  Duration(defaultSemanticMaxRange),
			MaxResultSize: defaultSemanticMaxResults,
		},
		Memory: MemoryConfig{
			ShortTermTTL: Duration(defaultMemoryShortTTL),
			TaskTTL:      Duration(defaultMemoryTaskTTL),
			SearchLimit:  defaultMemorySearchLimit,
		},
		Guardrail: GuardrailConfig{
			ApprovalTTL: Duration(defaultApprovalTTL),
		},
		Observability: ObservabilityConfig{
			ServiceName: "autonomous-agent",
		},
		Resilience: ResilienceConfig{
			Retry: RetryConfig{MaxAttempts: defaultLLMRetryAttempts, Backoff: Duration(defaultESRetryBackoff)},
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold: defaultBreakerFailures,
				ResetTimeout:     Duration(defaultBreakerReset),
			},
			RateLimit:   RateLimitConfig{RPS: defaultRateLimitRPS, Burst: defaultRateLimitBurst},
			Concurrency: ConcurrencyConfig{Max: defaultConcurrencyMax},
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

func applyEnv(cfg *Config) error {
	if v := os.Getenv(envLLMBaseURL); v != "" {
		cfg.LLM.BaseURL = v
	}
	if v := os.Getenv(envLLMAPIKey); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv(envLLMModel); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv(envAgentMaxSteps); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid AGENT_MAX_STEPS %q: %w", v, err)
		}
		cfg.Agent.MaxSteps = n
	}
	if v := os.Getenv(envAgentMaxTasks); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid AGENT_MAX_TASKS %q: %w", v, err)
		}
		cfg.Agent.MaxTasks = n
	}
	if v := os.Getenv(envToolTimeout); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid TOOL_TIMEOUT %q: %w", v, err)
		}
		cfg.Tools.Timeout = Duration(d)
	}
	if v := os.Getenv(envElasticsearchMCPURL); v != "" {
		cfg.Tools.Elasticsearch.MCPURL = v
	}
	if v := os.Getenv(envSemanticDir); v != "" {
		cfg.Semantic.Dir = v
	}
	if v := os.Getenv(envSemanticMaxTimeRange); v != "" {
		d, err := parseFlexibleDuration(v)
		if err != nil {
			return fmt.Errorf("invalid SEMANTIC_MAX_TIME_RANGE %q: %w", v, err)
		}
		cfg.Semantic.MaxTimeRange = Duration(d)
	}
	if v := os.Getenv(envSemanticMaxResultSize); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid SEMANTIC_MAX_RESULT_SIZE %q: %w", v, err)
		}
		cfg.Semantic.MaxResultSize = n
	}
	if v := os.Getenv(envMemoryPostgresURL); v != "" {
		cfg.Memory.PostgresURL = v
	}
	if v := os.Getenv(envMemoryShortTermTTL); v != "" {
		d, err := parseFlexibleDuration(v)
		if err != nil {
			return fmt.Errorf("invalid MEMORY_SHORT_TERM_TTL %q: %w", v, err)
		}
		cfg.Memory.ShortTermTTL = Duration(d)
	}
	if v := os.Getenv(envMemoryTaskTTL); v != "" {
		d, err := parseFlexibleDuration(v)
		if err != nil {
			return fmt.Errorf("invalid MEMORY_TASK_TTL %q: %w", v, err)
		}
		cfg.Memory.TaskTTL = Duration(d)
	}
	if v := os.Getenv(envMemoryLongTermTTL); v != "" {
		d, err := parseFlexibleDuration(v)
		if err != nil {
			return fmt.Errorf("invalid MEMORY_LONG_TERM_TTL %q: %w", v, err)
		}
		cfg.Memory.LongTermTTL = Duration(d)
	}
	if v := os.Getenv(envMemorySearchLimit); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid MEMORY_SEARCH_LIMIT %q: %w", v, err)
		}
		cfg.Memory.SearchLimit = n
	}
	if v := os.Getenv(envGuardrailMediumAuto); v != "" {
		b, err := parseBool(v)
		if err != nil {
			return fmt.Errorf("invalid GUARDRAIL_MEDIUM_AUTO %q: %w", v, err)
		}
		cfg.Guardrail.MediumAuto = &b
	}
	if v := os.Getenv(envGuardrailApprovalTTL); v != "" {
		d, err := parseFlexibleDuration(v)
		if err != nil {
			return fmt.Errorf("invalid GUARDRAIL_APPROVAL_TTL %q: %w", v, err)
		}
		cfg.Guardrail.ApprovalTTL = Duration(d)
	}
	if v := firstEnv(envLLMFallbackBaseURL); v != "" {
		cfg.LLM.Fallback.BaseURL = v
	}
	if v := firstEnv(envLLMFallbackAPIKey); v != "" {
		cfg.LLM.Fallback.APIKey = v
	}
	if v := firstEnv(envLLMFallbackModel); v != "" {
		cfg.LLM.Fallback.Model = v
	}
	if v := firstEnv(envAgentMaxToolCalls, envMaxToolCallsAlias); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid AGENT_MAX_TOOL_CALLS %q: %w", v, err)
		}
		cfg.Agent.Budget.MaxToolCalls = n
	}
	if v := firstEnv(envAgentMaxRuntime, envMaxRuntimeAlias); v != "" {
		d, err := parseFlexibleDuration(v)
		if err != nil {
			return fmt.Errorf("invalid AGENT_MAX_RUNTIME %q: %w", v, err)
		}
		cfg.Agent.Budget.MaxRuntime = Duration(d)
	}
	if v := firstEnv(envAgentMaxLLMCost, envMaxLLMCostAlias); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid AGENT_MAX_LLM_COST %q: %w", v, err)
		}
		cfg.Agent.Budget.MaxLLMCost = f
	}
	if v := firstEnv(envMaxStepsAlias); v != "" && os.Getenv(envAgentMaxSteps) == "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid MAX_STEPS %q: %w", v, err)
		}
		cfg.Agent.MaxSteps = n
	}
	if v := firstEnv(envOTELEndpoint); v != "" {
		cfg.Observability.OTLPEndpoint = v
	}
	if v := firstEnv(envOTELServiceName); v != "" {
		cfg.Observability.ServiceName = v
	}
	if v := firstEnv(envRateLimitRPS); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid RATE_LIMIT_RPS %q: %w", v, err)
		}
		cfg.Resilience.RateLimit.RPS = f
	}
	if v := firstEnv(envRateLimitBurst); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid RATE_LIMIT_BURST %q: %w", v, err)
		}
		cfg.Resilience.RateLimit.Burst = n
	}
	if v := firstEnv(envConcurrencyMax); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid CONCURRENCY_MAX %q: %w", v, err)
		}
		cfg.Resilience.Concurrency.Max = n
	}
	if v := firstEnv(envLLMRetryMaxAttempts); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid LLM_RETRY_MAX_ATTEMPTS %q: %w", v, err)
		}
		cfg.Resilience.Retry.MaxAttempts = n
	}
	if v := firstEnv(envBreakerFailures); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid CIRCUIT_BREAKER_FAILURE_THRESHOLD %q: %w", v, err)
		}
		cfg.Resilience.CircuitBreaker.FailureThreshold = n
	}
	if v := firstEnv(envBreakerReset); v != "" {
		d, err := parseFlexibleDuration(v)
		if err != nil {
			return fmt.Errorf("invalid CIRCUIT_BREAKER_RESET_TIMEOUT %q: %w", v, err)
		}
		cfg.Resilience.CircuitBreaker.ResetTimeout = Duration(d)
	}
	return nil
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("not a boolean")
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Server.ShutdownTimeout == 0 {
		cfg.Server.ShutdownTimeout = Duration(15 * time.Second)
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = Duration(15 * time.Second)
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = Duration(120 * time.Second)
	}
	if cfg.LLM.Timeout == 0 {
		cfg.LLM.Timeout = Duration(60 * time.Second)
	}
	if cfg.Agent.Name == "" {
		cfg.Agent.Name = "autonomous-agent"
	}
	if cfg.Agent.Instruction == "" {
		cfg.Agent.Instruction = "You are a helpful assistant."
	}
	if cfg.Agent.MaxSteps == 0 {
		cfg.Agent.MaxSteps = defaultMaxSteps
	}
	if cfg.Agent.MaxTasks == 0 {
		cfg.Agent.MaxTasks = defaultMaxTasks
	}
	if cfg.Tools.Timeout == 0 {
		cfg.Tools.Timeout = Duration(defaultToolTimeout)
	}
	applyESDefaults(&cfg.Tools.Elasticsearch)
	applySemanticDefaults(&cfg.Semantic)
	applyMemoryDefaults(&cfg.Memory)
	applyGuardrailDefaults(&cfg.Guardrail)
	applyBudgetDefaults(&cfg.Agent.Budget)
	applyObservabilityDefaults(&cfg.Observability)
	applyResilienceDefaults(&cfg.Resilience)
	applyServerTimeoutDefaults(cfg)
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
}

func applyServerTimeoutDefaults(cfg *Config) {
	if cfg == nil {
		return
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = Duration(90 * time.Second)
	}
	if cfg.Server.HandlerTimeout == 0 {
		cfg.Server.HandlerTimeout = cfg.Agent.Budget.MaxRuntime
	}
	writeFloor := cfg.Server.HandlerTimeout.Duration() + 30*time.Second
	if cfg.LLM.Timeout.Duration() > 0 {
		alt := cfg.Server.HandlerTimeout.Duration() + cfg.LLM.Timeout.Duration() + 15*time.Second
		if alt > writeFloor {
			writeFloor = alt
		}
	}
	if cfg.Server.WriteTimeout.Duration() < writeFloor {
		cfg.Server.WriteTimeout = Duration(writeFloor)
	}
}

func applyESDefaults(es *ElasticsearchConfig) {
	if es.Timeout == 0 {
		es.Timeout = Duration(defaultESTimeout)
	}
	if es.Retry.MaxAttempts == 0 {
		es.Retry.MaxAttempts = defaultESRetryAttempts
	}
	if es.Retry.Backoff == 0 {
		es.Retry.Backoff = Duration(defaultESRetryBackoff)
	}
	if es.MaxSize == 0 {
		es.MaxSize = defaultESMaxSize
	}
	if es.MaxAggregationBuckets == 0 {
		es.MaxAggregationBuckets = defaultESMaxAggBuckets
	}
	if len(es.AllowedTools) == 0 {
		es.AllowedTools = []string{"search", "get", "count", "aggregate"}
	}
	if len(es.DeniedTools) == 0 {
		es.DeniedTools = []string{"delete", "update", "index", "bulk"}
	}
	if es.ReadOnly == nil {
		readOnly := true
		es.ReadOnly = &readOnly
	}
	if strings.TrimSpace(es.ConfigPath) == "" {
		es.ConfigPath = "configs/tools/elasticsearch.yaml"
	}
}

func applySemanticDefaults(s *SemanticConfig) {
	if strings.TrimSpace(s.Dir) == "" {
		s.Dir = "configs/semantic"
	}
	if s.MaxTimeRange == 0 {
		s.MaxTimeRange = Duration(defaultSemanticMaxRange)
	}
	if s.MaxResultSize == 0 {
		s.MaxResultSize = defaultSemanticMaxResults
	}
}

func applyMemoryDefaults(m *MemoryConfig) {
	if m.ShortTermTTL == 0 {
		m.ShortTermTTL = Duration(defaultMemoryShortTTL)
	}
	if m.TaskTTL == 0 {
		m.TaskTTL = Duration(defaultMemoryTaskTTL)
	}
	if m.SearchLimit == 0 {
		m.SearchLimit = defaultMemorySearchLimit
	}
}

func applyGuardrailDefaults(g *GuardrailConfig) {
	if g.ApprovalTTL == 0 {
		g.ApprovalTTL = Duration(defaultApprovalTTL)
	}
}

func applyBudgetDefaults(b *BudgetConfig) {
	if b.MaxToolCalls == 0 {
		b.MaxToolCalls = defaultMaxToolCalls
	}
	if b.MaxRuntime == 0 {
		b.MaxRuntime = Duration(defaultMaxRuntime)
	}
	if b.MaxLLMCost == 0 {
		b.MaxLLMCost = defaultMaxLLMCost
	}
}

func applyObservabilityDefaults(o *ObservabilityConfig) {
	if strings.TrimSpace(o.ServiceName) == "" {
		o.ServiceName = "autonomous-agent"
	}
}

func applyResilienceDefaults(r *ResilienceConfig) {
	if r.Retry.MaxAttempts == 0 {
		r.Retry.MaxAttempts = defaultLLMRetryAttempts
	}
	if r.Retry.Backoff == 0 {
		r.Retry.Backoff = Duration(defaultESRetryBackoff)
	}
	if r.CircuitBreaker.FailureThreshold == 0 {
		r.CircuitBreaker.FailureThreshold = defaultBreakerFailures
	}
	if r.CircuitBreaker.ResetTimeout == 0 {
		r.CircuitBreaker.ResetTimeout = Duration(defaultBreakerReset)
	}
	if r.RateLimit.RPS == 0 {
		r.RateLimit.RPS = defaultRateLimitRPS
	}
	if r.RateLimit.Burst == 0 {
		r.RateLimit.Burst = defaultRateLimitBurst
	}
	if r.Concurrency.Max == 0 {
		r.Concurrency.Max = defaultConcurrencyMax
	}
}

// ResolvePath picks configs/{dev,test,prod}/agent.yaml from CONFIG_ENV when path is the default.
func ResolvePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "configs/agent.yaml"
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv(envConfigEnv)))
	if env == "" {
		return path
	}
	if path != "configs/agent.yaml" && path != "./configs/agent.yaml" {
		return path
	}
	switch env {
	case "prod", "production":
		return "configs/prod/agent.yaml"
	case "test":
		return "configs/test/agent.yaml"
	case "dev", "development":
		return "configs/dev/agent.yaml"
	default:
		return path
	}
}

func resolveSemanticDir(agentPath string, cfg *Config) {
	if cfg == nil {
		return
	}
	resolved := findSemanticDir(agentPath, cfg.Semantic.Dir)
	if resolved != "" {
		cfg.Semantic.Dir = resolved
	}
}

func findSemanticDir(agentPath, configured string) string {
	candidates := make([]string, 0, 6)
	if strings.TrimSpace(configured) != "" {
		candidates = append(candidates, configured)
		if !filepath.IsAbs(configured) {
			candidates = append(candidates, filepath.Join(filepath.Dir(agentPath), configured))
			candidates = append(candidates, filepath.Join(filepath.Dir(agentPath), filepath.Base(configured)))
		}
	}
	candidates = append(candidates,
		filepath.Join(filepath.Dir(agentPath), "semantic"),
		"configs/semantic",
	)
	for _, c := range candidates {
		if strings.TrimSpace(c) == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return ""
}

func loadElasticsearchFile(agentPath string, cfg *Config) error {
	path := resolveElasticsearchPath(agentPath, cfg.Tools.Elasticsearch.ConfigPath)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && strings.TrimSpace(cfg.Tools.Elasticsearch.ConfigPath) == "" {
			return nil
		}
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read elasticsearch config %s: %w", path, err)
	}
	var file ElasticsearchConfig
	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parse elasticsearch config %s: %w", path, err)
	}
	mergeElasticsearch(&cfg.Tools.Elasticsearch, file)
	return nil
}

func resolveElasticsearchPath(agentPath, configured string) string {
	candidates := make([]string, 0, 4)
	if strings.TrimSpace(configured) != "" {
		candidates = append(candidates, configured)
		if !filepath.IsAbs(configured) {
			candidates = append(candidates, filepath.Join(filepath.Dir(agentPath), configured))
		}
	}
	candidates = append(candidates,
		filepath.Join(filepath.Dir(agentPath), "tools", "elasticsearch.yaml"),
		"configs/tools/elasticsearch.yaml",
	)
	for _, c := range candidates {
		if strings.TrimSpace(c) == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func mergeElasticsearch(dst *ElasticsearchConfig, src ElasticsearchConfig) {
	if strings.TrimSpace(src.MCPURL) != "" {
		dst.MCPURL = src.MCPURL
	}
	if src.Timeout != 0 {
		dst.Timeout = src.Timeout
	}
	if src.Retry.MaxAttempts != 0 {
		dst.Retry.MaxAttempts = src.Retry.MaxAttempts
	}
	if src.Retry.Backoff != 0 {
		dst.Retry.Backoff = src.Retry.Backoff
	}
	if src.MaxSize != 0 {
		dst.MaxSize = src.MaxSize
	}
	if src.MaxAggregationBuckets != 0 {
		dst.MaxAggregationBuckets = src.MaxAggregationBuckets
	}
	if len(src.AllowedTools) > 0 {
		dst.AllowedTools = append([]string(nil), src.AllowedTools...)
	}
	if len(src.DeniedTools) > 0 {
		dst.DeniedTools = append([]string(nil), src.DeniedTools...)
	}
	if src.ReadOnly != nil {
		dst.ReadOnly = src.ReadOnly
	}
}
