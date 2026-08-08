//go:build envtest

package kube

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	platformv1alpha1 "github.com/forge-platform/forge/api/v1alpha1"
	"github.com/forge-platform/forge/internal/modules/application/app"
)

var testClient client.Client

func TestMain(m *testing.M) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(platformv1alpha1.AddToScheme(scheme))

	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "..", "..", "config", "crd")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := env.Start()
	if err != nil {
		panic("start envtest: " + err.Error())
	}
	testClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic("build client: " + err.Error())
	}
	code := m.Run()
	_ = env.Stop()
	os.Exit(code)
}

func TestReaderListsAndGets(t *testing.T) {
	ctx := context.Background()
	app1 := &platformv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "payments", Namespace: "default"},
		Spec:       platformv1alpha1.ApplicationSpec{Image: "ghcr.io/acme/payments:1.0", Port: 8080, Replicas: 2, Tier: 2},
	}
	require.NoError(t, testClient.Create(ctx, app1))

	reader := NewReader(testClient)

	views, err := reader.List(ctx, "default")
	require.NoError(t, err)
	require.NotEmpty(t, views)
	found := false
	for _, v := range views {
		if v.Name == "payments" {
			found = true
			assert.Equal(t, "ghcr.io/acme/payments:1.0", v.Image)
			assert.Equal(t, int32(2), v.DesiredReplicas)
		}
	}
	assert.True(t, found, "listed applications should include the created one")

	got, err := reader.Get(ctx, "default", "payments")
	require.NoError(t, err)
	assert.Equal(t, "payments", got.Name)

	_, err = reader.Get(ctx, "default", "does-not-exist")
	assert.ErrorIs(t, err, app.ErrNotFound)
}
