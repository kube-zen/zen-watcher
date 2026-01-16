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

package watcher

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GVRAllowlist enforces namespace + allowlist restrictions for zen-watcher GVR writes
// H037: Prevents zen-watcher from writing to arbitrary GVRs
type GVRAllowlist struct {
	allowedGVRs       map[string]bool // key: "group/version/resource"
	allowedNamespaces map[string]bool // Set of allowed namespaces
	defaultNamespace  string          // Default namespace if not specified
}

// getAllowedGVRs returns the list of allowed GVRs (internal helper)
func (a *GVRAllowlist) getAllowedGVRs() []string {
	result := make([]string, 0, len(a.allowedGVRs))
	for gvr := range a.allowedGVRs {
		result = append(result, gvr)
	}
	return result
}

// getAllowedNamespaces returns the list of allowed namespaces (internal helper)
func (a *GVRAllowlist) getAllowedNamespaces() []string {
	result := make([]string, 0, len(a.allowedNamespaces))
	for ns := range a.allowedNamespaces {
		result = append(result, ns)
	}
	return result
}

// NewGVRAllowlist creates a new GVR allowlist
// H037: Reads allowlist from environment variables
func NewGVRAllowlist() *GVRAllowlist {
	allowlist := &GVRAllowlist{
		allowedGVRs:       make(map[string]bool),
		allowedNamespaces: make(map[string]bool),
		defaultNamespace:  os.Getenv("WATCH_NAMESPACE"),
	}

	// Default allowed GVR: observations.zen.kube-zen.io (zen-watcher's own resource)
	allowlist.allowedGVRs["zen.kube-zen.io/v1/observations"] = true

	// Read allowed GVRs from environment variable (comma-separated)
	// Format: "group/version/resource,group2/version2/resource2"
	allowedGVRsEnv := os.Getenv("ALLOWED_GVRS")
	if allowedGVRsEnv != "" {
		for _, gvrStr := range strings.Split(allowedGVRsEnv, ",") {
			gvrStr = strings.TrimSpace(gvrStr)
			if gvrStr != "" {
				allowlist.allowedGVRs[gvrStr] = true
			}
		}
	}

	// Read allowed namespaces from environment variable (comma-separated)
	allowedNamespacesEnv := os.Getenv("ALLOWED_NAMESPACES")
	if allowedNamespacesEnv != "" {
		for _, ns := range strings.Split(allowedNamespacesEnv, ",") {
			ns = strings.TrimSpace(ns)
			if ns != "" {
				allowlist.allowedNamespaces[ns] = true
			}
		}
	} else {
		// Default: only allow writes to WATCH_NAMESPACE
		if allowlist.defaultNamespace != "" {
			allowlist.allowedNamespaces[allowlist.defaultNamespace] = true
		}
	}

	return allowlist
}

// IsAllowed checks if a GVR write is allowed
// H037: Validates namespace + GVR allowlist
func (a *GVRAllowlist) IsAllowed(gvr schema.GroupVersionResource, namespace string) error {
	// Check GVR allowlist
	gvrKey := fmt.Sprintf("%s/%s/%s", gvr.Group, gvr.Version, gvr.Resource)
	if !a.allowedGVRs[gvrKey] {
		return fmt.Errorf("GVR %s is not in allowlist. Allowed GVRs: %v", gvrKey, a.getAllowedGVRs())
	}

	// Check namespace allowlist
	if namespace == "" {
		namespace = a.defaultNamespace
	}
	if namespace != "" && len(a.allowedNamespaces) > 0 {
		if !a.allowedNamespaces[namespace] {
			return fmt.Errorf("namespace %s is not in allowlist. Allowed namespaces: %v", namespace, a.getAllowedNamespaces())
		}
	}

	return nil
}

// GetAllowedGVRs returns the list of allowed GVRs
func (a *GVRAllowlist) GetAllowedGVRs() []string {
	result := make([]string, 0, len(a.allowedGVRs))
	for gvr := range a.allowedGVRs {
		result = append(result, gvr)
	}
	return result
}

// GetAllowedNamespaces returns the list of allowed namespaces
func (a *GVRAllowlist) GetAllowedNamespaces() []string {
	result := make([]string, 0, len(a.allowedNamespaces))
	for ns := range a.allowedNamespaces {
		result = append(result, ns)
	}
	return result
}
