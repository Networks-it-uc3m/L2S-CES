/*
Copyright 2024 Universidad Carlos III de Madrid

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SliceNetworkSpec defines the desired state of SliceNetwork
type SliceClusterConfig struct {
	// Name is the name of the target cluster where the L2Network should be created.
	Name string `json:"name"`

	// Optional: You might eventually need CIDRs here to pass to the L2Network,
	// but strictly following your prompt, we are only adding Name, Type, and Provider.
}

// SliceNetworkSpec defines the desired state of SliceNetwork
type SliceNetworkSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Clusters is an array of configurations. The controller will create an L2Network
	// in each of these clusters based on the provided Name, Type, and Provider.
	// +kubebuilder:validation:MinItems=1
	Clusters []string `json:"clusters"`

	// Type specifies the type of L2Network to create in this cluster (e.g., ext-vnet, vnet, vlink).
	Type l2smv1.NetworkType `json:"type"`

	// ProviderRef references a shared ProviderProfile.
	ProviderRef *ProviderReference `json:"providerRef,omitempty"`

	// Provider defines the provider's name and domain for the network in this cluster.
	// Deprecated: prefer providerRef and ProviderProfile resources for shared configuration.
	Provider *l2smv1.ProviderSpec `json:"provider,omitempty"`
}

// SliceClusterStatus tracks the state of the L2Network in a specific cluster.
type SliceClusterStatus struct {
	// ClusterName is the name of the cluster this status refers to.
	ClusterName string `json:"clusterName"`

	// Status represents the current state of the L2Network in that cluster (e.g., "Ready", "Pending", "Error").
	Status string `json:"status,omitempty"`

	// Message provides details about the status, useful for debugging errors.
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SliceNetwork is the Schema for the slicenetworks API
type SliceNetwork struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SliceNetwork
	// +required
	Spec SliceNetworkSpec `json:"spec"`

	// status defines the observed state of SliceNetwork
	// +optional
	Status SliceNetworkStatus `json:"status,omitzero"`
}

// SliceNetworkStatus defines the observed state of SliceNetwork.
type SliceNetworkStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ClusterStatuses tracks the status of the L2Network provisioning in each defined cluster.
	// +optional
	ClusterStatuses []SliceClusterStatus `json:"clusterStatuses,omitempty"`

	// Conditions represent the current global state of the SliceNetwork resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true

// SliceNetworkList contains a list of SliceNetwork
type SliceNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SliceNetwork `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SliceNetwork{}, &SliceNetworkList{})
}
