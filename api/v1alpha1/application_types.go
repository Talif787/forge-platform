package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationSpec is the developer contract. It intentionally contains no
// Kubernetes or cloud-specific fields; the controller owns the translation to
// substrate resources, so the platform can change that translation without
// asking teams to edit their manifests.
type ApplicationSpec struct {
	// Image is the container image to run.
	Image string `json:"image"`
	// Port is the port the application listens on.
	Port int32 `json:"port"`
	// Replicas is the desired number of instances (default 1).
	Replicas int32 `json:"replicas,omitempty"`
	// Tier is the criticality (1 to 4); it drives resource and reliability defaults.
	Tier int32 `json:"tier,omitempty"`
	// Env is a list of environment variables.
	Env []EnvVar `json:"env,omitempty"`
	// Resources optionally overrides the tier-derived resource defaults.
	Resources ResourceRequests `json:"resources,omitempty"`
	// Expose, when true, creates a ClusterIP Service in front of the pods.
	Expose bool `json:"expose,omitempty"`
	// Security lets an application opt out of specific hardening defaults when
	// its workload genuinely requires it. Defaults remain strict.
	Security SecuritySettings `json:"security,omitempty"`
}

// SecuritySettings carries opt-outs from the platform's default hardening. Every
// field defaults to the secure value, so omitting this block yields a fully
// hardened workload.
type SecuritySettings struct {
	// ReadOnlyRootFilesystem defaults to true. Set to false only for images that
	// must write to the container filesystem at runtime (for example, servers
	// that create temp directories on startup).
	ReadOnlyRootFilesystem *bool `json:"readOnlyRootFilesystem,omitempty"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ResourceRequests struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type ApplicationStatus struct {
	Phase              string             `json:"phase,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Replicas           int32              `json:"replicas,omitempty"`
	ReadyReplicas      int32              `json:"readyReplicas,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
