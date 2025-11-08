package adapters

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kube-zen/zen-watcher/pkg/models"
)

// FalcoAdapter normalizes Falco events to SecurityEvent
type FalcoAdapter struct {
	source string
}

// NewFalcoAdapter creates a new Falco adapter
func NewFalcoAdapter() *FalcoAdapter {
	return &FalcoAdapter{
		source: "falco",
	}
}

// GetSource returns the adapter source name
func (fa *FalcoAdapter) GetSource() string {
	return fa.source
}

// Validate checks if the raw event is a Falco event
func (fa *FalcoAdapter) Validate(rawEvent interface{}) bool {
	switch e := rawEvent.(type) {
	case *models.SecurityEvent:
		return e.Source == "falco" || (e.Details != nil && (e.Details["rule"] != nil || e.Details["priority"] != nil))
	case map[string]interface{}:
		return fa.validateMap(e)
	case string:
		return fa.validateString(e)
	default:
		return false
	}
}

// validateMap validates a map-based Falco event
func (fa *FalcoAdapter) validateMap(event map[string]interface{}) bool {
	// Check for Falco-specific fields
	hasRule := false
	hasPriority := false
	hasMessage := false

	for key := range event {
		keyLower := strings.ToLower(key)
		if strings.Contains(keyLower, "rule") {
			hasRule = true
		}
		if strings.Contains(keyLower, "priority") {
			hasPriority = true
		}
		if strings.Contains(keyLower, "message") {
			hasMessage = true
		}
	}

	return hasPriority || (hasRule && hasMessage)
}

// validateString validates a string-based Falco event (log line)
func (fa *FalcoAdapter) validateString(line string) bool {
	// Check for Falco log patterns
	falcoPatterns := []string{
		"Rule:",
		"Priority:",
		"Notice:",
		"Warning:",
		"Critical:",
		"falco:",
	}

	lineLower := strings.ToLower(line)
	for _, pattern := range falcoPatterns {
		if strings.Contains(lineLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// Normalize converts a Falco event to a normalized SecurityEvent
func (fa *FalcoAdapter) Normalize(ctx context.Context, rawEvent interface{}) (*models.SecurityEvent, error) {
	switch e := rawEvent.(type) {
	case *models.SecurityEvent:
		return fa.normalizeFromModel(e), nil
	case map[string]interface{}:
		return fa.normalizeFromMap(ctx, e)
	case string:
		return fa.normalizeFromString(ctx, e)
	default:
		return nil, fmt.Errorf("unsupported Falco event type: %T", rawEvent)
	}
}

// normalizeFromModel normalizes from an existing SecurityEvent model
func (fa *FalcoAdapter) normalizeFromModel(event *models.SecurityEvent) *models.SecurityEvent {
	// Ensure consistent structure
	normalized := &models.SecurityEvent{
		ID:          fa.generateID(event),
		Source:      fa.source,
		Type:        "runtime",
		Timestamp:   event.Timestamp,
		Severity:    fa.mapPriorityToSeverityFromEvent(event),
		Namespace:   event.Namespace,
		Resource:    event.Resource,
		Description: event.Description,
		Details:     make(map[string]interface{}),
	}

	// Copy existing details
	if event.Details != nil {
		for k, v := range event.Details {
			normalized.Details[k] = v
		}
	}

	// Ensure priority is set in details if severity was calculated from it
	if event.Details != nil {
		if p, ok := event.Details["priority"].(string); ok {
			normalized.Details["priority"] = p
		}
	}

	// Set description if missing
	if normalized.Description == "" {
		if rule, ok := normalized.Details["rule"].(string); ok && rule != "" {
			normalized.Description = fmt.Sprintf("Falco alert: %s", rule)
		} else {
			normalized.Description = "Falco security event detected"
		}
	}

	return normalized
}

// normalizeFromMap normalizes from a map structure
func (fa *FalcoAdapter) normalizeFromMap(ctx context.Context, event map[string]interface{}) (*models.SecurityEvent, error) {
	normalized := &models.SecurityEvent{
		ID:        fa.generateIDFromMap(event),
		Source:    fa.source,
		Type:      "runtime",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Extract common fields
	for k, v := range event {
		kLower := strings.ToLower(k)
		switch kLower {
		case "rule", "falco.rule":
			if str, ok := v.(string); ok {
				normalized.Details["rule"] = str
			}
		case "priority", "falco.priority", "priority_num":
			if priority, ok := fa.extractPriority(v); ok {
				normalized.Details["priority"] = priority
				normalized.Severity = fa.mapPriorityToSeverity(priority)
			}
		case "message", "output", "falco.output":
			if str, ok := v.(string); ok {
				normalized.Description = str
			}
		case "namespace", "k8s.ns.name":
			if str, ok := v.(string); ok {
				normalized.Namespace = str
			}
		case "pod", "k8s.pod.name", "podname":
			if str, ok := v.(string); ok {
				normalized.Resource = str
				normalized.Details["podName"] = str
			}
		case "container.id", "container_id":
			if str, ok := v.(string); ok {
				normalized.Details["containerID"] = str
			}
		case "time", "timestamp", "@timestamp":
			if ts, ok := fa.extractTimestamp(v); ok {
				normalized.Timestamp = ts
			}
		default:
			normalized.Details[k] = v
		}
	}

	// Set defaults
	if normalized.Severity == "" {
		normalized.Severity = "medium"
	}
	if normalized.Description == "" {
		if rule, ok := normalized.Details["rule"].(string); ok {
			normalized.Description = fmt.Sprintf("Falco alert: %s", rule)
		} else {
			normalized.Description = "Falco security event detected"
		}
	}

	return normalized, nil
}

// normalizeFromString normalizes from a log line string
func (fa *FalcoAdapter) normalizeFromString(ctx context.Context, line string) (*models.SecurityEvent, error) {
	normalized := &models.SecurityEvent{
		ID:          fmt.Sprintf("falco-%d", time.Now().UnixNano()),
		Source:      fa.source,
		Type:        "runtime",
		Timestamp:   time.Now(),
		Severity:    "medium",
		Description: line,
		Details:     make(map[string]interface{}),
	}

	// Extract fields from log line
	rule := fa.extractField(line, "Rule:")
	priority := fa.extractField(line, "Priority:")

	if rule != "" {
		normalized.Details["rule"] = rule
	}
	if priority != "" {
		normalized.Details["priority"] = priority
		normalized.Severity = fa.mapPriorityToSeverity(priority)
	}

	source := fa.extractField(line, "Source:")
	if source != "" {
		normalized.Details["source"] = source
	}

	podName := fa.extractField(line, "Pod:")
	if podName != "" {
		normalized.Details["podName"] = podName
		normalized.Resource = podName
	}

	namespace := fa.extractField(line, "Namespace:")
	if namespace != "" {
		normalized.Namespace = namespace
	}

	containerID := fa.extractField(line, "Container:")
	if containerID != "" {
		normalized.Details["containerID"] = containerID
	}

	return normalized, nil
}

// extractPriority extracts priority from various formats
func (fa *FalcoAdapter) extractPriority(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case int, int32, int64:
		// Map numeric priority to string
		return fa.mapNumericPriority(val), true
	case float64:
		return fa.mapNumericPriority(int(val)), true
	default:
		return "", false
	}
}

// mapNumericPriority maps numeric priority to string
func (fa *FalcoAdapter) mapNumericPriority(num interface{}) string {
	var n int
	switch v := num.(type) {
	case int:
		n = v
	case int32:
		n = int(v)
	case int64:
		n = int(v)
	}

	// Falco priority levels: 0=Emergency, 1=Alert, 2=Critical, 3=Error, 4=Warning, 5=Notice, 6=Info, 7=Debug
	switch n {
	case 0, 1, 2:
		return "Critical"
	case 3, 4:
		return "Warning"
	case 5:
		return "Notice"
	case 6:
		return "Info"
	default:
		return "Debug"
	}
}

// extractTimestamp extracts timestamp from various formats
func (fa *FalcoAdapter) extractTimestamp(v interface{}) (time.Time, bool) {
	switch val := v.(type) {
	case time.Time:
		return val, true
	case string:
		// Try parsing RFC3339
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return t, true
		}
		// Try parsing Unix timestamp
		if t, err := time.Parse(time.UnixDate, val); err == nil {
			return t, true
		}
	case int64:
		return time.Unix(val, 0), true
	case float64:
		return time.Unix(int64(val), 0), true
	}
	return time.Now(), false
}

// extractField extracts a field value from a log line
func (fa *FalcoAdapter) extractField(line, field string) string {
	start := strings.Index(line, field)
	if start == -1 {
		return ""
	}

	start += len(field)
	end := strings.Index(line[start:], " ")
	if end == -1 {
		end = len(line) - start
	}

	return strings.TrimSpace(line[start : start+end])
}

// mapPriorityToSeverity maps Falco priority to normalized severity
func (fa *FalcoAdapter) mapPriorityToSeverity(priority string) string {
	priorityUpper := strings.ToUpper(priority)

	switch {
	case strings.Contains(priorityUpper, "EMERGENCY") || strings.Contains(priorityUpper, "ALERT") || strings.Contains(priorityUpper, "CRITICAL"):
		return "critical"
	case strings.Contains(priorityUpper, "ERROR") || strings.Contains(priorityUpper, "WARNING"):
		return "high"
	case strings.Contains(priorityUpper, "NOTICE") || strings.Contains(priorityUpper, "INFO"):
		return "medium"
	default:
		return "low"
	}
}

// generateID generates a unique ID for an event
func (fa *FalcoAdapter) generateID(event *models.SecurityEvent) string {
	if event.ID != "" && strings.HasPrefix(event.ID, "falco-") {
		return event.ID
	}
	return fmt.Sprintf("falco-%d", time.Now().UnixNano())
}

// mapPriorityToSeverityFromEvent extracts priority from event and maps to severity
func (fa *FalcoAdapter) mapPriorityToSeverityFromEvent(event *models.SecurityEvent) string {
	if event.Details != nil {
		if priority, ok := event.Details["priority"].(string); ok {
			return fa.mapPriorityToSeverity(priority)
		}
	}
	return "medium" // Default
}

// generateIDFromMap generates an ID from a map event
func (fa *FalcoAdapter) generateIDFromMap(event map[string]interface{}) string {
	if id, ok := event["id"].(string); ok && id != "" {
		return id
	}
	if id, ok := event["_id"].(string); ok && id != "" {
		return id
	}
	return fmt.Sprintf("falco-%d", time.Now().UnixNano())
}
