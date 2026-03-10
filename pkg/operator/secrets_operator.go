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

package operator

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GetClusterCertificates(clusterConfig *rest.Config) (map[string][]byte, error) {

	clusterList := make(map[string][]byte)

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(clusterConfig)

	if err != nil {
		return map[string][]byte{}, err
	}

	secrets, err := clientset.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{LabelSelector: "l2sm-cert"})
	if err != nil {
		return map[string][]byte{}, err
	}
	for _, secret := range secrets.Items {
		clusterList[secret.Labels["l2sm-cert"]] = secret.Data["cert-value"]
	}

	return clusterList, nil
}
func ApplyCertificateSecrets(clusterConfig *rest.Config, namespace string, clusterName string, certificateData []byte) error {

	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return fmt.Errorf("could not create new cluster client: %v", err)
	}

	secretName := fmt.Sprintf("%s-cert", clusterName)

	// Define the desired secret state.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				"l2sm-cert": clusterName,
			},
		},
		Data: map[string][]byte{
			"cert-value": certificateData,
		},
		Type: corev1.SecretTypeOpaque,
	}

	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create secret: %v", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get secret: %v", err)
	}

	existingSecret.Labels = secret.Labels
	existingSecret.Data = secret.Data
	existingSecret.Type = secret.Type

	_, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), existingSecret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret: %v", err)
	}

	return nil
}

func CreateCertificateSecrets(clusterConfig *rest.Config, namespace string, clusterName string, certificateData []byte) error {
	return ApplyCertificateSecrets(clusterConfig, namespace, clusterName, certificateData)
}
