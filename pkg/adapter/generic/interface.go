// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may Obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generic

import (
	"context"
)

// GenericAdapter is the interface for all generic adapters
// All adapters work with ANY tool via YAML configuration
type GenericAdapter interface {
	// Type returns the adapter type (informer, webhook, logs)
	Type() string

	// Start starts the adapter and returns a channel of RawEvents
	// The adapter should run until ctx is cancelled
	Start(ctx context.Context, config *SourceConfig) (<-chan RawEvent, error)

	// Stop gracefully stops the adapter and cleans up resources
	Stop()

	// Validate validates the configuration for this adapter type
	Validate(config *SourceConfig) error
}
