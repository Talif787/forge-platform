package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/forge-platform/forge/internal/platform/config"
	"github.com/forge-platform/forge/internal/platform/events"
	"github.com/forge-platform/forge/internal/platform/log"
	pg "github.com/forge-platform/forge/internal/platform/postgres"
	"github.com/forge-platform/forge/internal/relay"
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
	logger := log.New(cfg.Env, "forge-relay")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pg.New(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	nc, js, err := events.Connect("forge-relay", cfg.Events.NATSURL)
	if err != nil {
		return err
	}
	defer nc.Drain()

	if err := events.EnsureStream(ctx, js, cfg.Events.StreamName, cfg.Events.SubjectPrefix); err != nil {
		return err
	}

	store := relay.NewPgStore(pool.Pool)
	pub := events.NewJetStreamPublisher(js)
	r := relay.New(store, pub, cfg.Events.SubjectPrefix, cfg.Relay.BatchSize, cfg.Relay.PollInterval, logger)

	if err := r.Run(ctx); err != nil && err != context.Canceled {
		return err
	}
	logger.Info("relay shutdown complete")
	return nil
}
