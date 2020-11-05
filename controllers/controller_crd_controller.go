package controllers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/codeformio/declare/template"
	templatefactory "github.com/codeformio/declare/template/factory"

	apiv1 "github.com/codeformio/declare/api/v1"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	AnnotationOwnershipKey = "ctrl.declare.dev/ownership"
	// AnnotationOwnershipValueNone avoids setting any owner references.
	AnnotationOwnershipValueNone = "none"
	// AnnotationOwnershipValueNonController sets an owner reference without "controller: true".
	AnnotationOwnershipValueNonController = "non-controller"

	EventReasonFailedTemplating = "FailedTemplating"
	EventReasonFailedApplying   = "FailedApplying"
	EventReasonApplied          = "Applied"
)

// ControllerCRDReconciler reconciles a CRD created by SiteDefinition with SiteDeployment objects.
type ControllerCRDReconciler struct {
	Log logr.Logger

	controllerInfo

	recorder record.EventRecorder
	client   client.Client
	scheme   *runtime.Scheme
}

func (r *ControllerCRDReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues(strings.ToLower(r.mainType.Kind), req.NamespacedName)

	log.Info("Received reconcile request", "name", req.NamespacedName)

	// Get main resource.
	var main unstructured.Unstructured
	main.SetGroupVersionKind(r.mainType)
	if err := r.client.Get(ctx, req.NamespacedName, &main); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting main resource: %w", err)
	}

	log.Info("Reconciling", "name", main.GetName())

	// Get Controller that corresponds to the main resource.
	var c apiv1.Controller
	// TODO: Remove hardcoded "default" namespace.
	if err := r.client.Get(ctx, types.NamespacedName{Name: r.controllerName, Namespace: "default"}, &c); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting controller: %w", err)
	}

	dependencies := make(map[schema.GroupVersionKind]bool)
	for _, c := range c.Spec.Dependencies {
		dependencies[schema.FromAPIVersionAndKind(c.APIVersion, c.Kind)] = true
	}

	cfg := make(map[string]string)
	// Add ownership to referenced configuration (Secrets/ConfigMaps).
	for _, cfgSrc := range c.Spec.Config {
		lg := log.WithValues("namespace", c.Namespace)
		// TODO: Validate that only one source is defined per ConfigSource.
		if name := cfgSrc.Secret; name != "" {
			lg := lg.WithValues("name", name)
			lg.Info("Getting config Secret")
			var s corev1.Secret
			if err := r.client.Get(ctx, types.NamespacedName{Name: name, Namespace: c.Namespace}, &s); err != nil {
				if apierrors.IsNotFound(err) {
					lg.Info("Config Secret not found")
				} else {
					log.Error(err, "Error getting config Secret")
				}
				continue
			}
			if err := controllerutil.SetOwnerReference(&c, &s, r.scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("setting owner reference on secret: %w", err)
			}
			if err := r.client.Update(ctx, &s); err != nil {
				return ctrl.Result{}, fmt.Errorf("updating config secret with owner reference: %v", err)
			}
			for k, v := range s.Data {
				cfg[k] = string(v)
			}
		}
		if name := cfgSrc.ConfigMap; name != "" {
			lg := lg.WithValues("name", name)
			lg.Info("Getting config ConfigMap")
			var cm corev1.ConfigMap
			if err := r.client.Get(ctx, types.NamespacedName{Name: name, Namespace: c.Namespace}, &cm); err != nil {
				if apierrors.IsNotFound(err) {
					lg.Info("Config ConfigMap not found")
				} else {
					log.Error(err, "Error getting config ConfigMap")
				}
				continue
			}
			if err := controllerutil.SetOwnerReference(&c, &cm, r.scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("setting owner reference on configmap: %w", err)
			}
			if err := r.client.Update(ctx, &cm); err != nil {
				return ctrl.Result{}, fmt.Errorf("updating config configmap with owner reference: %v", err)
			}
			for k, v := range cm.Data {
				cfg[k] = string(v)
			}
		}
	}

	tmpl, err := templatefactory.New(c.Spec.Source)
	if err != nil {
		r.recorder.Event(&main, corev1.EventTypeWarning, EventReasonFailedTemplating, "Invalid source: "+err.Error())
		log.Info("detecting source language", "error", err.Error())
		return ctrl.Result{}, nil
	}

	res, err := tmpl.Template(r.client, &template.Input{Object: &main, Config: cfg, Supported: r.supportedDependencies})
	if err != nil {
		r.recorder.Event(&main, corev1.EventTypeWarning, EventReasonFailedTemplating, err.Error())
		log.Info("templating resulting in an error", "error", err.Error())
		return ctrl.Result{}, nil
	}

	for _, obj := range res.Apply {
		log := log.WithValues("kind", obj.GetKind())
		publishFailure := func(err error) {
			r.recorder.Event(&main, corev1.EventTypeWarning, EventReasonFailedApplying, err.Error())
			log.Info("Apply failed", "error", err.Error())
		}

		if gvk := obj.GroupVersionKind(); !dependencies[gvk] {
			apiV, kind := gvk.ToAPIVersionAndKind()
			publishFailure(fmt.Errorf("dependency is not declared in Controller (.spec.dependencies): apiVersion: %s kind: %s", apiV, kind))
			continue
		}

		// FYI: Creating unstructured object will fail without namespacing.
		// TODO: For cluster scoped resources, check above to see if this can be avoided
		// by avoiding setting the namespace in the Get for the CRD instance.
		// NOTE: If the namespace is specified, do not override it.
		if isNamespaced(obj) && obj.GetNamespace() == "" {
			obj.SetNamespace(c.Namespace)
		}

		// Allow for avoiding ownership references because it can interfere with some
		// objects (i.e. Cluster API resources that add owner references, some with
		// "controller: true" and some without).
		switch obj.GetAnnotations()[AnnotationOwnershipKey] {
		case AnnotationOwnershipValueNone:
			// Avoid setting any owner references.
		case AnnotationOwnershipValueNonController:
			if err := controllerutil.SetOwnerReference(&main, obj, r.scheme); err != nil {
				log.Error(err, "Unable to set owner reference")
				continue
			}
		default:
			if err := controllerutil.SetControllerReference(&main, obj, r.scheme); err != nil {
				log.Error(err, "Unable to set controller reference")
				continue
			}
		}

		log.Info("Applying", "name", obj.GetName(), "namespace", obj.GetNamespace(), "gvk", obj.GroupVersionKind())

		{ // TODO: Remove kubectl exec here once server-side apply works for all CRDs.

			// NOTE: Server-side apply fails via kubectl as well as via the Go pkg call below.
			// kubectl apply --force-conflicts=true --server-side

			apply := exec.Command("kubectl", "apply", "--overwrite=true", "-f", "-")
			var stdin, stderr bytes.Buffer
			if err := json.NewEncoder(&stdin).Encode(obj); err != nil {
				publishFailure(fmt.Errorf("encoding: %w", err))
				continue
			}
			apply.Stdin = &stdin
			apply.Stderr = &stderr
			if err := apply.Run(); err != nil {
				publishFailure(fmt.Errorf("patching (kubectl apply): %w: %v", err, stderr.String()))
				continue
			}
		}

		// TODO: Once server-side apply works, start using it over kubectl
		// This currently fails for CAPI CRDs (MachineDeloyment .spec.replicas)
		/*
			if err := r.client.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(r.name())); err != nil {
				// problem, _ := json.Marshal(obj)
				return ctrl.Result{}, fmt.Errorf("patching (server-side apply): %w", err)
			}
		*/

		r.recorder.Eventf(&main, corev1.EventTypeNormal, EventReasonApplied, "Successfully applied object %s: %s", obj.GetKind(), obj.GetName())
		log.Info("Applied object")
	}

	main.Object["status"] = res.Status
	if err := r.client.Update(ctx, &main); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating main status: %v", err)
	}

	// TODO: Account for garbage collection of conditional resources.

	return ctrl.Result{}, nil
}

func isNamespaced(u *unstructured.Unstructured) bool {
	kind := u.GetKind()

	switch kind {
	// TODO: Add more or find existing func.
	case "Namespace":
		return false
	}

	return true
}

func (r *ControllerCRDReconciler) name() string {
	return strings.ToLower(r.mainType.Kind) + "_controller"
}

func (r *ControllerCRDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	main := &unstructured.Unstructured{}
	main.SetGroupVersionKind(r.mainType)

	r.client = mgr.GetClient()
	r.scheme = mgr.GetScheme()
	r.recorder = mgr.GetEventRecorderFor(r.controllerName)

	c := ctrl.NewControllerManagedBy(mgr).
		Named(r.name()).
		For(main)

	// Watch all dependents.
	for _, gvk := range r.dependentTypes {
		if !r.supportedDependencies[supportedDependencyKey(gvk)] {
			r.Log.Info("Skipping watch for unsupported dependent", "gvk", gvk.String())
			continue
		}
		r.Log.Info("Starting watch for dependent", "gvk", gvk.String())
		dependent := &unstructured.Unstructured{}
		dependent.SetGroupVersionKind(gvk)
		// Using Watches here with "IsController: false" instead of c.Owns() because
		// Owns sets IsController to true and that we do not always set "controller: true"
		// on dependent.
		c.Watches(&source.Kind{Type: dependent}, &handler.EnqueueRequestForOwner{OwnerType: main, IsController: false})
	}

	// Enqueue requests for all instances of this Controller when this
	// Controller itself gets updated.
	c.Watches(
		&source.Kind{Type: &apiv1.Controller{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.enqueueSelfRequests)},
	)

	// Add watches in for Controller configurations (ConfigMaps & Secrets).
	// NOTE: This appears to be additive in the case where a Controller also
	// creates its own ConfigMap/Secret resources as dependents.
	// These global configuration changes should trigger reconcile loops for
	// each instance of this Controller.
	c.Watches(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.enqueueConfigRequests)},
	)
	c.Watches(
		&source.Kind{Type: &corev1.ConfigMap{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.enqueueConfigRequests)},
	)

	return c.Complete(r)
}

func (r *ControllerCRDReconciler) enqueueSelfRequests(a handler.MapObject) []reconcile.Request {
	return r.listInstancesToReconcile()
}

func (r *ControllerCRDReconciler) enqueueConfigRequests(a handler.MapObject) []reconcile.Request {
	var requests []reconcile.Request

	ownRefs := a.Meta.GetOwnerReferences()
	for _, ref := range ownRefs {
		if ref.APIVersion == apiv1.GroupVersion.String() && ref.Kind == apiv1.ControllerKind {
			requests = append(requests, r.listInstancesToReconcile()...)
		}
	}

	return requests
}

func (r *ControllerCRDReconciler) listInstancesToReconcile() []reconcile.Request {
	log := r.Log

	var list unstructured.UnstructuredList
	list.SetGroupVersionKind(r.mainType)
	if err := r.client.List(context.Background(), &list); err != nil {
		log.Error(err, "Listing instance of CRD")
		return nil
	}

	var requests []reconcile.Request

	for _, c := range list.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      c.GetName(),
				Namespace: c.GetNamespace(),
			},
		})
	}

	return requests
}
