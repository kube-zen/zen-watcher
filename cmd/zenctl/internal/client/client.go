package client

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewDynamicClient creates a dynamic Kubernetes client from kubeconfig or in-cluster config
func NewDynamicClient(kubeconfig, context string) (dynamic.Interface, *rest.Config, error) {
	var config *rest.Config
	var err error

	// Try in-cluster config first
	config, err = rest.InClusterConfig()
	if err == nil {
		dynClient, err := dynamic.NewForConfig(config)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create dynamic client from in-cluster config: %w", err)
		}
		return dynClient, config, nil
	}

	// Fall back to kubeconfig
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get user home directory: %w", err)
			}
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	if context != "" {
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			&clientcmd.ConfigOverrides{CurrentContext: context},
		).ClientConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build config with context %s: %w", context, err)
		}
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return dynClient, config, nil
}

