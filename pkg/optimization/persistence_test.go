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

package optimization

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/config"
)

func TestFileStatePersistence_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	persistence, err := NewFileStatePersistence(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	source := "test-source"
	state := &OptimizationState{
		Source:          source,
		CurrentStrategy: "dedup_first",
		LastDecision:    time.Now(),
		DecisionHistory: []OptimizationDecision{
			{
				Type:       "strategy_change",
				NewValue:   "dedup_first",
				Confidence: 0.85,
				Reason:     "High dedup effectiveness",
			},
		},
		ActiveRules: []config.DynamicFilterRule{
			{
				Condition: "severity == 'low'",
				Action:    "filter",
			},
		},
		Metrics: map[string]float64{
			"cpu_usage": 75.5,
		},
		LastUpdated: time.Now(),
	}

	// Save state
	err = persistence.Save(source, state)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Verify file exists
	filename := filepath.Join(tmpDir, source+".json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("State file was not created: %v", err)
	}

	// Verify temp file doesn't exist (atomic write)
	tmpFile := filepath.Join(tmpDir, "."+source+".tmp")
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("Temp file should not exist after atomic rename")
	}

	// Load state
	loadedState, err := persistence.Load(source)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if loadedState == nil {
		t.Fatal("Loaded state is nil")
	}

	if loadedState.Source != source {
		t.Errorf("Expected source '%s', got '%s'", source, loadedState.Source)
	}

	if loadedState.CurrentStrategy != "dedup_first" {
		t.Errorf("Expected strategy 'dedup_first', got '%s'", loadedState.CurrentStrategy)
	}

	if len(loadedState.DecisionHistory) != 1 {
		t.Errorf("Expected 1 decision in history, got %d", len(loadedState.DecisionHistory))
	}

	if len(loadedState.ActiveRules) != 1 {
		t.Errorf("Expected 1 active rule, got %d", len(loadedState.ActiveRules))
	}

	if loadedState.Metrics["cpu_usage"] != 75.5 {
		t.Errorf("Expected cpu_usage 75.5, got %f", loadedState.Metrics["cpu_usage"])
	}
}

func TestFileStatePersistence_Load_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	persistence, err := NewFileStatePersistence(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Load non-existent state
	state, err := persistence.Load("non-existent")
	if err != nil {
		t.Fatalf("Unexpected error loading non-existent state: %v", err)
	}

	if state != nil {
		t.Error("Expected nil state for non-existent source")
	}
}

func TestFileStatePersistence_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	persistence, err := NewFileStatePersistence(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	source := "test-source"
	state := &OptimizationState{
		Source:          source,
		CurrentStrategy: "filter_first",
		LastUpdated:     time.Now(),
	}

	// Save state
	err = persistence.Save(source, state)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Verify file exists
	filename := filepath.Join(tmpDir, source+".json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatal("State file should exist before delete")
	}

	// Delete state
	err = persistence.Delete(source)
	if err != nil {
		t.Fatalf("Failed to delete state: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Error("State file should not exist after delete")
	}
}

func TestFileStatePersistence_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	persistence, err := NewFileStatePersistence(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	source := "test-source"
	state := &OptimizationState{
		Source:          source,
		CurrentStrategy: "filter_first",
		LastUpdated:     time.Now(),
	}

	// Save should use atomic write (temp file then rename)
	err = persistence.Save(source, state)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Final file should exist
	filename := filepath.Join(tmpDir, source+".json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatal("State file should exist after save")
	}

	// Temp file should not exist (atomic rename completed)
	tmpFile := filepath.Join(tmpDir, "."+source+".tmp")
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("Temp file should not exist after atomic rename")
	}
}
