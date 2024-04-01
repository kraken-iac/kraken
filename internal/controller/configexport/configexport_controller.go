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

package configexport

import (
	"context"
	"fmt"
	"reflect"

	k8scorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kraken-iac/kraken/api/configexport/v1alpha1"
	corev1alpha1 "github.com/kraken-iac/kraken/api/core/v1alpha1"
)

const (
	externalResourcePrefix string = "configexport"
)

// ConfigExportReconciler reconciles a ConfigExport object
type ConfigExportReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=configexport.kraken-iac.eoinfennessy.com,resources=configexports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=configexport.kraken-iac.eoinfennessy.com,resources=configexports/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=configexport.kraken-iac.eoinfennessy.com,resources=configexports/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ConfigExport object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *ConfigExportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	log := log.FromContext(ctx)

	// Fetch ConfigExport resource
	configExport := &v1alpha1.ConfigExport{}
	if err := r.Client.Get(ctx, req.NamespacedName, configExport); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ConfigExport resource not found: Ignoring because it must have been deleted")
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to fetch ConfigExport resource: Requeuing")
			return ctrl.Result{}, err
		}
	}

	// Add initial status conditions if not present
	if configExport.Status.Conditions == nil || len(configExport.Status.Conditions) == 0 {
		log.Info("Setting initial status conditions for ConfigExport")
		meta.SetStatusCondition(
			&configExport.Status.Conditions,
			metav1.Condition{
				Type:    v1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionUnknown,
				Reason:  "Reconciling",
				Message: "Initial reconciliation",
			},
		)
	}

	// Update status on every return
	defer func() {
		if statusUpdateErr := r.Status().Update(ctx, configExport); statusUpdateErr != nil {
			log.Error(err, "Failed to update ConfigExport status")
			err = statusUpdateErr
		}
	}()

	// Construct DependencyRequest spec
	newDependencyRequestSpec := configExport.Spec.GenerateDependencyRequestSpec()

	// Fetch existing DependencyRequest if one exists
	oldDependencyRequest := &corev1alpha1.DependencyRequest{}
	var oldDependencyRequestExists bool
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{
			Name:      fmt.Sprintf("%s-%s", externalResourcePrefix, req.Name),
			Namespace: req.Namespace,
		},
		oldDependencyRequest,
	); err != nil {
		if apierrors.IsNotFound(err) {
			oldDependencyRequestExists = false
		} else {
			log.Error(err, "Failed to fetch DependencyRequest resource: Requeuing")
			return ctrl.Result{}, err
		}
	} else {
		oldDependencyRequestExists = true
	}

	// Delete old DependencyRequest if new DependencyRequest has no dependencies
	if !newDependencyRequestSpec.HasDependencies() && oldDependencyRequestExists {
		if err := r.Delete(ctx, oldDependencyRequest); err != nil {
			log.Error(err, "Failed to delete DependencyRequest")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		log.Info("Deleted DependencyRequest")
		return ctrl.Result{}, nil
	}

	if newDependencyRequestSpec.HasDependencies() {
		// Create DependencyRequest
		if !oldDependencyRequestExists {
			dependencyRequest := &corev1alpha1.DependencyRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s", externalResourcePrefix, req.Name),
					Namespace: req.Namespace,
				},
				Spec: newDependencyRequestSpec,
			}

			if err := controllerutil.SetControllerReference(
				configExport,
				dependencyRequest,
				r.Scheme); err != nil {
				log.Error(err, "Failed to set owner reference on DependencyRequest")
				return reconcile.Result{}, err
			}

			log.Info("Creating DependencyRequest", "name", dependencyRequest.ObjectMeta.Name)
			if err := r.Client.Create(ctx, dependencyRequest); err != nil {
				if apierrors.IsAlreadyExists(err) {
					log.Info("DependencyRequest already exists")
					return ctrl.Result{Requeue: true}, nil
				}
				log.Error(err, "Error creating DependencyRequest", "name", dependencyRequest.ObjectMeta.Name)
				return ctrl.Result{}, err
			}
			// TODO: Update status conditions to include DependencyRequest info
			return ctrl.Result{}, nil
		}

		// Update DependencyRequest if old and new specs are different
		if !reflect.DeepEqual(oldDependencyRequest.Spec, newDependencyRequestSpec) {
			updatedDependencyRequest := oldDependencyRequest.DeepCopy()
			updatedDependencyRequest.Spec = newDependencyRequestSpec

			log.Info("Updating DependencyRequest", "name", updatedDependencyRequest.ObjectMeta.Name)
			if err := r.Client.Update(ctx, updatedDependencyRequest); err != nil {
				log.Error(err, "Could not update DependencyRequest", "name", updatedDependencyRequest.ObjectMeta.Name)
				return ctrl.Result{}, err
			}
			// TODO: Update status conditions to include DependencyRequest info
			return ctrl.Result{}, nil
		}

		// Return without requeue if DependencyRequest is not ready
		if !meta.IsStatusConditionTrue(
			oldDependencyRequest.Status.Conditions,
			corev1alpha1.ConditionTypeReady,
		) {
			log.Info("DependencyRequest is not yet ready")
			dependencyRequestCondition := meta.FindStatusCondition(
				oldDependencyRequest.Status.Conditions,
				corev1alpha1.ConditionTypeReady,
			)
			if dependencyRequestCondition == nil {
				log.Info("DependencyRequest's status conditions are not yet set")
				return ctrl.Result{Requeue: true}, nil
			}
			meta.SetStatusCondition(
				&configExport.Status.Conditions,
				metav1.Condition{
					Type:    v1alpha1.ConditionTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "MissingDependency",
					Message: dependencyRequestCondition.Message,
				},
			)
			return ctrl.Result{}, nil
		}
	}

	// Construct applicableValues object containing the actual values that will be applied
	av, err := configExport.Spec.ToApplicableValues(oldDependencyRequest.Status.DependentValues)
	if err != nil {
		log.Error(err, "Could not construct applicable values")
		meta.SetStatusCondition(
			&configExport.Status.Conditions,
			metav1.Condition{
				Type:    v1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "DependencyError",
				Message: fmt.Sprintf("Could not construct applicable values: %s", err),
			},
		)
		return ctrl.Result{}, nil
	}

	// Create or update ConfigMap/Secret
	switch configExport.Spec.ConfigType {
	case v1alpha1.ConfigTypeConfigMap:
		if _, err := r.createOrUpdateConfigMap(ctx, configExport, av); err != nil {
			meta.SetStatusCondition(
				&configExport.Status.Conditions,
				metav1.Condition{
					Type:    v1alpha1.ConditionTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "ConfigMapExportError",
					Message: fmt.Sprintf("Failed to create/update ConfigMap: %s", err),
				},
			)
		}
	case v1alpha1.ConfigTypeSecret:
		if _, err := r.createOrUpdateSecret(ctx, configExport, av); err != nil {
			meta.SetStatusCondition(
				&configExport.Status.Conditions,
				metav1.Condition{
					Type:    v1alpha1.ConditionTypeReady,
					Status:  metav1.ConditionFalse,
					Reason:  "SecretExportError",
					Message: fmt.Sprintf("Failed to create/update Secret: %s", err),
				},
			)
		}
	default:
		err := fmt.Errorf("configType \"%s\" is not valid", configExport.Spec.ConfigType)
		log.Error(err, "invalid configType")
		meta.SetStatusCondition(
			&configExport.Status.Conditions,
			metav1.Condition{
				Type:    v1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  "ConfigTypeError",
				Message: fmt.Sprintf("Invalid spec.configType: %s", err),
			},
		)
	}

	meta.SetStatusCondition(
		&configExport.Status.Conditions,
		metav1.Condition{
			Type:    v1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  "Reconciled",
			Message: "Config has been successfully exported",
		},
	)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigExportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ConfigExport{}).
		Owns(&corev1alpha1.DependencyRequest{}).
		Owns(&k8scorev1.ConfigMap{}).
		Owns(&k8scorev1.Secret{}).
		Complete(r)
}

func (r *ConfigExportReconciler) createOrUpdateConfigMap(
	ctx context.Context,
	configExport *v1alpha1.ConfigExport,
	data map[string]string,
) (*controllerutil.OperationResult, error) {
	log := ctrl.LoggerFrom(ctx)

	cm := &k8scorev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configExport.Spec.ConfigMeta.Name,
			Namespace: configExport.Spec.ConfigMeta.Namespace,
		},
	}

	if err := controllerutil.SetControllerReference(
		configExport,
		cm,
		r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference on ConfigMap")
		return nil, err
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm,
		func() error {
			cm.Data = data
			return nil
		},
	)
	if err != nil {
		log.Error(err, "Failed to create or update ConfigMap")
		return nil, err
	}
	log.Info("Created/updated ConfigMap", "operationResult", string(result))
	return &result, nil
}

func (r *ConfigExportReconciler) createOrUpdateSecret(
	ctx context.Context,
	configExport *v1alpha1.ConfigExport,
	data map[string]string,
) (*controllerutil.OperationResult, error) {
	log := ctrl.LoggerFrom(ctx)

	s := &k8scorev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configExport.Spec.ConfigMeta.Name,
			Namespace: configExport.Spec.ConfigMeta.Namespace,
		},
	}

	if err := controllerutil.SetControllerReference(
		configExport,
		s,
		r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference on Secret")
		return nil, err
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, s,
		func() error {
			s.StringData = data
			return nil
		},
	)
	if err != nil {
		log.Error(err, "Failed to create or update Secret")
		return nil, err
	}
	log.Info("Created/updated Secret", "operationResult", string(result))
	return &result, nil
}
