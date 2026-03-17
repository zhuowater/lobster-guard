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
        <div class="card-header"><span class="card-icon">📈</span><span class="card-title">24h 请求趋势</span></div>
        <div v-if="!trendData.length" class="empty"><div class="empty-icon">📈</div>暂无趋势数据</div>
        <div v-else v-html="trendSvg"></div>
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

    <!-- Attacks + Top Rules -->
    <div class="ov-row">
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
      <div class="card">
        <div class="card-header"><span class="card-icon">🎯</span><span class="card-title">规则命中 TOP5</span></div>
        <div v-if="!topRules.length" class="empty"><div class="empty-icon">🎯</div>暂无命中数据</div>
        <div v-else>
          <div class="hbar-row" v-for="(r, i) in topRules" :key="r.name">
            <span class="hbar-name" :title="r.name">{{ r.name }}</span>
            <div class="hbar-track">
              <div class="hbar-fill" :style="{ width: Math.max(5, r.pct) + '%', background: barColors[i % barColors.length] }">{{ r.hits }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, inject, onMounted, onUnmounted } from 'vue'
import { api } from '../api.js'
import StatCard from '../components/StatCard.vue'

const appState = inject('appState')
const barColors = ['#00d4ff', '#00ff88', '#ffcc00', '#ff4466', '#9b59b6']

const stats = ref({ total: '--', blocked: '--', warned: '--', rate: '--' })
const trendData = ref([])
const recentAttacks = ref([])
const topRules = ref([])

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

const trendSvg = computed(() => {
  const tl = trendData.value
  if (!tl.length) return ''
  const W = 600, H = 140, PX = 40, PY = 10, GW = W - PX * 2, GH = H - PY * 2
  let maxV = 1
  for (const t of tl) { const s = (t.pass || 0) + (t.block || 0) + (t.warn || 0); if (s > maxV) maxV = s }

  let svg = `<svg viewBox="0 0 ${W} ${H}" style="width:100%;height:160px" xmlns="http://www.w3.org/2000/svg">`
  // Grid
  for (let g = 0; g <= 4; g++) {
    const gy = PY + GH * g / 4
    svg += `<line x1="${PX}" y1="${gy}" x2="${W - PX}" y2="${gy}" stroke="rgba(255,255,255,0.06)" stroke-width="0.5"/>`
    svg += `<text x="${PX - 4}" y="${gy + 4}" fill="#8892b0" font-size="8" text-anchor="end">${Math.round(maxV * (4 - g) / 4)}</text>`
  }
  // X labels
  const step = Math.max(1, Math.floor(tl.length / 6))
  for (let i = 0; i < tl.length; i += step) {
    const x = PX + (i / (tl.length - 1 || 1)) * GW
    const hr = tl[i].hour ? tl[i].hour.substring(11, 13) : ''
    svg += `<text x="${x}" y="${H - 2}" fill="#8892b0" font-size="8" text-anchor="middle">${hr}h</text>`
  }
  // Total line
  const totalPts = tl.map((t, i) => {
    const tot = (t.pass || 0) + (t.block || 0) + (t.warn || 0)
    const x = PX + (i / (tl.length - 1 || 1)) * GW
    const y = PY + GH - (tot / maxV) * GH
    return `${x.toFixed(1)},${y.toFixed(1)}`
  }).join(' ')
  svg += `<polyline points="${totalPts}" fill="none" stroke="#00d4ff" stroke-width="2" stroke-linejoin="round" opacity="0.8"/>`
  // Block line
  const blockPts = tl.map((t, i) => {
    const x = PX + (i / (tl.length - 1 || 1)) * GW
    const y = PY + GH - ((t.block || 0) / maxV) * GH
    return `${x.toFixed(1)},${y.toFixed(1)}`
  }).join(' ')
  svg += `<polyline points="${blockPts}" fill="none" stroke="#ff4466" stroke-width="1.5" stroke-linejoin="round" opacity="0.7"/>`
  svg += '</svg>'
  svg += '<div style="display:flex;gap:12px;margin-top:4px;font-size:.65rem;color:var(--text-dim)"><span><span style="display:inline-block;width:8px;height:8px;background:var(--neon-blue);border-radius:2px;margin-right:2px"></span>总请求</span><span><span style="display:inline-block;width:8px;height:8px;background:var(--neon-red);border-radius:2px;margin-right:2px"></span>拦截</span></div>'
  return svg
})

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

  try {
    const d = await api('/api/v1/audit/timeline?hours=24')
    trendData.value = d.timeline || []
  } catch { trendData.value = [] }

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
  } catch { topRules.value = [] }
}

let refreshTimer = null
onMounted(() => {
  loadData()
  refreshTimer = setInterval(loadData, 30000)
})
onUnmounted(() => clearInterval(refreshTimer))
</script>
