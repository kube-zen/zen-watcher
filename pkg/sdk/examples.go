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

// NewTrivyIngester creates a canonical Trivy Ingester example
func NewTrivyIngester(namespace, name string) *Ingester {
	enabled := true
	return &Ingester{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Ingester",
		Metadata: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: IngesterSpec{
			Source:   "trivy",
			Ingester: "informer",
			Destinations: []Destination{
				{
					Type:  "crd",
					Value: "observations",
					Mapping: &NormalizationMapping{
						Domain: "security",
						Type:   "vulnerability",
						Priority: map[string]interface{}{
							"HIGH":   0.8,
							"MEDIUM": 0.5,
							"LOW":    0.3,
						},
						FieldMapping: []FieldMapping{
							{From: "vulnerabilityID", To: "cve"},
							{From: "package", To: "package"},
							{From: "installedVersion", To: "version"},
						},
					},
				},
			},
			Informer: &InformerConfig{
				GVR: &GVRConfig{
					Group:    "aquasecurity.github.io",
					Version:  "v1alpha1",
					Resource: "vulnerabilityreports",
				},
			},
			Deduplication: &DeduplicationConfig{
				Enabled:  &enabled,
				Strategy: "fingerprint",
				Window:   "24h",
			},
			Filters: &FilterConfig{
				MinPriority: floatPtr(0.3),
			},
		},
	}
}

// NewKyvernoIngester creates a canonical Kyverno Ingester example
func NewKyvernoIngester(namespace, name string) *Ingester {
	enabled := true
	return &Ingester{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Ingester",
		Metadata: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: IngesterSpec{
			Source:   "kyverno",
			Ingester: "informer",
			Destinations: []Destination{
				{
					Type:  "crd",
					Value: "observations",
					Mapping: &NormalizationMapping{
						Domain: "compliance",
						Type:   "policy_violation",
						Priority: map[string]interface{}{
							"fail":  0.9,
							"warn":  0.5,
							"error": 0.7,
						},
						FieldMapping: []FieldMapping{
							{From: "policy", To: "policy"},
							{From: "rule", To: "rule"},
							{From: "message", To: "message"},
						},
					},
				},
			},
			Informer: &InformerConfig{
				GVR: &GVRConfig{
					Group:    "kyverno.io",
					Version:  "v1",
					Resource: "policyviolations",
				},
			},
			Deduplication: &DeduplicationConfig{
				Enabled:  &enabled,
				Strategy: "key",
				Fields:   []string{"policy", "rule", "resource.name"},
				Window:   "1h",
			},
			Filters: &FilterConfig{
				MinSeverity: "MEDIUM",
			},
		},
	}
}

// NewKubeBenchIngester creates a canonical Kube-bench Ingester example
func NewKubeBenchIngester(namespace, name string) *Ingester {
	enabled := true
	return &Ingester{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Ingester",
		Metadata: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: IngesterSpec{
			Source:   "kube-bench",
			Ingester: "informer",
			Destinations: []Destination{
				{
					Type:  "crd",
					Value: "observations",
					Mapping: &NormalizationMapping{
						Domain: "compliance",
						Type:   "cis_benchmark",
						Priority: map[string]interface{}{
							"FAIL": 0.8,
							"WARN": 0.4,
							"PASS": 0.1,
						},
						FieldMapping: []FieldMapping{
							{From: "test_number", To: "test"},
							{From: "test_desc", To: "description"},
							{From: "status", To: "status"},
						},
					},
				},
			},
			Informer: &InformerConfig{
				GVR: &GVRConfig{
					Group:    "aquasecurity.github.io",
					Version:  "v1alpha1",
					Resource: "ciskubernetesbenchmarks",
				},
			},
			Deduplication: &DeduplicationConfig{
				Enabled:  &enabled,
				Strategy: "key",
				Fields:   []string{"test_number", "node"},
				Window:   "12h",
			},
		},
	}
}

// NewK8sEventsIngester creates a canonical Kubernetes Events Ingester example
func NewK8sEventsIngester(namespace, name string) *Ingester {
	enabled := true
	return &Ingester{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       "Ingester",
		Metadata: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: IngesterSpec{
			Source:   "k8s-events",
			Ingester: "k8s-events",
			Destinations: []Destination{
				{
					Type:  "crd",
					Value: "observations",
					Mapping: &NormalizationMapping{
						Domain: "operations",
						Type:   "kubernetes_event",
					},
				},
			},
			K8sEvents: &K8sEventsConfig{
				InvolvedObjectKinds: []string{"Pod", "Deployment", "Service"},
			},
			Deduplication: &DeduplicationConfig{
				Enabled:  &enabled,
				Strategy: "fingerprint",
				Window:   "5m",
			},
			Filters: &FilterConfig{
				MinSeverity: "MEDIUM",
				ExcludeNamespaces: []string{"kube-system", "kube-public"},
			},
		},
	}
}

func floatPtr(f float64) *float64 {
	return &f
}

