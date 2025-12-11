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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type IngesterV1Alpha1 struct {
	APIVersion string                 `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                 `yaml:"kind" json:"kind"`
	Metadata   map[string]interface{} `yaml:"metadata" json:"metadata"`
	Spec       IngesterSpecV1Alpha1   `yaml:"spec" json:"spec"`
}

type IngesterSpecV1Alpha1 struct {
	Source         string                    `yaml:"source" json:"source"`
	Ingester       string                    `yaml:"ingester" json:"ingester"`
	Destinations   []DestinationV1Alpha1     `yaml:"destinations" json:"destinations"`
	Normalization  *NormalizationConfig      `yaml:"normalization,omitempty" json:"normalization,omitempty"`
	Deduplication  map[string]interface{}    `yaml:"deduplication,omitempty" json:"deduplication,omitempty"`
	Filters        map[string]interface{}    `yaml:"filters,omitempty" json:"filters,omitempty"`
	Optimization   map[string]interface{}    `yaml:"optimization,omitempty" json:"optimization,omitempty"`
	Processing     map[string]interface{}    `yaml:"processing,omitempty" json:"processing,omitempty"`
	Informer       map[string]interface{}    `yaml:"informer,omitempty" json:"informer,omitempty"`
	Webhook        map[string]interface{}    `yaml:"webhook,omitempty" json:"webhook,omitempty"`
	Logs           map[string]interface{}    `yaml:"logs,omitempty" json:"logs,omitempty"`
	K8sEvents      map[string]interface{}    `yaml:"k8sEvents,omitempty" json:"k8sEvents,omitempty"`
}

type DestinationV1Alpha1 struct {
	Type        string                 `yaml:"type" json:"type"`
	Value       string                 `yaml:"value,omitempty" json:"value,omitempty"`
	URL         string                 `yaml:"url,omitempty" json:"url,omitempty"`
	Name        string                 `yaml:"name,omitempty" json:"name,omitempty"`
	RetryPolicy map[string]interface{} `yaml:"retryPolicy,omitempty" json:"retryPolicy,omitempty"`
}

type NormalizationConfig struct {
	Domain       string                 `yaml:"domain,omitempty" json:"domain,omitempty"`
	Type         string                 `yaml:"type,omitempty" json:"type,omitempty"`
	Priority     map[string]interface{} `yaml:"priority,omitempty" json:"priority,omitempty"`
	SeverityMap  map[string]interface{} `yaml:"severityMap,omitempty" json:"severityMap,omitempty"`
	FieldMapping []interface{}          `yaml:"fieldMapping,omitempty" json:"fieldMapping,omitempty"`
	Resource     map[string]interface{}  `yaml:"resource,omitempty" json:"resource,omitempty"`
	Templates    map[string]interface{} `yaml:"templates,omitempty" json:"templates,omitempty"`
}

type IngesterV1 struct {
	APIVersion string                 `yaml:"apiVersion" json:"apiVersion"`
	Kind       string                 `yaml:"kind" json:"kind"`
	Metadata   map[string]interface{} `yaml:"metadata" json:"metadata"`
	Spec       IngesterSpecV1         `yaml:"spec" json:"spec"`
}

type IngesterSpecV1 struct {
	Source        string                 `yaml:"source" json:"source"`
	Ingester      string                 `yaml:"ingester" json:"ingester"`
	Destinations  []DestinationV1        `yaml:"destinations" json:"destinations"`
	Deduplication map[string]interface{} `yaml:"deduplication,omitempty" json:"deduplication,omitempty"`
	Filters       map[string]interface{} `yaml:"filters,omitempty" json:"filters,omitempty"`
	Optimization  map[string]interface{} `yaml:"optimization,omitempty" json:"optimization,omitempty"`
	Processing    map[string]interface{} `yaml:"processing,omitempty" json:"processing,omitempty"`
	Informer      map[string]interface{} `yaml:"informer,omitempty" json:"informer,omitempty"`
	Webhook       map[string]interface{} `yaml:"webhook,omitempty" json:"webhook,omitempty"`
	Logs          map[string]interface{} `yaml:"logs,omitempty" json:"logs,omitempty"`
	K8sEvents     map[string]interface{} `yaml:"k8sEvents,omitempty" json:"k8sEvents,omitempty"`
}

type DestinationV1 struct {
	Type    string                 `yaml:"type" json:"type"`
	Value   string                 `yaml:"value" json:"value"`
	Mapping *NormalizationConfig   `yaml:"mapping,omitempty" json:"mapping,omitempty"`
}

func main() {
	var inputFile, outputFile string
	flag.StringVar(&inputFile, "f", "", "Input YAML file (v1alpha1 Ingester)")
	flag.StringVar(&outputFile, "o", "", "Output YAML file (v1 Ingester)")
	flag.Parse()

	if inputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -f input file is required\n")
		os.Exit(1)
	}

	input, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// Parse YAML (may contain multiple documents)
	decoder := yaml.NewDecoder(strings.NewReader(string(input)))
	var outputDocs []string
	var warnings []string

	for {
		var doc IngesterV1Alpha1
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing YAML: %v\n", err)
			os.Exit(1)
		}

		// Skip non-Ingester resources
		if doc.Kind != "Ingester" || !strings.Contains(doc.APIVersion, "zen.kube-zen.io") {
			continue
		}

		// Migrate to v1
		v1, docWarnings := migrateToV1(doc)
		warnings = append(warnings, docWarnings...)

		// Convert to YAML
		v1YAML, err := yaml.Marshal(v1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling v1 YAML: %v\n", err)
			os.Exit(1)
		}

		// Add comments for warnings
		if len(docWarnings) > 0 {
			comment := "# WARNING: " + strings.Join(docWarnings, "; ") + "\n"
			v1YAML = append([]byte(comment), v1YAML...)
		}

		outputDocs = append(outputDocs, string(v1YAML))
	}

	// Write output
	output := strings.Join(outputDocs, "---\n")
	if outputFile != "" {
		err := os.WriteFile(outputFile, []byte(output), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Migrated Ingester(s) written to %s\n", outputFile)
	} else {
		fmt.Print(output)
	}

	// Print warnings
	if len(warnings) > 0 {
		fmt.Fprintf(os.Stderr, "\nWarnings:\n")
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "  - %s\n", w)
		}
	}
}

func migrateToV1(v1alpha1 IngesterV1Alpha1) (IngesterV1, []string) {
	var warnings []string

	v1 := IngesterV1{
		APIVersion: "zen.kube-zen.io/v1",
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.Metadata,
		Spec: IngesterSpecV1{
			Source:        v1alpha1.Spec.Source,
			Ingester:      v1alpha1.Spec.Ingester,
			Deduplication: v1alpha1.Spec.Deduplication,
			Filters:       v1alpha1.Spec.Filters,
			Optimization:  v1alpha1.Spec.Optimization,
			Processing:    v1alpha1.Spec.Processing,
			Informer:      v1alpha1.Spec.Informer,
			Webhook:       v1alpha1.Spec.Webhook,
			Logs:          v1alpha1.Spec.Logs,
			K8sEvents:     v1alpha1.Spec.K8sEvents,
		},
	}

	// Migrate destinations
	var v1Destinations []DestinationV1
	hasCRDDestination := false

	for _, dest := range v1alpha1.Spec.Destinations {
		if dest.Type == "crd" {
			hasCRDDestination = true
			v1Dest := DestinationV1{
				Type:  "crd",
				Value: dest.Value,
			}
			// Move normalization to mapping if present
			if v1alpha1.Spec.Normalization != nil {
				v1Dest.Mapping = v1alpha1.Spec.Normalization
			}
			v1Destinations = append(v1Destinations, v1Dest)
		} else {
			// Non-CRD destinations are not supported in v1
			warnings = append(warnings, fmt.Sprintf(
				"Destination type '%s' is not supported in v1 (only 'crd' is supported). This destination was removed.",
				dest.Type,
			))
		}
	}

	if len(v1Destinations) == 0 {
		warnings = append(warnings, "No CRD destinations found. Added default 'crd' destination with value 'observations'.")
		v1Destinations = []DestinationV1{
			{
				Type:  "crd",
				Value: "observations",
			}
		}
		// Still add normalization as mapping if present
		if v1alpha1.Spec.Normalization != nil {
			v1Destinations[0].Mapping = v1alpha1.Spec.Normalization
		}
	}

	v1.Spec.Destinations = v1Destinations

	// Add comment about normalization migration if it was moved
	if v1alpha1.Spec.Normalization != nil && hasCRDDestination {
		// Normalization was moved to destinations[].mapping, which is handled above
	}

	return v1, warnings
}

