/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AutomotiveDevConfigSpec defines the desired state of AutomotiveDevConfig
type AutomotiveDevConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// BuildConfig defines the global configuration for build operations
	BuildConfig *BuildConfig `json:"buildConfig,omitempty"`
}

// BuildConfig defines configuration options for build operations
type BuildConfig struct {
	// UseMemoryVolumes determines whether to use memory-backed volumes for build operations
	UseMemoryVolumes bool `json:"useMemoryVolumes,omitempty"`

	// MemoryVolumeSize specifies the size limit for memory-backed volumes (required if UseMemoryVolumes is true)
	// Example: "2Gi"
	MemoryVolumeSize string `json:"memoryVolumeSize,omitempty"`

	// PVCSize specifies the size for persistent volume claims created for build workspaces
	// Default: "8Gi"
	// +optional
	PVCSize string `json:"pvcSize,omitempty"`

	// RuntimeClassName specifies the runtime class to use for the build pod
	// More info: https://kubernetes.io/docs/concepts/containers/runtime-class/
	// +optional
	RuntimeClassName string `json:"runtimeClassName,omitempty"`

	// ServeExpiryHours specifies how long to serve build artifacts before automatic cleanup
	// Default: 24
	// +optional
	ServeExpiryHours int32 `json:"serveExpiryHours,omitempty"`
}

// AutomotiveDevConfigStatus defines the observed state of AutomotiveDevConfig
type AutomotiveDevConfigStatus struct {
	// Phase represents the current phase of the AutomotiveDevConfig environment (Ready, Pending, Failed)
	Phase string `json:"phase,omitempty"`

	// Message provides more detail about the current phase
	Message string `json:"message,omitempty"`

	// LastUpdated is when the status was last updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AutomotiveDevConfig is the Schema for the automotivedevconfigs API
type AutomotiveDevConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AutomotiveDevConfigSpec   `json:"spec,omitempty"`
	Status AutomotiveDevConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AutomotiveDevConfigList contains a list of AutomotiveDevConfig
type AutomotiveDevConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AutomotiveDevConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AutomotiveDevConfig{}, &AutomotiveDevConfigList{})
}
