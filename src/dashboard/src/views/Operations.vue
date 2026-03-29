<template>
  <div>
    <div class="tab-header" style="margin-bottom:20px">
      <button class="tab-btn" :class="{ active: tab === 'config' }" @click="tab = 'config'"><Icon name="file-text" :size="14" /> 配置</button>
      <button class="tab-btn" :class="{ active: tab === 'backup' }" @click="tab = 'backup'"><Icon name="save" :size="14" /> 备份</button>
      <button class="tab-btn" :class="{ active: tab === 'diag' }" @click="tab = 'diag'"><Icon name="search" :size="14" /> 诊断</button>
      <button class="tab-btn" :class="{ active: tab === 'alert' }" @click="tab = 'alert'"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg> 告警</button>
    </div>

    <!-- ===== 配置 Tab ===== -->
    <div v-if="tab === 'config'">
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg></span>
          <span class="card-title">运行配置</span>
          <div class="card-actions">
            <span v-if="configData.path" style="font-size:var(--text-xs);color:var(--text-tertiary);font-family:var(--font-mono)">{{ configData.path }}</span>
            <button class="btn btn-ghost btn-sm" @click="copyConfig" title="复制配置内容">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
              复制
            </button>
            <button class="btn btn-ghost btn-sm" @click="loadConfig">刷新</button>
          </div>
        </div>
        <Skeleton v-if="configLoading" type="text" />
        <div v-else-if="configData.content" class="config-viewer">
          <pre class="config-pre" v-html="highlightYaml(configData.content)"></pre>
        </div>
        <div v-else class="empty"><div class="empty-icon">📄</div>无法加载配置</div>
      </div>
    </div>

    <!-- ===== 备份 Tab ===== -->
    <div v-if="tab === 'backup'">
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/><polyline points="17 21 17 13 7 13 7 21"/><polyline points="7 3 7 8 15 8"/></svg></span>
          <span class="card-title">备份管理</span>
          <div class="card-actions">
            <button class="btn btn-sm" @click="createBackup" :disabled="backupCreating">
              {{ backupCreating ? '创建中...' : '创建备份' }}
            </button>
            <button class="btn btn-ghost btn-sm" @click="loadBackups">刷新</button>
          </div>
        </div>
        <Skeleton v-if="backupsLoading" type="table" />
        <div v-else-if="!backups.length" class="empty"><div class="empty-icon"><Icon name="save" :size="48" color="var(--text-quaternary)" /></div>暂无备份<div class="empty-hint">点击"创建备份"开始</div></div>
        <DataTable v-else :columns="backupColumns" :data="backups" :show-toolbar="false">
          <template #cell-name="{ value }"><span style="font-family:var(--font-mono);font-size:var(--text-xs)">{{ value }}</span></template>
          <template #cell-size="{ value }">{{ formatSize(value) }}</template>
          <template #cell-created_at="{ value }">{{ fmtTime(value) }}</template>
          <template #actions="{ row }">
            <button class="btn btn-ghost btn-sm" @click="downloadBackup(row)" title="下载">下载</button>
            <button class="btn btn-purple btn-sm" @click="confirmRestore(row)" title="恢复">恢复</button>
            <button class="btn btn-danger btn-sm" @click="confirmDeleteBackup(row)" title="删除">删除</button>
          </template>
        </DataTable>
      </div>
    </div>

    <!-- ===== 诊断 Tab ===== -->
    <div v-if="tab === 'diag'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span>
          <span class="card-title">上游连通性</span>
          <div class="card-actions"><button class="btn btn-ghost btn-sm" @click="loadDiag">刷新</button></div>
        </div>
        <Skeleton v-if="diagLoading" type="text" />
        <div v-else-if="diagData.upstreams && diagData.upstreams.length">
          <div v-for="up in diagData.upstreams" :key="up.id" class="diag-upstream-row">
            <span class="dot" :class="up.healthy ? 'dot-healthy' : 'dot-unhealthy'"></span>
            <span class="diag-up-id">{{ up.id }}</span>
            <span class="diag-up-addr">{{ up.address }}</span>
            <span class="diag-up-latency">{{ up.latency_ms > 0 ? up.latency_ms.toFixed(1) + ' ms' : '--' }}</span>
            <span class="tag" :class="up.healthy ? 'tag-pass' : 'tag-block'">{{ up.healthy ? '健康' : '异常' }}</span>
          </div>
        </div>
        <div v-else class="empty" style="padding:var(--space-4)"><div class="empty-icon">🔗</div>无上游节点</div>
      </div>

      <div style="display:grid;grid-template-columns:1fr 1fr;gap:20px">
        <div class="card">
          <div class="card-header">
            <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg></span>
            <span class="card-title">规则统计</span>
          </div>
          <Skeleton v-if="diagLoading" type="text" />
          <div v-else-if="diagData.rules" class="stat-big">
            <div class="stat-item"><div class="stat-num blue">{{ diagData.rules.inbound_total || 0 }}</div><div class="stat-label">入站规则</div></div>
            <div class="stat-item"><div class="stat-num green">{{ diagData.rules.outbound_total || 0 }}</div><div class="stat-label">出站规则</div></div>
            <div class="stat-item"><div class="stat-num yellow">{{ diagData.rules.inbound_keyword || 0 }}</div><div class="stat-label">关键词规则</div></div>
            <div class="stat-item"><div class="stat-num red">{{ diagData.rules.inbound_regex || 0 }}</div><div class="stat-label">正则规则</div></div>
          </div>
        </div>

        <div class="card">
          <div class="card-header">
            <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/></svg></span>
            <span class="card-title">数据库</span>
          </div>
          <Skeleton v-if="diagLoading" type="text" />
          <div v-else-if="diagData.database">
            <div class="status-row"><span class="status-key">路径</span><span class="status-val" style="font-size:var(--text-xs)">{{ diagData.database.path }}</span></div>
            <div class="status-row"><span class="status-key">数据库大小</span><span class="status-val">{{ diagData.database.size_human || '--' }}</span></div>
            <div class="status-row" v-if="diagData.database.wal_size_bytes != null"><span class="status-key">WAL 大小</span><span class="status-val">{{ formatSize(diagData.database.wal_size_bytes) }}</span></div>
            <div class="status-row"><span class="status-key">版本</span><span class="status-val">{{ diagData.version || '--' }}</span></div>
            <div class="status-row"><span class="status-key">运行时间</span><span class="status-val">{{ diagData.uptime || '--' }}</span></div>
          </div>
        </div>
      </div>
    </div>

    <!-- ===== 告警 Tab ===== -->
    <div v-if="tab === 'alert'">
      <div class="card" style="margin-bottom:20px">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg></span>
          <span class="card-title">告警配置</span>
          <div class="card-actions"><button class="btn btn-ghost btn-sm" @click="loadAlerts">刷新</button></div>
        </div>
        <Skeleton v-if="alertConfigLoading" type="text" />
        <div v-else-if="alertConfig">
          <div class="status-row"><span class="status-key">Webhook</span><span class="status-val">{{ alertConfig.webhook_configured ? (alertConfig.webhook_url || '已配置') : '未配置' }}</span></div>
          <div class="status-row"><span class="status-key">格式</span><span class="status-val">{{ alertConfig.format || 'generic' }}</span></div>
          <div class="status-row"><span class="status-key">最小间隔</span><span class="status-val">{{ alertConfig.min_interval_sec || 60 }}s</span></div>
          <div class="status-row"><span class="status-key">已发送告警</span><span class="status-val" style="color:var(--color-warning)">{{ alertConfig.total_alerts_sent || 0 }}</span></div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg></span>
          <span class="card-title">最近告警 (Block 事件)</span>
        </div>
        <Skeleton v-if="alertHistoryLoading" type="table" />
        <div v-else-if="!alertHistory.length" class="empty"><div class="empty-icon">🔔</div>暂无告警记录<div class="empty-hint">系统一切正常</div></div>
        <DataTable v-else :columns="alertColumns" :data="alertHistory" :show-toolbar="false">
          <template #cell-timestamp="{ value }">{{ fmtTime(value) }}</template>
          <template #cell-direction="{ value }"><span class="tag tag-info">{{ value === 'inbound' ? '入站' : '出站' }}</span></template>
          <template #cell-reason="{ value }"><span style="color:var(--color-danger);font-size:var(--text-xs)">{{ value }}</span></template>
          <template #cell-sender_id="{ value }"><span style="font-family:var(--font-mono);font-size:var(--text-xs)">{{ value }}</span></template>
        </DataTable>
      </div>
    </div>

    <ConfirmModal :visible="confirmVisible" :title="confirmTitle" :message="confirmMessage" :type="confirmType" @confirm="doConfirm" @cancel="confirmVisible = false" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api, apiPost, downloadFile } from '../api.js'
import { showToast } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Icon from '../components/Icon.vue'
import Skeleton from '../components/Skeleton.vue'

const tab = ref('config')

// ===== Config =====
const configLoading = ref(false)
const configData = ref({})

async function loadConfig() {
  configLoading.value = true
  try { configData.value = await api('/api/v1/config/view') } catch { configData.value = {} }
  configLoading.value = false
}

function copyConfig() {
  if (!configData.value.content) return
  navigator.clipboard.writeText(configData.value.content)
    .then(() => showToast('已复制到剪贴板', 'success'))
    .catch(() => showToast('复制失败', 'error'))
}

function highlightYaml(text) {
  if (!text) return ''
  return text.split('\n').map(line => {
    // comments
    if (line.trimStart().startsWith('#')) {
      return `<span style="color:var(--text-tertiary);font-style:italic">${esc(line)}</span>`
    }
    // key: value
    const m = line.match(/^(\s*)([\w._-]+)(\s*:\s*)(.*)$/)
    if (m) {
      return `${esc(m[1])}<span style="color:var(--color-primary)">${esc(m[2])}</span>${esc(m[3])}<span style="color:var(--text-primary)">${esc(m[4])}</span>`
    }
    return esc(line)
  }).join('\n')
}

function esc(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

// ===== Backup =====
const backups = ref([])
const backupsLoading = ref(false)
const backupCreating = ref(false)

const backupColumns = [
  { key: 'name', label: '文件名', sortable: true },
  { key: 'size', label: '大小', sortable: true },
  { key: 'created_at', label: '创建时间', sortable: true },
]

async function loadBackups() {
  backupsLoading.value = true
  try { const d = await api('/api/v1/backups'); backups.value = d.backups || [] } catch { backups.value = [] }
  backupsLoading.value = false
}

async function createBackup() {
  backupCreating.value = true
  try {
    await apiPost('/api/v1/backup', {})
    showToast('备份创建成功', 'success')
    loadBackups()
  } catch (e) { showToast('备份失败: ' + e.message, 'error') }
  backupCreating.value = false
}

function downloadBackup(row) {
  downloadFile(location.origin + '/api/v1/backups/' + encodeURIComponent(row.name) + '/download', row.name)
}

// ===== Diag =====
const diagLoading = ref(false)
const diagData = ref({})

async function loadDiag() {
  diagLoading.value = true
  try { diagData.value = await api('/api/v1/system/diag') } catch { diagData.value = {} }
  diagLoading.value = false
}

// ===== Alert =====
const alertConfigLoading = ref(false)
const alertConfig = ref(null)
const alertHistoryLoading = ref(false)
const alertHistory = ref([])

const alertColumns = [
  { key: 'timestamp', label: '时间', sortable: true },
  { key: 'direction', label: '方向' },
  { key: 'sender_id', label: '发送者' },
  { key: 'reason', label: '原因' },
  { key: 'app_id', label: 'Bot' },
]

async function loadAlerts() {
  alertConfigLoading.value = true
  alertHistoryLoading.value = true
  try { alertConfig.value = await api('/api/v1/alerts/config') } catch { alertConfig.value = { webhook_configured: false } }
  alertConfigLoading.value = false
  try { const d = await api('/api/v1/alerts/history'); alertHistory.value = d.alerts || [] } catch { alertHistory.value = [] }
  alertHistoryLoading.value = false
}

// ===== Confirm =====
const confirmVisible = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmType = ref('danger')
let confirmAction = null

function confirmDeleteBackup(row) {
  confirmTitle.value = '删除备份'
  confirmMessage.value = `确认删除备份 ${row.name}？此操作不可恢复。`
  confirmType.value = 'danger'
  confirmAction = async () => {
    try {
      await api('/api/v1/backups/' + encodeURIComponent(row.name), { method: 'DELETE' })
      showToast('备份已删除', 'success')
      loadBackups()
    } catch (e) { showToast('删除失败: ' + e.message, 'error') }
  }
  confirmVisible.value = true
}

function confirmRestore(row) {
  confirmTitle.value = '恢复备份'
  confirmMessage.value = `确认从备份 ${row.name} 恢复数据库？当前数据将被覆盖，请确保已创建最新备份。`
  confirmType.value = 'danger'
  confirmAction = async () => {
    try {
      await apiPost('/api/v1/backups/' + encodeURIComponent(row.name) + '/restore', {})
      showToast('恢复成功，建议重启服务', 'success')
    } catch (e) { showToast('恢复失败: ' + e.message, 'error') }
  }
  confirmVisible.value = true
}

function doConfirm() { confirmVisible.value = false; if (confirmAction) confirmAction() }

// ===== Util =====
function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false })
}

function formatSize(bytes) {
  const kb = Math.round((bytes || 0) / 1024)
  return kb > 1024 ? (kb / 1024).toFixed(1) + ' MB' : kb + ' KB'
}

// ===== Init =====
onMounted(() => {
  loadConfig()
  loadBackups()
  loadDiag()
  loadAlerts()
})
</script>

<style scoped>
.config-viewer {
  background: var(--bg-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: auto;
  max-height: 600px;
}
.config-pre {
  margin: 0;
  padding: var(--space-4);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  line-height: 1.7;
  white-space: pre;
  color: var(--text-primary);
}
.diag-upstream-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--border-subtle);
  font-size: var(--text-sm);
}
.diag-upstream-row:last-child { border-bottom: none; }
.diag-up-id { font-weight: 600; color: var(--text-primary); min-width: 120px; }
.diag-up-addr { color: var(--text-secondary); font-family: var(--font-mono); font-size: var(--text-xs); flex: 1; }
.diag-up-latency { color: var(--color-success); font-family: var(--font-mono); font-size: var(--text-xs); min-width: 60px; text-align: right; }
</style>
