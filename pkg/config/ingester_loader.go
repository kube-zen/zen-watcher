package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
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

	if _, err := ii.informer.AddEventHandler(handlers); err != nil {
		return fmt.Errorf("failed to add event handlers: %w", err)
	}

	// Start the informer factory
	ii.factory.Start(ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(ctx.Done(), ii.informer.HasSynced) {
		return fmt.Errorf("failed to sync Ingester informer cache")
	}

	logger := sdklog.NewLogger("zen-watcher-config")
	logger.Info("Ingester informer started and synced",
		sdklog.Operation("ingester_informer_synced"))

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
		logger := sdklog.NewLogger("zen-watcher-config")
		logger.Warn("Failed to convert Ingester CRD to unstructured",
			sdklog.Operation("ingester_add_convert"))
		return
	}

	config := ii.ConvertToIngesterConfig(u)
	if config != nil {
		ii.store.AddOrUpdate(config)
		logger := sdklog.NewLogger("zen-watcher-config")
		logger.Debug("Added Ingester config",
			sdklog.Operation("ingester_added"),
			sdklog.String("namespace", config.Namespace),
			sdklog.String("name", config.Name),
			sdklog.String("source", config.Source),
			sdklog.String("ingester", config.Ingester))
	}
}

// onUpdate handles Ingester CRD update events
func (ii *IngesterInformer) onUpdate(oldObj, newObj interface{}) {
	logger := sdklog.NewLogger("zen-watcher-config")
	u, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		logger.Warn("Failed to convert Ingester CRD to unstructured",
			sdklog.Operation("ingester_update_convert"))
		return
	}

	config := ii.ConvertToIngesterConfig(u)
	if config != nil {
		ii.store.AddOrUpdate(config)
		logger.Debug("Updated Ingester config",
			sdklog.Operation("ingester_updated"),
			sdklog.String("namespace", config.Namespace),
			sdklog.String("name", config.Name),
			sdklog.String("source", config.Source),
			sdklog.String("ingester", config.Ingester))
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
			logger := sdklog.NewLogger("zen-watcher-config")
			logger.Warn("Failed to convert deleted Ingester CRD to unstructured",
				sdklog.Operation("ingester_delete_convert"))
			return
		}
	}

	namespace := u.GetNamespace()
	name := u.GetName()
	ii.store.Delete(namespace, name)

	logger := sdklog.NewLogger("zen-watcher-config")
	logger.Debug("Deleted Ingester config",
		sdklog.Operation("ingester_deleted"),
		sdklog.String("namespace", namespace),
		sdklog.String("name", name))
}

// ConvertToIngesterConfigs converts an unstructured Ingester CRD to one or more IngesterConfigs
// If spec.sources[] is present, returns one config per source.
// Otherwise, returns a single config using legacy spec.source/spec.ingester fields.
func (ii *IngesterInformer) ConvertToIngesterConfigs(u *unstructured.Unstructured) []*IngesterConfig {
	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		logger := sdklog.NewLogger("zen-watcher-config")
		logger.Warn("Ingester CRD missing spec",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("name", u.GetName()))
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
	logger := sdklog.NewLogger("zen-watcher-config")
	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		logger.Warn("Ingester CRD missing spec",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("name", u.GetName()))
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
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("name", u.GetName()))
		return nil
	}
	config.Source = source

	// Extract ingester type (required field)
	ingester, ingesterOk := spec["ingester"].(string)
	if !ingesterOk || ingester == "" {
		logger.Warn("Ingester CRD missing required field: ingester",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("source", source),
			sdklog.String("name", u.GetName()))
		return nil
	}
	config.Ingester = ingester

	// Debug: log spec keys for logs ingester
	if ingester == "logs" {
		logger.Info("Processing logs ingester",
			sdklog.Operation("ingester_convert"),
			sdklog.String("source", source),
			sdklog.String("name", u.GetName()),
			sdklog.String("namespace", u.GetNamespace()))
	}

	// Validate destinations (required field)
	destinations, destinationsOk := spec["destinations"].([]interface{})
	if !destinationsOk || len(destinations) == 0 {
		logger.Warn("Ingester CRD missing required field: destinations",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("source", source),
			sdklog.String("name", u.GetName()),
			sdklog.String("ingester", ingester))
		return nil
	}

	// Extract destinations and resolve GVRs
	config.Destinations = extractDestinations(destinations, logger)

	// Ensure at least one destination was extracted
	if len(config.Destinations) == 0 {
		logger.Warn("No valid CRD destinations found",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("source", source))
		return nil
	}

	// Extract source-specific configs
	config.Informer = extractInformerConfig(spec)
	config.Webhook = extractWebhookConfig(spec)
	config.Logs = extractLogsConfig(spec, logger, config.Source)

	// Extract normalization config
	config.Normalization = extractNormalization(spec)

	// Extract processing config (filter and dedup)
	config.Processing, config.Filter, config.Dedup = extractProcessingConfig(spec)

	// Extract optimization config
	config.Optimization = extractOptimizationConfig(spec)

	return config
}

// convertMultiSourceIngester converts a multi-source Ingester CRD to multiple IngesterConfigs
func (ii *IngesterInformer) convertMultiSourceIngester(u *unstructured.Unstructured, spec map[string]interface{}, sources []interface{}) []*IngesterConfig {
	logger := sdklog.NewLogger("zen-watcher-config")
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
				sdklog.Operation("ingester_convert"),
				sdklog.String("namespace", namespace),
				sdklog.String("name", name),
				sdklog.Int("index", idx),
				sdklog.String("source_type", fmt.Sprintf("%T", sourceItem)))
			continue
		}

		// Extract source name and type
		sourceName := getString(sourceMap, "name")
		sourceType := getString(sourceMap, "type")
		if sourceName == "" || sourceType == "" {
			logger.Warn("Source missing name or type",
				sdklog.Operation("ingester_convert"),
				sdklog.String("namespace", namespace),
				sdklog.String("name", name),
				sdklog.Int("index", idx),
				sdklog.String("sourceName", sourceName),
				sdklog.String("sourceType", sourceType))
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
		extractSourceConfig(config, sourceMap, sourceType, logger, namespace, name, sourceName)
		if config.Informer == nil && config.Logs == nil && config.Webhook == nil {
			// No valid source config extracted, skip this source
			continue
		}

		// Validate config has required fields
		if len(config.Destinations) == 0 {
			logger.Warn("Source has no valid destinations",
				sdklog.Operation("ingester_convert"),
				sdklog.String("namespace", namespace),
				sdklog.String("name", name),
				sdklog.String("sourceName", sourceName),
				sdklog.String("sourceType", sourceType))
			continue
		}

		configs = append(configs, config)
	}

	return configs
}

// convertLegacyIngester converts a legacy single-source Ingester CRD to IngesterConfig
func (ii *IngesterInformer) convertLegacyIngester(u *unstructured.Unstructured, spec map[string]interface{}) *IngesterConfig {
	logger := sdklog.NewLogger("zen-watcher-config")
	config := &IngesterConfig{
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
	}

	// Extract source (required field)
	source, sourceOk := spec["source"].(string)
	if !sourceOk || source == "" {
		logger := sdklog.NewLogger("zen-watcher-config")
		logger.Warn("Ingester CRD missing required field: source",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("name", u.GetName()))
		return nil
	}
	config.Source = source

	// Extract ingester type (required field)
	ingester, ingesterOk := spec["ingester"].(string)
	if !ingesterOk || ingester == "" {
		logger := sdklog.NewLogger("zen-watcher-config")
		logger.Warn("Ingester CRD missing required field: ingester",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("source", source),
			sdklog.String("name", u.GetName()))
		return nil
	}
	config.Ingester = ingester

	// Extract destinations (required field) - reuse existing logic
	destinations, destinationsOk := spec["destinations"].([]interface{})
	if !destinationsOk || len(destinations) == 0 {
		logger.Warn("Ingester CRD missing required field: destinations",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("source", source),
			sdklog.String("name", u.GetName()),
			sdklog.String("ingester", ingester))
		return nil
	}

	// Extract destinations and resolve GVRs (reuse existing logic)
	config.Destinations = extractDestinations(destinations, logger)

	// Ensure at least one destination was extracted
	if len(config.Destinations) == 0 {
		logger.Warn("No valid CRD destinations found",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("source", source))
		return nil
	}

	// Extract other configs (informer, webhook, logs, etc.) - reuse existing logic
	sharedConfig := ii.extractSharedConfig(spec, u.GetNamespace(), u.GetName())
	if sharedConfig != nil {
		config.Normalization = sharedConfig.Normalization
		config.Filter = sharedConfig.Filter
		config.Dedup = sharedConfig.Dedup
		config.Processing = sharedConfig.Processing
		config.Optimization = sharedConfig.Optimization
	}

	// Extract source-specific configs
	config.Informer = extractInformerConfig(spec)
	config.Webhook = extractWebhookConfig(spec)
	config.Logs = extractLogsConfig(spec, logger, source)

	return config
}

// extractSharedConfig extracts shared configuration fields (destinations, processing, etc.)
// that are common to all sources in a multi-source Ingester
func (ii *IngesterInformer) extractSharedConfig(spec map[string]interface{}, namespace, name string) *IngesterConfig {
	logger := sdklog.NewLogger("zen-watcher-config")
	config := &IngesterConfig{
		Namespace: namespace,
		Name:      name,
	}

	// Extract destinations (required field)
	destinations, destinationsOk := spec["destinations"].([]interface{})
	if !destinationsOk || len(destinations) == 0 {
		logger.Warn("Ingester CRD missing required field: destinations",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", namespace),
			sdklog.String("name", name))
		return nil
	}

	// Extract destinations and resolve GVRs
	config.Destinations = extractDestinations(destinations, logger)

	// Ensure at least one destination was extracted
	if len(config.Destinations) == 0 {
		logger.Warn("No valid CRD destinations found",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", namespace))
		return nil
	}

	// Extract normalization config
	config.Normalization = extractNormalization(spec)

	// Extract processing config (filter and dedup)
	config.Processing, config.Filter, config.Dedup = extractProcessingConfig(spec)

	// Extract optimization config
	config.Optimization = extractOptimizationConfig(spec)

	return config
}

// extractDestinations extracts and validates destination configurations
func extractDestinations(destinations []interface{}, logger *sdklog.Logger) []DestinationConfig {
	result := make([]DestinationConfig, 0, len(destinations))
	for _, dest := range destinations {
		if destMap, ok := dest.(map[string]interface{}); ok {
			destType := getString(destMap, "type")
			destValue := getString(destMap, "value")

			if destType == "crd" {
				gvr := resolveDestinationGVR(destMap, destValue, logger)
				if gvr.Resource != "" {
					result = append(result, DestinationConfig{
						Type:  destType,
						Value: destValue,
						GVR:   gvr,
					})
				}
			}
		}
	}
	return result
}

// resolveDestinationGVR resolves GVR from destination map
func resolveDestinationGVR(destMap map[string]interface{}, destValue string, logger *sdklog.Logger) schema.GroupVersionResource {
	var gvr schema.GroupVersionResource
	if gvrMap, ok := destMap["gvr"].(map[string]interface{}); ok {
		group := getString(gvrMap, "group")
		version := getString(gvrMap, "version")
		resource := getString(gvrMap, "resource")

		if version != "" && resource != "" {
			if err := ValidateGVRConfig(group, version, resource); err != nil {
				logger.Warn("Invalid GVR in destination configuration",
					sdklog.Operation("ingester_convert"),
					sdklog.String("group", group),
					sdklog.String("version", version),
					sdklog.String("resource", resource),
					sdklog.Error(err))
				return gvr
			}
			gvr = schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: resource,
			}
		} else if destValue != "" {
			gvr = ResolveDestinationGVR(destValue)
		} else {
			logger.Warn("Destination has neither gvr nor value",
				sdklog.Operation("ingester_convert"))
		}
	} else if destValue != "" {
		gvr = ResolveDestinationGVR(destValue)
	} else {
		logger.Warn("Destination has neither gvr nor value",
			sdklog.Operation("ingester_convert"))
	}
	return gvr
}

// extractNormalization extracts normalization configuration
func extractNormalization(spec map[string]interface{}) *NormalizationConfig {
	norm, ok := spec["normalization"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &NormalizationConfig{
		Domain:   getString(norm, "domain"),
		Type:     getString(norm, "type"),
		Priority: make(map[string]float64),
	}
	if priority, ok := norm["priority"].(map[string]interface{}); ok {
		for k, v := range priority {
			if f, ok := v.(float64); ok {
				config.Priority[k] = f
			}
		}
	}
	if fieldMapping, ok := norm["fieldMapping"].([]interface{}); ok {
		for _, fm := range fieldMapping {
			if fmMap, ok := fm.(map[string]interface{}); ok {
				config.FieldMapping = append(config.FieldMapping, FieldMapping{
					From:      getString(fmMap, "from"),
					To:        getString(fmMap, "to"),
					Transform: getString(fmMap, "transform"),
				})
			}
		}
	}
	return config
}

// extractProcessingConfig extracts processing configuration (filter and dedup)
func extractProcessingConfig(spec map[string]interface{}) (*ProcessingConfig, *FilterConfig, *DedupConfig) {
	var processingConfig *ProcessingConfig
	var filterConfig *FilterConfig
	var dedupConfig *DedupConfig

	if processing, ok := spec["processing"].(map[string]interface{}); ok {
		processingConfig = &ProcessingConfig{
			Order: getString(processing, "order"),
		}
		filterConfig = extractFilterFromProcessing(processing)
		dedupConfig = extractDedupFromProcessing(processing)
	}

	// Fallback to legacy locations
	if dedupConfig == nil {
		dedupConfig = extractDedupFromLegacy(spec)
	}
	if filterConfig == nil {
		filterConfig = extractFilterFromLegacy(spec)
	}

	return processingConfig, filterConfig, dedupConfig
}

// extractFilterFromProcessing extracts filter config from processing.filter
func extractFilterFromProcessing(processing map[string]interface{}) *FilterConfig {
	filter, ok := processing["filter"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &FilterConfig{}
	if expression, ok := filter["expression"].(string); ok && expression != "" {
		config.Expression = expression
	}
	if minPriority, ok := filter["minPriority"].(float64); ok {
		config.MinPriority = minPriority
	}
	if includeNS, ok := filter["includeNamespaces"].([]interface{}); ok {
		for _, ns := range includeNS {
			if nsStr, ok := ns.(string); ok {
				config.IncludeNamespaces = append(config.IncludeNamespaces, nsStr)
			}
		}
	}
	if excludeNS, ok := filter["excludeNamespaces"].([]interface{}); ok {
		for _, ns := range excludeNS {
			if nsStr, ok := ns.(string); ok {
				config.ExcludeNamespaces = append(config.ExcludeNamespaces, nsStr)
			}
		}
	}
	return config
}

// extractDedupFromProcessing extracts dedup config from processing.dedup
func extractDedupFromProcessing(processing map[string]interface{}) *DedupConfig {
	dedup, ok := processing["dedup"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &DedupConfig{
		Enabled: true,
	}
	if enabled, ok := dedup["enabled"].(bool); ok {
		config.Enabled = enabled
	}
	config.Window = getString(dedup, "window")
	config.Strategy = getString(dedup, "strategy")
	if config.Strategy == "" {
		config.Strategy = "fingerprint"
	}
	if fields, ok := dedup["fields"].([]interface{}); ok {
		for _, f := range fields {
			if fStr, ok := f.(string); ok {
				config.Fields = append(config.Fields, fStr)
			}
		}
	}
	if maxEvents, ok := dedup["maxEventsPerWindow"].(float64); ok {
		config.MaxEventsPerWindow = int(maxEvents)
	}
	return config
}

// extractDedupFromLegacy extracts dedup config from legacy location
func extractDedupFromLegacy(spec map[string]interface{}) *DedupConfig {
	dedup, ok := spec["deduplication"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &DedupConfig{
		Enabled: getBool(dedup, "enabled"),
	}
	config.Window = getString(dedup, "window")
	config.Strategy = getString(dedup, "strategy")
	if config.Strategy == "" {
		config.Strategy = "fingerprint"
	}
	if fields, ok := dedup["fields"].([]interface{}); ok {
		for _, f := range fields {
			if fStr, ok := f.(string); ok {
				config.Fields = append(config.Fields, fStr)
			}
		}
	}
	return config
}

// extractFilterFromLegacy extracts filter config from legacy location
func extractFilterFromLegacy(spec map[string]interface{}) *FilterConfig {
	filter, ok := spec["filters"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &FilterConfig{}
	if expression, ok := filter["expression"].(string); ok && expression != "" {
		config.Expression = expression
	}
	if minPriority, ok := filter["minPriority"].(float64); ok {
		config.MinPriority = minPriority
	}
	if includeNS, ok := filter["includeNamespaces"].([]interface{}); ok {
		for _, ns := range includeNS {
			if nsStr, ok := ns.(string); ok {
				config.IncludeNamespaces = append(config.IncludeNamespaces, nsStr)
			}
		}
	}
	if excludeNS, ok := filter["excludeNamespaces"].([]interface{}); ok {
		for _, ns := range excludeNS {
			if nsStr, ok := ns.(string); ok {
				config.ExcludeNamespaces = append(config.ExcludeNamespaces, nsStr)
			}
		}
	}
	return config
}

// extractOptimizationConfig extracts optimization configuration
func extractOptimizationConfig(spec map[string]interface{}) *OptimizationConfig {
	optimization, ok := spec["optimization"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &OptimizationConfig{
		Order:      getString(optimization, "order"),
		Processing: make(map[string]*ProcessingThreshold),
	}
	if thresholds, ok := optimization["thresholds"].(map[string]interface{}); ok {
		config.Thresholds = extractOptimizationThresholds(thresholds)
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
				config.Processing[key] = pt
			}
		}
	}
	return config
}

// extractOptimizationThresholds extracts optimization thresholds
func extractOptimizationThresholds(thresholds map[string]interface{}) *OptimizationThresholds {
	result := &OptimizationThresholds{}
	if dedupEff, ok := thresholds["dedupEffectiveness"].(map[string]interface{}); ok {
		result.DedupEffectiveness = &ThresholdRange{}
		if w, ok := dedupEff["warning"].(float64); ok {
			result.DedupEffectiveness.Warning = w
		}
		if c, ok := dedupEff["critical"].(float64); ok {
			result.DedupEffectiveness.Critical = c
		}
	}
	if lowSev, ok := thresholds["lowSeverityPercent"].(map[string]interface{}); ok {
		result.LowSeverityPercent = &ThresholdRange{}
		if w, ok := lowSev["warning"].(float64); ok {
			result.LowSeverityPercent.Warning = w
		}
		if c, ok := lowSev["critical"].(float64); ok {
			result.LowSeverityPercent.Critical = c
		}
	}
	if obsPerMin, ok := thresholds["observationsPerMinute"].(map[string]interface{}); ok {
		result.ObservationsPerMinute = &ThresholdRange{}
		if w, ok := obsPerMin["warning"].(float64); ok {
			result.ObservationsPerMinute.Warning = w
		}
		if c, ok := obsPerMin["critical"].(float64); ok {
			result.ObservationsPerMinute.Critical = c
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
				result.Custom = append(result.Custom, ct)
			}
		}
	}
	return result
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

// extractSourceConfig extracts source-specific configuration for multi-source ingester
func extractSourceConfig(config *IngesterConfig, sourceMap map[string]interface{}, sourceType string, logger *sdklog.Logger, namespace, name, sourceName string) {
	switch sourceType {
	case "informer":
		config.Informer = extractInformerConfigFromMap(sourceMap)
	case "logs":
		config.Logs = extractLogsConfigFromMap(sourceMap)
	case "webhook":
		config.Webhook = extractWebhookConfigFromMap(sourceMap)
	default:
		logger.Warn("Unknown source type",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", namespace),
			sdklog.String("name", name),
			sdklog.String("sourceName", sourceName),
			sdklog.String("sourceType", sourceType))
	}
}

// extractInformerConfigFromMap extracts informer config from a map
func extractInformerConfigFromMap(sourceMap map[string]interface{}) *InformerConfig {
	informer, ok := sourceMap["informer"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &InformerConfig{}
	if gvr, ok := informer["gvr"].(map[string]interface{}); ok {
		config.GVR = GVRConfig{
			Group:    getString(gvr, "group"),
			Version:  getString(gvr, "version"),
			Resource: getString(gvr, "resource"),
		}
	}
	config.Namespace = getString(informer, "namespace")
	config.LabelSelector = getString(informer, "labelSelector")
	config.ResyncPeriod = getString(informer, "resyncPeriod")
	return config
}

// extractLogsConfigFromMap extracts logs config from a map
func extractLogsConfigFromMap(sourceMap map[string]interface{}) *LogsConfig {
	logs, ok := sourceMap["logs"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &LogsConfig{
		PodSelector:  getString(logs, "podSelector"),
		Container:    getString(logs, "container"),
		PollInterval: getString(logs, "pollInterval"),
	}
	if config.PollInterval == "" {
		config.PollInterval = "1s"
	}
	if sinceSeconds, ok := logs["sinceSeconds"].(int); ok {
		config.SinceSeconds = sinceSeconds
	} else if sinceSeconds, ok := logs["sinceSeconds"].(float64); ok {
		config.SinceSeconds = int(sinceSeconds)
	} else {
		config.SinceSeconds = DefaultLogsSinceSeconds
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
				config.Patterns = append(config.Patterns, pattern)
			}
		}
	}
	return config
}

// extractWebhookConfigFromMap extracts webhook config from a map
func extractWebhookConfigFromMap(sourceMap map[string]interface{}) *WebhookConfig {
	webhook, ok := sourceMap["webhook"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &WebhookConfig{
		Path: getString(webhook, "path"),
	}
	if auth, ok := webhook["auth"].(map[string]interface{}); ok {
		config.Auth = &AuthConfig{
			Type:      getString(auth, "type"),
			SecretRef: getString(auth, "secretRef"),
		}
	}
	if rateLimit, ok := webhook["rateLimit"].(map[string]interface{}); ok {
		if rpm, ok := rateLimit["requestsPerMinute"].(int); ok {
			config.RateLimit = &RateLimitConfig{
				RequestsPerMinute: rpm,
			}
		}
	}
	return config
}

// extractInformerConfig extracts informer config from spec
func extractInformerConfig(spec map[string]interface{}) *InformerConfig {
	informer, ok := spec["informer"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &InformerConfig{}
	if gvr, ok := informer["gvr"].(map[string]interface{}); ok {
		config.GVR = GVRConfig{
			Group:    getString(gvr, "group"),
			Version:  getString(gvr, "version"),
			Resource: getString(gvr, "resource"),
		}
	}
	config.Namespace = getString(informer, "namespace")
	config.LabelSelector = getString(informer, "labelSelector")
	config.ResyncPeriod = getString(informer, "resyncPeriod")
	return config
}

// extractWebhookConfig extracts webhook config from spec
func extractWebhookConfig(spec map[string]interface{}) *WebhookConfig {
	webhook, ok := spec["webhook"].(map[string]interface{})
	if !ok {
		return nil
	}
	config := &WebhookConfig{
		Path: getString(webhook, "path"),
	}
	if auth, ok := webhook["auth"].(map[string]interface{}); ok {
		config.Auth = &AuthConfig{
			Type:      getString(auth, "type"),
			SecretRef: getString(auth, "secretRef"),
		}
	}
	if rateLimit, ok := webhook["rateLimit"].(map[string]interface{}); ok {
		if rpm, ok := rateLimit["requestsPerMinute"].(int); ok {
			config.RateLimit = &RateLimitConfig{
				RequestsPerMinute: rpm,
			}
		}
	}
	return config
}

// extractLogsConfig extracts logs config from spec
func extractLogsConfig(spec map[string]interface{}, logger *sdklog.Logger, source string) *LogsConfig {
	logs, logsOk := spec["logs"]
	if logsOk {
		logger.Info("Found logs section in spec",
			sdklog.Operation("ingester_convert"),
			sdklog.String("source", source),
			sdklog.String("logs_type", fmt.Sprintf("%T", logs)))
	}
	return extractLogsConfigFromMap(spec)
}

// ResolveDestinationGVR is now defined in gvrs.go to use configurable API group
