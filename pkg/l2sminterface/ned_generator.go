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
	"net"

	l2smv1 "github.com/Networks-it-uc3m/L2S-M/api/v1"
	"github.com/Networks-it-uc3m/l2sc-es/internal/env"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const SWITCH_DOCKER_IMAGE = "alexdecb/l2sm-switch:1.2.9"

type NEDValues struct {
	ClusterName    string
	NodeConfig     *NodeConfig
	Neighbors      []Neighbor
	MonitoredNodes map[string]net.IPNet
}

type SDNController struct {
	Name        string
	Domain      string
	SDNPort     string
	DNSPort     string
	OFPort      string
	DNSGRPCPort string
}

type NodeConfig struct {
	NodeName  string
	IPAddress string
}

type Neighbor struct {
	Node   string
	Domain string
}

type NEDGenerator struct {
	SliceName  string
	Provider   SDNController
	Monitoring *l2smv1.MonitorSpec
}

func NewNEDGenerator(sdnController SDNController) *NEDGenerator {
	sdnPort := sdnController.SDNPort
	dnsPort := sdnController.DNSPort
	ofPort := sdnController.OFPort
	dnsGRPCPort := sdnController.DNSGRPCPort

	if sdnPort == "" {
		sdnPort = env.GetDefaultSDNPort()
	}
	if dnsGRPCPort == "" {
		dnsGRPCPort = env.GetDefaultDNSGRPCPort()
	}
	if dnsPort == "" {
		dnsPort = env.GetDefaultDNSPort()
	}
	if ofPort == "" {
		ofPort = env.GetDefaultOFPort()

	}
	return &NEDGenerator{
		SliceName: sdnController.Name,
		Provider: SDNController{
			Name:        sdnController.Name,
			Domain:      sdnController.Domain,
			SDNPort:     sdnPort,
			DNSGRPCPort: dnsGRPCPort,
			DNSPort:     dnsPort,
			OFPort:      ofPort,
		}}
}
func (nedGenerator *NEDGenerator) ConstructNED(nedValues NEDValues) *l2smv1.NetworkEdgeDevice {
	var nodeConf *l2smv1.NodeConfigSpec
	if nedValues.NodeConfig != nil {
		nodeConf = &l2smv1.NodeConfigSpec{
			NodeName:  nedValues.NodeConfig.NodeName,
			IPAddress: nedValues.NodeConfig.IPAddress,
		}
	}
	lpmIp, monitored := nedValues.MonitoredNodes[nedValues.ClusterName]

	neighbors := make([]l2smv1.NeighborSpec, len(nedValues.Neighbors))
	for i := range neighbors {
		neighbors[i].Node = nedValues.Neighbors[i].Node
		neighbors[i].Domain = nedValues.Neighbors[i].Domain
		if monitored {
			if neighLPMIp, ok := nedValues.MonitoredNodes[neighbors[i].Node]; ok {
				ip := neighLPMIp.IP.String()
				neighbors[i].LpmIp = &ip
			}
		}
	}

	ned := &l2smv1.NetworkEdgeDevice{
		TypeMeta: metav1.TypeMeta{
			Kind:       GetKind(NetworkEdgeDevice),
			APIVersion: l2smv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nedGenerator.SliceName + "-ned",
		},
		Spec: l2smv1.NetworkEdgeDeviceSpec{
			Provider: &l2smv1.ProviderSpec{
				Name:        nedGenerator.Provider.Name,
				Domain:      []string{nedGenerator.Provider.Domain},
				OFPort:      nedGenerator.Provider.OFPort,
				SDNPort:     nedGenerator.Provider.SDNPort,
				DNSPort:     nedGenerator.Provider.DNSPort,
				DNSGRPCPort: nedGenerator.Provider.DNSGRPCPort,
			},
			NodeConfig: nodeConf,
			Neighbors:  neighbors,
		},
	}

	if monitored {
		ned.Spec.Monitor = nedGenerator.Monitoring.DeepCopy()
		// as ip, we take the ip cidr entirely, 10.0.0.4/24 for instance
		ip := lpmIp.String()
		ned.Spec.Monitor.IpCIDR = &ip
	}

	return ned
}
