package main

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	platformv1alpha1 "github.com/forge-platform/forge/api/v1alpha1"
	"github.com/forge-platform/forge/internal/controller"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(platformv1alpha1.AddToScheme(scheme))
}

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: ":8081"},
		HealthProbeBindAddress: ":8082",
		LeaderElection:         false,
		LeaderElectionID:       "forge-operator.platform.forge.dev",
	})
	if err != nil {
		setupExit("unable to start manager", err)
	}

	if err := (&controller.ApplicationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupExit("unable to create controller", err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupExit("unable to set up health check", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupExit("unable to set up ready check", err)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupExit("manager exited with error", err)
	}
}

func setupExit(msg string, err error) {
	ctrl.Log.Error(err, msg)
	os.Exit(1)
}
