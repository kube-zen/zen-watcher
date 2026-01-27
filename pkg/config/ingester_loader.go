package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kube-zen/zen-sdk/pkg/k8s/crdstore"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// Package-level logger to avoid repeated allocations
var configLogger = sdklog.NewLogger("zen-watcher-config")

// IngesterGVR is defined in gvrs.go to use configurable API group

// IngesterConfig represents the compiled configuration from an Ingester CRD
type IngesterConfig struct {
	Namespace     string
	Name          string
	Source        string
	Ingester      string // informer, webhook, logs
	Informer      *InformerConfig
	Webhook       *WebhookConfig
	Logs          *LogsConfig
	Normalization *NormalizationConfig
	Filter        *FilterConfig
	Dedup         *DedupConfig
	Processing    *ProcessingConfig
	Optimization  *OptimizationConfig
	Destinations  []DestinationConfig // Destination GVR configuration
}

// GetNamespace returns the namespace (implements CRDConfig interface)
func (c *IngesterConfig) GetNamespace() string {
	return c.Namespace
}

// GetName returns the name (implements CRDConfig interface)
func (c *IngesterConfig) GetName() string {
	return c.Name
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
	Path       string
	Port       int
	BufferSize int
	Auth       *AuthConfig
	RateLimit  *RateLimitConfig
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

// IngesterStore maintains a cached view of Ingester configurations.
// It embeds CRDStore for base functionality and adds a bySource index for O(1) lookups.
// The byType and byNamespace indexes were removed as they were unused.
type IngesterStore struct {
	*crdstore.CRDStore[*IngesterConfig] // Embed generic CRD store
	bySource map[string]*IngesterConfig // source -> config (O(1) lookup for hot path)
	mu       sync.RWMutex                // Protects bySource index
}

// NewIngesterStore creates a new IngesterStore
func NewIngesterStore() *IngesterStore {
	return &IngesterStore{
		CRDStore: crdstore.NewCRDStore[*IngesterConfig](),
		bySource: make(map[string]*IngesterConfig),
	}
}

// Get retrieves an IngesterConfig by namespace and name (O(1) lookup)
func (s *IngesterStore) Get(namespace, name string) (*IngesterConfig, bool) {
	return s.CRDStore.Get(namespace, name)
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
	return s.CRDStore.ListAll()
}

// NotifyChange sends a notification that the store has changed
func (s *IngesterStore) NotifyChange() {
	s.CRDStore.NotifyChange()
}

// ChangeChannel returns the channel for change notifications
func (s *IngesterStore) ChangeChannel() <-chan struct{} {
	return s.CRDStore.ChangeChannel()
}

// AddOrUpdate adds or updates an IngesterConfig
func (s *IngesterStore) AddOrUpdate(config *IngesterConfig) {
	// Update base store
	s.CRDStore.AddOrUpdate(config)

	// Update bySource index
	s.mu.Lock()
	if config.Source != "" {
		s.bySource[config.Source] = config
	}
	s.mu.Unlock()
}

// Delete removes an IngesterConfig by namespace and name
func (s *IngesterStore) Delete(namespace, name string) {
	// Get config first to clean up bySource index
	config, exists := s.CRDStore.Get(namespace, name)
	if !exists {
		return
	}

	// Delete from base store
	s.CRDStore.Delete(namespace, name)

	// Clean up bySource index
	s.mu.Lock()
	if config.Source != "" {
		delete(s.bySource, config.Source)
	}
	s.mu.Unlock()
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

	configLogger.Info("Ingester informer started and synced",
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
		configLogger.Warn("Failed to convert Ingester CRD to unstructured",
			sdklog.Operation("ingester_add_convert"))
		return
	}

	config := ii.ConvertToIngesterConfig(u)
	if config != nil {
		ii.store.AddOrUpdate(config)
		configLogger.Debug("Added Ingester config",
			sdklog.Operation("ingester_added"),
			sdklog.String("namespace", config.Namespace),
			sdklog.String("name", config.Name),
			sdklog.String("source", config.Source),
			sdklog.String("ingester", config.Ingester))
	}
}

// onUpdate handles Ingester CRD update events
func (ii *IngesterInformer) onUpdate(oldObj, newObj interface{}) {
	u, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		configLogger.Warn("Failed to convert Ingester CRD to unstructured",
			sdklog.Operation("ingester_update_convert"))
		return
	}

	config := ii.ConvertToIngesterConfig(u)
	if config != nil {
		ii.store.AddOrUpdate(config)
		configLogger.Debug("Updated Ingester config",
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
			u, _ = deleted.Obj.(*unstructured.Unstructured)
		}
		if !ok {
			configLogger.Warn("Failed to convert deleted Ingester CRD to unstructured",
				sdklog.Operation("ingester_delete_convert"))
			return
		}
	}

	namespace := u.GetNamespace()
	name := u.GetName()
	ii.store.Delete(namespace, name)

	configLogger.Debug("Deleted Ingester config",
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
		configLogger.Warn("Ingester CRD missing spec",
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
	spec, ok := u.Object["spec"].(map[string]interface{})
	if !ok {
		configLogger.Warn("Ingester CRD missing spec",
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
		configLogger.Warn("Ingester CRD missing required field: source",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("name", u.GetName()))
		return nil
	}
	config.Source = source

	// Extract ingester type (required field)
	ingester, ingesterOk := spec["ingester"].(string)
	if !ingesterOk || ingester == "" {
		configLogger.Warn("Ingester CRD missing required field: ingester",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("source", source),
			sdklog.String("name", u.GetName()))
		return nil
	}
	config.Ingester = ingester

	// Debug: log spec keys for logs ingester
	if ingester == "logs" {
		configLogger.Info("Processing logs ingester",
			sdklog.Operation("ingester_convert"),
			sdklog.String("source", source),
			sdklog.String("name", u.GetName()),
			sdklog.String("namespace", u.GetNamespace()))
	}

	// Validate destinations (required field)
	destinations, destinationsOk := spec["destinations"].([]interface{})
	if !destinationsOk || len(destinations) == 0 {
		configLogger.Warn("Ingester CRD missing required field: destinations",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("source", source),
			sdklog.String("name", u.GetName()),
			sdklog.String("ingester", ingester))
		return nil
	}

	// Extract destinations and resolve GVRs
	config.Destinations = extractDestinations(destinations, configLogger)

	// Ensure at least one destination was extracted
	if len(config.Destinations) == 0 {
		configLogger.Warn("No valid CRD destinations found",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("source", source))
		return nil
	}

	// Extract source-specific configs
	config.Informer = extractInformerConfig(spec)
	config.Webhook = extractWebhookConfig(spec)
	config.Logs = extractLogsConfig(spec, configLogger, config.Source)

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
			configLogger.Warn("Invalid source entry in spec.sources",
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
			configLogger.Warn("Source missing name or type",
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
			Ingester:      sourceType,                                           // informer, logs, webhook
			Destinations:  sharedConfig.Destinations,
			Normalization: sharedConfig.Normalization,
			Filter:        sharedConfig.Filter,
			Dedup:         sharedConfig.Dedup,
			Processing:    sharedConfig.Processing,
			Optimization:  sharedConfig.Optimization,
		}

		// Extract source-specific config
		extractSourceConfig(config, sourceMap, sourceType, configLogger, namespace, name, sourceName)
		if config.Informer == nil && config.Logs == nil && config.Webhook == nil {
			// No valid source config extracted, skip this source
			continue
		}

		// Validate config has required fields
		if len(config.Destinations) == 0 {
			configLogger.Warn("Source has no valid destinations",
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
	config := &IngesterConfig{
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
	}

	// Extract source (required field)
	source, sourceOk := spec["source"].(string)
	if !sourceOk || source == "" {
		configLogger.Warn("Ingester CRD missing required field: source",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("name", u.GetName()))
		return nil
	}
	config.Source = source

	// Extract ingester type (required field)
	ingester, ingesterOk := spec["ingester"].(string)
	if !ingesterOk || ingester == "" {
		configLogger.Warn("Ingester CRD missing required field: ingester",
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
		configLogger.Warn("Ingester CRD missing required field: destinations",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", u.GetNamespace()),
			sdklog.String("source", source),
			sdklog.String("name", u.GetName()),
			sdklog.String("ingester", ingester))
		return nil
	}

	// Extract destinations and resolve GVRs (reuse existing logic)
	config.Destinations = extractDestinations(destinations, configLogger)

	// Ensure at least one destination was extracted
	if len(config.Destinations) == 0 {
		configLogger.Warn("No valid CRD destinations found",
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
	config.Logs = extractLogsConfig(spec, configLogger, source)

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
		configLogger.Warn("Ingester CRD missing required field: destinations",
			sdklog.Operation("ingester_convert"),
			sdklog.String("namespace", namespace),
			sdklog.String("name", name))
		return nil
	}

	// Extract destinations and resolve GVRs
	config.Destinations = extractDestinations(destinations, configLogger)

	// Ensure at least one destination was extracted
	if len(config.Destinations) == 0 {
		configLogger.Warn("No valid CRD destinations found",
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
