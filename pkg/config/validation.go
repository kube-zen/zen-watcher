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
	// dnsSubdomainRegex matches Kubernetes DNS subdomain format (for API groups)
	// Must be a valid DNS subdomain: lowercase alphanumeric, dots, hyphens
	dnsSubdomainRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

	// resourceNameRegex matches Kubernetes resource name format
	// Must be lowercase alphanumeric with hyphens, no dots
	resourceNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
)

// ValidateGVR validates a GroupVersionResource
// Returns an error if any component is invalid
func ValidateGVR(gvr schema.GroupVersionResource) error {
	// Validate group (must be valid DNS subdomain or empty for core resources)
	if gvr.Group != "" {
		if !dnsSubdomainRegex.MatchString(gvr.Group) {
			return fmt.Errorf("invalid API group %q: must be a valid DNS subdomain (lowercase alphanumeric, dots, hyphens)", gvr.Group)
		}
	}

	// Validate version (must be non-empty and valid version string)
	if gvr.Version == "" {
		return fmt.Errorf("API version cannot be empty")
	}
	// Version should start with 'v' followed by numbers, or be a valid semver-like string
	if !strings.HasPrefix(gvr.Version, "v") {
		return fmt.Errorf("invalid API version %q: must start with 'v' (e.g., 'v1', 'v1alpha1')", gvr.Version)
	}

	// Validate resource (must be valid Kubernetes resource name)
	if gvr.Resource == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	if !resourceNameRegex.MatchString(gvr.Resource) {
		return fmt.Errorf("invalid resource name %q: must be lowercase alphanumeric with hyphens, no dots", gvr.Resource)
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
