package kube

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	platformv1alpha1 "github.com/forge-platform/forge/api/v1alpha1"
)

// BuildClient resolves cluster config the standard way (in-cluster, then
// KUBECONFIG, then ~/.kube/config) and returns a read client that knows the
// Application type. It does not connect until the first request, so failures to
// reach the cluster surface at call time, not at startup. A nil-config
// environment (no cluster available) returns an error, which the caller treats
// as "applications endpoints disabled" rather than fatal.
func BuildClient() (client.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("resolve kube config: %w", err)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(platformv1alpha1.AddToScheme(scheme))

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("build kube client: %w", err)
	}
	return c, nil
}
