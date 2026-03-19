<template>
  <div class="bigscreen" @mousemove="showControls" ref="root">
    <!-- Mode switch pills (top-right) -->
    <div class="screen-mode-switch" :class="{ visible: controlsVisible }">
      <button :class="{ active: screenMode === 'data' }" @click="screenMode = 'data'">📊 数据大屏</button>
      <button :class="{ active: screenMode === 'map' }" @click="screenMode = 'map'">🗺️ 威胁地图</button>
      <button class="bs-exit-pill" @click="exitBigScreen" title="退出大屏">✕</button>
    </div>
    <!-- Threat Map mode -->
    <div v-if="screenMode === 'map'" class="threat-map-wrapper">
      <ThreatMap />
    </div>
    <!-- Data dashboard mode -->
    <header v-if="screenMode === 'data'" class="bs-header">
      <div class="bs-title"><span class="bs-logo">🦞</span><span class="bs-title-text">龙虾卫士 态势感知中心</span></div>
      <div class="bs-header-right">
        <span class="bs-clock">{{ clock }}</span>
      </div>
    </header>
    <div v-if="screenMode === 'data'" class="bs-grid">
      <div class="bs-card bs-metric" v-for="m in metrics" :key="m.key">
        <div class="bs-metric-label">{{ m.label }}</div>
        <div class="bs-metric-value" :style="{ color: m.color }"><span class="bs-metric-num">{{ m.display }}</span><span class="bs-metric-unit" v-if="m.unit">{{ m.unit }}</span></div>
        <div class="bs-metric-trend" :class="m.trendClass">{{ m.trendText }}</div>
      </div>
      <div class="bs-card bs-trend-chart">
        <div class="bs-card-title">攻击趋势 (24h)</div>
        <svg class="bs-svg-chart" viewBox="0 0 400 100" preserveAspectRatio="none">
          <defs>
            <linearGradient id="tg" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="#6366f1" stop-opacity="0.3"/><stop offset="100%" stop-color="#6366f1" stop-opacity="0"/></linearGradient>
            <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="#ef4444" stop-opacity="0.3"/><stop offset="100%" stop-color="#ef4444" stop-opacity="0"/></linearGradient>
          </defs>
          <line v-for="i in 3" :key="'g'+i" x1="0" :y1="i*25" x2="400" :y2="i*25" stroke="rgba(255,255,255,0.06)" stroke-width="0.5"/>
          <path :d="totalArea" fill="url(#tg)"/><polyline :points="totalLine" fill="none" stroke="#6366f1" stroke-width="2" stroke-linejoin="round"/>
          <path :d="blockArea" fill="url(#bg)"/><polyline :points="blockLine" fill="none" stroke="#ef4444" stroke-width="2" stroke-linejoin="round"/>
        </svg>
        <div class="bs-legend"><span class="bs-legend-item"><span class="bs-legend-dot" style="background:#6366f1"></span>总请求</span><span class="bs-legend-item"><span class="bs-legend-dot" style="background:#ef4444"></span>拦截数</span></div>
      </div>
      <div class="bs-card bs-owasp">
        <div class="bs-card-title">OWASP 威胁矩阵</div>
        <div class="bs-owasp-list">
          <div class="bs-owasp-item" v-for="o in owaspSorted" :key="o.id">
            <span class="bs-owasp-id">{{ o.id.replace('LLM','') }}</span>
            <span class="bs-owasp-name">{{ o.name_zh }}</span>
            <div class="bs-owasp-bar-bg"><div class="bs-owasp-bar-fill" :style="{ width: owaspBarW(o.count)+'%', background: owaspColor(o.risk_level) }"></div></div>
            <span class="bs-owasp-count" :style="{color:owaspColor(o.risk_level)}">{{ o.count }}</span>
          </div>
        </div>
      </div>
      <div class="bs-card bs-chains">
        <div class="bs-card-title">攻击链活跃</div>
        <div class="bs-chains-body" v-if="chainStats">
          <div class="bs-chain-big"><span class="bs-chain-num">{{ chainStats.active_chains }}</span><span>活跃链</span></div>
          <div class="bs-chain-sevs">
            <div v-if="chainStats.critical_chains" class="bs-sev"><span class="dot-c"></span>{{ chainStats.critical_chains }} Critical</div>
            <div v-if="chainStats.high_chains" class="bs-sev"><span class="dot-h"></span>{{ chainStats.high_chains }} High</div>
            <div v-if="chainStats.medium_chains" class="bs-sev"><span class="dot-m"></span>{{ chainStats.medium_chains }} Medium</div>
            <div v-if="chainStats.low_chains" class="bs-sev"><span class="dot-l"></span>{{ chainStats.low_chains }} Low</div>
          </div>
          <div class="bs-chain-info">Agent: <b>{{ chainStats.agents_involved }}</b> · 事件: <b>{{ chainStats.total_events }}</b></div>
        </div>
        <div v-else class="bs-empty">暂无数据</div>
      </div>
      <div class="bs-card bs-leaderboard">
        <div class="bs-card-title">安全排行榜 TOP5</div>
        <div class="bs-lb-list">
          <div class="bs-lb-item" v-for="(t,i) in lbTop5" :key="t.tenant_id">
            <span class="bs-lb-rank" :class="'r'+(i+1)">{{ i+1 }}</span>
            <span class="bs-lb-name">{{ t.tenant_name }}</span>
            <div class="bs-lb-bar"><div class="bs-lb-fill" :style="{width:t.health_score+'%',background:sc(t.health_score)}"></div></div>
            <span class="bs-lb-score" :style="{color:sc(t.health_score)}">{{ t.health_score }}</span>
          </div>
        </div>
        <div v-if="!lbTop5.length" class="bs-empty">暂无排行数据</div>
      </div>
      <div class="bs-card bs-honeypot">
        <div class="bs-card-title">🍯 蜜罐实时</div>
        <div class="bs-hp-grid" v-if="hpStats">
          <div class="bs-hp-cell"><div class="bs-hp-num">{{ hpStats.active_templates }}</div><div class="bs-hp-lbl">活跃模板</div></div>
          <div class="bs-hp-cell"><div class="bs-hp-num" style="color:#f59e0b">{{ hpStats.total_triggers }}</div><div class="bs-hp-lbl">触发数</div></div>
          <div class="bs-hp-cell"><div class="bs-hp-num" style="color:#ef4444">{{ hpStats.total_detonated }}</div><div class="bs-hp-lbl">已引爆</div></div>
          <div class="bs-hp-cell"><div class="bs-hp-num" style="color:#22c55e">{{ hpStats.active_watermarks }}</div><div class="bs-hp-lbl">活跃水印</div></div>
        </div>
        <div v-else class="bs-empty">暂无蜜罐数据</div>
      </div>
      <div class="bs-card bs-right-panel">
        <div class="bs-tab-hdr">
          <button class="bs-tab" :class="{on:tab==='events'}" @click="tab='events'">事件流</button>
          <button class="bs-tab" :class="{on:tab==='carousel'}" @click="tab='carousel'">轮播</button>
        </div>
        <div class="bs-event-stream" v-show="tab==='events'">
          <div class="bs-ev-scroll" ref="evScroll">
            <div class="bs-ev" v-for="e in auditEvents" :key="e.id" :class="'ev-'+e.action">
              <span class="bs-ev-t">{{ fmtTime(e.timestamp) }}</span>
              <span class="bs-ev-a" :class="'a-'+e.action">{{ e.action }}</span>
              <span class="bs-ev-d">{{ trunc(e.reason||e.content_preview||'-',40) }}</span>
            </div>
          </div>
        </div>
        <div class="bs-carousel" v-show="tab==='carousel'">
          <transition name="cfade" mode="out-in">
            <div :key="cIdx" class="bs-slide">
              <template v-if="cIdx===0">
                <div class="bs-sl-title">🎯 红队检测率</div>
                <div class="bs-sl-big" v-if="rtData">{{ rtData.pass_rate?rtData.pass_rate.toFixed(1):'0' }}%</div>
                <div class="bs-sl-sub" v-if="rtData">{{ rtData.total_tests }} 测试 / {{ rtData.passed }} 通过</div>
                <div class="bs-sl-big" v-else>N/A</div>
              </template>
              <template v-else-if="cIdx===1">
                <div class="bs-sl-title">🔬 A/B 测试</div>
                <div class="bs-sl-big">{{ abCount }} 个测试</div>
                <div class="bs-sl-sub">{{ abRunning }} 运行中</div>
              </template>
              <template v-else>
                <div class="bs-sl-title">⚠️ 高风险用户</div>
                <div class="bs-risk-list" v-if="riskUsers.length">
                  <div class="bs-risk-row" v-for="u in riskUsers.slice(0,5)" :key="u.sender_id">
                    <span>{{ u.sender_id }}</span><span :style="{color:u.risk_score>=50?'#ef4444':'#f59e0b'}">{{ u.risk_score.toFixed(0) }}</span>
                  </div>
                </div>
                <div v-else class="bs-sl-sub">暂无</div>
              </template>
            </div>
          </transition>
          <div class="bs-dots"><span v-for="i in 3" :key="i" class="bs-dot" :class="{on:cIdx===i-1}" @click="cIdx=i-1"></span></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import ThreatMap from '../components/ThreatMap.vue'

const router = useRouter()
const screenMode = ref('data')
const clock = ref('')
let clockT = null
function updClock() {
  const n = new Date()
  clock.value = `${n.getFullYear()}-${S(n.getMonth()+1)}-${S(n.getDate())} ${S(n.getHours())}:${S(n.getMinutes())}:${S(n.getSeconds())}`
}
function S(v){ return String(v).padStart(2,'0') }

const controlsVisible = ref(true)
let hideT = null
function showControls(){ controlsVisible.value=true; clearTimeout(hideT); hideT=setTimeout(()=>{controlsVisible.value=false},5000) }
function exitBigScreen(){ try{document.exitFullscreen()}catch{}; router.push('/overview') }

const healthScore = ref(null)
const statsData = ref(null)
const prevStats = ref(null)
const realtimeSnap = ref(null)
const uniqueAgents = ref(0)
const owaspMatrix = ref([])
const chainStats = ref(null)
const leaderboard = ref([])
const hpStats = ref(null)
const auditEvents = ref([])
const rtData = ref(null)
const abCount = ref(0)
const abRunning = ref(0)
const riskUsers = ref([])
const trendData = ref({total:[],blocked:[]})
const tab = ref('events')
const cIdx = ref(0)

function sc(s){ return s>=90?'#22c55e':s>=70?'#84cc16':s>=50?'#f59e0b':s>=30?'#f97316':'#ef4444' }

const metrics = computed(()=>{
  const hs=healthScore.value, st=statsData.value, ps=prevStats.value, rt=realtimeSnap.value
  const score=hs?hs.score:0
  // QPS: prefer realtime snapshot, fallback to stats
  const qps=rt?(rt.total_req||0)/Math.max(rt.slots?.length||60,1):st?(st.requests_per_sec||st.qps||0):0
  const tot=st?(st.total||st.total_requests||0):0, blk=st?(st.blocked||st.blocked_requests||0):0
  const br=tot>0?(blk/tot*100):0
  // Agent count: prefer unique_senders from stats, fallback to audit distinct senders, then upstreams
  const auDistinct=auditEvents.value?[...new Set(auditEvents.value.filter(e=>e.sender_id).map(e=>e.sender_id))].length:0
  const ag=st?(st.unique_senders||st.active_agents||0):0 || auDistinct || uniqueAgents.value || 0
  const pq=ps?(ps.requests_per_sec||ps.qps||0):qps
  const pt=ps?(ps.total||ps.total_requests||0):tot, pb=ps?(ps.blocked||ps.blocked_requests||0):blk
  const pbr=pt>0?(pb/pt*100):br
  const pScore=hs&&hs.trend&&hs.trend.length>=2?hs.trend[hs.trend.length-2].score:score
  return [
    {key:'health',label:'安全健康分',display:score,unit:'',color:sc(score),trendText:score>=pScore?`▲ +${score-pScore}`:`▼ ${score-pScore}`,trendClass:score>=pScore?'trend-up':'trend-down'},
    {key:'qps',label:'实时 QPS',display:Math.round(qps),unit:'',color:'#818cf8',trendText:qps>=pq?`▲ ${Math.round(qps-pq)}`:`▼ ${Math.round(qps-pq)}`,trendClass:qps>=pq?'trend-up':'trend-down'},
    {key:'br',label:'拦截率',display:br.toFixed(1),unit:'%',color:br>30?'#ef4444':br>10?'#f59e0b':'#22c55e',trendText:br>=pbr?`▲ +${(br-pbr).toFixed(1)}%`:`▼ ${(br-pbr).toFixed(1)}%`,trendClass:br<=pbr?'trend-up':'trend-down'},
    {key:'ag',label:'在线 Agent',display:ag,unit:'',color:'#22c55e',trendText:'稳定',trendClass:'trend-stable'},
  ]
})

const owaspSorted = computed(()=>[...owaspMatrix.value].sort((a,b)=>b.count-a.count))
function owaspBarW(c){const mx=owaspSorted.value.length?Math.max(...owaspSorted.value.map(o=>o.count),1):1;return Math.min((c/mx)*100,100)}
function owaspColor(l){return l==='high'?'#ef4444':l==='low'?'#f59e0b':'#334155'}
const lbTop5 = computed(()=>leaderboard.value.slice(0,5))

function genPts(arr,w,h){if(!arr.length)return '';const mx=Math.max(...arr,1),s=w/Math.max(arr.length-1,1);return arr.map((v,i)=>`${(i*s).toFixed(1)},${(h-(v/mx)*(h-10)).toFixed(1)}`).join(' ')}
function genArea(arr,w,h){if(!arr.length)return '';const mx=Math.max(...arr,1),s=w/Math.max(arr.length-1,1);const p=arr.map((v,i)=>`${(i*s).toFixed(1)},${(h-(v/mx)*(h-10)).toFixed(1)}`);return `M0,${h} L${p.join(' L')} L${w},${h} Z`}
const totalLine = computed(()=>genPts(trendData.value.total,400,100))
const blockLine = computed(()=>genPts(trendData.value.blocked,400,100))
const totalArea = computed(()=>genArea(trendData.value.total,400,100))
const blockArea = computed(()=>genArea(trendData.value.blocked,400,100))

const evScroll = ref(null)
let scrollT = null
function startScroll(){if(scrollT)clearInterval(scrollT);scrollT=setInterval(()=>{const e=evScroll.value;if(!e)return;if(e.scrollTop+e.clientHeight>=e.scrollHeight-2)e.scrollTop=0;else e.scrollTop+=1},80)}
function fmtTime(ts){if(!ts)return '';const d=new Date(ts);return `${S(d.getHours())}:${S(d.getMinutes())}`}
function trunc(s,n){return s.length>n?s.slice(0,n)+'…':s}

let carouselT = null
function startCarousel(){if(carouselT)clearInterval(carouselT);carouselT=setInterval(()=>{cIdx.value=(cIdx.value+1)%3},10000)}

async function fetchAll(){
  try{
    const [st,hs,lb,cs,hp,au,rt,hz]=await Promise.allSettled([api('/api/v1/stats'),api('/api/v1/health/score'),api('/api/v1/leaderboard'),api('/api/v1/attack-chains/stats'),api('/api/v1/honeypot/stats'),api('/api/v1/audit/logs?limit=30'),api('/api/v1/metrics/realtime'),api('/healthz')])
    if(st.status==='fulfilled'){prevStats.value=statsData.value;statsData.value=st.value}
    if(hs.status==='fulfilled')healthScore.value=hs.value
    if(lb.status==='fulfilled')leaderboard.value=Array.isArray(lb.value)?lb.value:(lb.value.leaderboard||lb.value.scores||[])
    if(cs.status==='fulfilled')chainStats.value=cs.value
    if(hp.status==='fulfilled')hpStats.value=hp.value
    if(au.status==='fulfilled'){const l=Array.isArray(au.value)?au.value:(au.value.logs||au.value.items||[]);auditEvents.value=l}
    if(rt.status==='fulfilled')realtimeSnap.value=rt.value
    // Derive unique agents from healthz upstreams or stats
    if(hz.status==='fulfilled'){
      const h=hz.value
      const ups=h?.checks?.upstream||h?.upstreams||{}
      uniqueAgents.value=ups.healthy||ups.total||0
    }
  }catch(e){console.error('[BigScreen]',e)}
}
async function fetchOwasp(){
  // Prefer bigscreen/data which includes OWASP matrix + trend data
  try{
    const d=await api('/api/v1/bigscreen/data')
    if(d){
      if(d.owasp_matrix&&d.owasp_matrix.length)owaspMatrix.value=d.owasp_matrix
      if(d.trend_total)trendData.value.total=d.trend_total
      if(d.trend_blocked)trendData.value.blocked=d.trend_blocked
    }
  }catch{
    // Fallback: try health score owasp matrix
    try{const d=await api('/api/v1/health/score');if(d&&d.owasp_matrix)owaspMatrix.value=d.owasp_matrix}catch{}
  }
}
async function fetchCarousel(){
  try{
    const [rt,ab,ru]=await Promise.allSettled([api('/api/v1/redteam/reports?limit=1'),api('/api/v1/ab-tests'),api('/api/v1/users/risk-top?limit=5')])
    if(rt.status==='fulfilled'){const r=Array.isArray(rt.value)?rt.value:(rt.value.reports||[]);rtData.value=r.length?r[0]:null}
    if(ab.status==='fulfilled'){const t=Array.isArray(ab.value)?ab.value:(ab.value.tests||[]);abCount.value=t.length;abRunning.value=t.filter(x=>x.status==='running').length}
    if(ru.status==='fulfilled')riskUsers.value=Array.isArray(ru.value)?ru.value:(ru.value.users||[])
  }catch{}
}
function genDemoTrend(){if(trendData.value.total.length>0)return;const t=[],b=[];for(let i=0;i<24;i++){const v=50+Math.floor(Math.random()*100);t.push(v);b.push(Math.floor(v*(0.05+Math.random()*0.15)))};trendData.value={total:t,blocked:b}}

let dataT = null
onMounted(async()=>{
  updClock();clockT=setInterval(updClock,1000)
  try{await document.documentElement.requestFullscreen()}catch{}
  await fetchAll();await fetchOwasp();await fetchCarousel();genDemoTrend()
  dataT=setInterval(async()=>{await fetchAll();await fetchOwasp()},30000)
  setInterval(fetchCarousel,30000)
  startCarousel();await nextTick();startScroll();showControls()
})
onUnmounted(()=>{clearInterval(clockT);clearInterval(dataT);clearInterval(carouselT);clearInterval(scrollT);clearTimeout(hideT);try{if(document.fullscreenElement)document.exitFullscreen()}catch{}})
</script>

<style scoped>
.bigscreen{position:fixed;top:0;left:0;right:0;bottom:0;background:#06060e;color:#e2e8f0;font-family:'Inter',-apple-system,BlinkMacSystemFont,sans-serif;overflow:hidden;display:flex;flex-direction:column;z-index:10000}
.bs-header{display:flex;justify-content:space-between;align-items:center;padding:0.6vh 1.5vw;background:linear-gradient(180deg,rgba(99,102,241,0.1) 0%,transparent 100%);border-bottom:1px solid rgba(99,102,241,0.2);flex-shrink:0;height:5vh}
.bs-title{display:flex;align-items:center;gap:0.8vw}
.bs-logo{font-size:clamp(1rem,1.8vw,1.8rem)}
.bs-title-text{font-size:clamp(0.85rem,1.4vw,1.5rem);font-weight:700;background:linear-gradient(135deg,#818cf8,#6366f1,#a78bfa);-webkit-background-clip:text;-webkit-text-fill-color:transparent;letter-spacing:0.05em}
.bs-header-right{display:flex;align-items:center;gap:1.2vw}
.bs-clock{font-size:clamp(0.75rem,1.1vw,1.1rem);font-family:'JetBrains Mono',monospace;color:#94a3b8;letter-spacing:0.05em}
.bs-exit-btn{background:rgba(239,68,68,0.15);border:1px solid rgba(239,68,68,0.3);color:#f87171;border-radius:6px;padding:3px 10px;cursor:pointer;opacity:0;transition:opacity 0.3s;font-size:0.7rem}
.bs-exit-btn.visible{opacity:1}
.bs-exit-btn:hover{background:rgba(239,68,68,0.3)}

/* Mode switch pills */
.screen-mode-switch{position:fixed;top:12px;right:16px;z-index:10001;display:flex;gap:2px;background:rgba(10,10,25,0.6);backdrop-filter:blur(10px);-webkit-backdrop-filter:blur(10px);border:1px solid rgba(99,102,241,0.2);border-radius:20px;padding:3px;opacity:0;transition:opacity 0.3s}
.screen-mode-switch.visible{opacity:1}
.screen-mode-switch button{background:transparent;border:none;color:#64748b;font-size:0.7rem;padding:4px 12px;border-radius:16px;cursor:pointer;transition:all 0.2s;white-space:nowrap}
.screen-mode-switch button.active{background:rgba(99,102,241,0.2);color:#a5b4fc;font-weight:600}
.screen-mode-switch button:hover:not(.active){background:rgba(255,255,255,0.05);color:#94a3b8}
.bs-exit-pill{color:#f87171!important;font-weight:700;font-size:0.75rem!important}
.bs-exit-pill:hover{background:rgba(239,68,68,0.2)!important;color:#fca5a5!important}
.threat-map-wrapper{position:absolute;inset:0;z-index:1}

.bs-grid{flex:1;display:grid;grid-template-columns:1fr 1fr 1fr 1fr 2fr;grid-template-rows:1fr 1.2fr 1.2fr;gap:clamp(3px,0.4vw,8px);padding:clamp(3px,0.4vw,8px);overflow:hidden}
.bs-card{background:rgba(15,15,30,0.8);border:1px solid rgba(99,102,241,0.15);border-radius:8px;padding:clamp(6px,0.8vh,16px) clamp(6px,0.6vw,14px);overflow:hidden;box-shadow:0 0 12px rgba(99,102,241,0.08);display:flex;flex-direction:column}
.bs-card-title{font-size:clamp(0.6rem,0.8vw,0.9rem);color:#94a3b8;font-weight:600;margin-bottom:clamp(2px,0.4vh,8px);white-space:nowrap;flex-shrink:0}

/* Metrics */
.bs-metric{align-items:center;justify-content:center;text-align:center}
.bs-metric-label{font-size:clamp(0.55rem,0.7vw,0.8rem);color:#64748b;margin-bottom:clamp(2px,0.3vh,6px);font-weight:500}
.bs-metric-value{display:flex;align-items:baseline;gap:2px}
.bs-metric-num{font-size:clamp(1.4rem,3.5vw,4rem);font-weight:800;line-height:1;transition:color 0.5s}
.bs-metric-unit{font-size:clamp(0.7rem,1.2vw,1.4rem);opacity:0.7}
.bs-metric-trend{font-size:clamp(0.5rem,0.6vw,0.7rem);margin-top:clamp(2px,0.3vh,6px);font-weight:600}
.trend-up{color:#22c55e}.trend-down{color:#ef4444}.trend-stable{color:#64748b}

/* Trend chart */
.bs-trend-chart{grid-row:1/2;grid-column:5/6}
.bs-svg-chart{width:100%;flex:1;min-height:0}
.bs-legend{display:flex;gap:1vw;justify-content:center;flex-shrink:0;padding-top:2px}
.bs-legend-item{display:flex;align-items:center;gap:4px;font-size:clamp(0.45rem,0.55vw,0.65rem);color:#94a3b8}
.bs-legend-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}

/* OWASP */
.bs-owasp{grid-column:1/3;grid-row:2/3}
.bs-owasp-list{flex:1;display:flex;flex-direction:column;gap:clamp(1px,0.3vh,4px);overflow:hidden}
.bs-owasp-item{display:flex;align-items:center;gap:clamp(3px,0.4vw,8px);font-size:clamp(0.5rem,0.65vw,0.75rem)}
.bs-owasp-id{color:#64748b;font-family:monospace;width:2em;text-align:right;flex-shrink:0}
.bs-owasp-name{width:5em;flex-shrink:0;color:#94a3b8;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.bs-owasp-bar-bg{flex:1;height:clamp(6px,0.8vh,12px);background:rgba(255,255,255,0.05);border-radius:3px;overflow:hidden}
.bs-owasp-bar-fill{height:100%;border-radius:3px;transition:width 0.8s ease}
.bs-owasp-count{width:2.5em;text-align:right;font-weight:700;font-family:monospace}

/* Attack chains */
.bs-chains{grid-column:3/5;grid-row:2/3}
.bs-chains-body{flex:1;display:flex;flex-direction:column;gap:clamp(4px,0.5vh,10px)}
.bs-chain-big{display:flex;align-items:baseline;gap:0.5vw}
.bs-chain-num{font-size:clamp(1.5rem,3vw,3.5rem);font-weight:800;color:#818cf8;line-height:1}
.bs-chain-sevs{display:flex;flex-wrap:wrap;gap:clamp(4px,0.5vw,10px)}
.bs-sev{display:flex;align-items:center;gap:4px;font-size:clamp(0.5rem,0.6vw,0.7rem)}
.dot-c,.dot-h,.dot-m,.dot-l{width:8px;height:8px;border-radius:50%;flex-shrink:0}
.dot-c{background:#ef4444}.dot-h{background:#f59e0b}.dot-m{background:#3b82f6}.dot-l{background:#64748b}
.bs-chain-info{font-size:clamp(0.5rem,0.6vw,0.7rem);color:#64748b}

/* Leaderboard */
.bs-leaderboard{grid-column:1/3;grid-row:3/4}
.bs-lb-list{flex:1;display:flex;flex-direction:column;gap:clamp(2px,0.4vh,6px);overflow:hidden}
.bs-lb-item{display:flex;align-items:center;gap:clamp(4px,0.4vw,8px);font-size:clamp(0.5rem,0.65vw,0.75rem)}
.bs-lb-rank{width:1.5em;height:1.5em;border-radius:50%;display:flex;align-items:center;justify-content:center;font-weight:800;font-size:0.7em;flex-shrink:0;background:rgba(255,255,255,0.08);color:#94a3b8}
.r1{background:rgba(234,179,8,0.2);color:#fbbf24}.r2{background:rgba(192,192,192,0.15);color:#d1d5db}.r3{background:rgba(180,83,9,0.15);color:#d97706}
.bs-lb-name{width:6em;flex-shrink:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#cbd5e1}
.bs-lb-bar{flex:1;height:clamp(6px,0.8vh,10px);background:rgba(255,255,255,0.05);border-radius:3px;overflow:hidden}
.bs-lb-fill{height:100%;border-radius:3px;transition:width 0.8s}
.bs-lb-score{width:2.5em;text-align:right;font-weight:800;font-family:monospace}

/* Honeypot */
.bs-honeypot{grid-column:3/5;grid-row:3/4}
.bs-hp-grid{flex:1;display:grid;grid-template-columns:1fr 1fr;gap:clamp(4px,0.5vw,10px);align-content:center}
.bs-hp-cell{text-align:center;padding:clamp(4px,0.5vh,10px)}
.bs-hp-num{font-size:clamp(1.2rem,2.2vw,2.6rem);font-weight:800;color:#818cf8;line-height:1.2}
.bs-hp-lbl{font-size:clamp(0.45rem,0.55vw,0.65rem);color:#64748b;margin-top:2px}

/* Right panel (events/carousel) */
.bs-right-panel{grid-column:5/6;grid-row:2/4}
.bs-tab-hdr{display:flex;gap:2px;flex-shrink:0;margin-bottom:clamp(2px,0.3vh,6px)}
.bs-tab{flex:1;padding:3px 0;background:rgba(255,255,255,0.03);border:1px solid rgba(255,255,255,0.08);border-radius:4px;color:#64748b;font-size:clamp(0.5rem,0.6vw,0.7rem);cursor:pointer;transition:all 0.2s}
.bs-tab.on{background:rgba(99,102,241,0.15);border-color:rgba(99,102,241,0.3);color:#a5b4fc}
.bs-event-stream{flex:1;overflow:hidden;position:relative}
.bs-ev-scroll{height:100%;overflow:hidden}
.bs-ev{display:flex;align-items:center;gap:clamp(4px,0.4vw,8px);padding:clamp(2px,0.2vh,4px) 0;border-bottom:1px solid rgba(255,255,255,0.03);font-size:clamp(0.48rem,0.58vw,0.68rem);animation:evFadeIn 0.5s ease}
@keyframes evFadeIn{from{opacity:0;transform:translateY(-4px)}to{opacity:1;transform:translateY(0)}}
.bs-ev-t{color:#475569;font-family:monospace;flex-shrink:0;width:3.5em}
.bs-ev-a{padding:1px 6px;border-radius:3px;font-weight:700;font-size:0.85em;flex-shrink:0;text-transform:uppercase}
.a-block{background:rgba(239,68,68,0.15);color:#f87171}
.a-warn{background:rgba(245,158,11,0.15);color:#fbbf24}
.a-pass{background:rgba(100,116,139,0.1);color:#64748b}
.bs-ev-d{color:#94a3b8;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.ev-block .bs-ev-d{color:#fca5a5}.ev-warn .bs-ev-d{color:#fcd34d}

/* Carousel */
.bs-carousel{flex:1;display:flex;flex-direction:column;justify-content:center;align-items:center;position:relative}
.bs-slide{text-align:center;width:100%}
.bs-sl-title{font-size:clamp(0.6rem,0.8vw,0.9rem);color:#94a3b8;margin-bottom:clamp(4px,0.5vh,10px)}
.bs-sl-big{font-size:clamp(1.8rem,3.5vw,4rem);font-weight:800;color:#818cf8;line-height:1.2}
.bs-sl-sub{font-size:clamp(0.5rem,0.6vw,0.7rem);color:#64748b;margin-top:4px}
.bs-risk-list{text-align:left;max-width:80%;margin:0 auto}
.bs-risk-row{display:flex;justify-content:space-between;padding:clamp(2px,0.2vh,4px) 0;font-size:clamp(0.5rem,0.6vw,0.7rem);border-bottom:1px solid rgba(255,255,255,0.03)}
.bs-dots{display:flex;gap:6px;margin-top:clamp(4px,0.5vh,10px)}
.bs-dot{width:8px;height:8px;border-radius:50%;background:rgba(255,255,255,0.15);cursor:pointer;transition:all 0.3s}
.bs-dot.on{background:#6366f1;box-shadow:0 0 6px rgba(99,102,241,0.5)}

.cfade-enter-active,.cfade-leave-active{transition:opacity 0.4s ease}
.cfade-enter-from,.cfade-leave-to{opacity:0}

.bs-empty{flex:1;display:flex;align-items:center;justify-content:center;color:#475569;font-size:clamp(0.5rem,0.6vw,0.7rem)}

/* Glow animation for cards */
.bs-card{transition:box-shadow 0.5s ease}
.bs-card:hover{box-shadow:0 0 20px rgba(99,102,241,0.15)}

/* Responsive 4K */
@media(min-width:3000px){.bs-metric-num{font-size:5rem}.bs-chain-num{font-size:4rem}.bs-hp-num{font-size:3rem}.bs-sl-big{font-size:5rem}}
</style>
