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

	l2scesv1 "github.com/Networks-it-uc3m/l2sc-es/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type RestClient struct {
	ManagerClusterConfig rest.Config
}

func (restcli *RestClient) ApplyNetwork(network *l2scesv1.SliceNetwork, namespace string) error {
	return fmt.Errorf("rest client migration pending for ApplyNetwork(%s) in namespace %s", network.Name, namespace)
}

func (restcli *RestClient) DeleteNetwork(network *l2scesv1.SliceNetwork, namespace string) error {
	return fmt.Errorf("rest client migration pending for DeleteNetwork(%s) in namespace %s", network.Name, namespace)
}

func (restcli *RestClient) ApplySlice(slice *l2scesv1.SliceOverlay, namespace string) error {
	return fmt.Errorf("rest client migration pending for ApplySlice(%s) in namespace %s", slice.Name, namespace)
}

func (restcli *RestClient) DeleteSlice(slice *l2scesv1.SliceOverlay, namespace string) error {
	return fmt.Errorf("rest client migration pending for DeleteSlice(%s) in namespace %s", slice.Name, namespace)
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
		kubeconfig := filepath.Join(absKubeconfigDirectory, configEntry.Name())
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return []rest.Config{}, fmt.Errorf("failed to build config %s from flags: %v", configEntry.Name(), err)
		}
		clusterConfigs = append(clusterConfigs, *config)
	}

	return clusterConfigs, nil
}
