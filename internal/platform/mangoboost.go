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

// MangoBoostDetector implements VendorDetector for MangoBoost DPUs.
// This integrates with the pkg/plugin registry for unified device detection.
type MangoBoostDetector struct {
	name string
}

// NewMangoBoostDetector creates a new MangoBoost detector.
func NewMangoBoostDetector() *MangoBoostDetector {
	return &MangoBoostDetector{name: "MangoBoost DPU"}
}

// Name returns the detector name.
func (d *MangoBoostDetector) Name() string {
	return d.name
}

// GetVendorName returns the vendor identifier.
func (d *MangoBoostDetector) GetVendorName() string {
	return "mangoboost"
}

// DpuPlatformName returns the platform directory name.
func (d *MangoBoostDetector) DpuPlatformName() string {
	return "mangoboost"
}

// IsDpuPlatform checks if we are running on a MangoBoost DPU (DPU side).
func (d *MangoBoostDetector) IsDpuPlatform(platform Platform) (bool, error) {
	product, err := platform.Product()
	if err != nil {
		return false, fmt.Errorf("error getting product info: %v", err)
	}

	// MangoBoost DPUs identify with MangoBoost in product name when running on DPU
	if strings.Contains(strings.ToLower(product.Name), "mangoboost") {
		return true, nil
	}
	return false, nil
}

// IsDPU checks if a PCI device is a MangoBoost DPU on the host side.
func (d *MangoBoostDetector) IsDPU(platform Platform, pci ghw.PCIDevice, dpuDevices []plugin.DpuIdentifier) (bool, error) {
	// Check using the plugin registry for device ID matching
	deviceID := strings.ToLower(pci.Vendor.ID + ":" + pci.Product.ID)
	p := pkgplugin.GetByDeviceID(deviceID)
	if p == nil {
		return false, nil
	}

	// Verify it's the mangoboost plugin
	if p.Info().Name != "mangoboost" {
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
func (d *MangoBoostDetector) GetDpuIdentifier(platform Platform, pci *ghw.PCIDevice) (plugin.DpuIdentifier, error) {
	// Try to get serial number from PCI config space
	serial, err := platform.ReadDeviceSerialNumber(pci)
	if err != nil {
		// Fall back to PCI address if serial unavailable
		identifier := fmt.Sprintf("mangoboost-%s", SanitizePCIAddress(pci.Address))
		return plugin.DpuIdentifier(identifier), nil
	}

	identifier := fmt.Sprintf("mangoboost-%s", serial)
	return plugin.DpuIdentifier(identifier), nil
}

// DpuPlatformIdentifier returns a unique identifier when running on DPU side.
func (d *MangoBoostDetector) DpuPlatformIdentifier(platform Platform) (plugin.DpuIdentifier, error) {
	product, err := platform.Product()
	if err != nil {
		return "", fmt.Errorf("error getting product info: %v", err)
	}

	// Use product serial or UUID as identifier
	identifier := fmt.Sprintf("mangoboost-%s", SanitizeForTemplate(product.Name))
	return plugin.DpuIdentifier(identifier), nil
}

// VspPlugin creates a GrpcPlugin for communication with the VSP.
func (d *MangoBoostDetector) VspPlugin(dpuMode bool, imageManager images.ImageManager, client client.Client, pm utils.PathManager, dpuIdentifier plugin.DpuIdentifier) (*plugin.GrpcPlugin, error) {
	return plugin.NewGrpcPlugin(dpuMode, dpuIdentifier, client, plugin.WithPathManager(pm))
}
