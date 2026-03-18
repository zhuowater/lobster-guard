# 数据流全链路审计 — 从代理流量到功能闭环

## 数据根源

龙虾卫士所有数据来源只有一个：**IM/LLM 透明代理流过的流量**。

```
用户消息 → InboundProxy → RuleEngine → audit_log
                                     → 转发给上游
上游回复 → OutboundProxy → OutboundRuleEngine → audit_log
                                              → 回复给用户

LLM请求 → LLMProxy → LLMRuleEngine → llm_calls + llm_tool_calls
                                    → 转发给LLM提供商
```

## 一级数据表（直接来自流量）

| 表 | 写入来源 | 写入代码 | 状态 |
|---|---------|---------|------|
| audit_log | proxy.go 入站/出站 | logger.Log/LogWithTrace | ✅ 真实流量写入 |
| llm_calls | llm_proxy.go LLM 审计 | llmAuditor.RecordCall | ⚠️ 只有demo数据写入！llm_proxy.go中无RecordCall调用 |
| llm_tool_calls | llm_proxy.go 工具调用 | llmAuditor.RecordToolCall | ⚠️ 同上 |
| prompt_versions | prompt_tracker.go | RecordPrompt | ⚠️ 只有demo数据写入 |
| user_routes | proxy.go 用户路由 | ✅ 真实写入 |
| upstreams | 容器注册 | ✅ 真实写入 |

### 🔴 致命问题：LLM 侧数据全是空的！

`llm_proxy.go` 中 **没有调用** `llmAuditor.RecordCall` 或 `RecordToolCall`。
这意味着：
- llm_calls 表只有 demo seed 数据
- llm_tool_calls 表只有 demo seed 数据
- **所有依赖 LLM 数据的功能在真实场景下全是空数据**

## 二级数据表（依赖一级数据分析生成）

| 表 | 依赖 | 触发方式 | 问题 |
|---|------|---------|------|
| honeypot_triggers | 入站检测结果=warn时 | 🔴 **未集成到代理流水线！** ShouldTrigger 只有 API 手动调用 |
| behavior_anomalies | llm_calls + llm_tool_calls | 🔴 只有 API 手动触发（POST /scan），无自动调度 |
| attack_chains | audit_log + llm_calls + honeypot_triggers | 🔴 只有 API 手动触发（POST /analyze），无自动调度 |
| redteam_reports | 内部调用 RuleEngine | ✅ API 触发，这个设计合理（红队是主动测试） |

### 🔴 致命问题：蜜罐、攻击链、行为画像在真实流量中从不触发！

## 三级数据表（用户配置）

| 表 | CRUD API | Dashboard UI | 问题 |
|---|---------|-------------|------|
| users (认证) | ✅ GET/POST/PUT/DELETE | 🔴 **无用户管理页面！** Login.vue 只有登录，无增删改查 |
| tenants | ✅ CRUD | ✅ Tenants.vue | 基本可用 |
| tenant_members | ✅ CRUD | ⚠️ Tenants.vue 有列表但无完整管理 |
| tenant_configs | ✅ GET/PUT | ✅ Tenants.vue | 可用 |
| honeypot_templates | ✅ CRUD | ✅ Honeypot.vue 有创建/删除 | ⚠️ 缺编辑 |
| dashboard_layouts | ✅ CRUD | ✅ CustomDashboard.vue | 可用 |
| ab_tests | ✅ CRUD + start/stop | ✅ ABTesting.vue | 可用 |
| reports | ✅ CRUD | ✅ Reports.vue | 可用 |

## 规则/策略配置

| 配置项 | API | Dashboard UI | 问题 |
|-------|-----|-------------|------|
| IM 入站规则 | ✅ CRUD + reload | ✅ Rules.vue | 可用 |
| IM 出站规则 | 🔴 只有 GET！无增删改 | 🔴 无UI | 出站规则无法管理 |
| LLM 规则 | ✅ CRUD | ✅ LLMRules.vue | 可用 |
| LLM proxy targets | ✅ GET/PUT config | ⚠️ Settings.vue 显示但操作有限 | |
| SLA 阈值 | ✅ GET/PUT | ✅ Leaderboard.vue | 可用 |
| 告警 webhook | ⚠️ 只有 GET config | ⚠️ Operations.vue 只读显示 | 无法在UI配置 |
| 异常检测阈值 | 🔴 硬编码 | 🔴 无UI | 基线参数不可配 |
| 行为画像参数 | 🔴 硬编码(7天窗口/30min关联) | 🔴 无UI | 时间窗口等不可配 |

---

## 按功能模块的闭环性分析

### ✅ 完全闭环
1. **IM 入站检测** — 流量→规则检测→审计日志→Dashboard展示→规则CRUD
2. **用户画像** — 审计日志→聚合分析→风险评分→Dashboard→点击详情
3. **健康分** — 多维度聚合→Dashboard展示→扣分原因→跳转对应模块
4. **报告引擎** — 聚合数据→生成报告→Dashboard→导出
5. **红队测试** — 内部引擎→运行→结果→漏洞列表→跳转规则修复

### ⚠️ 部分闭环（缺配置管理）
6. **租户系统** — CRUD 可用，但成员管理UI不完整
7. **蜜罐模板** — 创建/删除可用，缺编辑功能
8. **告警** — 只读配置，无法在 Dashboard 配置 webhook

### 🔴 严重不闭环
9. **用户认证** — 有登录/API，但 Dashboard 无用户管理页面（增删改查/启用禁用/角色变更）
10. **IM 出站规则** — 只能读取，无法增删改
11. **LLM 审计** — llm_proxy.go 未集成 RecordCall，真实流量无数据！
12. **蜜罐触发** — 未集成到代理流水线，真实流量不会触发蜜罐
13. **攻击链** — 仅手动分析（API），无自动调度
14. **行为画像** — 仅手动扫描（API），无自动调度
15. **异常检测参数** — 硬编码，不可配

---

## 修复优先级

### P0 — 数据管道（不修=核心功能空壳）
1. **LLM 审计集成** — llm_proxy.go 中调用 RecordCall/RecordToolCall
2. **蜜罐集成到代理流水线** — proxy.go 检测结果=warn时调用 ShouldTrigger
3. **出站蜜罐引爆检测** — outbound 代理调用 CheckDetonation

### P1 — 用户管理（不修=登录功能不完整）
4. **用户管理页面** — Users.vue（列表/创建/编辑角色/启用禁用/重置密码/删除）
5. **出站规则 CRUD** — API + OutboundRules.vue

### P2 — 自动化（不修=需要人工触发）
6. **攻击链自动分析** — 定时任务或流量触发
7. **行为画像自动扫描** — 定时任务
8. **异常检测参数可配** — API + UI

### P3 — 完善
9. **蜜罐模板编辑**
10. **告警 webhook 配置 UI**
11. **租户成员管理完善**
