// Copyright 2025 kube-zen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// LoadGenerator generates sustained traffic for e2e validation
type LoadGenerator struct {
	watcherURL string
	rate       int           // events per second
	duration   time.Duration // total duration
	namespace  string
}

// NewLoadGenerator creates a new load generator
func NewLoadGenerator(watcherURL string, rate int, duration time.Duration, namespace string) *LoadGenerator {
	return &LoadGenerator{
		watcherURL: watcherURL,
		rate:       rate,
		duration:   duration,
		namespace:  namespace,
	}
}

// GenerateFalcoWebhook generates a Falco webhook payload
func (lg *LoadGenerator) GenerateFalcoWebhook(priority, rule, podName string) map[string]interface{} {
	return map[string]interface{}{
		"output": fmt.Sprintf("Test event: %s - %s", rule, time.Now().Format(time.RFC3339)),
		"priority": priority,
		"rule":     rule,
		"time":     time.Now().UTC().Format(time.RFC3339Nano),
		"output_fields": map[string]interface{}{
			"k8s.pod.name": podName,
			"k8s.ns.name":  lg.namespace,
		},
		"source": "syscall",
		"tags":   []string{"container", "mitre"},
	}
}

// SendWebhook sends a webhook to zen-watcher
func (lg *LoadGenerator) SendWebhook(endpoint string, payload map[string]interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Use kubectl exec to send webhook from within cluster (avoids port-forward complexity)
	// This is a minimal approach - reuse existing send-webhooks.sh pattern
	url := fmt.Sprintf("%s%s", lg.watcherURL, endpoint)
	
	// For e2e, we'll use a simple HTTP client approach if URL is accessible
	// Otherwise, fall back to kubectl exec pattern
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		// If direct HTTP fails, try kubectl exec approach (reuse send-webhooks.sh pattern)
		return lg.sendWebhookViaKubectl(endpoint, jsonData)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// sendWebhookViaKubectl sends webhook using kubectl exec (reuses send-webhooks.sh pattern)
func (lg *LoadGenerator) sendWebhookViaKubectl(endpoint string, jsonData []byte) error {
	// This is a minimal implementation - in practice, we'd reuse scripts/data/send-webhooks.sh
	// For e2e, we'll use a pod-based approach similar to send-webhooks.sh
	kubeconfig := getKubeconfigPath()
	clusterName := "zen-demo"
	
	// Create a temporary pod to send webhook (reuse send-webhooks.sh pattern)
	podName := fmt.Sprintf("load-gen-%d", time.Now().Unix())
	watcherURL := fmt.Sprintf("http://zen-watcher.%s.svc.cluster.local:8080%s", lg.namespace, endpoint)
	
	// Use curl pod pattern from send-webhooks.sh
	podYAML := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 65534
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: curl
    image: curlimages/curl:latest
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop: ["ALL"]
    command: ["sh", "-c", "curl -s -X POST -H 'Content-Type: application/json' -d '%s' %s"]
  restartPolicy: Never
`, podName, lg.namespace, string(jsonData), watcherURL)

	cmd := exec.Command("kubectl", "--context=k3d-"+clusterName, "apply", "-f", "-")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	cmd.Stdin = bytes.NewReader([]byte(podYAML))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create webhook pod: %w", err)
	}

	// Wait for pod to complete
	time.Sleep(2 * time.Second)

	// Cleanup
	exec.Command("kubectl", "--context=k3d-"+clusterName, "delete", "pod", podName, "-n", lg.namespace, "--ignore-not-found=true").Run()

	return nil
}

// Run generates load for the specified duration
func (lg *LoadGenerator) Run() error {
	interval := time.Duration(1000/lg.rate) * time.Millisecond
	deadline := time.Now().Add(lg.duration)
	eventCount := 0

	for time.Now().Before(deadline) {
		// Generate and send Falco webhook
		payload := lg.GenerateFalcoWebhook("Warning", "Test rule", fmt.Sprintf("load-pod-%d", eventCount))
		if err := lg.SendWebhook("/falco/webhook", payload); err != nil {
			// Log but continue (non-fatal for load test)
			fmt.Printf("Warning: failed to send webhook %d: %v\n", eventCount, err)
		}
		eventCount++

		time.Sleep(interval)
	}

	fmt.Printf("Load generation complete: sent %d events over %v\n", eventCount, lg.duration)
	return nil
}

