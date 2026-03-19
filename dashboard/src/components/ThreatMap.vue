<template>
  <div class="threat-map-root" @click="selectedNode = null">
    <div class="tm-hud-top-left">
      <span class="tm-live-badge"><span class="tm-live-dot"></span>LIVE</span>
      <span class="tm-clock">{{ clock }}</span>
    </div>
    <div class="tm-stats-bar">
      <div class="tm-stat"><span class="tm-stat-label">总请求</span><span class="tm-stat-val">{{ summary.total_requests }}</span></div>
      <div class="tm-stat"><span class="tm-stat-label">拦截</span><span class="tm-stat-val tm-red">{{ summary.blocked }}</span></div>
      <div class="tm-stat"><span class="tm-stat-label">告警</span><span class="tm-stat-val tm-orange">{{ summary.warned }}</span></div>
      <div class="tm-stat"><span class="tm-stat-label">通过</span><span class="tm-stat-val tm-green">{{ summary.passed }}</span></div>
    </div>
    <svg class="tm-svg" :viewBox="'0 0 '+vw+' '+vh" preserveAspectRatio="xMidYMid meet">
      <defs>
        <filter id="glow-green" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur in="SourceGraphic" stdDeviation="4" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter>
        <filter id="glow-red" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur in="SourceGraphic" stdDeviation="5" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter>
        <path v-for="s in sourceNodes" :key="'pl-'+s.id" :id="'path-l-'+s.id" :d="bezier(s.x+s.r, s.y, coreX-125, coreY)" fill="none"/>
        <path v-for="t in targetNodes" :key="'pr-'+t.id" :id="'path-r-'+t.id" :d="bezier(coreX+125, coreY, t.x-t.r, t.y)" fill="none"/>
      </defs>
      <line v-for="i in 5" :key="'gh'+i" :x1="0" :y1="vh*i/6" :x2="vw" :y2="vh*i/6" stroke="rgba(99,102,241,0.04)" stroke-width="1"/>
      <line v-for="i in 7" :key="'gv'+i" :x1="vw*i/8" :y1="0" :x2="vw*i/8" :y2="vh" stroke="rgba(99,102,241,0.04)" stroke-width="1"/>
      <g v-for="s in sourceNodes" :key="'cl-'+s.id">
        <path :d="bezier(s.x+s.r, s.y, coreX-125, coreY)" fill="none" stroke="rgba(99,102,241,0.12)" stroke-width="2" stroke-dasharray="6 4"/>
        <path :d="bezier(s.x+s.r, s.y, coreX-125, coreY)" fill="none" stroke="rgba(99,102,241,0.25)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      </g>
      <g v-for="t in targetNodes" :key="'cr-'+t.id">
        <path :d="bezier(coreX+125, coreY, t.x-t.r, t.y)" fill="none" stroke="rgba(99,102,241,0.12)" stroke-width="2" stroke-dasharray="6 4"/>
        <path :d="bezier(coreX+125, coreY, t.x-t.r, t.y)" fill="none" stroke="rgba(99,102,241,0.25)" stroke-width="1.5" stroke-dasharray="6 4" class="tm-flow-line"/>
      </g>
      <template v-for="p in particles" :key="p.id">
        <circle v-if="p.side==='left'" :r="4" :fill="p.color" opacity="0" :filter="p.filter">
          <animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath :href="'#path-l-'+p.nodeId"/></animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.side==='right'" :r="3.5" :fill="p.color" opacity="0" :filter="p.filter">
          <animateMotion :dur="p.dur+'s'" fill="freeze" repeatCount="1" :begin="p.begin+'s'"><mpath :href="'#path-r-'+p.nodeId"/></animateMotion>
          <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;0.1;0.85;1" :dur="p.dur+'s'" :begin="p.begin+'s'" fill="freeze"/>
        </circle>
        <circle v-if="p.action==='block'&&p.side==='left'" :cx="coreX-125" :cy="coreY" r="0" fill="none" stroke="#ef4444" stroke-width="2" opacity="0">
          <animate attributeName="r" values="0;28" dur="0.6s" :begin="(p.begin+p.dur)+'s'" fill="freeze"/>
          <animate attributeName="opacity" values="0.8;0" dur="0.6s" :begin="(p.begin+p.dur)+'s'" fill="freeze"/>
        </circle>
      </template>
      <g v-for="s in sourceNodes" :key="'sn-'+s.id" class="tm-node" @click.stop="selectNode('source',s)">
        <circle :cx="s.x" :cy="s.y" :r="s.r+6" fill="none" :stroke="s.color" stroke-width="1.5" opacity="0.3" class="tm-pulse"/>
        <circle :cx="s.x" :cy="s.y" :r="s.r" fill="rgba(15,15,35,0.9)" :stroke="s.color" stroke-width="2"/>
        <circle :cx="s.x" :cy="s.y" :r="s.r-3" fill="none" :stroke="s.color" stroke-width="0.5" opacity="0.4"/>
        <text :x="s.x" :y="s.y-4" text-anchor="middle" :font-size="s.iconSize||18" class="tm-icon">{{ s.icon }}</text>
        <text :x="s.x" :y="s.y+16" text-anchor="middle" :font-size="s.labelSize||10" fill="#94a3b8" font-weight="600">{{ s.label }}</text>
      </g>
      <g class="tm-core-group" @click.stop="selectNode('core',null)">
        <path :d="shieldAt(138,138)" fill="none" stroke="rgba(99,102,241,0.2)" stroke-width="3" class="tm-shield-pulse"/>
        <path :d="shieldAt(130,130)" fill="rgba(10,10,30,0.95)" stroke="#6366f1" stroke-width="2.5"/>
        <path :d="shieldAt(130,130)" fill="none" stroke="rgba(99,102,241,0.15)" stroke-width="1" stroke-dasharray="4 3"/>
        <text :x="coreX" :y="coreY-78" text-anchor="middle" font-size="16" fill="#e2e8f0" font-weight="800">🦞 龙虾卫士</text>
        <g v-for="(layer,i) in engineLayers" :key="'eng-'+i">
          <rect :x="coreX-82" :y="coreY-52+i*24" width="164" height="20" rx="4" :fill="layer.active?'rgba(99,102,241,0.12)':'rgba(255,255,255,0.03)'" :stroke="layer.active?'rgba(99,102,241,0.3)':'rgba(255,255,255,0.06)'" stroke-width="1"/>
          <text :x="coreX-72" :y="coreY-38+i*24" font-size="11" :fill="layer.active?'#a5b4fc':'#475569'" font-weight="600">{{ layer.label }}</text>
          <text :x="coreX+72" :y="coreY-38+i*24" font-size="10" :fill="layer.active?'#818cf8':'#334155'" text-anchor="end">{{ layer.count }}</text>
        </g>
        <text :x="coreX" :y="coreY+82" text-anchor="middle" font-size="12" fill="#94a3b8">🌡 健康: <tspan :fill="sc(healthScore)" font-weight="700">{{ healthScore }}</tspan>/100</text>
        <text :x="coreX" :y="coreY+100" text-anchor="middle" font-size="11" fill="#64748b">🔥 拦截: <tspan fill="#ef4444" font-weight="700">{{ summary.blocked }}</tspan>  ⚠️ 告警: <tspan fill="#f59e0b" font-weight="700">{{ summary.warned }}</tspan></text>
      </g>
      <g v-for="t in targetNodes" :key="'tn-'+t.id" class="tm-node" @click.stop="selectNode('target',t)">
        <circle :cx="t.x" :cy="t.y" :r="t.r+5" fill="none" :stroke="t.color" stroke-width="1.5" opacity="0.25" class="tm-pulse"/>
        <circle :cx="t.x" :cy="t.y" :r="t.r" fill="rgba(15,15,35,0.9)" :stroke="t.color" stroke-width="2"/>
        <circle :cx="t.x" :cy="t.y" :r="t.r-3" fill="none" :stroke="t.color" stroke-width="0.5" opacity="0.4"/>
        <text :x="t.x" :y="t.y-4" text-anchor="middle" font-size="16" class="tm-icon">{{ t.icon }}</text>
        <text :x="t.x" :y="t.y+16" text-anchor="middle" font-size="10" fill="#94a3b8" font-weight="600">{{ t.label }}</text>
      </g>
      <text :x="sourceX" :y="28" text-anchor="middle" font-size="12" fill="rgba(99,102,241,0.45)" font-weight="700" letter-spacing="0.15em">消 息 入 口</text>
      <text :x="coreX" :y="28" text-anchor="middle" font-size="12" fill="rgba(99,102,241,0.45)" font-weight="700" letter-spacing="0.15em">检 测 引 擎</text>
      <text :x="targetX" :y="28" text-anchor="middle" font-size="12" fill="rgba(99,102,241,0.45)" font-weight="700" letter-spacing="0.15em">上 游 服 务</text>
    </svg>
    <transition name="tm-panel-fade">
      <div class="tm-detail-panel" v-if="selectedNode" @click.stop>
        <button class="tm-panel-close" @click="selectedNode=null">✕</button>
        <template v-if="selectedNode.type==='source'">
          <div class="tm-panel-title">{{ selectedNode.data.icon }} {{ selectedNode.data.label }}</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ srcStat(selectedNode.data.id).requests }}</div><div class="tm-pg-label">请求数</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-red">{{ srcStat(selectedNode.data.id).blocked }}</div><div class="tm-pg-label">拦截</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-orange">{{ srcStat(selectedNode.data.id).warned }}</div><div class="tm-pg-label">告警</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ srcStat(selectedNode.data.id).blockRate }}%</div><div class="tm-pg-label">拦截率</div></div>
          </div>
          <div class="tm-panel-subtitle">最近事件</div>
          <div class="tm-panel-events">
            <div class="tm-pe-row" v-for="e in srcEvents(selectedNode.data.id)" :key="e.id"><span class="tm-pe-time">{{ fmtT(e.timestamp) }}</span><span class="tm-pe-action" :class="'a-'+e.action">{{ e.action }}</span><span class="tm-pe-desc">{{ trn(e.reason||e.content_preview||'-',30) }}</span></div>
            <div v-if="!srcEvents(selectedNode.data.id).length" class="tm-pe-empty">暂无事件</div>
          </div>
        </template>
        <template v-if="selectedNode.type==='core'">
          <div class="tm-panel-title">🦞 龙虾卫士 引擎状态</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num" :style="{color:sc(healthScore)}">{{ healthScore }}</div><div class="tm-pg-label">健康分</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ summary.total_requests }}</div><div class="tm-pg-label">总检测</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num tm-red">{{ summary.blocked }}</div><div class="tm-pg-label">拦截</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ avgLatency }}ms</div><div class="tm-pg-label">平均延迟</div></div>
          </div>
          <div class="tm-panel-subtitle">检测层统计</div>
          <div class="tm-engine-list">
            <div class="tm-eng-row" v-for="(l,i) in engineLayers" :key="i"><span class="tm-eng-name">{{ l.label }}</span><div class="tm-eng-bar-bg"><div class="tm-eng-bar" :style="{width:engBarW(l.count)+'%',background:l.active?'#6366f1':'#334155'}"></div></div><span class="tm-eng-count">{{ l.count }}</span></div>
          </div>
        </template>
        <template v-if="selectedNode.type==='target'">
          <div class="tm-panel-title">{{ selectedNode.data.icon }} {{ selectedNode.data.label }}</div>
          <div class="tm-panel-grid">
            <div class="tm-pg-cell"><div class="tm-pg-num tm-green">● 在线</div><div class="tm-pg-label">状态</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ tgtLatency(selectedNode.data.id) }}ms</div><div class="tm-pg-label">延迟</div></div>
            <div class="tm-pg-cell"><div class="tm-pg-num">{{ tgtReqs(selectedNode.data.id) }}</div><div class="tm-pg-label">请求数</div></div>
          </div>
        </template>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, reactive } from 'vue'
import { api } from '../api.js'

const vw = 1200, vh = 600, sourceX = 160, coreX = 600, coreY = vh / 2, targetX = 1040
const clock = ref('')
let clockT = null
function updClock() { const n = new Date(); clock.value = n.getFullYear()+'-'+P(n.getMonth()+1)+'-'+P(n.getDate())+' '+P(n.getHours())+':'+P(n.getMinutes())+':'+P(n.getSeconds()) }
function P(v) { return String(v).padStart(2, '0') }

const summary = reactive({ total_requests: 0, blocked: 0, warned: 0, passed: 0 })
const healthScore = ref(100)
const avgLatency = ref(3)
const auditLogs = ref([])
const particles = ref([])
let pid = 0, svgT0 = 0

const sourceNodes = computed(() => {
  const items = [
    { id: 'lanxin', icon: '📱', label: '蓝信', color: '#6366f1', r: 36, iconSize: 20, labelSize: 12 },
    { id: 'feishu', icon: '💬', label: '飞书', color: '#3b82f6', r: 26 },
    { id: 'dingtalk', icon: '💬', label: '钉钉', color: '#22c55e', r: 26 },
    { id: 'wecom', icon: '💬', label: '企微', color: '#f59e0b', r: 26 },
    { id: 'slack', icon: '💬', label: 'Slack', color: '#a855f7', r: 26 },
  ]
  const sp = (vh - 100) / (items.length + 1)
  items.forEach((it, i) => { it.x = sourceX; it.y = 60 + sp * (i + 1) })
  return items
})

const targetNodes = computed(() => {
  const items = [
    { id: 'openclaw', icon: '🤖', label: 'OpenClaw', color: '#818cf8', r: 30 },
    { id: 'anthropic', icon: '🧠', label: 'Anthropic', color: '#a78bfa', r: 28 },
    { id: 'tools', icon: '🔧', label: 'Tool Services', color: '#22d3ee', r: 26 },
  ]
  const sp = (vh - 100) / (items.length + 1)
  items.forEach((it, i) => { it.x = targetX; it.y = 60 + sp * (i + 1) })
  return items
})

const engineLayers = ref([
  { label: 'L1: 模式匹配', count: 0, active: true },
  { label: 'L2: 语义检测', count: 0, active: true },
  { label: 'L3: 行为分析', count: 0, active: true },
  { label: 'L4: 密码学信封', count: 0, active: false },
  { label: 'L5: 自进化引擎', count: 0, active: false },
])

function engBarW(c) { const mx = Math.max(...engineLayers.value.map(l => l.count), 1); return Math.min((c / mx) * 100, 100) }

function shieldAt(w, h) {
  const cx = coreX, cy = coreY
  return 'M'+cx+' '+(cy-h)+' C'+(cx+w*0.7)+' '+(cy-h)+' '+(cx+w)+' '+(cy-h*0.6)+' '+(cx+w)+' '+(cy-h*0.2)+' L'+(cx+w)+' '+(cy+h*0.15)+' C'+(cx+w)+' '+(cy+h*0.5)+' '+(cx+w*0.5)+' '+(cy+h*0.8)+' '+cx+' '+(cy+h)+' C'+(cx-w*0.5)+' '+(cy+h*0.8)+' '+(cx-w)+' '+(cy+h*0.5)+' '+(cx-w)+' '+(cy+h*0.15)+' L'+(cx-w)+' '+(cy-h*0.2)+' C'+(cx-w)+' '+(cy-h*0.6)+' '+(cx-w*0.7)+' '+(cy-h)+' '+cx+' '+(cy-h)+' Z'
}

function bezier(x1, y1, x2, y2) { const dx = (x2-x1)*0.45; return 'M'+x1+','+y1+' C'+(x1+dx)+','+y1+' '+(x2-dx)+','+y2+' '+x2+','+y2 }
function sc(s) { return s>=90?'#22c55e':s>=70?'#84cc16':s>=50?'#f59e0b':s>=30?'#f97316':'#ef4444' }
function fmtT(ts) { if(!ts)return''; const d=new Date(ts); return P(d.getHours())+':'+P(d.getMinutes())+':'+P(d.getSeconds()) }
function trn(s, n) { return s&&s.length>n?s.slice(0,n)+'…':(s||'-') }

function spawnFromEvents(events) {
  if (!events||!events.length) return
  const now = (Date.now()-svgT0)/1000, sIds = sourceNodes.value.map(s=>s.id), tIds = targetNodes.value.map(t=>t.id), np = []
  events.forEach((ev, i) => {
    const act = ev.action||'pass', col = act==='block'?'#ef4444':act==='warn'?'#f59e0b':'#22c55e', flt = act==='block'?'url(#glow-red)':act==='warn'?'':'url(#glow-green)'
    const si = sIds[i%sIds.length], ti = tIds[i%tIds.length], d = now+i*0.5
    np.push({ id:++pid, side:'left', nodeId:si, color:col, filter:flt, dur:2.5, begin:d, action:act })
    if (act!=='block') np.push({ id:++pid, side:'right', nodeId:ti, color:col, filter:flt, dur:2, begin:d+2.8, action:act })
  })
  const cut = now-25
  particles.value = [...particles.value.filter(p=>p.begin+p.dur>cut), ...np]
}

function spawnAmbient() {
  const now=(Date.now()-svgT0)/1000, sIds=sourceNodes.value.map(s=>s.id), tIds=targetNodes.value.map(t=>t.id), np=[], cnt=2+Math.floor(Math.random()*3)
  for(let i=0;i<cnt;i++){
    const si=sIds[Math.floor(Math.random()*sIds.length)],ti=tIds[Math.floor(Math.random()*tIds.length)],d=now+Math.random()*3
    np.push({id:++pid,side:'left',nodeId:si,color:'#22c55e',filter:'url(#glow-green)',dur:2.2+Math.random()*0.8,begin:d,action:'pass'})
    np.push({id:++pid,side:'right',nodeId:ti,color:'#22c55e',filter:'url(#glow-green)',dur:1.8+Math.random()*0.6,begin:d+2.6,action:'pass'})
  }
  const cut=now-25
  particles.value=[...particles.value.filter(p=>p.begin+p.dur>cut),...np]
}

const selectedNode = ref(null)
function selectNode(type, data) {
  if (selectedNode.value&&selectedNode.value.type===type&&selectedNode.value.data?.id===data?.id){selectedNode.value=null;return}
  selectedNode.value={type,data}
}

function srcStat(id) {
  const idx=sourceNodes.value.findIndex(s=>s.id===id), evts=auditLogs.value.filter((_,i)=>(i%sourceNodes.value.length)===idx)
  const b=evts.filter(e=>e.action==='block').length, w=evts.filter(e=>e.action==='warn').length, t=evts.length
  return {requests:t,blocked:b,warned:w,blockRate:t>0?((b/t)*100).toFixed(1):'0.0'}
}
function srcEvents(id) { const idx=sourceNodes.value.findIndex(s=>s.id===id); return auditLogs.value.filter((_,i)=>(i%sourceNodes.value.length)===idx).slice(0,5) }
function tgtLatency(id) { return id==='openclaw'?12:id==='anthropic'?85:45 }
function tgtReqs(id) { const t=Math.max(summary.total_requests-summary.blocked,0); return id==='openclaw'?Math.floor(t*0.6):id==='anthropic'?Math.floor(t*0.3):Math.floor(t*0.1) }

let prevLogIds = new Set()
async function fetchData() {
  try {
    const [sumR,hR,lR] = await Promise.allSettled([api('/api/v1/overview/summary'),api('/api/v1/health/score'),api('/api/v1/audit/logs?limit=10')])
    if(sumR.status==='fulfilled'&&sumR.value){const d=sumR.value;summary.total_requests=d.total_requests||d.total||0;summary.blocked=d.blocked_requests||d.blocked||0;summary.warned=d.warned_requests||d.warned||0;summary.passed=Math.max(summary.total_requests-summary.blocked-summary.warned,0)}
    if(hR.status==='fulfilled'&&hR.value){const h=hR.value;healthScore.value=h.score||100;avgLatency.value=h.avg_latency_ms||h.latency||3
      if(h.layer_stats||h.details){const ls=h.layer_stats||h.details;const ly=engineLayers.value
        if(ls.pattern_match!==undefined)ly[0].count=ls.pattern_match;if(ls.semantic!==undefined)ly[1].count=ls.semantic
        if(ls.behavior!==undefined)ly[2].count=ls.behavior;if(ls.envelope!==undefined){ly[3].count=ls.envelope;ly[3].active=ls.envelope>0}
        if(ls.evolution!==undefined){ly[4].count=ls.evolution;ly[4].active=ls.evolution>0}}}
    if(lR.status==='fulfilled'&&lR.value){const logs=Array.isArray(lR.value)?lR.value:(lR.value.logs||lR.value.items||[])
      auditLogs.value=logs
      const newLogs=logs.filter(l=>!prevLogIds.has(l.id))
      if(newLogs.length>0){spawnFromEvents(newLogs);prevLogIds=new Set(logs.map(l=>l.id))}}
  } catch(e){console.error('[ThreatMap] fetch error',e)}
}

let dataT=null, ambientT=null
onMounted(()=>{
  svgT0=Date.now();updClock();clockT=setInterval(updClock,1000)
  fetchData();dataT=setInterval(fetchData,5000)
  spawnAmbient();ambientT=setInterval(spawnAmbient,4000)
})
onUnmounted(()=>{clearInterval(clockT);clearInterval(dataT);clearInterval(ambientT)})
</script>

<style scoped>
.threat-map-root{position:absolute;inset:0;background:#050510;overflow:hidden;display:flex;flex-direction:column;align-items:center;justify-content:center;font-family:'Inter',-apple-system,BlinkMacSystemFont,sans-serif;color:#e2e8f0}
.tm-hud-top-left{position:absolute;top:12px;left:20px;display:flex;align-items:center;gap:14px;z-index:10}
.tm-live-badge{display:flex;align-items:center;gap:6px;background:rgba(239,68,68,0.15);border:1px solid rgba(239,68,68,0.3);border-radius:20px;padding:3px 12px 3px 8px;font-size:11px;font-weight:800;color:#f87171;letter-spacing:0.08em}
.tm-live-dot{width:8px;height:8px;border-radius:50%;background:#ef4444;animation:tm-blink 1.2s ease-in-out infinite}
@keyframes tm-blink{0%,100%{opacity:1}50%{opacity:0.3}}
.tm-clock{font-family:'JetBrains Mono',monospace;font-size:12px;color:#64748b;letter-spacing:0.05em}
.tm-stats-bar{position:absolute;bottom:16px;left:50%;transform:translateX(-50%);display:flex;gap:24px;z-index:10;background:rgba(10,10,25,0.7);backdrop-filter:blur(10px);-webkit-backdrop-filter:blur(10px);border:1px solid rgba(99,102,241,0.15);border-radius:12px;padding:8px 28px}
.tm-stat{display:flex;flex-direction:column;align-items:center;gap:2px}
.tm-stat-label{font-size:10px;color:#475569;font-weight:500}
.tm-stat-val{font-size:18px;font-weight:800;color:#818cf8;font-family:'JetBrains Mono',monospace}
.tm-stat-val.tm-red{color:#ef4444}.tm-stat-val.tm-orange{color:#f59e0b}.tm-stat-val.tm-green{color:#22c55e}
.tm-svg{width:92%;max-height:80vh;flex-shrink:0}
.tm-flow-line{animation:tm-dash 1.5s linear infinite}
@keyframes tm-dash{to{stroke-dashoffset:-20px}}
.tm-node{cursor:pointer;transition:transform 0.2s}.tm-node:hover{filter:brightness(1.3)}
.tm-icon{pointer-events:none}
.tm-pulse{animation:tm-pulse-ring 2.5s ease-in-out infinite}
@keyframes tm-pulse-ring{0%{r:inherit;opacity:0.3}50%{opacity:0.1}100%{opacity:0.3}}
.tm-core-group{cursor:pointer}.tm-core-group:hover path{filter:brightness(1.15)}
.tm-shield-pulse{animation:tm-shield-glow 3s ease-in-out infinite}
@keyframes tm-shield-glow{0%,100%{stroke:rgba(99,102,241,0.2);stroke-width:3}50%{stroke:rgba(99,102,241,0.4);stroke-width:4}}
.tm-detail-panel{position:absolute;right:20px;top:50%;transform:translateY(-50%);width:280px;background:rgba(10,10,30,0.85);backdrop-filter:blur(16px);-webkit-backdrop-filter:blur(16px);border:1px solid rgba(99,102,241,0.25);border-radius:12px;padding:20px;z-index:20;box-shadow:0 8px 32px rgba(0,0,0,0.5)}
.tm-panel-close{position:absolute;top:8px;right:10px;background:none;border:none;color:#64748b;font-size:14px;cursor:pointer;padding:4px 8px;border-radius:4px}.tm-panel-close:hover{background:rgba(255,255,255,0.1);color:#e2e8f0}
.tm-panel-title{font-size:15px;font-weight:700;margin-bottom:14px;color:#e2e8f0}
.tm-panel-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-bottom:14px}
.tm-pg-cell{text-align:center;padding:8px 4px;background:rgba(255,255,255,0.03);border-radius:8px;border:1px solid rgba(255,255,255,0.06)}
.tm-pg-num{font-size:18px;font-weight:800;color:#818cf8;font-family:'JetBrains Mono',monospace}
.tm-pg-num.tm-red{color:#ef4444}.tm-pg-num.tm-orange{color:#f59e0b}.tm-pg-num.tm-green{color:#22c55e}
.tm-pg-label{font-size:10px;color:#475569;margin-top:2px}
.tm-panel-subtitle{font-size:11px;color:#64748b;font-weight:600;margin-bottom:8px;text-transform:uppercase;letter-spacing:0.05em}
.tm-panel-events{max-height:150px;overflow-y:auto}
.tm-pe-row{display:flex;align-items:center;gap:6px;padding:4px 0;border-bottom:1px solid rgba(255,255,255,0.04);font-size:11px}
.tm-pe-time{color:#475569;font-family:monospace;flex-shrink:0;width:55px}
.tm-pe-action{padding:1px 6px;border-radius:3px;font-weight:700;font-size:10px;flex-shrink:0;text-transform:uppercase}
.a-block{background:rgba(239,68,68,0.15);color:#f87171}.a-warn{background:rgba(245,158,11,0.15);color:#fbbf24}.a-pass{background:rgba(100,116,139,0.1);color:#64748b}
.tm-pe-desc{color:#94a3b8;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.tm-pe-empty{color:#475569;font-size:11px;padding:8px 0;text-align:center}
.tm-engine-list{display:flex;flex-direction:column;gap:6px}
.tm-eng-row{display:flex;align-items:center;gap:8px;font-size:11px}
.tm-eng-name{width:100px;flex-shrink:0;color:#94a3b8}
.tm-eng-bar-bg{flex:1;height:8px;background:rgba(255,255,255,0.05);border-radius:4px;overflow:hidden}
.tm-eng-bar{height:100%;border-radius:4px;transition:width 0.8s ease}
.tm-eng-count{width:30px;text-align:right;font-weight:700;font-family:monospace;color:#818cf8}
.tm-panel-fade-enter-active,.tm-panel-fade-leave-active{transition:opacity 0.3s ease,transform 0.3s ease}
.tm-panel-fade-enter-from{opacity:0;transform:translateY(-50%) translateX(20px)}.tm-panel-fade-leave-to{opacity:0;transform:translateY(-50%) translateX(20px)}
</style>