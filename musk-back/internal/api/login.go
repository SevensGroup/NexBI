package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"autonomous-agent/internal/requestid"
)

const (
	demoAccount  = "demo"
	demoPassword = "musk888"
)

type loginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type loginResponse struct {
	OK bool `json:"ok"`
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := requestid.From(ctx)

	var req loginRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	if err := dec.Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, id, "invalid json body")
		return
	}

	account := strings.TrimSpace(req.Account)
	ok := account == demoAccount && req.Password == demoPassword
	ip := clientIP(r)
	if h.logins != nil {
		if _, err := h.logins.Record(ctx, account, ok, ip, r.UserAgent()); err != nil {
			h.logger.ErrorContext(ctx, "login log write failed", "request_id", id, "account", account, "ip", ip, "error", err)
			h.writeError(w, http.StatusInternalServerError, id, "could not record sign-in")
			return
		}
	}
	h.logger.InfoContext(ctx, "user login", "request_id", id, "account", account, "success", ok, "ip", ip)
	if !ok {
		h.writeError(w, http.StatusUnauthorized, id, "Incorrect account or password")
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{OK: true})
}

func clientIP(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP"} {
		if ip := strings.TrimSpace(r.Header.Get(header)); ip != "" {
			return ip
		}
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if ip := strings.TrimSpace(strings.Split(fwd, ",")[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
