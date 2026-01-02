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

package watcher

import (
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Note: sync.Map is used instead of map[string][]string with RWMutex
// for better concurrent read performance (lock-free reads)

// FieldExtractor provides optimized field extraction with caching
type FieldExtractor struct {
	// Cache for compiled field paths to reduce map navigation overhead
	// Using sync.Map for lock-free reads and better concurrent performance
	fieldPathCache sync.Map // map[string][]string
}

// NewFieldExtractor creates a new field extractor with caching
func NewFieldExtractor() *FieldExtractor {
	return &FieldExtractor{}
}

// ExtractString extracts a string field using cached path
func (fe *FieldExtractor) ExtractString(obj map[string]interface{}, path ...string) (string, bool) {
	if len(path) == 0 {
		return "", false
	}

	// Use cached path if available (optimized key generation)
	// Using sync.Map for lock-free reads
	cacheKey := strings.Join(path, ":")
	if cachedPath, exists := fe.fieldPathCache.Load(cacheKey); exists {
		val, found, _ := unstructured.NestedString(obj, cachedPath.([]string)...)
		return val, found
	}

	// Cache the path for future use (only if not already cached by another goroutine)
	fe.fieldPathCache.LoadOrStore(cacheKey, path)

	// Use the path (either newly cached or already cached)
	val, found, _ := unstructured.NestedString(obj, path...)
	return val, found
}

// ExtractMap extracts a map field using cached path
func (fe *FieldExtractor) ExtractMap(obj map[string]interface{}, path ...string) (map[string]interface{}, bool) {
	if len(path) == 0 {
		return nil, false
	}

	// Use cached path if available (optimized key generation)
	// Using sync.Map for lock-free reads
	cacheKey := strings.Join(path, ":")
	if cachedPath, exists := fe.fieldPathCache.Load(cacheKey); exists {
		val, found, _ := unstructured.NestedMap(obj, cachedPath.([]string)...)
		return val, found
	}

	// Cache the path for future use (only if not already cached by another goroutine)
	fe.fieldPathCache.LoadOrStore(cacheKey, path)

	// Use the path (either newly cached or already cached)
	val, found, _ := unstructured.NestedMap(obj, path...)
	return val, found
}

// ExtractFieldCopy extracts any field using cached path
func (fe *FieldExtractor) ExtractFieldCopy(obj map[string]interface{}, path ...string) (interface{}, bool) {
	if len(path) == 0 {
		return nil, false
	}

	// Use cached path if available (optimized key generation)
	// Using sync.Map for lock-free reads
	cacheKey := strings.Join(path, ":")
	if cachedPath, exists := fe.fieldPathCache.Load(cacheKey); exists {
		val, found, _ := unstructured.NestedFieldCopy(obj, cachedPath.([]string)...)
		return val, found
	}

	// Cache the path for future use (only if not already cached by another goroutine)
	fe.fieldPathCache.LoadOrStore(cacheKey, path)

	// Use the path (either newly cached or already cached)
	val, found, _ := unstructured.NestedFieldCopy(obj, path...)
	return val, found
}

// ExtractInt64 extracts an int64 field using cached path
func (fe *FieldExtractor) ExtractInt64(obj map[string]interface{}, path ...string) (int64, bool) {
	if len(path) == 0 {
		return 0, false
	}

	// Use cached path if available (optimized key generation)
	// Using sync.Map for lock-free reads
	cacheKey := strings.Join(path, ":")
	if cachedPath, exists := fe.fieldPathCache.Load(cacheKey); exists {
		val, found, _ := unstructured.NestedInt64(obj, cachedPath.([]string)...)
		return val, found
	}

	// Cache the path for future use (only if not already cached by another goroutine)
	fe.fieldPathCache.LoadOrStore(cacheKey, path)

	// Use the path (either newly cached or already cached)
	val, found, _ := unstructured.NestedInt64(obj, path...)
	return val, found
}
