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

// Package metrics provides custom Prometheus metrics for the DPU operator
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	// Metric namespace
	namespace = "dpu_operator"

	// Label names
	labelVendor     = "vendor"
	labelModel      = "model"
	labelPlugin     = "plugin"
	labelNode       = "node"
	labelController = "controller"
	labelResult     = "result"
	labelCapability = "capability"
)

var (
	// PluginRegistrations tracks the number of registered plugins
	PluginRegistrations = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "plugins_registered_total",
			Help:      "Total number of registered plugins by vendor",
		},
		[]string{labelVendor, labelPlugin},
	)

	// DevicesDiscovered tracks the number of discovered DPU devices
	DevicesDiscovered = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "devices_discovered_total",
			Help:      "Total number of discovered DPU devices",
		},
		[]string{labelVendor, labelModel, labelNode},
	)

	// DeviceHealthStatus tracks device health (1=healthy, 0=unhealthy)
	DeviceHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "device_health_status",
			Help:      "Health status of DPU devices (1=healthy, 0=unhealthy)",
		},
		[]string{labelVendor, labelModel, labelNode, "device_id"},
	)

	// ReconciliationDuration tracks reconciliation loop duration
	ReconciliationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "reconciliation_duration_seconds",
			Help:      "Duration of reconciliation loops in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{labelController},
	)

	// ReconciliationErrors tracks reconciliation errors
	ReconciliationErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "reconciliation_errors_total",
			Help:      "Total number of reconciliation errors",
		},
		[]string{labelController, "error_type"},
	)

	// ReconciliationTotal tracks total reconciliation attempts
	ReconciliationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "reconciliation_total",
			Help:      "Total number of reconciliation attempts",
		},
		[]string{labelController, labelResult},
	)

	// OPIBridgeLatency tracks latency of OPI bridge API calls
	OPIBridgeLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "opi_bridge_latency_seconds",
			Help:      "Latency of OPI bridge API calls in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{labelVendor, "operation"},
	)

	// OPIBridgeErrors tracks OPI bridge API errors
	OPIBridgeErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "opi_bridge_errors_total",
			Help:      "Total number of OPI bridge API errors",
		},
		[]string{labelVendor, "operation", "error_code"},
	)

	// PluginCapabilities tracks available plugin capabilities
	PluginCapabilities = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "plugin_capabilities",
			Help:      "Available plugin capabilities (1=available, 0=not available)",
		},
		[]string{labelPlugin, labelCapability},
	)

	// DeviceInventoryInfo provides device inventory metadata
	DeviceInventoryInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "device_inventory_info",
			Help:      "Device inventory metadata (always 1, use labels for info)",
		},
		[]string{labelVendor, labelModel, "device_id", "firmware_version", "serial_number", "pci_address"},
	)

	// NetworkOperations tracks network operations performed
	NetworkOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "network_operations_total",
			Help:      "Total number of network operations performed",
		},
		[]string{labelVendor, "operation", labelResult},
	)

	// StorageOperations tracks storage operations performed
	StorageOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "storage_operations_total",
			Help:      "Total number of storage operations performed",
		},
		[]string{labelVendor, "operation", labelResult},
	)
)

func init() {
	// Register all custom metrics with the controller-runtime metrics registry
	metrics.Registry.MustRegister(
		PluginRegistrations,
		DevicesDiscovered,
		DeviceHealthStatus,
		ReconciliationDuration,
		ReconciliationErrors,
		ReconciliationTotal,
		OPIBridgeLatency,
		OPIBridgeErrors,
		PluginCapabilities,
		DeviceInventoryInfo,
		NetworkOperations,
		StorageOperations,
	)
}

// RecordPluginRegistration records a plugin registration event
func RecordPluginRegistration(vendor, plugin string) {
	PluginRegistrations.WithLabelValues(vendor, plugin).Set(1)
}

// RecordDeviceDiscovery records a device discovery event
func RecordDeviceDiscovery(vendor, model, node string, count int) {
	DevicesDiscovered.WithLabelValues(vendor, model, node).Set(float64(count))
}

// RecordDeviceHealth records device health status
func RecordDeviceHealth(vendor, model, node, deviceID string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	DeviceHealthStatus.WithLabelValues(vendor, model, node, deviceID).Set(value)
}

// classifyError classifies errors into fixed categories to avoid unbounded cardinality in metrics
func classifyError(err error) string {
	if err == nil {
		return "none"
	}

	// Check for Kubernetes API errors first
	switch {
	case apierrors.IsNotFound(err):
		return "not_found"
	case apierrors.IsAlreadyExists(err):
		return "already_exists"
	case apierrors.IsConflict(err):
		return "conflict"
	case apierrors.IsInvalid(err):
		return "invalid"
	case apierrors.IsTimeout(err):
		return "timeout"
	case apierrors.IsServerTimeout(err):
		return "server_timeout"
	case apierrors.IsServiceUnavailable(err):
		return "service_unavailable"
	case apierrors.IsTooManyRequests(err):
		return "rate_limited"
	case apierrors.IsUnauthorized(err):
		return "unauthorized"
	case apierrors.IsForbidden(err):
		return "forbidden"
	case apierrors.IsBadRequest(err):
		return "bad_request"
	case apierrors.IsInternalError(err):
		return "internal_error"
	default:
		return "unknown"
	}
}

// RecordReconciliation records reconciliation metrics
func RecordReconciliation(controller string, duration float64, err error) {
	ReconciliationDuration.WithLabelValues(controller).Observe(duration)

	result := "success"
	if err != nil {
		result = "error"
		ReconciliationErrors.WithLabelValues(controller, classifyError(err)).Inc()
	}
	ReconciliationTotal.WithLabelValues(controller, result).Inc()
}

// RecordOPICall records OPI bridge API call metrics
func RecordOPICall(vendor, operation string, duration float64, err error) {
	OPIBridgeLatency.WithLabelValues(vendor, operation).Observe(duration)

	if err != nil {
		errorCode := "unknown"
		// You could parse gRPC error codes here
		OPIBridgeErrors.WithLabelValues(vendor, operation, errorCode).Inc()
	}
}

// RecordPluginCapability records available plugin capabilities
func RecordPluginCapability(plugin, capability string, available bool) {
	value := 0.0
	if available {
		value = 1.0
	}
	PluginCapabilities.WithLabelValues(plugin, capability).Set(value)
}

// RecordDeviceInventory records device inventory information
func RecordDeviceInventory(vendor, model, deviceID, firmwareVersion, serialNumber, pciAddress string) {
	DeviceInventoryInfo.WithLabelValues(vendor, model, deviceID, firmwareVersion, serialNumber, pciAddress).Set(1)
}

// RecordNetworkOperation records a network operation
func RecordNetworkOperation(vendor, operation string, success bool) {
	result := "success"
	if !success {
		result = "error"
	}
	NetworkOperations.WithLabelValues(vendor, operation, result).Inc()
}

// RecordStorageOperation records a storage operation
func RecordStorageOperation(vendor, operation string, success bool) {
	result := "success"
	if !success {
		result = "error"
	}
	StorageOperations.WithLabelValues(vendor, operation, result).Inc()
}
