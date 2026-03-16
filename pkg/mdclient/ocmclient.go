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

package mdclient

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strings"

	l2smv1 "github.com/Networks-it-uc3m/L2S-M/api/v1"
	l2scesv1 "github.com/Networks-it-uc3m/l2sc-es/api/v1"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/l2sminterface"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/topologygenerator"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/util/retry"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
)

type OCMClient struct {
	kclient ctrlclient.Client
}

func (c *OCMClient) ApplyNetwork(network *l2scesv1.SliceNetwork, namespace string) error {
	fmt.Printf("Creating network %s", network.Name)

	namespace = utils.DefaultIfEmpty(namespace, "default")

	l2network, err := l2sminterface.ConstructL2NetworkFromL2smmd(network)
	if err != nil {
		return fmt.Errorf("failed to construct l2network: %v", err)
	}

	for _, clusterName := range network.Spec.Clusters {
		clusterNamespace := namespace

		currentNetwork := l2network.DeepCopy()
		currentNetwork.Namespace = clusterNamespace

		manifest, err := toManifest(currentNetwork)
		if err != nil {
			return fmt.Errorf("failed to convert l2network manifest for cluster %s: %v", clusterName, err)
		}

		work := newManifestWork(clusterName, currentNetwork.GetName(), manifest)
		if err := c.applyManifestWork(context.Background(), work); err != nil {
			return fmt.Errorf("failed to apply ManifestWork for network %s on cluster %s: %v", currentNetwork.GetName(), clusterName, err)
		}
	}

	return nil
}

func (c *OCMClient) DeleteNetwork(network *l2scesv1.SliceNetwork, namespace string) error {
	namespace = utils.DefaultIfEmpty(namespace, "default")
	for _, clusterName := range network.Spec.Clusters {
		workName := sanitizeManifestWorkName(network.GetName())
		work := &workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      workName,
				Namespace: clusterName,
			},
		}
		err := c.kclient.Delete(context.Background(), work)
		if err != nil && ctrlclient.IgnoreNotFound(err) != nil {
			return fmt.Errorf("error deleting ManifestWork %s in cluster namespace %s: %v", workName, clusterName, err)
		}
	}

	return nil
}

func (c *OCMClient) ApplySlice(slice *l2scesv1.SliceOverlay, namespace string) error {
	fmt.Printf("Creating slice %s", slice.Name)

	namespace = utils.DefaultIfEmpty(namespace, "he-codeco-netma")

	if slice.Spec.Topology == nil {
		return fmt.Errorf("slice overlay %s has no topology", slice.Name)
	}

	sliceClusters := slice.Spec.Topology.Nodes
	sliceLinks := slice.Spec.Topology.Links
	if len(sliceLinks) == 0 && len(sliceClusters) > 1 {
		sliceLinks = topologygenerator.GenerateTopology(getSliceClusterNames(sliceClusters))
	}
	clusterMaps := make(map[string]l2sminterface.NodeConfig, len(sliceClusters))

	for _, cluster := range sliceClusters {
		resolvedIP := ""
		if cluster.Gateway != nil {
			resolvedIP = cluster.Gateway.IPAddress
		}
		if resolvedIP == "" {
			var err error
			resolvedIP, err = c.getManagedClusterIPAddress(context.Background(), cluster.Name)
			if err != nil {
				return fmt.Errorf("failed to resolve managed cluster endpoint for %s: %v", cluster.Name, err)
			}
		}

		clusterMaps[cluster.Name] = l2sminterface.NodeConfig{
			NodeName:  cluster.Name,
			IPAddress: resolvedIP,
		}
	}

	provider := slice.Spec.Provider
	if provider == nil {
		provider = &l2smv1.ProviderSpec{}
	}
	nedGenerator := l2sminterface.NewNEDGenerator(l2sminterface.SDNController{
		Name:        provider.Name,
		Domain:      firstProviderDomain(provider),
		SDNPort:     provider.SDNPort,
		DNSPort:     provider.DNSPort,
		OFPort:      provider.OFPort,
		DNSGRPCPort: provider.DNSGRPCPort,
	})

	// monitored nodes is a map where k: node (cluster name) v: lpm ip address
	monitoredNodes := make(map[string]string)
	if slice.Spec.Monitoring != nil {
		networkCIDR := "10.0.0.0/24"
		if slice.Spec.Monitoring.NetworkCIDR != nil && *slice.Spec.Monitoring.NetworkCIDR != "" {
			networkCIDR = *slice.Spec.Monitoring.NetworkCIDR
		}
		nedGenerator.Monitoring = slice.Spec.Monitoring
		var err error
		monitoredNodes, err = allocateMonitoringNodeIPs(networkCIDR, sliceClusters)
		if err != nil {
			return fmt.Errorf("failed to allocate monitoring IPs for slice %s: %w", slice.Name, err)
		}
	}
	for _, cluster := range sliceClusters {
		clusterNamespace := namespace

		clusterNeighbors := make([]l2sminterface.Neighbor, 0, len(sliceLinks))
		for _, link := range sliceLinks {
			switch cluster.Name {
			case link.EndpointA:
				clusterNeighbors = append(clusterNeighbors, l2sminterface.Neighbor{
					Node:   clusterMaps[link.EndpointB].NodeName,
					Domain: clusterMaps[link.EndpointB].IPAddress,
				})
			case link.EndpointB:
				clusterNeighbors = append(clusterNeighbors, l2sminterface.Neighbor{
					Node:   clusterMaps[link.EndpointA].NodeName,
					Domain: clusterMaps[link.EndpointA].IPAddress,
				})
			}
		}

		ned := nedGenerator.ConstructNED(l2sminterface.NEDValues{
			Neighbors:      clusterNeighbors,
			MonitoredNodes: monitoredNodes,
			ClusterName:    cluster.Name,
		})
		ned.Namespace = clusterNamespace

		nedManifest, err := toManifest(ned)
		if err != nil {
			return fmt.Errorf("failed to convert NED manifest for cluster %s: %v", cluster.Name, err)
		}

		work := newManifestWork(cluster.Name, slice.Name, nedManifest)
		if err := c.applyManifestWork(context.Background(), work); err != nil {
			return fmt.Errorf("failed to apply ManifestWork for slice cluster %s: %v", cluster.Name, err)
		}
	}

	return nil
}

func (c *OCMClient) DeleteSlice(slice *l2scesv1.SliceOverlay, namespace string) error {
	if slice.Spec.Topology == nil {
		return nil
	}

	workName := sanitizeManifestWorkName(slice.Name)
	for _, cluster := range slice.Spec.Topology.Nodes {
		work := &workv1.ManifestWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      workName,
				Namespace: cluster.Name,
			},
		}
		err := c.kclient.Delete(context.Background(), work)
		if err != nil && ctrlclient.IgnoreNotFound(err) != nil {
			return fmt.Errorf("error deleting ManifestWork %s in cluster namespace %s: %v", workName, cluster.Name, err)
		}
	}

	return nil
}

func getSliceClusterNames(clusters []l2scesv1.OverlayCluster) []string {
	names := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		names = append(names, cluster.Name)
	}
	return names
}

func firstProviderDomain(provider *l2smv1.ProviderSpec) string {
	if provider == nil || len(provider.Domain) == 0 {
		return ""
	}
	return provider.Domain[0]
}

func (c *OCMClient) applyManifestWork(ctx context.Context, desired *workv1.ManifestWork) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &workv1.ManifestWork{}
		key := types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}
		err := c.kclient.Get(ctx, key, current)
		if err != nil {
			if ctrlclient.IgnoreNotFound(err) == nil {
				return c.kclient.Create(ctx, desired)
			}
			return err
		}

		current.Labels = desired.Labels
		current.Spec = desired.Spec
		return c.kclient.Update(ctx, current)
	})
}

func (c *OCMClient) getManagedClusterIPAddress(ctx context.Context, clusterName string) (string, error) {
	managedCluster := &clusterv1.ManagedCluster{}
	if err := c.kclient.Get(ctx, types.NamespacedName{Name: clusterName}, managedCluster); err != nil {
		return "", err
	}

	for _, config := range managedCluster.Spec.ManagedClusterClientConfigs {
		if config.URL == "" {
			continue
		}

		resolvedIP, err := resolveEndpointToIP(config.URL)
		if err == nil {
			return resolvedIP, nil
		}
	}

	return "", fmt.Errorf("managed cluster %s has no resolvable client endpoint", clusterName)
}

func resolveEndpointToIP(endpoint string) (string, error) {
	host := strings.TrimSpace(endpoint)
	if host == "" {
		return "", fmt.Errorf("endpoint is empty")
	}

	if parsedURL, err := url.Parse(host); err == nil && parsedURL.Host != "" {
		host = parsedURL.Host
	}

	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	}

	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}

	addresses, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("failed to resolve endpoint %q: %w", endpoint, err)
	}

	for _, address := range addresses {
		if ipv4 := address.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}

	if len(addresses) > 0 {
		return addresses[0].String(), nil
	}

	return "", fmt.Errorf("endpoint %q resolved without IP addresses", endpoint)
}

func allocateMonitoringNodeIPs(cidr string, clusters []l2scesv1.OverlayCluster) (map[string]string, error) {
	if len(clusters) == 0 {
		return map[string]string{}, nil
	}

	networkIP, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid monitoring CIDR %q: %w", cidr, err)
	}

	baseIPv4 := networkIP.To4()
	if baseIPv4 == nil {
		return nil, fmt.Errorf("monitoring CIDR %q is not IPv4", cidr)
	}

	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return nil, fmt.Errorf("unexpected mask size for monitoring CIDR %q", cidr)
	}

	totalAddresses := 1 << uint32(32-ones)
	if totalAddresses < 4 {
		return nil, fmt.Errorf("monitoring CIDR %q is too small or unsupported", cidr)
	}

	usableHosts := totalAddresses - 2
	if len(clusters) > usableHosts {
		return nil, fmt.Errorf(
			"monitoring CIDR %q has %d usable addresses, but slice requires %d",
			cidr,
			usableHosts,
			len(clusters),
		)
	}

	baseUint := binary.BigEndian.Uint32(baseIPv4)
	allocated := make(map[string]string, len(clusters))
	for i, cluster := range clusters {
		if cluster.Name == "" {
			return nil, fmt.Errorf("slice contains a cluster with an empty name")
		}

		if _, exists := allocated[cluster.Name]; exists {
			return nil, fmt.Errorf("slice contains duplicate cluster name %q", cluster.Name)
		}

		// Start from host offset 1 to avoid the network address.
		ip := make(net.IP, net.IPv4len)
		binary.BigEndian.PutUint32(ip, baseUint+uint32(i+1))
		allocated[cluster.Name] = ip.String()
	}

	return allocated, nil
}

func newManifestWork(clusterName, objectName string, manifests ...workv1.Manifest) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sanitizeManifestWorkName(objectName),
			Namespace: clusterName,
			Labels: map[string]string{
				"app": "l2sm",
			},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
		},
	}
}

func toManifest(obj runtime.Object) (workv1.Manifest, error) {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return workv1.Manifest{}, err
	}

	return workv1.Manifest{
		RawExtension: runtime.RawExtension{
			Object: &unstructured.Unstructured{Object: raw},
		},
	}, nil
}

func sanitizeManifestWorkName(name string) string {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) == 0 {
		return name
	}

	sanitized := strings.ToLower(name)
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")

	var builder strings.Builder
	lastDash := false
	for _, r := range sanitized {
		isAllowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if !isAllowed {
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
			continue
		}

		if r == '-' && lastDash {
			continue
		}

		builder.WriteRune(r)
		lastDash = r == '-'
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "manifestwork"
	}
	if len(result) > 63 {
		result = strings.Trim(result[:63], "-")
	}
	if result == "" {
		return "manifestwork"
	}
	return result
}
