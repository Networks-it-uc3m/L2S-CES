/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	l2smv1 "github.com/Networks-it-uc3m/L2S-M/api/v1"
	l2scesv1 "github.com/Networks-it-uc3m/l2sc-es/api/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultProviderProfileName = "default"

func resolveNetworkProvider(ctx context.Context, kclient ctrlclient.Client, network *l2scesv1.SliceNetwork) (*l2smv1.ProviderSpec, error) {
	if network.Spec.Provider != nil {
		return network.Spec.Provider.DeepCopy(), nil
	}

	return resolveProviderReference(ctx, kclient, network.Namespace, network.Spec.ProviderRef)
}

func resolveOverlayProvider(ctx context.Context, kclient ctrlclient.Client, overlay *l2scesv1.SliceOverlay) (*l2smv1.ProviderSpec, error) {
	if overlay.Spec.Provider != nil {
		return overlay.Spec.Provider.DeepCopy(), nil
	}

	return resolveProviderReference(ctx, kclient, overlay.Namespace, overlay.Spec.ProviderRef)
}

func resolveProviderReference(ctx context.Context, kclient ctrlclient.Client, defaultNamespace string, ref *l2scesv1.ProviderReference) (*l2smv1.ProviderSpec, error) {
	namespace := defaultNamespace
	name := defaultProviderProfileName

	if ref != nil {
		if ref.Namespace != "" {
			namespace = ref.Namespace
		}
		if ref.Name != "" {
			name = ref.Name
		}
	}

	profile := &l2scesv1.ProviderProfile{}
	if err := kclient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, profile); err != nil {
		if ref == nil {
			return nil, fmt.Errorf("provider is not set: define spec.provider, spec.providerRef, or create ProviderProfile %s/%s: %w", namespace, name, err)
		}
		return nil, fmt.Errorf("failed to resolve providerRef %s/%s: %w", namespace, name, err)
	}

	return profile.Spec.Provider.DeepCopy(), nil
}
