<template>
  <div class="events-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">📡 事件总线</h1>
        <p class="page-subtitle">全局安全事件流 — 所有模块的事件统一收集与分发</p>
      </div>
      <button class="btn btn-sm" @click="loadAll">🔄 刷新</button>
    </div>

    <!-- StatCards -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon">📊</div>
        <div class="stat-value">{{ stats.total_events ?? '-' }}</div>
        <div class="stat-label">总事件数</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🔴</div>
        <div class="stat-value severity-display">
          <span class="sev-dot sev-critical-dot">{{ sevCounts.critical || 0 }}</span>
          <span class="sev-dot sev-high-dot">{{ sevCounts.high || 0 }}</span>
          <span class="sev-dot sev-medium-dot">{{ sevCounts.medium || 0 }}</span>
          <span class="sev-dot sev-low-dot">{{ sevCounts.low || 0 }}</span>
        </div>
        <div class="stat-label">严重程度分布</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🔗</div>
        <div class="stat-value">{{ stats.webhook_targets ?? '-' }}</div>
        <div class="stat-label">Webhook 目标</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">⏰</div>
        <div class="stat-value">{{ stats.last_24h ?? '-' }}</div>
        <div class="stat-label">最近 24h</div>
      </div>
    </div>

    <!-- 筛选栏 -->
    <div class="filter-bar">
      <div class="filter-group">
        <label>类型</label>
        <select v-model="filterType" @change="loadEvents">
          <option value="">全部</option>
          <option v-for="t in eventTypes" :key="t" :value="t">{{ t }}</option>
        </select>
      </div>
      <div class="filter-group">
        <label>严重程度</label>
        <select v-model="filterSeverity" @change="loadEvents">
          <option value="">全部</option>
          <option value="critical">Critical</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </select>
      </div>
      <div class="filter-group">
        <label>时间范围</label>
        <select v-model="filterTime" @change="loadEvents">
          <option value="">全部</option>
          <option value="1h">最近 1 小时</option>
          <option value="24h">最近 24 小时</option>
          <option value="7d">最近 7 天</option>
        </select>
      </div>
    </div>

    <!-- 事件列表 -->
    <div class="table-wrap">
      <table class="data-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>类型</th>
            <th>严重程度</th>
            <th>域</th>
            <th>描述</th>
            <th>TraceID</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="e in events" :key="e.id || e.timestamp">
            <td class="td-mono">{{ formatTime(e.timestamp) }}</td>
            <td><span class="type-badge">{{ e.type || e.event_type }}</span></td>
            <td>
              <span class="severity-badge" :class="'sev-' + (e.severity || 'low')">{{ e.severity || 'low' }}</span>
            </td>
            <td>{{ e.domain || '-' }}</td>
            <td class="td-desc">{{ truncate(e.description || e.message || e.detail, 60) }}</td>
            <td class="td-mono">{{ truncate(e.trace_id, 12) }}</td>
          </tr>
        </tbody>
      </table>
      <div v-if="events.length === 0" class="empty-state">暂无事件记录</div>
    </div>

    <!-- 错误提示 -->
    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { api } from '../api.js'

const stats = ref({})
const events = ref([])
const error = ref('')
const filterType = ref('')
const filterSeverity = ref('')
const filterTime = ref('')
const eventTypes = ref([])

const sevCounts = computed(() => {
  return stats.value.by_severity || stats.value.severity_counts || {}
})

async function loadStats() {
  try {
    const d = await api('/api/v1/events/stats')
    stats.value = d
    // 提取事件类型列表
    if (d.event_types) eventTypes.value = d.event_types
  } catch (e) { error.value = '加载统计失败: ' + e.message }
}

async function loadEvents() {
  try {
    let url = '/api/v1/events/list?limit=50'
    if (filterType.value) url += '&type=' + encodeURIComponent(filterType.value)
    if (filterSeverity.value) url += '&severity=' + encodeURIComponent(filterSeverity.value)
    if (filterTime.value) url += '&time_range=' + encodeURIComponent(filterTime.value)
    const d = await api(url)
    events.value = d.events || d || []
  } catch (e) { error.value = '加载事件列表失败: ' + e.message }
}

function loadAll() {
  error.value = ''
  loadStats()
  loadEvents()
}

function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function formatTime(ts) {
  if (!ts) return '-'
  try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) } catch { return ts }
}

onMounted(loadAll)
</script>

<style scoped>
.events-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

/* Stats */
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); text-align: center; }
.stat-icon { font-size: 1.5rem; margin-bottom: var(--space-1); }
.stat-value { font-size: 1.75rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }

.severity-display { display: flex; gap: 6px; justify-content: center; font-size: 0.9rem; }
.sev-dot { padding: 1px 5px; border-radius: 4px; font-size: 11px; font-weight: 700; }
.sev-critical-dot { background: #DC2626; color: #fff; }
.sev-high-dot { background: #F97316; color: #fff; }
.sev-medium-dot { background: #F59E0B; color: #1a1a2e; }
.sev-low-dot { background: #10B981; color: #fff; }

/* Filter Bar */
.filter-bar { display: flex; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; align-items: flex-end; }
.filter-group { display: flex; flex-direction: column; gap: 4px; }
.filter-group label { font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; }
.filter-group select {
  background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md);
  color: var(--text-primary); padding: 6px 10px; font-size: var(--text-xs); cursor: pointer;
}

.table-wrap { overflow-x: auto; }

/* Data Table */
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th {
  text-align: left; padding: 8px 10px; background: var(--bg-elevated);
  color: var(--text-tertiary); font-weight: 600; font-size: 10px;
  text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle);
  white-space: nowrap;
}
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.td-desc { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.type-badge { display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 600; background: rgba(99,102,241,.15); color: #a5b4fc; }

.severity-badge { display: inline-block; padding: 1px 8px; border-radius: 9999px; font-size: 10px; font-weight: 700; text-transform: uppercase; }
.sev-critical { background: #DC2626; color: #fff; }
.sev-high { background: #F97316; color: #fff; }
.sev-medium { background: #F59E0B; color: #1a1a2e; }
.sev-low { background: #10B981; color: #fff; }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-sm { padding: 6px 12px; font-size: var(--text-xs); }

.empty-state { text-align: center; padding: var(--space-6); color: var(--text-tertiary); }

.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); }

@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .filter-bar { flex-direction: column; }
}
</style>
