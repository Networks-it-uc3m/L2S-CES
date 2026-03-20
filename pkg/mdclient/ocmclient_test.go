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
	"testing"

	l2scesv1 "github.com/Networks-it-uc3m/l2sc-es/api/v1"
)

func TestAllocateMonitoringNodeIPs(t *testing.T) {
	clusters := []l2scesv1.OverlayCluster{
		{Name: "cluster-a"},
		{Name: "cluster-b"},
		{Name: "cluster-c"},
	}

	allocated, err := allocateMonitoringNodeIPs("10.42.0.0/29", clusters)
	if err != nil {
		t.Fatalf("allocateMonitoringNodeIPs returned error: %v", err)
	}

	expected := map[string]string{
		"cluster-a": "10.42.0.1",
		"cluster-b": "10.42.0.2",
		"cluster-c": "10.42.0.3",
	}

	for clusterName, wantIP := range expected {
		if gotIP := allocated[clusterName]; gotIP != wantIP {
			t.Fatalf("cluster %s got IP %s, want %s", clusterName, gotIP, wantIP)
		}
	}
}

func TestAllocateMonitoringNodeIPsRejectsTooSmallCIDR(t *testing.T) {
	clusters := []l2scesv1.OverlayCluster{
		{Name: "cluster-a"},
		{Name: "cluster-b"},
		{Name: "cluster-c"},
	}

	if _, err := allocateMonitoringNodeIPs("10.42.0.0/30", clusters); err == nil {
		t.Fatalf("expected allocation to fail for undersized CIDR")
	}
}

func TestAllocateMonitoringNodeIPsRejectsDuplicateClusterNames(t *testing.T) {
	clusters := []l2scesv1.OverlayCluster{
		{Name: "cluster-a"},
		{Name: "cluster-a"},
	}

	if _, err := allocateMonitoringNodeIPs("10.42.0.0/29", clusters); err == nil {
		t.Fatalf("expected allocation to fail for duplicate cluster names")
	}
}
