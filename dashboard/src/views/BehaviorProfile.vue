<template>
  <div @click="openMenu = null">
    <div class="page-header">
      <div class="page-header-left">
        <h2 class="page-title"><Icon name="behavior" :size="20" /> Agent 行为画像</h2>
        <p class="page-desc">学习 Agent 正常行为模式，检测语义行为突变</p>
      </div>
      <div class="page-header-actions">
        <button class="btn btn-ghost btn-sm" @click.stop="showStrategy = !showStrategy">⚙️ 策略配置</button>
        <button class="btn btn-ghost btn-sm" @click="exportProfiles" :disabled="!profiles.length">📤 导出</button>
        <button class="btn btn-primary btn-sm" @click="scanAll" :disabled="scanningAll">{{ scanningAll ? '扫描中...' : '🔍 全量扫描' }}</button>
      </div>
    </div>

    <div v-if="showStrategy" class="card strategy-panel" style="margin-bottom:16px">
      <div class="card-header"><span class="card-icon">⚙️</span><span class="card-title">行为分析策略配置</span></div>
      <div class="strategy-grid">
        <div class="strategy-group">
          <div class="strategy-group-title">异常阈值</div>
          <div class="strategy-row"><label class="strategy-label">异常请求频率</label><div class="strategy-input-wrap"><input type="number" v-model.number="strategy.abnormalReqRate" class="strategy-input" min="1" max="1000" /><span class="strategy-unit">次/小时</span></div></div>
          <div class="strategy-row"><label class="strategy-label">敏感操作阈值</label><div class="strategy-input-wrap"><input type="number" v-model.number="strategy.sensitiveOpLimit" class="strategy-input" min="1" max="100" /><span class="strategy-unit">次/天</span></div></div>
          <div class="strategy-row"><label class="strategy-label">突变检测灵敏度</label><select v-model="strategy.sensitivity" class="filter-select"><option value="low">低 — 仅重大偏差</option><option value="medium">中 — 平衡检测</option><option value="high">高 — 敏感检测</option></select></div>
        </div>
        <div class="strategy-group">
          <div class="strategy-group-title">风险评分权重</div>
          <div class="strategy-row" v-for="w in weightItems" :key="w.key"><label class="strategy-label">{{ w.label }}</label><div class="strategy-slider-wrap"><input type="range" v-model.number="strategy.weights[w.key]" min="0" max="100" class="strategy-slider" /><span class="strategy-slider-val">{{ strategy.weights[w.key] }}%</span></div></div>
        </div>
      </div>
      <div class="strategy-footer"><button class="btn btn-ghost btn-sm" @click="resetStrategy">恢复默认</button><button class="btn btn-primary btn-sm" @click="saveStrategy">💾 保存策略</button></div>
    </div>

    <div class="ov-cards" v-if="loaded">
      <StatCard :iconSvg="svgAgent" :value="stats.totalProfiles" label="Agent Profiled" color="blue" />
      <StatCard :iconSvg="svgAlert" :value="stats.totalAnomalies" label="突变 Anomalies" color="red" />
      <StatCard :iconSvg="svgRisk" :value="stats.highRiskCount" label="高风险 High Risk" color="yellow" />
      <StatCard :iconSvg="svgPattern" :value="stats.totalPatterns" label="行为模式 Patterns" color="purple" />
    </div>
    <div class="ov-cards" v-else><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /></div>

    <div class="filter-bar card" style="margin-bottom:16px">
      <div class="filter-bar-inner">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="searchText" class="search-input" placeholder="搜索 Agent ID 或名称..." />
          <button v-if="searchText" class="search-clear" @click="searchText=''">&times;</button>
        </div>
        <div class="filter-selects">
          <select v-model="filterRisk" class="filter-select"><option value="">全部风险</option><option value="critical">极危</option><option value="high">高风险</option><option value="elevated">轻微异常</option><option value="normal">正常</option></select>
          <select v-model="sortBy" class="filter-select"><option value="risk">按风险</option><option value="requests">按请求量</option><option value="anomalies">按异常数</option><option value="recent">按最近活跃</option></select>
        </div>
        <span class="filter-count">{{ filteredProfiles.length }}/{{ profiles.length }}</span>
      </div>
    </div>

    <div v-if="loaded && filteredProfiles.length > 0">
      <div class="profile-card" v-for="p in pagedProfiles" :key="p.agent_id" :class="cardClass(p)">
        <div class="profile-header">
          <div class="profile-id">
            <a class="profile-name link-accent" @click.stop="$router.push('/user-profiles/' + encodeURIComponent(p.agent_id))">{{ p.agent_id }}</a>
            <span class="profile-display" v-if="p.display_name && p.display_name !== p.agent_id">{{ p.display_name }}</span>
          </div>
          <div class="profile-header-right">
            <span class="risk-badge" :class="'risk-' + p.risk_level">{{ riskIcon(p.risk_level) }} {{ riskLabel(p.risk_level) }}</span>
            <div class="profile-dropdown" @click.stop>
              <button class="btn-icon" @click="toggleMenu(p.agent_id)">⋮</button>
              <div v-if="openMenu === p.agent_id" class="dropdown-menu">
                <button @click="markRisk(p.agent_id,'high')" class="dropdown-item">🔴 标记高风险</button>
                <button @click="markRisk(p.agent_id,'normal')" class="dropdown-item">🟢 标记正常</button>
                <button @click="exportSingle(p)" class="dropdown-item">📤 导出画像</button>
              </div>
            </div>
          </div>
        </div>
        <div class="profile-stats">
          <span>总请求: <b>{{ p.total_requests }}</b></span>
          <span v-if="p.avg_tokens > 0">平均Token: <b>{{ Math.round(p.avg_tokens) }}</b></span>
          <span v-if="p.avg_tools_per_req > 0">工具/请求: <b>{{ p.avg_tools_per_req.toFixed(1) }}</b></span>
          <span v-if="p.peak_hours && p.peak_hours.length">活跃: <b>{{ p.peak_hours.map(h => h+':00').join('-') }}</b></span>
        </div>
        <div v-if="p.typical_tools && p.typical_tools.length" class="profile-section">
          <div class="section-label">常用工具</div>
          <div class="tool-bar" v-for="tool in p.typical_tools.slice(0,6)" :key="tool.tool_name">
            <span class="tool-name" :title="tool.tool_name">{{ tool.tool_name }}</span>
            <div class="tool-track"><div class="tool-fill" :style="{ width: Math.max(3, tool.percentage)+'%', background: toolColor(tool.tool_name) }"></div></div>
            <span class="tool-pct">{{ tool.percentage.toFixed(0) }}%</span>
          </div>
        </div>
        <div class="profile-section expand-section" v-if="expandedCards.has(p.agent_id)">
          <div v-if="p.common_patterns && p.common_patterns.length" style="margin-bottom:16px">
            <div class="section-label">行为模式序列</div>
            <div class="pattern-item" v-for="(pat, pi) in p.common_patterns.slice(0,8)" :key="pi">
              <span class="pattern-dot" :style="{ background: riskColor(pat.risk_score) }"></span>
              <span class="pattern-seq">{{ pat.sequence.join(' → ') }}</span>
              <span class="pattern-count">({{ pat.count }}次</span>
              <span class="pattern-risk" :style="{ color: riskColor(pat.risk_score) }">{{ pat.risk_score > 50 ? '高风险' : pat.risk_score > 20 ? '中风险' : '低风险' }})</span>
            </div>
          </div>
          <div v-if="p.typical_tools && p.typical_tools.length" style="margin-bottom:16px">
            <div class="section-label">操作类型分布</div>
            <div class="op-type-grid">
              <div class="op-type-item" v-for="cat in getToolCategories(p.typical_tools)" :key="cat.name">
                <span class="op-type-label">{{ cat.name }}</span>
                <div class="op-type-track"><div class="op-type-fill" :style="{ width: cat.pct+'%', background: cat.color }"></div></div>
                <span class="op-type-val">{{ cat.pct.toFixed(0) }}%</span>
              </div>
            </div>
          </div>
          <div v-if="p.peak_hours && p.peak_hours.length">
            <div class="section-label">24h 活跃分布</div>
            <div class="hour-grid">
              <div class="hour-cell" v-for="h in 24" :key="h-1" :class="{'hour-active': p.peak_hours.includes(h-1)}" :title="(h-1)+':00'">{{ h-1 }}</div>
            </div>
          </div>
        </div>
        <div v-if="p.anomalies && p.anomalies.length" class="profile-section anomaly-section">
          <div class="section-label">⚠️ 突变告警 ({{ p.anomalies.length }})</div>
          <div class="anomaly-item" v-for="a in p.anomalies.slice(0,5)" :key="a.id">
            <span class="anomaly-severity" :class="'sev-'+a.severity">{{ sevIcon(a.severity) }}</span>
            <span class="anomaly-desc">{{ a.description }}</span>
            <a class="link-accent" style="font-size:11px;margin-left:auto" @click.stop="$router.push('/attack-chains')">🔗 攻击链→</a>
          </div>
        </div>
        <div class="profile-actions">
          <button class="btn-sm btn-primary" @click="scanAgent(p.agent_id)" :disabled="scanning===p.agent_id">{{ scanning===p.agent_id ? '扫描中...' : '🔍 扫描' }}</button>
          <button class="btn-sm btn-ghost" @click="toggleExpand(p.agent_id)">{{ expandedCards.has(p.agent_id) ? '收起 ▲' : '展开 ▼' }}</button>
          <button class="btn-sm btn-ghost" @click="showDetail(p.agent_id)">完整画像</button>
          <button v-if="p.anomalies && p.anomalies.length && p.anomalies.some(a=>a.trace_id)" class="btn-sm btn-ghost" @click="goToReplay(p.anomalies.find(a=>a.trace_id)?.trace_id)">📹 回放</button>
        </div>
      </div>
      <div v-if="filteredProfiles.length > pageSize" class="pagination">
        <button class="btn btn-ghost btn-sm" :disabled="currentPage<=1" @click="currentPage--">上一页</button>
        <span class="page-info">{{ currentPage }}/{{ totalPages }}</span>
        <button class="btn btn-ghost btn-sm" :disabled="currentPage>=totalPages" @click="currentPage++">下一页</button>
      </div>
    </div>
    <EmptyState v-else-if="loaded && !profiles.length" :iconSvg="svgBrain" title="暂无 Agent 画像" description="注入演示数据或等待 Agent 活动后自动生成画像" />
    <EmptyState v-else-if="loaded && profiles.length && !filteredProfiles.length" :iconSvg="svgBrain" title="无匹配结果" description="尝试调整搜索条件或过滤器" />
    <Skeleton v-else type="table" />

    <div v-if="detailProfile" class="modal-overlay" @click.self="detailProfile=null">
      <div class="modal-content">
        <div class="modal-header"><h3>{{ detailProfile.agent_id }} — 详细画像</h3><button class="btn-close" @click="detailProfile=null">✕</button></div>
        <div class="modal-body">
          <div class="detail-kv"><span class="dl">风险等级</span><span class="risk-badge" :class="'risk-'+detailProfile.risk_level">{{ riskLabel(detailProfile.risk_level) }}</span></div>
          <div class="detail-kv"><span class="dl">总请求</span><span>{{ detailProfile.total_requests }}</span></div>
          <div class="detail-kv"><span class="dl">平均 Token</span><span>{{ Math.round(detailProfile.avg_tokens) }}</span></div>
          <div class="detail-kv"><span class="dl">平均工具/请求</span><span>{{ detailProfile.avg_tools_per_req?.toFixed(2) }}</span></div>
          <div class="detail-kv"><span class="dl">活跃时段</span><span>{{ (detailProfile.peak_hours||[]).map(h=>h+':00').join(', ')||'无数据' }}</span></div>
          <div class="detail-kv"><span class="dl">首次活跃</span><span>{{ fmtTime(detailProfile.profiled_since) }}</span></div>
          <div class="detail-kv"><span class="dl">最近活跃</span><span>{{ fmtTime(detailProfile.last_seen) }}</span></div>
          <div v-if="detailProfile.typical_tools?.length" style="margin-top:16px">
            <div class="section-label">全部工具使用</div>
            <div class="tool-bar" v-for="tool in detailProfile.typical_tools" :key="tool.tool_name">
              <span class="tool-name">{{ tool.tool_name }}</span>
              <div class="tool-track"><div class="tool-fill" :style="{width:Math.max(3,tool.percentage)+'%',background:toolColor(tool.tool_name)}"></div></div>
              <span class="tool-pct">{{ tool.count }} ({{ tool.percentage.toFixed(1) }}%)</span>
            </div>
          </div>
          <div v-if="detailProfile.common_patterns?.length" style="margin-top:16px">
            <div class="section-label">全部行为模式</div>
            <div class="pattern-item" v-for="(pat,pi) in detailProfile.common_patterns" :key="pi">
              <span class="pattern-dot" :style="{background:riskColor(pat.risk_score)}"></span>
              <span class="pattern-seq">{{ pat.sequence.join(' → ') }}</span>
              <span class="pattern-count">({{ pat.count }}次, 风险={{ pat.risk_score }})</span>
            </div>
          </div>
          <div v-if="detailProfile.anomalies?.length" style="margin-top:16px">
            <div class="section-label">全部突变记录</div>
            <div class="anomaly-item" v-for="a in detailProfile.anomalies" :key="a.id">
              <span class="anomaly-severity" :class="'sev-'+a.severity">{{ sevIcon(a.severity) }}</span>
              <span class="anomaly-desc">{{ a.description }}</span>
              <span class="anomaly-time">{{ fmtTime(a.timestamp) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, reactive, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api, apiPost } from '../api.js'
import { showToast } from '../stores/app.js'
import StatCard from '../components/StatCard.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'
import Icon from '../components/Icon.vue'

const svgAgent='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><line x1="12" y1="7" x2="12" y2="11"/><line x1="8" y1="16" x2="8" y2="16.01"/><line x1="16" y1="16" x2="16" y2="16.01"/></svg>'
const svgAlert='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgRisk='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>'
const svgPattern='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgBrain='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 2a4 4 0 014 4c0 2.5-2 4-2 6h-4c0-2-2-3.5-2-6a4 4 0 014-4z"/><path d="M10 12h4"/><path d="M10 16h4"/></svg>'

const criticalTools=new Set(['exec','shell','bash','run_command','execute_command'])
const highTools=new Set(['write_file','edit_file','delete_file','write','edit','http_request','curl','send_email','send_message','message'])
const readTools=new Set(['read_file','read','list_directory','web_search','browser','web_fetch'])

function toolColor(n){return criticalTools.has(n)?'#EF4444':highTools.has(n)?'#F59E0B':'#3B82F6'}
function riskColor(s){return s>=50?'#EF4444':s>=20?'#F59E0B':'#3B82F6'}
function riskIcon(l){return{critical:'🔴',high:'🟠',elevated:'🟡',normal:'🟢'}[l]||'⚪'}
function riskLabel(l){return{critical:'极危',high:'高风险',elevated:'轻微异常',normal:'正常'}[l]||l}
function sevIcon(s){return{critical:'🔴',high:'🟠',medium:'🟡',low:'🔵'}[s]||'⚪'}
function fmtTime(ts){if(!ts)return'--';const d=new Date(ts);return isNaN(d.getTime())?String(ts):d.toLocaleString('zh-CN',{hour12:false})}
function cardClass(p){return{'profile-critical':p.risk_level==='critical','profile-high':p.risk_level==='high','profile-elevated':p.risk_level==='elevated'}}
const riskOrder={critical:0,high:1,elevated:2,normal:3}

function getToolCategories(tools){
  const cats={'执行类':{pct:0,color:'#EF4444'},'写入类':{pct:0,color:'#F59E0B'},'读取类':{pct:0,color:'#3B82F6'},'其他':{pct:0,color:'#6B7280'}}
  for(const t of tools){if(criticalTools.has(t.tool_name))cats['执行类'].pct+=t.percentage;else if(highTools.has(t.tool_name))cats['写入类'].pct+=t.percentage;else if(readTools.has(t.tool_name))cats['读取类'].pct+=t.percentage;else cats['其他'].pct+=t.percentage}
  return Object.entries(cats).filter(([,v])=>v.pct>0).map(([name,v])=>({name,...v}))
}

const router=useRouter()
const loaded=ref(false),profiles=ref([]),stats=ref({totalProfiles:0,totalAnomalies:0,highRiskCount:0,totalPatterns:0})
const scanning=ref(null),scanningAll=ref(false),detailProfile=ref(null)
const expandedCards=reactive(new Set()),openMenu=ref(null)
const searchText=ref(''),filterRisk=ref(''),sortBy=ref('risk'),currentPage=ref(1),pageSize=10
const showStrategy=ref(false)
const defaultStrategy={abnormalReqRate:100,sensitiveOpLimit:20,sensitivity:'medium',weights:{tool_risk:35,pattern_risk:25,anomaly_count:20,frequency:10,time_deviation:10}}
const strategy=reactive(JSON.parse(JSON.stringify(defaultStrategy)))
const weightItems=[{key:'tool_risk',label:'工具风险'},{key:'pattern_risk',label:'模式风险'},{key:'anomaly_count',label:'异常数量'},{key:'frequency',label:'请求频率'},{key:'time_deviation',label:'时间偏差'}]

const filteredProfiles=computed(()=>{
  let list=[...profiles.value]
  if(searchText.value){const q=searchText.value.toLowerCase();list=list.filter(p=>p.agent_id.toLowerCase().includes(q)||(p.display_name||'').toLowerCase().includes(q))}
  if(filterRisk.value)list=list.filter(p=>p.risk_level===filterRisk.value)
  if(sortBy.value==='risk')list.sort((a,b)=>(riskOrder[a.risk_level]??9)-(riskOrder[b.risk_level]??9))
  else if(sortBy.value==='requests')list.sort((a,b)=>(b.total_requests||0)-(a.total_requests||0))
  else if(sortBy.value==='anomalies')list.sort((a,b)=>(b.anomalies?.length||0)-(a.anomalies?.length||0))
  else if(sortBy.value==='recent')list.sort((a,b)=>new Date(b.last_seen||0)-new Date(a.last_seen||0))
  return list
})
const totalPages=computed(()=>Math.max(1,Math.ceil(filteredProfiles.value.length/pageSize)))
const pagedProfiles=computed(()=>{const s=(currentPage.value-1)*pageSize;return filteredProfiles.value.slice(s,s+pageSize)})
watch([searchText,filterRisk,sortBy],()=>{currentPage.value=1})

function toggleExpand(id){expandedCards.has(id)?expandedCards.delete(id):expandedCards.add(id)}
function toggleMenu(id){openMenu.value=openMenu.value===id?null:id}

async function loadData(){
  try{const d=await api('/api/v1/behavior/profiles');profiles.value=d.profiles||[];stats.value={totalProfiles:d.total||0,totalAnomalies:d.total_anomalies||0,highRiskCount:d.high_risk_count||0,totalPatterns:d.total_patterns||0}}catch{profiles.value=[]}
  loaded.value=true
}
async function scanAgent(id){scanning.value=id;try{await apiPost('/api/v1/behavior/profiles/'+encodeURIComponent(id)+'/scan');showToast('扫描完成: '+id,'success');await loadData()}catch(e){showToast('扫描失败: '+e.message,'error')}scanning.value=null}
async function scanAll(){scanningAll.value=true;try{for(const p of profiles.value)await apiPost('/api/v1/behavior/profiles/'+encodeURIComponent(p.agent_id)+'/scan');showToast('全量扫描完成','success');await loadData()}catch(e){showToast('扫描失败','error')}scanningAll.value=false}
function markRisk(id,level){openMenu.value=null;const p=profiles.value.find(x=>x.agent_id===id);if(p)p.risk_level=level;showToast('已标记 '+id+' 为 '+riskLabel(level),'success')}
async function showDetail(id){try{detailProfile.value=await api('/api/v1/behavior/profiles/'+encodeURIComponent(id))}catch{showToast('加载详情失败','error')}}
function goToReplay(tid){if(tid)router.push('/sessions/'+tid)}
function exportProfiles(){const blob=new Blob([JSON.stringify(profiles.value,null,2)],{type:'application/json'});const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download='behavior-profiles-'+new Date().toISOString().slice(0,10)+'.json';a.click();URL.revokeObjectURL(a.href);showToast('导出成功','success')}
function exportSingle(p){openMenu.value=null;const blob=new Blob([JSON.stringify(p,null,2)],{type:'application/json'});const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download='profile-'+p.agent_id+'.json';a.click();URL.revokeObjectURL(a.href)}
function resetStrategy(){Object.assign(strategy,JSON.parse(JSON.stringify(defaultStrategy)))}
function saveStrategy(){showToast('策略已保存','success');showStrategy.value=false}

let timer=null
onMounted(()=>{loadData();timer=setInterval(loadData,30000)})
onUnmounted(()=>clearInterval(timer))
</script>

<style scoped>
.page-header{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:20px;flex-wrap:wrap;gap:12px}
.page-header-left{flex:1;min-width:200px}
.page-title{font-size:var(--text-xl);font-weight:700;margin:0 0 4px 0}
.page-desc{font-size:var(--text-sm);color:var(--text-tertiary);margin:0}
.page-header-actions{display:flex;gap:8px;flex-wrap:wrap}

.strategy-panel{animation:slideDown .2s ease-out}
@keyframes slideDown{from{opacity:0;transform:translateY(-8px)}to{opacity:1;transform:translateY(0)}}
.strategy-grid{display:grid;grid-template-columns:1fr 1fr;gap:24px;padding:0 4px}
@media(max-width:768px){.strategy-grid{grid-template-columns:1fr}}
.strategy-group{display:flex;flex-direction:column;gap:12px}
.strategy-group-title{font-size:var(--text-xs);font-weight:700;text-transform:uppercase;letter-spacing:.05em;color:var(--text-tertiary);border-bottom:1px solid var(--border-subtle);padding-bottom:6px}
.strategy-row{display:flex;align-items:center;gap:12px}
.strategy-label{width:110px;font-size:var(--text-sm);color:var(--text-secondary);flex-shrink:0}
.strategy-input-wrap{display:flex;align-items:center;gap:6px}
.strategy-input{width:80px;padding:4px 8px;background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-sm);text-align:center}
.strategy-input:focus{outline:none;border-color:var(--color-primary)}
.strategy-unit{font-size:var(--text-xs);color:var(--text-tertiary)}
.strategy-slider-wrap{display:flex;align-items:center;gap:8px;flex:1}
.strategy-slider{flex:1;accent-color:var(--color-primary);height:4px}
.strategy-slider-val{font-size:var(--text-xs);font-weight:600;color:var(--color-primary);width:36px;text-align:right}
.strategy-footer{display:flex;justify-content:flex-end;gap:8px;margin-top:16px;padding-top:12px;border-top:1px solid var(--border-subtle)}

.filter-bar{padding:12px 16px}
.filter-bar-inner{display:flex;gap:12px;align-items:center;flex-wrap:wrap}
.search-box{position:relative;flex:1;min-width:200px}
.search-icon{position:absolute;left:10px;top:50%;transform:translateY(-50%);color:var(--text-tertiary);pointer-events:none}
.search-input{width:100%;padding:8px 30px 8px 32px;background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-sm)}
.search-input:focus{outline:none;border-color:var(--color-primary)}
.search-clear{position:absolute;right:8px;top:50%;transform:translateY(-50%);background:none;border:none;color:var(--text-tertiary);font-size:1.1rem;cursor:pointer;padding:0 4px}
.search-clear:hover{color:var(--text-primary)}
.filter-selects{display:flex;gap:8px;flex-wrap:wrap}
.filter-select{padding:6px 10px;background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-xs);cursor:pointer}
.filter-select:focus{outline:none;border-color:var(--color-primary)}
.filter-count{font-size:var(--text-xs);color:var(--text-tertiary);white-space:nowrap}

.profile-card{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:var(--space-4);margin-bottom:16px;transition:border-color .2s}
.profile-card:hover{border-color:var(--color-primary)}
.profile-critical{border-left:3px solid #EF4444}
.profile-high{border-left:3px solid #F59E0B}
.profile-elevated{border-left:3px solid #8B5CF6}

.profile-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:12px}
.profile-header-right{display:flex;align-items:center;gap:8px}
.profile-id{display:flex;align-items:center;gap:8px}
.profile-name{font-size:var(--text-base);font-weight:700;font-family:var(--font-mono)}
.profile-display{font-size:var(--text-sm);color:var(--text-tertiary)}
.profile-dropdown{position:relative}
.btn-icon{background:none;border:none;color:var(--text-tertiary);cursor:pointer;font-size:16px;padding:4px 8px;border-radius:var(--radius-sm)}
.btn-icon:hover{background:var(--bg-elevated);color:var(--text-primary)}
.dropdown-menu{position:absolute;right:0;top:100%;z-index:50;background:var(--bg-overlay);border:1px solid var(--border-default);border-radius:var(--radius-md);padding:4px 0;min-width:160px;box-shadow:var(--shadow-lg)}
.dropdown-item{display:block;width:100%;text-align:left;padding:6px 12px;font-size:var(--text-sm);color:var(--text-primary);background:none;border:none;cursor:pointer}
.dropdown-item:hover{background:var(--bg-elevated)}

.risk-badge{display:inline-block;padding:2px 10px;border-radius:9999px;font-size:var(--text-xs);font-weight:600}
.risk-normal{background:rgba(34,197,94,.15);color:#22C55E}
.risk-elevated{background:rgba(139,92,246,.15);color:#8B5CF6}
.risk-high{background:rgba(245,158,11,.15);color:#F59E0B}
.risk-critical{background:rgba(239,68,68,.15);color:#EF4444}

.profile-stats{display:flex;flex-wrap:wrap;gap:16px;font-size:var(--text-sm);color:var(--text-secondary);margin-bottom:16px;padding-bottom:12px;border-bottom:1px solid var(--border-subtle)}
.profile-stats b{color:var(--text-primary)}
.profile-section{margin-bottom:12px}
.section-label{font-size:11px;font-weight:600;color:var(--text-tertiary);text-transform:uppercase;letter-spacing:.05em;margin-bottom:8px}

.tool-bar{display:flex;align-items:center;gap:8px;margin-bottom:4px}
.tool-name{width:120px;font-size:var(--text-xs);font-family:var(--font-mono);color:var(--text-secondary);text-align:right;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex-shrink:0}
.tool-track{flex:1;height:16px;background:rgba(255,255,255,.05);border-radius:4px;overflow:hidden}
.tool-fill{height:100%;border-radius:4px;transition:width .6s ease-out;min-width:2px}
.tool-pct{width:40px;font-size:var(--text-xs);color:var(--text-tertiary);text-align:right;flex-shrink:0}

.expand-section{padding:12px;background:var(--bg-elevated);border-radius:var(--radius-md);animation:slideDown .2s ease-out}
.op-type-grid{display:flex;flex-direction:column;gap:6px}
.op-type-item{display:flex;align-items:center;gap:8px}
.op-type-label{width:56px;font-size:var(--text-xs);color:var(--text-secondary);text-align:right;flex-shrink:0}
.op-type-track{flex:1;height:14px;background:rgba(255,255,255,.05);border-radius:4px;overflow:hidden}
.op-type-fill{height:100%;border-radius:4px;transition:width .6s}
.op-type-val{width:36px;font-size:var(--text-xs);color:var(--text-tertiary);text-align:right}

.hour-grid{display:flex;gap:2px;flex-wrap:wrap}
.hour-cell{width:28px;height:22px;display:flex;align-items:center;justify-content:center;font-size:10px;color:var(--text-tertiary);background:rgba(255,255,255,.03);border-radius:3px;transition:all .2s}
.hour-active{background:rgba(99,102,241,.25);color:var(--color-primary);font-weight:600}

.pattern-item{display:flex;align-items:center;gap:6px;font-size:var(--text-sm);margin-bottom:4px}
.pattern-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}
.pattern-seq{font-family:var(--font-mono);font-size:var(--text-xs);color:var(--text-primary)}
.pattern-count{font-size:var(--text-xs);color:var(--text-tertiary)}
.pattern-risk{font-size:var(--text-xs);font-weight:600}

.anomaly-section{background:rgba(239,68,68,.04);border-radius:var(--radius-md);padding:10px}
.anomaly-item{display:flex;align-items:flex-start;gap:6px;font-size:var(--text-sm);margin-bottom:4px}
.anomaly-severity{flex-shrink:0;font-size:12px}
.anomaly-desc{color:var(--text-secondary)}
.anomaly-time{font-size:var(--text-xs);color:var(--text-tertiary);margin-left:auto;white-space:nowrap}

.profile-actions{display:flex;gap:8px;margin-top:12px;padding-top:12px;border-top:1px solid var(--border-subtle)}
.btn-sm{padding:4px 12px;font-size:var(--text-xs);border-radius:var(--radius-md);cursor:pointer;border:1px solid var(--border-subtle);transition:all .15s}
.btn-primary{background:var(--color-primary);color:#fff;border-color:var(--color-primary)}
.btn-primary:hover{opacity:.85}
.btn-primary:disabled{opacity:.5;cursor:not-allowed}
.btn-ghost{background:transparent;color:var(--text-secondary)}
.btn-ghost:hover{background:var(--bg-elevated);color:var(--text-primary)}
.pagination{display:flex;align-items:center;justify-content:center;gap:12px;padding:16px 0}
.page-info{font-size:var(--text-sm);color:var(--text-tertiary)}

.modal-overlay{position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.6);z-index:9999;display:flex;align-items:center;justify-content:center}
.modal-content{background:var(--bg-surface);border-radius:var(--radius-lg);width:90%;max-width:700px;max-height:85vh;overflow-y:auto;box-shadow:var(--shadow-lg)}
.modal-header{display:flex;justify-content:space-between;align-items:center;padding:16px 20px;border-bottom:1px solid var(--border-subtle)}
.modal-header h3{margin:0;font-size:var(--text-base)}
.btn-close{background:none;border:none;font-size:18px;cursor:pointer;color:var(--text-secondary);padding:4px 8px}
.btn-close:hover{color:var(--text-primary)}
.modal-body{padding:20px}
.detail-kv{display:flex;justify-content:space-between;padding:6px 0;font-size:var(--text-sm);border-bottom:1px solid rgba(255,255,255,.04)}
.dl{color:var(--text-tertiary);font-weight:600}
.link-accent{color:var(--color-primary);cursor:pointer;text-decoration:none}
.link-accent:hover{text-decoration:underline}
</style>