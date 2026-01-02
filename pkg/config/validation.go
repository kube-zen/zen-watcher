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

package config

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// DNS1123SubdomainRegex matches valid Kubernetes DNS-1123 subdomain format
	// DNS-1123 subdomain: lowercase alphanumeric, '-' or '.', must start/end with alphanumeric
	DNS1123SubdomainRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

	// DNS1123LabelRegex matches valid Kubernetes DNS-1123 label format
	// DNS-1123 label: lowercase alphanumeric with hyphens, no dots
	DNS1123LabelRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
)

// validateDNS1123Subdomain validates a DNS-1123 subdomain name
func validateDNS1123Subdomain(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if !DNS1123SubdomainRegex.MatchString(name) {
		return fmt.Errorf("invalid DNS-1123 subdomain %q: must be lowercase alphanumeric with dots or hyphens, must start/end with alphanumeric", name)
	}
	return nil
}

// validateDNS1123Label validates a DNS-1123 label name
func validateDNS1123Label(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if !DNS1123LabelRegex.MatchString(name) {
		return fmt.Errorf("invalid DNS-1123 label %q: must be lowercase alphanumeric with hyphens, no dots, must start/end with alphanumeric", name)
	}
	return nil
}

// ValidateGVR validates a GroupVersionResource
// Returns an error if any component is invalid
func ValidateGVR(gvr schema.GroupVersionResource) error {
	// Validate group (must be valid DNS subdomain or empty for core resources)
	if gvr.Group != "" {
		if err := validateDNS1123Subdomain(gvr.Group); err != nil {
			return fmt.Errorf("invalid API group %q: %w", gvr.Group, err)
		}
	}

	// Validate version (must be non-empty and valid version string)
	if gvr.Version == "" {
		return fmt.Errorf("API version cannot be empty")
	}
	if !strings.HasPrefix(gvr.Version, "v") {
		return fmt.Errorf("invalid API version %q: must start with 'v' (e.g., 'v1', 'v1alpha1')", gvr.Version)
	}

	// Validate resource (must be valid Kubernetes resource name, no dots)
	if gvr.Resource == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	if err := validateDNS1123Label(gvr.Resource); err != nil {
		return fmt.Errorf("invalid resource name %q: %w", gvr.Resource, err)
	}

	return nil
}

// ValidateGVRConfig validates a GVRConfig (from destination configuration)
func ValidateGVRConfig(group, version, resource string) error {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
	return ValidateGVR(gvr)
}
