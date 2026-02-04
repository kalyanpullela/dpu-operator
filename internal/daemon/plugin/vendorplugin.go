package plugin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	nfapi "github.com/openshift/dpu-operator/dpu-api/gen"
	"github.com/openshift/dpu-operator/internal/utils"
	pkgplugin "github.com/openshift/dpu-operator/pkg/plugin"
	opi "github.com/opiproject/opi-api/network/evpn-gw/v1alpha1/gen/go"
	pb "github.com/opiproject/opi-api/v1/gen/go/lifecycle"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ReadyConditionType = "Ready"

type DpuIdentifier string

type VendorPlugin interface {
	Start(ctx context.Context) (string, int32, error)
	Close()
	CreateBridgePort(bpr *opi.CreateBridgePortRequest) (*opi.BridgePort, error)
	DeleteBridgePort(bpr *opi.DeleteBridgePortRequest) error
	CreateNetworkFunction(input string, output string) error
	DeleteNetworkFunction(input string, output string) error
	GetDevices() (*pb.DeviceListResponse, error)
	SetNumVfs(vfCount int32) (*pb.VfCount, error)
}

type GrpcPlugin struct {
	log           logr.Logger
	client        pb.LifeCycleServiceClient
	k8sClient     client.Client
	opiClient     opi.BridgePortServiceClient
	nfclient      nfapi.NetworkFunctionServiceClient
	dsClient      pb.DeviceServiceClient
	dpuMode       bool
	dpuIdentifier DpuIdentifier
	conn          *grpc.ClientConn
	pathManager   utils.PathManager
	initialized   bool
	initMutex     sync.RWMutex

	registryPlugin      pkgplugin.Plugin
	registryConfig      pkgplugin.PluginConfig
	registryInitMutex   sync.Mutex
	registryInitialized bool
	registryLastAttempt time.Time
	registryInitErr     error
}

func (g *GrpcPlugin) Start(ctx context.Context) (string, int32, error) {
	start := time.Now()
	interval := 100 * time.Millisecond

	// Best-effort initialization of registry plugin for hybrid mode.
	_ = g.ensureRegistryInitialized(ctx)

	for {
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		default:
		}

		err := g.ensureConnected()
		if err != nil {
			select {
			case <-ctx.Done():
				return "", 0, ctx.Err()
			case <-time.After(interval):
			}
			continue
		}

		ipPort, err := g.client.Init(ctx, &pb.InitRequest{DpuMode: g.dpuMode, DpuIdentifier: string(g.dpuIdentifier)})
		if err != nil {
			if strings.Contains(err.Error(), "already initialized") {
				// VSP was already initialized, mark as initialized and return the error
				g.SetInitDone(true)
				return "", 0, err
			}
			select {
			case <-ctx.Done():
				return "", 0, ctx.Err()
			case <-time.After(interval):
			}
			continue
		}

		// Init succeeded, mark as initialized
		g.SetInitDone(true)

		g.log.Info("GrpcPlugin Start() succeeded", "duration", time.Since(start), "ip", ipPort.Ip, "port", ipPort.Port, "dpuMode",
			g.dpuMode, "dpuIdentifier", g.dpuIdentifier)
		return ipPort.Ip, ipPort.Port, nil
	}
}

func (g *GrpcPlugin) Close() {
	if g.registryPlugin != nil {
		g.registryInitMutex.Lock()
		initialized := g.registryInitialized
		g.registryInitMutex.Unlock()

		if initialized {
			if err := g.registryPlugin.Shutdown(context.Background()); err != nil {
				g.log.Info("Registry plugin shutdown failed", "error", err)
			}
			g.registryInitMutex.Lock()
			g.registryInitialized = false
			g.registryInitErr = nil
			g.registryLastAttempt = time.Time{}
			g.registryInitMutex.Unlock()
		}
	}

	if g.conn != nil {
		g.conn.Close()
		g.conn = nil
		g.client = nil
		g.nfclient = nil
		g.opiClient = nil
		g.dsClient = nil
	}
}

func WithPathManager(pathManager utils.PathManager) func(*GrpcPlugin) {
	return func(d *GrpcPlugin) {
		d.pathManager = pathManager
	}
}

// AttachRegistryPlugin configures a registry plugin for hybrid runtime mode.
// The registry plugin is used for discovery/VF configuration when available,
// with safe fallback to the VSP gRPC path.
func (g *GrpcPlugin) AttachRegistryPlugin(p pkgplugin.Plugin, config pkgplugin.PluginConfig) {
	g.registryInitMutex.Lock()
	defer g.registryInitMutex.Unlock()
	g.registryPlugin = p
	g.registryConfig = config
	g.registryInitialized = false
	g.registryInitErr = nil
	g.registryLastAttempt = time.Time{}
}

func NewGrpcPlugin(dpuMode bool, dpuIdentifier DpuIdentifier, client client.Client, opts ...func(*GrpcPlugin)) (*GrpcPlugin, error) {
	gp := &GrpcPlugin{
		dpuMode:       dpuMode,
		dpuIdentifier: dpuIdentifier,
		k8sClient:     client,
		log:           ctrl.Log.WithName("GrpcPlugin"),
		pathManager:   *utils.NewPathManager("/"),
	}

	for _, opt := range opts {
		opt(gp)
	}

	return gp, nil
}

const registryInitRetryInterval = 30 * time.Second

func (g *GrpcPlugin) ensureRegistryInitialized(ctx context.Context) bool {
	if g.registryPlugin == nil {
		return false
	}

	g.registryInitMutex.Lock()
	defer g.registryInitMutex.Unlock()

	if g.registryInitialized {
		return true
	}

	if !g.registryLastAttempt.IsZero() && time.Since(g.registryLastAttempt) < registryInitRetryInterval {
		return false
	}
	g.registryLastAttempt = time.Now()

	if err := g.registryPlugin.Initialize(ctx, g.registryConfig); err != nil {
		if errors.Is(err, pkgplugin.ErrAlreadyInitialized) {
			g.registryInitialized = true
			g.registryInitErr = nil
			return true
		}
		g.registryInitErr = err
		g.log.Info("Registry plugin initialization failed; falling back to VSP", "plugin", g.registryPlugin.Info().Name, "error", err)
		return false
	}

	g.registryInitialized = true
	g.registryInitErr = nil
	g.log.Info("Registry plugin initialized for hybrid runtime", "plugin", g.registryPlugin.Info().Name)
	return true
}

func (g *GrpcPlugin) registryNetworkPlugin() (pkgplugin.NetworkPlugin, bool) {
	if g.registryPlugin == nil {
		return nil, false
	}
	np, ok := g.registryPlugin.(pkgplugin.NetworkPlugin)
	return np, ok
}

func (g *GrpcPlugin) ensureConnected() error {
	if g.conn != nil {
		state := g.conn.GetState()
		if state == connectivity.Ready || state == connectivity.Idle {
			if g.client != nil {
				return nil
			}
		} else {
			g.log.Info("gRPC connection not ready, reconnecting", "state", state)
			_ = g.conn.Close()
			g.conn = nil
			g.client = nil
			g.nfclient = nil
			g.opiClient = nil
			g.dsClient = nil
		}
	}
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return net.Dial("unix", addr)
		}),
	}

	conn, err := grpc.DialContext(context.Background(), g.pathManager.VendorPluginSocket(), dialOptions...)

	if err != nil {
		g.log.Error(err, "Failed to connect to vendor plugin")
		return err
	}
	g.conn = conn

	g.client = pb.NewLifeCycleServiceClient(conn)
	g.nfclient = nfapi.NewNetworkFunctionServiceClient(conn)
	g.opiClient = opi.NewBridgePortServiceClient(conn)
	g.dsClient = pb.NewDeviceServiceClient(conn)
	return nil
}

func (g *GrpcPlugin) CreateBridgePort(createRequest *opi.CreateBridgePortRequest) (*opi.BridgePort, error) {
	if createRequest == nil {
		return nil, fmt.Errorf("CreateBridgePort request is nil")
	}

	if g.ensureRegistryInitialized(context.Background()) {
		if networkPlugin, ok := g.registryNetworkPlugin(); ok {
			bridgeReq := bridgePortRequestFromOPI(createRequest)
			port, err := networkPlugin.CreateBridgePort(context.Background(), bridgeReq)
			if err == nil {
				return bridgePortToOPI(port, bridgeReq.Name), nil
			}
			if pkgplugin.IsNotImplemented(err) || pkgplugin.IsCapabilityNotSupported(err) {
				g.log.Info("Registry plugin CreateBridgePort not implemented; falling back to VSP", "error", err)
			} else {
				return nil, err
			}
		}
	}

	err := g.ensureConnected()
	if err != nil {
		return nil, fmt.Errorf("CreateBridgePort failed to ensure GRPC connection: %v", err)
	}
	return g.opiClient.CreateBridgePort(context.TODO(), createRequest)
}

func (g *GrpcPlugin) DeleteBridgePort(deleteRequest *opi.DeleteBridgePortRequest) error {
	if deleteRequest == nil {
		return fmt.Errorf("DeleteBridgePort request is nil")
	}

	if g.ensureRegistryInitialized(context.Background()) {
		if networkPlugin, ok := g.registryNetworkPlugin(); ok {
			err := networkPlugin.DeleteBridgePort(context.Background(), deleteRequest.Name)
			if err == nil {
				return nil
			}
			if pkgplugin.IsNotImplemented(err) || pkgplugin.IsCapabilityNotSupported(err) {
				g.log.Info("Registry plugin DeleteBridgePort not implemented; falling back to VSP", "error", err)
			} else {
				return err
			}
		}
	}

	err := g.ensureConnected()
	if err != nil {
		return fmt.Errorf("DeleteBridgePort failed to ensure GRPC connection: %v", err)
	}
	_, err = g.opiClient.DeleteBridgePort(context.TODO(), deleteRequest)
	return err
}

func (g *GrpcPlugin) CreateNetworkFunction(input string, output string) error {
	g.log.Info("CreateNetworkFunction", "input", input, "output", output)

	if g.ensureRegistryInitialized(context.Background()) {
		if networkPlugin, ok := g.registryNetworkPlugin(); ok {
			if err := networkPlugin.CreateNetworkFunction(context.Background(), input, output); err == nil {
				return nil
			} else if pkgplugin.IsNotImplemented(err) || pkgplugin.IsCapabilityNotSupported(err) {
				g.log.Info("Registry plugin CreateNetworkFunction not implemented; falling back to VSP", "error", err)
			} else {
				return err
			}
		}
	}

	err := g.ensureConnected()
	if err != nil {
		return fmt.Errorf("CreateNetworkFunction failed to ensure GRPC connection: %v", err)
	}
	req := nfapi.NFRequest{Input: input, Output: output}
	_, err = g.nfclient.CreateNetworkFunction(context.TODO(), &req)
	return err
}

func (g *GrpcPlugin) DeleteNetworkFunction(input string, output string) error {
	g.log.Info("DeleteNetworkFunction", "input", input, "output", output)

	if g.ensureRegistryInitialized(context.Background()) {
		if networkPlugin, ok := g.registryNetworkPlugin(); ok {
			if err := networkPlugin.DeleteNetworkFunction(context.Background(), input, output); err == nil {
				return nil
			} else if pkgplugin.IsNotImplemented(err) || pkgplugin.IsCapabilityNotSupported(err) {
				g.log.Info("Registry plugin DeleteNetworkFunction not implemented; falling back to VSP", "error", err)
			} else {
				return err
			}
		}
	}

	err := g.ensureConnected()
	if err != nil {
		return fmt.Errorf("DeleteNetworkFunction failed to ensure GRPC connection: %v", err)
	}
	req := nfapi.NFRequest{Input: input, Output: output}
	_, err = g.nfclient.DeleteNetworkFunction(context.TODO(), &req)
	return err
}

func (g *GrpcPlugin) GetDevices() (*pb.DeviceListResponse, error) {
	if g.ensureRegistryInitialized(context.Background()) {
		devices, err := g.registryPlugin.DiscoverDevices(context.Background())
		if err == nil && len(devices) > 0 {
			return devicesToLifecycleResponse(devices), nil
		}
		if err != nil && !pkgplugin.IsNotImplemented(err) && !pkgplugin.IsCapabilityNotSupported(err) {
			g.log.Info("Registry plugin DiscoverDevices failed; falling back to VSP", "error", err)
		}
		if err == nil && len(devices) == 0 {
			g.log.Info("Registry plugin returned no devices; falling back to VSP")
		}
	}

	err := g.ensureConnected()
	if err != nil {
		return nil, fmt.Errorf("GetDevices failed to ensure GRPC connection: %v", err)
	}
	return g.dsClient.GetDevices(context.Background(), &emptypb.Empty{})
}

func (g *GrpcPlugin) SetNumVfs(count int32) (*pb.VfCount, error) {
	if g.ensureRegistryInitialized(context.Background()) {
		if networkPlugin, ok := g.registryNetworkPlugin(); ok {
			devices, err := g.registryPlugin.DiscoverDevices(context.Background())
			if err == nil && len(devices) > 0 {
				// The legacy VSP path applies VF changes at the device level without a specific ID.
				// For the registry plugin, apply to the device that best matches the DPU identifier.
				target := selectDeviceForIdentifier(string(g.dpuIdentifier), devices)
				targetID := resolveDeviceID(target)
				if err := networkPlugin.SetVFCount(context.Background(), targetID, int(count)); err == nil {
					return &pb.VfCount{VfCnt: count}, nil
				} else if pkgplugin.IsNotImplemented(err) || pkgplugin.IsCapabilityNotSupported(err) {
					g.log.Info("Registry plugin SetVFCount not implemented; falling back to VSP", "error", err)
				} else {
					return nil, err
				}
			} else if err != nil {
				g.log.Info("Registry plugin DiscoverDevices failed; falling back to VSP", "error", err)
			} else {
				g.log.Info("Registry plugin returned no devices; falling back to VSP")
			}
		}
	}

	err := g.ensureConnected()
	if err != nil {
		return nil, fmt.Errorf("SetNumvfs failed to ensure GRPC connection: %v", err)
	}
	c := &pb.VfCount{
		VfCnt: count,
	}
	return g.dsClient.SetNumVfs(context.Background(), c)
}

func resolveDeviceID(device pkgplugin.Device) string {
	if device.ID != "" {
		return device.ID
	}
	if device.PCIAddress != "" {
		return device.PCIAddress
	}
	if device.PCIID.VendorID != "" || device.PCIID.DeviceID != "" {
		return device.PCIID.String()
	}
	return "unknown"
}

func selectDeviceForIdentifier(identifier string, devices []pkgplugin.Device) pkgplugin.Device {
	if len(devices) == 0 {
		return pkgplugin.Device{}
	}
	if identifier == "" {
		return devices[0]
	}

	candidates := identifierCandidates(identifier)

	// Prefer exact matches first.
	for _, candidate := range candidates {
		for _, device := range devices {
			if device.ID != "" && candidate == normalizeIdentifier(device.ID) {
				return device
			}
			if device.SerialNumber != "" && candidate == normalizeIdentifier(device.SerialNumber) {
				return device
			}
			if device.PCIAddress != "" && candidate == normalizeIdentifier(device.PCIAddress) {
				return device
			}
		}
	}

	// Fallback to a contains match for compatibility.
	for _, candidate := range candidates {
		for _, device := range devices {
			if device.ID != "" && strings.Contains(candidate, normalizeIdentifier(device.ID)) {
				return device
			}
			if device.SerialNumber != "" && strings.Contains(candidate, normalizeIdentifier(device.SerialNumber)) {
				return device
			}
			if device.PCIAddress != "" && strings.Contains(candidate, normalizeIdentifier(device.PCIAddress)) {
				return device
			}
		}
	}

	return devices[0]
}

func normalizeIdentifier(value string) string {
	value = strings.ToLower(value)
	return strings.ReplaceAll(value, ":", "-")
}

func identifierCandidates(identifier string) []string {
	normalized := normalizeIdentifier(identifier)
	candidates := []string{normalized}

	prefixes := []string{
		"nvidia-bf-",
		"xsight-",
		"mangoboost-",
		"intel-ipu-",
		"intel-netsec-",
		"marvell-dpu-",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			candidates = append(candidates, strings.TrimPrefix(normalized, prefix))
			break
		}
	}

	if strings.HasSuffix(normalized, "-dpu") {
		candidates = append(candidates, strings.TrimSuffix(normalized, "-dpu"))
	}

	return candidates
}

func bridgePortRequestFromOPI(req *opi.CreateBridgePortRequest) *pkgplugin.BridgePortRequest {
	if req == nil {
		return &pkgplugin.BridgePortRequest{}
	}

	name := req.BridgePortId
	mac := ""
	var vlanID *int
	portType := ""
	metadata := map[string]string{}

	if req.BridgePort != nil {
		if req.BridgePort.Name != "" {
			name = req.BridgePort.Name
		}
		if req.BridgePort.Spec != nil {
			if len(req.BridgePort.Spec.MacAddress) > 0 {
				mac = net.HardwareAddr(req.BridgePort.Spec.MacAddress).String()
			}
			if req.BridgePort.Spec.Ptype != opi.BridgePortType_BRIDGE_PORT_TYPE_UNSPECIFIED {
				portType = strings.ToLower(req.BridgePort.Spec.Ptype.String())
			}
			if len(req.BridgePort.Spec.LogicalBridges) > 0 {
				metadata["logical_bridges"] = strings.Join(req.BridgePort.Spec.LogicalBridges, ",")
				if len(req.BridgePort.Spec.LogicalBridges) == 1 {
					if parsed, err := strconv.Atoi(req.BridgePort.Spec.LogicalBridges[0]); err == nil {
						vlanID = &parsed
					}
				}
			}
		}
	}

	return &pkgplugin.BridgePortRequest{
		Name:       name,
		MACAddress: mac,
		VLANID:     vlanID,
		Type:       portType,
		Metadata:   metadata,
	}
}

func bridgePortToOPI(port *pkgplugin.BridgePort, fallbackName string) *opi.BridgePort {
	if port == nil {
		return &opi.BridgePort{Name: fallbackName}
	}

	name := port.ID
	if name == "" {
		name = port.Name
	}
	if name == "" {
		name = fallbackName
	}

	spec := &opi.BridgePortSpec{}
	if port.MACAddress != "" {
		if parsed, err := net.ParseMAC(port.MACAddress); err == nil {
			spec.MacAddress = parsed
		}
	}
	if port.VLANID != nil {
		spec.LogicalBridges = []string{strconv.Itoa(*port.VLANID)}
	}

	switch strings.ToLower(port.Type) {
	case "trunk", "bridge_port_type_trunk", "bridge-port-type-trunk":
		spec.Ptype = opi.BridgePortType_BRIDGE_PORT_TYPE_TRUNK
	case "access", "bridge_port_type_access", "bridge-port-type-access":
		spec.Ptype = opi.BridgePortType_BRIDGE_PORT_TYPE_ACCESS
	default:
		if spec.Ptype == opi.BridgePortType_BRIDGE_PORT_TYPE_UNSPECIFIED {
			spec.Ptype = opi.BridgePortType_BRIDGE_PORT_TYPE_ACCESS
		}
	}

	return &opi.BridgePort{
		Name: name,
		Spec: spec,
	}
}

func devicesToLifecycleResponse(devices []pkgplugin.Device) *pb.DeviceListResponse {
	resp := &pb.DeviceListResponse{
		Devices: make(map[string]*pb.Device),
	}
	for _, dev := range devices {
		deviceID := resolveDeviceID(dev)
		health := "unhealthy"
		if dev.Healthy {
			health = "healthy"
		}
		resp.Devices[deviceID] = &pb.Device{
			ID:     deviceID,
			Health: health,
			Topology: &pb.TopologyInfo{
				Node: dev.PCIAddress,
			},
		}
	}
	return resp
}

// IsInitialized returns true if the VSP has been successfully initialized
func (g *GrpcPlugin) IsInitialized() bool {
	g.initMutex.RLock()
	defer g.initMutex.RUnlock()
	return g.initialized
}

// SetInitDone sets the initialization status with proper mutex locking
func (g *GrpcPlugin) SetInitDone(initialized bool) {
	g.initMutex.Lock()
	defer g.initMutex.Unlock()
	g.initialized = initialized
}
