package manager

import (
	"context"
	"log"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/actions"
	"github.com/kube-zen/zen-watcher/pkg/config"
	"github.com/kube-zen/zen-watcher/pkg/watcher"
	"github.com/kube-zen/zen-watcher/pkg/writer"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// WatcherManager manages all watchers and coordinates their activities
type WatcherManager struct {
	clientset        kubernetes.Interface
	dynamicClient    dynamic.Interface
	behaviorConfig   *config.BehaviorConfig
	trivyHandler     *actions.TrivyActionHandler
	falcoHandler     *actions.FalcoActionHandler
	auditHandler     *actions.AuditActionHandler
	kyvernoHandler   *actions.KyvernoActionHandler
	kubeBenchHandler *actions.KubeBenchActionHandler
	crdWriter        *writer.CRDWriter
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewWatcherManager creates a new WatcherManager
func NewWatcherManager(
	clientset kubernetes.Interface,
	dynamicClient dynamic.Interface,
	behaviorConfig *config.BehaviorConfig,
	trivyHandler *actions.TrivyActionHandler,
	falcoHandler *actions.FalcoActionHandler,
	auditHandler *actions.AuditActionHandler,
	kyvernoHandler *actions.KyvernoActionHandler,
	kubeBenchHandler *actions.KubeBenchActionHandler,
	crdWriter *writer.CRDWriter,
) *WatcherManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &WatcherManager{
		clientset:        clientset,
		dynamicClient:    dynamicClient,
		behaviorConfig:   behaviorConfig,
		trivyHandler:     trivyHandler,
		falcoHandler:     falcoHandler,
		auditHandler:     auditHandler,
		kyvernoHandler:   kyvernoHandler,
		kubeBenchHandler: kubeBenchHandler,
		crdWriter:        crdWriter,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start starts all watchers based on the behavior configuration
func (wm *WatcherManager) Start() error {
	log.Println("🚀 Starting WatcherManager...")

	// Start watchers based on behavior configuration
	if wm.behaviorConfig.IsTrivyEnabled() {
		log.Println("🔍 Starting Trivy watcher...")
		go wm.startTrivyWatcher()
	}

	if wm.behaviorConfig.IsFalcoEnabled() {
		log.Println("🔍 Starting Falco watcher...")
		go wm.startFalcoWatcher()
	}

	if wm.behaviorConfig.IsAuditEnabled() {
		log.Println("🔍 Starting Audit watcher...")
		go wm.startAuditWatcher()
	}

	if wm.behaviorConfig.IsKyvernoEnabled() {
		log.Println("🔍 Starting Kyverno watcher...")
		go wm.startKyvernoWatcher()
	}

	if wm.behaviorConfig.IsKubeBenchEnabled() {
		log.Println("🔍 Starting Kube-bench watcher...")
		go wm.startKubeBenchWatcher()
	}

	// Start periodic event writing
	go wm.startEventWriter()

	log.Println("✅ WatcherManager started successfully")
	return nil
}

// Stop stops all watchers
func (wm *WatcherManager) Stop() {
	log.Println("🛑 Stopping WatcherManager...")
	wm.cancel()
	log.Println("✅ WatcherManager stopped")
}

// startTrivyWatcher starts the Trivy watcher
func (wm *WatcherManager) startTrivyWatcher() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			log.Println("🛑 Trivy watcher stopped")
			return
		case <-ticker.C:
			log.Println("🔍 Checking Trivy for new vulnerabilities...")

			// Collect Trivy events
			events := wm.trivyHandler.GetRecentEvents()
			if len(events) > 0 {
				log.Printf("📊 Found %d Trivy events", len(events))

				// Write events to CRDs
				if err := wm.crdWriter.WriteSecurityEvents(wm.ctx, wm.trivyHandler, wm.falcoHandler, wm.auditHandler, wm.kyvernoHandler); err != nil {
					log.Printf("❌ Failed to write Trivy events: %v", err)
				}
			}
		}
	}
}

// startFalcoWatcher starts the Falco watcher
func (wm *WatcherManager) startFalcoWatcher() {
	ticker := time.NewTicker(15 * time.Second) // Check every 15 seconds
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			log.Println("🛑 Falco watcher stopped")
			return
		case <-ticker.C:
			log.Println("🔍 Checking Falco for new security events...")

			// Collect Falco events
			events := wm.falcoHandler.GetRecentEvents()
			if len(events) > 0 {
				log.Printf("📊 Found %d Falco events", len(events))

				// Write events to CRDs
				if err := wm.crdWriter.WriteSecurityEvents(wm.ctx, wm.trivyHandler, wm.falcoHandler, wm.auditHandler, wm.kyvernoHandler); err != nil {
					log.Printf("❌ Failed to write Falco events: %v", err)
				}
			}
		}
	}
}

// startAuditWatcher starts the Audit watcher
func (wm *WatcherManager) startAuditWatcher() {
	ticker := time.NewTicker(60 * time.Second) // Check every 60 seconds
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			log.Println("🛑 Audit watcher stopped")
			return
		case <-ticker.C:
			log.Println("🔍 Checking Audit logs for new events...")

			// Collect Audit events
			events := wm.auditHandler.GetRecentEvents()
			if len(events) > 0 {
				log.Printf("📊 Found %d Audit events", len(events))

				// Write events to CRDs
				if err := wm.crdWriter.WriteSecurityEvents(wm.ctx, wm.trivyHandler, wm.falcoHandler, wm.auditHandler, wm.kyvernoHandler); err != nil {
					log.Printf("❌ Failed to write Audit events: %v", err)
				}
			}
		}
	}
}

// startKyvernoWatcher starts the Kyverno watcher
func (wm *WatcherManager) startKyvernoWatcher() {
	// Import the watcher package
	kyvernoWatcher := watcher.NewKyvernoWatcher(
		wm.clientset.(*kubernetes.Clientset),
		wm.dynamicClient,
		wm.behaviorConfig.WatchNamespace,
		wm.kyvernoHandler,
	)

	// Start the Kyverno watcher
	if err := kyvernoWatcher.Start(wm.ctx); err != nil {
		log.Printf("❌ Failed to start Kyverno watcher: %v", err)
		return
	}

	// Start periodic collection of Kyverno events
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			log.Println("🛑 Kyverno watcher stopped")
			kyvernoWatcher.Stop()
			return
		case <-ticker.C:
			log.Println("🔍 Checking Kyverno for new policy violations...")

			// Collect Kyverno events
			events := wm.kyvernoHandler.GetRecentEvents()
			if len(events) > 0 {
				log.Printf("📊 Found %d Kyverno events", len(events))

				// Write events to CRDs
				if err := wm.crdWriter.WriteSecurityEvents(wm.ctx, wm.trivyHandler, wm.falcoHandler, wm.auditHandler, wm.kyvernoHandler); err != nil {
					log.Printf("❌ Failed to write Kyverno events: %v", err)
				}
			}
		}
	}
}

// startEventWriter starts the periodic event writer
func (wm *WatcherManager) startEventWriter() {
	ticker := time.NewTicker(5 * time.Minute) // Write events every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			log.Println("🛑 Event writer stopped")
			return
		case <-ticker.C:
			log.Println("📝 Writing collected events to CRDs...")

			// Write all collected events
			if err := wm.crdWriter.WriteSecurityEvents(wm.ctx, wm.trivyHandler, wm.falcoHandler, wm.auditHandler, wm.kyvernoHandler); err != nil {
				log.Printf("❌ Failed to write events: %v", err)
			}
		}
	}
}

// GetStatus returns the current status of all watchers
func (wm *WatcherManager) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"trivy": map[string]interface{}{
			"status":   getStatusString(wm.behaviorConfig.IsTrivyEnabled()),
			"category": "security",
			"events":   len(wm.trivyHandler.GetRecentEvents()),
		},
		"falco": map[string]interface{}{
			"status":   getStatusString(wm.behaviorConfig.IsFalcoEnabled()),
			"category": "security",
			"events":   len(wm.falcoHandler.GetRecentEvents()),
		},
		"audit": map[string]interface{}{
			"status":   getStatusString(wm.behaviorConfig.IsAuditEnabled()),
			"category": "compliance",
			"events":   len(wm.auditHandler.GetRecentEvents()),
		},
		"kyverno": map[string]interface{}{
			"status":   getStatusString(wm.behaviorConfig.IsKyvernoEnabled()),
			"category": "security",
			"events":   len(wm.kyvernoHandler.GetRecentEvents()),
		},
		"kube-bench": map[string]interface{}{
			"status":   getStatusString(wm.behaviorConfig.IsKubeBenchEnabled()),
			"category": "compliance",
			"events":   0, // Kube-bench doesn't have recent events yet
		},
		"behavior": map[string]interface{}{
			"mode":         wm.behaviorConfig.Mode,
			"enabledTools": wm.behaviorConfig.GetEnabledTools(),
		},
	}

	return status
}

// getStatusString converts boolean to status string
func getStatusString(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// startKubeBenchWatcher starts the Kube-bench watcher
func (wm *WatcherManager) startKubeBenchWatcher() {
	kubeBenchWatcher := watcher.NewKubeBenchWatcher(
		wm.clientset.(*kubernetes.Clientset),
		wm.behaviorConfig.WatchNamespace,
		wm.kubeBenchHandler,
	)

	if err := kubeBenchWatcher.Start(wm.ctx); err != nil {
		log.Printf("❌ Kube-bench watcher failed: %v", err)
	}
}
