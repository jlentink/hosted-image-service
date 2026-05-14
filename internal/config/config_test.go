package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	content := `
[auth]
jwt_secret = "test-secret"
allowed_domains = ["example.com"]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Image.DefaultQualityJPEG != 85 {
		t.Errorf("expected default jpeg quality 85, got %d", cfg.Image.DefaultQualityJPEG)
	}
	if cfg.Auth.JWTSecret != "test-secret" {
		t.Errorf("expected jwt_secret 'test-secret', got '%s'", cfg.Auth.JWTSecret)
	}
	if len(cfg.Auth.AllowedDomains) != 1 || cfg.Auth.AllowedDomains[0] != "example.com" {
		t.Errorf("expected allowed_domains [example.com], got %v", cfg.Auth.AllowedDomains)
	}
}

func TestLoadMissingJWTSecret(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	content := `
[server]
port = 9090
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for missing jwt_secret")
	}
}

func TestLoadMissingAllowedDomains(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// jwt_secret present but no allowed_domains.
	content := "[auth]\njwt_secret = \"secret\"\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for missing allowed_domains")
	}
}

func TestLoadFullConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	content := `
[server]
port = 9090
host = "127.0.0.1"
max_upload_size = "100MB"
read_timeout = "10s"
write_timeout = "30s"

[auth]
jwt_secret = "my-secret"
jwt_expiry = "10m"
allowed_domains = ["site-a.com", "site-b.com"]

[whitelist]
enabled = true
ips = ["10.0.0.1", "192.168.0.0/16"]

[image]
max_width = 2048
max_height = 2048
default_quality_jpeg = 90
default_quality_webp = 85
allowed_formats = ["jpeg", "webp"]
max_fetch_size = "10MB"
allowed_fetch_domains = ["example.com"]

[logging]
level = "debug"
file = "/var/log/image-service.log"

[metrics]
enabled = false
path = "/custom-metrics"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Whitelist.Enabled != true {
		t.Error("expected whitelist enabled")
	}
	if len(cfg.Whitelist.IPs) != 2 {
		t.Errorf("expected 2 whitelist IPs, got %d", len(cfg.Whitelist.IPs))
	}
	if cfg.Image.MaxWidth != 2048 {
		t.Errorf("expected max_width 2048, got %d", cfg.Image.MaxWidth)
	}
	if len(cfg.Image.AllowedFormats) != 2 {
		t.Errorf("expected 2 allowed formats, got %d", len(cfg.Image.AllowedFormats))
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}
	if cfg.Metrics.Enabled != false {
		t.Error("expected metrics disabled")
	}
	if len(cfg.Auth.AllowedDomains) != 2 {
		t.Errorf("expected 2 allowed domains, got %d", len(cfg.Auth.AllowedDomains))
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"50MB", 50 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"100KB", 100 * 1024, false},
		{"512B", 512, false},
		{"1024", 1024, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		result, err := ParseSize(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("ParseSize(%q): expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSize(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("ParseSize(%q): expected %d, got %d", tt.input, tt.expected, result)
		}
	}
}

// -- Edge cases ---------------------------------------------------------------

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/non/existent/path/config.toml")
	if err == nil {
		t.Fatal("expected error for non-existent config file")
	}
}

func TestLoadMalformedTOML(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "bad.toml")

	if err := os.WriteFile(cfgPath, []byte("this is not [ valid toml = [[["), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for malformed TOML")
	}
}

func TestLoadEnvVarOverride(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	content := "[auth]\njwt_secret = \"base-secret\"\nallowed_domains = [\"example.com\"]\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("IMAGE_SERVICE_SERVER_PORT", "7777")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 7777 {
		t.Errorf("expected port 7777 from env var, got %d", cfg.Server.Port)
	}
}

func TestLoadEnvVarJWTSecret(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// No jwt_secret in file -- must be satisfied by env var.
	content := "[server]\nport = 8080\n[auth]\nallowed_domains = [\"example.com\"]\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("IMAGE_SERVICE_AUTH_JWT_SECRET", "env-supplied-secret")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Auth.JWTSecret != "env-supplied-secret" {
		t.Errorf("expected jwt_secret from env var, got %q", cfg.Auth.JWTSecret)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"port zero", 0},
		{"port negative", -1},
		{"port too high", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "config.toml")

			content := fmt.Sprintf(
				"[server]\nport = %d\n[auth]\njwt_secret = \"secret\"\nallowed_domains = [\"example.com\"]\n",
				tt.port,
			)
			if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(cfgPath)
			if err == nil {
				t.Errorf("expected validation error for port %d", tt.port)
			}
		})
	}
}

func TestValidate_InvalidDimensions(t *testing.T) {
	tests := []struct {
		name      string
		maxWidth  int
		maxHeight int
	}{
		{"zero max_width", 0, 4096},
		{"negative max_width", -1, 4096},
		{"zero max_height", 4096, 0},
		{"negative max_height", 4096, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfgPath := filepath.Join(tmpDir, "config.toml")

			content := fmt.Sprintf(
				"[auth]\njwt_secret = \"secret\"\nallowed_domains = [\"example.com\"]\n[image]\nmax_width = %d\nmax_height = %d\n",
				tt.maxWidth, tt.maxHeight,
			)
			if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(cfgPath)
			if err == nil {
				t.Errorf("expected validation error for dimensions %dx%d", tt.maxWidth, tt.maxHeight)
			}
		})
	}
}
