#!/bin/bash
# QA Round 2 - Full Test Suite with correct API endpoints
set -o pipefail

BASE="http://localhost:9090"
HDR="Authorization: Bearer test-token-2026!"
CT="Content-Type: application/json"
RESULTS="/tmp/qa-r2-results.json"
echo "[]" > "$RESULTS"

log() {
    local id="$1" name="$2" expect="$3" actual="$4" status="$5"
    echo "[$status] $id: $name"
    python3 -c "
import json
with open('$RESULTS') as f: data=json.load(f)
data.append({'id':'$id','name':$(python3 -c "import json;print(json.dumps('$name'))"),'expected':$(python3 -c "import json;print(json.dumps('''$expect'''[:200]))"),'actual':$(python3 -c "import json;print(json.dumps('''$actual'''[:200]))"),'status':'$status'})
with open('$RESULTS','w') as f: json.dump(data,f)
" 2>/dev/null
}

C() { curl -s "$@"; }
CW() { curl -s -w '\n%{http_code}' "$@"; }
CODE() { echo "$1" | tail -1; }
BODY() { echo "$1" | sed '$d'; }

get_user_count() {
    C -H "$HDR" "$BASE/api/v1/upstreams" | python3 -c "
import sys,json; d=json.load(sys.stdin)
for u in d.get('upstreams',[]):
    if u['id']=='$1': print(u.get('user_count',0)); exit()
print('NOT_FOUND')" 2>/dev/null
}

echo "=== QA Round 2 Full Test Suite ==="
echo "=== Starting: $(date -u) ==="

#############################################
# SECTION A: Fix Verification (Regression)
#############################################
echo ""
echo "--- A. Fix Verification ---"

# Setup: Create test upstreams via POST /api/v1/upstreams (RESTful)
C -X POST "$BASE/api/v1/upstreams" -H "$HDR" -H "$CT" -d '{"id":"qa-upstream-A","address":"10.0.0.100","port":8080}' > /dev/null
C -X POST "$BASE/api/v1/upstreams" -H "$HDR" -H "$CT" -d '{"id":"qa-upstream-B","address":"10.0.0.101","port":8080}' > /dev/null
sleep 0.5

# T1a: Bind → user_count +1
UC_BEFORE=$(get_user_count qa-upstream-A)
C -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-user-1","upstream_id":"qa-upstream-A","app_id":"qa-test"}' > /dev/null
sleep 0.3
UC_AFTER=$(get_user_count qa-upstream-A)
EXPECTED=$((UC_BEFORE + 1))
if [ "$UC_AFTER" = "$EXPECTED" ]; then
    log "T1a" "BUG-004: bind +1" "$EXPECTED" "$UC_AFTER" "PASS"
else
    log "T1a" "BUG-004: bind +1" "$EXPECTED" "before=$UC_BEFORE,after=$UC_AFTER" "FAIL"
fi

# T1b: Unbind → user_count -1
C -X POST "$BASE/api/v1/routes/unbind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-user-1","app_id":"qa-test"}' > /dev/null
sleep 0.3
UC_UNBIND=$(get_user_count qa-upstream-A)
if [ "$UC_UNBIND" = "$UC_BEFORE" ]; then
    log "T1b" "BUG-004: unbind -1" "$UC_BEFORE" "$UC_UNBIND" "PASS"
else
    log "T1b" "BUG-004: unbind -1" "$UC_BEFORE" "$UC_UNBIND" "FAIL"
fi

# T1c: Batch-bind 5 users → +5
C -X POST "$BASE/api/v1/routes/batch-bind" -H "$HDR" -H "$CT" \
  -d '{"upstream_id":"qa-upstream-A","entries":[
    {"sender_id":"qa-b1"},{"sender_id":"qa-b2"},{"sender_id":"qa-b3"},
    {"sender_id":"qa-b4"},{"sender_id":"qa-b5"}
  ]}' > /dev/null
sleep 0.3
UC_BATCH=$(get_user_count qa-upstream-A)
EXPECTED=$((UC_BEFORE + 5))
if [ "$UC_BATCH" = "$EXPECTED" ]; then
    log "T1c" "BUG-004: batch-bind +5" "$EXPECTED" "$UC_BATCH" "PASS"
else
    log "T1c" "BUG-004: batch-bind +5" "$EXPECTED" "before=$UC_BEFORE,after=$UC_BATCH" "FAIL"
fi

# T1d: Migrate → old -1, new +1
UC_A_PRE=$(get_user_count qa-upstream-A)
UC_B_PRE=$(get_user_count qa-upstream-B)
C -X POST "$BASE/api/v1/routes/migrate" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-b1","app_id":"","from":"qa-upstream-A","to":"qa-upstream-B"}' > /dev/null
sleep 0.3
UC_A_POST=$(get_user_count qa-upstream-A)
UC_B_POST=$(get_user_count qa-upstream-B)
EA=$((UC_A_PRE - 1)); EB=$((UC_B_PRE + 1))
if [ "$UC_A_POST" = "$EA" ] && [ "$UC_B_POST" = "$EB" ]; then
    log "T1d" "BUG-004: migrate A-1,B+1" "A=$EA,B=$EB" "A=$UC_A_POST,B=$UC_B_POST" "PASS"
else
    log "T1d" "BUG-004: migrate A-1,B+1" "A=$EA,B=$EB" "A=$UC_A_POST,B=$UC_B_POST" "FAIL"
fi

# T2: BUG-001 duplicate policy → 409
C -X POST "$BASE/api/v1/route-policies" -H "$HDR" -H "$CT" \
  -d '{"match":{"department":"QA测试部R2"},"upstream_id":"qa-upstream-A","priority":10}' > /dev/null
sleep 0.3
R=$(CW -X POST "$BASE/api/v1/route-policies" -H "$HDR" -H "$CT" \
  -d '{"match":{"department":"QA测试部R2"},"upstream_id":"qa-upstream-A","priority":10}')
T2_CODE=$(CODE "$R"); T2_BODY=$(BODY "$R")
if [ "$T2_CODE" = "409" ]; then
    log "T2" "BUG-001: dup policy → 409" "409" "$T2_CODE" "PASS"
else
    log "T2" "BUG-001: dup policy → 409" "409" "$T2_CODE body=$T2_BODY" "FAIL"
fi

# T3: BUG-002 bind non-existent upstream → 400
R=$(CW -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-fake-bind","upstream_id":"fake-upstream-xyz","app_id":"qa-test"}')
T3_CODE=$(CODE "$R"); T3_BODY=$(BODY "$R")
if [ "$T3_CODE" = "400" ]; then
    log "T3" "BUG-002: bind fake upstream → 400" "400" "$T3_CODE" "PASS"
else
    log "T3" "BUG-002: bind fake upstream → 400" "400" "$T3_CODE body=$T3_BODY" "FAIL"
fi

# T4: BUG-005 register → metrics healthy +1
METRICS_BEFORE=$(C "$BASE/metrics" 2>/dev/null | grep '^lobster_upstreams_healthy ' | awk '{print $2}')
C -X POST "$BASE/api/v1/upstreams" -H "$HDR" -H "$CT" \
  -d '{"id":"qa-metrics-fresh","address":"10.0.0.222","port":8080}' > /dev/null
sleep 0.5
METRICS_AFTER=$(C "$BASE/metrics" 2>/dev/null | grep '^lobster_upstreams_healthy ' | awk '{print $2}')
if [ -n "$METRICS_BEFORE" ] && [ -n "$METRICS_AFTER" ]; then
    DIFF=$(echo "$METRICS_AFTER - $METRICS_BEFORE" | bc 2>/dev/null || echo "err")
    if [ "$DIFF" = "1" ]; then
        log "T4" "BUG-005: metrics healthy +1" "+1" "+$DIFF" "PASS"
    else
        log "T4" "BUG-005: metrics healthy +1" "+1" "diff=$DIFF (B=$METRICS_BEFORE,A=$METRICS_AFTER)" "FAIL"
    fi
else
    log "T4" "BUG-005: metrics healthy +1" "metrics exist" "B=$METRICS_BEFORE,A=$METRICS_AFTER" "FAIL"
fi

# T5: BUG-012 healthz not degraded
HEALTH=$(C "$BASE/healthz" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status','?'))")
if [ "$HEALTH" = "healthy" ]; then
    log "T5" "BUG-012: healthz not degraded" "healthy" "$HEALTH" "PASS"
else
    log "T5" "BUG-012: healthz not degraded" "healthy" "$HEALTH" "FAIL"
fi

#############################################
# SECTION B: Policy Route Priority
#############################################
echo ""
echo "--- B. Policy Route Priority ---"

# T6: Policy overrides affinity
# Policy exists: 天眼事业部 → openclaw-team1
# Bind a 天眼 user to team3 (conflicting)
C -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-tianyan-user","upstream_id":"openclaw-team3","app_id":"qa-test","department":"天眼事业部","display_name":"天眼测试"}' > /dev/null
sleep 0.3
T6_ROUTE=$(C -H "$HDR" "$BASE/api/v1/routes" | python3 -c "
import sys,json; d=json.load(sys.stdin)
for r in d.get('routes',[]):
    if r['sender_id']=='qa-tianyan-user':
        print(json.dumps({'conflict':r.get('policy_conflict',False),'policy_upstream':r.get('policy_upstream',''),'policy_rule':r.get('policy_rule','')},ensure_ascii=False))
        break
else:
    print('NOT_FOUND')
")
echo "T6 route: $T6_ROUTE"
if echo "$T6_ROUTE" | python3 -c "import sys,json; d=json.loads(sys.stdin.read()); exit(0 if d.get('conflict')==True and d.get('policy_upstream')=='openclaw-team1' else 1)" 2>/dev/null; then
    log "T6" "Policy overrides affinity" "conflict=true,policy=team1" "$T6_ROUTE" "PASS"
else
    log "T6" "Policy overrides affinity" "conflict=true,policy=team1" "$T6_ROUTE" "FAIL"
fi

# T7: Default policy fallback
# First check if there's a default policy already
T7_CHECK=$(C -H "$HDR" "$BASE/api/v1/route-policies" 2>/dev/null)
echo "T7 policies: $T7_CHECK"
# Bind a user with a department that doesn't match any specific policy
C -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-random-dept","upstream_id":"openclaw-team5","app_id":"qa-test","department":"随机部门","display_name":"随机用户"}' > /dev/null
sleep 0.3
T7_ROUTE=$(C -H "$HDR" "$BASE/api/v1/routes" | python3 -c "
import sys,json; d=json.load(sys.stdin)
for r in d.get('routes',[]):
    if r['sender_id']=='qa-random-dept':
        print(json.dumps({'conflict':r.get('policy_conflict',False),'policy_upstream':r.get('policy_upstream','')},ensure_ascii=False))
        break
else:
    print('NOT_FOUND')
")
echo "T7 result: $T7_ROUTE"
# Default policy in config has empty upstream_id, so it may or may not conflict
log "T7" "Default policy fallback" "see details" "$T7_ROUTE" "INFO"

# T8: No policy → affinity not affected
# We can't delete policies from config, but let's check a user whose department has no matching policy
C -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-no-policy","upstream_id":"openclaw-team1","app_id":"qa-test","department":"无策略部门","display_name":"无策略用户"}' > /dev/null
sleep 0.3
T8_ROUTE=$(C -H "$HDR" "$BASE/api/v1/routes" | python3 -c "
import sys,json; d=json.load(sys.stdin)
for r in d.get('routes',[]):
    if r['sender_id']=='qa-no-policy':
        print(json.dumps({'conflict':r.get('policy_conflict',False),'upstream':r.get('upstream_id','')},ensure_ascii=False))
        break
else:
    print('NOT_FOUND')
")
echo "T8 result: $T8_ROUTE"
if echo "$T8_ROUTE" | python3 -c "import sys,json; d=json.loads(sys.stdin.read()); exit(0 if d.get('conflict')==False else 1)" 2>/dev/null; then
    log "T8" "No policy → no conflict" "conflict=false" "$T8_ROUTE" "PASS"
else
    log "T8" "No policy → no conflict" "conflict=false" "$T8_ROUTE" "FAIL"
fi

# T9: conflict_count statistic
T9_DATA=$(C -H "$HDR" "$BASE/api/v1/routes" | python3 -c "
import sys,json; d=json.load(sys.stdin)
count=d.get('conflict_count',0)
actual_conflicts=sum(1 for r in d.get('routes',[]) if r.get('policy_conflict'))
print(f'{count},{actual_conflicts}')
")
T9_REPORTED=$(echo "$T9_DATA" | cut -d, -f1)
T9_ACTUAL=$(echo "$T9_DATA" | cut -d, -f2)
if [ "$T9_REPORTED" = "$T9_ACTUAL" ]; then
    log "T9" "conflict_count accurate" "match" "reported=$T9_REPORTED,actual=$T9_ACTUAL" "PASS"
else
    log "T9" "conflict_count accurate" "match" "reported=$T9_REPORTED,actual=$T9_ACTUAL" "FAIL"
fi

#############################################
# SECTION C: User Info Refresh
#############################################
echo ""
echo "--- C. User Refresh ---"

# T10: Single user refresh
R=$(CW -X POST "$BASE/api/v1/users/2285568-DWuKGcwAOPYPI2yrjxws2zPEzycPKG/refresh" -H "$HDR")
T10_CODE=$(CODE "$R"); T10_BODY=$(BODY "$R")
if [ "$T10_CODE" = "200" ]; then
    log "T10" "User refresh" "200" "$T10_CODE" "PASS"
    echo "T10 body: $T10_BODY"
else
    log "T10" "User refresh" "200" "$T10_CODE body=$T10_BODY" "FAIL"
fi

# T11: Refresh non-existent user
R=$(CW -X POST "$BASE/api/v1/users/fake-user-12345/refresh" -H "$HDR")
T11_CODE=$(CODE "$R"); T11_BODY=$(BODY "$R")
if [ "$T11_CODE" = "404" ] || [ "$T11_CODE" = "200" ]; then
    log "T11" "Refresh non-existent user" "404 or 200" "$T11_CODE" "PASS"
else
    log "T11" "Refresh non-existent user" "404 or 200" "$T11_CODE body=$T11_BODY" "FAIL"
fi

# T12: Refresh then check routes conflict update
C -X POST "$BASE/api/v1/users/2285568-DWuKGcwAOPYPI2yrjxws2zPEzycPKG/refresh" -H "$HDR" > /dev/null
sleep 0.5
T12_ROUTE=$(C -H "$HDR" "$BASE/api/v1/routes" | python3 -c "
import sys,json; d=json.load(sys.stdin)
for r in d.get('routes',[]):
    if r['sender_id']=='2285568-DWuKGcwAOPYPI2yrjxws2zPEzycPKG':
        print(json.dumps({'conflict':r.get('policy_conflict'),'dept':r.get('department',''),'upstream':r.get('upstream_id',''),'policy_upstream':r.get('policy_upstream','')},ensure_ascii=False))
        break
else:
    print('NOT_FOUND')
")
log "T12" "Refresh→conflict update" "see details" "$T12_ROUTE" "INFO"

#############################################
# SECTION D: Abnormal Input Tests
#############################################
echo ""
echo "--- D. Abnormal Input ---"

# T13: Empty body bind
R=$(CW -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" -d '{}')
T13_CODE=$(CODE "$R")
if [ "$T13_CODE" = "400" ]; then
    log "T13" "Empty body → 400" "400" "$T13_CODE" "PASS"
else
    log "T13" "Empty body → 400" "400" "$T13_CODE body=$(BODY "$R")" "FAIL"
fi

# T14: 1000-char sender_id
LONG_ID=$(python3 -c "print('x'*1000)")
R=$(CW -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d "{\"sender_id\":\"$LONG_ID\",\"upstream_id\":\"qa-upstream-A\",\"app_id\":\"qa-test\"}")
T14_CODE=$(CODE "$R")
log "T14" "1000-char sender_id" "200 or 400" "$T14_CODE" "INFO"
# Cleanup if it was created
C -X POST "$BASE/api/v1/routes/unbind" -H "$HDR" -H "$CT" -d "{\"sender_id\":\"$LONG_ID\",\"app_id\":\"qa-test\"}" > /dev/null

# T15: Special chars
T15a=$(CW -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"用户中文名","upstream_id":"qa-upstream-A","app_id":"qa-test"}')
T15b=$(CW -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"user-🦞-test","upstream_id":"qa-upstream-A","app_id":"qa-test"}')
T15c=$(CW -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d "{\"sender_id\":\"'; DROP TABLE users; --\",\"upstream_id\":\"qa-upstream-A\",\"app_id\":\"qa-test\"}")
HEALTH_T15=$(C "$BASE/healthz" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status','?'))")
if [ "$HEALTH_T15" = "healthy" ]; then
    log "T15" "Special chars no crash" "healthy" "status=$HEALTH_T15,codes=$(CODE "$T15a"),$(CODE "$T15b"),$(CODE "$T15c")" "PASS"
else
    log "T15" "Special chars no crash" "healthy" "$HEALTH_T15" "FAIL"
fi

# T16: Empty upstream_id
R=$(CW -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-test-empty-up","upstream_id":"","app_id":"qa-test"}')
T16_CODE=$(CODE "$R")
if [ "$T16_CODE" = "400" ]; then
    log "T16" "Empty upstream_id → 400" "400" "$T16_CODE" "PASS"
else
    log "T16" "Empty upstream_id → 400" "400" "$T16_CODE body=$(BODY "$R")" "FAIL"
fi

# T17: Rebind same user → user_count adjusts
UC_A_17=$(get_user_count qa-upstream-A)
C -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-rebind-u","upstream_id":"qa-upstream-A","app_id":"qa-test"}' > /dev/null
sleep 0.3
UC_A_AFTER1=$(get_user_count qa-upstream-A)
UC_B_AFTER1=$(get_user_count qa-upstream-B)
C -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-rebind-u","upstream_id":"qa-upstream-B","app_id":"qa-test"}' > /dev/null
sleep 0.3
UC_A_AFTER2=$(get_user_count qa-upstream-A)
UC_B_AFTER2=$(get_user_count qa-upstream-B)
echo "T17 debug: A_init=$UC_A_17, A_after_bind=$UC_A_AFTER1, B_after_bind=$UC_B_AFTER1, A_after_rebind=$UC_A_AFTER2, B_after_rebind=$UC_B_AFTER2"
# After rebind: A should be back to UC_A_17, B should have +1
if [ "$UC_A_AFTER2" = "$UC_A_17" ]; then
    log "T17" "Rebind adjusts count" "A=$UC_A_17" "A=$UC_A_AFTER2" "PASS"
else
    log "T17" "Rebind adjusts count" "A=$UC_A_17" "A=$UC_A_AFTER2 (init=$UC_A_17)" "FAIL"
fi

# T18: bind+unbind rapid → count returns
UC_A_18=$(get_user_count qa-upstream-A)
C -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-rapid-u","upstream_id":"qa-upstream-A","app_id":"qa-test"}' > /dev/null
C -X POST "$BASE/api/v1/routes/unbind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-rapid-u","app_id":"qa-test"}' > /dev/null
sleep 0.3
UC_A_18A=$(get_user_count qa-upstream-A)
if [ "$UC_A_18A" = "$UC_A_18" ]; then
    log "T18" "bind+unbind rapid → 0 net" "$UC_A_18" "$UC_A_18A" "PASS"
else
    log "T18" "bind+unbind rapid → 0 net" "$UC_A_18" "$UC_A_18A" "FAIL"
fi

# T19: Batch-bind empty entries
R=$(CW -X POST "$BASE/api/v1/routes/batch-bind" -H "$HDR" -H "$CT" \
  -d '{"upstream_id":"qa-upstream-A","entries":[]}')
T19_CODE=$(CODE "$R")
if [ "$T19_CODE" = "200" ] || [ "$T19_CODE" = "400" ]; then
    log "T19" "Batch-bind empty list" "200 or 400" "$T19_CODE" "PASS"
else
    log "T19" "Batch-bind empty list" "200 or 400" "$T19_CODE body=$(BODY "$R")" "FAIL"
fi

# T20: Batch-bind partial failure (mixing valid/invalid upstreams)
# batch-bind takes a single upstream_id, so mixed per-entry is not supported
# Instead test with an upstream that doesn't exist
R=$(CW -X POST "$BASE/api/v1/routes/batch-bind" -H "$HDR" -H "$CT" \
  -d '{"upstream_id":"nonexistent-upstream","entries":[{"sender_id":"qa-m1"},{"sender_id":"qa-m2"}]}')
T20_CODE=$(CODE "$R")
T20_BODY=$(BODY "$R")
log "T20" "Batch-bind to nonexistent upstream" "400 or 200" "$T20_CODE body=$T20_BODY" "INFO"

#############################################
# SECTION E: Policy Route Edge Cases
#############################################
echo ""
echo "--- E. Policy Edge Cases ---"

# T21: Policy upstream unhealthy
# qa-upstream-A is "healthy" because we just registered it
# But it's not really reachable - that's the test
T21_ROUTES=$(C -H "$HDR" "$BASE/api/v1/routes" | python3 -c "
import sys,json; d=json.load(sys.stdin)
for r in d.get('routes',[]):
    if r['sender_id']=='qa-tianyan-user':
        print(json.dumps({'conflict':r.get('policy_conflict'),'upstream':r.get('upstream_id'),'policy_upstream':r.get('policy_upstream')},ensure_ascii=False))
        break
")
log "T21" "Policy upstream unhealthy" "see behavior" "$T21_ROUTES" "INFO"

# T22: User matches both department and email_suffix policy
# Config has: 安全运营BU → team2, @security.qianxin.com → team2
# These go to the same upstream, so no conflict. Let's test a user with both
C -X POST "$BASE/api/v1/routes/bind" -H "$HDR" -H "$CT" \
  -d '{"sender_id":"qa-dual-match","upstream_id":"openclaw-team3","app_id":"qa-test","department":"安全运营BU","display_name":"双重匹配"}' > /dev/null
sleep 0.3
T22_ROUTE=$(C -H "$HDR" "$BASE/api/v1/routes" | python3 -c "
import sys,json; d=json.load(sys.stdin)
for r in d.get('routes',[]):
    if r['sender_id']=='qa-dual-match':
        print(json.dumps({'conflict':r.get('policy_conflict'),'policy_upstream':r.get('policy_upstream',''),'policy_rule':r.get('policy_rule','')},ensure_ascii=False))
        break
else:
    print('NOT_FOUND')
")
echo "T22: $T22_ROUTE"
log "T22" "Dual match priority" "see behavior" "$T22_ROUTE" "INFO"

# T23: Empty match condition
R=$(CW -X POST "$BASE/api/v1/route-policies" -H "$HDR" -H "$CT" \
  -d '{"match":{},"upstream_id":"qa-upstream-A","priority":1}')
T23_CODE=$(CODE "$R"); T23_BODY=$(BODY "$R")
if [ "$T23_CODE" = "400" ]; then
    log "T23" "Empty match → 400" "400" "$T23_CODE" "PASS"
else
    log "T23" "Empty match → 400" "400" "$T23_CODE body=$T23_BODY" "FAIL"
fi

# T24: Policy → ghost upstream
R=$(CW -X POST "$BASE/api/v1/route-policies" -H "$HDR" -H "$CT" \
  -d '{"match":{"department":"幽灵部门"},"upstream_id":"ghost-upstream","priority":5}')
T24_CODE=$(CODE "$R"); T24_BODY=$(BODY "$R")
log "T24" "Policy → ghost upstream" "create ok, route degrades" "$T24_CODE body=$T24_BODY" "INFO"

# T25: Delete active policy → conflict disappears
# Get list of policies
T25_POLICIES=$(C -H "$HDR" "$BASE/api/v1/route-policies")
echo "T25 policies: $T25_POLICIES"
# We need the ID of QA测试部R2 policy to delete
T25_POLICY_ID=$(echo "$T25_POLICIES" | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    policies = d if isinstance(d,list) else d.get('policies',[])
    for p in policies:
        m=p.get('match',{})
        if m.get('department')=='QA测试部R2':
            print(p.get('id','')); break
    else:
        print('NOT_FOUND')
except:
    print('PARSE_ERROR')
" 2>/dev/null)
echo "T25 policy to delete: $T25_POLICY_ID
