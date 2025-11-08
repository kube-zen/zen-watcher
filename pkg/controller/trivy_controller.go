package controller

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// TrivyController handles watching Trivy ConfigMaps and triggering actions
type TrivyController struct {
	clientSet     kubernetes.Interface
	actionHandler ActionHandler
	workqueue     workqueue.RateLimitingInterface
	informer      cache.SharedIndexInformer
	namespace     string
	resourceType  string
}

// ActionHandler interface for handling detected events
type ActionHandler interface {
	HandleTrivyConfigMap(ctx context.Context, configMap *corev1."ConfigMap) error
}

// NewTrivyController creates a new Trivy controller
func NewTrivyController(
	clientSet kubernetes.Interface,
	actionHandler ActionHandler,
	namespace string,
	resourceType string,
) *TrivyController {
	// Create workqueue
	workqueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Create informer for ConfigMaps in the namespace
	listWatcher := cache.NewListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"configmaps",
		namespace,
		fields.Everything(),
	)

	informer := cache.NewSharedIndexInformer(
		listWatcher,
		&corev1."ConfigMap{},
		time.Second*30,
		cache.Indexers{},
	)

	controller := &TrivyController{
		clientSet:     clientSet,
		actionHandler: actionHandler,
		workqueue:     workqueue,
		informer:      informer,
		namespace:     namespace,
		resourceType:  resourceType,
	}

	// Add event handlers with filtering
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if controller.isTrivyConfigMap(obj) {
				controller.enqueueWork("ADD", obj)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if controller.isTrivyConfigMap(newObj) {
				controller.enqueueWork("UPDATE", newObj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if controller.isTrivyConfigMap(obj) {
				controller.enqueueWork("DELETE", obj)
			}
		},
	})

	return controller
}

// isTrivyConfigMap checks if a ConfigMap is a Trivy report
func (tc *TrivyController) isTrivyConfigMap(obj interface{}) bool {
	if configMap, ok := obj.(*corev1."ConfigMap); ok {
		if configMap.Labels != nil {
			// Check if it has Trivy operator labels
			if kind, exists := configMap.Labels["trivy-operator.resource.kind"]; exists {
				return kind == tc.resourceType
			}
		}
	}
	return false
}

// setupEventHandlers configures the event handlers for the informer
func (tc *TrivyController) setupEventHandlers() {
	tc.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			tc.enqueueWork("ADD", obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			tc.enqueueWork("UPDATE", newObj)
		},
		DeleteFunc: func(obj interface{}) {
			tc.enqueueWork("DELETE", obj)
		},
	})
}

// enqueueWork adds work items to the queue
func (tc *TrivyController) enqueueWork(eventType string, obj interface{}) {
	var key string
	var err error

	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		log.Printf("Error getting key for object: %v", err)
		return
	}

	// Add event type to the key for processing
	workItem := fmt.Sprintf("%s:%s", eventType, key)
	tc.workqueue.Add(workItem)
}

// Run starts the controller
func (tc *TrivyController) Run(ctx context.Context, workers int) error {
	defer tc.workqueue.ShutDown()

	// Start the informer
	go tc.informer.Run(ctx.Done())

	// Wait for cache to sync
	if !cache.WaitForCacheSync(ctx.Done(), tc.informer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	log.Printf("🚀 Starting Trivy controller for %s in namespace %s", tc.resourceType, tc.namespace)

	// Start workers
	for i := 0; i < workers; i++ {
		go tc.runWorker(ctx)
	}

	<-ctx.Done()
	log.Printf("🛑 Trivy controller stopped")
	return nil
}

// runWorker processes work items from the queue
func (tc *TrivyController) runWorker(ctx context.Context) {
	for tc.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes the next work item in the queue
func (tc *TrivyController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := tc.workqueue.Get()
	if shutdown {
		return false
	}

	defer tc.workqueue.Done(obj)

	if err := tc.processWorkItem(ctx, obj); err != nil {
		log.Printf("Error processing work item: %v", err)
		tc.workqueue.AddRateLimited(obj)
		return true
	}

	tc.workqueue.Forget(obj)
	return true
}

// processWorkItem processes a single work item
func (tc *TrivyController) processWorkItem(ctx context.Context, obj interface{}) error {
	workItem := obj.(string)

	// Parse event type and key
	parts := strings.SplitN(workItem, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid work item format: %s", workItem)
	}

	eventType := parts[0]
	key := parts[1]

	// Get the object from the cache
	obj, exists, err := tc.informer.GetIndexer().GetByKey(key)
	if err != nil {
		return fmt.Errorf("error getting object %s: %v", key, err)
	}

	if !exists {
		log.Printf("Object %s no longer exists", key)
		return nil
	}

	// Process based on event type
	switch eventType {
	case "ADD":
		return tc.handleAdd(ctx, obj)
	case "UPDATE":
		return tc.handleUpdate(ctx, obj)
	case "DELETE":
		return tc.handleDelete(ctx, obj)
	default:
		return fmt.Errorf("unknown event type: %s", eventType)
	}
}

// handleAdd processes ADD events
func (tc *TrivyController) handleAdd(ctx context.Context, obj interface{}) error {
	log.Printf("🆕 New %s created", tc.resourceType)
	return tc.processObject(ctx, obj, "ADD")
}

// handleUpdate processes UPDATE events
func (tc *TrivyController) handleUpdate(ctx context.Context, obj interface{}) error {
	log.Printf("🔄 %s updated", tc.resourceType)
	return tc.processObject(ctx, obj, "UPDATE")
}

// handleDelete processes DELETE events
func (tc *TrivyController) handleDelete(ctx context.Context, obj interface{}) error {
	log.Printf("🗑️ %s deleted", tc.resourceType)
	return tc.processObject(ctx, obj, "DELETE")
}

// processObject processes the object based on its type
func (tc *TrivyController) processObject(ctx context.Context, obj interface{}, eventType string) error {
	if configMap, ok := obj.(*corev1."ConfigMap); ok {
		return tc.actionHandler.HandleTrivyConfigMap(ctx, configMap)
	}

	return fmt.Errorf("unknown object type for processing")
}
