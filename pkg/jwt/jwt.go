package jwt

import (
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims used by the image service.
type Claims struct {
	// Domain is the originating site domain this token was issued for (e.g. "example.com").
	// It is required and validated against the server's allowed-domains list and the
	// X-Site-Domain request header on every authenticated request.
	Domain string `json:"domain"`
	gojwt.RegisteredClaims
}

// GenerateToken creates a signed JWT token with the given secret, domain, and expiry duration.
// Both secret and domain must be non-empty.
func GenerateToken(secret, domain string, expiry time.Duration) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret must not be empty")
	}
	if domain == "" {
		return "", fmt.Errorf("domain must not be empty")
	}

	now := time.Now()
	claims := &Claims{
		Domain: domain,
		RegisteredClaims: gojwt.RegisteredClaims{
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(now.Add(expiry)),
			Issuer:    "image-service",
		},
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken parses and validates a JWT token string against the given secret.
// Returns the claims if valid, or an error if invalid/expired.
func ValidateToken(tokenString, secret string) (*Claims, error) {
	if secret == "" {
		return nil, fmt.Errorf("secret must not be empty")
	}

	token, err := gojwt.ParseWithClaims(tokenString, &Claims{}, func(token *gojwt.Token) (any, error) {
		if _, ok := token.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
