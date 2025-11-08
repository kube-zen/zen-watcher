package config

import (
	"os"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Config holds the application configuration
type Config struct {
	KubeConfig     *rest.Config
	ClientSet      *kubernetes.Clientset
	DynamicClient  dynamic.Interface
	TrivyNamespace string
	WatchNamespace string
	Behavior       *BehaviorConfig
}

// LoadConfig initializes the Kubernetes configuration
func LoadConfig() (*Config, error) {
	var config *rest.Config
	var err error

	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		// Running inside a pod
		config, err = rest.InClusterConfig()
	} else {
		// Running locally
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Get namespaces from environment variables with defaults
	trivyNamespace := os.Getenv("TRIVY_NAMESPACE")
	if trivyNamespace == "" {
		trivyNamespace = "trivy"
	}

	watchNamespace := os.Getenv("WATCH_NAMESPACE")
	if watchNamespace == "" {
		watchNamespace = "default"
	}

	// Load behavior configuration
	behaviorConfig, err := LoadBehaviorConfig()
	if err != nil {
		return nil, err
	}

	return &Config{
		KubeConfig:     config,
		ClientSet:      clientset,
		DynamicClient:  dynamicClient,
		TrivyNamespace: trivyNamespace,
		WatchNamespace: watchNamespace,
		Behavior:       behaviorConfig,
	}, nil
}
