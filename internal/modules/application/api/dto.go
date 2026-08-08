package api

import "github.com/forge-platform/forge/internal/modules/application/app"

type ConditionResponse struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
}

type ApplicationResponse struct {
	Namespace          string              `json:"namespace"`
	Name               string              `json:"name"`
	Image              string              `json:"image"`
	Port               int32               `json:"port"`
	DesiredReplicas    int32               `json:"desiredReplicas"`
	Tier               int32               `json:"tier"`
	Expose             bool                `json:"expose"`
	Phase              string              `json:"phase"`
	ReadyReplicas      int32               `json:"readyReplicas"`
	ObservedGeneration int64               `json:"observedGeneration"`
	Conditions         []ConditionResponse `json:"conditions"`
}

func toResponse(v app.ApplicationView) ApplicationResponse {
	conditions := make([]ConditionResponse, 0, len(v.Conditions))
	for _, c := range v.Conditions {
		conditions = append(conditions, ConditionResponse{
			Type:               c.Type,
			Status:             c.Status,
			Reason:             c.Reason,
			Message:            c.Message,
			ObservedGeneration: c.ObservedGeneration,
		})
	}
	return ApplicationResponse{
		Namespace:          v.Namespace,
		Name:               v.Name,
		Image:              v.Image,
		Port:               v.Port,
		DesiredReplicas:    v.DesiredReplicas,
		Tier:               v.Tier,
		Expose:             v.Expose,
		Phase:              v.Phase,
		ReadyReplicas:      v.ReadyReplicas,
		ObservedGeneration: v.ObservedGeneration,
		Conditions:         conditions,
	}
}
