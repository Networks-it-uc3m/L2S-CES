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

// client.go
//
// This is a simple client that interacts with the given gRPC server to create or delete
// a network or a slice resource based on command-line flags. We assume the server is
// running on "localhost:50051" or another address specified in the YAML config.
//
// Usage examples (assuming you have Go modules set up):
//   go run client.go --test-slice-create --config ./config.yaml
//   go run client.go --test-network-create --config ./config.yaml

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// Adjust import to point to where you keep your proto-generated code
	l2sces "github.com/Networks-it-uc3m/l2sc-es/internal/server"
)

// main is the entry point for this client application
func main() {
	// Command-line flags
	testNetworkCreate := flag.Bool("test-network-create", false, "Simulate creating a network resource")
	testNetworkDelete := flag.Bool("test-network-delete", false, "Simulate deleting a network resource")
	testSliceCreate := flag.Bool("test-slice-create", false, "Simulate creating a slice resource")
	testSliceDelete := flag.Bool("test-slice-delete", false, "Simulate deleting a slice resource")

	configPath := flag.String("config", "./test/config.yaml", "Path to YAML config file")
	namespace := flag.String("namespace", "l2sm-system", "Kubernetes namespace to place resources in")

	// Parse the command-line flags
	flag.Parse()

	// Load the YAML configuration
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create a gRPC connection
	conn, err := grpc.NewClient(cfg.ServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to server at %s: %v", cfg.ServerAddress, err)
	}
	defer conn.Close()

	// Create a client for our L2SMMultiDomainService
	client := l2sces.NewL2SMMultiDomainServiceClient(conn)
	// Build the cluster list from config
	clusters := make([]*l2sces.Cluster, 0, len(cfg.Clusters))
	fmt.Println(cfg.Clusters[0].GatewayNode)

	provider := &l2sces.Provider{
		Name:        cfg.Provider.Name,
		Domain:      cfg.Provider.Domain,
		DnsPort:     cfg.Provider.DnsPort,
		SdnPort:     cfg.Provider.SdnPort,
		DnsGrpcPort: cfg.Provider.DnsGrpcPort,
		OfPort:      cfg.Provider.OfPort,
	}

	for _, c := range cfg.Clusters {

		gatewayNode := &l2sces.Node{
			Name:      c.GatewayNode.Name,
			IpAddress: c.GatewayNode.IPAddress,
		}

		clusters = append(clusters, &l2sces.Cluster{
			Name: c.Name,
			RestConfig: &l2sces.RestConfig{
				BearerToken: c.BearerToken,
				ApiKey:      c.ApiKey,
			},
			Overlay: &l2sces.Overlay{
				// If you have pre-defined links, set them here. Otherwise they get generated automatically.
				Nodes: c.Nodes,
			},
			GatewayNode:    gatewayNode,
			PodAddressPool: c.AddressPool,
			Namespace:      c.Namespace,
		})
	}

	// Wrap actions in a context with a 10-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1) Test Slice Create
	if *testSliceCreate {
		fmt.Println("Creating Slice...")

		// Build the create request
		createSliceReq := &l2sces.CreateSliceRequest{
			Namespace: *namespace,
			Slice: &l2sces.Slice{
				Provider: provider,
				Clusters: clusters,
			},
		}

		// Call CreateSlice
		resp, err := client.CreateSlice(ctx, createSliceReq)
		if err != nil {
			log.Fatalf("Failed to create slice: %v", err)
		}
		fmt.Printf("CreateSlice response: %s\n", resp.GetMessage())
	}

	// 2) Test Slice Delete
	if *testSliceDelete {
		fmt.Println("Deleting Slice...")

		deleteSliceReq := &l2sces.DeleteSliceRequest{
			Namespace: *namespace,
			Slice: &l2sces.Slice{
				Provider: provider,
				Clusters: clusters,
				// If links are necessary for the request, fill them here as well
			},
		}

		// Call DeleteSlice
		resp, err := client.DeleteSlice(ctx, deleteSliceReq)
		if err != nil {
			log.Fatalf("Failed to delete slice: %v", err)
		}
		fmt.Printf("DeleteSlice response: %s\n", resp.GetMessage())
	}

	// 3) Test Network Create
	if *testNetworkCreate {
		fmt.Println("Creating L2Network...")

		createNetworkReq := &l2sces.CreateNetworkRequest{
			Namespace: *namespace,
			Network: &l2sces.L2Network{
				Name:     cfg.NetworkName,
				Provider: provider,
				Type:     cfg.NetworkType,
				Clusters: clusters,
				PodCidr:  "10.1.0.0/16",
			},
		}

		// Call CreateNetwork
		res, err := client.CreateNetwork(ctx, createNetworkReq)
		if err != nil {
			log.Fatalf("Failed to create network: %v", err)
		}
		fmt.Printf("CreateNetwork response: %s\n", res.GetMessage())
		fmt.Printf("Please append the following fields to your workload: %s\n", res.GetPatches())
	}

	// 4) Test Network Delete
	if *testNetworkDelete {
		fmt.Println("Deleting L2Network...")

		deleteNetworkReq := &l2sces.DeleteNetworkRequest{
			Namespace: *namespace,
			Network: &l2sces.L2Network{
				Name:     cfg.NetworkName,
				Provider: provider,
				Type:     cfg.NetworkType,
				Clusters: clusters,
			},
		}

		// Call DeleteNetwork
		res, err := client.DeleteNetwork(ctx, deleteNetworkReq)
		if err != nil {
			log.Fatalf("Failed to delete network: %v", err)
		}
		fmt.Printf("DeleteNetwork response: %s\n", res.GetMessage())
	}
}
