package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	apiv1 "github.com/codeformio/declare/api/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControllerReconciler watches Controller types and notifies the stop channel when
// it sees ones it does not know about.
type ControllerReconciler struct {
	Log     logr.Logger
	Restart chan struct{}

	ControllerRegistry map[string]bool

	client client.Client
	scheme *runtime.Scheme
}

func (r *ControllerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	con := &apiv1.Controller{}
	err := r.client.Get(ctx, req.NamespacedName, con)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.Log.Info("previous controller no longer exists, triggering restart")
			close(r.Restart)
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Ensure parent resource type exists before starting controllers for it.
	parentGVK := schema.FromAPIVersionAndKind(con.Spec.For.APIVersion, con.Spec.For.Kind)
	if err := resourceTypeExists(ctx, r.client, parentGVK); err != nil {
		delay := 10 * time.Second
		r.Log.Error(err, "found controller but unable to find parent resource type, checking again after delay", "delay", delay)
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	if _, ok := r.ControllerRegistry[con.Name]; !ok {
		r.Log.Info("found unregistered controller, triggering restart")
		close(r.Restart)
	}

	return ctrl.Result{}, nil
}

func (r *ControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()
	r.scheme = mgr.GetScheme()
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Controller{}).
		Named("watcher").
		Complete(r)
}
