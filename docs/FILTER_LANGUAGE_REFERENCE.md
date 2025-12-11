# Filter Language Reference

zen-watcher supports a powerful expression-based filter language (v1.1) for fine-grained control over which Observations are processed.

## Overview

Filter expressions allow you to combine multiple conditions using logical operators (AND, OR, NOT) and comparison operators. Expressions are evaluated against Observation data and must return a boolean result.

## Syntax

### Basic Structure

```yaml
filters:
  expression: |
    (condition1) AND (condition2) OR (condition3)
```

### Field Access

Fields are accessed using dot notation:

- `spec.severity` - Severity level
- `spec.category` - Category
- `spec.source` - Source name
- `spec.eventType` - Event type
- `spec.details.vulnerabilityID` - Nested field access

## Operators

### Comparison Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `=` | Equality | `spec.severity = "HIGH"` |
| `!=` | Inequality | `spec.category != "ops"` |
| `>` | Greater than | `spec.priority > 0.7` |
| `>=` | Greater than or equal | `spec.severity >= "HIGH"` |
| `<` | Less than | `spec.priority < 0.5` |
| `<=` | Less than or equal | `spec.severity <= "MEDIUM"` |
| `IN` | List membership | `spec.category IN ["security", "compliance"]` |
| `NOT IN` | Not in list | `spec.namespace NOT IN ["kube-system", "default"]` |
| `CONTAINS` | String contains | `spec.message CONTAINS "error"` |
| `STARTS_WITH` | String prefix | `spec.source STARTS_WITH "trivy"` |
| `ENDS_WITH` | String suffix | `spec.eventType ENDS_WITH "_violation"` |
| `EXISTS` | Field exists | `spec.details.vulnerabilityID EXISTS` |
| `NOT EXISTS` | Field missing | `spec.details.vulnerabilityID NOT EXISTS` |

### Logical Operators

| Operator | Description | Precedence |
|----------|-------------|------------|
| `NOT` | Logical negation | Highest |
| `AND` | Logical AND | Medium |
| `OR` | Logical OR | Lowest |

**Precedence**: NOT > AND > OR

Use parentheses to override precedence:

```yaml
filters:
  expression: |
    (spec.severity >= "HIGH") AND (spec.category IN ["security", "compliance"])
```

## Macros

Predefined macros provide convenient shortcuts for common patterns:

| Macro | Equivalent To | Description |
|-------|---------------|-------------|
| `is_critical` | `spec.severity = "CRITICAL"` | Critical severity |
| `is_high` | `spec.severity = "HIGH"` | High severity |
| `is_security` | `spec.category = "security"` | Security category |
| `is_compliance` | `spec.category = "compliance"` | Compliance category |

**Example**:

```yaml
filters:
  expression: |
    is_critical OR (is_high AND is_security)
```

## Severity Comparison

Severity levels are ordered: `CRITICAL > HIGH > MEDIUM > LOW > UNKNOWN`

Comparisons like `>=` and `<=` respect this ordering:

```yaml
filters:
  expression: |
    spec.severity >= "HIGH"  # Matches HIGH, CRITICAL
```

## Examples

### High Severity Security Events

```yaml
filters:
  expression: |
    (spec.severity >= "HIGH") AND (spec.category = "security")
```

### Exclude Specific Namespaces

```yaml
filters:
  expression: |
    spec.namespace NOT IN ["kube-system", "default", "kube-public"]
```

### Complex Logic

```yaml
filters:
  expression: |
    (is_critical OR (is_high AND is_security)) AND
    spec.namespace NOT IN ["kube-system"]
```

### Field Existence Check

```yaml
filters:
  expression: |
    spec.details.vulnerabilityID EXISTS AND
    spec.severity >= "HIGH"
```

### String Matching

```yaml
filters:
  expression: |
    spec.message CONTAINS "CVE" OR
    spec.source STARTS_WITH "trivy"
```

## Backwards Compatibility

Filter expressions are **additive** and **backwards compatible**:

- If `expression` is **not set**, legacy list-based filters are used (`minSeverity`, `includeNamespaces`, etc.)
- If `expression` **is set**, it takes precedence and legacy fields are ignored
- You can migrate gradually: start with legacy filters, then add expressions where needed

### Legacy Filters (Still Supported)

```yaml
filters:
  minSeverity: "HIGH"
  includeNamespaces:
    - production
    - staging
  excludeEventTypes:
    - info
    - debug
```

### Expression-Based Filters (v1.1)

```yaml
filters:
  expression: |
    spec.severity >= "HIGH" AND
    spec.namespace IN ["production", "staging"] AND
    spec.eventType NOT IN ["info", "debug"]
```

## Error Handling

Invalid expressions result in:

1. **Parse errors**: Syntax errors are logged and legacy filters are used as fallback
2. **Evaluation errors**: Runtime errors (e.g., type mismatches) are logged and legacy filters are used as fallback

**Best Practice**: Test expressions in a development environment before deploying to production.

## Performance Considerations

- Expressions are evaluated for each Observation
- Complex expressions with many nested conditions may impact throughput
- Use field existence checks (`EXISTS`) before accessing nested fields to avoid errors

## Related Documentation

- [Ingester API](INGESTER_API.md) - Complete Ingester CRD reference
- [Filtering Guide](FILTERING.md) - Legacy filtering documentation
- [Troubleshooting](TROUBLESHOOTING.md) - Common filter issues

