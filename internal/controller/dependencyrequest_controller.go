/*
Copyright 2024.

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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kraken-iac/kraken/api/v1alpha1"
)

const (
	configMapDependenciesField      string = ".spec.configMapDependencies"
	krakenResourceDependenciesField string = ".spec.krakenResourceDependenciesField"

	conditionTypeReady string = "Ready"
)

// DependencyRequestReconciler reconciles a DependencyRequest object
type DependencyRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.kraken-iac.eoinfennessy.com,resources=dependencyrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.kraken-iac.eoinfennessy.com,resources=dependencyrequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.kraken-iac.eoinfennessy.com,resources=dependencyrequests/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DependencyRequest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *DependencyRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	log := log.FromContext(ctx)
	log.Info("Reconcile triggered")

	// Fetch DependencyRequest resource
	dependencyRequest := &v1alpha1.DependencyRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, dependencyRequest); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("DependencyRequest resource not found: Ignoring because it must have been deleted")
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to fetch DependencyRequest resource: Requeuing")
			return ctrl.Result{}, err
		}
	}

	// Add initial status conditions if not present
	if dependencyRequest.Status.Conditions == nil || len(dependencyRequest.Status.Conditions) == 0 {
		log.Info("Setting initial status conditions for DependencyRequest")
		meta.SetStatusCondition(
			&dependencyRequest.Status.Conditions,
			metav1.Condition{
				Type:    conditionTypeReady,
				Status:  metav1.ConditionUnknown,
				Reason:  "Reconciling",
				Message: "Initial reconciliation",
			},
		)
	}

	// Update status on every return
	defer func() {
		if statusUpdateErr := r.Status().Update(ctx, dependencyRequest); statusUpdateErr != nil {
			log.Error(err, "Failed to update DependencyRequest status")
			err = statusUpdateErr
		}
	}()

	// Reconcile ConfigMap dependencies
	if dependencyRequest.Spec.ConfigMapDependencies != nil {
		// Create new empty object to contain ConfigMap values and add it to the resource's status
		newDependentValuesFromConfigMaps := make(v1alpha1.DependentValuesFromConfigMap)
		dependencyRequest.Status.DependentValues.FromConfigMaps = newDependentValuesFromConfigMaps

		for _, cmDep := range dependencyRequest.Spec.ConfigMapDependencies {
			// Fetch ConfigMap
			cm := &corev1.ConfigMap{}
			if err := r.Get(
				ctx,
				types.NamespacedName{
					Name:      cmDep.Name,
					Namespace: req.Namespace,
				},
				cm,
			); err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("ConfigMap does not exist", "configMapName", cmDep.Name)
					meta.SetStatusCondition(
						&dependencyRequest.Status.Conditions,
						metav1.Condition{
							Type:    conditionTypeReady,
							Status:  metav1.ConditionFalse,
							Reason:  "ConfigMapNotPresent",
							Message: fmt.Sprintf("ConfigMap %s does not exist", cmDep.Name),
						},
					)
					return ctrl.Result{}, nil
				} else {
					log.Error(err, "Failed to fetch ConfigMap; Requeuing")
					meta.SetStatusCondition(
						&dependencyRequest.Status.Conditions,
						metav1.Condition{
							Type:    conditionTypeReady,
							Status:  metav1.ConditionFalse,
							Reason:  "ErrorFetchingConfigMap",
							Message: fmt.Sprintf("An error occurred fetching ConfigMap %s: %s", cmDep.Name, err),
						},
					)
					return ctrl.Result{}, err
				}
			}

			// Get value from ConfigMap data
			cmVal, exists := cm.Data[cmDep.Key]
			if !exists {
				log.Info("ConfigMap does not contain key", "configMapName", cmDep.Name, "configMapKey", cmDep.Key)
				meta.SetStatusCondition(
					&dependencyRequest.Status.Conditions,
					metav1.Condition{
						Type:    conditionTypeReady,
						Status:  metav1.ConditionFalse,
						Reason:  "ConfigMapKeyNotPresent",
						Message: fmt.Sprintf("ConfigMap %s does not contain key %s", cmDep.Name, cmDep.Key),
					},
				)
				return ctrl.Result{}, nil
			}

			// Add ConfigMap value to resource's status
			if _, exists := newDependentValuesFromConfigMaps[cmDep.Name]; !exists {
				newDependentValuesFromConfigMaps[cmDep.Name] = make(map[string]string)
			}
			newDependentValuesFromConfigMaps[cmDep.Name][cmDep.Key] = cmVal
		}
	}

	// Resource successfully reconciled
	meta.SetStatusCondition(
		&dependencyRequest.Status.Conditions,
		metav1.Condition{
			Type:    conditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  "Reconciled",
			Message: "Successfully retrieved all dependent values",
		},
	)
	log.Info("Reconciliation has been successful")
	return ctrl.Result{}, nil
}

func (r *DependencyRequestReconciler) findDependencyRequestsForConfigMap(ctx context.Context, configMap client.Object) []reconcile.Request {
	attachedDependencyRequests := &v1alpha1.DependencyRequestList{}
	listOps := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(configMapDependenciesField, configMap.GetName()),
		Namespace:     configMap.GetNamespace(),
	}
	err := r.List(ctx, attachedDependencyRequests, listOps)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(attachedDependencyRequests.Items))
	for i, item := range attachedDependencyRequests.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *DependencyRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create index for DependencyRequests on ConfigMapDependencies field
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1alpha1.DependencyRequest{},
		configMapDependenciesField,
		indexByConfigMapDependencies); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DependencyRequest{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.findDependencyRequestsForConfigMap),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func indexByConfigMapDependencies(rawDependencyRequest client.Object) []string {
	dr := rawDependencyRequest.(*v1alpha1.DependencyRequest)
	if dr.Spec.ConfigMapDependencies == nil {
		return nil
	}

	indexes := make([]string, len(dr.Spec.ConfigMapDependencies))
	for i, cm := range dr.Spec.ConfigMapDependencies {
		indexes[i] = cm.Name
	}
	return indexes
}
