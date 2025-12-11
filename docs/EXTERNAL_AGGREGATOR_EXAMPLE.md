# External Aggregator Example

This document shows how to build an external aggregator that reads Observations from multiple clusters without requiring changes to zen-watcher core.

## Overview

The external aggregator is a separate component that:
- Reads Observations from multiple clusters via Kubernetes API
- Aggregates data across clusters
- Writes to external stores (Elasticsearch, Postgres, etc.)

**Important**: This is not part of zen-watcher. It's an example pattern for operators.

## Stub Implementation

A minimal stub implementation is provided in `examples/aggregator/`.

### Building

```bash
cd zen-watcher/examples/aggregator
go build -o observation-aggregator
```

### Usage

```bash
# Aggregate from multiple clusters
./observation-aggregator \
  --kubeconfigs=/path/to/cluster1/config,/path/to/cluster2/config \
  --namespace=default \
  --interval=1m
```

## Extending the Stub

### Add External Sink

```go
type Sink interface {
    Write(ctx context.Context, obs []unstructured.Unstructured) error
}

type ElasticsearchSink struct {
    endpoint string
    index    string
}

func (s *ElasticsearchSink) Write(ctx context.Context, obs []unstructured.Unstructured) error {
    // Write to Elasticsearch
    return nil
}
```

### Add Filtering

```go
func filterObservations(obs []unstructured.Unstructured, filters map[string]string) []unstructured.Unstructured {
    filtered := make([]unstructured.Unstructured, 0)
    for _, o := range obs {
        if matchesFilters(o, filters) {
            filtered = append(filtered, o)
        }
    }
    return filtered
}
```

### Add Aggregation Logic

```go
type Aggregation struct {
    Source   string
    Severity string
    Count    int
    Clusters []string
}

func aggregateObservations(obs []unstructured.Unstructured) []Aggregation {
    // Aggregate by source and severity
    // Track which clusters contributed
    return aggregations
}
```

## Use Cases

### Centralized SIEM

Aggregate Observations from multiple clusters to a central SIEM:

```go
sink := &ElasticsearchSink{
    endpoint: "https://siem.example.com:9200",
    index:    "observations",
}

for _, cluster := range clusters {
    obs := readObservations(cluster)
    sink.Write(ctx, obs)
}
```

### Cross-Cluster Analytics

Analyze Observations across clusters for trends:

```go
aggregations := aggregateObservations(allObservations)
for _, agg := range aggregations {
    fmt.Printf("%s/%s: %d observations across %d clusters\n",
        agg.Source, agg.Severity, agg.Count, len(agg.Clusters))
}
```

### Compliance Reporting

Aggregate compliance Observations for reporting:

```go
complianceObs := filterObservations(allObservations, map[string]string{
    "zen.io/category": "compliance",
})

generateReport(complianceObs)
```

## Related Documentation

- [Observation API Public Guide](OBSERVATION_API_PUBLIC_GUIDE.md) - Complete Observation CRD API reference
- [Multi-Team RBAC Patterns](MULTI_TEAM_RBAC_PATTERNS.md) - RBAC patterns for multi-team clusters

