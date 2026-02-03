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
	"strings"

	configv1 "github.com/openshift/dpu-operator/api/v1"
	"github.com/openshift/dpu-operator/pkgs/vars"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ServiceFunctionChainReconciler reconciles a ServiceFunctionChain object
type ServiceFunctionChainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=config.openshift.io,resources=servicefunctionchains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=config.openshift.io,resources=servicefunctionchains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=config.openshift.io,resources=servicefunctionchains/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ServiceFunctionChain object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile

// +kubebuilder:rbac:groups=config.openshift.io,resources=servicefunctionchains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.openshift.io,resources=servicefunctionchains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=config.openshift.io,resources=servicefunctionchains/finalizers,verbs=update
func (r *ServiceFunctionChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	sfc := &configv1.ServiceFunctionChain{}
	if err := r.Get(ctx, req.NamespacedName, sfc); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	for _, nf := range sfc.Spec.NetworkFunctions {
		pod := networkFunctionPod(sfc, nf)
		if err := controllerutil.SetControllerReference(sfc, pod, r.Scheme); err != nil {
			logger.Error(err, "Failed to set owner reference on Pod", "pod", pod.Name)
			return ctrl.Result{}, err
		}

		if err := r.createOrUpdatePod(ctx, pod); err != nil {
			logger.Error(err, "Failed to create or update pod", "pod", pod.Name)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func networkFunctionPod(sfc *configv1.ServiceFunctionChain, nf configv1.NetworkFunction) *corev1.Pod {
	trueVar := true
	podName := fmt.Sprintf("%s-%s", sfc.Name, nf.Name)

	defaultNetworks := []string{"dpunfcni-conf", "dpunfcni-conf"}
	networks := nf.Networks
	if len(networks) == 0 {
		networks = defaultNetworks
	}
	networkAnnotation := strings.Join(networks, ", ")

	defaultDpuCount := int64(2)
	dpuRequests := defaultDpuCount
	dpuLimits := defaultDpuCount
	if nf.DpuResources != nil {
		if nf.DpuResources.Requests > 0 {
			dpuRequests = int64(nf.DpuResources.Requests)
		}
		if nf.DpuResources.Limits > 0 {
			dpuLimits = int64(nf.DpuResources.Limits)
		}
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: vars.Namespace,
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/networks": networkAnnotation,
			},
		},
		Spec: corev1.PodSpec{
			NodeSelector: sfc.Spec.NodeSelector,
			Containers: []corev1.Container{
				{
					Name:  podName,
					Image: nf.Image,
					Ports: []corev1.ContainerPort{
						{
							Name:          "web",
							ContainerPort: 8080,
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"openshift.io/dpu": *resource.NewQuantity(dpuRequests, resource.DecimalSI),
						},
						Limits: corev1.ResourceList{
							"openshift.io/dpu": *resource.NewQuantity(dpuLimits, resource.DecimalSI),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &trueVar,
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
							Add:  []corev1.Capability{"NET_RAW", "NET_ADMIN"},
						},
					},
				},
			},
		},
	}
}

func (r *ServiceFunctionChainReconciler) createOrUpdatePod(ctx context.Context, pod *corev1.Pod) error {
	existing := &corev1.Pod{}
	err := r.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, existing)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, pod)
	}
	if err != nil {
		return err
	}

	// Preserve immutable fields
	pod.ResourceVersion = existing.ResourceVersion
	pod.UID = existing.UID
	pod.CreationTimestamp = existing.CreationTimestamp

	return r.Update(ctx, pod)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceFunctionChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1.ServiceFunctionChain{}).
		Complete(r)
}
