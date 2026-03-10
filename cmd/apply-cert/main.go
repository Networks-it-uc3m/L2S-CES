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

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Networks-it-uc3m/l2sc-es/pkg/operator"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Usage: program [options] <certificate_file_path>")
		os.Exit(1)
	}

	// Parse command-line flags
	var kubeconfig *string
	var namespace *string
	var clusterName *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	namespace = flag.String("namespace", "default", "Kubernetes namespace")
	clusterName = flag.String("clustername", "test", "Cluster name")
	flag.Parse()

	// Get the certificate file path from the last argument
	certificateFilePath := flag.Arg(0)
	if certificateFilePath == "" {
		fmt.Println("Certificate file path is required as the last argument.")
		os.Exit(1)
	}

	// Read the certificate data from the file
	certificate, err := os.ReadFile(certificateFilePath)
	if err != nil {
		fmt.Printf("Failed to read certificate file: %v\n", err)
		os.Exit(1)
	}

	// Build Kubernetes client configuration
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}

	// Apply the secret
	err = operator.ApplyCertificateSecrets(config, *namespace, *clusterName, certificate)
	if err != nil {
		panic(err)
	}

	fmt.Println("Secret applied successfully.")
}
