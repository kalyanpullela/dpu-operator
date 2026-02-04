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

package nvidia

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
}

func (m *mockLifecycleServer) Init(ctx context.Context, req *lifecyclepb.InitRequest) (*lifecyclepb.IpPort, error) {
	return &lifecyclepb.IpPort{Ip: "127.0.0.1", Port: 50051}, nil
}

func (m *mockLifecycleServer) GetDevices(ctx context.Context, req *lifecyclepb.GetDevicesRequest) (*lifecyclepb.DeviceListResponse, error) {
	return &lifecyclepb.DeviceListResponse{
		Devices: map[string]*lifecyclepb.Device{
			"device-1": {Id: "device-1", Health: "healthy"},
		},
	}, nil
}

func (m *mockLifecycleServer) SetNumVfs(ctx context.Context, req *lifecyclepb.SetNumVfsRequest) (*lifecyclepb.VfCount, error) {
	return &lifecyclepb.VfCount{VfCnt: req.VfCnt}, nil
}

func (m *mockLifecycleServer) Ping(ctx context.Context, req *lifecyclepb.PingRequest) (*lifecyclepb.PingResponse, error) {
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
	return req.BridgePort, nil
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
	return req.LogicalBridge, nil
}

func (m *mockNetworkServer) GetLogicalBridge(ctx context.Context, req *evpnpb.GetLogicalBridgeRequest) (*evpnpb.LogicalBridge, error) {
	return &evpnpb.LogicalBridge{Name: req.Name}, nil
}

func (m *mockNetworkServer) DeleteLogicalBridge(ctx context.Context, req *evpnpb.DeleteLogicalBridgeRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockNetworkServer) CreateVrf(ctx context.Context, req *evpnpb.CreateVrfRequest) (*evpnpb.Vrf, error) {
	return req.Vrf, nil
}

func (m *mockNetworkServer) GetVrf(ctx context.Context, req *evpnpb.GetVrfRequest) (*evpnpb.Vrf, error) {
	return &evpnpb.Vrf{Name: req.Name}, nil
}

func (m *mockNetworkServer) DeleteVrf(ctx context.Context, req *evpnpb.DeleteVrfRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (m *mockNetworkServer) CreateSvi(ctx context.Context, req *evpnpb.CreateSviRequest) (*evpnpb.Svi, error) {
	return req.Svi, nil
}

func (m *mockNetworkServer) GetSvi(ctx context.Context, req *evpnpb.GetSviRequest) (*evpnpb.Svi, error) {
	return &evpnpb.Svi{Name: req.Name}, nil
}

func (m *mockNetworkServer) DeleteSvi(ctx context.Context, req *evpnpb.DeleteSviRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// startMockServer starts a mock gRPC server and returns the server address and cleanup function
func startMockServer(t *testing.T) (string, func()) {
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
			// t.Logf("Server stopped: %v", err)
		}
	}()

	// Verify connection
	conn, err := grpc.Dial(listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	conn.Close()

	address := listener.Addr().String()

	cleanup := func() {
		server.Stop()
	}

	return address, cleanup
}
