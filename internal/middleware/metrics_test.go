package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsMiddleware_RecordsRequest(t *testing.T) {
	handler := Metrics()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/resize", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Verify the counter was incremented.
	count := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("POST", "/resize", "200"))
	if count < 1 {
		t.Errorf("expected request counter >= 1, got %f", count)
	}
}

func TestMetricsMiddleware_NormalizesPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/resize", "/resize"},
		{"/health", "/health"},
		{"/ready", "/ready"},
		{"/metrics", "/metrics"},
		{"/unknown", "/other"},
		{"/foo/bar", "/other"},
	}

	for _, tt := range tests {
		got := normalizePath(tt.path)
		if got != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestRecordProcessingDuration(t *testing.T) {
	// Should not panic.
	RecordProcessingDuration(100 * time.Millisecond)
}

func TestRecordOutputBytes(t *testing.T) {
	// Should not panic.
	RecordOutputBytes(1024)
}

func TestRecordFormat(t *testing.T) {
	// Should not panic.
	RecordFormat("jpeg")
}

func TestRecordError(t *testing.T) {
	// Should not panic.
	RecordError()
}

func TestStatusRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

	sr.WriteHeader(http.StatusNotFound)
	if sr.statusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", sr.statusCode)
	}
}
