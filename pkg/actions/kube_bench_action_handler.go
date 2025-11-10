package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/models"
	"github.com/kube-zen/zen-watcher/pkg/watcher"

	"k8s.io/client-go/kubernetes"
)

// KubeBenchActionHandler handles kube-bench security findings
type KubeBenchActionHandler struct {
	clientSet *kubernetes.Clientset
	namespace string
}

// NewKubeBenchActionHandler creates a new kube-bench action handler
func NewKubeBenchActionHandler(clientSet *kubernetes.Clientset, namespace string) *KubeBenchActionHandler {
	return &KubeBenchActionHandler{
		clientSet: clientSet,
		namespace: namespace,
	}
}

// HandleKubeBenchFinding handles a kube-bench security finding
func (kbh *KubeBenchActionHandler) HandleKubeBenchFinding(ctx context.Context, finding watcher.KubeBenchFinding) error {
	log.Printf("🔍 Processing kube-bench finding: %s - %s", finding.ControlID, finding.TestID)

	// Create security event from finding
	_ = kbh.createSecurityEvent(finding)

	// Log the finding
	log.Printf("🚨 Kube-bench finding: %s (Level %d) - %s", finding.Severity, finding.Level, finding.Description)

	// Store the finding as a ZenEvent CRD
	if err := kbh.storeFinding(ctx, finding); err != nil {
		log.Printf("❌ Failed to store kube-bench finding: %v", err)
		return err
	}

	// Generate remediation if available
	if finding.Remediation != "" {
		if err := kbh.generateRemediation(ctx, finding); err != nil {
			log.Printf("❌ Failed to generate remediation: %v", err)
			return err
		}
	}

	log.Printf("✅ Successfully processed kube-bench finding: %s", finding.TestID)
	return nil
}

// createSecurityEvent creates a security event from a kube-bench finding
func (kbh *KubeBenchActionHandler) createSecurityEvent(finding watcher.KubeBenchFinding) models.SecurityEvent {
	details := map[string]interface{}{
		"control_id":  finding.ControlID,
		"test_id":     finding.TestID,
		"level":       finding.Level,
		"scored":      finding.Scored,
		"node_name":   finding.NodeName,
		"remediation": finding.Remediation,
		"status":      finding.Status,
	}
	return models.SecurityEvent{
		ID:          fmt.Sprintf("kube-bench-%s-%s-%d", finding.ControlID, finding.TestID, time.Now().Unix()),
		Source:      "kube-bench",
		Type:        "kube-bench-finding",
		Severity:    strings.ToLower(finding.Severity),
		Namespace:   kbh.namespace,
		Resource:    "node/" + finding.NodeName,
		Description: fmt.Sprintf("CIS Benchmark Failure: %s - %s", finding.TestID, finding.Description),
		Details:     details,
		Timestamp:   finding.Timestamp.UTC(),
	}
}

// storeFinding stores a kube-bench finding
func (kbh *KubeBenchActionHandler) storeFinding(ctx context.Context, finding watcher.KubeBenchFinding) error {
	// Finding will be written to CRD by the event writer
	// For now, we'll just log it
	findingJSON, err := json.MarshalIndent(finding, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal finding: %w", err)
	}

	log.Printf("📝 Storing kube-bench finding:\n%s", string(findingJSON))
	return nil
}

// generateRemediation generates a remediation for a kube-bench finding
func (kbh *KubeBenchActionHandler) generateRemediation(ctx context.Context, finding watcher.KubeBenchFinding) error {
	log.Printf("🔧 Generating remediation for kube-bench finding: %s", finding.TestID)

	// Get remediation template
	remediationAction := finding.Remediation
	verificationCommand := ""

	// Create remediation based on the finding
	// Log the remediation suggestion
	payload := map[string]interface{}{
		"action":       remediationAction,
		"verification": verificationCommand,
		"control_id":   finding.ControlID,
		"test_id":      finding.TestID,
	}
	remediationJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal remediation: %w", err)
	}

	log.Printf("🔧 Generated remediation:\n%s", string(remediationJSON))
	return nil
}

// SecurityEvent represents a security event
type SecurityEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Source      string                 `json:"source"`
	Tool        string                 `json:"tool"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// Remediation represents a remediation
type Remediation struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Severity     string                 `json:"severity"`
	Tool         string                 `json:"tool"`
	Type         string                 `json:"type"`
	Source       string                 `json:"source"`
	Action       string                 `json:"action"`
	Verification string                 `json:"verification"`
	Status       string                 `json:"status"`
	DetectedAt   time.Time              `json:"detected_at"`
	Metadata     map[string]interface{} `json:"metadata"`
}
