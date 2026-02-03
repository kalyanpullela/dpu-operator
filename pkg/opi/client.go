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

// Package opi provides gRPC client wrappers for OPI APIs.
// This package uses the vendored OPI protobuf types for type-safe API calls.
package opi

import (
	"context"
	"fmt"
	"time"

	evpnpb "github.com/opiproject/opi-api/network/evpn-gw/v1alpha1/gen/go"
	lifecyclepb "github.com/opiproject/opi-api/v1/gen/go/lifecycle"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Client is the main OPI client that provides access to all OPI services.
type Client struct {
	conn     *grpc.ClientConn
	endpoint string
	options  *clientOptions

	// Service clients using actual OPI protobuf types
	lifecycle *LifecycleClient
	network   *NetworkClient
}

// ClientOption configures the OPI client.
type ClientOption func(*clientOptions)

type clientOptions struct {
	dialTimeout   time.Duration
	callTimeout   time.Duration
	maxRetries    int
	retryInterval time.Duration
}

// WithDialTimeout sets the connection dial timeout.
func WithDialTimeout(d time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.dialTimeout = d
	}
}

// WithCallTimeout sets the default timeout for RPC calls.
func WithCallTimeout(d time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.callTimeout = d
	}
}

// WithRetry configures retry behavior.
func WithRetry(maxRetries int, interval time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.maxRetries = maxRetries
		o.retryInterval = interval
	}
}

// NewClient creates a new OPI client connected to the specified endpoint.
func NewClient(endpoint string, opts ...ClientOption) (*Client, error) {
	options := &clientOptions{
		dialTimeout:   10 * time.Second,
		callTimeout:   30 * time.Second,
		maxRetries:    3,
		retryInterval: 1 * time.Second,
	}
	for _, opt := range opts {
		opt(options)
	}

	// Use a bounded dial so initialization doesn't hang forever if server is unavailable.
	ctx, cancel := context.WithTimeout(context.Background(), options.dialTimeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to OPI endpoint %s: %w", endpoint, err)
	}

	return newClientWithConn(conn, endpoint, options), nil
}

// NewClientWithConn creates a new OPI client from an existing gRPC connection.
// Useful for testing with mock servers.
func NewClientWithConn(conn *grpc.ClientConn) *Client {
	options := &clientOptions{
		dialTimeout:   10 * time.Second,
		callTimeout:   30 * time.Second,
		maxRetries:    3,
		retryInterval: 1 * time.Second,
	}
	return newClientWithConn(conn, "mock", options)
}

func newClientWithConn(conn *grpc.ClientConn, endpoint string, options *clientOptions) *Client {
	c := &Client{
		conn:     conn,
		endpoint: endpoint,
		options:  options,
	}

	c.lifecycle = newLifecycleClient(conn, options)
	c.network = newNetworkClient(conn, options)

	return c
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Lifecycle returns the lifecycle service client.
func (c *Client) Lifecycle() *LifecycleClient {
	return c.lifecycle
}

// Network returns the network service client.
func (c *Client) Network() *NetworkClient {
	return c.network
}

// Endpoint returns the connected endpoint.
func (c *Client) Endpoint() string {
	return c.endpoint
}

// IsConnected returns true if the client has an active connection.
func (c *Client) IsConnected() bool {
	if c.conn == nil {
		return false
	}

	// Check connection state
	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// withTimeout wraps a context with the configured call timeout
func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.options.callTimeout > 0 {
		return context.WithTimeout(ctx, c.options.callTimeout)
	}
	return ctx, func() {}
}

// LifecycleClient provides access to OPI Lifecycle APIs using actual protobuf types.
type LifecycleClient struct {
	lifecycleClient lifecyclepb.LifeCycleServiceClient
	deviceClient    lifecyclepb.DeviceServiceClient
	heartbeatClient lifecyclepb.HeartbeatServiceClient
	conn            *grpc.ClientConn
	options         *clientOptions
}

func newLifecycleClient(conn *grpc.ClientConn, opts *clientOptions) *LifecycleClient {
	return &LifecycleClient{
		lifecycleClient: lifecyclepb.NewLifeCycleServiceClient(conn),
		deviceClient:    lifecyclepb.NewDeviceServiceClient(conn),
		heartbeatClient: lifecyclepb.NewHeartbeatServiceClient(conn),
		conn:            conn,
		options:         opts,
	}
}

func (c *LifecycleClient) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.options.callTimeout > 0 {
		return context.WithTimeout(ctx, c.options.callTimeout)
	}
	return ctx, func() {}
}

// Init initializes the xPU (DPU/IPU).
func (c *LifecycleClient) Init(ctx context.Context, req *lifecyclepb.InitRequest) (*lifecyclepb.IpPort, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.lifecycleClient.Init(ctx, req)
}

// GetDevices retrieves available devices managed by the xPU.
func (c *LifecycleClient) GetDevices(ctx context.Context) (*lifecyclepb.DeviceListResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.deviceClient.GetDevices(ctx, &emptypb.Empty{})
}

// SetNumVfs configures number of virtual functions for a device.
func (c *LifecycleClient) SetNumVfs(ctx context.Context, count int32) (*lifecyclepb.VfCount, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &lifecyclepb.VfCount{
		VfCnt: count,
	}
	return c.deviceClient.SetNumVfs(ctx, req)
}

// Ping checks xPU lifecycle health status.
func (c *LifecycleClient) Ping(ctx context.Context) (*lifecyclepb.PingResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &lifecyclepb.PingRequest{}
	return c.heartbeatClient.Ping(ctx, req)
}

// NetworkClient provides access to OPI Network APIs (EVPN-GW) using actual protobuf types.
type NetworkClient struct {
	bridgePortClient    evpnpb.BridgePortServiceClient
	logicalBridgeClient evpnpb.LogicalBridgeServiceClient
	vrfClient           evpnpb.VrfServiceClient
	sviClient           evpnpb.SviServiceClient
	conn                *grpc.ClientConn
	options             *clientOptions
}

func newNetworkClient(conn *grpc.ClientConn, opts *clientOptions) *NetworkClient {
	return &NetworkClient{
		bridgePortClient:    evpnpb.NewBridgePortServiceClient(conn),
		logicalBridgeClient: evpnpb.NewLogicalBridgeServiceClient(conn),
		vrfClient:           evpnpb.NewVrfServiceClient(conn),
		sviClient:           evpnpb.NewSviServiceClient(conn),
		conn:                conn,
		options:             opts,
	}
}

func (c *NetworkClient) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.options.callTimeout > 0 {
		return context.WithTimeout(ctx, c.options.callTimeout)
	}
	return ctx, func() {}
}

// --- BridgePort Operations ---

// CreateBridgePort creates a new bridge port.
func (c *NetworkClient) CreateBridgePort(ctx context.Context, req *evpnpb.CreateBridgePortRequest) (*evpnpb.BridgePort, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.bridgePortClient.CreateBridgePort(ctx, req)
}

// GetBridgePort retrieves a bridge port by name.
func (c *NetworkClient) GetBridgePort(ctx context.Context, name string) (*evpnpb.BridgePort, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &evpnpb.GetBridgePortRequest{Name: name}
	return c.bridgePortClient.GetBridgePort(ctx, req)
}

// ListBridgePorts lists all bridge ports.
func (c *NetworkClient) ListBridgePorts(ctx context.Context) (*evpnpb.ListBridgePortsResponse, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &evpnpb.ListBridgePortsRequest{}
	return c.bridgePortClient.ListBridgePorts(ctx, req)
}

// UpdateBridgePort updates an existing bridge port.
func (c *NetworkClient) UpdateBridgePort(ctx context.Context, req *evpnpb.UpdateBridgePortRequest) (*evpnpb.BridgePort, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.bridgePortClient.UpdateBridgePort(ctx, req)
}

// DeleteBridgePort deletes a bridge port.
func (c *NetworkClient) DeleteBridgePort(ctx context.Context, name string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &evpnpb.DeleteBridgePortRequest{Name: name}
	_, err := c.bridgePortClient.DeleteBridgePort(ctx, req)
	return err
}

// --- LogicalBridge Operations ---

// CreateLogicalBridge creates a new logical bridge.
func (c *NetworkClient) CreateLogicalBridge(ctx context.Context, req *evpnpb.CreateLogicalBridgeRequest) (*evpnpb.LogicalBridge, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.logicalBridgeClient.CreateLogicalBridge(ctx, req)
}

// GetLogicalBridge retrieves a logical bridge by name.
func (c *NetworkClient) GetLogicalBridge(ctx context.Context, name string) (*evpnpb.LogicalBridge, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &evpnpb.GetLogicalBridgeRequest{Name: name}
	return c.logicalBridgeClient.GetLogicalBridge(ctx, req)
}

// DeleteLogicalBridge deletes a logical bridge.
func (c *NetworkClient) DeleteLogicalBridge(ctx context.Context, name string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &evpnpb.DeleteLogicalBridgeRequest{Name: name}
	_, err := c.logicalBridgeClient.DeleteLogicalBridge(ctx, req)
	return err
}

// --- VRF Operations ---

// CreateVrf creates a new VRF.
func (c *NetworkClient) CreateVrf(ctx context.Context, req *evpnpb.CreateVrfRequest) (*evpnpb.Vrf, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.vrfClient.CreateVrf(ctx, req)
}

// GetVrf retrieves a VRF by name.
func (c *NetworkClient) GetVrf(ctx context.Context, name string) (*evpnpb.Vrf, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &evpnpb.GetVrfRequest{Name: name}
	return c.vrfClient.GetVrf(ctx, req)
}

// DeleteVrf deletes a VRF.
func (c *NetworkClient) DeleteVrf(ctx context.Context, name string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &evpnpb.DeleteVrfRequest{Name: name}
	_, err := c.vrfClient.DeleteVrf(ctx, req)
	return err
}

// --- SVI Operations ---

// CreateSvi creates a new SVI.
func (c *NetworkClient) CreateSvi(ctx context.Context, req *evpnpb.CreateSviRequest) (*evpnpb.Svi, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.sviClient.CreateSvi(ctx, req)
}

// GetSvi retrieves an SVI by name.
func (c *NetworkClient) GetSvi(ctx context.Context, name string) (*evpnpb.Svi, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &evpnpb.GetSviRequest{Name: name}
	return c.sviClient.GetSvi(ctx, req)
}

// DeleteSvi deletes an SVI.
func (c *NetworkClient) DeleteSvi(ctx context.Context, name string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req := &evpnpb.DeleteSviRequest{Name: name}
	_, err := c.sviClient.DeleteSvi(ctx, req)
	return err
}
