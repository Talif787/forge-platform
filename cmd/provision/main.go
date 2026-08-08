package main

import (
	"context"
	"flag"
	"os"

	"go.temporal.io/sdk/client"

	"github.com/forge-platform/forge/internal/platform/config"
	"github.com/forge-platform/forge/internal/platform/log"
	"github.com/forge-platform/forge/internal/provisioning"
)

// A small starter that kicks off a tenant provisioning workflow and waits for
// the result. In a full system an event consumer would start this on the
// tenant.created event; here it is a CLI so the workflow is easy to demo.
func main() {
	slug := flag.String("slug", "", "tenant slug to provision")
	maxServices := flag.Int("max-services", 10, "service quota to apply")
	flag.Parse()

	if err := run(*slug, *maxServices); err != nil {
		os.Stderr.WriteString("fatal: " + err.Error() + "\n")
		os.Exit(1)
	}
}

func run(slug string, maxServices int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := log.New(cfg.Env, "forge-provision")
	if slug == "" {
		logger.Error("a -slug is required")
		os.Exit(2)
	}

	c, err := client.Dial(client.Options{HostPort: cfg.Temporal.HostPort, Namespace: cfg.Temporal.Namespace})
	if err != nil {
		return err
	}
	defer c.Close()

	ctx := context.Background()
	we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "provision-tenant-" + slug,
		TaskQueue: cfg.Temporal.TaskQueue,
	}, provisioning.ProvisionTenantWorkflow, provisioning.ProvisionTenantInput{Slug: slug, MaxServices: maxServices})
	if err != nil {
		return err
	}
	logger.Info("workflow started", "workflowID", we.GetID(), "runID", we.GetRunID())

	var result provisioning.ProvisionTenantResult
	if err := we.Get(ctx, &result); err != nil {
		return err
	}
	logger.Info("tenant provisioned", "namespace", result.Namespace, "endpoint", result.Endpoint)
	return nil
}
