package controllers

import (
	"context"
	"fmt"

	apiv1 "github.com/codeformio/declare/api/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Register(ctx context.Context, cl client.Client, mgr ctrl.Manager, stop chan struct{}) error {
	controllers, err := getParentChildrenGVKs(ctx, cl)
	if err != nil {
		return fmt.Errorf("getting children GVKs: %w", err)
	}

	crdMap := map[schema.GroupVersionKind]bool{}
	for _, c := range controllers {
		crdMap[c.gvk] = true
	}

	if err := (&ControllerReconciler{
		Log:      ctrl.Log.WithName("controllers").WithName("ControllerCRD"),
		Restart:  stop,
		Registry: crdMap,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setting up custom site watcher: %w", err)
	}

	for _, c := range controllers {
		r := ControllerCRDReconciler{
			Log:            ctrl.Log.WithName("controllers").WithName(c.gvk.Kind + "Controller"),
			ControllerName: c.name,
			GVK:            c.gvk,
			ChildGVKs:      c.childrenGVKs,
		}
		if err := r.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("setting up controller crd reconciler for Kind=%v: %w", c.gvk.Kind, err)
		}
	}

	return nil
}

type controllerNameGVK struct {
	name         string
	gvk          schema.GroupVersionKind
	childrenGVKs []schema.GroupVersionKind
}

func getParentChildrenGVKs(ctx context.Context, cl client.Client) ([]controllerNameGVK, error) {
	var ctrlList apiv1.ControllerList
	if err := cl.List(ctx, &ctrlList); err != nil {
		return nil, fmt.Errorf("listing: %w", err)
	}

	var result []controllerNameGVK

	for _, c := range ctrlList.Items {
		var crd apiext.CustomResourceDefinition
		if err := cl.Get(ctx, types.NamespacedName{Name: c.Spec.CRDName}, &crd); err != nil {
			return nil, fmt.Errorf("getting crd: %w", err)
		}

		var children []schema.GroupVersionKind
		for _, c := range c.Spec.Children {
			children = append(children, schema.FromAPIVersionAndKind(c.APIVersion, c.Kind))
		}

		result = append(result, controllerNameGVK{
			name: c.Name,
			gvk: schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Version: crd.Spec.Version,
				Kind:    crd.Spec.Names.Kind,
			},
			childrenGVKs: children,
		})
	}

	return result, nil
}
