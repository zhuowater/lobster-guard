<template>
  <div>
    <div class="stats-row">
      <div class="stat-card"><div class="sv">{{ total }}</div><div class="sl2">总会话数</div></div>
      <div class="stat-card sc-green"><div class="sv">{{ activeSessions }}</div><div class="sl2">活跃会话</div></div>
      <div class="stat-card sc-red"><div class="sv">{{ securityEvents }}</div><div class="sl2">安全事件</div></div>
      <div class="stat-card"><div class="sv">{{ avgMessages }}</div><div class="sl2">平均消息数</div></div>
    </div>
    <div class="card">
      <div class="card-header">
        <span class="card-icon"><Icon name="film" :size="18" /></span>
        <span class="card-title">会话回放</span>
        <div class="card-actions">
          <button class="btn btn-ghost btn-sm" @click="showAdv=!showAdv"><Icon name="settings" :size="14" /> {{ showAdv?'收起':'高级筛选' }}</button>
          <button class="btn btn-ghost btn-sm" @click="load"><Icon name="refresh" :size="14" /> 刷新</button>
        </div>
      </div>
      <!-- Filters -->
      <div class="replay-filters">
        <div class="filter-group">
          <select v-model="f.range" @change="fc" class="filter-select"><option value="24h">最近24小时</option><option value="7d">最近7天</option><option value="30d">最近30天</option><option value="">全部</option></select>
          <select v-model="f.risk" @change="fc" class="filter-select"><option value="">全部风险</option><option value="critical">🔴 严重</option><option value="high">🟠 高危</option><option value="medium">🟡 中等</option><option value="low">🟢 低风险</option></select>
          <select v-model="sortMode" @change="load" class="filter-select"><option value="newest">最新优先</option><option value="messages">最多消息</option><option value="risk">最高风险</option><option value="duration">最长时间</option></select>
          <div class="search-box"><Icon name="search" :size="14" class="search-icon" /><input v-model="f.q" placeholder="搜索 trace_id / 用户 / 内容..." @keyup.enter="fc" /><button v-if="f.q" class="search-clear" @click="f.q='';fc()">×</button></div>
          <button class="btn btn-sm btn-primary" @click="fc">搜索</button>
        </div>
      </div>
      <!-- Advanced Filters -->
      <div v-if="showAdv" class="adv-filters">
        <div class="adv-row">
          <div class="adv-item"><label>用户ID</label><input v-model="f.sender_id" placeholder="sender_id..." @keyup.enter="fc" /></div>
          <div class="adv-item"><label>状态</label><select v-model="f.status" @change="fc"><option value="">全部</option><option value="active">活跃</option><option value="ended">已结束</option><option value="blocked">已拦截</option></select></div>
          <div class="adv-item"><label>开始</label><input type="datetime-local" v-model="f.from" @change="fc" /></div>
          <div class="adv-item"><label>结束</label><input type="datetime-local" v-model="f.to" @change="fc" /></div>
        </div>
        <div style="margin-top:8px;text-align:right"><button class="btn btn-ghost btn-sm" @click="resetF">重置</button></div>
      </div>
      <!-- Active Filter Tags -->
      <div class="active-filter-tags" v-if="filterTags.length">
        <span class="aft-label">筛选:</span>
        <span class="ftag" v-for="t in filterTags" :key="t.k">{{ t.l }} <button class="ftag-x" @click="rmF(t.k)">×</button></span>
        <button class="btn-link" @click="resetF">清除全部</button>
      </div>
      <!-- Loading Skeleton -->
      <div v-if="loading" class="skeleton-list">
        <div class="skeleton-card" v-for="i in 5" :key="i"><div class="sk-line sk-w60"></div><div class="sk-line sk-w40"></div><div class="sk-line sk-w80"></div><div class="sk-line sk-w30"></div></div>
      </div>
      <!-- Empty -->
      <div v-else-if="!sessions.length" class="empty-state">
        <div class="empty-icon">🎬</div><div class="empty-title">暂无会话</div>
        <div class="empty-desc">{{ hasF ? '没有匹配的会话，调整筛选条件试试' : '系统尚未记录任何会话' }}</div>
        <button v-if="hasF" class="btn btn-sm" style="margin-top:12px" @click="resetF">重置筛选</button>
      </div>
      <!-- Session Cards -->
      <div v-else class="session-list">
        <div v-for="s in sorted" :key="s.trace_id" class="session-card" :class="['risk-'+s.risk_level,{'has-sec':hasSec(s)}]">
          <div class="sc-main" @click="go(s.session_id || s.trace_id)">
            <div class="sc-top">
              <div class="sc-top-l"><span class="sc-trace mono" v-html="hl(s.trace_id)"></span><span class="risk-badge" :class="'badge-'+s.risk_level">{{ rlabel(s.risk_level) }}</span></div>
              <div class="sc-top-r"><span class="sc-dur"><Icon name="clock" :size="12" /> {{ fmtDur(s.duration_ms) }}</span><span class="sc-time">{{ fmtTime(s.start_time) }}</span></div>
            </div>
            <div class="sc-meta"><span v-if="s.sender_id" class="meta-item"><Icon name="users" :size="12" /> <span v-html="hl(s.display_name || s.sender_id)"></span><span v-if="s.display_name" class="meta-sub" :title="s.sender_id"> ({{ s.sender_id }})</span></span><span v-if="s.department" class="meta-item meta-dept">{{ s.department }}</span><span v-if="s.model" class="meta-item"><Icon name="brain" :size="12" /> {{ s.model }}</span></div>
            <div class="sc-stats"><span class="chip"><span class="chip-l">IM</span><span class="chip-v">{{ s.im_events }}</span></span><span class="chip"><span class="chip-l">LLM</span><span class="chip-v">{{ s.llm_calls }}</span></span><span class="chip"><span class="chip-l">Tools</span><span class="chip-v">{{ s.tool_calls }}</span></span><span class="chip"><span class="chip-l">Tokens</span><span class="chip-v">{{ fmtNum(s.total_tokens) }}</span></span></div>
            <div class="sc-flags" v-if="hasSec(s)"><span class="flag-danger" v-if="s.canary_leaked"><Icon name="alert-triangle" :size="12" /> Canary泄露</span><span class="flag-danger" v-if="s.blocked"><Icon name="shield" :size="12" /> 已拦截</span><span class="flag-warn" v-if="s.budget_exceeded"><Icon name="zap" :size="12" /> 预算超限</span><span class="flag-warn" v-if="s.flagged_tools>0"><Icon name="alert-triangle" :size="12" /> {{ s.flagged_tools }}可疑工具</span></div>
            <div class="sc-tags" v-if="(s.tags||[]).length"><span class="tag-pill" v-for="t in (s.tags||[]).slice(0,5)" :key="t">{{ t }}</span></div>
          </div>
          <div class="sc-footer">
            <button class="btn-expand" @click.stop="togExp(s.trace_id)"><span class="expand-arrow" :class="{rotated:expId===s.trace_id}">▾</span> {{ expId===s.trace_id?'收起':'预览' }}</button>
            <button class="btn-play" @click.stop="go(s.session_id || s.trace_id)"><Icon name="play" :size="12" /> 查看回放</button>
          </div>
          <div v-if="expId===s.trace_id" class="preview-panel">
            <div v-if="pvLoading" class="pv-loading">加载中...</div>
            <div v-else-if="!pvMsgs.length" class="pv-empty">暂无消息</div>
            <div v-else class="pv-msgs">
              <div v-for="(m,i) in pvMsgs.slice(0,6)" :key="i" class="pv-msg" :class="['pv-'+m.type,{'pv-blocked':m.action==='block'}]">
                <span class="pv-dir">{{ m.type==='im_inbound'?'⬅ 用户':'➡ Agent' }}</span>
                <span class="pv-text" :class="{'pv-lt':m.action==='block'}">{{ trunc(m.content,120) }}</span>
                <span class="pv-time">{{ fmtTimeFull(m.timestamp) }}</span>
                <span class="pv-act" :class="'pa-'+m.action" v-if="m.action&&m.action!=='pass'">{{ m.action }}</span>
              </div>
              <div class="pv-more" v-if="pvMsgs.length>6">还有 {{ pvMsgs.length-6 }} 条...</div>
            </div>
          </div>
        </div>
      </div>
      <!-- Pagination -->
      <div class="pagination" v-if="total>pageSize">
        <button class="btn btn-ghost btn-sm" :disabled="page<=1" @click="page--;load()"><Icon name="arrow-left" :size="12" /> 上一页</button>
        <div class="page-nums"><button v-for="p in vPages" :key="p" class="pg-btn" :class="{active:p===page,ellipsis:p==='...'}" :disabled="p==='...'" @click="p!=='...'&&(page=p,load())">{{ p }}</button></div>
        <span class="pg-info">共 {{ total }} 条</span>
        <button class="btn btn-ghost btn-sm" :disabled="page>=tPages" @click="page++;load()">下一页 <Icon name="chevron-right" :size="12" /></button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import Icon from '../components/Icon.vue'

const router = useRouter()
const loading = ref(false)
const sessions = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const showAdv = ref(false)
const sortMode = ref('newest')
const expId = ref(null)
const pvLoading = ref(false)
const pvMsgs = ref([])
const f = reactive({ range:'7d', risk:'', q:'', sender_id:'', status:'', from:'', to:'' })

const activeSessions = computed(() => sessions.value.filter(s => !s.end_time || (Date.now()-new Date(s.end_time).getTime())<300000).length)
const securityEvents = computed(() => sessions.value.filter(hasSec).length)
const avgMessages = computed(() => { const l=sessions.value.length; return l ? Math.round(sessions.value.reduce((a,s)=>a+(s.im_events||0),0)/l) : 0 })

const rw = l => ({critical:4,high:3,medium:2,low:1})[l]||0
const sorted = computed(() => {
  const ls = [...sessions.value]
  if (sortMode.value==='messages') ls.sort((a,b)=>(b.im_events||0)-(a.im_events||0))
  else if (sortMode.value==='risk') ls.sort((a,b)=>rw(b.risk_level)-rw(a.risk_level))
  else if (sortMode.value==='duration') ls.sort((a,b)=>(b.duration_ms||0)-(a.duration_ms||0))
  return ls
})
const tPages = computed(() => Math.ceil(total.value/pageSize))
const vPages = computed(() => {
  const tp=tPages.value, cp=page.value
  if (tp<=7) return Array.from({length:tp},(_,i)=>i+1)
  const r=[1]; if(cp>3)r.push('...'); for(let i=Math.max(2,cp-1);i<=Math.min(tp-1,cp+1);i++)r.push(i); if(cp<tp-2)r.push('...'); r.push(tp); return r
})
const filterTags = computed(() => {
  const t=[]
  if(f.range) t.push({k:'range',l:{'24h':'24小时','7d':'7天','30d':'30天'}[f.range]||f.range})
  if(f.risk) t.push({k:'risk',l:rlabel(f.risk)})
  if(f.q) t.push({k:'q',l:'搜索:'+f.q})
  if(f.sender_id) t.push({k:'sender_id',l:'用户:'+f.sender_id})
  if(f.status) t.push({k:'status',l:{active:'活跃',ended:'已结束',blocked:'已拦截'}[f.status]})
  if(f.from) t.push({k:'from',l:'从:'+f.from})
  if(f.to) t.push({k:'to',l:'至:'+f.to})
  return t
})
const hasF = computed(() => filterTags.value.length>0)

function rmF(k){ f[k]=''; fc() }
function resetF(){ Object.assign(f,{range:'7d',risk:'',q:'',sender_id:'',status:'',from:'',to:''}); sortMode.value='newest'; fc() }
function fmtTime(ts){ if(!ts)return'--'; const d=new Date(ts); return isNaN(d)?ts:d.toLocaleString('zh-CN',{hour12:false}) }
function fmtTimeFull(ts){ if(!ts)return''; const d=new Date(ts); return isNaN(d)?ts:d.toLocaleTimeString('zh-CN',{hour12:false}) }
function fmtDur(ms){ if(!ms||ms<=0)return'--'; if(ms<1000)return Math.round(ms)+'ms'; if(ms<60000)return(ms/1000).toFixed(1)+'s'; return Math.floor(ms/60000)+'m '+Math.floor((ms%60000)/1000)+'s' }
function fmtNum(n){ if(!n)return'0'; return n>=1000?(n/1000).toFixed(1)+'K':String(n) }
function rlabel(l){ return{critical:'🔴 严重',high:'🟠 高危',medium:'🟡 中等',low:'🟢 低风险'}[l]||l||'未知' }
function trunc(t,n){ return !t?'':t.length>n?t.slice(0,n)+'...':t }
function hl(text){ if(!text||!f.q)return text; const q=f.q.replace(/[.*+?^${}()|[\]\\]/g,'\\$&'); return text.replace(new RegExp('('+q+')','gi'),'<mark class="hl-match">$1</mark>') }
function hasSec(s){ return s.canary_leaked||s.blocked||s.budget_exceeded||s.flagged_tools>0 }

async function togExp(id){
  if(expId.value===id){expId.value=null;pvMsgs.value=[];return}
  expId.value=id;pvLoading.value=true;pvMsgs.value=[]
  try{const d=await api('/api/v1/sessions/replay/'+encodeURIComponent(id));pvMsgs.value=(d.events||[]).filter(e=>e.type==='im_inbound'||e.type==='im_outbound')}catch{pvMsgs.value=[]}
  pvLoading.value=false
}
function go(id){
  try{sessionStorage.setItem('lg_rf',JSON.stringify({...f,sortMode:sortMode.value,page:page.value}));sessionStorage.setItem('lg_nav_ids',JSON.stringify(sessions.value.map(s=>s.session_id||s.trace_id)))}catch{}
  router.push('/sessions/'+encodeURIComponent(id))
}
function fc(){ page.value=1; load() }
async function load(){
  loading.value=true; const p=[]
  if(f.from)p.push('from='+encodeURIComponent(f.from)); else if(f.range)p.push('from='+f.range)
  if(f.to)p.push('to='+encodeURIComponent(f.to))
  if(f.sender_id)p.push('sender_id='+encodeURIComponent(f.sender_id))
  if(f.risk)p.push('risk='+f.risk)
  if(f.q)p.push('q='+encodeURIComponent(f.q))
  p.push('limit='+pageSize,'offset='+((page.value-1)*pageSize))
  try{
    const d=await api('/api/v1/sessions/replay?'+p.join('&'))
    let ls=d.sessions||[]
    if(f.status==='active')ls=ls.filter(s=>!s.end_time||(Date.now()-new Date(s.end_time).getTime())<300000)
    else if(f.status==='ended')ls=ls.filter(s=>s.end_time&&(Date.now()-new Date(s.end_time).getTime())>=300000)
    else if(f.status==='blocked')ls=ls.filter(s=>s.blocked)
    sessions.value=ls;total.value=d.total||0
  }catch{sessions.value=[];total.value=0}
  loading.value=false
}
function restoreF(){try{const s=JSON.parse(sessionStorage.getItem('lg_rf')||'null');if(s){Object.keys(f).forEach(k=>{if(s[k]!==undefined)f[k]=s[k]});if(s.sortMode)sortMode.value=s.sortMode;if(s.page)page.value=s.page}}catch{}}
let timer=null
onMounted(()=>{restoreF();load();timer=setInterval(load,60000)})
onUnmounted(()=>clearInterval(timer))
</script>

<style scoped>
.stats-row{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:16px}
.stat-card{background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:16px;text-align:center;transition:all .15s}
.stat-card:hover{border-color:var(--color-primary);transform:translateY(-2px)}
.sv{font-size:1.75rem;font-weight:800;color:var(--text-primary);font-family:var(--font-mono);line-height:1.2}
.sl2{font-size:11px;color:var(--text-tertiary);margin-top:4px;text-transform:uppercase;letter-spacing:.05em}
.sc-green .sv{color:#22C55E}.sc-red .sv{color:#EF4444}
@media(max-width:640px){.stats-row{grid-template-columns:repeat(2,1fr)}}

.replay-filters{margin-bottom:12px}
.filter-group{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.filter-select{background:var(--bg-elevated);border:1px solid var(--border-default);border-radius:var(--radius-md);color:var(--text-primary);padding:6px 10px;font-size:13px;outline:none;cursor:pointer}
.filter-select:focus{border-color:var(--color-primary)}
.filter-select option{background:var(--bg-elevated)}
.search-box{position:relative;flex:1;min-width:200px}
.search-box .search-icon{position:absolute;left:10px;top:50%;transform:translateY(-50%);color:var(--text-tertiary);pointer-events:none}
.search-box input{width:100%;padding:6px 32px;background:var(--bg-elevated);border:1px solid var(--border-default);border-radius:var(--radius-md);color:var(--text-primary);font-size:13px;outline:none}
.search-box input:focus{border-color:var(--color-primary)}
.search-clear{position:absolute;right:8px;top:50%;transform:translateY(-50%);background:none;border:none;color:var(--text-tertiary);cursor:pointer;font-size:16px;padding:2px}
.search-clear:hover{color:var(--text-primary)}
.btn-primary{background:var(--color-primary);color:#fff;border:none;border-radius:var(--radius-md);padding:6px 12px;font-size:13px;font-weight:600;cursor:pointer}
.btn-primary:hover{opacity:.9}

.adv-filters{background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);padding:16px;margin-bottom:12px}
.adv-row{display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:12px}
.adv-item label{display:block;font-size:11px;color:var(--text-tertiary);margin-bottom:4px;font-weight:500;text-transform:uppercase;letter-spacing:.05em}
.adv-item input,.adv-item select{width:100%;background:var(--bg-elevated);border:1px solid var(--border-default);border-radius:var(--radius-md);color:var(--text-primary);padding:6px 10px;font-size:13px;outline:none}
.adv-item input:focus,.adv-item select:focus{border-color:var(--color-primary)}

.active-filter-tags{display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin-bottom:12px;padding:6px 10px;background:var(--bg-base);border-radius:var(--radius-md)}
.aft-label{font-size:11px;color:var(--text-tertiary);font-weight:500}
.ftag{display:inline-flex;align-items:center;gap:4px;font-size:11px;background:rgba(99,102,241,.15);color:#818CF8;padding:2px 8px;border-radius:12px}
.ftag-x{background:none;border:none;color:inherit;cursor:pointer;font-size:14px;line-height:1;padding:0 2px;opacity:.7}
.ftag-x:hover{opacity:1}
.btn-link{background:none;border:none;color:var(--text-tertiary);font-size:11px;cursor:pointer;text-decoration:underline}
.btn-link:hover{color:var(--text-primary)}

.skeleton-list{display:flex;flex-direction:column;gap:12px}
.skeleton-card{background:var(--bg-elevated);border-radius:var(--radius-lg);padding:16px}
.sk-line{height:12px;background:var(--border-subtle);border-radius:6px;margin-bottom:8px;animation:skp 1.5s ease-in-out infinite}
.sk-w30{width:30%}.sk-w40{width:40%}.sk-w60{width:60%}.sk-w80{width:80%}
@keyframes skp{0%,100%{opacity:.4}50%{opacity:1}}

.empty-state{text-align:center;padding:32px}
.empty-icon{font-size:3rem;margin-bottom:8px}
.empty-title{font-size:18px;font-weight:600;color:var(--text-primary);margin-bottom:4px}
.empty-desc{font-size:13px;color:var(--text-tertiary)}

.session-list{display:flex;flex-direction:column;gap:12px}
.session-card{background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);overflow:hidden;transition:all .15s;border-left:4px solid var(--border-subtle)}
.session-card:hover{border-color:var(--color-primary)}
.session-card.risk-critical{border-left-color:#EF4444}.session-card.risk-high{border-left-color:#F97316}.session-card.risk-medium{border-left-color:#EAB308}.session-card.risk-low{border-left-color:#22C55E}
.session-card.has-sec{box-shadow:inset 0 0 0 1px rgba(239,68,68,.12)}
.sc-main{padding:16px;cursor:pointer;transition:background .15s}
.sc-main:hover{background:var(--bg-base)}
.sc-top{display:flex;justify-content:space-between;align-items:center;margin-bottom:8px;flex-wrap:wrap;gap:8px}
.sc-top-l{display:flex;align-items:center;gap:8px}
.sc-trace{font-size:13px;color:var(--color-primary);font-weight:600}
.sc-top-r{display:flex;align-items:center;gap:12px;font-size:11px;color:var(--text-tertiary)}
.sc-dur{display:flex;align-items:center;gap:4px;font-family:var(--font-mono)}
.sc-time{color:var(--text-tertiary)}
.mono{font-family:var(--font-mono)}
.sc-meta{display:flex;gap:12px;font-size:11px;color:var(--text-secondary);margin-bottom:12px;flex-wrap:wrap}
.meta-item{display:flex;align-items:center;gap:4px}
.meta-sub{color:var(--text-tertiary);font-size:10px}
.meta-dept{color:#0d9488;background:rgba(20,184,166,0.1);border-radius:4px;padding:1px 6px}
.sc-stats{display:flex;gap:12px;margin-bottom:8px}
.chip{display:flex;align-items:center;gap:2px;font-size:13px}
.chip-l{font-size:10px;font-weight:500;color:var(--text-tertiary);text-transform:uppercase;letter-spacing:.05em;margin-right:2px}
.chip-v{font-weight:600;color:var(--text-primary);font-family:var(--font-mono)}
.sc-flags{display:flex;flex-wrap:wrap;gap:8px;margin-bottom:8px}
.flag-danger{display:flex;align-items:center;gap:4px;font-size:11px;color:#EF4444;font-weight:600;background:rgba(239,68,68,.1);padding:2px 8px;border-radius:10px}
.flag-warn{display:flex;align-items:center;gap:4px;font-size:11px;color:#F97316;font-weight:600;background:rgba(249,115,22,.1);padding:2px 8px;border-radius:10px}
.sc-tags{display:flex;flex-wrap:wrap;gap:4px}
.tag-pill{font-size:10px;background:rgba(255,255,255,.08);padding:2px 6px;border-radius:8px;color:var(--text-tertiary)}
.sc-footer{display:flex;justify-content:space-between;align-items:center;padding:0 16px 12px;gap:8px}
.btn-expand{display:flex;align-items:center;gap:4px;background:none;border:1px dashed var(--border-subtle);border-radius:var(--radius-md);color:var(--text-tertiary);font-size:11px;padding:4px 10px;cursor:pointer;transition:all .15s}
.btn-expand:hover{color:var(--color-primary);border-color:var(--color-primary)}
.expand-arrow{display:inline-block;transition:transform .2s}.expand-arrow.rotated{transform:rotate(180deg)}
.btn-play{display:flex;align-items:center;gap:4px;background:rgba(99,102,241,.12);color:var(--color-primary);border:none;border-radius:var(--radius-md);padding:6px 12px;font-size:11px;font-weight:600;cursor:pointer;transition:all .15s}
.btn-play:hover{background:var(--color-primary);color:#fff}
.preview-panel{border-top:1px solid var(--border-subtle);padding:12px 16px;background:var(--bg-base)}
.pv-loading,.pv-empty{font-size:12px;color:var(--text-tertiary);text-align:center;padding:8px}
.pv-msgs{display:flex;flex-direction:column;gap:6px}
.pv-msg{display:flex;align-items:center;gap:8px;font-size:12px;padding:4px 8px;border-radius:6px}
.pv-im_inbound{background:rgba(59,130,246,.06)}.pv-im_outbound{background:rgba(34,197,94,.06)}
.pv-blocked{background:rgba(239,68,68,.06)!important}
.pv-dir{font-weight:600;color:var(--text-secondary);white-space:nowrap;min-width:60px}
.pv-text{flex:1;color:var(--text-primary);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.pv-lt{text-decoration:line-through;color:var(--text-tertiary)}
.pv-time{font-size:10px;color:var(--text-tertiary);font-family:var(--font-mono);white-space:nowrap}
.pv-act{font-size:10px;font-weight:700;padding:1px 6px;border-radius:6px;text-transform:uppercase}
.pa-block{background:rgba(239,68,68,.15);color:#EF4444}
.pa-warn{background:rgba(234,179,8,.15);color:#EAB308}
.pv-more{font-size:11px;color:var(--text-tertiary);text-align:center;padding:4px}
.risk-badge{font-size:11px;font-weight:600;padding:2px 8px;border-radius:12px}
.badge-critical{background:rgba(239,68,68,.15);color:#EF4444}.badge-high{background:rgba(249,115,22,.15);color:#F97316}.badge-medium{background:rgba(234,179,8,.15);color:#EAB308}.badge-low{background:rgba(34,197,94,.15);color:#22C55E}

.pagination{display:flex;align-items:center;justify-content:center;gap:12px;margin-top:16px;padding-top:12px;border-top:1px solid var(--border-subtle)}
.page-nums{display:flex;gap:4px}
.pg-btn{min-width:32px;height:32px;display:flex;align-items:center;justify-content:center;border:1px solid var(--border-subtle);background:var(--bg-elevated);color:var(--text-secondary);border-radius:var(--radius-sm);font-size:12px;cursor:pointer;transition:all .15s}
.pg-btn:hover{border-color:var(--color-primary);color:var(--color-primary)}
.pg-btn.active{background:var(--color-primary);color:#fff;border-color:var(--color-primary)}
.pg-btn.ellipsis{border:none;background:none;cursor:default}
.pg-info{font-size:11px;color:var(--text-tertiary)}
.hl-match{background:rgba(99,102,241,.3);color:var(--color-primary);padding:0 2px;border-radius:2px}
</style>
