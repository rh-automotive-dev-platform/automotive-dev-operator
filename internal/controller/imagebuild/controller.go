package imagebuild

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	automotivev1 "github.com/rh-automotive-dev-platform/automotive-dev-operator/api/v1"
	"github.com/rh-automotive-dev-platform/automotive-dev-operator/internal/common/tasks"
	pod "github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OperatorNamespace = "automotive-dev-operator-system"
)

// ImageBuildReconciler reconciles a ImageBuild object
type ImageBuildReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=automotive.sdv.cloud.redhat.com,resources=imagebuilds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=automotive.sdv.cloud.redhat.com,resources=imagebuilds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=automotive.sdv.cloud.redhat.com,resources=imagebuilds/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;watch;create;update;patch;delete;use
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tekton.dev,resources=tasks;pipelines;pipelineruns;taskruns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete

// Reconcile ImageBuild
func (r *ImageBuildReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("imagebuild", req.NamespacedName)

	imageBuild := &automotivev1.ImageBuild{}
	if err := r.Get(ctx, req.NamespacedName, imageBuild); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	switch imageBuild.Status.Phase {
	case "":
		return r.handleInitialState(ctx, imageBuild)
	case "Uploading":
		return r.handleUploadingState(ctx, imageBuild)
	case "Building":
		return r.handleBuildingState(ctx, imageBuild)
	case "Completed":
		return r.handleCompletedState(ctx, imageBuild)
	case "Failed":
		return ctrl.Result{}, nil
	default:
		log.Info("Unknown phase", "phase", imageBuild.Status.Phase)
		return ctrl.Result{}, nil
	}
}

func (r *ImageBuildReconciler) handleInitialState(ctx context.Context, imageBuild *automotivev1.ImageBuild) (ctrl.Result, error) {
	if imageBuild.Spec.InputFilesServer {
		if err := r.createUploadPod(ctx, imageBuild); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create upload server: %w", err)
		}
		if err := r.updateStatus(ctx, imageBuild, "Uploading", "Waiting for file uploads"); err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.updateStatus(ctx, imageBuild, "Building", "Build started"); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *ImageBuildReconciler) handleUploadingState(ctx context.Context, imageBuild *automotivev1.ImageBuild) (ctrl.Result, error) {
	uploadsComplete := imageBuild.Annotations != nil &&
		imageBuild.Annotations["automotive.sdv.cloud.redhat.com/uploads-complete"] == "true"

	if !uploadsComplete {
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	if err := r.shutdownUploadPod(ctx, imageBuild); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, fmt.Errorf("failed to shutdown upload server: %w", err)
	}

	if err := r.updateStatus(ctx, imageBuild, "Building", "Build started"); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *ImageBuildReconciler) handleBuildingState(ctx context.Context, imageBuild *automotivev1.ImageBuild) (ctrl.Result, error) {
	log := r.Log.WithValues("imagebuild", types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace})

	if imageBuild.Status.TaskRunName != "" {
		return r.checkBuildProgress(ctx, imageBuild)
	}

	taskRunList := &tektonv1.TaskRunList{}
	if err := r.List(ctx, taskRunList,
		client.InNamespace(imageBuild.Namespace),
		client.MatchingLabels{
			"automotive.sdv.cloud.redhat.com/imagebuild-name": imageBuild.Name,
		}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list existing task runs: %w", err)
	}

	for _, tr := range taskRunList.Items {
		if tr.DeletionTimestamp == nil {
			log.Info("Found existing TaskRun for this ImageBuild", "taskRun", tr.Name)

			latestImageBuild := &automotivev1.ImageBuild{}
			if err := r.Get(ctx, types.NamespacedName{
				Name:      imageBuild.Name,
				Namespace: imageBuild.Namespace,
			}, latestImageBuild); err != nil {
				log.Error(err, "Failed to get latest ImageBuild")
				return ctrl.Result{RequeueAfter: time.Second * 5}, nil
			}

			patch := client.MergeFrom(latestImageBuild.DeepCopy())
			latestImageBuild.Status.TaskRunName = tr.Name

			if err := r.Status().Patch(ctx, latestImageBuild, patch); err != nil {
				log.Error(err, "Failed to patch ImageBuild with existing TaskRun name")
				return ctrl.Result{RequeueAfter: time.Second * 5}, nil
			}

			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
	}

	return r.startNewBuild(ctx, imageBuild)
}

func (r *ImageBuildReconciler) handleCompletedState(ctx context.Context, imageBuild *automotivev1.ImageBuild) (ctrl.Result, error) {
	if !imageBuild.Spec.ServeArtifact {
		return ctrl.Result{}, nil
	}

	log := r.Log.WithValues("imagebuild", types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace})

	expiryHours := int32(24)
	if imageBuild.Spec.ServeExpiryHours > 0 {
		expiryHours = imageBuild.Spec.ServeExpiryHours
	}

	if imageBuild.Status.CompletionTime == nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	expiryAt := imageBuild.Status.CompletionTime.Time.Add(time.Duration(expiryHours) * time.Hour)
	now := time.Now()
	if now.Before(expiryAt) {
		return ctrl.Result{RequeueAfter: time.Until(expiryAt)}, nil
	}

	svcName := fmt.Sprintf("%s-artifact-service", imageBuild.Name)
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: imageBuild.Namespace}}
	if err := r.Delete(ctx, svc); err != nil && !errors.IsNotFound(err) {
		log.Error(err, "failed to delete artifact Service", "service", svcName)
	}

	routeName := fmt.Sprintf("%s-artifacts", imageBuild.Name)
	route := &routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: routeName, Namespace: imageBuild.Namespace}}
	if err := r.Delete(ctx, route); err != nil && !errors.IsNotFound(err) {
		log.Error(err, "failed to delete artifact Route", "route", routeName)
	}

	podName := fmt.Sprintf("%s-artifact-pod", imageBuild.Name)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: imageBuild.Namespace}}
	if err := r.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
		log.Error(err, "failed to delete artifact Pod", "pod", podName)
	}

	cmName := fmt.Sprintf("%s-nginx-config", imageBuild.Name)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: imageBuild.Namespace}}
	if err := r.Delete(ctx, cm); err != nil && !errors.IsNotFound(err) {
		log.Error(err, "failed to delete nginx ConfigMap", "configMap", cmName)
	}

	fresh := &automotivev1.ImageBuild{}
	if err := r.Get(ctx, types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace}, fresh); err == nil {
		patch := client.MergeFrom(fresh.DeepCopy())
		fresh.Status.ArtifactURL = ""
		fresh.Status.ArtifactFileName = ""
		fresh.Status.ArtifactPath = ""
		fresh.Status.Message = "Build expired"
		if err := r.Status().Patch(ctx, fresh, patch); err != nil {
			log.Error(err, "failed to update ImageBuild status after expiry cleanup")
		}
	}

	return ctrl.Result{}, nil
}

func (r *ImageBuildReconciler) checkBuildProgress(ctx context.Context, imageBuild *automotivev1.ImageBuild) (ctrl.Result, error) {
	taskRun := &tektonv1.TaskRun{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      imageBuild.Status.TaskRunName,
		Namespace: imageBuild.Namespace,
	}, taskRun)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if errors.IsNotFound(err) {
		return r.startNewBuild(ctx, imageBuild)
	}

	if !isTaskRunCompleted(taskRun) {
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	if isTaskRunSuccessful(taskRun) {
		var artifactFileName string
		for _, res := range taskRun.Status.TaskRunStatusFields.Results {
			if res.Name == "artifact-filename" && res.Value.StringVal != "" {
				artifactFileName = res.Value.StringVal
				break
			}
		}

		if imageBuild.Spec.ServeArtifact {
			if err := r.createArtifactPod(ctx, imageBuild); err != nil {
				return ctrl.Result{}, err
			}

			if imageBuild.Spec.ExposeRoute {
				if err := r.createArtifactServingResources(ctx, imageBuild); err != nil {
					return ctrl.Result{}, err
				}
			}
		}

		fresh := &automotivev1.ImageBuild{}
		if err := r.Get(ctx, types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace}, fresh); err != nil {
			return ctrl.Result{}, err
		}

		patch := client.MergeFrom(fresh.DeepCopy())

		if artifactFileName != "" {
			fresh.Status.ArtifactFileName = artifactFileName
		}

		fresh.Status.Phase = "Completed"
		fresh.Status.Message = "Build completed successfully"
		if fresh.Status.CompletionTime == nil {
			now := metav1.Now()
			fresh.Status.CompletionTime = &now
		}

		if err := r.Status().Patch(ctx, fresh, patch); err != nil {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// Update artifact info after status is set
		if imageBuild.Spec.ServeArtifact {
			return r.updateArtifactInfo(ctx, imageBuild)
		}

		return ctrl.Result{}, nil
	}

	if err := r.updateStatus(ctx, imageBuild, "Failed", "Build failed"); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}
	return ctrl.Result{}, nil
}

func (r *ImageBuildReconciler) startNewBuild(ctx context.Context, imageBuild *automotivev1.ImageBuild) (ctrl.Result, error) {
	pvcName, err := r.getOrCreateWorkspacePVC(ctx, imageBuild)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get or create workspace PVC: %w", err)
	}

	if imageBuild.Status.PVCName != pvcName {
		fresh := &automotivev1.ImageBuild{}
		if err := r.Get(ctx, types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace}, fresh); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get fresh ImageBuild: %w", err)
		}

		fresh.Status.PVCName = pvcName
		if err := r.Status().Update(ctx, fresh); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update ImageBuild status with PVC name: %w", err)
		}

		imageBuild.Status.PVCName = pvcName
	}

	if err := r.createBuildTaskRun(ctx, imageBuild); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create build task run: %w", err)
	}

	return ctrl.Result{RequeueAfter: time.Second * 30}, nil
}

func (r *ImageBuildReconciler) createBuildTaskRun(ctx context.Context, imageBuild *automotivev1.ImageBuild) error {
	log := r.Log.WithValues("imagebuild", types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace})
	log.Info("Creating TaskRun for ImageBuild")

	autoDev := &automotivev1.AutomotiveDevConfig{}
	err := r.Get(ctx, types.NamespacedName{Name: "automotive-dev", Namespace: OperatorNamespace}, autoDev)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get AutomotiveDevConfig configuration: %w", err)
	}

	var buildConfig *automotivev1.BuildConfig
	if err == nil && autoDev.Spec.BuildConfig != nil {
		buildConfig = autoDev.Spec.BuildConfig
	}
	buildTask := tasks.GenerateBuildAutomotiveImageTask(OperatorNamespace, buildConfig, imageBuild.Spec.EnvSecretRef)

	if imageBuild.Status.PVCName == "" {
		workspacePVCName, err := r.getOrCreateWorkspacePVC(ctx, imageBuild)
		if err != nil {
			return err
		}

		fresh := &automotivev1.ImageBuild{}
		if err := r.Get(ctx, types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace}, fresh); err != nil {
			return fmt.Errorf("failed to get fresh ImageBuild: %w", err)
		}

		fresh.Status.PVCName = workspacePVCName
		if err := r.Status().Update(ctx, fresh); err != nil {
			return fmt.Errorf("failed to update ImageBuild status with PVC name: %w", err)
		}

		imageBuild.Status.PVCName = workspacePVCName
	}

	workspacePVCName := imageBuild.Status.PVCName

	params := []tektonv1.Param{
		{
			Name: "target-architecture",
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: imageBuild.Spec.Architecture,
			},
		},
		{
			Name: "distro",
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: imageBuild.Spec.Distro,
			},
		},
		{
			Name: "target",
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: imageBuild.Spec.Target,
			},
		},
		{
			Name: "mode",
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: imageBuild.Spec.Mode,
			},
		},
		{
			Name: "export-format",
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: imageBuild.Spec.ExportFormat,
			},
		},
		{
			Name: "automotive-image-builder",
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: imageBuild.Spec.AutomotiveImageBuilder,
			},
		},
		{
			Name: "compression",
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: imageBuild.Spec.Compression,
			},
		},
	}

	workspaces := []tektonv1.WorkspaceBinding{
		{
			Name: "shared-workspace",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: workspacePVCName,
			},
		},
		{
			Name: "manifest-config-workspace",
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: imageBuild.Spec.ManifestConfigMap,
				},
			},
		},
	}

	nodeAffinity := &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{imageBuild.Spec.Architecture},
						},
					},
				},
			},
		},
	}

	// prepare podTemplate with runtime class fallback
	podTemplate := &pod.PodTemplate{
		Affinity: &corev1.Affinity{NodeAffinity: nodeAffinity},
	}
	if buildConfig != nil && buildConfig.RuntimeClassName != "" {
		podTemplate.RuntimeClassName = &buildConfig.RuntimeClassName
	}
	if imageBuild.Spec.RuntimeClassName != "" {
		log.Info("Setting RuntimeClassName from ImageBuild spec", "runtimeClassName", imageBuild.Spec.RuntimeClassName)
		podTemplate.RuntimeClassName = &imageBuild.Spec.RuntimeClassName
	}
	taskRun := &tektonv1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-build-", imageBuild.Name),
			Namespace:    imageBuild.Namespace,
			Labels: map[string]string{
				tektonv1.ManagedByLabelKey:                        "automotive-dev-operator",
				"automotive.sdv.cloud.redhat.com/imagebuild-name": imageBuild.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: imageBuild.APIVersion,
					Kind:       imageBuild.Kind,
					Name:       imageBuild.Name,
					UID:        imageBuild.UID,
					Controller: ptr.To(true),
				},
			},
		},
		Spec: tektonv1.TaskRunSpec{
			TaskSpec:    &buildTask.Spec,
			Params:      params,
			Workspaces:  workspaces,
			PodTemplate: podTemplate,
		},
	}

	if err := r.Create(ctx, taskRun); err != nil {
		return fmt.Errorf("failed to create TaskRun: %w", err)
	}

	fresh := &automotivev1.ImageBuild{}
	if err := r.Get(ctx, types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace}, fresh); err != nil {
		return fmt.Errorf("failed to get fresh ImageBuild: %w", err)
	}

	fresh.Status.TaskRunName = taskRun.Name
	if err := r.Status().Update(ctx, fresh); err != nil {
		return fmt.Errorf("failed to update ImageBuild with TaskRun name: %w", err)
	}

	log.Info("Successfully created TaskRun", "name", taskRun.Name)
	return nil
}

func (r *ImageBuildReconciler) updateArtifactInfo(ctx context.Context, imageBuild *automotivev1.ImageBuild) (ctrl.Result, error) {
	log := r.Log.WithValues("imagebuild", types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace})

	latestImageBuild := &automotivev1.ImageBuild{}
	if err := r.Get(ctx, types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace}, latestImageBuild); err != nil {
		log.Error(err, "Failed to get latest ImageBuild")
		return ctrl.Result{}, err
	}

	pvcName := latestImageBuild.Status.PVCName
	if pvcName == "" {
		log.Error(nil, "No PVC name found in ImageBuild status")
		return ctrl.Result{}, fmt.Errorf("no PVC name found in ImageBuild status")
	}

	// Only set ArtifactFileName if it's not already set (from Tekton results)
	if latestImageBuild.Status.ArtifactFileName == "" {
		var fileExtension string
		switch latestImageBuild.Spec.ExportFormat {
		case "image":
			fileExtension = ".raw"
		case "qcow2":
			fileExtension = ".qcow2"
		default:
			fileExtension = fmt.Sprintf(".%s", latestImageBuild.Spec.ExportFormat)
		}

		fileName := fmt.Sprintf("%s-%s%s",
			latestImageBuild.Spec.Distro,
			latestImageBuild.Spec.Target,
			fileExtension)
		latestImageBuild.Status.ArtifactFileName = fileName
	}

	log.Info("Setting artifact info", "pvc", pvcName, "fileName", latestImageBuild.Status.ArtifactFileName)

	patch := client.MergeFrom(latestImageBuild.DeepCopy())

	latestImageBuild.Status.ArtifactPath = "/"

	if err := r.Status().Patch(ctx, latestImageBuild, patch); err != nil {
		log.Error(err, "Failed to patch status with artifact info")
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}

	if latestImageBuild.Spec.ExposeRoute {
		routeName := "ado-build-api"
		timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		route := &routev1.Route{}
		err := wait.PollUntilContextTimeout(
			timeoutCtx,
			time.Second,
			30*time.Second,
			false,
			func(ctx context.Context) (bool, error) {
				if err := r.Get(ctx, client.ObjectKey{Name: routeName, Namespace: latestImageBuild.Namespace}, route); err != nil {
					log.Error(err, "Error getting route")
					return false, nil
				}
				return len(route.Status.Ingress) > 0 && route.Status.Ingress[0].Host != "", nil
			},
		)
		if err != nil {
			log.Error(err, "timed out waiting for route hostname")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		if len(route.Status.Ingress) == 0 || route.Status.Ingress[0].Host == "" {
			log.Info("route status not yet populated with host", "route", routeName)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		scheme := "https"
		if route.Spec.TLS == nil {
			scheme = "http"
			log.Info("TLS is not enabled")
		}

		artifactURL := fmt.Sprintf("%s://%s", scheme, route.Status.Ingress[0].Host)
		log.Info("setting artifact URL in status", "url", artifactURL)

		freshBuild := &automotivev1.ImageBuild{}
		if err := r.Get(ctx, types.NamespacedName{Name: latestImageBuild.Name, Namespace: latestImageBuild.Namespace}, freshBuild); err != nil {
			log.Error(err, "Failed to get fresh ImageBuild for URL update")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		urlPatch := client.MergeFrom(freshBuild.DeepCopy())
		freshBuild.Status.ArtifactURL = artifactURL

		if err := r.Status().Patch(ctx, freshBuild, urlPatch); err != nil {
			log.Error(err, "failed to update ImageBuild status with route URL")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		log.Info("artifact serving resources created and status updated", "route", route.Status.Ingress[0].Host)
	}

	return ctrl.Result{}, nil
}

func (r *ImageBuildReconciler) createArtifactPod(ctx context.Context, imageBuild *automotivev1.ImageBuild) error {
	log := r.Log.WithValues("imagebuild", types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace})

	podName := fmt.Sprintf("%s-artifact-pod", imageBuild.Name)
	existingPod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      podName,
		Namespace: imageBuild.Namespace,
	}, existingPod)

	if err == nil {
		if existingPod.Status.Phase == corev1.PodRunning {
			log.Info("Artifact pod already exists and is running", "pod", podName)
			return nil
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("error checking for existing pod: %w", err)
	}

	workspacePVCName := imageBuild.Status.PVCName
	if workspacePVCName == "" {
		var err error
		workspacePVCName, err = r.getOrCreateWorkspacePVC(ctx, imageBuild)
		if err != nil {
			return err
		}

		fresh := &automotivev1.ImageBuild{}
		if err := r.Get(ctx, types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace}, fresh); err != nil {
			return fmt.Errorf("failed to get fresh ImageBuild: %w", err)
		}

		fresh.Status.PVCName = workspacePVCName
		if err := r.Status().Update(ctx, fresh); err != nil {
			return fmt.Errorf("failed to update ImageBuild status with PVC name: %w", err)
		}

		imageBuild.Status.PVCName = workspacePVCName
	}

	nginxConfigMapName, err := r.createNginxConfigMap(ctx, imageBuild)
	if err != nil {
		return fmt.Errorf("failed to create nginx config map: %w", err)
	}

	labels := map[string]string{
		"app.kubernetes.io/managed-by":                    "automotive-dev-operator",
		"automotive.sdv.cloud.redhat.com/imagebuild-name": imageBuild.Name,
		"app.kubernetes.io/name":                          "artifact-pod",
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: imageBuild.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         imageBuild.APIVersion,
					Kind:               imageBuild.Kind,
					Name:               imageBuild.Name,
					UID:                imageBuild.UID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:    ptr.To[int64](1000),
				RunAsGroup:   ptr.To[int64](1000),
				FSGroup:      ptr.To[int64](1000),
				RunAsNonRoot: ptr.To(true),
			},
			Containers: []corev1.Container{
				{
					Name:  "fileserver",
					Image: "quay.io/nginx/nginx-unprivileged:latest",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "artifacts",
							MountPath: "/workspace/shared",
							ReadOnly:  true,
						},
						{
							Name:      "nginx-config",
							MountPath: "/etc/nginx/conf.d",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "artifacts",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: workspacePVCName,
						},
					},
				},
				{
					Name: "nginx-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: nginxConfigMapName,
							},
						},
					},
				},
			},
		},
	}

	if err := r.Create(ctx, pod); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create artifact pod: %w", err)
	}

	log.Info("Waiting for artifact pod to be ready")
	err = wait.PollUntilContextTimeout(
		ctx,
		5*time.Second,
		2*time.Minute,
		false,
		func(ctx context.Context) (bool, error) {
			if err := r.Get(ctx, client.ObjectKey{Name: podName, Namespace: imageBuild.Namespace}, pod); err != nil {
				return false, nil
			}
			return pod.Status.Phase == corev1.PodRunning, nil
		})

	if err != nil {
		return fmt.Errorf("artifact pod not ready: %w", err)
	}

	log.Info("Artifact pod is ready", "pod", podName)
	return nil
}

func (r *ImageBuildReconciler) createNginxConfigMap(ctx context.Context, imageBuild *automotivev1.ImageBuild) (string, error) {
	configMapName := fmt.Sprintf("%s-nginx-config", imageBuild.Name)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: imageBuild.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         imageBuild.APIVersion,
					Kind:               imageBuild.Kind,
					Name:               imageBuild.Name,
					UID:                imageBuild.UID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
		Data: map[string]string{
			"default.conf": `
server {
    listen 8080;
    server_name localhost;

    # Serve artifacts directly from the mounted PVC
    root /workspace/shared;
    autoindex on;
    autoindex_exact_size off;
    autoindex_localtime on;

    location / {
        try_files $uri =404;
        add_header Cache-Control "no-store" always;
        add_header X-Content-Type-Options nosniff always;
    }

    error_page   500 502 503 504  /50x.html;
    location = /50x.html {
        root   /usr/share/nginx/html;
    }
}
    `,
		},
	}

	if err := r.Create(ctx, configMap); err != nil {
		if errors.IsAlreadyExists(err) {
			return configMapName, nil
		}
		return "", fmt.Errorf("failed to create nginx config ConfigMap: %w", err)
	}

	return configMapName, nil
}

func (r *ImageBuildReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&automotivev1.ImageBuild{}).
		Owns(&tektonv1.TaskRun{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

func isTaskRunCompleted(taskRun *tektonv1.TaskRun) bool {
	return taskRun.Status.CompletionTime != nil
}

func isTaskRunSuccessful(taskRun *tektonv1.TaskRun) bool {
	conditions := taskRun.Status.Conditions
	if len(conditions) == 0 {
		return false
	}

	return conditions[0].Status == corev1.ConditionTrue
}

func (r *ImageBuildReconciler) createUploadPod(ctx context.Context, imageBuild *automotivev1.ImageBuild) error {
	log := r.Log.WithValues("imagebuild", types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace})

	podName := fmt.Sprintf("%s-upload-pod", imageBuild.Name)
	existingPod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      podName,
		Namespace: imageBuild.Namespace,
	}, existingPod)

	if err == nil {
		if existingPod.Status.Phase == corev1.PodRunning {
			log.Info("Upload pod already exists and is running", "pod", podName)
			return nil
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("error checking for existing pod: %w", err)
	}

	workspacePVCName, err := r.getOrCreateWorkspacePVC(ctx, imageBuild)
	if err != nil {
		return err
	}

	if imageBuild.Status.PVCName != workspacePVCName {
		fresh := &automotivev1.ImageBuild{}
		if err := r.Get(ctx, types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace}, fresh); err != nil {
			return fmt.Errorf("failed to get fresh ImageBuild: %w", err)
		}

		fresh.Status.PVCName = workspacePVCName
		if err := r.Status().Update(ctx, fresh); err != nil {
			return fmt.Errorf("failed to update ImageBuild status with PVC name: %w", err)
		}

		imageBuild.Status.PVCName = workspacePVCName
	}

	labels := map[string]string{
		"app.kubernetes.io/managed-by":                    "automotive-dev-operator",
		"automotive.sdv.cloud.redhat.com/imagebuild-name": imageBuild.Name,
		"app.kubernetes.io/name":                          "upload-pod",
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: imageBuild.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         imageBuild.APIVersion,
					Kind:               imageBuild.Kind,
					Name:               imageBuild.Name,
					UID:                imageBuild.UID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:    ptr.To[int64](1000),
				RunAsGroup:   ptr.To[int64](1000),
				FSGroup:      ptr.To[int64](1000),
				RunAsNonRoot: ptr.To(true),
			},
			Containers: []corev1.Container{
				{
					Name:    "fileserver",
					Image:   "quay.io/nginx/nginx-unprivileged:latest",
					Command: []string{"sleep", "infinity"},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "workspace",
							MountPath: "/workspace/shared",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "workspace",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: workspacePVCName,
						},
					},
				},
			},
		},
	}

	if err := r.Create(ctx, pod); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create upload pod: %w", err)
	}

	log.Info("Waiting for upload pod to be ready")
	err = wait.PollUntilContextTimeout(
		ctx,
		5*time.Second,
		2*time.Minute,
		false,
		func(ctx context.Context) (bool, error) {
			if err := r.Get(ctx, client.ObjectKey{Name: podName, Namespace: imageBuild.Namespace}, pod); err != nil {
				return false, nil
			}
			return pod.Status.Phase == corev1.PodRunning, nil
		})

	if err != nil {
		return fmt.Errorf("upload pod not ready: %w", err)
	}

	log.Info("Upload pod is ready", "pod", podName)
	return nil
}

func (r *ImageBuildReconciler) updateStatus(ctx context.Context, imageBuild *automotivev1.ImageBuild, phase, message string) error {
	fresh := &automotivev1.ImageBuild{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      imageBuild.Name,
		Namespace: imageBuild.Namespace,
	}, fresh); err != nil {
		return err
	}

	patch := client.MergeFrom(fresh.DeepCopy())

	fresh.Status.Phase = phase
	fresh.Status.Message = message

	if phase == "Building" && fresh.Status.StartTime == nil {
		now := metav1.Now()
		fresh.Status.StartTime = &now
	} else if (phase == "Completed" || phase == "Failed") && fresh.Status.CompletionTime == nil {
		now := metav1.Now()
		fresh.Status.CompletionTime = &now
	}

	return r.Status().Patch(ctx, fresh, patch)
}

func (r *ImageBuildReconciler) getOrCreateWorkspacePVC(ctx context.Context, imageBuild *automotivev1.ImageBuild) (string, error) {
	log := r.Log.WithValues("imagebuild", types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace})

	if imageBuild.Status.PVCName != "" {
		existingPVC := &corev1.PersistentVolumeClaim{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      imageBuild.Status.PVCName,
			Namespace: imageBuild.Namespace,
		}, existingPVC)

		if err == nil && existingPVC.DeletionTimestamp == nil {
			log.Info("Using existing workspace PVC from status", "pvc", imageBuild.Status.PVCName)
			return imageBuild.Status.PVCName, nil
		}

		log.Info("PVC from status is not available, creating a new one",
			"old-pvc", imageBuild.Status.PVCName)
	}

	autoDev := &automotivev1.AutomotiveDevConfig{}
	err := r.Get(ctx, types.NamespacedName{Name: "automotive-dev", Namespace: OperatorNamespace}, autoDev)

	storageSize := resource.MustParse("8Gi")
	if err == nil && autoDev.Spec.BuildConfig != nil && autoDev.Spec.BuildConfig.PVCSize != "" {
		storageSize = resource.MustParse(autoDev.Spec.BuildConfig.PVCSize)
		log.Info("Using BuildConfig PVCSize", "size", autoDev.Spec.BuildConfig.PVCSize)
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	uniquePVCName := fmt.Sprintf("%s-ws-%s", imageBuild.Name, timestamp)

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uniquePVCName,
			Namespace: imageBuild.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":                    "automotive-dev-operator",
				"automotive.sdv.cloud.redhat.com/imagebuild-name": imageBuild.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         imageBuild.APIVersion,
					Kind:               imageBuild.Kind,
					Name:               imageBuild.Name,
					UID:                imageBuild.UID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
		},
	}

	if imageBuild.Spec.StorageClass != "" {
		pvc.Spec.StorageClassName = &imageBuild.Spec.StorageClass
	}

	if err := r.Create(ctx, pvc); err != nil {
		return "", fmt.Errorf("failed to create workspace PVC: %w", err)
	}

	log.Info("Created new workspace PVC with unique name", "pvc", uniquePVCName)
	return uniquePVCName, nil
}

func (r *ImageBuildReconciler) shutdownUploadPod(ctx context.Context, imageBuild *automotivev1.ImageBuild) error {
	log := r.Log.WithValues("imagebuild", types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace})

	podName := fmt.Sprintf("%s-upload-pod", imageBuild.Name)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: imageBuild.Namespace,
		},
	}

	if err := r.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete upload pod: %w", err)
	}

	log.Info("Upload pod deleted")
	return nil
}

func (r *ImageBuildReconciler) createArtifactServingResources(ctx context.Context, imageBuild *automotivev1.ImageBuild) error {
	log := r.Log.WithValues("imagebuild", types.NamespacedName{Name: imageBuild.Name, Namespace: imageBuild.Namespace})

	podList := &corev1.PodList{}
	if err := r.List(ctx, podList,
		client.InNamespace(imageBuild.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/name":                          "artifact-pod",
			"automotive.sdv.cloud.redhat.com/imagebuild-name": imageBuild.Name,
		}); err != nil {
		return fmt.Errorf("failed to list artifact pods: %w", err)
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no existing artifact pod found for ImageBuild %s", imageBuild.Name)
	}
	artifactPod := &podList.Items[0]

	svcName := fmt.Sprintf("%s-artifact-service", imageBuild.Name)
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: imageBuild.Namespace}, svc)
	if errors.IsNotFound(err) {
		log.Info("Creating artifact service", "name", svcName)
		svc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcName,
				Namespace: imageBuild.Namespace,
				Labels:    artifactPod.Labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         imageBuild.APIVersion,
						Kind:               imageBuild.Kind,
						Name:               imageBuild.Name,
						UID:                imageBuild.UID,
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: artifactPod.Labels,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       8080,
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
		}
		if err := r.Create(ctx, svc); err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check for existing service: %w", err)
	} else {
		log.Info("Artifact service already exists", "name", svcName)
	}

	routeName := fmt.Sprintf("%s-artifacts", imageBuild.Name)
	route := &routev1.Route{}
	err = r.Get(ctx, types.NamespacedName{Name: routeName, Namespace: imageBuild.Namespace}, route)
	if errors.IsNotFound(err) {
		log.Info("Creating artifact route", "name", routeName)
		route = &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      routeName,
				Namespace: imageBuild.Namespace,
				Labels:    artifactPod.Labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         imageBuild.APIVersion,
						Kind:               imageBuild.Kind,
						Name:               imageBuild.Name,
						UID:                imageBuild.UID,
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				},
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: svcName,
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromInt(8080),
				},
			},
		}
		if err := r.Create(ctx, route); err != nil {
			return fmt.Errorf("failed to create route: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check for existing route: %w", err)
	} else {
		log.Info("Artifact route already exists", "name", routeName)
	}

	return nil
}
