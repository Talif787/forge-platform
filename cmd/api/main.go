package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	catalog "github.com/forge-platform/forge/internal/modules/catalog"
	"github.com/forge-platform/forge/internal/platform/config"
	"github.com/forge-platform/forge/internal/platform/httpx"
	"github.com/forge-platform/forge/internal/platform/idempotency"
	"github.com/forge-platform/forge/internal/platform/log"
	"github.com/forge-platform/forge/internal/platform/observability"
	pg "github.com/forge-platform/forge/internal/platform/postgres"
	"github.com/forge-platform/forge/migrations"
)

func main() {
	if err := run(); err != nil {
		os.Stderr.WriteString("fatal: " + err.Error() + "\n")
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := log.New(cfg.Env, cfg.Telemetry.ServiceName)
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.InitTracing(rootCtx, cfg.Telemetry.ServiceName, cfg.Telemetry.OTLPEndpoint)
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(ctx)
	}()

	pool, err := pg.New(rootCtx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pg.Migrate(rootCtx, pool.Pool, migrations.FS); err != nil {
		return err
	}
	logger.Info("migrations applied")

	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	metrics := observability.NewMetrics(registry)

	verifier := buildVerifier(cfg.Auth)
	idemStore := idempotency.New(pool.Pool)

	router := chi.NewRouter()
	router.Use(httpx.Recoverer)
	router.Use(httpx.Correlate(logger))
	router.Use(httpx.Observe(metrics))
	router.Use(httpx.Timeout(30 * time.Second))

	router.Get("/healthz", httpx.Liveness())
	router.Get("/readyz", httpx.Readiness(func(ctx context.Context) error { return pool.Health(ctx) }))
	router.Method(http.MethodGet, "/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	router.Route("/api/v1", func(r chi.Router) {
		r.Use(httpx.Authenticate(verifier))
		r.Use(httpx.RateLimit(cfg.RateLimit.RPS, cfg.RateLimit.Burst))
		catalog.New(pool.Pool, idemStore).Mount(r)
	})

	server := httpx.NewServer(cfg.HTTPAddr, router)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", cfg.HTTPAddr)
		errCh <- server.Start()
	}()

	select {
	case err := <-errCh:
		return err
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("shutdown complete")
	return nil
}

func buildVerifier(cfg config.Auth) httpx.Verifier {
	if cfg.Mode == "jwks" {
		return httpx.NewJWKSVerifier(cfg.JWKSURL, cfg.Issuer, cfg.Audience)
	}
	return httpx.NewHMACVerifier(cfg.HMACSecret)
}
