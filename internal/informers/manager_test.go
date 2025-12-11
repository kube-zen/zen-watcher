// Copyright 2025 The Zen Watcher Authors
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

package informers

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestNewManager(t *testing.T) {
	dynamicClient := fake.NewSimpleDynamicClient(nil)

	config := Config{
		DynamicClient: dynamicClient,
		DefaultResync: 30 * time.Minute,
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.defaultResync != 30*time.Minute {
		t.Errorf("Expected defaultResync 30m, got %v", manager.defaultResync)
	}
}

func TestNewManager_DefaultResyncZero(t *testing.T) {
	dynamicClient := fake.NewSimpleDynamicClient(nil)

	config := Config{
		DynamicClient: dynamicClient,
		DefaultResync: 0, // Watch-only
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.defaultResync != 0 {
		t.Errorf("Expected defaultResync 0, got %v", manager.defaultResync)
	}
}

func TestGetInformer(t *testing.T) {
	dynamicClient := fake.NewSimpleDynamicClient(nil)

	config := Config{
		DynamicClient: dynamicClient,
		DefaultResync: 30 * time.Minute,
	}

	manager := NewManager(config)

	gvr := schema.GroupVersionResource{
		Group:    "test.kube-zen.io",
		Version:  "v1",
		Resource: "testresources",
	}

	informer := manager.GetInformer(gvr, 0)
	if informer == nil {
		t.Fatal("GetInformer returned nil")
	}

	// Verify it's a valid informer
	if informer.GetStore() == nil {
		t.Error("Informer store is nil")
	}
}

func TestGetInformer_CustomResync(t *testing.T) {
	dynamicClient := fake.NewSimpleDynamicClient(nil)

	config := Config{
		DynamicClient: dynamicClient,
		DefaultResync: 30 * time.Minute,
	}

	manager := NewManager(config)

	gvr := schema.GroupVersionResource{
		Group:    "test.kube-zen.io",
		Version:  "v1",
		Resource: "testresources",
	}

	// Request custom resync period
	customResync := 5 * time.Minute
	informer := manager.GetInformer(gvr, customResync)
	if informer == nil {
		t.Fatal("GetInformer returned nil")
	}

	// Verify it's a valid informer
	if informer.GetStore() == nil {
		t.Error("Informer store is nil")
	}
}

func TestStart(t *testing.T) {
	dynamicClient := fake.NewSimpleDynamicClient(nil)

	config := Config{
		DynamicClient: dynamicClient,
		DefaultResync: 0,
	}

	manager := NewManager(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should not panic
	manager.Start(ctx)

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)
}

func TestWaitForCacheSync(t *testing.T) {
	dynamicClient := fake.NewSimpleDynamicClient(nil)

	config := Config{
		DynamicClient: dynamicClient,
		DefaultResync: 0,
	}

	manager := NewManager(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)

	// WaitForCacheSync should complete quickly with fake client
	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 1*time.Second)
	defer cancelTimeout()

	err := manager.WaitForCacheSync(ctxTimeout)
	if err != nil {
		t.Errorf("WaitForCacheSync failed: %v", err)
	}
}
