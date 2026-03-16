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
