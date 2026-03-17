<template>
  <div>
    <!-- Stat Cards -->
    <div class="ov-cards" v-if="loaded">
      <StatCard
        :iconSvg="svgGlobe" :value="stats.total" label="总请求" color="blue"
        class="stat-clickable" @click="router.push('/audit')"
      />
      <StatCard
        :iconSvg="svgShieldX" :value="stats.blocked" label="拦截数" color="red"
        class="stat-clickable" @click="router.push('/audit')"
      />
      <StatCard
        :iconSvg="svgAlertTriangle" :value="stats.warned" label="告警数" color="yellow"
        class="stat-clickable" @click="router.push('/audit')"
      />
      <StatCard
        :iconSvg="svgPercent" :value="stats.rate" label="拦截率" color="green"
        class="stat-clickable" @click="router.push('/rules')"
      />
    </div>
    <div class="ov-cards" v-else>
      <Skeleton type="card" />
      <Skeleton type="card" />
      <Skeleton type="card" />
      <Skeleton type="card" />
    </div>

    <!-- Trend + Health -->
    <div class="ov-row">
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span>
          <span class="card-title">请求趋势</span>
        </div>
        <Skeleton v-if="!loaded" type="chart" />
        <EmptyState v-else-if="!trendData.length"
          :iconSvg="svgTrend" title="暂无趋势数据" description="系统运行后将自动收集趋势数据"
        />
        <TrendChart v-else
          :data="trendChartData"
          :lines="trendLines"
          :xLabels="trendXLabels"
          :height="170"
          :timeRanges="[{label:'24h',value:'24h'},{label:'7d',value:'7d'}]"
          :currentRange="trendRange"
          @rangeChange="onTrendRangeChange"
        />
      </div>
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span>
          <span class="card-title">健康状态</span>
        </div>
        <Skeleton v-if="!loaded" type="text" />
        <EmptyState v-else-if="!healthBars.length"
          :iconSvg="svgHeart" title="无健康数据" description="等待系统上报健康信息"
        />
        <div v-else>
          <div class="hb-row" v-for="hb in healthBars" :key="hb.name">
            <span class="hb-label">{{ hb.name }}</span>
            <div class="hb-track"><div class="hb-fill" :style="{ width: Math.max(5, hb.pct) + '%', background: hb.color }"></div></div>
            <span class="hb-val" :style="{ color: hb.color }">{{ hb.val }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Pie + Top Rules -->
    <div class="ov-row">
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.21 15.89A10 10 0 1 1 8 2.83"/><path d="M22 12A10 10 0 0 0 12 2v10z"/></svg></span>
          <span class="card-title">拦截类型分布</span>
        </div>
        <Skeleton v-if="!loaded" type="chart" />
        <PieChart v-else :data="pieData" :size="180" />
      </div>
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg></span>
          <span class="card-title">规则命中 TOP5</span>
        </div>
        <Skeleton v-if="!loaded" type="text" />
        <EmptyState v-else-if="!topRules.length"
          :iconSvg="svgTarget" title="规则正在保护中" description="命中数据将在检测到威胁后显示"
        />
        <div v-else>
          <TransitionGroup name="list-anim" tag="div">
            <div class="hbar-row" v-for="(r, i) in topRules" :key="r.name">
              <span class="hbar-rank">#{{ i + 1 }}</span>
              <span class="hbar-name" :title="r.name">{{ r.name }}</span>
              <div class="hbar-track">
                <div class="hbar-fill hbar-fill-anim" :style="{ '--target-w': Math.max(5, r.pct) + '%', background: barColors[i % barColors.length] }">{{ r.hits }}</div>
              </div>
            </div>
          </TransitionGroup>
        </div>
      </div>
    </div>

    <!-- Heatmap -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg></span>
        <span class="card-title">7 天攻击频率热力图</span>
      </div>
      <Skeleton v-if="!loaded" type="chart" />
      <EmptyState v-else-if="!heatmapData.length"
        :iconSvg="svgGrid" title="暂无热力图数据" description="系统运行 24 小时后将生成攻击频率热力图"
      />
      <HeatMap v-else :data="heatmapData" title="" />
    </div>

    <!-- Recent Attacks -->
    <div class="card">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg></span>
        <span class="card-title">最近攻击事件</span>
      </div>
      <Skeleton v-if="!loaded" type="table" />
      <EmptyState v-else-if="!recentAttacks.length"
        :iconSvg="svgShieldCheck" title="当前环境安全" description="没有检测到攻击事件"
      />
      <div v-else class="table-wrap">
        <table>
          <thead><tr><th>时间</th><th>方向</th><th>发送者</th><th>原因</th></tr></thead>
          <TransitionGroup name="list-anim" tag="tbody">
            <tr v-for="a in recentAttacks" :key="a.id || a.trace_id || a.timestamp" class="row-block" style="cursor:pointer" @click="$router.push('/audit')">
              <td>{{ fmtTime(a.timestamp || a.time) }}</td>
              <td>{{ a.direction === 'inbound' ? '入站' : '出站' }}</td>
              <td>{{ a.sender_id || '--' }}</td>
              <td>{{ a.reason || '--' }}</td>
            </tr>
          </TransitionGroup>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, inject, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import StatCard from '../components/StatCard.vue'
import TrendChart from '../components/TrendChart.vue'
import PieChart from '../components/PieChart.vue'
import HeatMap from '../components/HeatMap.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'

const appState = inject('appState')
const router = useRouter()
const barColors = ['#3B82F6', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6']
const pieColors = ['#EF4444', '#F59E0B', '#3B82F6', '#10B981', '#8B5CF6', '#06B6D4', '#EC4899', '#F97316']

// SVG icons for stat cards
const svgGlobe = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
const svgShieldX = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><line x1="9.5" y1="9.5" x2="14.5" y2="14.5"/><line x1="14.5" y1="9.5" x2="9.5" y2="14.5"/></svg>'
const svgAlertTriangle = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgPercent = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" y1="5" x2="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/></svg>'

// SVG for empty states
const svgTrend = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgHeart = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgPie = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21.21 15.89A10 10 0 1 1 8 2.83"/><path d="M22 12A10 10 0 0 0 12 2v10z"/></svg>'
const svgTarget = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg>'
const svgGrid = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg>'
const svgShieldCheck = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>'

const loaded = ref(false)
const stats = ref({ total: '--', blocked: '--', warned: '--', rate: '--' })
const trendData = ref([])
const trendRange = ref('24h')
const recentAttacks = ref([])
const topRules = ref([])
const pieData = ref([])
const heatmapData = ref([])

function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false })
}

const healthBars = computed(() => {
  const h = appState.health
  if (!h || !h.checks) return []
  const dims = [
    { k: 'database', n: '数据库', fn: c => c.latency_ms != null ? Math.min(100, Math.max(0, 100 - c.latency_ms * 2)) : 50, vfn: c => c.latency_ms != null ? c.latency_ms.toFixed(1) + 'ms' : '--' },
    { k: 'upstream', n: '上游', fn: c => c.total > 0 ? (c.healthy / c.total * 100) : 0, vfn: c => c.healthy != null ? c.healthy + '/' + c.total : '--' },
    { k: 'disk', n: '磁盘', fn: c => c.used_percent != null ? (100 - c.used_percent) : 50, vfn: c => c.used_percent != null ? c.used_percent.toFixed(1) + '%' : '--' },
    { k: 'memory', n: '内存', fn: c => c.alloc_mb != null ? Math.max(0, 100 - c.alloc_mb / 10) : 50, vfn: c => c.alloc_mb != null ? c.alloc_mb.toFixed(1) + ' MB' : '--' },
    { k: 'goroutines', n: 'Goroutine', fn: c => c.count != null ? Math.max(0, 100 - c.count / 10) : 50, vfn: c => c.count != null ? String(c.count) : '' },
  ]
  const result = []
  for (const dm of dims) {
    const c = h.checks[dm.k]
    if (!c) continue
    const pct = dm.fn(c)
    const color = c.status === 'ok' ? 'var(--color-success)' : (c.status === 'warning' ? 'var(--color-warning)' : 'var(--color-danger)')
    result.push({ name: dm.n, pct, color, val: dm.vfn(c) })
  }
  return result
})

const trendChartData = computed(() => {
  return trendData.value.map(t => ({
    total: (t.pass || 0) + (t.block || 0) + (t.warn || 0),
    block: t.block || 0,
    warn: t.warn || 0,
  }))
})

const trendLines = [
  { key: 'total', color: '#3B82F6', label: '总请求' },
  { key: 'block', color: '#EF4444', label: '拦截' },
  { key: 'warn', color: '#F59E0B', label: '告警' },
]

const trendXLabels = computed(() => {
  return trendData.value.map(t => {
    const h = t.hour || ''
    if (trendRange.value === '7d') {
      return h.substring(5, 10) + ' ' + h.substring(11, 13) + ':00'
    }
    const hourPart = h.substring(11, 13)
    return hourPart ? hourPart + ':00' : ''
  })
})

function onTrendRangeChange(range) {
  trendRange.value = range
  loadTrend()
}

async function loadTrend() {
  try {
    const hours = trendRange.value === '7d' ? 168 : 24
    const d = await api('/api/v1/audit/timeline?hours=' + hours)
    trendData.value = d.timeline || []
  } catch { trendData.value = [] }
}

async function loadData() {
  try {
    const d = await api('/api/v1/stats')
    const total = d.total || 0
    const breakdown = d.breakdown || {}
    let blocked = 0, warned = 0
    for (const k of Object.keys(breakdown)) {
      if (k.indexOf('block') >= 0) blocked += breakdown[k]
      if (k.indexOf('warn') >= 0) warned += breakdown[k]
    }
    const rate = total > 0 ? (blocked / total * 100).toFixed(1) : '0.0'
    stats.value = { total, blocked, warned, rate: rate + '%' }
  } catch { /* ignore */ }

  await loadTrend()

  try {
    const d = await api('/api/v1/audit/logs?action=block&limit=5')
    recentAttacks.value = d.logs || []
  } catch { recentAttacks.value = [] }

  try {
    const d = await api('/api/v1/rules/hits')
    let list = Array.isArray(d) ? d : (d.hits || [])
    list.sort((a, b) => (b.hits || 0) - (a.hits || 0))
    const top = list.slice(0, 5)
    const maxH = top.length ? (top[0].hits || 1) : 1
    topRules.value = top.map(r => ({ ...r, pct: (r.hits / maxH) * 100 }))

    const groupMap = {}
    for (const r of list) {
      const g = r.group || 'other'
      if (!groupMap[g]) groupMap[g] = 0
      groupMap[g] += r.hits || 0
    }
    const groups = Object.entries(groupMap).sort((a, b) => b[1] - a[1])
    pieData.value = groups.map(([label, value], i) => ({
      label,
      value,
      color: pieColors[i % pieColors.length],
    }))
  } catch {
    topRules.value = []
    pieData.value = []
  }

  try {
    const d = await api('/api/v1/audit/timeline?hours=168')
    const tl = d.timeline || []
    const matrix = Array.from({ length: 7 }, () => Array(24).fill(0))
    const now = new Date()
    for (const t of tl) {
      if (!t.hour) continue
      const dt = new Date(t.hour)
      if (isNaN(dt.getTime())) continue
      const diffDays = Math.floor((now - dt) / 86400000)
      const dayIdx = 6 - Math.min(6, diffDays)
      const hourIdx = dt.getHours()
      matrix[dayIdx][hourIdx] += (t.block || 0) + (t.warn || 0)
    }
    heatmapData.value = matrix
  } catch { heatmapData.value = [] }

  loaded.value = true
}

let refreshTimer = null
onMounted(() => {
  loadData()
  refreshTimer = setInterval(loadData, 30000)
})
onUnmounted(() => clearInterval(refreshTimer))
</script>

<style scoped>
.stat-clickable { cursor: pointer !important; }
.stat-clickable:hover { transform: translateY(-3px) !important; box-shadow: var(--shadow-lg) !important; border-color: var(--color-primary) !important; }
.hbar-rank {
  width: 24px; font-size: var(--text-xs); color: var(--color-primary); font-weight: 700; text-align: center; flex-shrink: 0;
}
.hbar-fill-anim {
  width: 0;
  animation: hbar-grow .8s ease-out forwards;
}
@keyframes hbar-grow {
  from { width: 0; }
  to { width: var(--target-w); }
}

/* List transition animations */
.list-anim-enter-active {
  animation: list-in .2s ease-out;
}
.list-anim-leave-active {
  animation: list-out .2s ease-in;
}
.list-anim-move {
  transition: transform .2s ease;
}
@keyframes list-in {
  from { opacity: 0; transform: translateY(-10px); }
  to { opacity: 1; transform: translateY(0); }
}
@keyframes list-out {
  from { opacity: 1; transform: translateY(0); }
  to { opacity: 0; transform: translateY(10px); }
}
</style>
