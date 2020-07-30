package controllers

import (
	"context"

	"github.com/go-logr/logr"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	apiv1 "github.com/codeformio/declare/api/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControllerReconciler watches Controller types and notifies the stop channel when
// it sees ones it does not know about.
type ControllerReconciler struct {
	Log     logr.Logger
	Restart chan struct{}

	Registry map[schema.GroupVersionKind]bool

	client client.Client
	scheme *runtime.Scheme
}

func (r *ControllerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	app := &apiv1.Controller{}
	err := r.client.Get(ctx, req.NamespacedName, app)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Ensure CRD exists before starting controllers for it.
	crd := &apiext.CustomResourceDefinition{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: app.Spec.CRDName}, crd); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	if _, ok := r.Registry[schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: crd.Spec.Version,
		Kind:    crd.Spec.Names.Kind,
	}]; !ok {
		close(r.Restart)
	}

	return ctrl.Result{}, nil
}

func (r *ControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()
	r.scheme = mgr.GetScheme()
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Controller{}).
		Named("app").
		Complete(r)
}
