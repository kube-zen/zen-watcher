package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🚀 zen-watcher v1.0.19 (Go 1.22, Apache 2.0)")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// K8s client
	log.Println("📡 Initializing Kubernetes client...")
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("❌ Failed to get in-cluster config: %v", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("❌ Failed to create dynamic client: %v", err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("❌ Failed to create clientset: %v", err)
	}
	log.Println("✅ Kubernetes client ready")

	// ZenAgentEvent GVR (for creating events)
	eventGVR := schema.GroupVersionResource{
		Group:    "zen.kube-zen.io",
		Version:  "v1",
		Resource: "zenagentevents",
	}

	// Security tool GVRs
	log.Println("📋 Configuring security tool watchers...")

	// Kyverno PolicyReport GVR
	policyGVR := schema.GroupVersionResource{
		Group:    "wgpolicyk8s.io",
		Version:  "v1alpha2",
		Resource: "policyreports",
	}
	log.Println("  → Kyverno PolicyReports (wgpolicyk8s.io/v1alpha2)")

	// Trivy VulnerabilityReports GVR
	trivyGVR := schema.GroupVersionResource{
		Group:    "aquasecurity.github.io",
		Version:  "v1alpha1",
		Resource: "vulnerabilityreports",
	}
	log.Println("  → Trivy VulnerabilityReports (aquasecurity.github.io/v1alpha1)")

	// Falco Events (via ConfigMaps or CRDs)
	log.Println("  → Falco Events (via sidekick/exporter)")
	log.Println("  → Kube-bench Reports (via ConfigMaps)")
	log.Println("  → Audit Logs (via K8s audit webhook)")

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("⚠️  Shutdown signal received")
		cancel()
	}()

	// Watch loop
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("✅ zen-watcher READY - Starting watch loop")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Auto-detect configuration
	autoDetect := os.Getenv("AUTO_DETECT_ENABLED")
	if autoDetect == "" {
		autoDetect = "true" // Enabled by default
	}
	log.Printf("🔍 Auto-detect: %s", autoDetect)

	// Tool detection state
	type ToolState struct {
		Installed bool
		Namespace string
		LastCheck time.Time
	}

	// Read namespaces from ENV or use defaults
	kyvernoNs := os.Getenv("KYVERNO_NAMESPACE")
	if kyvernoNs == "" {
		kyvernoNs = "kyverno"
	}
	trivyNs := os.Getenv("TRIVY_NAMESPACE")
	if trivyNs == "" {
		trivyNs = "trivy-system"
	}
	falcoNs := os.Getenv("FALCO_NAMESPACE")
	if falcoNs == "" {
		falcoNs = "falco"
	}
	kubebenchNs := os.Getenv("KUBEBENCH_NAMESPACE")
	if kubebenchNs == "" {
		kubebenchNs = "kube-bench"
	}

	toolStates := map[string]*ToolState{
		"kyverno":    {Namespace: kyvernoNs},
		"trivy":      {Namespace: trivyNs},
		"falco":      {Namespace: falcoNs},
		"kube-bench": {Namespace: kubebenchNs},
	}
	log.Printf("📋 Tool namespaces: Kyverno=%s, Trivy=%s, Falco=%s, kube-bench=%s", kyvernoNs, trivyNs, falcoNs, kubebenchNs)

	// Falco alerts channel (buffered for webhook)
	falcoAlertsChan := make(chan map[string]interface{}, 100)
	
	// Audit events channel (buffered for webhook)
	auditEventsChan := make(chan map[string]interface{}, 200)

	// Falco webhook handler (receives JSON alerts from Falco)
	http.HandleFunc("/falco/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		
		var alert map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			log.Printf("⚠️  Failed to parse Falco alert: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		// Send to channel for processing
		select {
		case falcoAlertsChan <- alert:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		default:
			log.Println("⚠️  Falco alerts channel full, dropping alert")
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	
	// Audit webhook handler (receives K8s audit events)
	http.HandleFunc("/audit/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		
		var auditEvent map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&auditEvent); err != nil {
			log.Printf("⚠️  Failed to parse audit event: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		// Send to channel for processing
		select {
		case auditEventsChan <- auditEvent:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		default:
			log.Println("⚠️  Audit events channel full, dropping event")
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})

	// Start HTTP server for health/readiness checks
	ready := false
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "healthy")
	})
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if ready {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "ready")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "not ready")
		}
	})
	go func() {
		port := os.Getenv("WATCHER_PORT")
		if port == "" {
			port = "8080"
		}
		log.Printf("🌐 HTTP server starting on :%s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("❌ HTTP server failed: %v", err)
		}
	}()

	// Mark as ready after initialization
	ready = true

	// Event tracking
	totalEventCount := 0
	recentEvents := []time.Time{}
	lastLoopCount := 0

	for {
		select {
		case <-ctx.Done():
			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Printf("✅ zen-watcher stopped (created %d events)", totalEventCount)
			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			return
		case <-ticker.C:
			loopStart := time.Now()
			lastLoopCount = 0

			// Auto-detect if enabled
			if autoDetect == "true" {
				log.Println("🔍 Auto-detecting security tools...")

				// Kyverno - check for any pod in namespace
				log.Printf("  → Listing pods in namespace '%s'...", toolStates["kyverno"].Namespace)
				kyvernoPods, err := clientSet.CoreV1().Pods(toolStates["kyverno"].Namespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					log.Printf("  ⚠️  Error listing pods in namespace '%s': %v", toolStates["kyverno"].Namespace, err)
					toolStates["kyverno"].Installed = false
				} else {
					log.Printf("  → Found %d pods in namespace '%s'", len(kyvernoPods.Items), toolStates["kyverno"].Namespace)
					if len(kyvernoPods.Items) > 0 {
						if !toolStates["kyverno"].Installed {
							log.Printf("  ✅ Kyverno detected in namespace '%s' (%d pods)", toolStates["kyverno"].Namespace, len(kyvernoPods.Items))
						}
						toolStates["kyverno"].Installed = true
					} else {
						toolStates["kyverno"].Installed = false
					}
				}
				toolStates["kyverno"].LastCheck = time.Now()

				// Trivy - check for any pod in namespace
				trivyPods, err := clientSet.CoreV1().Pods(toolStates["trivy"].Namespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					log.Printf("  ⚠️  Error listing pods in namespace '%s': %v", toolStates["trivy"].Namespace, err)
					toolStates["trivy"].Installed = false
				} else if len(trivyPods.Items) > 0 {
					if !toolStates["trivy"].Installed {
						log.Printf("  ✅ Trivy Operator detected in namespace '%s' (%d pods)", toolStates["trivy"].Namespace, len(trivyPods.Items))
					}
					toolStates["trivy"].Installed = true
				} else {
					log.Printf("  ℹ️  Namespace '%s' exists but has 0 pods", toolStates["trivy"].Namespace)
					toolStates["trivy"].Installed = false
				}
				toolStates["trivy"].LastCheck = time.Now()

				// Falco - check for any pod in namespace
				falcoPods, err := clientSet.CoreV1().Pods(toolStates["falco"].Namespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					log.Printf("  ⚠️  Error listing pods in namespace '%s': %v", toolStates["falco"].Namespace, err)
					toolStates["falco"].Installed = false
				} else if len(falcoPods.Items) > 0 {
					if !toolStates["falco"].Installed {
						log.Printf("  ✅ Falco detected in namespace '%s' (%d pods)", toolStates["falco"].Namespace, len(falcoPods.Items))
					}
					toolStates["falco"].Installed = true
				} else {
					log.Printf("  ℹ️  Namespace '%s' exists but has 0 pods", toolStates["falco"].Namespace)
					toolStates["falco"].Installed = false
				}
				toolStates["falco"].LastCheck = time.Now()
			}

			log.Println("🔍 Scanning security tool reports...")

			// 1. Kyverno PolicyReports
			if toolStates["kyverno"].Installed || autoDetect != "true" {
				log.Println("  → Checking Kyverno PolicyReports...")
				reports, err := dynClient.Resource(policyGVR).Namespace("").List(ctx, metav1.ListOptions{})
				if err != nil {
					log.Printf("  ⚠️  Cannot access Kyverno PolicyReports: %v", err)
				} else {
					log.Printf("  ✓ Found %d PolicyReports", len(reports.Items))

					// Get existing ZenAgentEvents for deduplication
					existingEvents, err := dynClient.Resource(eventGVR).Namespace("").List(ctx, metav1.ListOptions{
						LabelSelector: "source=kyverno,category=security",
					})
					existingKeys := make(map[string]bool)
					if err != nil {
						log.Printf("  ⚠️  Cannot load existing events for dedup: %v", err)
					} else {
						for _, ev := range existingEvents.Items {
							spec, _ := ev.Object["spec"].(map[string]interface{})
							if spec != nil {
								resource, _ := spec["resource"].(map[string]interface{})
								details, _ := spec["details"].(map[string]interface{})
								if resource != nil && details != nil {
									key := fmt.Sprintf("%s/%s/%s/%s/%s",
										ev.GetNamespace(),
										resource["kind"],
										resource["name"],
										details["policy"],
										details["rule"])
									existingKeys[key] = true
								}
							}
						}
					}
					log.Printf("  📋 Dedup: %d existing events, checking for new policy violations...", len(existingKeys))

					kyvernoCount := 0
					// Create ZenAgentEvents for NEW policy violations only
					for _, report := range reports.Items {
						results, found, _ := unstructured.NestedSlice(report.Object, "results")
						if !found || len(results) == 0 {
							continue
						}

						// Get resource info from report scope (Kyverno puts it here)
						scope, scopeFound, _ := unstructured.NestedMap(report.Object, "scope")
						if !scopeFound {
							continue
						}

						resourceKind := fmt.Sprintf("%v", scope["kind"])
						resourceName := fmt.Sprintf("%v", scope["name"])
						resourceNs := fmt.Sprintf("%v", scope["namespace"])
						if resourceNs == "" {
							resourceNs = report.GetNamespace()
						}

						for _, r := range results {
							result := r.(map[string]interface{})
							resultStatus := fmt.Sprintf("%v", result["result"])

							// Only process failed policies
							if resultStatus != "fail" {
								continue
							}

							policy := fmt.Sprintf("%v", result["policy"])
							rule := fmt.Sprintf("%v", result["rule"])
							severity := fmt.Sprintf("%v", result["severity"])
							message := fmt.Sprintf("%v", result["message"])

							dedupKey := fmt.Sprintf("%s/%s/%s/%s/%s",
								resourceNs, resourceKind, resourceName, policy, rule)

							// Skip if already exists
							if existingKeys[dedupKey] {
								continue
							}

							// Map severity to standard levels
							mappedSeverity := "MEDIUM"
							switch severity {
							case "high", "critical":
								mappedSeverity = "HIGH"
							case "low":
								mappedSeverity = "LOW"
							}

							// Create ZenAgentEvent
							event := &unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "zen.kube-zen.io/v1",
									"kind":       "ZenAgentEvent",
									"metadata": map[string]interface{}{
										"generateName": "kyverno-policy-",
										"namespace":    resourceNs,
										"labels": map[string]interface{}{
											"source":   "kyverno",
											"category": "security",
											"severity": mappedSeverity,
										},
									},
									"spec": map[string]interface{}{
										"source":     "kyverno",
										"category":   "security",
										"severity":   mappedSeverity,
										"eventType":  "policy-violation",
										"detectedAt": time.Now().Format(time.RFC3339),
										"resource": map[string]interface{}{
											"kind":      resourceKind,
											"name":      resourceName,
											"namespace": resourceNs,
										},
										"details": map[string]interface{}{
											"policy":  policy,
											"rule":    rule,
											"message": message,
											"result":  resultStatus,
										},
									},
								},
							}

							_, err := dynClient.Resource(eventGVR).Namespace(resourceNs).Create(ctx, event, metav1.CreateOptions{})
							if err != nil {
								log.Printf("  ⚠️  Failed to create ZenAgentEvent: %v", err)
							} else {
								kyvernoCount++
								existingKeys[dedupKey] = true
								lastLoopCount++
							}
						}
					}

					if kyvernoCount > 0 {
						log.Printf("  ✅ Created %d NEW ZenAgentEvents from Kyverno policy violations", kyvernoCount)
					}
				}
			} else {
				log.Println("  ℹ️  Kyverno not detected (skipping)")
			}

			// 2. Trivy VulnerabilityReports
			if toolStates["trivy"].Installed || autoDetect != "true" {
				log.Println("  → Checking Trivy VulnerabilityReports...")
				trivyReports, err := dynClient.Resource(trivyGVR).Namespace("").List(ctx, metav1.ListOptions{})
				if err != nil {
					log.Printf("  ⚠️  Cannot access Trivy reports: %v", err)
				} else {
					log.Printf("  ✓ Found %d VulnerabilityReports", len(trivyReports.Items))

					// Get existing ZenAgentEvents for deduplication
					existingEvents, err := dynClient.Resource(eventGVR).Namespace("").List(ctx, metav1.ListOptions{
						LabelSelector: "source=trivy,category=security",
					})
					existingKeys := make(map[string]bool)
					if err != nil {
						log.Printf("  ⚠️  Cannot load existing events for dedup: %v", err)
					} else {
						for _, ev := range existingEvents.Items {
							spec, _ := ev.Object["spec"].(map[string]interface{})
							if spec != nil {
								resource, _ := spec["resource"].(map[string]interface{})
								details, _ := spec["details"].(map[string]interface{})
								if resource != nil && details != nil {
									key := fmt.Sprintf("%s/%s/%s/%s",
										ev.GetNamespace(),
										resource["kind"],
										resource["name"],
										details["vulnerabilityID"])
									existingKeys[key] = true
								}
							}
						}
					}
					log.Printf("  📋 Dedup: %d existing events, checking for new vulnerabilities...", len(existingKeys))

					// Create ZenAgentEvents for NEW vulnerabilities only
					for _, report := range trivyReports.Items {
						vulnerabilities, found, _ := unstructured.NestedSlice(report.Object, "report", "vulnerabilities")
						if !found || len(vulnerabilities) == 0 {
							continue
						}

						resourceKind := report.GetLabels()["trivy-operator.resource.kind"]
						resourceName := report.GetLabels()["trivy-operator.resource.name"]

						// Only process HIGH and CRITICAL vulnerabilities
						for _, v := range vulnerabilities {
							vuln := v.(map[string]interface{})
							severity := vuln["severity"]
							if severity != "HIGH" && severity != "CRITICAL" {
								continue
							}

							vulnID := fmt.Sprintf("%v", vuln["vulnerabilityID"])
							dedupKey := fmt.Sprintf("%s/%s/%s/%s",
								report.GetNamespace(), resourceKind, resourceName, vulnID)

							// Skip if already exists
							if existingKeys[dedupKey] {
								continue
							}

							// Create ZenAgentEvent
							event := &unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "zen.kube-zen.io/v1",
									"kind":       "ZenAgentEvent",
									"metadata": map[string]interface{}{
										"generateName": "trivy-vuln-",
										"namespace":    report.GetNamespace(),
										"labels": map[string]interface{}{
											"source":   "trivy",
											"category": "security",
											"severity": fmt.Sprintf("%v", severity),
										},
									},
									"spec": map[string]interface{}{
										"source":     "trivy",
										"category":   "security",
										"severity":   severity,
										"eventType":  "vulnerability",
										"detectedAt": time.Now().Format(time.RFC3339),
										"resource": map[string]interface{}{
											"kind":      resourceKind,
											"name":      resourceName,
											"namespace": report.GetNamespace(),
										},
										"details": map[string]interface{}{
											"vulnerabilityID":  vulnID,
											"title":            vuln["title"],
											"description":      vuln["description"],
											"score":            vuln["score"],
											"fixedVersion":     vuln["fixedVersion"],
											"installedVersion": vuln["installedVersion"],
										},
									},
								},
							}

							_, err := dynClient.Resource(eventGVR).Namespace(report.GetNamespace()).Create(ctx, event, metav1.CreateOptions{})
							if err != nil {
								log.Printf("  ⚠️  Failed to create ZenAgentEvent: %v", err)
							} else {
								lastLoopCount++
								existingKeys[dedupKey] = true
							}
						}
					}

					if lastLoopCount > 0 {
						log.Printf("  ✅ Created %d NEW ZenAgentEvents (skipped %d duplicates)", lastLoopCount, len(existingKeys)-lastLoopCount)
					} else {
						log.Printf("  ℹ️  All vulnerabilities already have events (0 new)")
					}
				}
			} else {
				log.Println("  ℹ️  Trivy not detected (skipping)")
			}

			// 3. Falco - Process alerts from webhook channel
			if toolStates["falco"].Installed {
				log.Println("  → Checking Falco events...")

				// Get existing ZenAgentEvents for deduplication
				existingEvents, err := dynClient.Resource(eventGVR).Namespace("").List(ctx, metav1.ListOptions{
					LabelSelector: "source=falco,category=security",
				})
				existingKeys := make(map[string]bool)
				if err != nil {
					log.Printf("  ⚠️  Cannot load existing events for dedup: %v", err)
				} else {
					for _, ev := range existingEvents.Items {
						spec, _ := ev.Object["spec"].(map[string]interface{})
						if spec != nil {
							details, _ := spec["details"].(map[string]interface{})
							if details != nil {
								// Dedup by rule + output + pod/container
								rule := fmt.Sprintf("%v", details["rule"])
								output := fmt.Sprintf("%v", details["output"])
								podName := fmt.Sprintf("%v", details["k8s_pod_name"])
								outputKey := output
								if len(output) > 50 {
									outputKey = output[:50]
								}
								key := fmt.Sprintf("%s/%s/%s", rule, podName, outputKey)
								existingKeys[key] = true
							}
						}
					}
				}

				// Process alerts from channel (non-blocking)
				falcoCount := 0
			drainLoop:
				for {
					select {
					case alert := <-falcoAlertsChan:
						priority := fmt.Sprintf("%v", alert["priority"])
						rule := fmt.Sprintf("%v", alert["rule"])
						output := fmt.Sprintf("%v", alert["output"])

						// Only process Warning, Error, Critical, Alert, Emergency
						if priority != "Warning" && priority != "Error" && priority != "Critical" && priority != "Alert" && priority != "Emergency" {
							continue
						}

						// Get K8s context if present
						k8sPodName := fmt.Sprintf("%v", alert["k8s.pod.name"])
						k8sNs := fmt.Sprintf("%v", alert["k8s.ns.name"])
						if k8sNs == "<nil>" || k8sNs == "" {
							k8sNs = "falco"
						}

						// Dedup key
						outputKey := output
						if len(output) > 50 {
							outputKey = output[:50]
						}
						dedupKey := fmt.Sprintf("%s/%s/%s", rule, k8sPodName, outputKey)
						if existingKeys[dedupKey] {
							continue
						}

						// Map priority to severity
						severity := "MEDIUM"
						if priority == "Critical" || priority == "Alert" || priority == "Emergency" {
							severity = "HIGH"
						}

						// Create ZenAgentEvent
						event := &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "zen.kube-zen.io/v1",
								"kind":       "ZenAgentEvent",
								"metadata": map[string]interface{}{
									"generateName": "falco-",
									"namespace":    k8sNs,
									"labels": map[string]interface{}{
										"source":   "falco",
										"category": "security",
										"severity": severity,
									},
								},
								"spec": map[string]interface{}{
									"source":     "falco",
									"category":   "security",
									"severity":   severity,
									"eventType":  "runtime-security",
									"detectedAt": time.Now().Format(time.RFC3339),
									"resource": map[string]interface{}{
										"kind":      "Pod",
										"name":      k8sPodName,
										"namespace": k8sNs,
									},
									"details": map[string]interface{}{
										"rule":         rule,
										"priority":     priority,
										"output":       output,
										"k8s_pod_name": k8sPodName,
										"k8s_ns_name":  k8sNs,
									},
								},
							},
						}

						_, err := dynClient.Resource(eventGVR).Namespace(k8sNs).Create(ctx, event, metav1.CreateOptions{})
						if err != nil {
							log.Printf("  ⚠️  Failed to create Falco ZenAgentEvent: %v", err)
						} else {
							falcoCount++
							existingKeys[dedupKey] = true
							lastLoopCount++
						}

					default:
						break drainLoop
					}
				}

				if falcoCount > 0 {
					log.Printf("  ✅ Created %d NEW ZenAgentEvents from Falco alerts", falcoCount)
				} else {
					log.Println("  ℹ️  No new Falco alerts (configure Falco http_output to: http://zen-agent-zen-watcher.zen-cluster.svc.cluster.local:8080/falco/webhook)")
				}
			}

			// 4. Kube-bench - Check for kube-bench job results in ConfigMaps
			log.Println("  → Checking Kube-bench reports...")
			kubeBenchNs := os.Getenv("KUBE_BENCH_NAMESPACE")
			if kubeBenchNs == "" {
				kubeBenchNs = "kube-bench"
			}

			// Look for kube-bench ConfigMaps with app=kube-bench label
			configMaps, err := clientSet.CoreV1().ConfigMaps(kubeBenchNs).List(ctx, metav1.ListOptions{
				LabelSelector: "app=kube-bench",
			})
			if err == nil && len(configMaps.Items) > 0 {
				log.Printf("  ✓ Found %d kube-bench ConfigMaps", len(configMaps.Items))

				// Get existing ZenAgentEvents for deduplication
				existingEvents, err := dynClient.Resource(eventGVR).Namespace("").List(ctx, metav1.ListOptions{
					LabelSelector: "source=kube-bench,category=compliance",
				})
				existingKeys := make(map[string]bool)
				if err != nil {
					log.Printf("  ⚠️  Cannot load existing events for dedup: %v", err)
				} else {
					for _, ev := range existingEvents.Items {
						spec, _ := ev.Object["spec"].(map[string]interface{})
						if spec != nil {
							details, _ := spec["details"].(map[string]interface{})
							if details != nil {
								testNum := fmt.Sprintf("%v", details["testNumber"])
								existingKeys[testNum] = true
							}
						}
					}
				}
				log.Printf("  📋 Dedup: %d existing events, checking for new CIS benchmark failures...", len(existingKeys))

				kubeBenchCount := 0
				// Parse ConfigMaps for kube-bench JSON results
				for _, cm := range configMaps.Items {
					resultsJSON, found := cm.Data["results.json"]
					if !found {
						continue
					}

					// Parse kube-bench JSON output
					var benchResults map[string]interface{}
					if err := json.Unmarshal([]byte(resultsJSON), &benchResults); err != nil {
						log.Printf("  ⚠️  Failed to parse kube-bench JSON: %v", err)
						continue
					}

					controls, found := benchResults["Controls"].([]interface{})
					if !found {
						continue
					}

					// Iterate through all controls and tests
					for _, c := range controls {
						control := c.(map[string]interface{})
						tests, _ := control["tests"].([]interface{})

						for _, t := range tests {
							test := t.(map[string]interface{})
							results, _ := test["results"].([]interface{})
							section := fmt.Sprintf("%v", test["section"])

							for _, r := range results {
								result := r.(map[string]interface{})
								status := fmt.Sprintf("%v", result["status"])

								// Only process FAIL (ignore PASS, WARN)
								if status != "FAIL" {
									continue
								}

								testNumber := fmt.Sprintf("%v", result["test_number"])
								testDesc := fmt.Sprintf("%v", result["test_desc"])
								remediation := fmt.Sprintf("%v", result["remediation"])
								scored := result["scored"] == true

								// Skip if already exists
								if existingKeys[testNumber] {
									continue
								}

								// Scored FAIL = HIGH, unscored FAIL = MEDIUM
								severity := "MEDIUM"
								if scored {
									severity = "HIGH"
								}

								// Create ZenAgentEvent
								event := &unstructured.Unstructured{
									Object: map[string]interface{}{
										"apiVersion": "zen.kube-zen.io/v1",
										"kind":       "ZenAgentEvent",
										"metadata": map[string]interface{}{
											"generateName": "kube-bench-",
											"namespace":    kubeBenchNs,
											"labels": map[string]interface{}{
												"source":   "kube-bench",
												"category": "compliance",
												"severity": severity,
											},
										},
										"spec": map[string]interface{}{
											"source":     "kube-bench",
											"category":   "security",
											"severity":   severity,
											"eventType":  "cis-benchmark-fail",
											"detectedAt": time.Now().Format(time.RFC3339),
											"resource": map[string]interface{}{
												"kind": "Node",
												"name": "k3d-zen-agent-server-0",
											},
											"details": map[string]interface{}{
												"testNumber":  testNumber,
												"section":     section,
												"testDesc":    testDesc,
												"remediation": remediation,
												"scored":      scored,
											},
										},
									},
								}

								_, err := dynClient.Resource(eventGVR).Namespace(kubeBenchNs).Create(ctx, event, metav1.CreateOptions{})
								if err != nil {
									log.Printf("  ⚠️  Failed to create ZenAgentEvent: %v", err)
								} else {
									kubeBenchCount++
									existingKeys[testNumber] = true
									lastLoopCount++
								}
							}
						}
					}
				}

				if kubeBenchCount > 0 {
					log.Printf("  ✅ Created %d NEW ZenAgentEvents from kube-bench CIS failures", kubeBenchCount)
				}
			} else {
				log.Println("  ℹ️  No kube-bench ConfigMaps found (run kube-bench job to generate reports)")
			}

			// 5. Checkov - Static analysis results from ConfigMaps
			log.Println("  → Checking Checkov reports...")
			checkovNs := os.Getenv("CHECKOV_NAMESPACE")
			if checkovNs == "" {
				checkovNs = "checkov"
			}

			// Look for checkov ConfigMaps with app=checkov label
			checkovCMs, err := clientSet.CoreV1().ConfigMaps(checkovNs).List(ctx, metav1.ListOptions{
				LabelSelector: "app=checkov",
			})
			if err == nil && len(checkovCMs.Items) > 0 {
				log.Printf("  ✓ Found %d checkov ConfigMaps", len(checkovCMs.Items))

				// Get existing ZenAgentEvents for deduplication
				existingEvents, err := dynClient.Resource(eventGVR).Namespace("").List(ctx, metav1.ListOptions{
					LabelSelector: "source=checkov,category=security",
				})
				existingKeys := make(map[string]bool)
				if err != nil {
					log.Printf("  ⚠️  Cannot load existing events for dedup: %v", err)
				} else {
					for _, ev := range existingEvents.Items {
						spec, _ := ev.Object["spec"].(map[string]interface{})
						if spec != nil {
							details, _ := spec["details"].(map[string]interface{})
							if details != nil {
								checkID := fmt.Sprintf("%v", details["checkId"])
								resource := fmt.Sprintf("%v", details["resource"])
								key := fmt.Sprintf("%s/%s", checkID, resource)
								existingKeys[key] = true
							}
						}
					}
				}
				log.Printf("  📋 Dedup: %d existing events, checking for new Checkov failures...", len(existingKeys))

				checkovCount := 0
				// Parse ConfigMaps for Checkov JSON results
				for _, cm := range checkovCMs.Items {
					resultsJSON, found := cm.Data["results.json"]
					if !found {
						continue
					}

					// Parse Checkov JSON output
					var checkovResults map[string]interface{}
					if err := json.Unmarshal([]byte(resultsJSON), &checkovResults); err != nil {
						log.Printf("  ⚠️  Failed to parse Checkov JSON: %v", err)
						continue
					}

					results, found := checkovResults["results"].(map[string]interface{})
					if !found {
						continue
					}

					failedChecks, found := results["failed_checks"].([]interface{})
					if !found {
						continue
					}

					// Iterate through all failed checks
					for _, fc := range failedChecks {
						failedCheck := fc.(map[string]interface{})

						checkID := fmt.Sprintf("%v", failedCheck["check_id"])
						checkName := fmt.Sprintf("%v", failedCheck["check_name"])
						resource := fmt.Sprintf("%v", failedCheck["resource"])
						guideline := fmt.Sprintf("%v", failedCheck["guideline"])

						// Parse resource (format: "Kind.namespace.name")
						resourceParts := []string{resource}
						if len(resource) > 0 {
							parts := []string{}
							for _, p := range []byte(resource) {
								if p == '.' {
									parts = append(parts, "")
								} else if len(parts) > 0 {
									parts[len(parts)-1] += string(p)
								} else {
									parts = append(parts, string(p))
								}
							}
							resourceParts = parts
						}

						resourceKind := "Unknown"
						resourceNs := checkovNs
						resourceName := resource
						if len(resourceParts) >= 3 {
							resourceKind = resourceParts[0]
							resourceNs = resourceParts[1]
							resourceName = resourceParts[2]
						}

						// Dedup key
						dedupKey := fmt.Sprintf("%s/%s", checkID, resource)
						if existingKeys[dedupKey] {
							continue
						}

						// All Checkov failures are compliance/security issues
						// Map by check prefix
						category := "security"
						severity := "MEDIUM"
						if checkID[:7] == "CKV_K8S" {
							// Kubernetes checks - mostly security
							category = "security"
							// High severity for privilege escalation, root, capabilities
							if checkID == "CKV_K8S_20" || checkID == "CKV_K8S_23" || checkID == "CKV_K8S_16" {
								severity = "HIGH"
							}
						}

						// Create ZenAgentEvent
						event := &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "zen.kube-zen.io/v1",
								"kind":       "ZenAgentEvent",
								"metadata": map[string]interface{}{
									"generateName": "checkov-",
									"namespace":    resourceNs,
									"labels": map[string]interface{}{
										"source":   "checkov",
										"category": category,
										"severity": severity,
									},
								},
								"spec": map[string]interface{}{
									"source":     "checkov",
									"category":   category,
									"severity":   severity,
									"eventType":  "static-analysis",
									"detectedAt": time.Now().Format(time.RFC3339),
									"resource": map[string]interface{}{
										"kind":      resourceKind,
										"name":      resourceName,
										"namespace": resourceNs,
									},
									"details": map[string]interface{}{
										"checkId":   checkID,
										"checkName": checkName,
										"resource":  resource,
										"guideline": guideline,
									},
								},
							},
						}

						_, err := dynClient.Resource(eventGVR).Namespace(resourceNs).Create(ctx, event, metav1.CreateOptions{})
						if err != nil {
							log.Printf("  ⚠️  Failed to create Checkov ZenAgentEvent: %v", err)
						} else {
							checkovCount++
							existingKeys[dedupKey] = true
							lastLoopCount++
						}
					}
				}

				if checkovCount > 0 {
					log.Printf("  ✅ Created %d NEW ZenAgentEvents from Checkov static analysis failures", checkovCount)
				}
			} else {
				log.Println("  ℹ️  No Checkov ConfigMaps found (run checkov scan to generate reports)")
			}

			// 6. Audit logs - Process events from webhook
			log.Println("  → Checking Audit events...")
			
			// Get existing ZenAgentEvents for deduplication
			existingEvents, err := dynClient.Resource(eventGVR).Namespace("").List(ctx, metav1.ListOptions{
				LabelSelector: "source=audit,category=compliance",
			})
			existingKeys := make(map[string]bool)
			if err != nil {
				log.Printf("  ⚠️  Cannot load existing events for dedup: %v", err)
			} else {
				for _, ev := range existingEvents.Items {
					spec, _ := ev.Object["spec"].(map[string]interface{})
					if spec != nil {
						details, _ := spec["details"].(map[string]interface{})
						if details != nil {
							// Dedup by auditID
							auditID := fmt.Sprintf("%v", details["auditID"])
							existingKeys[auditID] = true
						}
					}
				}
			}
			
			// Process audit events from channel (non-blocking)
			auditCount := 0
			drainAuditLoop:
			for {
				select {
				case auditEvent := <-auditEventsChan:
					// Extract audit event fields
					auditID := fmt.Sprintf("%v", auditEvent["auditID"])
					stage := fmt.Sprintf("%v", auditEvent["stage"])
					verb := fmt.Sprintf("%v", auditEvent["verb"])
					
					// Only process ResponseComplete stage
					if stage != "ResponseComplete" {
						continue
					}
					
					// Filter for important actions: delete, create secrets/configmaps, RBAC changes
					objectRef, _ := auditEvent["objectRef"].(map[string]interface{})
					resource := fmt.Sprintf("%v", objectRef["resource"])
					namespace := fmt.Sprintf("%v", objectRef["namespace"])
					name := fmt.Sprintf("%v", objectRef["name"])
					apiGroup := fmt.Sprintf("%v", objectRef["apiGroup"])
					
					// Filter logic: only important events
					important := false
					category := "compliance"
					severity := "MEDIUM"
					eventType := "audit-event"
					
					// Delete operations (HIGH severity)
					if verb == "delete" {
						important = true
						severity = "HIGH"
						eventType = "resource-deletion"
					}
					
					// Secret/ConfigMap operations
					if resource == "secrets" || resource == "configmaps" {
						if verb == "create" || verb == "update" || verb == "patch" || verb == "delete" {
							important = true
							severity = "HIGH"
							eventType = "secret-access"
						}
					}
					
					// RBAC changes
					if apiGroup == "rbac.authorization.k8s.io" {
						if verb == "create" || verb == "update" || verb == "patch" || verb == "delete" {
							important = true
							severity = "HIGH"
							eventType = "rbac-change"
						}
					}
					
					// Privileged pod creation
					if resource == "pods" && verb == "create" {
						// Check request object for privileged
						requestObject, _ := auditEvent["requestObject"].(map[string]interface{})
						if requestObject != nil {
							spec, _ := requestObject["spec"].(map[string]interface{})
							if spec != nil {
								containers, _ := spec["containers"].([]interface{})
								for _, c := range containers {
									container, _ := c.(map[string]interface{})
									securityContext, _ := container["securityContext"].(map[string]interface{})
									if securityContext != nil {
										privileged, _ := securityContext["privileged"].(bool)
										if privileged {
											important = true
											severity = "HIGH"
											eventType = "privileged-pod-creation"
											break
										}
									}
								}
							}
						}
					}
					
					if !important {
						continue
					}
					
					// Dedup check
					if existingKeys[auditID] {
						continue
					}
					
					// Extract user info
					user, _ := auditEvent["user"].(map[string]interface{})
					username := fmt.Sprintf("%v", user["username"])
					
					// Extract response code
					responseStatus, _ := auditEvent["responseStatus"].(map[string]interface{})
					statusCode := fmt.Sprintf("%v", responseStatus["code"])
					
					if namespace == "<nil>" || namespace == "" {
						namespace = "default"
					}
					
					// Create ZenAgentEvent
					event := &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "zen.kube-zen.io/v1",
							"kind":       "ZenAgentEvent",
							"metadata": map[string]interface{}{
								"generateName": "audit-",
								"namespace":    namespace,
								"labels": map[string]interface{}{
									"source":   "audit",
									"category": category,
									"severity": severity,
								},
							},
							"spec": map[string]interface{}{
								"source":     "audit",
								"category":   category,
								"severity":   severity,
								"eventType":  eventType,
								"detectedAt": time.Now().Format(time.RFC3339),
								"resource": map[string]interface{}{
									"kind":      resource,
									"name":      name,
									"namespace": namespace,
									"apiGroup":  apiGroup,
								},
								"details": map[string]interface{}{
									"auditID":    auditID,
									"verb":       verb,
									"user":       username,
									"stage":      stage,
									"statusCode": statusCode,
								},
							},
						},
					}
					
					_, err := dynClient.Resource(eventGVR).Namespace(namespace).Create(ctx, event, metav1.CreateOptions{})
					if err != nil {
						log.Printf("  ⚠️  Failed to create Audit ZenAgentEvent: %v", err)
					} else {
						auditCount++
						existingKeys[auditID] = true
						lastLoopCount++
					}
					
				default:
					break drainAuditLoop
				}
			}
			
			if auditCount > 0 {
				log.Printf("  ✅ Created %d NEW ZenAgentEvents from Audit logs", auditCount)
			} else {
				log.Println("  ℹ️  No new audit events (configure K8s audit webhook to: http://zen-agent-zen-watcher.zen-cluster.svc.cluster.local:8080/audit/webhook)")
			}

			// Update totals
			totalEventCount += lastLoopCount
			now := time.Now()
			cutoff := now.Add(-5 * time.Minute)
			for i := 0; i < lastLoopCount; i++ {
				recentEvents = append(recentEvents, now)
			}
			newRecent := []time.Time{}
			for _, t := range recentEvents {
				if t.After(cutoff) {
					newRecent = append(newRecent, t)
				}
			}
			recentEvents = newRecent

			loopDuration := time.Since(loopStart)
			log.Printf("📊 Total ZenAgentEvents: %d. Created last 5 minutes: %d (loop took %v)",
				totalEventCount, len(recentEvents), loopDuration.Round(time.Millisecond))
		}
	}
}
