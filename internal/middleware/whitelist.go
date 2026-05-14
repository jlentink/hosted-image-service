package middleware

import (
	"net"
	"net/http"
	"strings"

	log "github.com/jlentink/yaglogger"
)

// IPWhitelist returns middleware that restricts access to whitelisted IPs/CIDRs.
// If the whitelist is empty, all requests are allowed.
func IPWhitelist(ips []string) func(http.Handler) http.Handler {
	networks := parseNetworks(ips)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(networks) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := extractClientIP(r)
			if clientIP == "" {
				log.Warn("Could not determine client IP")
				forbidden(w, "could not determine client IP")
				return
			}

			ip := net.ParseIP(clientIP)
			if ip == nil {
				log.Warn("Invalid client IP: %s", clientIP)
				forbidden(w, "invalid client IP")
				return
			}

			for _, n := range networks {
				if n.Contains(ip) {
					next.ServeHTTP(w, r)
					return
				}
			}

			log.Debug("IP %s not in whitelist", clientIP)
			forbidden(w, "access denied")
		})
	}
}

// extractClientIP returns the client's IP address, checking X-Forwarded-For
// and X-Real-IP headers before falling back to RemoteAddr.
func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For (first IP is the client).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// Check X-Real-IP.
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func parseNetworks(ips []string) []*net.IPNet {
	var networks []*net.IPNet
	for _, s := range ips {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		// Try parsing as CIDR.
		if strings.Contains(s, "/") {
			_, network, err := net.ParseCIDR(s)
			if err != nil {
				log.Warn("Invalid CIDR in whitelist: %s", s)
				continue
			}
			networks = append(networks, network)
			continue
		}

		// Single IP — convert to /32 or /128.
		ip := net.ParseIP(s)
		if ip == nil {
			log.Warn("Invalid IP in whitelist: %s", s)
			continue
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		networks = append(networks, &net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(bits, bits),
		})
	}
	return networks
}

func forbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
