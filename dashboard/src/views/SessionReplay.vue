<template>
  <div>
    <div class="card">
      <div class="card-header">
        <span class="card-icon"><Icon name="film" :size="18" /></span>
        <span class="card-title">会话回放</span>
        <div class="card-actions">
          <button class="btn btn-ghost btn-sm" @click="loadSessions"><Icon name="refresh" :size="14" /> 刷新</button>
        </div>
      </div>

      <!-- Filters -->
      <div class="replay-filters">
        <div class="filter-group">
          <select v-model="filters.range" @change="onRangeChange" class="filter-select">
            <option value="24h">最近 24 小时</option>
            <option value="7d">最近 7 天</option>
            <option value="30d">最近 30 天</option>
            <option value="">全部</option>
          </select>
          <select v-model="filters.risk" @change="loadSessions" class="filter-select">
            <option value="">全部风险</option>
            <option value="high">高风险+</option>
            <option value="critical">严重</option>
          </select>
          <div class="search-box">
            <Icon name="search" :size="14" class="search-icon" />
            <input v-model="filters.q" placeholder="搜索 trace_id / 用户 / 内容..." @keyup.enter="loadSessions" />
          </div>
          <button class="btn btn-sm" @click="loadSessions">搜索</button>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="loading-state">
        <Icon name="loader" :size="20" /> 加载中...
      </div>

      <!-- Empty -->
      <div v-else-if="sessions.length === 0" class="empty-state">
        <div class="empty-icon">🎬</div>
        <div class="empty-title">暂无会话</div>
        <div class="empty-desc">尝试注入演示数据或调整筛选条件</div>
      </div>

      <!-- Session Cards -->
      <div v-else class="session-list">
        <div
          v-for="s in sessions" :key="s.trace_id"
          class="session-card"
          :class="'risk-' + s.risk_level"
          @click="goDetail(s.trace_id)"
        >
          <div class="session-top">
            <span class="session-trace mono">{{ s.trace_id }}</span>
            <span class="session-duration">{{ fmtDuration(s.duration_ms) }}</span>
          </div>
          <div class="session-meta">
            <span v-if="s.sender_id">👤 {{ s.sender_id }}</span>
            <span v-if="s.model">🧠 {{ s.model }}</span>
            <span class="session-time">{{ fmtTime(s.start_time) }}</span>
          </div>
          <div class="session-stats">
            <span class="stat-item" title="IM 事件"><span class="stat-label">IM</span> {{ s.im_events }}</span>
            <span class="stat-item" title="LLM 调用"><span class="stat-label">LLM</span> {{ s.llm_calls }}</span>
            <span class="stat-item" title="工具调用"><span class="stat-label">Tools</span> {{ s.tool_calls }}</span>
            <span class="stat-item" title="Token 用量"><span class="stat-label">Tokens</span> {{ fmtNum(s.total_tokens) }}</span>
          </div>
          <div class="session-bottom">
            <div class="risk-badges">
              <span class="risk-badge" :class="'badge-' + s.risk_level">{{ riskLabel(s.risk_level) }}</span>
              <span class="risk-flag" v-if="s.canary_leaked">🔴 canary leaked</span>
              <span class="risk-flag" v-if="s.budget_exceeded">⚠️ budget exceeded</span>
              <span class="risk-flag" v-if="s.blocked">🚫 blocked</span>
              <span class="risk-flag flagged" v-if="s.flagged_tools > 0">⛳ {{ s.flagged_tools }} flagged</span>
              <span class="tag-pill" v-for="tag in (s.tags || []).slice(0, 3)" :key="tag">💬 {{ tag }}</span>
            </div>
            <button class="btn-play" @click.stop="goDetail(s.trace_id)">
              <Icon name="play" :size="12" /> 查看回放
            </button>
          </div>
        </div>
      </div>

      <!-- Pagination -->
      <div class="pagination" v-if="total > pageSize">
        <button class="btn btn-ghost btn-sm" :disabled="page <= 1" @click="page--; loadSessions()">上一页</button>
        <span class="page-info">{{ page }} / {{ Math.ceil(total / pageSize) }}（共 {{ total }} 条）</span>
        <button class="btn btn-ghost btn-sm" :disabled="page >= Math.ceil(total / pageSize)" @click="page++; loadSessions()">下一页</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import Icon from '../components/Icon.vue'

const router = useRouter()
const loading = ref(false)
const sessions = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20

const filters = reactive({ range: '7d', risk: '', q: '' })

function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? ts : d.toLocaleString('zh-CN', { hour12: false })
}

function fmtDuration(ms) {
  if (!ms || ms <= 0) return '--'
  if (ms < 1000) return Math.round(ms) + 'ms'
  if (ms < 60000) return (ms / 1000).toFixed(1) + 's'
  const min = Math.floor(ms / 60000)
  const sec = Math.floor((ms % 60000) / 1000)
  return min + 'm ' + sec + 's'
}

function fmtNum(n) {
  if (!n) return '0'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

function riskLabel(level) {
  const map = { critical: '🔴 严重', high: '🟠 高危', medium: '🟡 中等', low: '🟢 低风险' }
  return map[level] || level
}

function onRangeChange() { page.value = 1; loadSessions() }

function goDetail(traceId) {
  router.push('/sessions/' + encodeURIComponent(traceId))
}

async function loadSessions() {
  loading.value = true
  const params = []
  if (filters.range) params.push('from=' + filters.range)
  if (filters.risk) params.push('risk=' + filters.risk)
  if (filters.q) params.push('q=' + encodeURIComponent(filters.q))
  params.push('limit=' + pageSize)
  params.push('offset=' + ((page.value - 1) * pageSize))
  const qs = params.length ? '?' + params.join('&') : ''
  try {
    const d = await api('/api/v1/sessions/replay' + qs)
    sessions.value = d.sessions || []
    total.value = d.total || 0
  } catch (e) {
    sessions.value = []
    total.value = 0
  }
  loading.value = false
}

let timer = null
onMounted(() => { loadSessions(); timer = setInterval(loadSessions, 60000) })
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.replay-filters { margin-bottom: var(--space-4); }
.filter-group {
  display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap;
}
.filter-select {
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-2) var(--space-3);
  font-size: var(--text-sm); outline: none;
}
.filter-select option { background: var(--bg-elevated); }
.search-box {
  position: relative; flex: 1; min-width: 200px;
}
.search-box .search-icon {
  position: absolute; left: 10px; top: 50%; transform: translateY(-50%);
  color: var(--text-tertiary); pointer-events: none;
}
.search-box input {
  width: 100%; padding: var(--space-2) var(--space-3) var(--space-2) 32px;
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary);
  font-size: var(--text-sm); outline: none;
}
.search-box input:focus { border-color: var(--color-primary); }

.loading-state {
  display: flex; align-items: center; justify-content: center; gap: 8px;
  padding: var(--space-8); color: var(--text-tertiary); font-size: var(--text-sm);
}
.empty-state { text-align: center; padding: var(--space-8); }
.empty-icon { font-size: 3rem; margin-bottom: var(--space-2); }
.empty-title { font-size: var(--text-lg); font-weight: 600; color: var(--text-primary); margin-bottom: var(--space-1); }
.empty-desc { font-size: var(--text-sm); color: var(--text-tertiary); }

.session-list { display: flex; flex-direction: column; gap: var(--space-3); }

.session-card {
  background: var(--bg-elevated); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-4);
  cursor: pointer; transition: all var(--transition-fast);
  border-left: 4px solid var(--border-subtle);
}
.session-card:hover { border-color: var(--color-primary); background: var(--bg-base); transform: translateX(2px); }
.session-card.risk-critical { border-left-color: #EF4444; }
.session-card.risk-high { border-left-color: #F97316; }
.session-card.risk-medium { border-left-color: #EAB308; }
.session-card.risk-low { border-left-color: #22C55E; }

.session-top {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: var(--space-2);
}
.session-trace { font-size: var(--text-sm); color: var(--color-primary); font-weight: 600; }
.session-duration { font-size: var(--text-xs); color: var(--text-tertiary); font-family: var(--font-mono); }
.mono { font-family: var(--font-mono); }

.session-meta {
  display: flex; gap: var(--space-3); font-size: var(--text-xs); color: var(--text-secondary);
  margin-bottom: var(--space-3); flex-wrap: wrap;
}
.session-time { color: var(--text-tertiary); }

.session-stats {
  display: flex; gap: var(--space-4); margin-bottom: var(--space-3);
}
.stat-item {
  font-size: var(--text-sm); font-weight: 600; color: var(--text-primary);
  font-family: var(--font-mono);
}
.stat-label {
  font-size: 10px; font-weight: 500; color: var(--text-tertiary);
  text-transform: uppercase; letter-spacing: 0.05em; margin-right: 4px;
  font-family: var(--font-sans);
}

.session-bottom {
  display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: var(--space-2);
}
.risk-badges { display: flex; flex-wrap: wrap; gap: var(--space-2); align-items: center; }
.risk-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 12px;
}
.badge-critical { background: rgba(239,68,68,0.15); color: #EF4444; }
.badge-high { background: rgba(249,115,22,0.15); color: #F97316; }
.badge-medium { background: rgba(234,179,8,0.15); color: #EAB308; }
.badge-low { background: rgba(34,197,94,0.15); color: #22C55E; }
.risk-flag { font-size: 11px; color: var(--text-secondary); }
.risk-flag.flagged { color: #F97316; font-weight: 600; }
.tag-pill {
  font-size: 10px; background: rgba(255,255,255,0.08); padding: 2px 6px;
  border-radius: 8px; color: var(--text-tertiary);
}

.btn-play {
  display: flex; align-items: center; gap: 4px;
  background: var(--color-primary-dim); color: var(--color-primary);
  border: none; border-radius: var(--radius-md); padding: 6px 12px;
  font-size: var(--text-xs); font-weight: 600; cursor: pointer;
  transition: all var(--transition-fast);
}
.btn-play:hover { background: var(--color-primary); color: #fff; }

.pagination {
  display: flex; align-items: center; justify-content: center; gap: var(--space-3);
  margin-top: var(--space-4); padding-top: var(--space-3);
  border-top: 1px solid var(--border-subtle);
}
.page-info { font-size: var(--text-xs); color: var(--text-tertiary); }
</style>
