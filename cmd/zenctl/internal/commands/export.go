package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/client"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/discovery"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func NewExportCommand() *cobra.Command {
	var namespace string
	var outDir string
	var kustomize bool

	cmd := &cobra.Command{
		Use:   "export <flow|ingester> <name>",
		Short: "Export resource as GitOps YAML bundle",
		Long: `Exports a resource as clean YAML suitable for GitOps.

Generates deterministic YAML without runtime metadata (e.g., status, 
creationTimestamp). Includes stable labels (tenant, cluster, adapter-group).

Optionally generates kustomization.yaml for the output directory.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceType := args[0]
			name := args[1]

			ctx := cmd.Context()
			opts := OptionsFromContext(ctx)

			if namespace == "" {
				namespace = opts.Namespace
				if namespace == "" {
					namespace = "default"
				}
			}

			if outDir == "" {
				outDir = fmt.Sprintf("%s-%s", resourceType, name)
			}

			// Create output directory
			if err := os.MkdirAll(outDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
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

			var gvr schema.GroupVersionResource
			var resourceName string

			switch resourceType {
			case "flow":
				gvrObj, err := resolver.ResolveGVR(discovery.ExpectedGVKs["DeliveryFlow"])
				if err != nil {
					return fmt.Errorf("DeliveryFlow CRD not installed; enable crds.enabled or apply CRDs separately: %w", err)
				}
				gvr = gvrObj
				resourceName = "DeliveryFlow"
			case "ingester":
				gvrObj, err := resolver.ResolveGVR(discovery.ExpectedGVKs["Ingester"])
				if err != nil {
					return fmt.Errorf("Ingester CRD not installed; enable crds.enabled or apply CRDs separately: %w", err)
				}
				gvr = gvrObj
				resourceName = "Ingester"
			default:
				return fmt.Errorf("unsupported resource type: %s (supported: flow, ingester)", resourceType)
			}

			// Fetch resource
			obj, err := dynClient.(dynamic.Interface).Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get %s: %w", resourceType, err)
			}

			// Clean the object (remove status, runtime metadata)
			cleanObj := cleanForGitOps(obj)

			// Write YAML
			yamlFile := filepath.Join(outDir, fmt.Sprintf("%s.yaml", name))
			if err := writeYAML(cleanObj, yamlFile); err != nil {
				return fmt.Errorf("failed to write YAML: %w", err)
			}

			cmd.Printf("Exported %s/%s to %s\n", resourceType, name, yamlFile)

			// Generate kustomization.yaml if requested
			if kustomize {
				kustomizationFile := filepath.Join(outDir, "kustomization.yaml")
				if err := writeKustomization(kustomizationFile, name, resourceName); err != nil {
					return fmt.Errorf("failed to write kustomization.yaml: %w", err)
				}
				cmd.Printf("Generated %s\n", kustomizationFile)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory (default: <resource-type>-<name>)")
	cmd.Flags().BoolVar(&kustomize, "kustomize", false, "Generate kustomization.yaml")

	return cmd
}

func cleanForGitOps(obj *unstructured.Unstructured) *unstructured.Unstructured {
	clean := obj.DeepCopy()

	// Remove status
	delete(clean.Object, "status")

	// Clean metadata: keep labels/annotations, remove runtime fields
	if metadata, ok := clean.Object["metadata"].(map[string]interface{}); ok {
		delete(metadata, "creationTimestamp")
		delete(metadata, "generation")
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "managedFields")
	}

	return clean
}

func writeYAML(obj *unstructured.Unstructured, path string) error {
	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func writeKustomization(path, resourceName, kind string) error {
	kustomization := map[string]interface{}{
		"apiVersion": "kustomize.config.k8s.io/v1beta1",
		"kind":       "Kustomization",
		"resources":  []string{resourceName + ".yaml"},
	}

	data, err := yaml.Marshal(kustomization)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

