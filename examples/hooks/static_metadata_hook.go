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
	"os"

	"github.com/kube-zen/zen-watcher/pkg/hooks"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// StaticMetadataHook enriches Observations with static metadata from environment
type StaticMetadataHook struct {
	Environment string
	Cluster     string
}

func NewStaticMetadataHook() *StaticMetadataHook {
	return &StaticMetadataHook{
		Environment: getEnvOrDefault("ZEN_ENVIRONMENT", "default"),
		Cluster:     getEnvOrDefault("ZEN_CLUSTER", "unknown"),
	}
}

func (h *StaticMetadataHook) Process(ctx context.Context, obs *unstructured.Unstructured) error {
	annotations := obs.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations["zen.io/environment"] = h.Environment
	annotations["zen.io/cluster"] = h.Cluster

	obs.SetAnnotations(annotations)
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func init() {
	hooks.RegisterHook(NewStaticMetadataHook())
}

