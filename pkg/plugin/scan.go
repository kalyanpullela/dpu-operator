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

package plugin

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/openshift/dpu-operator/pkg/plugin/pci"
)

// ScanDevices scans the PCI bus for devices matching supportedDevices and returns
// plugin.Device entries.  vendorName is stored in the Vendor field; idPrefix and
// serialPrefix are prepended to the PCI address to form ID and fallback serial
// number respectively.  This is the single shared implementation used by all
// vendor plugins — do not duplicate this loop.
func ScanDevices(supportedDevices []PCIDeviceID, vendorName, idPrefix, serialPrefix string, log logr.Logger) ([]Device, error) {
	scanner := pci.NewScanner()
	var devices []Device

	for _, supportedDevice := range supportedDevices {
		pciDevices, err := scanner.ScanByVendorDevice(supportedDevice.VendorID, supportedDevice.DeviceID)
		if err != nil {
			log.V(1).Info("Failed to scan for PCI device",
				"vendorID", supportedDevice.VendorID,
				"deviceID", supportedDevice.DeviceID,
				"error", err)
			continue
		}

		for _, pciDev := range pciDevices {
			device := Device{
				ID:         fmt.Sprintf("%s-%s", idPrefix, pciDev.Address),
				PCIAddress: pciDev.Address,
				Vendor:     vendorName,
				Model:      supportedDevice.Description,
				Healthy:    true,
				Metadata: map[string]string{
					"pci_vendor_id": pciDev.VendorID,
					"pci_device_id": pciDev.DeviceID,
					"pci_class":     pciDev.Class,
					"device_type":   supportedDevice.Description,
					"driver":        pciDev.Driver,
					"numa_node":     pciDev.NumaNode,
				},
			}

			if serialNum, err := scanner.GetSerialNumber(pciDev.Address); err == nil {
				device.SerialNumber = serialNum
			} else {
				device.SerialNumber = fmt.Sprintf("%s-%s", serialPrefix, pciDev.Address)
			}

			devices = append(devices, device)
			log.Info("Discovered device",
				"vendor", vendorName,
				"pciAddress", pciDev.Address,
				"model", device.Model,
				"driver", pciDev.Driver)
		}
	}

	return devices, nil
}
