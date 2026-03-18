# 系统性 Review — v17.1 全量审计

## 审计范围
- 42 个 Go 源文件，34 个测试文件
- 28 个 Vue 页面，19 个 Vue 组件
- 169 个 API 端点
- 3 个分组（IM 安全 7 项 / LLM 安全 9 项 / 系统 8 项）

---

## 🔴 一、ICO/Favicon 问题（张卓之前反馈过，又出现了）

**问题**: index.html 没有 favicon 定义，浏览器标签页显示默认空白图标。

**根因**: 
- `dashboard/public/` 目录不存在
- index.html 中没有 `<link rel="icon">` 标签
- 每次子 agent 改 index.html 都没加

**修复**: 
1. 创建 `dashboard/public/favicon.svg`（龙虾 SVG 图标）
2. index.html 添加 `<link rel="icon" type="image/svg+xml" href="/favicon.svg">`

---

## 🔴 二、功能孤岛问题（页面之间互不链接）

### 2.1 新功能页面完全没有跳转

| 页面 | 跳出链接数 | 问题 |
|------|-----------|------|
| Honeypot.vue | 0 | 触发记录无法跳到用户画像/会话回放 |
| ABTesting.vue | 0 | 测试结果无法跳到 Prompt 追踪对比 |
| Leaderboard.vue | 0 | 排行榜租户点击无法跳到租户详情 |
| RedTeam.vue | 0 | 漏洞无法跳到规则页面修复 |
| BehaviorProfile.vue | 1 | 仅能跳会话回放，不能跳异常/攻击链 |
| AttackChain.vue | 1 | 仅能跳会话回放，不能跳用户画像/蜜罐 |

### 2.2 概览页不知道新功能存在

Overview.vue 只跳到 `/audit` 和 `/user-profiles`。完全不知道有：
- 红队测试（检测率/漏洞）
- 蜜罐（触发/引爆）
- 攻击链（活跃链）
- 排行榜（SLA 状态）
- 行为画像（异常 Agent）
- A/B 测试

**用户体验**: 概览页本应是"一眼看全局，点击钻取细节"，现在只能看到 IM 审计数据，v14-v17 的所有创新功能完全不可见。

### 2.3 LLM 概览同样问题

LLMOverview.vue 只跳到 `/agent` 和 `/prompts`。不知道有蜜罐、A/B 测试、攻击链、行为画像。

---

## 🔴 三、数据孤岛问题

### 3.1 Tenant 注入不一致

api.js 的 `skipPaths` 列表跳过了 `/api/v1/redteam` 和 `/api/v1/leaderboard`，意味着红队和排行榜 API 调用不带 tenant 参数。但后端这两个模块支持 tenant 隔离。

同时：
- Honeypot.vue: 0 次 tenant 引用 → 蜜罐数据不分租户显示
- BehaviorProfile.vue: 0 次 tenant 引用 → 行为画像不分租户

### 3.2 概览页健康分 vs 排行榜健康分

Overview.vue 调用 `/api/v1/health/score` 得到整体健康分。
Leaderboard.vue 各租户有各自的健康分。
但两者没有关联——概览页不显示租户维度的健康分布。

### 3.3 攻击链不引用蜜罐

attack_chain.go 收集 honeypot_triggers 作为事件源，但：
- AttackChain.vue 点击蜜罐相关事件无法跳到 Honeypot 页面
- Honeypot.vue 触发记录无法跳到关联攻击链

### 3.4 红队漏洞不联动规则

RedTeam 发现 IO（Insecure Output）检测率只有 25%，但：
- 没有链接到规则管理页面让用户添加对应规则
- 没有提供"一键修复"或"推荐规则"功能

### 3.5 大屏数据不完整

BigScreen.vue 的指标：
- "在线 Agent" 显示 0（数据源来自 stats，但可能没有这个字段）
- "实时 QPS" 显示 0（非实时数据源）
- OWASP 矩阵通过多次 fallback 调用（先 health/score，再 owasp/matrix，再 bigscreen/data）

---

## 🟡 四、使用逻辑问题

### 4.1 Sidebar 分组混乱

当前分组：
- **IM 安全**: 概览、自定义大屏、上游、路由、规则、审计、用户画像
- **LLM 安全**: LLM概览、LLM规则、Agent行为、会话回放、Prompt追踪、蜜罐、A/B测试、行为画像、攻击链
- **系统**: 监控、异常检测、报告、租户、红队测试、排行榜、运维、设置

**问题**:
1. "自定义大屏" 不属于 IM 安全，应该和"态势大屏"放一起
2. "红队测试" 放在系统组不合理——它测试的是安全规则，应在 LLM 安全或独立安全组
3. "排行榜" 是跨租户比较，放系统组不够直觉
4. "异常检测" 和 "行为画像" 功能重叠但分属不同组
5. LLM 安全组 9 项太多，需要子分组

### 4.2 Icon 系统退化

v7.0 建立了 SVG 图标系统（Icon.vue 有 33 个图标），但：
- Sidebar 完全没使用 Icon.vue，而是内联 SVG（每个导航项都复制了完整 SVG 代码）
- 新页面（v14+）几乎不使用 Icon.vue，各自内联 SVG 或用 emoji
- 导致 Icon.vue 有 33 个定义但实际只有老页面在用
- **这就是"ico 问题又出现了"的根因**——不是 favicon，是 Icon 系统一致性退化

### 4.3 数据时间范围不一致

- Overview.vue 有全局时间选择器（24h/7d/30d）
- BigScreen.vue 固定 24h
- CustomDashboard.vue 没有时间选择器
- 各子页面各自请求不带时间范围

---

## 🟡 五、数据一致性问题

### 5.1 Demo Seed 数据量差异大

不同模块的 demo 数据量差异极大：
- 审计日志: 3317 条
- 攻击链: 35 条（但部分是全租户数据叠加）
- 蜜罐触发: 12 条
- A/B 测试: 2 条
- 行为画像: 5 个 Agent

### 5.2 Stats API 返回字段不标准

各 stats API 返回格式不统一：
- `/api/v1/stats` → 返回 `{total, breakdown}`
- `/api/v1/honeypot/stats` → 返回 `{active_templates, triggers, ...}`  
- `/api/v1/attack-chains/stats` → 返回格式不同
- 大屏需要逐个适配每个 stats 的字段名

---

## 📋 修复优先级

### P0 — 用户直接感知
1. **favicon** — 浏览器标签页空白图标
2. **概览页信息闭环** — 添加红队/蜜罐/攻击链/排行榜摘要卡片
3. **页面互跳** — 所有功能页面关键数据点可跳转到关联页面

### P1 — 体验一致性
4. **Sidebar 重新分组** — 合理化分组 + 减少 LLM 组项数
5. **Icon 系统统一** — Sidebar 和新页面统一使用 Icon.vue
6. **Tenant 注入修复** — 修正 skipPaths，确保所有页面正确带 tenant

### P2 — 数据质量
7. **大屏数据修复** — 确保所有指标有真实数据源
8. **LLM 概览增强** — 添加蜜罐/攻击链/A/B 测试摘要
9. **时间范围统一** — CustomDashboard 加时间选择器

### P3 — 深度联动
10. **红队→规则修复链路** — 漏洞关联推荐规则
11. **蜜罐↔攻击链双向链接**
12. **行为画像→异常检测融合**
