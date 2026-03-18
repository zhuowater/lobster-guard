<template>
  <div>
    <!-- OWASP LLM Top 10 矩阵 (v11.1) -->
    <div class="card owasp-section" style="margin-bottom:20px" v-if="owaspMatrix.length">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg></span>
        <span class="card-title">OWASP LLM Top 10 矩阵</span>
        <div class="refresh-control" style="margin-left:auto">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
          <select v-model="refreshInterval" @change="onRefreshChange" class="refresh-select">
            <option value="30000">30s</option><option value="60000">1m</option><option value="300000">5m</option><option value="0">手动</option>
          </select>
        </div>
      </div>
      <div class="owasp-grid">
        <div v-for="item in owaspMatrix" :key="item.id" class="owasp-card" :class="'owasp-'+item.risk_level" @click="onOwaspClick(item)">
          <div class="owasp-id">{{ item.id }}</div>
          <div class="owasp-name">{{ item.name_zh }}</div>
          <div class="owasp-count">{{ item.count }}</div>
          <div class="owasp-label">24h 事件</div>
        </div>
      </div>
    </div>

    <!-- Stat Cards -->
    <div class="ov-cards" v-if="loaded">
      <StatCard
        :iconSvg="svgCalls" :value="overview.total_calls" label="总调用数" color="indigo"
        class="stat-clickable" @click="router.push('/agent')"
      />
      <StatCard
        :iconSvg="svgToken" :value="formatTokens(overview.total_tokens)" label="Token 用量" color="blue"
        class="stat-clickable" @click="router.push({ path: '/settings', query: { section: 'cost' } })"
      />
      <StatCard
        :iconSvg="svgSpeed" :value="overview.avg_latency_ms + 'ms'" label="平均延迟" color="green"
        class="stat-clickable" @click="router.push('/agent')"
      />
      <StatCard
        :iconSvg="svgError" :value="(overview.error_rate * 100).toFixed(1) + '%'" label="错误率" color="red"
        class="stat-clickable" @click="router.push('/agent')"
      />
    </div>
    <div class="ov-cards" v-else>
      <Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" />
    </div>

    <!-- 成本看板 -->
    <div class="ov-row" v-if="loaded" style="margin-bottom:20px">
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 6v12"/><path d="M15.5 9h-5a2.5 2.5 0 0 0 0 5h3a2.5 2.5 0 0 1 0 5h-5"/></svg></span>
          <span class="card-title">今日成本</span>
        </div>
        <div class="cost-today">
          <div class="cost-big" :class="costAlertClass">${{ overview.today_cost_usd?.toFixed(2) || '0.00' }}</div>
          <div v-if="overview.daily_limit_usd > 0" class="cost-limit-bar">
            <div class="cost-limit-label">
              <span>日限额 ${{ overview.daily_limit_usd }}</span>
              <span :class="costAlertClass">{{ costPct }}%</span>
            </div>
            <div class="cost-bar-track">
              <div class="cost-bar-fill" :style="{ width: Math.min(costPct, 100) + '%', background: costBarColor }"></div>
            </div>
            <div v-if="overview.cost_alert_triggered" class="cost-alert-msg">⚠️ 已超出日限额！</div>
          </div>
          <div v-else class="cost-no-limit">未设置日限额</div>
        </div>
      </div>
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg></span>
          <span class="card-title">7 天成本趋势</span>
        </div>
        <div class="cost-trend-chart">
          <div v-for="d in costTrendData" :key="d.date" class="cost-bar-col">
            <div class="cost-bar-value">${{ d.cost_usd.toFixed(2) }}</div>
            <div class="cost-bar-outer">
              <div class="cost-bar-inner" :style="{ height: d.barPct + '%', background: d.overLimit ? '#EF4444' : '#6366F1' }"></div>
            </div>
            <div class="cost-bar-date">{{ d.dateShort }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 调用趋势 -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span>
        <span class="card-title">LLM 调用趋势</span>
      </div>
      <Skeleton v-if="!loaded" type="chart" />
      <EmptyState v-else-if="!callsData.length"
        :iconSvg="svgTrend" title="暂无调用数据" description="LLM 代理运行后将自动收集调用数据"
      />
      <TrendChart v-else
        :data="trendChartData" :lines="trendLines" :xLabels="trendXLabels" :height="170"
        :timeRanges="[{label:'24h',value:'24h'},{label:'7d',value:'7d'}]"
        :currentRange="trendRange" @rangeChange="onTrendRangeChange"
      />
    </div>

    <!-- 模型分布 + 模型成本明细 -->
    <div class="ov-row">
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.21 15.89A10 10 0 1 1 8 2.83"/><path d="M22 12A10 10 0 0 0 12 2v10z"/></svg></span>
          <span class="card-title">模型使用分布</span>
        </div>
        <Skeleton v-if="!loaded" type="chart" />
        <PieChart v-else :data="modelPieData" :size="180" />
      </div>
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 6v12"/><path d="M15.5 9h-5a2.5 2.5 0 0 0 0 5h3a2.5 2.5 0 0 1 0 5h-5"/></svg></span>
          <span class="card-title">模型成本明细</span>
        </div>
        <Skeleton v-if="!loaded" type="table" />
        <EmptyState v-else-if="!costByModel.length"
          :iconSvg="svgTrend" title="暂无成本数据" description="LLM 代理运行后将自动计算成本"
        />
        <div v-else class="table-wrap">
          <table>
            <thead>
              <tr><th>模型</th><th>调用次数</th><th>Token 用量</th><th>成本(USD)</th><th>占比</th></tr>
            </thead>
            <tbody>
              <tr v-for="m in costByModel" :key="m.model">
                <td><code>{{ shortModel(m.model) }}</code></td>
                <td>{{ m.calls }}</td>
                <td>{{ formatTokens(m.tokens) }}</td>
                <td style="font-weight:600;color:var(--color-warning)">${{ m.cost_usd.toFixed(2) }}</td>
                <td>
                  <div class="cost-pct-bar">
                    <div class="cost-pct-fill" :style="{ width: m.pct + '%' }"></div>
                    <span>{{ m.pct.toFixed(1) }}%</span>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- 最近调用 -->
    <div class="card" style="margin-top:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg></span>
        <span class="card-title">最近调用</span>
      </div>
      <Skeleton v-if="!loaded" type="table" />
      <EmptyState v-else-if="!callsData.length"
        :iconSvg="svgTrend" title="暂无调用记录" description="LLM 代理运行后将自动收集数据"
      />
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr><th>时间</th><th>模型</th><th>Token</th><th>延迟</th><th>工具数</th><th>状态</th></tr>
          </thead>
          <tbody>
            <tr v-for="c in callsData" :key="c.id"
                :class="{'row-error': c.status_code >= 400}">
              <td>{{ fmtTime(c.timestamp) }}</td>
              <td><code>{{ shortModel(c.model) }}</code></td>
              <td>{{ c.total_tokens }}</td>
              <td>{{ Math.round(c.latency_ms) }}ms</td>
              <td>{{ c.tool_count }}</td>
              <td>
                <span class="status-badge" :class="c.status_code < 400 ? 'status-ok' : 'status-err'">
                  {{ c.status_code }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import StatCard from '../components/StatCard.vue'
import TrendChart from '../components/TrendChart.vue'
import PieChart from '../components/PieChart.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'

const svgCalls = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44 2.5 2.5 0 0 1-2.96-3.08 3 3 0 0 1-.34-5.58 2.5 2.5 0 0 1 1.32-4.24A2.5 2.5 0 0 1 9.5 2"/><path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44 2.5 2.5 0 0 0 2.96-3.08 3 3 0 0 0 .34-5.58 2.5 2.5 0 0 0-1.32-4.24A2.5 2.5 0 0 0 14.5 2"/></svg>'
const svgToken = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 6v12"/><path d="M15.5 9h-5a2.5 2.5 0 0 0 0 5h3a2.5 2.5 0 0 1 0 5h-5"/></svg>'
const svgSpeed = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>'
const svgError = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>'
const svgTrend = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'

const modelColors = ['#6366F1', '#3B82F6', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6']
const router = useRouter()

// v11.1: OWASP 矩阵
const owaspMatrix = ref([])
const refreshInterval = ref(localStorage.getItem('llm_refresh') || '30000')

async function loadOwaspMatrix() {
  try { const d = await api('/api/v1/llm/owasp-matrix'); owaspMatrix.value = d.items || [] } catch { owaspMatrix.value = [] }
}
function onOwaspClick(item) {
  // 跳转到 LLM 规则页面，带上对应的 category
  const catMap = { 'LLM01': 'prompt_injection', 'LLM02': 'pii_leak', 'LLM04': 'token_abuse', 'LLM06': 'pii_leak', 'LLM07': 'custom' }
  const cat = catMap[item.id] || ''
  router.push({ path: '/llm-rules', query: cat ? { category: cat } : {} })
}
function onRefreshChange() { localStorage.setItem('llm_refresh', refreshInterval.value); setupLLMTimer() }

const loaded = ref(false)
const overview = ref({ total_calls: 0, total_tokens: 0, avg_latency_ms: 0, error_rate: 0, models: [], cost_by_model: [], cost_trend: [], daily_limit_usd: 0, today_cost_usd: 0, cost_alert_triggered: false })
const callsData = ref([])
const trendRange = ref('24h')
const timelineData = ref([])

function formatTokens(n) {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}
function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false })
}
function shortModel(m) {
  if (!m) return '--'
  return m.replace(/^claude-/, '').replace(/-2025.*$/, '')
}

// 成本看板计算
const costPct = computed(() => {
  if (!overview.value.daily_limit_usd || overview.value.daily_limit_usd <= 0) return 0
  return Math.round((overview.value.today_cost_usd || 0) / overview.value.daily_limit_usd * 100)
})
const costAlertClass = computed(() => {
  if (overview.value.cost_alert_triggered) return 'cost-alert'
  if (costPct.value >= 80) return 'cost-warning'
  return ''
})
const costBarColor = computed(() => {
  if (costPct.value >= 100) return '#EF4444'
  if (costPct.value >= 80) return '#F59E0B'
  return '#10B981'
})

// 7 天成本趋势
const costTrendData = computed(() => {
  const trend = overview.value.cost_trend || []
  const maxCost = Math.max(...trend.map(d => d.cost_usd || 0), 0.01)
  const limit = overview.value.daily_limit_usd || 0
  return trend.map(d => ({
    ...d,
    dateShort: (d.date || '').substring(5),
    barPct: Math.max(3, (d.cost_usd / maxCost) * 90),
    overLimit: limit > 0 && d.cost_usd >= limit,
  }))
})

// 模型成本明细
const costByModel = computed(() => {
  const items = overview.value.cost_by_model || []
  const totalCost = items.reduce((s, m) => s + (m.cost_usd || 0), 0) || 1
  return items.map(m => ({ ...m, pct: (m.cost_usd / totalCost) * 100 }))
    .sort((a, b) => b.cost_usd - a.cost_usd)
})

const modelPieData = computed(() => {
  const models = overview.value.models || []
  return models.map((m, i) => ({
    label: shortModel(m.name), value: m.count, color: modelColors[i % modelColors.length]
  }))
})

const trendChartData = computed(() => {
  return timelineData.value.map(t => ({ total: t.total || 0 }))
})
const trendLines = [{ key: 'total', color: '#6366F1', label: '调用数' }]
const trendXLabels = computed(() => {
  return timelineData.value.map(t => {
    const h = t.hour || ''
    if (trendRange.value === '7d') return h.substring(5, 10) + ' ' + h.substring(11, 13) + ':00'
    const hp = h.substring(11, 13)
    return hp ? hp + ':00' : ''
  })
})

function onTrendRangeChange(range) {
  trendRange.value = range
  loadTimeline()
}

async function loadTimeline() {
  try {
    const hours = trendRange.value === '7d' ? 168 : 24
    const d = await api('/api/v1/llm/tools/timeline?hours=' + hours)
    timelineData.value = d.timeline || []
  } catch { timelineData.value = [] }
}

async function loadData() {
  try {
    const d = await api('/api/v1/llm/overview')
    overview.value = d
  } catch {
    overview.value = { total_calls: 0, total_tokens: 0, avg_latency_ms: 0, error_rate: 0, models: [], cost_by_model: [], cost_trend: [], daily_limit_usd: 0, today_cost_usd: 0, cost_alert_triggered: false }
  }
  try {
    const d = await api('/api/v1/llm/calls?limit=20')
    callsData.value = d.records || []
  } catch { callsData.value = [] }
  await loadTimeline()
  loaded.value = true
}

let timer = null
function setupLLMTimer() { clearInterval(timer); const ms = parseInt(refreshInterval.value); if (ms > 0) timer = setInterval(() => { loadData(); loadOwaspMatrix() }, ms) }
onMounted(() => { loadData(); loadOwaspMatrix(); setupLLMTimer() })
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.stat-clickable { cursor: pointer !important; }
.stat-clickable:hover { transform: translateY(-3px) !important; box-shadow: var(--shadow-lg) !important; border-color: var(--color-primary) !important; }
code { background: var(--bg-elevated); padding: 2px 6px; border-radius: 4px; font-size: var(--text-xs); font-family: var(--font-mono); }
.row-error { background: rgba(239, 68, 68, 0.06) !important; }
.status-badge { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: var(--text-xs); font-weight: 600; }
.status-ok { background: rgba(16, 185, 129, 0.15); color: #10B981; }
.status-err { background: rgba(239, 68, 68, 0.15); color: #EF4444; }

/* 成本看板 */
.cost-today { padding: var(--space-2) 0; }
.cost-big { font-size: 2rem; font-weight: 800; color: var(--text-primary); font-family: var(--font-mono); }
.cost-big.cost-warning { color: #F59E0B; }
.cost-big.cost-alert { color: #EF4444; }
.cost-limit-bar { margin-top: var(--space-2); }
.cost-limit-label { display: flex; justify-content: space-between; font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: 4px; }
.cost-bar-track { height: 8px; background: rgba(255,255,255,0.06); border-radius: 9999px; overflow: hidden; }
.cost-bar-fill { height: 100%; border-radius: 9999px; transition: width .6s ease; }
.cost-alert-msg { font-size: var(--text-xs); color: #EF4444; margin-top: 6px; font-weight: 600; }
.cost-no-limit { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: 8px; }

/* 7 天条形图 */
.cost-trend-chart { display: flex; align-items: flex-end; gap: 6px; height: 150px; padding: var(--space-2) 0; }
.cost-bar-col { flex: 1; display: flex; flex-direction: column; align-items: center; height: 100%; }
.cost-bar-value { font-size: 9px; color: var(--text-tertiary); margin-bottom: 4px; white-space: nowrap; }
.cost-bar-outer { flex: 1; width: 100%; max-width: 32px; background: rgba(255,255,255,0.04); border-radius: 4px 4px 0 0; display: flex; align-items: flex-end; overflow: hidden; }
.cost-bar-inner { width: 100%; border-radius: 4px 4px 0 0; transition: height .6s ease; min-height: 2px; }
.cost-bar-date { font-size: 10px; color: var(--text-tertiary); margin-top: 4px; }

/* 成本占比条 */
.cost-pct-bar { display: flex; align-items: center; gap: 6px; }
.cost-pct-fill { height: 6px; border-radius: 3px; background: var(--color-primary); min-width: 2px; }
.cost-pct-bar span { font-size: var(--text-xs); color: var(--text-tertiary); white-space: nowrap; }
/* OWASP 矩阵 */
.owasp-grid { display: grid; grid-template-columns: repeat(5, 1fr); gap: 10px; padding: var(--space-2) 0; }
.owasp-card { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-3); text-align: center; cursor: pointer; transition: all var(--transition-fast); }
.owasp-card:hover { transform: translateY(-2px); box-shadow: var(--shadow-md); border-color: var(--color-primary); }
.owasp-id { font-size: 10px; font-weight: 700; color: var(--text-tertiary); font-family: var(--font-mono); }
.owasp-name { font-size: var(--text-xs); font-weight: 600; color: var(--text-primary); margin: 4px 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.owasp-count { font-size: 1.25rem; font-weight: 800; font-family: var(--font-mono); }
.owasp-label { font-size: 9px; color: var(--text-disabled); }
.owasp-none .owasp-count { color: var(--text-disabled); }
.owasp-none { opacity: 0.6; }
.owasp-low .owasp-count { color: #F59E0B; }
.owasp-low { border-color: rgba(245, 158, 11, 0.3); }
.owasp-high .owasp-count { color: #EF4444; }
.owasp-high { border-color: rgba(239, 68, 68, 0.3); background: rgba(239, 68, 68, 0.05); }
.refresh-control { display: flex; align-items: center; gap: var(--space-1); color: var(--text-tertiary); }
.refresh-select { background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); font-size: var(--text-xs); padding: 2px 6px; cursor: pointer; }
@media(max-width:768px) { .owasp-grid { grid-template-columns: repeat(2, 1fr); } }
</style>
