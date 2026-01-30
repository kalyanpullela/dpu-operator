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

package platform

import (
	"fmt"
	"strings"

	"github.com/jaypipes/ghw"
	"github.com/openshift/dpu-operator/internal/daemon/plugin"
	"github.com/openshift/dpu-operator/internal/images"
	"github.com/openshift/dpu-operator/internal/utils"
	pkgplugin "github.com/openshift/dpu-operator/pkg/plugin"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NvidiaDetector implements VendorDetector for NVIDIA BlueField DPUs.
// This integrates with the pkg/plugin registry for unified device detection.
type NvidiaDetector struct {
	name string
}

// NewNvidiaDetector creates a new NVIDIA BlueField detector.
func NewNvidiaDetector() *NvidiaDetector {
	return &NvidiaDetector{name: "NVIDIA BlueField"}
}

// Name returns the detector name.
func (d *NvidiaDetector) Name() string {
	return d.name
}

// GetVendorName returns the vendor identifier.
func (d *NvidiaDetector) GetVendorName() string {
	return "nvidia"
}

// DpuPlatformName returns the platform directory name.
func (d *NvidiaDetector) DpuPlatformName() string {
	return "nvidia-bf"
}

// IsDpuPlatform checks if we are running on a BlueField DPU (ARM side).
func (d *NvidiaDetector) IsDpuPlatform(platform Platform) (bool, error) {
	product, err := platform.Product()
	if err != nil {
		return false, fmt.Errorf("error getting product info: %v", err)
	}

	// BlueField DPUs identify as "BlueField" in product name when running on ARM
	if strings.Contains(product.Name, "BlueField") {
		return true, nil
	}
	return false, nil
}

// IsDPU checks if a PCI device is a BlueField DPU on the host side.
func (d *NvidiaDetector) IsDPU(platform Platform, pci ghw.PCIDevice, dpuDevices []plugin.DpuIdentifier) (bool, error) {
	// Check using the plugin registry for device ID matching
	deviceID := strings.ToLower(pci.Vendor.ID + ":" + pci.Product.ID)
	p := pkgplugin.GetByDeviceID(deviceID)
	if p == nil {
		return false, nil
	}

	// Verify it's the nvidia plugin
	if p.Info().Name != "nvidia" {
		return false, nil
	}

	// Must be a network device
	if pci.Class.Name != "Network controller" && pci.Class.Name != "Ethernet controller" {
		return false, nil
	}

	// Check for duplicate - avoid counting multi-port devices multiple times
	identifier, err := d.GetDpuIdentifier(platform, &pci)
	if err != nil {
		return false, err
	}
	for _, existing := range dpuDevices {
		if existing == identifier {
			return false, nil // Already detected this DPU
		}
	}

	return true, nil
}

// GetDpuIdentifier returns a unique identifier for the DPU based on serial number.
func (d *NvidiaDetector) GetDpuIdentifier(platform Platform, pci *ghw.PCIDevice) (plugin.DpuIdentifier, error) {
	// Try to get serial number from PCI config space
	serial, err := platform.ReadDeviceSerialNumber(pci)
	if err != nil {
		// Fall back to PCI address if serial unavailable
		identifier := fmt.Sprintf("nvidia-bf-%s", SanitizePCIAddress(pci.Address))
		return plugin.DpuIdentifier(identifier), nil
	}

	identifier := fmt.Sprintf("nvidia-bf-%s", serial)
	return plugin.DpuIdentifier(identifier), nil
}

// DpuPlatformIdentifier returns a unique identifier when running on DPU side.
func (d *NvidiaDetector) DpuPlatformIdentifier(platform Platform) (plugin.DpuIdentifier, error) {
	product, err := platform.Product()
	if err != nil {
		return "", fmt.Errorf("error getting product info: %v", err)
	}

	// Use product serial or UUID as identifier
	identifier := fmt.Sprintf("nvidia-bf-%s", SanitizeForTemplate(product.Name))
	return plugin.DpuIdentifier(identifier), nil
}

// VspPlugin creates a GrpcPlugin for communication with the VSP.
func (d *NvidiaDetector) VspPlugin(dpuMode bool, imageManager images.ImageManager, client client.Client, pm utils.PathManager, dpuIdentifier plugin.DpuIdentifier) (*plugin.GrpcPlugin, error) {
	return plugin.NewGrpcPlugin(dpuMode, dpuIdentifier, client, plugin.WithPathManager(pm))
}
