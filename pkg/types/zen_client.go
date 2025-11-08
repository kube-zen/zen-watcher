package types

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

// ZenClient handles CRUD operations for ZenEvent CRDs
type ZenClient struct {
	dynamicClient dynamic.Interface
	eventGVR      schema.GroupVersionResource
}

// NewZenClient creates a new client for Zen CRDs
func NewZenClient(dynamicClient dynamic.Interface) *ZenClient {
	return &ZenClient{
		dynamicClient: dynamicClient,
		eventGVR: schema.GroupVersionResource{
			Group:    "zen.kube-zen.com",
			Version:  "v1",
			Resource: "zenevents",
		},
	}
}

// CreateZenEvent creates a new ZenEvent
func (c *ZenClient) CreateZenEvent(ctx context.Context, event *ZenEvent) (*ZenEvent, error) {
	// Set default values
	if event.Status.Phase == "" {
		event.Status.Phase = PhaseActive
	}

	// Convert to unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(event)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %v", err)
	}

	// Create the resource
	created, err := c.dynamicClient.Resource(c.eventGVR).Namespace(event.Namespace).Create(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1."CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create ZenEvent: %v", err)
	}

	// Convert back to typed object
	var result ZenEvent
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(created.Object, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to convert from unstructured: %v", err)
	}

	return &result, nil
}

// GetZenEvent retrieves a ZenEvent by name
func (c *ZenClient) GetZenEvent(ctx context.Context, name, namespace string) (*ZenEvent, error) {
	obj, err := c.dynamicClient.Resource(c.eventGVR).Namespace(namespace).Get(ctx, name, metav1."GetOptions{})
	if err != nil {
		return nil, err
	}

	var event ZenEvent
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &event)
	if err != nil {
		return nil, fmt.Errorf("failed to convert from unstructured: %v", err)
	}

	return &event, nil
}

// ListZenEvents lists all ZenEvents in a namespace
func (c *ZenClient) ListZenEvents(ctx context.Context, namespace string) (*ZenEventList, error) {
	list, err := c.dynamicClient.Resource(c.eventGVR).Namespace(namespace).List(ctx, metav1."ListOptions{})
	if err != nil {
		return nil, err
	}

	var events []ZenEvent
	for _, item := range list.Items {
		var event ZenEvent
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &event)
		if err != nil {
			continue // Skip invalid items
		}
		events = append(events, event)
	}

	return &ZenEventList{
		TypeMeta: metav1."TypeMeta{
			APIVersion: "zen.kube-zen.com/v1",
			Kind:       "ZenEventList",
		},
		ListMeta: metav1."ListMeta{
			ResourceVersion: list.GetResourceVersion(),
		},
		Items: events,
	}, nil
}

// UpdateZenEvent updates a ZenEvent
func (c *ZenClient) UpdateZenEvent(ctx context.Context, event *ZenEvent) (*ZenEvent, error) {
	// Convert to unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(event)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %v", err)
	}

	// Update the resource
	updated, err := c.dynamicClient.Resource(c.eventGVR).Namespace(event.Namespace).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1."UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update ZenEvent: %v", err)
	}

	// Convert back to typed object
	var result ZenEvent
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(updated.Object, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to convert from unstructured: %v", err)
	}

	return &result, nil
}

// DeleteZenEvent deletes a ZenEvent
func (c *ZenClient) DeleteZenEvent(ctx context.Context, name, namespace string) error {
	return c.dynamicClient.Resource(c.eventGVR).Namespace(namespace).Delete(ctx, name, metav1."DeleteOptions{})
}

// WatchZenEvents watches for changes to ZenEvents
func (c *ZenClient) WatchZenEvents(ctx context.Context, namespace string, options metav1."ListOptions) (watch.Interface, error) {
	return c.dynamicClient.Resource(c.eventGVR).Namespace(namespace).Watch(ctx, options)
}
