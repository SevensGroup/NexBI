package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"autonomous-agent/internal/agent/runtime"
	"autonomous-agent/internal/api"
	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/loginlog"
	"autonomous-agent/internal/model"
	"autonomous-agent/internal/observability"
)

func main() {
	configPath := flag.String("config", "configs/agent.yaml", "path to agent YAML config")
	flag.Parse()

	path := config.ResolvePath(*configPath)
	if err := run(path); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	log := logger.New(cfg.Log.Level, cfg.Log.Format, os.Stdout)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tel, err := observability.Setup(ctx, observability.Config{
		ServiceName:  cfg.Observability.ServiceName,
		OTLPEndpoint: cfg.Observability.OTLPEndpoint,
		Traces:       cfg.Observability.TracesEnabled(),
		Metrics:      cfg.Observability.MetricsEnabled(),
		Cost: observability.CostModel{
			InputUSDPer1K:  cfg.Agent.Budget.InputUSDPer1K,
			OutputUSDPer1K: cfg.Agent.Budget.OutputUSDPer1K,
		},
	}, log)
	if err != nil {
		return err
	}
	defer tel.Shutdown(context.Background())

	provider := model.NewProviderFromConfig(cfg.LLM, cfg.Resilience, tel, log)
	rt, err := runtime.New(ctx, provider, cfg.Agent, cfg.Tools, cfg.Semantic, cfg.Memory, cfg.Guardrail, log, runtime.Deps{Telemetry: tel})
	if err != nil {
		return err
	}
	defer rt.Close()

	logins, err := loginlog.Open(ctx, "")
	if err != nil {
		return err
	}
	defer logins.Close()
	log.InfoContext(ctx, "login log store ready", "database", logins.UsesDatabase())

	srv := api.NewServer(cfg.Server, rt, log, rt.Boards(), logins, cfg.Resilience)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	if cfg.LLM.LooksPlaceholder() {
		log.WarnContext(ctx, "LLM_API_KEY looks like a placeholder; /api/v1/agent/run will fail until a real OpenAI-compatible key is set",
			"hint", "copy .env.example to .env and set LLM_API_KEY, LLM_BASE_URL, LLM_MODEL")
	}

	log.InfoContext(ctx, "autonomous-agent started",
		"config", configPath,
		"addr", cfg.Server.Addr,
		"model", cfg.LLM.Model,
		"fallback_model", cfg.LLM.Fallback.Model,
		"otlp", cfg.Observability.OTLPEndpoint,
	)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout.Duration())
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errCh
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	}
}
