package resources

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// DeliveryFlow represents a DeliveryFlow resource
type DeliveryFlow struct {
	Namespace       string
	Name            string
	ActiveTarget    string
	ActiveNamespace string
	Entitlement     string
	EntitlementReason string
	Ready           string
	Age             time.Duration
	FailoverCount   int64
	Sources         int64
	Outputs         int64
	Object          *unstructured.Unstructured
}

// Destination represents a Destination resource
type Destination struct {
	Namespace string
	Name      string
	Type      string
	Transport string
	Ready     string
	Health    string
	Age       time.Duration
	Object    *unstructured.Unstructured
}

// Ingester represents an Ingester resource
type Ingester struct {
	Namespace     string
	Name          string
	Sources       int64
	Destinations  int64
	Ready         string
	SourceHealth  string
	LastSeen      string
	Entitled      string
	Blocked       string
	Age           time.Duration
	Object        *unstructured.Unstructured
}

// ListDeliveryFlows lists DeliveryFlow resources
func ListDeliveryFlows(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, namespace string, allNamespaces bool) ([]DeliveryFlow, error) {
	var resourceInterface dynamic.ResourceInterface
	if allNamespaces || namespace == "" {
		resourceInterface = client.Resource(gvr)
	} else {
		resourceInterface = client.Resource(gvr).Namespace(namespace)
	}

	list, err := resourceInterface.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list DeliveryFlows: %w", err)
	}

	flows := make([]DeliveryFlow, 0, len(list.Items))
	for _, item := range list.Items {
		flow := parseDeliveryFlow(&item)
		flows = append(flows, flow)
	}

	return flows, nil
}

// GetDeliveryFlow gets a specific DeliveryFlow
func GetDeliveryFlow(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string) (*DeliveryFlow, error) {
	obj, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get DeliveryFlow: %w", err)
	}

	flow := parseDeliveryFlow(obj)
	return &flow, nil
}

func parseDeliveryFlow(obj *unstructured.Unstructured) DeliveryFlow {
	ns := obj.GetNamespace()
	name := obj.GetName()
	created := obj.GetCreationTimestamp().Time

	// Extract status.outputs[0].activeTarget
	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	var activeTarget, activeNamespace string
	if outputs, _, _ := unstructured.NestedSlice(status, "outputs"); len(outputs) > 0 {
		if output, ok := outputs[0].(map[string]interface{}); ok {
			if at, _, _ := unstructured.NestedMap(output, "activeTarget"); at != nil {
				if dr, _, _ := unstructured.NestedMap(at, "destinationRef"); dr != nil {
					activeTarget, _, _ = unstructured.NestedString(dr, "name")
					activeNamespace, _, _ = unstructured.NestedString(dr, "namespace")
				}
			}
		}
	}

	// Extract entitlement condition
	var entitlement, entitlementReason string
	if conditions, _, _ := unstructured.NestedSlice(status, "conditions"); conditions != nil {
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				if typ, _, _ := unstructured.NestedString(cond, "type"); typ == "Entitled" {
					entitlement, _, _ = unstructured.NestedString(cond, "status")
					entitlementReason, _, _ = unstructured.NestedString(cond, "reason")
					break
				}
			}
		}
	}

	// Extract Ready condition
	var ready string
	if conditions, _, _ := unstructured.NestedSlice(status, "conditions"); conditions != nil {
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				if typ, _, _ := unstructured.NestedString(cond, "type"); typ == "Ready" {
					ready, _, _ = unstructured.NestedString(cond, "status")
					break
				}
			}
		}
	}

	// Extract failover count (from first output if present)
	failoverCount := int64(0)
	if outputs, _, _ := unstructured.NestedSlice(status, "outputs"); len(outputs) > 0 {
		if output, ok := outputs[0].(map[string]interface{}); ok {
			failoverCount, _, _ = unstructured.NestedInt64(output, "failoverCount")
		}
	}

	// Count sources and outputs from spec
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	sourcesCount := int64(0)
	if sources, _, _ := unstructured.NestedSlice(spec, "sources"); sources != nil {
		sourcesCount = int64(len(sources))
	}
	outputsCount := int64(0)
	if outputs, _, _ := unstructured.NestedSlice(spec, "outputs"); outputs != nil {
		outputsCount = int64(len(outputs))
	}

	return DeliveryFlow{
		Namespace:        ns,
		Name:             name,
		ActiveTarget:     activeTarget,
		ActiveNamespace:  activeNamespace,
		Entitlement:      entitlement,
		EntitlementReason: entitlementReason,
		Ready:            ready,
		Age:              time.Since(created),
		FailoverCount:    failoverCount,
		Sources:          sourcesCount,
		Outputs:          outputsCount,
		Object:           obj,
	}
}

// ListDestinations lists Destination resources
func ListDestinations(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, namespace string, allNamespaces bool) ([]Destination, error) {
	var resourceInterface dynamic.ResourceInterface
	if allNamespaces || namespace == "" {
		resourceInterface = client.Resource(gvr)
	} else {
		resourceInterface = client.Resource(gvr).Namespace(namespace)
	}

	list, err := resourceInterface.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Destinations: %w", err)
	}

	dests := make([]Destination, 0, len(list.Items))
	for _, item := range list.Items {
		dest := parseDestination(&item)
		dests = append(dests, dest)
	}

	return dests, nil
}

func parseDestination(obj *unstructured.Unstructured) Destination {
	ns := obj.GetNamespace()
	name := obj.GetName()
	created := obj.GetCreationTimestamp().Time

	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	typ, _, _ := unstructured.NestedString(spec, "type")
	transport, _, _ := unstructured.NestedString(spec, "transport", "type")

	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	var ready string
	if conditions, _, _ := unstructured.NestedSlice(status, "conditions"); conditions != nil {
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				if condType, _, _ := unstructured.NestedString(cond, "type"); condType == "Ready" {
					ready, _, _ = unstructured.NestedString(cond, "status")
					break
				}
			}
		}
	}

	// Check for health condition
	health := ""
	if conditions, _, _ := unstructured.NestedSlice(status, "conditions"); conditions != nil {
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				if condType, _, _ := unstructured.NestedString(cond, "type"); condType == "Healthy" {
					health, _, _ = unstructured.NestedString(cond, "status")
					break
				}
			}
		}
	}

	return Destination{
		Namespace: ns,
		Name:      name,
		Type:      typ,
		Transport: transport,
		Ready:     ready,
		Health:    health,
		Age:       time.Since(created),
		Object:    obj,
	}
}

// ListIngesters lists Ingester resources
func ListIngesters(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, namespace string, allNamespaces bool) ([]Ingester, error) {
	var resourceInterface dynamic.ResourceInterface
	if allNamespaces || namespace == "" {
		resourceInterface = client.Resource(gvr)
	} else {
		resourceInterface = client.Resource(gvr).Namespace(namespace)
	}

	list, err := resourceInterface.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Ingesters: %w", err)
	}

	ingesters := make([]Ingester, 0, len(list.Items))
	for _, item := range list.Items {
		ing := parseIngester(&item)
		ingesters = append(ingesters, ing)
	}

	return ingesters, nil
}

func parseIngester(obj *unstructured.Unstructured) Ingester {
	ns := obj.GetNamespace()
	name := obj.GetName()
	created := obj.GetCreationTimestamp().Time

	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	sourcesCount := int64(1) // Default for legacy single-source
	if sources, _, _ := unstructured.NestedSlice(spec, "sources"); sources != nil {
		sourcesCount = int64(len(sources))
	}
	destinationsCount := int64(0)
	if destinations, _, _ := unstructured.NestedSlice(spec, "destinations"); destinations != nil {
		destinationsCount = int64(len(destinations))
	}

	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	var ready string
	if conditions, _, _ := unstructured.NestedSlice(status, "conditions"); conditions != nil {
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				if condType, _, _ := unstructured.NestedString(cond, "type"); condType == "Ready" {
					ready, _, _ = unstructured.NestedString(cond, "status")
					break
				}
			}
		}
	}

	// Extract source health/lastSeen from status.sources
	sourceHealth := ""
	lastSeen := ""
	if sources, _, _ := unstructured.NestedSlice(status, "sources"); sources != nil {
		var states []string
		var lastSeens []string
		for _, s := range sources {
			if src, ok := s.(map[string]interface{}); ok {
				if state, _, _ := unstructured.NestedString(src, "state"); state != "" {
					states = append(states, state)
				}
				if ls, _, _ := unstructured.NestedString(src, "lastSeen"); ls != "" {
					lastSeens = append(lastSeens, ls)
				}
			}
		}
		if len(states) > 0 {
			sourceHealth = states[0] // Use first source's state as summary
		}
		if len(lastSeens) > 0 {
			lastSeen = lastSeens[0]
		}
	}

	// Check for entitled/blocked conditions
	entitled := ""
	blocked := ""
	if conditions, _, _ := unstructured.NestedSlice(status, "conditions"); conditions != nil {
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				if condType, _, _ := unstructured.NestedString(cond, "type"); condType == "Entitled" {
					entitled, _, _ = unstructured.NestedString(cond, "status")
				}
				if condType, _, _ := unstructured.NestedString(cond, "type"); condType == "Blocked" {
					blocked, _, _ = unstructured.NestedString(cond, "status")
				}
			}
		}
	}

	return Ingester{
		Namespace:    ns,
		Name:         name,
		Sources:      sourcesCount,
		Destinations: destinationsCount,
		Ready:        ready,
		SourceHealth: sourceHealth,
		LastSeen:     lastSeen,
		Entitled:     entitled,
		Blocked:      blocked,
		Age:          time.Since(created),
		Object:       obj,
	}
}

