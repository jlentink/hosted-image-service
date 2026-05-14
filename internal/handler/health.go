package handler

import (
	"encoding/json"
	"net/http"
)

// ReadinessChecker is an interface for checking if the server is ready.
type ReadinessChecker interface {
	IsReady() bool
}

// HealthHandler serves health and readiness endpoints.
type HealthHandler struct {
	checker ReadinessChecker
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(checker ReadinessChecker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

type healthResponse struct {
	Status string `json:"status"`
}

// Health is a liveness probe — returns 200 if the process is running.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// Ready is a readiness probe — returns 200 only when govips is initialized
// and the server is ready to process requests.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if !h.checker.IsReady() {
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "not ready"})
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ready"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
