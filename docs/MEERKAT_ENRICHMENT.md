# zen-meerkat Enrichment Integration

## Overview

Optional enrichment of CVE events with threat intelligence from zen-meerkat (EPSS scores, KEV status).

## Architecture Decision

**zen-meerkat is SaaS-side** (requires CockroachDB, Redis) and cannot run in customer clusters.

**Solution**: zen-watcher makes **optional API calls** to zen-meerkat SaaS endpoint to enrich CVE events.

## Design Principles

1. **Optional**: Disabled by default (keeps zen-watcher standalone)
2. **Non-blocking**: Enrichment failures don't prevent event creation
3. **Cached**: Cache enrichment results to reduce API calls
4. **Rate-limited**: Respect API rate limits

## Implementation

### 1. Environment Variables

```bash
# Enable zen-meerkat enrichment (default: false)
MEERKAT_ENRICHMENT_ENABLED=false

# zen-meerkat SaaS endpoint (required if enabled)
MEERKAT_API_ENDPOINT=https://api.kube-zen.io

# API key for authentication (optional, uses HMAC if not provided)
MEERKAT_API_KEY=

# Cache TTL for enrichment results (default: 24h)
MEERKAT_CACHE_TTL=24h

# Rate limit: max requests per minute (default: 10)
MEERKAT_RATE_LIMIT=10
```

### 2. Enrichment Flow

```
Trivy finds CVE → Create ZenAgentEvent → (Optional) Enrich with zen-meerkat
                                              ↓
                                    Query: GET /api/v1/threat-events/{cve-id}
                                              ↓
                                    Enrich: Add EPSS, KEV status, risk_score
                                              ↓
                                    Update ZenAgentEvent details
```

### 3. Enrichment Data Added

When zen-meerkat enrichment is enabled, add to `ZenAgentEvent.details`:

```yaml
details:
  vulnerabilityID: CVE-2024-1234
  # ... existing Trivy data ...
  
  # Enriched from zen-meerkat:
  epss: 0.65                    # Exploit probability (0.0-1.0)
  is_kev: true                  # CISA Known Exploited Vulnerability
  risk_score: 95.0              # Composite risk score (0.0-100.0)
  threat_intel_source: meerkat  # Indicates enrichment source
  enriched_at: "2025-01-27T10:00:00Z"
```

### 4. Error Handling

- **API Unavailable**: Log warning, create event without enrichment
- **Rate Limited**: Log warning, retry with backoff
- **CVE Not Found**: Log info, create event without enrichment
- **Timeout**: Log warning, create event without enrichment

### 5. Caching

Cache enrichment results in-memory (map) with TTL:

```go
type EnrichmentCache struct {
    data map[string]*EnrichmentResult
    ttl  time.Duration
    mu   sync.RWMutex
}

type EnrichmentResult struct {
    EPSS      float64
    IsKEV     bool
    RiskScore float64
    CachedAt  time.Time
}
```

### 6. Example Code Structure

```go
// pkg/enrichment/meerkat.go
package enrichment

type MeerkatEnricher struct {
    enabled    bool
    endpoint   string
    apiKey     string
    cache      *EnrichmentCache
    rateLimiter *rate.Limiter
    client     *http.Client
}

func (e *MeerkatEnricher) EnrichCVE(ctx context.Context, cveID string) (*EnrichmentResult, error) {
    if !e.enabled {
        return nil, nil // Enrichment disabled
    }
    
    // Check cache
    if cached := e.cache.Get(cveID); cached != nil {
        return cached, nil
    }
    
    // Rate limit
    if err := e.rateLimiter.Wait(ctx); err != nil {
        return nil, err
    }
    
    // Query zen-meerkat API
    result, err := e.queryAPI(ctx, cveID)
    if err != nil {
        log.Printf("⚠️  Meerkat enrichment failed for %s: %v", cveID, err)
        return nil, err // Non-blocking: return error but don't fail event creation
    }
    
    // Cache result
    e.cache.Set(cveID, result)
    
    return result, nil
}
```

## Usage in zen-watcher

```go
// In Trivy watcher, after creating ZenAgentEvent:

if meerkatEnricher != nil {
    enrichment, err := meerkatEnricher.EnrichCVE(ctx, vulnID)
    if err == nil && enrichment != nil {
        // Update event details with enrichment
        details["epss"] = enrichment.EPSS
        details["is_kev"] = enrichment.IsKEV
        details["risk_score"] = enrichment.RiskScore
        details["threat_intel_source"] = "meerkat"
        details["enriched_at"] = time.Now().Format(time.RFC3339)
        
        // Update severity if KEV (always HIGH/CRITICAL)
        if enrichment.IsKEV && spec["severity"] != "CRITICAL" {
            spec["severity"] = "CRITICAL"
            labels["severity"] = "CRITICAL"
        }
    }
}
```

## Benefits

- ✅ **Enhanced Prioritization**: EPSS + KEV status helps prioritize CVEs
- ✅ **Non-intrusive**: Optional, doesn't break standalone usage
- ✅ **Cached**: Reduces API calls, improves performance
- ✅ **Graceful Degradation**: Works even if zen-meerkat is unavailable

## Future Enhancements

- **Batch Enrichment**: Enrich multiple CVEs in one API call
- **Webhook Push**: zen-meerkat pushes enrichment updates (instead of polling)
- **Local Cache**: Persist cache to disk for restarts

