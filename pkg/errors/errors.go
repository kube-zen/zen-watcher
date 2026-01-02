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

// Package errors provides structured error types for zen-watcher with pipeline context.
// This package now uses zen-sdk/pkg/errors as the base implementation.
package errors

import (
	sdkerrors "github.com/kube-zen/zen-sdk/pkg/errors"
)

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

// PipelineError is an alias for zen-sdk's ContextError.
// This maintains backward compatibility while using the shared implementation.
type PipelineError = sdkerrors.ContextError

// NewConfigError creates a new CONFIG_ERROR
func NewConfigError(source, ingester, code, message string, err error) *PipelineError {
	ctxErr := sdkerrors.Wrap(err, string(CONFIG_ERROR), message)
	return sdkerrors.WithMultipleContext(ctxErr, map[string]string{
		"source":   source,
		"ingester": ingester,
		"code":     code,
	})
}

// NewFilterError creates a new FILTER_ERROR
func NewFilterError(source, ingester, code, message string, err error) *PipelineError {
	ctxErr := sdkerrors.Wrap(err, string(FILTER_ERROR), message)
	return sdkerrors.WithMultipleContext(ctxErr, map[string]string{
		"source":   source,
		"ingester": ingester,
		"code":     code,
	})
}

// NewDedupError creates a new DEDUP_ERROR
func NewDedupError(source, ingester, code, message string, err error) *PipelineError {
	ctxErr := sdkerrors.Wrap(err, string(DEDUP_ERROR), message)
	return sdkerrors.WithMultipleContext(ctxErr, map[string]string{
		"source":   source,
		"ingester": ingester,
		"code":     code,
	})
}

// NewNormalizeError creates a new NORMALIZE_ERROR
func NewNormalizeError(source, ingester, code, message string, err error) *PipelineError {
	ctxErr := sdkerrors.Wrap(err, string(NORMALIZE_ERROR), message)
	return sdkerrors.WithMultipleContext(ctxErr, map[string]string{
		"source":   source,
		"ingester": ingester,
		"code":     code,
	})
}

// NewCRDWriteError creates a new CRD_WRITE_ERROR
func NewCRDWriteError(source, ingester, code, message string, err error) *PipelineError {
	ctxErr := sdkerrors.Wrap(err, string(CRD_WRITE_ERROR), message)
	return sdkerrors.WithMultipleContext(ctxErr, map[string]string{
		"source":   source,
		"ingester": ingester,
		"code":     code,
	})
}

// NewPipelineError creates a new PIPELINE_ERROR
func NewPipelineError(source, ingester, code, message string, err error) *PipelineError {
	ctxErr := sdkerrors.Wrap(err, string(PIPELINE_ERROR), message)
	return sdkerrors.WithMultipleContext(ctxErr, map[string]string{
		"source":   source,
		"ingester": ingester,
		"code":     code,
	})
}
