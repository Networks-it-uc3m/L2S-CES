/*
Copyright 2025.

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
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	l2scesv1 "github.com/Networks-it-uc3m/l2sc-es/api/v1"
	"github.com/Networks-it-uc3m/l2sc-es/internal/env"
	"github.com/Networks-it-uc3m/l2sc-es/pkg/mdclient"
)

const sliceOverlayFinalizer = "l2sces.l2sm.io/sliceoverlay-cleanup"

// SliceOverlayReconciler reconciles a SliceOverlay object
type SliceOverlayReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	MDClient mdclient.MDClient
}

// +kubebuilder:rbac:groups=l2sces.l2sm.io,resources=sliceoverlays,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=l2sces.l2sm.io,resources=sliceoverlays/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=l2sces.l2sm.io,resources=sliceoverlays/finalizers,verbs=update
// +kubebuilder:rbac:groups=l2sces.l2sm.io,resources=providerprofiles,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SliceOverlay object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *SliceOverlayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	sliceOverlay := &l2scesv1.SliceOverlay{}
	if err := r.Get(ctx, req.NamespacedName, sliceOverlay); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if r.MDClient == nil {
		return ctrl.Result{}, fmt.Errorf("multi-domain client is not initialized")
	}

	if !sliceOverlay.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(sliceOverlay, sliceOverlayFinalizer) {
			if err := r.MDClient.DeleteSlice(sliceOverlay, sliceOverlay.Namespace); err != nil {
				log.Error(err, "failed to delete slice", "sliceOverlay", req.NamespacedName)
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(sliceOverlay, sliceOverlayFinalizer)
			if err := r.Update(ctx, sliceOverlay); err != nil {
				return ctrl.Result{}, err
			}
		}

		log.Info("deleted slice overlay", "sliceOverlay", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(sliceOverlay, sliceOverlayFinalizer) {
		controllerutil.AddFinalizer(sliceOverlay, sliceOverlayFinalizer)
		if err := r.Update(ctx, sliceOverlay); err != nil {
			return ctrl.Result{}, err
		}
	}

	resolvedOverlay := sliceOverlay.DeepCopy()
	resolvedProvider, err := resolveOverlayProvider(ctx, r.Client, sliceOverlay)
	if err != nil {
		log.Error(err, "failed to resolve provider", "sliceOverlay", req.NamespacedName)
		return ctrl.Result{}, err
	}
	resolvedOverlay.Spec.Provider = resolvedProvider

	if err := r.MDClient.ApplySlice(resolvedOverlay, sliceOverlay.Namespace); err != nil {
		log.Error(err, "failed to apply slice", "sliceOverlay", req.NamespacedName)
		return ctrl.Result{}, err
	}

	clusterCount := 0
	if sliceOverlay.Spec.Topology != nil {
		clusterCount = len(sliceOverlay.Spec.Topology.Nodes)
	}
	log.Info("reconciled slice overlay", "sliceOverlay", req.NamespacedName, "clusters", clusterCount)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SliceOverlayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.MDClient == nil {
		mdcli, err := newMDClient(mgr.GetConfig())
		if err != nil {
			return err
		}
		r.MDClient = mdcli
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&l2scesv1.SliceOverlay{}).
		Watches(
			&l2scesv1.ProviderProfile{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForProviderProfile),
		).
		Named("sliceoverlay").
		Complete(r)
}

func newMDClient(config *rest.Config) (mdclient.MDClient, error) {
	clientType := mdclient.ClientType(env.GetMultiDomainClientType())
	return mdclient.NewClient(clientType, config)
}

func (r *SliceOverlayReconciler) requestsForProviderProfile(ctx context.Context, obj client.Object) []reconcile.Request {
	profile, ok := obj.(*l2scesv1.ProviderProfile)
	if !ok {
		return nil
	}

	overlayList := &l2scesv1.SliceOverlayList{}
	if err := r.List(ctx, overlayList); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(overlayList.Items))
	for _, overlay := range overlayList.Items {
		if overlay.Spec.Provider != nil {
			continue
		}

		if overlay.Spec.ProviderRef == nil {
			if profile.Name != defaultProviderProfileName || profile.Namespace != overlay.Namespace {
				continue
			}
		} else {
			refNamespace := overlay.Namespace
			if overlay.Spec.ProviderRef.Namespace != "" {
				refNamespace = overlay.Spec.ProviderRef.Namespace
			}
			refName := overlay.Spec.ProviderRef.Name
			if refName == "" {
				refName = defaultProviderProfileName
			}
			if refNamespace != profile.Namespace || refName != profile.Name {
				continue
			}
		}

		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: overlay.Name, Namespace: overlay.Namespace},
		})
	}

	return requests
}
