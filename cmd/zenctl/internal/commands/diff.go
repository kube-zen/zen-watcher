package commands

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/client"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/discovery"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/cached/disk"
)

func NewDiffCommand() *cobra.Command {
	var filePath string
	var ignoreStatus bool
	var ignoreAnnotations bool

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

			// Load desired manifests
			desiredObjects, err := loadManifests(filePath)
			if err != nil {
				return fmt.Errorf("failed to load manifests: %w", err)
			}

			var drifts []string
			var errors []string

			// Compare each desired object with live state
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

				gvr, err := resolver.ResolveGVR(gvk)
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s %s/%s: CRD not found: %v", gvk.Kind, desired.GetNamespace(), desired.GetName(), err))
					continue
				}

				// Get live object
				objNS := desired.GetNamespace()
				if objNS == "" {
					objNS = "default"
				}
				live, err := dynClient.Resource(gvr).Namespace(objNS).Get(ctx, desired.GetName(), metav1.GetOptions{})
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s %s/%s: not found in cluster: %v", gvk.Kind, objNS, desired.GetName(), err))
					continue
				}

				// Normalize both objects
				desiredNormalized := normalizeForDiff(desired, ignoreStatus, ignoreAnnotations)
				liveNormalized := normalizeForDiff(live, ignoreStatus, ignoreAnnotations)

				// Generate diff
				diff := generateDiff(desiredNormalized, liveNormalized, fmt.Sprintf("%s/%s/%s", gvk.Kind, objNS, desired.GetName()))
				if diff != "" {
					drifts = append(drifts, diff)
				}
			}

			// Report errors
			if len(errors) > 0 {
				for _, errMsg := range errors {
					cmd.PrintErrln("ERROR:", errMsg)
				}
				return fmt.Errorf("validation failed: %d error(s)", len(errors))
			}

			// Report drifts
			if len(drifts) > 0 {
				for _, drift := range drifts {
					cmd.Println(drift)
				}
				// Exit code 2 for drift detected
				os.Exit(2)
			}

			cmd.Println("No drift detected")
			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to YAML file or directory (required)")
	cmd.Flags().BoolVar(&ignoreStatus, "ignore-status", false, "Ignore status field in diff")
	cmd.Flags().BoolVar(&ignoreAnnotations, "ignore-annotations", false, "Ignore annotations in diff")

	return cmd
}

// loadManifests loads YAML manifests from file or directory
func loadManifests(path string) ([]*unstructured.Unstructured, error) {
	var objects []*unstructured.Unstructured

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		err = filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
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

// loadYAMLFile loads YAML file (supports multi-document)
func loadYAMLFile(path string) ([]*unstructured.Unstructured, error) {
	data, err := os.ReadFile(path)
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

// generateDiff generates a text diff between desired and live objects
func generateDiff(desired, live map[string]interface{}, resourceName string) string {
	desiredYAML, err := yaml.Marshal(desired)
	if err != nil {
		return fmt.Sprintf("%s: failed to marshal desired: %v", resourceName, err)
	}

	liveYAML, err := yaml.Marshal(live)
	if err != nil {
		return fmt.Sprintf("%s: failed to marshal live: %v", resourceName, err)
	}

	if string(desiredYAML) == string(liveYAML) {
		return ""
	}

	// Simple line-by-line diff
	desiredLines := strings.Split(string(desiredYAML), "\n")
	liveLines := strings.Split(string(liveYAML), "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- desired: %s\n", resourceName))
	diff.WriteString(fmt.Sprintf("+++ live: %s\n", resourceName))

	// For now, just show both versions
	// In production, use a proper diff library
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

	return diff.String()
}

