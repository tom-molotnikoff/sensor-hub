package api

import (
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"example/sensorHub/api/middleware"
	appProps "example/sensorHub/application_properties"
	gen "example/sensorHub/gen"
	"example/sensorHub/telemetry"
	"example/sensorHub/web"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

//go:embed openapi.yaml
var openapiSpec []byte

// NewEngine returns the base router. trustedProxies is the comma-separated
// http.trusted.proxies value: only a request from one of those addresses has
// its X-Forwarded-For and X-Real-IP believed, and c.ClientIP() is then the
// rightmost address in the chain that is not a trusted proxy. Empty trusts no
// proxy, so c.ClientIP() is always the connecting peer.
func NewEngine(trustedProxies string) (*gin.Engine, error) {
	router := gin.New()
	router.RedirectTrailingSlash = false
	router.UseRawPath = true
	if err := router.SetTrustedProxies(splitList(trustedProxies)); err != nil {
		return nil, fmt.Errorf("invalid http.trusted.proxies: %w", err)
	}
	router.Use(gin.Recovery())
	return router, nil
}

func splitList(s string) []string {
	var items []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func RegisterAPIRoutes(router *gin.Engine, server *Server) {
	apiGroup := router.Group("/api")
	apiGroup.Use(middleware.Compression())
	apiGroup.Use(middleware.CSRFMiddleware())
	gen.RegisterHandlersWithOptions(apiGroup, server, gen.GinServerOptions{
		Middlewares: []gen.MiddlewareFunc{RouteAuthAndPermissionMiddleware()},
	})
}

func newRouter(logger *slog.Logger, trustedProxies string, server *Server) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	router, err := NewEngine(trustedProxies)
	if err != nil {
		return nil, err
	}
	router.Use(otelgin.Middleware("sensor-hub"))
	router.Use(telemetry.GinLoggerMiddleware(logger))

	// CORS is only needed when the UI is served from a different origin (e.g. Vite dev server)
	allowedOrigin := os.Getenv("SENSOR_HUB_ALLOWED_ORIGIN")
	if allowedOrigin != "" {
		router.Use(cors.New(cors.Config{
			AllowOrigins:     []string{allowedOrigin},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "X-CSRF-Token", "X-API-Key"},
			ExposeHeaders:    []string{"Content-Length", "Retry-After"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	RegisterAPIRoutes(router, server)

	// Serve embedded Docusaurus docs at /docs (before SPA catch-all)
	web.RegisterDocsHandler(router)

	// Serve embedded UI for all non-API routes
	web.RegisterSPAHandler(router)

	return router, nil
}

// newMetricsServer serves the unauthenticated Prometheus endpoint on its own
// address, so it is never reachable through the API port or the proxy in
// front of it.
func newMetricsServer(addr string, prometheusHandler http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", prometheusHandler)
	return &http.Server{Addr: addr, Handler: mux}
}

// InitialiseAndListen serves the API on cfg.HTTPListenAddress and, unless
// cfg.MetricsListenAddress is empty, /metrics on that address, until ctx is
// done or either server fails.
func InitialiseAndListen(ctx context.Context, logger *slog.Logger, cfg *appProps.ApplicationConfiguration, prometheusHandler http.Handler, server *Server) error {
	logger.Info("API server starting")

	router, err := newRouter(logger, cfg.HTTPTrustedProxies, server)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:    cfg.HTTPListenAddress,
		Handler: router,
	}

	var metricsSrv *http.Server
	if cfg.MetricsListenAddress != "" {
		metricsSrv = newMetricsServer(cfg.MetricsListenAddress, prometheusHandler)
	}

	errCh := make(chan error, 2)

	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")
	useTLS := certFile != "" && keyFile != ""

	if useTLS {
		logger.Info("starting with TLS", "cert", certFile, "key", keyFile)
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificate: %w", err)
		}
		srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	}

	logger.Info("API server listening", "address", srv.Addr, "tls", useTLS)

	go func() {
		var err error
		if useTLS {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("API server error: %w", err)
		}
	}()

	if metricsSrv != nil {
		logger.Info("metrics server listening", "address", metricsSrv.Addr)
		go func() {
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("metrics server error: %w", err)
			}
		}()
	} else {
		logger.Info("metrics server disabled: metrics.listen.address is empty")
	}

	// Wait for shutdown signal or server error
	var serveErr error
	select {
	case serveErr = <-errCh:
	case <-ctx.Done():
		logger.Info("shutting down API server")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("metrics server forced to shutdown", "error", err)
		}
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("API server forced to shutdown: %w", err)
	}
	if serveErr != nil {
		return serveErr
	}
	logger.Info("API server stopped")
	return nil
}
