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
	"reflect"

	configv1 "github.com/openshift/dpu-operator/api/v1"
	"github.com/openshift/dpu-operator/pkgs/vars"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// DataProcessingUnitConfigReconciler reconciles a DataProcessingUnitConfig object
type DataProcessingUnitConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=config.openshift.io,resources=dataprocessingunitconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.openshift.io,resources=dataprocessingunitconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=config.openshift.io,resources=dataprocessingunitconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DataProcessingUnitConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *DataProcessingUnitConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	cfg := &configv1.DataProcessingUnitConfig{}
	if err := r.Get(ctx, req.NamespacedName, cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	selector := labels.Everything()
	if cfg.Spec.DpuSelector != nil {
		parsed, err := metav1.LabelSelectorAsSelector(cfg.Spec.DpuSelector)
		if err != nil {
			logger.Error(err, "Invalid DPU selector")
			return ctrl.Result{}, err
		}
		selector = parsed
	}

	dpuList := &configv1.DataProcessingUnitList{}
	if err := r.List(ctx, dpuList); err != nil {
		logger.Error(err, "Failed to list DPUs")
		return ctrl.Result{}, err
	}

	annotationKey := fmt.Sprintf("dpu.config.openshift.io/config-%s", cfg.Name)
	vfKey := vars.DpuConfigVFCountAnnotationPrefix + cfg.Name

	matchedNames := make([]string, 0, len(dpuList.Items))

	for i := range dpuList.Items {
		dpu := &dpuList.Items[i]
		matches := selector.Matches(labels.Set(dpu.Labels))
		changed := false

		if dpu.Annotations == nil {
			dpu.Annotations = map[string]string{}
		}

		if matches {
			matchedNames = append(matchedNames, dpu.Name)
			if dpu.Annotations[annotationKey] != "true" {
				dpu.Annotations[annotationKey] = "true"
				changed = true
			}
			if cfg.Spec.VfCount != nil {
				vfValue := fmt.Sprintf("%d", *cfg.Spec.VfCount)
				if dpu.Annotations[vfKey] != vfValue {
					dpu.Annotations[vfKey] = vfValue
					changed = true
				}
			} else if _, exists := dpu.Annotations[vfKey]; exists {
				delete(dpu.Annotations, vfKey)
				changed = true
			}
		} else {
			if _, exists := dpu.Annotations[annotationKey]; exists {
				delete(dpu.Annotations, annotationKey)
				changed = true
			}
			if _, exists := dpu.Annotations[vfKey]; exists {
				delete(dpu.Annotations, vfKey)
				changed = true
			}
		}

		if changed {
			if err := r.Update(ctx, dpu); err != nil {
				logger.Error(err, "Failed to update DPU annotations", "dpu", dpu.Name)
				return ctrl.Result{}, err
			}
		}
	}

	statusChanged := cfg.Status.ObservedGeneration != cfg.Generation ||
		!reflect.DeepEqual(cfg.Status.MatchedDPUs, matchedNames)
	if statusChanged {
		cfg.Status.ObservedGeneration = cfg.Generation
		cfg.Status.MatchedDPUs = matchedNames
		if err := r.Status().Update(ctx, cfg); err != nil {
			logger.Error(err, "Failed to update DataProcessingUnitConfig status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DataProcessingUnitConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1.DataProcessingUnitConfig{}).
		Named("dataprocessingunitconfig").
		Complete(r)
}
