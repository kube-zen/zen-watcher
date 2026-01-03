package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/client"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/discovery"
	clierrors "github.com/kube-zen/zen-watcher/cmd/zenctl/internal/errors"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
)

func NewDiffCommand() *cobra.Command {
	var filePath string
	var ignoreStatus bool
	var ignoreAnnotations bool
	var outputFormat string
	var excludePatterns []string
	var reportFormat string
	var reportFilePath string
	var selectPatterns []string
	var labelSelector string

	cmd := &cobra.Command{
		Use:   "diff -f <file|dir>",
		Short: "Compare desired manifests with cluster state",
		Long: `Compares desired YAML manifests from file/directory with live cluster objects.

Produces deterministic diff output showing drift between desired and live state.
Exit codes:
  0: No drift detected
  2: Drift detected
  1: Error (missing CRD, RBAC, etc.)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("file or directory must be specified with -f")
			}

			ctx := cmd.Context()
			opts := OptionsFromContext(ctx)

			namespace := opts.Namespace
			if !opts.AllNamespaces && namespace == "" {
				return fmt.Errorf("namespace must be specified with -n or use -A for all namespaces")
			}

			// Create client
			dynClient, config, err := client.NewDynamicClient(opts.Kubeconfig, opts.Context)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			// Create discovery client and resolver
			discClient, err := disk.NewCachedDiscoveryClientForConfig(config, "", "", 0)
			if err != nil {
				return fmt.Errorf("failed to create discovery client: %w", err)
			}
			resolver, err := discovery.NewResourceResolver(discClient)
			if err != nil {
				return fmt.Errorf("failed to create resource resolver: %w", err)
			}

			// Load desired manifests with exclusions
			desiredObjects, err := loadManifests(filePath, excludePatterns)
			if err != nil {
				return fmt.Errorf("failed to load manifests: %w", err)
			}

			clusterContext := opts.Context
			if clusterContext == "" {
				clusterContext = "default"
			}

			// Parse and apply select patterns
			var selectPatternsParsed []SelectPattern
			var selectWarnings []string
			var filtersApplied *FiltersApplied
			var filteredObjects []*unstructured.Unstructured

			if len(selectPatterns) > 0 {
				parsed, parseWarnings, parseErr := NormalizeSelectPatterns(selectPatterns)
				if parseErr != nil {
					return fmt.Errorf("failed to parse select patterns: %w", parseErr)
				}
				if len(parseWarnings) > 0 {
					return fmt.Errorf("select pattern parse errors: %s", strings.Join(parseWarnings, "; "))
				}
				selectPatternsParsed = parsed

				// Filter by select patterns
				filteredObjects, selectWarnings = FilterObjectsBySelect(desiredObjects, selectPatternsParsed)
				if len(filteredObjects) == 0 {
					// All selections missed - emit JSON if requested, then exit 1
					if reportFormat == "json" {
						filtersApplied = &FiltersApplied{Select: selectPatterns}
						jsonReport := buildDiffReport(clusterContext, []ResourceReport{}, filtersApplied, selectWarnings)
						if reportFilePath != "" {
							if err := writeReportFile(jsonReport, reportFilePath); err != nil {
								cmd.PrintErrln("ERROR: Failed to write report file:", err)
							}
						} else {
							encoder := json.NewEncoder(os.Stdout)
							encoder.SetIndent("", "  ")
							encoder.Encode(jsonReport)
						}
					}
					for _, warn := range selectWarnings {
						cmd.PrintErrln("WARNING:", warn)
					}
					return clierrors.NewExitError(1, fmt.Errorf("all select patterns matched no resources"))
				}
			} else {
				filteredObjects = desiredObjects
			}

			// Apply label selector
			if labelSelector != "" {
				var labelErr error
				filteredObjects, labelErr = FilterObjectsByLabelSelector(filteredObjects, labelSelector)
				if labelErr != nil {
					return fmt.Errorf("label selector error: %w", labelErr)
				}
				if len(filteredObjects) == 0 && len(selectPatterns) == 0 {
					// Only label selector, no matches
					if reportFormat == "json" {
						filtersApplied = &FiltersApplied{LabelSelector: labelSelector}
						jsonReport := buildDiffReport(clusterContext, []ResourceReport{}, filtersApplied, []string{})
						if reportFilePath != "" {
							if err := writeReportFile(jsonReport, reportFilePath); err != nil {
								cmd.PrintErrln("ERROR: Failed to write report file:", err)
							}
						} else {
							encoder := json.NewEncoder(os.Stdout)
							encoder.SetIndent("", "  ")
							encoder.Encode(jsonReport)
						}
					}
					return clierrors.NewExitError(1, fmt.Errorf("label selector matched no resources"))
				}
			}

			// Build filtersApplied for JSON report
			if len(selectPatterns) > 0 || labelSelector != "" {
				filtersApplied = &FiltersApplied{}
				if len(selectPatterns) > 0 {
					filtersApplied.Select = selectPatterns
				}
				if labelSelector != "" {
					filtersApplied.LabelSelector = labelSelector
				}
			}

			var drifts []string
			var errorMessages []string
			var resourceReports []ResourceReport

			// Compare each desired object with live state
			drifts, resourceReports, errorMessages = compareObjects(
				ctx, filteredObjects, dynClient, resolver,
				ignoreStatus, ignoreAnnotations, outputFormat)

			// Sort resources for determinism
			resourceReports = sortResources(resourceReports)

			// Generate JSON report if requested
			var jsonReport *DiffReport
			if reportFormat == "json" {
				jsonReport = buildDiffReport(clusterContext, resourceReports, filtersApplied, selectWarnings)

				// Handle output precedence
				if reportFilePath != "" {
					// Write JSON to file (atomic)
					if err := writeReportFile(jsonReport, reportFilePath); err != nil {
						cmd.PrintErrln("ERROR: Failed to write report file:", err)
						return fmt.Errorf("report write failed: %w", err)
					}
					// Human output to stdout (preserved)
				} else {
					// Write JSON to stdout, suppress human output
					encoder := json.NewEncoder(os.Stdout)
					encoder.SetIndent("", "  ")
					if err := encoder.Encode(jsonReport); err != nil {
						return fmt.Errorf("failed to encode report: %w", err)
					}
					// Human output suppressed when JSON to stdout
				}
			}

			// Determine exit code based on resource reports
			hasErrors := false
			hasDrift := false
			for _, res := range resourceReports {
				switch res.Status {
				case "error":
					hasErrors = true
				case "drift":
					hasDrift = true
				}
			}

			// Report errors (always to stderr)
			if len(errorMessages) > 0 {
				for _, errMsg := range errorMessages {
					cmd.PrintErrln("ERROR:", errMsg)
				}
			}

			// Handle exit codes
			if hasErrors {
				return clierrors.NewExitError(1, fmt.Errorf("validation failed: %d error(s)", len(errorMessages)))
			}

			// Report drifts (to stdout if not JSON to stdout)
			if hasDrift {
				if reportFormat != "json" || reportFilePath != "" {
					// Human output enabled
					cmd.Printf("Drift Summary:\n")
					cmd.Printf("  Total resources: %d\n", len(resourceReports))
					driftCount := 0
					for _, res := range resourceReports {
						if res.Status == "drift" {
							driftCount++
						}
					}
					cmd.Printf("  Drifted resources: %d\n", driftCount)
					cmd.Println("")
					for _, drift := range drifts {
						cmd.Println(drift)
					}
				}
				return clierrors.NewExitError(2, fmt.Errorf("drift detected"))
			}

			// No drift
			if reportFormat != "json" || reportFilePath != "" {
				cmd.Println("No drift detected")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML file or directory (required)")
	cmd.Flags().BoolVar(&ignoreStatus, "ignore-status", false, "Ignore status field in diff")
	cmd.Flags().BoolVar(&ignoreAnnotations, "ignore-annotations", false, "Ignore annotations in diff")
	cmd.Flags().StringVar(&outputFormat, "format", "unified", "Diff output format (unified or plain)")
	cmd.Flags().StringArrayVar(&excludePatterns, "exclude", []string{}, "Exclude pattern (repeatable, gitignore-style)")
	cmd.Flags().StringVar(&reportFormat, "report", "", "Report format (json)")
	cmd.Flags().StringVar(&reportFilePath, "report-file", "", "Path to write JSON report (atomic write)")
	cmd.Flags().StringArrayVar(&selectPatterns, "select", []string{}, "Select pattern (repeatable, format: Kind/name, Kind/namespace/name, Group/Kind/name, or Group/Kind/namespace/name)")
	cmd.Flags().StringVar(&labelSelector, "label-selector", "", "Label selector (kubernetes label selector syntax)")

	return cmd
}

// compareObjects compares desired objects with live cluster state
func compareObjects(
	ctx context.Context,
	desiredObjects []*unstructured.Unstructured,
	dynClient dynamic.Interface,
	resolver *discovery.ResourceResolver,
	ignoreStatus, ignoreAnnotations bool,
	outputFormat string,
) ([]string, []ResourceReport, []string) {
	var drifts []string
	var errorMessages []string
	var resourceReports []ResourceReport

	for _, desired := range desiredObjects {
		// Resolve GVR
		gvk := schema.GroupVersionKind{
			Group:   desired.GetAPIVersion(),
			Version: "", // Will be parsed from APIVersion
			Kind:    desired.GetKind(),
		}
		if parts := strings.Split(desired.GetAPIVersion(), "/"); len(parts) == 2 {
			gvk.Group = parts[0]
			gvk.Version = parts[1]
		} else {
			gvk.Version = desired.GetAPIVersion()
		}

		objNS := desired.GetNamespace()
		if objNS == "" {
			objNS = "default"
		}

		// Create base ResourceReport
		resourceReport := ResourceReport{
			Group:     gvk.Group,
			Version:   gvk.Version,
			Kind:      gvk.Kind,
			Namespace: objNS,
			Name:      desired.GetName(),
			Status:    "no_drift",
			DriftType: "none",
			Redacted:  isSecretResource(desired),
		}

		// Try to resolve GVR and fetch live object
		gvr, err := resolver.ResolveGVR(gvk)
		if err != nil {
			resourceReport.Status = "error"
			resourceReport.DriftType = "unknown"
			resourceReport.Error = fmt.Sprintf("CRD not found: %v", err)
			resourceReports = append(resourceReports, resourceReport)
			errorMessages = append(errorMessages, fmt.Sprintf("%s %s/%s: CRD not found: %v", gvk.Kind, objNS, desired.GetName(), err))
			continue
		}

		live, err := dynClient.Resource(gvr).Namespace(objNS).Get(ctx, desired.GetName(), metav1.GetOptions{})
		if err != nil {
			resourceReport.Status = "error"
			resourceReport.DriftType = "unknown"
			resourceReport.Error = fmt.Sprintf("Resource not found: %v", err)
			resourceReports = append(resourceReports, resourceReport)
			errorMessages = append(errorMessages, fmt.Sprintf("%s %s/%s: not found in cluster: %v", gvk.Kind, objNS, desired.GetName(), err))
			continue
		}

		// Check if live object is also a Secret
		resourceReport.Redacted = isSecretResource(desired) || isSecretResource(live)

		// Normalize both objects
		desiredNormalized := normalizeForDiff(desired, ignoreStatus, ignoreAnnotations)
		liveNormalized := normalizeForDiff(live, ignoreStatus, ignoreAnnotations)

		// Generate diff
		format := outputFormat
		if format == "" {
			format = "unified"
		}
		diff, driftType := generateDiff(desiredNormalized, liveNormalized, fmt.Sprintf("%s/%s/%s", gvk.Kind, objNS, desired.GetName()), format)

		if diff != "" {
			resourceReport.Status = "drift"
			resourceReport.DriftType = driftType
			resourceReport.DiffStats = calculateDiffStats(diff)
			drifts = append(drifts, diff)
		} else {
			resourceReport.Status = "no_drift"
			resourceReport.DriftType = "none"
		}

		resourceReports = append(resourceReports, resourceReport)
	}

	return drifts, resourceReports, errorMessages
}

// loadManifests loads YAML manifests from file or directory with exclusions
func loadManifests(path string, excludePatterns []string) ([]*unstructured.Unstructured, error) {
	var objects []*unstructured.Unstructured

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Load .zenignore if it exists
	var zenignorePath string
	if info.IsDir() {
		zenignorePath = filepath.Join(path, ".zenignore")
	} else {
		zenignorePath = filepath.Join(filepath.Dir(path), ".zenignore")
	}

	ignorePatterns, err := loadZenignore(zenignorePath)
	if err != nil {
		// .zenignore not found is OK
		ignorePatterns = []string{}
	}

	// Merge CLI exclude patterns
	ignorePatterns = append(ignorePatterns, excludePatterns...)

	// Add default excludes
	defaultExcludes := []string{".git/", "node_modules/", "dist/", ".venv/", "vendor/", ".terraform/"}
	ignorePatterns = append(ignorePatterns, defaultExcludes...)

	if info.IsDir() {
		err = filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Check if path should be excluded
			relPath, err := filepath.Rel(path, filePath)
			if err != nil {
				return err
			}

			if shouldExclude(relPath, ignorePatterns) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(filePath), ".yaml") && !strings.HasSuffix(strings.ToLower(filePath), ".yml") {
				return nil
			}
			fileObjects, err := loadYAMLFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to load %s: %w", filePath, err)
			}
			objects = append(objects, fileObjects...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		objects, err = loadYAMLFile(path)
		if err != nil {
			return nil, err
		}
	}

	return objects, nil
}

// loadZenignore loads patterns from .zenignore file
func loadZenignore(path string) ([]string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is from user input (cli flag)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var patterns []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, nil
}

// shouldExclude checks if a path matches any exclude pattern (gitignore-style)
func shouldExclude(path string, patterns []string) bool {
	for _, pattern := range patterns {
		// Simple pattern matching (supports trailing / for directories)
		if strings.HasSuffix(pattern, "/") {
			// Directory pattern
			if strings.HasPrefix(path, pattern) || strings.Contains(path, "/"+pattern) {
				return true
			}
		} else {
			// File or exact match
			if strings.Contains(path, pattern) {
				return true
			}
		}
	}
	return false
}

// loadYAMLFile loads YAML file (supports multi-document)
func loadYAMLFile(path string) ([]*unstructured.Unstructured, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is from user input (cli flag)
	if err != nil {
		return nil, err
	}

	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	var objects []*unstructured.Unstructured

	for {
		var obj map[string]interface{}
		if err := decoder.Decode(&obj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("failed to decode YAML: %w", err)
		}
		if obj == nil {
			continue
		}

		unstructuredObj := &unstructured.Unstructured{Object: obj}
		objects = append(objects, unstructuredObj)
	}

	return objects, nil
}

// normalizeForDiff normalizes an object for comparison
func normalizeForDiff(obj *unstructured.Unstructured, ignoreStatus, ignoreAnnotations bool) map[string]interface{} {
	normalized := obj.DeepCopy().Object

	// Remove status
	if ignoreStatus {
		delete(normalized, "status")
	} else {
		delete(normalized, "status")
	}

	// Clean metadata
	if metadata, ok := normalized["metadata"].(map[string]interface{}); ok {
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "managedFields")
		delete(metadata, "creationTimestamp")
		delete(metadata, "generation")
		delete(metadata, "selfLink")

		if ignoreAnnotations {
			delete(metadata, "annotations")
		} else {
			// Remove kubectl last-applied-configuration
			if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
				delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
			}
		}
	}

	// Redact secrets
	redactSecrets(normalized, "")

	// Stable sort maps/slices
	sorted := stableSortObject(normalized)
	if sortedMap, ok := sorted.(map[string]interface{}); ok {
		return sortedMap
	}
	return normalized
}

// stableSortObject recursively sorts maps and slices for stable output
func stableSortObject(obj interface{}) interface{} {
	switch v := obj.(type) {
	case map[string]interface{}:
		sorted := make(map[string]interface{})
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sorted[k] = stableSortObject(v[k])
		}
		return sorted
	case []interface{}:
		sorted := make([]interface{}, len(v))
		for i, item := range v {
			sorted[i] = stableSortObject(item)
		}
		return sorted
	default:
		return v
	}
}

// generateDiff generates a unified diff between desired and live objects
// Returns: (diff string, drift type: "spec" or "metadata")
func generateDiff(desired, live map[string]interface{}, resourceName string, format string) (string, string) {
	desiredYAML, err := yaml.Marshal(desired)
	if err != nil {
		return fmt.Sprintf("%s: failed to marshal desired: %v", resourceName, err), "spec"
	}

	liveYAML, err := yaml.Marshal(live)
	if err != nil {
		return fmt.Sprintf("%s: failed to marshal live: %v", resourceName, err), "spec"
	}

	if string(desiredYAML) == string(liveYAML) {
		return "", ""
	}

	// Determine drift type (simple heuristic: check if spec changed)
	driftType := "metadata"
	if desiredSpec, ok := desired["spec"]; ok {
		if liveSpec, ok := live["spec"]; ok {
			desiredSpecYAML, _ := yaml.Marshal(desiredSpec)
			liveSpecYAML, _ := yaml.Marshal(liveSpec)
			if string(desiredSpecYAML) != string(liveSpecYAML) {
				driftType = "spec"
			}
		}
	}

	desiredLines := strings.Split(string(desiredYAML), "\n")
	liveLines := strings.Split(string(liveYAML), "\n")

	if format == "plain" {
		// Plain format (simple line-by-line)
		var diff strings.Builder
		diff.WriteString(fmt.Sprintf("--- desired: %s\n", resourceName))
		diff.WriteString(fmt.Sprintf("+++ live: %s\n", resourceName))

		maxLen := len(desiredLines)
		if len(liveLines) > maxLen {
			maxLen = len(liveLines)
		}

		for i := 0; i < maxLen; i++ {
			if i < len(desiredLines) && i < len(liveLines) {
				if desiredLines[i] != liveLines[i] {
					diff.WriteString(fmt.Sprintf("- %s\n", desiredLines[i]))
					diff.WriteString(fmt.Sprintf("+ %s\n", liveLines[i]))
				}
			} else if i < len(desiredLines) {
				diff.WriteString(fmt.Sprintf("- %s\n", desiredLines[i]))
			} else if i < len(liveLines) {
				diff.WriteString(fmt.Sprintf("+ %s\n", liveLines[i]))
			}
		}
		return diff.String(), driftType
	}

	// Unified diff format (default)
	unifiedDiff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        desiredLines,
		B:        liveLines,
		FromFile: fmt.Sprintf("desired: %s", resourceName),
		ToFile:   fmt.Sprintf("live: %s", resourceName),
		Context:  3,
	})
	if err != nil {
		// Fallback to plain format on error
		return generateDiff(desired, live, resourceName, "plain")
	}

	return unifiedDiff, driftType
}
