package image

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchImage(t *testing.T) {
	// Create a test server that serves a small image-like response.
	imageData := []byte("fake-image-data-for-testing")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/image.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(imageData)
		case "/large":
			w.Header().Set("Content-Type", "image/jpeg")
			// Write more than the max size.
			for i := 0; i < 100; i++ {
				w.Write(imageData)
			}
		case "/notimage":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html></html>"))
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	testCfg := DefaultFetchConfig()
	testCfg.AllowPrivateAddrs = true

	t.Run("successful fetch", func(t *testing.T) {
		data, err := FetchImage(server.URL+"/image.jpg", testCfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != string(imageData) {
			t.Errorf("data mismatch: got %q", string(data))
		}
	})

	t.Run("size limit exceeded", func(t *testing.T) {
		cfg := &FetchConfig{
			MaxSize:           10,
			Timeout:           testCfg.Timeout,
			AllowPrivateAddrs: true,
		}
		_, err := FetchImage(server.URL+"/image.jpg", cfg)
		if err == nil {
			t.Fatal("expected error for oversized response")
		}
	})

	t.Run("not an image content type", func(t *testing.T) {
		_, err := FetchImage(server.URL+"/notimage", testCfg)
		if err == nil {
			t.Fatal("expected error for non-image content type")
		}
	})

	t.Run("server error", func(t *testing.T) {
		_, err := FetchImage(server.URL+"/error", testCfg)
		if err == nil {
			t.Fatal("expected error for server error")
		}
	})

	t.Run("invalid URL scheme", func(t *testing.T) {
		_, err := FetchImage("ftp://example.com/image.jpg", DefaultFetchConfig())
		if err == nil {
			t.Fatal("expected error for ftp scheme")
		}
	})

	t.Run("domain whitelist - blocked", func(t *testing.T) {
		cfg := &FetchConfig{
			MaxSize:        DefaultFetchConfig().MaxSize,
			Timeout:        DefaultFetchConfig().Timeout,
			AllowedDomains: []string{"allowed.com"},
		}
		_, err := FetchImage(server.URL+"/image.jpg", cfg)
		if err == nil {
			t.Fatal("expected error for blocked domain")
		}
	})
}

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		hostname string
		allowed  []string
		wantErr  bool
	}{
		{"example.com", nil, false},                        // No whitelist
		{"example.com", []string{}, false},                 // Empty whitelist
		{"example.com", []string{"example.com"}, false},    // Exact match
		{"EXAMPLE.COM", []string{"example.com"}, false},    // Case insensitive
		{"other.com", []string{"example.com"}, true},       // Not in list
		{"example.com", []string{"a.com", "example.com"}, false}, // In list
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			err := validateDomain(tt.hostname, tt.allowed)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsImageContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"image/jpeg", true},
		{"image/png", true},
		{"image/webp", true},
		{"image/avif", true},
		{"application/octet-stream", true},
		{"text/html", false},
		{"application/json", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isImageContentType(tt.ct)
		if got != tt.want {
			t.Errorf("isImageContentType(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}
