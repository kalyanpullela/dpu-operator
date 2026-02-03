package platform

import (
	stderrors "errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jaypipes/ghw"
	v1 "github.com/openshift/dpu-operator/api/v1"
	"github.com/openshift/dpu-operator/internal/daemon/plugin"
	"github.com/openshift/dpu-operator/internal/images"
	"github.com/openshift/dpu-operator/internal/utils"
	pkgplugin "github.com/openshift/dpu-operator/pkg/plugin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kind/pkg/errors"
)

type DpuDetectorManager struct {
	platform       Platform
	detectors      []VendorDetector
	pluginRegistry *pkgplugin.Registry
}

type VendorDetector interface {
	Name() string

	// Returns true if the platform is a DPU, otherwise false.
	// platform - The platform of the host system (host being the DPU).
	IsDpuPlatform(platform Platform) (bool, error)

	// Returns a VSP plugin for the detected DPU platform.
	// dpuMode - If true, the plugin is created for DPU mode, otherwise for host mode.
	// imageManager - The image manager to retrieve VSP images.
	// client - The Kubernetes client used to deploy the VSP.
	// dpuPciDevice - The PCI device of the DPU, if available. This is used to identify the DPU device for the plugin.
	VspPlugin(dpuMode bool, imageManager images.ImageManager, client client.Client, pm utils.PathManager, dpuIdentifier plugin.DpuIdentifier) (*plugin.GrpcPlugin, error)

	// Returns true if the device is a DPU detected by the detector, otherwise false.
	// platform - The platform of the host system (host with DPU).
	// pci - This argument is the PCI device to check if it matches what the detector is looking for.
	// dpuDevices (optional) - Is a list of already detected DPU devices used for excluding multi-port devices to be counted more than once.
	IsDPU(platform Platform, pci ghw.PCIDevice, dpuDevices []plugin.DpuIdentifier) (bool, error)

	// Returns a unique identifier for the DPU device.
	// platform - The platform of the host system (host with DPU).
	// pci - The PCI device of the DPU's network interface.
	GetDpuIdentifier(platform Platform, pci *ghw.PCIDevice) (plugin.DpuIdentifier, error)

	GetVendorName() string

	// The name of the DPU platform.
	DpuPlatformName() string

	// A unique identifier for when detection happens on the DPU
	DpuPlatformIdentifier(platform Platform) (plugin.DpuIdentifier, error)
}

// SanitizeForTemplate converts identifiers to be template-safe by replacing hyphens with underscores
func SanitizeForTemplate(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// SanitizeForK8sName normalizes strings for use in Kubernetes resource names.
// It lowercases and replaces unsupported characters with "-".
func SanitizeForK8sName(name string) string {
	if name == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "unknown"
	}
	return out
}

func NewDpuDetectorManager(platform Platform) *DpuDetectorManager {
	return &DpuDetectorManager{
		platform: platform,
		detectors: []VendorDetector{
			NewIntelDetector(),
			NewMarvellDetector(),
			NewNetsecAcceleratorDetector(),
			NewNvidiaDetector(),
			NewXSightDetector(),
			NewMangoBoostDetector(),
			// add more detectors here
		},
		pluginRegistry: pkgplugin.DefaultRegistry(),
	}
}

func (d *DpuDetectorManager) GetVendorDirectory(dpuProductName string) (string, error) {
	for _, detector := range d.detectors {
		if detector.Name() == dpuProductName {
			return detector.DpuPlatformName(), nil
		}
	}
	return "", fmt.Errorf("unknown DPU product name: %s", dpuProductName)
}

// GetVendorNameByProductName returns the vendor name for a given DPU product name.
func (d *DpuDetectorManager) GetVendorNameByProductName(dpuProductName string) (string, error) {
	for _, detector := range d.detectors {
		if detector.Name() == dpuProductName {
			return detector.GetVendorName(), nil
		}
	}
	return "", fmt.Errorf("unknown DPU product name: %s", dpuProductName)
}

func (d *DpuDetectorManager) GetDetectors() []VendorDetector {
	return d.detectors
}

func (pi *DpuDetectorManager) IsDpu() (bool, error) {
	detector, err := pi.detectDpuPlatform(false)
	return detector != nil, err
}

func (pi *DpuDetectorManager) postFixDpuSideToIdentifier(identifier plugin.DpuIdentifier, dpuSide bool) plugin.DpuIdentifier {
	var postfix string
	if dpuSide {
		postfix = "-dpu"
	} else {
		postfix = "-host"
	}
	return plugin.DpuIdentifier(string(identifier) + postfix)
}

func (pi *DpuDetectorManager) uniqueIdentifierForNode(identifier plugin.DpuIdentifier, nodeName string) plugin.DpuIdentifier {
	if nodeName == "" {
		return identifier
	}
	suffix := SanitizeForK8sName(nodeName)
	return plugin.DpuIdentifier(fmt.Sprintf("%s-%s", identifier, suffix))
}

func (pi *DpuDetectorManager) detectDpuPlatform(required bool) (VendorDetector, error) {
	var activeDetectors []VendorDetector
	var errResult error

	for _, detector := range pi.detectors {
		isDPU, err := detector.IsDpuPlatform(pi.platform)
		if err != nil {
			errResult = stderrors.Join(errResult, err)
			continue
		}
		if isDPU {
			activeDetectors = append(activeDetectors, detector)
		}
	}
	if errResult != nil {
		return nil, errors.Errorf("Failed to detect DPU platform: %v", errResult)
	}
	if len(activeDetectors) != 1 {
		if len(activeDetectors) != 0 {
			return nil, errors.Errorf("Failed to detect DPU platform unambiguously: %v", activeDetectors)
		}
		if required {
			return nil, errors.Errorf("Failed to detect any DPU platform")
		}
		return nil, nil
	}
	return activeDetectors[0], nil
}

type DetectedDpuWithPlugin struct {
	DpuCR  *v1.DataProcessingUnit
	Plugin *plugin.GrpcPlugin
}

func (d *DpuDetectorManager) DetectAll(imageManager images.ImageManager, client client.Client, pm utils.PathManager, nodeName string) ([]*DetectedDpuWithPlugin, error) {
	var detectedDpus []*DetectedDpuWithPlugin

	for _, detector := range d.detectors {
		dpuPlatform, err := detector.IsDpuPlatform(d.platform)
		if err != nil {
			return nil, fmt.Errorf("Error detecting if running on DPU platform with detector %v: %v", detector.Name(), err)
		}

		if dpuPlatform {
			isDpuSide := true
			identifier, err := detector.DpuPlatformIdentifier(d.platform)
			if err != nil {
				return nil, err
			}
			vsp, err := detector.VspPlugin(true, imageManager, client, pm, identifier)
			if err != nil {
				return nil, err
			}
			d.attachRegistryPlugin(detector, vsp)

			uniqueIdentifier := d.uniqueIdentifierForNode(identifier, nodeName)
			dpuCR := &v1.DataProcessingUnit{
				ObjectMeta: metav1.ObjectMeta{
					Name: string(d.postFixDpuSideToIdentifier(uniqueIdentifier, isDpuSide)),
				},
				Spec: v1.DataProcessingUnitSpec{
					DpuProductName: detector.Name(),
					IsDpuSide:      isDpuSide,
					NodeName:       nodeName,
				},
				Status: v1.DataProcessingUnitStatus{
					Conditions: []metav1.Condition{
						{
							Type:               plugin.ReadyConditionType,
							Status:             metav1.ConditionFalse,
							Reason:             "Initializing",
							Message:            "DPU resource is being initialized.",
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			}
			detectedDpus = append(detectedDpus, &DetectedDpuWithPlugin{
				DpuCR:  dpuCR,
				Plugin: vsp,
			})
			continue
		}

		devices, err := d.platform.PciDevices()
		if err != nil {
			return nil, errors.Errorf("Error getting PCI info: %v", err)
		}

		var dpuDevices []plugin.DpuIdentifier
		for _, pci := range devices {
			isDpu, err := detector.IsDPU(d.platform, *pci, dpuDevices)
			if err != nil {
				return nil, errors.Errorf("Error detecting if device is DPU with detector %v: %v", detector.Name(), err)
			}
			if isDpu {
				isDpuSide := false
				identifier, err := detector.GetDpuIdentifier(d.platform, pci)
				if err != nil {
					return nil, errors.Errorf("Error getting DPU identifier with detector %v: %v", detector.Name(), err)
				}
				// WARN: The identifier used in the dpuDevices slice & VSP plugin MUST NOT have the DPU side postfix, since it is used to compare multiple host
				// PCI interfaces. The same DPU has the same Serial Number, but different PCI addresses in order for the code to ignore the other ports.
				dpuDevices = append(dpuDevices, identifier)
				vsp, err := detector.VspPlugin(false, imageManager, client, pm, identifier)
				if err != nil {
					return nil, err
				}
				d.attachRegistryPlugin(detector, vsp)

				uniqueIdentifier := d.uniqueIdentifierForNode(identifier, nodeName)
				dpuCR := &v1.DataProcessingUnit{
					ObjectMeta: metav1.ObjectMeta{
						Name: string(d.postFixDpuSideToIdentifier(uniqueIdentifier, isDpuSide)),
					},
					Spec: v1.DataProcessingUnitSpec{
						DpuProductName: detector.Name(),
						IsDpuSide:      isDpuSide,
						NodeName:       nodeName,
					},
					Status: v1.DataProcessingUnitStatus{
						Conditions: []metav1.Condition{
							{
								Type:               plugin.ReadyConditionType,
								Status:             metav1.ConditionFalse,
								Reason:             "Initializing",
								Message:            "DPU resource is being initialized.",
								LastTransitionTime: metav1.Now(),
							},
						},
					},
				}
				detectedDpus = append(detectedDpus, &DetectedDpuWithPlugin{
					DpuCR:  dpuCR,
					Plugin: vsp,
				})
			}
		}
	}
	return detectedDpus, nil
}

func (d *DpuDetectorManager) attachRegistryPlugin(detector VendorDetector, vsp *plugin.GrpcPlugin) {
	if vsp == nil || d.pluginRegistry == nil || detector == nil {
		return
	}
	vendor := detector.GetVendorName()
	if vendor == "" {
		return
	}
	registryPlugin := d.findRegistryPluginByVendor(vendor)
	if registryPlugin == nil {
		return
	}
	vsp.AttachRegistryPlugin(registryPlugin, pluginConfigFromEnv(vendor))
}

func (d *DpuDetectorManager) findRegistryPluginByVendor(vendor string) pkgplugin.Plugin {
	for _, p := range d.pluginRegistry.List() {
		info := p.Info()
		if strings.EqualFold(info.Vendor, vendor) || strings.EqualFold(info.Name, vendor) {
			return p
		}
	}
	return nil
}

func pluginConfigFromEnv(vendor string) pkgplugin.PluginConfig {
	endpoint := os.Getenv("DPU_PLUGIN_OPI_ENDPOINT")

	if vendor != "" {
		key := "DPU_PLUGIN_OPI_ENDPOINT_" + strings.ToUpper(strings.ReplaceAll(vendor, "-", "_"))
		if value := os.Getenv(key); value != "" {
			endpoint = value
		}
	}

	logLevel := 0
	if raw := os.Getenv("DPU_PLUGIN_LOG_LEVEL"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			logLevel = parsed
		}
	}

	return pkgplugin.PluginConfig{
		OPIEndpoint: endpoint,
		LogLevel:    logLevel,
	}
}
