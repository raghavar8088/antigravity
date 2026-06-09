package executiongateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"antigravity-engine/internal/security"
)

// Handler serves POST /api/execution/request — the only human-facing execution API.
type Handler struct {
	exec Executor
	audit func(event string, req Request, resp Response, principal security.Principal)
}

func NewHandler(exec Executor) *Handler {
	return &Handler{exec: exec}
}

func (h *Handler) SetAuditFn(fn func(string, Request, Response, security.Principal)) {
	h.audit = fn
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"POST only"}`, http.StatusMethodNotAllowed)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{OK: false, Status: "REJECTED", Message: "invalid JSON"})
		return
	}
	req.Venue = strings.ToLower(strings.TrimSpace(req.Venue))
	req.Symbol = strings.TrimSpace(req.Symbol)
	req.Side = strings.ToUpper(strings.TrimSpace(req.Side))
	if req.StrategyName == "" {
		req.StrategyName = "MANUAL_REQUEST"
	}
	if req.RequestID == "" {
		req.RequestID = "REQ-" + time.Now().UTC().Format("20060102150405.000")
	}

	principal, _ := security.PrincipalFrom(r.Context())
	if principal.UserID == "" && principal.ServiceName == "" {
		writeJSON(w, http.StatusUnauthorized, Response{OK: false, Status: "REJECTED", Message: "authentication required"})
		return
	}
	// Human principals may submit requests only — engine service identity proxies authenticated users.
	if !principal.IsService && !principal.HasPermission(security.PermTradeRequest) {
		writeJSON(w, http.StatusForbidden, Response{OK: false, Status: "REJECTED", Message: "trade.request permission required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	resp, err := h.exec.ProcessExecutionRequest(ctx, req)
	if err != nil && resp.Message == "" {
		resp = Response{OK: false, Status: "REJECTED", RequestID: req.RequestID, Message: err.Error()}
	}
	resp.ProcessedAt = time.Now().UTC()
	if h.audit != nil {
		h.audit("execution_request", req, resp, principal)
	}
	if !resp.OK {
		log.Printf("[EXEC GATEWAY] REJECTED %s venue=%s symbol=%s: %s", req.RequestID, req.Venue, req.Symbol, resp.Message)
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}
	log.Printf("[EXEC GATEWAY] ACCEPTED %s venue=%s symbol=%s order=%s", req.RequestID, req.Venue, req.Symbol, resp.ClientOrderID)
	writeJSON(w, http.StatusAccepted, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
