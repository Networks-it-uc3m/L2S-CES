/*
Copyright 2026.

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
	l2smv1 "github.com/Networks-it-uc3m/L2S-M/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderReference identifies an externally managed provider profile.
type ProviderReference struct {
	// Name is the name of the referenced ProviderProfile.
	Name string `json:"name"`

	// Namespace is the namespace of the referenced ProviderProfile.
	// When omitted, the namespace of the referencing resource is used.
	Namespace string `json:"namespace,omitempty"`
}

// ProviderProfileSpec defines the desired state of ProviderProfile.
type ProviderProfileSpec struct {
	// Provider contains the provider configuration shared by SliceNetwork and SliceOverlay.
	Provider l2smv1.ProviderSpec `json:"provider"`
}

// ProviderProfileStatus defines the observed state of ProviderProfile.
type ProviderProfileStatus struct {
	// Conditions represent the current state of the ProviderProfile resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ProviderProfile is the Schema for shared provider configuration.
type ProviderProfile struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ProviderProfile
	// +required
	Spec ProviderProfileSpec `json:"spec"`

	// status defines the observed state of ProviderProfile
	// +optional
	Status ProviderProfileStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ProviderProfileList contains a list of ProviderProfile
type ProviderProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ProviderProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProviderProfile{}, &ProviderProfileList{})
}
