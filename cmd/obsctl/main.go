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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig  = flag.String("kubeconfig", "", "Path to kubeconfig file")
	contextName = flag.String("context", "", "Kubernetes context to use")
	namespace   = flag.String("namespace", "", "Namespace to query (empty = all namespaces)")
	output      = flag.String("output", "table", "Output format: table, json, yaml")
)

var observationGVR = schema.GroupVersionResource{
	Group:    "zen.kube-zen.io",
	Version:  "v1",
	Resource: "observations",
}

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: obsctl <command> [flags]\n")
		fmt.Fprintf(os.Stderr, "Commands: list, stats, get\n")
		os.Exit(1)
	}

	command := flag.Arg(0)

	// Build kube client
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building kubeconfig: %v\n", err)
		os.Exit(1)
	}

	if *contextName != "" {
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeconfig},
			&clientcmd.ConfigOverrides{CurrentContext: *contextName},
		).ClientConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading context: %v\n", err)
			os.Exit(1)
		}
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dynamic client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	switch command {
	case "list":
		selector := flag.Arg(1)
		if err := listObservations(ctx, dynamicClient, selector); err != nil {
			fmt.Fprintf(os.Stderr, "Error listing observations: %v\n", err)
			os.Exit(1)
		}
	case "stats":
		groupBy := flag.Arg(1)
		if err := showStats(ctx, dynamicClient, groupBy); err != nil {
			fmt.Fprintf(os.Stderr, "Error showing stats: %v\n", err)
			os.Exit(1)
		}
	case "get":
		name := flag.Arg(1)
		if name == "" {
			fmt.Fprintf(os.Stderr, "Error: name required for 'get' command\n")
			os.Exit(1)
		}
		if err := getObservation(ctx, dynamicClient, name); err != nil {
			fmt.Fprintf(os.Stderr, "Error getting observation: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func listObservations(ctx context.Context, client dynamic.Interface, selectorStr string) error {
	var labelSelector labels.Selector
	var err error

	if selectorStr != "" {
		labelSelector, err = labels.Parse(selectorStr)
		if err != nil {
			return fmt.Errorf("invalid selector: %w", err)
		}
	} else {
		labelSelector = labels.Everything()
	}

	var listOptions metav1.ListOptions
	if labelSelector != nil {
		listOptions.LabelSelector = labelSelector.String()
	}

	var observations *unstructured.UnstructuredList
	if *namespace != "" {
		observations, err = client.Resource(observationGVR).Namespace(*namespace).List(ctx, listOptions)
	} else {
		observations, err = client.Resource(observationGVR).List(ctx, listOptions)
	}

	if err != nil {
		return err
	}

	if *output == "json" {
		jsonData, _ := json.MarshalIndent(observations.Items, "", "  ")
		fmt.Println(string(jsonData))
		return nil
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tSOURCE\tCATEGORY\tSEVERITY\tEVENT TYPE\tAGE")
	_, _ = fmt.Fprintln(w, "----\t------\t--------\t--------\t----------\t---")

	for _, obs := range observations.Items {
		name := obs.GetName()
		source, _, _ := unstructured.NestedString(obs.Object, "spec", "source")
		category, _, _ := unstructured.NestedString(obs.Object, "spec", "category")
		severity, _, _ := unstructured.NestedString(obs.Object, "spec", "severity")
		eventType, _, _ := unstructured.NestedString(obs.Object, "spec", "eventType")
		age := getAge(obs.GetCreationTimestamp().Time)

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", name, source, category, severity, eventType, age)
	}

	_ = w.Flush()
	return nil
}

func showStats(ctx context.Context, client dynamic.Interface, groupBy string) error {
	var observations *unstructured.UnstructuredList
	var err error

	if *namespace != "" {
		observations, err = client.Resource(observationGVR).Namespace(*namespace).List(ctx, metav1.ListOptions{})
	} else {
		observations, err = client.Resource(observationGVR).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return err
	}

	// Simple stats by source and severity
	stats := make(map[string]int)

	for _, obs := range observations.Items {
		source, _, _ := unstructured.NestedString(obs.Object, "spec", "source")
		severity, _, _ := unstructured.NestedString(obs.Object, "spec", "severity")
		key := fmt.Sprintf("%s/%s", source, severity)
		stats[key]++
	}

	if *output == "json" {
		jsonData, _ := json.MarshalIndent(stats, "", "  ")
		fmt.Println(string(jsonData))
		return nil
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SOURCE\tSEVERITY\tCOUNT")
	_, _ = fmt.Fprintln(w, "------\t--------\t-----")

	for key, count := range stats {
		parts := strings.Split(key, "/")
		if len(parts) == 2 {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%d\n", parts[0], parts[1], count)
		}
	}

	_ = w.Flush()
	return nil
}

func getObservation(ctx context.Context, client dynamic.Interface, name string) error {
	var obs *unstructured.Unstructured
	var err error

	if *namespace != "" {
		obs, err = client.Resource(observationGVR).Namespace(*namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		// Try to find in all namespaces
		obs, err = findObservationInAllNamespaces(ctx, client, name)
	}

	if err != nil {
		return err
	}

	if *output == "json" {
		jsonData, _ := json.MarshalIndent(obs.Object, "", "  ")
		fmt.Println(string(jsonData))
		return nil
	}

	// YAML output (simplified - would use proper YAML marshaler)
	fmt.Printf("Name: %s\n", obs.GetName())
	fmt.Printf("Namespace: %s\n", obs.GetNamespace())
	source, _, _ := unstructured.NestedString(obs.Object, "spec", "source")
	category, _, _ := unstructured.NestedString(obs.Object, "spec", "category")
	severity, _, _ := unstructured.NestedString(obs.Object, "spec", "severity")
	eventType, _, _ := unstructured.NestedString(obs.Object, "spec", "eventType")
	fmt.Printf("Source: %s\n", source)
	fmt.Printf("Category: %s\n", category)
	fmt.Printf("Severity: %s\n", severity)
	fmt.Printf("Event Type: %s\n", eventType)

	return nil
}

func findObservationInAllNamespaces(ctx context.Context, client dynamic.Interface, name string) (*unstructured.Unstructured, error) {
	observations, err := client.Resource(observationGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, obs := range observations.Items {
		if obs.GetName() == name {
			return &obs, nil
		}
	}

	return nil, fmt.Errorf("observation %s not found", name)
}

func getAge(creationTime interface{}) string {
	// Simplified - would use proper time calculation
	return "<unknown>"
}
