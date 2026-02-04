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

package opi

import (
	"context"
	"net"
	"testing"

	evpnpb "github.com/opiproject/opi-api/network/evpn-gw/v1alpha1/gen/go"
	lifecyclepb "github.com/opiproject/opi-api/v1/gen/go/lifecycle"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockLifecycleServer implements the lifecycle service for testing
type mockLifecycleServer struct {
	lifecyclepb.UnimplementedLifeCycleServiceServer
	lifecyclepb.UnimplementedDeviceServiceServer
	lifecyclepb.UnimplementedHeartbeatServiceServer

	initResponse    *lifecyclepb.IpPort
	devicesResponse *lifecyclepb.DeviceListResponse
	vfCountResponse *lifecyclepb.VfCount
	pingResponse    *lifecyclepb.PingResponse
}

func (m *mockLifecycleServer) Init(ctx context.Context, req *lifecyclepb.InitRequest) (*lifecyclepb.IpPort, error) {
	if m.initResponse != nil {
		return m.initResponse, nil
	}
	return &lifecyclepb.IpPort{Ip: "192.168.1.1", Port: 50051}, nil
}

func (m *mockLifecycleServer) GetDevices(ctx context.Context, req *lifecyclepb.GetDevicesRequest) (*lifecyclepb.DeviceListResponse, error) {
	if m.devicesResponse != nil {
		return m.devicesResponse, nil
	}
	return &lifecyclepb.DeviceListResponse{
		Devices: map[string]*lifecyclepb.Device{
			"dev0": {Id: "dev0", Health: "healthy"},
		},
	}, nil
}

func (m *mockLifecycleServer) SetNumVfs(ctx context.Context, req *lifecyclepb.SetNumVfsRequest) (*lifecyclepb.VfCount, error) {
	if m.vfCountResponse != nil {
		return m.vfCountResponse, nil
	}
	return &lifecyclepb.VfCount{VfCnt: req.VfCnt}, nil
}

func (m *mockLifecycleServer) Ping(ctx context.Context, req *lifecyclepb.PingRequest) (*lifecyclepb.PingResponse, error) {
	if m.pingResponse != nil {
		return m.pingResponse, nil
	}
	return &lifecyclepb.PingResponse{Healthy: true}, nil
}

// mockNetworkServer implements the network service for testing
type mockNetworkServer struct {
	evpnpb.UnimplementedBridgePortServiceServer
	evpnpb.UnimplementedLogicalBridgeServiceServer
	evpnpb.UnimplementedVrfServiceServer
	evpnpb.UnimplementedSviServiceServer
}

func (m *mockNetworkServer) CreateBridgePort(ctx context.Context, req *evpnpb.CreateBridgePortRequest) (*evpnpb.BridgePort, error) {
	return &evpnpb.BridgePort{Name: "bridgePorts/" + req.BridgePortId}, nil
}

func (m *mockNetworkServer) GetBridgePort(ctx context.Context, req *evpnpb.GetBridgePortRequest) (*evpnpb.BridgePort, error) {
	return &evpnpb.BridgePort{Name: req.Name}, nil
}

func (m *mockNetworkServer) ListBridgePorts(ctx context.Context, req *evpnpb.ListBridgePortsRequest) (*evpnpb.ListBridgePortsResponse, error) {
	return &evpnpb.ListBridgePortsResponse{}, nil
}

func (m *mockNetworkServer) DeleteBridgePort(ctx context.Context, req *evpnpb.DeleteBridgePortRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockNetworkServer) CreateLogicalBridge(ctx context.Context, req *evpnpb.CreateLogicalBridgeRequest) (*evpnpb.LogicalBridge, error) {
	return &evpnpb.LogicalBridge{Name: "logicalBridges/" + req.LogicalBridgeId}, nil
}

func (m *mockNetworkServer) GetLogicalBridge(ctx context.Context, req *evpnpb.GetLogicalBridgeRequest) (*evpnpb.LogicalBridge, error) {
	return &evpnpb.LogicalBridge{Name: req.Name}, nil
}

func (m *mockNetworkServer) DeleteLogicalBridge(ctx context.Context, req *evpnpb.DeleteLogicalBridgeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockNetworkServer) CreateVrf(ctx context.Context, req *evpnpb.CreateVrfRequest) (*evpnpb.Vrf, error) {
	return &evpnpb.Vrf{Name: "vrfs/" + req.VrfId}, nil
}

func (m *mockNetworkServer) GetVrf(ctx context.Context, req *evpnpb.GetVrfRequest) (*evpnpb.Vrf, error) {
	return &evpnpb.Vrf{Name: req.Name}, nil
}

func (m *mockNetworkServer) DeleteVrf(ctx context.Context, req *evpnpb.DeleteVrfRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockNetworkServer) CreateSvi(ctx context.Context, req *evpnpb.CreateSviRequest) (*evpnpb.Svi, error) {
	return &evpnpb.Svi{Name: "svis/" + req.SviId}, nil
}

func (m *mockNetworkServer) GetSvi(ctx context.Context, req *evpnpb.GetSviRequest) (*evpnpb.Svi, error) {
	return &evpnpb.Svi{Name: req.Name}, nil
}

func (m *mockNetworkServer) DeleteSvi(ctx context.Context, req *evpnpb.DeleteSviRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// startMockServer starts a mock gRPC server and returns the client connection
func startMockServer(t *testing.T) (*grpc.ClientConn, func()) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	server := grpc.NewServer()
	mockLifecycle := &mockLifecycleServer{}
	mockNetwork := &mockNetworkServer{}

	lifecyclepb.RegisterLifeCycleServiceServer(server, mockLifecycle)
	lifecyclepb.RegisterDeviceServiceServer(server, mockLifecycle)
	lifecyclepb.RegisterHeartbeatServiceServer(server, mockLifecycle)
	evpnpb.RegisterBridgePortServiceServer(server, mockNetwork)
	evpnpb.RegisterLogicalBridgeServiceServer(server, mockNetwork)
	evpnpb.RegisterVrfServiceServer(server, mockNetwork)
	evpnpb.RegisterSviServiceServer(server, mockNetwork)

	go func() {
		if err := server.Serve(listener); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	conn, err := grpc.NewClient(listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	cleanup := func() {
		conn.Close()
		server.Stop()
	}

	return conn, cleanup
}

func TestNewClientWithConn(t *testing.T) {
	conn, cleanup := startMockServer(t)
	defer cleanup()

	client := NewClientWithConn(conn)
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	if !client.IsConnected() {
		t.Error("Expected client to be connected")
	}
	if client.Endpoint() != "mock" {
		t.Errorf("Expected endpoint 'mock', got %s", client.Endpoint())
	}
}

func TestLifecycleClient_GetDevices(t *testing.T) {
	conn, cleanup := startMockServer(t)
	defer cleanup()

	client := NewClientWithConn(conn)
	ctx := context.Background()

	resp, err := client.Lifecycle().GetDevices(ctx)
	if err != nil {
		t.Fatalf("GetDevices failed: %v", err)
	}
	if len(resp.Devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(resp.Devices))
	}
	if resp.Devices["dev0"].Id != "dev0" {
		t.Errorf("Expected device ID 'dev0', got %s", resp.Devices["dev0"].Id)
	}
}

func TestLifecycleClient_SetNumVfs(t *testing.T) {
	conn, cleanup := startMockServer(t)
	defer cleanup()

	client := NewClientWithConn(conn)
	ctx := context.Background()

	resp, err := client.Lifecycle().SetNumVfs(ctx, 16)
	if err != nil {
		t.Fatalf("SetNumVfs failed: %v", err)
	}
	if resp.VfCnt != 16 {
		t.Errorf("Expected VfCnt 16, got %d", resp.VfCnt)
	}
}

func TestLifecycleClient_Ping(t *testing.T) {
	conn, cleanup := startMockServer(t)
	defer cleanup()

	client := NewClientWithConn(conn)
	ctx := context.Background()

	resp, err := client.Lifecycle().Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if !resp.Healthy {
		t.Error("Expected healthy response")
	}
}

func TestLifecycleClient_Init(t *testing.T) {
	conn, cleanup := startMockServer(t)
	defer cleanup()

	client := NewClientWithConn(conn)
	ctx := context.Background()

	req := &lifecyclepb.InitRequest{
		DpuMode:       true,
		DpuIdentifier: "test-dpu-1",
	}
	resp, err := client.Lifecycle().Init(ctx, req)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if resp.Ip != "192.168.1.1" {
		t.Errorf("Expected IP 192.168.1.1, got %s", resp.Ip)
	}
	if resp.Port != 50051 {
		t.Errorf("Expected port 50051, got %d", resp.Port)
	}
}

func TestNetworkClient_BridgePort(t *testing.T) {
	conn, cleanup := startMockServer(t)
	defer cleanup()

	client := NewClientWithConn(conn)
	ctx := context.Background()

	// Create
	createReq := &evpnpb.CreateBridgePortRequest{
		BridgePortId: "bp1",
		BridgePort:   &evpnpb.BridgePort{},
	}
	created, err := client.Network().CreateBridgePort(ctx, createReq)
	if err != nil {
		t.Fatalf("CreateBridgePort failed: %v", err)
	}
	if created.Name != "bridgePorts/bp1" {
		t.Errorf("Expected name 'bridgePorts/bp1', got %s", created.Name)
	}

	// Get
	got, err := client.Network().GetBridgePort(ctx, "bridgePorts/bp1")
	if err != nil {
		t.Fatalf("GetBridgePort failed: %v", err)
	}
	if got.Name != "bridgePorts/bp1" {
		t.Errorf("Expected name 'bridgePorts/bp1', got %s", got.Name)
	}

	// List
	list, err := client.Network().ListBridgePorts(ctx)
	if err != nil {
		t.Fatalf("ListBridgePorts failed: %v", err)
	}
	if list == nil {
		t.Error("Expected non-nil list response")
	}

	// Delete
	err = client.Network().DeleteBridgePort(ctx, "bridgePorts/bp1")
	if err != nil {
		t.Fatalf("DeleteBridgePort failed: %v", err)
	}
}

func TestNetworkClient_LogicalBridge(t *testing.T) {
	conn, cleanup := startMockServer(t)
	defer cleanup()

	client := NewClientWithConn(conn)
	ctx := context.Background()

	// Create
	createReq := &evpnpb.CreateLogicalBridgeRequest{
		LogicalBridgeId: "lb1",
		LogicalBridge:   &evpnpb.LogicalBridge{},
	}
	created, err := client.Network().CreateLogicalBridge(ctx, createReq)
	if err != nil {
		t.Fatalf("CreateLogicalBridge failed: %v", err)
	}
	if created.Name != "logicalBridges/lb1" {
		t.Errorf("Expected name 'logicalBridges/lb1', got %s", created.Name)
	}

	// Get
	got, err := client.Network().GetLogicalBridge(ctx, "logicalBridges/lb1")
	if err != nil {
		t.Fatalf("GetLogicalBridge failed: %v", err)
	}
	if got.Name != "logicalBridges/lb1" {
		t.Errorf("Expected name 'logicalBridges/lb1', got %s", got.Name)
	}

	// Delete
	err = client.Network().DeleteLogicalBridge(ctx, "logicalBridges/lb1")
	if err != nil {
		t.Fatalf("DeleteLogicalBridge failed: %v", err)
	}
}

func TestNetworkClient_Vrf(t *testing.T) {
	conn, cleanup := startMockServer(t)
	defer cleanup()

	client := NewClientWithConn(conn)
	ctx := context.Background()

	// Create
	createReq := &evpnpb.CreateVrfRequest{
		VrfId: "vrf1",
		Vrf:   &evpnpb.Vrf{},
	}
	created, err := client.Network().CreateVrf(ctx, createReq)
	if err != nil {
		t.Fatalf("CreateVrf failed: %v", err)
	}
	if created.Name != "vrfs/vrf1" {
		t.Errorf("Expected name 'vrfs/vrf1', got %s", created.Name)
	}

	// Get
	got, err := client.Network().GetVrf(ctx, "vrfs/vrf1")
	if err != nil {
		t.Fatalf("GetVrf failed: %v", err)
	}
	if got.Name != "vrfs/vrf1" {
		t.Errorf("Expected name 'vrfs/vrf1', got %s", got.Name)
	}

	// Delete
	err = client.Network().DeleteVrf(ctx, "vrfs/vrf1")
	if err != nil {
		t.Fatalf("DeleteVrf failed: %v", err)
	}
}

func TestNetworkClient_Svi(t *testing.T) {
	conn, cleanup := startMockServer(t)
	defer cleanup()

	client := NewClientWithConn(conn)
	ctx := context.Background()

	// Create
	createReq := &evpnpb.CreateSviRequest{
		SviId: "svi1",
		Svi:   &evpnpb.Svi{},
	}
	created, err := client.Network().CreateSvi(ctx, createReq)
	if err != nil {
		t.Fatalf("CreateSvi failed: %v", err)
	}
	if created.Name != "svis/svi1" {
		t.Errorf("Expected name 'svis/svi1', got %s", created.Name)
	}

	// Get
	got, err := client.Network().GetSvi(ctx, "svis/svi1")
	if err != nil {
		t.Fatalf("GetSvi failed: %v", err)
	}
	if got.Name != "svis/svi1" {
		t.Errorf("Expected name 'svis/svi1', got %s", got.Name)
	}

	// Delete
	err = client.Network().DeleteSvi(ctx, "svis/svi1")
	if err != nil {
		t.Fatalf("DeleteSvi failed: %v", err)
	}
}

func TestClient_Close(t *testing.T) {
	conn, cleanup := startMockServer(t)
	defer cleanup()

	client := NewClientWithConn(conn)
	err := client.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestClient_CloseNil(t *testing.T) {
	client := &Client{}
	err := client.Close()
	if err != nil {
		t.Errorf("Close on nil connection should not error: %v", err)
	}
}
