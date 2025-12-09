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
	"testing"
	"time"
)

func TestPerformanceTracker_GetAverageLatency_Empty(t *testing.T) {
	pt := NewPerformanceTracker("test-source")

	avg := pt.GetAverageLatency()
	if avg != 0 {
		t.Errorf("Expected 0 latency for empty tracker, got %v", avg)
	}
}

func TestPerformanceTracker_GetAverageLatency_WithData(t *testing.T) {
	pt := NewPerformanceTracker("test-source")

	// Record some latencies
	pt.RecordEvent(100 * time.Millisecond)
	pt.RecordEvent(200 * time.Millisecond)
	pt.RecordEvent(300 * time.Millisecond)

	avg := pt.GetAverageLatency()
	expected := 200 * time.Millisecond // (100 + 200 + 300) / 3

	if avg != expected {
		t.Errorf("Expected average latency %v, got %v", expected, avg)
	}
}

func TestPerformanceTracker_GetPeakLatency(t *testing.T) {
	pt := NewPerformanceTracker("test-source")

	// Record some latencies
	pt.RecordEvent(100 * time.Millisecond)
	pt.RecordEvent(500 * time.Millisecond)
	pt.RecordEvent(200 * time.Millisecond)

	peak := pt.GetPeakLatency()
	expected := 500 * time.Millisecond

	if peak != expected {
		t.Errorf("Expected peak latency %v, got %v", expected, peak)
	}
}

func TestPerformanceTracker_GetPeakLatency_Empty(t *testing.T) {
	pt := NewPerformanceTracker("test-source")

	peak := pt.GetPeakLatency()
	if peak != 0 {
		t.Errorf("Expected 0 peak latency for empty tracker, got %v", peak)
	}
}

func TestPerformanceTracker_GetThroughput_NotActive(t *testing.T) {
	pt := NewPerformanceTracker("test-source")

	throughput := pt.GetThroughput()
	if throughput != 0 {
		t.Errorf("Expected 0 throughput when not active, got %f", throughput)
	}
}

func TestPerformanceTracker_GetThroughput_Active(t *testing.T) {
	pt := NewPerformanceTracker("test-source")

	pt.StartProcessing()

	// Record some events
	pt.RecordEvent(100 * time.Millisecond)
	pt.RecordEvent(200 * time.Millisecond)
	pt.RecordEvent(300 * time.Millisecond)

	// Small delay to ensure time has passed
	time.Sleep(10 * time.Millisecond)

	throughput := pt.GetThroughput()
	if throughput <= 0 {
		t.Errorf("Expected positive throughput when active, got %f", throughput)
	}

	pt.EndProcessing()
}

func TestPerformanceTracker_Reset(t *testing.T) {
	pt := NewPerformanceTracker("test-source")

	pt.StartProcessing()
	pt.RecordEvent(100 * time.Millisecond)
	pt.RecordEvent(200 * time.Millisecond)

	pt.Reset()

	avg := pt.GetAverageLatency()
	if avg != 0 {
		t.Errorf("Expected 0 latency after reset, got %v", avg)
	}

	throughput := pt.GetThroughput()
	if throughput != 0 {
		t.Errorf("Expected 0 throughput after reset, got %f", throughput)
	}
}

func TestPerformanceTracker_GetPerformanceData(t *testing.T) {
	pt := NewPerformanceTracker("test-source")

	pt.StartProcessing()
	pt.RecordEvent(100 * time.Millisecond)
	pt.RecordEvent(200 * time.Millisecond)
	pt.RecordEvent(300 * time.Millisecond)

	data := pt.GetPerformanceData()

	if data.Source != "test-source" {
		t.Errorf("Expected source 'test-source', got %s", data.Source)
	}

	if data.TotalProcessed != 3 {
		t.Errorf("Expected 3 total processed, got %d", data.TotalProcessed)
	}

	if data.AverageLatency == 0 {
		t.Error("Expected non-zero average latency")
	}

	if data.PeakLatency == 0 {
		t.Error("Expected non-zero peak latency")
	}

	pt.EndProcessing()
}
