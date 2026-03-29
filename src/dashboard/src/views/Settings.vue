<template>
  <div>
    <!-- Tab 导航 -->
    <div class="settings-tabs">
      <button
        v-for="tab in tabs" :key="tab.key"
        class="settings-tab" :class="{ active: activeTab === tab.key }"
        @click="activeTab = tab.key"
      >
        <span class="tab-icon" v-html="tab.icon"></span>
        <span class="tab-label">{{ tab.label }}</span>
      </button>
    </div>

    <!-- Tab 1: 认证与安全 -->
    <div v-show="activeTab === 'auth'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg></span>
          <span class="card-title">认证设置</span>
        </div>
        <div class="settings-section">
          <label class="settings-label">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
            Bearer Token
          </label>
          <div class="token-input-wrap">
            <input :type="showToken ? 'text' : 'password'" v-model="tokenValue" placeholder="输入 Bearer Token" class="token-input" />
            <button class="token-toggle" @click="showToken = !showToken" :title="showToken ? '隐藏' : '显示'">
              <svg v-if="showToken" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
              <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
            </button>
          </div>
          <div class="token-actions">
            <button class="btn btn-sm" @click="doSaveToken">保存</button>
            <button class="btn btn-danger btn-sm" @click="doClearToken">清除</button>
          </div>
        </div>
      </div>

      <div class="card" style="margin-bottom:20px">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg></span>
          <span class="card-title">演示数据</span>
        </div>
        <div style="font-size:var(--text-sm);color:var(--text-secondary);margin-bottom:var(--space-3)">注入模拟审计数据用于演示 Dashboard 图表效果。数据包含 250-300 条过去 7 天的模拟记录。</div>
        <div v-if="demoResult" style="margin-bottom:var(--space-3);padding:var(--space-2) var(--space-3);border-radius:var(--radius-md);font-size:var(--text-sm);background:var(--bg-elevated)">
          <span :style="{ color: demoResult.ok ? 'var(--color-success)' : 'var(--color-danger)' }">{{ demoResult.message }}</span>
        </div>
        <div style="display:flex;gap:var(--space-2)">
          <button class="btn btn-sm" @click="seedDemo" :disabled="demoLoading">{{ demoLoading ? '注入中...' : '注入演示数据' }}</button>
          <button class="btn btn-danger btn-sm" @click="clearDemo" :disabled="demoLoading">清除演示数据</button>
        </div>
      </div>

      <div class="card" style="margin-bottom:20px">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg></span>
          <span class="card-title">备份管理</span>
          <div class="card-actions">
            <button class="btn btn-sm" @click="createBackup">创建备份</button>
            <button class="btn btn-ghost btn-sm" @click="loadBackups">刷新</button>
          </div>
        </div>
        <Skeleton v-if="backupsLoading" type="table" />
        <div v-else-if="!backups.length" class="empty"><div class="empty-icon"><Icon name="save" :size="48" color="var(--text-quaternary)" /></div>暂无备份<div class="empty-hint">点击"创建备份"开始</div></div>
        <DataTable v-else :columns="backupColumns" :data="backups" :show-toolbar="false">
          <template #cell-name="{ value }"><span style="font-family:monospace;font-size:.8rem">{{ value }}</span></template>
          <template #cell-size="{ value }">{{ formatSize(value) }}</template>
          <template #cell-mod_time="{ value }">{{ fmtTime(value) }}</template>
          <template #actions="{ row }">
            <button class="btn btn-danger btn-sm" @click="confirmDeleteBackup(row)">删除</button>
          </template>
        </DataTable>
      </div>
    </div>

    <!-- Tab 2: 系统信息 -->
    <div v-show="activeTab === 'system'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg></span>
          <span class="card-title">系统信息</span>
          <div class="card-actions"><button class="btn btn-ghost btn-sm" @click="refreshHealth">刷新</button></div>
        </div>
        <Skeleton v-if="!appState.health" type="text" />
        <div v-else>
          <div class="status-grid">
            <div class="ring-chart">
              <svg width="100" height="100" viewBox="0 0 100 100">
                <circle cx="50" cy="50" r="40" fill="none" stroke="rgba(255,255,255,0.06)" stroke-width="8" />
                <circle cx="50" cy="50" r="40" fill="none" :stroke="ringColor" stroke-width="8" :stroke-dasharray="C" :stroke-dashoffset="ringOffset" stroke-linecap="round" style="transition:stroke-dashoffset .6s" />
              </svg>
              <span class="ring-label" :style="{ color: ringColor }">{{ pct }}%</span>
            </div>
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
          <div v-if="health.mode === 'bridge' && health.bridge" style="margin-top:16px;border-top:1px solid var(--border-subtle);padding-top:12px">
            <div style="font-size:.85rem;color:var(--color-primary);font-weight:600;margin-bottom:8px">Bridge 状态</div>
            <div class="status-row"><span class="status-key">连接</span><span class="status-val"><span class="dot" :class="health.bridge.connected ? 'dot-healthy' : 'dot-unhealthy'"></span>{{ health.bridge.connected ? '已连接' : '已断开' }}</span></div>
            <div class="status-row"><span class="status-key">重连</span><span class="status-val">{{ health.bridge.reconnects ?? '--' }}</span></div>
            <div class="status-row"><span class="status-key">消息数</span><span class="status-val">{{ health.bridge.message_count ?? '--' }}</span></div>
          </div>
        </div>
      </div>
      <div class="card">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span><span class="card-title">健康检查详情</span></div>
        <Skeleton v-if="!appState.health || !appState.health.checks" type="text" />
        <div v-else>
          <div v-for="hc in healthCheckList" :key="hc.name" class="status-row">
            <span class="status-key">{{ hc.icon }} {{ hc.name }}</span>
            <span class="status-val" :style="{ color: hc.color }">{{ hc.val }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Tab 3: LLM 代理 -->
    <div v-show="activeTab === 'llm'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44 2.5 2.5 0 0 1-2.96-3.08 3 3 0 0 1-.34-5.58 2.5 2.5 0 0 1 1.32-4.24A2.5 2.5 0 0 1 9.5 2"/><path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44 2.5 2.5 0 0 0 2.96-3.08 3 3 0 0 0 .34-5.58 2.5 2.5 0 0 0-1.32-4.24A2.5 2.5 0 0 0 14.5 2"/></svg></span>
          <span class="card-title">LLM 代理配置</span>
        </div>
        <Skeleton v-if="llmConfigLoading" type="text" />
        <div v-else-if="!llmConfig" style="color:var(--text-tertiary);font-size:var(--text-sm)">
          LLM 代理未启用 · <a href="https://github.com/lobster-guard/docs/llm-proxy" target="_blank" style="color:var(--color-primary)">查看文档</a>
        </div>
        <div v-else>
          <div class="llm-row">
            <span class="llm-label">启用状态</span>
            <span class="llm-val">
              <label class="toggle-switch">
                <input type="checkbox" v-model="llmConfig.enabled" />
                <span class="toggle-slider"></span>
              </label>
              <span style="margin-left:8px;font-size:var(--text-xs);color:var(--text-tertiary)">{{ llmConfig.enabled ? '已启用' : '未启用' }} · 修改需重启</span>
            </span>
          </div>
          <div class="llm-row"><span class="llm-label">监听端口</span><input type="text" v-model="llmConfig.listen" class="llm-input-sm" style="width:140px;font-family:var(--font-mono)" placeholder=":8445" /> <span class="llm-hint">修改需重启</span></div>

          <div id="section-security" class="llm-section-title">安全策略</div>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.security.scan_pii_in_response" /> 扫描响应中的 PII</label>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.security.prompt_injection_scan" /> 扫描请求中的 Prompt Injection</label>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.security.block_high_risk_tools" /> 拦截高危工具调用</label>
          <div class="llm-row" style="margin-top:8px"><span class="llm-label">高危工具</span><input type="text" v-model="highRiskToolsStr" class="llm-input" placeholder="exec, shell, bash" /></div>

          <div id="section-canary" class="llm-section-title">🐤 Canary Token</div>
          <label class="llm-checkbox"><input type="checkbox" v-model="canaryEnabled" /> 启用 Canary Token 注入</label>
          <div class="llm-row" v-if="canaryEnabled"><span class="llm-label">Token</span><span class="llm-val" style="font-family:var(--font-mono);font-size:var(--text-xs)">{{ canaryStatus.token || '(未配置)' }}</span><button class="btn btn-sm" @click="rotateCanary" style="margin-left:8px;font-size:11px;padding:2px 8px">轮换</button></div>
          <div class="llm-row" v-if="canaryEnabled"><span class="llm-label">泄露动作</span><select v-model="canaryAlertAction" class="llm-input-sm" style="width:100px"><option value="log">log</option><option value="warn">warn</option><option value="block">block</option></select></div>
          <label class="llm-checkbox" v-if="canaryEnabled"><input type="checkbox" v-model="canaryAutoRotate" /> 每24小时自动轮换</label>
          <div class="llm-row" v-if="canaryEnabled"><span class="llm-label">最近泄露</span><span class="llm-val" :style="{ color: (canaryStatus.leak_count || 0) > 0 ? 'var(--color-danger)' : 'var(--text-secondary)' }">{{ canaryStatus.leak_count || 0 }} 次</span></div>

          <div id="section-budget" class="llm-section-title"><Icon name="bar-chart" :size="16" /> Response Budget</div>
          <label class="llm-checkbox"><input type="checkbox" v-model="budgetEnabled" /> 启用预算控制</label>
          <div v-if="budgetEnabled">
            <div class="llm-row"><span class="llm-label">最大工具调用</span><input type="number" v-model.number="budgetMaxTools" class="llm-input-sm" min="1" max="100" /> <span class="llm-hint">次/请求</span></div>
            <div class="llm-row"><span class="llm-label">单类工具</span><input type="number" v-model.number="budgetMaxSingle" class="llm-input-sm" min="1" max="50" /> <span class="llm-hint">次/请求</span></div>
            <div class="llm-row"><span class="llm-label">最大 Token</span><input type="number" v-model.number="budgetMaxTokens" class="llm-input-sm" style="width:100px" min="1000" max="10000000" step="10000" /> <span class="llm-hint">Token/请求</span></div>
            <div class="llm-row"><span class="llm-label">超限动作</span><select v-model="budgetAction" class="llm-input-sm" style="width:100px"><option value="warn">warn</option><option value="block">block</option></select></div>
            <div class="llm-row"><span class="llm-label">工具限制</span><input type="text" v-model="budgetToolLimitsStr" class="llm-input" placeholder="exec=3, shell=2" /></div>
            <div class="llm-row"><span class="llm-label">24h超限</span><span class="llm-val" :style="{ color: (budgetStatus.violations_24h || 0) > 0 ? 'var(--color-warning)' : 'var(--text-secondary)' }">{{ budgetStatus.violations_24h || 0 }} 次</span></div>
          </div>

          <div class="llm-section-title">审计配置</div>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.audit.log_tool_input" /> 记录工具调用输入</label>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.audit.log_tool_result" /> 记录工具调用结果</label>
          <label class="llm-checkbox"><input type="checkbox" v-model="llmConfig.audit.log_system_prompt" /> 记录 System Prompt</label>
          <div class="llm-row" style="margin-top:8px"><span class="llm-label">摘要长度</span><input type="number" v-model.number="llmConfig.audit.max_preview_len" class="llm-input-sm" min="100" max="5000" /> <span class="llm-hint">字符</span></div>

          <div id="section-cost" class="llm-section-title">成本预警</div>
          <div class="llm-row"><span class="llm-label">日限额</span><span class="llm-hint" style="margin-right:4px">$</span><input type="number" v-model.number="llmConfig.cost_alert.daily_limit_usd" class="llm-input-sm" min="0" step="5" /> <span class="llm-hint">USD</span></div>
          <div class="llm-row"><span class="llm-label">Webhook</span><input type="text" v-model="llmConfig.cost_alert.webhook_url" class="llm-input" placeholder="https://..." /></div>

          <div class="llm-section-title llm-advanced-toggle" @click="showAdvanced = !showAdvanced" style="cursor:pointer;user-select:none">
            <span>{{ showAdvanced ? '▾' : '▸' }} 高级配置</span>
            <span style="font-size:var(--text-xs);color:var(--text-tertiary);font-weight:400;margin-left:8px">上游目标、超时等</span>
          </div>
          <div v-show="showAdvanced">
            <div class="llm-row"><span class="llm-label">超时</span><input type="number" v-model.number="llmConfig.timeout_sec" class="llm-input-sm" min="5" max="300" /> <span class="llm-hint">秒</span></div>
            <div class="llm-row"><span class="llm-label">请求体限制</span><input type="number" v-model.number="llmConfig.max_body_bytes" class="llm-input-sm" style="width:120px" min="0" step="1048576" /> <span class="llm-hint">字节 (0=不限)</span></div>
            <div style="margin-top:8px;margin-bottom:4px;font-size:var(--text-xs);color:var(--text-tertiary)">上游目标（透明代理转发地址）</div>
            <div v-if="llmConfig.targets && llmConfig.targets.length" class="llm-targets">
              <div v-for="t in llmConfig.targets" :key="t.name" class="llm-target-row">
                <code>{{ t.name }}</code>
                <span style="color:var(--text-tertiary)">{{ t.upstream }}</span>
                <span class="llm-tag">{{ t.api_key_header }}</span>
              </div>
            </div>
            <div v-else style="color:var(--text-tertiary);font-size:var(--text-sm)">无上游目标</div>
          </div>

          <div style="margin-top:var(--space-4);display:flex;align-items:center;gap:var(--space-3)">
            <button class="btn btn-sm" @click="saveLLMConfig" :disabled="llmSaving">{{ llmSaving ? '保存中...' : '保存配置' }}</button>
            <span class="llm-restart-hint">⚠️ 部分变更需重启生效</span>
          </div>
        </div>
      </div>
    </div>

    <ConfirmModal :visible="confirmVisible" :title="confirmTitle" :message="confirmMessage" :type="confirmType" @confirm="doConfirm" @cancel="confirmVisible = false" />
  </div>
</template>

<script setup>
import { ref, computed, inject, onMounted, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { api, apiPost, apiPut, apiDelete, saveToken, clearToken, getToken } from '../api.js'
import { showToast, updateHealth } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import Icon from '../components/Icon.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Skeleton from '../components/Skeleton.vue'

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

// Tab management
const tabs = [
  { key: 'auth', label: '认证与安全', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>' },
  { key: 'system', label: '系统信息', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>' },
  { key: 'llm', label: 'LLM 代理', icon: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44"/><path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44"/></svg>' },
]
const activeTab = ref('auth')

// LLM config
const llmConfigLoading = ref(true)
const llmConfig = ref(null)
const llmSaving = ref(false)
const showAdvanced = ref(false)
const highRiskToolsStr = computed({
  get: () => llmConfig.value?.security?.high_risk_tool_list?.join(', ') || '',
  set: (v) => { if (llmConfig.value?.security) { llmConfig.value.security.high_risk_tool_list = v.split(',').map(s => s.trim()).filter(Boolean) } }
})

// Canary Token
const canaryStatus = ref({ token: '', leak_count: 0, last_leak: '' })
const canaryEnabled = computed({
  get: () => llmConfig.value?.security?.canary_token?.enabled ?? true,
  set: (v) => { if (llmConfig.value?.security?.canary_token) llmConfig.value.security.canary_token.enabled = v }
})
const canaryAlertAction = computed({
  get: () => llmConfig.value?.security?.canary_token?.alert_action || 'warn',
  set: (v) => { if (llmConfig.value?.security?.canary_token) llmConfig.value.security.canary_token.alert_action = v }
})
const canaryAutoRotate = computed({
  get: () => llmConfig.value?.security?.canary_token?.auto_rotate ?? false,
  set: (v) => { if (llmConfig.value?.security?.canary_token) llmConfig.value.security.canary_token.auto_rotate = v }
})

// Response Budget
const budgetStatus = ref({ violations_24h: 0, total_violations: 0 })
const budgetEnabled = computed({
  get: () => llmConfig.value?.security?.response_budget?.enabled ?? false,
  set: (v) => { if (llmConfig.value?.security?.response_budget) llmConfig.value.security.response_budget.enabled = v }
})
const budgetMaxTools = computed({
  get: () => llmConfig.value?.security?.response_budget?.max_tool_calls_per_req || 20,
  set: (v) => { if (llmConfig.value?.security?.response_budget) llmConfig.value.security.response_budget.max_tool_calls_per_req = v }
})
const budgetMaxSingle = computed({
  get: () => llmConfig.value?.security?.response_budget?.max_single_tool_per_req || 5,
  set: (v) => { if (llmConfig.value?.security?.response_budget) llmConfig.value.security.response_budget.max_single_tool_per_req = v }
})
const budgetMaxTokens = computed({
  get: () => llmConfig.value?.security?.response_budget?.max_tokens_per_req || 100000,
  set: (v) => { if (llmConfig.value?.security?.response_budget) llmConfig.value.security.response_budget.max_tokens_per_req = v }
})
const budgetAction = computed({
  get: () => llmConfig.value?.security?.response_budget?.over_budget_action || 'warn',
  set: (v) => { if (llmConfig.value?.security?.response_budget) llmConfig.value.security.response_budget.over_budget_action = v }
})
const budgetToolLimitsStr = computed({
  get: () => {
    const limits = llmConfig.value?.security?.response_budget?.tool_limits
    if (!limits || typeof limits !== 'object') return ''
    return Object.entries(limits).map(([k, v]) => `${k}=${v}`).join(', ')
  },
  set: (v) => {
    if (!llmConfig.value?.security?.response_budget) return
    const limits = {}
    v.split(',').map(s => s.trim()).filter(Boolean).forEach(pair => {
      const [k, val] = pair.split('=').map(s => s.trim())
      if (k && val) limits[k] = parseInt(val) || 5
    })
    llmConfig.value.security.response_budget.tool_limits = limits
  }
})

const backupColumns = [
  { key: 'name', label: '文件名', sortable: true },
  { key: 'size', label: '大小', sortable: true },
  { key: 'mod_time', label: '时间', sortable: true },
]

const C = 2 * Math.PI * 40
const health = computed(() => appState.health || {})
const totalUp = computed(() => health.value.upstreams?.total || 0)
const healthyUp = computed(() => health.value.upstreams?.healthy || 0)
const pct = computed(() => totalUp.value > 0 ? Math.round(healthyUp.value / totalUp.value * 100) : 100)
const ringOffset = computed(() => C - pct.value / 100 * C)
const ringColor = computed(() => pct.value >= 80 ? 'var(--color-success)' : (pct.value >= 50 ? 'var(--color-warning)' : 'var(--color-danger)'))
const statusText = computed(() => { const s = health.value.status; return s === 'healthy' ? '健康' : (s === 'degraded' ? '降级' : '异常') })
const statusColor = computed(() => { const s = health.value.status; return s === 'healthy' ? 'var(--color-success)' : (s === 'degraded' ? 'var(--color-warning)' : 'var(--color-danger)') })
const rlText = computed(() => { const rl = health.value.rate_limiter; if (!rl || !rl.enabled) return '未启用'; return `${rl.global_rps || '?'} rps / ${rl.per_sender_rps || '?'} rps` })

const formattedUptime = computed(() => {
  const raw = health.value.uptime
  if (!raw || raw === '--') return '--'
  let totalSeconds = 0
  const hMatch = raw.match(/([\d.]+)h/); if (hMatch) totalSeconds += parseFloat(hMatch[1]) * 3600
  const mMatch = raw.match(/([\d.]+)m(?!s)/); if (mMatch) totalSeconds += parseFloat(mMatch[1]) * 60
  const sMatch = raw.match(/([\d.]+)s/); if (sMatch) totalSeconds += parseFloat(sMatch[1])
  if (totalSeconds <= 0) return raw
  const minutes = Math.floor(totalSeconds / 60), hours = Math.floor(totalSeconds / 3600), days = Math.floor(totalSeconds / 86400)
  if (minutes < 1) return '< 1 min'
  if (hours < 1) return minutes + ' min'
  if (days < 1) return hours + 'h ' + (minutes % 60) + 'm'
  if (days < 7) return days + 'd ' + (hours % 24) + 'h'
  return days + 'd'
})

const healthCheckList = computed(() => {
  const checks = appState.health?.checks; if (!checks) return []
  const dims = [
    { k: 'database', n: '数据库', fn: c => c.latency_ms != null ? c.latency_ms.toFixed(1) + 'ms' : '' },
    { k: 'upstream', n: '上游服务', fn: c => c.healthy != null ? c.healthy + '/' + c.total : '' },
    { k: 'disk', n: '磁盘空间', fn: c => c.used_percent != null ? c.used_percent.toFixed(1) + '%' : '' },
    { k: 'memory', n: '内存', fn: c => c.alloc_mb != null ? c.alloc_mb.toFixed(1) + ' MB' : '' },
    { k: 'goroutines', n: 'Goroutines', fn: c => c.count != null ? String(c.count) : '' },
  ]
  const result = []
  for (const dm of dims) { const c = checks[dm.k]; if (!c) continue; const color = c.status === 'ok' ? 'var(--color-success)' : (c.status === 'warning' ? 'var(--color-warning)' : 'var(--color-danger)'); const icon = c.status === 'ok' ? '✅' : (c.status === 'warning' ? '⚠️' : '❌'); result.push({ name: dm.n, val: dm.fn(c), color, icon }) }
  return result
})

function fmtTime(ts) { if (!ts) return '--'; const d = new Date(ts); return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false }) }
function formatSize(bytes) { const kb = Math.round((bytes || 0) / 1024); return kb > 1024 ? (kb / 1024).toFixed(1) + ' MB' : kb + ' KB' }
function doSaveToken() { const v = tokenValue.value.trim(); if (v) { saveToken(v); showToast('Token 已保存', 'success') } else showToast('请输入 Token', 'error') }
function doClearToken() { clearToken(); tokenValue.value = ''; showToast('Token 已清除', 'success') }
async function refreshHealth() { try { const d = await api('/healthz'); updateHealth(d) } catch {} }
async function loadBackups() { backupsLoading.value = true; try { const d = await api('/api/v1/backups'); backups.value = d.backups || [] } catch { backups.value = [] }; backupsLoading.value = false }
async function createBackup() { try { await apiPost('/api/v1/backup', {}); showToast('备份创建成功', 'success'); loadBackups() } catch (e) { showToast('备份失败: ' + e.message, 'error') } }
async function seedDemo() {
  demoLoading.value = true; demoResult.value = null
  try { const d = await apiPost('/api/v1/demo/seed', {}); demoResult.value = { ok: true, message: '✅ 成功注入 ' + d.count + ' 条模拟数据' }; showToast('注入了 ' + d.count + ' 条演示数据', 'success') }
  catch (e) { demoResult.value = { ok: false, message: '❌ 注入失败: ' + e.message }; showToast('注入失败: ' + e.message, 'error') }
  demoLoading.value = false
}
async function clearDemo() {
  demoLoading.value = true; demoResult.value = null
  try { const d = await apiDelete('/api/v1/demo/clear'); demoResult.value = { ok: true, message: '✅ 已清除 ' + d.deleted + ' 条数据' }; showToast('清除了 ' + d.deleted + ' 条数据', 'success') }
  catch (e) { demoResult.value = { ok: false, message: '❌ 清除失败: ' + e.message }; showToast('清除失败: ' + e.message, 'error') }
  demoLoading.value = false
}
function confirmDeleteBackup(row) {
  confirmTitle.value = '删除备份'; confirmMessage.value = '确认删除备份 ' + row.name + ' ? 此操作不可恢复。'; confirmType.value = 'danger'
  confirmAction = async () => { try { await api('/api/v1/backups/' + encodeURIComponent(row.name), { method: 'DELETE' }); showToast('备份已删除', 'success'); loadBackups() } catch (e) { showToast('删除失败: ' + e.message, 'error') } }
  confirmVisible.value = true
}
function doConfirm() { confirmVisible.value = false; if (confirmAction) confirmAction() }

async function loadLLMConfig() {
  llmConfigLoading.value = true
  try {
    const d = await api('/api/v1/llm/config')
    if (!d.audit) d.audit = { log_system_prompt: false, log_tool_input: true, log_tool_result: true, max_preview_len: 500 }
    if (!d.cost_alert) d.cost_alert = { daily_limit_usd: 50, webhook_url: '' }
    if (!d.security) d.security = { scan_pii_in_response: true, block_high_risk_tools: false, high_risk_tool_list: ['exec','shell','bash'], prompt_injection_scan: true }
    if (!d.security.canary_token) d.security.canary_token = { enabled: true, auto_rotate: false, alert_action: 'warn' }
    if (!d.security.response_budget) d.security.response_budget = { enabled: false, max_tool_calls_per_req: 20, max_single_tool_per_req: 5, max_tokens_per_req: 100000, over_budget_action: 'warn', tool_limits: {} }
    llmConfig.value = d; loadCanaryStatus(); loadBudgetStatus()
  } catch { llmConfig.value = null }
  llmConfigLoading.value = false
}
async function loadCanaryStatus() { try { const d = await api('/api/v1/llm/canary/status'); canaryStatus.value = d } catch {} }
async function loadBudgetStatus() { try { const d = await api('/api/v1/llm/budget/status'); budgetStatus.value = d } catch {} }
async function rotateCanary() { try { const d = await apiPost('/api/v1/llm/canary/rotate', {}); showToast('Canary Token 已轮换', 'success'); canaryStatus.value.token = d.token; loadCanaryStatus() } catch (e) { showToast('轮换失败: ' + e.message, 'error') } }
async function saveLLMConfig() {
  if (!llmConfig.value) return; llmSaving.value = true
  try { const d = await apiPut('/api/v1/llm/config', llmConfig.value); showToast(d.need_restart ? '配置已保存（部分变更需重启生效）' : 'LLM 配置已保存', 'success') }
  catch (e) { showToast('保存失败: ' + e.message, 'error') }
  llmSaving.value = false
}

function scrollToSection(section) {
  if (!section) return
  if (['canary', 'budget', 'security', 'cost'].includes(section)) activeTab.value = 'llm'
  nextTick(() => {
    const el = document.getElementById('section-' + section)
    if (el) { el.scrollIntoView({ behavior: 'smooth', block: 'start' }); el.classList.add('section-highlight'); setTimeout(() => el.classList.remove('section-highlight'), 2000) }
  })
}

onMounted(() => {
  loadBackups(); loadLLMConfig()
  const section = route.query.section
  if (section) { const poll = () => { if (!llmConfigLoading.value) scrollToSection(section); else setTimeout(poll, 100) }; poll() }
})
</script>

<style scoped>
.settings-tabs { display: flex; gap: 0; margin-bottom: 20px; border-bottom: 2px solid var(--border-subtle); }
.settings-tab { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-3) var(--space-4); background: none; border: none; cursor: pointer; color: var(--text-secondary); font-size: var(--text-sm); font-weight: 500; border-bottom: 2px solid transparent; margin-bottom: -2px; transition: all var(--transition-fast); }
.settings-tab:hover { color: var(--text-primary); background: var(--bg-hover); }
.settings-tab.active { color: var(--color-primary); border-bottom-color: var(--color-primary); }
.tab-icon { display: flex; align-items: center; }
.tab-label { white-space: nowrap; }
@keyframes section-flash { 0%, 100% { background: transparent; } 50% { background: rgba(99, 102, 241, 0.1); } }
.section-highlight { animation: section-flash 0.5s ease 2; border-radius: var(--radius-sm); padding: 2px 4px; margin: -2px -4px; }
.settings-section { margin-bottom: var(--space-4); }
.settings-label { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); color: var(--text-secondary); font-weight: 500; margin-bottom: var(--space-2); }
.token-input-wrap { position: relative; display: inline-flex; align-items: center; width: 320px; max-width: 100%; }
.token-input { width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-2) 40px var(--space-2) var(--space-3); font-size: var(--text-sm); outline: none; font-family: var(--font-mono); transition: border-color var(--transition-fast); }
.token-input:focus { border-color: var(--color-primary); box-shadow: 0 0 0 3px var(--color-primary-dim); }
.token-toggle { position: absolute; right: 8px; top: 50%; transform: translateY(-50%); background: none; border: none; color: var(--text-tertiary); cursor: pointer; padding: 4px; border-radius: var(--radius-sm); display: flex; align-items: center; transition: all var(--transition-fast); }
.token-toggle:hover { color: var(--text-primary); background: var(--bg-hover); }
.token-actions { display: flex; gap: var(--space-2); margin-top: var(--space-3); }
.llm-row { display: flex; align-items: center; gap: var(--space-2); margin-bottom: 6px; font-size: var(--text-sm); }
.llm-label { color: var(--text-secondary); min-width: 80px; flex-shrink: 0; }
.llm-val { color: var(--text-primary); display: flex; align-items: center; gap: var(--space-1); }
.llm-section-title { font-size: var(--text-xs); font-weight: 600; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text-tertiary); margin: 16px 0 8px; padding-top: 12px; border-top: 1px solid var(--border-subtle); }
.llm-targets { display: flex; flex-direction: column; gap: 4px; }
.llm-target-row { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); padding: 4px 8px; background: var(--bg-elevated); border-radius: var(--radius-sm); }
.llm-target-row code { background: var(--bg-base); padding: 1px 6px; border-radius: 3px; font-size: var(--text-xs); font-family: var(--font-mono); color: var(--color-primary); }
.llm-tag { font-size: 10px; padding: 1px 6px; border-radius: 9999px; background: rgba(99,102,241,0.12); color: var(--color-primary); }
.llm-checkbox { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); color: var(--text-primary); cursor: pointer; margin-bottom: 4px; }
.llm-checkbox input[type="checkbox"] { accent-color: var(--color-primary); width: 16px; height: 16px; cursor: pointer; }
.llm-input-sm { width: 80px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 4px 8px; font-size: var(--text-sm); outline: none; font-family: var(--font-mono); }
.llm-input-sm:focus { border-color: var(--color-primary); }
.llm-input { flex: 1; max-width: 320px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); padding: 4px 8px; font-size: var(--text-sm); outline: none; }
.llm-input:focus { border-color: var(--color-primary); }
.llm-hint { font-size: var(--text-xs); color: var(--text-tertiary); }
.llm-restart-hint { font-size: var(--text-xs); color: var(--color-warning); }
.toggle-switch { position: relative; display: inline-block; width: 36px; height: 20px; cursor: pointer; }
.toggle-switch input { opacity: 0; width: 0; height: 0; }
.toggle-slider { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: 20px; transition: .3s; }
.toggle-slider:before { content: ''; position: absolute; height: 14px; width: 14px; left: 2px; bottom: 2px; background: var(--text-tertiary); border-radius: 50%; transition: .3s; }
.toggle-switch input:checked + .toggle-slider { background: var(--color-primary); border-color: var(--color-primary); }
.toggle-switch input:checked + .toggle-slider:before { transform: translateX(16px); background: #fff; }
.llm-advanced-toggle { display: flex; align-items: center; }
</style>
