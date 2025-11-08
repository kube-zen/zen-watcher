package manager

import (
	"fmt"
	"log"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/detection"
	"github.com/kube-zen/zen-watcher/pkg/installation"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ToolManager manages security tool detection and installation
type ToolManager struct {
	detector  *detection.ToolDetector
	installer *installation.ToolInstaller
}

// ToolManagementResult represents the result of tool management operations
type ToolManagementResult struct {
	ToolsDetected  map[string]*detection.ToolStatus            `json:"toolsDetected"`
	ToolsInstalled map[string]*installation.InstallationResult `json:"toolsInstalled"`
	OverallStatus  string                                      `json:"overallStatus"`
	LastChecked    time.Time                                   `json:"lastChecked"`
	Errors         []string                                    `json:"errors,omitempty"`
}

// NewToolManager creates a new tool manager
func NewToolManager(clientSet *kubernetes.Clientset, config *rest.Config, namespace string) *ToolManager {
	// Create dynamic client from config
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Printf("⚠️ Failed to create dynamic client: %v", err)
		dynamicClient = nil
	}

	detector := detection.NewToolDetector(clientSet, dynamicClient, namespace)
	installer := installation.NewToolInstaller(clientSet, config, namespace)

	return &ToolManager{
		detector:  detector,
		installer: installer,
	}
}

// EnsureToolsInstalled ensures all required security tools are installed
func (tm *ToolManager) EnsureToolsInstalled() (*ToolManagementResult, error) {
	log.Println("🔧 Ensuring security tools are installed...")

	result := &ToolManagementResult{
		ToolsDetected:  make(map[string]*detection.ToolStatus),
		ToolsInstalled: make(map[string]*installation.InstallationResult),
		LastChecked:    time.Now(),
		Errors:         []string{},
	}

	// Detect all tools
	tools, err := tm.detector.DetectAllTools()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Detection failed: %v", err))
		return result, err
	}

	result.ToolsDetected = tools

	// Check which tools need installation
	toolsToInstall := []string{}
	for toolName, toolStatus := range tools {
		if !toolStatus.Installed || toolStatus.HealthStatus == "unhealthy" {
			toolsToInstall = append(toolsToInstall, toolName)
		}
	}

	if len(toolsToInstall) == 0 {
		result.OverallStatus = "all-tools-installed"
		log.Println("✅ All required tools are installed and healthy")
		return result, nil
	}

	log.Printf("📦 Tools requiring installation: %v", toolsToInstall)

	// Install missing tools
	for _, toolName := range toolsToInstall {
		log.Printf("🚀 Installing %s...", toolName)

		var installResult *installation.InstallationResult
		var installErr error

		switch toolName {
		case "trivy":
			installResult, installErr = tm.installer.InstallTrivy()
		case "falco":
			installResult, installErr = tm.installer.InstallFalco()
		case "kyverno":
			installResult, installErr = tm.installer.InstallKyverno()
		case "kube-bench", "kubebench":
			installResult, installErr = tm.installer.InstallKubeBench()
		case "kubernetes-audit", "audit", "k8s-audit":
			installResult, installErr = tm.installer.InstallKubernetesAudit()
		default:
			installErr = fmt.Errorf("installation not implemented for: %s", toolName)
		}

		if installErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to install %s: %v", toolName, installErr))
			log.Printf("❌ Failed to install %s: %v", toolName, installErr)
			continue
		}

		result.ToolsInstalled[toolName] = installResult
		log.Printf("✅ Successfully installed %s", toolName)
	}

	// Re-detect tools after installation
	log.Println("🔍 Re-detecting tools after installation...")
	updatedTools, err := tm.detector.DetectAllTools()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Re-detection failed: %v", err))
	} else {
		result.ToolsDetected = updatedTools
	}

	// Determine overall status
	if len(result.Errors) == 0 {
		result.OverallStatus = "success"
	} else {
		result.OverallStatus = "partial-success"
	}

	log.Printf("✅ Tool management completed with status: %s", result.OverallStatus)
	return result, nil
}

// GetToolStatus returns the current status of all tools
func (tm *ToolManager) GetToolStatus() (*ToolManagementResult, error) {
	log.Println("🔍 Getting tool status...")

	result := &ToolManagementResult{
		ToolsDetected:  make(map[string]*detection.ToolStatus),
		ToolsInstalled: make(map[string]*installation.InstallationResult),
		LastChecked:    time.Now(),
		Errors:         []string{},
	}

	// Detect all tools
	tools, err := tm.detector.DetectAllTools()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Detection failed: %v", err))
		return result, err
	}

	result.ToolsDetected = tools

	// Determine overall status
	allHealthy := true
	for _, toolStatus := range tools {
		if !toolStatus.Installed || toolStatus.HealthStatus != "healthy" {
			allHealthy = false
			break
		}
	}

	if allHealthy {
		result.OverallStatus = "all-tools-healthy"
	} else {
		result.OverallStatus = "some-tools-unhealthy"
	}

	log.Printf("✅ Tool status check completed with status: %s", result.OverallStatus)
	return result, nil
}

// InstallSpecificTool installs a specific tool
func (tm *ToolManager) InstallSpecificTool(toolName string) (*installation.InstallationResult, error) {
	log.Printf("🚀 Installing specific tool: %s", toolName)

	switch toolName {
	case "trivy":
		return tm.installer.InstallTrivy()
	case "falco":
		return tm.installer.InstallFalco()
	case "kyverno":
		return tm.installer.InstallKyverno()
	case "kube-bench", "kubebench":
		return tm.installer.InstallKubeBench()
	case "kubernetes-audit", "audit", "k8s-audit":
		return tm.installer.InstallKubernetesAudit()
	default:
		return nil, fmt.Errorf("installation not implemented for: %s", toolName)
	}
}

// DetectSpecificTool detects a specific tool
func (tm *ToolManager) DetectSpecificTool(toolName string) (*detection.ToolStatus, error) {
	log.Printf("🔍 Detecting specific tool: %s", toolName)

	return tm.detector.GetToolStatus(toolName)
}
