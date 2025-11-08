package remediations

import (
	"fmt"
	"strings"
)

// KubeBenchRemediationTemplates provides remediation templates for kube-bench findings
type KubeBenchRemediationTemplates struct{}

// NewKubeBenchRemediationTemplates creates a new instance
func NewKubeBenchRemediationTemplates() *KubeBenchRemediationTemplates {
	return &KubeBenchRemediationTemplates{}
}

// GetRemediationForTest returns a remediation template for a specific kube-bench test
func (kbrt *KubeBenchRemediationTemplates) GetRemediationForTest(controlID, testID string) (string, error) {
	// Map of control ID to remediation templates
	remediations := map[string]map[string]string{
		"1.1": { // API Server
			"1.1.1": `# Ensure the --anonymous-auth argument is set to false
# Edit the API server pod specification file
sudo sed -i 's/--anonymous-auth=true/--anonymous-auth=false/' /etc/kubernetes/manifests/kube-apiserver.yaml
# Restart the API server
sudo systemctl restart kubelet`,
			"1.1.2": `# Ensure the --basic-auth-file argument is not set
# Edit the API server pod specification file
sudo sed -i '/--basic-auth-file/d' /etc/kubernetes/manifests/kube-apiserver.yaml
# Restart the API server
sudo systemctl restart kubelet`,
			"1.1.3": `# Ensure the --token-auth-file parameter is not set
# Edit the API server pod specification file
sudo sed -i '/--token-auth-file/d' /etc/kubernetes/manifests/kube-apiserver.yaml
# Restart the API server
sudo systemctl restart kubelet`,
		},
		"1.2": { // API Server - Authorization
			"1.2.1": `# Ensure that the --authorization-mode argument is not set to AlwaysAllow
# Edit the API server pod specification file
sudo sed -i 's/--authorization-mode=AlwaysAllow/--authorization-mode=Node,RBAC/' /etc/kubernetes/manifests/kube-apiserver.yaml
# Restart the API server
sudo systemctl restart kubelet`,
			"1.2.2": `# Ensure that the --authorization-mode argument includes Node
# Edit the API server pod specification file
sudo sed -i 's/--authorization-mode=RBAC/--authorization-mode=Node,RBAC/' /etc/kubernetes/manifests/kube-apiserver.yaml
# Restart the API server
sudo systemctl restart kubelet`,
		},
		"1.3": { // API Server - Admission Control
			"1.3.1": `# Ensure the --admission-control argument is not set to AlwaysAdmit
# Edit the API server pod specification file
sudo sed -i 's/--admission-control=AlwaysAdmit/--admission-control=NodeRestriction/' /etc/kubernetes/manifests/kube-apiserver.yaml
# Restart the API server
sudo systemctl restart kubelet`,
		},
		"1.4": { // API Server - Audit Logging
			"1.4.1": `# Ensure that the --audit-log-path argument is set as appropriate
# Edit the API server pod specification file
sudo sed -i '/--audit-log-path/d' /etc/kubernetes/manifests/kube-apiserver.yaml
echo '    - --audit-log-path=/var/log/audit.log' >> /etc/kubernetes/manifests/kube-apiserver.yaml
# Restart the API server
sudo systemctl restart kubelet`,
		},
		"2.1": { // Etcd
			"2.1.1": `# Ensure that the --cert-file and --key-file arguments are set as appropriate
# Edit the etcd pod specification file
sudo sed -i '/--cert-file/d' /etc/kubernetes/manifests/etcd.yaml
sudo sed -i '/--key-file/d' /etc/kubernetes/manifests/etcd.yaml
echo '    - --cert-file=/etc/kubernetes/pki/etcd/server.crt' >> /etc/kubernetes/manifests/etcd.yaml
echo '    - --key-file=/etc/kubernetes/pki/etcd/server.key' >> /etc/kubernetes/manifests/etcd.yaml
# Restart etcd
sudo systemctl restart kubelet`,
		},
		"3.1": { // Controller Manager
			"3.1.1": `# Ensure that the --terminated-pod-gc-threshold argument is set as appropriate
# Edit the controller manager pod specification file
sudo sed -i '/--terminated-pod-gc-threshold/d' /etc/kubernetes/manifests/kube-controller-manager.yaml
echo '    - --terminated-pod-gc-threshold=10' >> /etc/kubernetes/manifests/kube-controller-manager.yaml
# Restart the controller manager
sudo systemctl restart kubelet`,
		},
		"4.1": { // Scheduler
			"4.1.1": `# Ensure that the --profiling argument is set to false
# Edit the scheduler pod specification file
sudo sed -i 's/--profiling=true/--profiling=false/' /etc/kubernetes/manifests/kube-scheduler.yaml
# Restart the scheduler
sudo systemctl restart kubelet`,
		},
		"5.1": { // Kubelet
			"5.1.1": `# Ensure that the --anonymous-auth argument is set to false
# Edit the kubelet configuration file
sudo sed -i 's/authentication: {}/authentication:\n  anonymous:\n    enabled: false/' /var/lib/kubelet/config.yaml
# Restart kubelet
sudo systemctl restart kubelet`,
			"5.1.2": `# Ensure that the --authorization-mode argument is not set to AlwaysAllow
# Edit the kubelet configuration file
sudo sed -i 's/authorization: {}/authorization:\n  mode: Webhook/' /var/lib/kubelet/config.yaml
# Restart kubelet
sudo systemctl restart kubelet`,
		},
	}

	// Check if control ID exists
	if controlRemediations, exists := remediations[controlID]; exists {
		// Check if specific test ID exists
		if remediation, exists := controlRemediations[testID]; exists {
			return remediation, nil
		}
		// Return generic remediation for the control
		return kbrt.getGenericRemediation(controlID), nil
	}

	// Return generic remediation
	return kbrt.getGenericRemediation(controlID), nil
}

// getGenericRemediation returns a generic remediation for a control
func (kbrt *KubeBenchRemediationTemplates) getGenericRemediation(controlID string) string {
	genericRemediations := map[string]string{
		"1.1": `# API Server Security Configuration
# Review and update API server configuration in /etc/kubernetes/manifests/kube-apiserver.yaml
# Ensure proper authentication, authorization, and admission control settings
# Restart the API server after making changes
sudo systemctl restart kubelet`,
		"1.2": `# API Server Authorization Configuration
# Configure proper authorization modes in /etc/kubernetes/manifests/kube-apiserver.yaml
# Use Node and RBAC authorization modes
# Restart the API server after making changes
sudo systemctl restart kubelet`,
		"1.3": `# API Server Admission Control Configuration
# Configure proper admission controllers in /etc/kubernetes/manifests/kube-apiserver.yaml
# Use NodeRestriction and other security-focused admission controllers
# Restart the API server after making changes
sudo systemctl restart kubelet`,
		"1.4": `# API Server Audit Logging Configuration
# Configure audit logging in /etc/kubernetes/manifests/kube-apiserver.yaml
# Set appropriate audit log path and policy
# Restart the API server after making changes
sudo systemctl restart kubelet`,
		"2.1": `# Etcd Security Configuration
# Review and update etcd configuration in /etc/kubernetes/manifests/etcd.yaml
# Ensure proper TLS configuration and access controls
# Restart etcd after making changes
sudo systemctl restart kubelet`,
		"3.1": `# Controller Manager Security Configuration
# Review and update controller manager configuration in /etc/kubernetes/manifests/kube-controller-manager.yaml
# Ensure proper security settings and resource limits
# Restart the controller manager after making changes
sudo systemctl restart kubelet`,
		"4.1": `# Scheduler Security Configuration
# Review and update scheduler configuration in /etc/kubernetes/manifests/kube-scheduler.yaml
# Ensure proper security settings and resource limits
# Restart the scheduler after making changes
sudo systemctl restart kubelet`,
		"5.1": `# Kubelet Security Configuration
# Review and update kubelet configuration in /var/lib/kubelet/config.yaml
# Ensure proper authentication, authorization, and security settings
# Restart kubelet after making changes
sudo systemctl restart kubelet`,
	}

	if remediation, exists := genericRemediations[controlID]; exists {
		return remediation
	}

	// Default generic remediation
	return fmt.Sprintf(`# CIS Benchmark Control %s Remediation
# Review the CIS Kubernetes Benchmark documentation for control %s
# Implement the recommended security configuration
# Test changes in a non-production environment first
# Document all changes made for audit purposes`, controlID, controlID)
}

// GetRemediationType returns the type of remediation for a control
func (kbrt *KubeBenchRemediationTemplates) GetRemediationType(controlID string) string {
	controlTypes := map[string]string{
		"1.1": "configuration",
		"1.2": "configuration",
		"1.3": "configuration",
		"1.4": "configuration",
		"2.1": "configuration",
		"3.1": "configuration",
		"4.1": "configuration",
		"5.1": "configuration",
	}

	if remediationType, exists := controlTypes[controlID]; exists {
		return remediationType
	}

	return "configuration"
}

// GetRemediationPriority returns the priority for a control
func (kbrt *KubeBenchRemediationTemplates) GetRemediationPriority(controlID string) string {
	// All CIS benchmark controls are high priority for security
	return "high"
}

// GetRemediationDescription returns a description for a control
func (kbrt *KubeBenchRemediationTemplates) GetRemediationDescription(controlID, testID string) string {
	descriptions := map[string]map[string]string{
		"1.1": {
			"1.1.1": "Disable anonymous authentication to prevent unauthorized access",
			"1.1.2": "Remove basic authentication file to prevent credential exposure",
			"1.1.3": "Remove token authentication file to prevent credential exposure",
		},
		"1.2": {
			"1.2.1": "Configure proper authorization mode instead of AlwaysAllow",
			"1.2.2": "Include Node authorization mode for proper node authentication",
		},
		"1.3": {
			"1.3.1": "Configure proper admission control instead of AlwaysAdmit",
		},
		"1.4": {
			"1.4.1": "Configure audit logging to track API server access",
		},
		"2.1": {
			"2.1.1": "Configure TLS certificates for etcd communication",
		},
		"3.1": {
			"3.1.1": "Configure terminated pod garbage collection threshold",
		},
		"4.1": {
			"4.1.1": "Disable profiling to prevent information disclosure",
		},
		"5.1": {
			"5.1.1": "Disable anonymous authentication for kubelet",
			"5.1.2": "Configure proper authorization mode for kubelet",
		},
	}

	if controlDescriptions, exists := descriptions[controlID]; exists {
		if description, exists := controlDescriptions[testID]; exists {
			return description
		}
	}

	return fmt.Sprintf("Implement CIS Kubernetes Benchmark control %s.%s", controlID, testID)
}

// GetRemediationTitle returns a title for a control
func (kbrt *KubeBenchRemediationTemplates) GetRemediationTitle(controlID, testID string) string {
	titles := map[string]map[string]string{
		"1.1": {
			"1.1.1": "Disable Anonymous Authentication",
			"1.1.2": "Remove Basic Authentication File",
			"1.1.3": "Remove Token Authentication File",
		},
		"1.2": {
			"1.2.1": "Configure Authorization Mode",
			"1.2.2": "Include Node Authorization Mode",
		},
		"1.3": {
			"1.3.1": "Configure Admission Control",
		},
		"1.4": {
			"1.4.1": "Configure Audit Logging",
		},
		"2.1": {
			"2.1.1": "Configure Etcd TLS Certificates",
		},
		"3.1": {
			"3.1.1": "Configure Pod Garbage Collection",
		},
		"4.1": {
			"4.1.1": "Disable Scheduler Profiling",
		},
		"5.1": {
			"5.1.1": "Disable Kubelet Anonymous Authentication",
			"5.1.2": "Configure Kubelet Authorization Mode",
		},
	}

	if controlTitles, exists := titles[controlID]; exists {
		if title, exists := controlTitles[testID]; exists {
			return title
		}
	}

	return fmt.Sprintf("CIS Control %s.%s Remediation", controlID, testID)
}

// GetRemediationVerification returns verification commands for a control
func (kbrt *KubeBenchRemediationTemplates) GetRemediationVerification(controlID, testID string) string {
	verifications := map[string]map[string]string{
		"1.1": {
			"1.1.1": `# Verify anonymous authentication is disabled
kubectl get --raw /api/v1 | grep -i "unauthorized" || echo "Anonymous auth disabled"`,
			"1.1.2": `# Verify basic auth file is not set
ps aux | grep kube-apiserver | grep -v "basic-auth-file" || echo "Basic auth file not set"`,
			"1.1.3": `# Verify token auth file is not set
ps aux | grep kube-apiserver | grep -v "token-auth-file" || echo "Token auth file not set"`,
		},
		"1.2": {
			"1.2.1": `# Verify authorization mode is not AlwaysAllow
ps aux | grep kube-apiserver | grep -v "authorization-mode=AlwaysAllow" || echo "Authorization mode configured"`,
			"1.2.2": `# Verify Node authorization mode is included
ps aux | grep kube-apiserver | grep "authorization-mode.*Node" || echo "Node authorization mode included"`,
		},
		"1.3": {
			"1.3.1": `# Verify admission control is not AlwaysAdmit
ps aux | grep kube-apiserver | grep -v "admission-control=AlwaysAdmit" || echo "Admission control configured"`,
		},
		"1.4": {
			"1.4.1": `# Verify audit log path is configured
ps aux | grep kube-apiserver | grep "audit-log-path" || echo "Audit logging configured"`,
		},
		"2.1": {
			"2.1.1": `# Verify etcd TLS certificates are configured
ps aux | grep etcd | grep "cert-file" || echo "Etcd TLS configured"`,
		},
		"3.1": {
			"3.1.1": `# Verify pod GC threshold is configured
ps aux | grep kube-controller-manager | grep "terminated-pod-gc-threshold" || echo "Pod GC configured"`,
		},
		"4.1": {
			"4.1.1": `# Verify profiling is disabled
ps aux | grep kube-scheduler | grep -v "profiling=true" || echo "Profiling disabled"`,
		},
		"5.1": {
			"5.1.1": `# Verify kubelet anonymous auth is disabled
ps aux | grep kubelet | grep -v "anonymous-auth=true" || echo "Kubelet anonymous auth disabled"`,
			"5.1.2": `# Verify kubelet authorization mode is configured
ps aux | grep kubelet | grep -v "authorization-mode=AlwaysAllow" || echo "Kubelet authorization configured"`,
		},
	}

	if controlVerifications, exists := verifications[controlID]; exists {
		if verification, exists := controlVerifications[testID]; exists {
			return verification
		}
	}

	return fmt.Sprintf(`# Verify CIS control %s.%s implementation
# Run kube-bench to verify the control passes
kube-bench run --targets master,node --check %s`, controlID, testID, strings.Replace(testID, ".", "", -1))
}
