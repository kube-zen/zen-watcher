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
	"flag"
	"fmt"
	"os"
	"time"

	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfigs = flag.String("kubeconfigs", "", "Comma-separated list of kubeconfig files (one per cluster)")
	namespace   = flag.String("namespace", "", "Namespace to read from (empty = all namespaces)")
	interval    = flag.Duration("interval", 1*time.Minute, "Polling interval")
)

var observationGVR = schema.GroupVersionResource{
	Group:    "zen.kube-zen.io",
	Version:  "v1",
	Resource: "observations",
}

func main() {
	flag.Parse()

	if *kubeconfigs == "" {
		fmt.Fprintf(os.Stderr, "Error: --kubeconfigs is required\n")
		os.Exit(1)
	}

	configFiles := parseKubeconfigs(*kubeconfigs)
	clients := make([]dynamic.Interface, 0, len(configFiles))

	// Build clients for each cluster
	for _, configFile := range configFiles {
		config, err := clientcmd.BuildConfigFromFlags("", configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building kubeconfig for %s: %v\n", configFile, err)
			continue
		}

		client, err := dynamic.NewForConfig(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating client for %s: %v\n", configFile, err)
			continue
		}

		clients = append(clients, client)
	}

	if len(clients) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no valid clients created\n")
		os.Exit(1)
	}

	ctx := context.Background()

	// Aggregate Observations from all clusters
	for {
		aggregate := make(map[string]int)

		for i, client := range clients {
			clusterName := fmt.Sprintf("cluster-%d", i)
			obs, err := readObservations(ctx, client)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading from %s: %v\n", clusterName, err)
				continue
			}

			for _, o := range obs {
				source, _, _ := unstructured.NestedString(o.Object, "spec", "source")
				severity, _, _ := unstructured.NestedString(o.Object, "spec", "severity")
				key := fmt.Sprintf("%s/%s", source, severity)
				aggregate[key]++
			}
		}

		// Print aggregated stats
		fmt.Println("Aggregated Observations:")
		for key, count := range aggregate {
			fmt.Printf("  %s: %d\n", key, count)
		}

		time.Sleep(*interval)
	}
}

func parseKubeconfigs(s string) []string {
	parts := make([]string, 0)
	for _, part := range splitAndTrim(s, ",") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, part := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func readObservations(ctx context.Context, client dynamic.Interface) ([]unstructured.Unstructured, error) {
	var listOptions metav1.ListOptions

	var observations *unstructured.UnstructuredList
	var err error

	if *namespace != "" {
		observations, err = client.Resource(observationGVR).Namespace(*namespace).List(ctx, listOptions)
	} else {
		observations, err = client.Resource(observationGVR).List(ctx, listOptions)
	}

	if err != nil {
		return nil, err
	}

	return observations.Items, nil
}
