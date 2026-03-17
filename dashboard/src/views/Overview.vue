<template>
  <div>
    <!-- Stat Cards -->
    <div class="ov-cards">
      <StatCard icon="📨" :value="stats.total" label="总请求" color="blue" />
      <StatCard icon="🛡️" :value="stats.blocked" label="拦截数" color="red" />
      <StatCard icon="⚠️" :value="stats.warned" label="告警数" color="yellow" />
      <StatCard icon="📊" :value="stats.rate" label="拦截率" color="green" />
    </div>

    <!-- Trend + Health -->
    <div class="ov-row">
      <div class="card">
        <div class="card-header"><span class="card-icon">📈</span><span class="card-title">请求趋势</span></div>
        <div v-if="!trendData.length" class="empty"><div class="empty-icon">📈</div>暂无趋势数据</div>
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
        <div class="card-header"><span class="card-icon">🏥</span><span class="card-title">健康状态</span></div>
        <div v-if="!healthBars.length" class="empty">无健康数据</div>
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
        <div class="card-header"><span class="card-icon">🧩</span><span class="card-title">拦截类型分布</span></div>
        <div v-if="!pieData.length" class="empty"><div class="empty-icon">🧩</div>暂无拦截数据</div>
        <PieChart v-else :data="pieData" :size="180" />
      </div>
      <div class="card">
        <div class="card-header"><span class="card-icon">🎯</span><span class="card-title">规则命中 TOP5</span></div>
        <div v-if="!topRules.length" class="empty"><div class="empty-icon">🎯</div>暂无命中数据</div>
        <div v-else>
          <div class="hbar-row" v-for="(r, i) in topRules" :key="r.name">
            <span class="hbar-rank">#{{ i + 1 }}</span>
            <span class="hbar-name" :title="r.name">{{ r.name }}</span>
            <div class="hbar-track">
              <div class="hbar-fill hbar-fill-anim" :style="{ '--target-w': Math.max(5, r.pct) + '%', background: barColors[i % barColors.length] }">{{ r.hits }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Heatmap -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header"><span class="card-icon">🔥</span><span class="card-title">7 天攻击频率热力图</span></div>
      <div v-if="!heatmapData.length" class="empty"><div class="empty-icon">🔥</div>暂无热力图数据</div>
      <HeatMap v-else :data="heatmapData" title="" />
    </div>

    <!-- Recent Attacks -->
    <div class="card">
      <div class="card-header"><span class="card-icon">🚨</span><span class="card-title">最近攻击事件</span></div>
      <div v-if="!recentAttacks.length" class="empty"><div class="empty-icon">✅</div>暂无攻击事件<div class="empty-hint">系统安全运行中</div></div>
      <div v-else class="table-wrap">
        <table>
          <thead><tr><th>时间</th><th>方向</th><th>发送者</th><th>原因</th></tr></thead>
          <tbody>
            <tr v-for="a in recentAttacks" :key="a.id" class="row-block" style="cursor:pointer" @click="$router.push('/audit')">
              <td>{{ fmtTime(a.timestamp || a.time) }}</td>
              <td>{{ a.direction === 'inbound' ? '入站' : '出站' }}</td>
              <td>{{ a.sender_id || '--' }}</td>
              <td>{{ a.reason || '--' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, inject, onMounted, onUnmounted } from 'vue'
import { api } from '../api.js'
import StatCard from '../components/StatCard.vue'
import TrendChart from '../components/TrendChart.vue'
import PieChart from '../components/PieChart.vue'
import HeatMap from '../components/HeatMap.vue'

const appState = inject('appState')
const barColors = ['#00d4ff', '#00ff88', '#ffcc00', '#ff4466', '#9b59b6']
const pieColors = ['#ff4466', '#ffa94d', '#00d4ff', '#00ff88', '#9b59b6', '#74c0fc', '#e599f7', '#ffcc00']

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
    const color = c.status === 'ok' ? 'var(--neon-green)' : (c.status === 'warning' ? 'var(--neon-yellow)' : 'var(--neon-red)')
    result.push({ name: dm.n, pct, color, val: dm.vfn(c) })
  }
  return result
})

// Trend chart data
const trendChartData = computed(() => {
  return trendData.value.map(t => ({
    total: (t.pass || 0) + (t.block || 0) + (t.warn || 0),
    block: t.block || 0,
    warn: t.warn || 0,
  }))
})

const trendLines = [
  { key: 'total', color: '#00d4ff', label: '总请求' },
  { key: 'block', color: '#ff4466', label: '拦截' },
  { key: 'warn', color: '#ffcc00', label: '告警' },
]

const trendXLabels = computed(() => {
  return trendData.value.map(t => {
    const h = t.hour || ''
    if (trendRange.value === '7d') {
      // Show date for 7d
      return h.substring(5, 10) + '\n' + h.substring(11, 13) + 'h'
    }
    return h.substring(11, 13) + 'h'
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

    // Build pie data from groups
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

  // Load heatmap data (7d × 24h)
  try {
    const d = await api('/api/v1/audit/timeline?hours=168')
    const tl = d.timeline || []
    // Aggregate into 7×24 matrix
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
}

let refreshTimer = null
onMounted(() => {
  loadData()
  refreshTimer = setInterval(loadData, 30000)
})
onUnmounted(() => clearInterval(refreshTimer))
</script>

<style scoped>
.hbar-rank {
  width: 24px; font-size: .72rem; color: var(--neon-blue); font-weight: 700; text-align: center; flex-shrink: 0;
}
.hbar-fill-anim {
  width: 0;
  animation: hbar-grow .8s ease-out forwards;
}
@keyframes hbar-grow {
  from { width: 0; }
  to { width: var(--target-w); }
}
</style>
