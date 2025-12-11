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

package hooks

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ObservationHook is the interface for processing hooks that can modify Observations
// before they are written to CRDs.
//
// Hooks are called after normalization but before CRD write.
// Hooks must be fast, non-blocking, and in-process (no network I/O).
type ObservationHook interface {
	// Process processes an Observation and may modify it.
	// Returns an error if processing should fail (Observation will not be written).
	Process(ctx context.Context, obs *unstructured.Unstructured) error
}
