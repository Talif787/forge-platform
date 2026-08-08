package main

import (
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/forge-platform/forge/internal/platform/config"
	"github.com/forge-platform/forge/internal/platform/log"
	"github.com/forge-platform/forge/internal/provisioning"
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
	logger := log.New(cfg.Env, "forge-worker")

	c, err := client.Dial(client.Options{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
	})
	if err != nil {
		return err
	}
	defer c.Close()

	w := worker.New(c, cfg.Temporal.TaskQueue, worker.Options{})
	w.RegisterWorkflow(provisioning.ProvisionTenantWorkflow)
	w.RegisterActivity(provisioning.NewActivities(provisioning.NewSimulatedProvisioner(logger)))

	logger.Info("worker started", "taskQueue", cfg.Temporal.TaskQueue)
	return w.Run(worker.InterruptCh())
}
