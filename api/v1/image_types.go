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

// ImageSpec defines the desired state of Image
type ImageSpec struct {
	// Distro specifies the distribution
	Distro string `json:"distro"`

	// Target specifies the build target
	Target string `json:"target"`

	// Architecture specifies the target architecture
	Architecture string `json:"architecture"`

	// ExportFormat specifies the output format
	ExportFormat string `json:"exportFormat"`

	// Mode specifies the build mode
	Mode string `json:"mode,omitempty"`

	// Size specifies the image size information
	Size *ImageSize `json:"size,omitempty"`

	// Location defines where the image is stored
	Location ImageLocation `json:"location"`

	// Metadata contains additional information about the image
	Metadata *ImageMetadata `json:"metadata,omitempty"`

	// Tags are labels that can be used to categorize and search images
	Tags []string `json:"tags,omitempty"`

	// Description provides a human-readable description of the image
	Description string `json:"description,omitempty"`

	// Version specifies the version of this image
	Version string `json:"version,omitempty"`
}

// ImageSize contains size information about the image
type ImageSize struct {
	// CompressedBytes is the size of the compressed image in bytes
	CompressedBytes *int64 `json:"compressedBytes,omitempty"`

	// UncompressedBytes is the size of the uncompressed image in bytes
	UncompressedBytes *int64 `json:"uncompressedBytes,omitempty"`

	// VirtualBytes is the virtual disk size for disk images
	VirtualBytes *int64 `json:"virtualBytes,omitempty"`
}

// ImageLocation defines where an image is stored with support for different storage types
type ImageLocation struct {
	// Type specifies the storage type
	// +kubebuilder:validation:Enum=registry
	// +kubebuilder:default=registry
	Type string `json:"type"`

	// Registry contains configuration for container registry storage
	Registry *RegistryLocation `json:"registry,omitempty"`
}

// RegistryLocation defines storage in a container registry
type RegistryLocation struct {
	// URL is the full URL to the image in the registry (e.g., "quay.io/myorg/myimage:tag")
	URL string `json:"url"`

	// Digest is the content-addressable digest of the image
	Digest string `json:"digest,omitempty"`

	// SecretRef is the name of the secret containing registry credentials
	SecretRef string `json:"secretRef,omitempty"`
}

// ImageMetadata contains additional metadata about the image
type ImageMetadata struct {
	// CreatedBy identifies who or what created this image
	CreatedBy string `json:"createdBy,omitempty"`

	// SourceImageBuild references the ImageBuild that created this image
	SourceImageBuild string `json:"sourceImageBuild,omitempty"`

	// BuildDate is when the image was built
	BuildDate *metav1.Time `json:"buildDate,omitempty"`

	// Labels are key-value pairs for additional metadata
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are key-value pairs for additional annotations
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ImageStatus defines the observed state of Image
type ImageStatus struct {
	// Phase represents the current phase of the image (Available, Unavailable, Verifying)
	Phase string `json:"phase,omitempty"`

	// LastVerified is when the image location was last verified to be accessible
	LastVerified *metav1.Time `json:"lastVerified,omitempty"`

	// Message provides more detail about the current phase
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the image's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AccessCount tracks how many times this image has been accessed/downloaded
	AccessCount int64 `json:"accessCount,omitempty"`

	// LastAccessed is when the image was last accessed
	LastAccessed *metav1.Time `json:"lastAccessed,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Distro",type=string,JSONPath=`.spec.distro`
// +kubebuilder:printcolumn:name="Architecture",type=string,JSONPath=`.spec.architecture`
// +kubebuilder:printcolumn:name="Format",type=string,JSONPath=`.spec.exportFormat`
// +kubebuilder:printcolumn:name="Location Type",type=string,JSONPath=`.spec.location.type`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Image is the Schema for the images API
type Image struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ImageSpec   `json:"spec,omitempty"`
	Status ImageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ImageList contains a list of Image
type ImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Image `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Image{}, &ImageList{})
}
