/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	discoveryv1alpha1 "github.com/yndd/discovery-operator/api/v1alpha1"
	discoveryrules "github.com/yndd/discovery-operator/discovery/discovery_rules"
	"github.com/yndd/ndd-runtime/pkg/logging"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

func NewReconciler() *DiscoveryRuleReconciler {
	return &DiscoveryRuleReconciler{
		m:              new(sync.Mutex),
		discoveryRules: make(map[string]discoveryrules.DiscoveryRule),
	}
}

// DiscoveryRuleReconciler reconciles a DiscoveryRule object
type DiscoveryRuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Logger         logging.Logger
	m              *sync.Mutex
	discoveryRules map[string]discoveryrules.DiscoveryRule
}

//+kubebuilder:rbac:groups=discovery.yndd.io,resources=discoveryrules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=discovery.yndd.io,resources=discoveryrules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=discovery.yndd.io,resources=discoveryrules/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DiscoveryRule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DiscoveryRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.WithValues("request", req)
	logger.Info("reconciling")

	drFullName := fmt.Sprintf("%s/%s", req.Namespace, req.Name)
	dr := new(discoveryv1alpha1.DiscoveryRule)

	err := r.Client.Get(ctx, req.NamespacedName, dr)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Debug("discoveryRule not found")
			r.m.Lock()
			if oldDR, ok := r.discoveryRules[drFullName]; ok {
				oldDR.Stop()
				delete(r.discoveryRules, drFullName)
			}
			r.m.Unlock()
			return ctrl.Result{}, nil
		}
		logger.Debug("could not get discoveryRule", "error", err)
	}
	drFullName = fmt.Sprintf("%s/%s", dr.GetNamespace(), dr.GetName())
	logger = r.Logger.WithValues("discovery-rule", drFullName)
	logger.Info("")

	r.m.Lock()
	defer r.m.Unlock()
	if eDR, ok := r.discoveryRules[drFullName]; ok {
		if !dr.Spec.Enabled {
			eDR.Stop()
			delete(r.discoveryRules, drFullName)
		}
		return ctrl.Result{}, nil
	}
	// run discovery rule,
	drInit, ok := discoveryrules.DiscoveryRules[dr.Spec.Type]
	if !ok {
		logger.Info("unknown discovery-rule type", "type", dr.Spec.Type)
		return reconcile.Result{}, fmt.Errorf("unknown discovery rule type %s", dr.Spec.Type)
	}
	drule := drInit()
	r.discoveryRules[drFullName] = drule
	go drule.Run(context.TODO(), dr, discoveryrules.WithLogger(logger), discoveryrules.WithClient(r.Client))

	// update discovery rule start time
	dr.Status.StartTime = time.Now().UnixNano()
	err = r.Client.Status().Update(ctx, dr)
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *DiscoveryRuleReconciler) SetupWithManager(mgr ctrl.Manager, o controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.DiscoveryRule{}).
		Owns(&discoveryv1alpha1.DiscoveryRule{}).
		WithEventFilter(predicate.Funcs{
			// CreateFunc: func(event.CreateEvent) bool { return true },
			UpdateFunc: func(e event.UpdateEvent) bool {
				// Generation is only updated on spec changes (also on deletion),
				// not metadata or status
				// Filter out events where the generation hasn't changed to
				// avoid being triggered by status updates
				return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
			},
			// DeleteFunc:  func(event.DeleteEvent) bool { return true },
			// GenericFunc: func(event.GenericEvent) bool { return true },
		}).
		WithOptions(o).
		Complete(r)
}
