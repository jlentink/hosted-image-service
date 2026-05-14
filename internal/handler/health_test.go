package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockReadinessChecker struct {
	ready bool
}

func (m *mockReadinessChecker) IsReady() bool { return m.ready }

func TestHealthEndpoint(t *testing.T) {
	h := NewHealthHandler(&mockReadinessChecker{ready: true})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp healthResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
}

func TestReadyEndpoint_Ready(t *testing.T) {
	h := NewHealthHandler(&mockReadinessChecker{ready: true})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	h.Ready(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp healthResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != "ready" {
		t.Errorf("expected status 'ready', got %q", resp.Status)
	}
}

func TestReadyEndpoint_NotReady(t *testing.T) {
	h := NewHealthHandler(&mockReadinessChecker{ready: false})

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	h.Ready(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var resp healthResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != "not ready" {
		t.Errorf("expected status 'not ready', got %q", resp.Status)
	}
}
