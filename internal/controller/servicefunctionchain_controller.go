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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	configv1 "github.com/openshift/dpu-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

const (
	sfcLabelKey        = "dpu.config.openshift.io/sfc"
	sfcFunctionLabel   = "dpu.config.openshift.io/network-function"
	sfcSpecHashKey     = "dpu.config.openshift.io/pod-spec-hash"
	sfcNetworksAnnoKey = "k8s.v1.cni.cncf.io/networks"
	nodeSideLabelKey   = "dpu.config.openshift.io/dpuside"
	defaultDpuNAD      = "dpunfcni-conf"
	defaultHostNAD     = "default-sriov-net"
	defaultDpuResource = "openshift.io/dpu"
)

//+kubebuilder:rbac:groups=config.openshift.io,resources=servicefunctionchains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=config.openshift.io,resources=servicefunctionchains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=config.openshift.io,resources=servicefunctionchains/finalizers,verbs=update
//+kubebuilder:rbac:groups=config.openshift.io,resources=dpuoperatorconfigs,verbs=get;list;watch

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

	desiredPods := make(map[string]*corev1.Pod)
	resourceName := r.resourceNameForSFC(ctx)
	for _, nf := range sfc.Spec.NetworkFunctions {
		pod := networkFunctionPod(sfc, nf, resourceName)
		if err := controllerutil.SetControllerReference(sfc, pod, r.Scheme); err != nil {
			logger.Error(err, "Failed to set owner reference on Pod", "pod", pod.Name)
			return ctrl.Result{}, err
		}
		if err := setPodSpecHash(pod); err != nil {
			logger.Error(err, "Failed to compute pod spec hash", "pod", pod.Name)
			return ctrl.Result{}, err
		}
		desiredPods[pod.Name] = pod
	}

	// Cleanup pods that no longer exist in spec
	existingPods := &corev1.PodList{}
	if err := r.List(ctx, existingPods,
		client.InNamespace(sfc.Namespace),
		client.MatchingLabels{sfcLabelKey: sfc.Name},
	); err != nil {
		logger.Error(err, "Failed to list existing pods for ServiceFunctionChain")
		return ctrl.Result{}, err
	}
	for i := range existingPods.Items {
		existing := &existingPods.Items[i]
		if _, ok := desiredPods[existing.Name]; !ok {
			logger.Info("Deleting stale ServiceFunctionChain pod", "pod", existing.Name)
			if err := r.Delete(ctx, existing); err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "Failed to delete stale pod", "pod", existing.Name)
				return ctrl.Result{}, err
			}
		}
	}

	for _, pod := range desiredPods {
		requeue, err := r.reconcilePod(ctx, pod)
		if err != nil {
			logger.Error(err, "Failed to reconcile pod", "pod", pod.Name)
			return ctrl.Result{}, err
		}
		if requeue {
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}
	}

	return ctrl.Result{}, nil
}

func networkFunctionPod(sfc *configv1.ServiceFunctionChain, nf configv1.NetworkFunction, resourceName string) *corev1.Pod {
	trueVar := true
	podName := fmt.Sprintf("%s-%s", sfc.Name, nf.Name)
	resourceKey := corev1.ResourceName(resourceName)

	networks := nf.Networks
	if len(networks) == 0 {
		networks = defaultNetworksForNodeSelector(sfc.Spec.NodeSelector)
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
			Namespace: sfc.Namespace,
			Labels: map[string]string{
				sfcLabelKey:      sfc.Name,
				sfcFunctionLabel: nf.Name,
			},
			Annotations: map[string]string{
				sfcNetworksAnnoKey: networkAnnotation,
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
							resourceKey: *resource.NewQuantity(dpuRequests, resource.DecimalSI),
						},
						Limits: corev1.ResourceList{
							resourceKey: *resource.NewQuantity(dpuLimits, resource.DecimalSI),
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

func defaultNetworksForNodeSelector(nodeSelector map[string]string) []string {
	if nodeSelector != nil {
		if side, ok := nodeSelector[nodeSideLabelKey]; ok {
			switch strings.ToLower(side) {
			case "dpu":
				return []string{defaultDpuNAD}
			case "dpu-host", "host":
				return []string{defaultHostNAD}
			}
		}
		if val, ok := nodeSelector["dpu"]; ok && strings.EqualFold(val, "true") {
			return []string{defaultDpuNAD}
		}
	}
	return []string{defaultHostNAD}
}

func (r *ServiceFunctionChainReconciler) resourceNameForSFC(ctx context.Context) string {
	logger := log.FromContext(ctx)
	cfg := &configv1.DpuOperatorConfig{}
	if err := r.Get(ctx, configv1.DpuOperatorConfigNamespacedName, cfg); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to get DpuOperatorConfig; using default resource name", "resourceName", defaultDpuResource)
		}
		return defaultDpuResource
	}
	if cfg.Spec.ResourceName == "" {
		return defaultDpuResource
	}
	return cfg.Spec.ResourceName
}

func (r *ServiceFunctionChainReconciler) reconcilePod(ctx context.Context, desired *corev1.Pod) (bool, error) {
	existing := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, r.Create(ctx, desired)
		}
		return false, err
	}

	desiredHash := desired.Annotations[sfcSpecHashKey]
	if desiredHash == "" {
		return false, fmt.Errorf("missing desired pod spec hash for %s", desired.Name)
	}
	if existing.Annotations == nil || existing.Annotations[sfcSpecHashKey] != desiredHash {
		// Pod spec changes require recreation.
		return true, r.Delete(ctx, existing)
	}

	updated := false
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	for k, v := range desired.Labels {
		if existing.Labels[k] != v {
			existing.Labels[k] = v
			updated = true
		}
	}
	if existing.Annotations == nil {
		existing.Annotations = map[string]string{}
	}
	for k, v := range desired.Annotations {
		if existing.Annotations[k] != v {
			existing.Annotations[k] = v
			updated = true
		}
	}

	if updated {
		return false, r.Update(ctx, existing)
	}
	return false, nil
}

func setPodSpecHash(pod *corev1.Pod) error {
	if pod == nil {
		return fmt.Errorf("pod is nil")
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	fingerprint := struct {
		Spec     corev1.PodSpec `json:"spec"`
		Networks string         `json:"networks"`
	}{
		Spec:     pod.Spec,
		Networks: pod.Annotations[sfcNetworksAnnoKey],
	}

	data, err := json.Marshal(fingerprint)
	if err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	pod.Annotations[sfcSpecHashKey] = hex.EncodeToString(sum[:])
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceFunctionChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1.ServiceFunctionChain{}).
		Complete(r)
}
