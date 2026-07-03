#!/bin/bash
# continue-feature.sh — auto-approve gates, run bolts, continue through construction
# Usage: ./continue-feature.sh <feature-id> <bolt-number>
set -e

FEATURE_ID="$1"
BOLT_NUMBER="${2:-1}"
API="http://localhost:18080/api"

get_stage_status() {
    curl -s "$API/features/$FEATURE_ID/stages" | python3 -c "
import json,sys
stages=json.load(sys.stdin)
for s in stages:
    if s['stage_id'].startswith('3.') and s['status'] != 'completed' and s['status'] != 'not_started':
        print(f'{s[\"stage_id\"]} {s[\"status\"]}')
        break
" 2>/dev/null || echo "none"
}

get_bolt_status() {
    curl -s "$API/features/$FEATURE_ID/bolts" | python3 -c "
import json,sys
bolts=json.load(sys.stdin)
print(bolts[0].get('status','none') if bolts else 'none')
" 2>/dev/null || echo "none"
}

echo "=== Current state ==="
echo "Bolt: $(get_bolt_status)"
echo "Stage: $(get_stage_status)"

# If a stage is revising, re-run it first
STAGE_INFO=$(get_stage_status)
if echo "$STAGE_INFO" | grep -q "revising"; then
    STAGE_ID=$(echo "$STAGE_INFO" | awk '{print $1}')
    echo "=== Re-running $STAGE_ID (was revising) ==="
    curl -s -X POST "$API/features/$FEATURE_ID/run-stage" -H "Content-Type: application/json" -d "{\"stage_id\":\"$STAGE_ID\"}"
    echo ""
    # Wait for completion
    while true; do
        sleep 15
        STAGE_INFO=$(get_stage_status)
        echo "$(date +%H:%M:%S) $STAGE_INFO"
        if echo "$STAGE_INFO" | grep -q "awaiting_approval"; then break; fi
        if echo "$STAGE_INFO" | grep -q "none"; then break; fi
    done
fi

# Approve any awaiting_approval stage and continue the bolt
while true; do
    BOLT_STATUS=$(get_bolt_status)
    STAGE_INFO=$(get_stage_status)
    echo "$(date +%H:%M:%S) Bolt: $BOLT_STATUS, Stage: $STAGE_INFO"

    # If bolt completed or failed, we're done
    if [ "$BOLT_STATUS" = "completed" ] || [ "$BOLT_STATUS" = "failed" ]; then
        echo "=== Bolt $BOLT_NUMBER: $BOLT_STATUS ==="
        break
    fi

    # If a stage is awaiting_approval, approve it
    if echo "$STAGE_INFO" | grep -q "awaiting_approval"; then
        STAGE_ID=$(echo "$STAGE_INFO" | awk '{print $1}')
        echo "  Approving $STAGE_ID..."
        curl -s -X POST "$API/features/$FEATURE_ID/stages/$STAGE_ID/approve" > /dev/null
        sleep 1
        # Re-run the bolt to continue
        echo "  Continuing bolt $BOLT_NUMBER..."
        curl -s -X POST "$API/features/$FEATURE_ID/run-bolt/$BOLT_NUMBER" > /dev/null
        sleep 2
        continue
    fi

    # If bolt is pending, run it
    if [ "$BOLT_STATUS" = "pending" ]; then
        echo "  Starting bolt $BOLT_NUMBER..."
        curl -s -X POST "$API/features/$FEATURE_ID/run-bolt/$BOLT_NUMBER" > /dev/null
        sleep 2
        continue
    fi

    # If bolt is in_progress, wait
    if [ "$BOLT_STATUS" = "in_progress" ]; then
        sleep 30
        continue
    fi

    # Unknown state
    echo "  Unknown state, waiting..."
    sleep 15
done

echo ""
echo "=== Final state ==="
curl -s "$API/features/$FEATURE_ID/stages" | python3 -c "
import json,sys
stages=json.load(sys.stdin)
completed = sum(1 for s in stages if s['status'] == 'completed')
print(f'Progress: {completed}/{len(stages)}')
for s in stages:
    if s['stage_id'].startswith('3.'):
        print(f'  {s[\"stage_id\"]:5s} {s[\"status\"]}')
"