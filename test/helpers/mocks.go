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

package helpers

import (
	"time"

	"github.com/kube-zen/zen-watcher/pkg/adapter/generic"
)

// CreateTestRawEvent creates a test RawEvent
func CreateTestRawEvent(source, severity string) *generic.RawEvent {
	return &generic.RawEvent{
		Source:    source,
		Timestamp: time.Now(),
		RawData: map[string]interface{}{
			"severity": severity,
			"message":  "Test event",
		},
		Metadata: make(map[string]interface{}),
	}
}

// CreateTestRawEventWithID creates a test RawEvent with a specific ID
func CreateTestRawEventWithID(source, severity, id string) *generic.RawEvent {
	event := CreateTestRawEvent(source, severity)
	event.RawData["id"] = id
	return event
}

// CreateTestRawEventWithData creates a test RawEvent with custom data
func CreateTestRawEventWithData(source string, data map[string]interface{}) *generic.RawEvent {
	return &generic.RawEvent{
		Source:    source,
		Timestamp: time.Now(),
		RawData:   data,
		Metadata:  make(map[string]interface{}),
	}
}
