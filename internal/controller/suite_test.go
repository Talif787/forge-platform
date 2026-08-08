//go:build envtest

package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	platformv1alpha1 "github.com/forge-platform/forge/api/v1alpha1"
)

var (
	testEnv    *envtest.Environment
	cfg        *rest.Config
	k8s        client.Client
	testScheme *runtime.Scheme
)

func TestMain(m *testing.M) {
	testScheme = runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(testScheme))
	utilruntime.Must(platformv1alpha1.AddToScheme(testScheme))

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd")},
		ErrorIfCRDPathMissing: true,
	}
	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		panic("start envtest: " + err.Error())
	}
	k8s, err = client.New(cfg, client.Options{Scheme: testScheme})
	if err != nil {
		panic("build client: " + err.Error())
	}

	code := m.Run()
	_ = testEnv.Stop()
	os.Exit(code)
}

func newReconciler() *ApplicationReconciler {
	return &ApplicationReconciler{Client: k8s, Scheme: testScheme}
}

func uniqueName() string { return "app-" + uuid.NewString() }

func createApp(t *testing.T, spec platformv1alpha1.ApplicationSpec) string {
	t.Helper()
	name := uniqueName()
	app := &platformv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       spec,
	}
	require.NoError(t, k8s.Create(context.Background(), app))
	return name
}

func reconcile(t *testing.T, name string) {
	t.Helper()
	_, err := newReconciler().Reconcile(context.Background(),
		ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "default"}})
	require.NoError(t, err)
}

func key(name string) types.NamespacedName {
	return types.NamespacedName{Name: name, Namespace: "default"}
}
