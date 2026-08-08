//go:build envtest

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	platformv1alpha1 "github.com/forge-platform/forge/api/v1alpha1"
)

func TestReconcileCreatesDeploymentAndService(t *testing.T) {
	ctx := context.Background()
	name := createApp(t, platformv1alpha1.ApplicationSpec{
		Image: "ghcr.io/acme/payments:1.0", Port: 8080, Replicas: 3, Tier: 2, Expose: true,
		Env: []platformv1alpha1.EnvVar{{Name: "LOG_LEVEL", Value: "info"}},
	})
	reconcile(t, name)

	var dep appsv1.Deployment
	require.NoError(t, k8s.Get(ctx, key(name), &dep))
	assert.Equal(t, int32(3), *dep.Spec.Replicas)

	c := dep.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "ghcr.io/acme/payments:1.0", c.Image)
	assert.Equal(t, int32(8080), c.Ports[0].ContainerPort)
	require.Len(t, c.Env, 1)
	assert.Equal(t, "LOG_LEVEL", c.Env[0].Name)

	// Platform-injected standards the developer never wrote.
	require.NotNil(t, c.LivenessProbe)
	require.NotNil(t, c.ReadinessProbe)
	assert.False(t, c.Resources.Requests.Cpu().IsZero(), "requests must be set")
	assert.False(t, c.Resources.Limits.Cpu().IsZero(), "limits must be enforced")
	require.NotNil(t, c.SecurityContext)
	require.NotNil(t, c.SecurityContext.RunAsNonRoot)
	assert.True(t, *c.SecurityContext.RunAsNonRoot)

	// Owned by the Application for garbage collection.
	require.Len(t, dep.OwnerReferences, 1)
	assert.Equal(t, "Application", dep.OwnerReferences[0].Kind)
	assert.Equal(t, name, dep.OwnerReferences[0].Name)

	var svc corev1.Service
	require.NoError(t, k8s.Get(ctx, key(name), &svc))
	assert.Equal(t, int32(8080), svc.Spec.Ports[0].Port)
}

func TestReconcileInjectsTierDefaults(t *testing.T) {
	ctx := context.Background()
	name := createApp(t, platformv1alpha1.ApplicationSpec{
		Image: "ghcr.io/acme/critical:1.0", Port: 9090, Replicas: 2, Tier: 1,
	})
	reconcile(t, name)

	var dep appsv1.Deployment
	require.NoError(t, k8s.Get(ctx, key(name), &dep))
	c := dep.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "1", c.Resources.Requests.Cpu().String())
	assert.Equal(t, "1Gi", c.Resources.Requests.Memory().String())
}

func TestReconcileCorrectsDrift(t *testing.T) {
	ctx := context.Background()
	name := createApp(t, platformv1alpha1.ApplicationSpec{
		Image: "img:1", Port: 8080, Replicas: 1, Tier: 3,
	})
	reconcile(t, name)

	var dep appsv1.Deployment
	require.NoError(t, k8s.Get(ctx, key(name), &dep))
	require.NoError(t, k8s.Delete(ctx, &dep))

	reconcile(t, name)
	var recreated appsv1.Deployment
	require.NoError(t, k8s.Get(ctx, key(name), &recreated), "controller must recreate a deleted deployment")
}

func TestReconcileNoServiceWhenNotExposed(t *testing.T) {
	ctx := context.Background()
	name := createApp(t, platformv1alpha1.ApplicationSpec{
		Image: "img:1", Port: 8080, Replicas: 1, Tier: 3, Expose: false,
	})
	reconcile(t, name)

	var svc corev1.Service
	err := k8s.Get(ctx, key(name), &svc)
	assert.True(t, apierrors.IsNotFound(err), "no service should exist when expose is false")
}

func TestReconcileUpdatesStatus(t *testing.T) {
	ctx := context.Background()
	name := createApp(t, platformv1alpha1.ApplicationSpec{
		Image: "img:1", Port: 8080, Replicas: 2, Tier: 3,
	})
	reconcile(t, name)

	var got platformv1alpha1.Application
	require.NoError(t, k8s.Get(ctx, key(name), &got))
	assert.Equal(t, got.Generation, got.Status.ObservedGeneration)
	assert.NotEmpty(t, got.Status.Phase)
	require.NotEmpty(t, got.Status.Conditions)
	assert.Equal(t, "Available", got.Status.Conditions[0].Type)
}

func TestReconcileReadOnlyRootDefaultsTrue(t *testing.T) {
	ctx := context.Background()
	name := createApp(t, platformv1alpha1.ApplicationSpec{
		Image: "img:1", Port: 8080, Replicas: 1, Tier: 3,
	})
	reconcile(t, name)

	var dep appsv1.Deployment
	require.NoError(t, k8s.Get(ctx, key(name), &dep))
	sc := dep.Spec.Template.Spec.Containers[0].SecurityContext
	require.NotNil(t, sc.ReadOnlyRootFilesystem)
	assert.True(t, *sc.ReadOnlyRootFilesystem, "read-only root must be the strict default")
}

func TestReconcileReadOnlyRootOptOut(t *testing.T) {
	ctx := context.Background()
	optOut := false
	name := createApp(t, platformv1alpha1.ApplicationSpec{
		Image: "img:1", Port: 8080, Replicas: 1, Tier: 3,
		Security: platformv1alpha1.SecuritySettings{ReadOnlyRootFilesystem: &optOut},
	})
	reconcile(t, name)

	var dep appsv1.Deployment
	require.NoError(t, k8s.Get(ctx, key(name), &dep))
	sc := dep.Spec.Template.Spec.Containers[0].SecurityContext
	require.NotNil(t, sc.ReadOnlyRootFilesystem)
	assert.False(t, *sc.ReadOnlyRootFilesystem, "explicit opt-out must be honored")
	// Other hardening must remain in force.
	assert.True(t, *sc.RunAsNonRoot)
	assert.False(t, *sc.AllowPrivilegeEscalation)
}

func TestAdmissionRejectsLatestTag(t *testing.T) {
	err := tryCreate(platformv1alpha1.ApplicationSpec{
		Image: "nginx:latest", Port: 8080, Replicas: 1, Tier: 3,
	})
	require.Error(t, err, "the API server must reject a :latest image via CEL")
	assert.Contains(t, err.Error(), "latest")
}

func TestAdmissionRejectsTier1SingleReplica(t *testing.T) {
	err := tryCreate(platformv1alpha1.ApplicationSpec{
		Image: "nginx:1.27", Port: 8080, Replicas: 1, Tier: 1,
	})
	require.Error(t, err, "tier 1 with a single replica must be rejected")
	assert.Contains(t, err.Error(), "replicas")
}

func TestAdmissionAcceptsValid(t *testing.T) {
	err := tryCreate(platformv1alpha1.ApplicationSpec{
		Image: "nginx:1.27", Port: 8080, Replicas: 2, Tier: 1,
	})
	require.NoError(t, err)
}

func TestReconcileCreatesObservabilityConfigMap(t *testing.T) {
	ctx := context.Background()
	name := createApp(t, platformv1alpha1.ApplicationSpec{
		Image: "img:1", Port: 8080, Replicas: 1, Tier: 3,
	})
	reconcile(t, name)

	var cm corev1.ConfigMap
	require.NoError(t, k8s.Get(ctx, key(name+"-observability"), &cm))
	assert.Contains(t, cm.Data, "dashboard.json")
	assert.Contains(t, cm.Data, "alerts.yaml")
	assert.Contains(t, cm.Data["dashboard.json"], name, "dashboard should reference the app")
	assert.Equal(t, "1", cm.Labels["grafana_dashboard"])
	require.Len(t, cm.OwnerReferences, 1)
	assert.Equal(t, "Application", cm.OwnerReferences[0].Kind)
}

func TestReconcileIsIdempotent(t *testing.T) {
	ctx := context.Background()
	name := createApp(t, platformv1alpha1.ApplicationSpec{
		Image: "img:1", Port: 8080, Replicas: 1, Tier: 3,
	})
	reconcile(t, name)

	var first appsv1.Deployment
	require.NoError(t, k8s.Get(ctx, key(name), &first))
	rv := first.ResourceVersion

	// A second reconcile of an unchanged Application must not rewrite the
	// Deployment: server-side apply is idempotent, so the resourceVersion is
	// stable. This is what stops the reconcile churn.
	reconcile(t, name)
	var second appsv1.Deployment
	require.NoError(t, k8s.Get(ctx, key(name), &second))
	assert.Equal(t, rv, second.ResourceVersion, "a no-op reconcile must not churn the deployment")
}
