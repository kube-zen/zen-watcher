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

// Processor executes all registered hooks on an Observation.
// Hooks are executed in registration order.
// If any hook returns an error, processing stops and the error is returned.
func Processor(ctx context.Context, obs *unstructured.Unstructured) error {
	hooks := GetHooks()
	for _, hook := range hooks {
		if err := hook.Process(ctx, obs); err != nil {
			return err
		}
	}
	return nil
}
