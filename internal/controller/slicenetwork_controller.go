/*
Copyright 2024 Universidad Carlos III de Madrid

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	l2scesv1 "github.com/Networks-it-uc3m/l2sc-es/api/v1"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/mdclient"
)

const sliceNetworkFinalizer = "l2sces.l2sm.io/slicenetwork-cleanup"

// SliceNetworkReconciler reconciles a SliceNetwork object
type SliceNetworkReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	MDClient mdclient.MDClient
}

// +kubebuilder:rbac:groups=l2sces.l2sm.io,resources=slicenetworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=l2sces.l2sm.io,resources=slicenetworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=l2sces.l2sm.io,resources=slicenetworks/finalizers,verbs=update
// +kubebuilder:rbac:groups=l2sces.l2sm.io,resources=providerprofiles,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SliceNetwork object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *SliceNetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	sliceNetwork := &l2scesv1.SliceNetwork{}
	if err := r.Get(ctx, req.NamespacedName, sliceNetwork); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.MDClient == nil {
		return ctrl.Result{}, fmt.Errorf("multi-domain client is not initialized")
	}

	if !sliceNetwork.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(sliceNetwork, sliceNetworkFinalizer) {
			if err := r.MDClient.DeleteNetwork(sliceNetwork, sliceNetwork.Namespace); err != nil {
				log.Error(err, "failed to delete network", "sliceNetwork", req.NamespacedName)
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(sliceNetwork, sliceNetworkFinalizer)
			if err := r.Update(ctx, sliceNetwork); err != nil {
				return ctrl.Result{}, err
			}
		}

		log.Info("deleted slice network", "sliceNetwork", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(sliceNetwork, sliceNetworkFinalizer) {
		controllerutil.AddFinalizer(sliceNetwork, sliceNetworkFinalizer)
		if err := r.Update(ctx, sliceNetwork); err != nil {
			return ctrl.Result{}, err
		}
	}

	resolvedNetwork := sliceNetwork.DeepCopy()
	resolvedProvider, err := resolveNetworkProvider(ctx, r.Client, sliceNetwork)
	if err != nil {
		log.Error(err, "failed to resolve provider", "sliceNetwork", req.NamespacedName)
		return ctrl.Result{}, err
	}
	resolvedNetwork.Spec.Provider = resolvedProvider

	if err := r.MDClient.ApplyNetwork(resolvedNetwork, sliceNetwork.Namespace); err != nil {
		log.Error(err, "failed to apply network", "sliceNetwork", req.NamespacedName)
		return ctrl.Result{}, err
	}

	log.Info("reconciled slice network", "sliceNetwork", req.NamespacedName, "clusters", len(sliceNetwork.Spec.Clusters))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SliceNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.MDClient == nil {
		mdcli, err := newMDClient(mgr.GetConfig())
		if err != nil {
			return err
		}
		r.MDClient = mdcli
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&l2scesv1.SliceNetwork{}).
		Watches(
			&l2scesv1.ProviderProfile{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForProviderProfile),
		).
		Named("slicenetwork").
		Complete(r)
}

func (r *SliceNetworkReconciler) requestsForProviderProfile(ctx context.Context, obj client.Object) []reconcile.Request {
	profile, ok := obj.(*l2scesv1.ProviderProfile)
	if !ok {
		return nil
	}

	networkList := &l2scesv1.SliceNetworkList{}
	if err := r.List(ctx, networkList); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(networkList.Items))
	for _, network := range networkList.Items {
		if network.Spec.Provider != nil {
			continue
		}

		if network.Spec.ProviderRef == nil {
			if profile.Name != defaultProviderProfileName || profile.Namespace != network.Namespace {
				continue
			}
		} else {
			refNamespace := network.Namespace
			if network.Spec.ProviderRef.Namespace != "" {
				refNamespace = network.Spec.ProviderRef.Namespace
			}
			refName := network.Spec.ProviderRef.Name
			if refName == "" {
				refName = defaultProviderProfileName
			}
			if refNamespace != profile.Namespace || refName != profile.Name {
				continue
			}
		}

		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: network.Name, Namespace: network.Namespace},
		})
	}

	return requests
}
