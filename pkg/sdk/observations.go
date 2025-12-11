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

package sdk

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Observation represents an Observation CRD (v1)
type Observation struct {
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Kind       string            `json:"kind" yaml:"kind"`
	Metadata   metav1.ObjectMeta `json:"metadata" yaml:"metadata"`
	Spec       ObservationSpec   `json:"spec" yaml:"spec"`
	Status     *ObservationStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// ObservationSpec represents the spec section of an Observation CRD
type ObservationSpec struct {
	Source                 string                 `json:"source" yaml:"source"`
	Category               string                 `json:"category" yaml:"category"`
	Severity               string                 `json:"severity" yaml:"severity"`
	EventType              string                 `json:"eventType" yaml:"eventType"`
	Resource               *ResourceRef           `json:"resource,omitempty" yaml:"resource,omitempty"`
	Details                map[string]interface{} `json:"details,omitempty" yaml:"details,omitempty"`
	DetectedAt             string                 `json:"detectedAt,omitempty" yaml:"detectedAt,omitempty"`
	TTLSecondsAfterCreation *int64                `json:"ttlSecondsAfterCreation,omitempty" yaml:"ttlSecondsAfterCreation,omitempty"`
}

// ResourceRef represents a Kubernetes resource reference
type ResourceRef struct {
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string `json:"kind" yaml:"kind"`
	Name       string `json:"name" yaml:"name"`
	Namespace  string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// ObservationStatus represents the status section of an Observation CRD
type ObservationStatus struct {
	Processed        bool   `json:"processed,omitempty" yaml:"processed,omitempty"`
	LastProcessedAt string `json:"lastProcessedAt,omitempty" yaml:"lastProcessedAt,omitempty"`
}

