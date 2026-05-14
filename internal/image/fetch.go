package image

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FetchConfig holds configuration for URL-based image fetching.
type FetchConfig struct {
	MaxSize           int64    // Maximum download size in bytes
	AllowedDomains    []string // If non-empty, only these domains are allowed
	Timeout           time.Duration
	AllowPrivateAddrs bool // For testing only: skip SSRF private address check
}

// DefaultFetchConfig returns sensible defaults for image fetching.
func DefaultFetchConfig() *FetchConfig {
	return &FetchConfig{
		MaxSize: 20 * 1024 * 1024, // 20MB
		Timeout: 30 * time.Second,
	}
}

// FetchImage downloads an image from a URL with size and domain restrictions.
func FetchImage(rawURL string, cfg *FetchConfig) ([]byte, error) {
	if cfg == nil {
		cfg = DefaultFetchConfig()
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("only http and https URLs are supported")
	}

	if err := validateDomain(parsed.Hostname(), cfg.AllowedDomains); err != nil {
		return nil, err
	}

	if !cfg.AllowPrivateAddrs {
		if err := validateNotPrivate(parsed.Hostname()); err != nil {
			return nil, err
		}
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			// Validate redirect target domain too.
			if err := validateDomain(req.URL.Hostname(), cfg.AllowedDomains); err != nil {
				return err
			}
			return nil
		},
	}

	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetching image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch returned status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !isImageContentType(contentType) {
		return nil, fmt.Errorf("URL does not point to an image (content-type: %s)", contentType)
	}

	// Read with size limit.
	limitedReader := io.LimitReader(resp.Body, cfg.MaxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if int64(len(data)) > cfg.MaxSize {
		return nil, fmt.Errorf("image exceeds maximum size of %d bytes", cfg.MaxSize)
	}

	return data, nil
}

func validateDomain(hostname string, allowedDomains []string) error {
	if len(allowedDomains) == 0 {
		return nil
	}
	for _, d := range allowedDomains {
		if strings.EqualFold(hostname, d) {
			return nil
		}
	}
	return fmt.Errorf("domain %q is not in the allowed domains list", hostname)
}

func validateNotPrivate(hostname string) error {
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil // Let the HTTP client handle resolution errors.
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("fetching from private/loopback addresses is not allowed")
		}
	}
	return nil
}

func isImageContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "image/") || strings.Contains(ct, "octet-stream")
}
