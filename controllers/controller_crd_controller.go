package controllers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/codeformio/declare/template"
	"github.com/codeformio/declare/template/jsonnet"

	apiv1 "github.com/codeformio/declare/api/v1"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
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
)

// ControllerCRDReconciler reconciles a CRD created by SiteDefinition with SiteDeployment objects.
type ControllerCRDReconciler struct {
	Log logr.Logger

	ControllerName string
	GVK            schema.GroupVersionKind
	ChildGVKs      []schema.GroupVersionKind

	client client.Client
	scheme *runtime.Scheme
}

func (r *ControllerCRDReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues(strings.ToLower(r.GVK.Kind), req.NamespacedName)

	log.Info("Received reconcile request", "name", req.NamespacedName)

	// Get parent resource.
	var parent unstructured.Unstructured
	parent.SetGroupVersionKind(r.GVK)
	if err := r.client.Get(ctx, req.NamespacedName, &parent); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting parent: %w", err)
	}

	log.Info("Reconciling", "name", parent.GetName())

	// Get Controller that corresponds to the parent resource.
	var c apiv1.Controller
	// TODO: Remove hardcoded "default" namespace.
	if err := r.client.Get(ctx, types.NamespacedName{Name: r.ControllerName, Namespace: "default"}, &c); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting controller: %w", err)
	}

	children := make(map[schema.GroupVersionKind]bool)
	for _, c := range c.Spec.Children {
		children[schema.FromAPIVersionAndKind(c.APIVersion, c.Kind)] = true
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

	tmpl := jsonnet.Templater{
		Files: c.Spec.Source,
	}
	res, err := tmpl.Template(r.client, &template.Input{Object: &parent, Config: cfg})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("templating: %w", err)
	}

	for _, child := range res.Children {
		if gvk := child.GroupVersionKind(); !children[gvk] {
			log.Info("Blocking Controller from creating undeclared child (add child to Controller .spec.children to fix)", "apiVersion", child.GetAPIVersion(), "kind", child.GetKind())
			continue
		}

		// FYI: Creating unstructured object will fail without namespacing.
		// TODO: For cluster scoped resources, check above to see if this can be avoided
		// by avoiding setting the namespace in the Get for the CRD instance.
		// NOTE: If the namespace is specified, do not override it.
		if isNamespaced(child) && child.GetNamespace() == "" {
			child.SetNamespace(c.Namespace)
		}

		// Allow for avoiding ownership references because it can interfere with some
		// children (i.e. Cluster API resources that add owner references, some with
		// "controller: true" and some without).
		switch child.GetAnnotations()[AnnotationOwnershipKey] {
		case AnnotationOwnershipValueNone:
			// Avoid setting any owner references.
		case AnnotationOwnershipValueNonController:
			if err := controllerutil.SetOwnerReference(&parent, child, r.scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("setting owner reference: %w", err)
			}
		default:
			if err := controllerutil.SetControllerReference(&parent, child, r.scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("setting controller reference: %w", err)
			}
		}

		log.Info("Applying child", "name", child.GetName(), "namespace", child.GetNamespace(), "gvk", child.GroupVersionKind())

		{ // TODO: Remove kubectl exec here once server-side apply works for all CRDs.

			// NOTE: Server-side apply fails via kubectl as well as via the Go pkg call below.
			// kubectl apply --force-conflicts=true --server-side

			apply := exec.Command("kubectl", "apply", "--overwrite=true", "-f", "-")
			var stdin, stderr bytes.Buffer
			if err := json.NewEncoder(&stdin).Encode(child); err != nil {
				return ctrl.Result{}, fmt.Errorf("encoding: %w", err)
			}
			apply.Stdin = &stdin
			apply.Stderr = &stderr
			if err := apply.Run(); err != nil {
				return ctrl.Result{}, fmt.Errorf("patching (kubectl apply): %w: %v", err, stderr.String())
			}
		}

		// TODO: Once server-side apply works, start using it over kubectl
		// This currently fails for CAPI CRDs (MachineDeloyment .spec.replicas)
		/*
			if err := r.client.Patch(ctx, child, client.Apply, client.ForceOwnership, client.FieldOwner(r.name())); err != nil {
				// problem, _ := json.Marshal(child)
				return ctrl.Result{}, fmt.Errorf("patching (server-side apply): %w", err)
			}
		*/
	}

	parent.Object["status"] = res.Status
	if err := r.client.Update(ctx, &parent); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating parent status: %v", err)
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
	return strings.ToLower(r.GVK.Kind) + "_controller"
}

func (r *ControllerCRDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	parent := &unstructured.Unstructured{}
	parent.SetGroupVersionKind(r.GVK)

	r.client = mgr.GetClient()
	r.scheme = mgr.GetScheme()

	c := ctrl.NewControllerManagedBy(mgr).
		Named(r.name()).
		For(parent)

	// Watch all children.
	for _, gvk := range r.ChildGVKs {
		r.Log.Info("Starting watch for child", "gvk", gvk.String())
		child := &unstructured.Unstructured{}
		child.SetGroupVersionKind(gvk)
		// Using Watches here with "IsController: false" instead of c.Owns() because
		// Owns sets IsController to true and that we do not always set "controller: true"
		// on children.
		c.Watches(&source.Kind{Type: child}, &handler.EnqueueRequestForOwner{OwnerType: parent, IsController: false})
	}

	// Enqueue requests for all instances of this Controller when this
	// Controller itself gets updated.
	c.Watches(
		&source.Kind{Type: &apiv1.Controller{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: handler.ToRequestsFunc(r.enqueueSelfRequests)},
	)

	// Add watches in for Controller configurations (ConfigMaps & Secrets).
	// NOTE: This appears to be additive in the case where a Controller also
	// creates its own ConfigMap/Secret resources as children.
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
	list.SetGroupVersionKind(r.GVK)
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
