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
	"errors"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Security policy errors
var (
	ErrGVRNotAllowed           = errors.New("GVR not in allowlist")
	ErrNamespaceNotAllowed     = errors.New("namespace not in allowlist")
	ErrGVRDenied               = errors.New("GVR categorically denied by security policy")
	ErrClusterScopedNotAllowed = errors.New("cluster-scoped resource not allowed")
)

// GVRAllowlist enforces namespace + allowlist restrictions for zen-watcher GVR writes
// H037: Prevents zen-watcher from writing to arbitrary GVRs
// Security: Includes hard deny list for dangerous resources
type GVRAllowlist struct {
	allowedGVRs          map[string]bool // key: "group/version/resource"
	allowedNamespaces    map[string]bool // Set of allowed namespaces
	defaultNamespace     string          // Default namespace if not specified
	clusterScopedAllowed map[string]bool // Explicitly allowed cluster-scoped resources
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
// Security: Includes hard deny list for dangerous resources
func NewGVRAllowlist() *GVRAllowlist {
	allowlist := &GVRAllowlist{
		allowedGVRs:          make(map[string]bool),
		allowedNamespaces:    make(map[string]bool),
		defaultNamespace:     os.Getenv("WATCH_NAMESPACE"),
		clusterScopedAllowed: make(map[string]bool),
	}

	// Default allowed GVR: observations.zen.kube-zen.io (zen-watcher's own resource)
	// Format: "group/version/resource" (or "version/resource" for core resources)
	allowlist.allowedGVRs["zen.kube-zen.io/v1/observations"] = true

	// Read allowed GVRs from environment variable (comma-separated)
	// Format: "group/version/resource,group2/version2/resource2" or "version/resource" for core resources
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

	// Read explicitly allowed cluster-scoped resources (comma-separated)
	// Format: "group/version/resource,group2/version2/resource2"
	clusterScopedEnv := os.Getenv("ALLOWED_CLUSTER_SCOPED_GVRS")
	if clusterScopedEnv != "" {
		for _, gvrStr := range strings.Split(clusterScopedEnv, ",") {
			gvrStr = strings.TrimSpace(gvrStr)
			if gvrStr != "" {
				allowlist.clusterScopedAllowed[gvrStr] = true
			}
		}
	}

	return allowlist
}

// NewGVRAllowlistFromConfig creates a GVR allowlist from explicit configuration
// H041: Allows tests to configure allowlist deterministically without relying on environment variables
func NewGVRAllowlistFromConfig(config GVRAllowlistConfig) *GVRAllowlist {
	allowlist := &GVRAllowlist{
		allowedGVRs:          make(map[string]bool),
		allowedNamespaces:    make(map[string]bool),
		defaultNamespace:     config.DefaultNamespace,
		clusterScopedAllowed: make(map[string]bool),
	}

	// Set default allowed GVR if not provided
	if len(config.AllowedGVRs) == 0 {
		allowlist.allowedGVRs["zen.kube-zen.io/v1/observations"] = true
	} else {
		for _, gvrStr := range config.AllowedGVRs {
			gvrStr = strings.TrimSpace(gvrStr)
			if gvrStr != "" {
				allowlist.allowedGVRs[gvrStr] = true
			}
		}
	}

	// Set allowed namespaces
	for _, ns := range config.AllowedNamespaces {
		ns = strings.TrimSpace(ns)
		if ns != "" {
			allowlist.allowedNamespaces[ns] = true
		}
	}

	// Set allowed cluster-scoped resources
	for _, gvrStr := range config.ClusterScopedAllowed {
		gvrStr = strings.TrimSpace(gvrStr)
		if gvrStr != "" {
			allowlist.clusterScopedAllowed[gvrStr] = true
		}
	}

	return allowlist
}

// GVRAllowlistConfig holds explicit configuration for GVR allowlist
// H041: Used for deterministic test configuration
type GVRAllowlistConfig struct {
	AllowedGVRs          []string // List of allowed GVRs (format: "group/version/resource" or "version/resource" for core)
	AllowedNamespaces    []string // List of allowed namespaces
	DefaultNamespace     string   // Default namespace if not specified
	ClusterScopedAllowed []string // List of explicitly allowed cluster-scoped GVRs
}

// buildGVRKey builds a normalized GVR key string
// Handles empty groups (core resources) correctly
func buildGVRKey(gvr schema.GroupVersionResource) string {
	if gvr.Group == "" {
		return fmt.Sprintf("%s/%s", gvr.Version, gvr.Resource)
	}
	return fmt.Sprintf("%s/%s/%s", gvr.Group, gvr.Version, gvr.Resource)
}

// IsAllowed checks if a GVR write is allowed
// H037: Validates namespace + GVR allowlist
// Security: Includes hard deny list for dangerous resources
func (a *GVRAllowlist) IsAllowed(gvr schema.GroupVersionResource, namespace string) error {
	gvrKey := buildGVRKey(gvr)

	// STEP 1: Hard deny list - always reject these, even if in allowlist
	deniedGVRs := []string{
		"v1/secrets",
		"rbac.authorization.k8s.io/v1/roles",
		"rbac.authorization.k8s.io/v1/rolebindings",
		"rbac.authorization.k8s.io/v1/clusterroles",
		"rbac.authorization.k8s.io/v1/clusterrolebindings",
		"v1/serviceaccounts",
		"admissionregistration.k8s.io/v1/validatingwebhookconfigurations",
		"admissionregistration.k8s.io/v1/mutatingwebhookconfigurations",
		"apiextensions.k8s.io/v1/customresourcedefinitions",
		"apiextensions.k8s.io/v1beta1/customresourcedefinitions",
	}

	for _, denied := range deniedGVRs {
		if gvrKey == denied {
			return fmt.Errorf("%w: GVR %s is categorically denied (security policy)", ErrGVRDenied, gvrKey)
		}
	}

	// STEP 2: Check for cluster-scoped resources (namespace == "")
	if namespace == "" {
		// Check if this cluster-scoped resource is explicitly allowed
		if !a.isClusterScopedAllowed(gvrKey) {
			return fmt.Errorf("%w: cluster-scoped resource %s requires explicit approval (set ALLOWED_CLUSTER_SCOPED_GVRS)", ErrClusterScopedNotAllowed, gvrKey)
		}
		// Cluster-scoped resource is explicitly allowed, skip namespace check
		return nil
	}

	// STEP 3: Check GVR allowlist (for namespaced resources)
	if !a.allowedGVRs[gvrKey] {
		return fmt.Errorf("%w: GVR %s is not in allowlist. Allowed GVRs: %v", ErrGVRNotAllowed, gvrKey, a.getAllowedGVRs())
	}

	// STEP 4: Check namespace allowlist
	if namespace == "" {
		namespace = a.defaultNamespace
	}
	if namespace != "" && len(a.allowedNamespaces) > 0 {
		if !a.allowedNamespaces[namespace] {
			return fmt.Errorf("%w: namespace %s is not in allowlist. Allowed namespaces: %v", ErrNamespaceNotAllowed, namespace, a.getAllowedNamespaces())
		}
	}

	return nil
}

// isClusterScopedAllowed checks if a cluster-scoped GVR is explicitly allowed
func (a *GVRAllowlist) isClusterScopedAllowed(gvrKey string) bool {
	return a.clusterScopedAllowed[gvrKey]
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
