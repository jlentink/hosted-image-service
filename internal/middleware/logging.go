package middleware

import (
	"net/http"
	"time"

	log "github.com/jlentink/yaglogger"
)

// Logging returns middleware that logs each HTTP request with yaglogger.
func Logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			sr := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(sr, r)

			duration := time.Since(start)
			clientIP := extractClientIP(r)

			log.Info("%s %s %d %s [%s]", r.Method, r.URL.Path, sr.statusCode, duration, clientIP)
		})
	}
}
