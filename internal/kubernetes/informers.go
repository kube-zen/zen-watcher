package kubernetes

import (
	"context"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// EventProcessor interface for processing events (to avoid import cycle)
type EventProcessor interface {
	ProcessKyvernoPolicyReport(ctx context.Context, report *unstructured.Unstructured)
	ProcessTrivyVulnerabilityReport(ctx context.Context, report *unstructured.Unstructured)
}

// SetupInformers configures and starts informers for Kyverno and Trivy
func SetupInformers(
	ctx context.Context,
	factory dynamicinformer.DynamicSharedInformerFactory,
	gvrs *GVRs,
	eventProcessor EventProcessor,
	stopCh chan struct{},
) error {
	logger.Info("Starting informers",
		logger.Fields{
			Component: "kubernetes",
			Operation: "informers_start",
		})

	// Setup Kyverno PolicyReport informer
	policyInformer := factory.ForResource(gvrs.PolicyReport).Informer()
	policyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			report, ok := obj.(*unstructured.Unstructured)
			if !ok {
				logger.Warn("Invalid object type in Kyverno PolicyReport AddFunc",
					logger.Fields{
						Component: "kubernetes",
						Operation: "informer_add",
						Source:    "kyverno",
					})
				return
			}
			logger.Debug("PolicyReport added",
				logger.Fields{
					Component:    "kubernetes",
					Operation:    "informer_add",
					Source:       "kyverno",
					ResourceKind: "PolicyReport",
					Namespace:    report.GetNamespace(),
					ResourceName: report.GetName(),
				})
			eventProcessor.ProcessKyvernoPolicyReport(ctx, report)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			report, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				logger.Warn("Invalid object type in Kyverno PolicyReport UpdateFunc",
					logger.Fields{
						Component: "kubernetes",
						Operation: "informer_update",
						Source:    "kyverno",
					})
				return
			}
			logger.Debug("PolicyReport updated",
				logger.Fields{
					Component:    "kubernetes",
					Operation:    "informer_update",
					Source:       "kyverno",
					ResourceKind: "PolicyReport",
					Namespace:    report.GetNamespace(),
					ResourceName: report.GetName(),
				})
			eventProcessor.ProcessKyvernoPolicyReport(ctx, report)
		},
	})

	// Setup Trivy VulnerabilityReport informer
	trivyInformer := factory.ForResource(gvrs.TrivyReport).Informer()
	trivyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			report, ok := obj.(*unstructured.Unstructured)
			if !ok {
				logger.Warn("Invalid object type in Trivy VulnerabilityReport AddFunc",
					logger.Fields{
						Component: "kubernetes",
						Operation: "informer_add",
						Source:    "trivy",
					})
				return
			}
			logger.Debug("VulnerabilityReport added",
				logger.Fields{
					Component:    "kubernetes",
					Operation:    "informer_add",
					Source:       "trivy",
					ResourceKind: "VulnerabilityReport",
					Namespace:    report.GetNamespace(),
					ResourceName: report.GetName(),
				})
			eventProcessor.ProcessTrivyVulnerabilityReport(ctx, report)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			report, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				logger.Warn("Invalid object type in Trivy VulnerabilityReport UpdateFunc",
					logger.Fields{
						Component: "kubernetes",
						Operation: "informer_update",
						Source:    "trivy",
					})
				return
			}
			logger.Debug("VulnerabilityReport updated",
				logger.Fields{
					Component:    "kubernetes",
					Operation:    "informer_update",
					Source:       "trivy",
					ResourceKind: "VulnerabilityReport",
					Namespace:    report.GetNamespace(),
					ResourceName: report.GetName(),
				})
			eventProcessor.ProcessTrivyVulnerabilityReport(ctx, report)
		},
	})

	// Start informers
	factory.Start(stopCh)

	// Wait for caches to sync (with timeout to avoid blocking if CRDs don't exist)
	syncCtx, syncCancel := context.WithTimeout(ctx, 30*time.Second)
	defer syncCancel()

	if !cache.WaitForCacheSync(syncCtx.Done(), policyInformer.HasSynced, trivyInformer.HasSynced) {
		logger.Warn("Informer caches did not sync within timeout (CRDs may not be installed - this is OK)",
			logger.Fields{
				Component: "kubernetes",
				Operation: "informers_sync",
				Reason:    "timeout",
			})
		logger.Info("Informers will continue running and will sync when CRDs become available",
			logger.Fields{
				Component: "kubernetes",
				Operation: "informers_sync",
			})
	} else {
		logger.Info("Informers started and synced - real-time event processing enabled",
			logger.Fields{
				Component: "kubernetes",
				Operation: "informers_sync",
			})
	}

	return nil
}
