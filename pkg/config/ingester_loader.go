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

// IngesterGVR is defined in gvrs.go to use configurable API group

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
	Destinations  []DestinationConfig // Destination GVR configuration
}

// DestinationConfig holds destination GVR configuration
type DestinationConfig struct {
	Type  string                      // "crd"
	Value string                      // Resource name (e.g., "observations")
	GVR   schema.GroupVersionResource // Resolved GVR
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

// LogsConfig holds logs-specific configuration
type LogsConfig struct {
	PodSelector  string
	Container    string
	Patterns     []LogPattern
	SinceSeconds int
	PollInterval string
}

// LogPattern defines a regex pattern to match in logs
type LogPattern struct {
	Regex    string
	Type     string
	Priority float64
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

// ProcessingConfig holds processing order settings
type ProcessingConfig struct {
	Order string // filter_first or dedup_first
}

// OptimizationConfig holds optimization configuration from spec.optimization
// Note: Auto-optimization has been removed. Only manual order selection is supported.
type OptimizationConfig struct {
	Order      string // filter_first or dedup_first
	Thresholds *OptimizationThresholds
	Processing map[string]*ProcessingThreshold
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

	config := ii.ConvertToIngesterConfig(u)
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

	config := ii.ConvertToIngesterConfig(u)
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

// ConvertToIngesterConfigs converts an unstructured Ingester CRD to one or more IngesterConfigs
// If spec.sources[] is present, returns one config per source.
// Otherwise, returns a single config using legacy spec.source/spec.ingester fields.
func (ii *IngesterInformer) ConvertToIngesterConfigs(u *unstructured.Unstructured) []*IngesterConfig {
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

	// Check for multi-source configuration
	if sources, ok := spec["sources"].([]interface{}); ok && len(sources) > 0 {
		return ii.convertMultiSourceIngester(u, spec, sources)
	}

	// Legacy single-source mode
	config := ii.convertLegacyIngester(u, spec)
	if config == nil {
		return nil
	}
	return []*IngesterConfig{config}
}

// ConvertToIngesterConfig converts an unstructured Ingester CRD to IngesterConfig (legacy single-source)
// Deprecated: Use ConvertToIngesterConfigs for multi-source support
func (ii *IngesterInformer) ConvertToIngesterConfig(u *unstructured.Unstructured) *IngesterConfig {
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

	// Debug: log spec keys for logs ingester
	if ingester == "logs" {
		logger.Info("Processing logs ingester",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Source:    source,
				Additional: map[string]interface{}{
					"name":      u.GetName(),
					"namespace": u.GetNamespace(),
					"spec_keys": getSpecKeys(spec),
				},
			})
	}

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

	// Extract destinations and resolve GVRs
	config.Destinations = make([]DestinationConfig, 0, len(destinations))
	for _, dest := range destinations {
		if destMap, ok := dest.(map[string]interface{}); ok {
			destType := getString(destMap, "type")
			destValue := getString(destMap, "value")

			if destType == "crd" {
				var gvr schema.GroupVersionResource

				// Check if full GVR is specified
				if gvrMap, ok := destMap["gvr"].(map[string]interface{}); ok {
					group := getString(gvrMap, "group")
					version := getString(gvrMap, "version")
					resource := getString(gvrMap, "resource")

					if version != "" && resource != "" {
						// Validate GVR before using
						if err := ValidateGVRConfig(group, version, resource); err != nil {
							logger.Warn("Invalid GVR in destination configuration",
								logger.Fields{
									Component: "config",
									Operation: "ingester_convert",
									Source:    source,
									Error:     err,
									Additional: map[string]interface{}{
										"group":    group,
										"version":  version,
										"resource": resource,
									},
								})
							continue
						}
						// Use specified GVR
						gvr = schema.GroupVersionResource{
							Group:    group,
							Version:  version,
							Resource: resource,
						}
					} else if destValue != "" {
						// Fallback to resolving from value
						gvr = ResolveDestinationGVR(destValue)
					} else {
						logger.Warn("Destination has neither gvr nor value",
							logger.Fields{
								Component: "config",
								Operation: "ingester_convert",
								Source:    source,
							})
						continue
					}
				} else if destValue != "" {
					// Resolve GVR from destination value
					gvr = ResolveDestinationGVR(destValue)
				} else {
					logger.Warn("Destination has neither gvr nor value",
						logger.Fields{
							Component: "config",
							Operation: "ingester_convert",
							Source:    source,
						})
					continue
				}

				config.Destinations = append(config.Destinations, DestinationConfig{
					Type:  destType,
					Value: destValue,
					GVR:   gvr,
				})
			}
		}
	}

	// Ensure at least one destination was extracted
	if len(config.Destinations) == 0 {
		logger.Warn("No valid CRD destinations found",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Namespace: u.GetNamespace(),
				Source:    source,
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

	// Extract logs config
	logs, logsOk := spec["logs"]
	if logsOk {
		logger.Info("Found logs section in spec",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Source:    config.Source,
				Additional: map[string]interface{}{
					"logs_type":  fmt.Sprintf("%T", logs),
					"logs_value": logs,
				},
			})
	}
	if logs, ok := spec["logs"].(map[string]interface{}); ok {
		config.Logs = &LogsConfig{
			PodSelector:  getString(logs, "podSelector"),
			Container:    getString(logs, "container"),
			PollInterval: getString(logs, "pollInterval"),
		}
		// Default poll interval if not set
		if config.Logs.PollInterval == "" {
			config.Logs.PollInterval = "1s"
		}
		// Default sinceSeconds if not set
		if sinceSeconds, ok := logs["sinceSeconds"].(int); ok {
			config.Logs.SinceSeconds = sinceSeconds
		} else if sinceSeconds, ok := logs["sinceSeconds"].(float64); ok {
			config.Logs.SinceSeconds = int(sinceSeconds)
		} else {
			config.Logs.SinceSeconds = DefaultLogsSinceSeconds
		}
		// Extract patterns
		if patterns, ok := logs["patterns"].([]interface{}); ok {
			for _, p := range patterns {
				if patternMap, ok := p.(map[string]interface{}); ok {
					pattern := LogPattern{
						Regex: getString(patternMap, "regex"),
						Type:  getString(patternMap, "type"),
					}
					if priority, ok := patternMap["priority"].(float64); ok {
						pattern.Priority = priority
					}
					config.Logs.Patterns = append(config.Logs.Patterns, pattern)
				}
			}
		}
		logger.Info("Extracted logs config",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Source:    config.Source,
				Additional: map[string]interface{}{
					"podSelector":  config.Logs.PodSelector,
					"container":    config.Logs.Container,
					"patterns":     len(config.Logs.Patterns),
					"pollInterval": config.Logs.PollInterval,
				},
			})
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
		// Extract processing-level config (order only - auto-optimization removed)
		config.Processing = &ProcessingConfig{
			Order: getString(processing, "order"),
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
	// Note: Auto-optimization removed, only manual order selection supported
	if optimization, ok := spec["optimization"].(map[string]interface{}); ok {
		config.Optimization = &OptimizationConfig{
			Order:      getString(optimization, "order"),
			Processing: make(map[string]*ProcessingThreshold),
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

// convertMultiSourceIngester converts a multi-source Ingester CRD to multiple IngesterConfigs
func (ii *IngesterInformer) convertMultiSourceIngester(u *unstructured.Unstructured, spec map[string]interface{}, sources []interface{}) []*IngesterConfig {
	namespace := u.GetNamespace()
	name := u.GetName()
	var configs []*IngesterConfig

	// Extract shared fields (destinations, processing, etc.)
	sharedConfig := ii.extractSharedConfig(spec, namespace, name)
	if sharedConfig == nil {
		return nil
	}

	for idx, sourceItem := range sources {
		sourceMap, ok := sourceItem.(map[string]interface{})
		if !ok {
			logger.Warn("Invalid source entry in spec.sources",
				logger.Fields{
					Component: "config",
					Operation: "ingester_convert",
					Namespace: namespace,
					Additional: map[string]interface{}{
						"name":       name,
						"index":      idx,
						"source_type": fmt.Sprintf("%T", sourceItem),
					},
				})
			continue
		}

		// Extract source name and type
		sourceName := getString(sourceMap, "name")
		sourceType := getString(sourceMap, "type")
		if sourceName == "" || sourceType == "" {
			logger.Warn("Source missing name or type",
				logger.Fields{
					Component: "config",
					Operation: "ingester_convert",
					Namespace: namespace,
					Additional: map[string]interface{}{
						"name":       name,
						"index":      idx,
						"sourceName": sourceName,
						"sourceType": sourceType,
					},
				})
			continue
		}

		// Create config for this source
		config := &IngesterConfig{
			Namespace:     namespace,
			Name:          name,
			Source:        fmt.Sprintf("%s/%s/%s", namespace, name, sourceName), // Unique source identifier
			Ingester:      sourceType, // informer, logs, webhook
			Destinations:  sharedConfig.Destinations,
			Normalization: sharedConfig.Normalization,
			Filter:        sharedConfig.Filter,
			Dedup:         sharedConfig.Dedup,
			Processing:    sharedConfig.Processing,
			Optimization:  sharedConfig.Optimization,
		}

		// Extract source-specific config
		switch sourceType {
		case "informer":
			if informer, ok := sourceMap["informer"].(map[string]interface{}); ok {
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
		case "logs":
			if logs, ok := sourceMap["logs"].(map[string]interface{}); ok {
				config.Logs = &LogsConfig{
					PodSelector:  getString(logs, "podSelector"),
					Container:    getString(logs, "container"),
					PollInterval: getString(logs, "pollInterval"),
				}
				if config.Logs.PollInterval == "" {
					config.Logs.PollInterval = "1s"
				}
				if sinceSeconds, ok := logs["sinceSeconds"].(int); ok {
					config.Logs.SinceSeconds = sinceSeconds
				} else if sinceSeconds, ok := logs["sinceSeconds"].(float64); ok {
					config.Logs.SinceSeconds = int(sinceSeconds)
				} else {
					config.Logs.SinceSeconds = DefaultLogsSinceSeconds
				}
				if patterns, ok := logs["patterns"].([]interface{}); ok {
					for _, p := range patterns {
						if patternMap, ok := p.(map[string]interface{}); ok {
							pattern := LogPattern{
								Regex: getString(patternMap, "regex"),
								Type:  getString(patternMap, "type"),
							}
							if priority, ok := patternMap["priority"].(float64); ok {
								pattern.Priority = priority
							}
							config.Logs.Patterns = append(config.Logs.Patterns, pattern)
						}
					}
				}
			}
		case "webhook":
			if webhook, ok := sourceMap["webhook"].(map[string]interface{}); ok {
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
		default:
			logger.Warn("Unknown source type",
				logger.Fields{
					Component: "config",
					Operation: "ingester_convert",
					Namespace: namespace,
					Additional: map[string]interface{}{
						"name":       name,
						"sourceName": sourceName,
						"sourceType": sourceType,
					},
				})
			continue
		}

		// Validate config has required fields
		if len(config.Destinations) == 0 {
			logger.Warn("Source has no valid destinations",
				logger.Fields{
					Component: "config",
					Operation: "ingester_convert",
					Namespace: namespace,
					Additional: map[string]interface{}{
						"name":       name,
						"sourceName": sourceName,
						"sourceType": sourceType,
					},
				})
			continue
		}

		configs = append(configs, config)
	}

	return configs
}

// convertLegacyIngester converts a legacy single-source Ingester CRD to IngesterConfig
func (ii *IngesterInformer) convertLegacyIngester(u *unstructured.Unstructured, spec map[string]interface{}) *IngesterConfig {
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

	// Extract destinations (required field) - reuse existing logic
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

	// Extract destinations and resolve GVRs (reuse existing logic from ConvertToIngesterConfig)
	config.Destinations = make([]DestinationConfig, 0, len(destinations))
	for _, dest := range destinations {
		if destMap, ok := dest.(map[string]interface{}); ok {
			destType := getString(destMap, "type")
			destValue := getString(destMap, "value")

			if destType == "crd" {
				var gvr schema.GroupVersionResource

				// Check if full GVR is specified
				if gvrMap, ok := destMap["gvr"].(map[string]interface{}); ok {
					group := getString(gvrMap, "group")
					version := getString(gvrMap, "version")
					resource := getString(gvrMap, "resource")

					if version != "" && resource != "" {
						// Validate GVR before using
						if err := ValidateGVRConfig(group, version, resource); err != nil {
							logger.Warn("Invalid GVR in destination configuration",
								logger.Fields{
									Component: "config",
									Operation: "ingester_convert",
									Source:    source,
									Error:     err,
									Additional: map[string]interface{}{
										"group":    group,
										"version":  version,
										"resource": resource,
									},
								})
							continue
						}
						// Use specified GVR
						gvr = schema.GroupVersionResource{
							Group:    group,
							Version:  version,
							Resource: resource,
						}
					} else if destValue != "" {
						// Fallback to resolving from value
						gvr = ResolveDestinationGVR(destValue)
					} else {
						logger.Warn("Destination has neither gvr nor value",
							logger.Fields{
								Component: "config",
								Operation: "ingester_convert",
								Source:    source,
							})
						continue
					}
				} else if destValue != "" {
					// Resolve GVR from destination value
					gvr = ResolveDestinationGVR(destValue)
				} else {
					logger.Warn("Destination has neither gvr nor value",
						logger.Fields{
							Component: "config",
							Operation: "ingester_convert",
							Source:    source,
						})
					continue
				}

				config.Destinations = append(config.Destinations, DestinationConfig{
					Type:  destType,
					Value: destValue,
					GVR:   gvr,
				})
			}
		}
	}

	// Ensure at least one destination was extracted
	if len(config.Destinations) == 0 {
		logger.Warn("No valid CRD destinations found",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Namespace: u.GetNamespace(),
				Source:    source,
			})
		return nil
	}

	// Extract other configs (informer, webhook, logs, etc.) - reuse existing logic
	// This is handled by the rest of ConvertToIngesterConfig, so we'll call it
	// But we need to extract shared config first, so let's create extractSharedConfig
	sharedConfig := ii.extractSharedConfig(spec, u.GetNamespace(), u.GetName())
	if sharedConfig != nil {
		config.Normalization = sharedConfig.Normalization
		config.Filter = sharedConfig.Filter
		config.Dedup = sharedConfig.Dedup
		config.Processing = sharedConfig.Processing
		config.Optimization = sharedConfig.Optimization
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

	// Extract logs config
	if logs, ok := spec["logs"].(map[string]interface{}); ok {
		config.Logs = &LogsConfig{
			PodSelector:  getString(logs, "podSelector"),
			Container:    getString(logs, "container"),
			PollInterval: getString(logs, "pollInterval"),
		}
		// Default poll interval if not set
		if config.Logs.PollInterval == "" {
			config.Logs.PollInterval = "1s"
		}
		// Default sinceSeconds if not set
		if sinceSeconds, ok := logs["sinceSeconds"].(int); ok {
			config.Logs.SinceSeconds = sinceSeconds
		} else if sinceSeconds, ok := logs["sinceSeconds"].(float64); ok {
			config.Logs.SinceSeconds = int(sinceSeconds)
		} else {
			config.Logs.SinceSeconds = DefaultLogsSinceSeconds
		}
		// Extract patterns
		if patterns, ok := logs["patterns"].([]interface{}); ok {
			for _, p := range patterns {
				if patternMap, ok := p.(map[string]interface{}); ok {
					pattern := LogPattern{
						Regex: getString(patternMap, "regex"),
						Type:  getString(patternMap, "type"),
					}
					if priority, ok := patternMap["priority"].(float64); ok {
						pattern.Priority = priority
					}
					config.Logs.Patterns = append(config.Logs.Patterns, pattern)
				}
			}
		}
	}

	return config
}

// extractSharedConfig extracts shared configuration fields (destinations, processing, etc.)
// that are common to all sources in a multi-source Ingester
func (ii *IngesterInformer) extractSharedConfig(spec map[string]interface{}, namespace, name string) *IngesterConfig {
	config := &IngesterConfig{
		Namespace: namespace,
		Name:      name,
	}

	// Extract destinations (required field)
	destinations, destinationsOk := spec["destinations"].([]interface{})
	if !destinationsOk || len(destinations) == 0 {
		logger.Warn("Ingester CRD missing required field: destinations",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Namespace: namespace,
				Additional: map[string]interface{}{
					"name": name,
				},
			})
		return nil
	}

	// Extract destinations and resolve GVRs
	config.Destinations = make([]DestinationConfig, 0, len(destinations))
	for _, dest := range destinations {
		if destMap, ok := dest.(map[string]interface{}); ok {
			destType := getString(destMap, "type")
			destValue := getString(destMap, "value")

			if destType == "crd" {
				var gvr schema.GroupVersionResource

				// Check if full GVR is specified
				if gvrMap, ok := destMap["gvr"].(map[string]interface{}); ok {
					group := getString(gvrMap, "group")
					version := getString(gvrMap, "version")
					resource := getString(gvrMap, "resource")

					if version != "" && resource != "" {
						// Validate GVR before using
						if err := ValidateGVRConfig(group, version, resource); err != nil {
							logger.Warn("Invalid GVR in destination configuration",
								logger.Fields{
									Component: "config",
									Operation: "ingester_convert",
									Error:     err,
									Additional: map[string]interface{}{
										"group":    group,
										"version":  version,
										"resource": resource,
									},
								})
							continue
						}
						// Use specified GVR
						gvr = schema.GroupVersionResource{
							Group:    group,
							Version:  version,
							Resource: resource,
						}
					} else if destValue != "" {
						// Fallback to resolving from value
						gvr = ResolveDestinationGVR(destValue)
					} else {
						logger.Warn("Destination has neither gvr nor value",
							logger.Fields{
								Component: "config",
								Operation: "ingester_convert",
							})
						continue
					}
				} else if destValue != "" {
					// Resolve GVR from destination value
					gvr = ResolveDestinationGVR(destValue)
				} else {
					logger.Warn("Destination has neither gvr nor value",
						logger.Fields{
							Component: "config",
							Operation: "ingester_convert",
						})
					continue
				}

				config.Destinations = append(config.Destinations, DestinationConfig{
					Type:  destType,
					Value: destValue,
					GVR:   gvr,
				})
			}
		}
	}

	// Ensure at least one destination was extracted
	if len(config.Destinations) == 0 {
		logger.Warn("No valid CRD destinations found",
			logger.Fields{
				Component: "config",
				Operation: "ingester_convert",
				Namespace: namespace,
			})
		return nil
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

	// Extract dedup and filter configs (reuse existing logic from ConvertToIngesterConfig)
	var dedupConfig *DedupConfig
	var filterConfig *FilterConfig

	// First, try spec.processing (canonical v1.1+ location)
	if processing, ok := spec["processing"].(map[string]interface{}); ok {
		// Extract processing-level config (order only)
		config.Processing = &ProcessingConfig{
			Order: getString(processing, "order"),
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

	// Fallback to legacy locations
	if dedupConfig == nil {
		if dedup, ok := spec["deduplication"].(map[string]interface{}); ok {
			dedupConfig = &DedupConfig{
				Enabled: getBool(dedup, "enabled"),
			}
			dedupConfig.Window = getString(dedup, "window")
			dedupConfig.Strategy = getString(dedup, "strategy")
			if dedupConfig.Strategy == "" {
				dedupConfig.Strategy = "fingerprint"
			}
			if fields, ok := dedup["fields"].([]interface{}); ok {
				for _, f := range fields {
					if fStr, ok := f.(string); ok {
						dedupConfig.Fields = append(dedupConfig.Fields, fStr)
					}
				}
			}
		}
	}
	if filterConfig == nil {
		if filter, ok := spec["filters"].(map[string]interface{}); ok {
			filterConfig = &FilterConfig{}
			if expression, ok := filter["expression"].(string); ok && expression != "" {
				filterConfig.Expression = expression
			}
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
	}

	config.Dedup = dedupConfig
	config.Filter = filterConfig

	// Extract optimization config (reuse existing logic)
	if optimization, ok := spec["optimization"].(map[string]interface{}); ok {
		config.Optimization = &OptimizationConfig{
			Order:      getString(optimization, "order"),
			Processing: make(map[string]*ProcessingThreshold),
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

// getSpecKeys returns all keys in the spec map for debugging
func getSpecKeys(spec map[string]interface{}) []string {
	keys := make([]string, 0, len(spec))
	for k := range spec {
		keys = append(keys, k)
	}
	return keys
}

// ResolveDestinationGVR is now defined in gvrs.go to use configurable API group
