# 🖥️ 管理后台说明

> 返回 [README](../README.md)

访问 `http://your-server:9090/` 即可打开管理后台（需 JWT 登录）。

## Vue 3 Dashboard（29 页面 · 19 组件）

> 深色科技主题 · Indigo 配色

### Dashboard 全览

![Dashboard Full](screenshots/dashboard-full.jpg)

### 安全拦截一览

> Block 红色高亮 · Warn 黄色高亮 · 支持方向/动作/发送者多维筛选

![Dashboard Block Filter](screenshots/dashboard-block-filter.jpg)

## 页面列表

| 分组 | 页面 | 说明 |
|------|------|------|
| **IM 安全** | Overview | 总览：请求/拦截/告警大数字 + 趋势图 + 饼图 |
| | Rules | 入站/出站规则管理 + 热更新 + 命中率排行 |
| | Audit | 审计日志 + 时间线 + 全文搜索 + CSV/JSON 导出 |
| | Monitor | 实时监控 + QPS 柱状图 + 攻击实时流 |
| | Routes | 路由管理 + 策略匹配 + Bot/部门筛选 |
| **LLM 安全** | LLMOverview | LLM 代理概览 + Token 成本看板 |
| | LLMRules | LLM 规则管理（11 条默认 + 自定义）|
| | PromptTracker | Prompt 版本追踪 + Diff 对比 |
| | Honeypot | 蜜罐管理 + 8 模板 + 引爆记录 |
| | ABTesting | Prompt A/B 测试 + 效果量化 |
| | SessionReplay | 会话回放 + 时间线 + 标签 |
| **威胁分析** | UserProfiles | 攻击者画像 + 驾驶舱模式 |
| | BehaviorProfile | 行为画像 + 特征提取 + 模式学习 |
| | AttackChain | 攻击链检测 + Kill Chain 映射 |
| | AnomalyDetection | 异常基线检测 + 健康分 + OWASP 矩阵 |
| | RedTeam | Red Team Autopilot + 33 攻击向量 |
| **安全治理** | Reports | 安全日报/周报 + 合规审计 + PDF 导出 |
| | Leaderboard | 安全排行榜 + SLA 达成率 |
| | Tenants | 多租户管理 + 租户隔离 |
| | Settings | 系统设置 + 参数配置 |
| **系统** | Operations | 运维工具箱（配置/备份/诊断/告警）|
| | Upstream | 上游容器管理 + 健康检查 |
| | Users | 用户管理 CRUD |
| | BigScreen | 态势感知大屏 + 4 预设模板 |

*附加页面：CustomDashboard（可拖拽自定义布局）/ Login / UserDetail / SessionDetail*

## 组件库（19 个）

TrendChart / PieChart / HeatMap / RuleEditor / TimelineChart 等，统一 Indigo 配色主题。

## 侧边栏导航（5 组）

| 分组 | 页面 | 功能 |
|------|------|------|
| 🛡️ **IM 安全** | Overview / Rules / Audit / Monitor / Routes | IM 入站出站安全管控 |
| 🤖 **LLM 安全** | LLMOverview / LLMRules / PromptTracker / Honeypot / ABTesting / SessionReplay | LLM 代理审计与检测 |
| 🔍 **威胁分析** | UserProfiles / BehaviorProfile / AttackChain / AnomalyDetection / RedTeam | 高级威胁分析与画像 |
| 📋 **安全治理** | Reports / Leaderboard / Tenants / Settings | 报告、排行榜、租户管理 |
| ⚙️ **系统** | Operations / Upstream / Users / BigScreen | 运维工具箱、态势大屏 |

## 更多截图

<details>
<summary>展开查看全部截图</summary>

### LLM 安全域
![LLM Overview](screenshots/dashboard-llm.jpg)
![LLM Cost](screenshots/dashboard-llm-cost.jpg)
![LLM Rules](screenshots/dashboard-llm-rules.jpg)

### 威胁分析
![User Profiles](screenshots/dashboard-user-profiles.jpg)
![User Detail](screenshots/dashboard-user-detail.jpg)
![Agent Behavior](screenshots/dashboard-agent-behavior.jpg)
![Cockpit](screenshots/dashboard-cockpit-v11.4.jpg)
![OWASP Matrix](screenshots/dashboard-owasp-matrix-v11.1.jpg)

### 审计与监控
![Audit](screenshots/dashboard-audit.jpg)
![Monitor](screenshots/dashboard-monitor.jpg)
![Routes](screenshots/dashboard-routes.jpg)
![Rules](screenshots/dashboard-rules.jpg)

### 运维管理
![Operations](screenshots/dashboard-ops.jpg)
![Agent Alerts](screenshots/dashboard-agent-alert.jpg)

### 报告与会话
![Reports List](screenshots/reports-list-v12.0.jpg)
![Report Preview](screenshots/report-preview-v12.0.jpg)
![Session List](screenshots/session-list-v13.jpg)
![Session Detail](screenshots/session-detail-v13.jpg)
![Session Timeline](screenshots/session-timeline-v13.jpg)

### Prompt 追踪
![Prompt History](screenshots/prompt-history-v13.1.jpg)
![Prompt Current](screenshots/prompt-current-v13.1.jpg)
![Prompt Diff](screenshots/prompt-diff-v13.1.jpg)

### 多租户
![Tenants](screenshots/tenants-v14.jpg)
![Tenant Members](screenshots/tenant-members-v14.jpg)
![Tenant Security Config](screenshots/tenant-security-config-v14.jpg)
![Tenant Switch](screenshots/tenant-switch-v14.jpg)

### 设置
![LLM Settings](screenshots/settings-llm-v11.4.jpg)

</details>
