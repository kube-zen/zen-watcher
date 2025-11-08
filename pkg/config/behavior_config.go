package config

import (
	"fmt"
	"os"
	"strings"
)

// BehaviorMode represents the monitoring behavior mode
type BehaviorMode string

const (
	// BehaviorModeFalcoOnly - Only Falco monitoring (current behavior)
	BehaviorModeFalcoOnly BehaviorMode = "falco-only"
	// BehaviorModeAll - All tools (Trivy, Falco, Audit)
	BehaviorModeAll BehaviorMode = "all"
	// BehaviorModeConservative - Only Trivy and Audit (no Falco)
	BehaviorModeConservative BehaviorMode = "conservative"
	// BehaviorModeTrivyOnly - Only Trivy monitoring
	BehaviorModeTrivyOnly BehaviorMode = "trivy-only"
	// BehaviorModeAuditOnly - Only Kubernetes audit monitoring
	BehaviorModeAuditOnly BehaviorMode = "audit-only"
	// BehaviorModeKyvernoOnly - Only Kyverno policy monitoring
	BehaviorModeKyvernoOnly BehaviorMode = "kyverno-only"
	// BehaviorModeKubeBenchOnly - Only Kube-bench monitoring
	BehaviorModeKubeBenchOnly BehaviorMode = "kube-bench-only"
)

// BehaviorConfig holds the behavior configuration
type BehaviorConfig struct {
	Mode BehaviorMode `json:"mode"`

	// Tool-specific configurations
	TrivyEnabled     bool `json:"trivy_enabled"`
	FalcoEnabled     bool `json:"falco_enabled"`
	AuditEnabled     bool `json:"audit_enabled"`
	KyvernoEnabled   bool `json:"kyverno_enabled"`
	KubeBenchEnabled bool `json:"kube_bench_enabled"`

	// Namespace configurations
	TrivyNamespace string `json:"trivy_namespace"`
	FalcoNamespace string `json:"falco_namespace"`
	WatchNamespace string `json:"watch_namespace"`
}

// LoadBehaviorConfig loads behavior configuration from environment variables
func LoadBehaviorConfig() (*BehaviorConfig, error) {
	// Get behavior mode from environment variable
	modeStr := os.Getenv("BEHAVIOR_MODE")
	if modeStr == "" {
		modeStr = "all" // Default to all tools
	}

	mode := BehaviorMode(strings.ToLower(modeStr))

	// Validate mode
	if !isValidMode(mode) {
		return nil, fmt.Errorf("invalid behavior mode: %s. Valid modes: falco-only, all, conservative, trivy-only, audit-only, kyverno-only, kube-bench-only", mode)
	}

	// Get namespace configurations
	trivyNamespace := os.Getenv("TRIVY_NAMESPACE")
	if trivyNamespace == "" {
		trivyNamespace = "trivy"
	}

	falcoNamespace := os.Getenv("FALCO_NAMESPACE")
	if falcoNamespace == "" {
		falcoNamespace = "falco"
	}

	watchNamespace := os.Getenv("WATCH_NAMESPACE")
	if watchNamespace == "" {
		watchNamespace = "default"
	}

	// Determine which tools are enabled based on mode
	config := &BehaviorConfig{
		Mode:           mode,
		TrivyNamespace: trivyNamespace,
		FalcoNamespace: falcoNamespace,
		WatchNamespace: watchNamespace,
	}

	// Set tool enablement based on mode
	switch mode {
	case BehaviorModeFalcoOnly:
		config.TrivyEnabled = false
		config.FalcoEnabled = true
		config.AuditEnabled = false
		config.KyvernoEnabled = false
	case BehaviorModeAll:
		config.TrivyEnabled = true
		config.FalcoEnabled = true
		config.AuditEnabled = true
		config.KyvernoEnabled = true
		config.KubeBenchEnabled = true
	case BehaviorModeConservative:
		config.TrivyEnabled = true
		config.FalcoEnabled = false
		config.AuditEnabled = true
		config.KyvernoEnabled = true
		config.KubeBenchEnabled = true
	case BehaviorModeTrivyOnly:
		config.TrivyEnabled = true
		config.FalcoEnabled = false
		config.AuditEnabled = false
		config.KyvernoEnabled = false
	case BehaviorModeAuditOnly:
		config.TrivyEnabled = false
		config.FalcoEnabled = false
		config.AuditEnabled = true
		config.KyvernoEnabled = false
	case BehaviorModeKyvernoOnly:
		config.TrivyEnabled = false
		config.FalcoEnabled = false
		config.AuditEnabled = false
		config.KyvernoEnabled = true
		config.KubeBenchEnabled = false
	case BehaviorModeKubeBenchOnly:
		config.TrivyEnabled = false
		config.FalcoEnabled = false
		config.AuditEnabled = false
		config.KyvernoEnabled = false
		config.KubeBenchEnabled = true
	}

	return config, nil
}

// isValidMode checks if the behavior mode is valid
func isValidMode(mode BehaviorMode) bool {
	validModes := []BehaviorMode{
		BehaviorModeFalcoOnly,
		BehaviorModeAll,
		BehaviorModeConservative,
		BehaviorModeTrivyOnly,
		BehaviorModeAuditOnly,
		BehaviorModeKyvernoOnly,
		BehaviorModeKubeBenchOnly,
	}

	for _, validMode := range validModes {
		if mode == validMode {
			return true
		}
	}

	return false
}

// GetEnabledTools returns a list of enabled tools
func (bc *BehaviorConfig) GetEnabledTools() []string {
	var tools []string

	if bc.TrivyEnabled {
		tools = append(tools, "trivy")
	}
	if bc.FalcoEnabled {
		tools = append(tools, "falco")
	}
	if bc.AuditEnabled {
		tools = append(tools, "audit")
	}
	if bc.KyvernoEnabled {
		tools = append(tools, "kyverno")
	}
	if bc.KubeBenchEnabled {
		tools = append(tools, "kube-bench")
	}

	return tools
}

// IsTrivyEnabled checks if Trivy is enabled
func (bc *BehaviorConfig) IsTrivyEnabled() bool {
	return bc.TrivyEnabled
}

// IsFalcoEnabled checks if Falco is enabled
func (bc *BehaviorConfig) IsFalcoEnabled() bool {
	return bc.FalcoEnabled
}

// IsAuditEnabled checks if Audit is enabled
func (bc *BehaviorConfig) IsAuditEnabled() bool {
	return bc.AuditEnabled
}

// IsKyvernoEnabled checks if Kyverno is enabled
func (bc *BehaviorConfig) IsKyvernoEnabled() bool {
	return bc.KyvernoEnabled
}

// IsKubeBenchEnabled checks if Kube-bench is enabled
func (bc *BehaviorConfig) IsKubeBenchEnabled() bool {
	return bc.KubeBenchEnabled
}

// IsToolEnabled checks if a specific tool is enabled
func (bc *BehaviorConfig) IsToolEnabled(tool string) bool {
	switch strings.ToLower(tool) {
	case "trivy":
		return bc.TrivyEnabled
	case "falco":
		return bc.FalcoEnabled
	case "audit":
		return bc.AuditEnabled
	case "kyverno":
		return bc.KyvernoEnabled
	case "kube-bench":
		return bc.KubeBenchEnabled
	default:
		return false
	}
}

// String returns a string representation of the behavior config
func (bc *BehaviorConfig) String() string {
	return fmt.Sprintf("BehaviorConfig{Mode: %s, Tools: %v}", bc.Mode, bc.GetEnabledTools())
}
