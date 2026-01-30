package pci

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanner_readHexFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "pci-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file with hex content
	testFile := filepath.Join(tmpDir, "test_hex")
	if err := os.WriteFile(testFile, []byte("0x15b3\n"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	scanner := NewScanner()
	result, err := scanner.readHexFile(testFile)
	if err != nil {
		t.Errorf("readHexFile failed: %v", err)
	}

	expected := "15b3"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestScanner_readDevice(t *testing.T) {
	scanner := NewScanner()

	// This test requires actual sysfs, so skip if not available
	if _, err := os.Stat(scanner.sysfsPath); os.IsNotExist(err) {
		t.Skip("Skipping test: sysfs not available")
	}

	// Try to read any device
	entries, err := os.ReadDir(scanner.sysfsPath)
	if err != nil {
		t.Fatalf("Failed to read sysfs: %v", err)
	}

	if len(entries) == 0 {
		t.Skip("No PCI devices found")
	}

	// Read first device
	device, err := scanner.readDevice(entries[0].Name())
	if err != nil {
		t.Errorf("readDevice failed: %v", err)
	}

	// Verify basic fields are populated
	if device.Address == "" {
		t.Error("Device address is empty")
	}
	if device.VendorID == "" {
		t.Error("Vendor ID is empty")
	}
	if device.DeviceID == "" {
		t.Error("Device ID is empty")
	}
}

func TestScanner_ScanAll(t *testing.T) {
	scanner := NewScanner()

	// Skip if sysfs not available
	if _, err := os.Stat(scanner.sysfsPath); os.IsNotExist(err) {
		t.Skip("Skipping test: sysfs not available")
	}

	devices, err := scanner.ScanAll()
	if err != nil {
		t.Errorf("ScanAll failed: %v", err)
	}

	// We should find at least some PCI devices on any system
	if len(devices) == 0 {
		t.Log("Warning: No PCI devices found")
	}

	for _, device := range devices {
		if device.VendorID == "" || device.DeviceID == "" {
			t.Errorf("Device %s has empty vendor or device ID", device.Address)
		}
	}
}

func TestScanner_ScanByVendorDevice(t *testing.T) {
	scanner := NewScanner()

	// Skip if sysfs not available
	if _, err := os.Stat(scanner.sysfsPath); os.IsNotExist(err) {
		t.Skip("Skipping test: sysfs not available")
	}

	// Test with a common vendor ID (Intel)
	devices, err := scanner.ScanByVendorDevice("8086", "")
	if err != nil {
		t.Errorf("ScanByVendorDevice failed: %v", err)
	}

	// We might or might not find Intel devices, so just check for errors
	for _, device := range devices {
		if device.VendorID != "8086" {
			t.Errorf("Expected vendor 8086, got %s", device.VendorID)
		}
	}
}
