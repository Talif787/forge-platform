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
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	var app platformv1alpha1.Application
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		r.mutateDeployment(&app, dep)
		return ctrl.SetControllerReference(&app, dep, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile deployment: %w", err)
	}
	if op != controllerutil.OperationResultNone {
		log.Info("deployment reconciled", "operation", op, "name", dep.Name)
	}

	if app.Spec.Expose {
		svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
			r.mutateService(&app, svc)
			return ctrl.SetControllerReference(&app, svc, r.Scheme)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconcile service: %w", err)
		}
	}

	return ctrl.Result{}, r.updateStatus(ctx, &app, dep)
}

func (r *ApplicationReconciler) mutateDeployment(app *platformv1alpha1.Application, dep *appsv1.Deployment) {
	replicas := app.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}
	labels := managedLabels(app)
	selector := map[string]string{"forge.dev/application": app.Name}

	dep.Labels = labels
	dep.Spec.Replicas = int32Ptr(replicas)
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: selector}
	dep.Spec.Template = corev1.PodTemplateSpec{
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
				Name:  "app",
				Image: app.Spec.Image,
				Ports: []corev1.ContainerPort{{ContainerPort: app.Spec.Port}},
				Env:   toEnvVars(app.Spec.Env),
				Resources: resourceRequirements(app),
				LivenessProbe:  tcpProbe(app.Spec.Port, 15),
				ReadinessProbe: tcpProbe(app.Spec.Port, 5),
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot:             boolPtr(true),
					AllowPrivilegeEscalation: boolPtr(false),
					ReadOnlyRootFilesystem:   boolPtr(true),
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				},
			}},
		},
	}
}

func (r *ApplicationReconciler) mutateService(app *platformv1alpha1.Application, svc *corev1.Service) {
	svc.Labels = managedLabels(app)
	svc.Spec.Selector = map[string]string{"forge.dev/application": app.Name}
	svc.Spec.Type = corev1.ServiceTypeClusterIP
	svc.Spec.Ports = []corev1.ServicePort{{
		Name:       "http",
		Port:       app.Spec.Port,
		TargetPort: intstr.FromInt32(app.Spec.Port),
		Protocol:   corev1.ProtocolTCP,
	}}
}

func (r *ApplicationReconciler) updateStatus(ctx context.Context, app *platformv1alpha1.Application, dep *appsv1.Deployment) error {
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
	return r.Status().Update(ctx, app)
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
