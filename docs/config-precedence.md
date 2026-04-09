# config.yaml / conf.d 优先级规则

> 适用版本：v36.5 起

本文固化 Lobster Guard 的分层配置规则，解决“API 看起来保存了，但重启后又被 conf.d 覆盖”的歧义。

## 1. 加载顺序

启动时配置按下面顺序加载：

1. 读取主配置 `config.yaml`
2. 收集 `conf_dir` 下所有 `*.yaml` 与 `*.yml` 模块文件
3. 将这些模块文件放在同一个列表里，按**完整文件名字母序**排序
4. 按排序结果依次 merge 到主配置之上

结论：
- 同一个字段，**后加载者优先**
- `conf.d` 总体上优先于主配置
- `conf.d` 内部则是**字母序靠后的文件优先**

## 2. 标量 / 普通嵌套结构的规则

对普通标量字段和普通嵌套 struct 字段，当前语义是：

- 未出现的字段：保留前面的值
- 新文件里显式出现的字段：覆盖前面的值

例如：

```yaml
# config.yaml
llm_proxy:
  enabled: false
  listen: ":8445"
  timeout_sec: 300
```

```yaml
# conf.d/10-llm.yaml
llm_proxy:
  enabled: true
```

最终结果：
- `llm_proxy.enabled = true`
- `llm_proxy.listen = ":8445"`
- `llm_proxy.timeout_sec = 300`

也就是说：**嵌套 struct 是字段级覆盖，不是整个 section 清空重来。**

## 3. 需要手动 merge 的 slice 字段

YAML 对 slice 默认是“整段替换”，但 Lobster Guard 对下面这些高价值字段做了显式 merge，避免多层配置互相覆盖：

| 字段 | merge key | 规则 |
|---|---|---|
| `inbound_rules` | `name` | 同名覆盖，不同名追加 |
| `outbound_rules` | `name` | 同名覆盖，不同名追加 |
| `static_upstreams` | `id` | 同 ID 覆盖，不同 ID 追加 |
| `route_policies` | `match` 组合键 | 同 match 覆盖，不同 match 追加 |
| `rule_bindings` | `app_id` | 同 `app_id` 覆盖，不同 `app_id` 追加 |
| `outbound_pii_patterns` | `name` | 同名覆盖，不同名追加 |
| `llm_proxy.targets` | `name` | 同名覆盖，不同名追加 |

### route_policies 的 match 组合键

`route_policies` 用下面五元组作为唯一键：

- `department`
- `email_suffix`
- `email`
- `app_id`
- `default`

只要这五个匹配条件完全一致，就视为同一条策略，后者覆盖前者。

## 4. API 写回规则（重启一致性）

### 4.1 当前会同步回 conf.d 的 section

下面这些 section 通过管理 API 写回时，会同时：

1. 更新 `config.yaml`
2. 按当前 `conf_dir` 定位模块目录（支持默认目录、相对路径、自定义绝对路径）
3. 遍历其中已存在的 `*.yaml` / `*.yml`
4. 如果某个模块文件里已经存在同名 section，则把该 section 一并更新

当前已固化为同步写回的 section：

- `llm_proxy`
- `route_policies`
- `outbound_rules`

这意味着：
- 如果这些 section 最初定义在 `conf.d` 中，API 修改后，**重启不会被旧 conf.d 值打回去**
- 如果某个 `conf.d` 文件里根本没有该 section，不会强行新增该 section

### 4.2 当前不会自动同步回 conf.d 的写法

下面两类情况仍按“主配置优先写回，但不回填新模块文件”处理：

- 纯 top-level / patch 型写入
- 没有调用 `ReplaceSectionAndSyncConfD()` 的 section

因此建议：
- **运行期经常通过 API 修改的 section**，优先集中放在 `config.yaml`，或保证已有对应 `conf.d` section 可被同步
- **只在部署时维护、运行期不改的 section**，适合放在 `conf.d`

## 5. 实际推荐分层

### 建议只放主配置 `config.yaml`

这些配置更适合作为“运行期主真相源”：

- 监听端口、token、数据库路径等基础启动项
- Dashboard/Management API 高频修改项
- 需要通过 API 在线编辑的 section

推荐包括：
- `management_*`
- `auth`
- `route_policies`
- `outbound_rules`
- `llm_proxy`

### 建议放 `conf.d`

这些配置更适合作为“部署模块化配置”：

- 通道凭据
- 较长的规则集合
- 上游/发现相关的大段配置
- 环境差异明显、希望按文件拆分管理的 section

推荐包括：
- `inbound_rules`
- `static_upstreams`
- `rule_bindings`
- `outbound_pii_patterns`
- `discovery`
- `api_gateway`

## 6. 冲突判定原则

如果主配置与 `conf.d` 同时定义了同一 section：

1. 先按加载顺序合并
2. 对普通字段：后者覆盖前者
3. 对受支持的 slice：按对应 merge key 合并
4. 如果运行期 API 写回了支持同步的 section，则 `config.yaml` 与已有 `conf.d` section 会一起更新

## 7. 运维建议

### 推荐做法
- 把“运行期会被 UI/API 修改”的 section 收敛到主配置或可同步的模块文件
- `conf.d` 文件名使用前缀排序，例如：
  - `10-routing.yaml`
  - `20-llm.yaml`
  - `90-local-override.yaml`
- 对同一个 slice key 的 override，固定放在单一模块文件中，减少跨文件覆盖心智负担

### 避免做法
- 在多个 `conf.d` 文件里反复改同一条 `route_policies.match`
- 在 `config.yaml` 用 API 改 `route_policies`，同时保留一个不会同步的旧外部配置副本
- 依赖“YAML 自然 merge”去处理 slice；项目里只有表中列出的字段才有显式 merge 语义

## 8. 对应实现

核心实现位置：
- `src/config.go`
  - `loadConfDir()`
  - `appendUniqueStaticUpstreams()`
  - `appendUniqueRoutePolicies()`
  - `appendUniqueRuleBindings()`
  - `appendUniqueOutboundPIIPatterns()`
  - `appendUniqueLLMTargets()`
- `src/config_persistence.go`
  - `ReplaceSectionAndSyncConfD()`
  - `syncConfDSectionUnlocked()`

对应回归测试：
- `src/config_confdir_test.go`
- `src/config_persistence_test.go`
