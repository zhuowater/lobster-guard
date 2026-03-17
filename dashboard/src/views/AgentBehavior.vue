<template>
  <div>
    <!-- 安全告警面板 -->
    <div v-if="loaded && alertCount > 0" class="alert-panel" style="margin-bottom:20px">
      <div class="alert-header">
        <span class="alert-icon">⚠️</span>
        <span class="alert-title">检测到 {{ alertCount }} 个高危工具调用</span>
      </div>
      <div class="alert-items">
        <div v-for="a in alertPreview" :key="a.id" class="alert-item">
          <code>{{ a.tool_name }}</code>
          <span class="alert-time">{{ fmtTime(a.timestamp) }}</span>
          <span class="alert-reason">{{ a.flag_reason || a.risk_level }}</span>
        </div>
      </div>
      <a class="alert-link" @click="scrollToHighRisk">查看全部 →</a>
    </div>

    <!-- Stat Cards -->
    <div class="ov-cards" v-if="loaded">
      <StatCard :iconSvg="svgRobot" :value="stats.total" label="总工具调用" color="blue" />
      <StatCard :iconSvg="svgAlert" :value="stats.high_risk_count" label="高危调用" color="red" />
      <StatCard :iconSvg="svgFlag" :value="stats.flagged_count" label="已标记" color="yellow" />
      <StatCard :iconSvg="svgPercent" :value="stats.high_risk_rate" label="24h高危率" color="purple" />
    </div>
    <div class="ov-cards" v-else>
      <Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" />
    </div>

    <!-- Timeline Chart -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span>
        <span class="card-title">工具调用趋势</span>
      </div>
      <Skeleton v-if="!loaded" type="chart" />
      <EmptyState v-else-if="!timelineData.length"
        :iconSvg="svgTrend" title="暂无趋势数据" description="Agent 运行后将自动收集工具调用数据"
      />
      <TrendChart v-else
        :data="trendChartData" :lines="trendLines" :xLabels="trendXLabels" :height="170"
        :timeRanges="[{label:'24h',value:'24h'},{label:'7d',value:'7d'}]"
        :currentRange="trendRange" @rangeChange="onTrendRangeChange"
      />
    </div>

    <!-- Top10 + Pie Chart -->
    <div class="ov-row">
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg></span>
          <span class="card-title">工具调用 TOP10</span>
        </div>
        <Skeleton v-if="!loaded" type="text" />
        <EmptyState v-else-if="!topTools.length"
          :iconSvg="svgBar" title="暂无工具调用数据" description="Agent 运行后将自动收集调用统计"
        />
        <div v-else>
          <TransitionGroup name="list-anim" tag="div">
            <div class="hbar-row" v-for="(t, i) in topTools" :key="t.name">
              <span class="hbar-rank">#{{ i + 1 }}</span>
              <span class="hbar-name" :title="t.name">{{ t.name }}</span>
              <div class="hbar-track">
                <div class="hbar-fill hbar-fill-anim" :style="{ '--target-w': Math.max(5, t.pct) + '%', background: getRiskColor(classifyRisk(t.name)) }">{{ t.count }}</div>
              </div>
              <span class="risk-badge" :class="'risk-' + classifyRisk(t.name)" style="margin-left:8px;font-size:10px;">{{ classifyRisk(t.name) }}</span>
            </div>
          </TransitionGroup>
        </div>
      </div>
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.21 15.89A10 10 0 1 1 8 2.83"/><path d="M22 12A10 10 0 0 0 12 2v10z"/></svg></span>
          <span class="card-title">风险等级分布</span>
        </div>
        <Skeleton v-if="!loaded" type="chart" />
        <PieChart v-else :data="pieData" :size="180" />
      </div>
    </div>

    <!-- High Risk Table (with expandable details) -->
    <div class="card" ref="highRiskRef">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg></span>
        <span class="card-title">最近高危调用</span>
      </div>
      <Skeleton v-if="!loaded" type="table" />
      <EmptyState v-else-if="!highRiskRecords.length"
        :iconSvg="svgShieldCheck" title="暂无高危调用" description="系统运行正常，未检测到高危工具调用"
      />
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr><th style="width:30px"></th><th>时间</th><th>工具名</th><th>风险等级</th><th>参数摘要</th><th>标记原因</th></tr>
          </thead>
          <tbody>
            <template v-for="rec in highRiskRecords" :key="rec.id">
              <tr :class="{'row-critical': rec.risk_level === 'critical', 'row-high': rec.risk_level === 'high', 'row-expanded': expandedIds.has(rec.id)}"
                  @click="toggleExpand(rec.id)" style="cursor:pointer">
                <td class="expand-toggle">
                  <svg :class="{'rotated': expandedIds.has(rec.id)}" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
                </td>
                <td>{{ fmtTime(rec.timestamp) }}</td>
                <td><code>{{ rec.tool_name }}</code></td>
                <td><span class="risk-badge" :class="'risk-' + rec.risk_level">{{ rec.risk_level }}</span></td>
                <td class="td-preview" :title="rec.tool_input_preview">{{ rec.tool_input_preview || '--' }}</td>
                <td>{{ rec.flag_reason || '--' }}</td>
              </tr>
              <tr v-if="expandedIds.has(rec.id)" class="detail-row">
                <td colspan="6">
                  <div class="detail-grid">
                    <div class="detail-section">
                      <div class="detail-label">工具输入参数</div>
                      <JsonHighlight :content="rec.tool_input_preview || '(无数据)'" />
                    </div>
                    <div class="detail-section">
                      <div class="detail-label">工具返回结果</div>
                      <JsonHighlight :content="rec.tool_result_preview || '(无数据)'" />
                    </div>
                    <div class="detail-section" v-if="rec.risk_level === 'critical' || rec.flag_reason">
                      <div class="detail-label">风险评估</div>
                      <div class="risk-assessment">
                        <span class="risk-badge" :class="'risk-' + rec.risk_level" style="font-size:11px;">{{ rec.risk_level }}</span>
                        <span v-if="rec.flag_reason">{{ rec.flag_reason }}</span>
                        <span v-if="rec.risk_level === 'critical'" class="risk-desc">此工具可直接执行系统命令，存在远程代码执行（RCE）风险。建议配合安全策略拦截。</span>
                        <span v-else-if="rec.risk_level === 'high'" class="risk-desc">此工具可能修改文件或发送数据，需审查操作内容。</span>
                      </div>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { api } from '../api.js'
import StatCard from '../components/StatCard.vue'
import TrendChart from '../components/TrendChart.vue'
import PieChart from '../components/PieChart.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'
import JsonHighlight from '../components/JsonHighlight.vue'

const svgRobot = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><line x1="12" y1="7" x2="12" y2="11"/><line x1="8" y1="16" x2="8" y2="16.01"/><line x1="16" y1="16" x2="16" y2="16.01"/></svg>'
const svgAlert = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgFlag = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"/><line x1="4" y1="22" x2="4" y2="15"/></svg>'
const svgPercent = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="5" x2="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/></svg>'
const svgTrend = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgBar = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="12" width="4" height="8"/><rect x="10" y="8" width="4" height="12"/><rect x="17" y="4" width="4" height="16"/></svg>'
const svgShieldCheck = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>'

const riskColors = { low: '#6B7280', medium: '#3B82F6', high: '#F59E0B', critical: '#EF4444' }
const criticalTools = new Set(['exec', 'shell', 'bash', 'run_command', 'execute_command'])
const highTools = new Set(['write_file', 'edit_file', 'delete_file', 'write', 'edit', 'http_request', 'curl', 'web_fetch', 'send_email', 'send_message', 'message'])
const mediumTools = new Set(['read_file', 'read', 'list_directory', 'web_search', 'browser'])

function classifyRisk(name) {
  if (criticalTools.has(name)) return 'critical'
  if (highTools.has(name)) return 'high'
  if (mediumTools.has(name)) return 'medium'
  return 'low'
}
function getRiskColor(risk) { return riskColors[risk] || '#6B7280' }

const loaded = ref(false)
const stats = ref({ total: 0, high_risk_count: 0, flagged_count: 0, high_risk_rate: '0%' })
const timelineData = ref([])
const trendRange = ref('24h')
const topTools = ref([])
const pieData = ref([])
const highRiskRecords = ref([])
const expandedIds = ref(new Set())
const highRiskRef = ref(null)

// 安全告警
const alertCount = computed(() => highRiskRecords.value.filter(r => r.flagged || r.risk_level === 'critical').length)
const alertPreview = computed(() => highRiskRecords.value.filter(r => r.flagged || r.risk_level === 'critical').slice(0, 3))

function toggleExpand(id) {
  const s = new Set(expandedIds.value)
  if (s.has(id)) { s.delete(id) } else { s.add(id) }
  expandedIds.value = s
}

function scrollToHighRisk() {
  nextTick(() => {
    highRiskRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  })
}

function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false })
}

const trendChartData = computed(() => {
  return timelineData.value.map(t => ({
    total: t.total || 0, critical: t.critical || 0, high: t.high || 0, medium: t.medium || 0,
  }))
})
const trendLines = [
  { key: 'total', color: '#3B82F6', label: '总调用' },
  { key: 'critical', color: '#EF4444', label: '极危' },
  { key: 'high', color: '#F59E0B', label: '高危' },
  { key: 'medium', color: '#8B5CF6', label: '中危' },
]
const trendXLabels = computed(() => {
  return timelineData.value.map(t => {
    const h = t.hour || ''
    if (trendRange.value === '7d') return h.substring(5, 10) + ' ' + h.substring(11, 13) + ':00'
    const hp = h.substring(11, 13)
    return hp ? hp + ':00' : ''
  })
})

function onTrendRangeChange(range) { trendRange.value = range; loadTimeline() }

async function loadTimeline() {
  try {
    const hours = trendRange.value === '7d' ? 168 : 24
    const d = await api('/api/v1/llm/tools/timeline?hours=' + hours)
    timelineData.value = d.timeline || []
  } catch { timelineData.value = [] }
}

async function loadData() {
  try {
    const d = await api('/api/v1/llm/tools/stats')
    const highRiskCount = (d.by_risk?.high || 0) + (d.by_risk?.critical || 0)
    const total = d.total || 0
    const rate = total > 0 ? ((d.high_risk_24h || 0) / total * 100).toFixed(1) : '0.0'
    stats.value = { total, high_risk_count: highRiskCount, flagged_count: d.flagged_count || 0, high_risk_rate: rate + '%' }

    const byTool = d.by_tool || []
    const maxCount = byTool.length ? byTool[0].count : 1
    topTools.value = byTool.slice(0, 10).map(t => ({
      name: t.name, count: t.count, pct: (t.count / maxCount) * 100,
    }))

    const byRisk = d.by_risk || {}
    pieData.value = [
      { label: 'critical', value: byRisk.critical || 0, color: '#EF4444' },
      { label: 'high', value: byRisk.high || 0, color: '#F59E0B' },
      { label: 'medium', value: byRisk.medium || 0, color: '#3B82F6' },
      { label: 'low', value: byRisk.low || 0, color: '#6B7280' },
    ].filter(d => d.value > 0)
  } catch {
    stats.value = { total: 0, high_risk_count: 0, flagged_count: 0, high_risk_rate: '0%' }
    topTools.value = []; pieData.value = []
  }

  await loadTimeline()

  // 高危调用列表
  try {
    const d = await api('/api/v1/llm/tools?risk_level=critical&limit=10')
    const d2 = await api('/api/v1/llm/tools?risk_level=high&limit=10')
    highRiskRecords.value = [...(d.records || []), ...(d2.records || [])].sort((a, b) => b.id - a.id).slice(0, 20)
  } catch { highRiskRecords.value = [] }

  loaded.value = true
}

let timer = null
onMounted(() => { loadData(); timer = setInterval(loadData, 30000) })
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
/* 安全告警面板 */
.alert-panel {
  background: rgba(239, 68, 68, 0.08);
  border: 1px solid rgba(239, 68, 68, 0.25);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
}
.alert-header { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-3); }
.alert-icon { font-size: 1.2rem; }
.alert-title { font-size: var(--text-base); font-weight: 700; color: #EF4444; }
.alert-items { display: flex; flex-direction: column; gap: 6px; margin-bottom: var(--space-2); }
.alert-item {
  display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm);
  padding: 6px 10px; background: rgba(239, 68, 68, 0.06); border-radius: var(--radius-sm);
}
.alert-item code { background: rgba(239, 68, 68, 0.12); padding: 1px 6px; border-radius: 3px; font-size: var(--text-xs); font-family: var(--font-mono); color: #EF4444; }
.alert-time { color: var(--text-tertiary); font-size: var(--text-xs); }
.alert-reason { color: var(--text-secondary); font-size: var(--text-xs); margin-left: auto; }
.alert-link { font-size: var(--text-sm); color: #EF4444; cursor: pointer; text-decoration: underline; }

/* 展开/收起 */
.expand-toggle { width: 30px; text-align: center; }
.expand-toggle svg { transition: transform .2s ease; color: var(--text-tertiary); }
.expand-toggle svg.rotated { transform: rotate(90deg); }
.row-expanded { background: rgba(99, 102, 241, 0.04) !important; }

.detail-row td { padding: 0 !important; }
.detail-grid { padding: var(--space-3); display: flex; flex-direction: column; gap: var(--space-3); border-top: 1px solid var(--border-subtle); background: var(--bg-base); }
.detail-section { }
.detail-label { font-size: var(--text-xs); font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 4px; }
.risk-assessment { display: flex; flex-direction: column; gap: 4px; font-size: var(--text-sm); color: var(--text-secondary); }
.risk-desc { font-size: var(--text-xs); color: var(--text-tertiary); font-style: italic; }

.risk-badge { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: var(--text-xs); font-weight: 600; text-transform: uppercase; letter-spacing: 0.02em; }
.risk-low { background: rgba(107, 114, 128, 0.15); color: #6B7280; }
.risk-medium { background: rgba(59, 130, 246, 0.15); color: #3B82F6; }
.risk-high { background: rgba(245, 158, 11, 0.15); color: #F59E0B; }
.risk-critical { background: rgba(239, 68, 68, 0.15); color: #EF4444; }
.row-critical { background: rgba(239, 68, 68, 0.06) !important; }
.row-high { background: rgba(245, 158, 11, 0.04) !important; }
.td-preview { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: var(--text-xs); font-family: var(--font-mono); }
code { background: var(--bg-elevated); padding: 2px 6px; border-radius: 4px; font-size: var(--text-xs); font-family: var(--font-mono); }
.hbar-rank { width: 24px; font-size: var(--text-xs); color: var(--color-primary); font-weight: 700; text-align: center; flex-shrink: 0; }
.hbar-fill-anim { width: 0; animation: hbar-grow .8s ease-out forwards; }
@keyframes hbar-grow { from { width: 0; } to { width: var(--target-w); } }
.list-anim-enter-active { animation: list-in .2s ease-out; }
.list-anim-leave-active { animation: list-out .2s ease-in; }
.list-anim-move { transition: transform .2s ease; }
@keyframes list-in { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }
@keyframes list-out { from { opacity: 1; transform: translateY(0); } to { opacity: 0; transform: translateY(10px); } }
</style>
