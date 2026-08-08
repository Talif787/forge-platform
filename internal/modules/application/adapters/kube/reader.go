package kube

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/forge-platform/forge/api/v1alpha1"
	"github.com/forge-platform/forge/internal/modules/application/app"
)

// Reader reads Application custom resources through a controller-runtime client.
type Reader struct {
	c client.Client
}

func NewReader(c client.Client) *Reader { return &Reader{c: c} }

func (r *Reader) List(ctx context.Context, namespace string) ([]app.ApplicationView, error) {
	var list platformv1alpha1.ApplicationList
	var opts []client.ListOption
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}
	if err := r.c.List(ctx, &list, opts...); err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	views := make([]app.ApplicationView, 0, len(list.Items))
	for i := range list.Items {
		views = append(views, toView(&list.Items[i]))
	}
	return views, nil
}

func (r *Reader) Get(ctx context.Context, namespace, name string) (app.ApplicationView, error) {
	var a platformv1alpha1.Application
	if err := r.c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &a); err != nil {
		if apierrors.IsNotFound(err) {
			return app.ApplicationView{}, app.ErrNotFound
		}
		return app.ApplicationView{}, fmt.Errorf("get application: %w", err)
	}
	return toView(&a), nil
}

func toView(a *platformv1alpha1.Application) app.ApplicationView {
	conditions := make([]app.Condition, 0, len(a.Status.Conditions))
	for _, c := range a.Status.Conditions {
		conditions = append(conditions, app.Condition{
			Type:               c.Type,
			Status:             string(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			ObservedGeneration: c.ObservedGeneration,
		})
	}
	return app.ApplicationView{
		Namespace:          a.Namespace,
		Name:               a.Name,
		Image:              a.Spec.Image,
		Port:               a.Spec.Port,
		DesiredReplicas:    a.Spec.Replicas,
		Tier:               a.Spec.Tier,
		Expose:             a.Spec.Expose,
		Phase:              a.Status.Phase,
		ReadyReplicas:      a.Status.ReadyReplicas,
		ObservedGeneration: a.Status.ObservedGeneration,
		Conditions:         conditions,
	}
}
