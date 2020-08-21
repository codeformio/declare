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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	// TODO: Better way?
	regularClient client.Client

	client client.Client
	scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ctrl.declare.dev,resources=controllers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ctrl.declare.dev,resources=controllers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *ControllerCRDReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues(strings.ToLower(r.GVK.Kind), req.NamespacedName)

	log.Info("Received reconcile request", "name", req.NamespacedName)

	// TODO: Update this placeholder logic.
	if req.Name == r.ControllerName {
		var list unstructured.UnstructuredList
		list.SetGroupVersionKind(r.GVK)
		if err := r.client.List(ctx, &list); err != nil {
			return ctrl.Result{}, fmt.Errorf("listing controllers: %w", err)
		}
		for _, c := range list.Items {
			log.Info("Rereconciling CRD after Controller change", "namespace", c.GetNamespace(), "name", c.GetName(), "apiVersion", c.GetAPIVersion(), "kind", c.GetKind())
			_, err := r.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: c.GetNamespace(),
				Name:      c.GetName(),
			}})
			if err != nil {
				log.Error(err, "rereconciling CRD instances after Controller change")
			}
		}

		return ctrl.Result{}, nil
	}

	var parent unstructured.Unstructured
	parent.SetGroupVersionKind(r.GVK)
	if err := r.client.Get(ctx, req.NamespacedName, &parent); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("getting parent: %w", err)
	}

	log.Info("Reconciling", "name", parent.GetName())

	var c apiv1.Controller
	// TODO: Remove hardcoded "default" namespace.
	if err := r.regularClient.Get(ctx, types.NamespacedName{Name: r.ControllerName, Namespace: "default"}, &c); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting controller: %w", err)
	}

	tmpl := jsonnet.Templater{
		Files: c.Spec.Source,
	}
	res, err := tmpl.Template(&template.Input{Object: &parent})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("templating: %w", err)
	}

	for _, child := range res.Children {
		for _, specChild := range c.Spec.Children {
			// TODO: Make sure child GVK exists in declared Children.
			_ = specChild
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

	for _, gvk := range r.ChildGVKs {
		r.Log.Info("Starting watch for child", "gvk", gvk.String())
		child := &unstructured.Unstructured{}
		child.SetGroupVersionKind(gvk)
		// Using Watches here with "IsController: false" instead of c.Owns() because
		// Owns sets IsController to true and that we do not always set "controller: true"
		// on children.
		c.Watches(&source.Kind{Type: child}, &handler.EnqueueRequestForOwner{OwnerType: parent, IsController: false})
	}

	// TODO: Is this the right way to watch the Controller?
	c.Watches(&source.Kind{Type: &apiv1.Controller{}}, &handler.EnqueueRequestForObject{})

	return c.Complete(r)
}
