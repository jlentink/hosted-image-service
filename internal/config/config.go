package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Whitelist WhitelistConfig `mapstructure:"whitelist"`
	Image     ImageConfig     `mapstructure:"image"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
}

type ServerConfig struct {
	Port          int           `mapstructure:"port"`
	Host          string        `mapstructure:"host"`
	MaxUploadSize string        `mapstructure:"max_upload_size"`
	ReadTimeout   time.Duration `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration `mapstructure:"write_timeout"`
}

type AuthConfig struct {
	JWTSecret      string        `mapstructure:"jwt_secret"`
	JWTExpiry      time.Duration `mapstructure:"jwt_expiry"`
	AllowedDomains []string      `mapstructure:"allowed_domains"`
}

type WhitelistConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	IPs     []string `mapstructure:"ips"`
	Domains []string `mapstructure:"domains"`
}

type ImageConfig struct {
	MaxWidth            int      `mapstructure:"max_width"`
	MaxHeight           int      `mapstructure:"max_height"`
	DefaultQualityJPEG  int      `mapstructure:"default_quality_jpeg"`
	DefaultQualityWebP  int      `mapstructure:"default_quality_webp"`
	DefaultQualityAVIF  int      `mapstructure:"default_quality_avif"`
	DefaultQualityPNG   int      `mapstructure:"default_quality_png"`
	AllowedFormats      []string `mapstructure:"allowed_formats"`
	MaxFetchSize        string   `mapstructure:"max_fetch_size"`
	AllowedFetchDomains []string `mapstructure:"allowed_fetch_domains"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.max_upload_size", "50MB")
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "60s")

	v.SetDefault("auth.jwt_secret", "")
	v.SetDefault("auth.jwt_expiry", "5m")
	v.SetDefault("auth.allowed_domains", []string{})

	v.SetDefault("whitelist.enabled", false)
	v.SetDefault("whitelist.ips", []string{})
	v.SetDefault("whitelist.domains", []string{})

	v.SetDefault("image.max_width", 4096)
	v.SetDefault("image.max_height", 4096)
	v.SetDefault("image.default_quality_jpeg", 85)
	v.SetDefault("image.default_quality_webp", 80)
	v.SetDefault("image.default_quality_avif", 60)
	v.SetDefault("image.default_quality_png", 6)
	v.SetDefault("image.allowed_formats", []string{"jpeg", "png", "webp", "avif"})
	v.SetDefault("image.max_fetch_size", "20MB")
	v.SetDefault("image.allowed_fetch_domains", []string{})

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.file", "")

	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.path", "/metrics")
}

func Load(configPath string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetConfigType("toml")

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("image-service")
		v.SetConfigType("toml")
		v.AddConfigPath(".")
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".config"))
		}
		v.AddConfigPath("/etc")
	}

	v.SetEnvPrefix("IMAGE_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwt_secret is required")
	}
	if len(cfg.Auth.AllowedDomains) == 0 {
		return fmt.Errorf("auth.allowed_domains must contain at least one domain")
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if cfg.Image.MaxWidth < 1 {
		return fmt.Errorf("image.max_width must be positive")
	}
	if cfg.Image.MaxHeight < 1 {
		return fmt.Errorf("image.max_height must be positive")
	}
	return nil
}

// ParseSize parses a human-readable size string (e.g., "50MB") into bytes.
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))

	// Check longest suffixes first to avoid "MB" matching "B".
	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, entry := range suffixes {
		if strings.HasSuffix(s, entry.suffix) {
			numStr := strings.TrimSuffix(s, entry.suffix)
			var num int64
			if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
				return 0, fmt.Errorf("invalid size: %s", s)
			}
			return num * entry.mult, nil
		}
	}

	var num int64
	if _, err := fmt.Sscanf(s, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid size: %s", s)
	}
	return num, nil
}
