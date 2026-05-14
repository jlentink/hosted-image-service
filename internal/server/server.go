package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/jlentink/yaglogger"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jlentink/image-service/internal/config"
	"github.com/jlentink/image-service/internal/handler"
	img "github.com/jlentink/image-service/internal/image"
	"github.com/jlentink/image-service/internal/middleware"
)

// Server wraps the HTTP server and its dependencies.
type Server struct {
	cfg        *config.Config
	httpServer *http.Server
	processor  *img.Processor
	ready      bool
}

// New creates a new Server instance.
func New(cfg *config.Config) *Server {
	return &Server{
		cfg: cfg,
	}
}

// Run starts the server and blocks until shutdown.
func (s *Server) Run() error {
	// Initialize govips.
	if err := img.Startup(); err != nil {
		return fmt.Errorf("initializing image processor: %w", err)
	}
	defer img.Shutdown()

	// Create image processor from config.
	s.processor = img.NewProcessor(
		s.cfg.Image.MaxWidth,
		s.cfg.Image.MaxHeight,
		s.cfg.Image.DefaultQualityJPEG,
		s.cfg.Image.DefaultQualityWebP,
		s.cfg.Image.DefaultQualityAVIF,
		s.cfg.Image.DefaultQualityPNG,
	)

	maxUploadBytes, err := config.ParseSize(s.cfg.Server.MaxUploadSize)
	if err != nil {
		return fmt.Errorf("parsing max_upload_size: %w", err)
	}

	maxFetchBytes, err := config.ParseSize(s.cfg.Image.MaxFetchSize)
	if err != nil {
		return fmt.Errorf("parsing max_fetch_size: %w", err)
	}

	fetchCfg := &img.FetchConfig{
		MaxSize:        maxFetchBytes,
		AllowedDomains: s.cfg.Image.AllowedFetchDomains,
		Timeout:        s.cfg.Server.ReadTimeout,
	}

	// Build handler dependencies.
	resizeHandler := handler.NewResizeHandler(s.processor, s.cfg, maxUploadBytes, fetchCfg)
	healthHandler := handler.NewHealthHandler(s)

	// Build middleware chain.
	authMiddleware := middleware.Auth(s.cfg.Auth.JWTSecret, s.cfg.Auth.AllowedDomains)
	log.Info("JWT domain validation enabled for %d domain(s)", len(s.cfg.Auth.AllowedDomains))

	var whitelistMiddleware func(http.Handler) http.Handler
	if s.cfg.Whitelist.Enabled {
		whitelistMiddleware = middleware.IPWhitelist(s.cfg.Whitelist.IPs)
		log.Info("IP whitelist enabled with %d entries", len(s.cfg.Whitelist.IPs))
	}

	// Register routes.
	mux := http.NewServeMux()

	// Health endpoints are public (no auth/whitelist).
	mux.HandleFunc("GET /health", healthHandler.Health)
	mux.HandleFunc("GET /ready", healthHandler.Ready)

	// Prometheus metrics endpoint (public).
	if s.cfg.Metrics.Enabled {
		mux.Handle("GET "+s.cfg.Metrics.Path, promhttp.Handler())
		log.Info("Prometheus metrics enabled at %s", s.cfg.Metrics.Path)
	}

	// Resize endpoint is protected by auth (and optionally whitelist).
	var resizeChain http.Handler = http.HandlerFunc(resizeHandler.Handle)
	resizeChain = authMiddleware(resizeChain)
	if whitelistMiddleware != nil {
		resizeChain = whitelistMiddleware(resizeChain)
	}
	mux.Handle("POST /resize", resizeChain)

	// Wrap entire mux with logging and metrics middleware.
	var rootHandler http.Handler = mux
	rootHandler = middleware.Metrics()(rootHandler)
	rootHandler = middleware.Logging()(rootHandler)

	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      rootHandler,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
	}

	// Signal handling for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Mark ready.
	s.ready = true

	errCh := make(chan error, 1)
	go func() {
		log.Info("Listening on %s", addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Info("Shutdown signal received, draining connections...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.Server.WriteTimeout)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		log.Info("Server stopped gracefully")
		return nil
	case err := <-errCh:
		return err
	}
}

// IsReady returns whether the server is ready to serve requests.
func (s *Server) IsReady() bool {
	return s.ready
}
