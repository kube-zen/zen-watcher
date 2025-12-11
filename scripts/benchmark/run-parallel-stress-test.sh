#!/usr/bin/env bash
# Run multiple stress test jobs in parallel
# Usage: ./run-parallel-stress-test.sh [--jobs N] [--rate N] [--qps N] [--client-burst N] [--skip-validation] [--skip-cleanup]

set -euo pipefail

KUBECTL_CONTEXT="${KUBECTL_CONTEXT:-par-dev-eks-1}"
NAMESPACE="${NAMESPACE:-zen-system}"
NUM_JOBS="${NUM_JOBS:-5}"
RATE_PER_JOB="${RATE_PER_JOB:-2000}"
QPS="${QPS:-500}"
CLIENT_BURST="${CLIENT_BURST:-1000}"
DURATION="${DURATION:-60s}"
WORKERS="${WORKERS:-20}"
SKIP_VALIDATION="${SKIP_VALIDATION:-false}"
SKIP_CLEANUP="${SKIP_CLEANUP:-false}"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --jobs)
            NUM_JOBS="$2"
            shift 2
            ;;
        --rate)
            RATE_PER_JOB="$2"
            shift 2
            ;;
        --qps)
            QPS="$2"
            shift 2
            ;;
        --client-burst)
            CLIENT_BURST="$2"
            shift 2
            ;;
        --duration)
            DURATION="$2"
            shift 2
            ;;
        --workers)
            WORKERS="$2"
            shift 2
            ;;
        --skip-validation)
            SKIP_VALIDATION="true"
            shift
            ;;
        --skip-cleanup)
            SKIP_CLEANUP="true"
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Function to run a validation test
run_validation_test() {
    local val_rate="${1:-50}"
    local val_duration="${2:-10s}"
    local val_workers="${3:-5}"
    
    echo "=== Validation Test ==="
    echo "Running minimal test: ${val_rate} obs/sec for ${val_duration}"
    echo ""
    
    # Clean up any existing validation jobs
    kubectl --context="$KUBECTL_CONTEXT" delete job -n "$NAMESPACE" zen-watcher-stress-test-validation --ignore-not-found=true >/dev/null 2>&1
    sleep 1
    
    # Create validation job
    kubectl --context="$KUBECTL_CONTEXT" apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: zen-watcher-stress-test-validation
  namespace: ${NAMESPACE}
spec:
  ttlSecondsAfterFinished: 60
  template:
    metadata:
      labels:
        app: zen-watcher-stress-test
        test-type: validation
    spec:
      serviceAccountName: zen-watcher-stress-test
      restartPolicy: Never
      containers:
      - name: stress-test
        image: kubezen/zen-watcher-stress-test:latest
        imagePullPolicy: Always
        args:
        - -namespace
        - ${NAMESPACE}
        - -rate
        - "${val_rate}"
        - -duration
        - "${val_duration}"
        - -workers
        - "${val_workers}"
        - -burst
        - "100"
        - -qps
        - "$((QPS / 2))"
        - -client-burst
        - "$((CLIENT_BURST / 2))"
EOF
    
    # Wait for completion
    echo "Waiting for validation test to complete..."
    if kubectl --context="$KUBECTL_CONTEXT" wait --for=condition=complete --timeout=30s -n "$NAMESPACE" job/zen-watcher-stress-test-validation >/dev/null 2>&1; then
        POD=$(kubectl --context="$KUBECTL_CONTEXT" get pods -n "$NAMESPACE" -l job-name=zen-watcher-stress-test-validation -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
        if [ -n "$POD" ]; then
            RESULTS=$(kubectl --context="$KUBECTL_CONTEXT" logs -n "$NAMESPACE" "$POD" 2>&1 | grep -E "Actual rate:|Errors:" | head -2)
            echo "$RESULTS"
            echo ""
            
            # Check if validation passed
            ACTUAL_RATE=$(echo "$RESULTS" | grep "Actual rate:" | awk '{print $3}' || echo "0")
            TARGET_RATE=$val_rate
            if [ -n "$ACTUAL_RATE" ] && [ "$ACTUAL_RATE" != "0" ]; then
                # Check if within 20% of target
                RATE_CHECK=$(awk "BEGIN {if ($ACTUAL_RATE >= $TARGET_RATE * 0.8) print 1; else print 0}")
                if [ "$RATE_CHECK" = "1" ]; then
                    echo "✅ Validation passed: ${ACTUAL_RATE} obs/sec (target: ${TARGET_RATE})"
                    echo ""
                    return 0
                else
                    echo "❌ Validation failed: ${ACTUAL_RATE} obs/sec (target: ${TARGET_RATE})"
                    echo ""
                    return 1
                fi
            fi
        fi
    else
        echo "❌ Validation test timed out or failed"
        echo ""
        return 1
    fi
    
    # Cleanup validation job
    kubectl --context="$KUBECTL_CONTEXT" delete job -n "$NAMESPACE" zen-watcher-stress-test-validation --ignore-not-found=true >/dev/null 2>&1
    return 0
}

# Function to cleanup observations
cleanup_observations() {
    echo "=== Cleanup ==="
    echo "Deleting observations in ${NAMESPACE}..."
    DELETED=$(kubectl --context="$KUBECTL_CONTEXT" delete observations -n "$NAMESPACE" --all --timeout=60s 2>&1 | grep -c "deleted" || echo "0")
    echo "Deleted observations: ${DELETED}"
    echo ""
}

echo "=== Parallel Stress Test Configuration ==="
echo "Jobs: $NUM_JOBS"
echo "Rate per job: $RATE_PER_JOB obs/sec"
echo "Total target rate: $((NUM_JOBS * RATE_PER_JOB)) obs/sec"
echo "Client QPS per job: $QPS"
echo "Client Burst per job: $CLIENT_BURST"
echo "Duration: $DURATION"
echo "Workers per job: $WORKERS"
echo ""

# Run validation test unless skipped
if [ "$SKIP_VALIDATION" != "true" ]; then
    VAL_RATE=$((RATE_PER_JOB / 10))
    if [ $VAL_RATE -lt 10 ]; then
        VAL_RATE=10
    fi
    if [ $VAL_RATE -gt 100 ]; then
        VAL_RATE=100
    fi
    
    if ! run_validation_test "$VAL_RATE" "10s" "5"; then
        echo "Validation failed. Exiting."
        exit 1
    fi
fi

# Clean up any existing jobs
echo "Cleaning up existing stress test jobs..."
kubectl --context="$KUBECTL_CONTEXT" delete job -n "$NAMESPACE" -l app=zen-watcher-stress-test --ignore-not-found=true 2>&1 | head -5
sleep 2

# Create jobs
echo "Creating $NUM_JOBS parallel stress test jobs..."
for i in $(seq 1 $NUM_JOBS); do
    kubectl --context="$KUBECTL_CONTEXT" create job -n "$NAMESPACE" "zen-watcher-stress-test-${i}" \
        --from=job/zen-watcher-stress-test 2>&1 | grep -v "already exists" || \
    kubectl --context="$KUBECTL_CONTEXT" apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: zen-watcher-stress-test-${i}
  namespace: ${NAMESPACE}
spec:
  ttlSecondsAfterFinished: 300
  template:
    metadata:
      labels:
        app: zen-watcher-stress-test
        job-id: "${i}"
    spec:
      serviceAccountName: zen-watcher-stress-test
      restartPolicy: Never
      containers:
      - name: stress-test
        image: kubezen/zen-watcher-stress-test:latest
        imagePullPolicy: Always
        args:
        - -namespace
        - ${NAMESPACE}
        - -rate
        - "${RATE_PER_JOB}"
        - -duration
        - "${DURATION}"
        - -workers
        - "${WORKERS}"
        - -burst
        - "100"
        - -qps
        - "${QPS}"
        - -client-burst
        - "${CLIENT_BURST}"
EOF
done

echo ""
echo "Waiting for jobs to start..."
sleep 5

# Wait for all jobs to complete
echo "Waiting for all jobs to complete..."
kubectl --context="$KUBECTL_CONTEXT" wait --for=condition=complete --timeout=180s \
    -n "$NAMESPACE" job -l app=zen-watcher-stress-test 2>&1 | tail -5

echo ""
echo "=== Results ==="

# Aggregate results from all jobs
TOTAL_CREATED=0
TOTAL_ERRORS=0
TOTAL_DURATION=0

for i in $(seq 1 $NUM_JOBS); do
    JOB_NAME="zen-watcher-stress-test-${i}"
    POD_NAME=$(kubectl --context="$KUBECTL_CONTEXT" get pods -n "$NAMESPACE" -l job-name="$JOB_NAME" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    if [ -n "$POD_NAME" ]; then
        RESULTS=$(kubectl --context="$KUBECTL_CONTEXT" logs -n "$NAMESPACE" "$POD_NAME" 2>&1 | grep -E "Created:|Errors:|Actual rate:" | head -3)
        CREATED=$(echo "$RESULTS" | grep "Created:" | awk '{print $2}' || echo "0")
        ERRORS=$(echo "$RESULTS" | grep "Errors:" | awk '{print $2}' || echo "0")
        RATE=$(echo "$RESULTS" | grep "Actual rate:" | awk '{print $3}' || echo "0")
        
        echo "Job $i: Created=$CREATED, Errors=$ERRORS, Rate=$RATE obs/sec"
        
        TOTAL_CREATED=$((TOTAL_CREATED + CREATED))
        TOTAL_ERRORS=$((TOTAL_ERRORS + ERRORS))
    fi
done

echo ""
echo "=== Aggregate Results ==="
echo "Total Created: $TOTAL_CREATED observations"
echo "Total Errors: $TOTAL_ERRORS"
if [ "$TOTAL_CREATED" -gt 0 ]; then
    # Calculate actual duration from first and last job
    DURATION_SEC=$(echo "$DURATION" | sed 's/s$//')
    if [ -z "$DURATION_SEC" ] || [ "$DURATION_SEC" = "$DURATION" ]; then
        DURATION_SEC=60  # Default to 60s
    fi
    AVG_RATE=$(awk "BEGIN {printf \"%.2f\", $TOTAL_CREATED / $DURATION_SEC}")
    echo "Average Rate: ${AVG_RATE} obs/sec (across all jobs)"
    echo "Target Rate: $((NUM_JOBS * RATE_PER_JOB)) obs/sec"
fi

# Cleanup observations unless skipped
if [ "$SKIP_CLEANUP" != "true" ]; then
    cleanup_observations
fi

