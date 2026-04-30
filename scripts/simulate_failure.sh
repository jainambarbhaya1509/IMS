#!/bin/bash

# ─────────────────────────────────────────────────────────
# Simulate a cascading failure: RDBMS outage → MCP failure
# This script sends signals, transitions states, and submits RCA
# ─────────────────────────────────────────────────────────

BASE_URL="http://localhost:8080"
echo "🚨 Starting failure simulation..."
echo ""

# ── Phase 1: RDBMS goes down ──────────────────────────────
echo "Phase 1: Simulating RDBMS outage (sending 5 signals)..."
for i in {1..5}; do
  curl -s -X POST "$BASE_URL/signals" \
    -H "Content-Type: application/json" \
    -d "{
      \"component_id\": \"RDBMS\",
      \"severity\": \"P0\",
      \"message\": \"Connection pool exhausted - attempt $i\",
      \"payload\": {
        \"active_connections\": 100,
        \"max_connections\": 100,
        \"waiting_requests\": $((i * 23))
      }
    }" > /dev/null
  echo "  ✓ RDBMS signal $i sent"
  sleep 0.5
done

echo ""
sleep 2

# ── Phase 2: MCP Host fails (cascade from RDBMS) ──────────
echo "Phase 2: Simulating MCP_HOST cascade failure (sending 3 signals)..."
for i in {1..3}; do
  curl -s -X POST "$BASE_URL/signals" \
    -H "Content-Type: application/json" \
    -d "{
      \"component_id\": \"MCP_HOST\",
      \"severity\": \"P1\",
      \"message\": \"MCP Host cannot reach RDBMS - timeout $i\",
      \"payload\": {
        \"timeout_ms\": $((i * 1000)),
        \"retries_attempted\": $i
      }
    }" > /dev/null
  echo "  ✓ MCP_HOST signal $i sent"
  sleep 0.5
done

echo ""
sleep 2

# ── Phase 3: Fetch created incidents ──────────────────────
echo "Phase 3: Fetching created work items..."
WORK_ITEMS=$(curl -s "$BASE_URL/work-items")
echo "$WORK_ITEMS" | python3 -m json.tool 2>/dev/null || echo "$WORK_ITEMS"

# Extract first work item ID (RDBMS one)
WORK_ITEM_ID=$(echo "$WORK_ITEMS" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
echo ""
echo "Working with incident: $WORK_ITEM_ID"

# ── Phase 4: Transition through states ────────────────────
echo ""
echo "Phase 4: Transitioning RDBMS incident through states..."

echo "  → OPEN to INVESTIGATING..."
curl -s -X PATCH "$BASE_URL/work-items/$WORK_ITEM_ID/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "INVESTIGATING"}' > /dev/null
echo "  ✓ Now INVESTIGATING"
sleep 1

echo "  → INVESTIGATING to RESOLVED..."
curl -s -X PATCH "$BASE_URL/work-items/$WORK_ITEM_ID/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "RESOLVED"}' > /dev/null
echo "  ✓ Now RESOLVED"
sleep 1

# ── Phase 5: Try closing without RCA (should fail) ────────
echo ""
echo "Phase 5: Attempting to CLOSE without RCA (should fail)..."
RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH "$BASE_URL/work-items/$WORK_ITEM_ID/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "CLOSED"}')
HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -1)
echo "  Response ($HTTP_CODE): $BODY"

# ── Phase 6: Submit RCA ────────────────────────────────────
echo ""
echo "Phase 6: Submitting RCA..."
START_TIME=$(date -u -v-2H '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '2 hours ago' '+%Y-%m-%dT%H:%M:%SZ')
END_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

curl -s -X POST "$BASE_URL/work-items/$WORK_ITEM_ID/rca" \
  -H "Content-Type: application/json" \
  -d "{
    \"start_time\": \"$START_TIME\",
    \"end_time\": \"$END_TIME\",
    \"root_cause_category\": \"Infrastructure\",
    \"fix_applied\": \"Increased connection pool size from 100 to 500. Restarted RDBMS primary node.\",
    \"prevention_steps\": \"1. Add connection pool monitoring alerts at 80% capacity. 2. Implement read replicas. 3. Add circuit breaker on all DB clients.\"
  }" > /dev/null
echo "  ✓ RCA submitted"
sleep 1

# ── Phase 7: Now close with RCA (should succeed) ──────────
echo ""
echo "Phase 7: Closing incident with RCA (should succeed)..."
RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH "$BASE_URL/work-items/$WORK_ITEM_ID/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "CLOSED"}')
HTTP_CODE=$(echo "$RESPONSE" | tail -1)
echo "  Response ($HTTP_CODE): $([ $HTTP_CODE -eq 200 ] && echo 'Incident closed successfully ✅' || echo 'Failed ❌')"

# ── Phase 8: Check metrics ─────────────────────────────────
echo ""
echo "Phase 8: Checking timeseries metrics..."
curl -s "$BASE_URL/metrics/signals-per-hour" | python3 -m json.tool 2>/dev/null

echo ""
echo "✅ Simulation complete."
echo "   Check http://localhost:8080/work-items for final state"