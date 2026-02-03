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
	"strings"
	"sync"
)

// Registry is the global plugin registry that manages all vendor plugins.
// Plugins register themselves via init() functions, and the operator
// queries the registry to find appropriate plugins for discovered hardware.
type Registry struct {
	mu sync.RWMutex

	// plugins maps plugin name -> Plugin instance
	plugins map[string]Plugin

	// deviceIndex maps "vendorID:deviceID" -> plugin name for fast lookup
	deviceIndex map[string]string
}

// defaultRegistry is the singleton registry instance
var defaultRegistry = NewRegistry()

// NewRegistry creates a new empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins:     make(map[string]Plugin),
		deviceIndex: make(map[string]string),
	}
}

// DefaultRegistry returns the global default registry instance.
// This is the registry that plugins should register with during init().
func DefaultRegistry() *Registry {
	return defaultRegistry
}

// Register adds a plugin to the registry.
// This is typically called from a plugin's init() function.
// Returns an error if a plugin with the same name already exists.
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return fmt.Errorf("cannot register nil plugin")
	}

	info := p.Info()
	if info.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate plugin name
	if _, exists := r.plugins[info.Name]; exists {
		return fmt.Errorf("plugin %q already registered", info.Name)
	}

	// Validate device IDs before registering the plugin to avoid ambiguous mapping.
	for _, device := range info.SupportedDevices {
		deviceKey := strings.ToLower(device.String())
		if existingPlugin, exists := r.deviceIndex[deviceKey]; exists {
			return fmt.Errorf("device %s already claimed by plugin %s", deviceKey, existingPlugin)
		}
	}

	// Register the plugin
	r.plugins[info.Name] = p

	// Build device index for fast lookup
	for _, device := range info.SupportedDevices {
		deviceKey := strings.ToLower(device.String())
		r.deviceIndex[deviceKey] = info.Name
	}

	return nil
}

// Unregister removes a plugin from the registry.
// Returns an error if the plugin is not found.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %q not found", name)
	}

	// Remove from device index
	info := p.Info()
	for _, device := range info.SupportedDevices {
		deviceKey := strings.ToLower(device.String())
		if r.deviceIndex[deviceKey] == name {
			delete(r.deviceIndex, deviceKey)
		}
	}

	delete(r.plugins, name)
	return nil
}

// Get retrieves a plugin by name.
// Returns nil if not found.
func (r *Registry) Get(name string) Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[name]
}

// GetByDeviceID finds a plugin that supports the given PCI device ID.
// The deviceID should be in "vendorID:deviceID" format (e.g., "15b3:a2d6").
// Returns nil if no plugin supports this device.
func (r *Registry) GetByDeviceID(deviceID string) Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	deviceKey := strings.ToLower(deviceID)
	if pluginName, exists := r.deviceIndex[deviceKey]; exists {
		return r.plugins[pluginName]
	}
	return nil
}

// GetByVendorDevice finds a plugin that supports the given vendor and device IDs.
func (r *Registry) GetByVendorDevice(vendorID, deviceID string) Plugin {
	return r.GetByDeviceID(vendorID + ":" + deviceID)
}

// List returns all registered plugins.
func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	return result
}

// ListNames returns the names of all registered plugins.
func (r *Registry) ListNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		result = append(result, name)
	}
	return result
}

// Count returns the number of registered plugins.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}

// GetByCapability returns all plugins that support the given capability.
func (r *Registry) GetByCapability(cap Capability) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Plugin
	for _, p := range r.plugins {
		info := p.Info()
		for _, supported := range info.Capabilities {
			if supported == cap {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

// GetNetworkPlugins returns all plugins that implement NetworkPlugin.
func (r *Registry) GetNetworkPlugins() []NetworkPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []NetworkPlugin
	for _, p := range r.plugins {
		if np, ok := p.(NetworkPlugin); ok {
			result = append(result, np)
		}
	}
	return result
}

// GetStoragePlugins returns all plugins that implement StoragePlugin.
func (r *Registry) GetStoragePlugins() []StoragePlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []StoragePlugin
	for _, p := range r.plugins {
		if sp, ok := p.(StoragePlugin); ok {
			result = append(result, sp)
		}
	}
	return result
}

// GetSecurityPlugins returns all plugins that implement SecurityPlugin.
func (r *Registry) GetSecurityPlugins() []SecurityPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []SecurityPlugin
	for _, p := range r.plugins {
		if sp, ok := p.(SecurityPlugin); ok {
			result = append(result, sp)
		}
	}
	return result
}

// FindPluginForDevice finds the best plugin for a given PCI device.
// Returns the plugin and true if found, nil and false otherwise.
func (r *Registry) FindPluginForDevice(vendorID, deviceID string) (Plugin, bool) {
	p := r.GetByVendorDevice(vendorID, deviceID)
	if p != nil {
		return p, true
	}
	return nil, false
}

// Clear removes all plugins from the registry.
// This is primarily useful for testing.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = make(map[string]Plugin)
	r.deviceIndex = make(map[string]string)
}

// --- Convenience functions for the default registry ---

// Register adds a plugin to the default registry.
func Register(p Plugin) error {
	return defaultRegistry.Register(p)
}

// Unregister removes a plugin from the default registry.
func Unregister(name string) error {
	return defaultRegistry.Unregister(name)
}

// Get retrieves a plugin from the default registry by name.
func Get(name string) Plugin {
	return defaultRegistry.Get(name)
}

// GetByDeviceID finds a plugin in the default registry by PCI device ID.
func GetByDeviceID(deviceID string) Plugin {
	return defaultRegistry.GetByDeviceID(deviceID)
}

// List returns all plugins in the default registry.
func List() []Plugin {
	return defaultRegistry.List()
}

// GetByCapability returns all plugins with the given capability from the default registry.
func GetByCapability(cap Capability) []Plugin {
	return defaultRegistry.GetByCapability(cap)
}

// MustRegister registers a plugin and panics if registration fails.
// This is useful for init() functions where failure should be fatal.
func MustRegister(p Plugin) {
	if err := Register(p); err != nil {
		panic(fmt.Sprintf("failed to register plugin: %v", err))
	}
}
