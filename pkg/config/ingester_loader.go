package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/logger"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// IngesterGVR is the GroupVersionResource for Ingester CRDs
var IngesterGVR = schema.GroupVersionResource{
	Group:    "zen.kube-zen.io",
	Version:  "v1alpha1",
	Resource: "ingesters",
}

// IngesterConfig represents the compiled configuration from an Ingester CRD
type IngesterConfig struct {
	Namespace     string
	Name          string
	Source        string
	Ingester      string // informer, webhook, logs, k8s-events
	Informer      *InformerConfig
	Webhook       *WebhookConfig
	Logs          *LogsConfig
	K8sEvents     *K8sEventsConfig
	Normalization *NormalizationConfig
	Filter        *FilterConfig
	Dedup         *DedupConfig
	Processing    *ProcessingConfig
	Optimization  *OptimizationConfig
}

// InformerConfig holds informer-specific configuration
type InformerConfig struct {
	GVR           GVRConfig
	Namespace     string
	LabelSelector string
	ResyncPeriod  string
}

// GVRConfig represents GroupVersionResource
type GVRConfig struct {
	Group    string
	Version  string
	Resource string
}

// WebhookConfig holds webhook-specific configuration
type WebhookConfig struct {
	Path      string
	Auth      *AuthConfig
	RateLimit *RateLimitConfig
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Type      string // hmac, none
	SecretRef string
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerMinute int
}

// LogsConfig holds logs-specific configuration (placeholder)
type LogsConfig struct {
	// TBD
}

// K8sEventsConfig holds Kubernetes events configuration
type K8sEventsConfig struct {
	InvolvedObjectKinds []string
}

// NormalizationConfig holds normalization rules
type NormalizationConfig struct {
	Domain       string
	Type         string
	Priority     map[string]float64
	FieldMapping []FieldMapping
}

// FieldMapping represents a field transformation rule
type FieldMapping struct {
	From      string
	To        string
	Transform string // lower, upper, truncate:N, etc.
}

// FilterConfig holds filtering rules
type FilterConfig struct {
	Expression        string // Filter expression (v1.1)
	MinPriority       float64
	IncludeNamespaces []string
	ExcludeNamespaces []string
}

// DedupConfig holds deduplication configuration
type DedupConfig struct {
	Enabled            bool
	Window             string
	Strategy           string   // fingerprint, key, event-stream (strict-window renamed)
	Fields             []string // For key-based strategy
	MaxEventsPerWindow int      // For event-stream strategy
}

// ProcessingConfig holds processing order and optimization settings
type ProcessingConfig struct {
	Order               string
	AutoOptimize        bool
	AnalysisInterval    string
	ConfidenceThreshold float64
}

// OptimizationConfig holds auto-optimization configuration from spec.optimization
type OptimizationConfig struct {
	Enabled             bool
	Order               string // auto, filter_first, dedup_first, hybrid, adaptive
	AutoOptimize        bool
	AnalysisInterval    string
	ConfidenceThreshold float64
	Thresholds          *OptimizationThresholds
	Processing          map[string]*ProcessingThreshold
}

// OptimizationThresholds holds optimization thresholds
type OptimizationThresholds struct {
	DedupEffectiveness    *ThresholdRange
	LowSeverityPercent    *ThresholdRange
	ObservationsPerMinute *ThresholdRange
	Custom                []CustomThreshold
}

// ThresholdRange holds warning and critical thresholds
type ThresholdRange struct {
	Warning  float64
	Critical float64
}

// CustomThreshold holds custom threshold configuration
type CustomThreshold struct {
	Name     string
	Field    string
	Operator string
	Value    string
	Message  string
}

// ProcessingThreshold holds per-metric threshold configuration
type ProcessingThreshold struct {
	Action      string // warn, alert, optimize
	Warning     float64
	Critical    float64
	Description string
}

// IngesterStore maintains a cached view of Ingester configurations
type IngesterStore struct {
	mu          sync.RWMutex
	byName      map[types.NamespacedName]*IngesterConfig // namespace/name -> config
	bySource    map[string]*IngesterConfig               // source -> config (assumes unique source)
	byType      map[string][]*IngesterConfig             // ingester type -> configs
	byNamespace map[string]map[string]*IngesterConfig    // namespace -> name -> config
}

// NewIngesterStore creates a new IngesterStore
func NewIngesterStore() *IngesterStore {
	return &IngesterStore{
		byName:      make(map[types.NamespacedName]*IngesterConfig),
		bySource:    make(map[string]*IngesterConfig),
		byType:      make(map[string][]*IngesterConfig),
		byNamespace: make(map[string]map[string]*IngesterConfig),
	}
}

// Get retrieves an IngesterConfig by namespace and name (O(1) lookup)
func (s *IngesterStore) Get(namespace, name string) (*IngesterConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	config, exists := s.byName[nn]
	return config, exists
}

// GetBySource retrieves an IngesterConfig by source name (O(1) lookup)
func (s *IngesterStore) GetBySource(source string) (*IngesterConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	config, exists := s.bySource[source]
	return config, exists
}

// ListAll returns all IngesterConfigs
func (s *IngesterStore) ListAll() []*IngesterConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	configs := make([]*IngesterConfig, 0, len(s.byName))
	for _, config := range s.byName {
		configs = append(configs, config)
	}
	return configs
}

// ListByType returns all IngesterConfigs of a specific type (informer, webhook, logs, k8s-events)
func (s *IngesterStore) ListByType(ingesterType string) []*IngesterConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byType[ingesterType]
}

// AddOrUpdate adds or updates an IngesterConfig
func (s *IngesterStore) AddOrUpdate(config *IngesterConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nn := types.NamespacedName{Namespace: config.Namespace, Name: config.Name}
	s.byName[nn] = config

	if config.Source != "" {
		s.bySource[config.Source] = config
	}

	// Update byType index
	if config.Ingester != "" {
		// Remove from old type if it exists
		for t, configs := range s.byType {
			for i, c := range configs {
				if c.Namespace == config.Namespace && c.Name == config.Name {
					s.byType[t] = append(configs[:i], configs[i+1:]...)
					break
				}
			}
		}
		// Add to new type
		s.byType[config.Ingester] = append(s.byType[config.Ingester], config)
	}

	// Update byNamespace index
	if s.byNamespace[config.Namespace] == nil {
		s.byNamespace[config.Namespace] = make(map[string]*IngesterConfig)
	}
	s.byNamespace[config.Namespace][config.Name] = config
}

// Delete removes an IngesterConfig by namespace and name
func (s *IngesterStore) Delete(namespace, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nn := types.NamespacedName{Namespace: namespace, Name: name}
	config, exists := s.byName[nn]
	if !exists {
		return
	}

	delete(s.byName, nn)

	if config.Source != "" {
		delete(s.bySource, config.Source)
	}

	// Remove from byType index
	if config.Ingester != "" {
		configs := s.byType[config.Ingester]
		for i, c := range configs {
			if c.Namespace == namespace && c.Name == name {
				s.byType[config.Ingester] = append(configs[:i], configs[i+1:]...)
				break
			}
		}
	}

	// Remove from byNamespace index
	if nsMap := s.byNamespace[namespace]; nsMap != nil {
		delete(nsMap, name)
		if len(nsMap) == 0 {
			delete(s.byNamespace, namespace)
		}
	}
}

// IngesterInformer manages watching Ingester CRDs and updating the store
type IngesterInformer struct {
	store     *IngesterStore
	dynClient dynamic.Interface
	factory   dynamicinformer.DynamicSharedInformerFactory
	informer  cache.SharedInformer
	stopper   chan struct{}
}

// NewIngesterInformer creates a new IngesterInformer
func NewIngesterInformer(store *IngesterStore, dynClient dynamic.Interface) *IngesterInformer {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 10*time.Minute)

	return &IngesterInformer{
		store:     store,
		dynClient: dynClient,
		factory:   factory,
		stopper:   make(chan struct{}),
	}
}

// Start starts watching Ingester CRDs
func (ii *IngesterInformer) Start(ctx context.Context) error {
	// Get informer for Ingester CRDs
	ii.informer = ii.factory.ForResource(IngesterGVR).Informer()

	// Set up event handlers
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    ii.onAdd,
		UpdateFunc: ii.onUpdate,
		DeleteFunc: ii.onDelete,
	}

	ii.informer.AddEventHandler(handlers)

	// Start the informer factory
	ii.factory.Start(ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), ii.informer.HasSynced) {
		return fmt.Errorf("failed to sync Ingester informer cache")
	}

	logger.Info("Ingester informer started and synced",
		logger.Fields{
			Component: "config",
			Operation: "ingester_informer_synced",
		})

	return nil
}

// Stop stops the informer
func (ii *IngesterInformer) Stop() {
	close(ii.stopper)
}

// onAdd handles Ingester CRD add events
func (ii *IngesterInformer) onAdd(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		logger.Warn("Failed to convert Ingester CRD to unstructured",
			logger.Fields{
				Component: "config",
				Operation: "ingester_add_convert",
			})
		return
	}

	config := ii.convertToIngesterConfig(u)
	if config != nil {
		ii.store.AddOrUpdate(config)
		logger.Debug("Added Ingester config",
			logger.Fields{
				Component: "config",
				Operation: "ingester_added",
				Namespace: config.Namespace,
				Additional: map[string]interface{}{
					"name":     config.Name,
					"source":   config.Source,
					"ingester": config.Ingester,
				},
			})
	}
}

// onUpdate handles Ingester CRD update events
func (ii *IngesterInformer) onUpdate(oldObj, newObj interface{}) {
	u, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		logger.Warn("Failed to convert Ingester CRD to unstructured",
			logger.Fields{
				Component: "config",
				Operation: "ingester_update_convert",
			})
		return
	}

	config := ii.convertToIngesterConfig(u)
	if config != nil {
		ii.store.AddOrUpdate(config)
		logger.Debug("Updated Ingester config",
			logger.Fields{
				Component: "config",
				Operation: "ingester_updated",
				Namespace: config.Namespace,
				Additional: map[string]interface{}{
					"name":     config.Name,
					"source":   config.Source,
					"ingester": config.Ingester,
				},
				Source: config.Source,
			})
	}
}

// onDelete handles Ingester CRD delete events
func (ii *IngesterInformer) onDelete(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Handle DeletedFinalStateUnknown
		if deleted, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			u, ok = deleted.Obj.(*unstructured.Unstructured)
		}
		if !ok {
			logger.Warn("Failed to convert deleted Ingester CRD to unstructured",
				logger.Fields{
					Component: "config",
					Operation: "ingester_delete_convert",
				})
			return
		}
	}

	namespace := u.GetNamespace()
	name := u.GetName()
	ii.store.Delete(namespace, name)

	logger.Debug("Deleted Ingester config",
		logger.Fields{
			Component: "config",
			Operation: "ingester_deleted",
			Namespace: namespace,
			Additional: map[string]interface{}{
				"name": name,
			},
		})
}

// convertToIngesterConfig converts an unstructured Ingester CRD to IngesterConfig
func (ii *IngesterInformer) convertToIngesterConfig(u *unstructured.Unstructured) *IngesterConfig {
	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		logger.Warn("Ingester CRD missing spec",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Namespace: u.GetNamespace(),
				Additional: map[string]interface{}{
					"name": u.GetName(),
				},
			})
		return nil
	}

	config := &IngesterConfig{
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
	}

	// Extract source (required field)
	source, sourceOk := spec["source"].(string)
	if !sourceOk || source == "" {
		logger.Warn("Ingester CRD missing required field: source",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Namespace: u.GetNamespace(),
				Additional: map[string]interface{}{
					"name": u.GetName(),
				},
			})
		return nil
	}
	config.Source = source

	// Extract ingester type (required field)
	ingester, ingesterOk := spec["ingester"].(string)
	if !ingesterOk || ingester == "" {
		logger.Warn("Ingester CRD missing required field: ingester",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Namespace: u.GetNamespace(),
				Source:    source,
				Additional: map[string]interface{}{
					"name": u.GetName(),
				},
			})
		return nil
	}
	config.Ingester = ingester

	// Validate destinations (required field)
	destinations, destinationsOk := spec["destinations"].([]interface{})
	if !destinationsOk || len(destinations) == 0 {
		logger.Warn("Ingester CRD missing required field: destinations",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Namespace: u.GetNamespace(),
				Source:    source,
				Additional: map[string]interface{}{
					"name":     u.GetName(),
					"ingester": ingester,
				},
			})
		return nil
	}

	// Extract informer config
	if informer, ok := spec["informer"].(map[string]interface{}); ok {
		config.Informer = &InformerConfig{}
		if gvr, ok := informer["gvr"].(map[string]interface{}); ok {
			config.Informer.GVR = GVRConfig{
				Group:    getString(gvr, "group"),
				Version:  getString(gvr, "version"),
				Resource: getString(gvr, "resource"),
			}
		}
		config.Informer.Namespace = getString(informer, "namespace")
		config.Informer.LabelSelector = getString(informer, "labelSelector")
		config.Informer.ResyncPeriod = getString(informer, "resyncPeriod")
	}

	// Extract webhook config
	if webhook, ok := spec["webhook"].(map[string]interface{}); ok {
		config.Webhook = &WebhookConfig{
			Path: getString(webhook, "path"),
		}
		if auth, ok := webhook["auth"].(map[string]interface{}); ok {
			config.Webhook.Auth = &AuthConfig{
				Type:      getString(auth, "type"),
				SecretRef: getString(auth, "secretRef"),
			}
		}
		if rateLimit, ok := webhook["rateLimit"].(map[string]interface{}); ok {
			if rpm, ok := rateLimit["requestsPerMinute"].(int); ok {
				config.Webhook.RateLimit = &RateLimitConfig{
					RequestsPerMinute: rpm,
				}
			}
		}
	}

	// Extract normalization config
	if norm, ok := spec["normalization"].(map[string]interface{}); ok {
		config.Normalization = &NormalizationConfig{
			Domain:   getString(norm, "domain"),
			Type:     getString(norm, "type"),
			Priority: make(map[string]float64),
		}
		if priority, ok := norm["priority"].(map[string]interface{}); ok {
			for k, v := range priority {
				if f, ok := v.(float64); ok {
					config.Normalization.Priority[k] = f
				}
			}
		}
		if fieldMapping, ok := norm["fieldMapping"].([]interface{}); ok {
			for _, fm := range fieldMapping {
				if fmMap, ok := fm.(map[string]interface{}); ok {
					config.Normalization.FieldMapping = append(config.Normalization.FieldMapping, FieldMapping{
						From:      getString(fmMap, "from"),
						To:        getString(fmMap, "to"),
						Transform: getString(fmMap, "transform"),
					})
				}
			}
		}
	}

	// Extract dedup config from spec.processing.dedup (canonical location)
	var dedupConfig *DedupConfig
	var filterConfig *FilterConfig

	// First, try spec.processing (canonical v1.1+ location)
	if processing, ok := spec["processing"].(map[string]interface{}); ok {
		// Extract processing-level config (order, optimization settings)
		config.Processing = &ProcessingConfig{
			Order:        getString(processing, "order"),
			AutoOptimize: getBool(processing, "autoOptimize"),
		}
		config.Processing.AnalysisInterval = getString(processing, "analysisInterval")
		if threshold, ok := processing["confidenceThreshold"].(float64); ok {
			config.Processing.ConfidenceThreshold = threshold
		}

		// Extract filter from processing.filter (canonical location)
		if filter, ok := processing["filter"].(map[string]interface{}); ok {
			filterConfig = &FilterConfig{}
			// Check for expression (v1.1 feature)
			if expression, ok := filter["expression"].(string); ok && expression != "" {
				filterConfig.Expression = expression
			}
			// Legacy fields (only used if expression is not set)
			if minPriority, ok := filter["minPriority"].(float64); ok {
				filterConfig.MinPriority = minPriority
			}
			if includeNS, ok := filter["includeNamespaces"].([]interface{}); ok {
				for _, ns := range includeNS {
					if nsStr, ok := ns.(string); ok {
						filterConfig.IncludeNamespaces = append(filterConfig.IncludeNamespaces, nsStr)
					}
				}
			}
			if excludeNS, ok := filter["excludeNamespaces"].([]interface{}); ok {
				for _, ns := range excludeNS {
					if nsStr, ok := ns.(string); ok {
						filterConfig.ExcludeNamespaces = append(filterConfig.ExcludeNamespaces, nsStr)
					}
				}
			}
		}

		// Extract dedup from processing.dedup (canonical location)
		if dedup, ok := processing["dedup"].(map[string]interface{}); ok {
			dedupConfig = &DedupConfig{
				Enabled: true, // Default enabled
			}
			if enabled, ok := dedup["enabled"].(bool); ok {
				dedupConfig.Enabled = enabled
			}
			dedupConfig.Window = getString(dedup, "window")
			dedupConfig.Strategy = getString(dedup, "strategy")
			if dedupConfig.Strategy == "" {
				dedupConfig.Strategy = "fingerprint" // Default strategy
			}
			if fields, ok := dedup["fields"].([]interface{}); ok {
				for _, f := range fields {
					if fStr, ok := f.(string); ok {
						dedupConfig.Fields = append(dedupConfig.Fields, fStr)
					}
				}
			}
			if maxEvents, ok := dedup["maxEventsPerWindow"].(float64); ok {
				dedupConfig.MaxEventsPerWindow = int(maxEvents)
			}
		}
	}

	// Set filter config if found
	if filterConfig != nil {
		config.Filter = filterConfig
	}

	// Set dedup config if found
	if dedupConfig != nil {
		config.Dedup = dedupConfig
	}

	// Extract optimization config (canonical)
	if optimization, ok := spec["optimization"].(map[string]interface{}); ok {
		config.Optimization = &OptimizationConfig{
			Enabled:             getBool(optimization, "enabled"),
			Order:               getString(optimization, "order"),
			AutoOptimize:        getBool(optimization, "autoOptimize"),
			AnalysisInterval:    getString(optimization, "analysisInterval"),
			ConfidenceThreshold: 0.7, // default
			Processing:          make(map[string]*ProcessingThreshold),
		}

		if threshold, ok := optimization["confidenceThreshold"].(float64); ok {
			config.Optimization.ConfidenceThreshold = threshold
		}

		// Extract thresholds
		if thresholds, ok := optimization["thresholds"].(map[string]interface{}); ok {
			config.Optimization.Thresholds = &OptimizationThresholds{}

			if dedupEff, ok := thresholds["dedupEffectiveness"].(map[string]interface{}); ok {
				config.Optimization.Thresholds.DedupEffectiveness = &ThresholdRange{}
				if w, ok := dedupEff["warning"].(float64); ok {
					config.Optimization.Thresholds.DedupEffectiveness.Warning = w
				}
				if c, ok := dedupEff["critical"].(float64); ok {
					config.Optimization.Thresholds.DedupEffectiveness.Critical = c
				}
			}

			if lowSev, ok := thresholds["lowSeverityPercent"].(map[string]interface{}); ok {
				config.Optimization.Thresholds.LowSeverityPercent = &ThresholdRange{}
				if w, ok := lowSev["warning"].(float64); ok {
					config.Optimization.Thresholds.LowSeverityPercent.Warning = w
				}
				if c, ok := lowSev["critical"].(float64); ok {
					config.Optimization.Thresholds.LowSeverityPercent.Critical = c
				}
			}

			if obsPerMin, ok := thresholds["observationsPerMinute"].(map[string]interface{}); ok {
				config.Optimization.Thresholds.ObservationsPerMinute = &ThresholdRange{}
				if w, ok := obsPerMin["warning"].(float64); ok {
					config.Optimization.Thresholds.ObservationsPerMinute.Warning = w
				}
				if c, ok := obsPerMin["critical"].(float64); ok {
					config.Optimization.Thresholds.ObservationsPerMinute.Critical = c
				}
			}

			if custom, ok := thresholds["custom"].([]interface{}); ok {
				for _, c := range custom {
					if cMap, ok := c.(map[string]interface{}); ok {
						ct := CustomThreshold{
							Name:     getString(cMap, "name"),
							Field:    getString(cMap, "field"),
							Operator: getString(cMap, "operator"),
							Value:    getString(cMap, "value"),
							Message:  getString(cMap, "message"),
						}
						config.Optimization.Thresholds.Custom = append(config.Optimization.Thresholds.Custom, ct)
					}
				}
			}
		}

		// Extract processing thresholds
		if processing, ok := optimization["processing"].(map[string]interface{}); ok {
			for key, val := range processing {
				if pMap, ok := val.(map[string]interface{}); ok {
					pt := &ProcessingThreshold{
						Action:      getString(pMap, "action"),
						Description: getString(pMap, "description"),
					}
					if w, ok := pMap["warning"].(float64); ok {
						pt.Warning = w
					}
					if c, ok := pMap["critical"].(float64); ok {
						pt.Critical = c
					}
					config.Optimization.Processing[key] = pt
				}
			}
		}
	}

	return config
}

// Helper functions for extracting values from unstructured maps
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
