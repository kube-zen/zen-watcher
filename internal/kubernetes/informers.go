package kubernetes

import (
	"context"
	"fmt"
	"log"

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
	log.Println("🚀 Starting informers...")

	// Setup Kyverno PolicyReport informer
	policyInformer := factory.ForResource(gvrs.PolicyReport).Informer()
	policyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			report, ok := obj.(*unstructured.Unstructured)
			if !ok {
				log.Printf("⚠️  Invalid object type in Kyverno PolicyReport AddFunc")
				return
			}
			log.Printf("📊 [KYVERNO] PolicyReport added: %s/%s", report.GetNamespace(), report.GetName())
			eventProcessor.ProcessKyvernoPolicyReport(ctx, report)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			report, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				log.Printf("⚠️  Invalid object type in Kyverno PolicyReport UpdateFunc")
				return
			}
			log.Printf("📊 [KYVERNO] PolicyReport updated: %s/%s", report.GetNamespace(), report.GetName())
			eventProcessor.ProcessKyvernoPolicyReport(ctx, report)
		},
	})

	// Setup Trivy VulnerabilityReport informer
	trivyInformer := factory.ForResource(gvrs.TrivyReport).Informer()
	trivyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			report, ok := obj.(*unstructured.Unstructured)
			if !ok {
				log.Printf("⚠️  Invalid object type in Trivy VulnerabilityReport AddFunc")
				return
			}
			log.Printf("🔍 [TRIVY] VulnerabilityReport added: %s/%s", report.GetNamespace(), report.GetName())
			eventProcessor.ProcessTrivyVulnerabilityReport(ctx, report)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			report, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				log.Printf("⚠️  Invalid object type in Trivy VulnerabilityReport UpdateFunc")
				return
			}
			log.Printf("🔍 [TRIVY] VulnerabilityReport updated: %s/%s", report.GetNamespace(), report.GetName())
			eventProcessor.ProcessTrivyVulnerabilityReport(ctx, report)
		},
	})

	// Start informers
	factory.Start(stopCh)

	// Wait for caches to sync
	if !cache.WaitForCacheSync(ctx.Done(), policyInformer.HasSynced, trivyInformer.HasSynced) {
		return fmt.Errorf("failed to sync informer caches")
	}

	log.Println("✅ Informers started and synced - real-time event processing enabled")
	return nil
}
