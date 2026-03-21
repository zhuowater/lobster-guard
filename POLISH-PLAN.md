# 龙虾卫士企业级打磨计划

## 审计结果：38 个页面分级

### ✅ T1 - 已有完整 CRUD（需打磨 UI/交互细节）

| # | 页面 | 行数 | 功能 | 现状 |
|---|------|------|------|------|
| 1 | Rules | 521 | 入站/出站规则管理 | 有增删改、导入导出、正则测试器、命中率展示 |
| 2 | LLMRules | 468 | LLM 规则管理 | 有 CRUD + 启用/禁用 |
| 3 | Upstream | 829 | 上游服务管理 | 有 CRUD + 健康检查 + 发现状态 |
| 4 | Routes | 325 | 路由策略管理 | 有绑定/解绑/迁移/批量绑定/策略测试 |
| 5 | Tenants | 548 | 租户管理 | 有 CRUD + 成员管理 + 安全配置 |
| 6 | Users | 563 | 用户管理 | 有 CRUD + 角色管理 + 用户信息 |
| 7 | Settings | 466 | 系统设置 | 有配置管理 + 告警设置 |
| 8 | APIGateway | 487 | API 网关 | 有路由 CRUD + JWT 认证配置 |
| 9 | ToolPolicy | 361 | 工具策略 | 有 CRUD |
| 10 | Honeypot | 334 | Agent 蜜罐 | 有模板创建 + 注入 + 统计 |
| 11 | ABTesting | 422 | A/B 测试 | 有创建/管理测试 |
| 12 | Operations | 349 | 运维工具 | 有配置/备份/诊断/告警 4 个 Tab |
| 13 | CustomDashboard | 764 | 自定义大屏 | 有拖拽网格 + 组件选择 |
| 14 | Audit | 355 | 审计日志 | 有查询/导出/清理/归档 |
| 15 | LLMCache | 344 | 响应缓存 | 有清除/配置 |

### 🟡 T2 - 有展示和部分操作（需补全交互闭环）

| # | 页面 | 行数 | 功能 | 缺失 |
|---|------|------|------|------|
| 16 | Overview | 314 | 概览驾驶舱 | 纯展示+健康评分，缺快捷操作入口 |
| 17 | SemanticDetector | 334 | 语义检测 | 有测试+配置 Tab，缺规则管理 |
| 18 | AttackChain | 390 | 攻击链分析 | 有分析触发，缺链详情/处置操作 |
| 19 | TaintTracker | 381 | 污染追踪 | 有注入/查询，缺清理/配置 |
| 20 | AnomalyDetection | 379 | 异常检测 | 有基线展示，缺阈值配置/告警规则 |
| 21 | RedTeam | 430 | 红队测试 | 有运行测试，缺场景定制/历史管理 |
| 22 | Reports | 409 | 报告中心 | 有生成/查看，缺定时任务/模板管理 |
| 23 | SessionReplay | 276 | 会话回放 | 有搜索/列表，缺回放详情交互 |
| 24 | SessionDetail | 456 | 会话详情 | 有展示，缺标签/注释/处置 |
| 25 | PromptTracker | 516 | Prompt 追踪 | 有版本对比，缺标签管理/回滚 |
| 26 | Leaderboard | 536 | 排行榜 | 有展示，缺时间范围选择/导出 |
| 27 | Monitor | 262 | 监控指标 | 有图表，缺阈值设置/告警联动 |
| 28 | Singularity | 384 | 奇点蜜罐 | 有部署/统计，缺配置管理 |
| 29 | LLMOverview | 513 | LLM 概览 | 有 dashboard，缺快捷操作 |

### 🔴 T3 - 纯展示/功能空壳（需重点建设）

| # | 页面 | 行数 | 功能 | 问题 |
|---|------|------|------|------|
| 30 | Envelopes | 246 | 执行信封 | 纯列表展示，无创建/管理操作 |
| 31 | EventBus | 221 | 事件总线 | 纯统计展示，无订阅/配置 |
| 32 | Evolution | 288 | 自进化 | 有触发按钮，缺策略配置/历史管理 |
| 33 | BehaviorProfile | 320 | 行为画像 | 有列表+扫描，缺策略/阈值配置 |
| 34 | UserProfiles | 207 | 用户画像 | 极简展示，缺风险阈值/处置 |
| 35 | UserDetail | 285 | 用户详情 | 纯展示，缺处置操作 |
| 36 | AgentBehavior | 400 | Agent 行为 | 有数据展示，缺策略配置 |
| 37 | BigScreen | 392 | 态势大屏 | 展示页，数据丰富但无交互 |
| 38 | Login | 414 | 登录 | 功能完整 ✅ |

## 打磨优先级排序

### Phase 1: 核心安全功能（最高优先级）
> 这些是用户买龙虾卫士的核心理由，必须企业产品级

1. **Rules** — 入站/出站规则完整管理
2. **LLMRules** — LLM 规则完整管理  
3. **Audit** — 审计日志查询/分析/导出
4. **Overview** — 驾驶舱要有快捷操作入口
5. **SessionReplay + SessionDetail** — 会话回放是差异化卖点

### Phase 2: 基础设施管理
> 运维人员日常使用

6. **Upstream** — 上游服务管理
7. **Routes** — 路由策略
8. **Settings** — 系统配置
9. **Operations** — 运维工具
10. **Users + Tenants** — 用户/租户管理

### Phase 3: 高级安全功能
> 安全分析师使用

11. **AttackChain** — 攻击链分析
12. **AnomalyDetection** — 异常检测
13. **SemanticDetector** — 语义检测
14. **RedTeam** — 红队测试
15. **TaintTracker** — 污染追踪
16. **Honeypot + Singularity** — 蜜罐

### Phase 4: 数据洞察
> 管理层/报告使用

17. **Reports** — 报告中心
18. **Leaderboard** — 排行榜
19. **Monitor** — 监控
20. **PromptTracker** — Prompt 追踪
21. **LLMOverview + LLMCache** — LLM 管理

### Phase 5: 辅助功能
> 可以后做

22. **Envelopes** — 执行信封
23. **EventBus** — 事件总线
24. **Evolution** — 自进化
25. **BehaviorProfile + AgentBehavior** — 行为画像
26. **UserProfiles + UserDetail** — 用户画像详情
27. **APIGateway** — API 网关
28. **ABTesting** — A/B 测试
29. **ToolPolicy** — 工具策略
30. **BigScreen + CustomDashboard** — 大屏

## 打磨标准（每个页面必须满足）

### 功能完整性
- [ ] CRUD 闭环：能创建就能编辑、删除
- [ ] 配置页面化：所有可配置项都在页面操作，不需要改 YAML
- [ ] 操作反馈：成功/失败 Toast、Loading 状态
- [ ] 确认对话框：删除等危险操作必须二次确认
- [ ] 空状态引导：无数据时有清晰的引导文案和操作按钮

### 交互质量
- [ ] 表单验证：必填项、格式校验、错误提示
- [ ] 键盘支持：Enter 提交、Esc 关闭弹窗
- [ ] 搜索/过滤：列表页必须有搜索
- [ ] 分页：数据量大时有分页
- [ ] 批量操作：列表页支持多选批量操作（如批量删除/启用/禁用）

### UI 质量
- [ ] 使用统一的组件库（DataTable/Modal/Toast/EmptyState/Icon）
- [ ] 间距/配色一致（Indigo 主色 #6366F1）
- [ ] 响应式（至少 1024px+ 正常使用）
- [ ] 加载状态（Skeleton 或 Loading 组件）
