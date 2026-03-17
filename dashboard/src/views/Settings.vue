<template>
  <div>
    <!-- Token -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header"><span class="card-icon">🔑</span><span class="card-title">Token 管理</span></div>
      <div class="token-section">
        <label style="font-size:.85rem;color:var(--text-dim)">🔑 Bearer Token:</label>
        <input type="password" v-model="tokenValue" placeholder="输入 Bearer Token" />
        <button class="btn" @click="doSaveToken">保存</button>
        <button class="btn btn-red" @click="doClearToken">清除</button>
      </div>
    </div>

    <!-- System Info -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon">📊</span><span class="card-title">系统信息</span>
        <div class="card-actions"><button class="btn btn-sm" @click="refreshHealth">🔄 刷新</button></div>
      </div>
      <div v-if="!appState.health" class="loading">加载中...</div>
      <div v-else>
        <div class="status-grid">
          <!-- Ring chart -->
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
            <div class="status-row"><span class="status-key">运行时间</span><span class="status-val">{{ health.uptime || '--' }}</span></div>
            <div class="status-row"><span class="status-key">模式</span><span class="status-val">{{ health.mode || '--' }}</span></div>
            <div class="status-row"><span class="status-key">上游</span><span class="status-val">{{ healthyUp }}/{{ totalUp }}</span></div>
            <div class="status-row"><span class="status-key">路由数</span><span class="status-val">{{ health.routes?.total || 0 }}</span></div>
            <div class="status-row"><span class="status-key">审计日志</span><span class="status-val">{{ health.audit?.total || 0 }}</span></div>
            <div class="status-row"><span class="status-key">限流</span><span class="status-val">{{ rlText }}</span></div>
          </div>
        </div>

        <!-- Bridge -->
        <div v-if="health.mode === 'bridge' && health.bridge" style="margin-top:16px;border-top:1px solid rgba(0,212,255,.08);padding-top:12px">
          <div style="font-size:.85rem;color:var(--neon-blue);font-weight:600;margin-bottom:8px">🌉 Bridge 状态</div>
          <div class="status-row"><span class="status-key">连接</span><span class="status-val"><span class="dot" :class="health.bridge.connected ? 'dot-healthy' : 'dot-unhealthy'"></span>{{ health.bridge.connected ? '已连接' : '已断开' }}</span></div>
          <div class="status-row"><span class="status-key">重连</span><span class="status-val">{{ health.bridge.reconnects ?? '--' }}</span></div>
          <div class="status-row"><span class="status-key">消息数</span><span class="status-val">{{ health.bridge.message_count ?? '--' }}</span></div>
        </div>
      </div>
    </div>

    <!-- Backups -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon">💾</span><span class="card-title">备份管理</span>
        <div class="card-actions">
          <button class="btn btn-sm btn-green" @click="createBackup">创建备份</button>
          <button class="btn btn-sm" @click="loadBackups">🔄 刷新</button>
        </div>
      </div>
      <div v-if="backupsLoading" class="loading">加载中...</div>
      <div v-else-if="!backups.length" class="empty"><div class="empty-icon">💾</div>暂无备份<div class="empty-hint">点击"创建备份"开始</div></div>
      <DataTable v-else :columns="backupColumns" :data="backups" :show-toolbar="false">
        <template #cell-name="{ value }"><span style="font-family:monospace;font-size:.8rem">{{ value }}</span></template>
        <template #cell-size="{ value }">{{ formatSize(value) }}</template>
        <template #cell-mod_time="{ value }">{{ fmtTime(value) }}</template>
        <template #actions="{ row }">
          <button class="btn btn-sm btn-red" @click="confirmDeleteBackup(row)">删除</button>
        </template>
      </DataTable>
    </div>

    <!-- Health Checks -->
    <div class="card">
      <div class="card-header"><span class="card-icon">🏥</span><span class="card-title">健康检查详情</span></div>
      <div v-if="!appState.health || !appState.health.checks" class="loading">加载中...</div>
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
import { api, apiPost, saveToken, clearToken, getToken } from '../api.js'
import { showToast, updateHealth } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const appState = inject('appState')
const tokenValue = ref(getToken())

const backups = ref([])
const backupsLoading = ref(false)

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
const ringColor = computed(() => pct.value >= 80 ? 'var(--neon-green)' : (pct.value >= 50 ? 'var(--neon-yellow)' : 'var(--neon-red)'))
const statusText = computed(() => { const s = health.value.status; return s === 'healthy' ? '健康' : (s === 'degraded' ? '降级' : '异常') })
const statusColor = computed(() => { const s = health.value.status; return s === 'healthy' ? 'var(--neon-green)' : (s === 'degraded' ? 'var(--neon-yellow)' : 'var(--neon-red)') })
const rlText = computed(() => { const rl = health.value.rate_limiter; if (!rl || !rl.enabled) return '未启用'; return `${rl.global_rps || '?'} rps (全局) / ${rl.per_sender_rps || '?'} rps (每用户)` })

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
    const color = c.status === 'ok' ? 'var(--neon-green)' : (c.status === 'warning' ? 'var(--neon-yellow)' : 'var(--neon-red)')
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
  try { const d = await api('/healthz'); updateHealth(d) } catch { /* ignore */ }
}

async function loadBackups() {
  backupsLoading.value = true
  try { const d = await api('/api/v1/backups'); backups.value = d.backups || [] } catch { backups.value = [] }
  backupsLoading.value = false
}

async function createBackup() {
  try { await apiPost('/api/v1/backup', {}); showToast('备份创建成功', 'success'); loadBackups() } catch (e) { showToast('备份失败: ' + e.message, 'error') }
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
