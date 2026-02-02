package pci

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Device represents a PCI device discovered on the system
type Device struct {
	Address           string // e.g., "0000:03:00.0"
	VendorID          string // e.g., "15b3"
	DeviceID          string // e.g., "a2d6"
	SubsystemVendorID string
	SubsystemDeviceID string
	Class             string
	Driver            string
	NumaNode          string
}

// Scanner scans for PCI devices on the system
type Scanner struct {
	sysfsPath string
}

// NewScanner creates a new PCI scanner
func NewScanner() *Scanner {
	return &Scanner{
		sysfsPath: "/sys/bus/pci/devices",
	}
}

// ScanAll scans for all PCI devices
func (s *Scanner) ScanAll() ([]Device, error) {
	entries, err := os.ReadDir(s.sysfsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PCI devices directory: %w", err)
	}

	var devices []Device
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		device, err := s.readDevice(entry.Name())
		if err != nil {
			// Log error but continue scanning
			continue
		}

		devices = append(devices, device)
	}

	return devices, nil
}

// ScanByVendorDevice scans for PCI devices matching vendor and device IDs
func (s *Scanner) ScanByVendorDevice(vendorID, deviceID string) ([]Device, error) {
	allDevices, err := s.ScanAll()
	if err != nil {
		return nil, err
	}

	var matched []Device
	for _, device := range allDevices {
		if device.VendorID == vendorID && device.DeviceID == deviceID {
			matched = append(matched, device)
		}
	}

	return matched, nil
}

// readDevice reads device information from sysfs
func (s *Scanner) readDevice(address string) (Device, error) {
	device := Device{
		Address: address,
	}

	devicePath := filepath.Join(s.sysfsPath, address)

	// Read vendor ID
	vendorID, err := s.readHexFile(filepath.Join(devicePath, "vendor"))
	if err == nil {
		device.VendorID = vendorID
	}

	// Read device ID
	deviceID, err := s.readHexFile(filepath.Join(devicePath, "device"))
	if err == nil {
		device.DeviceID = deviceID
	}

	// Read subsystem vendor ID
	subsystemVendorID, err := s.readHexFile(filepath.Join(devicePath, "subsystem_vendor"))
	if err == nil {
		device.SubsystemVendorID = subsystemVendorID
	}

	// Read subsystem device ID
	subsystemDeviceID, err := s.readHexFile(filepath.Join(devicePath, "subsystem_device"))
	if err == nil {
		device.SubsystemDeviceID = subsystemDeviceID
	}

	// Read class
	class, err := s.readHexFile(filepath.Join(devicePath, "class"))
	if err == nil {
		device.Class = class
	}

	// Read driver (if bound)
	driverLink := filepath.Join(devicePath, "driver")
	if target, err := os.Readlink(driverLink); err == nil {
		device.Driver = filepath.Base(target)
	}

	// Read NUMA node
	if numaNode, err := s.readFile(filepath.Join(devicePath, "numa_node")); err == nil {
		device.NumaNode = strings.TrimSpace(numaNode)
	}

	return device, nil
}

// readHexFile reads a sysfs file containing a hex value (e.g., 0x15b3)
// and returns just the hex digits without the 0x prefix
func (s *Scanner) readHexFile(path string) (string, error) {
	content, err := s.readFile(path)
	if err != nil {
		return "", err
	}

	// Trim whitespace and remove 0x prefix
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "0x")

	return content, nil
}

// readFile reads a text file and returns its content
func (s *Scanner) readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetDeviceInfo returns detailed information about a specific PCI device
func (s *Scanner) GetDeviceInfo(address string) (*Device, error) {
	device, err := s.readDevice(address)
	if err != nil {
		return nil, fmt.Errorf("failed to read device %s: %w", address, err)
	}
	return &device, nil
}

// GetSerialNumber attempts to read the device serial number from VPD
func (s *Scanner) GetSerialNumber(address string) (string, error) {
	devicePath := filepath.Join(s.sysfsPath, address)
	vpdPath := filepath.Join(devicePath, "vpd")

	file, err := os.Open(vpdPath)
	if err != nil {
		return "", fmt.Errorf("failed to open VPD: %w", err)
	}
	defer file.Close()

	// Parse VPD for serial number field (SN)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "SN") {
			// Extract serial number after SN tag
			parts := strings.SplitN(line, "SN", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", fmt.Errorf("serial number not found in VPD")
}

// GetFirmwareVersion attempts to read firmware version from device-specific location
func (s *Scanner) GetFirmwareVersion(address string) (string, error) {
	// This is device-specific and may require vendor-specific methods
	// For now, return empty string as it should be queried via OPI/vendor SDK
	return "", fmt.Errorf("firmware version must be queried via vendor-specific API")
}
