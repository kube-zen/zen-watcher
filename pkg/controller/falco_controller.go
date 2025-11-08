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

// FalcoController handles watching Falco security events and triggering actions
type FalcoController struct {
	clientSet     kubernetes.Interface
	actionHandler FalcoActionHandler
	workqueue     workqueue.RateLimitingInterface
	informer      cache.SharedIndexInformer
	namespace     string
}

// FalcoActionHandler interface for handling Falco security events
type FalcoActionHandler interface {
	HandleFalcoEvent(ctx context.Context, event *FalcoSecurityEvent) error
}

// FalcoSecurityEvent represents a Falco security event
type FalcoSecurityEvent struct {
	Timestamp   string
	Rule        string
	Priority    string
	Source      string
	Message     string
	Namespace   string
	PodName     string
	ContainerID string
	RawEvent    string
}

// NewFalcoController creates a new Falco controller
func NewFalcoController(
	clientSet kubernetes.Interface,
	actionHandler FalcoActionHandler,
	namespace string,
) *FalcoController {
	// Create workqueue
	workqueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// Create informer for Pods in the Falco namespace
	listWatcher := cache.NewListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"pods",
		namespace,
		fields.Everything(),
	)

	informer := cache.NewSharedIndexInformer(
		listWatcher,
		&corev1."Pod{},
		time.Second*30,
		cache.Indexers{},
	)

	controller := &FalcoController{
		clientSet:     clientSet,
		actionHandler: actionHandler,
		workqueue:     workqueue,
		informer:      informer,
		namespace:     namespace,
	}

	// Add event handlers for Falco pods
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if controller.isFalcoPod(obj) {
				controller.enqueueWork("ADD", obj)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if controller.isFalcoPod(newObj) {
				controller.enqueueWork("UPDATE", newObj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if controller.isFalcoPod(obj) {
				controller.enqueueWork("DELETE", obj)
			}
		},
	})

	return controller
}

// isFalcoPod checks if a pod is a Falco pod
func (fc *FalcoController) isFalcoPod(obj interface{}) bool {
	if pod, ok := obj.(*corev1."Pod); ok {
		if pod.Labels != nil {
			// Check if it's a Falco pod
			if name, exists := pod.Labels["app.kubernetes.io/name"]; exists {
				return name == "falco"
			}
		}
	}
	return false
}

// enqueueWork adds work items to the queue
func (fc *FalcoController) enqueueWork(eventType string, obj interface{}) {
	var key string
	var err error

	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		log.Printf("Error getting key for object: %v", err)
		return
	}

	// Add event type to the key for processing
	workItem := fmt.Sprintf("%s:%s", eventType, key)
	fc.workqueue.Add(workItem)
}

// Run starts the controller
func (fc *FalcoController) Run(ctx context.Context, workers int) error {
	defer fc.workqueue.ShutDown()

	// Start the informer
	go fc.informer.Run(ctx.Done())

	// Wait for cache to sync
	if !cache.WaitForCacheSync(ctx.Done(), fc.informer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	log.Printf("🚀 Starting Falco controller in namespace %s", fc.namespace)

	// Start workers
	for i := 0; i < workers; i++ {
		go fc.runWorker(ctx)
	}

	<-ctx.Done()
	log.Printf("🛑 Falco controller stopped")
	return nil
}

// runWorker processes work items from the queue
func (fc *FalcoController) runWorker(ctx context.Context) {
	for fc.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes the next work item in the queue
func (fc *FalcoController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := fc.workqueue.Get()
	if shutdown {
		return false
	}

	defer fc.workqueue.Done(obj)

	if err := fc.processWorkItem(ctx, obj); err != nil {
		log.Printf("Error processing work item: %v", err)
		fc.workqueue.AddRateLimited(obj)
		return true
	}

	fc.workqueue.Forget(obj)
	return true
}

// processWorkItem processes a single work item
func (fc *FalcoController) processWorkItem(ctx context.Context, obj interface{}) error {
	workItem := obj.(string)

	// Parse event type and key
	parts := strings.SplitN(workItem, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid work item format: %s", workItem)
	}

	eventType := parts[0]
	key := parts[1]

	// Get the object from the cache
	obj, exists, err := fc.informer.GetIndexer().GetByKey(key)
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
		return fc.handleAdd(ctx, obj)
	case "UPDATE":
		return fc.handleUpdate(ctx, obj)
	case "DELETE":
		return fc.handleDelete(ctx, obj)
	default:
		return fmt.Errorf("unknown event type: %s", eventType)
	}
}

// handleAdd processes ADD events
func (fc *FalcoController) handleAdd(ctx context.Context, obj interface{}) error {
	log.Printf("🆕 New Falco pod detected")
	return fc.processFalcoPod(ctx, obj, "ADD")
}

// handleUpdate processes UPDATE events
func (fc *FalcoController) handleUpdate(ctx context.Context, obj interface{}) error {
	log.Printf("🔄 Falco pod updated")
	return fc.processFalcoPod(ctx, obj, "UPDATE")
}

// handleDelete processes DELETE events
func (fc *FalcoController) handleDelete(ctx context.Context, obj interface{}) error {
	log.Printf("🗑️ Falco pod deleted")
	return fc.processFalcoPod(ctx, obj, "DELETE")
}

// processFalcoPod processes Falco pod events
func (fc *FalcoController) processFalcoPod(ctx context.Context, obj interface{}, eventType string) error {
	if pod, ok := obj.(*corev1."Pod); ok {
		log.Printf("📊 Falco pod %s in namespace %s: %s", pod.Name, pod.Namespace, eventType)
		// Here we could trigger additional actions based on pod state changes
	}
	return nil
}
