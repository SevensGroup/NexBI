package api

import (
	"net/http"
	"time"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/observability"
	"autonomous-agent/internal/resilience"
)

func wrapHandler(h http.Handler, resCfg config.ResilienceConfig, log logger.Logger) http.Handler {
	limiter := resilience.NewRateLimiter(resCfg.RateLimit.RPS, resCfg.RateLimit.Burst)
	gate := resilience.NewConcurrencyLimiter(resCfg.Concurrency.Max)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		if err := gate.TryAcquire(2 * time.Second); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"concurrency limit reached"}`))
			return
		}
		defer gate.Release()

		ctx, span := observability.Global().StartSpan(r.Context(), "http.server")
		defer span.End()
		if log != nil {
			log.InfoContext(ctx, "http request", "method", r.Method, "path", r.URL.Path)
		}
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
