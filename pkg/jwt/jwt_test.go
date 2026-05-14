package jwt

import (
	"testing"
	"time"
)

const testDomain = "example.com"

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret-key"
	tokenStr, err := GenerateToken(secret, testDomain, 5*time.Minute)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("token is empty")
	}

	claims, err := ValidateToken(tokenStr, secret)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.Issuer != "image-service" {
		t.Errorf("expected issuer 'image-service', got %q", claims.Issuer)
	}
	if claims.Domain != testDomain {
		t.Errorf("expected domain %q, got %q", testDomain, claims.Domain)
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	tokenStr, _ := GenerateToken("secret-a", testDomain, 5*time.Minute)
	_, err := ValidateToken(tokenStr, "secret-b")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	tokenStr, _ := GenerateToken("secret", testDomain, -1*time.Minute) // Already expired.
	_, err := ValidateToken(tokenStr, "secret")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestGenerateToken_EmptySecret(t *testing.T) {
	_, err := GenerateToken("", testDomain, 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestGenerateToken_EmptyDomain(t *testing.T) {
	_, err := GenerateToken("secret", "", 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for empty domain")
	}
}

func TestValidateToken_EmptySecret(t *testing.T) {
	tokenStr, _ := GenerateToken("secret", testDomain, 5*time.Minute)
	_, err := ValidateToken(tokenStr, "")
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestValidateToken_InvalidString(t *testing.T) {
	_, err := ValidateToken("not-a-valid-jwt", "secret")
	if err == nil {
		t.Fatal("expected error for invalid token string")
	}
}
