# 🧪 测试说明

> 返回 [README](../README.md)

## 运行测试

```bash
# 运行全部测试（940 个用例，约 30 秒）
CGO_ENABLED=1 go test -v -count=1 ./...
```

## 端到端模拟测试（v18 新增）

lobster-guard 内置端到端流量模拟器，可一键验证入站检测→路由→出站拦截→蜜罐引爆→审计记录全链路：

```bash
# 触发端到端模拟
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:9090/api/v1/simulate/traffic

# 响应示例
{
  "status": "completed",
  "scenarios_run": 12,
  "scenarios_passed": 12,
  "scenarios_failed": 0,
  "duration_ms": 234,
  "results": [
    {"scenario": "inbound_injection_block", "passed": true},
    {"scenario": "outbound_pii_block", "passed": true},
    {"scenario": "llm_canary_detect", "passed": true},
    {"scenario": "honeypot_trigger", "passed": true},
    {"scenario": "trace_correlation", "passed": true},
    ...
  ]
}
```

模拟覆盖场景：
- ✅ 入站 Prompt Injection 检测与拦截
- ✅ 出站 PII/凭据泄露拦截
- ✅ LLM 规则引擎检测（11 条默认规则）
- ✅ Canary Token 泄露检测
- ✅ 蜜罐引爆检测（8 模板）
- ✅ 路由亲和与策略匹配
- ✅ trace_id 全链路关联
- ✅ 审计日志完整性

## 测试覆盖（940 个用例，49 个测试文件，全部通过）

| 类别 | 用例数 | 内容 |
|------|--------|------|
| AC 自动机 | 6 | 基本匹配、大小写、中文、多模式、空输入 |
| 入站规则引擎 | 20+ | block/warn/log/PII/优先级权重/自定义消息 |
| 正则规则 | 9 | 匹配/优先级/编译失败/热更新/命中统计 |
| 规则分组/绑定 | 10+ | group 标签/app_id 绑定/通配符/无分组兼容 |
| 出站规则引擎 | 8+ | block/warn/热更新/优先级/v18 智能合并/CRUD |
| PII 可配置 | 8+ | 默认模式/自定义模式/编译失败回退/API |
| 蓝信加解密 | 4 | 初始化/签名/加密解密全链路 |
| 飞书插件 | 6 | 加解密/URL Verification/出站提取 |
| 钉钉插件 | 5 | 加解密/HMAC 签名/出站提取 |
| 企微插件 | 7 | XML 加解密/GET 验证/签名校验/出站提取 |
| 通用插件 | 4 | 默认/自定义字段/出站审计 |
| Bridge Mode | 5 | 状态序列化/Token 刷新/Ticket 获取/支持矩阵 |
| Rate Limiting | 14 | 令牌桶/全局/每用户/白名单/清理/统计/API |
| Prometheus | 11 | 计数器/直方图/格式输出/端点/开关 |
| 规则热更新 | 16 | 文件加载/验证/优先级/并发 reload+detect |
| 路由表 | 12+ | 复合键 CRUD/迁移/批量绑定/策略匹配 |
| 用户信息 | 15+ | 缓存 GetOrFetch/ListAll/刷新/Provider/API |
| 审计日志 | 18+ | 导出CSV/JSON/清理/时间线/归档/全文搜索 |
| 告警通知 | 6+ | webhook/蓝信格式/最小间隔/内容截断 |
| WebSocket 代理 | 24 | 连接/帧转发/检测拦截/超时/心跳/并发限制 |
| Store 抽象层 | 20+ | SQLiteStore CRUD/备份/恢复/Ping |
| 优雅关闭 | 10+ | 信号处理/健康检查/5维检查/关闭流程 |
| 配置验证 | 9 | 端口冲突/通道/模式/正则编译/PII/上游 |
| 检测链 Pipeline | 14 | 阶段串行/block 终止/自定义顺序 |
| 上下文感知 | 14 | 风险积分/时间衰减/自动升级/重置 |
| LLM 检测 | 18 | async/sync/超时/fail-open/mock |
| 检测缓存 | 14 | LRU/TTL/block 不缓存/命中统计 |
| 规则模板库 | 18 | 加载/合并/go:embed/API |
| LLM 代理 | 15+ | 反向代理/审计/成本/规则引擎/Canary Token |
| LLM 规则 | 20+ | 11 默认规则/合并/CRUD/v18 修复 |
| JWT 认证 | 15+ | 登录/Token 验证/过期/刷新/多租户 |
| Red Team | 12+ | 33 攻击向量/结果收集/排行榜/SLA |
| 蜜罐 | 15+ | 8 模板/水印/引爆检测/代理集成 |
| A/B 测试 | 12+ | 创建/分流/效果量化/推荐 |
| 行为画像 | 12+ | 特征提取/模式学习/基线/异常 |
| 攻击链 | 15+ | 多阶段关联/Kill Chain/升级策略 |
| 异常检测 | 12+ | 基线建立/偏差检测/告警/衰减 |
| 布局 | 12+ | 4 预设模板/自定义/拖拽/保存 |
| 报告引擎 | 10+ | 日报/周报/合规/PDF 导出 |
| 会话回放 | 10+ | 时间线/标签/搜索/Prompt 追踪 |
| 多租户 | 12+ | 租户 CRUD/隔离/JWT/权限 |
| 用户管理 | 10+ | CRUD/v18 修复/批量操作 |
| 端到端模拟 | 8+ | 全链路/trace 关联/蜜罐集成（v18）|
| **集成测试** | 10+ | Mock 上游 + 加密 webhook 全链路 |
| **并发测试** | 5+ | 多 goroutine 入站/出站/混合攻防 |
| 执行信封 | 15+ | HMAC签名/验证/Merkle批次/哈希链/Proof |
| 事件总线 | 12+ | 事件发射/Webhook投递/ActionChain/重试 |
| 自适应决策 | 10+ | 贝叶斯更新/置信区间/误伤率/反馈衰减 |
| 奇点蜜罐 | 10+ | 拓扑预算/欧拉χ/推荐放置/暴露等级 |
| 对抗性自进化 | 15+ | 6策略变异/规则生成/适应度/跨代 |
| 语义检测 | 12+ | TF-IDF/句法/异常/意图/四维融合 |
| 蜜罐深度交互 | 10+ | 忠诚度曲线/交互记录/自进化回馈 |
| 工具策略 | 12+ | 规则匹配/滑窗限流/CRUD/事件 |
| 污染追踪 | 15+ | 12 PII/三端传播/血缘阻断/扫描 |
| 污染逆转 | 10+ | soft/hard/stealth/12模板/测试 |
| LLM 缓存 | 12+ | TF-IDF匹配/TTL/租户隔离/清除 |
| API 网关 | 15+ | JWT生成/验证/APIKey/灰度路由/CRUD |

## 性能

| 指标 | 数值 |
|------|------|
| 检测延迟（P99） | < 5ms |
| 入站吞吐（单核） | > 5,000 req/s |
| 审计写入 | 异步，不阻塞请求 |
| 内存占用 | < 80MB |
| 二进制大小 | ~19MB |
| Dashboard 加载 | < 10KB (gzip) |
| 测试用例 | 940 (~30s) |

- 规则引擎基于 **Aho-Corasick 算法**，O(n) 时间复杂度，文本长度无关
- SQLite **WAL 模式**，支持并发读写
- HTTP 连接池复用，减少 TCP 握手开销
