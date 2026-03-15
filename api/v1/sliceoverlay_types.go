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
	l2smv1 "github.com/Networks-it-uc3m/L2S-M/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OverlayLink defines a connection between two clusters in the slice.
// This translates to "Neighbors" in the NetworkEdgeDevice.
type OverlayLink struct {
	// EndpointA is the Name of the first cluster node
	EndpointA string `json:"endpointA"`
	// EndpointB is the Name of the second cluster node
	EndpointB string `json:"endpointB"`
}

// OverlayNode defines a specific cluster participating in the slice.
type OverlayCluster struct {
	// Name of the cluster. This must match the cluster name in your kubeconfig/targeting logic.
	Name string `json:"name"`

	// Gateway is the public IP or Domain where this node's NED can be reached.
	// This maps to 'NeighborSpec.Domain' for other nodes and 'NodeConfigSpec.IPAddress' for itself.
	Gateway *l2smv1.NodeConfigSpec `json:"gateway,omitempty"`
}

// OverlayTopology defines the graph of the network.
type OverlayTopology struct {
	// List of clusters participating in this overlay
	Nodes []OverlayCluster `json:"nodes"`

	// List of connections between the clusters
	Links []OverlayLink `json:"links,omitempty"`
}

// SliceOverlaySpec defines the desired state of SliceOverlay
type SliceOverlaySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Provider contains the SDN Controller configuration (global for the slice).
	// This will be copied to every NetworkEdgeDevice.
	Provider *l2smv1.ProviderSpec `json:"provider,omitempty"`

	// SwitchTemplate describes the virtual switch pod that will be deployed in every cluster.
	// This ensures all clusters in the slice run the same switch configuration.
	SwitchTemplate *l2smv1.SwitchTemplateSpec `json:"switchTemplate,omitempty"`

	// Topology defines the clusters involved and how they connect.
	Topology *OverlayTopology `json:"topology"`

	Monitoring *l2smv1.MonitorSpec `json:"monitor,omitempty"`
}

// SliceOverlayStatus defines the observed state of SliceOverlay.
type SliceOverlayStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster

	// Conditions represent the current state of the SliceOverlay resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Number of NetworkEdgeDevices successfully deployed
	DeployedSwitches int32 `json:"deployedSwitches,omitempty"`

	// Overall health of the slice connectivity
	Phase string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="SWITCHES",type="integer",JSONPath=".status.deployedSwitches"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// SliceOverlay is the Schema for the sliceoverlays API
type SliceOverlay struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SliceOverlay
	// +required
	Spec SliceOverlaySpec `json:"spec"`

	// status defines the observed state of SliceOverlay
	// +optional
	Status SliceOverlayStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SliceOverlayList contains a list of SliceOverlay
type SliceOverlayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SliceOverlay `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SliceOverlay{}, &SliceOverlayList{})
}
