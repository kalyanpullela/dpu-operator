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
	"errors"
	"fmt"
)

// Common errors for plugin operations.
var (
	// ErrNotImplemented indicates the operation is not implemented by this plugin.
	ErrNotImplemented = errors.New("operation not implemented")

	// ErrPluginNotFound indicates the requested plugin was not found.
	ErrPluginNotFound = errors.New("plugin not found")

	// ErrDeviceNotFound indicates the requested device was not found.
	ErrDeviceNotFound = errors.New("device not found")

	// ErrNotInitialized indicates the plugin has not been initialized.
	ErrNotInitialized = errors.New("plugin not initialized")

	// ErrAlreadyInitialized indicates the plugin was already initialized.
	ErrAlreadyInitialized = errors.New("plugin already initialized")

	// ErrCapabilityNotSupported indicates the requested capability is not supported.
	ErrCapabilityNotSupported = errors.New("capability not supported")

	// ErrInvalidConfig indicates the plugin configuration is invalid.
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrConnectionFailed indicates a connection to the OPI endpoint failed.
	ErrConnectionFailed = errors.New("connection failed")

	// ErrResourceNotFound indicates a resource (port, subsystem, tunnel) was not found.
	ErrResourceNotFound = errors.New("resource not found")

	// ErrResourceExists indicates a resource already exists.
	ErrResourceExists = errors.New("resource already exists")

	// ErrOperationFailed indicates a generic operation failure.
	ErrOperationFailed = errors.New("operation failed")
)

// PluginError wraps an error with additional context about the plugin.
type PluginError struct {
	Plugin  string
	Op      string
	Err     error
	Details string
}

func (e *PluginError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("plugin %s: %s: %v (%s)", e.Plugin, e.Op, e.Err, e.Details)
	}
	return fmt.Sprintf("plugin %s: %s: %v", e.Plugin, e.Op, e.Err)
}

func (e *PluginError) Unwrap() error {
	return e.Err
}

// NewPluginError creates a new PluginError.
func NewPluginError(plugin, op string, err error) *PluginError {
	return &PluginError{
		Plugin: plugin,
		Op:     op,
		Err:    err,
	}
}

// NewPluginErrorWithDetails creates a PluginError with additional details.
func NewPluginErrorWithDetails(plugin, op string, err error, details string) *PluginError {
	return &PluginError{
		Plugin:  plugin,
		Op:      op,
		Err:     err,
		Details: details,
	}
}

// DeviceError wraps an error with device context.
type DeviceError struct {
	DeviceID string
	Op       string
	Err      error
}

func (e *DeviceError) Error() string {
	return fmt.Sprintf("device %s: %s: %v", e.DeviceID, e.Op, e.Err)
}

func (e *DeviceError) Unwrap() error {
	return e.Err
}

// NewDeviceError creates a new DeviceError.
func NewDeviceError(deviceID, op string, err error) *DeviceError {
	return &DeviceError{
		DeviceID: deviceID,
		Op:       op,
		Err:      err,
	}
}

// IsNotImplemented checks if an error indicates an unimplemented operation.
func IsNotImplemented(err error) bool {
	return errors.Is(err, ErrNotImplemented)
}

// IsNotFound checks if an error indicates a resource was not found.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrPluginNotFound) ||
		errors.Is(err, ErrDeviceNotFound) ||
		errors.Is(err, ErrResourceNotFound)
}

// IsCapabilityNotSupported checks if an error indicates unsupported capability.
func IsCapabilityNotSupported(err error) bool {
	return errors.Is(err, ErrCapabilityNotSupported)
}
