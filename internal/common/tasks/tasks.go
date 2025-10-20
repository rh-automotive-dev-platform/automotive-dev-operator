package tasks

import (
	_ "embed"
	"time"

	automotivev1 "github.com/rh-sdv-cloud-incubator/automotive-dev-operator/api/v1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const AutomotiveImageBuilder = "quay.io/centos-sig-automotive/automotive-image-builder:1.0.0"

// GeneratePushArtifactRegistryTask creates a Tekton Task for pushing artifacts to a registry
func GeneratePushArtifactRegistryTask(namespace string) *tektonv1.Task {
	return &tektonv1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1",
			Kind:       "Task",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "push-artifact-registry",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "automotive-dev-operator",
				"app.kubernetes.io/part-of":    "automotive-dev",
			},
		},
		Spec: tektonv1.TaskSpec{
			Params: []tektonv1.ParamSpec{
				{
					Name:        "distro",
					Type:        tektonv1.ParamTypeString,
					Description: "Distribution to build",
				},
				{
					Name:        "target",
					Type:        tektonv1.ParamTypeString,
					Description: "Build target",
				},
				{
					Name:        "export-format",
					Type:        tektonv1.ParamTypeString,
					Description: "Export format for the build",
				},
				{
					Name:        "repository-url",
					Type:        tektonv1.ParamTypeString,
					Description: "URL of the artifact registry",
				},
				{
					Name:        "secret-ref",
					Type:        tektonv1.ParamTypeString,
					Description: "Name of the secret containing registry credentials",
				},
			},
			Workspaces: []tektonv1.WorkspaceDeclaration{
				{
					Name:        "shared-workspace",
					Description: "Workspace containing the build artifacts",
					MountPath:   "/workspace/shared",
				},
			},
			Steps: []tektonv1.Step{
				{
					Name:  "push-artifact",
					Image: "ghcr.io/oras-project/oras:v1.2.0",
					Env: []corev1.EnvVar{
						{
							Name:  "DOCKER_CONFIG",
							Value: "/tekton/home/.docker",
						},
					},
					Script:     PushArtifactScript,
					WorkingDir: "/workspace/shared",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "docker-config",
							MountPath: "/tekton/home/.docker/config.json",
							SubPath:   ".dockerconfigjson",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "docker-config",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "$(params.secret-ref)",
						},
					},
				},
			},
		},
	}
}

// GenerateBuildAutomotiveImageTask creates a Tekton Task for building automotive images
func GenerateBuildAutomotiveImageTask(namespace string, buildConfig *automotivev1.BuildConfig, envSecretRef string) *tektonv1.Task {
	task := &tektonv1.Task{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1",
			Kind:       "Task",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-automotive-image",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "automotive-dev-operator",
				"app.kubernetes.io/part-of":    "automotive-dev",
			},
		},
		Spec: tektonv1.TaskSpec{
			Params: []tektonv1.ParamSpec{
				{
					Name:        "target-architecture",
					Type:        tektonv1.ParamTypeString,
					Description: "Target architecture for the build",
				},
				{
					Name:        "distro",
					Type:        tektonv1.ParamTypeString,
					Description: "Distribution to build",
				},
				{
					Name:        "target",
					Type:        tektonv1.ParamTypeString,
					Description: "Build target",
				},
				{
					Name:        "mode",
					Type:        tektonv1.ParamTypeString,
					Description: "Build mode",
				},
				{
					Name:        "export-format",
					Type:        tektonv1.ParamTypeString,
					Description: "Export format for the build",
				},
				{
					Name:        "compression",
					Type:        tektonv1.ParamTypeString,
					Description: "Compression algorithm for artifacts (lz4, gzip)",
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: "gzip",
					},
				},
				{
					Name:        "automotive-image-builder",
					Type:        tektonv1.ParamTypeString,
					Description: "automotive-image-builder container image to use",
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: AutomotiveImageBuilder,
					},
				},
			},
			Results: []tektonv1.TaskResult{
				{
					Name:        "manifest-file-path",
					Description: "Path to the manifest file used for building",
				},
				{
					Name:        "artifact-filename",
					Description: "artifact filename placed in the shared workspace",
				},
			},
			Workspaces: []tektonv1.WorkspaceDeclaration{
				{
					Name:        "shared-workspace",
					Description: "Workspace for sharing data between steps",
					MountPath:   "/workspace/shared",
				},
				{
					Name:        "manifest-config-workspace",
					Description: "Workspace for manifest configuration",
					MountPath:   "/workspace/manifest-config",
				},
			},
			Steps: []tektonv1.Step{
				{
					Name:   "find-manifest-file",
					Image:  "quay.io/konflux-ci/yq:latest",
					Script: FindManifestScript,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "manifest-work",
							MountPath: "/manifest-work",
						},
					},
				},
				{
					Name:  "build-image",
					Image: "$(params.automotive-image-builder)",
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To(true),
						SELinuxOptions: &corev1.SELinuxOptions{
							Type: "unconfined_t",
						},
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{},
						},
					},
					Script:  BuildImageScript,
					EnvFrom: buildEnvFrom(envSecretRef),
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "build-dir",
							MountPath: "/_build",
						},
						{
							Name:      "output-dir",
							MountPath: "/output",
						},
						{
							Name:      "run-dir",
							MountPath: "/run/osbuild",
						},
						{
							Name:      "dev",
							MountPath: "/dev",
						},
						{
							Name:      "manifest-work",
							MountPath: "/manifest-work",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "manifest-work",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "build-dir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "output-dir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "run-dir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "dev",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/dev",
						},
					},
				},
			},
		},
	}

	if buildConfig != nil && buildConfig.UseMemoryVolumes {
		for i := range task.Spec.Volumes {
			vol := &task.Spec.Volumes[i]

			if vol.Name == "build-dir" || vol.Name == "run-dir" {
				vol.EmptyDir = &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				}

				if buildConfig.MemoryVolumeSize != "" {
					sizeLimit := resource.MustParse(buildConfig.MemoryVolumeSize)
					vol.EmptyDir.SizeLimit = &sizeLimit
				}
			}
		}
	}

	return task
}

// GenerateTektonPipeline creates a Tekton Pipeline for automotive building process
func GenerateTektonPipeline(name, namespace string) *tektonv1.Pipeline {
	pipeline := &tektonv1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1",
			Kind:       "Pipeline",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "automotive-dev-operator",
			},
		},
		Spec: tektonv1.PipelineSpec{
			Params: []tektonv1.ParamSpec{
				{
					Name: "distro",
					Type: tektonv1.ParamTypeString,
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: "cs9",
					},
					Description: "Build for this distro specification",
				},
				{
					Name: "target",
					Type: tektonv1.ParamTypeString,
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: "qemu",
					},
					Description: "Build for this target",
				},
				{
					Name: "arch",
					Type: tektonv1.ParamTypeString,
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: "aarch64",
					},
					Description: "Build for this architecture",
				},
				{
					Name: "export-format",
					Type: tektonv1.ParamTypeString,
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: "image",
					},
					Description: "Export format for the image (qcow2, image)",
				},
				{
					Name: "mode",
					Type: tektonv1.ParamTypeString,
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: "image",
					},
					Description: "Build this image mode (package, image)",
				},
				{
					Name: "compression",
					Type: tektonv1.ParamTypeString,
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: "lz4",
					},
					Description: "Compression algorithm for artifacts (lz4, gzip)",
				},
				{
					Name:        "storage-class",
					Type:        tektonv1.ParamTypeString,
					Description: "Storage class for the PVC to build on (optional, uses cluster default if not specified)",
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: "",
					},
				},
				{
					Name: "automotive-image-builder",
					Type: tektonv1.ParamTypeString,
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: AutomotiveImageBuilder,
					},
					Description: "automotive-image-builder container image to use for building",
				},
				{
					Name:        "repository-url",
					Type:        tektonv1.ParamTypeString,
					Description: "URL of the artifact registry to push to",
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: "",
					},
				},
				{
					Name:        "secret-ref",
					Type:        tektonv1.ParamTypeString,
					Description: "Secret reference for registry credentials",
					Default: &tektonv1.ParamValue{
						Type:      tektonv1.ParamTypeString,
						StringVal: "",
					},
				},
			},
			Workspaces: []tektonv1.PipelineWorkspaceDeclaration{
				{Name: "shared-workspace"},
				{Name: "manifest-config-workspace"},
			},
			Tasks: []tektonv1.PipelineTask{
				{
					Name: "build-image",
					TaskRef: &tektonv1.TaskRef{
						ResolverRef: tektonv1.ResolverRef{
							Resolver: "cluster",
							Params: []tektonv1.Param{
								{
									Name: "kind",
									Value: tektonv1.ParamValue{
										Type:      tektonv1.ParamTypeString,
										StringVal: "task",
									},
								},
								{
									Name: "name",
									Value: tektonv1.ParamValue{
										Type:      tektonv1.ParamTypeString,
										StringVal: "build-automotive-image",
									},
								},
								{
									Name: "namespace",
									Value: tektonv1.ParamValue{
										Type:      tektonv1.ParamTypeString,
										StringVal: namespace,
									},
								},
							},
						},
					},
					Params: []tektonv1.Param{
						{
							Name: "target-architecture",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.arch)",
							},
						},
						{
							Name: "distro",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.distro)",
							},
						},
						{
							Name: "target",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.target)",
							},
						},
						{
							Name: "mode",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.mode)",
							},
						},
						{
							Name: "export-format",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.export-format)",
							},
						},
						{
							Name: "compression",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.compression)",
							},
						},
						{
							Name: "automotive-image-builder",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.automotive-image-builder)",
							},
						},
					},
					Workspaces: []tektonv1.WorkspacePipelineTaskBinding{
						{Name: "shared-workspace", Workspace: "shared-workspace"},
						{Name: "manifest-config-workspace", Workspace: "manifest-config-workspace"},
					},
					Timeout: &metav1.Duration{Duration: 1 * time.Hour},
				},
				{
					Name: "push-registry",
					TaskRef: &tektonv1.TaskRef{
						ResolverRef: tektonv1.ResolverRef{
							Resolver: "cluster",
							Params: []tektonv1.Param{
								{
									Name: "kind",
									Value: tektonv1.ParamValue{
										Type:      tektonv1.ParamTypeString,
										StringVal: "task",
									},
								},
								{
									Name: "name",
									Value: tektonv1.ParamValue{
										Type:      tektonv1.ParamTypeString,
										StringVal: "push-artifact-registry",
									},
								},
								{
									Name: "namespace",
									Value: tektonv1.ParamValue{
										Type:      tektonv1.ParamTypeString,
										StringVal: namespace,
									},
								},
							},
						},
					},
					Params: []tektonv1.Param{
						{
							Name: "distro",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.distro)",
							},
						},
						{
							Name: "target",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.target)",
							},
						},
						{
							Name: "export-format",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.export-format)",
							},
						},
						{
							Name: "repository-url",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.repository-url)",
							},
						},
						{
							Name: "secret-ref",
							Value: tektonv1.ParamValue{
								Type:      tektonv1.ParamTypeString,
								StringVal: "$(params.secret-ref)",
							},
						},
					},
					Workspaces: []tektonv1.WorkspacePipelineTaskBinding{
						{Name: "shared-workspace", Workspace: "shared-workspace"},
					},
					RunAfter: []string{"build-image"},
					When: []tektonv1.WhenExpression{
						{
							Input:    "$(params.repository-url)",
							Operator: "notin",
							Values:   []string{"", "null"},
						},
						{
							Input:    "$(params.secret-ref)",
							Operator: "notin",
							Values:   []string{"", "null"},
						},
					},
				},
			},
		},
	}

	return pipeline
}

func buildEnvFrom(envSecretRef string) []corev1.EnvFromSource {
	if envSecretRef == "" {
		return nil
	}

	return []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: envSecretRef,
				},
			},
		},
	}
}
