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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileStatePersistence implements StatePersistence using atomic file writes
type FileStatePersistence struct {
	baseDir string
}

// NewFileStatePersistence creates a new file-based state persistence
func NewFileStatePersistence(baseDir string) (*FileStatePersistence, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil { //nolint:gosec // G301: 0755 is standard for state directory
		return nil, fmt.Errorf("failed to create persistence directory: %w", err)
	}
	return &FileStatePersistence{baseDir: baseDir}, nil
}

// Save saves state to a file using atomic write (write to temp, then rename)
func (fsp *FileStatePersistence) Save(source string, state *OptimizationState) error {
	// Serialize state
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temp file first
	filename := filepath.Join(fsp.baseDir, fmt.Sprintf("%s.json", source))
	tmpFile := filepath.Join(fsp.baseDir, fmt.Sprintf(".%s.tmp", source))

	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, filename); err != nil {
		// Clean up temp file on error
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Load loads state from a file
func (fsp *FileStatePersistence) Load(source string) (*OptimizationState, error) {
	filename := filepath.Join(fsp.baseDir, fmt.Sprintf("%s.json", source))

	data, err := os.ReadFile(filename) //nolint:gosec // G304: filename is from trusted source (internal state file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File doesn't exist yet, not an error
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state OptimizationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// Delete deletes a state file
func (fsp *FileStatePersistence) Delete(source string) error {
	filename := filepath.Join(fsp.baseDir, fmt.Sprintf("%s.json", source))
	return os.Remove(filename)
}
