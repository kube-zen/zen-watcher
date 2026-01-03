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

func NewStatusCommand() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Summarize DeliveryFlows, Destinations, and Ingesters",
		Long: `Summarizes the status of DeliveryFlows, Destinations, and Ingesters
across the current or all namespaces.`,
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

			// Resolve GVRs
			gvrs, errors := resolver.ResolveAll()
			if len(errors) > 0 {
				for name, err := range errors {
					fmt.Fprintf(os.Stderr, "Warning: %s CRD not found: %v\n", name, err)
				}
			}

			printer := output.NewPrinter(output.ParseFormat(outputFormat))

			// Determine namespace
			namespace := opts.Namespace
			if opts.AllNamespaces {
				namespace = ""
			}

			type statusSummary struct {
				DeliveryFlows []resources.DeliveryFlow
				Destinations  []resources.Destination
				Ingesters     []resources.Ingester
			}

			summary := statusSummary{}

			// Collect status for each resource type
			if gvr, ok := gvrs["DeliveryFlow"]; ok {
				flows, err := resources.ListDeliveryFlows(ctx, dynClient, gvr, namespace, opts.AllNamespaces)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to list DeliveryFlows: %v\n", err)
				} else {
					summary.DeliveryFlows = flows
				}
			}

			if gvr, ok := gvrs["Destination"]; ok {
				dests, err := resources.ListDestinations(ctx, dynClient, gvr, namespace, opts.AllNamespaces)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to list Destinations: %v\n", err)
				} else {
					summary.Destinations = dests
				}
			}

			if gvr, ok := gvrs["Ingester"]; ok {
				ingesters, err := resources.ListIngesters(ctx, dynClient, gvr, namespace, opts.AllNamespaces)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to list Ingesters: %v\n", err)
				} else {
					summary.Ingesters = ingesters
				}
			}

			if printer.Format() != output.FormatTable {
				return printer.Print(summary)
			}

			// Print table format
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			defer func() { _ = w.Flush() }()

			_, _ = fmt.Fprintln(w, "\n=== DeliveryFlows ===")
			if len(summary.DeliveryFlows) == 0 {
				_, _ = fmt.Fprintln(w, "No DeliveryFlows found")
			} else {
				_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tACTIVE TARGET\tENTITLEMENT\tREADY\tAGE")
				for _, f := range summary.DeliveryFlows {
					activeTarget := output.FormatActiveTarget(f.ActiveNamespace, f.ActiveTarget)
					entitlement := output.FormatEntitlement(f.Entitlement, f.EntitlementReason)
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
						f.Namespace, f.Name, activeTarget, entitlement, f.Ready, output.FormatAge(f.Object.GetCreationTimestamp().Time))
				}
			}

			_, _ = fmt.Fprintln(w, "\n=== Destinations ===")
			if len(summary.Destinations) == 0 {
				_, _ = fmt.Fprintln(w, "No Destinations found")
			} else {
				_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tTYPE\tTRANSPORT\tHEALTH\tREADY\tAGE")
				for _, d := range summary.Destinations {
					health := d.Health
					if health == "" {
						health = "—"
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
						d.Namespace, d.Name, d.Type, d.Transport, health, d.Ready, output.FormatAge(d.Object.GetCreationTimestamp().Time))
				}
			}

			fmt.Fprintln(w, "\n=== Ingesters ===")
			if len(summary.Ingesters) == 0 {
				fmt.Fprintln(w, "No Ingesters found")
			} else {
				fmt.Fprintln(w, "NAMESPACE\tNAME\tSOURCES\tHEALTH\tLAST SEEN\tENTITLED\tBLOCKED\tREADY\tAGE")
				for _, i := range summary.Ingesters {
					health := i.SourceHealth
					if health == "" {
						health = "—"
					}
					lastSeen := i.LastSeen
					if lastSeen == "" {
						lastSeen = "—"
					}
					entitled := i.Entitled
					if entitled == "" {
						entitled = "—"
					}
					blocked := i.Blocked
					if blocked == "" {
						blocked = "—"
					}
					fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
						i.Namespace, i.Name, i.Sources, health, lastSeen, entitled, blocked, i.Ready, output.FormatAge(i.Object.GetCreationTimestamp().Time))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format (table, json, yaml)")

	return cmd
}
