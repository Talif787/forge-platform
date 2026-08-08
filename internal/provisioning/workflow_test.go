package provisioning

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestProvisionTenant_Success(t *testing.T) {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()

	sim := NewSimulatedProvisioner(discardLogger())
	env.RegisterActivity(NewActivities(sim))

	env.ExecuteWorkflow(ProvisionTenantWorkflow, ProvisionTenantInput{Slug: "acme", MaxServices: 10})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result ProvisionTenantResult
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, "tenant-acme", result.Namespace)
	assert.Equal(t, "acme.forge.example.com", result.Endpoint)

	// Every resource should be provisioned and present.
	assert.True(t, sim.HasNamespace("acme"))
	assert.True(t, sim.HasQuota("acme"))
	assert.True(t, sim.HasDNS("acme"))
	assert.True(t, sim.HasSecret("acme"))
}

func TestProvisionTenant_CompensatesOnFailure(t *testing.T) {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()

	sim := NewSimulatedProvisioner(discardLogger())
	sim.FailRegisterDNS = true // step 3 fails after namespace and quota succeed
	env.RegisterActivity(NewActivities(sim))

	env.ExecuteWorkflow(ProvisionTenantWorkflow, ProvisionTenantInput{Slug: "beta", MaxServices: 5})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError(), "workflow must fail when a step cannot complete")

	// The completed steps must have been rolled back.
	assert.False(t, sim.HasNamespace("beta"), "namespace must be compensated")
	assert.False(t, sim.HasQuota("beta"), "quota must be compensated")
	// DNS never succeeded, so nothing to roll back there, and the secret step was
	// never reached.
	assert.False(t, sim.HasDNS("beta"))
	assert.False(t, sim.HasSecret("beta"))
}
