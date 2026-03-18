<template>
  <div class="card">
    <div class="card-header">
      <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg></span>
      <span class="card-title">审计日志</span>
      <div class="card-actions">
        <button class="btn btn-ghost btn-sm" @click="loadLogs">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
          刷新
        </button>
      </div>
    </div>

    <!-- Timeline trend chart -->
    <div style="margin-bottom:var(--space-4)">
      <div style="font-size:var(--text-sm);color:var(--text-secondary);margin-bottom:var(--space-2);font-weight:500;display:flex;align-items:center;gap:6px">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
        请求趋势
      </div>
      <TrendChart v-if="timelineData.length"
        :data="timelineChartData"
        :lines="timelineLines"
        :xLabels="timelineXLabels"
        :height="140"
        :timeRanges="[{label:'24h',value:'24h'},{label:'7d',value:'7d'}]"
        :currentRange="timelineRange"
        @rangeChange="onTimelineRangeChange"
      />
      <div v-else style="color:var(--text-tertiary);font-size:var(--text-sm);text-align:center;padding:var(--space-3)">暂无趋势数据</div>
    </div>

    <!-- Filters — single row -->
    <div class="audit-filters">
      <div class="audit-filter-row">
        <div class="date-range-picker">
          <span class="date-label">开始</span>
          <input type="datetime-local" v-model="filters.start_time" class="date-input" />
          <span class="date-label">至</span>
          <input type="datetime-local" v-model="filters.end_time" class="date-input" />
          <button class="btn btn-ghost btn-sm" @click="applyDateRange" title="按日期范围筛选">筛选</button>
          <button v-if="filters.start_time || filters.end_time" class="btn btn-ghost btn-sm" @click="clearDateRange" title="清除日期范围">✕</button>
        </div>
      </div>
      <div class="audit-filter-row">
        <select v-model="filters.direction" @change="loadLogs">
          <option value="">全部方向</option>
          <option value="inbound">入站</option>
          <option value="outbound">出站</option>
        </select>
        <select v-model="filters.action" @change="loadLogs">
          <option value="">全部动作</option>
          <option value="pass">Pass</option>
          <option value="block">Block</option>
          <option value="warn">Warn</option>
        </select>
        <input v-model="filters.sender_id" placeholder="发送者 ID" />
        <input v-model="filters.q" placeholder="搜索内容..." />
        <button class="btn btn-sm" @click="loadLogs">搜索</button>
        <button class="btn btn-ghost btn-sm" @click="exportAudit('csv')">CSV</button>
        <button class="btn btn-ghost btn-sm" @click="exportAudit('json')">JSON</button>
      </div>
    </div>

    <!-- DataTable -->
    <DataTable
      :columns="columns"
      :data="logs"
      :loading="loading"
      :page-size="20"
      :page-sizes="[20, 50, 100]"
      empty-text="暂无日志记录"
      empty-desc="调整筛选条件或等待新请求"
      :expandable="true"
      :row-class="rowClass"
    >
      <template #empty-hint>调整筛选条件或等待新请求</template>
      <template #cell-timestamp="{ row }">{{ fmtTime(row.timestamp || row.time || row.created_at) }}</template>
      <template #cell-direction="{ value }">{{ value === 'inbound' ? '入站' : '出站' }}</template>
      <template #cell-action="{ value }"><span class="tag" :class="actTag(value)">{{ value }}</span></template>
      <template #cell-sender_id="{ row }">
        <a v-if="row.sender_id" class="user-link" @click.stop="$router.push('/user-profiles/' + encodeURIComponent(row.sender_id))">{{ row.sender_id }}</a>
        <span v-else>--</span>
      </template>
      <template #cell-trace_id="{ row }">
        <span style="font-size:var(--text-xs);font-family:var(--font-mono);color:var(--color-primary);cursor:pointer"
              :title="row.trace_id"
              @click.stop="filters.trace_id = row.trace_id; loadLogs()">
          {{ (row.trace_id || '--').substring(0, 8) }}{{ row.trace_id && row.trace_id.length > 8 ? '...' : '' }}
        </span>
      </template>
      <template #cell-content_preview="{ row }">
        <span style="max-width:300px;overflow:hidden;text-overflow:ellipsis;display:inline-block" :title="row.content_preview">
          {{ (row.content_preview || '--').substring(0, 80) }}{{ (row.content_preview || '').length > 80 ? '...' : '' }}
        </span>
      </template>
      <template #cell-latency="{ row }">{{ row.latency != null ? row.latency + 'ms' : (row.latency_ms != null ? row.latency_ms + 'ms' : '--') }}</template>
      <template #expand="{ row }">
        <div style="font-size:var(--text-sm);line-height:1.8;background:var(--bg-elevated);padding:var(--space-3);border-radius:var(--radius-md)">
          <div><b style="color:var(--color-primary)">时间:</b> {{ fmtTime(row.timestamp || row.time) }}</div>
          <div><b style="color:var(--color-primary)">方向:</b> {{ row.direction }} | <b style="color:var(--color-primary)">动作:</b> {{ row.action }}</div>
          <div><b style="color:var(--color-primary)">发送者:</b> <a v-if="row.sender_id" class="user-link" @click.stop="$router.push('/user-profiles/' + encodeURIComponent(row.sender_id))">{{ row.sender_id }}</a><span v-else>--</span> | <b style="color:var(--color-primary)">App ID:</b> {{ row.app_id || '--' }}</div>
          <div><b style="color:var(--color-primary)">Trace ID:</b> {{ row.trace_id || '--' }}</div>
          <div><b style="color:var(--color-primary)">上游:</b> {{ row.upstream_id || '--' }}</div>
          <div><b style="color:var(--color-primary)">原因:</b> {{ row.reason || '--' }}</div>
          <div><b style="color:var(--color-primary)">延迟:</b> {{ row.latency || row.latency_ms || '--' }}ms</div>
          <div v-if="row.content_preview"><b style="color:var(--color-primary)">内容:</b>
            <JsonHighlight :content="row.content_preview" />
          </div>
        </div>
      </template>
    </DataTable>

    <!-- Actions -->
    <div style="margin-top:var(--space-3);display:flex;gap:var(--space-2);flex-wrap:wrap">
      <button class="btn btn-ghost btn-sm" @click="loadAuditStats">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>
        统计
      </button>
      <button class="btn btn-danger btn-sm" @click="confirmCleanup">清理过期</button>
      <button class="btn btn-ghost btn-sm" @click="confirmArchive">手动归档</button>
      <button class="btn btn-ghost btn-sm" @click="loadArchives">查看归档</button>
    </div>

    <!-- Stats -->
    <div v-if="auditStatsHtml" v-html="auditStatsHtml" style="margin-top:var(--space-2)"></div>

    <!-- Archives -->
    <div v-if="archives.length" style="margin-top:var(--space-2)">
      <div style="font-size:var(--text-sm);color:var(--color-primary);font-weight:500;margin-bottom:var(--space-2)">归档文件 ({{ archives.length }} 个)</div>
      <div class="table-wrap">
        <table>
          <tr><th>文件名</th><th>大小</th><th>时间</th><th>操作</th></tr>
          <tr v-for="a in archives" :key="a.name">
            <td style="font-family:var(--font-mono);font-size:var(--text-xs)">{{ a.name }}</td>
            <td>{{ formatSize(a.size) }}</td>
            <td>{{ fmtTime(a.mod_time) }}</td>
            <td><a :href="'/api/v1/audit/archives/' + encodeURIComponent(a.name)" class="btn btn-ghost btn-sm" style="text-decoration:none" target="_blank">下载</a></td>
          </tr>
        </table>
      </div>
    </div>

    <ConfirmModal :visible="confirmVisible" :title="confirmTitle" :message="confirmMessage" :type="confirmType" @confirm="doConfirm" @cancel="confirmVisible = false" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { api, apiPost, downloadFile, getToken } from '../api.js'
import { showToast } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import TrendChart from '../components/TrendChart.vue'
import JsonHighlight from '../components/JsonHighlight.vue'
import Skeleton from '../components/Skeleton.vue'

const route = useRoute()
const loading = ref(false)
const logs = ref([])
const timelineData = ref([])
const timelineRange = ref('24h')
const auditStatsHtml = ref('')
const archives = ref([])

const filters = reactive({ direction: '', action: '', sender_id: '', app_id: '', trace_id: '', q: '', start_time: '', end_time: '' })

const columns = [
  { key: 'timestamp', label: '时间', sortable: true },
  { key: 'direction', label: '方向', sortable: true },
  { key: 'app_id', label: 'App ID', sortable: true, tdStyle: { fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' } },
  { key: 'sender_id', label: '发送者', sortable: true },
  { key: 'action', label: '动作', sortable: true },
  { key: 'trace_id', label: 'Trace ID', sortable: false },
  { key: 'content_preview', label: '内容', sortable: false },
  { key: 'reason', label: '原因', sortable: true },
  { key: 'latency', label: '延迟', sortable: true },
]

const confirmVisible = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmType = ref('warning')
let confirmAction = null

function fmtTime(ts) { if (!ts) return '--'; const d = new Date(ts); return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false }) }
function actTag(a) { a = (a || '').toLowerCase(); return a === 'block' ? 'tag-block' : a === 'warn' ? 'tag-warn' : a === 'log' ? 'tag-log' : 'tag-pass' }
function rowClass(row) { const a = (row.action || '').toLowerCase(); return a === 'block' ? 'row-block' : a === 'warn' ? 'row-warn' : '' }
function formatSize(bytes) { const kb = Math.round((bytes || 0) / 1024); return kb > 1024 ? (kb / 1024).toFixed(1) + ' MB' : kb + ' KB' }

const timelineChartData = computed(() => {
  return timelineData.value.map(t => ({
    pass: t.pass || 0,
    block: t.block || 0,
    warn: t.warn || 0,
  }))
})

const timelineLines = [
  { key: 'pass', color: '#10B981', label: 'Pass' },
  { key: 'block', color: '#EF4444', label: 'Block' },
  { key: 'warn', color: '#F59E0B', label: 'Warn' },
]

const timelineXLabels = computed(() => {
  return timelineData.value.map(t => {
    const h = t.hour || ''
    if (timelineRange.value === '7d') return h.substring(5, 10)
    const hourPart = h.substring(11, 13)
    return hourPart ? hourPart + ':00' : ''
  })
})

function onTimelineRangeChange(range) {
  timelineRange.value = range
  loadTimeline()
}

function applyDateRange() { loadLogs() }
function clearDateRange() { filters.start_time = ''; filters.end_time = ''; loadLogs() }

async function loadLogs() {
  loading.value = true
  const params = []
  if (filters.direction) params.push('direction=' + encodeURIComponent(filters.direction))
  if (filters.action) params.push('action=' + encodeURIComponent(filters.action))
  if (filters.sender_id) params.push('sender_id=' + encodeURIComponent(filters.sender_id))
  if (filters.app_id) params.push('app_id=' + encodeURIComponent(filters.app_id))
  if (filters.trace_id) params.push('trace_id=' + encodeURIComponent(filters.trace_id))
  if (filters.q) params.push('q=' + encodeURIComponent(filters.q))
  if (filters.start_time) params.push('start_time=' + encodeURIComponent(new Date(filters.start_time).toISOString()))
  if (filters.end_time) params.push('end_time=' + encodeURIComponent(new Date(filters.end_time).toISOString()))
  const qs = params.length ? '?' + params.join('&') : ''
  try { const d = await api('/api/v1/audit/logs' + qs); logs.value = d.logs || [] } catch { logs.value = [] }
  loading.value = false
}

async function loadTimeline() {
  try {
    const hours = timelineRange.value === '7d' ? 168 : 24
    const d = await api('/api/v1/audit/timeline?hours=' + hours)
    timelineData.value = d.timeline || []
  } catch { timelineData.value = [] }
}

async function exportAudit(fmt) {
  const params = ['format=' + fmt, 'limit=10000']
  if (filters.direction) params.push('direction=' + encodeURIComponent(filters.direction))
  if (filters.action) params.push('action=' + encodeURIComponent(filters.action))
  if (filters.sender_id) params.push('sender_id=' + encodeURIComponent(filters.sender_id))
  if (filters.app_id) params.push('app_id=' + encodeURIComponent(filters.app_id))
  if (filters.q) params.push('q=' + encodeURIComponent(filters.q))
  const url = location.origin + '/api/v1/audit/export?' + params.join('&')
  try { await downloadFile(url, 'audit_logs.' + fmt); showToast('导出 ' + fmt.toUpperCase() + ' 成功', 'success') } catch (e) { showToast('导出失败: ' + e.message, 'error') }
}

async function loadAuditStats() {
  try {
    const d = await api('/api/v1/audit/stats')
    const kb = d.disk_bytes ? Math.round(d.disk_bytes / 1024) : 0
    const ss = kb > 1024 ? (kb / 1024).toFixed(1) + ' MB' : kb + ' KB'
    auditStatsHtml.value = `<div style="display:flex;gap:var(--space-4);flex-wrap:wrap;font-size:var(--text-sm);padding:var(--space-2);background:var(--bg-elevated);border-radius:var(--radius-md)"><span>总数: <b style="color:var(--color-primary)">${d.total}</b></span><span>最早: <b>${fmtTime(d.earliest)}</b></span><span>最晚: <b>${fmtTime(d.latest)}</b></span><span>磁盘: <b>${ss}</b></span></div>`
  } catch (e) { showToast('获取统计失败: ' + e.message, 'error') }
}

async function loadArchives() {
  try { const d = await api('/api/v1/audit/archives'); archives.value = d.archives || [] } catch (e) { showToast('加载归档失败: ' + e.message, 'error') }
}

function confirmCleanup() {
  confirmTitle.value = '清理过期日志'
  confirmMessage.value = '确认清理过期日志？此操作将删除超过保留天数的日志记录。'
  confirmType.value = 'danger'
  confirmAction = async () => {
    try { const d = await apiPost('/api/v1/audit/cleanup', {}); showToast('清理完成：删除 ' + d.deleted + ' 条', 'success'); loadLogs(); loadAuditStats() } catch (e) { showToast('清理失败: ' + e.message, 'error') }
  }
  confirmVisible.value = true
}

function confirmArchive() {
  confirmTitle.value = '手动归档'
  confirmMessage.value = '确认手动归档过期日志？归档后日志将被压缩保存。'
  confirmType.value = 'warning'
  confirmAction = async () => {
    try {
      const d = await apiPost('/api/v1/audit/archive', {})
      if (d.status === 'no_data') { showToast('没有需要归档的日志', 'success'); return }
      showToast('归档完成：已归档 ' + d.deleted + ' 条', 'success')
      loadLogs(); loadAuditStats(); loadArchives()
    } catch (e) { showToast('归档失败: ' + e.message, 'error') }
  }
  confirmVisible.value = true
}

function doConfirm() { confirmVisible.value = false; if (confirmAction) confirmAction() }

let refreshTimer = null
onMounted(() => {
  // v11.4: 读取 route.query.since 并自动设置日期过滤器
  const sincePar = route.query.since
  if (sincePar) {
    const sinceMap = { '1h': 1, '24h': 24, '7d': 168, '30d': 720 }
    const hours = sinceMap[sincePar]
    if (hours) {
      const d = new Date(Date.now() - hours * 3600000)
      filters.start_time = d.toISOString().slice(0, 16)
      timelineRange.value = hours > 24 ? '7d' : '24h'
    }
  }
  loadLogs()
  loadTimeline()
  refreshTimer = setInterval(() => { loadLogs(); loadTimeline() }, 30000)
})
onUnmounted(() => clearInterval(refreshTimer))
</script>

<style scoped>
.audit-filters {
  margin-bottom: var(--space-3);
}
.audit-filter-row {
  display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap;
  margin-bottom: var(--space-2);
}
.audit-filter-row select,
.audit-filter-row input:not([type="datetime-local"]) {
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-2) var(--space-3);
  font-size: var(--text-sm); outline: none; font-family: var(--font-sans);
  transition: border-color var(--transition-fast);
}
.audit-filter-row select:focus,
.audit-filter-row input:focus { border-color: var(--color-primary); }
.audit-filter-row select option { background: var(--bg-elevated); }
.date-range-picker {
  display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap;
  background: var(--bg-elevated); border-radius: var(--radius-md); padding: var(--space-1) var(--space-2);
  border: 1px solid var(--border-subtle);
}
.date-label { font-size: var(--text-xs); color: var(--text-tertiary); white-space: nowrap; }
.date-input {
  background: var(--bg-base); border: 1px solid var(--border-default);
  border-radius: var(--radius-sm); color: var(--text-primary); padding: var(--space-1) var(--space-2);
  font-size: var(--text-xs); outline: none; color-scheme: dark;
  font-family: var(--font-sans);
}
.date-input:focus { border-color: var(--color-primary); }
.user-link { color: var(--color-primary); cursor: pointer; text-decoration: none; font-weight: 500; }
.user-link:hover { text-decoration: underline; }
</style>
