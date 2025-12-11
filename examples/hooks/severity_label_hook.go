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

package main

import (
	"context"

	"github.com/kube-zen/zen-watcher/pkg/hooks"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SeverityLabelHook adds labels based on severity thresholds
type SeverityLabelHook struct{}

func (h *SeverityLabelHook) Process(ctx context.Context, obs *unstructured.Unstructured) error {
	severity, _, _ := unstructured.NestedString(obs.Object, "spec", "severity")
	labels := obs.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	if severity == "critical" || severity == "high" {
		labels["zen.io/requires-attention"] = "true"
	}

	obs.SetLabels(labels)
	return nil
}

func init() {
	hooks.RegisterHook(&SeverityLabelHook{})
}
