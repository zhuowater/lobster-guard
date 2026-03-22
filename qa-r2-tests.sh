#!/bin/bash
# QA Round 2 - Lobster Guard Test Suite
# Usage: Run on remote server via SSH

BASE="http://localhost:9090"
TOKEN="Authorization: Bearer test-token-2026!"
RESULTS_FILE="/tmp/qa-r2-results.txt"

> "$RESULTS_FILE"

log_result() {
    local test_id="$1"
    local test_name="$2"
    local expected="$3"
    local actual="$4"
    local status="$5"
    echo "[$status] $test_id: $test_name | Expected: $expected | Actual: $actual" >> "$RESULTS_FILE"
    echo "[$status] $test_id: $test_name"
}

echo "=== QA Round 2 Tests Starting ==="

#############################################
# A. Fix Verification (Regression Tests)
#############################################

echo "--- Section A: Fix Verification ---"

# Test 1: BUG-004 - bind updates user_count
# First, register a test upstream
echo "Registering test upstreams..."
curl -s -X POST "$BASE/api/v1/upstreams/register" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"id":"qa-upstream-A","address":"10.0.0.100","port":8080}' > /dev/null
curl -s -X POST "$BASE/api/v1/upstreams/register" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"id":"qa-upstream-B","address":"10.0.0.101","port":8080}' > /dev/null

sleep 0.5

# Get initial user_count for qa-upstream-A
INITIAL_COUNT=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-A':
        print(u.get('user_count',0))
        break
else:
    print(0)
")
echo "Initial user_count for qa-upstream-A: $INITIAL_COUNT"

# Test 1a: Bind new user → user_count +1
curl -s -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"qa-test-user-1","upstream_id":"qa-upstream-A","app_id":"qa-test"}' > /dev/null
sleep 0.3
COUNT_AFTER_BIND=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-A':
        print(u.get('user_count',0))
        break
else:
    print(0)
")
EXPECTED=$((INITIAL_COUNT + 1))
if [ "$COUNT_AFTER_BIND" = "$EXPECTED" ]; then
    log_result "T1a" "BUG-004: bind +1 user_count" "$EXPECTED" "$COUNT_AFTER_BIND" "PASS"
else
    log_result "T1a" "BUG-004: bind +1 user_count" "$EXPECTED" "$COUNT_AFTER_BIND" "FAIL"
fi

# Test 1b: Unbind → user_count -1
curl -s -X POST "$BASE/api/v1/unbind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"qa-test-user-1","app_id":"qa-test"}' > /dev/null
sleep 0.3
COUNT_AFTER_UNBIND=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-A':
        print(u.get('user_count',0))
        break
else:
    print(0)
")
if [ "$COUNT_AFTER_UNBIND" = "$INITIAL_COUNT" ]; then
    log_result "T1b" "BUG-004: unbind -1 user_count" "$INITIAL_COUNT" "$COUNT_AFTER_UNBIND" "PASS"
else
    log_result "T1b" "BUG-004: unbind -1 user_count" "$INITIAL_COUNT" "$COUNT_AFTER_UNBIND" "FAIL"
fi

# Test 1c: Batch bind 5 users → user_count +5
curl -s -X POST "$BASE/api/v1/batch-bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"entries":[
    {"sender_id":"qa-batch-1","upstream_id":"qa-upstream-A","app_id":"qa-test"},
    {"sender_id":"qa-batch-2","upstream_id":"qa-upstream-A","app_id":"qa-test"},
    {"sender_id":"qa-batch-3","upstream_id":"qa-upstream-A","app_id":"qa-test"},
    {"sender_id":"qa-batch-4","upstream_id":"qa-upstream-A","app_id":"qa-test"},
    {"sender_id":"qa-batch-5","upstream_id":"qa-upstream-A","app_id":"qa-test"}
  ]}' > /dev/null
sleep 0.3
COUNT_AFTER_BATCH=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-A':
        print(u.get('user_count',0))
        break
else:
    print(0)
")
EXPECTED=$((INITIAL_COUNT + 5))
if [ "$COUNT_AFTER_BATCH" = "$EXPECTED" ]; then
    log_result "T1c" "BUG-004: batch-bind +5 user_count" "$EXPECTED" "$COUNT_AFTER_BATCH" "PASS"
else
    log_result "T1c" "BUG-004: batch-bind +5 user_count" "$EXPECTED" "$COUNT_AFTER_BATCH" "FAIL"
fi

# Test 1d: Migrate user (rebind to different upstream) → old -1, new +1
curl -s -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"qa-batch-1","upstream_id":"qa-upstream-B","app_id":"qa-test"}' > /dev/null
sleep 0.3
COUNT_A_AFTER_MIGRATE=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-A':
        print(u.get('user_count',0))
        break
else:
    print(0)
")
COUNT_B_AFTER_MIGRATE=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-B':
        print(u.get('user_count',0))
        break
else:
    print(0)
")
EXPECTED_A=$((INITIAL_COUNT + 4))
EXPECTED_B=1
if [ "$COUNT_A_AFTER_MIGRATE" = "$EXPECTED_A" ] && [ "$COUNT_B_AFTER_MIGRATE" = "$EXPECTED_B" ]; then
    log_result "T1d" "BUG-004: migrate old-1,new+1" "A=$EXPECTED_A,B=$EXPECTED_B" "A=$COUNT_A_AFTER_MIGRATE,B=$COUNT_B_AFTER_MIGRATE" "PASS"
else
    log_result "T1d" "BUG-004: migrate old-1,new+1" "A=$EXPECTED_A,B=$EXPECTED_B" "A=$COUNT_A_AFTER_MIGRATE,B=$COUNT_B_AFTER_MIGRATE" "FAIL"
fi

# Test 2: BUG-001 - Duplicate policy returns 409
# First create a policy
curl -s -X POST "$BASE/api/v1/policy" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"match":{"department":"QA测试部"},"upstream_id":"qa-upstream-A","priority":10}' > /dev/null
sleep 0.3
# Try to create duplicate
DUP_CODE=$(curl -s -w '%{http_code}' -o /tmp/dup_resp.txt -X POST "$BASE/api/v1/policy" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"match":{"department":"QA测试部"},"upstream_id":"qa-upstream-A","priority":10}')
DUP_BODY=$(cat /tmp/dup_resp.txt)
if [ "$DUP_CODE" = "409" ]; then
    log_result "T2" "BUG-001: duplicate policy → 409" "409" "$DUP_CODE" "PASS"
else
    log_result "T2" "BUG-001: duplicate policy → 409" "409" "$DUP_CODE ($DUP_BODY)" "FAIL"
fi

# Test 3: BUG-002 - Bind to non-existent upstream → 400
FAKE_CODE=$(curl -s -w '%{http_code}' -o /tmp/fake_resp.txt -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"qa-fake-test","upstream_id":"fake-upstream-xyz","app_id":"qa-test"}')
FAKE_BODY=$(cat /tmp/fake_resp.txt)
if [ "$FAKE_CODE" = "400" ]; then
    log_result "T3" "BUG-002: bind non-existent upstream → 400" "400" "$FAKE_CODE" "PASS"
else
    log_result "T3" "BUG-002: bind non-existent upstream → 400" "400" "$FAKE_CODE ($FAKE_BODY)" "FAIL"
fi

# Test 4: BUG-005 - Register upstream → metrics healthy +1
HEALTHY_BEFORE=$(curl -s "$BASE/metrics" | grep 'lobster_upstreams_healthy' | grep -v '#' | awk '{print $2}')
curl -s -X POST "$BASE/api/v1/upstreams/register" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"id":"qa-metrics-test","address":"10.0.0.200","port":8080}' > /dev/null
sleep 0.5
HEALTHY_AFTER=$(curl -s "$BASE/metrics" | grep 'lobster_upstreams_healthy' | grep -v '#' | awk '{print $2}')
if [ -n "$HEALTHY_BEFORE" ] && [ -n "$HEALTHY_AFTER" ]; then
    DIFF=$(echo "$HEALTHY_AFTER - $HEALTHY_BEFORE" | bc 2>/dev/null || echo "calc_err")
    if [ "$DIFF" = "1" ]; then
        log_result "T4" "BUG-005: register → metrics healthy +1" "+1" "+$DIFF" "PASS"
    else
        log_result "T4" "BUG-005: register → metrics healthy +1" "+1" "before=$HEALTHY_BEFORE,after=$HEALTHY_AFTER" "FAIL"
    fi
else
    log_result "T4" "BUG-005: register → metrics healthy +1" "+1" "before=$HEALTHY_BEFORE,after=$HEALTHY_AFTER" "FAIL"
fi

# Test 5: BUG-012 - healthz not degraded within 5 min
HEALTHZ=$(curl -s "$BASE/healthz" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','?'))")
if [ "$HEALTHZ" = "healthy" ]; then
    log_result "T5" "BUG-012: healthz not degraded" "healthy" "$HEALTHZ" "PASS"
else
    log_result "T5" "BUG-012: healthz not degraded" "healthy" "$HEALTHZ" "FAIL"
fi

#############################################
# D. Abnormal Input Tests
#############################################
echo "--- Section D: Abnormal Input Tests ---"

# Test 13: Empty sender_id
T13_CODE=$(curl -s -w '%{http_code}' -o /tmp/t13.txt -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{}')
T13_BODY=$(cat /tmp/t13.txt)
if [ "$T13_CODE" = "400" ]; then
    log_result "T13" "Empty body bind → 400" "400" "$T13_CODE" "PASS"
else
    log_result "T13" "Empty body bind → 400" "400" "$T13_CODE ($T13_BODY)" "FAIL"
fi

# Test 14: Super long sender_id (1000 chars)
LONG_ID=$(python3 -c "print('x'*1000)")
T14_CODE=$(curl -s -w '%{http_code}' -o /tmp/t14.txt -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d "{\"sender_id\":\"$LONG_ID\",\"upstream_id\":\"qa-upstream-A\",\"app_id\":\"qa-test\"}")
T14_BODY=$(cat /tmp/t14.txt)
if [ "$T14_CODE" = "400" ] || [ "$T14_CODE" = "200" ]; then
    log_result "T14" "1000-char sender_id" "400 or 200" "$T14_CODE" "PASS"
else
    log_result "T14" "1000-char sender_id" "400 or 200" "$T14_CODE ($T14_BODY)" "FAIL"
fi

# Test 15: Special characters in sender_id
# Chinese
T15a_CODE=$(curl -s -w '%{http_code}' -o /tmp/t15a.txt -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"用户中文名","upstream_id":"qa-upstream-A","app_id":"qa-test"}')
# Emoji
T15b_CODE=$(curl -s -w '%{http_code}' -o /tmp/t15b.txt -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"user-🦞-test","upstream_id":"qa-upstream-A","app_id":"qa-test"}')
# SQL injection
T15c_CODE=$(curl -s -w '%{http_code}' -o /tmp/t15c.txt -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d "{\"sender_id\":\"'; DROP TABLE users; --\",\"upstream_id\":\"qa-upstream-A\",\"app_id\":\"qa-test\"}")

HEALTHZ_AFTER_T15=$(curl -s "$BASE/healthz" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','?'))")
if [ "$HEALTHZ_AFTER_T15" = "healthy" ]; then
    log_result "T15" "Special chars (中文/emoji/SQL inject) - no crash" "healthy" "healthy, codes=$T15a_CODE,$T15b_CODE,$T15c_CODE" "PASS"
else
    log_result "T15" "Special chars (中文/emoji/SQL inject) - no crash" "healthy" "$HEALTHZ_AFTER_T15, codes=$T15a_CODE,$T15b_CODE,$T15c_CODE" "FAIL"
fi

# Test 16: Empty upstream_id
T16_CODE=$(curl -s -w '%{http_code}' -o /tmp/t16.txt -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"qa-test-empty-upstream","upstream_id":"","app_id":"qa-test"}')
T16_BODY=$(cat /tmp/t16.txt)
if [ "$T16_CODE" = "400" ]; then
    log_result "T16" "Empty upstream_id → 400" "400" "$T16_CODE" "PASS"
else
    log_result "T16" "Empty upstream_id → 400" "400" "$T16_CODE ($T16_BODY)" "FAIL"
fi

# Test 17: Re-bind same user to different upstream
curl -s -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"qa-rebind-test","upstream_id":"qa-upstream-A","app_id":"qa-test"}' > /dev/null
sleep 0.3
COUNT_A_BEFORE_REBIND=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-A': print(u.get('user_count',0)); break
else: print(-1)
")
curl -s -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"qa-rebind-test","upstream_id":"qa-upstream-B","app_id":"qa-test"}' > /dev/null
sleep 0.3
COUNT_A_AFTER_REBIND=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-A': print(u.get('user_count',0)); break
else: print(-1)
")
COUNT_B_AFTER_REBIND=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-B': print(u.get('user_count',0)); break
else: print(-1)
")
EXPECTED_A_REBIND=$((COUNT_A_BEFORE_REBIND - 1))
if [ "$COUNT_A_AFTER_REBIND" = "$EXPECTED_A_REBIND" ]; then
    log_result "T17" "Re-bind adjusts user_count" "A=$EXPECTED_A_REBIND" "A=$COUNT_A_AFTER_REBIND,B=$COUNT_B_AFTER_REBIND" "PASS"
else
    log_result "T17" "Re-bind adjusts user_count" "A=$EXPECTED_A_REBIND" "A=$COUNT_A_AFTER_REBIND,B=$COUNT_B_AFTER_REBIND" "FAIL"
fi

# Test 18: bind + unbind rapid → user_count returns to initial
COUNT_A_BEFORE_T18=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-A': print(u.get('user_count',0)); break
else: print(-1)
")
curl -s -X POST "$BASE/api/v1/bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"qa-rapid-test","upstream_id":"qa-upstream-A","app_id":"qa-test"}' > /dev/null
curl -s -X POST "$BASE/api/v1/unbind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"sender_id":"qa-rapid-test","app_id":"qa-test"}' > /dev/null
sleep 0.3
COUNT_A_AFTER_T18=$(curl -s -H "$TOKEN" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='qa-upstream-A': print(u.get('user_count',0)); break
else: print(-1)
")
if [ "$COUNT_A_AFTER_T18" = "$COUNT_A_BEFORE_T18" ]; then
    log_result "T18" "bind+unbind rapid → count unchanged" "$COUNT_A_BEFORE_T18" "$COUNT_A_AFTER_T18" "PASS"
else
    log_result "T18" "bind+unbind rapid → count unchanged" "$COUNT_A_BEFORE_T18" "$COUNT_A_AFTER_T18" "FAIL"
fi

# Test 19: Batch-bind empty list
T19_CODE=$(curl -s -w '%{http_code}' -o /tmp/t19.txt -X POST "$BASE/api/v1/batch-bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"entries":[]}')
T19_BODY=$(cat /tmp/t19.txt)
if [ "$T19_CODE" = "200" ] || [ "$T19_CODE" = "400" ]; then
    log_result "T19" "Batch-bind empty list" "200 or 400" "$T19_CODE" "PASS"
else
    log_result "T19" "Batch-bind empty list" "200 or 400" "$T19_CODE ($T19_BODY)" "FAIL"
fi

# Test 20: Batch-bind with mixed valid/invalid upstreams
T20_RESP=$(curl -s -w '\nHTTP_CODE:%{http_code}' -X POST "$BASE/api/v1/batch-bind" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"entries":[
    {"sender_id":"qa-mixed-1","upstream_id":"qa-upstream-A","app_id":"qa-test"},
    {"sender_id":"qa-mixed-2","upstream_id":"nonexistent-upstream","app_id":"qa-test"},
    {"sender_id":"qa-mixed-3","upstream_id":"qa-upstream-A","app_id":"qa-test"}
  ]}')
T20_CODE=$(echo "$T20_RESP" | grep 'HTTP_CODE:' | sed 's/HTTP_CODE://')
T20_BODY=$(echo "$T20_RESP" | grep -v 'HTTP_CODE:')
log_result "T20" "Batch-bind mixed valid/invalid" "partial success or 400" "$T20_CODE body=$T20_BODY" "INFO"

#############################################
# F. Upstream Management
#############################################
echo "--- Section F: Upstream Management ---"

# Test 26: Register duplicate upstream (should update/heartbeat)
T26_CODE1=$(curl -s -w '%{http_code}' -o /dev/null -X POST "$BASE/api/v1/upstreams/register" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"id":"qa-dup-upstream","address":"10.0.0.50","port":8080}')
T26_CODE2=$(curl -s -w '%{http_code}' -o /tmp/t26.txt -X POST "$BASE/api/v1/upstreams/register" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"id":"qa-dup-upstream","address":"10.0.0.50","port":8080}')
T26_BODY=$(cat /tmp/t26.txt)
if [ "$T26_CODE2" = "200" ]; then
    log_result "T26" "Duplicate upstream register → update" "200" "$T26_CODE2" "PASS"
else
    log_result "T26" "Duplicate upstream register → update" "200" "$T26_CODE2 ($T26_BODY)" "FAIL"
fi

# Test 27: Register without address
T27_CODE=$(curl -s -w '%{http_code}' -o /tmp/t27.txt -X POST "$BASE/api/v1/upstreams/register" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"id":"qa-no-address","port":8080}')
T27_BODY=$(cat /tmp/t27.txt)
if [ "$T27_CODE" = "400" ]; then
    log_result "T27" "Register without address → 400" "400" "$T27_CODE" "PASS"
else
    log_result "T27" "Register without address → 400" "400" "$T27_CODE ($T27_BODY)" "FAIL"
fi

# Test 29: Upstream ID with special chars
T29a_CODE=$(curl -s -w '%{http_code}' -o /tmp/t29a.txt -X POST "$BASE/api/v1/upstreams/register" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"id":"upstream with spaces","address":"10.0.0.60","port":8080}')
T29b_CODE=$(curl -s -w '%{http_code}' -o /tmp/t29b.txt -X POST "$BASE/api/v1/upstreams/register" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"id":"upstream/with/slashes","address":"10.0.0.61","port":8080}')
T29c_CODE=$(curl -s -w '%{http_code}' -o /tmp/t29c.txt -X POST "$BASE/api/v1/upstreams/register" -H "$TOKEN" -H "Content-Type: application/json" \
  -d '{"id":"上游中文名","address":"10.0.0.62","port":8080}')

HEALTHZ_AFTER_T29=$(curl -s "$BASE/healthz" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','?'))")
log_result "T29" "Upstream ID special chars" "no crash" "codes=$T29a_CODE/$T29b_CODE/$T29c_CODE, health=$HEALTHZ_AFTER_T29" "INFO"

#############################################
# G. Auth & Audit
#############################################
echo "--- Section G: Auth & Audit ---"

# Test 31: No token
T31_CODE=$(curl -s -w '%{http_code}' -o /tmp/t31.txt "$BASE/api/v1/upstreams")
if [ "$T31_CODE" = "401" ]; then
    log_result "T31" "No token → 401" "401" "$T31_CODE" "PASS"
else
    log_result "T31" "No token → 401" "401" "$T31_CODE" "FAIL"
fi

# Test 32: Wrong token
T32_CODE=$(curl -s -w '%{http_code}' -o /tmp/t32.txt -H "Authorization: Bearer wrong-token" "$BASE/api/v1/upstreams")
if [ "$T32_CODE" = "401" ]; then
    log_result "T32" "Wrong token → 401" "401" "$T32_CODE" "PASS"
else
    log_result "T32" "Wrong token → 401" "401" "$T32_CODE" "FAIL"
fi

# Test 33: Audit log query
T33_START=$(date -d '30 days ago' '+%Y-%m-%dT00:00:00Z' 2>/dev/null || echo '2026-02-20T00:00:00Z')
T33_START_MS=$(python3 -c "
import time
from datetime import datetime, timedelta
t = datetime.utcnow() - timedelta(days=30)
print(int(t.timestamp()*1000))
")
T33_RESP=$(curl -s -w '\nHTTP_CODE:%{http_code}' -H "$TOKEN" "$BASE/api/v1/audit?start=$T33_START_MS&limit=50")
T33_CODE=$(echo "$T33_RESP" | grep 'HTTP_CODE:' | sed 's/HTTP_CODE://')
T33_BODY=$(echo "$T33_RESP" | grep -v 'HTTP_CODE:' | head -5)
if [ "$T33_CODE" = "200" ]; then
    log_result "T33" "Audit log large range query" "200" "$T33_CODE" "PASS"
else
    log_result "T33" "Audit log large range query" "200" "$T33_CODE ($T33_BODY)" "FAIL"
fi

# Test 34: Audit log pagination
T34a_RESP=$(curl -s -H "$TOKEN" "$BASE/api/v1/audit?limit=10&offset=0")
T34b_RESP=$(curl -s -H "$TOKEN" "$BASE/api/v1/audit?limit=10&offset=10")
T34a_COUNT=$(echo "$T34a_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('entries',d.get('events',d.get('logs',[])))))" 2>/dev/null || echo "parse_err")
T34b_COUNT=$(echo "$T34b_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('entries',d.get('events',d.get('logs',[])))))" 2>/dev/null || echo "parse_err")
log_result "T34" "Audit pagination" "10/10 per page" "page1=$T34a_COUNT,page2=$T34b_COUNT" "INFO"

echo ""
echo "=== Results Summary ==="
cat "$RESULTS_FILE"
echo ""
echo "PASS count: $(grep -c '\[PASS\]' $RESULTS_FILE)"
echo "FAIL count: $(grep -c '\[FAIL\]' $RESULTS_FILE)"
echo "INFO count: $(grep -c '\[INFO\]' $RESULTS_FILE)"
