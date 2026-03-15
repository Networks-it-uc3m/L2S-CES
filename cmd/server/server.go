// Copyright 2024 Universidad Carlos III de Madrid
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"

	l2smv1 "github.com/Networks-it-uc3m/L2S-M/api/v1"
	l2scesv1 "github.com/Networks-it-uc3m/l2sc-es/api/v1"
	l2sces "github.com/Networks-it-uc3m/l2sc-es/internal/server"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type server struct {
	l2sces.UnimplementedL2SMMultiDomainServiceServer
	kclient ctrlclient.Client
}

func (s *server) CreateNetwork(ctx context.Context, req *l2sces.CreateNetworkRequest) (*l2sces.CreateNetworkResponse, error) {
	network := sliceNetworkFromProto(req.GetNetwork(), req.GetNamespace())
	if err := s.kclient.Create(ctx, network); err != nil {
		return nil, fmt.Errorf("could not create network: %v", err)
	}
	return &l2sces.CreateNetworkResponse{Message: "Network created successfully"}, nil
}

func (s *server) DeleteNetwork(ctx context.Context, req *l2sces.DeleteNetworkRequest) (*l2sces.DeleteNetworkResponse, error) {
	network := sliceNetworkFromProto(req.GetNetwork(), req.GetNamespace())
	if err := s.kclient.Delete(ctx, network); err != nil {
		return nil, fmt.Errorf("could not delete network: %v", err)
	}
	return &l2sces.DeleteNetworkResponse{Message: "Network deleted successfully"}, nil
}

func (s *server) CreateSlice(ctx context.Context, req *l2sces.CreateSliceRequest) (*l2sces.CreateSliceResponse, error) {
	slice := sliceOverlayFromProto(req.GetSlice(), req.GetNamespace())
	if err := s.kclient.Create(ctx, slice); err != nil {
		return nil, fmt.Errorf("could not create slice: %v", err)
	}
	return &l2sces.CreateSliceResponse{Message: "Slice created succesfully"}, nil
}

func (s *server) DeleteSlice(ctx context.Context, req *l2sces.DeleteSliceRequest) (*l2sces.DeleteSliceResponse, error) {
	slice := sliceOverlayFromProto(req.GetSlice(), req.GetNamespace())
	if err := s.kclient.Delete(ctx, slice); err != nil {
		return nil, fmt.Errorf("could not delete network: %v", err)
	}
	return &l2sces.DeleteSliceResponse{Message: "Slice deleted successfully"}, nil
}

func sliceNetworkFromProto(network *l2sces.L2Network, namespace string) *l2scesv1.SliceNetwork {
	sliceNetwork := &l2scesv1.SliceNetwork{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SliceNetwork",
			APIVersion: l2scesv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      network.GetName(),
			Namespace: utils.DefaultIfEmpty(namespace, "default"),
		},
		Spec: l2scesv1.SliceNetworkSpec{
			Type:     l2smv1.NetworkType(network.GetType()),
			Clusters: make([]string, 0, len(network.GetClusters())),
			Provider: providerFromProto(network.GetProvider()),
		},
	}

	for _, cluster := range network.GetClusters() {
		sliceNetwork.Spec.Clusters = append(sliceNetwork.Spec.Clusters, cluster.GetName())
	}

	return sliceNetwork
}

func sliceOverlayFromProto(slice *l2sces.Slice, namespace string) *l2scesv1.SliceOverlay {
	name := "slice"
	if provider := slice.GetProvider(); provider != nil && provider.GetName() != "" {
		name = provider.GetName()
	}

	sliceOverlay := &l2scesv1.SliceOverlay{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SliceOverlay",
			APIVersion: l2scesv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: utils.DefaultIfEmpty(namespace, "default"),
		},
		Spec: l2scesv1.SliceOverlaySpec{
			Provider: providerFromProto(slice.GetProvider()),
			Topology: &l2scesv1.OverlayTopology{
				Nodes: make([]l2scesv1.OverlayCluster, 0, len(slice.GetClusters())),
				Links: make([]l2scesv1.OverlayLink, 0, len(slice.GetLinks())),
			},
		},
	}

	for _, cluster := range slice.GetClusters() {
		node := l2scesv1.OverlayCluster{Name: cluster.GetName()}
		if cluster.GetGatewayNode() != nil {
			node.Gateway = &l2smv1.NodeConfigSpec{
				NodeName:  cluster.GetGatewayNode().GetName(),
				IPAddress: cluster.GetGatewayNode().GetIpAddress(),
			}
		}
		sliceOverlay.Spec.Topology.Nodes = append(sliceOverlay.Spec.Topology.Nodes, node)
	}

	for _, link := range slice.GetLinks() {
		sliceOverlay.Spec.Topology.Links = append(sliceOverlay.Spec.Topology.Links, l2scesv1.OverlayLink{
			EndpointA: link.GetEndpointA(),
			EndpointB: link.GetEndpointB(),
		})
	}

	return sliceOverlay
}

func providerFromProto(provider *l2sces.Provider) *l2smv1.ProviderSpec {
	if provider == nil {
		return nil
	}

	domains := []string{}
	if provider.GetDomain() != "" {
		domains = append(domains, provider.GetDomain())
	}

	return &l2smv1.ProviderSpec{
		Name:        provider.GetName(),
		Domain:      domains,
		DNSPort:     provider.GetDnsPort(),
		SDNPort:     provider.GetSdnPort(),
		OFPort:      provider.GetOfPort(),
		DNSGRPCPort: provider.GetDnsGrpcPort(),
	}
}
