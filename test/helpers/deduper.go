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

package helpers

import (
	"testing"

	sdkdedup "github.com/kube-zen/zen-sdk/pkg/dedup"
)

// NewTestDeduper creates a deduper for testing and ensures it's cleaned up
// This prevents goroutine leaks and test timeouts
func NewTestDeduper(t *testing.T, windowSeconds, maxSize int) *sdkdedup.Deduper {
	t.Helper()
	deduper := sdkdedup.NewDeduper(windowSeconds, maxSize)

	// Ensure deduper is stopped when test completes
	t.Cleanup(func() {
		if deduper != nil {
			deduper.Stop()
		}
	})

	return deduper
}

// NewTestDeduperWithDefaults creates a deduper with default test values
func NewTestDeduperWithDefaults(t *testing.T) *sdkdedup.Deduper {
	t.Helper()
	return NewTestDeduper(t, 60, 10000)
}
