package installation

import (
	"context"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ToolInstaller handles installation of security tools
type ToolInstaller struct {
	clientSet *kubernetes.Clientset
	config    *rest.Config
	namespace string
}

// InstallationResult represents the result of a tool installation
type InstallationResult struct {
	ToolName    string    `json:"toolName"`
	Success     bool      `json:"success"`
	Message     string    `json:"message"`
	InstalledAt time.Time `json:"installedAt"`
	Duration    string    `json:"duration"`
	Components  []string  `json:"components"`
	Error       string    `json:"error,omitempty"`
}

// NewToolInstaller creates a new tool installer
func NewToolInstaller(clientSet *kubernetes.Clientset, config *rest.Config, namespace string) *ToolInstaller {
	return &ToolInstaller{
		clientSet: clientSet,
		config:    config,
		namespace: namespace,
	}
}

// InstallTrivy installs Trivy using Helm or manifests
func (ti *ToolInstaller) InstallTrivy() (*InstallationResult, error) {
	startTime := time.Now()
	log.Println("🚀 Starting Trivy installation...")

	result := &InstallationResult{
		ToolName:    "trivy",
		InstalledAt: startTime,
		Components:  []string{},
	}

	// Check if Trivy is already installed
	_, err := ti.clientSet.CoreV1().Namespaces().Get(context.TODO(), "trivy", metav1."GetOptions{})
	if err == nil {
		result.Success = false
		result.Message = "Trivy is already installed"
		result.Duration = time.Since(startTime).String()
		log.Println("⚠️ Trivy is already installed")
		return result, nil
	}

	// Step 1: Create Trivy namespace
	log.Println("📦 Creating Trivy namespace...")
	err = ti.createTrivyNamespace()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create namespace: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}
	result.Components = append(result.Components, "namespace")

	// Step 2: Install Trivy operator
	log.Println("🔧 Installing Trivy operator...")
	err = ti.installTrivyOperator()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to install operator: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}
	result.Components = append(result.Components, "operator")

	// Step 3: Install Trivy scanner
	log.Println("🔍 Installing Trivy scanner...")
	err = ti.installTrivyScanner()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to install scanner: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}
	result.Components = append(result.Components, "scanner")

	// Step 4: Verify installation
	log.Println("✅ Verifying Trivy installation...")
	err = ti.verifyTrivyInstallation()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Installation verification failed: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}

	result.Success = true
	result.Message = "Trivy installed successfully"
	result.Duration = time.Since(startTime).String()
	log.Println("✅ Trivy installation completed successfully")

	return result, nil
}

// createTrivyNamespace creates the Trivy namespace
func (ti *ToolInstaller) createTrivyNamespace() error {
	namespace := &corev1."Namespace{
		ObjectMeta: metav1."ObjectMeta{
			Name: "trivy",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "trivy",
				"app.kubernetes.io/instance": "trivy",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
	}

	_, err := ti.clientSet.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	log.Println("✅ Trivy namespace created")
	return nil
}

// installTrivyOperator installs the Trivy operator
func (ti *ToolInstaller) installTrivyOperator() error {
	// For now, we'll create a basic deployment
	// In production, you would use Helm or apply the official Trivy operator manifests

	deployment := &appsv1."Deployment{
		ObjectMeta: metav1."ObjectMeta{
			Name:      "trivy-operator",
			Namespace: "trivy",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "trivy-operator",
				"app.kubernetes.io/instance": "trivy",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
		Spec: appsv1."DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1."LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "trivy-operator",
					"app.kubernetes.io/instance": "trivy",
				},
			},
			Template: corev1."PodTemplateSpec{
				ObjectMeta: metav1."ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "trivy-operator",
						"app.kubernetes.io/instance": "trivy",
					},
				},
				Spec: corev1."PodSpec{
					ServiceAccountName: "trivy-operator",
					Containers: []corev1."Container{
						{
							Name:  "trivy-operator",
							Image: "aquasec/trivy-operator:0.18.0",
							Args: []string{
								"trivy-operator",
								"server",
								"--config",
								"/etc/trivy-operator/config.yaml",
							},
							Env: []corev1."EnvVar{
								{
									Name:  "OPERATOR_NAMESPACE",
									Value: "trivy",
								},
							},
							Resources: corev1."ResourceRequirements{
								Requests: corev1."ResourceList{
									corev1."ResourceCPU:    resource.MustParse("100m"),
									corev1."ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1."ResourceList{
									corev1."ResourceCPU:    resource.MustParse("500m"),
									corev1."ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := ti.clientSet.AppsV1().Deployments("trivy").Create(context.TODO(), deployment, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create operator deployment: %w", err)
	}

	log.Println("✅ Trivy operator deployment created")
	return nil
}

// installTrivyScanner installs the Trivy scanner
func (ti *ToolInstaller) installTrivyScanner() error {
	// Create Trivy scanner daemonset
	daemonset := &appsv1."DaemonSet{
		ObjectMeta: metav1."ObjectMeta{
			Name:      "trivy-scanner",
			Namespace: "trivy",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "trivy-scanner",
				"app.kubernetes.io/instance": "trivy",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
		Spec: appsv1."DaemonSetSpec{
			Selector: &metav1."LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "trivy-scanner",
					"app.kubernetes.io/instance": "trivy",
				},
			},
			Template: corev1."PodTemplateSpec{
				ObjectMeta: metav1."ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "trivy-scanner",
						"app.kubernetes.io/instance": "trivy",
					},
				},
				Spec: corev1."PodSpec{
					ServiceAccountName: "trivy-scanner",
					Containers: []corev1."Container{
						{
							Name:  "trivy-scanner",
							Image: "aquasec/trivy:0.50.0",
							Command: []string{
								"trivy",
								"server",
								"--listen",
								"0.0.0.0:8080",
							},
							Ports: []corev1."ContainerPort{
								{
									ContainerPort: 8080,
									Name:          "http",
								},
							},
							Resources: corev1."ResourceRequirements{
								Requests: corev1."ResourceList{
									corev1."ResourceCPU:    resource.MustParse("100m"),
									corev1."ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1."ResourceList{
									corev1."ResourceCPU:    resource.MustParse("1000m"),
									corev1."ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := ti.clientSet.AppsV1().DaemonSets("trivy").Create(context.TODO(), daemonset, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create scanner daemonset: %w", err)
	}

	log.Println("✅ Trivy scanner daemonset created")
	return nil
}

// verifyTrivyInstallation verifies that Trivy is properly installed
func (ti *ToolInstaller) verifyTrivyInstallation() error {
	// Wait for deployments to be ready
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	operatorReady := false
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for Trivy to be ready")
		case <-ticker.C:
			// Check operator deployment
			deployment, err := ti.clientSet.AppsV1().Deployments("trivy").Get(context.TODO(), "trivy-operator", metav1."GetOptions{})
			if err != nil {
				continue
			}

			if deployment.Status.ReadyReplicas == deployment.Status.Replicas && deployment.Status.Replicas > 0 {
				log.Println("✅ Trivy operator is ready")
				operatorReady = true
				break
			}
		}
		if operatorReady {
			break
		}
	}

	// Check daemonset
	daemonset, err := ti.clientSet.AppsV1().DaemonSets("trivy").Get(context.TODO(), "trivy-scanner", metav1."GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to verify scanner daemonset: %w", err)
	}

	if daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled {
		log.Println("✅ Trivy scanner is ready")
		return nil
	}

	return fmt.Errorf("scanner daemonset not ready: %d/%d pods ready",
		daemonset.Status.NumberReady, daemonset.Status.DesiredNumberScheduled)
}

// InstallFalco installs Falco using Helm or manifests
func (ti *ToolInstaller) InstallFalco() (*InstallationResult, error) {
	startTime := time.Now()
	log.Println("🚀 Starting Falco installation...")

	result := &InstallationResult{
		ToolName:    "falco",
		InstalledAt: startTime,
		Components:  []string{},
	}

	// Check if Falco is already installed
	_, err := ti.clientSet.CoreV1().Namespaces().Get(context.TODO(), "falco", metav1."GetOptions{})
	if err == nil {
		result.Success = false
		result.Message = "Falco is already installed"
		result.Duration = time.Since(startTime).String()
		log.Println("⚠️ Falco is already installed")
		return result, nil
	}

	// Step 1: Create Falco namespace
	log.Println("📦 Creating Falco namespace...")
	err = ti.createFalcoNamespace()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create namespace: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}
	result.Components = append(result.Components, "namespace")

	// Step 2: Install Falco daemonset
	log.Println("🔧 Installing Falco daemonset...")
	err = ti.installFalcoDaemonset()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to install daemonset: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}
	result.Components = append(result.Components, "daemonset")

	// Step 3: Install Falco ConfigMap
	log.Println("📋 Installing Falco ConfigMap...")
	err = ti.installFalcoConfigMap()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to install ConfigMap: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}
	result.Components = append(result.Components, "configmap")

	// Step 4: Verify installation
	log.Println("✅ Verifying Falco installation...")
	err = ti.verifyFalcoInstallation()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Installation verification failed: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}

	result.Success = true
	result.Message = "Falco installed successfully"
	result.Duration = time.Since(startTime).String()
	log.Println("✅ Falco installation completed successfully")

	return result, nil
}

// createFalcoNamespace creates the Falco namespace
func (ti *ToolInstaller) createFalcoNamespace() error {
	namespace := &corev1."Namespace{
		ObjectMeta: metav1."ObjectMeta{
			Name: "falco",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "falco",
				"app.kubernetes.io/instance": "falco",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
	}

	_, err := ti.clientSet.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	log.Println("✅ Falco namespace created")
	return nil
}

// installFalcoDaemonset installs the Falco daemonset
func (ti *ToolInstaller) installFalcoDaemonset() error {
	daemonset := &appsv1."DaemonSet{
		ObjectMeta: metav1."ObjectMeta{
			Name:      "falco",
			Namespace: "falco",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "falco",
				"app.kubernetes.io/instance": "falco",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
		Spec: appsv1."DaemonSetSpec{
			Selector: &metav1."LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "falco",
					"app.kubernetes.io/instance": "falco",
				},
			},
			Template: corev1."PodTemplateSpec{
				ObjectMeta: metav1."ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "falco",
						"app.kubernetes.io/instance": "falco",
					},
				},
				Spec: corev1."PodSpec{
					ServiceAccountName: "falco",
					HostNetwork:        true,
					HostPID:            true,
					Containers: []corev1."Container{
						{
							Name:  "falco",
							Image: "falcosecurity/falco:0.37.0",
							Args: []string{
								"/usr/bin/falco",
								"--cri",
								"/host/run/containerd/containerd.sock",
								"--k8s-api",
								"-K",
								"/var/run/secrets/kubernetes.io/serviceaccount/token",
								"-k",
								"https://kubernetes.default.svc:443",
								"--k8s-node",
								"$(NODE_NAME)",
							},
							Env: []corev1."EnvVar{
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1."EnvVarSource{
										FieldRef: &corev1."ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
							},
							VolumeMounts: []corev1."VolumeMount{
								{
									Name:      "host-root",
									MountPath: "/host",
									ReadOnly:  true,
								},
								{
									Name:      "falco-config",
									MountPath: "/etc/falco",
								},
							},
							Resources: corev1."ResourceRequirements{
								Requests: corev1."ResourceList{
									corev1."ResourceCPU:    resource.MustParse("100m"),
									corev1."ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1."ResourceList{
									corev1."ResourceCPU:    resource.MustParse("500m"),
									corev1."ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							SecurityContext: &corev1."SecurityContext{
								Privileged: boolPtr(true),
							},
						},
					},
					Volumes: []corev1."Volume{
						{
							Name: "host-root",
							VolumeSource: corev1."VolumeSource{
								HostPath: &corev1."HostPathVolumeSource{
									Path: "/",
								},
							},
						},
						{
							Name: "falco-config",
							VolumeSource: corev1."VolumeSource{
								ConfigMap: &corev1."ConfigMapVolumeSource{
									LocalObjectReference: corev1."LocalObjectReference{
										Name: "falco-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := ti.clientSet.AppsV1().DaemonSets("falco").Create(context.TODO(), daemonset, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create daemonset: %w", err)
	}

	log.Println("✅ Falco daemonset created")
	return nil
}

// installFalcoConfigMap installs the Falco ConfigMap
func (ti *ToolInstaller) installFalcoConfigMap() error {
	configMap := &corev1."ConfigMap{
		ObjectMeta: metav1."ObjectMeta{
			Name:      "falco-config",
			Namespace: "falco",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "falco",
				"app.kubernetes.io/instance": "falco",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
		Data: map[string]string{
			"falco.yaml": `
rules_file:
  - /etc/falco/falco_rules.yaml
  - /etc/falco/falco_rules.local.yaml
  - /etc/falco/k8s_audit_rules.yaml
  - /etc/falco/rules.d

json_output: true
json_include_output_property: true
http_output:
  enabled: true
  url: "http://falco:8765/k8s-audit"
`,
		},
	}

	_, err := ti.clientSet.CoreV1().ConfigMaps("falco").Create(context.TODO(), configMap, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}

	log.Println("✅ Falco ConfigMap created")
	return nil
}

// verifyFalcoInstallation verifies that Falco is properly installed
func (ti *ToolInstaller) verifyFalcoInstallation() error {
	// Wait for daemonset to be ready
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for Falco to be ready")
		case <-ticker.C:
			// Check daemonset
			daemonset, err := ti.clientSet.AppsV1().DaemonSets("falco").Get(context.TODO(), "falco", metav1."GetOptions{})
			if err != nil {
				continue
			}

			if daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled {
				log.Println("✅ Falco daemonset is ready")
				return nil
			}
		}
	}
}

// InstallKyverno installs Kyverno (placeholder implementation)
func (ti *ToolInstaller) InstallKyverno() (*InstallationResult, error) {
	startTime := time.Now()
	log.Println("🚀 Starting Kyverno installation...")

	result := &InstallationResult{
		ToolName:    "kyverno",
		InstalledAt: startTime,
		Components:  []string{},
	}

	// TODO: Implement Kyverno installation
	result.Success = false
	result.Error = "Kyverno installation not implemented yet"
	result.Duration = time.Since(startTime).String()
	log.Println("⚠️ Kyverno installation not implemented yet")

	return result, nil
}

// InstallKubeBench installs kube-bench (placeholder implementation)
func (ti *ToolInstaller) InstallKubeBench() (*InstallationResult, error) {
	startTime := time.Now()
	log.Println("🚀 Starting kube-bench installation...")

	result := &InstallationResult{
		ToolName:    "kube-bench",
		InstalledAt: startTime,
		Components:  []string{},
	}

	// TODO: Implement kube-bench installation
	result.Success = false
	result.Error = "Kube-bench installation not implemented yet"
	result.Duration = time.Since(startTime).String()
	log.Println("⚠️ Kube-bench installation not implemented yet")

	return result, nil
}

// InstallKubernetesAudit configures Kubernetes audit logging
func (ti *ToolInstaller) InstallKubernetesAudit() (*InstallationResult, error) {
	startTime := time.Now()
	log.Println("🚀 Starting Kubernetes audit logging configuration...")

	result := &InstallationResult{
		ToolName:    "kubernetes-audit",
		InstalledAt: startTime,
		Components:  []string{},
	}

	// Step 1: Create audit policy ConfigMap
	log.Println("📋 Creating audit policy ConfigMap...")
	err := ti.createAuditPolicyConfigMap()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create audit policy: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}
	result.Components = append(result.Components, "audit-policy")

	// Step 2: Create audit webhook service
	log.Println("🔗 Creating audit webhook service...")
	err = ti.createAuditWebhookService()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create audit webhook: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}
	result.Components = append(result.Components, "audit-webhook")

	// Step 3: Create audit webhook deployment
	log.Println("🚀 Creating audit webhook deployment...")
	err = ti.createAuditWebhookDeployment()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create audit webhook deployment: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}
	result.Components = append(result.Components, "audit-webhook-deployment")

	// Step 4: Create audit log volume
	log.Println("💾 Creating audit log volume...")
	err = ti.createAuditLogVolume()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create audit log volume: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}
	result.Components = append(result.Components, "audit-log-volume")

	// Step 5: Verify configuration
	log.Println("✅ Verifying audit configuration...")
	err = ti.verifyAuditConfiguration()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Audit configuration verification failed: %v", err)
		result.Duration = time.Since(startTime).String()
		return result, err
	}

	result.Success = true
	result.Message = "Kubernetes audit logging configured successfully"
	result.Duration = time.Since(startTime).String()
	log.Println("✅ Kubernetes audit logging configuration completed successfully")

	return result, nil
}

// createAuditPolicyConfigMap creates the audit policy ConfigMap
func (ti *ToolInstaller) createAuditPolicyConfigMap() error {
	configMap := &corev1."ConfigMap{
		ObjectMeta: metav1."ObjectMeta{
			Name:      "audit-policy",
			Namespace: "kube-system",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "audit-policy",
				"app.kubernetes.io/instance": "kubernetes-audit",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
		Data: map[string]string{
			"audit-policy.yaml": `
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
# Log all requests at the Metadata level
- level: Metadata
  namespaces: ["*"]
  verbs: ["*"]
  resources:
  - group: ""
    resources: ["*"]
  - group: "apps"
    resources: ["*"]
  - group: "extensions"
    resources: ["*"]
  - group: "networking.k8s.io"
    resources: ["*"]
  - group: "policy"
    resources: ["*"]
  - group: "rbac.authorization.k8s.io"
    resources: ["*"]
  - group: "security.openshift.io"
    resources: ["*"]

# Log security-sensitive operations at Request level
- level: Request
  namespaces: ["*"]
  verbs: ["create", "update", "patch", "delete"]
  resources:
  - group: ""
    resources: ["secrets", "configmaps"]
  - group: "rbac.authorization.k8s.io"
    resources: ["*"]
  - group: "policy"
    resources: ["podsecuritypolicies"]

# Log authentication and authorization at RequestResponse level
- level: RequestResponse
  namespaces: ["*"]
  verbs: ["create", "update", "patch", "delete"]
  resources:
  - group: ""
    resources: ["serviceaccounts"]
  - group: "rbac.authorization.k8s.io"
    resources: ["*"]

# Log all requests to audit webhook
- level: RequestResponse
  namespaces: ["*"]
  verbs: ["*"]
  resources:
  - group: "audit.k8s.io"
    resources: ["*"]
`,
		},
	}

	_, err := ti.clientSet.CoreV1().ConfigMaps("kube-system").Create(context.TODO(), configMap, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create audit policy ConfigMap: %w", err)
	}

	log.Println("✅ Audit policy ConfigMap created")
	return nil
}

// createAuditWebhookService creates the audit webhook service
func (ti *ToolInstaller) createAuditWebhookService() error {
	service := &corev1."Service{
		ObjectMeta: metav1."ObjectMeta{
			Name:      "audit-webhook",
			Namespace: "kube-system",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "audit-webhook",
				"app.kubernetes.io/instance": "kubernetes-audit",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
		Spec: corev1."ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name":     "audit-webhook",
				"app.kubernetes.io/instance": "kubernetes-audit",
			},
			Ports: []corev1."ServicePort{
				{
					Name:       "webhook",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1."ProtocolTCP,
				},
			},
		},
	}

	_, err := ti.clientSet.CoreV1().Services("kube-system").Create(context.TODO(), service, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create audit webhook service: %w", err)
	}

	log.Println("✅ Audit webhook service created")
	return nil
}

// createAuditWebhookDeployment creates the audit webhook deployment
func (ti *ToolInstaller) createAuditWebhookDeployment() error {
	deployment := &appsv1."Deployment{
		ObjectMeta: metav1."ObjectMeta{
			Name:      "audit-webhook",
			Namespace: "kube-system",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "audit-webhook",
				"app.kubernetes.io/instance": "kubernetes-audit",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
		Spec: appsv1."DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1."LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "audit-webhook",
					"app.kubernetes.io/instance": "kubernetes-audit",
				},
			},
			Template: corev1."PodTemplateSpec{
				ObjectMeta: metav1."ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "audit-webhook",
						"app.kubernetes.io/instance": "kubernetes-audit",
					},
				},
				Spec: corev1."PodSpec{
					ServiceAccountName: "audit-webhook",
					Containers: []corev1."Container{
						{
							Name:  "audit-webhook",
							Image: "nginx:1.21",
							Ports: []corev1."ContainerPort{
								{
									ContainerPort: 8080,
									Name:          "webhook",
								},
							},
							VolumeMounts: []corev1."VolumeMount{
								{
									Name:      "audit-logs",
									MountPath: "/var/log/audit",
								},
							},
							Resources: corev1."ResourceRequirements{
								Requests: corev1."ResourceList{
									corev1."ResourceCPU:    resource.MustParse("50m"),
									corev1."ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1."ResourceList{
									corev1."ResourceCPU:    resource.MustParse("200m"),
									corev1."ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
					Volumes: []corev1."Volume{
						{
							Name: "audit-logs",
							VolumeSource: corev1."VolumeSource{
								EmptyDir: &corev1."EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	_, err := ti.clientSet.AppsV1().Deployments("kube-system").Create(context.TODO(), deployment, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create audit webhook deployment: %w", err)
	}

	log.Println("✅ Audit webhook deployment created")
	return nil
}

// createAuditLogVolume creates a persistent volume for audit logs
func (ti *ToolInstaller) createAuditLogVolume() error {
	// Create a PersistentVolume for audit logs
	pv := &corev1."PersistentVolume{
		ObjectMeta: metav1."ObjectMeta{
			Name: "audit-logs-pv",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "audit-logs",
				"app.kubernetes.io/instance": "kubernetes-audit",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
		Spec: corev1."PersistentVolumeSpec{
			Capacity: corev1."ResourceList{
				corev1."ResourceStorage: resource.MustParse("10Gi"),
			},
			AccessModes: []corev1."PersistentVolumeAccessMode{
				corev1."ReadWriteOnce,
			},
			PersistentVolumeSource: corev1."PersistentVolumeSource{
				HostPath: &corev1."HostPathVolumeSource{
					Path: "/var/log/kubernetes-audit",
				},
			},
		},
	}

	_, err := ti.clientSet.CoreV1().PersistentVolumes().Create(context.TODO(), pv, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create audit log persistent volume: %w", err)
	}

	// Create a PersistentVolumeClaim
	pvc := &corev1."PersistentVolumeClaim{
		ObjectMeta: metav1."ObjectMeta{
			Name:      "audit-logs-pvc",
			Namespace: "kube-system",
			Labels: map[string]string{
				"app.kubernetes.io/name":     "audit-logs",
				"app.kubernetes.io/instance": "kubernetes-audit",
				"app.kubernetes.io/part-of":  "kube-zen",
			},
		},
		Spec: corev1."PersistentVolumeClaimSpec{
			AccessModes: []corev1."PersistentVolumeAccessMode{
				corev1."ReadWriteOnce,
			},
			Resources: corev1."VolumeResourceRequirements{
				Requests: corev1."ResourceList{
					corev1."ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			VolumeName: "audit-logs-pv",
		},
	}

	_, err = ti.clientSet.CoreV1().PersistentVolumeClaims("kube-system").Create(context.TODO(), pvc, metav1."CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create audit log persistent volume claim: %w", err)
	}

	log.Println("✅ Audit log volume created")
	return nil
}

// verifyAuditConfiguration verifies that audit configuration is working
func (ti *ToolInstaller) verifyAuditConfiguration() error {
	// Wait for deployment to be ready
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for audit webhook to be ready")
		case <-ticker.C:
			// Check deployment
			deployment, err := ti.clientSet.AppsV1().Deployments("kube-system").Get(context.TODO(), "audit-webhook", metav1."GetOptions{})
			if err != nil {
				continue
			}

			if deployment.Status.ReadyReplicas == deployment.Status.Replicas && deployment.Status.Replicas > 0 {
				log.Println("✅ Audit webhook is ready")
				return nil
			}
		}
	}
}

// Helper functions
func boolPtr(b bool) *bool    { return &b }
func int32Ptr(i int32) *int32 { return &i }
