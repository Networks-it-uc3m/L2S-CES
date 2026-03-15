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

package l2sminterface

import (
	"encoding/binary"
	"fmt"
	"net"

	l2smv1 "github.com/Networks-it-uc3m/L2S-M/api/v1"
	l2scesv1 "github.com/Networks-it-uc3m/l2sc-es/api/v1"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConstructL2NetworkFromL2smmd(network *l2scesv1.SliceNetwork) (*l2smv1.L2Network, error) {
	var provider *l2smv1.ProviderSpec
	if network.Spec.Provider != nil {
		provider = network.Spec.Provider.DeepCopy()
	}

	l2network := &l2smv1.L2Network{
		TypeMeta: metav1.TypeMeta{
			Kind:       GetKind(L2Network),
			APIVersion: l2smv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: network.Name,
		},
		Spec: l2smv1.L2NetworkSpec{
			Type:     l2smv1.NetworkType(utils.DefaultIfEmpty(string(network.Spec.Type), "vnet")),
			Provider: provider,
		},
	}
	return l2network, nil
}

func ApplyCIDRs(networkCIDR string, l2network l2smv1.L2Network, numberClusters int) (*l2smv1.L2NetworkList, error) {
	// Parse the input CIDR
	ip, ipNet, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return nil, err
	}

	// Get the current prefix length (e.g., 16 for /16)
	ones, _ := ipNet.Mask.Size()

	// Determine how many bits are needed.
	// Using the binary representation of numberClusters gives us the length.
	bitsNeeded := len(fmt.Sprintf("%b", numberClusters))
	newPrefix := ones + bitsNeeded
	if newPrefix > 32 {
		return nil, fmt.Errorf("new prefix length %d exceeds 32", newPrefix)
	}

	// Calculate the size of each new subnet (number of IP addresses)
	blockSize := uint32(1) << (32 - newPrefix)

	// Get the base IP (aligned to the original network boundary)
	baseIP := ip.Mask(ipNet.Mask)
	ipInt := binary.BigEndian.Uint32(baseIP.To4())

	// Prepare the list to hold the new L2Network entries
	networks := make([]l2smv1.L2Network, 0, numberClusters)

	// Iterate over clusters. We start at 1 so that the subnet bits become, e.g., 001, 010, etc.
	for i := 1; i <= numberClusters; i++ {
		// Compute the new network address offset by i*blockSize
		newIPInt := ipInt + uint32(i)*blockSize
		newIP := make(net.IP, 4)
		binary.BigEndian.PutUint32(newIP, newIPInt)

		// Form the new CIDR string
		newCIDR := fmt.Sprintf("%s/%d", newIP.String(), newPrefix)

		// Clone the input l2network and update its CIDR
		newL2 := l2network
		newL2.Spec.NetworkCIDR = networkCIDR
		newL2.Spec.PodAddressRange = newCIDR
		networks = append(networks, newL2)
	}

	return &l2smv1.L2NetworkList{Items: networks}, nil
}
