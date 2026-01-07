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
	"os"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DefaultAPIGroup is the default API group used for zen-watcher resources.
// This can be overridden via the ZEN_API_GROUP environment variable.
// Default: "zen.kube-zen.io" (for backward compatibility)
var DefaultAPIGroup = getDefaultAPIGroup()

func getDefaultAPIGroup() string {
	if group := os.Getenv("ZEN_API_GROUP"); group != "" {
		return group
	}
	return "zen.kube-zen.io"
}

// DefaultAPIVersion is the default API version for observations and other resources.
// Note: Currently using v1alpha1 as that's what's installed in the cluster
const DefaultAPIVersion = "v1alpha1"

// IngesterGVR is the GroupVersionResource for Ingester CRDs
var IngesterGVR = schema.GroupVersionResource{
	Group:    DefaultAPIGroup,
	Version:  "v1alpha1",
	Resource: "ingesters",
}

// ObservationsGVR returns the GVR for Observation CRDs
func ObservationsGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    DefaultAPIGroup,
		Version:  DefaultAPIVersion,
		Resource: "observations",
	}
}

// ResolveDestinationGVR resolves a GVR from a destination value.
// If value is "observations", returns the Observations GVR.
// Otherwise, defaults to DefaultAPIGroup/v1/{value}
func ResolveDestinationGVR(value string) schema.GroupVersionResource {
	if value == "observations" {
		return ObservationsGVR()
	}
	return schema.GroupVersionResource{
		Group:    DefaultAPIGroup,
		Version:  DefaultAPIVersion,
		Resource: value,
	}
}
