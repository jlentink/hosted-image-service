package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jlentink/image-service/pkg/jwt"
)

const (
	testSecret      = "test-secret-for-middleware"
	testDomain      = "example.com"
	testDomainAlt   = "other.com"
)

var testAllowedDomains = []string{testDomain}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

// makeRequest is a helper that builds a request with the Authorization header set.
// It also sets X-Site-Domain to domain (pass "" to omit the header).
func makeRequest(t *testing.T, token, siteDomain string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if siteDomain != "" {
		req.Header.Set("X-Site-Domain", siteDomain)
	}
	return req
}

// validToken generates a token for testDomain that is not expired.
func validToken(t *testing.T) string {
	t.Helper()
	token, err := jwt.GenerateToken(testSecret, testDomain, 5*time.Minute)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return token
}

// --- Happy path ---

func TestAuth_ValidToken(t *testing.T) {
	token := validToken(t)
	handler := Auth(testSecret, testAllowedDomains)(okHandler())
	req := makeRequest(t, token, testDomain)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_CaseInsensitiveBearer(t *testing.T) {
	token := validToken(t)
	handler := Auth(testSecret, testAllowedDomains)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "BEARER "+token)
	req.Header.Set("X-Site-Domain", testDomain)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for case-insensitive Bearer, got %d", w.Code)
	}
}

// X-Site-Domain may arrive with a scheme prefix; normalization must handle it.
func TestAuth_SiteDomainWithScheme(t *testing.T) {
	token := validToken(t)
	handler := Auth(testSecret, testAllowedDomains)(okHandler())
	req := makeRequest(t, token, "https://example.com/")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for https:// prefixed X-Site-Domain, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Token / header failures ---

func TestAuth_MissingAuthHeader(t *testing.T) {
	handler := Auth(testSecret, testAllowedDomains)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_InvalidFormat(t *testing.T) {
	handler := Auth(testSecret, testAllowedDomains)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic abc123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_EmptyToken(t *testing.T) {
	handler := Auth(testSecret, testAllowedDomains)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	token, _ := jwt.GenerateToken(testSecret, testDomain, -1*time.Minute)
	handler := Auth(testSecret, testAllowedDomains)(okHandler())
	req := makeRequest(t, token, testDomain)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_WrongSecret(t *testing.T) {
	token, _ := jwt.GenerateToken("different-secret", testDomain, 5*time.Minute)
	handler := Auth(testSecret, testAllowedDomains)(okHandler())
	req := makeRequest(t, token, testDomain)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// --- Domain validation failures ---

// Token issued for a domain not in the allowed list is rejected.
func TestAuth_DomainNotInAllowedList(t *testing.T) {
	token, _ := jwt.GenerateToken(testSecret, testDomainAlt, 5*time.Minute)
	handler := Auth(testSecret, testAllowedDomains)(okHandler()) // only testDomain is allowed
	req := makeRequest(t, token, testDomainAlt)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unlisted domain, got %d", w.Code)
	}
}

// Missing X-Site-Domain header is rejected even with a valid, allowed token.
func TestAuth_MissingSiteDomainHeader(t *testing.T) {
	token := validToken(t)
	handler := Auth(testSecret, testAllowedDomains)(okHandler())
	req := makeRequest(t, token, "") // no X-Site-Domain
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing X-Site-Domain, got %d", w.Code)
	}
}

// X-Site-Domain set to a different domain than the token claim is rejected.
func TestAuth_SiteDomainMismatch(t *testing.T) {
	token := validToken(t) // token domain = testDomain
	handler := Auth(testSecret, []string{testDomain, testDomainAlt})(okHandler())
	req := makeRequest(t, token, testDomainAlt) // header domain = testDomainAlt
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for domain mismatch, got %d", w.Code)
	}
}

// --- normalizeDomain unit tests ---

func TestNormalizeDomain(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"example.com", "example.com"},
		{"Example.COM", "example.com"},
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"https://example.com/", "example.com"},
		{"https://Example.com/", "example.com"},
		{"  example.com  ", "example.com"},
		{"", ""},
	}
	for _, tc := range cases {
		got := normalizeDomain(tc.input)
		if got != tc.want {
			t.Errorf("normalizeDomain(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
