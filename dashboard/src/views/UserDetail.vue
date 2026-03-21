<template>
  <div>
    <!-- Breadcrumb -->
    <div class="breadcrumb" style="margin-bottom:16px">
      <a class="breadcrumb-link" @click="$router.push('/user-profiles')">用户画像</a>
      <span class="breadcrumb-sep">›</span>
      <span class="breadcrumb-current">{{ userId }}</span>
    </div>

    <!-- User Header -->
    <div class="card user-header" v-if="loaded && profile">
      <div class="user-header-content">
        <div class="score-ring" :class="'ring-' + profile.risk_level">
          <svg viewBox="0 0 100 100" class="ring-svg">
            <circle cx="50" cy="50" r="42" fill="none" stroke="var(--bg-elevated)" stroke-width="8"/>
            <circle cx="50" cy="50" r="42" fill="none" :stroke="riskColor(profile.risk_level)" stroke-width="8"
              stroke-linecap="round" :stroke-dasharray="ringDash" stroke-dashoffset="0"
              transform="rotate(-90 50 50)" class="ring-progress"/>
          </svg>
          <div class="ring-center">
            <div class="ring-score">{{ profile.risk_score }}</div>
            <div class="ring-label">{{ riskLabel(profile.risk_level) }}</div>
          </div>
        </div>
        <div class="user-info">
          <div class="user-info-top">
            <h2 class="user-name">{{ profile.display_name || profile.user_id }}</h2>
            <div class="user-tags">
              <span class="tag" v-for="tag in userTags" :key="tag" :class="'tag-' + tag">{{ tagLabels[tag] || tag }}
                <button class="tag-remove" @click="removeTag(tag)">&times;</button>
              </span>
              <button class="tag-add-btn" @click="showTagMenu = !showTagMenu">+ 标签</button>
              <div v-if="showTagMenu" class="tag-menu">
                <button v-for="t in availableTags" :key="t.key" class="tag-menu-item" @click="addTag(t.key)">{{ t.icon }} {{ t.label }}</button>
              </div>
            </div>
          </div>
          <div class="user-meta">
            <span>首次出现: {{ fmtDate(profile.first_seen) }}</span><span>|</span>
            <span>最后活跃: {{ fmtTimeAgo(profile.last_seen) }}</span><span>|</span>
            <span>活跃天数: {{ profile.active_days }}天</span><span>|</span>
            <span>高峰时段: {{ profile.peak_hour }}:00</span>
          </div>
        </div>
      </div>
      <!-- 操作按钮 -->
      <div class="user-actions">
        <button v-if="!profile.banned" class="btn btn-sm btn-danger" @click="showBanConfirm = true">🚫 封禁用户</button>
        <button v-else class="btn btn-sm btn-success" @click="showUnbanConfirm = true">✅ 解封用户</button>
        <select v-model="manualRiskLevel" @change="setRiskLevel" class="risk-select">
          <option value="">手动设置风险等级</option>
          <option value="critical">极危</option>
          <option value="high">高危</option>
          <option value="medium">中危</option>
          <option value="low">低危</option>
        </select>
        <button class="btn btn-sm btn-ghost" @click="refreshProfile">🔄 刷新</button>
      </div>
    </div>
    <Skeleton v-else-if="!loaded" type="card" />
    <EmptyState v-else :iconSvg="svgUserX" title="用户不存在" description="未找到该用户的安全数据" />

    <!-- Risk Dimensions -->
    <div class="card" v-if="loaded && profile" style="margin-top:16px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg></span>
        <span class="card-title">风险维度分析</span>
      </div>
      <div class="dimensions">
        <div class="dim-row" v-for="d in dimensions" :key="d.label">
          <span class="dim-label">{{ d.label }}</span>
          <div class="dim-track"><div class="dim-fill" :style="{ width: d.pct + '%', background: d.color }"></div></div>
          <span class="dim-value">{{ d.display }}</span>
        </div>
      </div>
    </div>

    <!-- 关联数据: 会话 + 规则 -->
    <div class="ov-row" v-if="loaded && profile" style="margin-top:16px">
      <div class="card" style="flex:1">
        <div class="card-header"><span class="card-icon">📋</span><span class="card-title">关联会话</span></div>
        <div v-if="sessions.length" class="session-list">
          <div class="session-item" v-for="s in sessions" :key="s.trace_id" @click="$router.push('/sessions/' + s.trace_id)">
            <span class="session-id">{{ s.trace_id.slice(0, 8) }}...</span>
            <span class="session-time">{{ fmtShortTime(s.timestamp) }}</span>
            <span class="risk-badge" :class="'risk-' + (s.risk_level || 'low')">{{ s.risk_level || 'low' }}</span>
          </div>
        </div>
        <div v-else class="empty-mini">暂无关联会话</div>
      </div>
      <div class="card" style="flex:1">
        <div class="card-header"><span class="card-icon">🛡️</span><span class="card-title">触发规则</span></div>
        <div v-if="triggeredRules.length" class="rule-list">
          <div class="rule-item" v-for="r in triggeredRules" :key="r.name">
            <span class="rule-name">{{ r.name }}</span>
            <span class="rule-count">{{ r.count }}次</span>
            <span class="action-badge" :class="'action-' + r.action">{{ r.action }}</span>
          </div>
        </div>
        <div v-else class="empty-mini">暂无触发规则</div>
      </div>
    </div>

    <!-- Behavior Timeline -->
    <div class="card" v-if="loaded && profile" style="margin-top:16px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></span>
        <span class="card-title">行为时间线</span>
      </div>
      <Skeleton v-if="!timelineLoaded" type="text" />
      <EmptyState v-else-if="!timeline.length" :iconSvg="svgTimeline" title="暂无时间线数据" description="该用户无行为记录" />
      <div v-else class="timeline">
        <div class="tl-item" v-for="(evt, i) in timeline" :key="i">
          <div class="tl-left"><span class="tl-time">{{ fmtShortTime(evt.timestamp) }}</span></div>
          <div class="tl-dot-wrap">
            <div class="tl-dot" :class="'dot-' + evt.risk_level"></div>
            <div class="tl-line" v-if="i < timeline.length - 1"></div>
          </div>
          <div class="tl-content" :class="'tl-' + evt.risk_level" @click="toggleDetail(i)">
            <div class="tl-summary">
              <span class="tl-type-badge" :class="'type-' + evt.event_type">{{ eventTypeLabel(evt.event_type) }}</span>
              <span class="tl-text">{{ evt.summary }}</span>
            </div>
            <div v-if="expandedTimeline.has(i)" class="tl-detail"><pre class="tl-json">{{ formatDetails(evt.details) }}</pre></div>
          </div>
        </div>
        <div v-if="hasMore" class="tl-load-more"><button class="btn btn-ghost btn-sm" @click="loadMore">加载更多</button></div>
      </div>
    </div>

    <!-- Confirm Modals -->
    <ConfirmModal :visible="showBanConfirm" title="封禁用户" :message="'确定封禁用户 ' + userId + '？封禁后该用户所有请求将被拦截。'" type="danger" confirmText="确认封禁" @confirm="banUser" @cancel="showBanConfirm=false" />
    <ConfirmModal :visible="showUnbanConfirm" title="解封用户" :message="'确定解封用户 ' + userId + '？'" type="info" confirmText="确认解封" @confirm="unbanUser" @cancel="showUnbanConfirm=false" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, apiPost } from '../api.js'
import { showToast } from '../stores/app.js'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const route = useRoute()
const router = useRouter()
const userId = route.params.id

const loaded = ref(false), timelineLoaded = ref(false), profile = ref(null)
const timeline = ref([]), hasMore = ref(false), expandedTimeline = reactive(new Set()), timelineLimit = ref(30)
const showBanConfirm = ref(false), showUnbanConfirm = ref(false), manualRiskLevel = ref('')
const showTagMenu = ref(false)
const userTags = ref([])
const sessions = ref([])
const triggeredRules = ref([])

const availableTags = [
  { key: 'suspicious', label: '可疑', icon: '🔍' },
  { key: 'confirmed', label: '已确认威胁', icon: '⚠️' },
  { key: 'whitelist', label: '白名单', icon: '✅' },
  { key: 'vip', label: 'VIP用户', icon: '⭐' },
  { key: 'bot', label: '机器人', icon: '🤖' },
]
const tagLabels = { suspicious: '🔍 可疑', confirmed: '⚠️ 已确认', whitelist: '✅ 白名单', vip: '⭐ VIP', bot: '🤖 Bot' }

const svgUserX = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="18" y1="8" x2="23" y2="13"/><line x1="23" y1="8" x2="18" y2="13"/></svg>'
const svgTimeline = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>'

function riskColor(l){return{critical:'#EF4444',high:'#F59E0B',medium:'#3B82F6',low:'#6B7280'}[l]||'#6B7280'}
function riskLabel(l){return{critical:'极危',high:'高危',medium:'中危',low:'低危'}[l]||l}
function eventTypeLabel(t){return{'im_blocked':'拦截','im_request':'请求','llm_call':'LLM调用','tool_call':'工具调用','canary_leak':'泄露','budget_violation':'超限'}[t]||t}

const ringDash = computed(()=>{if(!profile.value)return'0 264';const pct=profile.value.risk_score/100;const c=2*Math.PI*42;return`${c*pct} ${c*(1-pct)}`})

const dimensions = computed(()=>{
  if(!profile.value)return[]
  const p=profile.value
  return[
    {label:'拦截率',pct:Math.min(100,p.block_rate*100),display:(p.block_rate*100).toFixed(1)+'%',color:p.block_rate>0.3?'#EF4444':p.block_rate>0.1?'#F59E0B':'#3B82F6'},
    {label:'注入尝试',pct:Math.min(100,p.injection_attempts*4),display:p.injection_attempts+'次',color:p.injection_attempts>10?'#EF4444':p.injection_attempts>3?'#F59E0B':'#3B82F6'},
    {label:'高危工具',pct:Math.min(100,p.high_risk_tools*8),display:p.high_risk_tools+'次',color:p.high_risk_tools>5?'#EF4444':p.high_risk_tools>2?'#F59E0B':'#3B82F6'},
    {label:'Canary泄露',pct:Math.min(100,p.canary_leaks*25),display:p.canary_leaks+'次',color:p.canary_leaks>0?'#EF4444':'#6B7280'},
    {label:'异常时段',pct:Math.min(100,p.off_hours_rate*100),display:(p.off_hours_rate*100).toFixed(1)+'%',color:p.off_hours_rate>0.5?'#EF4444':p.off_hours_rate>0.2?'#F59E0B':'#6B7280'},
  ]
})

function fmtDate(ts){if(!ts)return'--';const d=new Date(ts);return isNaN(d.getTime())?'--':d.toLocaleDateString('zh-CN')}
function fmtTimeAgo(ts){if(!ts)return'--';const d=new Date(ts);if(isNaN(d.getTime()))return'--';const diff=Date.now()-d.getTime();if(diff<60000)return'刚刚';if(diff<3600000)return Math.floor(diff/60000)+'分钟前';if(diff<86400000)return Math.floor(diff/3600000)+'小时前';return Math.floor(diff/86400000)+'天前'}
function fmtShortTime(ts){if(!ts)return'';const d=new Date(ts);if(isNaN(d.getTime()))return'';return d.toLocaleString('zh-CN',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false})}
function toggleDetail(idx){expandedTimeline.has(idx)?expandedTimeline.delete(idx):expandedTimeline.add(idx)}
function formatDetails(s){if(!s)return'';try{return JSON.stringify(JSON.parse(s),null,2)}catch{return s}}

function addTag(key){showTagMenu.value=false;if(!userTags.value.includes(key)){userTags.value.push(key);showToast('标签已添加','success')}}
function removeTag(key){userTags.value=userTags.value.filter(t=>t!==key);showToast('标签已移除','success')}

async function banUser(){showBanConfirm.value=false;if(profile.value)profile.value.banned=true;showToast('用户已封禁: '+userId,'success')}
async function unbanUser(){showUnbanConfirm.value=false;if(profile.value)profile.value.banned=false;showToast('用户已解封: '+userId,'success')}
function setRiskLevel(){if(!manualRiskLevel.value)return;if(profile.value)profile.value.risk_level=manualRiskLevel.value;showToast('风险等级已设置为 '+riskLabel(manualRiskLevel.value),'success');manualRiskLevel.value=''}

async function refreshProfile(){
  try{const p=await api('/api/v1/users/risk/'+encodeURIComponent(userId));profile.value=p;showToast('画像已刷新','success')}
  catch{showToast('刷新失败','error')}
}

async function loadTimeline(){
  try{const d=await api('/api/v1/users/timeline/'+encodeURIComponent(userId)+'?limit='+timelineLimit.value);timeline.value=d.events||[];hasMore.value=(d.events||[]).length>=timelineLimit.value}catch{timeline.value=[]}
  timelineLoaded.value=true
}
function loadMore(){timelineLimit.value+=30;loadTimeline()}

async function loadSessions(){
  try{const d=await api('/api/v1/users/timeline/'+encodeURIComponent(userId)+'?limit=5');sessions.value=(d.events||[]).filter(e=>e.trace_id).slice(0,5).map(e=>({trace_id:e.trace_id,timestamp:e.timestamp,risk_level:e.risk_level}))}catch{sessions.value=[]}
}

onMounted(async()=>{
  try{profile.value=await api('/api/v1/users/risk/'+encodeURIComponent(userId))}catch{profile.value=null}
  loaded.value=true
  loadTimeline()
  loadSessions()
  // Mock triggered rules for demo
  triggeredRules.value=[{name:'注入检测规则',count:3,action:'block'},{name:'敏感操作审计',count:7,action:'log'},{name:'异常频率告警',count:1,action:'warn'}]
})
</script>

<style scoped>
.breadcrumb{display:flex;align-items:center;gap:8px;font-size:var(--text-sm)}
.breadcrumb-link{color:var(--color-primary);cursor:pointer;text-decoration:none}.breadcrumb-link:hover{text-decoration:underline}
.breadcrumb-sep{color:var(--text-tertiary)}.breadcrumb-current{color:var(--text-primary);font-weight:600}

.user-header{padding:var(--space-5)}
.user-header-content{display:flex;align-items:center;gap:24px}
.user-actions{display:flex;gap:8px;margin-top:16px;padding-top:12px;border-top:1px solid var(--border-subtle);flex-wrap:wrap;align-items:center}

.score-ring{position:relative;width:100px;height:100px;flex-shrink:0}
.ring-svg{width:100%;height:100%}.ring-progress{transition:stroke-dasharray .8s ease}
.ring-center{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center}
.ring-score{font-size:24px;font-weight:800;color:var(--text-primary)}.ring-label{font-size:11px;font-weight:600;text-transform:uppercase}
.ring-critical .ring-label{color:#EF4444}.ring-high .ring-label{color:#F59E0B}.ring-medium .ring-label{color:#3B82F6}.ring-low .ring-label{color:#6B7280}

.user-info{flex:1}.user-info-top{display:flex;align-items:center;gap:12px;flex-wrap:wrap;margin-bottom:8px}
.user-name{font-size:1.25rem;font-weight:700;color:var(--text-primary);margin:0}
.user-meta{display:flex;flex-wrap:wrap;gap:8px;font-size:var(--text-sm);color:var(--text-secondary)}

.user-tags{display:flex;gap:6px;flex-wrap:wrap;align-items:center;position:relative}
.tag{display:inline-flex;align-items:center;gap:4px;padding:2px 8px;border-radius:9999px;font-size:var(--text-xs);font-weight:600}
.tag-suspicious{background:rgba(245,158,11,.15);color:#F59E0B}.tag-confirmed{background:rgba(239,68,68,.15);color:#EF4444}
.tag-whitelist{background:rgba(34,197,94,.15);color:#22C55E}.tag-vip{background:rgba(99,102,241,.15);color:#6366F1}
.tag-bot{background:rgba(107,114,128,.15);color:#6B7280}
.tag-remove{background:none;border:none;color:inherit;cursor:pointer;font-size:12px;line-height:1;padding:0 2px;opacity:.6}.tag-remove:hover{opacity:1}
.tag-add-btn{background:none;border:1px dashed var(--border-subtle);color:var(--text-tertiary);padding:2px 8px;border-radius:9999px;font-size:var(--text-xs);cursor:pointer}
.tag-add-btn:hover{border-color:var(--color-primary);color:var(--color-primary)}
.tag-menu{position:absolute;top:100%;left:0;z-index:50;background:var(--bg-overlay);border:1px solid var(--border-default);border-radius:var(--radius-md);padding:4px 0;min-width:140px;box-shadow:var(--shadow-lg);margin-top:4px}
.tag-menu-item{display:block;width:100%;text-align:left;padding:6px 12px;font-size:var(--text-sm);color:var(--text-primary);background:none;border:none;cursor:pointer}
.tag-menu-item:hover{background:var(--bg-elevated)}

.btn-danger{background:#EF4444;color:#fff;border-color:#EF4444}.btn-danger:hover{opacity:.85}
.btn-success{background:#22C55E;color:#fff;border-color:#22C55E}.btn-success:hover{opacity:.85}
.risk-select{padding:4px 10px;background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-xs);cursor:pointer}
.risk-select:focus{outline:none;border-color:var(--color-primary)}

.dimensions{display:flex;flex-direction:column;gap:12px;padding:var(--space-2) 0}
.dim-row{display:flex;align-items:center;gap:12px}
.dim-label{width:80px;font-size:var(--text-sm);color:var(--text-secondary);text-align:right;flex-shrink:0}
.dim-track{flex:1;height:20px;background:var(--bg-elevated);border-radius:10px;overflow:hidden}
.dim-fill{height:100%;border-radius:10px;transition:width .6s ease}
.dim-value{width:60px;font-size:var(--text-sm);font-weight:600;color:var(--text-primary)}

.session-list{display:flex;flex-direction:column;gap:4px}
.session-item{display:flex;align-items:center;gap:8px;padding:6px 10px;border-radius:var(--radius-sm);cursor:pointer;transition:background .15s;font-size:var(--text-sm)}
.session-item:hover{background:var(--bg-elevated)}
.session-id{font-family:var(--font-mono);font-size:var(--text-xs);color:var(--color-primary)}
.session-time{color:var(--text-tertiary);font-size:var(--text-xs);margin-left:auto}
.rule-list{display:flex;flex-direction:column;gap:4px}
.rule-item{display:flex;align-items:center;gap:8px;padding:6px 10px;font-size:var(--text-sm)}
.rule-name{flex:1;color:var(--text-primary)}.rule-count{font-size:var(--text-xs);color:var(--text-tertiary)}
.action-badge{display:inline-block;padding:2px 8px;border-radius:9999px;font-size:10px;font-weight:600;text-transform:uppercase}
.action-log{background:rgba(107,114,128,.15);color:#6B7280}.action-warn{background:rgba(245,158,11,.15);color:#F59E0B}.action-block{background:rgba(239,68,68,.15);color:#EF4444}
.empty-mini{padding:20px;text-align:center;color:var(--text-tertiary);font-size:var(--text-sm)}

.risk-badge{display:inline-block;padding:2px 8px;border-radius:9999px;font-size:var(--text-xs);font-weight:600}
.risk-low{background:rgba(107,114,128,.15);color:#6B7280}.risk-medium{background:rgba(59,130,246,.15);color:#3B82F6}
.risk-high{background:rgba(245,158,11,.15);color:#F59E0B}.risk-critical{background:rgba(239,68,68,.15);color:#EF4444}

.timeline{padding:var(--space-2) 0}
.tl-item{display:flex;gap:0;min-height:50px}
.tl-left{width:100px;text-align:right;padding-right:12px;flex-shrink:0}
.tl-time{font-size:11px;color:var(--text-tertiary);font-family:var(--font-mono);white-space:nowrap}
.tl-dot-wrap{display:flex;flex-direction:column;align-items:center;width:20px;flex-shrink:0}
.tl-dot{width:10px;height:10px;border-radius:50%;flex-shrink:0;margin-top:4px}
.dot-critical{background:#EF4444;box-shadow:0 0 6px rgba(239,68,68,.4)}.dot-high{background:#F59E0B;box-shadow:0 0 6px rgba(245,158,11,.4)}
.dot-medium{background:#3B82F6}.dot-low{background:#6B7280}
.tl-line{flex:1;width:2px;background:var(--border-subtle);margin-top:4px}
.tl-content{flex:1;padding:4px 12px 12px;margin-left:4px;cursor:pointer;border-radius:var(--radius-sm);transition:background .15s}
.tl-content:hover{background:var(--bg-elevated)}
.tl-critical{border-left:2px solid #EF4444}.tl-high{border-left:2px solid #F59E0B}.tl-medium{border-left:2px solid #3B82F6}.tl-low{border-left:2px solid transparent}
.tl-summary{display:flex;align-items:center;gap:8px;font-size:var(--text-sm)}
.tl-type-badge{display:inline-block;padding:1px 6px;border-radius:4px;font-size:10px;font-weight:600;flex-shrink:0}
.type-im_blocked{background:rgba(239,68,68,.15);color:#EF4444}.type-im_request{background:rgba(107,114,128,.15);color:#6B7280}
.type-llm_call{background:rgba(59,130,246,.15);color:#3B82F6}.type-tool_call{background:rgba(245,158,11,.15);color:#F59E0B}
.type-canary_leak{background:rgba(217,119,6,.15);color:#D97706}.type-budget_violation{background:rgba(234,88,12,.15);color:#EA580C}
.tl-text{color:var(--text-primary)}.tl-detail{margin-top:8px}
.tl-json{font-size:11px;font-family:var(--font-mono);color:var(--text-secondary);background:var(--bg-base);padding:8px 12px;border-radius:var(--radius-sm);overflow-x:auto;white-space:pre-wrap;margin:0}
.tl-load-more{text-align:center;padding:var(--space-3)}
</style>