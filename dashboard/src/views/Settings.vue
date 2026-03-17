<template>
  <div>
    <!-- Auth Settings -->
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
          <input
            :type="showToken ? 'text' : 'password'"
            v-model="tokenValue"
            placeholder="输入 Bearer Token"
            class="token-input"
          />
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

    <!-- System Info -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg></span><span class="card-title">系统信息</span>
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

    <!-- Demo Data -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg></span>
        <span class="card-title">演示数据</span>
      </div>
      <div style="font-size:var(--text-sm);color:var(--text-secondary);margin-bottom:var(--space-3)">
        注入模拟审计数据用于演示 Dashboard 图表效果。数据包含 250-300 条过去 7 天的模拟记录。
      </div>
      <div v-if="demoResult" style="margin-bottom:var(--space-3);padding:var(--space-2) var(--space-3);border-radius:var(--radius-md);font-size:var(--text-sm);background:var(--bg-elevated)">
        <span :style="{ color: demoResult.ok ? 'var(--color-success)' : 'var(--color-danger)' }">{{ demoResult.message }}</span>
      </div>
      <div style="display:flex;gap:var(--space-2)">
        <button class="btn btn-sm" @click="seedDemo" :disabled="demoLoading">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/></svg>
          {{ demoLoading ? '注入中...' : '注入演示数据' }}
        </button>
        <button class="btn btn-danger btn-sm" @click="clearDemo" :disabled="demoLoading">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
          清除演示数据
        </button>
      </div>
    </div>

    <!-- Backups -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg></span><span class="card-title">备份管理</span>
        <div class="card-actions">
          <button class="btn btn-sm" @click="createBackup">创建备份</button>
          <button class="btn btn-ghost btn-sm" @click="loadBackups">刷新</button>
        </div>
      </div>
      <Skeleton v-if="backupsLoading" type="table" />
      <div v-else-if="!backups.length" class="empty"><div class="empty-icon">💾</div>暂无备份<div class="empty-hint">点击"创建备份"开始</div></div>
      <DataTable v-else :columns="backupColumns" :data="backups" :show-toolbar="false">
        <template #cell-name="{ value }"><span style="font-family:monospace;font-size:.8rem">{{ value }}</span></template>
        <template #cell-size="{ value }">{{ formatSize(value) }}</template>
        <template #cell-mod_time="{ value }">{{ fmtTime(value) }}</template>
        <template #actions="{ row }">
          <button class="btn btn-danger btn-sm" @click="confirmDeleteBackup(row)">删除</button>
        </template>
      </DataTable>
    </div>

    <!-- Health Checks -->
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

    <ConfirmModal :visible="confirmVisible" :title="confirmTitle" :message="confirmMessage" :type="confirmType" @confirm="doConfirm" @cancel="confirmVisible = false" />
  </div>
</template>

<script setup>
import { ref, computed, inject, onMounted } from 'vue'
import { api, apiPost, apiDelete, saveToken, clearToken, getToken } from '../api.js'
import { showToast, updateHealth } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Skeleton from '../components/Skeleton.vue'

const appState = inject('appState')
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

// Format uptime like Topbar
const formattedUptime = computed(() => {
  const raw = health.value.uptime
  if (!raw || raw === '--') return '--'
  return formatUptime(raw)
})

function formatUptime(raw) {
  let totalSeconds = 0
  const hMatch = raw.match(/([\d.]+)h/)
  if (hMatch) totalSeconds += parseFloat(hMatch[1]) * 3600
  const mMatch = raw.match(/([\d.]+)m(?!s)/)
  if (mMatch) totalSeconds += parseFloat(mMatch[1]) * 60
  const sMatch = raw.match(/([\d.]+)s/)
  if (sMatch) totalSeconds += parseFloat(sMatch[1])
  if (totalSeconds <= 0) return raw
  const minutes = Math.floor(totalSeconds / 60)
  const hours = Math.floor(totalSeconds / 3600)
  const days = Math.floor(totalSeconds / 86400)
  if (minutes < 1) return '< 1 min'
  if (hours < 1) return minutes + ' min'
  if (days < 1) return hours + 'h ' + (minutes % 60) + 'm'
  if (days < 7) return days + 'd ' + (hours % 24) + 'h'
  return days + 'd'
}

const healthCheckList = computed(() => {
  const checks = appState.health?.checks
  if (!checks) return []
  const dims = [
    { k: 'database', n: '数据库', fn: c => c.latency_ms != null ? c.latency_ms.toFixed(1) + 'ms' : '' },
    { k: 'upstream', n: '上游服务', fn: c => c.healthy != null ? c.healthy + '/' + c.total : '' },
    { k: 'disk', n: '磁盘空间', fn: c => c.used_percent != null ? c.used_percent.toFixed(1) + '%' : '' },
    { k: 'memory', n: '内存', fn: c => c.alloc_mb != null ? c.alloc_mb.toFixed(1) + ' MB' : '' },
    { k: 'goroutines', n: 'Goroutines', fn: c => c.count != null ? String(c.count) : '' },
  ]
  const result = []
  for (const dm of dims) {
    const c = checks[dm.k]
    if (!c) continue
    const color = c.status === 'ok' ? 'var(--color-success)' : (c.status === 'warning' ? 'var(--color-warning)' : 'var(--color-danger)')
    const icon = c.status === 'ok' ? '✅' : (c.status === 'warning' ? '⚠️' : '❌')
    result.push({ name: dm.n, val: dm.fn(c), color, icon })
  }
  return result
})

function fmtTime(ts) { if (!ts) return '--'; const d = new Date(ts); return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false }) }
function formatSize(bytes) { const kb = Math.round((bytes || 0) / 1024); return kb > 1024 ? (kb / 1024).toFixed(1) + ' MB' : kb + ' KB' }

function doSaveToken() {
  const v = tokenValue.value.trim()
  if (v) { saveToken(v); showToast('Token 已保存', 'success') }
  else showToast('请输入 Token', 'error')
}
function doClearToken() { clearToken(); tokenValue.value = ''; showToast('Token 已清除', 'success') }

async function refreshHealth() {
  try { const d = await api('/healthz'); updateHealth(d) } catch {}
}

async function loadBackups() {
  backupsLoading.value = true
  try { const d = await api('/api/v1/backups'); backups.value = d.backups || [] } catch { backups.value = [] }
  backupsLoading.value = false
}

async function createBackup() {
  try { await apiPost('/api/v1/backup', {}); showToast('备份创建成功', 'success'); loadBackups() } catch (e) { showToast('备份失败: ' + e.message, 'error') }
}

// Demo data management
async function seedDemo() {
  demoLoading.value = true
  demoResult.value = null
  try {
    const d = await apiPost('/api/v1/demo/seed', {})
    demoResult.value = { ok: true, message: `✅ 成功注入 ${d.count} 条模拟数据` }
    showToast(`注入了 ${d.count} 条演示数据`, 'success')
  } catch (e) {
    demoResult.value = { ok: false, message: `❌ 注入失败: ${e.message}` }
    showToast('注入失败: ' + e.message, 'error')
  }
  demoLoading.value = false
}

async function clearDemo() {
  demoLoading.value = true
  demoResult.value = null
  try {
    const d = await apiDelete('/api/v1/demo/clear')
    demoResult.value = { ok: true, message: `✅ 已清除 ${d.deleted} 条数据` }
    showToast(`清除了 ${d.deleted} 条数据`, 'success')
  } catch (e) {
    demoResult.value = { ok: false, message: `❌ 清除失败: ${e.message}` }
    showToast('清除失败: ' + e.message, 'error')
  }
  demoLoading.value = false
}

function confirmDeleteBackup(row) {
  confirmTitle.value = '删除备份'
  confirmMessage.value = `确认删除备份 ${row.name} ? 此操作不可恢复。`
  confirmType.value = 'danger'
  confirmAction = async () => {
    try { await api('/api/v1/backups/' + encodeURIComponent(row.name), { method: 'DELETE' }); showToast('备份已删除', 'success'); loadBackups() } catch (e) { showToast('删除失败: ' + e.message, 'error') }
  }
  confirmVisible.value = true
}

function doConfirm() { confirmVisible.value = false; if (confirmAction) confirmAction() }

onMounted(() => { loadBackups() })
</script>

<style scoped>
.settings-section { margin-bottom: var(--space-4); }
.settings-label {
  display: flex; align-items: center; gap: var(--space-2);
  font-size: var(--text-sm); color: var(--text-secondary); font-weight: 500;
  margin-bottom: var(--space-2);
}
.token-input-wrap {
  position: relative; display: inline-flex; align-items: center;
  width: 320px; max-width: 100%;
}
.token-input {
  width: 100%;
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary);
  padding: var(--space-2) 40px var(--space-2) var(--space-3);
  font-size: var(--text-sm); outline: none; font-family: var(--font-mono);
  transition: border-color var(--transition-fast);
}
.token-input:focus { border-color: var(--color-primary); box-shadow: 0 0 0 3px var(--color-primary-dim); }
.token-toggle {
  position: absolute; right: 8px; top: 50%; transform: translateY(-50%);
  background: none; border: none; color: var(--text-tertiary); cursor: pointer;
  padding: 4px; border-radius: var(--radius-sm); display: flex; align-items: center;
  transition: all var(--transition-fast);
}
.token-toggle:hover { color: var(--text-primary); background: var(--bg-hover); }
.token-actions {
  display: flex; gap: var(--space-2); margin-top: var(--space-3);
}
</style>
