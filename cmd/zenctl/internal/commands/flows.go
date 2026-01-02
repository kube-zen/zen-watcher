package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/client"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/discovery"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/output"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/resources"
	"github.com/spf13/cobra"
	"k8s.io/client-go/discovery/cached/disk"
)

func NewFlowsCommand() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "flows",
		Short: "List DeliveryFlows in table format",
		Long: `Lists DeliveryFlows in a table format aligned with ACTIVE_TARGET_UX_GUIDE.md.
Columns: NAMESPACE | NAME | ACTIVE_TARGET | ENTITLEMENT | ENTITLEMENT_REASON | READY | AGE`,
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

			// Resolve DeliveryFlow GVR
			gvr, err := resolver.ResolveGVR(discovery.ExpectedGVKs["DeliveryFlow"])
			if err != nil {
				return fmt.Errorf("DeliveryFlow CRD not installed; enable crds.enabled or apply CRDs separately: %w", err)
			}

			// Determine namespace
			namespace := opts.Namespace
			if opts.AllNamespaces {
				namespace = ""
			}

			// List flows
			flows, err := resources.ListDeliveryFlows(ctx, dynClient, gvr, namespace, opts.AllNamespaces)
			if err != nil {
				return fmt.Errorf("failed to list DeliveryFlows: %w", err)
			}

			printer := output.NewPrinter(output.ParseFormat(outputFormat))

			if printer.Format() != output.FormatTable {
				return printer.Print(flows)
			}

			// Print table format
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			defer w.Flush()

			fmt.Fprintln(w, "NAMESPACE\tNAME\tACTIVE_TARGET\tENTITLEMENT\tENTITLEMENT_REASON\tREADY\tAGE")
			for _, f := range flows {
				activeTarget := output.FormatActiveTarget(f.ActiveNamespace, f.ActiveTarget)
				entitlement := output.FormatEntitlement(f.Entitlement, f.EntitlementReason)
				entitlementReason := f.EntitlementReason
				if entitlementReason == "" || entitlementReason == "<none>" {
					entitlementReason = "—"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					f.Namespace, f.Name, activeTarget, entitlement, entitlementReason, f.Ready, output.FormatAge(f.Object.GetCreationTimestamp().Time))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format (table, json, yaml)")

	return cmd
}
