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

package errors

import "fmt"

// ErrorCategory represents the category of an error
type ErrorCategory string

const (
	// CONFIG_ERROR indicates a configuration error
	CONFIG_ERROR ErrorCategory = "CONFIG_ERROR"
	// FILTER_ERROR indicates an error in the filter stage
	FILTER_ERROR ErrorCategory = "FILTER_ERROR"
	// DEDUP_ERROR indicates an error in the deduplication stage
	DEDUP_ERROR ErrorCategory = "DEDUP_ERROR"
	// NORMALIZE_ERROR indicates an error in the normalization stage
	NORMALIZE_ERROR ErrorCategory = "NORMALIZE_ERROR"
	// CRD_WRITE_ERROR indicates an error writing to CRD
	CRD_WRITE_ERROR ErrorCategory = "CRD_WRITE_ERROR"
	// PIPELINE_ERROR indicates a general pipeline error
	PIPELINE_ERROR ErrorCategory = "PIPELINE_ERROR"
)

// PipelineError represents a categorized pipeline error
type PipelineError struct {
	Category    ErrorCategory
	Source      string
	Ingester    string
	Code        string
	Message     string
	OriginalErr error
}

func (e *PipelineError) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("[%s] %s: %s (source: %s, ingester: %s): %v",
			e.Category, e.Code, e.Message, e.Source, e.Ingester, e.OriginalErr)
	}
	return fmt.Sprintf("[%s] %s: %s (source: %s, ingester: %s)",
		e.Category, e.Code, e.Message, e.Source, e.Ingester)
}

// NewConfigError creates a new CONFIG_ERROR
func NewConfigError(source, ingester, code, message string, err error) *PipelineError {
	return &PipelineError{
		Category:    CONFIG_ERROR,
		Source:      source,
		Ingester:    ingester,
		Code:        code,
		Message:     message,
		OriginalErr: err,
	}
}

// NewFilterError creates a new FILTER_ERROR
func NewFilterError(source, ingester, code, message string, err error) *PipelineError {
	return &PipelineError{
		Category:    FILTER_ERROR,
		Source:      source,
		Ingester:    ingester,
		Code:        code,
		Message:     message,
		OriginalErr: err,
	}
}

// NewDedupError creates a new DEDUP_ERROR
func NewDedupError(source, ingester, code, message string, err error) *PipelineError {
	return &PipelineError{
		Category:    DEDUP_ERROR,
		Source:      source,
		Ingester:    ingester,
		Code:        code,
		Message:     message,
		OriginalErr: err,
	}
}

// NewNormalizeError creates a new NORMALIZE_ERROR
func NewNormalizeError(source, ingester, code, message string, err error) *PipelineError {
	return &PipelineError{
		Category:    NORMALIZE_ERROR,
		Source:      source,
		Ingester:    ingester,
		Code:        code,
		Message:     message,
		OriginalErr: err,
	}
}

// NewCRDWriteError creates a new CRD_WRITE_ERROR
func NewCRDWriteError(source, ingester, code, message string, err error) *PipelineError {
	return &PipelineError{
		Category:    CRD_WRITE_ERROR,
		Source:      source,
		Ingester:    ingester,
		Code:        code,
		Message:     message,
		OriginalErr: err,
	}
}

// NewPipelineError creates a new PIPELINE_ERROR
func NewPipelineError(source, ingester, code, message string, err error) *PipelineError {
	return &PipelineError{
		Category:    PIPELINE_ERROR,
		Source:      source,
		Ingester:    ingester,
		Code:        code,
		Message:     message,
		OriginalErr: err,
	}
}

