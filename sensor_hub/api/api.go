package api

import (
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"example/sensorHub/api/middleware"
	gen "example/sensorHub/gen"
	"example/sensorHub/telemetry"
	"example/sensorHub/web"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

//go:embed openapi.yaml
var openapiSpec []byte

func InitialiseAndListen(ctx context.Context, logger *slog.Logger, prometheusHandler http.Handler, server *Server) error {
	logger.Info("API server starting")

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.RedirectTrailingSlash = false
	router.Use(gin.Recovery())
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

	// All API routes live under /api
	apiGroup := router.Group("/api")

	apiGroup.Use(middleware.CSRFMiddleware())

	gen.RegisterHandlersWithOptions(apiGroup, server, gen.GinServerOptions{
		Middlewares: []gen.MiddlewareFunc{RouteAuthAndPermissionMiddleware()},
	})

	// Prometheus metrics endpoint (no auth)
	if prometheusHandler != nil {
		router.GET("/metrics", gin.WrapH(prometheusHandler))
	}

	// Serve embedded Docusaurus docs at /docs (before SPA catch-all)
	web.RegisterDocsHandler(router)

	// Serve embedded UI for all non-API routes
	web.RegisterSPAHandler(router)

	srv := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: router,
	}

	// Start serving in a goroutine
	errCh := make(chan error, 1)

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

	logger.Info("API server listening", "port", 8080, "tls", useTLS)

	go func() {
		var err error
		if useTLS {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-errCh:
		return fmt.Errorf("API server error: %w", err)
	case <-ctx.Done():
		logger.Info("shutting down API server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("API server forced to shutdown: %w", err)
		}
		logger.Info("API server stopped")
		return nil
	}
}
