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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"context"

	l2smv1 "github.com/Networks-it-uc3m/L2S-M/api/v1"

	"github.com/Networks-it-uc3m/l2sc-es/api/v1/l2sces"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/l2sminterface"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/operator"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/topologygenerator"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type RestClient struct {
	ManagerClusterConfig rest.Config
}

func (restcli *RestClient) ApplyNetwork(network *l2sces.L2Network, namespace string) error {

	fmt.Printf("Creating network %s", network.GetName())
	namespace = utils.DefaultIfEmpty(namespace, "default")

	l2network, err := l2sminterface.ConstructL2NetworkFromL2smmd(network)

	if err != nil {
		return fmt.Errorf("failed to construct l2network: %v", err)
	}

	var l2networkArray *l2smv1.L2NetworkList

	if network.GetPodCidr() != "" {
		l2networkArray, err = l2sminterface.ApplyCIDRs(network.PodCidr, *l2network, len(network.Clusters))
		if err != nil {
			return fmt.Errorf("failed to apply cidr configs to l2network: %v", err)
		}

	}

	// creates the in-cluster config

	clusterCrts, err := operator.GetClusterCertificates(&restcli.ManagerClusterConfig)

	if err != nil {
		return fmt.Errorf("could not get cluster certificates error: %v", err)
	}

	for index, cluster := range network.Clusters {

		var unstructuredL2network map[string]interface{}
		if network.GetPodCidr() != "" {
			l2network = &l2networkArray.Items[index]

		}

		if cluster.GetPodAddressPool() != "" {
			l2network.Spec.PodAddressRange = cluster.GetPodAddressPool()
		}

		unstructuredL2network, err = runtime.DefaultUnstructuredConverter.ToUnstructured(l2network)

		if err != nil {
			return fmt.Errorf("failed to assign unstructured l2network: %v", err)
		}

		unstructuredObj := &unstructured.Unstructured{Object: unstructuredL2network}

		clusterConfig := &rest.Config{Host: cluster.RestConfig.ApiKey, BearerToken: cluster.RestConfig.BearerToken,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: false, // Set to true if self-signed certs are acceptable
				CAData:   clusterCrts[cluster.Name],
			},
		}
		dynClient, err := dynamic.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("error contacting cluster %s: %v", clusterConfig.String(), err)
		}

		resource := l2sminterface.GetGVR(l2sminterface.L2Network)

		namespace = utils.DefaultIfEmpty(cluster.Namespace, namespace)

		_, err = dynClient.Resource(resource).Namespace(namespace).Create(context.Background(), unstructuredObj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating resource: %v", err)
		}

	}

	return nil
}

func (restcli *RestClient) DeleteNetwork(network *l2sces.L2Network, namespace string) error {

	clusterCrts, err := operator.GetClusterCertificates(&restcli.ManagerClusterConfig)

	if err != nil {
		return fmt.Errorf("could not get cluster certificates error: %v", err)
	}

	fmt.Printf("Deleting network %s", network.Name)
	namespace = utils.DefaultIfEmpty(namespace, "default")
	for _, cluster := range network.Clusters {
		clusterConfig := &rest.Config{Host: cluster.RestConfig.ApiKey, BearerToken: cluster.RestConfig.BearerToken,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: false, // Set to true if self-signed certs are acceptable
				CAData:   clusterCrts[cluster.Name],
			},
		}
		dynClient, err := dynamic.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("error contacting cluster %s: %v", clusterConfig.String(), err)
		}
		resource := l2sminterface.GetGVR(l2sminterface.L2Network)

		err = dynClient.Resource(resource).Namespace(namespace).Delete(context.Background(), network.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting resource: %v", err)
		}

	}

	return nil

}

func (restcli *RestClient) ApplySlice(slice *l2sces.Slice, namespace string) error {

	fmt.Printf("Creating slice %s", slice)

	namespace = utils.DefaultIfEmpty(namespace, "default")

	clusterMaps := make(map[string]l2sminterface.NodeConfig)
	sliceClusters := slice.GetClusters()
	isMultiCluster := len(sliceClusters) > 1
	sliceLinks := slice.GetLinks()

	if isMultiCluster {
		if len(sliceLinks) == 0 {
			clusterNames := make([]string, len(sliceClusters))
			for index, cluster := range sliceClusters {
				clusterNames[index] = cluster.Name
			}
			sliceLinks = topologygenerator.GenerateTopology(clusterNames)
			// fmt.Println(sliceLinks)
		}
		for _, cluster := range sliceClusters {
			clusterMaps[cluster.GetName()] = l2sminterface.NodeConfig{
				NodeName:  cluster.GetGatewayNode().GetName(),
				IPAddress: cluster.GetGatewayNode().GetIpAddress()}
		}
	}

	clusterCrts, err := operator.GetClusterCertificates(&restcli.ManagerClusterConfig)

	if err != nil {
		return fmt.Errorf("could not get cluster certificates error: %v", err)
	}

	nedGenerator := l2sminterface.NewNEDGenerator(l2sminterface.SDNController{
		Name:    slice.GetProvider().GetName(),
		Domain:  slice.GetProvider().GetDomain(),
		SDNPort: slice.GetProvider().GetSdnPort(),
		DNSPort: slice.GetProvider().GetDnsPort(),
		OFPort:  slice.GetProvider().GetOfPort(),
	})

	for _, cluster := range sliceClusters {

		clusterConfig := &rest.Config{
			Host:        cluster.RestConfig.ApiKey,
			BearerToken: cluster.RestConfig.BearerToken,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: false, // Set to true if self-signed certs are acceptable
				CAData:   clusterCrts[cluster.Name],
			},
		}
		dynClient, err := dynamic.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("error contacting cluster %s: %v", clusterConfig.String(), err)
		}

		if isMultiCluster {

			clusterNeighbors := []l2sminterface.Neighbor{}
			// fmt.Println(sliceLinks)
			for _, link := range sliceLinks {
				switch cluster.GetName() {
				case link.EndpointA:
					clusterNeighbors = append(clusterNeighbors, l2sminterface.Neighbor{
						Node:   link.GetEndpointB(),
						Domain: clusterMaps[link.EndpointB].IPAddress,
					})
				case link.EndpointB:
					clusterNeighbors = append(clusterNeighbors, l2sminterface.Neighbor{
						Node:   link.GetEndpointA(),
						Domain: clusterMaps[link.EndpointA].IPAddress,
					})
				}
			}

			ned := nedGenerator.ConstructNED(l2sminterface.NEDValues{
				NodeConfig: l2sminterface.NodeConfig{NodeName: cluster.GetGatewayNode().GetName(), IPAddress: cluster.GetGatewayNode().GetIpAddress()},
				Neighbors:  clusterNeighbors})

			unstructuredNED, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ned)

			if err != nil {
				return fmt.Errorf("failed to assign unstructured l2network: %v", err)
			}

			resource := l2sminterface.GetGVR(l2sminterface.NetworkEdgeDevice)

			// fmt.Println(unstructuredNED)
			// fmt.Println(resource)
			_, err = dynClient.Resource(resource).Namespace(namespace).Create(context.Background(), &unstructured.Unstructured{Object: unstructuredNED}, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("error creating resource: %v", err)
			}
			dynClient.Resource(resource).Namespace(namespace).Get(context.Background(), "overlay-sample", metav1.GetOptions{})
		}

		//////////////////////////////////////7
		overlay := l2sminterface.ConstructOverlayFromL2smmd(cluster.GetOverlay())
		unstructuredOverlay, err := runtime.DefaultUnstructuredConverter.ToUnstructured(overlay)
		if err != nil {
			return fmt.Errorf("failed to assign unstructured l2network: %v", err)
		}

		// fmt.Println(unstructuredOverlay)
		_, err = dynClient.Resource(l2sminterface.GetGVR(l2sminterface.Overlay)).Namespace(namespace).Create(context.Background(), &unstructured.Unstructured{Object: unstructuredOverlay}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating resource: %v", err)
		}

	}

	return nil
}

func (restcli *RestClient) DeleteSlice(slice *l2sces.Slice, namespace string) error {
	clusterCrts, err := operator.GetClusterCertificates(&restcli.ManagerClusterConfig)
	if err != nil {
		return fmt.Errorf("could not get cluster certificates error: %v", err)
	}

	fmt.Printf("Deleting slice %s", slice)
	namespace = utils.DefaultIfEmpty(namespace, "default")

	nedName := slice.GetProvider().GetName() + "-ned"

	for _, cluster := range slice.GetClusters() {
		clusterConfig := &rest.Config{
			Host:        cluster.RestConfig.ApiKey,
			BearerToken: cluster.RestConfig.BearerToken,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: false,
				CAData:   clusterCrts[cluster.Name],
			},
		}
		dynClient, err := dynamic.NewForConfig(clusterConfig)
		if err != nil {
			return fmt.Errorf("error contacting cluster %s: %v", clusterConfig.String(), err)
		}

		clusterNamespace := utils.DefaultIfEmpty(cluster.Namespace, namespace)

		nedResource := l2sminterface.GetGVR(l2sminterface.NetworkEdgeDevice)
		err = dynClient.Resource(nedResource).Namespace(clusterNamespace).Delete(context.Background(), nedName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error deleting network edge device %s in cluster %s: %v", nedName, cluster.Name, err)
		}

		overlayResource := l2sminterface.GetGVR(l2sminterface.Overlay)
		err = dynClient.Resource(overlayResource).Namespace(clusterNamespace).Delete(context.Background(), "overlay-sample", metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error deleting overlay in cluster %s: %v", cluster.Name, err)
		}
	}

	return nil
}

func GetRestConfigs(absKubeconfigDirectory string) ([]rest.Config, error) {
	kubeFiles, err := os.ReadDir(absKubeconfigDirectory)
	if err != nil {
		return []rest.Config{}, fmt.Errorf("couldn't get kube config files in %s: %v", absKubeconfigDirectory, err)

	}
	clusterConfigs, err := readKubernetesConfigs(absKubeconfigDirectory, kubeFiles)
	if err != nil {
		return []rest.Config{}, fmt.Errorf("failed to read configs in %s: %v", absKubeconfigDirectory, err)
	}
	return clusterConfigs, nil
}

func readKubernetesConfigs(absKubeconfigDirectory string, configDirectories []fs.DirEntry) ([]rest.Config, error) {

	var clusterConfigs []rest.Config

	for _, configEntry := range configDirectories {
		fmt.Println(filepath.Join(absKubeconfigDirectory, configEntry.Name()))
		kubeconfig := filepath.Join(absKubeconfigDirectory, configEntry.Name())
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return []rest.Config{}, fmt.Errorf("failed to build config %s from flags: %v", configEntry.Name(), err)
		}
		clusterConfigs = append(clusterConfigs, *config)
	}

	return clusterConfigs, nil

}
