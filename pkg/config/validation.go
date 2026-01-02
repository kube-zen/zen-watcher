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
	sdkvalidation "github.com/kube-zen/zen-sdk/pkg/k8s/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ValidateGVR validates a GroupVersionResource
// Returns an error if any component is invalid
func ValidateGVR(gvr schema.GroupVersionResource) error {
	return sdkvalidation.ValidateGVR(gvr)
}

// ValidateGVRConfig validates a GVRConfig (from destination configuration)
func ValidateGVRConfig(group, version, resource string) error {
	return sdkvalidation.ValidateGVRConfig(group, version, resource)
}
