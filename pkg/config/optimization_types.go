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

// DynamicFilterRule represents a dynamically adjustable filter rule
type DynamicFilterRule struct {
	Expression string
	Enabled    bool
	Priority   float64
	Source     string
}

// FilterConfigAdvanced represents advanced filter configuration
// Used by the optimization system
type FilterConfigAdvanced struct {
	DynamicRules []DynamicFilterRule
}
