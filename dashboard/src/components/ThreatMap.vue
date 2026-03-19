<template>
  <div class="threat-map-root" ref="mapRoot">
    <!-- LIVE indicator + clock -->
    <div class="tm-hud-top-left">
      <span class="tm-live-badge"><span class="tm-live-dot"></span>LIVE</span>
      <span class="tm-clock">{{ clock }}</span>
    </div>

    <!-- Stats summary bar -->
    <div class="tm-stats-bar">
      <div class="tm-stat"><span class="tm-stat-label">总请求</span><span class="tm-stat-val">{{ summary.total_requests }}</span></div>
      <div class="tm-stat"><span class="tm-stat-label">拦截</span><span class="tm-stat-val tm-red">{{ summary.blocked }}</span></div>
      <div class="tm-stat"><span class="tm-stat-label">告警</span><span class="tm-stat-val tm-orange">{{ summary.warned }}</span></div>
      <div class="tm-stat"><span class="tm-stat-label">通过</span><span class="tm-stat-val tm-green">{{ summary.passed }}</span></div>
    </div>

    <!-- Main SVG Topology -->
    <svg class="tm-svg" :viewBox="`0 0 ${vw} ${vh}`" preserveAspectRatio="xMidYMid meet">
      <defs>
        <filter id="glow-green" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur in="SourceGraphic" stdDeviation="4" result="blur"/>
          <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>
        </filter>
        <filter id="glow-blue" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur in="SourceGraphic" stdDeviation="6" result="blur"/>
          <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>
        </filter>
        <filter id="glow-red" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur in="SourceGraphic" stdDeviation="5" result="blur"/>
          <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>
        </filter>

        <!-- Connection paths: left to center -->
        <path v-for="s in sourceNodes" :key="'pl-'+s.id"
              :id="'path-l-'+s.id"
              :d="connPath(s.x + s.r, s.y, coreX - 125, coreY)"
              fill="none"/>
        <!-- Connection paths: center to right -->
        <path v-for="t in targetNodes" :key="'pr-'+t.id"
              :id="'path-r-'+t.id"
              :d="connPath(coreX + 125, coreY, t.x - t.r, t.y)"
              fill="none"/>
      </defs>

      <!-- Subtle grid -->
      <line v-for="i in 5" :key="'gl-'+i" :x1="0" :y1="vh*i/6" :x2="vw" :y2="vh*i/6" stroke="rgba(99,102,241,0.04)" stroke-width="1"/>
      <line v-for="i in 7" :key="'gv-'+i" :x1="vw*i/8" :y1="0" :x2="vw*i/8" :y2="vh" stroke="rgba(99,102,241,0.04)" stroke-width="1"/>

      <!-- Connection lines: left to center -->
      <g v-for="s in sourceNodes" :key="'cl-'+s.id">
        <path :d="connPath(s.x + s.r, s.y, coreX - 125, coreY)"
              fill="none" stroke="rgba(99,102,241,0.12)" stroke-width="2" stroke-dasharray="6 4"/>
        <path :d="connPath(s.x + s.r, s.y, coreX - 125, coreY)"
              fill="none" stroke="rgba(99,102,241,0.25)" stroke-width="1.5"
              stroke-dasharray="6 4" class="tm-flow-line"/>
      </g>

      <!-- Connection lines: center to right -->
      <g v-for="t in targetNodes" :key="'cr-'+t.id">
        <path :d="connPath(coreX + 125, coreY, t.x - t.r, t.y)"
              fill="none" stroke="rgba(99,102,241,0.12)" stroke-width="2" stroke-dasharray="6 4"/>
        <path :d="connPath(coreX + 125, coreY, t.x - t.r, t.y)"
              fill="none" stroke="rgba(99,102,241,0.25)" stroke-width="1.5"
              stroke-dasharray="6 4" class="tm-flow-line"/>
      </g>

      <!-- Animated particles -->
      <template v-for="p in particles" :key="p.id">
        <circle v-if="p.side === 'left'" :r="4" :fill="p.color" :opacity="0.9" :filter="p.filter">
          <animateMotion :dur="p.dur + 's'" fill="freeze" repeatCount="1" :begin="p.begin + 's'">
            <mpath :href="'#path-l-' + p.nodeId"/>
          </animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur + 's'" :begin="p.begin + 's'" fill="freeze"/>
        </circle>
        <circle v-if="p.side === 'right'" :r="3.5" :fill="p.color" :opacity="0.9" :filter="p.filter">
          <animateMotion :dur="p.dur + 's'" fill="freeze" repeatCount="1" :begin="p.begin + 's'">
            <mpath :href="'#path-r-' + p.nodeId"/>
          </animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur + 's'" :begin="p.begin + 's'" fill="freeze"/>
        </circle>
        <!-- Blocked explosion ring -->
        <circle v-if="p.action === 'block' && p.side === 'left'" :cx="coreX - 125" :cy="coreY" r="0" fill="none" stroke="#ef4444" stroke-width="2" opacity="0">
          <animate attributeName="r" values="0;28" dur="0.6s" :begin="(p.begin + p.dur) + 's'" fill="freeze"/>
          <animate attributeName="opacity" values="0.8;0" dur="0.6s" :begin="(p.begin + p.dur) + 's'" fill="freeze"/>
          <animate attributeName="stroke-width" values="3;0" dur="0.6s" :begin="(p.begin + p.dur) + 's'" fill="freeze"/>
        </circle>
      </template>

      <!-- Source nodes (left) -->
      <g v-for="s in sourceNodes" :key="'sn-'+s.id" class="tm-node" @click.stop="selectNode('source', s)">
        <circle :cx="s.x" :cy="s.y" :r="s.r + 6" fill="none" :stroke="s.color" stroke-width="1.5" opacity="0.3" class="tm-pulse"/>
        <circle :cx="s.x" :cy="s.y" :r="s.r" fill="rgba(15,15,35,0.9)" :stroke="s.color" stroke-width="2"/>
        <circle :cx="s.x" :cy="s.y" :r="s.r - 3" fill="none" :stroke="s.color" stroke-width="0.5" opacity="0.4"/>
        <text :x="s.x" :y="s.y - 4" text-anchor="middle" :font-size="s.iconSize || 18" class="tm-icon">{{ s.icon }}</text>
        <text :x="s.x" :y="s.y + 16" text-anchor="middle" :font-size="s.labelSize || 10" fill="#94a3b8" font-weight="600">{{ s.label }}</text>
      </g>

      <!-- Core node (center shield) -->
      <g class="tm-core-group" @click.stop="selectNode('core', null)">
        <path :d="shieldPathAt(coreX, coreY, 138, 138)" fill="none" stroke="rgba(99,102,241,0.2)" stroke-width="3" class="tm-shield-pulse"/>
        <path :d="shieldPathAt(coreX, coreY, 130, 130)" fill="rgba(10,10,30,0.95)" stroke="#6366f1" stroke-width="2.5"/>
        <path :d="shieldPathAt(coreX, coreY, 130, 130)" fill="none" stroke="rgba(99,102,241,0.15)" stroke-width="1" stroke-dasharray="4 3"/>

        <text :x="coreX" :y="coreY - 78" text-anchor="middle" font-size="16" fill="#e2e8f0" font-weight="800">🦞 龙虾卫士</text>

        <!-- Engine layers -->
        <g v-for="(layer, i) in engineLayers" :key="'eng-'+i">
          <rect :x="coreX - 82" :y="coreY - 52 + i * 24" width="164" height="20" rx="4"
                :fill="layer.active ? 'rgba(99,102,241,0.12)' : 'rgba(255,255,255,0.03)'"
                :stroke="layer.active ? 'rgba(99,102,241,0.3)' : 'rgba(255,255,255,0.06)'" stroke-width="1"/>
          <text :x="coreX - 72" :y="coreY - 38 + i * 24" font-size="11" :fill="layer.active ? '#a5b4fc' : '#475569'" font-weight="600">{{ layer.label }}</text>
          <text :x="coreX + 72" :y="coreY - 38 + i * 24" font-size="10" :fill="layer.active ? '#818cf8' : '#334155'" text-anchor="end">{{ layer.count }}</text>
        </g>

        <text :x="coreX" :y="coreY + 82" text-anchor="middle" font-size="12" fill="#94a3b8">
          🌡 健康: <tspan :fill="scoreColor(healthScore)" font-weight="700">{{ healthScore }}</tspan>/100
        </text>
        <text :x="coreX" :y="coreY + 100" text-anchor="middle" font-size="11" fill="#64748b">
          🔥 拦截: <tspan fill="#ef4444" font-weight="700">{{ summary.blocked }}</tspan>  ⚠️ 告警: <tspan fill="#f59e0b" font-weight="700">{{ summary.warned }}</tspan>
        </text>
      </g>

      <!-- Target nodes (right) -->
      <g v-for="t in targetNodes" :key="'tn-'+t.id" class="tm-node" @click.stop="selectNode('target', t)">
        <circle :cx="t.x" :cy="t.y" :r="t.r + 5" fill="none" :stroke="t.color" stroke-width="1.5" opacity="0.25" class="tm-pulse"/>
        <circle :cx="t.x" :cy="t.y" :r="t.r" fill="rgba(15,15,35,0.9)" :stroke="t.color" stroke-width="2"/>
        <circle :cx="t.x" :cy="t.y" :r="t.r - 3" fill="none" :stroke="t.color" stroke-width="0.5" opacity="0.4"/>
        <text :x="t.x" :y="t.y - 4" text-anchor="middle" font-size="16" class="tm-icon">{{ t.icon }}</text>
        <text :x="t.x" :y="t.y + 16" text-anchor="middle" font-size="10" fill="#94a3b8" font-weight="600">{{ t.label }}</text>
      </g>

      <!-- Column labels -->
      <text :x="sourceX" :y="28" text-anchor="middle" font-size="12" fill="rgba(99,102,241,0.45)" font-weight="700" letter-spacing="0.15em">消 息 入 口</text>
      <text :x="coreX" :y="28" text-anchor="middle" font-size="12" fill="rgba(99,102,241,0.45)" font-weight="700" letter-spacing="0.15em">检 测 引 擎</text>
      <text :x="targetX" :y="28" text-anchor="middle" font-size="12" fill="rgba(99,102,241,0.45)" font-weight="700" letter-spacing="0.15em">上 游 服 务</text>
    </svg>

    <!-- Detail popover panel -->
    <transition name="tm-panel-fade">
      <div class="tm-detail-panel" v-if="selectedNode" @click.stop>
        <button class="tm-panel-close" @click="selectedNode = null">✕</button>

        <!-- Source detail -->
        <template v-if="selectedNode.type === 'source'">
          <div class="tm-panel-title">{{ selectedNode.data.icon }} {{ selectedNode.data.label }}</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ sourceStats(selectedNode.data.id).requests }}</div><div class="tm-pg-label">请求数</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-red">{{ sourceStats(selectedNode.data.id).blocked }}</div><div class="tm-pg-label">拦截</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-orange">{{ sourceStats(selectedNode.data.id).warned }}</div><div class="tm-pg-label">告警</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ sourceStats(selectedNode.data.id).blockRate }}%</div><div class="tm-pg-label">拦截率</div></div>
          </div>
          <div class="tm-panel-subtitle">最近事件</div>
          <div class="tm-panel-events">
            <div class="tm-pe-row" v-for="e in sourceEvents(selectedNode.data.id)" :key="e.id">
              <span class="tm-pe-time">{{ fmtTime(e.timestamp) }}</span>
              <span class="tm-pe-action" :class="'a-' + e.action">{{ e.action }}</span>
              <span class="tm-pe-desc">{{ trunc(e.reason || e.content_preview || '-', 30) }}</span>
            </div>
            <div v-if="!sourceEvents(selectedNode.data.id).length" class="tm-pe-empty">暂无事件</div>
          </div>
        </template>

        <!-- Core detail -->
        <template v-if="selectedNode.type === 'core'">
          <div class="tm-panel-title">🦞 龙虾卫士 引擎状态</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num" :style="{ color: scoreColor(healthScore) }">{{ healthScore }}</div><div class="tm-pg-label">健康分</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ summary.total_requests }}</div><div class="tm-pg-label">总检测</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-red">{{ summary.blocked }}</div><div class="tm-pg-label">拦截</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ avgLatency }}ms</div><div class="tm-pg-label">平均延迟</div></div>
          </div>
          <div class="tm-panel-subtitle">检测层统计</div>
          <div class="tm-engine-list">
            <div class="tm-eng-row" v-for="(layer, i) in engineLayers" :key="i">
              <span class="tm-eng-name">{{ layer.label }}</span>
              <div class="tm-eng-bar-bg"><div class="tm-eng-bar" :style="{ width: engBarW(layer.count) + '%', background: layer.active ? '#6366f1' : '#334155' }"></div></div>
              <span class="tm-eng-count">{{ layer.count }}</span>
            </div>
          </div>
        </template>

        <!-- Target detail -->
        <template v-if="selectedNode.type === 'target'">
          <div class="tm-panel-title">{{ selectedNode.data.icon }} {{ selectedNode.data.label }}</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num tm-green">● 在线</div><div class="tm-pg-label">状态</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ targetLatency(selectedNode.data.id) }}ms</div><div class="tm-pg-label">延迟</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ targetReqs(selectedNode.data.id) }}</div><div class="tm-pg-label">请求数</div></div>
          </div>
        </template>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, reactive } from 'vue'
import { api } from '../api.js'

// ─── Layout constants ───
const vw = 1200
const vh = 600
const sourceX = 160
const coreX = 600
const coreY = vh / 2
const targetX = 1040

// ─── Clock ───
const clock = ref('')
let clockT = null
function updClock() {
  const n = new Date()
  clock.value = `${n.getFullYear()}-${S(n.getMonth()+1)}-${S(n.getDate())} ${S(n.getHours())}:${S(n.getMinutes())}:${S(n.getSeconds())}`
}
function S(v) { return String(v).padStart(2, '0') }

// ─── Data ───
const summary = reactive({ total_requests: 0, blocked: 0, warned: 0, passed: 0 })
const healthScore = ref(100)
const avgLatency = ref(3)
const auditLogs = ref([])
const particles = ref([])
let particleId = 0
let svgStartTime = 0

// ─── Source nodes ───
const sourceNodes = computed(() => {
  const items = [
    { id: 'lanxin', icon: '📱', label: '蓝信', color: '#6366f1', r: 36, iconSize: 20, labelSize: 12 },
    { id: 'feishu', icon: '💬', label: '飞书', color: '#3b82f6', r: 26, iconSize: 16, labelSize: 10 },
    { id: 'dingtalk', icon: '💬', label: '钉钉', color: '#22c55e', r: 26, iconSize: 16, labelSize: 10 },
    { id: 'wecom', icon: '💬', label: '企微', color: '#f59e0b', r: 26, iconSize: 16, labelSize: 10 },
    { id: 'slack', icon: '💬', label: 'Slack', color: '#a855f7', r: 26, iconSize: 16, labelSize: 10 },
  ]
  const spacing = (vh - 100) / (items.length + 1)
  items.forEach((item, i) => { item.x = sourceX; item.y = 60 + spacing * (i + 1) })
  return items
})

// ─── Target nodes ───
const targetNodes = computed(() => {
  const items = [
    { id: 'openclaw', icon: '🤖', label: 'OpenClaw', color: '#818cf8', r: 30 },
    { id: 'anthropic', icon: '🧠', label: 'Anthropic', color: '#a78bfa', r: 28 },
    { id: 'tools', icon: '🔧', label: 'Tool Services', color: '#22d3ee', r: 26 },
  ]
  const spacing = (vh - 100) / (items.length + 1)
  items.forEach((item, i) => { item.x = targetX; item.y = 60 + spacing * (i + 1) })
  return items
})

// ─── Engine layers ───
const engineLayers = ref([
  { label: 'L1: 模式匹配', count: 0, active: true },
  { label: 'L2: 语义检测', count: 0, active: true },
  { label: 'L3: 行为分析', count: 0, active: true },
  { label: 'L4: 密码学信封', count: 0, active: false },
  { label: 'L5: 自进化引擎', count: 0, active: false },
])

function engBarW(c) {
  const mx = Math.max(...engineLayers.value.map(l => l.count), 1)
  return Math.min((c / mx) * 100, 100)
}

// ─── Shield path generator ───
function shieldPathAt(cx, cy, w, h) {
  return `M${cx} ${cy - h}
    C${cx + w * 0.7} ${cy - h} ${cx + w} ${cy - h * 0.6} ${cx + w} ${cy - h * 0.2}
    L${cx + w} ${cy + h * 0.15}
    C${cx + w} ${cy + h * 0.5} ${cx + w * 0.5} ${cy + h * 0.8} ${cx} ${cy + h}
    C${cx - w * 0.5} ${cy + h * 0.8} ${cx - w} ${cy + h * 0.5} ${cx - w} ${cy + h * 0.15}
    L${cx - w} ${cy - h * 0.2}
    C${cx - w} ${cy - h * 0.6} ${cx - w * 0.7} ${cy - h} ${cx} ${cy - h} Z`
}

// ─── Connection bezier ───
function connPath(x1, y1, x2, y2) {
  const dx = (x2 - x1) * 0.45
  return `M${x1},${y1} C${x1 + dx},${y1} ${x2 - dx},${y2} ${x2},${y2}`
}

// ─── Helpers ───
function scoreColor(s) {
  return s >= 90 ? '#22c55e' : s >= 70 ? '#84cc16' : s >= 50 ? '#f59e0b' : s >= 30 ? '#f97316' : '#ef4444'
}
function fmtTime(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  return `${S(d.getHours())}:${S(d.getMinutes())}:${S(d.getSeconds())}`
}
function trunc(s, n) { return s && s.length > n ? s.slice(0, n) + '…' : (s || '-') }

// ─── Particle generation ───
function spawnParticlesFromEvents(events) {
  if (!events || !events.length) return
  const now = (Date.now() - svgStartTime) / 1000
  const srcIds = sourceNodes.value.map(s => s.id)
  const tgtIds = targetNodes.value.map(t => t.id)
  const newP = []

  events.forEach((ev, i) => {
    const action = ev.action || 'pass'
    const color = action === 'block' ? '#ef4444' : action === 'warn' ? '#f59e0b' : '#22c55e'
    const filter = action === 'block' ? 'url(#glow-red)' : action === 'warn' ? '' : 'url(#glow-green)'
    const srcNode = srcIds[i % srcIds.length]
    const tgtNode = tgtIds[i % tgtIds.length]
    const delay = now + i * 0.5

    newP.push({ id: ++particleId, side: 'left', nodeId: srcNode, color, filter, dur: 2.5, begin: delay, action })

    if (action !== 'block') {
      newP.push({ id: ++particleId, side: 'right', nodeId: tgtNode, color, filter, dur: 2, begin: delay + 2.8, action })
    }
  })

  const cutoff = now - 25
  particles.value = [...particles.value.filter(p => p.begin + p.dur > cutoff), ...newP]
}

function spawnAmbient() {
  const now = (Date.now() - svgStartTime) / 1000
  const srcIds = sourceNodes.value.map(s => s.id)
  const tgtIds = targetNodes.value.map(t => t.id)
  const count = 2 + Math.floor(Math.random() * 3)
  const newP = []

  for (let i = 0; i < count; i++) {
    const si = Math.floor(Math.random() * srcIds.length)
    const ti = Math.floor(Math.random() * tgtIds.length)
    const d = now + Math.random() * 3
    newP.push({ id: ++particleId, side: 'left', nodeId: srcIds[si], color: '#22c55e', filter: 'url(#glow-green)', dur: 2.2 + Math.random() * 0.8, begin: d, action: 'pass' })
    newP.push({ id: ++particleId, side: 'right', nodeId: tgtIds[ti], color: '#22c55e', filter: 'url(#glow-green)', dur: 1.8 + Math.random() * 0.6, begin: d + 2.6, action: 'pass' })
  }

  const cutoff = now - 25
  particles.value = [...particles.value.filter(p => p.begin + p.dur > cutoff), ...newP]
}

// ─── Node detail panel ───
const selectedNode = ref(null)

function selectNode(type, data) {
  if (selectedNode.value && selectedNode.value.type === type && selectedNode.value.data?.id === data?.id) {
    selectedNode.value = null
    return
  }
  selectedNode.value = { type, data }
}

function sourceStats(id) {
  const srcIdx = sourceNodes.value.findIndex(s => s.id === id)
  const events = auditLogs.value.filter((_, i) => (i % sourceNodes.value.length) === srcIdx)
  const blocked = events.filter(e => e.action === 'block').length
  const warned = events.filter(e => e.action === 'warn').length
  const total = events.length
  return {
    requests: total,
    blocked,
    warned,
    blockRate: total > 0 ? ((blocked / total) * 100).toFixed(1) : '0.0',
  }
}

function sourceEvents(id) {
  const srcIdx = sourceNodes.value.findIndex(s => s.id === id)
  return auditLogs.value.filter((_, i) => (i % sourceNodes.value.length) === srcIdx).slice(0, 5)
}

function targetLatency(id) {
  if (id === 'openclaw') return 12
  if (id === 'anthropic') return 85
  return 45
}

function targetReqs(id) {
  const total = summary.total_requests - summary.blocked
  if (id === 'openclaw') return Math.floor(total * 0.6)
  if (id === 'anthropic') return Math.floor(total * 0.3)
  return Math.floor(total * 0.1)
}

// ─── Data fetching ───
let prevLogIds = new Set()

async function fetchData() {
  try {
    const [sumRes, healthRes, logsRes] = await Promise.allSettled([
      api('/api/v1/overview/summary'),
      api('/api/v1/health/score'),
      api('/api/v1/audit/logs?limit=10'),
    ])

    if (sumRes.status === 'fulfilled' && sumRes.value) {
      const d = sumRes.value
      summary.total_requests = d.total_requests || d.total || 0
      summary.blocked = d.blocked_requests || d.blocked || 0
      summary.warned = d.warned_requests || d.warned || 0
      summary.passed = summary.total_requests - summary.blocked - summary.warned
    }

    if (healthRes.status === 'fulfilled' && healthRes.value) {
      const h = healthRes.value
      healthScore.value = h.score || 100
      avgLatency.value = h.avg_latency_ms || h.latency || 3
      // Update engine layer counts from health details if available
      if (h.layer_stats || h.details) {
        const ls = h.layer_stats || h.details
        const layers = engineLayers.value
        if (ls.pattern_match !== undefined) layers[0].count = ls.pattern_match
        if (ls.semantic !== undefined) layers[1].count = ls.semantic
        if (ls.behavior !== undefined) layers[2].count = ls.behavior
        if (ls.envelope !== undefined) {