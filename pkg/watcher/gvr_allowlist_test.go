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
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGVRAllowlist_HardDenyList(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	defer os.Unsetenv("WATCH_NAMESPACE")

	allowlist := NewGVRAllowlist()

	// Test denied GVRs - should always be rejected
	deniedGVRs := []struct {
		group    string
		version  string
		resource string
		name     string
	}{
		{"", "v1", "secrets", "secrets"},
		{"rbac.authorization.k8s.io", "v1", "roles", "roles"},
		{"rbac.authorization.k8s.io", "v1", "rolebindings", "rolebindings"},
		{"rbac.authorization.k8s.io", "v1", "clusterroles", "clusterroles"},
		{"rbac.authorization.k8s.io", "v1", "clusterrolebindings", "clusterrolebindings"},
		{"", "v1", "serviceaccounts", "serviceaccounts"},
		{"admissionregistration.k8s.io", "v1", "validatingwebhookconfigurations", "validatingwebhookconfigurations"},
		{"admissionregistration.k8s.io", "v1", "mutatingwebhookconfigurations", "mutatingwebhookconfigurations"},
		{"apiextensions.k8s.io", "v1", "customresourcedefinitions", "customresourcedefinitions"},
		{"apiextensions.k8s.io", "v1beta1", "customresourcedefinitions", "customresourcedefinitions_v1beta1"},
	}

	for _, tc := range deniedGVRs {
		t.Run(tc.name, func(t *testing.T) {
			gvr := schema.GroupVersionResource{
				Group:    tc.group,
				Version:  tc.version,
				Resource: tc.resource,
			}
			err := allowlist.IsAllowed(gvr, "test-ns")
			if err == nil {
				t.Errorf("Expected denial for %s/%s/%s, but got nil error", tc.group, tc.version, tc.resource)
			}
			if !errors.Is(err, ErrGVRDenied) {
				t.Errorf("Expected ErrGVRDenied, got: %v", err)
			}
		})
	}
}

func TestGVRAllowlist_AllowedGVR(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	defer os.Unsetenv("WATCH_NAMESPACE")

	allowlist := NewGVRAllowlist()

	// Test allowed GVR (default: observations)
	gvr := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	err := allowlist.IsAllowed(gvr, "test-ns")
	if err != nil {
		t.Errorf("Expected allowed for observations, got error: %v", err)
	}
}

func TestGVRAllowlist_NamespaceRestriction(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	defer os.Unsetenv("WATCH_NAMESPACE")

	allowlist := NewGVRAllowlist()

	// Test allowed namespace
	gvr := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	err := allowlist.IsAllowed(gvr, "test-ns")
	if err != nil {
		t.Errorf("Expected allowed for test-ns, got error: %v", err)
	}

	// Test disallowed namespace
	err = allowlist.IsAllowed(gvr, "other-ns")
	if err == nil {
		t.Errorf("Expected denial for other-ns, but got nil error")
	}
	if !errors.Is(err, ErrNamespaceNotAllowed) {
		t.Errorf("Expected ErrNamespaceNotAllowed, got: %v", err)
	}
}

func TestGVRAllowlist_ClusterScopedRejection(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	defer os.Unsetenv("WATCH_NAMESPACE")

	allowlist := NewGVRAllowlist()

	// Test cluster-scoped resource (no namespace) - should be rejected
	gvr := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	err := allowlist.IsAllowed(gvr, "")
	if err == nil {
		t.Errorf("Expected denial for cluster-scoped resource, but got nil error")
	}
	if !errors.Is(err, ErrClusterScopedNotAllowed) {
		t.Errorf("Expected ErrClusterScopedNotAllowed, got: %v", err)
	}
}

func TestGVRAllowlist_ClusterScopedExplicitAllow(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	os.Setenv("ALLOWED_CLUSTER_SCOPED_GVRS", "zen.kube-zen.io/v1/clusterobservations")
	defer func() {
		os.Unsetenv("WATCH_NAMESPACE")
		os.Unsetenv("ALLOWED_CLUSTER_SCOPED_GVRS")
	}()

	allowlist := NewGVRAllowlist()

	// Test explicitly allowed cluster-scoped resource
	gvr := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "clusterobservations",
	}

	err := allowlist.IsAllowed(gvr, "")
	if err != nil {
		t.Errorf("Expected allowed for explicitly approved cluster-scoped resource, got error: %v", err)
	}
}

func TestGVRAllowlist_NotInAllowlist(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	defer os.Unsetenv("WATCH_NAMESPACE")

	allowlist := NewGVRAllowlist()

	// Test GVR not in allowlist
	gvr := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "customresources",
	}

	err := allowlist.IsAllowed(gvr, "test-ns")
	if err == nil {
		t.Errorf("Expected denial for non-allowlisted GVR, but got nil error")
	}
	if !errors.Is(err, ErrGVRNotAllowed) {
		t.Errorf("Expected ErrGVRNotAllowed, got: %v", err)
	}
}

func TestGVRAllowlist_CustomAllowedGVRs(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	os.Setenv("ALLOWED_GVRS", "example.com/v1/customresources,other.com/v1beta1/things")
	defer func() {
		os.Unsetenv("WATCH_NAMESPACE")
		os.Unsetenv("ALLOWED_GVRS")
	}()

	allowlist := NewGVRAllowlist()

	// Test custom allowed GVR
	gvr := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "customresources",
	}

	err := allowlist.IsAllowed(gvr, "test-ns")
	if err != nil {
		t.Errorf("Expected allowed for custom GVR, got error: %v", err)
	}
}

func TestGVRAllowlist_CustomAllowedNamespaces(t *testing.T) {
	// Set up test environment
	os.Setenv("WATCH_NAMESPACE", "test-ns")
	os.Setenv("ALLOWED_NAMESPACES", "ns1,ns2,ns3")
	defer func() {
		os.Unsetenv("WATCH_NAMESPACE")
		os.Unsetenv("ALLOWED_NAMESPACES")
	}()

	allowlist := NewGVRAllowlist()

	gvr := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "observations",
	}

	// Test allowed namespaces
	for _, ns := range []string{"ns1", "ns2", "ns3"} {
		err := allowlist.IsAllowed(gvr, ns)
		if err != nil {
			t.Errorf("Expected allowed for namespace %s, got error: %v", ns, err)
		}
	}

	// Test disallowed namespace
	err := allowlist.IsAllowed(gvr, "other-ns")
	if err == nil {
		t.Errorf("Expected denial for other-ns, but got nil error")
	}
	if !errors.Is(err, ErrNamespaceNotAllowed) {
		t.Errorf("Expected ErrNamespaceNotAllowed, got: %v", err)
	}
}
