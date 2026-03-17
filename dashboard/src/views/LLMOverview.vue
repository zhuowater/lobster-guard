<template>
  <div>
    <!-- Stat Cards -->
    <div class="ov-cards" v-if="loaded">
      <StatCard
        :iconSvg="svgCalls" :value="overview.total_calls" label="总调用数" color="indigo"
      />
      <StatCard
        :iconSvg="svgToken" :value="formatTokens(overview.total_tokens)" label="Token 用量" color="blue"
      />
      <StatCard
        :iconSvg="svgSpeed" :value="overview.avg_latency_ms + 'ms'" label="平均延迟" color="green"
      />
      <StatCard
        :iconSvg="svgError" :value="(overview.error_rate * 100).toFixed(1) + '%'" label="错误率" color="red"
      />
    </div>
    <div class="ov-cards" v-else>
      <Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" />
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

    <!-- 模型分布 + 最近调用 -->
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
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
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

const loaded = ref(false)
const overview = ref({ total_calls: 0, total_tokens: 0, avg_latency_ms: 0, error_rate: 0, models: [] })
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
    // 使用 tool timeline 作为趋势代理（包含按小时聚合的数据）
    const d = await api('/api/v1/llm/tools/timeline?hours=' + hours)
    timelineData.value = d.timeline || []
  } catch { timelineData.value = [] }
}

async function loadData() {
  try {
    const d = await api('/api/v1/llm/overview')
    overview.value = d
  } catch {
    overview.value = { total_calls: 0, total_tokens: 0, avg_latency_ms: 0, error_rate: 0, models: [] }
  }
  try {
    const d = await api('/api/v1/llm/calls?limit=20')
    callsData.value = d.records || []
  } catch { callsData.value = [] }
  await loadTimeline()
  loaded.value = true
}

let timer = null
onMounted(() => { loadData(); timer = setInterval(loadData, 30000) })
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
code { background: var(--bg-elevated); padding: 2px 6px; border-radius: 4px; font-size: var(--text-xs); font-family: var(--font-mono); }
.row-error { background: rgba(239, 68, 68, 0.06) !important; }
.status-badge { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: var(--text-xs); font-weight: 600; }
.status-ok { background: rgba(16, 185, 129, 0.15); color: #10B981; }
.status-err { background: rgba(239, 68, 68, 0.15); color: #EF4444; }
</style>
