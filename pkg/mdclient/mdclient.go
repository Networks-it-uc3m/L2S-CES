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
	"errors"

	l2scesv1 "github.com/Networks-it-uc3m/l2sc-es/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ClientType string

const (
	RestType ClientType = "rest"
	OCMType  ClientType = "ocm"
)

type MDClient interface {
	ApplyNetwork(network *l2scesv1.SliceNetwork, namespace string) error
	DeleteNetwork(network *l2scesv1.SliceNetwork, namespace string) error
	ApplySlice(slice *l2scesv1.SliceOverlay, namespace string) error
	DeleteSlice(slice *l2scesv1.SliceOverlay, namespace string) error
}

func NewClient(clientType ClientType, config ...interface{}) (MDClient, error) {

	switch clientType {
	case RestType:
		clusterConfig := rest.Config{}
		// Convert each element in the config slice to rest.Config
		for _, cfg := range config {
			// Assert that cfg is of type rest.Config
			if c, ok := cfg.(*rest.Config); ok {
				clusterConfig = *c
			}
		}
		client := &RestClient{ManagerClusterConfig: clusterConfig}
		return client, nil
	case OCMType:
		var clusterConfig *rest.Config
		for _, cfg := range config {
			if c, ok := cfg.(*rest.Config); ok {
				clusterConfig = c
				break
			}
		}
		if clusterConfig == nil {
			return nil, errors.New("ocm client requires a hub rest config")
		}

		scheme := runtime.NewScheme()
		if err := clusterv1.Install(scheme); err != nil {
			return nil, err
		}
		if err := workv1.Install(scheme); err != nil {
			return nil, err
		}

		kclient, err := ctrlclient.New(clusterConfig, ctrlclient.Options{Scheme: scheme})
		if err != nil {
			return nil, err
		}

		client := &OCMClient{kclient: kclient}
		return client, nil
	default:
		return nil, errors.New("unsupported client type")
	}
}
