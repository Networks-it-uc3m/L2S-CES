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

	"github.com/Networks-it-uc3m/l2sc-es/api/v1/l2sces"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

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

	l2Network := sliceNetworkToL2SCES(sliceNetwork)
	if !sliceNetwork.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(sliceNetwork, sliceNetworkFinalizer) {
			if err := r.MDClient.DeleteNetwork(l2Network, sliceNetwork.Namespace); err != nil {
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

	if err := r.MDClient.ApplyNetwork(l2Network, sliceNetwork.Namespace); err != nil {
		log.Error(err, "failed to apply network", "sliceNetwork", req.NamespacedName)
		return ctrl.Result{}, err
	}

	log.Info("reconciled slice network", "sliceNetwork", req.NamespacedName, "clusters", len(l2Network.GetClusters()))
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
		Named("slicenetwork").
		Complete(r)
}

func sliceNetworkToL2SCES(sliceNetwork *l2scesv1.SliceNetwork) *l2sces.L2Network {
	network := &l2sces.L2Network{
		Name:     sliceNetwork.Name,
		Provider: providerSpecToProto(sliceNetwork.Spec.Provider),
		Type:     string(sliceNetwork.Spec.Type),
		Clusters: make([]*l2sces.Cluster, 0, len(sliceNetwork.Spec.Clusters)),
	}

	for _, clusterName := range sliceNetwork.Spec.Clusters {
		network.Clusters = append(network.Clusters, &l2sces.Cluster{
			Name: clusterName,
		})
	}

	return network
}
