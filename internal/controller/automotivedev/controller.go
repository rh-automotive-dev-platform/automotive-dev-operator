package automotivedev

import (
	"context"
	"fmt"

	_ "embed"

	"github.com/go-logr/logr"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	automotivev1 "github.com/rh-sdv-cloud-incubator/automotive-dev-operator/api/v1"
	"github.com/rh-sdv-cloud-incubator/automotive-dev-operator/internal/common/tasks"
)

// AutomotiveDevConfigReconciler reconciles a AutomotiveDevConfig object
type AutomotiveDevConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	Ready  chan struct{}
}

const (
	TektonResourcesNamespace = "automotive-dev-operator-system"
)

// +kubebuilder:rbac:groups=automotive.sdv.cloud.redhat.com,resources=automotivedevconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=automotive.sdv.cloud.redhat.com,resources=automotivedevconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=automotive.sdv.cloud.redhat.com,resources=automotivedevconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;watch;create;update;patch;delete;use
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tekton.dev,resources=tasks;pipelines;pipelineruns,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AutomotiveDevConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("automotivedevconfig", req.NamespacedName)
	log.Info("Reconciling AutomotiveDevConfig")

	av := &automotivev1.AutomotiveDevConfig{}
	if err := r.Get(ctx, req.NamespacedName, av); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("AutomotiveDevConfig fetched successfully", "name", av.Name)

	tasks := generateTektonTasks(TektonResourcesNamespace, av.Spec.BuildConfig)
	for _, task := range tasks {
		task.Labels["automotive.sdv.cloud.redhat.com/managed-by"] = av.Name

		if err := controllerutil.SetControllerReference(av, task, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set controller reference: %w", err)
		}

		if err := r.createOrUpdateTask(ctx, task); err != nil {
			log.Error(err, "Failed to create/update Task", "task", task.Name)
			return ctrl.Result{}, err
		}

		log.Info("Task created successfully", "name", task.Name)
	}

	pipeline := generateTektonPipeline("automotive-build-pipeline", TektonResourcesNamespace)

	pipeline.Labels["automotive.sdv.cloud.redhat.com/managed-by"] = av.Name

	if err := controllerutil.SetControllerReference(av, pipeline, r.Scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set controller reference: %w", err)
	}

	if err := r.createOrUpdatePipeline(ctx, pipeline); err != nil {
		log.Error(err, "Failed to create/update Pipeline")
		return ctrl.Result{}, err
	}

	select {
	case <-r.Ready:
	default:
		close(r.Ready)
	}

	log.Info("Successfully reconciled ")
	return ctrl.Result{}, nil
}

func (r *AutomotiveDevConfigReconciler) createOrUpdatePipeline(ctx context.Context, pipeline *tektonv1.Pipeline) error {
	existingPipeline := &tektonv1.Pipeline{}
	err := r.Get(ctx, client.ObjectKey{Name: pipeline.Name, Namespace: pipeline.Namespace}, existingPipeline)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Pipeline: %w", err)
		}
		return r.Create(ctx, pipeline)
	}

	pipeline.ResourceVersion = existingPipeline.ResourceVersion
	return r.Update(ctx, pipeline)
}

func (r *AutomotiveDevConfigReconciler) createOrUpdateTask(ctx context.Context, task *tektonv1.Task) error {
	existingTask := &tektonv1.Task{}
	err := r.Get(ctx, client.ObjectKey{Name: task.Name, Namespace: task.Namespace}, existingTask)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get Task: %w", err)
		}
		return r.Create(ctx, task)
	}

	task.ResourceVersion = existingTask.ResourceVersion
	return r.Update(ctx, task)
}

func (r *AutomotiveDevConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&automotivev1.AutomotiveDevConfig{}).
		Complete(r)
}

func generateTektonTasks(namespace string, buildConfig *automotivev1.BuildConfig) []*tektonv1.Task {
	return []*tektonv1.Task{
		tasks.GenerateBuildAutomotiveImageTask(namespace, buildConfig, ""),
		tasks.GeneratePushArtifactRegistryTask(namespace),
	}
}

func generateTektonPipeline(name, namespace string) *tektonv1.Pipeline {
	return tasks.GenerateTektonPipeline(name, namespace)
}
