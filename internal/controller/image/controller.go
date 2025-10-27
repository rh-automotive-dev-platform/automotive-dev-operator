package image

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	automotivev1 "github.com/rh-automotive-dev-platform/automotive-dev-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ImageReconciler reconciles an Image object
type ImageReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=automotive.sdv.cloud.redhat.com,resources=images,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=automotive.sdv.cloud.redhat.com,resources=images/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=automotive.sdv.cloud.redhat.com,resources=images/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile Image
func (r *ImageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("image", req.NamespacedName)

	image := &automotivev1.Image{}
	if err := r.Get(ctx, req.NamespacedName, image); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle different phases
	switch image.Status.Phase {
	case "":
		return r.handleInitialState(ctx, image)
	case "Verifying":
		return r.handleVerifyingState(ctx, image)
	case "Available":
		return r.handleAvailableState(ctx, image)
	case "Unavailable":
		return r.handleUnavailableState(ctx, image)
	default:
		log.Info("Unknown phase", "phase", image.Status.Phase)
		return ctrl.Result{}, nil
	}
}

func (r *ImageReconciler) handleInitialState(ctx context.Context, image *automotivev1.Image) (ctrl.Result, error) {
	if err := r.updateStatus(ctx, image, "Verifying", "Starting image location verification"); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *ImageReconciler) handleVerifyingState(ctx context.Context, image *automotivev1.Image) (ctrl.Result, error) {
	log := r.Log.WithValues("image", types.NamespacedName{Name: image.Name, Namespace: image.Namespace})

	// Verify the image location is accessible
	accessible, err := r.verifyImageLocation(ctx, image)
	if err != nil {
		log.Error(err, "Failed to verify image location")
		if err := r.updateStatus(ctx, image, "Unavailable", fmt.Sprintf("Verification failed: %v", err)); err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	if accessible {
		if err := r.updateStatus(ctx, image, "Available", "Image location verified and accessible"); err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		// Set LastVerified timestamp
		if err := r.updateLastVerified(ctx, image); err != nil {
			log.Error(err, "Failed to update LastVerified timestamp")
		}
		return ctrl.Result{RequeueAfter: time.Hour * 1}, nil // Recheck every hour
	}

	if err := r.updateStatus(ctx, image, "Unavailable", "Image location is not accessible"); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

func (r *ImageReconciler) handleAvailableState(ctx context.Context, image *automotivev1.Image) (ctrl.Result, error) {
	// Periodically re-verify the image is still accessible
	accessible, err := r.verifyImageLocation(ctx, image)
	if err != nil || !accessible {
		if err := r.updateStatus(ctx, image, "Verifying", "Re-verifying image location"); err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update LastVerified timestamp
	if err := r.updateLastVerified(ctx, image); err != nil {
		r.Log.Error(err, "Failed to update LastVerified timestamp")
	}

	return ctrl.Result{RequeueAfter: time.Hour * 1}, nil // Recheck every hour
}

func (r *ImageReconciler) handleUnavailableState(ctx context.Context, image *automotivev1.Image) (ctrl.Result, error) {
	// Try to verify again after some time
	if err := r.updateStatus(ctx, image, "Verifying", "Retrying image location verification"); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *ImageReconciler) verifyImageLocation(ctx context.Context, image *automotivev1.Image) (bool, error) {
	switch image.Spec.Location.Type {
	case "registry":
		return r.verifyRegistryLocation(ctx, image)
	default:
		return false, fmt.Errorf("unsupported location type: %s (only 'registry' is currently supported)", image.Spec.Location.Type)
	}
}

func (r *ImageReconciler) verifyRegistryLocation(ctx context.Context, image *automotivev1.Image) (bool, error) {
	if image.Spec.Location.Registry == nil {
		return false, fmt.Errorf("registry location configuration is nil")
	}

	if image.Spec.Location.Registry.URL == "" {
		return false, fmt.Errorf("registry URL is required")
	}

	return true, nil // Placeholder - assume accessible for now
}

func (r *ImageReconciler) updateStatus(ctx context.Context, image *automotivev1.Image, phase, message string) error {
	fresh := &automotivev1.Image{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      image.Name,
		Namespace: image.Namespace,
	}, fresh); err != nil {
		return err
	}

	patch := client.MergeFrom(fresh.DeepCopy())

	fresh.Status.Phase = phase
	fresh.Status.Message = message

	// Update conditions
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		Reason:             "Verifying",
		Message:            message,
		LastTransitionTime: now,
	}

	if phase == "Available" {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "Available"
	} else if phase == "Unavailable" {
		condition.Reason = "Unavailable"
	}

	// Update or add the condition
	updated := false
	for i, existingCondition := range fresh.Status.Conditions {
		if existingCondition.Type == "Available" {
			fresh.Status.Conditions[i] = condition
			updated = true
			break
		}
	}
	if !updated {
		fresh.Status.Conditions = append(fresh.Status.Conditions, condition)
	}

	return r.Status().Patch(ctx, fresh, patch)
}

func (r *ImageReconciler) updateLastVerified(ctx context.Context, image *automotivev1.Image) error {
	fresh := &automotivev1.Image{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      image.Name,
		Namespace: image.Namespace,
	}, fresh); err != nil {
		return err
	}

	patch := client.MergeFrom(fresh.DeepCopy())
	now := metav1.Now()
	fresh.Status.LastVerified = &now

	return r.Status().Patch(ctx, fresh, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ImageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&automotivev1.Image{}).
		Complete(r)
}
