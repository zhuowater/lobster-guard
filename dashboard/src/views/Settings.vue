<template>
  <div>
    <div class="settings-tabs">
      <button v-for="tab in tabs" :key="tab.key" class="settings-tab" :class="{ active: activeTab === tab.key }" @click="switchTab(tab.key)">
        <span class="tab-icon" v-html="tab.icon"></span>
        <span class="tab-label">{{ tab.label }}</span>
        <span v-if="tab.key === 'config' && hasChanges" class="tab-badge">●</span>
      </button>
    </div>

    <!-- Tab 1: 配置管理 -->
    <div v-show="activeTab === 'config'">
      <Skeleton v-if="configLoading" type="text" />
      <template v-else>
        <div class="card config-card" style="margin-bottom: 16px; border-color: rgba(99,102,241,.22); background: linear-gradient(135deg, rgba(49,46,129,.18), rgba(15,23,42,.96));">
          <div class="card-header" style="display:flex;justify-content:space-between;align-items:center;gap:12px;">
            <div>
              <div class="card-title" style="color:#e0e7ff;">🚀 快速配置向导</div>
              <div class="config-desc" style="margin-top:6px;color:#a5b4fc;">适合新手：4 步生成可下载的 config.yaml 模板，不会直接写入服务器。</div>
            </div>
            <button class="btn btn-sm btn-primary" style="background:#4f46e5;border-color:#6366f1;" @click="wizardVisible = true">打开向导</button>
          </div>
        </div>
        <div class="config-group-nav">
          <button v-for="g in configGroups" :key="g.key" class="config-group-btn" :class="{ active: activeGroup === g.key }" @click="activeGroup = g.key">{{ g.icon }} {{ g.label }}</button>
        </div>
        <div v-if="hasChanges" class="changes-bar">
          <div class="changes-bar-inner">
            <span class="changes-dot">●</span>
            <span>{{ changedFields.length }} 项配置已修改</span>
            <button class="btn btn-sm" @click="showChangesPreview = !showChangesPreview" style="margin-left:8px">{{ showChangesPreview ? '收起' : '查看变更' }}</button>
            <button class="btn btn-sm btn-primary" @click="saveConfig" :disabled="configSaving" style="margin-left:4px">{{ configSaving ? '保存中...' : '💾 保存' }}</button>
            <button class="btn btn-sm btn-ghost" @click="resetConfig" style="margin-left:4px">撤销</button>
          </div>
          <div v-if="showChangesPreview" class="changes-preview">
            <div v-for="c in changedFields" :key="c.key" class="change-row">
              <span class="change-key">{{ c.label }}</span>
              <span class="change-old">{{ c.oldVal }}</span>
              <span class="change-arrow">→</span>
              <span class="change-new">{{ c.newVal }}</span>
              <span v-if="c.restart" class="cfg-restart-tag">需重启</span>
            </div>
          </div>
        </div>

        <div v-for="group in visibleGroups" :key="group.key" v-show="activeGroup === group.key" class="config-section">
          <div class="card config-card">
            <div class="card-header"><span class="card-icon">{{ group.icon }}</span><span class="card-title">{{ group.title }}</span></div>
            <div class="config-desc">{{ group.desc }}</div>
            <div class="config-items">
              <div v-for="item in group.items" :key="item.key || item.field" class="cfg-item">
                <div class="cfg-item-head">
                  <span class="cfg-item-label">{{ item.label }}</span>
                  <span v-if="item.restart" class="cfg-restart-tag">需重启</span>
                  <span v-if="item.field ? isRLChanged(item.field) : isChanged(item.key)" class="cfg-changed-dot">●</span>
                </div>
                <div class="cfg-item-desc">{{ item.desc }}</div>
                <select v-if="item.options" v-model="form[item.key]" class="cfg-select"><option v-for="o in item.options" :key="o.value" :value="o.value">{{ o.label }}</option></select>
                <div v-else-if="item.type === 'toggle'" class="cfg-toggle-row"><label class="toggle-switch"><input type="checkbox" v-model="form[item.key]" /><span class="toggle-slider"></span></label><span class="cfg-toggle-label">{{ form[item.key] ? '已开启' : '已关闭' }}</span></div>
                <div v-else-if="item.field" class="cfg-inline"><input type="number" :value="form.rate_limit[item.field]" @input="form.rate_limit[item.field] = Number($event.target.value)" class="cfg-input-num" :min="item.min" :max="item.max" :step="item.step" /><span v-if="item.unit" class="cfg-unit">{{ item.unit }}</span></div>
                <div v-else-if="item.type === 'number'" class="cfg-inline"><input type="number" v-model.number="form[item.key]" class="cfg-input-num" :min="item.min" :max="item.max" :step="item.step" /><span v-if="item.unit" class="cfg-unit">{{ item.unit }}</span></div>
                <input v-else type="text" v-model="form[item.key]" class="cfg-input" :class="{ 'cfg-input-wide': item.wide }" :placeholder="item.placeholder" />
                <span v-if="item.key && errors[item.key]" class="cfg-error">{{ errors[item.key] }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 告警特殊区域 -->
        <div v-show="activeGroup === 'alerts'" class="config-section" style="margin-top:0">
          <div class="card config-card" style="margin-bottom:16px">
            <div class="card-header"><span class="card-icon">🔔</span><span class="card-title">告警通知</span></div>
            <div class="config-desc">Webhook 通知与告警频率设置</div>
            <div class="config-items">
              <div class="cfg-item">
                <div class="cfg-item-head"><span class="cfg-item-label">Webhook URL</span><span v-if="isChanged('alert_webhook')" class="cfg-changed-dot">●</span></div>
                <div class="cfg-item-desc">告警推送目标地址</div>
                <input type="text" v-model="form.alert_webhook" class="cfg-input cfg-input-wide" placeholder="https://your-webhook-url" />
              </div>
              <div class="cfg-item">
                <div class="cfg-item-head"><span class="cfg-item-label">告警格式</span></div>
                <div class="cfg-item-desc">推送消息格式</div>
                <select v-model="form.alert_format" class="cfg-select"><option value="generic">通用 JSON</option><option value="lanxin">蓝信</option></select>
              </div>
              <div class="cfg-item">
                <div class="cfg-item-head"><span class="cfg-item-label">最小间隔</span></div>
                <div class="cfg-item-desc">两次告警之间的最小间隔，防告警风暴</div>
                <div class="cfg-inline"><input type="number" v-model.number="form.alert_min_interval" class="cfg-input-num" min="0" max="3600" step="10" /><span class="cfg-unit">秒</span></div>
              </div>
            </div>
            <div class="cfg-actions-row">
              <button class="btn btn-sm" @click="testAlert" :disabled="!form.alert_webhook || alertTesting">{{ alertTesting ? '发送中...' : '📤 测试告警' }}</button>
              <span v-if="alertTestResult" class="cfg-hint" :style="{ color: alertTestResult.ok ? 'var(--color-success)' : 'var(--color-danger)' }">{{ alertTestResult.msg }}</span>
            </div>
          </div>
          <div class="card config-card">
            <div class="card-header"><span class="card-icon">📋</span><span class="card-title">最近告警</span><div class="card-actions"><button class="btn btn-ghost btn-sm" @click="loadAlertHistory">刷新</button></div></div>
            <Skeleton v-if="alertsLoading" type="table" />
            <div v-else-if="!alerts.length" class="empty"><div class="empty-icon">🔕</div>暂无告警记录</div>
            <div v-else class="alert-list">
              <div v-for="a in alerts" :key="a.id" class="alert-item">
                <div class="alert-meta"><span class="alert-dir" :class="'dir-' + a.direction">{{ a.direction === 'inbound' ? '⬇ 入站' : '⬆ 出站' }}</span><span class="alert-time">{{ fmtTime(a.timestamp) }}</span><span class="alert-sender">{{ a.sender_id || '--' }}</span></div>
                <div class="alert-reason">{{ a.reason }}</div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>

    <div v-show="activeTab === 'security'" class="card canary-card" style="margin-top:16px; border-color: rgba(99,102,241,.22);">
      <div class="card-header"><div class="card-title">🐤 金丝雀令牌管理</div></div>
      <div class="config-desc">创建时间：{{ canaryStatus.token_created_at || '-' }} · 下次轮换：{{ canaryStatus.next_rotation_at || '-' }}</div>
      <div style="margin-top:12px; display:flex; gap:12px; align-items:center;">
        <button class="btn btn-primary" @click="openCanaryRotateConfirm">立即轮换</button>
      </div>
      <div style="margin-top:16px;">
        <div class="config-desc" style="margin-bottom:8px;">轮换历史</div>
        <div v-for="item in canaryHistory" :key="item.rotated_at" class="kv-row">
          <span>{{ fmtTime(item.rotated_at) }}</span>
          <span>{{ item.old_token_hash }} → {{ item.new_token_hash }}</span>
        </div>
      </div>
    </div>

    <ConfigWizard :visible="wizardVisible" @close="wizardVisible = false" />

    <!-- Tab 2: 认证与安全 -->
    <div v-show="activeTab === 'auth'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg></span><span class="card-title">认证设置</span></div>
        <div class="settings-section">
          <label class="settings-label"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg> Bearer Token</label>
          <div class="token-input-wrap">
            <input :type="showToken ? 'text' : 'password'" v-model="tokenValue" placeholder="输入 Bearer Token" class="token-input" />
            <button class="token-toggle" @click="showToken = !showToken"><svg v-if="showToken" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><line x1="1" y1="1" x2="23" y2="23"/></svg><svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg></button>
          </div>
          <div class="token-actions"><button class="btn btn-sm" @click="doSaveToken">保存</button><button class="btn btn-danger btn-sm" @click="confirmClearToken">清除</button></div>
        </div>
      </div>
      <div class="card" style="margin-bottom:20px">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/></svg></span><span class="card-title">演示数据</span></div>
        <div style="font-size:var(--text-sm);color:var(--text-secondary);margin-bottom:var(--space-3)">注入模拟审计数据用于演示。</div>
        <div v-if="demoResult" style="margin-bottom:var(--space-3);padding:var(--space-2) var(--space-3);border-radius:var(--radius-md);font-size:var(--text-sm);background:var(--bg-elevated)"><span :style="{ color: demoResult.ok ? 'var(--color-success)' : 'var(--color-danger)' }">{{ demoResult.message }}</span></div>
        <div style="display:flex;gap:var(--space-2)"><button class="btn btn-sm" @click="seedDemo" :disabled="demoLoading">{{ demoLoading ? '注入中...' : '注入演示数据' }}</button><button class="btn btn-danger btn-sm" @click="confirmClearDemo" :disabled="demoLoading">清除</button></div>
      </div>
      <div class="card" style="margin-bottom:20px">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/></svg></span><span class="card-title">备份管理</span><div class="card-actions"><button class="btn btn-sm" @click="createBackup">创建备份</button><button class="btn btn-ghost btn-sm" @click="loadBackups">刷新</button></div></div>
        <Skeleton v-if="backupsLoading" type="table" />
        <div v-else-if="!backups.length" class="empty"><div class="empty-icon"><Icon name="save" :size="48" color="var(--text-quaternary)" /></div>暂无备份</div>
        <DataTable v-else :columns="backupColumns" :data="backups" :show-toolbar="false">
          <template #cell-name="{ value }"><span style="font-family:monospace;font-size:.8rem">{{ value }}</span></template>
          <template #cell-size="{ value }">{{ formatSize(value) }}</template>
          <template #cell-mod_time="{ value }">{{ fmtTime(value) }}</template>
          <template #actions="{ row }"><button class="btn btn-danger btn-sm" @click="confirmDeleteBackup(row)">删除</button></template>
        </DataTable>
      </div>
    </div>


    <!-- Tab: 数据库 -->
    <div v-show="activeTab === 'database'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header"><span class="card-icon">🗄️</span><span class="card-title">SQLite 监控</span><div class="card-actions"><button class="btn btn-ghost btn-sm" @click="loadSQLiteStats">刷新</button></div></div>
        <Skeleton v-if="sqliteStatsLoading && !sqliteStats" type="text" />
        <div v-else-if="sqliteStats" class="sqlite-panel">
          <div class="sqlite-overview">
            <div class="sqlite-stat-card">
              <div class="sqlite-stat-label">数据库文件</div>
              <div class="sqlite-stat-value">{{ sqliteStats.database?.size_human || '--' }}</div>
              <div class="sqlite-stat-sub">{{ sqliteStats.database?.size_bytes || 0 }} B</div>
            </div>
            <div class="sqlite-stat-card">
              <div class="sqlite-stat-label">WAL 文件</div>
              <div class="sqlite-stat-value">{{ sqliteStats.database?.wal_size_human || '--' }}</div>
              <div class="sqlite-stat-sub">{{ sqliteStats.database?.wal_size_bytes || 0 }} B</div>
            </div>
            <div class="sqlite-stat-card">
              <div class="sqlite-stat-label">表数量</div>
              <div class="sqlite-stat-value">{{ sqliteStats.table_count ?? '--' }}</div>
              <div class="sqlite-stat-sub">page_count={{ sqliteStats.pragmas?.page_count ?? '--' }}</div>
            </div>
            <div class="sqlite-stat-card">
              <div class="sqlite-stat-label">最近写入 QPS</div>
              <div class="sqlite-stat-value">{{ formatQPS(sqliteStats.write_qps) }}</div>
              <div class="sqlite-stat-sub">1 分钟 {{ sqliteStats.recent_writes_1m || 0 }} 次</div>
            </div>
          </div>

          <div class="card" style="margin-bottom:16px">
            <div class="card-header"><span class="card-icon">⚙️</span><span class="card-title">SQLite PRAGMA</span></div>
            <div class="status-row"><span class="status-key">page_size</span><span class="status-val">{{ sqliteStats.pragmas?.page_size ?? '--' }}</span></div>
            <div class="status-row"><span class="status-key">page_count</span><span class="status-val">{{ sqliteStats.pragmas?.page_count ?? '--' }}</span></div>
            <div class="status-row"><span class="status-key">wal_autocheckpoint</span><span class="status-val">{{ sqliteStats.pragmas?.wal_autocheckpoint ?? '--' }}</span></div>
          </div>

          <div class="card">
            <div class="card-header"><span class="card-icon">📊</span><span class="card-title">Top 10 表行数</span></div>
            <div v-if="!(sqliteStats.tables || []).length" class="empty" style="padding:24px">暂无表数据</div>
            <div v-else class="sqlite-bars">
              <div v-for="table in sqliteStats.tables" :key="table.name" class="sqlite-bar-row">
                <div class="sqlite-bar-head">
                  <span class="sqlite-bar-name">{{ table.name }}</span>
                  <span class="sqlite-bar-value">{{ table.rows }}</span>
                </div>
                <div class="sqlite-bar-track">
                  <div class="sqlite-bar-fill" :style="{ width: sqliteBarWidth(table.rows) + '%' }"></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Tab 3: 系统信息 -->
    <div v-show="activeTab === 'system'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg></span><span class="card-title">系统信息</span><div class="card-actions"><button class="btn btn-ghost btn-sm" @click="refreshHealth">刷新</button></div></div>
        <Skeleton v-if="!appState.health" type="text" />
        <div v-else>
          <div class="status-grid">
            <div class="ring-chart"><svg width="100" height="100" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="none" stroke="rgba(255,255,255,0.06)" stroke-width="8" /><circle cx="50" cy="50" r="40" fill="none" :stroke="ringColor" stroke-width="8" :stroke-dasharray="C" :stroke-dashoffset="ringOffset" stroke-linecap="round" style="transition:stroke-dashoffset .6s" /></svg><span class="ring-label" :style="{ color: ringColor }">{{ pct }}%</span></div>
            <div class="status-info">
              <div class="status-row"><span class="status-key">总体状态</span><span class="status-val" :style="{ color: statusColor }">{{ statusText }}</span></div>
              <div class="status-row"><span class="status-key">版本</span><span class="status-val">{{ health.version }}</span></div>
              <div class="status-row"><span class="status-key">运行时间</span><span class="status-val">{{ formattedUptime }}</span></div>
              <div class="status-row"><span class="status-key">模式</span><span class="status-val">{{ health.mode || '--' }}</span></div>
              <div class="status-row"><span class="status-key">上游</span><span class="status-val">{{ healthyUp }}/{{ totalUp }}</span></div>
              <div class="status-row"><span class="status-key">路由数</span><span class="status-val">{{ health.routes?.total || 0 }}</span></div>
              <div class="status-row"><span class="status-key">审计日志</span><span class="status-val">{{ health.audit?.total || 0 }}</span></div>
              <div class="status-row"><span class="status-key">限流</span><span class="status-val">{{ rlText }}</span></div>
            </div>
          </div>
        </div>
      </div>
      <div class="card">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span><span class="card-title">健康检查详情</span></div>
        <Skeleton v-if="!appState.health || !appState.health.checks" type="text" />
        <div v-else><div v-for="hc in healthCheckList" :key="hc.name" class="status-row"><span class="status-key">{{ hc.icon }} {{ hc.name }}</span><span class="status-val" :style="{ color: hc.color }">{{ hc.val }}</span></div></div>
      </div>
    </div>

    <!-- Tab 4: LLM 代理 -->
    <div v-show="activeTab === 'llm'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44"/><path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44"/></svg></span><span class="card-title">LLM 代理配置</span></div>
        <Skeleton v-if="llmConfigLoading" type="text" />
        <div v-else-if="!llmConfig" style="color:var(--text-tertiary);font-size:var(--text-sm)">LLM 代理未启用</div>
        <div v-else>
          <div class="llm-row"><span class="llm-label">启用状态</span><span class="llm-val"><label class="toggle-switch"><input type="checkbox" v-model="llmConfig.enabled" /><span class="toggle-slider"></span></label><span style="margin-left:8px;font-size:var(--text-xs);color:var(--text-tertiary)">{{ llmConfig.enabled ? '已启用' : '未启用' }} · 修改需重启</span></span></div>
          <div class="llm-row"><span class="llm-label">监听端口</span><input type="text" v-model="llmConfig.listen" class="llm-input-sm" style="width:140px;font-family:var(--font-mono)" placeholder=":8445" /><span class="llm-hint">修改需重启</span></div>
          <div id="section-security" class="llm-section-title">安全策略</div>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.security.scan_pii_in_response" /> 扫描响应中的 PII</label>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.security.prompt_injection_scan" /> 扫描 Prompt Injection</label>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.security.block_high_risk_tools" /> 拦截高危工具调用</label>
          <div class="llm-row" style="margin-top:8px"><span class="llm-label">高危工具</span><input type="text" v-model="highRiskToolsStr" class="llm-input" placeholder="exec, shell, bash" /></div>
          <div id="section-canary" class="llm-section-title">🐤 Canary Token</div>
          <label class="llm-checkbox"><input type="checkbox" v-model="canaryEnabled" /> 启用 Canary Token</label>
          <template v-if="canaryEnabled">
            <div class="llm-row"><span class="llm-label">Token</span><span class="llm-val" style="font-family:var(--font-mono);font-size:var(--text-xs)">{{ canaryStatus.token || '(未配置)' }}</span><button class="btn btn-sm" @click="rotateCanary" style="margin-left:8px;font-size:11px;padding:2px 8px">轮换</button></div>
            <div class="llm-row"><span class="llm-label">泄露动作</span><select v-model="canaryAlertAction" class="llm-input-sm" style="width:100px"><option value="log">log</option><option value="warn">warn</option><option value="block">block</option></select></div>
            <label class="llm-checkbox"><input type="checkbox" v-model="canaryAutoRotate" /> 每24h自动轮换</label>
            <div class="llm-row"><span class="llm-label">最近泄露</span><span class="llm-val" :style="{ color: (canaryStatus.leak_count||0) > 0 ? 'var(--color-danger)' : 'var(--text-secondary)' }">{{ canaryStatus.leak_count || 0 }} 次</span></div>
          </template>
          <div id="section-budget" class="llm-section-title"><Icon name="bar-chart" :size="16" /> Response Budget</div>
          <label class="llm-checkbox"><input type="checkbox" v-model="budgetEnabled" /> 启用预算控制</label>
          <div v-if="budgetEnabled">
            <div class="llm-row"><span class="llm-label">最大工具调用</span><input type="number" v-model.number="budgetMaxTools" class="llm-input-sm" min="1" max="100" /><span class="llm-hint">次/请求</span></div>
            <div class="llm-row"><span class="llm-label">单类工具</span><input type="number" v-model.number="budgetMaxSingle" class="llm-input-sm" min="1" max="50" /><span class="llm-hint">次/请求</span></div>
            <div class="llm-row"><span class="llm-label">最大 Token</span><input type="number" v-model.number="budgetMaxTokens" class="llm-input-sm" style="width:100px" min="1000" max="10000000" step="10000" /><span class="llm-hint">Token/请求</span></div>
            <div class="llm-row"><span class="llm-label">超限动作</span><select v-model="budgetAction" class="llm-input-sm" style="width:100px"><option value="warn">warn</option><option value="block">block</option></select></div>
            <div class="llm-row"><span class="llm-label">工具限制</span><input type="text" v-model="budgetToolLimitsStr" class="llm-input" placeholder="exec=3, shell=2" /></div>
            <div class="llm-row"><span class="llm-label">24h超限</span><span class="llm-val" :style="{ color: (budgetStatus.violations_24h||0) > 0 ? 'var(--color-warning)' : 'var(--text-secondary)' }">{{ budgetStatus.violations_24h || 0 }} 次</span></div>
          </div>
          <div class="llm-section-title">审计配置</div>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.audit.log_tool_input" /> 记录工具调用输入</label>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.audit.log_tool_result" /> 记录工具调用结果</label>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.audit.log_system_prompt" /> 记录 System Prompt</label>
          <div class="llm-row" style="margin-top:8px"><span class="llm-label">摘要长度</span><input type="number" v-model.number="llmConfig.audit.max_preview_len" class="llm-input-sm" min="100" max="5000" /><span class="llm-hint">字符</span></div>
          <div id="section-cost" class="llm-section-title">成本预警</div>
          <div class="llm-row"><span class="llm-label">日限额</span><span class="llm-hint" style="margin-right:4px">$</span><input type="number" v-model.number="llmConfig.cost_alert.daily_limit_usd" class="llm-input-sm" min="0" step="5" /><span class="llm-hint">USD</span></div>
          <div class="llm-row"><span class="llm-label">Webhook</span><input type="text" v-model="llmConfig.cost_alert.webhook_url" class="llm-input" placeholder="https://..." /></div>
          <div class="llm-section-title llm-advanced-toggle" @click="showAdvanced = !showAdvanced" style="cursor:pointer;user-select:none"><span>{{ showAdvanced ? '▾' : '▸' }} 高级配置</span></div>
          <div v-show="showAdvanced">
            <div class="llm-row"><span class="llm-label">超时</span><input type="number" v-model.number="llmConfig.timeout_sec" class="llm-input-sm" min="5" max="300" /><span class="llm-hint">秒</span></div>
            <div class="llm-row"><span class="llm-label">请求体限制</span><input type="number" v-model.number="llmConfig.max_body_bytes" class="llm-input-sm" style="width:120px" min="0" step="1048576" /><span class="llm-hint">字节</span></div>
            <div v-if="llmConfig.targets && llmConfig.targets.length" class="llm-targets"><div v-for="t in llmConfig.targets" :key="t.name" class="llm-target-row"><code>{{ t.name }}</code><span style="color:var(--text-tertiary)">{{ t.upstream }}</span></div></div>
          </div>
          <div style="margin-top:var(--space-4);display:flex;align-items:center;gap:var(--space-3)">
            <button class="btn btn-sm" @click="saveLLMConfig" :disabled="llmSaving">{{ llmSaving ? '保存中...' : '保存配置' }}</button>
            <span class="llm-restart-hint">⚠️ 部分变更需重启生效</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Tab: 检测引擎开关 -->
    <div v-show="activeTab === 'engines'">
      <Skeleton v-if="enginesLoading" type="table" />
      <template v-else>
        <div class="card" style="margin-bottom:16px">
          <div class="card-header">
            <span class="card-icon">🧠</span><span class="card-title">检测引擎总览</span>
            <div class="card-actions">
              <span class="engine-stats"><span class="engine-stat-on">{{ engineOnCount }} 启用</span><span class="engine-stat-off">{{ engineOffCount }} 关闭</span></span>
              <button class="btn btn-ghost btn-sm" @click="loadEngineSettings">刷新</button>
            </div>
          </div>
          <div class="config-desc">统一管理所有安全引擎的启用/禁用状态，修改即时生效。</div>
        </div>
        <div class="engine-list">
          <div v-for="eng in engineList" :key="eng.configPath" class="engine-row card">
            <div class="engine-info">
              <div class="engine-name-row">
                <span class="engine-status-dot" :class="{ on: eng.alwaysOn || engineSettings[eng.configPath] }"></span>
                <span class="engine-name">{{ eng.name }}</span>
                <span v-if="eng.alwaysOn" class="engine-always-on-tag">始终启用</span>
              </div>
              <div class="engine-desc">{{ eng.desc }}</div>
              <div class="engine-path"><code>{{ eng.configPath }}</code></div>
            </div>
            <div class="engine-toggle">
              <label v-if="!eng.alwaysOn" class="toggle-switch toggle-switch-lg"><input type="checkbox" :checked="engineSettings[eng.configPath]" @change="toggleEngine(eng, $event)" :disabled="eng.saving" /><span class="toggle-slider"></span></label>
              <span v-else class="engine-locked">🔒</span>
              <span v-if="eng.saving" class="engine-saving">保存中...</span>
            </div>
          </div>
        </div>
        <div class="card" style="margin-top:16px;border-color:rgba(99,102,241,.22)">
          <div class="card-header"><span class="card-icon">🧩</span><span class="card-title">CaMeL 三引擎独立开关</span></div>
          <div class="engine-list">
            <div v-for="item in camelEngines" :key="item.name" class="engine-row card">
              <div class="engine-info">
                <div class="engine-name-row"><span class="engine-status-dot" :class="{ on: item.enabled }"></span><span class="engine-name">{{ item.title }}</span></div>
                <div class="engine-desc">{{ item.desc }}</div>
                <div class="engine-path"><code>{{ item.name }}</code></div>
                <div class="engine-desc" style="margin-top:8px">{{ item.stat1Label }}：{{ item.stat1 }} · {{ item.stat2Label }}：{{ item.stat2 }}</div>
              </div>
              <div class="engine-toggle">
                <label class="toggle-switch toggle-switch-lg"><input type="checkbox" :checked="item.enabled" @change="toggleCamelEngine(item, $event)" /><span class="toggle-slider"></span></label>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>

    <div v-show="activeTab === 'llm'" class="card" style="margin-top:16px;border-color:rgba(99,102,241,.22)">
      <div class="card-header"><span class="card-icon">🐤</span><span class="card-title">金丝雀令牌管理</span></div>
      <div class="config-desc">创建时间：{{ canaryRotationStatus.token_created_at || '--' }} · 下次自动轮换：{{ canaryRotationStatus.next_rotation_at || '--' }}</div>
      <div style="margin-top:12px"><button class="btn btn-sm btn-primary" @click="confirmCanaryRotateNow">立即轮换</button></div>
      <div style="margin-top:16px" v-if="canaryRotationHistory.length">
        <div v-for="item in canaryRotationHistory" :key="item.rotated_at" class="status-row"><span class="status-key">{{ fmtTime(item.rotated_at) }}</span><span class="status-val">{{ item.old_token_hash }} → {{ item.new_token_hash }}</span></div>
      </div>
    </div>

    <!-- Tab: AC 智能分级 -->
    <div v-show="activeTab === 'autoreview'">
      <Skeleton v-if="arLoading" type="text" />
      <template v-else>
        <div class="card" style="margin-bottom:16px">
          <div class="card-header"><span class="card-icon">🎚️</span><span class="card-title">AC 自动 Review</span><div class="card-actions"><button class="btn btn-ghost btn-sm" @click="loadAutoReview">刷新</button></div></div>
          <div class="cfg-item" style="border-bottom:none;padding-top:4px">
            <div class="cfg-toggle-row">
              <label class="toggle-switch toggle-switch-lg"><input type="checkbox" v-model="arConfig.enabled" /><span class="toggle-slider"></span></label>
              <span class="cfg-toggle-label" style="font-weight:600">{{ arConfig.enabled ? '已启用' : '未启用' }}</span>
            </div>
          </div>
          <div class="ar-params">
            <div class="ar-param"><span class="ar-param-label">窗口时间</span><div class="cfg-inline"><input type="number" v-model.number="arConfig.window_sec" class="cfg-input-num" min="10" max="3600" step="10" /><span class="cfg-unit">秒</span></div></div>
            <div class="ar-param"><span class="ar-param-label">突增阈值</span><div class="cfg-inline"><input type="number" v-model.number="arConfig.spike_threshold" class="cfg-input-num" min="1" max="100" step="1" /><span class="cfg-unit">次</span></div></div>
            <div class="ar-param"><span class="ar-param-label">突增倍数</span><div class="cfg-inline"><input type="number" v-model.number="arConfig.spike_ratio" class="cfg-input-num" min="1" max="20" step="0.5" /><span class="cfg-unit">x</span></div></div>
            <div class="ar-param"><span class="ar-param-label">Review TTL</span><div class="cfg-inline"><input type="number" v-model.number="arConfig.auto_review_ttl" class="cfg-input-num" min="60" max="86400" step="60" /><span class="cfg-unit">秒</span></div></div>
          </div>
          <div class="card-header" style="margin-top:16px;padding:0"><span class="card-icon">🤖</span><span class="card-title" style="font-size:.9rem">LLM 复核上游</span></div>
          <div class="ar-params" style="margin-top:8px">
            <div class="ar-param"><span class="ar-param-label">Base URL</span><input v-model="arConfig.llm_endpoint" class="cfg-input-text" placeholder="https://api.deepseek.com" /></div>
            <div class="ar-param"><span class="ar-param-label">模型名称</span><input v-model="arConfig.llm_model" class="cfg-input-text" placeholder="deepseek-chat" /></div>
            <div class="ar-param"><span class="ar-param-label">API Key</span><div class="cfg-inline" style="position:relative"><input :type="showArKey ? 'text' : 'password'" v-model="arConfig.llm_api_key" class="cfg-input-text" placeholder="sk-..." style="flex:1;padding-right:32px" /><button class="btn btn-ghost btn-xs" style="position:absolute;right:4px;top:50%;transform:translateY(-50%)" @click="showArKey=!showArKey">{{ showArKey ? '🙈' : '👁️' }}</button></div></div>
            <div class="ar-param"><span class="ar-param-label">超时</span><div class="cfg-inline"><input type="number" v-model.number="arConfig.llm_timeout_sec" class="cfg-input-num" min="1" max="30" step="1" /><span class="cfg-unit">秒</span></div></div>
          </div>
          <div style="margin-top:12px;display:flex;gap:8px"><button class="btn btn-sm btn-primary" @click="saveAutoReviewConfig" :disabled="arSaving">{{ arSaving ? '保存中...' : '保存配置' }}</button></div>
        </div>
        <div class="card" style="margin-bottom:16px">
          <div class="card-header"><span class="card-icon">📋</span><span class="card-title">规则状态</span></div>
          <div v-if="!arRules.length" class="empty" style="padding:24px"><div class="empty-icon">📭</div>暂无规则状态数据</div>
          <div v-else class="table-wrap">
            <table>
              <thead><tr><th>规则名</th><th>当前动作</th><th>原始动作</th><th>降级原因</th><th>剩余时间</th><th>操作</th></tr></thead>
              <tbody>
                <tr v-for="r in arRules" :key="r.name">
                  <td><code class="rule-name-code">{{ r.name }}</code></td>
                  <td><span class="ar-action-badge" :class="'ar-action-' + r.current_action">{{ r.current_action }}</span></td>
                  <td><span class="ar-action-badge ar-action-original">{{ r.original_action }}</span></td>
                  <td><span class="ar-reason">{{ r.reason || '--' }}</span></td>
                  <td><span class="ar-ttl">{{ r.ttl_remaining ? formatTTL(r.ttl_remaining) : '--' }}</span></td>
                  <td class="ar-ops">
                    <button v-if="r.current_action === r.original_action" class="btn btn-xs" @click="confirmReviewRule(r)" :disabled="r.reviewing">⬇ 降级</button>
                    <button v-else class="btn btn-xs btn-primary" @click="confirmRestoreRule(r)" :disabled="r.restoring">⬆ 恢复</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div class="card" style="margin-bottom:16px">
          <div class="card-header"><span class="card-icon">📊</span><span class="card-title">LLM 复核统计</span></div>
          <div class="ar-stats-grid">
            <div class="ar-stat-card"><div class="ar-stat-value">{{ arStats.total_reviews || 0 }}</div><div class="ar-stat-label">总复核次数</div></div>
            <div class="ar-stat-card"><div class="ar-stat-value" style="color:var(--color-success)">{{ arStats.pass_rate != null ? (arStats.pass_rate * 100).toFixed(1) + '%' : '--' }}</div><div class="ar-stat-label">通过率</div></div>
            <div class="ar-stat-card"><div class="ar-stat-value" style="color:var(--color-warning)">{{ arStats.avg_latency_ms != null ? arStats.avg_latency_ms.toFixed(0) + 'ms' : '--' }}</div><div class="ar-stat-label">平均延迟</div></div>
          </div>
        </div>
        <!-- 人工 Review 规则已移除：review 现在是规则的第四种 action，直接在规则编辑器里设置 -->
      </template>
    </div>

    <!-- Tab: 行业模板管理 -->
    <div v-show="activeTab === 'templates'">
      <Skeleton v-if="tplLoading" type="table" />
      <template v-else>
        <div class="card" style="margin-bottom:20px">
          <div class="card-header"><span class="card-icon">⬇️</span><span class="card-title">入站检测模板</span><div class="card-actions"><button class="btn btn-ghost btn-sm" @click="loadInboundTemplates">刷新</button></div></div>
          <div v-if="!inboundTemplates.length" class="empty" style="padding:24px"><div class="empty-icon">📦</div>暂无入站模板</div>
          <div v-else class="table-wrap">
            <table>
              <thead><tr><th>模板名称</th><th>规则数</th><th>分类</th><th>全局启用</th><th>操作</th></tr></thead>
              <tbody>
                <template v-for="tpl in inboundTemplates" :key="tpl.id">
                  <tr><td><strong>{{ tpl.name }}</strong></td><td>{{ (tpl.rules || []).length }}</td><td><span class="tpl-category-badge">{{ tpl.category || '通用' }}</span></td><td><label class="toggle-switch"><input type="checkbox" :checked="tpl.enabled" @change="toggleInboundTemplate(tpl, $event)" :disabled="tpl.toggling" /><span class="toggle-slider"></span></label></td><td><button class="btn btn-xs btn-ghost" @click="toggleTplExpand('inbound', tpl.id)">{{ tplExpandedIds['inbound_' + tpl.id] ? '收起' : '查看规则' }}</button></td></tr>
                  <tr v-if="tplExpandedIds['inbound_' + tpl.id]" class="expand-row"><td colspan="5"><div class="tpl-rules-detail"><div v-if="!tpl.rules || !tpl.rules.length" class="empty-hint">暂无规则</div><div v-for="(rule, ri) in (tpl.rules || [])" :key="ri" class="tpl-rule-item"><span class="tpl-rule-name">{{ rule.name }}</span><span class="tpl-rule-action" :class="'action-' + rule.action">{{ rule.action }}</span><span class="tpl-rule-type">{{ rule.type || 'keyword' }}</span></div></div></td></tr>
                </template>
              </tbody>
            </table>
          </div>
        </div>
        <div class="card">
          <div class="card-header"><span class="card-icon">🤖</span><span class="card-title">LLM 检测模板</span><div class="card-actions"><button class="btn btn-ghost btn-sm" @click="loadLLMTemplates">刷新</button></div></div>
          <div v-if="!llmTemplates.length" class="empty" style="padding:24px"><div class="empty-icon">📦</div>暂无 LLM 模板</div>
          <div v-else class="table-wrap">
            <table>
              <thead><tr><th>模板名称</th><th>规则数</th><th>分类</th><th>全局启用</th><th>操作</th></tr></thead>
              <tbody>
                <template v-for="tpl in llmTemplates" :key="tpl.id">
                  <tr><td><strong>{{ tpl.name }}</strong></td><td>{{ (tpl.rules || []).length }}</td><td><span class="tpl-category-badge">{{ tpl.category || '通用' }}</span></td><td><label class="toggle-switch"><input type="checkbox" :checked="tpl.enabled" @change="toggleLLMTemplate(tpl, $event)" :disabled="tpl.toggling" /><span class="toggle-slider"></span></label></td><td><button class="btn btn-xs btn-ghost" @click="toggleTplExpand('llm', tpl.id)">{{ tplExpandedIds['llm_' + tpl.id] ? '收起' : '查看规则' }}</button></td></tr>
                  <tr v-if="tplExpandedIds['llm_' + tpl.id]" class="expand-row"><td colspan="5"><div class="tpl-rules-detail"><div v-if="!tpl.rules || !tpl.rules.length" class="empty-hint">暂无规则</div><div v-for="(rule, ri) in (tpl.rules || [])" :key="ri" class="tpl-rule-item"><span class="tpl-rule-name">{{ rule.name }}</span><span class="tpl-rule-action" :class="'action-' + rule.action">{{ rule.action }}</span><span class="tpl-rule-type">{{ rule.type || 'keyword' }}</span></div></div></td></tr>
                </template>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </div>

    <ConfirmModal :visible="confirmVisible" :title="confirmTitle" :message="confirmMessage" :type="confirmType" @confirm="doConfirm" @cancel="confirmVisible = false" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, inject, onMounted, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { api, apiPost, apiPut, apiDelete, saveToken, clearToken, getToken } from '../api.js'
import { showToast, updateHealth } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import Icon from '../components/Icon.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Skeleton from '../components/Skeleton.vue'
import ConfigWizard from '../components/ConfigWizard.vue'

const appState = inject('appState')
const route = useRoute()
const tokenValue = ref(getToken())
const showToken = ref(false)
const backups = ref([])
const backupsLoading = ref(false)
const demoLoading = ref(false)
const demoResult = ref(null)
const confirmVisible = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmType = ref('danger')
let confirmAction = null

const tabs = [
  { key: 'config', label: '配置管理', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9c-.11-.65-.56-1.15-1.16-1.41l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06c.5.46 1.17.62 1.82.33A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09c.11.65.56 1.15 1.16 1.41.65.29 1.32.13 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06c-.46.5-.62 1.17-.33 1.82V9c.26.6.77 1.05 1.41 1.16H21a2 2 0 0 1 0 4h-.09c-.64.11-1.15.56-1.41 1.16z"/></svg>' },
  { key: 'engines', label: '检测引擎', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2a10 10 0 1 0 10 10A10 10 0 0 0 12 2zm0 18a8 8 0 1 1 8-8 8 8 0 0 1-8 8z"/><path d="M12 6v6l4 2"/></svg>' },
  { key: 'autoreview', label: 'AC 分级', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 3h18v18H3z"/><path d="M3 9h18M3 15h18M9 3v18"/></svg>' },
  { key: 'templates', label: '行业模板', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>' },
  { key: 'auth', label: '认证与安全', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>' },
  { key: 'system', label: '系统信息', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>' },
  { key: 'database', label: '数据库', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/></svg>' },
  { key: 'llm', label: 'LLM 代理', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44"/><path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44"/></svg>' },
]
const activeTab = ref('config')

const sqliteStatsLoading = ref(false)
const sqliteStats = ref(null)

const configGroups = [
  { key: 'basic', label: '配置基础', icon: '🌐' },
  { key: 'security', label: '安全检测', icon: '🛡️' },
  // 引擎开关子 Tab 已移除，统一在 "检测引擎" 顶级 Tab
  { key: 'ratelimit', label: '限流配置', icon: '⚡' },
  { key: 'session', label: '会话关联', icon: '🔗' },
  { key: 'alerts', label: '告警配置', icon: '🔔' },
  { key: 'advanced', label: '高级配置', icon: '⚙️' },
]
const activeGroup = ref('basic')
const configLoading = ref(true)
const configSaving = ref(false)
const showChangesPreview = ref(false)
const originalConfig = ref({})
const wizardVisible = ref(false)

const form = reactive({
  inbound_listen: '', outbound_listen: '', management_listen: '',
  openclaw_upstream: '', lanxin_upstream: '', default_gateway_origin: 'http://localhost', log_level: 'info', log_format: 'text',
  inbound_detect_enabled: true, outbound_audit_enabled: true, detect_timeout_ms: 50,
  rate_limit: { global_rps: 0, global_burst: 0, per_sender_rps: 0, per_sender_burst: 0 },
  session_idle_timeout_min: 60, session_fp_window_sec: 300,
  alert_webhook: '', alert_format: 'generic', alert_min_interval: 60,
  db_path: '', heartbeat_interval_sec: 10, route_default_policy: 'least-users',
  audit_retention_days: 30, ws_idle_timeout: 300, backup_auto_interval: 0,
  // 引擎开关已统一到 "检测引擎" Tab
})
const errors = reactive({})

const visibleGroups = computed(() => [
  { key: 'basic', icon: '🌐', title: '基础配置', desc: '监听端口、上游地址与日志配置', items: [
    { key: 'inbound_listen', label: '入站监听地址', desc: '入站流量代理监听端口（如 :8443）', placeholder: ':8443', restart: true },
    { key: 'outbound_listen', label: '出站监听地址', desc: '出站审计代理监听端口（如 :8444）', placeholder: ':8444', restart: true },
    { key: 'management_listen', label: '管理端口', desc: 'Management API 监听地址', placeholder: ':9090', restart: true },
    { key: 'openclaw_upstream', label: 'OpenClaw 上游', desc: 'OpenClaw Gateway 转发地址', placeholder: 'http://localhost:18790', wide: true },
    { key: 'default_gateway_origin', label: 'Gateway 默认 Origin', desc: 'WSS RPC 连接时发送的 Origin header，需在 Gateway allowedOrigins 白名单中', placeholder: 'http://localhost', wide: true },
    { key: 'lanxin_upstream', label: '蓝信上游', desc: '蓝信 API 网关地址', placeholder: 'https://apigw.lx.qianxin.com', wide: true },
    { key: 'log_level', label: '日志级别', desc: '运行日志详细程度', options: [{value:'debug',label:'debug'},{value:'info',label:'info'},{value:'warn',label:'warn'},{value:'error',label:'error'}] },
    { key: 'log_format', label: '日志格式', desc: '输出格式', options: [{value:'text',label:'text'},{value:'json',label:'json'}] },
  ]},
  { key: 'security', icon: '🛡️', title: '安全检测', desc: '入站检测与出站审计的开关和超时设置', items: [
    { key: 'inbound_detect_enabled', label: '入站检测', desc: '对入站流量执行规则匹配和威胁检测', type: 'toggle' },
    { key: 'outbound_audit_enabled', label: '出站审计', desc: '对出站流量执行 PII 扫描和内容审计', type: 'toggle' },
    { key: 'detect_timeout_ms', label: '检测超时', desc: '单次检测最大耗时，超时则放行', type: 'number', min: 1, max: 10000, step: 10, unit: 'ms' },
  ]},
  // 引擎开关已统一到 "检测引擎" 顶级 Tab，此处不再重复
  { key: 'ratelimit', icon: '⚡', title: '限流配置', desc: '令牌桶限流参数，0 = 不限制', items: [
    { field: 'global_rps', label: '全局 RPS', desc: '每秒最大通过数', type: 'number', min: 0, max: 100000, step: 10, unit: 'req/s' },
    { field: 'global_burst', label: '全局 Burst', desc: '突发容量', type: 'number', min: 0, max: 10000, step: 1 },
    { field: 'per_sender_rps', label: '每用户 RPS', desc: '单用户每秒最大通过数', type: 'number', min: 0, max: 10000, step: 1, unit: 'req/s' },
    { field: 'per_sender_burst', label: '每用户 Burst', desc: '单用户突发容量', type: 'number', min: 0, max: 1000, step: 1 },
  ]},
  { key: 'session', icon: '🔗', title: '会话关联', desc: 'IM↔LLM 会话自动关联参数', items: [
    { key: 'session_idle_timeout_min', label: '空闲超时', desc: '超过此时间则切换新会话', type: 'number', min: 1, max: 1440, step: 1, unit: '分钟' },
    { key: 'session_fp_window_sec', label: '指纹窗口', desc: 'LLM 请求匹配 IM 消息指纹窗口', type: 'number', min: 10, max: 3600, step: 10, unit: '秒' },
  ]},
  { key: 'advanced', icon: '⚙️', title: '高级配置', desc: '数据库、心跳、路由策略与备份', items: [
    { key: 'db_path', label: '数据库路径', desc: 'SQLite 审计日志存储路径', placeholder: '/var/lib/lobster-guard/audit.db', restart: true },
    { key: 'heartbeat_interval_sec', label: '心跳间隔', desc: '上游健康检查周期', type: 'number', min: 1, max: 300, step: 1, unit: '秒' },
    { key: 'route_default_policy', label: '路由策略', desc: '新用户默认分配策略', options: [{value:'least-users',label:'最少用户'},{value:'round-robin',label:'轮询'},{value:'random',label:'随机'}] },
    { key: 'audit_retention_days', label: '日志保留', desc: '审计日志自动清理天数', type: 'number', min: 1, max: 365, step: 1, unit: '天' },
    { key: 'ws_idle_timeout', label: 'WS 空闲超时', desc: 'WebSocket 空闲自动断开', type: 'number', min: 0, max: 86400, step: 60, unit: '秒' },
    { key: 'backup_auto_interval', label: '自动备份', desc: '0 = 不自动备份', type: 'number', min: 0, max: 168, step: 1, unit: '小时' },
  ]},
])

const restartFields = new Set(['inbound_listen', 'outbound_listen', 'management_listen', 'db_path'])
const fieldLabels = computed(() => {
  const m = {}
  for (const g of visibleGroups.value) for (const i of g.items) m[i.key || ('rate_limit.' + i.field)] = i.label
  m['alert_webhook'] = 'Webhook URL'; m['alert_format'] = '告警格式'; m['alert_min_interval'] = '最小间隔'
  return m
})
function isChanged(key) { return String(form[key]) !== String(originalConfig.value[key]) }
function isRLChanged(field) { return String(form.rate_limit[field]) !== String((originalConfig.value.rate_limit || {})[field]) }
const changedFields = computed(() => {
  const c = []
  const flat = ['inbound_listen','outbound_listen','management_listen','openclaw_upstream','lanxin_upstream','default_gateway_origin','log_level','log_format','inbound_detect_enabled','outbound_audit_enabled','detect_timeout_ms','session_idle_timeout_min','session_fp_window_sec','alert_webhook','alert_format','alert_min_interval','db_path','heartbeat_interval_sec','route_default_policy','audit_retention_days','ws_idle_timeout','backup_auto_interval']
  for (const k of flat) if (String(form[k]) !== String(originalConfig.value[k])) c.push({ key: k, label: fieldLabels.value[k]||k, oldVal: originalConfig.value[k], newVal: form[k], restart: restartFields.has(k) })
  const orl = originalConfig.value.rate_limit || {}
  for (const f of ['global_rps','global_burst','per_sender_rps','per_sender_burst']) if (String(form.rate_limit[f]) !== String(orl[f])) c.push({ key:'rate_limit.'+f, label: fieldLabels.value['rate_limit.'+f]||f, oldVal: orl[f], newVal: form.rate_limit[f] })
  return c
})
const hasChanges = computed(() => changedFields.value.length > 0)
const alerts = ref([]); const alertsLoading = ref(false); const alertTesting = ref(false); const alertTestResult = ref(null)

async function loadConfig() {
  configLoading.value = true
  try { const d = await api('/api/v1/config'); fillForm(d); originalConfig.value = JSON.parse(JSON.stringify(extractForm())) }
  catch (e) { showToast('加载配置失败: ' + e.message, 'error') }
  configLoading.value = false
}
function fillForm(d) {
  form.inbound_listen = d.inbound_listen || ':8443'; form.outbound_listen = d.outbound_listen || ':8444'
  form.management_listen = d.management_listen || ':9090'; form.openclaw_upstream = d.openclaw_upstream || ''
  form.lanxin_upstream = d.lanxin_upstream || ''; form.default_gateway_origin = d.default_gateway_origin || 'http://localhost'; form.log_level = d.log_level || 'info'; form.log_format = d.log_format || 'text'
  form.inbound_detect_enabled = d.inbound_detect_enabled !== false; form.outbound_audit_enabled = d.outbound_audit_enabled !== false
  form.detect_timeout_ms = d.detect_timeout_ms || 50
  const rl = d.rate_limit || {}; form.rate_limit.global_rps = rl.global_rps || 0; form.rate_limit.global_burst = rl.global_burst || 0
  form.rate_limit.per_sender_rps = rl.per_sender_rps || 0; form.rate_limit.per_sender_burst = rl.per_sender_burst || 0
  form.session_idle_timeout_min = d.session_idle_timeout_min || 60; form.session_fp_window_sec = d.session_fp_window_sec || 300
  form.alert_webhook = d.alert_webhook || ''; form.alert_format = d.alert_format || 'generic'; form.alert_min_interval = d.alert_min_interval || 60
  form.db_path = d.db_path || ''; form.heartbeat_interval_sec = d.heartbeat_interval_sec || 10
  form.route_default_policy = d.route_default_policy || 'least-users'; form.audit_retention_days = d.audit_retention_days || 30
  form.ws_idle_timeout = d.ws_idle_timeout || 300; form.backup_auto_interval = d.backup_auto_interval || 0
  // 引擎开关已统一到 "检测引擎" Tab，不再在配置管理表单中管理
}
function extractForm() {
  return { inbound_listen: form.inbound_listen, outbound_listen: form.outbound_listen, management_listen: form.management_listen,
    openclaw_upstream: form.openclaw_upstream, lanxin_upstream: form.lanxin_upstream, default_gateway_origin: form.default_gateway_origin, log_level: form.log_level, log_format: form.log_format,
    inbound_detect_enabled: form.inbound_detect_enabled, outbound_audit_enabled: form.outbound_audit_enabled, detect_timeout_ms: form.detect_timeout_ms,
    rate_limit: { ...form.rate_limit }, session_idle_timeout_min: form.session_idle_timeout_min, session_fp_window_sec: form.session_fp_window_sec,
    alert_webhook: form.alert_webhook, alert_format: form.alert_format, alert_min_interval: form.alert_min_interval,
    db_path: form.db_path, heartbeat_interval_sec: form.heartbeat_interval_sec, route_default_policy: form.route_default_policy,
    audit_retention_days: form.audit_retention_days, ws_idle_timeout: form.ws_idle_timeout, backup_auto_interval: form.backup_auto_interval,
    }
}
function resetConfig() { fillForm(originalConfig.value); Object.keys(errors).forEach(k => delete errors[k]); showChangesPreview.value = false }
function validateForm() {
  Object.keys(errors).forEach(k => delete errors[k]); let ok = true
  for (const k of ['inbound_listen','outbound_listen','management_listen']) {
    const v = form[k]; if (v && !/^:?\d+$/.test(v) && !/^[\w.-]+:\d+$/.test(v)) { errors[k] = '格式无效'; ok = false }
    else if (v) { const port = parseInt(v.split(':').pop()); if (port < 1 || port > 65535) { errors[k] = '端口 1-65535'; ok = false } }
  }
  if (form.openclaw_upstream && !/^https?:\/\//.test(form.openclaw_upstream)) { errors.openclaw_upstream = '需 http(s)://'; ok = false }
  if (form.detect_timeout_ms < 1) { errors.detect_timeout_ms = '> 0'; ok = false }
  if (form.session_idle_timeout_min < 1 || form.session_idle_timeout_min > 1440) { errors.session_idle_timeout_min = '1-1440'; ok = false }
  if (form.session_fp_window_sec < 10 || form.session_fp_window_sec > 3600) { errors.session_fp_window_sec = '10-3600'; ok = false }
  return ok
}
async function saveConfig() {
  if (!validateForm()) { showToast('请修正错误', 'error'); return }
  const danger = changedFields.value.filter(c => (c.key === 'inbound_detect_enabled' && !form.inbound_detect_enabled) || (c.key === 'outbound_audit_enabled' && !form.outbound_audit_enabled))
  if (danger.length) { confirmTitle.value = '⚠️ 危险变更'; confirmMessage.value = '关闭检测：' + danger.map(c => c.label).join('、') + '？'; confirmType.value = 'danger'; confirmAction = () => doSaveConfig(); confirmVisible.value = true; return }
  await doSaveConfig()
}
async function doSaveConfig() {
  configSaving.value = true
  try { const d = await apiPut('/api/v1/config/settings', extractForm()); showToast(d.need_restart ? '已保存 — 需重启生效' : '保存成功', d.need_restart ? 'warning' : 'success'); originalConfig.value = JSON.parse(JSON.stringify(extractForm())); showChangesPreview.value = false }
  catch (e) { showToast('保存失败: ' + e.message, 'error') }
  configSaving.value = false
}
async function testAlert() { alertTesting.value = true; alertTestResult.value = null; try { await apiPost('/api/v1/alerts/test', {}); alertTestResult.value = { ok: true, msg: '✅ 已发送' } } catch (e) { alertTestResult.value = { ok: false, msg: '❌ ' + e.message } } alertTesting.value = false }
async function loadAlertHistory() { alertsLoading.value = true; try { const d = await api('/api/v1/alerts/history'); alerts.value = d.alerts || [] } catch { alerts.value = [] } alertsLoading.value = false }

// LLM config
const llmConfigLoading = ref(true); const llmConfig = ref(null); const llmSaving = ref(false); const showAdvanced = ref(false)
const highRiskToolsStr = computed({ get: () => llmConfig.value?.security?.high_risk_tool_list?.join(', ') || '', set: (v) => { if (llmConfig.value?.security) llmConfig.value.security.high_risk_tool_list = v.split(',').map(s => s.trim()).filter(Boolean) } })
const canaryStatus = ref({ token: '', leak_count: 0 })
const canaryRotationStatus = ref({})
const canaryRotationHistory = ref([])
const planCompilerStats = ref({})
const capabilityEngineStats = ref({})
const deviationDetectorStats = ref({})
const camelEngines = computed(() => [
  { name: 'plan-compiler', title: 'PlanCompiler', desc: '执行计划编译器', enabled: !!engineSettings['engine_plan_compiler'], stat1Label: '活跃计划数', stat1: planCompilerStats.value.active_plans || 0, stat2Label: '模板数', stat2: planCompilerStats.value.templates || 0 },
  { name: 'capability-engine', title: 'CapabilityEngine', desc: '能力标签引擎', enabled: !!engineSettings['engine_capability'], stat1Label: '工具映射数', stat1: capabilityEngineStats.value.tool_mappings || 0, stat2Label: '活跃上下文数', stat2: capabilityEngineStats.value.active_contexts || 0 },
  { name: 'deviation-detector', title: 'DeviationDetector', desc: '偏差检测器', enabled: !!engineSettings['engine_deviation'], stat1Label: '检测到的偏差数', stat1: deviationDetectorStats.value.detected_deviations || 0, stat2Label: '策略数', stat2: deviationDetectorStats.value.policies || 0 },
])
const canaryEnabled = computed({ get: () => llmConfig.value?.security?.canary_token?.enabled ?? true, set: (v) => { if (llmConfig.value?.security?.canary_token) llmConfig.value.security.canary_token.enabled = v } })
const canaryAlertAction = computed({ get: () => llmConfig.value?.security?.canary_token?.alert_action || 'warn', set: (v) => { if (llmConfig.value?.security?.canary_token) llmConfig.value.security.canary_token.alert_action = v } })
const canaryAutoRotate = computed({ get: () => llmConfig.value?.security?.canary_token?.auto_rotate ?? false, set: (v) => { if (llmConfig.value?.security?.canary_token) llmConfig.value.security.canary_token.auto_rotate = v } })
const budgetStatus = ref({ violations_24h: 0 })
const budgetEnabled = computed({ get: () => llmConfig.value?.security?.response_budget?.enabled ?? false, set: (v) => { if (llmConfig.value?.security?.response_budget) llmConfig.value.security.response_budget.enabled = v } })
const budgetMaxTools = computed({ get: () => llmConfig.value?.security?.response_budget?.max_tool_calls_per_req || 20, set: (v) => { if (llmConfig.value?.security?.response_budget) llmConfig.value.security.response_budget.max_tool_calls_per_req = v } })
const budgetMaxSingle = computed({ get: () => llmConfig.value?.security?.response_budget?.max_single_tool_per_req || 5, set: (v) => { if (llmConfig.value?.security?.response_budget) llmConfig.value.security.response_budget.max_single_tool_per_req = v } })
const budgetMaxTokens = computed({ get: () => llmConfig.value?.security?.response_budget?.max_tokens_per_req || 100000, set: (v) => { if (llmConfig.value?.security?.response_budget) llmConfig.value.security.response_budget.max_tokens_per_req = v } })
const budgetAction = computed({ get: () => llmConfig.value?.security?.response_budget?.over_budget_action || 'warn', set: (v) => { if (llmConfig.value?.security?.response_budget) llmConfig.value.security.response_budget.over_budget_action = v } })
const budgetToolLimitsStr = computed({
  get: () => { const l = llmConfig.value?.security?.response_budget?.tool_limits; if (!l || typeof l !== 'object') return ''; return Object.entries(l).map(([k,v])=>`${k}=${v}`).join(', ') },
  set: (v) => { if (!llmConfig.value?.security?.response_budget) return; const l = {}; v.split(',').map(s=>s.trim()).filter(Boolean).forEach(p => { const [k,val] = p.split('=').map(s=>s.trim()); if(k&&val) l[k]=parseInt(val)||5 }); llmConfig.value.security.response_budget.tool_limits = l }
})
async function loadLLMConfig() {
  llmConfigLoading.value = true
  try { const d = await api('/api/v1/llm/config'); if (!d.audit) d.audit = { log_system_prompt: false, log_tool_input: true, log_tool_result: true, max_preview_len: 500 }; if (!d.cost_alert) d.cost_alert = { daily_limit_usd: 50, webhook_url: '' }; if (!d.security) d.security = { scan_pii_in_response: true, block_high_risk_tools: false, high_risk_tool_list: ['exec','shell','bash'], prompt_injection_scan: true }; if (!d.security.canary_token) d.security.canary_token = { enabled: true, auto_rotate: false, alert_action: 'warn' }; if (!d.security.response_budget) d.security.response_budget = { enabled: false, max_tool_calls_per_req: 20, max_single_tool_per_req: 5, max_tokens_per_req: 100000, over_budget_action: 'warn', tool_limits: {} }; llmConfig.value = d; loadCanaryStatus(); loadBudgetStatus() }
  catch { llmConfig.value = null }; llmConfigLoading.value = false
}
async function loadCanaryStatus() { try { canaryStatus.value = await api('/api/v1/llm/canary/status') } catch {} }
async function loadBudgetStatus() { try { budgetStatus.value = await api('/api/v1/llm/budget/status') } catch {} }
async function rotateCanary() { try { const d = await apiPost('/api/v1/llm/canary/rotate', {}); showToast('Canary Token 已轮换', 'success'); canaryStatus.value.token = d.token } catch (e) { showToast('轮换失败: ' + e.message, 'error') } }
async function loadCanaryRotationData() {
  try { canaryRotationStatus.value = await api('/api/v1/canary/status') } catch {}
  try { const d = await api('/api/v1/canary/history'); canaryRotationHistory.value = d.history || [] } catch {}
}
async function loadCamelStats() {
  try { planCompilerStats.value = await api('/api/v1/plans/stats') } catch {}
  try { capabilityEngineStats.value = await api('/api/v1/capabilities/stats') } catch {}
  try { deviationDetectorStats.value = await api('/api/v1/deviations/stats') } catch {}
}
function confirmCanaryRotateNow() { confirmTitle.value = '确认立即轮换金丝雀令牌？'; confirmMessage.value = '将生成新 token 并写回 config.yaml'; confirmType.value = 'danger'; confirmAction = async () => { try { await apiPost('/api/v1/canary/rotate', {}); showToast('金丝雀令牌已轮换', 'success'); await loadCanaryRotationData() } catch (e) { showToast('轮换失败: ' + e.message, 'error') } }; confirmVisible.value = true }
async function toggleCamelEngine(item, event) {
  const enabled = event.target.checked
  try {
    await apiPost(`/api/v1/engines/${item.name}/toggle`, { enabled })
    if (item.name === 'plan-compiler') engineSettings['engine_plan_compiler'] = enabled
    if (item.name === 'capability-engine') engineSettings['engine_capability'] = enabled
    if (item.name === 'deviation-detector') engineSettings['engine_deviation'] = enabled
    showToast(`${item.title} 已${enabled ? '启用' : '关闭'}`, 'success')
  } catch (e) {
    event.target.checked = !enabled
    showToast('切换失败: ' + e.message, 'error')
  }
}
async function saveLLMConfig() { if (!llmConfig.value) return; llmSaving.value = true; try { const d = await apiPut('/api/v1/llm/config', llmConfig.value); showToast(d.need_restart ? '已保存（需重启）' : 'LLM 已保存', 'success') } catch (e) { showToast('失败: ' + e.message, 'error') }; llmSaving.value = false }

// 通用
const backupColumns = [{ key: 'name', label: '文件名', sortable: true },{ key: 'size', label: '大小', sortable: true },{ key: 'mod_time', label: '时间', sortable: true }]
const C = 2 * Math.PI * 40
const health = computed(() => appState.health || {})
const totalUp = computed(() => health.value.upstreams?.total || 0)
const healthyUp = computed(() => health.value.upstreams?.healthy || 0)
const pct = computed(() => totalUp.value > 0 ? Math.round(healthyUp.value / totalUp.value * 100) : 100)
const ringOffset = computed(() => C - pct.value / 100 * C)
const ringColor = computed(() => pct.value >= 80 ? 'var(--color-success)' : pct.value >= 50 ? 'var(--color-warning)' : 'var(--color-danger)')
const statusText = computed(() => { const s = health.value.status; return s === 'healthy' ? '健康' : s === 'degraded' ? '降级' : '异常' })
const statusColor = computed(() => { const s = health.value.status; return s === 'healthy' ? 'var(--color-success)' : s === 'degraded' ? 'var(--color-warning)' : 'var(--color-danger)' })
const rlText = computed(() => { const rl = health.value.rate_limiter; if (!rl || !rl.enabled) return '未启用'; return `${rl.global_rps||'?'} / ${rl.per_sender_rps||'?'} rps` })
const formattedUptime = computed(() => {
  const raw = health.value.uptime; if (!raw || raw === '--') return '--'; let s = 0
  const hm = raw.match(/([\d.]+)h/); if (hm) s += parseFloat(hm[1]) * 3600
  const mm = raw.match(/([\d.]+)m(?!s)/); if (mm) s += parseFloat(mm[1]) * 60
  const sm = raw.match(/([\d.]+)s/); if (sm) s += parseFloat(sm[1])
  if (s <= 0) return raw; const min = Math.floor(s/60), hr = Math.floor(s/3600), d = Math.floor(s/86400)
  if (min < 1) return '< 1 min'; if (hr < 1) return min + ' min'; if (d < 1) return hr + 'h ' + (min%60) + 'm'; return d + 'd ' + (hr%24) + 'h'
})
const healthCheckList = computed(() => {
  const checks = appState.health?.checks; if (!checks) return []
  const dims = [{ k:'database',n:'数据库',fn:c=>c.latency_ms!=null?c.latency_ms.toFixed(1)+'ms':'' },{ k:'upstream',n:'上游服务',fn:c=>c.healthy!=null?c.healthy+'/'+c.total:'' },{ k:'disk',n:'磁盘',fn:c=>c.used_percent!=null?c.used_percent.toFixed(1)+'%':'' },{ k:'memory',n:'内存',fn:c=>c.alloc_mb!=null?c.alloc_mb.toFixed(1)+' MB':'' },{ k:'goroutines',n:'Goroutines',fn:c=>c.count!=null?String(c.count):'' }]
  return dims.filter(d=>checks[d.k]).map(d=>{ const c=checks[d.k]; return { name:d.n, val:d.fn(c), color: c.status==='ok'?'var(--color-success)':c.status==='warning'?'var(--color-warning)':'var(--color-danger)', icon: c.status==='ok'?'✅':c.status==='warning'?'⚠️':'❌' } })
})

function fmtTime(ts) { if (!ts) return '--'; const d = new Date(ts); return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false }) }
function formatSize(bytes) { const kb = Math.round((bytes||0)/1024); return kb > 1024 ? (kb/1024).toFixed(1)+' MB' : kb+' KB' }
function doSaveToken() { const v = tokenValue.value.trim(); if (v) { saveToken(v); showToast('Token 已保存', 'success') } else showToast('请输入', 'error') }
function doClearToken() { clearToken(); tokenValue.value = ''; showToast('已清除', 'success') }
function confirmClearToken() { confirmTitle.value = '清除 Token'; confirmMessage.value = '确认清除 Bearer Token？清除后需重新输入。'; confirmType.value = 'danger'; confirmAction = () => doClearToken(); confirmVisible.value = true }
function confirmClearDemo() { confirmTitle.value = '清除演示数据'; confirmMessage.value = '确认清除所有演示数据？此操作不可恢复。'; confirmType.value = 'danger'; confirmAction = () => clearDemo(); confirmVisible.value = true }
async function refreshHealth() { try { const d = await api('/healthz'); updateHealth(d) } catch {} }
async function loadBackups() { backupsLoading.value = true; try { const d = await api('/api/v1/backups'); backups.value = d.backups || [] } catch { backups.value = [] }; backupsLoading.value = false }
async function createBackup() { try { await apiPost('/api/v1/backup', {}); showToast('备份成功', 'success'); loadBackups() } catch (e) { showToast('失败: ' + e.message, 'error') } }
async function seedDemo() { demoLoading.value = true; demoResult.value = null; try { const d = await apiPost('/api/v1/demo/seed', {}); demoResult.value = { ok: true, message: '✅ 注入 ' + d.count + ' 条' }; showToast('注入 ' + d.count + ' 条', 'success') } catch (e) { demoResult.value = { ok: false, message: '❌ ' + e.message } }; demoLoading.value = false }
async function clearDemo() { demoLoading.value = true; demoResult.value = null; try { const d = await apiDelete('/api/v1/demo/clear'); demoResult.value = { ok: true, message: '✅ 清除 ' + d.deleted + ' 条' } } catch (e) { demoResult.value = { ok: false, message: '❌ ' + e.message } }; demoLoading.value = false }
function confirmDeleteBackup(row) { confirmTitle.value = '删除备份'; confirmMessage.value = '确认删除 ' + row.name + '？'; confirmType.value = 'danger'; confirmAction = async () => { try { await api('/api/v1/backups/' + encodeURIComponent(row.name), { method: 'DELETE' }); showToast('已删除', 'success'); loadBackups() } catch (e) { showToast('失败: ' + e.message, 'error') } }; confirmVisible.value = true }
function doConfirm() { confirmVisible.value = false; if (confirmAction) confirmAction() }

// === Tab switching with lazy loading ===
function switchTab(key) {
  activeTab.value = key
  if (key === 'engines' && !enginesLoaded.value) loadEngineSettings()
  if (key === 'autoreview' && !arLoaded.value) loadAutoReview()
  if (key === 'templates' && !tplLoaded.value) { loadInboundTemplates(); loadLLMTemplates() }
  if (key === 'database' && !sqliteStats.value) loadSQLiteStats()
}

async function loadSQLiteStats() {
  sqliteStatsLoading.value = true
  try { sqliteStats.value = await api('/api/v1/debug/sqlite-stats') }
  catch (e) { showToast('加载 SQLite 监控失败: ' + e.message, 'error') }
  sqliteStatsLoading.value = false
}

const sqliteTableMaxRows = computed(() => {
  const rows = sqliteStats.value?.tables || []
  return rows.reduce((m, it) => Math.max(m, Number(it.rows || 0)), 0) || 1
})
function sqliteBarWidth(rows) {
  return Math.max(6, Math.round((Number(rows || 0) / sqliteTableMaxRows.value) * 100))
}
function formatQPS(v) {
  return Number(v || 0).toFixed(2)
}

// === Tab 2: 检测引擎开关 ===
const enginesLoading = ref(false)
const enginesLoaded = ref(false)
const engineSettings = reactive({})
const engineList = [
  { name: '入站检测', configPath: 'engine_inbound_detect', desc: 'AC自动机+正则+PII三阶段检测' },
  { name: '会话检测', configPath: 'engine_session_detect', desc: '多轮会话上下文关联+风险累积' },
  { name: 'LLM检测', configPath: 'engine_llm_detect', desc: 'Pipeline内LLM调用检测' },
  { name: '语义检测', configPath: 'engine_semantic', desc: '基于embedding的语义匹配' },
  { name: '蜜罐引擎', configPath: 'honeypot', desc: '模式匹配→假响应+水印', alwaysOn: true },
  { name: '深度蜜罐', configPath: 'engine_honeypot_deep', desc: '多轮交互蜜罐+行为建模' },
  { name: '奇点引擎', configPath: 'engine_singularity', desc: '概率性诱饵投放+预算控制' },
  { name: 'IFC引擎', configPath: 'engine_ifc', desc: 'Bell-LaPadula双标签信息流控制' },
  { name: 'IFC隔离', configPath: 'engine_ifc_quarantine', desc: '违规隔离到安全上游' },
  { name: 'IFC隐藏', configPath: 'engine_ifc_hiding', desc: 'Selective Hide基于标签隐藏' },
  { name: '路径策略', configPath: 'engine_path_policy', desc: '条件匹配→allow/deny/audit' },
  { name: '工具策略', configPath: 'engine_tool_policy', desc: '工具级黑/白名单' },
  { name: '计划编译器', configPath: 'engine_plan_compiler', desc: '预编译合法tool_call序列' },
  { name: '能力引擎', configPath: 'engine_capability', desc: 'Sources并集+Labels交集+TrustScore' },
  { name: '偏差检测', configPath: 'engine_deviation', desc: '计划vs实际偏差+自动修复' },
  { name: '反事实验证', configPath: 'engine_counterfactual', desc: '置换输入验证因果关系' },
  { name: '执行信封', configPath: 'engine_envelope', desc: '密封决策证据链+Merkle树' },
  { name: '演化引擎', configPath: 'engine_evolution', desc: '红队结果→规则自动演化' },
  { name: '自适应决策', configPath: 'engine_adaptive', desc: '多因素动态决策阈值' },
  { name: '污点追踪', configPath: 'engine_taint_tracker', desc: '全链路数据流标记' },
  { name: '污点溯源', configPath: 'engine_taint_reversal', desc: '反向追踪数据来源' },
  { name: '事件总线', configPath: 'engine_event_bus', desc: '引擎间事件发布/订阅' },
]
const engineOnCount = computed(() => engineList.filter(e => e.alwaysOn || engineSettings[e.configPath]).length)
const engineOffCount = computed(() => engineList.length - engineOnCount.value)

// configPath → Config JSON 的取值路径映射
const engineConfigPaths = {
  engine_inbound_detect: 'inbound_detect_enabled',
  engine_session_detect: 'session_detect_enabled',
  engine_llm_detect: 'llm_detect_enabled',
  engine_semantic: 'semantic_detector.enabled',
  engine_honeypot_deep: 'honeypot_deep.enabled',
  engine_singularity: 'singularity.enabled',
  engine_ifc: 'ifc.enabled',
  engine_ifc_quarantine: 'ifc.quarantine_enabled',
  engine_ifc_hiding: 'ifc.hiding_enabled',
  engine_path_policy: 'path_policy.enabled',
  engine_tool_policy: 'tool_policy.enabled',
  engine_plan_compiler: 'plan_compiler.enabled',
  engine_capability: 'capability.enabled',
  engine_deviation: 'deviation.enabled',
  engine_counterfactual: 'counterfactual.enabled',
  engine_envelope: 'envelope_enabled',
  engine_evolution: 'evolution_enabled',
  engine_adaptive: 'adaptive_decision.enabled',
  engine_taint_tracker: 'taint_tracker.enabled',
  engine_taint_reversal: 'taint_reversal.enabled',
  engine_event_bus: 'event_bus.enabled',
}

async function loadEngineSettings() {
  enginesLoading.value = true
  try {
    const d = await api('/api/v1/config/settings')
    for (const eng of engineList) {
      if (eng.alwaysOn) continue
      const jsonPath = engineConfigPaths[eng.configPath] || eng.configPath
      const parts = jsonPath.split('.')
      let val = d
      for (const p of parts) { val = val?.[p] }
      engineSettings[eng.configPath] = val !== false && val !== undefined
    }
    enginesLoaded.value = true
  } catch (e) { showToast('加载引擎配置失败: ' + e.message, 'error') }
  enginesLoading.value = false
}

async function toggleEngine(eng, event) {
  const newVal = event.target.checked
  eng.saving = true
  try {
    await apiPut('/api/v1/config/settings', { [eng.configPath]: newVal })
    engineSettings[eng.configPath] = newVal
    showToast(`${eng.name} 已${newVal ? '启用' : '关闭'}`, 'success')
  } catch (e) {
    event.target.checked = !newVal
    showToast('切换失败: ' + e.message, 'error')
  }
  eng.saving = false
}

// === Tab 3: AC 智能分级 ===
const arLoading = ref(false)
const arLoaded = ref(false)
const arSaving = ref(false)
const arConfig = reactive({ enabled: false, window_sec: 300, spike_threshold: 10, spike_ratio: 3, auto_review_ttl: 3600, llm_endpoint: '', llm_model: '', llm_api_key: '', llm_timeout_sec: 10 })
const showArKey = ref(false)
const arRules = ref([])
const arStats = ref({})
// arManualRules removed: review is now a direct rule action

async function loadAutoReview() {
  arLoading.value = true
  try {
    const [status, stats] = await Promise.all([
      api('/api/v1/auto-review/status'),
      api('/api/v1/auto-review/stats').catch(() => ({}))
    ])
    if (status.config) {
      const c = status.config
      arConfig.enabled = c.enabled !== false
      arConfig.window_sec = c.window_seconds || 300
      arConfig.spike_threshold = c.spike_threshold || 10
      arConfig.spike_ratio = c.spike_ratio || 3
      arConfig.auto_review_ttl = c.auto_review_ttl || 3600
      arConfig.llm_endpoint = c.llm_endpoint || ''
      arConfig.llm_model = c.llm_model || ''
      arConfig.llm_api_key = c.llm_api_key || ''
      arConfig.llm_timeout_sec = c.llm_timeout_sec || 10
    }
    arRules.value = status.rules || []
    // manual_review_rules no longer used in UI
    arStats.value = stats || {}
    arLoaded.value = true
  } catch (e) { showToast('加载 AC 分级失败: ' + e.message, 'error') }
  arLoading.value = false
}

async function saveAutoReviewConfig() {
  arSaving.value = true
  try {
    await apiPost('/api/v1/auto-review/config', { enabled: arConfig.enabled, window_seconds: arConfig.window_sec, spike_threshold: arConfig.spike_threshold, spike_ratio: arConfig.spike_ratio, auto_review_ttl: arConfig.auto_review_ttl, llm_endpoint: arConfig.llm_endpoint, llm_model: arConfig.llm_model, llm_api_key: arConfig.llm_api_key, llm_timeout_sec: arConfig.llm_timeout_sec })
    showToast('AC 分级配置已保存', 'success')
  } catch (e) { showToast('保存失败: ' + e.message, 'error') }
  arSaving.value = false
}

function confirmReviewRule(r) {
  confirmTitle.value = '手动降级'; confirmMessage.value = '确认降级规则 "' + r.name + '" ？降级后该规则动作将被调低。'
  confirmType.value = 'warning'; confirmAction = () => doReviewRule(r); confirmVisible.value = true
}
async function doReviewRule(r) {
  r.reviewing = true
  try { await apiPost('/api/v1/auto-review/rules/' + encodeURIComponent(r.name) + '/review', {}); showToast('已降级 ' + r.name, 'success'); loadAutoReview() }
  catch (e) { showToast('降级失败: ' + e.message, 'error') }
  r.reviewing = false
}

function confirmRestoreRule(r) {
  confirmTitle.value = '手动恢复'; confirmMessage.value = '确认恢复规则 "' + r.name + '" 到原始动作？'
  confirmType.value = 'warning'; confirmAction = () => doRestoreRule(r); confirmVisible.value = true
}
async function doRestoreRule(r) {
  r.restoring = true
  try { await apiPost('/api/v1/auto-review/rules/' + encodeURIComponent(r.name) + '/restore', {}); showToast('已恢复 ' + r.name, 'success'); loadAutoReview() }
  catch (e) { showToast('恢复失败: ' + e.message, 'error') }
  r.restoring = false
}

function formatTTL(sec) {
  if (!sec || sec <= 0) return '--'
  if (sec < 60) return sec + 's'
  if (sec < 3600) return Math.floor(sec / 60) + 'm ' + (sec % 60) + 's'
  return Math.floor(sec / 3600) + 'h ' + Math.floor((sec % 3600) / 60) + 'm'
}

// === Tab 4: 行业模板管理 ===
const tplLoading = ref(false)
const tplLoaded = ref(false)
const inboundTemplates = ref([])
const llmTemplates = ref([])
const tplExpandedIds = reactive({})

async function loadInboundTemplates() {
  tplLoading.value = true
  try { const d = await api('/api/v1/inbound-templates'); inboundTemplates.value = d.templates || d || []; tplLoaded.value = true }
  catch (e) { showToast('加载入站模板失败: ' + e.message, 'error') }
  tplLoading.value = false
}

async function loadLLMTemplates() {
  try { const d = await api('/api/v1/llm/templates'); llmTemplates.value = d.templates || d || [] }
  catch (e) { showToast('加载 LLM 模板失败: ' + e.message, 'error') }
}

async function toggleInboundTemplate(tpl, event) {
  const newVal = event.target.checked
  tpl.toggling = true
  try { await apiPost('/api/v1/inbound-templates/' + tpl.id + '/enable', { enabled: newVal }); tpl.enabled = newVal; showToast(tpl.name + (newVal ? ' 已启用' : ' 已禁用'), 'success') }
  catch (e) { event.target.checked = !newVal; showToast('切换失败: ' + e.message, 'error') }
  tpl.toggling = false
}

async function toggleLLMTemplate(tpl, event) {
  const newVal = event.target.checked
  tpl.toggling = true
  try { await apiPost('/api/v1/llm/templates/' + tpl.id + '/enable', { enabled: newVal }); tpl.enabled = newVal; showToast(tpl.name + (newVal ? ' 已启用' : ' 已禁用'), 'success') }
  catch (e) { event.target.checked = !newVal; showToast('切换失败: ' + e.message, 'error') }
  tpl.toggling = false
}

function toggleTplExpand(type, id) { const key = type + '_' + id; tplExpandedIds[key] = !tplExpandedIds[key] }

function scrollToSection(section) {
  if (!section) return
  if (['canary','budget','security','cost'].includes(section)) activeTab.value = 'llm'
  nextTick(() => { const el = document.getElementById('section-' + section); if (el) { el.scrollIntoView({ behavior: 'smooth', block: 'start' }); el.classList.add('section-highlight'); setTimeout(() => el.classList.remove('section-highlight'), 2000) } })
}

onMounted(() => {
  loadConfig(); loadBackups(); loadLLMConfig(); loadAlertHistory(); loadSQLiteStats(); loadCanaryRotationData(); loadCamelStats(); loadEngineSettings()
  const section = route.query.section
  if (section) { const poll = () => { if (!llmConfigLoading.value) scrollToSection(section); else setTimeout(poll, 100) }; poll() }
})
</script>

<style scoped>
.settings-tabs { display: flex; gap: 0; margin-bottom: 20px; border-bottom: 2px solid var(--border-subtle); }
.settings-tab { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-3) var(--space-4); background: none; border: none; cursor: pointer; color: var(--text-secondary); font-size: var(--text-sm); font-weight: 500; border-bottom: 2px solid transparent; margin-bottom: -2px; transition: all var(--transition-fast); position: relative; }
.settings-tab:hover { color: var(--text-primary); background: var(--bg-hover); }
.settings-tab.active { color: var(--color-primary); border-bottom-color: var(--color-primary); }
.tab-icon { display: flex; align-items: center; }
.tab-label { white-space: nowrap; }
.tab-badge { color: var(--color-warning); font-size: 10px; position: absolute; top: 6px; right: 4px; }

/* 配置分组导航 */
.config-group-nav { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 16px; }
.config-group-btn { padding: 6px 14px; border-radius: 20px; border: 1px solid var(--border-default); background: var(--bg-elevated); color: var(--text-secondary); font-size: var(--text-sm); cursor: pointer; transition: all var(--transition-fast); white-space: nowrap; }
.config-group-btn:hover { border-color: var(--color-primary); color: var(--text-primary); }
.config-group-btn.active { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }

/* 变更预览栏 */
.changes-bar { background: rgba(99,102,241,0.08); border: 1px solid rgba(99,102,241,0.2); border-radius: var(--radius-md); padding: 10px 14px; margin-bottom: 16px; }
.changes-bar-inner { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); flex-wrap: wrap; }
.changes-dot { color: var(--color-warning); font-size: 14px; }
.changes-preview { margin-top: 10px; padding-top: 10px; border-top: 1px solid rgba(99,102,241,0.15); }
.change-row { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); padding: 3px 0; font-family: var(--font-mono); }
.change-key { color: var(--text-secondary); min-width: 120px; }
.change-old { color: var(--color-danger); text-decoration: line-through; }
.change-arrow { color: var(--text-tertiary); }
.change-new { color: var(--color-success); font-weight: 600; }

/* 配置卡片 */
.config-card { margin-bottom: 16px; }
.config-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin-bottom: var(--space-3); }
.config-items { display: flex; flex-direction: column; gap: 0; }

/* 配置项 */
.cfg-item { padding: 12px 0; border-bottom: 1px solid var(--border-subtle); }
.cfg-item:last-child { border-bottom: none; }
.cfg-item-head { display: flex; align-items: center; gap: var(--space-2); margin-bottom: 2px; }
.cfg-item-label { font-size: var(--text-sm); font-weight: 600; color: var(--text-primary); }
.cfg-item-desc { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: 8px; }
.cfg-restart-tag { font-size: 10px; padding: 1px 6px; border-radius: 9999px; background: rgba(245,158,11,0.15); color: var(--color-warning); font-weight: 500; }
.cfg-changed-dot { color: var(--color-primary); font-size: 10px; }
.cfg-input { background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); outline: none; width: 200px; max-width: 100%; font-family: var(--font-mono); transition: border-color var(--transition-fast); }
.cfg-input:focus { border-color: var(--color-primary); box-shadow: 0 0 0 2px var(--color-primary-dim); }
.cfg-input-wide { width: 360px; }
.cfg-input-num { width: 100px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); outline: none; font-family: var(--font-mono); }
.cfg-input-num:focus { border-color: var(--color-primary); }
.cfg-input-text { width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); outline: none; }
.cfg-input-text:focus { border-color: var(--color-primary); }
.cfg-input-text::placeholder { color: var(--text-tertiary); }
.cfg-select { background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); outline: none; min-width: 140px; cursor: pointer; }
.cfg-select:focus { border-color: var(--color-primary); }
.cfg-inline { display: flex; align-items: center; gap: var(--space-2); }
.cfg-unit { font-size: var(--text-xs); color: var(--text-tertiary); }
.cfg-error { display: block; font-size: var(--text-xs); color: var(--color-danger); margin-top: 4px; }
.cfg-toggle-row { display: flex; align-items: center; gap: var(--space-2); }
.cfg-toggle-label { font-size: var(--text-sm); color: var(--text-secondary); }
.cfg-actions-row { display: flex; align-items: center; gap: var(--space-2); margin-top: var(--space-3); padding-top: var(--space-3); border-top: 1px solid var(--border-subtle); }
.cfg-hint { font-size: var(--text-xs); }

/* 告警列表 */
.alert-list { max-height: 360px; overflow-y: auto; }
.alert-item { padding: 8px 0; border-bottom: 1px solid var(--border-subtle); }
.alert-item:last-child { border-bottom: none; }
.alert-meta { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); margin-bottom: 4px; }
.alert-dir { padding: 1px 6px; border-radius: 4px; font-weight: 500; }
.dir-inbound { background: rgba(99,102,241,0.1); color: var(--color-primary); }
.dir-outbound { background: rgba(245,158,11,0.1); color: var(--color-warning); }
.alert-time { color: var(--text-tertiary); }
.alert-sender { color: var(--text-secondary); font-family: var(--font-mono); }
.alert-reason { font-size: var(--text-sm); color: var(--text-primary); }

/* Toggle switch */
.toggle-switch { position: relative; display: inline-block; width: 36px; height: 20px; cursor: pointer; }
.toggle-switch input { opacity: 0; width: 0; height: 0; }
.toggle-slider { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: 20px; transition: .3s; }
.toggle-slider:before { content: ''; position: absolute; height: 14px; width: 14px; left: 2px; bottom: 2px; background: var(--text-tertiary); border-radius: 50%; transition: .3s; }
.toggle-switch input:checked + .toggle-slider { background: var(--color-primary); border-color: var(--color-primary); }
.toggle-switch input:checked + .toggle-slider:before { transform: translateX(16px); background: #fff; }

/* 原有样式 */
.settings-section { margin-bottom: var(--space-4); }
.settings-label { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); color: var(--text-secondary); font-weight: 500; margin-bottom: var(--space-2); }
.token-input-wrap { position: relative; display: inline-flex; align-items: center; width: 320px; max-width: 100%; }
.token-input { width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-2) 40px var(--space-2) var(--space-3); font-size: var(--text-sm); outline: none; font-family: var(--font-mono); }
.token-input:focus { border-color: var(--color-primary); box-shadow: 0 0 0 3px var(--color-primary-dim); }
.token-toggle { position: absolute; right: 8px; top: 50%; transform: translateY(-50%); background: none; border: none; color: var(--text-tertiary); cursor: pointer; padding: 4px; border-radius: var(--radius-sm); display: flex; }
.token-toggle:hover { color: var(--text-primary); background: var(--bg-hover); }
.token-actions { display: flex; gap: var(--space-2); margin-top: var(--space-3); }
.llm-row { display: flex; align-items: center; gap: var(--space-2); margin-bottom: 6px; font-size: var(--text-sm); }
.llm-label { color: var(--text-secondary); min-width: 80px; flex-shrink: 0; }
.llm-val { color: var(--text-primary); display: flex; align-items: center; gap: var(--space-1); }
.llm-section-title { font-size: var(--text-xs); font-weight: 600; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text-tertiary); margin: 16px 0 8px; padding-top: 12px; border-top: 1px solid var(--border-subtle); }
.llm-targets { display: flex; flex-direction: column; gap: 4px; }
.llm-target-row { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); padding: 4px 8px; background: var(--bg-elevated); border-radius: var(--radius-sm); }
.llm-target-row code { background: var(--bg-base); padding: 1px 6px; border-radius: 3px; font-size: var(--text-xs); font-family: var(--font-mono); color: var(--color-primary); }
.llm-checkbox { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); color: var(--text-primary); cursor: pointer; margin-bottom: 4px; }
.llm-checkbox input[type="checkbox"] { accent-color: var(--color-primary); width: 16px; height: 16px; cursor: pointer; }
.llm-input-sm { width: 80px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 4px 8px; font-size: var(--text-sm); outline: none; font-family: var(--font-mono); }
.llm-input-sm:focus { border-color: var(--color-primary); }
.llm-input { flex: 1; max-width: 320px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 4px 8px; font-size: var(--text-sm); outline: none; }
.llm-input:focus { border-color: var(--color-primary); }
.llm-hint { font-size: var(--text-xs); color: var(--text-tertiary); }
.llm-restart-hint { font-size: var(--text-xs); color: var(--color-warning); }
.llm-advanced-toggle { display: flex; align-items: center; }
@keyframes section-flash { 0%, 100% { background: transparent; } 50% { background: rgba(99,102,241,0.1); } }
.section-highlight { animation: section-flash 0.5s ease 2; border-radius: var(--radius-sm); padding: 2px 4px; margin: -2px -4px; }
/* 检测引擎 Tab */
.engine-list { display: flex; flex-direction: column; gap: 8px; }
.engine-row { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px; }
.engine-info { flex: 1; min-width: 0; }
.engine-name-row { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.engine-status-dot { width: 10px; height: 10px; border-radius: 50%; background: var(--text-quaternary, #4a5568); flex-shrink: 0; transition: background .3s; }
.engine-status-dot.on { background: var(--color-success, #22c55e); box-shadow: 0 0 6px rgba(34,197,94,0.4); }
.engine-name { font-size: var(--text-sm); font-weight: 600; color: var(--text-primary); }
.engine-always-on-tag { font-size: 10px; padding: 1px 8px; border-radius: 9999px; background: rgba(34,197,94,0.12); color: var(--color-success); font-weight: 500; }
.engine-desc { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: 2px; padding-left: 18px; }
.engine-path { padding-left: 18px; }
.engine-path code { font-size: 11px; color: var(--text-quaternary, #64748b); font-family: var(--font-mono); }
.engine-toggle { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.engine-locked { font-size: 18px; }
.engine-saving { font-size: var(--text-xs); color: var(--color-warning); }
.engine-stats { display: flex; gap: 12px; font-size: var(--text-xs); }
.engine-stat-on { color: var(--color-success); font-weight: 600; }
.engine-stat-off { color: var(--text-tertiary); }
.toggle-switch-lg { width: 44px; height: 24px; }
.toggle-switch-lg .toggle-slider:before { height: 18px; width: 18px; }
.toggle-switch-lg input:checked + .toggle-slider:before { transform: translateX(20px); }

/* AC 分级 Tab */
.ar-params { display: flex; flex-wrap: wrap; gap: 16px; padding: 8px 0; }
.ar-param { display: flex; align-items: center; gap: 8px; }
.ar-param-label { font-size: var(--text-sm); color: var(--text-secondary); min-width: 70px; }
.ar-action-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: var(--text-xs); font-weight: 600; }
.ar-action-block { background: rgba(239,68,68,0.12); color: #EF4444; }
.ar-action-review { background: rgba(245,158,11,0.12); color: var(--color-warning); }
.ar-action-log, .ar-action-warn { background: rgba(99,102,241,0.1); color: var(--color-primary); }
.ar-action-allow { background: rgba(34,197,94,0.12); color: var(--color-success); }
.ar-action-original { background: var(--bg-elevated); color: var(--text-tertiary); }
.ar-reason { font-size: var(--text-xs); color: var(--text-tertiary); }
.ar-ttl { font-size: var(--text-xs); color: var(--text-secondary); font-family: var(--font-mono); }
.ar-ops { white-space: nowrap; }
.ar-stats-grid { display: flex; gap: 16px; flex-wrap: wrap; }
.ar-stat-card { flex: 1; min-width: 120px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 16px; text-align: center; }
.ar-stat-value { font-size: 1.5rem; font-weight: 700; color: var(--text-primary); }
.ar-stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: 4px; }
.ar-manual-list { display: flex; flex-direction: column; gap: 0; }
.ar-manual-item { display: flex; align-items: center; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid var(--border-subtle); }
.ar-manual-item:last-child { border-bottom: none; }
.ar-manual-item code { font-size: var(--text-sm); color: var(--color-primary); font-family: var(--font-mono); }
.ar-add-rule { display: flex; gap: 8px; align-items: center; }
.rule-name-code { font-size: var(--text-sm); color: var(--color-primary); font-family: var(--font-mono); }


/* SQLite 监控 Tab */
.sqlite-panel { display: flex; flex-direction: column; gap: 16px; }
.sqlite-overview { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; margin-bottom: 4px; }
.sqlite-stat-card { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 16px; }
.sqlite-stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: 6px; }
.sqlite-stat-value { font-size: 1.4rem; font-weight: 700; color: var(--text-primary); }
.sqlite-stat-sub { font-size: var(--text-xs); color: var(--text-secondary); margin-top: 6px; font-family: var(--font-mono); }
.sqlite-bars { display: flex; flex-direction: column; gap: 12px; }
.sqlite-bar-row { display: flex; flex-direction: column; gap: 6px; }
.sqlite-bar-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; font-size: var(--text-sm); }
.sqlite-bar-name { color: var(--text-primary); font-family: var(--font-mono); }
.sqlite-bar-value { color: var(--text-secondary); font-weight: 600; }
.sqlite-bar-track { height: 10px; background: var(--bg-elevated); border-radius: 9999px; overflow: hidden; border: 1px solid var(--border-subtle); }
.sqlite-bar-fill { height: 100%; background: linear-gradient(90deg, var(--color-primary), #22c55e); border-radius: 9999px; }

/* 行业模板 Tab */
.tpl-category-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: var(--text-xs); background: rgba(99,102,241,0.1); color: var(--color-primary); }
.tpl-rules-detail { padding: 8px 0; }
.tpl-rule-item { display: flex; align-items: center; gap: 8px; padding: 6px 0; border-bottom: 1px solid var(--border-subtle); font-size: var(--text-xs); }
.tpl-rule-item:last-child { border-bottom: none; }
.tpl-rule-name { font-weight: 600; color: var(--text-primary); min-width: 120px; }
.tpl-rule-action { padding: 1px 6px; border-radius: 3px; font-weight: 600; }
.tpl-rule-action.action-block { background: rgba(239,68,68,0.12); color: #EF4444; }
.tpl-rule-action.action-review { background: rgba(245,158,11,0.12); color: var(--color-warning); }
.tpl-rule-action.action-log { background: rgba(99,102,241,0.1); color: var(--color-primary); }
.tpl-rule-action.action-allow { background: rgba(34,197,94,0.12); color: var(--color-success); }
.tpl-rule-type { color: var(--text-tertiary); }
.tpl-rule-patterns { color: var(--text-quaternary, #64748b); font-family: var(--font-mono); font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 300px; }
.expand-row td { background: var(--bg-elevated); padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--border-subtle); }
.empty-hint { font-size: var(--text-sm); color: var(--text-tertiary); padding: 8px 0; }

/* Responsive tab overflow */
.settings-tabs { overflow-x: auto; -webkit-overflow-scrolling: touch; scrollbar-width: none; }
.settings-tabs::-webkit-scrollbar { display: none; }

@media (max-width: 640px) {
  .cfg-input-wide { width: 100%; }
  .config-group-nav { gap: 4px; }
  .config-group-btn { padding: 4px 10px; font-size: var(--text-xs); }
  .changes-bar-inner { flex-direction: column; align-items: flex-start; }
  .engine-row { flex-direction: column; align-items: flex-start; gap: 8px; }
  .ar-params { flex-direction: column; }
  .ar-stats-grid { flex-direction: column; }
  .settings-tab { padding: var(--space-2) var(--space-3); font-size: var(--text-xs); }
}
</style>
