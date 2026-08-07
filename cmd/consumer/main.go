package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/forge-platform/forge/internal/platform/config"
	"github.com/forge-platform/forge/internal/platform/events"
	"github.com/forge-platform/forge/internal/platform/log"
)

// A minimal example consumer that logs every relayed event. It demonstrates the
// end-to-end path and is idempotent-friendly (handlers should tolerate
// at-least-once redelivery).
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
	logger := log.New(cfg.Env, "forge-consumer")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	nc, js, err := events.Connect("forge-consumer", cfg.Events.NATSURL)
	if err != nil {
		return err
	}
	defer nc.Drain()

	if err := events.EnsureStream(ctx, js, cfg.Events.StreamName, cfg.Events.SubjectPrefix); err != nil {
		return err
	}

	cons, err := js.CreateOrUpdateConsumer(ctx, cfg.Events.StreamName, jetstream.ConsumerConfig{
		Durable:       "forge-example-consumer",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: cfg.Events.SubjectPrefix + ".>",
		AckWait:       30 * time.Second,
	})
	if err != nil {
		return err
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		logger.Info("event received", "subject", msg.Subject(), "payload", string(msg.Data()))
		if err := msg.Ack(); err != nil {
			logger.Error("ack failed", "error", err.Error())
		}
	})
	if err != nil {
		return err
	}
	defer cc.Stop()

	logger.Info("consumer started; waiting for events", "stream", cfg.Events.StreamName)
	<-ctx.Done()
	logger.Info("consumer shutdown complete")
	return nil
}
