package controllers

import (
	"context"
	"fmt"
	"strings"

	apiv1 "github.com/codeformio/declare/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Register(ctx context.Context, cl client.Client, mgr ctrl.Manager, stop chan struct{}) error {
	controllers, err := getControllers(ctx, cl)
	if err != nil {
		return fmt.Errorf("getting controllers: %w", err)
	}

	controllerNames := map[string]bool{}
	for _, c := range controllers {
		controllerNames[c.controllerName] = true
	}

	if err := (&ControllerReconciler{
		Log:                ctrl.Log.WithName("controllers").WithName("ControllerCRD"),
		Restart:            stop,
		ControllerRegistry: controllerNames,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setting up custom site watcher: %w", err)
	}

	for _, c := range controllers {
		r := ControllerCRDReconciler{
			Log:            ctrl.Log.WithName("controllers").WithName(c.mainType.Kind + "Controller"),
			controllerInfo: c,
		}
		if err := r.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("setting up controller crd reconciler for Kind=%v: %w", c.mainType.Kind, err)
		}
	}

	return nil
}

type controllerInfo struct {
	controllerName        string
	mainType              schema.GroupVersionKind
	dependentTypes        []schema.GroupVersionKind
	supportedDependencies map[string]bool
	watchedDependencies   map[string]bool
}

func gvkString(gvk schema.GroupVersionKind) string {
	return strings.ToLower(fmt.Sprintf("%s.%s.%s", gvk.Kind, gvk.Version, gvk.Group))
}

func getControllers(ctx context.Context, cl client.Client) ([]controllerInfo, error) {
	var ctrlList apiv1.ControllerList
	if err := cl.List(ctx, &ctrlList); err != nil {
		return nil, fmt.Errorf("listing: %w", err)
	}

	var controllers []controllerInfo

	for _, c := range ctrlList.Items {
		info := controllerInfo{
			controllerName:        c.Name,
			mainType:              schema.FromAPIVersionAndKind(c.Spec.For.APIVersion, c.Spec.For.Kind),
			supportedDependencies: make(map[string]bool),
			watchedDependencies:   make(map[string]bool),
		}

		if err := resourceTypeExists(ctx, cl, info.mainType); err != nil {
			return nil, fmt.Errorf("checking controller main resource type: %w", err)
		}

		for _, c := range c.Spec.Dependencies {
			gvk := schema.FromAPIVersionAndKind(c.APIVersion, c.Kind)
			if err := resourceTypeExists(ctx, cl, gvk); err == nil {
				info.supportedDependencies[gvkString(gvk)] = true
			}
			info.watchedDependencies[gvkString(gvk)] = c.Watch
			info.dependentTypes = append(info.dependentTypes, gvk)
		}

		controllers = append(controllers, info)
	}

	return controllers, nil
}

// resourceTypeExists attempts to determine if a resource type exists on the API Server.
// if nil is returned, the resource type exists, otherwise it MIGHT not.
// TODO: Improve on this logic if possible.
func resourceTypeExists(ctx context.Context, c client.Client, gvk schema.GroupVersionKind) error {
	var list unstructured.UnstructuredList
	list.SetGroupVersionKind(gvk)
	err := c.List(context.Background(), &list)
	if err == nil {
		return nil
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
