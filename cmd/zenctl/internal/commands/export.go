package commands

import (
	"context"
	"encoding/json"
	"fmt"
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

func NewExportCommand() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "export <flow|ingester> <name>",
		Short: "Export resource as GitOps YAML bundle",
		Long: `Exports a resource as clean YAML/JSON suitable for GitOps.

Generates deterministic output without runtime metadata (status, resourceVersion, etc.).
Only includes spec and stable metadata fields.

Example:
  zenctl export flow my-flow -n zen-apps --format yaml
  zenctl export ingester my-ingester -n zen-apps --format json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			opts := OptionsFromContext(ctx)

			resourceType := args[0]
			resourceName := args[1]

			namespace := opts.Namespace
			if namespace == "" {
				return fmt.Errorf("namespace must be specified with -n or --namespace")
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
			var gvk schema.GroupVersionKind

			switch resourceType {
			case "flow":
				gvk = discovery.ExpectedGVKs["DeliveryFlow"]
			case "ingester":
				gvk = discovery.ExpectedGVKs["Ingester"]
			default:
				return fmt.Errorf("unsupported resource type: %s (supported: flow, ingester)", resourceType)
			}

			gvr, err = resolver.ResolveGVR(gvk)
			if err != nil {
				return fmt.Errorf("%s CRD not installed; enable crds.enabled or apply CRDs separately: %w", gvk.Kind, err)
			}

			// Fetch resource
			obj, err := dynClient.Resource(gvr).Namespace(namespace).Get(ctx, resourceName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get %s %s/%s: %w", gvk.Kind, namespace, resourceName, err)
			}

			// Clean object for GitOps (remove status, runtime metadata, secrets)
			cleanObj := cleanForGitOps(obj)

			// Output format
			if outputFormat == "" {
				outputFormat = "yaml"
			}

			var output []byte
			switch outputFormat {
			case "yaml":
				output, err = yaml.Marshal(cleanObj.Object)
				if err != nil {
					return fmt.Errorf("failed to marshal YAML: %w", err)
				}
			case "json":
				output, err = json.MarshalIndent(cleanObj.Object, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
			default:
				return fmt.Errorf("unsupported format: %s (supported: yaml, json)", outputFormat)
			}

			cmd.Print(string(output))
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "format", "f", "yaml", "Output format (yaml, json)")

	return cmd
}

// cleanForGitOps removes runtime metadata and secrets from an Unstructured object
func cleanForGitOps(obj *unstructured.Unstructured) *unstructured.Unstructured {
	clean := obj.DeepCopy()

	// Remove status (runtime state)
	delete(clean.Object, "status")

	// Clean metadata: keep labels/annotations, remove runtime fields
	if metadata, ok := clean.Object["metadata"].(map[string]interface{}); ok {
		delete(metadata, "creationTimestamp")
		delete(metadata, "generation")
		delete(metadata, "resourceVersion")
		delete(metadata, "uid")
		delete(metadata, "managedFields")
		delete(metadata, "selfLink")
		// Remove kubectl last-applied-configuration annotation
		if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
			delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
		}
	}

	// Redact known secret fields from spec
	redactSecrets(clean.Object, "spec")

	return clean
}

// redactSecrets recursively removes or redacts known secret fields
func redactSecrets(obj map[string]interface{}, path string) {
	if obj == nil {
		return
	}

	// Known secret field patterns to redact
	secretFields := []string{
		"token", "password", "secret", "apiKey", "api_key", "accessKey", "access_key",
		"secretKey", "secret_key", "credentials", "auth", "authorization",
	}

	for key, value := range obj {
		keyLower := strings.ToLower(key)
		for _, pattern := range secretFields {
			// Check if key matches secret pattern
			if strings.Contains(keyLower, pattern) {
				// Redact the value
				obj[key] = "[REDACTED]"
				continue
			}
		}

		// Recursively process nested maps
		if nestedMap, ok := value.(map[string]interface{}); ok {
			redactSecrets(nestedMap, path+"."+key)
		} else if nestedSlice, ok := value.([]interface{}); ok {
			for _, item := range nestedSlice {
				if itemMap, ok := item.(map[string]interface{}); ok {
					redactSecrets(itemMap, path+"."+key+"[]")
				}
			}
		}
	}
}
