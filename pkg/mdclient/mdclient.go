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

	"github.com/Networks-it-uc3m/l2sc-es/api/v1/l2sces"
	"k8s.io/client-go/rest"
)

type ClientType string

const (
	RestType ClientType = "rest"
	OCMType  ClientType = "ocm"
)

type MDClient interface {
	ApplyNetwork(network *l2sces.L2Network, namespace string) error
	DeleteNetwork(network *l2sces.L2Network, namespace string) error
	ApplySlice(slice *l2sces.Slice, namespace string) error
	DeleteSlice(slice *l2sces.Slice, namespace string) error
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
		client := &OCMClient{}
		return client, nil
	default:
		return nil, errors.New("unsupported client type")
	}
}
