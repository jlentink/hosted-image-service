package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "image_service_http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "image_service_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "path"})

	imageProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "image_service_processing_duration_seconds",
		Help:    "Image processing duration in seconds.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	})

	imageOutputBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "image_service_output_bytes",
		Help:    "Output image size in bytes.",
		Buckets: []float64{1024, 10240, 102400, 524288, 1048576, 5242880, 10485760},
	})

	imageRequestsByFormat = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "image_service_requests_by_format_total",
		Help: "Image requests by output format.",
	}, []string{"format"})

	imageErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "image_service_errors_total",
		Help: "Total number of image processing errors.",
	})
)

// RecordProcessingDuration records the image processing duration.
func RecordProcessingDuration(d time.Duration) {
	imageProcessingDuration.Observe(d.Seconds())
}

// RecordOutputBytes records the output image size.
func RecordOutputBytes(size int) {
	imageOutputBytes.Observe(float64(size))
}

// RecordFormat increments the request counter for the given format.
func RecordFormat(format string) {
	imageRequestsByFormat.WithLabelValues(format).Inc()
}

// RecordError increments the error counter.
func RecordError() {
	imageErrors.Inc()
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// Metrics returns middleware that records Prometheus metrics for each request.
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			sr := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(sr, r)

			duration := time.Since(start)
			status := strconv.Itoa(sr.statusCode)
			path := normalizePath(r.URL.Path)

			httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
			httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration.Seconds())
		})
	}
}

// normalizePath reduces cardinality by mapping paths to known routes.
func normalizePath(path string) string {
	switch path {
	case "/resize", "/health", "/ready", "/metrics":
		return path
	default:
		return "/other"
	}
}
