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

// Default configuration constants
const (
	// DefaultDedupMaxSize is the default maximum size for deduplication cache
	DefaultDedupMaxSize = 10000

	// DefaultLogsSinceSeconds is the default time window for log ingestion (5 minutes)
	DefaultLogsSinceSeconds = 300

	// DefaultTTLMinSeconds is the minimum TTL in seconds (1 minute)
	// Prevents immediate deletion due to misconfiguration
	DefaultTTLMinSeconds int64 = 60

	// DefaultTTLMaxSeconds is the maximum TTL in seconds (1 year)
	// Prevents indefinite retention
	DefaultTTLMaxSeconds int64 = 365 * 24 * 60 * 60
)
