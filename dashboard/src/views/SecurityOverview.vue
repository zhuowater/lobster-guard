<template>
  <div class="so-page">
    <!-- L0: 粒子英雄区 -->
    <div class="so-hero" ref="heroRef">
      <canvas ref="canvas" class="so-canvas"></canvas>
      <!-- 中心评分 -->
      <div class="so-center">
        <svg viewBox="0 0 160 160" class="so-center-svg">
          <circle cx="80" cy="80" r="68" fill="none" stroke="rgba(255,255,255,0.06)" stroke-width="6"/>
          <circle cx="80" cy="80" r="68" fill="none" :stroke="levelColor(avgLevel)" stroke-width="6" stroke-linecap="round"
            :stroke-dasharray="(avgScore/100*427)+' 427'" transform="rotate(-90 80 80)" style="transition:stroke-dasharray 1.2s"/>
          <text x="80" y="72" text-anchor="middle" fill="white" font-size="38" font-weight="800">{{ Math.round(avgScore) }}</text>
          <text x="80" y="92" text-anchor="middle" :fill="levelColor(avgLevel)" font-size="12" font-weight="600">{{ levelLabel(avgLevel) }}</text>
          <text x="80" y="108" text-anchor="middle" fill="rgba(255,255,255,0.4)" font-size="10">全局安全评分</text>
        </svg>
      </div>
      <!-- 统计卡片 -->
      <div class="so-stats">
        <div class="so-stat-card" :class="{ active: filter === 'all' }" @click="setFilter('all')">
          <div class="so-stat-num">{{ profiles.length }}</div>
          <div class="so-stat-label">上游实例</div>
        </div>
        <div class="so-stat-card" :class="{ active: filter === 'critical' }" @click="setFilter('critical')">
          <div class="so-stat-num so-c-critical">{{ countByLevel('critical') + countByLevel('high') }}</div>
          <div class="so-stat-label">🔴 高危实例</div>
        </div>
        <div class="so-stat-card" :class="{ active: filter === 'medium' }" @click="setFilter('medium')">
          <div class="so-stat-num so-c-medium">{{ countByLevel('medium') }}</div>
          <div class="so-stat-label">🟡 中等风险</div>
        </div>
        <div class="so-stat-card" :class="{ active: filter === 'safe' }" @click="setFilter('safe')">
          <div class="so-stat-num so-c-safe">{{ countByLevel('safe') + countByLevel('low') }}</div>
          <div class="so-stat-label">🟢 安全</div>
        </div>
        <div class="so-stat-card">
          <div class="so-stat-num">{{ totalAlerts }}</div>
          <div class="so-stat-label">24h 总告警</div>
        </div>
      </div>
      <!-- 维度环 -->
      <div class="so-dim-rings">
        <div v-for="(dim,i) in avgDimensions" :key="i" class="so-dim-ring" :class="{ active: sortDim === i }" @click="toggleSort(i)">
          <svg viewBox="0 0 52 52">
            <circle cx="26" cy="26" r="22" fill="none" stroke="rgba(255,255,255,0.08)" stroke-width="3"/>
            <circle cx="26" cy="26" r="22" fill="none" :stroke="levelColor(dimLevel(dim.score))" stroke-width="3" stroke-linecap="round"
              :stroke-dasharray="(dim.score/20*138)+' 138'" transform="rotate(-90 26 26)"/>
            <text x="26" y="29" text-anchor="middle" fill="white" font-size="11" font-weight="700">{{ dim.score.toFixed(0) }}</text>
          </svg>
          <div class="so-dim-label">{{ dim.icon }} {{ dim.name }}</div>
        </div>
      </div>
    </div>

    <!-- L1: 排名列表 -->
    <div class="so-rank-section">
      <div class="so-rank-header">
        <h3>🦞 上游安全排名</h3>
        <span class="so-rank-count">{{ filteredProfiles.length }} / {{ profiles.length }} 实例</span>
        <button v-if="filter !== 'all' || sortDim !== -1" class="so-clear-btn" @click="filter='all'; sortDim=-1">清除过滤</button>
      </div>
      <div v-if="loading" class="so-loading">加载中...</div>
      <table v-else-if="filteredProfiles.length > 0" class="so-table">
        <thead>
          <tr>
            <th style="width:40px">#</th>
            <th>实例 ID</th>
            <th style="width:90px" class="so-th-click" @click="toggleSort(-1)">总评分 {{ sortDim===-1?'▼':'' }}</th>
            <th v-for="(d,i) in dimNames" :key="i" style="width:70px" class="so-th-click" @click="toggleSort(i)">{{ d.icon }} {{ sortDim===i?'▼':'' }}</th>
            <th style="width:70px">等级</th>
            <th style="width:60px">告警</th>
            <th style="width:60px"></th>
          </tr>
        </thead>
        <tbody>
          <template v-for="(p,idx) in filteredProfiles" :key="p.upstream_id">
            <tr class="so-row" :class="{ 'so-row-expanded': expandedId === p.upstream_id }" @click="toggleDetail(p.upstream_id)">
              <td class="so-rank-num">{{ idx + 1 }}</td>
              <td class="so-id-cell">
                <span class="so-id-dot" :style="{ background: levelColor(p.risk_level) }"></span>
                {{ p.upstream_id }}
              </td>
              <td>
                <div class="so-score-cell">
                  <svg viewBox="0 0 36 36" class="so-mini-ring">
                    <circle cx="18" cy="18" r="14" fill="none" stroke="rgba(255,255,255,0.08)" stroke-width="3"/>
                    <circle cx="18" cy="18" r="14" fill="none" :stroke="levelColor(p.risk_level)" stroke-width="3" stroke-linecap="round"
                      :stroke-dasharray="(p.security_score/100*88)+' 88'" transform="rotate(-90 18 18)"/>
                    <text x="18" y="21" text-anchor="middle" fill="white" font-size="9" font-weight="700">{{ Math.round(p.security_score) }}</text>
                  </svg>
                </div>
              </td>
              <td v-for="(d,di) in (p.dimensions||[])" :key="di">
                <div class="so-dim-bar-wrap">
                  <div class="so-dim-bar" :style="{ width: (d.score/20*100)+'%', background: levelColor(d.level) }"></div>
                  <span class="so-dim-bar-text">{{ d.score.toFixed(0) }}</span>
                </div>
              </td>
              <td><span class="so-level-tag" :style="{ background: levelColor(p.risk_level)+'22', color: levelColor(p.risk_level), borderColor: levelColor(p.risk_level)+'44' }">{{ levelLabel(p.risk_level) }}</span></td>
              <td class="so-alert-cell">{{ dimAlertTotal(p) }}</td>
              <td><span class="so-detail-btn">{{ expandedId === p.upstream_id ? '收起' : '详情' }}</span></td>
            </tr>
            <!-- L2: 展开详情 -->
            <tr v-if="expandedId === p.upstream_id" class="so-detail-row">
              <td colspan="10">
                <div class="so-detail-body" v-if="detailLoading">加载详情...</div>
                <div class="so-detail-body" v-else-if="detailProfile">
                  <!-- 评分 + 雷达 -->
                  <div class="sp-top">
                    <div class="sp-score-ring">
                      <svg viewBox="0 0 120 120" class="sp-ring-svg">
                        <circle cx="60" cy="60" r="50" fill="none" stroke="rgba(255,255,255,0.08)" stroke-width="8"/>
                        <circle cx="60" cy="60" r="50" fill="none" :stroke="levelColor(detailProfile.risk_level)" stroke-width="8" stroke-linecap="round"
                          :stroke-dasharray="(detailProfile.security_score/100*314)+' 314'" transform="rotate(-90 60 60)" style="transition:stroke-dasharray .8s"/>
                        <text x="60" y="55" text-anchor="middle" fill="white" font-size="28" font-weight="bold">{{ Math.round(detailProfile.security_score) }}</text>
                        <text x="60" y="72" text-anchor="middle" :fill="levelColor(detailProfile.risk_level)" font-size="11">{{ levelLabel(detailProfile.risk_level) }}</text>
                      </svg>
                    </div>
                    <div class="sp-radar">
                      <svg viewBox="0 0 200 200" class="sp-radar-svg">
                        <polygon :points="radarBg(5,100)" fill="none" stroke="rgba(255,255,255,0.1)" stroke-width="1"/>
                        <polygon :points="radarBg(5,70)" fill="none" stroke="rgba(255,255,255,0.06)" stroke-width="1"/>
                        <polygon :points="radarBg(5,40)" fill="none" stroke="rgba(255,255,255,0.04)" stroke-width="1"/>
                        <line v-for="j in 5" :key="'rl'+j" x1="100" y1="100" :x2="radarPt(j-1,5,100).x" :y2="radarPt(j-1,5,100).y" stroke="rgba(255,255,255,0.06)"/>
                        <polygon :points="radarData(detailProfile.dimensions)" fill="rgba(99,102,241,0.25)" stroke="#6366f1" stroke-width="2"/>
                        <circle v-for="(d,j) in detailProfile.dimensions" :key="'rd'+j" :cx="radarPt(j,5,d.score/20*100).x" :cy="radarPt(j,5,d.score/20*100).y" r="3" fill="#6366f1"/>
                        <text v-for="(d,j) in detailProfile.dimensions" :key="'rt'+j" :x="radarPt(j,5,115).x" :y="radarPt(j,5,115).y" text-anchor="middle" fill="rgba(255,255,255,0.7)" font-size="9">{{ d.icon }} {{ d.name }}</text>
                      </svg>
                    </div>
                  </div>
                  <!-- 维度卡片 -->
                  <div class="sp-dims">
                    <div v-for="d in detailProfile.dimensions" :key="d.name" class="sp-dim-card" :style="{ borderLeftColor: levelColor(d.level) }">
                      <div class="sp-dim-head"><span>{{ d.icon }}</span><span class="sp-dim-name">{{ d.name }}</span><span :style="{ color: levelColor(d.level) }">{{ d.score }}/20</span></div>
                      <div class="sp-dim-detail">{{ d.details }}</div>
                      <div v-if="d.alerts>0" class="sp-dim-alerts">⚠️ {{ d.alerts }} 条告警</div>
                    </div>
                  </div>
                  <!-- 流量 -->
                  <div class="sp-traffic">
                    <h4 class="sp-section-title">📊 24h 流量</h4>
                    <div class="sp-traffic-grid">
                      <div class="sp-tcard"><div class="sp-tnum">{{ detailProfile.traffic.total_im_requests }}</div><div class="sp-tlabel">IM</div></div>
                      <div class="sp-tcard"><div class="sp-tnum">{{ detailProfile.traffic.total_llm_calls }}</div><div class="sp-tlabel">LLM</div></div>
                      <div class="sp-tcard"><div class="sp-tnum">{{ detailProfile.traffic.total_tool_calls }}</div><div class="sp-tlabel">工具</div></div>
                      <div class="sp-tcard sp-tcard-block"><div class="sp-tnum">{{ detailProfile.traffic.blocked_requests }}</div><div class="sp-tlabel">拦截</div></div>
                      <div class="sp-tcard sp-tcard-warn"><div class="sp-tnum">{{ detailProfile.traffic.warned_requests }}</div><div class="sp-tlabel">告警</div></div>
                      <div class="sp-tcard sp-tcard-review"><div class="sp-tnum">{{ detailProfile.traffic.reviewed_requests }}</div><div class="sp-tlabel">复核</div></div>
                    </div>
                  </div>
                  <!-- 引擎告警网格 -->
                  <div class="sp-engines">
                    <h4 class="sp-section-title">🦞 引擎告警 (24h)</h4>
                    <div class="sp-engine-grid">
                      <div v-for="ea in detailEngineCards" :key="ea.key" class="sp-engine-card" :class="{ 'sp-engine-hot': ea.count>0&&!ea.positive, 'sp-engine-good': ea.count>0&&ea.positive }">
                        <div class="sp-engine-icon">{{ ea.icon }}</div>
                        <div class="sp-engine-count" :class="{ 'sp-engine-count-hot': ea.count>0&&!ea.positive, 'sp-engine-count-good': ea.count>0&&ea.positive }">{{ ea.count }}</div>
                        <div class="sp-engine-label">{{ ea.label }}</div>
                      </div>
                    </div>
                  </div>
                  <!-- Top 风险事件 -->
                  <div v-if="detailProfile.top_risk_events?.length" class="sp-risk-events">
                    <h4 class="sp-section-title">🔥 Top 风险事件</h4>
                    <div v-for="(ev,j) in detailProfile.top_risk_events" :key="j" class="sp-risk-row">
                      <span class="sp-risk-sev">{{ ev.severity==='high'?'🔴':'🟡' }}</span>
                      <span class="sp-risk-time">{{ ev.timestamp?.slice(11,19) }}</span>
                      <span class="sp-risk-engine">{{ ev.engine }}</span>
                      <span class="sp-risk-summary">{{ ev.summary?.slice(0,80) }}</span>
                    </div>
                  </div>
                  <!-- 7天趋势 -->
                  <div v-if="detailProfile.trend?.length" class="sp-trend">
                    <h4 class="sp-section-title">📈 7 天趋势</h4>
                    <svg viewBox="0 0 400 120" class="sp-trend-svg">
                      <line v-for="j in 5" :key="'tg'+j" x1="40" :y1="10+j*20" x2="390" :y2="10+j*20" stroke="rgba(255,255,255,0.05)"/>
                      <text v-for="(d,j) in detailProfile.trend" :key="'td'+j" :x="40+j*50" y="118" text-anchor="middle" fill="rgba(255,255,255,0.4)" font-size="9">{{ d.date?.slice(5) }}</text>
                      <polyline :points="trendLine(detailProfile.trend)" fill="none" stroke="#6366f1" stroke-width="2" stroke-linejoin="round"/>
                      <circle v-for="(d,j) in detailProfile.trend" :key="'tc'+j" :cx="40+j*50" :cy="110-d.security_score" r="3" fill="#6366f1"/>
                    </svg>
                  </div>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
      <div v-else class="so-empty">暂无{{ filter !== 'all' ? '匹配的' : '' }}上游实例</div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { api } from '../api.js'

// === State ===
const profiles = ref([])
const loading = ref(true)
const filter = ref('all')
const sortDim = ref(-1) // -1 = total score, 0-4 = dimension index
const expandedId = ref(null)
const detailProfile = ref(null)
const detailLoading = ref(false)
const canvas = ref(null)
const heroRef = ref(null)

const dimNames = [
  { name: '入站', icon: '🛡️' },
  { name: 'LLM', icon: '🤖' },
  { name: '数据', icon: '🔒' },
  { name: '行为', icon: '📋' },
  { name: '工具', icon: '🔧' },
]

// === Computed ===
const avgScore = computed(() => {
  if (!profiles.value.length) return 0
  return profiles.value.reduce((s, p) => s + p.security_score, 0) / profiles.value.length
})
const avgLevel = computed(() => {
  const s = avgScore.value
  if (s >= 90) return 'safe'
  if (s >= 70) return 'low'
  if (s >= 50) return 'medium'
  if (s >= 30) return 'high'
  return 'critical'
})
const avgDimensions = computed(() => {
  if (!profiles.value.length) return dimNames.map((d, i) => ({ ...d, score: 0 }))
  return dimNames.map((d, i) => {
    const sum = profiles.value.reduce((s, p) => s + ((p.dimensions && p.dimensions[i]) ? p.dimensions[i].score : 0), 0)
    return { ...d, score: sum / profiles.value.length }
  })
})
const totalAlerts = computed(() => profiles.value.reduce((s, p) => s + dimAlertTotal(p), 0))

const filteredProfiles = computed(() => {
  let list = [...profiles.value]
  if (filter.value === 'critical') list = list.filter(p => p.risk_level === 'critical' || p.risk_level === 'high')
  else if (filter.value === 'medium') list = list.filter(p => p.risk_level === 'medium')
  else if (filter.value === 'safe') list = list.filter(p => p.risk_level === 'safe' || p.risk_level === 'low')
  // Sort
  if (sortDim.value === -1) {
    list.sort((a, b) => a.security_score - b.security_score) // lowest first (most dangerous)
  } else {
    const di = sortDim.value
    list.sort((a, b) => {
      const sa = a.dimensions && a.dimensions[di] ? a.dimensions[di].score : 0
      const sb = b.dimensions && b.dimensions[di] ? b.dimensions[di].score : 0
      return sa - sb
    })
  }
  return list
})

const detailEngineCards = computed(() => {
  const a = detailProfile.value?.engine_alerts || {}
  return [
    { key:'inbound', icon:'🛡️', label:'入站检测', count: a.inbound_detections||0 },
    { key:'chains', icon:'⛓️', label:'攻击链', count: a.attack_chains||0 },
    { key:'llm', icon:'🤖', label:'LLM规则', count: a.llm_rule_hits||0 },
    { key:'singularity', icon:'🔮', label:'蜜罐暴露', count: a.singularity_exposes||0 },
    { key:'honeypot', icon:'🍯', label:'蜜罐深度', count: a.honeypot_deep||0 },
    { key:'ifc', icon:'🔒', label:'IFC违规', count: a.ifc_violations||0 },
    { key:'hidden', icon:'🙈', label:'IFC隐藏', count: a.ifc_hidden||0, positive: true },
    { key:'taint', icon:'☣️', label:'污染追踪', count: a.taint_events||0 },
    { key:'reversal', icon:'✅', label:'污染逆转', count: a.taint_reversals||0, positive: true },
    { key:'outbound', icon:'🚫', label:'出站拦截', count: a.outbound_blocks||0 },
    { key:'plan', icon:'📋', label:'计划偏离', count: a.plan_deviations||0 },
    { key:'cap', icon:'🔑', label:'能力拒绝', count: a.capability_denials||0 },
    { key:'anomaly', icon:'📊', label:'行为异常', count: a.behavior_anomalies||0 },
    { key:'envelope', icon:'📜', label:'信封失败', count: a.envelope_failures||0 },
    { key:'cf', icon:'🔄', label:'反事实', count: a.counterfactual_flags||0 },
    { key:'evolution', icon:'🧬', label:'进化规则', count: a.evolution_rules||0, positive: true },
  ]
})

// === Helpers ===
function levelColor(l) { return { safe:'#22c55e', low:'#6366f1', medium:'#eab308', high:'#f97316', critical:'#ef4444' }[l] || '#6366f1' }
function levelLabel(l) { return { safe:'安全', low:'良好', medium:'中等', high:'较高', critical:'高危' }[l] || l }
function dimLevel(s) { if(s>=18) return 'safe'; if(s>=14) return 'low'; if(s>=10) return 'medium'; if(s>=6) return 'high'; return 'critical' }
function countByLevel(l) { return profiles.value.filter(p => p.risk_level === l).length }
function dimAlertTotal(p) { return (p.dimensions||[]).reduce((s, d) => s + (d.alerts||0), 0) }
function setFilter(f) { filter.value = filter.value === f ? 'all' : f }
function toggleSort(i) { sortDim.value = sortDim.value === i ? -1 : i }
function radarPt(i, n, r) { const a = (Math.PI*2*i/n)-Math.PI/2; return { x: Math.round(100+r*0.8*Math.cos(a)), y: Math.round(100+r*0.8*Math.sin(a)) } }
function radarBg(n, r) { return Array.from({length:n},(_,i)=>{ const p=radarPt(i,n,r); return p.x+','+p.y }).join(' ') }
function radarData(dims) { if(!dims) return ''; return dims.map((d,i)=>{ const p=radarPt(i,dims.length,d.score/20*100); return p.x+','+p.y }).join(' ') }
function trendLine(trend) { if(!trend) return ''; return trend.map((d,i)=>(40+i*50)+','+(110-d.security_score)).join(' ') }

async function toggleDetail(id) {
  if (expandedId.value === id) { expandedId.value = null; detailProfile.value = null; return }
  expandedId.value = id; detailLoading.value = true; detailProfile.value = null
  try { detailProfile.value = await api(`/api/v1/upstreams/${encodeURIComponent(id)}/security-profile`) }
  catch(e) { /* ignore */ }
  finally { detailLoading.value = false }
}

// === Particle System ===
let animId = null
let particles = []
let mouseX = -999, mouseY = -999
let hoveredParticle = null

function initParticles() {
  const c = canvas.value; if (!c) return
  const w = c.width = c.offsetWidth * (window.devicePixelRatio || 1)
  const h = c.height = c.offsetHeight * (window.devicePixelRatio || 1)
  const ctx = c.getContext('2d')
  ctx.scale(window.devicePixelRatio || 1, window.devicePixelRatio || 1)
  const dw = c.offsetWidth, dh = c.offsetHeight
  particles = []

  // Background particles
  const bgCount = Math.min(80, Math.floor(dw * dh / 6000))
  for (let i = 0; i < bgCount; i++) {
    particles.push({
      x: Math.random() * dw, y: Math.random() * dh,
      vx: (Math.random() - 0.5) * 0.3, vy: (Math.random() - 0.5) * 0.3,
      r: Math.random() * 1.5 + 0.5, alpha: Math.random() * 0.3 + 0.1,
      color: '#6366f1', isUp: false, data: null
    })
  }
  // Upstream particles
  profiles.value.forEach((p, i) => {
    const angle = (Math.PI * 2 * i / Math.max(profiles.value.length, 1)) - Math.PI / 2
    const rx = dw * 0.3, ry = dh * 0.32
    particles.push({
      x: dw / 2 + rx * Math.cos(angle), y: dh / 2 + ry * Math.sin(angle),
      vx: (Math.random() - 0.5) * 0.15, vy: (Math.random() - 0.5) * 0.15,
      r: 4 + (p.security_score / 100) * 5,
      alpha: 0.6 + (p.security_score / 100) * 0.4,
      color: levelColor(p.risk_level), isUp: true, data: p
    })
  })

  function draw() {
    const ctx2 = c.getContext('2d')
    ctx2.setTransform(window.devicePixelRatio || 1, 0, 0, window.devicePixelRatio || 1, 0, 0)
    ctx2.clearRect(0, 0, dw, dh)
    hoveredParticle = null

    // Connections
    ctx2.lineWidth = 0.5
    for (let i = 0; i < particles.length; i++) {
      for (let j = i + 1; j < particles.length; j++) {
        const dx = particles[i].x - particles[j].x, dy = particles[i].y - particles[j].y
        const d = Math.sqrt(dx * dx + dy * dy)
        const maxD = (particles[i].isUp || particles[j].isUp) ? 140 : 90
        if (d < maxD) {
          const a = 0.12 * (1 - d / maxD)
          ctx2.strokeStyle = `rgba(99,102,241,${a})`
          ctx2.beginPath(); ctx2.moveTo(particles[i].x, particles[i].y); ctx2.lineTo(particles[j].x, particles[j].y); ctx2.stroke()
        }
      }
    }

    // Particles
    particles.forEach(p => {
      p.x += p.vx; p.y += p.vy
      if (p.x < 0 || p.x > dw) p.vx *= -1
      if (p.y < 0 || p.y > dh) p.vy *= -1

      ctx2.globalAlpha = p.alpha
      if (p.isUp) {
        // Glow
        const grad = ctx2.createRadialGradient(p.x, p.y, 0, p.x, p.y, p.r * 3)
        grad.addColorStop(0, p.color + '44')
        grad.addColorStop(1, 'transparent')
        ctx2.fillStyle = grad
        ctx2.beginPath(); ctx2.arc(p.x, p.y, p.r * 3, 0, Math.PI * 2); ctx2.fill()
      }
      ctx2.fillStyle = p.color
      ctx2.beginPath(); ctx2.arc(p.x, p.y, p.r, 0, Math.PI * 2); ctx2.fill()

      // Hover detection
      if (p.isUp) {
        const dx = mouseX - p.x, dy = mouseY - p.y
        if (Math.sqrt(dx*dx+dy*dy) < p.r + 10) {
          hoveredParticle = p
        }
      }
    })
    ctx2.globalAlpha = 1

    // Tooltip
    if (hoveredParticle && hoveredParticle.data) {
      const pd = hoveredParticle.data
      const tx = hoveredParticle.x + 15, ty = hoveredParticle.y - 10
      ctx2.fillStyle = 'rgba(15,23,42,0.9)'
      ctx2.strokeStyle = hoveredParticle.color
      ctx2.lineWidth = 1
      const text = `${pd.upstream_id}  ${Math.round(pd.security_score)} 分`
      ctx2.font = '12px system-ui'
      const tw = ctx2.measureText(text).width + 16
      ctx2.beginPath()
      ctx2.roundRect(tx, ty - 16, tw, 28, 6)
      ctx2.fill(); ctx2.stroke()
      ctx2.fillStyle = '#fff'
      ctx2.fillText(text, tx + 8, ty + 4)
    }

    // Center glow
    const cx = dw / 2, cy = dh / 2
    const cg = ctx2.createRadialGradient(cx, cy, 0, cx, cy, 60)
    cg.addColorStop(0, levelColor(avgLevel.value) + '18')
    cg.addColorStop(1, 'transparent')
    ctx2.fillStyle = cg
    ctx2.beginPath(); ctx2.arc(cx, cy, 60, 0, Math.PI * 2); ctx2.fill()

    animId = requestAnimationFrame(draw)
  }
  draw()
}

function onMouseMove(e) {
  const c = canvas.value; if (!c) return
  const rect = c.getBoundingClientRect()
  mouseX = e.clientX - rect.left; mouseY = e.clientY - rect.top
  c.style.cursor = hoveredParticle ? 'pointer' : 'default'
}
function onMouseClick(e) {
  if (hoveredParticle && hoveredParticle.data) {
    toggleDetail(hoveredParticle.data.upstream_id)
    // Scroll to table
    nextTick(() => {
      document.querySelector('.so-rank-section')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })
  }
}
function onResize() {
  if (animId) cancelAnimationFrame(animId)
  initParticles()
}

onMounted(async () => {
  try {
    const data = await api('/api/v1/upstream-profiles')
    profiles.value = data.profiles || []
  } catch(e) { /* ignore */ }
  loading.value = false
  await nextTick()
  initParticles()
  canvas.value?.addEventListener('mousemove', onMouseMove)
  canvas.value?.addEventListener('click', onMouseClick)
  window.addEventListener('resize', onResize)
})
onUnmounted(() => {
  if (animId) cancelAnimationFrame(animId)
  canvas.value?.removeEventListener('mousemove', onMouseMove)
  canvas.value?.removeEventListener('click', onMouseClick)
  window.removeEventListener('resize', onResize)
})
</script>

<style scoped>
.so-page { padding: 0; }
/* === L0 Hero === */
.so-hero { position: relative; min-height: 380px; background: #0f172a; border-radius: 12px; overflow: hidden; margin-bottom: 20px; }
.so-canvas { position: absolute; inset: 0; width: 100%; height: 100%; }
.so-center { position: absolute; top: 50%; left: 50%; transform: translate(-50%,-50%); z-index: 2; pointer-events: none; }
.so-center-svg { width: 160px; height: 160px; filter: drop-shadow(0 0 20px rgba(99,102,241,0.3)); }
.so-stats { position: absolute; bottom: 16px; left: 50%; transform: translateX(-50%); z-index: 2; display: flex; gap: 10px; }
.so-stat-card { background: rgba(15,23,42,0.65); backdrop-filter: blur(12px); border: 1px solid rgba(255,255,255,0.08); border-radius: 10px; padding: 10px 16px; text-align: center; cursor: pointer; transition: all .2s; min-width: 90px; }
.so-stat-card:hover, .so-stat-card.active { border-color: #6366f1; background: rgba(99,102,241,0.12); }
.so-stat-num { font-size: 22px; font-weight: 800; color: rgba(255,255,255,0.9); }
.so-stat-label { font-size: 10px; color: rgba(255,255,255,0.45); margin-top: 2px; }
.so-c-critical { color: #ef4444; }
.so-c-medium { color: #eab308; }
.so-c-safe { color: #22c55e; }
.so-dim-rings { position: absolute; top: 14px; right: 16px; z-index: 2; display: flex; gap: 8px; }
.so-dim-ring { text-align: center; cursor: pointer; opacity: 0.7; transition: all .2s; }
.so-dim-ring:hover, .so-dim-ring.active { opacity: 1; transform: scale(1.1); }
.so-dim-ring svg { width: 48px; height: 48px; }
.so-dim-label { font-size: 9px; color: rgba(255,255,255,0.5); margin-top: 2px; }

/* === L1 Rank === */
.so-rank-section { background: var(--bg-surface, #1e293b); border: 1px solid var(--border-subtle, #334155); border-radius: 12px; padding: 16px 20px; }
.so-rank-header { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; }
.so-rank-header h3 { font-size: 15px; font-weight: 700; color: rgba(255,255,255,0.9); margin: 0; }
.so-rank-count { font-size: 12px; color: rgba(255,255,255,0.4); }
.so-clear-btn { background: rgba(99,102,241,0.15); color: #a5b4fc; border: 1px solid rgba(99,102,241,0.3); border-radius: 6px; padding: 3px 10px; font-size: 11px; cursor: pointer; }
.so-table { width: 100%; border-collapse: collapse; }
.so-table th { font-size: 11px; color: rgba(255,255,255,0.45); font-weight: 600; text-align: left; padding: 8px 6px; border-bottom: 1px solid rgba(255,255,255,0.06); }
.so-th-click { cursor: pointer; }
.so-th-click:hover { color: #a5b4fc; }
.so-row { cursor: pointer; transition: background .15s; }
.so-row:hover { background: rgba(99,102,241,0.06); }
.so-row-expanded { background: rgba(99,102,241,0.08); }
.so-row td { padding: 8px 6px; font-size: 12px; color: rgba(255,255,255,0.8); border-bottom: 1px solid rgba(255,255,255,0.04); vertical-align: middle; }
.so-rank-num { font-weight: 800; color: rgba(255,255,255,0.3); font-size: 14px; }
.so-id-cell { display: flex; align-items: center; gap: 6px; font-weight: 600; }
.so-id-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.so-score-cell { display: flex; align-items: center; }
.so-mini-ring { width: 36px; height: 36px; }
.so-dim-bar-wrap { position: relative; height: 6px; background: rgba(255,255,255,0.06); border-radius: 3px; min-width: 40px; }
.so-dim-bar { height: 100%; border-radius: 3px; transition: width .3s; }
.so-dim-bar-text { position: absolute; right: -20px; top: -3px; font-size: 10px; color: rgba(255,255,255,0.5); }
.so-level-tag { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; border: 1px solid; }
.so-alert-cell { font-weight: 600; }
.so-detail-btn { color: #a5b4fc; font-size: 11px; }
.so-loading, .so-empty { text-align: center; color: rgba(255,255,255,0.4); padding: 40px; }

/* === L2 Detail === */
.so-detail-row td { padding: 0 !important; border-bottom: 2px solid rgba(99,102,241,0.15); }
.so-detail-body { padding: 16px 12px; background: rgba(99,102,241,0.03); }

/* Reuse security profile styles from GatewayMonitor */
.sp-top { display:flex; gap:24px; align-items:center; margin-bottom:16px; flex-wrap:wrap; }
.sp-score-ring { flex:0 0 120px; }
.sp-ring-svg { width:120px; height:120px; }
.sp-radar { flex:1; min-width:180px; max-width:260px; }
.sp-radar-svg { width:100%; }
.sp-dims { display:grid; grid-template-columns:repeat(auto-fill,minmax(190px,1fr)); gap:8px; margin-bottom:14px; }
.sp-dim-card { background:rgba(255,255,255,0.03); border:1px solid rgba(255,255,255,0.08); border-left:3px solid; border-radius:8px; padding:8px 10px; }
.sp-dim-head { display:flex; align-items:center; gap:5px; margin-bottom:3px; font-weight:600; font-size:12px; color:rgba(255,255,255,0.9); }
.sp-dim-name { flex:1; }
.sp-dim-detail { font-size:10px; color:rgba(255,255,255,0.5); }
.sp-dim-alerts { font-size:10px; color:#f97316; margin-top:3px; }
.sp-section-title { font-size:12px; font-weight:600; color:rgba(255,255,255,0.8); margin:0 0 8px; }
.sp-traffic { margin-bottom:14px; }
.sp-traffic-grid { display:grid; grid-template-columns:repeat(6,1fr); gap:6px; }
.sp-tcard { background:rgba(255,255,255,0.03); border:1px solid rgba(255,255,255,0.08); border-radius:6px; padding:8px; text-align:center; }
.sp-tnum { font-size:18px; font-weight:700; color:rgba(255,255,255,0.9); }
.sp-tlabel { font-size:9px; color:rgba(255,255,255,0.5); }
.sp-tcard-block { border-color:rgba(239,68,68,0.3); } .sp-tcard-block .sp-tnum { color:#ef4444; }
.sp-tcard-warn { border-color:rgba(234,179,8,0.3); } .sp-tcard-warn .sp-tnum { color:#eab308; }
.sp-tcard-review { border-color:rgba(168,85,247,0.3); } .sp-tcard-review .sp-tnum { color:#a855f7; }
.sp-engines { margin-bottom:14px; }
.sp-engine-grid { display:grid; grid-template-columns:repeat(4,1fr); gap:6px; }
.sp-engine-card { background:rgba(255,255,255,0.02); border:1px solid rgba(255,255,255,0.06); border-radius:6px; padding:6px; text-align:center; }
.sp-engine-hot { border-color:rgba(234,179,8,0.3); background:rgba(234,179,8,0.05); }
.sp-engine-good { border-color:rgba(34,197,94,0.3); background:rgba(34,197,94,0.05); }
.sp-engine-icon { font-size:16px; }
.sp-engine-count { font-size:16px; font-weight:700; color:rgba(255,255,255,0.4); }
.sp-engine-count-hot { color:#eab308; }
.sp-engine-count-good { color:#22c55e; }
.sp-engine-label { font-size:8px; color:rgba(255,255,255,0.4); }
.sp-risk-events { margin-bottom:14px; }
.sp-risk-row { display:flex; align-items:center; gap:6px; padding:4px 6px; border-bottom:1px solid rgba(255,255,255,0.04); font-size:11px; }
.sp-risk-sev { font-size:11px; }
.sp-risk-time { color:rgba(255,255,255,0.4); font-family:monospace; font-size:10px; min-width:55px; }
.sp-risk-engine { background:rgba(99,102,241,0.15); color:#a5b4fc; padding:1px 5px; border-radius:4px; font-size:9px; }
.sp-risk-summary { color:rgba(255,255,255,0.7); flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.sp-trend { margin-bottom:8px; }
.sp-trend-svg { width:100%; background:rgba(255,255,255,0.02); border-radius:6px; }

@media (max-width:768px) {
  .so-stats { flex-wrap: wrap; gap: 6px; }
  .so-stat-card { min-width: 70px; padding: 6px 10px; }
  .so-dim-rings { position: static; justify-content: center; margin-top: 8px; }
  .sp-traffic-grid { grid-template-columns: repeat(3,1fr); }
  .sp-engine-grid { grid-template-columns: repeat(3,1fr); }
}
</style>
