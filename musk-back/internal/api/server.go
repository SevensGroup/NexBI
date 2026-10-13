package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"autonomous-agent/internal/agent/runtime"
	"autonomous-agent/internal/analyst"
	"autonomous-agent/internal/config"
	"autonomous-agent/internal/guardrail"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
)

// Server is the HTTP process boundary. It depends on runtime.Runner, not Eino.
type Server struct {
	httpServer *http.Server
	logger     logger.Logger
}

type handler struct {
	runner runtime.Runner
	logger logger.Logger
}

type runRequest struct {
	Message string `json:"message"`
}

type resumeRequest struct {
	RunID string `json:"run_id"`
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

type runResponse struct {
	RequestID  string          `json:"request_id"`
	RunID      string          `json:"run_id,omitempty"`
	Status     string          `json:"status,omitempty"`
	Answer     string          `json:"answer,omitempty"`
	Confidence float64         `json:"confidence,omitempty"`
	Evidence   []any           `json:"evidence,omitempty"`
	Steps      int             `json:"steps,omitempty"`
	Report     *analyst.Report `json:"report,omitempty"`
	ApprovalID string          `json:"approval_id,omitempty"`
	Tool       string          `json:"tool,omitempty"`
	Risk       string          `json:"risk,omitempty"`
}

type approvalResponse struct {
	RequestID  string `json:"request_id"`
	ApprovalID string `json:"approval_id"`
	Status     string `json:"status"`
}

type errorResponse struct {
	RequestID string `json:"request_id"`
	Error     string `json:"error"`
}

type healthResponse struct {
	Status string `json:"status"`
}

// NewServer wires HTTP routes to the agent runtime.
func NewServer(cfg config.ServerConfig, runner runtime.Runner, log logger.Logger, extras ...config.ResilienceConfig) *Server {
	h := &handler{runner: runner, logger: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /api/v1/agent/run", h.withRequestID(h.run))
	mux.HandleFunc("POST /api/v1/agent/resume", h.withRequestID(h.resume))
	mux.HandleFunc("POST /api/v1/agent/approvals/{id}/approve", h.withRequestID(h.approve))
	mux.HandleFunc("POST /api/v1/agent/approvals/{id}/reject", h.withRequestID(h.reject))
	mux.HandleFunc("GET /api/v1/runs/{id}", h.withRequestID(h.getRun))
	mux.HandleFunc("GET /api/v1/agents", h.withRequestID(h.listAgents))
	mux.HandleFunc("GET /api/v1/tools", h.withRequestID(h.listTools))
	mux.HandleFunc("GET /api/v1/semantic/metrics", h.withRequestID(h.listMetrics))
	mux.HandleFunc("GET /api/v1/semantic/dimensions", h.withRequestID(h.listDimensions))

	var handler http.Handler = mux
	if len(extras) > 0 {
		handler = wrapHandler(mux, extras[0], log)
	}
	if timeout := cfg.HandlerTimeout.Duration(); timeout > 0 {
		handler = http.TimeoutHandler(handler, timeout, `{"error":"agent run exceeded handler timeout"}`)
	}

	writeTimeout := cfg.WriteTimeout.Duration()
	if ht := cfg.HandlerTimeout.Duration(); ht > 0 && writeTimeout < ht+15*time.Second {
		writeTimeout = ht + 15*time.Second
	}

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.Addr,
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout.Duration(),
			WriteTimeout: writeTimeout,
			IdleTimeout:  cfg.IdleTimeout.Duration(),
		},
		logger: log,
	}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	s.logger.InfoContext(context.Background(), "http server listening", "addr", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown stops the server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.InfoContext(ctx, "http server shutting down")
	return s.httpServer.Shutdown(ctx)
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

func (h *handler) run(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := requestid.From(ctx)

	var req runRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, id, "invalid json body")
		return
	}

	answer, err := h.runner.Run(ctx, req.Message)
	h.writeRunResult(w, ctx, id, answer, err)
}

func (h *handler) resume(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := requestid.From(ctx)

	var req resumeRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, id, "invalid json body")
		return
	}
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		runID = id
	}

	answer, err := h.runner.Resume(ctx, runID)
	h.writeRunResult(w, ctx, id, answer, err)
}

func (h *handler) approve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := requestid.From(ctx)
	approvalID := strings.TrimSpace(r.PathValue("id"))
	if approvalID == "" {
		h.writeError(w, http.StatusBadRequest, id, "approval id is required")
		return
	}
	if err := h.runner.Approve(ctx, approvalID); err != nil {
		h.writeApprovalError(w, ctx, id, err)
		return
	}
	writeJSON(w, http.StatusOK, approvalResponse{
		RequestID:  id,
		ApprovalID: approvalID,
		Status:     string(guardrail.StatusApproved),
	})
}

func (h *handler) reject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := requestid.From(ctx)
	approvalID := strings.TrimSpace(r.PathValue("id"))
	if approvalID == "" {
		h.writeError(w, http.StatusBadRequest, id, "approval id is required")
		return
	}

	var req rejectRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	_ = dec.Decode(&req)

	if err := h.runner.Reject(ctx, approvalID, req.Reason); err != nil {
		h.writeApprovalError(w, ctx, id, err)
		return
	}
	writeJSON(w, http.StatusOK, approvalResponse{
		RequestID:  id,
		ApprovalID: approvalID,
		Status:     string(guardrail.StatusRejected),
	})
}

func (h *handler) writeRunResult(w http.ResponseWriter, ctx context.Context, id, answer string, err error) {
	if err == nil {
		resp := runResponse{
			RequestID: id,
			RunID:     id,
			Status:    "COMPLETED",
			Answer:    answer,
			Evidence:  []any{},
		}
		if report, ok := analyst.ParseReport(answer); ok {
			resp.Report = report
			resp.Confidence = report.Confidence
			if len(report.Evidence) > 0 {
				ev := make([]any, 0, len(report.Evidence))
				for _, item := range report.Evidence {
					ev = append(ev, item)
				}
				resp.Evidence = ev
			}
		}
		if run, getErr := h.runner.GetRun(ctx, id); getErr == nil && run != nil {
			resp.RunID = run.ID
			resp.Status = string(run.Status)
			resp.Confidence = run.Confidence
			if len(run.Evidence) > 0 {
				resp.Evidence = run.Evidence
			}
			resp.Steps = len(run.Steps)
			if resp.Steps == 0 && len(run.Decisions) > 0 {
				resp.Steps = len(run.Decisions)
			}
			if strings.TrimSpace(run.Answer) != "" {
				resp.Answer = run.Answer
			}
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	var waiting *guardrail.WaitingError
	if errors.As(err, &waiting) {
		writeJSON(w, http.StatusAccepted, runResponse{
			RequestID:  id,
			Status:     "WAITING_APPROVAL",
			RunID:      waiting.RunID,
			ApprovalID: waiting.ApprovalID,
			Tool:       waiting.Tool,
			Risk:       string(waiting.Risk),
			Evidence:   []any{},
		})
		return
	}
	if errors.Is(err, runtime.ErrEmptyMessage) {
		h.writeError(w, http.StatusBadRequest, id, err.Error())
		return
	}
	if errors.Is(err, guardrail.ErrNoCheckpoint) || errors.Is(err, guardrail.ErrNotFound) {
		h.writeError(w, http.StatusNotFound, id, err.Error())
		return
	}
	if errors.Is(err, guardrail.ErrExpired) || errors.Is(err, guardrail.ErrNotPending) {
		h.writeError(w, http.StatusConflict, id, err.Error())
		return
	}
	h.logger.ErrorContext(ctx, "agent run handler failed", "request_id", id, "error", err)
	status, msg := publicRunError(err)
	h.writeError(w, status, id, msg)
}

func (h *handler) writeApprovalError(w http.ResponseWriter, ctx context.Context, id string, err error) {
	switch {
	case errors.Is(err, guardrail.ErrNotFound):
		h.writeError(w, http.StatusNotFound, id, err.Error())
	case errors.Is(err, guardrail.ErrExpired), errors.Is(err, guardrail.ErrNotPending):
		h.writeError(w, http.StatusConflict, id, err.Error())
	default:
		h.logger.ErrorContext(ctx, "approval handler failed", "request_id", id, "error", err)
		h.writeError(w, http.StatusInternalServerError, id, "approval failed")
	}
}

func (h *handler) withRequestID(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" {
			id = requestid.New()
		}
		w.Header().Set("X-Request-ID", id)
		next(w, r.WithContext(requestid.With(r.Context(), id)))
	}
}

func (h *handler) getRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := requestid.From(ctx)
	runID := strings.TrimSpace(r.PathValue("id"))
	if runID == "" {
		h.writeError(w, http.StatusBadRequest, id, "run id is required")
		return
	}
	run, err := h.runner.GetRun(ctx, runID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, id, "agent run not found")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *handler) listAgents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"agents": h.runner.ListAgents()})
}

func (h *handler) listTools(w http.ResponseWriter, r *http.Request) {
	list := h.runner.ListTools()
	out := make([]map[string]any, 0, len(list))
	for _, t := range list {
		out = append(out, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.InputSchema,
			"risk":         t.Risk,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": out})
}

func (h *handler) listMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.runner.ListSemanticMetrics()
	out := make([]map[string]any, 0, len(metrics))
	for _, m := range metrics {
		item := map[string]any{
			"key":         m.Key,
			"name":        m.Name,
			"description": m.Description,
			"field":       m.Field,
			"aggregation": m.Aggregation,
			"aliases":     m.Aliases,
		}
		if m.Formula != nil && m.Formula.Expression != "" {
			item["formula"] = m.Formula.Expression
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"metrics": out})
}

func (h *handler) listDimensions(w http.ResponseWriter, r *http.Request) {
	dims := h.runner.ListSemanticDimensions()
	out := make([]map[string]any, 0, len(dims))
	for _, d := range dims {
		out = append(out, map[string]any{
			"key":         d.Key,
			"name":        d.Name,
			"description": d.Description,
			"field":       d.Field,
			"type":        d.Type,
			"entity":      d.Entity,
			"aliases":     d.Aliases,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"dimensions": out})
}

func (h *handler) writeError(w http.ResponseWriter, status int, id, msg string) {
	writeJSON(w, status, errorResponse{RequestID: id, Error: msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
