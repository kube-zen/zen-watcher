// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"fmt"
	"sync"
)

// DynamicEndpointRegistry manages dynamic webhook endpoints
type DynamicEndpointRegistry struct {
	mu        sync.RWMutex
	endpoints map[string]*EndpointConfig
}

// EndpointConfig represents configuration for a dynamic webhook endpoint
type EndpointConfig struct {
	Source     string            `json:"source"`
	Path       string            `json:"path"`
	Methods    []string          `json:"methods"`
	Auth       *AuthConfig       `json:"auth,omitempty"`
	RateLimit  *RateLimitConfig  `json:"rateLimit,omitempty"`
	Validation *ValidationConfig `json:"validation,omitempty"`
	Processing *ProcessingConfig `json:"processing,omitempty"`
}

// AuthConfig represents authentication configuration for an endpoint
type AuthConfig struct {
	Enabled    bool   `json:"enabled"`
	Type       string `json:"type"` // "apiKey", "bearer", "none"
	HeaderName string `json:"headerName,omitempty"`
	APIKey     string `json:"apiKey,omitempty"`     // From secret
	Secret     string `json:"secret,omitempty"`     // From secret
	SecretName string `json:"secretName,omitempty"` // Kubernetes secret name
	SecretKey  string `json:"secretKey,omitempty"`  // Key in secret
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled bool `json:"enabled"`
	RPM     int  `json:"rpm"`   // Requests per minute
	Burst   int  `json:"burst"` // Burst allowance
}

// ValidationConfig represents payload validation configuration
type ValidationConfig struct {
	Enabled     bool             `json:"enabled"`
	SchemaRef   string           `json:"schemaRef,omitempty"`
	Required    []string         `json:"required,omitempty"`
	CustomRules []ValidationRule `json:"customRules,omitempty"`
}

// ValidationRule represents a custom validation rule
type ValidationRule struct {
	Field    string `json:"field"`
	Required bool   `json:"required"`
	Type     string `json:"type"` // string, number, boolean, array, object
	Pattern  string `json:"pattern,omitempty"`
	Min      int    `json:"min,omitempty"`
	Max      int    `json:"max,omitempty"`
}

// ProcessingConfig represents processing configuration for the endpoint
type ProcessingConfig struct {
	Filters []FilterRule   `json:"filters,omitempty"`
	Dedup   *DedupConfig   `json:"dedup,omitempty"`
	Outputs []OutputConfig `json:"outputs,omitempty"`
}

// FilterRule represents a filter rule
type FilterRule struct {
	Field    string   `json:"field"`
	Operator string   `json:"operator"` // in, not_in, equals, not_equals, gt, lt
	Values   []string `json:"values,omitempty"`
	Value    string   `json:"value,omitempty"`
}

// DedupConfig represents deduplication configuration
type DedupConfig struct {
	Enabled  bool   `json:"enabled"`
	Window   string `json:"window,omitempty"`
	Strategy string `json:"strategy,omitempty"`
}

// OutputConfig represents output configuration
type OutputConfig struct {
	Type    string                 `json:"type"`
	Value   string                 `json:"value"`
	Mapping map[string]interface{} `json:"mapping,omitempty"`
}

// NewDynamicEndpointRegistry creates a new dynamic endpoint registry
func NewDynamicEndpointRegistry() *DynamicEndpointRegistry {
	return &DynamicEndpointRegistry{
		endpoints: make(map[string]*EndpointConfig),
	}
}

// RegisterEndpoint registers a new dynamic endpoint
func (r *DynamicEndpointRegistry) RegisterEndpoint(config *EndpointConfig) error {
	if config == nil {
		return fmt.Errorf("endpoint config cannot be nil")
	}
	if config.Source == "" {
		return fmt.Errorf("endpoint source cannot be empty")
	}
	if config.Path == "" {
		return fmt.Errorf("endpoint path cannot be empty")
	}
	if len(config.Methods) == 0 {
		config.Methods = []string{"POST"}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for path conflicts
	for source, existing := range r.endpoints {
		if existing.Path == config.Path && source != config.Source {
			return fmt.Errorf("path %s already registered for source %s", config.Path, source)
		}
	}

	r.endpoints[config.Source] = config
	return nil
}

// UnregisterEndpoint removes an endpoint from the registry
func (r *DynamicEndpointRegistry) UnregisterEndpoint(source string) error {
	if source == "" {
		return fmt.Errorf("source cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.endpoints[source]; !exists {
		return fmt.Errorf("endpoint for source %s not found", source)
	}

	delete(r.endpoints, source)
	return nil
}

// GetEndpoint retrieves an endpoint configuration by source
func (r *DynamicEndpointRegistry) GetEndpoint(source string) (*EndpointConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	config, exists := r.endpoints[source]
	return config, exists
}

// GetAllEndpoints returns all registered endpoints
func (r *DynamicEndpointRegistry) GetAllEndpoints() map[string]*EndpointConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*EndpointConfig)
	for source, config := range r.endpoints {
		result[source] = config
	}
	return result
}

// GetEndpointByPath retrieves an endpoint configuration by path
func (r *DynamicEndpointRegistry) GetEndpointByPath(path string) (*EndpointConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, config := range r.endpoints {
		if config.Path == path {
			return config, true
		}
	}
	return nil, false
}

// HasEndpoint checks if an endpoint exists for a source
func (r *DynamicEndpointRegistry) HasEndpoint(source string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.endpoints[source]
	return exists
}
