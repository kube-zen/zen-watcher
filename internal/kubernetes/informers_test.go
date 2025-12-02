// Copyright 2024 The Zen Watcher Authors
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

package kubernetes

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// mockEventProcessor is a test implementation of EventProcessor
type mockEventProcessor struct {
	kyvernoEvents []*unstructured.Unstructured
	trivyEvents   []*unstructured.Unstructured
}

func (m *mockEventProcessor) ProcessKyvernoPolicyReport(ctx context.Context, report *unstructured.Unstructured) {
	m.kyvernoEvents = append(m.kyvernoEvents, report)
}

func (m *mockEventProcessor) ProcessTrivyVulnerabilityReport(ctx context.Context, report *unstructured.Unstructured) {
	m.trivyEvents = append(m.trivyEvents, report)
}

func TestSetupInformers_KyvernoPolicyReport(t *testing.T) {
	scheme := runtime.NewScheme()
	
	// Create GVRs
	gvrs := &GVRs{
		PolicyReport: schema.GroupVersionResource{
			Group:    "wgpolicyk8s.io",
			Version:  "v1alpha2",
			Resource: "policyreports",
		},
		TrivyReport: schema.GroupVersionResource{
			Group:    "aquasecurity.github.io",
			Version:  "v1alpha1",
			Resource: "vulnerabilityreports",
		},
	}

	// Create fake dynamic client
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	
	// Create informer factory
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, time.Second)

	// Create mock event processor
	mockProcessor := &mockEventProcessor{
		kyvernoEvents: make([]*unstructured.Unstructured, 0),
		trivyEvents:   make([]*unstructured.Unstructured, 0),
	}

	// Setup informers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopCh := make(chan struct{})

	err := SetupInformers(ctx, factory, gvrs, mockProcessor, stopCh)
	if err != nil {
		t.Fatalf("SetupInformers() error = %v", err)
	}

	// Start informer factory
	factory.Start(ctx.Done())

	// Wait for cache sync
	policyInformer := factory.ForResource(gvrs.PolicyReport).Informer()
	if !meta.WaitForCacheSync(ctx.Done(), policyInformer.HasSynced) {
		t.Fatal("Failed to sync informer cache")
	}

	// Create a test PolicyReport
	policyReport := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "wgpolicyk8s.io/v1alpha2",
			"kind":       "PolicyReport",
			"metadata": map[string]interface{}{
				"name":      "test-policy-report",
				"namespace": "default",
			},
			"summary": map[string]interface{}{
				"fail": 1,
			},
		},
	}

	// Create the resource via dynamic client
	_, err = dynamicClient.Resource(gvrs.PolicyReport).Namespace("default").Create(
		ctx, policyReport, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PolicyReport: %v", err)
	}

	// Give informer time to process
	time.Sleep(100 * time.Millisecond)

	// Verify event was processed
	if len(mockProcessor.kyvernoEvents) == 0 {
		t.Error("Expected PolicyReport event to be processed, got 0 events")
	}

	if len(mockProcessor.kyvernoEvents) > 0 {
		processed := mockProcessor.kyvernoEvents[0]
		if processed.GetName() != "test-policy-report" {
			t.Errorf("Expected name 'test-policy-report', got '%s'", processed.GetName())
		}
		if processed.GetNamespace() != "default" {
			t.Errorf("Expected namespace 'default', got '%s'", processed.GetNamespace())
		}
	}
}

func TestSetupInformers_TrivyVulnerabilityReport(t *testing.T) {
	scheme := runtime.NewScheme()
	
	gvrs := &GVRs{
		PolicyReport: schema.GroupVersionResource{
			Group:    "wgpolicyk8s.io",
			Version:  "v1alpha2",
			Resource: "policyreports",
		},
		TrivyReport: schema.GroupVersionResource{
			Group:    "aquasecurity.github.io",
			Version:  "v1alpha1",
			Resource: "vulnerabilityreports",
		},
	}

	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, time.Second)

	mockProcessor := &mockEventProcessor{
		kyvernoEvents: make([]*unstructured.Unstructured, 0),
		trivyEvents:   make([]*unstructured.Unstructured, 0),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopCh := make(chan struct{})

	err := SetupInformers(ctx, factory, gvrs, mockProcessor, stopCh)
	if err != nil {
		t.Fatalf("SetupInformers() error = %v", err)
	}

	factory.Start(ctx.Done())

	trivyInformer := factory.ForResource(gvrs.TrivyReport).Informer()
	if !meta.WaitForCacheSync(ctx.Done(), trivyInformer.HasSynced) {
		t.Fatal("Failed to sync informer cache")
	}

	// Create a test VulnerabilityReport
	vulnReport := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "VulnerabilityReport",
			"metadata": map[string]interface{}{
				"name":      "test-vuln-report",
				"namespace": "default",
			},
			"report": map[string]interface{}{
				"summary": map[string]interface{}{
					"criticalCount": 1,
				},
			},
		},
	}

	_, err = dynamicClient.Resource(gvrs.TrivyReport).Namespace("default").Create(
		ctx, vulnReport, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create VulnerabilityReport: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if len(mockProcessor.trivyEvents) == 0 {
		t.Error("Expected VulnerabilityReport event to be processed, got 0 events")
	}

	if len(mockProcessor.trivyEvents) > 0 {
		processed := mockProcessor.trivyEvents[0]
		if processed.GetName() != "test-vuln-report" {
			t.Errorf("Expected name 'test-vuln-report', got '%s'", processed.GetName())
		}
	}
}

func TestSetupInformers_UpdateEvent(t *testing.T) {
	scheme := runtime.NewScheme()
	
	gvrs := &GVRs{
		PolicyReport: schema.GroupVersionResource{
			Group:    "wgpolicyk8s.io",
			Version:  "v1alpha2",
			Resource: "policyreports",
		},
		TrivyReport: schema.GroupVersionResource{
			Group:    "aquasecurity.github.io",
			Version:  "v1alpha1",
			Resource: "vulnerabilityreports",
		},
	}

	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, time.Second)

	mockProcessor := &mockEventProcessor{
		kyvernoEvents: make([]*unstructured.Unstructured, 0),
		trivyEvents:   make([]*unstructured.Unstructured, 0),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopCh := make(chan struct{})

	err := SetupInformers(ctx, factory, gvrs, mockProcessor, stopCh)
	if err != nil {
		t.Fatalf("SetupInformers() error = %v", err)
	}

	factory.Start(ctx.Done())

	policyInformer := factory.ForResource(gvrs.PolicyReport).Informer()
	if !meta.WaitForCacheSync(ctx.Done(), policyInformer.HasSynced) {
		t.Fatal("Failed to sync informer cache")
	}

	// Create initial PolicyReport
	policyReport := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "wgpolicyk8s.io/v1alpha2",
			"kind":       "PolicyReport",
			"metadata": map[string]interface{}{
				"name":      "test-policy-report",
				"namespace": "default",
			},
			"summary": map[string]interface{}{
				"fail": 1,
			},
		},
	}

	created, err := dynamicClient.Resource(gvrs.PolicyReport).Namespace("default").Create(
		ctx, policyReport, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PolicyReport: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	initialCount := len(mockProcessor.kyvernoEvents)

	// Update the PolicyReport
	created.Object["summary"] = map[string]interface{}{
		"fail": 2,
	}
	_, err = dynamicClient.Resource(gvrs.PolicyReport).Namespace("default").Update(
		ctx, created, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update PolicyReport: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify update event was processed
	if len(mockProcessor.kyvernoEvents) <= initialCount {
		t.Error("Expected update event to be processed")
	}
}

