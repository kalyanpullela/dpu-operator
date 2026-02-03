package platform

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jaypipes/ghw"
	"k8s.io/klog/v2"
)

type Platform interface {
	PciDevices() ([]*ghw.PCIDevice, error)
	NetDevs() ([]*ghw.NIC, error)
	Product() (*ghw.ProductInfo, error)
	ReadDeviceSerialNumber(pciDevice *ghw.PCIDevice) (string, error)
	GetNetDevNamesFromPCIeAddr(pcieAddress string) ([]string, error)
	GetNetDevNameFromPCIeAddr(pcieAddress string) (string, error)
	GetNetDevMACAddressFromPCIeAddr(pcieAddress string) (string, error)
}

type HardwarePlatform struct{}

func NewHardwarePlatform() *HardwarePlatform {
	return &HardwarePlatform{}
}

func (hp *HardwarePlatform) PciDevices() ([]*ghw.PCIDevice, error) {
	pciInfo, err := ghw.PCI()
	if err != nil {
		return nil, err
	}
	return pciInfo.Devices, nil
}

func (hp *HardwarePlatform) NetDevs() ([]*ghw.NIC, error) {
	netInfo, err := ghw.Network()
	if err != nil {
		return nil, err
	}
	return netInfo.NICs, nil
}

// GetNetDevNamesFromPCIeAddr retrieves the network device name associated with a given PCIe address.
// This can fail if the given PCIe address is not a NetDev or the driver is not loaded correctly.
func (hp *HardwarePlatform) GetNetDevNamesFromPCIeAddr(pcieAddress string) ([]string, error) {
	nics, err := hp.NetDevs()
	if err != nil {
		return nil, fmt.Errorf("failed to get network devices: %w", err)
	}

	var ifaces []string
	for _, nic := range nics {
		if nic.PCIAddress != nil && *nic.PCIAddress == pcieAddress {
			klog.V(2).Infof("GetNetDevNamesFromPCIeAddr(): found network device Name: %s PCIe: %s", nic.Name, *nic.PCIAddress)
			ifaces = append(ifaces, nic.Name)
		}
	}

	return ifaces, nil
}

// GetNetDevNameFromPCIeAddr returns the name of the single network device associated with a given PCIe address.
func (hp *HardwarePlatform) GetNetDevNameFromPCIeAddr(pcieAddress string) (string, error) {
	ifNames, err := hp.GetNetDevNamesFromPCIeAddr(pcieAddress)
	if err != nil {
		return "", err
	}

	if len(ifNames) != 1 {
		err = fmt.Errorf("expected exactly 1 interface for PCIe address %s, got %v", pcieAddress, ifNames)
		return "", err
	}

	return ifNames[0], nil
}

func (hp *HardwarePlatform) GetNetDevMACAddressFromPCIeAddr(pcieAddress string) (string, error) {
	nics, err := hp.NetDevs()
	if err != nil {
		return "", fmt.Errorf("failed to get network devices: %w", err)
	}

	for _, nic := range nics {
		if nic.PCIAddress != nil && *nic.PCIAddress == pcieAddress {
			klog.V(2).Infof("GetNetDevMACAddressFromPCIeAddr(): found network device Name: %s PCIe: %s MAC: %s", nic.Name, *nic.PCIAddress, nic.MacAddress)
			return nic.MacAddress, nil
		}
	}

	return "", fmt.Errorf("no network device found at address %s", pcieAddress)
}

func (hp *HardwarePlatform) Product() (*ghw.ProductInfo, error) {
	return ghw.Product()
}

func (hp *HardwarePlatform) ReadDeviceSerialNumber(pciDevice *ghw.PCIDevice) (string, error) {
	if pciDevice == nil {
		return "", fmt.Errorf("nil PCI device provided")
	}

	devicePath := filepath.Join("/sys/bus/pci/devices", pciDevice.Address, "config")

	data, err := os.ReadFile(devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to open config space: %v", err)
	}

	serial, err := readDeviceSerialFromConfig(data)
	if err != nil {
		return "", err
	}
	return serial, nil
}

// readDeviceSerialFromConfig searches PCI extended capabilities for the
// Device Serial Number (DSN) capability and returns it as hex.
func readDeviceSerialFromConfig(data []byte) (string, error) {
	// PCIe extended capabilities start at 0x100 and are 4-byte aligned.
	const extCapStart = 0x100
	const dsnCapabilityID = 0x0003

	if len(data) < extCapStart+4 {
		return "", fmt.Errorf("config space too small for extended capabilities")
	}

	offset := extCapStart
	visited := map[int]struct{}{}
	for offset != 0 && offset+4 <= len(data) {
		if _, seen := visited[offset]; seen {
			return "", fmt.Errorf("detected loop in PCI extended capabilities")
		}
		visited[offset] = struct{}{}

		header := binary.LittleEndian.Uint32(data[offset : offset+4])
		if header == 0 {
			break
		}

		capID := header & 0xFFFF
		nextPtr := (header >> 20) & 0xFFF

		if capID == dsnCapabilityID {
			if offset+12 > len(data) {
				return "", fmt.Errorf("invalid DSN capability length")
			}
			serialBytes := data[offset+4 : offset+12]
			return hex.EncodeToString(serialBytes), nil
		}

		if nextPtr == 0 {
			break
		}
		offset = int(nextPtr * 4)
	}

	return "", fmt.Errorf("device serial number capability not found")
}

// SanitizePCIAddress sanitizes a PCI address or device identifier for use as a Kubernetes cr name
// by replacing characters that are not allowed in resource names with hyphens
// (e.g., "0000:04:00.0" becomes "0000-04-00.0", "FAKE-SERIAL-0000:04:00.0" becomes "FAKE-SERIAL-0000-04-00.0")
func SanitizePCIAddress(input string) string {
	return strings.ReplaceAll(input, ":", "-")
}

type FakePlatform struct {
	platformName string
	devices      []*ghw.PCIDevice
	netdevs      []*ghw.NIC
	mu           sync.Mutex
}

func NewFakePlatform(platformName string) *FakePlatform {
	return &FakePlatform{
		platformName: platformName,
		devices:      make([]*ghw.PCIDevice, 0),
	}
}

func (p *FakePlatform) PciDevices() ([]*ghw.PCIDevice, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.devices, nil
}

func (p *FakePlatform) NetDevs() ([]*ghw.NIC, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.netdevs, nil
}

func (p *FakePlatform) Product() (*ghw.ProductInfo, error) {
	return &ghw.ProductInfo{
		Name: p.platformName,
	}, nil
}

func (p *FakePlatform) ReadDeviceSerialNumber(pciDevice *ghw.PCIDevice) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if pciDevice == nil {
		return "", fmt.Errorf("nil PCI device provided")
	}

	//TODO: Implement a more realistic serial number generation
	return "FAKE-SERIAL-" + pciDevice.Address, nil
}

func (p *FakePlatform) RemoveAllPciDevices() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.devices = make([]*ghw.PCIDevice, 0)
}

func (p *FakePlatform) GetNetDevNamesFromPCIeAddr(pcieAddress string) ([]string, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (p *FakePlatform) GetNetDevNameFromPCIeAddr(pcieAddress string) (string, error) {
	return "", fmt.Errorf("Not implemented")
}

func (p *FakePlatform) GetNetDevMACAddressFromPCIeAddr(pcieAddress string) (string, error) {
	return "", fmt.Errorf("Not implemented")
}

func (p *FakePlatform) AddPciDevice(dev *ghw.PCIDevice) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.devices = append(p.devices, dev)
}
