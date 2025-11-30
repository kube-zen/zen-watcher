package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// KubeBenchWatcher watches for kube-bench security findings
type KubeBenchWatcher struct {
	clientSet     *kubernetes.Clientset
	namespace     string
	actionHandler KubeBenchActionHandler
	interval      time.Duration
}

// KubeBenchResult represents the structure of kube-bench output
type KubeBenchResult struct {
	Controls []Control `json:"controls"`
}

// Control represents a CIS control group
type Control struct {
	ID          string  `json:"id"`
	Version     string  `json:"version"`
	Description string  `json:"description"`
	Tests       []Test  `json:"tests"`
	Groups      []Group `json:"groups"`
}

// Group represents a group of tests within a control
type Group struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Tests       []Test `json:"tests"`
}

// Test represents an individual test
type Test struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
	Status      string `json:"status"`
	Scored      bool   `json:"scored"`
	Level       int    `json:"level"`
}

// KubeBenchActionHandler interface for handling detected events
type KubeBenchActionHandler interface {
	HandleKubeBenchFinding(ctx context.Context, finding KubeBenchFinding) error
}

// KubeBenchFinding represents a security finding from kube-bench
type KubeBenchFinding struct {
	ControlID   string    `json:"control_id"`
	TestID      string    `json:"test_id"`
	Description string    `json:"description"`
	Remediation string    `json:"remediation"`
	Status      string    `json:"status"`
	Severity    string    `json:"severity"`
	Level       int       `json:"level"`
	Scored      bool      `json:"scored"`
	NodeName    string    `json:"node_name"`
	Timestamp   time.Time `json:"timestamp"`
	ClusterID   string    `json:"cluster_id"`
}

// NewKubeBenchWatcher creates a new kube-bench watcher
func NewKubeBenchWatcher(clientSet *kubernetes.Clientset, namespace string, actionHandler KubeBenchActionHandler) *KubeBenchWatcher {
	return &KubeBenchWatcher{
		clientSet:     clientSet,
		namespace:     namespace,
		actionHandler: actionHandler,
		interval:      15 * time.Minute, // Run every 15 minutes
	}
}

// Start starts the kube-bench watcher
func (kbw *KubeBenchWatcher) Start(ctx context.Context) error {
	logger.Info("Starting kube-bench watcher",
		logger.Fields{
			Component: "watcher",
			Operation: "kube_bench_start",
			Source:    "kube-bench",
		})

	// Run initial scan
	if err := kbw.runKubeBenchScan(ctx); err != nil {
		logger.Error("Initial kube-bench scan failed",
			logger.Fields{
				Component: "watcher",
				Operation: "kube_bench_scan",
				Source:    "kube-bench",
				Error:     err,
			})
	}

	// Start periodic scanning
	ticker := time.NewTicker(kbw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Kube-bench watcher stopped",
				logger.Fields{
					Component: "watcher",
					Operation: "kube_bench_stop",
					Source:    "kube-bench",
				})
			return ctx.Err()
		case <-ticker.C:
			if err := kbw.runKubeBenchScan(ctx); err != nil {
				logger.Error("Kube-bench scan failed",
					logger.Fields{
						Component: "watcher",
						Operation: "kube_bench_scan",
						Source:    "kube-bench",
						Error:     err,
					})
			}
		}
	}
}

// runKubeBenchScan runs kube-bench and processes the results
func (kbw *KubeBenchWatcher) runKubeBenchScan(ctx context.Context) error {
	logger.Debug("Running kube-bench security scan",
		logger.Fields{
			Component: "watcher",
			Operation: "kube_bench_scan",
			Source:    "kube-bench",
		})

	// Get cluster nodes
	nodes, err := kbw.clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	var findings []KubeBenchFinding

	// Run kube-bench on each node
	for _, node := range nodes.Items {
		nodeFindings, err := kbw.scanNode(ctx, node.Name)
		if err != nil {
			logger.Warn("Failed to scan node",
				logger.Fields{
					Component: "watcher",
					Operation: "kube_bench_scan",
					Source:    "kube-bench",
					Additional: map[string]interface{}{
						"node_name": node.Name,
					},
					Error: err,
				})
			continue
		}
		findings = append(findings, nodeFindings...)
	}

	// Process findings
	for _, finding := range findings {
		if err := kbw.actionHandler.HandleKubeBenchFinding(ctx, finding); err != nil {
			logger.Error("Failed to handle kube-bench finding",
				logger.Fields{
					Component: "watcher",
					Operation: "handle_kube_bench_finding",
					Source:    "kube-bench",
					Error:     err,
				})
		}
	}

	logger.Info("Kube-bench scan completed",
		logger.Fields{
			Component: "watcher",
			Operation: "kube_bench_scan",
			Source:    "kube-bench",
			Count:     len(findings),
		})
	return nil
}

// scanNode runs kube-bench on a specific node
func (kbw *KubeBenchWatcher) scanNode(ctx context.Context, nodeName string) ([]KubeBenchFinding, error) {
	logger.Debug("Scanning node",
		logger.Fields{
			Component: "watcher",
			Operation: "scan_node",
			Source:    "kube-bench",
			Additional: map[string]interface{}{
				"node_name": nodeName,
			},
		})

	// Create a kube-bench job for this node
	job := kbw.createKubeBenchJob(nodeName)

	// Create the job
	createdJob, err := kbw.clientSet.BatchV1().Jobs(kbw.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create kube-bench job: %w", err)
	}

	// Wait for job completion
	if err := kbw.waitForJobCompletion(ctx, createdJob.Name); err != nil {
		return nil, fmt.Errorf("job failed to complete: %w", err)
	}

	// Get job results
	findings, err := kbw.getJobResults(ctx, createdJob.Name, nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get job results: %w", err)
	}

	// Clean up the job
	if err := kbw.clientSet.BatchV1().Jobs(kbw.namespace).Delete(ctx, createdJob.Name, metav1.DeleteOptions{}); err != nil {
		logger.Warn("Failed to delete kube-bench job",
			logger.Fields{
				Component: "watcher",
				Operation: "cleanup_kube_bench_job",
				Source:    "kube-bench",
				Additional: map[string]interface{}{
					"job_name": createdJob.Name,
				},
				Error: err,
			})
	}

	return findings, nil
}

// createKubeBenchJob creates a Kubernetes job to run kube-bench
func (kbw *KubeBenchWatcher) createKubeBenchJob(nodeName string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("kube-bench-%s-%d", nodeName, time.Now().Unix()),
			Namespace: kbw.namespace,
			Labels: map[string]string{
				"app":       "kube-bench",
				"node":      nodeName,
				"scan-type": "security",
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeName:      nodeName,
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "kube-bench",
							Image:   "aquasec/kube-bench:latest",
							Command: []string{"kube-bench", "run", "--targets", "master,node", "--json"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "var-lib-etcd",
									MountPath: "/var/lib/etcd",
									ReadOnly:  true,
								},
								{
									Name:      "var-lib-kubelet",
									MountPath: "/var/lib/kubelet",
									ReadOnly:  true,
								},
								{
									Name:      "var-lib-kube-scheduler",
									MountPath: "/var/lib/kube-scheduler",
									ReadOnly:  true,
								},
								{
									Name:      "var-lib-kube-controller-manager",
									MountPath: "/var/lib/kube-controller-manager",
									ReadOnly:  true,
								},
								{
									Name:      "etc-systemd",
									MountPath: "/etc/systemd",
									ReadOnly:  true,
								},
								{
									Name:      "etc-kubernetes",
									MountPath: "/etc/kubernetes",
									ReadOnly:  true,
								},
								{
									Name:      "usr-bin",
									MountPath: "/usr/local/mount-from-host/bin",
									ReadOnly:  true,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &[]bool{true}[0],
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "var-lib-etcd",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/etcd",
								},
							},
						},
						{
							Name: "var-lib-kubelet",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet",
								},
							},
						},
						{
							Name: "var-lib-kube-scheduler",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kube-scheduler",
								},
							},
						},
						{
							Name: "var-lib-kube-controller-manager",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kube-controller-manager",
								},
							},
						},
						{
							Name: "etc-systemd",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/systemd",
								},
							},
						},
						{
							Name: "etc-kubernetes",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/kubernetes",
								},
							},
						},
						{
							Name: "usr-bin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/usr/bin",
								},
							},
						},
					},
				},
			},
		},
	}
}

// waitForJobCompletion waits for a job to complete
func (kbw *KubeBenchWatcher) waitForJobCompletion(ctx context.Context, jobName string) error {
	timeout := 5 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pod, err := kbw.clientSet.CoreV1().Pods(kbw.namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pod: %w", err)
		}

		if pod.Status.Phase == corev1.PodSucceeded {
			return nil
		}

		if pod.Status.Phase == corev1.PodFailed {
			return fmt.Errorf("kube-bench job failed")
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("kube-bench job timed out")
}

// getJobResults gets the results from a completed kube-bench job
func (kbw *KubeBenchWatcher) getJobResults(ctx context.Context, jobName, nodeName string) ([]KubeBenchFinding, error) {
	// Get pod logs
	logs, err := kbw.clientSet.CoreV1().Pods(kbw.namespace).GetLogs(jobName, &corev1.PodLogOptions{}).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer logs.Close()

	// Parse kube-bench JSON output
	var result KubeBenchResult
	if err := json.NewDecoder(logs).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse kube-bench output: %w", err)
	}

	// Convert to findings
	var findings []KubeBenchFinding
	for _, control := range result.Controls {
		for _, group := range control.Groups {
			for _, test := range group.Tests {
				if test.Status == "FAIL" {
					finding := KubeBenchFinding{
						ControlID:   control.ID,
						TestID:      test.ID,
						Description: test.Description,
						Remediation: test.Remediation,
						Status:      test.Status,
						Severity:    kbw.getSeverityFromLevel(test.Level),
						Level:       test.Level,
						Scored:      test.Scored,
						NodeName:    nodeName,
						Timestamp:   time.Now(),
						ClusterID:   os.Getenv("CLUSTER_ID"),
					}
					findings = append(findings, finding)
				}
			}
		}
	}

	return findings, nil
}

// getSeverityFromLevel converts CIS level to severity
func (kbw *KubeBenchWatcher) getSeverityFromLevel(level int) string {
	switch level {
	case 1:
		return "high"
	case 2:
		return "medium"
	default:
		return "low"
	}
}

// RunKubeBenchLocally runs kube-bench locally (for testing)
func (kbw *KubeBenchWatcher) RunKubeBenchLocally(ctx context.Context) ([]KubeBenchFinding, error) {
	logger.Debug("Running kube-bench locally",
		logger.Fields{
			Component: "watcher",
			Operation: "kube_bench_local",
			Source:    "kube-bench",
		})

	// Run kube-bench command
	cmd := exec.CommandContext(ctx, "kube-bench", "run", "--targets", "master,node", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run kube-bench: %w", err)
	}

	// Parse JSON output
	var result KubeBenchResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse kube-bench output: %w", err)
	}

	// Convert to findings
	var findings []KubeBenchFinding
	for _, control := range result.Controls {
		for _, group := range control.Groups {
			for _, test := range group.Tests {
				if test.Status == "FAIL" {
					finding := KubeBenchFinding{
						ControlID:   control.ID,
						TestID:      test.ID,
						Description: test.Description,
						Remediation: test.Remediation,
						Status:      test.Status,
						Severity:    kbw.getSeverityFromLevel(test.Level),
						Level:       test.Level,
						Scored:      test.Scored,
						NodeName:    "local",
						Timestamp:   time.Now(),
						ClusterID:   os.Getenv("CLUSTER_ID"),
					}
					findings = append(findings, finding)
				}
			}
		}
	}

	logger.Info("Local kube-bench scan completed",
		logger.Fields{
			Component: "watcher",
			Operation: "kube_bench_local",
			Source:    "kube-bench",
			Count:     len(findings),
		})
	return findings, nil
}
