package commands

import (
	"fmt"
	"time"

	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/client"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/discovery"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/output"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/resources"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery/cached/disk"
)

func NewExplainCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain flow <name>",
		Short: "Explain a DeliveryFlow in detail",
		Long: `Prints detailed information about a DeliveryFlow including:
- Resolved sourceKey list
- Outputs + active target per output
- Last failover timestamp/reason if present
- Entitlement condition + reason`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "flow" {
				return fmt.Errorf("only 'flow' resource type is supported")
			}

			ctx := cmd.Context()
			opts := OptionsFromContext(ctx)

			name := args[1]
			namespace := opts.Namespace
			if namespace == "" {
				namespace = "default"
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

			// Resolve DeliveryFlow GVR
			gvr, err := resolver.ResolveGVR(discovery.ExpectedGVKs["DeliveryFlow"])
			if err != nil {
				return fmt.Errorf("DeliveryFlow CRD not installed; enable crds.enabled or apply CRDs separately: %w", err)
			}

			// Get flow
			flow, err := resources.GetDeliveryFlow(ctx, dynClient, gvr, namespace, name)
			if err != nil {
				return fmt.Errorf("failed to get DeliveryFlow: %w", err)
			}

			// Print detailed information
			return printFlowDetails(flow.Object)
		},
	}

	return cmd
}

func printFlowDetails(obj *unstructured.Unstructured) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()

	fmt.Printf("DeliveryFlow: %s/%s\n\n", namespace, name)

	// Extract and print sourceKey list from spec.sources
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	if sources, _, _ := unstructured.NestedSlice(spec, "sources"); sources != nil {
		fmt.Println("Resolved sourceKey list:")
		for i, s := range sources {
			if src, ok := s.(map[string]interface{}); ok {
				if ingesterRef, _, _ := unstructured.NestedMap(src, "ingesterRef"); ingesterRef != nil {
					ingName, _, _ := unstructured.NestedString(ingesterRef, "name")
					ingNamespace, _, _ := unstructured.NestedString(ingesterRef, "namespace")
					sourceName, _, _ := unstructured.NestedString(src, "sourceName")

					sourceKey := fmt.Sprintf("%s/%s", ingNamespace, ingName)
					if ingNamespace == "" {
						sourceKey = ingName
					}
					if sourceName != "" {
						sourceKey = fmt.Sprintf("%s/%s", sourceKey, sourceName)
					}
					fmt.Printf("  [%d] %s\n", i+1, sourceKey)
				}
			}
		}
		fmt.Println()
	}

	// Extract and print outputs + active target
	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	if outputs, _, _ := unstructured.NestedSlice(status, "outputs"); outputs != nil {
		fmt.Println("Outputs:")
		for i, o := range outputs {
			if outputMap, ok := o.(map[string]interface{}); ok {
				outputName, _, _ := unstructured.NestedString(outputMap, "name")
				if outputName == "" {
					outputName = fmt.Sprintf("output-%d", i+1)
				}

				fmt.Printf("  [%d] %s:\n", i+1, outputName)

				// Active target
				if at, _, _ := unstructured.NestedMap(outputMap, "activeTarget"); at != nil {
					if dr, _, _ := unstructured.NestedMap(at, "destinationRef"); dr != nil {
						destName, _, _ := unstructured.NestedString(dr, "name")
						destNamespace, _, _ := unstructured.NestedString(dr, "namespace")
						role, _, _ := unstructured.NestedString(at, "role")
						activeTarget := output.FormatActiveTarget(destNamespace, destName)
						fmt.Printf("    Active Target: %s (role: %s)\n", activeTarget, role)
					}
				}

				// Failover info
				failoverReason, _, _ := unstructured.NestedString(outputMap, "failoverReason")
				failoverTime, _, _ := unstructured.NestedString(outputMap, "failoverTime")
				if failoverReason != "" || failoverTime != "" {
					fmt.Printf("    Last Failover:\n")
					if failoverReason != "" {
						fmt.Printf("      Reason: %s\n", failoverReason)
					}
					if failoverTime != "" {
						if t, err := time.Parse(time.RFC3339, failoverTime); err == nil {
							fmt.Printf("      Time: %s (%s ago)\n", t.Format(time.RFC3339), output.FormatAge(t))
						} else {
							fmt.Printf("      Time: %s\n", failoverTime)
						}
					}
				}
			}
		}
		fmt.Println()
	}

	// Extract and print entitlement condition
	if conditions, _, _ := unstructured.NestedSlice(status, "conditions"); conditions != nil {
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				typ, _, _ := unstructured.NestedString(cond, "type")
				if typ == "Entitled" {
					status, _, _ := unstructured.NestedString(cond, "status")
					reason, _, _ := unstructured.NestedString(cond, "reason")
					message, _, _ := unstructured.NestedString(cond, "message")
					lastTransition, _, _ := unstructured.NestedString(cond, "lastTransitionTime")

					fmt.Println("Entitlement Condition:")
					fmt.Printf("  Status: %s\n", output.FormatEntitlement(status, reason))
					if reason != "" && reason != "<none>" {
						fmt.Printf("  Reason: %s\n", reason)
					}
					if message != "" {
						fmt.Printf("  Message: %s\n", message)
					}
					if lastTransition != "" {
						if t, err := time.Parse(time.RFC3339, lastTransition); err == nil {
							fmt.Printf("  Last Transition: %s (%s ago)\n", t.Format(time.RFC3339), output.FormatAge(t))
						} else {
							fmt.Printf("  Last Transition: %s\n", lastTransition)
						}
					}
					fmt.Println()
					break
				}
			}
		}
	}

	return nil
}
