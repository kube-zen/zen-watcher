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
	"sync"
)

var (
	registeredHooks []ObservationHook
	mu              sync.RWMutex
)

// RegisterHook registers a hook to be executed for all Observations.
// This should be called during initialization (e.g., in init() functions).
func RegisterHook(hook ObservationHook) {
	if hook == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	registeredHooks = append(registeredHooks, hook)
}

// GetHooks returns all registered hooks.
func GetHooks() []ObservationHook {
	mu.RLock()
	defer mu.RUnlock()
	// Return a copy to prevent external modification
	result := make([]ObservationHook, len(registeredHooks))
	copy(result, registeredHooks)
	return result
}

// ClearHooks clears all registered hooks (mainly for testing).
func ClearHooks() {
	mu.Lock()
	defer mu.Unlock()
	registeredHooks = nil
}

