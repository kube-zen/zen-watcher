package commands

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/client"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/discovery"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
)

func NewValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate cluster configuration and contracts",
		Long: `Validates cluster configuration including:
- Required CRDs are installed
- Canonical sourceKey contract (namespace/ingesterName/sourceName)
- DeliveryFlow target group structure (primary/standby rules)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			opts := OptionsFromContext(ctx)

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

			var failures []string

			// Check CRDs
			cmd.Println("Validating CRDs...")
			requiredCRDs := []string{"DeliveryFlow", "Destination", "Ingester"}
			for _, kind := range requiredCRDs {
				gvk := discovery.ExpectedGVKs[kind]
				_, err := resolver.ResolveGVR(gvk)
				if err != nil {
					failures = append(failures, fmt.Sprintf("CRD %s (%s) not installed: %v", kind, gvk.GroupVersion().String(), err))
				} else {
					cmd.Printf("  ✓ %s CRD installed\n", kind)
				}
			}

			// Validate sourceKey contracts if CRDs are available
			if err == nil {
				cmd.Println("\nValidating sourceKey contracts...")
				sourceKeyFailures := validateSourceKeyContracts(ctx, dynClient, resolver, opts.Namespace, opts.AllNamespaces)
				failures = append(failures, sourceKeyFailures...)
			}

			// Validate DeliveryFlow target groups if CRDs are available
			if err == nil {
				cmd.Println("\nValidating DeliveryFlow target groups...")
				targetFailures := validateDeliveryFlowTargets(ctx, dynClient, resolver, opts.Namespace, opts.AllNamespaces)
				failures = append(failures, targetFailures...)
			}

			if len(failures) > 0 {
				cmd.Println("\nValidation failures:")
				for _, failure := range failures {
					cmd.Printf("  ✗ %s\n", failure)
				}
				return fmt.Errorf("validation failed with %d error(s)", len(failures))
			}

			cmd.Println("\n✓ All validations passed")
			return nil
		},
	}

	return cmd
}

// validateSourceKeyContracts validates canonical sourceKey format: namespace/ingesterName/sourceName
func validateSourceKeyContracts(ctx context.Context, dynClient dynamic.Interface, resolver *discovery.ResourceResolver, namespace string, allNamespaces bool) []string {
	var failures []string

	gvk := discovery.ExpectedGVKs["Ingester"]
	gvr, err := resolver.ResolveGVR(gvk)
	if err != nil {
		return failures // CRD not available, skip
	}

	// sourceKey pattern: namespace/ingesterName/sourceName
	sourceKeyPattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

	var listOpts metav1.ListOptions
	listInterface := dynClient.Resource(gvr)
	if allNamespaces {
		list, err := listInterface.List(ctx, listOpts)
		if err != nil {
			return []string{fmt.Sprintf("failed to list Ingesters: %v", err)}
		}
		for _, item := range list.Items {
			failures = append(failures, validateIngesterSourceKeys(&item, sourceKeyPattern)...)
		}
	} else {
		if namespace == "" {
			namespace = "default"
		}
		list, err := listInterface.Namespace(namespace).List(ctx, listOpts)
		if err != nil {
			return []string{fmt.Sprintf("failed to list Ingesters in namespace %s: %v", namespace, err)}
		}
		for _, item := range list.Items {
			failures = append(failures, validateIngesterSourceKeys(&item, sourceKeyPattern)...)
		}
	}

	return failures
}

func validateIngesterSourceKeys(ingester *unstructured.Unstructured, pattern *regexp.Regexp) []string {
	var failures []string
	ns := ingester.GetNamespace()
	name := ingester.GetName()

	// Check spec.sources for sourceKey fields
	sources, found, _ := unstructured.NestedSlice(ingester.Object, "spec", "sources")
	if !found {
		return failures // No sources to validate
	}

	for i, source := range sources {
		if sourceMap, ok := source.(map[string]interface{}); ok {
			sourceKey, found, _ := unstructured.NestedString(sourceMap, "sourceKey")
			if found && sourceKey != "" {
				if !pattern.MatchString(sourceKey) {
					failures = append(failures, fmt.Sprintf("Ingester %s/%s spec.sources[%d].sourceKey=%q does not match canonical format (namespace/ingesterName/sourceName)", ns, name, i, sourceKey))
				} else {
					// Verify it matches the ingester's namespace and name
					parts := strings.Split(sourceKey, "/")
					if len(parts) == 3 {
						if parts[0] != ns || parts[1] != name {
							failures = append(failures, fmt.Sprintf("Ingester %s/%s spec.sources[%d].sourceKey=%q namespace/ingesterName (%s/%s) does not match ingester location", ns, name, i, sourceKey, parts[0], parts[1]))
						}
					}
				}
			}
		}
	}

	return failures
}

// validateDeliveryFlowTargets validates DeliveryFlow target group structure (primary/standby rules)
func validateDeliveryFlowTargets(ctx context.Context, dynClient dynamic.Interface, resolver *discovery.ResourceResolver, namespace string, allNamespaces bool) []string {
	var failures []string

	gvk := discovery.ExpectedGVKs["DeliveryFlow"]
	gvr, err := resolver.ResolveGVR(gvk)
	if err != nil {
		return failures // CRD not available, skip
	}

	var listOpts metav1.ListOptions
	listInterface := dynClient.Resource(gvr)
	if allNamespaces {
		list, err := listInterface.List(ctx, listOpts)
		if err != nil {
			return []string{fmt.Sprintf("failed to list DeliveryFlows: %v", err)}
		}
		for _, item := range list.Items {
			failures = append(failures, validateFlowTargetStructure(&item)...)
		}
	} else {
		if namespace == "" {
			namespace = "default"
		}
		list, err := listInterface.Namespace(namespace).List(ctx, listOpts)
		if err != nil {
			return []string{fmt.Sprintf("failed to list DeliveryFlows in namespace %s: %v", namespace, err)}
		}
		for _, item := range list.Items {
			failures = append(failures, validateFlowTargetStructure(&item)...)
		}
	}

	return failures
}

func validateFlowTargetStructure(flow *unstructured.Unstructured) []string {
	var failures []string
	ns := flow.GetNamespace()
	name := flow.GetName()

	// Check spec.outputs for target group structure
	outputs, found, _ := unstructured.NestedSlice(flow.Object, "spec", "outputs")
	if !found {
		return failures // No outputs to validate
	}

	for i, output := range outputs {
		if outputMap, ok := output.(map[string]interface{}); ok {
			targets, found, _ := unstructured.NestedSlice(outputMap, "targets")
			if !found || len(targets) == 0 {
				continue // No targets to validate
			}

			// Check for primary/standby structure
			hasPrimary := false
			for j, target := range targets {
				if targetMap, ok := target.(map[string]interface{}); ok {
					priority, found, _ := unstructured.NestedInt64(targetMap, "priority")
					if found && priority == 0 {
						hasPrimary = true
					}
					// Validate target structure
					_, hasName := targetMap["name"]
					if !hasName {
						failures = append(failures, fmt.Sprintf("DeliveryFlow %s/%s spec.outputs[%d].targets[%d] missing required 'name' field", ns, name, i, j))
					}
				}
			}

			if !hasPrimary && len(targets) > 1 {
				failures = append(failures, fmt.Sprintf("DeliveryFlow %s/%s spec.outputs[%d] has multiple targets but no primary (priority=0)", ns, name, i))
			}
		}
	}

	return failures
}

