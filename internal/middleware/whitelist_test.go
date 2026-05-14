package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIPWhitelist_EmptyList(t *testing.T) {
	handler := IPWhitelist(nil)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("empty whitelist should allow all, got %d", w.Code)
	}
}

func TestIPWhitelist_AllowedIP(t *testing.T) {
	handler := IPWhitelist([]string{"10.0.0.1"})(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for whitelisted IP, got %d", w.Code)
	}
}

func TestIPWhitelist_BlockedIP(t *testing.T) {
	handler := IPWhitelist([]string{"10.0.0.1"})(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-whitelisted IP, got %d", w.Code)
	}
}

func TestIPWhitelist_CIDR(t *testing.T) {
	handler := IPWhitelist([]string{"10.0.0.0/8"})(okHandler())

	tests := []struct {
		ip     string
		expect int
	}{
		{"10.0.0.1:80", http.StatusOK},
		{"10.255.255.255:80", http.StatusOK},
		{"11.0.0.1:80", http.StatusForbidden},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = tt.ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tt.expect {
			t.Errorf("IP %s: expected %d, got %d", tt.ip, tt.expect, w.Code)
		}
	}
}

func TestIPWhitelist_XForwardedFor(t *testing.T) {
	handler := IPWhitelist([]string{"10.0.0.5"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:80" // Proxy IP, not whitelisted.
	req.Header.Set("X-Forwarded-For", "10.0.0.5, 192.168.1.1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 via X-Forwarded-For, got %d", w.Code)
	}
}

func TestIPWhitelist_XRealIP(t *testing.T) {
	handler := IPWhitelist([]string{"10.0.0.5"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:80"
	req.Header.Set("X-Real-IP", "10.0.0.5")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 via X-Real-IP, got %d", w.Code)
	}
}

func TestIPWhitelist_MultipleCIDRs(t *testing.T) {
	handler := IPWhitelist([]string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})(okHandler())

	tests := []struct {
		ip     string
		expect int
	}{
		{"10.1.2.3:80", http.StatusOK},
		{"172.20.1.1:80", http.StatusOK},
		{"192.168.50.1:80", http.StatusOK},
		{"8.8.8.8:80", http.StatusForbidden},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = tt.ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tt.expect {
			t.Errorf("IP %s: expected %d, got %d", tt.ip, tt.expect, w.Code)
		}
	}
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		want       string
	}{
		{"RemoteAddr only", "1.2.3.4:8080", "", "", "1.2.3.4"},
		{"X-Forwarded-For", "1.2.3.4:8080", "10.0.0.1, 1.2.3.4", "", "10.0.0.1"},
		{"X-Real-IP", "1.2.3.4:8080", "", "10.0.0.2", "10.0.0.2"},
		{"XFF takes priority over XRI", "1.2.3.4:8080", "10.0.0.1", "10.0.0.2", "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}
			got := extractClientIP(req)
			if got != tt.want {
				t.Errorf("extractClientIP: got %q, want %q", got, tt.want)
			}
		})
	}
}
