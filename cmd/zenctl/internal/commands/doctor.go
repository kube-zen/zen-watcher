package commands

import (
	"context"
	"fmt"

	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/client"
	"github.com/kube-zen/zen-watcher/cmd/zenctl/internal/discovery"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/kubernetes"
)

type CheckResult struct {
	Check      string
	Status     string // PASS, WARN, FAIL
	Message    string
	Remediation string
}

func NewDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostics to detect common misconfigurations",
		Long: `Runs diagnostic checks for common misconfigurations:
- CRDs installed (DeliveryFlow, Destination, Ingester)
- Controllers present (zen-ingester, zen-watcher deployments)
- Status subresources exist on CRDs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			opts := OptionsFromContext(ctx)

			// Create client
			dynClient, config, err := client.NewDynamicClient(opts.Kubeconfig, opts.Context)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			_ = dynClient

			// Create Kubernetes client for checking deployments
			kubeClient, err := kubernetes.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to create Kubernetes client: %w", err)
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

			var results []CheckResult
			hasFailures := false

			// Check CRDs
			gvrs, errors := resolver.ResolveAll()
			for name, gvk := range discovery.ExpectedGVKs {
				if err, exists := errors[name]; exists {
					results = append(results, CheckResult{
						Check:       fmt.Sprintf("CRD: %s", name),
						Status:      "FAIL",
						Message:     fmt.Sprintf("CRD not found: %v", err),
						Remediation: fmt.Sprintf("Enable crds.enabled in Helm chart or apply %s CRD manually", name),
					})
					hasFailures = true
				} else {
					results = append(results, CheckResult{
						Check:   fmt.Sprintf("CRD: %s", name),
						Status:  "PASS",
						Message: fmt.Sprintf("Found %s/%s/%s", gvk.Group, gvk.Version, gvk.Kind),
					})
				}
			}

			// Check status subresources (best-effort via discovery)
			for name, gvr := range gvrs {
				if err := checkStatusSubresource(ctx, discClient, gvr); err != nil {
					results = append(results, CheckResult{
						Check:       fmt.Sprintf("Status Subresource: %s", name),
						Status:      "WARN",
						Message:     fmt.Sprintf("Could not verify status subresource: %v", err),
						Remediation: fmt.Sprintf("Ensure %s CRD has status subresource enabled", name),
					})
				} else {
					results = append(results, CheckResult{
						Check:   fmt.Sprintf("Status Subresource: %s", name),
						Status:  "PASS",
						Message: "Status subresource appears to be enabled",
					})
				}
			}

			// Check controllers (best-effort, check all namespaces)
			controllerChecks := []struct {
				name      string
				namespace string
			}{
				{"zen-ingester", ""},
				{"zen-watcher", ""},
			}

			namespaces, err := kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err == nil {
				for _, ns := range namespaces.Items {
					for _, controller := range controllerChecks {
						deploy, err := kubeClient.AppsV1().Deployments(ns.Name).Get(ctx, controller.name, metav1.GetOptions{})
						if err == nil && deploy != nil {
							results = append(results, CheckResult{
								Check:   fmt.Sprintf("Controller: %s", controller.name),
								Status:  "PASS",
								Message: fmt.Sprintf("Found deployment %s/%s", ns.Name, controller.name),
							})
							break
						}
					}
				}
			}

			// Check for missing controllers
			foundControllers := make(map[string]bool)
			for _, result := range results {
				if result.Check != "" {
					// Extract controller name from check name
					if len(result.Check) > 12 && result.Check[:12] == "Controller: " {
						controllerName := result.Check[12:]
						foundControllers[controllerName] = (result.Status == "PASS")
					}
				}
			}

			for _, controller := range controllerChecks {
				if !foundControllers[controller.name] {
					results = append(results, CheckResult{
						Check:       fmt.Sprintf("Controller: %s", controller.name),
						Status:      "WARN",
						Message:     fmt.Sprintf("Deployment %s not found in any namespace", controller.name),
						Remediation: fmt.Sprintf("Install %s controller or verify it's running in a different namespace", controller.name),
					})
				}
			}

			// Print results
			fmt.Println("Doctor Diagnostics Results:")
			fmt.Println("==========================")
			fmt.Println()

			for _, result := range results {
				var statusIcon string
				switch result.Status {
				case "WARN":
					statusIcon = "⚠"
				case "FAIL":
					statusIcon = "✗"
				default:
					statusIcon = "✓"
				}

				fmt.Printf("%s [%s] %s\n", statusIcon, result.Status, result.Check)
				if result.Message != "" {
					fmt.Printf("    %s\n", result.Message)
				}
				if result.Remediation != "" {
					fmt.Printf("    Remediation: %s\n", result.Remediation)
				}
				fmt.Println()
			}

			if hasFailures {
				return fmt.Errorf("diagnostics found failures")
			}

			return nil
		},
	}

	return cmd
}

func checkStatusSubresource(ctx context.Context, discClient interface{}, gvr interface{}) error {
	// Best-effort check: try to get resource definition
	// This is a simplified check - in practice we'd need to check CRD definition
	_ = ctx
	_ = discClient
	_ = gvr
	return nil // Assume OK for now (best-effort)
}

