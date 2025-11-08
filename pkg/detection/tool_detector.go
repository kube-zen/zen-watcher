package detection

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// ToolDetector detects installed security tools in the cluster
type ToolDetector struct {
	clientSet     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	namespace     string
}

// ToolStatus represents the status of a security tool
type ToolStatus struct {
	Name         string            `json:"name"`
	Installed    bool              `json:"installed"`
	Version      string            `json:"version,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	HealthStatus string            `json:"healthStatus"`
	Components   []ComponentStatus `json:"components"`
	LastChecked  time.Time         `json:"lastChecked"`
	Error        string            `json:"error,omitempty"`
}

// ComponentStatus represents the status of a tool component
type ComponentStatus struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // deployment, daemonset, service, crd
	Status    string `json:"status"`
	Ready     bool   `json:"ready"`
	Replicas  int32  `json:"replicas,omitempty"`
	Available int32  `json:"available,omitempty"`
}

// NewToolDetector creates a new tool detector
func NewToolDetector(clientSet *kubernetes.Clientset, dynamicClient dynamic.Interface, namespace string) *ToolDetector {
	return &ToolDetector{
		clientSet:     clientSet,
		dynamicClient: dynamicClient,
		namespace:     namespace,
	}
}

// DetectTrivy detects if Trivy is installed and running
func (td *ToolDetector) DetectTrivy() (*ToolStatus, error) {
	log.Println("🔍 Detecting Trivy installation...")

	status := &ToolStatus{
		Name:        "trivy",
		LastChecked: time.Now(),
		Components:  []ComponentStatus{},
	}

	// Check for Trivy namespace
	namespaces := []string{"trivy", "trivy-system", "aqua"}
	var trivyNamespace string

	for _, ns := range namespaces {
		_, err := td.clientSet.CoreV1().Namespaces().Get(context.TODO(), ns, metav1."GetOptions{})
		if err == nil {
			trivyNamespace = ns
			break
		}
	}

	if trivyNamespace == "" {
		status.Installed = false
		status.HealthStatus = "not-installed"
		status.Error = "Trivy namespace not found"
		log.Println("❌ Trivy namespace not found")
		return status, nil
	}

	status.Namespace = trivyNamespace
	log.Printf("✅ Found Trivy namespace: %s", trivyNamespace)

	// Check for Trivy deployments
	deployments, err := td.clientSet.AppsV1().Deployments(trivyNamespace).List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		status.Error = fmt.Sprintf("Failed to list deployments: %v", err)
		return status, err
	}

	trivyDeployments := []appsv1."Deployment{}
	for _, deployment := range deployments.Items {
		if strings.Contains(strings.ToLower(deployment.Name), "trivy") {
			trivyDeployments = append(trivyDeployments, deployment)
		}
	}

	// Check for Trivy daemonsets
	daemonsets, err := td.clientSet.AppsV1().DaemonSets(trivyNamespace).List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		status.Error = fmt.Sprintf("Failed to list daemonsets: %v", err)
		return status, err
	}

	trivyDaemonsets := []appsv1."DaemonSet{}
	for _, daemonset := range daemonsets.Items {
		if strings.Contains(strings.ToLower(daemonset.Name), "trivy") {
			trivyDaemonsets = append(trivyDaemonsets, daemonset)
		}
	}

	// Check for Trivy services
	services, err := td.clientSet.CoreV1().Services(trivyNamespace).List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		status.Error = fmt.Sprintf("Failed to list services: %v", err)
		return status, err
	}

	trivyServices := []corev1."Service{}
	for _, service := range services.Items {
		if strings.Contains(strings.ToLower(service.Name), "trivy") {
			trivyServices = append(trivyServices, service)
		}
	}

	// Check for Trivy CRDs
	trivyCRDs, err := td.detectTrivyCRDs()
	if err != nil {
		log.Printf("⚠️ Failed to detect Trivy CRDs: %v", err)
	}

	// Analyze components
	allComponentsReady := true
	totalReplicas := int32(0)
	totalAvailable := int32(0)

	// Process deployments
	for _, deployment := range trivyDeployments {
		component := ComponentStatus{
			Name:      deployment.Name,
			Type:      "deployment",
			Status:    "running",
			Ready:     deployment.Status.ReadyReplicas == deployment.Status.Replicas,
			Replicas:  deployment.Status.Replicas,
			Available: deployment.Status.ReadyReplicas,
		}

		if !component.Ready {
			allComponentsReady = false
		}

		totalReplicas += component.Replicas
		totalAvailable += component.Available
		status.Components = append(status.Components, component)
	}

	// Process daemonsets
	for _, daemonset := range trivyDaemonsets {
		component := ComponentStatus{
			Name:      daemonset.Name,
			Type:      "daemonset",
			Status:    "running",
			Ready:     daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled,
			Replicas:  daemonset.Status.DesiredNumberScheduled,
			Available: daemonset.Status.NumberReady,
		}

		if !component.Ready {
			allComponentsReady = false
		}

		totalReplicas += component.Replicas
		totalAvailable += component.Available
		status.Components = append(status.Components, component)
	}

	// Process services
	for _, service := range trivyServices {
		component := ComponentStatus{
			Name:   service.Name,
			Type:   "service",
			Status: "running",
			Ready:  true, // Services don't have ready status
		}
		status.Components = append(status.Components, component)
	}

	// Process CRDs
	for _, crd := range trivyCRDs {
		component := ComponentStatus{
			Name:   crd,
			Type:   "crd",
			Status: "installed",
			Ready:  true,
		}
		status.Components = append(status.Components, component)
	}

	// Determine overall status
	if len(status.Components) == 0 {
		status.Installed = false
		status.HealthStatus = "not-installed"
		status.Error = "No Trivy components found"
		log.Println("❌ No Trivy components found")
	} else if allComponentsReady {
		status.Installed = true
		status.HealthStatus = "healthy"
		status.Version = td.extractTrivyVersion(trivyDeployments, trivyDaemonsets)
		log.Printf("✅ Trivy is installed and healthy (version: %s)", status.Version)
	} else {
		status.Installed = true
		status.HealthStatus = "unhealthy"
		status.Error = fmt.Sprintf("Some components not ready (%d/%d replicas available)", totalAvailable, totalReplicas)
		log.Printf("⚠️ Trivy is installed but unhealthy: %s", status.Error)
	}

	return status, nil
}

// detectTrivyCRDs detects Trivy-specific CRDs
func (td *ToolDetector) detectTrivyCRDs() ([]string, error) {
	var trivyCRDs []string

	// Check if dynamic client is available
	if td.dynamicClient == nil {
		log.Println("⚠️ Dynamic client not available, skipping CRD detection")
		return trivyCRDs, nil
	}

	// Common Trivy CRDs
	possibleCRDs := []string{
		"vulnerabilityreports.aquasecurity.github.io",
		"configauditreports.aquasecurity.github.io",
		"exposedsecrets.aquasecurity.github.io",
		"clusterconfigauditreports.aquasecurity.github.io",
		"clustercompliancereports.aquasecurity.github.io",
		"clustercompliancedetailreports.aquasecurity.github.io",
	}

	for _, crdName := range possibleCRDs {
		gvr := schema.GroupVersionResource{
			Group:    "apiextensions.k8s.io",
			Version:  "v1",
			Resource: "customresourcedefinitions",
		}

		_, err := td.dynamicClient.Resource(gvr).Get(context.TODO(), crdName, metav1."GetOptions{})
		if err == nil {
			trivyCRDs = append(trivyCRDs, crdName)
		}
	}

	return trivyCRDs, nil
}

// extractTrivyVersion extracts version information from Trivy deployments/daemonsets
func (td *ToolDetector) extractTrivyVersion(deployments []appsv1."Deployment, daemonsets []appsv1."DaemonSet) string {
	// Try to extract version from image tags
	for _, deployment := range deployments {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if strings.Contains(strings.ToLower(container.Image), "trivy") {
				// Extract version from image tag
				parts := strings.Split(container.Image, ":")
				if len(parts) > 1 {
					return parts[len(parts)-1]
				}
			}
		}
	}

	for _, daemonset := range daemonsets {
		for _, container := range daemonset.Spec.Template.Spec.Containers {
			if strings.Contains(strings.ToLower(container.Image), "trivy") {
				// Extract version from image tag
				parts := strings.Split(container.Image, ":")
				if len(parts) > 1 {
					return parts[len(parts)-1]
				}
			}
		}
	}

	return "unknown"
}

// DetectFalco detects if Falco is installed and running
func (td *ToolDetector) DetectFalco() (*ToolStatus, error) {
	log.Println("🔍 Detecting Falco installation...")

	status := &ToolStatus{
		Name:        "falco",
		LastChecked: time.Now(),
		Components:  []ComponentStatus{},
	}

	// Check for Falco namespace
	namespaces := []string{"falco", "falco-system", "falcosecurity"}
	var falcoNamespace string

	for _, ns := range namespaces {
		_, err := td.clientSet.CoreV1().Namespaces().Get(context.TODO(), ns, metav1."GetOptions{})
		if err == nil {
			falcoNamespace = ns
			break
		}
	}

	if falcoNamespace == "" {
		status.Installed = false
		status.HealthStatus = "not-installed"
		status.Error = "Falco namespace not found"
		log.Println("❌ Falco namespace not found")
		return status, nil
	}

	status.Namespace = falcoNamespace
	log.Printf("✅ Found Falco namespace: %s", falcoNamespace)

	// Check for Falco daemonsets
	daemonsets, err := td.clientSet.AppsV1().DaemonSets(falcoNamespace).List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		status.Error = fmt.Sprintf("Failed to list daemonsets: %v", err)
		return status, err
	}

	falcoDaemonsets := []appsv1."DaemonSet{}
	for _, daemonset := range daemonsets.Items {
		if strings.Contains(strings.ToLower(daemonset.Name), "falco") {
			falcoDaemonsets = append(falcoDaemonsets, daemonset)
		}
	}

	// Check for Falco services
	services, err := td.clientSet.CoreV1().Services(falcoNamespace).List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		status.Error = fmt.Sprintf("Failed to list services: %v", err)
		return status, err
	}

	falcoServices := []corev1."Service{}
	for _, service := range services.Items {
		if strings.Contains(strings.ToLower(service.Name), "falco") {
			falcoServices = append(falcoServices, service)
		}
	}

	// Check for Falco ConfigMaps
	configmaps, err := td.clientSet.CoreV1().ConfigMaps(falcoNamespace).List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		status.Error = fmt.Sprintf("Failed to list configmaps: %v", err)
		return status, err
	}

	falcoConfigMaps := []corev1."ConfigMap{}
	for _, cm := range configmaps.Items {
		if strings.Contains(strings.ToLower(cm.Name), "falco") {
			falcoConfigMaps = append(falcoConfigMaps, cm)
		}
	}

	// Analyze components
	allComponentsReady := true
	totalReplicas := int32(0)
	totalAvailable := int32(0)

	// Process daemonsets
	for _, daemonset := range falcoDaemonsets {
		component := ComponentStatus{
			Name:      daemonset.Name,
			Type:      "daemonset",
			Status:    "running",
			Ready:     daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled,
			Replicas:  daemonset.Status.DesiredNumberScheduled,
			Available: daemonset.Status.NumberReady,
		}

		if !component.Ready {
			allComponentsReady = false
		}

		totalReplicas += component.Replicas
		totalAvailable += component.Available
		status.Components = append(status.Components, component)
	}

	// Process services
	for _, service := range falcoServices {
		component := ComponentStatus{
			Name:   service.Name,
			Type:   "service",
			Status: "running",
			Ready:  true, // Services don't have ready status
		}
		status.Components = append(status.Components, component)
	}

	// Process configmaps
	for _, cm := range falcoConfigMaps {
		component := ComponentStatus{
			Name:   cm.Name,
			Type:   "configmap",
			Status: "installed",
			Ready:  true,
		}
		status.Components = append(status.Components, component)
	}

	// Determine overall status
	if len(status.Components) == 0 {
		status.Installed = false
		status.HealthStatus = "not-installed"
		status.Error = "No Falco components found"
		log.Println("❌ No Falco components found")
	} else if allComponentsReady {
		status.Installed = true
		status.HealthStatus = "healthy"
		status.Version = td.extractFalcoVersion(falcoDaemonsets)
		log.Printf("✅ Falco is installed and healthy (version: %s)", status.Version)
	} else {
		status.Installed = true
		status.HealthStatus = "unhealthy"
		status.Error = fmt.Sprintf("Some components not ready (%d/%d replicas available)", totalAvailable, totalReplicas)
		log.Printf("⚠️ Falco is installed but unhealthy: %s", status.Error)
	}

	return status, nil
}

// extractFalcoVersion extracts version information from Falco daemonsets
func (td *ToolDetector) extractFalcoVersion(daemonsets []appsv1."DaemonSet) string {
	// Try to extract version from image tags
	for _, daemonset := range daemonsets {
		for _, container := range daemonset.Spec.Template.Spec.Containers {
			if strings.Contains(strings.ToLower(container.Image), "falco") {
				// Extract version from image tag
				parts := strings.Split(container.Image, ":")
				if len(parts) > 1 {
					return parts[len(parts)-1]
				}
			}
		}
	}
	return "unknown"
}

// DetectKyverno detects if Kyverno is installed and running
func (td *ToolDetector) DetectKyverno() (*ToolStatus, error) {
	log.Println("🔍 Detecting Kyverno installation...")

	status := &ToolStatus{
		Name:        "kyverno",
		LastChecked: time.Now(),
		Components:  []ComponentStatus{},
	}

	// Check for Kyverno namespace
	namespaces := []string{"kyverno", "kyverno-system"}
	var kyvernoNamespace string

	for _, ns := range namespaces {
		_, err := td.clientSet.CoreV1().Namespaces().Get(context.TODO(), ns, metav1."GetOptions{})
		if err == nil {
			kyvernoNamespace = ns
			break
		}
	}

	if kyvernoNamespace == "" {
		status.Installed = false
		status.HealthStatus = "not-installed"
		status.Error = "Kyverno namespace not found"
		log.Println("❌ Kyverno namespace not found")
		return status, nil
	}

	status.Namespace = kyvernoNamespace
	log.Printf("✅ Found Kyverno namespace: %s", kyvernoNamespace)

	// Check for Kyverno deployments
	deployments, err := td.clientSet.AppsV1().Deployments(kyvernoNamespace).List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		status.Error = fmt.Sprintf("Failed to list deployments: %v", err)
		return status, err
	}

	kyvernoDeployments := []appsv1."Deployment{}
	for _, deployment := range deployments.Items {
		if strings.Contains(strings.ToLower(deployment.Name), "kyverno") {
			kyvernoDeployments = append(kyvernoDeployments, deployment)
		}
	}

	// Check for Kyverno services
	services, err := td.clientSet.CoreV1().Services(kyvernoNamespace).List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		status.Error = fmt.Sprintf("Failed to list services: %v", err)
		return status, err
	}

	kyvernoServices := []corev1."Service{}
	for _, service := range services.Items {
		if strings.Contains(strings.ToLower(service.Name), "kyverno") {
			kyvernoServices = append(kyvernoServices, service)
		}
	}

	// Check for Kyverno CRDs
	kyvernoCRDs, err := td.detectKyvernoCRDs()
	if err != nil {
		log.Printf("⚠️ Failed to detect Kyverno CRDs: %v", err)
	}

	// Analyze components
	allComponentsReady := true
	totalReplicas := int32(0)
	totalAvailable := int32(0)

	// Process deployments
	for _, deployment := range kyvernoDeployments {
		component := ComponentStatus{
			Name:      deployment.Name,
			Type:      "deployment",
			Status:    "running",
			Ready:     deployment.Status.ReadyReplicas == deployment.Status.Replicas,
			Replicas:  deployment.Status.Replicas,
			Available: deployment.Status.ReadyReplicas,
		}

		if !component.Ready {
			allComponentsReady = false
		}

		totalReplicas += component.Replicas
		totalAvailable += component.Available
		status.Components = append(status.Components, component)
	}

	// Process services
	for _, service := range kyvernoServices {
		component := ComponentStatus{
			Name:   service.Name,
			Type:   "service",
			Status: "running",
			Ready:  true,
		}
		status.Components = append(status.Components, component)
	}

	// Process CRDs
	for _, crd := range kyvernoCRDs {
		component := ComponentStatus{
			Name:   crd,
			Type:   "crd",
			Status: "installed",
			Ready:  true,
		}
		status.Components = append(status.Components, component)
	}

	// Determine overall status
	if len(status.Components) == 0 {
		status.Installed = false
		status.HealthStatus = "not-installed"
		status.Error = "No Kyverno components found"
		log.Println("❌ No Kyverno components found")
	} else if allComponentsReady {
		status.Installed = true
		status.HealthStatus = "healthy"
		status.Version = td.extractKyvernoVersion(kyvernoDeployments)
		log.Printf("✅ Kyverno is installed and healthy (version: %s)", status.Version)
	} else {
		status.Installed = true
		status.HealthStatus = "unhealthy"
		status.Error = fmt.Sprintf("Some components not ready (%d/%d replicas available)", totalAvailable, totalReplicas)
		log.Printf("⚠️ Kyverno is installed but unhealthy: %s", status.Error)
	}

	return status, nil
}

// detectKyvernoCRDs detects Kyverno-specific CRDs
func (td *ToolDetector) detectKyvernoCRDs() ([]string, error) {
	var kyvernoCRDs []string

	// Check if dynamic client is available
	if td.dynamicClient == nil {
		log.Println("⚠️ Dynamic client not available, skipping CRD detection")
		return kyvernoCRDs, nil
	}

	// Common Kyverno CRDs
	possibleCRDs := []string{
		"clusterpolicies.kyverno.io",
		"policies.kyverno.io",
		"policyreports.wgpolicyk8s.io",
		"clusterpolicyreports.wgpolicyk8s.io",
		"policyexceptions.kyverno.io",
		"admissionreports.kyverno.io",
		"clusteradmissionreports.kyverno.io",
	}

	for _, crdName := range possibleCRDs {
		gvr := schema.GroupVersionResource{
			Group:    "apiextensions.k8s.io",
			Version:  "v1",
			Resource: "customresourcedefinitions",
		}

		_, err := td.dynamicClient.Resource(gvr).Get(context.TODO(), crdName, metav1."GetOptions{})
		if err == nil {
			kyvernoCRDs = append(kyvernoCRDs, crdName)
		}
	}

	return kyvernoCRDs, nil
}

// extractKyvernoVersion extracts version information from Kyverno deployments
func (td *ToolDetector) extractKyvernoVersion(deployments []appsv1."Deployment) string {
	// Try to extract version from image tags
	for _, deployment := range deployments {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if strings.Contains(strings.ToLower(container.Image), "kyverno") {
				// Extract version from image tag
				parts := strings.Split(container.Image, ":")
				if len(parts) > 1 {
					return parts[len(parts)-1]
				}
			}
		}
	}
	return "unknown"
}

// DetectKubeBench detects if kube-bench is installed and running
func (td *ToolDetector) DetectKubeBench() (*ToolStatus, error) {
	log.Println("🔍 Detecting kube-bench installation...")

	status := &ToolStatus{
		Name:        "kube-bench",
		LastChecked: time.Now(),
		Components:  []ComponentStatus{},
	}

	// Check for kube-bench jobs (kube-bench runs as jobs)
	jobs, err := td.clientSet.BatchV1().Jobs("").List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		status.Error = fmt.Sprintf("Failed to list jobs: %v", err)
		return status, err
	}

	kubeBenchJobs := []batchv1."Job{}
	for _, job := range jobs.Items {
		if strings.Contains(strings.ToLower(job.Name), "kube-bench") ||
			strings.Contains(strings.ToLower(job.Name), "kubebench") {
			kubeBenchJobs = append(kubeBenchJobs, job)
		}
	}

	// Check for kube-bench CronJobs
	cronJobs, err := td.clientSet.BatchV1().CronJobs("").List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		log.Printf("⚠️ Failed to list cronjobs: %v", err)
	} else {
		kubeBenchCronJobs := []batchv1."CronJob{}
		for _, cronJob := range cronJobs.Items {
			if strings.Contains(strings.ToLower(cronJob.Name), "kube-bench") ||
				strings.Contains(strings.ToLower(cronJob.Name), "kubebench") {
				kubeBenchCronJobs = append(kubeBenchCronJobs, cronJob)
			}
		}

		// Process cronjobs
		for _, cronJob := range kubeBenchCronJobs {
			component := ComponentStatus{
				Name:   cronJob.Name,
				Type:   "cronjob",
				Status: "scheduled",
				Ready:  true, // CronJobs don't have ready status
			}
			status.Components = append(status.Components, component)
		}
	}

	// Process jobs
	for _, job := range kubeBenchJobs {
		// Calculate replicas
		var replicas int32 = 1
		if job.Spec.Completions != nil {
			replicas = *job.Spec.Completions
		}

		component := ComponentStatus{
			Name:      job.Name,
			Type:      "job",
			Status:    "running",
			Ready:     job.Status.Succeeded > 0 || job.Status.Active > 0,
			Replicas:  replicas,
			Available: job.Status.Succeeded,
		}
		status.Components = append(status.Components, component)
	}

	// Determine overall status
	if len(status.Components) == 0 {
		status.Installed = false
		status.HealthStatus = "not-installed"
		status.Error = "No kube-bench components found"
		log.Println("❌ No kube-bench components found")
	} else {
		status.Installed = true
		status.HealthStatus = "healthy"
		status.Version = "unknown" // kube-bench version is harder to extract
		log.Printf("✅ Kube-bench is installed and healthy")
	}

	return status, nil
}

// DetectKubernetesAudit detects if Kubernetes audit logging is configured
func (td *ToolDetector) DetectKubernetesAudit() (*ToolStatus, error) {
	log.Println("🔍 Detecting Kubernetes audit logging configuration...")

	status := &ToolStatus{
		Name:        "kubernetes-audit",
		LastChecked: time.Now(),
		Components:  []ComponentStatus{},
	}

	// Check for API server configuration
	apiServerConfig, err := td.checkAPIServerAuditConfig()
	if err != nil {
		log.Printf("⚠️ Failed to check API server audit config: %v", err)
	}

	// Check for audit webhook configuration
	webhookConfig, err := td.checkAuditWebhookConfig()
	if err != nil {
		log.Printf("⚠️ Failed to check audit webhook config: %v", err)
	}

	// Check for audit policy configuration
	policyConfig, err := td.checkAuditPolicyConfig()
	if err != nil {
		log.Printf("⚠️ Failed to check audit policy config: %v", err)
	}

	// Check for audit log files or volumes
	logConfig, err := td.checkAuditLogConfig()
	if err != nil {
		log.Printf("⚠️ Failed to check audit log config: %v", err)
	}

	// Analyze components
	allComponentsReady := true

	// Process API server configuration
	if apiServerConfig != nil {
		component := ComponentStatus{
			Name:   "api-server-audit",
			Type:   "configuration",
			Status: apiServerConfig.Status,
			Ready:  apiServerConfig.Ready,
		}
		if !component.Ready {
			allComponentsReady = false
		}
		status.Components = append(status.Components, component)
	}

	// Process webhook configuration
	if webhookConfig != nil {
		component := ComponentStatus{
			Name:   "audit-webhook",
			Type:   "webhook",
			Status: webhookConfig.Status,
			Ready:  webhookConfig.Ready,
		}
		if !component.Ready {
			allComponentsReady = false
		}
		status.Components = append(status.Components, component)
	}

	// Process policy configuration
	if policyConfig != nil {
		component := ComponentStatus{
			Name:   "audit-policy",
			Type:   "policy",
			Status: policyConfig.Status,
			Ready:  policyConfig.Ready,
		}
		if !component.Ready {
			allComponentsReady = false
		}
		status.Components = append(status.Components, component)
	}

	// Process log configuration
	if logConfig != nil {
		component := ComponentStatus{
			Name:   "audit-logs",
			Type:   "logs",
			Status: logConfig.Status,
			Ready:  logConfig.Ready,
		}
		if !component.Ready {
			allComponentsReady = false
		}
		status.Components = append(status.Components, component)
	}

	// Determine overall status
	if len(status.Components) == 0 {
		status.Installed = false
		status.HealthStatus = "not-configured"
		status.Error = "No Kubernetes audit configuration found"
		log.Println("❌ No Kubernetes audit configuration found")
	} else if allComponentsReady {
		status.Installed = true
		status.HealthStatus = "configured"
		status.Version = "unknown" // Audit version is tied to Kubernetes version
		log.Printf("✅ Kubernetes audit logging is configured and healthy")
	} else {
		status.Installed = true
		status.HealthStatus = "partially-configured"
		status.Error = "Some audit components not properly configured"
		log.Printf("⚠️ Kubernetes audit logging is partially configured: %s", status.Error)
	}

	return status, nil
}

// checkAPIServerAuditConfig checks if API server has audit configuration
func (td *ToolDetector) checkAPIServerAuditConfig() (*ComponentStatus, error) {
	// Get API server pods
	pods, err := td.clientSet.CoreV1().Pods("kube-system").List(context.TODO(), metav1."ListOptions{
		LabelSelector: "component=kube-apiserver",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list API server pods: %v", err)
	}

	if len(pods.Items) == 0 {
		return &ComponentStatus{
			Name:   "api-server-audit",
			Type:   "configuration",
			Status: "not-found",
			Ready:  false,
		}, nil
	}

	// Check the first API server pod for audit-related arguments
	pod := pods.Items[0]
	hasAuditConfig := false
	hasAuditPolicy := false
	hasAuditWebhook := false

	for _, container := range pod.Spec.Containers {
		if container.Name == "kube-apiserver" {
			for _, arg := range container.Args {
				if strings.Contains(arg, "--audit-log-path") || strings.Contains(arg, "--audit-log-maxage") {
					hasAuditConfig = true
				}
				if strings.Contains(arg, "--audit-policy-file") {
					hasAuditPolicy = true
				}
				if strings.Contains(arg, "--audit-webhook-config-file") {
					hasAuditWebhook = true
				}
			}
		}
	}

	status := "not-configured"
	ready := false

	if hasAuditConfig || hasAuditPolicy || hasAuditWebhook {
		status = "configured"
		ready = true
	}

	return &ComponentStatus{
		Name:   "api-server-audit",
		Type:   "configuration",
		Status: status,
		Ready:  ready,
	}, nil
}

// checkAuditWebhookConfig checks for audit webhook configuration
func (td *ToolDetector) checkAuditWebhookConfig() (*ComponentStatus, error) {
	// Check for audit webhook services
	services, err := td.clientSet.CoreV1().Services("").List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %v", err)
	}

	hasAuditWebhook := false
	for _, service := range services.Items {
		if strings.Contains(strings.ToLower(service.Name), "audit") &&
			strings.Contains(strings.ToLower(service.Name), "webhook") {
			hasAuditWebhook = true
			break
		}
	}

	status := "not-configured"
	ready := false

	if hasAuditWebhook {
		status = "configured"
		ready = true
	}

	return &ComponentStatus{
		Name:   "audit-webhook",
		Type:   "webhook",
		Status: status,
		Ready:  ready,
	}, nil
}

// checkAuditPolicyConfig checks for audit policy configuration
func (td *ToolDetector) checkAuditPolicyConfig() (*ComponentStatus, error) {
	// Check for audit policy ConfigMaps
	configmaps, err := td.clientSet.CoreV1().ConfigMaps("").List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list configmaps: %v", err)
	}

	hasAuditPolicy := false
	for _, cm := range configmaps.Items {
		if strings.Contains(strings.ToLower(cm.Name), "audit") &&
			strings.Contains(strings.ToLower(cm.Name), "policy") {
			hasAuditPolicy = true
			break
		}
	}

	status := "not-configured"
	ready := false

	if hasAuditPolicy {
		status = "configured"
		ready = true
	}

	return &ComponentStatus{
		Name:   "audit-policy",
		Type:   "policy",
		Status: status,
		Ready:  ready,
	}, nil
}

// checkAuditLogConfig checks for audit log configuration
func (td *ToolDetector) checkAuditLogConfig() (*ComponentStatus, error) {
	// Check for audit log volumes or persistent volumes
	pvs, err := td.clientSet.CoreV1().PersistentVolumes().List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list persistent volumes: %v", err)
	}

	hasAuditLogs := false
	for _, pv := range pvs.Items {
		if strings.Contains(strings.ToLower(pv.Name), "audit") ||
			strings.Contains(strings.ToLower(pv.Name), "log") {
			hasAuditLogs = true
			break
		}
	}

	// Also check for audit-related ConfigMaps
	configmaps, err := td.clientSet.CoreV1().ConfigMaps("").List(context.TODO(), metav1."ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list configmaps: %v", err)
	}

	for _, cm := range configmaps.Items {
		if strings.Contains(strings.ToLower(cm.Name), "audit") &&
			strings.Contains(strings.ToLower(cm.Name), "log") {
			hasAuditLogs = true
			break
		}
	}

	status := "not-configured"
	ready := false

	if hasAuditLogs {
		status = "configured"
		ready = true
	}

	return &ComponentStatus{
		Name:   "audit-logs",
		Type:   "logs",
		Status: status,
		Ready:  ready,
	}, nil
}

// DetectAllTools detects all security tools
func (td *ToolDetector) DetectAllTools() (map[string]*ToolStatus, error) {
	log.Println("🔍 Detecting all security tools...")

	tools := make(map[string]*ToolStatus)

	// Detect Trivy
	trivyStatus, err := td.DetectTrivy()
	if err != nil {
		log.Printf("❌ Failed to detect Trivy: %v", err)
		trivyStatus = &ToolStatus{
			Name:         "trivy",
			Installed:    false,
			HealthStatus: "error",
			Error:        err.Error(),
			LastChecked:  time.Now(),
		}
	}
	tools["trivy"] = trivyStatus

	// Detect Falco
	falcoStatus, err := td.DetectFalco()
	if err != nil {
		log.Printf("❌ Failed to detect Falco: %v", err)
		falcoStatus = &ToolStatus{
			Name:         "falco",
			Installed:    false,
			HealthStatus: "error",
			Error:        err.Error(),
			LastChecked:  time.Now(),
		}
	}
	tools["falco"] = falcoStatus

	// Detect Kyverno
	kyvernoStatus, err := td.DetectKyverno()
	if err != nil {
		log.Printf("❌ Failed to detect Kyverno: %v", err)
		kyvernoStatus = &ToolStatus{
			Name:         "kyverno",
			Installed:    false,
			HealthStatus: "error",
			Error:        err.Error(),
			LastChecked:  time.Now(),
		}
	}
	tools["kyverno"] = kyvernoStatus

	// Detect kube-bench
	kubeBenchStatus, err := td.DetectKubeBench()
	if err != nil {
		log.Printf("❌ Failed to detect kube-bench: %v", err)
		kubeBenchStatus = &ToolStatus{
			Name:         "kube-bench",
			Installed:    false,
			HealthStatus: "error",
			Error:        err.Error(),
			LastChecked:  time.Now(),
		}
	}
	tools["kube-bench"] = kubeBenchStatus

	// Detect Kubernetes audit logging
	auditStatus, err := td.DetectKubernetesAudit()
	if err != nil {
		log.Printf("❌ Failed to detect Kubernetes audit: %v", err)
		auditStatus = &ToolStatus{
			Name:         "kubernetes-audit",
			Installed:    false,
			HealthStatus: "error",
			Error:        err.Error(),
			LastChecked:  time.Now(),
		}
	}
	tools["kubernetes-audit"] = auditStatus

	log.Printf("✅ Tool detection completed. Found %d tools", len(tools))
	return tools, nil
}

// GetToolStatus returns the current status of a specific tool
func (td *ToolDetector) GetToolStatus(toolName string) (*ToolStatus, error) {
	switch strings.ToLower(toolName) {
	case "trivy":
		return td.DetectTrivy()
	case "falco":
		return td.DetectFalco()
	case "kyverno":
		return td.DetectKyverno()
	case "kube-bench", "kubebench":
		return td.DetectKubeBench()
	case "kubernetes-audit", "audit", "k8s-audit":
		return td.DetectKubernetesAudit()
	default:
		return nil, fmt.Errorf("tool detection not implemented for: %s", toolName)
	}
}
