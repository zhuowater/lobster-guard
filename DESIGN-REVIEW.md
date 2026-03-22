# 龙虾卫士设计审查 — 深层问题清单

> 审查人: OpenClaw Agent | 日期: 2026-03-22
> 审查范围: proxy.go, route.go, api.go 核心数据流
> 方法: 追踪数据流、找设计矛盾、分析边界行为

---

## 🔴 D-001: 策略路由不健康时静默降级到随机上游，管理员无感知

**文件**: proxy.go L460

```go
if pUID, ok := ip.policyEng.Match(info, appID); ok && pUID != "" && ip.pool.IsHealthy(pUID) {
    // 策略匹配到了，但多了一个 IsHealthy 检查
    ...
}
```

**问题**: 策略匹配到了 `pUID`，但如果 `pUID` 不健康，整个 if 块跳过，走到亲和路由或负载均衡。**管理员配了策略说"天眼→team1"，team1 挂了，用户被静默分配到 team3，管理员完全不知道。**

**影响**: 
- 策略规则被悄悄绕过
- 用户消息发到了不该去的上游（可能含敏感业务数据）
- Dashboard 冲突检测不会标记这种情况（因为策略匹配后没绑定）

**修复建议**:
1. 策略上游不健康时，日志标记为 `policy_degraded`
2. 审计日志记录一条 `policy_upstream_unhealthy` 事件
3. Dashboard 上游列表对"有策略指向但不健康"的上游加红色告警
4. 考虑：策略上游不健康时是否应该 **阻断** 而不是静默降级？（取决于业务——安全场景可能宁可拒绝也不能发错）

---

## 🔴 D-002: resolveUpstream 和实际转发用的可能不是同一个上游

**文件**: proxy.go L751-758

```go
var proxy *httputil.ReverseProxy
if upstreamID != "" {
    proxy = ip.pool.GetProxy(upstreamID)
}
if proxy == nil {
    proxy, upstreamID = ip.pool.GetAnyHealthyProxy()  // ← 这里！
}
```

**问题**: `resolveUpstream` 返回了 `upstreamID`，绑定到了路由表，计数也加了。但转发时 `GetProxy(upstreamID)` 返回 nil（上游在两步之间被移除或 proxy 为 nil），代码 **静默降级到 GetAnyHealthyProxy**，实际发到了另一个上游。

**但路由表记录的还是原来的 upstreamID，审计日志里记录的也是。**

数据不一致：
- 路由表说用户绑在 team1
- 消息实际发到了 team3
- 审计日志记录的 upstream 是 team1
- user_count team1+1 但 team3 才是真正接收者

**影响**: 
- 审计日志不准确（安全审计最怕的事）
- user_count 漂移
- 策略路由形同虚设（以为在 team1 其实在 team3）

**修复建议**:
```go
if proxy == nil {
    proxy, fallbackUID = ip.pool.GetAnyHealthyProxy()
    if fallbackUID != upstreamID {
        log.Printf("[路由] ⚠️ 上游 %s proxy 不可用，降级到 %s", upstreamID, fallbackUID)
        // 更新路由表和计数
        ip.routes.Bind(senderID, appID, fallbackUID)
        ip.pool.TransferUserCount(upstreamID, fallbackUID)
        upstreamID = fallbackUID  // 后续审计日志用这个
    }
}
```

---

## 🔴 D-003: 桥接模式的转发完全绕过了路由表记录的上游

**文件**: proxy.go L397-420（bridge mode 转发）

```go
// 获取上游地址
var targetURL string
func() {
    ip.pool.mu.RLock()
    defer ip.pool.mu.RUnlock()
    if upstreamID != "" {
        if up, ok := ip.pool.upstreams[upstreamID]; ok {
            targetURL = fmt.Sprintf("http://%s:%d", up.Address, up.Port)
        }
    }
    if targetURL == "" {
        for _, up := range ip.pool.upstreams {
            targetURL = fmt.Sprintf("http://%s:%d", up.Address, up.Port)
            break  // ← map 遍历顺序随机！
        }
    }
}()
```

**问题 1**: `targetURL` 构造时 **没有使用 path_prefix**。webhook 模式通过 `createReverseProxy` 自动加了 path_prefix，但桥接模式手动拼 URL，丢失了 prefix。

如果上游配置了 `path_prefix: /api/v1`，webhook 模式请求 `POST /callback` 会变成 `POST /api/v1/callback`，但桥接模式请求 `POST /` 就是 `POST /`，完全不一样。

**问题 2**: 降级时 `for _, up := range ip.pool.upstreams { break }` — Go map 遍历顺序不确定，每次可能选到不同的上游。不像 `SelectUpstream` 有 least-users 策略。

**问题 3**: 降级后没有更新路由表和计数（同 D-002 的桥接版本）。

**影响**:
- 桥接模式用户的消息永远走不到带 path_prefix 的上游
- 降级行为不确定、不可审计
- 路由表和实际行为不一致

**修复建议**: 桥接模式的转发也应该通过 `GetProxy` 或至少使用 `createReverseProxy` 相同的 URL 构造逻辑。

---

## 🟡 D-004: 策略变更不影响已绑定用户（直到下次请求）

**设计现状**: 管理员在 Dashboard 修改策略规则后，已绑定用户的路由不会立即变化。只有当用户下一条消息进来时，`resolveUpstream` 才会重新匹配策略并迁移。

**场景**:
1. 天眼→team1 策略生效中，100个天眼用户绑定在 team1
2. 管理员改策略：天眼→team2
3. 这 100 个用户的绑定仍然指向 team1
4. 直到每个用户各自发一条消息，才会逐个迁移到 team2

**期间**: team1 的管理员还能看到这些用户的消息（可能已经不该看到了）

**影响**: 策略变更的生效有不确定的延迟（取决于用户活跃度，可能是几分钟也可能是几天）

**修复建议**: 策略 CRUD API 变更后，触发一次 "策略重评估"：
```go
func (api *ManagementAPI) reevaluateAllRoutes() {
    routes := api.routes.ListRoutes()
    for _, r := range routes {
        info := api.userCache.GetCached(r.SenderID)
        if info == nil { continue }
        if pUID, ok := api.policyEng.Match(info, r.AppID); ok && pUID != r.UpstreamID {
            AtomicMigrate(api.routes, api.pool, r.SenderID, r.AppID, r.UpstreamID, pUID)
        }
    }
}
```

---

## 🟡 D-005: 出站代理无路由感知，不知道消息发给谁的上游

**文件**: proxy.go（OutboundProxy）

**设计现状**: 出站代理是一个无状态的反向代理，固定指向 `cfg.LanxinUpstream`（蓝信 API）。它不知道：
- 这条出站消息是哪个上游实例发的
- 它对应的入站用户是谁
- 这个用户属于哪个策略组

**后果**: 
- `X-Upstream-Id` header 依赖上游主动设置，龙虾卫士自己不知道
- trace 关联只靠 recipient→最近5分钟入站 sender 的映射（粗糙）
- 如果多个上游同时给同一个用户发消息，trace 会混

**更深的问题**: 出站规则是全局的，不区分上游。如果 team1 是安全团队（需要发送漏洞详情），team2 是客服团队（不应该发送技术细节），当前无法按来源上游配置不同的出站规则。

**修复建议**: 
1. 出站请求中注入 `X-Lobster-Upstream` header（标记来源上游）
2. 出站规则支持 `upstream_id` 维度的差异化配置
3. 审计日志的 upstream_id 字段应可靠填充

---

## 🟡 D-006: user_count 和路由表之间没有最终一致性保证

**现状**: user_count 是内存计数器，路由表绑定在 SQLite。重启后路由表从 DB 恢复，但 user_count 归零。

```go
// route.go L336
func (rt *RouteTable) loadFromDB() {
    // 只恢复路由绑定，不恢复 user_count
}
```

**影响**: 
- 重启后 least-users 策略完全失效（所有上游 count=0，等于随机）
- 直到下次请求触发绑定流程才会逐渐修复
- 大规模场景（1000 用户）重启后负载均衡完全不均

**修复建议**: 启动时从路由表聚合计算 user_count：
```go
rows := db.Query("SELECT upstream_id, COUNT(*) FROM user_routes GROUP BY upstream_id")
for rows.Next() {
    pool.IncrUserCount(uid, count)
}
```

---

## 🟡 D-007: 默认策略 upstream_id="" 意味着"不路由"，但行为不明确

**文件**: 测试服务器实际配置

```json
{"match":{"default":true},"upstream_id":""}
```

**问题**: 默认策略的 `upstream_id` 为空字符串。`Match()` 返回 `("", true)`。resolveUpstream 的逻辑：

```go
if pUID, ok := ip.policyEng.Match(info, appID); ok && pUID != "" && ip.pool.IsHealthy(pUID) {
```

`pUID != ""` 为 false → 跳过策略路由。所以默认策略 upstream_id="" **等于没有默认策略**。

但 Dashboard 冲突检测里：
```go
if pUID, ok := api.policyEng.Match(info, e.AppID); ok && pUID != "" && pUID != e.UpstreamID {
    // 标记冲突
}
```

同样跳过了空 upstream_id 的默认策略。

**管理员的期望可能是**: "没有匹配到任何部门规则的用户，就不要做策略路由，由负载均衡决定" — 这确实是当前行为。**但如果管理员把默认策略指向一个具体上游**（比如 `general-pool`），那些被 LB 随机分到其他地方的用户就不会被纠偏了——因为只有 `upstream_id != ""` 才生效。

**修复建议**: 要么让 `upstream_id=""` 在 Dashboard 上明确显示为"不路由(LB托管)"，要么在创建策略时校验 upstream_id 不能为空（default 策略也一样）。

---

## 🟢 D-008: 检测超时是 fail-open，静默放行

**文件**: proxy.go L779-784

```go
select {
case detectResult = <-ch:
case <-time.After(ip.timeout):
    detectResult = DetectResult{Action: "pass", Reasons: []string{"timeout"}}
}
```

**设计选择**: 检测超时时默认放行（fail-open）。这是 **正确的可用性选择**，但需要确保：
1. 超时放行的消息在审计日志中标记了 `timeout` 原因 ✅（已有）
2. 有 Prometheus 指标追踪超时率（需确认）
3. 超时阈值可配置 ✅（`DetectTimeoutMs`）

**当前超时**: `DetectTimeoutMs` 默认 50ms — 这非常短。语义检测、LLM 检测在 50ms 内几乎不可能完成。意味着 pipeline 中除了 keyword/regex 之外的阶段，在高负载下基本都会被超时跳过。

**建议**: 检查实际超时率，考虑是否需要调高默认值或为不同阶段设置独立超时。

---

## 🟢 D-009: 桥接模式 block 后直接 return，不通知用户

**文件**: proxy.go L346-350（bridge mode block）

```go
if detectResult.Action == "block" {
    log.Printf("[桥接入站] 拦截 sender=%s reasons=%v", senderID, detectResult.Reasons)
    // ... 记录告警等
    return  // 直接返回，消息静默丢弃
}
```

**问题**: webhook 模式下 block 会返回 HTTP 响应（`BlockResponseWithMessage`），上游/用户知道消息被拦截了。但桥接模式下 block 就是 `return` — 消息静默消失。

**用户视角**: 发了一条消息，没有任何反馈。不知道被拦了还是网络丢了。

**修复建议**: 桥接模式 block 时应通过蓝信 API 主动给发送者一条提示消息（如"您的消息因安全策略被拦截"），或者在桥接回调中返回拦截状态。

---

## 摘要

| 编号 | 严重度 | 问题 | 一句话 |
|------|--------|------|--------|
| D-001 | 🔴 | 策略上游不健康时静默降级 | 策略被悄悄绕过，管理员无感知 |
| D-002 | 🔴 | 转发用的上游可能不是路由表记录的 | 审计日志不准确，数据不一致 |
| D-003 | 🔴 | 桥接模式丢失 path_prefix + 降级随机 | 桥接和 webhook 行为不一致 |
| D-004 | 🟡 | 策略变更不即时生效 | 需要等用户下次请求才迁移 |
| D-005 | 🟡 | 出站代理无路由感知 | 无法按上游差异化出站规则 |
| D-006 | 🟡 | 重启后 user_count 归零 | least-users 策略重启后失效 |
| D-007 | 🟡 | 默认策略 upstream_id="" 行为模糊 | 管理员困惑 |
| D-008 | 🟢 | 检测超时 50ms 太短 | 语义/LLM 阶段可能被常态性跳过 |
| D-009 | 🟢 | 桥接模式 block 静默丢消息 | 用户无反馈 |
