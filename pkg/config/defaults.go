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
	"time"
)

// DefaultSourceConfig represents default configuration for a source when no CRD exists
type DefaultSourceConfig struct {
	DedupWindow       time.Duration
	FilterMinPriority float64
	TTLDefault        time.Duration
	RateLimitMax      int
}

// DefaultTypeConfig represents default configuration for an observation type when no CRD exists
type DefaultTypeConfig struct {
	Domain   string
	Priority float64
}

// GetDefaultSourceConfig returns default configuration for a source
func GetDefaultSourceConfig(source string) DefaultSourceConfig {
	// Default deduplication window: 60 seconds
	// Default filter: no minimum priority (allow all)
	// Default TTL: 7 days
	// Default rate limit: 100 per minute
	defaults := DefaultSourceConfig{
		DedupWindow:       60 * time.Second,
		FilterMinPriority: 0.0,
		TTLDefault:        7 * 24 * time.Hour,
		RateLimitMax:      100,
	}

	// Source-specific overrides
	switch source {
	case "cert-manager":
		// Certificate expiration events: longer dedup window to avoid flooding
		defaults.DedupWindow = 24 * time.Hour
		defaults.TTLDefault = 30 * 24 * time.Hour // 30 days
		defaults.FilterMinPriority = 0.5
	case "falco":
		// Runtime security: shorter window for faster detection
		defaults.DedupWindow = 60 * time.Second
		defaults.TTLDefault = 7 * 24 * time.Hour
	case "trivy":
		// Vulnerability scans: medium window
		defaults.DedupWindow = 1 * time.Hour
		defaults.TTLDefault = 14 * 24 * time.Hour // 14 days
	case "kyverno":
		// Policy violations: short window
		defaults.DedupWindow = 5 * time.Minute
		defaults.TTLDefault = 7 * 24 * time.Hour
	}

	return defaults
}

// GetDefaultTypeConfig returns default configuration for an observation type
func GetDefaultTypeConfig(obsType string) DefaultTypeConfig {
	// Default domain: security (for backward compatibility)
	// Default priority: 0.5 (medium)
	defaults := DefaultTypeConfig{
		Domain:   "security",
		Priority: 0.5,
	}

	// Type-specific overrides
	switch obsType {
	case "certificate_expiring":
		defaults.Domain = "operations"
		defaults.Priority = 0.7
	case "certificate_failed":
		defaults.Domain = "operations"
		defaults.Priority = 0.9
	case "vulnerability":
		defaults.Domain = "security"
		defaults.Priority = 0.8
	case "policy_violation":
		defaults.Domain = "compliance"
		defaults.Priority = 0.6
	}

	return defaults
}

