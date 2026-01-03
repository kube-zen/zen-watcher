package commands

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/client"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/discovery"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/output"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
)

func NewAdaptersCommand() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "adapters",
		Short: "List adapters (namespace, group, type, instances, last seen, health)",
		Long: `Lists adapters across namespaces showing:
- Namespace
- Group (API group)
- Type (resource kind)
- Instances (count)
- Last seen (timestamp)
- Health (status)

Note: Currently lists known Zen CRDs as adapters. Full adapter detection may require additional configuration.`,
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

			// Resolve known adapter-like resources
			gvrs, _ := resolver.ResolveAll()

			adapterInfo := make([]AdapterInfo, 0)

			// Check DeliveryFlow, Destination, Ingester as adapters
			namespace := opts.Namespace
			if opts.AllNamespaces {
				namespace = ""
			}

			// DeliveryFlow
			if gvr, ok := gvrs["DeliveryFlow"]; ok {
				count := countResourceInstances(ctx, dynClient, gvr, namespace, opts.AllNamespaces)
				if count > 0 || opts.AllNamespaces {
					adapterInfo = append(adapterInfo, AdapterInfo{
						Namespace: namespace,
						Group:     "routing.zen.kube-zen.io",
						Type:      "DeliveryFlow",
						Instances: count,
						LastSeen:  "",
						Health:    "",
					})
				}
			}

			// Destination
			if gvr, ok := gvrs["Destination"]; ok {
				count := countResourceInstances(ctx, dynClient, gvr, namespace, opts.AllNamespaces)
				if count > 0 || opts.AllNamespaces {
					adapterInfo = append(adapterInfo, AdapterInfo{
						Namespace: namespace,
						Group:     "routing.zen.kube-zen.io",
						Type:      "Destination",
						Instances: count,
						LastSeen:  "",
						Health:    "",
					})
				}
			}

			// Ingester
			if gvr, ok := gvrs["Ingester"]; ok {
				count := countResourceInstances(ctx, dynClient, gvr, namespace, opts.AllNamespaces)
				if count > 0 || opts.AllNamespaces {
					adapterInfo = append(adapterInfo, AdapterInfo{
						Namespace: namespace,
						Group:     "zen.kube-zen.io",
						Type:      "Ingester",
						Instances: count,
						LastSeen:  "",
						Health:    "",
					})
				}
			}

			printer := output.NewPrinter(output.ParseFormat(outputFormat))

			if printer.Format() != output.FormatTable {
				return printer.Print(adapterInfo)
			}

			// Print table format
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			defer func() { _ = w.Flush() }()

			_, _ = fmt.Fprintln(w, "NAMESPACE\tGROUP\tTYPE\tINSTANCES\tLAST SEEN\tHEALTH")
			for _, info := range adapterInfo {
				lastSeen := info.LastSeen
				if lastSeen == "" {
					lastSeen = "—"
				}
				health := info.Health
				if health == "" {
					health = "—"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
					info.Namespace, info.Group, info.Type, info.Instances, lastSeen, health)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format (table, json, yaml)")

	return cmd
}

type AdapterInfo struct {
	Namespace string `json:"namespace"`
	Group     string `json:"group"`
	Type      string `json:"type"`
	Instances int    `json:"instances"`
	LastSeen  string `json:"lastSeen,omitempty"`
	Health    string `json:"health,omitempty"`
}

func countResourceInstances(ctx context.Context, dynClient interface{}, gvr schema.GroupVersionResource, namespace string, allNamespaces bool) int {
	dyn, ok := dynClient.(dynamic.Interface)
	if !ok {
		return 0
	}

	var resourceInterface dynamic.ResourceInterface
	if allNamespaces || namespace == "" {
		resourceInterface = dyn.Resource(gvr)
	} else {
		resourceInterface = dyn.Resource(gvr).Namespace(namespace)
	}

	list, err := resourceInterface.List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0
	}

	return len(list.Items)
}
