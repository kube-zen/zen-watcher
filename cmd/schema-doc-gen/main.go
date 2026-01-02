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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	var outputDir string
	flag.StringVar(&outputDir, "output", "docs/generated", "Output directory for generated docs")
	flag.Parse()

	// Generate Ingester schema doc
	if err := generateIngesterDoc(outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating Ingester doc: %v\n", err)
		os.Exit(1)
	}

	// Generate Observations schema doc
	if err := generateObservationDoc(outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating Observation doc: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Schema documentation generated successfully")
}

func generateIngesterDoc(outputDir string) error {
	crdPath := "deployments/crds/ingester_crd.yaml"
	data, err := os.ReadFile(crdPath)
	if err != nil {
		return fmt.Errorf("failed to read CRD: %w", err)
	}

	var crd map[string]interface{}
	if err := yaml.Unmarshal(data, &crd); err != nil {
		return fmt.Errorf("failed to parse CRD: %w", err)
	}

	// Extract v1 schema
	spec, _ := crd["spec"].(map[string]interface{})
	versions, _ := spec["versions"].([]interface{})
	var v1Schema map[string]interface{}
	for _, v := range versions {
		version, _ := v.(map[string]interface{})
		if name, _ := version["name"].(string); name == "v1" {
			schema, _ := version["schema"].(map[string]interface{})
			openAPI, _ := schema["openAPIV3Schema"].(map[string]interface{})
			v1Schema = openAPI
			break
		}
	}

	if v1Schema == nil {
		return fmt.Errorf("v1 schema not found")
	}

	// Generate markdown
	md := generateMarkdown("Ingester", v1Schema)

	// Write to file
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, "INGESTER_SCHEMA_REFERENCE.md")
	return os.WriteFile(outputPath, []byte(md), 0600)
}

func generateObservationDoc(outputDir string) error {
	crdPath := "deployments/crds/observation_crd.yaml"
	data, err := os.ReadFile(crdPath)
	if err != nil {
		return fmt.Errorf("failed to read CRD: %w", err)
	}

	var crd map[string]interface{}
	if err := yaml.Unmarshal(data, &crd); err != nil {
		return fmt.Errorf("failed to parse CRD: %w", err)
	}

	// Extract v1 schema
	spec, _ := crd["spec"].(map[string]interface{})
	versions, _ := spec["versions"].([]interface{})
	var v1Schema map[string]interface{}
	for _, v := range versions {
		version, _ := v.(map[string]interface{})
		if name, _ := version["name"].(string); name == "v1" {
			schema, _ := version["schema"].(map[string]interface{})
			openAPI, _ := schema["openAPIV3Schema"].(map[string]interface{})
			v1Schema = openAPI
			break
		}
	}

	if v1Schema == nil {
		return fmt.Errorf("v1 schema not found")
	}

	// Generate markdown
	md := generateMarkdown("Observation", v1Schema)

	// Write to file
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, "OBSERVATIONS_SCHEMA_REFERENCE.md")
	return os.WriteFile(outputPath, []byte(md), 0600)
}

func generateMarkdown(kind string, schema map[string]interface{}) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# %s Schema Reference\n\n", kind))
	b.WriteString("**⚠️ This file is auto-generated. Do not edit manually.**\n\n")
	b.WriteString("This document is generated from the CRD schema. To update, modify the CRD and run:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("go run ./cmd/schema-doc-gen\n")
	b.WriteString("```\n\n")

	// Extract spec properties
	properties, _ := schema["properties"].(map[string]interface{})
	spec, _ := properties["spec"].(map[string]interface{})
	specProps, _ := spec["properties"].(map[string]interface{})
	required, _ := spec["required"].([]interface{})

	requiredSet := make(map[string]bool)
	for _, r := range required {
		if s, ok := r.(string); ok {
			requiredSet[s] = true
		}
	}

	// Generate spec table
	b.WriteString("## Spec Fields\n\n")
	b.WriteString("| Field | Type | Required | Description | Constraints |\n")
	b.WriteString("|-------|------|----------|-------------|-------------|\n")

	for field, prop := range specProps {
		propMap, _ := prop.(map[string]interface{})
		fieldType := getType(propMap)
		isRequired := requiredSet[field]
		description := getString(propMap, "description")
		constraints := getConstraints(propMap)

		requiredStr := "No"
		if isRequired {
			requiredStr = "Yes"
		}

		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n",
			field, fieldType, requiredStr, description, constraints))
	}

	// Extract status properties if present
	status, _ := properties["status"].(map[string]interface{})
	if status != nil {
		statusProps, _ := status["properties"].(map[string]interface{})
		if len(statusProps) > 0 {
			b.WriteString("\n## Status Fields\n\n")
			b.WriteString("| Field | Type | Required | Description | Constraints |\n")
			b.WriteString("|-------|------|----------|-------------|-------------|\n")

			for field, prop := range statusProps {
				propMap, _ := prop.(map[string]interface{})
				fieldType := getType(propMap)
				description := getString(propMap, "description")
				constraints := getConstraints(propMap)

				b.WriteString(fmt.Sprintf("| `%s` | %s | No | %s | %s |\n",
					field, fieldType, description, constraints))
			}
		}
	}

	return b.String()
}

func getType(prop map[string]interface{}) string {
	if t, ok := prop["type"].(string); ok {
		return t
	}
	if items, ok := prop["items"].(map[string]interface{}); ok {
		if itemType, ok := items["type"].(string); ok {
			return fmt.Sprintf("[]%s", itemType)
		}
	}
	return "object"
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getConstraints(prop map[string]interface{}) string {
	var constraints []string

	if pattern, ok := prop["pattern"].(string); ok {
		constraints = append(constraints, fmt.Sprintf("pattern: `%s`", pattern))
	}

	if enum, ok := prop["enum"].([]interface{}); ok && len(enum) > 0 {
		enumStrs := make([]string, len(enum))
		for i, e := range enum {
			enumStrs[i] = fmt.Sprintf("%v", e)
		}
		constraints = append(constraints, fmt.Sprintf("enum: %s", strings.Join(enumStrs, ", ")))
	}

	if min, ok := prop["minimum"].(float64); ok {
		constraints = append(constraints, fmt.Sprintf("min: %v", min))
	}

	if max, ok := prop["maximum"].(float64); ok {
		constraints = append(constraints, fmt.Sprintf("max: %v", max))
	}

	if len(constraints) == 0 {
		return "-"
	}

	return strings.Join(constraints, "; ")
}
