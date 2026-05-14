package middleware

import (
	"net/http"
	"strings"

	log "github.com/jlentink/yaglogger"

	"github.com/jlentink/image-service/pkg/jwt"
)

// Auth returns middleware that validates JWT Bearer tokens.
//
// After signature/expiry validation it enforces two domain checks:
//  1. The token's "domain" claim must be in allowedDomains.
//  2. The X-Site-Domain request header must match the token's domain claim.
//
// Both checks are required; a request fails if either is missing or mismatched.
func Auth(secret string, allowedDomains []string) func(http.Handler) http.Handler {
	// Build a lookup set for O(1) membership tests.
	domainSet := make(map[string]struct{}, len(allowedDomains))
	for _, d := range allowedDomains {
		domainSet[strings.ToLower(strings.TrimSpace(d))] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				unauthorised(w, "missing Authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				unauthorised(w, "Authorization header must be: Bearer <token>")
				return
			}

			tokenStr := strings.TrimSpace(parts[1])
			if tokenStr == "" {
				unauthorised(w, "empty token")
				return
			}

			claims, err := jwt.ValidateToken(tokenStr, secret)
			if err != nil {
				log.Debug("JWT validation failed: %s", err.Error())
				unauthorised(w, "invalid or expired token")
				return
			}

			// Domain claim must be present — tokens without it are rejected.
			if claims.Domain == "" {
				log.Debug("JWT missing domain claim")
				unauthorised(w, "invalid or expired token")
				return
			}

			tokenDomain := strings.ToLower(claims.Domain)

			// 1. Allowlist check: the token's domain must be a configured allowed domain.
			if _, ok := domainSet[tokenDomain]; !ok {
				log.Debug("JWT domain %q not in allowed list", tokenDomain)
				unauthorised(w, "invalid or expired token")
				return
			}

			// 2. Header check: X-Site-Domain must be present and match the token's domain.
			siteDomain := normalizeDomain(r.Header.Get("X-Site-Domain"))
			if siteDomain == "" {
				log.Debug("missing X-Site-Domain header")
				unauthorised(w, "missing X-Site-Domain header")
				return
			}
			if siteDomain != tokenDomain {
				log.Debug("X-Site-Domain %q does not match token domain %q", siteDomain, tokenDomain)
				unauthorised(w, "domain mismatch")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// normalizeDomain strips the scheme (http:// / https://) and trailing slashes
// from a domain string and lowercases the result so comparisons are consistent.
// For example, "https://Example.com/" → "example.com".
func normalizeDomain(raw string) string {
	d := strings.TrimSpace(raw)
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimRight(d, "/")
	return strings.ToLower(d)
}

func unauthorised(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
