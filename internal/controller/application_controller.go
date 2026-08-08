package controller

import (
	"context"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/forge-platform/forge/api/v1alpha1"
)

// tierDefaults maps criticality tier to default resource requests. Limits are
// derived from requests. This is where the platform enforces the standards a
// team would otherwise forget: every workload gets requests, limits, probes,
// and a hardened security context whether or not the developer asked.
var tierDefaults = map[int32]platformv1alpha1.ResourceRequests{
	1: {CPU: "1", Memory: "1Gi"},
	2: {CPU: "500m", Memory: "512Mi"},
	3: {CPU: "250m", Memory: "256Mi"},
	4: {CPU: "100m", Memory: "128Mi"},
}

type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=platform.forge.dev,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=platform.forge.dev,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// applyOpts configure server-side apply: a stable field owner and forced
// ownership so the operator authoritatively manages the fields it sets while
// leaving server-defaulted fields untouched. This makes reconciles idempotent
// (no update churn) and avoids read-modify-write conflicts.
func (r *ApplicationReconciler) applyOpts() []client.PatchOption {
	return []client.PatchOption{client.FieldOwner("forge-operator"), client.ForceOwnership}
}

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	var app platformv1alpha1.Application
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	dep := r.desiredDeployment(&app)
	if err := ctrl.SetControllerReference(&app, dep, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Patch(ctx, dep, client.Apply, r.applyOpts()...); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply deployment: %w", err)
	}

	if app.Spec.Expose {
		svc := r.desiredService(&app)
		if err := ctrl.SetControllerReference(&app, svc, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Patch(ctx, svc, client.Apply, r.applyOpts()...); err != nil {
			return ctrl.Result{}, fmt.Errorf("apply service: %w", err)
		}
	}

	obs := r.desiredObservability(&app)
	if err := ctrl.SetControllerReference(&app, obs, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Patch(ctx, obs, client.Apply, r.applyOpts()...); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply observability: %w", err)
	}

	// Read back the live Deployment for its status, then update the Application
	// status under a conflict retry.
	var live appsv1.Deployment
	if err := r.Get(ctx, client.ObjectKeyFromObject(dep), &live); err != nil {
		return ctrl.Result{}, fmt.Errorf("read deployment status: %w", err)
	}
	changed, err := r.updateStatus(ctx, req.NamespacedName, &live)
	if err != nil {
		return ctrl.Result{}, err
	}
	if changed {
		log.Info("application status updated", "name", app.Name, "readyReplicas", live.Status.ReadyReplicas)
	}
	return ctrl.Result{}, nil
}

// desiredObservability generates a Grafana dashboard and Prometheus alert rules
// for the application, delivered as a ConfigMap the respective sidecars pick up.
// Every managed service gets metrics discovery (via pod annotations), a
// dashboard, and alerts automatically, with no work from the developer.
func (r *ApplicationReconciler) desiredObservability(app *platformv1alpha1.Application) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name + "-observability",
			Namespace: app.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "forge",
				"forge.dev/application":        app.Name,
				"grafana_dashboard":            "1",
				"forge.dev/prometheus-rules":   "true",
			},
		},
		Data: map[string]string{
			"dashboard.json": dashboardJSON(app),
			"alerts.yaml":    alertRules(app),
		},
	}
}

func dashboardJSON(app *platformv1alpha1.Application) string {
	return fmt.Sprintf(`{
  "title": "Forge / %[1]s",
  "uid": "forge-%[1]s",
  "tags": ["forge", "managed"],
  "panels": [
    {"title": "Request rate", "type": "timeseries", "targets": [{"expr": "sum(rate(http_requests_total{app=\"%[1]s\"}[5m]))"}]},
    {"title": "p95 latency", "type": "timeseries", "targets": [{"expr": "histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{app=\"%[1]s\"}[5m])) by (le))"}]},
    {"title": "Ready replicas", "type": "stat", "targets": [{"expr": "kube_deployment_status_replicas_ready{deployment=\"%[1]s\"}"}]}
  ]
}`, app.Name)
}

func alertRules(app *platformv1alpha1.Application) string {
	desired := app.Spec.Replicas
	if desired == 0 {
		desired = 1
	}
	return fmt.Sprintf(`groups:
  - name: forge-%[1]s
    rules:
      - alert: HighErrorRate
        expr: sum(rate(http_requests_total{app="%[1]s",status=~"5.."}[5m])) / sum(rate(http_requests_total{app="%[1]s"}[5m])) > 0.05
        for: 10m
        labels:
          severity: page
          forge_application: %[1]s
        annotations:
          summary: "High 5xx error rate for %[1]s"
      - alert: InsufficientReplicas
        expr: kube_deployment_status_replicas_ready{deployment="%[1]s"} < %[2]d
        for: 15m
        labels:
          severity: page
          forge_application: %[1]s
        annotations:
          summary: "%[1]s has fewer ready replicas than desired"
`, app.Name, desired)
}

func (r *ApplicationReconciler) desiredDeployment(app *platformv1alpha1.Application) *appsv1.Deployment {
	replicas := app.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}
	labels := managedLabels(app)
	selector := map[string]string{"forge.dev/application": app.Name}

	return &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   strconv.Itoa(int(app.Spec.Port)),
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: boolPtr(true)},
					Containers: []corev1.Container{{
						Name:           "app",
						Image:          app.Spec.Image,
						Ports:          []corev1.ContainerPort{{ContainerPort: app.Spec.Port}},
						Env:            toEnvVars(app.Spec.Env),
						Resources:      resourceRequirements(app),
						LivenessProbe:  tcpProbe(app.Spec.Port, 15),
						ReadinessProbe: tcpProbe(app.Spec.Port, 5),
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             boolPtr(true),
							AllowPrivilegeEscalation: boolPtr(false),
							ReadOnlyRootFilesystem:   boolPtr(readOnlyRoot(app)),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					}},
				},
			},
		},
	}
}

func (r *ApplicationReconciler) desiredService(app *platformv1alpha1.Application) *corev1.Service {
	return &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace, Labels: managedLabels(app)},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"forge.dev/application": app.Name},
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       app.Spec.Port,
				TargetPort: intstr.FromInt32(app.Spec.Port),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

// updateStatus recomputes status from the live Deployment and writes it under a
// conflict retry, re-reading the Application each attempt. It returns whether a
// write actually happened, so an unchanged status does not churn the object or
// trigger further reconciles.
func (r *ApplicationReconciler) updateStatus(ctx context.Context, key types.NamespacedName, dep *appsv1.Deployment) (bool, error) {
	wrote := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var app platformv1alpha1.Application
		if err := r.Get(ctx, key, &app); err != nil {
			return err
		}

		desired := app.Spec.Replicas
		if desired == 0 {
			desired = 1
		}
		ready := dep.Status.ReadyReplicas
		phase := "Progressing"
		condStatus := metav1.ConditionFalse
		if ready >= desired && desired > 0 {
			phase = "Ready"
			condStatus = metav1.ConditionTrue
		}

		if app.Status.Phase == phase &&
			app.Status.ReadyReplicas == ready &&
			app.Status.Replicas == dep.Status.Replicas &&
			app.Status.ObservedGeneration == app.Generation {
			wrote = false
			return nil
		}

		app.Status.Phase = phase
		app.Status.ObservedGeneration = app.Generation
		app.Status.Replicas = dep.Status.Replicas
		app.Status.ReadyReplicas = ready
		setCondition(&app.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             condStatus,
			ObservedGeneration: app.Generation,
			Reason:             phase,
			Message:            fmt.Sprintf("%d/%d replicas ready", ready, desired),
		})
		wrote = true
		return r.Status().Update(ctx, &app)
	})
	return wrote, err
}

func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.Application{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func managedLabels(app *platformv1alpha1.Application) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       app.Name,
		"app.kubernetes.io/managed-by": "forge",
		"forge.dev/application":        app.Name,
		"forge.dev/tier":               strconv.Itoa(int(effectiveTier(app))),
	}
}

func effectiveTier(app *platformv1alpha1.Application) int32 {
	if app.Spec.Tier >= 1 && app.Spec.Tier <= 4 {
		return app.Spec.Tier
	}
	return 3
}

// readOnlyRoot enforces a read-only root filesystem unless the application
// explicitly opts out. Omitting the security block yields the strict default.
func readOnlyRoot(app *platformv1alpha1.Application) bool {
	if app.Spec.Security.ReadOnlyRootFilesystem != nil {
		return *app.Spec.Security.ReadOnlyRootFilesystem
	}
	return true
}

func resourceRequirements(app *platformv1alpha1.Application) corev1.ResourceRequirements {
	def := tierDefaults[effectiveTier(app)]
	cpu, mem := def.CPU, def.Memory
	if app.Spec.Resources.CPU != "" {
		cpu = app.Spec.Resources.CPU
	}
	if app.Spec.Resources.Memory != "" {
		mem = app.Spec.Resources.Memory
	}
	q := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpu),
		corev1.ResourceMemory: resource.MustParse(mem),
	}
	// Limits equal requests here (a conservative default); a real platform would
	// set limits to a multiple of requests per tier.
	return corev1.ResourceRequirements{Requests: q, Limits: q.DeepCopy()}
}

func toEnvVars(in []platformv1alpha1.EnvVar) []corev1.EnvVar {
	if len(in) == 0 {
		return nil
	}
	out := make([]corev1.EnvVar, 0, len(in))
	for _, e := range in {
		out = append(out, corev1.EnvVar{Name: e.Name, Value: e.Value})
	}
	return out
}

func tcpProbe(port int32, initialDelay int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(port)},
		},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       10,
		TimeoutSeconds:      2,
		FailureThreshold:    3,
	}
}

func int32Ptr(i int32) *int32 { return &i }
func boolPtr(b bool) *bool    { return &b }

// setCondition upserts a condition by type without importing the apimachinery
// meta helper, keeping the dependency surface small.
func setCondition(conditions *[]metav1.Condition, c metav1.Condition) {
	if c.LastTransitionTime.IsZero() {
		c.LastTransitionTime = metav1.Now()
	}
	for i := range *conditions {
		if (*conditions)[i].Type == c.Type {
			if (*conditions)[i].Status == c.Status {
				c.LastTransitionTime = (*conditions)[i].LastTransitionTime
			}
			(*conditions)[i] = c
			return
		}
	}
	*conditions = append(*conditions, c)
}
