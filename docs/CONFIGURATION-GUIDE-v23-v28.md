# 龙虾卫士 v23-v28 功能配置指南

> 本文档覆盖 v23.0（路径策略引擎）到 v28.0（行业模板+租户绑定）的全部新功能配置。

---

## 目录

1. [v23 路径策略引擎 (PathPolicy)](#v23-路径策略引擎)
2. [v24 反事实验证 (Counterfactual)](#v24-反事实验证)
3. [v25 执行计划编译器 (PlanCompiler)](#v25-执行计划编译器)
4. [v25.1 能力标签系统 (Capability)](#v251-能力标签系统)
5. [v25.2 偏差检测 (Deviation)](#v252-偏差检测)
6. [v26 信息流控制 (IFC/Fides)](#v26-信息流控制)
7. [v27 租户管理 + API Key 身份](#v27-租户管理)
8. [v28 行业模板 + 模板CRUD](#v28-行业模板)

---

## v23 路径策略引擎

**功能**: 基于 Agent 的调用路径（先做了什么→后做了什么）执行序列级、累积级、降级级策略。

### config.yaml

```yaml
path_policy:
  enabled: true
```

> 只需一个开关。规则通过 API / Dashboard 管理，不在 config.yaml 里写。

### Dashboard

入口：**策略引擎** → **治理引擎** → **路径治理**

### 内置规则 (15条，启动自动创建)

| ID | 类型 | 描述 |
|---|---|---|
| pp-001 | 序列 | 读取外部网页后30秒内禁止发送邮件 |
| pp-002 | 序列 | 调用任意工具后禁止执行shell |
| pp-003 | 累积 | 累计调用3次文件写入后降级到只读模式 |
| pp-004 | 降级 | 检测到高频工具调用后降级权限 |
| pp-005 | 序列 | 读取敏感文件后禁止网络访问 |
| pp-006 | 累积 | 累计5次外部API调用后限流 |
| pp-007 | 序列 | 读取凭据后禁止发送消息 |
| pp-008 | 降级 | 短时间内多次错误后降级 |
| pp-009 | 序列 | 查询数据库后禁止文件写入 |
| pp-010 | 累积 | 累计代码执行3次后阻断 |
| pp-011~015 | 各类 | AI法案合规 / 芯片行业专用规则 |

### API

```bash
# 列出所有规则
GET /api/v1/path-policies

# 创建规则
POST /api/v1/path-policies
{
  "id": "pp-custom-001",
  "type": "sequence",           # sequence | accumulative | degradation
  "trigger_tool": "web_fetch",
  "blocked_tool": "send_email",
  "time_window_sec": 60,
  "action": "block",            # block | warn | log
  "description": "读取网页后1分钟内不许发邮件",
  "enabled": true
}

# 查看风险仪表
GET /api/v1/path-policies/risk-gauge

# 查看运行时上下文
GET /api/v1/path-policies/contexts
```

### 策略模板 (9个)

通过 Dashboard「路径治理」页面的模板功能管理：

| 模板 | 描述 |
|---|---|
| 严格安全 | 全部规则启用，action=block |
| 仅监控 | 全部规则 action=log |
| 零信任Agent | 严格限制工具调用链 |
| AI法案合规 | EU AI Act 相关规则 |
| 金融行业 | 金融合规规则组 |
| 医疗健康 | HIPAA 相关规则 |
| 最小防护 | 只启用凭据保护 |
| 芯片/半导体 | IP保护规则 |
| DevOps/CI-CD | CI/CD场景规则 |

---

## v24 反事实验证

**功能**: 对 Agent 可疑操作生成"如果不执行会怎样"的反事实推理，辅助判断操作是否必要。

### config.yaml

```yaml
counterfactual:
  enabled: true
  # mode: "async"              # async(默认) | sync
  # max_per_hour: 100          # 每小时最大验证次数
  # risk_threshold: 0.7        # 风险阈值，超过触发验证
  # cache_ttl_sec: 3600        # 验证结果缓存时间
  # timeout_sec: 30            # 单次验证超时
  # fuzzy_match: false         # 模糊匹配模式
  # high_risk_tools:           # 自定义高风险工具列表（不配则用默认14个）
  #   - shell_exec
  #   - send_email
  #   - file_write
  #   - deploy_production      # 可添加自定义工具
```

### Dashboard

入口：**策略引擎** → **治理引擎** → **反事实验证**

### API

```bash
# 查看统计
GET /api/v1/counterfactual/stats

# 查看验证记录
GET /api/v1/counterfactual/verifications

# 查看/修改配置
GET /api/v1/counterfactual/config
PUT /api/v1/counterfactual/config
{
  "enabled": true,
  "risk_threshold": 0.8
}

# 查看缓存
GET /api/v1/counterfactual/cache

# 手动触发验证
POST /api/v1/counterfactual/verify
{
  "action": "send_email",
  "context": {"sender": "agent-001", "tool": "send_email", "args": {"to": "external@example.com"}}
}
```

---

## v25 执行计划编译器

**功能**: 将 Agent 的工具调用序列与预定义执行计划模板匹配，检测是否偏离正常行为模式。

### config.yaml

```yaml
plan_compiler:
  enabled: true
  match_threshold: 0.3         # 模板匹配阈值 (0-1)
  max_active_plans: 1000       # 最大活跃计划数
  strict_mode: false           # 严格模式：未匹配模板的操作直接阻断
  violation_action: warn       # 违规动作: warn | block | log
  # default_timeout_sec: 300   # 计划默认超时
  # auto_complete: true        # 自动完成已结束的计划
  # retention_days: 30         # 历史保留天数
```

### 内置模板 (20个)

| 模板 | 描述 | 关键词 |
|---|---|---|
| 搜索并总结 | web_search + 总结 | 搜索, search, 查找 |
| 读取文件 | file_read 操作 | 读取, 文件, read |
| 管理任务 | 任务创建/更新 | 任务, todo, task |
| 发送消息 | 消息发送流程 | 发送, 消息, send |
| 部署服务 | 部署操作 | 部署, deploy |
| ... | (共20个) | ... |

> 模板关键词支持中英文。用户请求中包含关键词时自动匹配。

### Dashboard

入口：**策略引擎** → **治理引擎** → **执行计划**

### API

```bash
# 查看活跃计划
GET /api/v1/plans

# 查看计划模板
GET /api/v1/plans/templates

# 提交工具调用事件（通常由 proxy 自动调用）
POST /api/v1/plans/events
{
  "sender_id": "agent-001",
  "tool": "web_search",
  "args": {"query": "天气预报"}
}
```

---

## v25.1 能力标签系统

**功能**: 为工具定义能力标签(capability)，限制 Agent 只能调用被授权的能力范围内的工具。

### config.yaml

```yaml
capability:
  enabled: true
  default_policy: allow        # allow | deny（未标记工具的默认策略）
  # enforce_intersect: false   # 是否强制取交集
  # propagate_labels: false    # 是否传播标签
  # max_contexts_per_user: 100
  # trust_threshold: 0.5
  # audit_all: false           # 审计所有调用（含允许的）
```

### Dashboard

入口：**策略引擎** → **治理引擎** → **能力标签**

### API

```bash
# 查看能力标签列表
GET /api/v1/capabilities

# 创建能力标签
POST /api/v1/capabilities
{
  "tool": "send_email",
  "capabilities": ["communication", "external"],
  "risk_level": "high"
}

# 查看工具-能力映射
GET /api/v1/capabilities/tools

# 检查工具是否允许
POST /api/v1/capabilities/check
{
  "sender_id": "agent-001",
  "tool": "send_email"
}
```

---

## v25.2 偏差检测

**功能**: 检测 Agent 的实际工具调用序列是否偏离了 PlanCompiler 编译的执行计划。

### config.yaml

```yaml
deviation:
  enabled: true
  auto_repair: false           # 自动修复偏差（回滚到计划步骤）
  max_repairs: 5               # 每个 trace 最多自动修复次数
```

### Dashboard

入口：**策略引擎** → **治理引擎** → **偏差检测**

### API

```bash
# 查看偏差统计
GET /api/v1/deviations/stats

# 查看偏差记录
GET /api/v1/deviations
```

---

## v26 信息流控制

**功能**: 基于 Bell-LaPadula 模型的双标签（机密性+完整性）信息流控制。追踪数据从来源到输出的流动路径，检测违规信息流。

### config.yaml

```yaml
ifc:
  enabled: true
  default_confidentiality: 1   # 默认机密性等级 (0=公开, 1=内部, 2=机密, 3=绝密)
  default_integrity: 2         # 默认完整性等级 (0=未验证, 1=低, 2=中, 3=高)
  violation_action: warn       # 违规动作: warn | block | log

  # 隔离 LLM（可选）
  quarantine_enabled: true     # 启用隔离路由：被污染数据发送到独立 LLM
  quarantine_upstream: "http://127.0.0.1:18444"  # 隔离 LLM 的地址

  # 选择性隐藏（可选）
  hiding_enabled: true         # 启用：高机密性数据对低权限 Agent 不可见
  hiding_threshold: 2          # 隐藏阈值：机密性 >= 此值的数据被隐藏

  # 来源规则（可选，定义数据来源的标签）
  source_rules:
    - tool: "read_secret"
      confidentiality: 3       # 读取密钥 → 绝密
      integrity: 3
    - tool: "web_fetch"
      confidentiality: 0       # 外部网页 → 公开但低完整性
      integrity: 0

  # 工具要求（可选，定义工具执行需要的最低权限）
  tool_requirements:
    - tool: "send_email"
      min_integrity: 2         # 发邮件需要中等以上完整性
    - tool: "deploy"
      min_integrity: 3         # 部署需要高完整性
```

### 关键概念

| 概念 | 说明 |
|---|---|
| **机密性 (Confidentiality)** | 数据的秘密等级。高机密数据不能流向低权限输出。类似"不可下读" |
| **完整性 (Integrity)** | 数据的可信等级。低完整性数据不能影响高完整性操作。类似"不可上写" |
| **隔离 LLM** | 被低完整性数据污染的请求自动路由到隔离 LLM，避免污染主 LLM |
| **选择性隐藏** | 高机密性数据在转发给低权限 Agent 时被 `[HIDDEN]` 替换 |
| **DOE 检测** | 检测 Data-Over-Exfiltration，即数据通过工具调用链泄露 |

### Dashboard

入口：**策略引擎** → **治理引擎** → **信息流控制**

### API

```bash
# 查看 IFC 统计
GET /api/v1/ifc/stats

# 查看变量追踪
GET /api/v1/ifc/variables

# 查看违规记录
GET /api/v1/ifc/violations

# 设置变量标签
POST /api/v1/ifc/variables
{
  "name": "api_key_prod",
  "confidentiality": 3,
  "integrity": 3,
  "source": "vault"
}

# 查看活跃 trace
GET /api/v1/ifc/traces

# 查看 DOE 检测结果
GET /api/v1/ifc/doe
```

---

## v27 租户管理

**功能**: 多租户隔离 — 不同团队/部门使用独立的检测规则、策略模板和 API Key 配额。

### 概念

```
租户 (Tenant)
├── 入站规则（per-tenant AC 自动机）
├── LLM 规则（per-tenant AC 自动机）
├── 策略模板绑定
├── 禁用规则列表
├── 工具黑名单
└── API Keys（通过 API Key 自动识别租户）
```

### 通过 Dashboard 配置

入口：**运营管理** → **租户管理**

#### 步骤 1：创建租户

```bash
POST /api/v1/tenants
{
  "id": "chip-team",
  "name": "芯片研发团队",
  "description": "芯片设计部门",
  "contact_email": "chip-team@company.com"
}
```

#### 步骤 2：绑定入站规则模板

```bash
POST /api/v1/tenants/chip-team/bind-inbound-template
{
  "template_id": "tpl-inbound-semiconductor"
}
```

#### 步骤 3：绑定 LLM 规则模板

```bash
POST /api/v1/tenants/chip-team/bind-llm-template
{
  "template_id": "tpl-llm-semiconductor"
}
```

#### 步骤 4：绑定策略模板（PathPolicy）

```bash
POST /api/v1/tenants/chip-team/bind-template
{
  "template_id": "tpl-semiconductor"
}
```

#### 步骤 5：创建 API Key 并关联租户

```bash
POST /api/v1/apikeys
{
  "name": "chip-team-key-1",
  "user_id": "engineer-001",
  "tenant_id": "chip-team",
  "department": "芯片设计部",
  "quota_daily": 1000
}
# 返回: { "key": "sk-xxxx...", "key_prefix": "sk-xxxx..." }
```

### API Key 自动发现

未知 API Key 首次访问 LLM 代理时，自动注册为 `pending` 状态：

```bash
# 查看待审核 Key
GET /api/v1/apikeys/pending

# 审核并绑定
POST /api/v1/apikeys/{id}/bind
{
  "user_id": "new-engineer",
  "tenant_id": "chip-team",
  "department": "芯片设计部",
  "quota_daily": 500
}
```

### 租户检测流程

```
请求进入 → 解析 API Key → 查找租户 → 加载租户专属规则
   ├── 入站: 全局 AC + 租户 AC 合并检测
   ├── LLM:  全局规则 + 租户规则合并检测
   └── PathPolicy: 全局 + 租户规则合并评估
```

---

## v28 行业模板

**功能**: 预置行业规则模板，一键绑定到租户。支持自定义模板 CRUD。

### 内置入站规则模板 (4个)

| 模板ID | 行业 | 规则数 | 检测内容 |
|---|---|---|---|
| tpl-inbound-semiconductor | 芯片 | 3 | EDA工具攻击、供应链渗透、芯片IP窃取 |
| tpl-inbound-financial | 金融 | 3 | 交易系统攻击、账户提权、金融数据窃取 |
| tpl-inbound-healthcare | 医疗 | 3 | 病历系统攻击、处方篡改、患者数据窃取 |
| tpl-inbound-compliance | AI合规 | 2 | EU AI Act 禁止/高风险类别 |

### 内置 LLM 规则模板 (4个)

| 模板ID | 行业 | 规则数 | 覆盖方向 |
|---|---|---|---|
| tpl-llm-semiconductor | 芯片 | 4 | 请求: IP查询拦截+出口管制; 响应: IP泄露+制程节点正则 |
| tpl-llm-financial | 金融 | 4 | 请求: 敏感查询+内幕交易; 响应: 数据泄露+银行账号正则 |
| tpl-llm-healthcare | 医疗 | 4 | 请求: 患者数据+管制药品; 响应: PHI泄露+处方正则 |
| tpl-llm-compliance | AI合规 | 4 | 请求: 违规指令+高风险; 响应: 歧视性内容+操纵正则 |

### Dashboard 管理

- **入站规则页** → 「规则模板」Tab → 查看/创建/编辑/删除模板
- **LLM 规则页** → 「规则模板」Tab → 同上
- **租户管理页** → 选择模板 → 一键绑定

### 创建自定义模板

```bash
# 创建入站模板
POST /api/v1/inbound-templates
{
  "id": "tpl-inbound-custom-001",
  "name": "自定义安全模板",
  "description": "针对内部研发团队的检测规则",
  "category": "security",
  "rules": [
    {
      "name": "block-code-exfil",
      "patterns": ["导出源代码", "export source", "发送代码到外部"],
      "action": "block",
      "message": "禁止导出源代码"
    }
  ]
}

# 创建 LLM 模板
POST /api/v1/llm/templates
{
  "id": "tpl-llm-custom-001",
  "name": "自定义 LLM 安全模板",
  "description": "限制 LLM 响应中的敏感内容",
  "category": "security",
  "rules": [
    {
      "id": "llm-custom-001",
      "name": "阻止泄露内部API",
      "direction": "response",
      "type": "keyword",
      "patterns": ["internal-api.company.com", "10.0.0.", "192.168."],
      "action": "rewrite",
      "rewrite_to": "[已脱敏-内部地址]",
      "enabled": true
    }
  ]
}
```

---

## 快速启用全部功能

最小配置，将以下内容追加到 `config.yaml` 末尾：

```yaml
# v23-v28 治理引擎全家桶
path_policy:
  enabled: true

counterfactual:
  enabled: true

plan_compiler:
  enabled: true
  match_threshold: 0.3
  violation_action: warn

capability:
  enabled: true
  default_policy: allow

deviation:
  enabled: true
  auto_repair: false
  max_repairs: 5

ifc:
  enabled: true
  default_confidentiality: 1
  default_integrity: 2
  violation_action: warn
  quarantine_enabled: true
  quarantine_upstream: "http://127.0.0.1:18444"
  hiding_enabled: true
  hiding_threshold: 2
```

然后通过 Dashboard 操作：
1. 创建租户
2. 选择行业模板绑定
3. 创建 API Key 关联租户
4. 在各治理引擎页面查看实时数据

---

## 注意事项

1. **规则通过 API/Dashboard 管理，不在 config.yaml 中定义**。config.yaml 只控制开关和参数。
2. **内置规则/模板启动自动创建**，DB 已有时不覆盖（通过 INSERT OR IGNORE + UPDATE 同步描述）。
3. **租户规则 DB 持久化**，重启后自动恢复 per-tenant AC 自动机。
4. **LLM 代理的租户识别**依赖 `X-Tenant-Id` header 或 API Key → 租户映射。
5. **IFC 隔离 LLM** 的 `quarantine_upstream` 建议指向独立的低权限 LLM 实例。
6. **Dashboard 浏览器缓存**：部署新版后需强制刷新（Ctrl+Shift+R）或打开新标签页。
