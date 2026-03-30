# 🖥️ 管理后台说明

> 返回 [README](../README.md)

访问 `http://your-server:9090/` 即可打开管理后台（需 JWT 登录）。

## Vue 3 Dashboard（50 页面 · 75 组件）

> 深色科技主题 · Indigo (#6366F1) 配色 · 企业产品级交互

### v33.0 企业级打磨标准

每个页面都满足：

- ✅ CRUD 闭环 — 能创建就能编辑、删除
- ✅ 配置页面化 — 所有配置项都在页面操作，修改自动回写 config.yaml
- ✅ 操作反馈 — Toast 成功/失败 + Loading 状态
- ✅ 确认对话框 — 删除等危险操作 ConfirmModal 二次确认
- ✅ 空状态引导 — EmptyState 组件 + 操作按钮
- ✅ 搜索和过滤 — 列表页有搜索框 + 多维过滤
- ✅ 批量操作 — checkbox 多选 + 批量启用/禁用/删除
- ✅ 表单验证 — 必填+格式+实时错误提示
- ✅ 统一组件库 — DataTable / StatCard / ConfirmModal / EmptyState / Skeleton / Icon / Toast
- ✅ 响应式 — 1024px+ 正常使用
- ✅ Skeleton 骨架屏 — 数据加载中显示骨架占位

## 页面列表（50 页面）

### 安全总览

| 页面 | 功能 | v33.0 |
|------|------|-----------|
| Overview | 安全驾驶舱 | 时间范围选择器 · 快捷操作 · 自动刷新 · 健康分环形图 · 数据变化闪烁 |
| CustomDashboard | 自定义大屏 | 布局保存/加载 · 预览模式 |
| AnomalyDetection | 异常检测 | 基线管理+迷你SVG图 · 独立阈值配置 · 趋势图弹窗 · 异常脉冲动画 |
| Monitor | 监控指标 | 4指标图表 · 阈值设置 · 告警联动(超阈值红色脉冲) · 自动刷新 |

### 威胁中心

| 页面 | 功能 | v33.0 |
|------|------|-----------|
| Audit | 审计日志 | 统计面板 · 高级过滤+URL同步 · 日志详情展开+关联跳转 · 归档管理 |
| SessionReplay | 会话回放 | 统计卡片 · 高级筛选 · 风险标记 · 卡片展开预览 |
| SessionDetail | 会话详情 | 聊天气泡风格 · 安全事件标注 · 标签注释 · 导出JSON/MD · 键盘导航 |
| AttackChain | 攻击链 | 攻击时间线 · 处置操作(确认/误报/封禁) · 分析配置 |
| UserProfiles | 用户画像 | 风险评分进度条 · 搜索过滤+排序 |
| UserDetail | 用户详情 | 行为时间线 · 封禁/解封 · 标签系统 · 关联数据 |

### 策略引擎

| 页面 | 功能 | v33.0 |
|------|------|-----------|
| Rules | 入站/出站规则 | 出站CRUD完整 · 批量操作 · 搜索过滤 · 正则校验 · 优先级可视化 |
| Routes | 路由策略 | 三Tab · 策略优先级调整 · 批量解绑/迁移 · SVG绑定关系图 |
| Upstream | 上游管理 | 搜索过滤 · 批量健检 · 健康可视化 · K8s发现面板 |
| Envelopes | 执行信封 | 3Tab(信封/Merkle/链验证) · 配置管理 |
| ToolPolicy | 工具策略 | 搜索过滤 · 批量操作 · 策略详情展开 |
| Evolution | 自进化 | 3Tab(日志/策略/学习曲线) · 配置 |
| EventBus | 事件总线 | 4Tab(事件流/目标/送达/ActionChain) · 目标CRUD |

### LLM 策略

| 页面 | 功能 | v33.0 |
|------|------|-----------|
| LLMRules | LLM规则 | 4维过滤 · 规则测试器 · 批量操作 · 影子模式 |
| LLMOverview | LLM概览 | 快捷操作 · Token消耗双饼图 |
| LLMCache | 响应缓存 | 缓存列表+搜索 · 策略配置(TTL/LRU/LFU) · 批量操作 |
| SemanticDetector | 语义检测 | 四维雷达图 · 模式库搜索+快速测试 · 检测历史 · 配置 |
| PromptTracker | Prompt追踪 | 版本diff(split/unified) · 标签管理 · 回滚 |
| ABTesting | A/B测试 | 统计面板 · 显著性检验 · 应用胜出方案 |
| APIGateway | API网关 | 搜索+方法过滤 · JWT增强 · 路由测试Tab · 批量 |

### 威胁狩猎

| 页面 | 功能 | v33.0 |
|------|------|-----------|
| RedTeam | 红队测试 | 场景管理+自定义 · 批量执行+进度 · 漏洞报告 |
| Honeypot | Agent蜜罐 | 模板搜索 · 部署管理Tab · 触发记录展开 |
| Singularity | 奇点蜜罐 | 诱饵配置 · 捕获记录+历史Tab |
| TaintTracker | 污染追踪 | 传播路径时间线 · 清理操作 · 配置Tab |
| BehaviorProfile | 行为画像 | 策略配置 · 行为模式可视化 · 活跃热力格 |
| AgentBehavior | Agent行为 | 规则CRUD · 异常标记 · 高危导出 |

### 运营管理

| 页面 | 功能 | v33.0 |
|------|------|-----------|
| Reports | 报告中心 | 模板管理 · 定时任务 · 预览/下载 · 进度动画 |
| Leaderboard | 排行榜 | 时间范围 · 多维排行 · 导出CSV/JSON · 金银铜 |
| Tenants | 租户管理 | 双视图 · 成员批量 · 安全配置分组 |
| Users | 用户管理 | 密码强度 · 角色权限 · 操作审计弹窗 |
| Settings | 系统设置 | ⭐ 6组配置页面化 · 变更预览 · YAML回写 |
| Operations | 运维工具 | YAML高亮 · 备份策略 · 诊断6宫格 · 告警CRUD |
| BigScreen | 态势大屏 | 全屏(F11) · 数字滚动 · 告警闪烁 |
| Login | 登录 | 记住登录 · 密码显隐 · 表单验证 |

## 组件库（21 个）

| 组件 | 用途 |
|------|------|
| DataTable | 通用数据表格（排序/分页/展开/多选） |
| StatCard | 统计卡片（动画数字/趋势/可点击） |
| ConfirmModal | 确认弹窗（danger/warning 两种模式） |
| EmptyState | 空状态引导（图标+文案+操作按钮） |
| Skeleton | 骨架屏加载占位 |
| Toast | 全局消息提示（success/error/warning） |
| Icon | SVG 图标系统 |
| TrendChart | 趋势折线图（纯 CSS/SVG） |
| PieChart | 饼图 |
| HeatMap | 热力图 |
| JsonHighlight | JSON 语法高亮 |
| Sidebar | 侧边栏导航（可折叠/子分组） |
| BindModal | 绑定操作弹窗 |
| UpstreamSelect | 上游选择器 |
| RuleEditor | 规则编辑器（入站/出站） |
| RegexTester | 正则测试器 |
| DraggableGrid | 拖拽网格布局 |
| 其他 | 4 个辅助组件 |

## 技术栈

- **Vue 3.5** + Vue Router 4.5 + Vite 6.2
- **零 UI 库** — 所有组件手写，纯 CSS
- **go:embed** — 构建产物嵌入 Go 二进制，部署仍为单文件
- **配色** — Indigo 主色 (#6366F1)，参考 Linear/Vercel 风格
