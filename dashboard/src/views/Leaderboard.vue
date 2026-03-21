<template>
  <div class="leaderboard-page">
    <div class="page-header">
      <div class="header-left">
        <h1 class="page-title"><Icon name="trophy" :size="20" /> 安全排行榜</h1>
        <span class="page-subtitle">租户安全态势全局视图</span>
      </div>
      <div class="header-right">
        <div class="sla-baseline" v-if="slaConfig">
          <span class="sla-tag">SLA 基线:</span>
          <span class="sla-item">健康≥{{ slaConfig.min_health_score }}</span>
          <span class="sla-sep">|</span>
          <span class="sla-item">事件≤{{ slaConfig.max_incident_count }}</span>
          <span class="sla-sep">|</span>
          <span class="sla-item">检测≥{{ slaConfig.min_redteam_score }}%</span>
        </div>
        <button class="btn btn-sm" @click="loadAll" :disabled="loading">
          <span v-if="loading">刷新中...</span>
          <span v-else><Icon name="refresh" :size="14" /> 刷新</span>
        </button>
      </div>
    </div>

    <!-- SLA StatCards -->
    <div class="stats-row" v-if="slaOverview">
      <StatCard :iconSvg="svgCheck" :value="slaOverview.summary.green" label="达标" color="green" :badge="greenPct" />
      <StatCard :iconSvg="svgWarn" :value="slaOverview.summary.yellow" label="警告" color="yellow" />
      <StatCard :iconSvg="svgX" :value="slaOverview.summary.red" label="未达标" color="red" />
      <StatCard :iconSvg="svgBar" :value="slaOverview.summary.total" label="总计" color="blue" />
    </div>
    <div class="stats-row" v-else-if="loading"><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>

    <!-- Rank Section -->
    <div class="section">
      <div class="section-header">
        <h2 class="section-title">排行榜</h2>
        <div class="section-actions">
          <div class="time-presets">
            <button v-for="tp in timePresets" :key="tp.value" class="btn btn-sm" :class="activeTimeRange===tp.value?'btn-active':'btn-ghost'" @click="setTimeRange(tp.value)">{{ tp.label }}</button>
          </div>
          <div class="rank-tabs">
            <button v-for="tab in rankTabs" :key="tab.value" class="rank-tab" :class="{active:activeRankTab===tab.value}" @click="activeRankTab=tab.value">{{ tab.label }}</button>
          </div>
          <div class="search-box">
            <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
            <input v-model="searchQuery" placeholder="搜索租户..." class="search-input"/>
            <button v-if="searchQuery" class="search-clear" @click="searchQuery=''">✕</button>
          </div>
          <div class="export-group">
            <button class="btn btn-ghost btn-sm" @click="exportData('csv')"><Icon name="download" :size="14"/> CSV</button>
            <button class="btn btn-ghost btn-sm" @click="exportData('json')">JSON</button>
          </div>
        </div>
      </div>

      <div class="leaderboard-list" v-if="filteredScores.length>0">
        <div v-for="score in filteredScores" :key="score.tenant_id" class="leaderboard-card" :class="['sla-border-'+score.sla_status, rankCardClass(score.rank)]" @click="goToTenant(score.tenant_id)">
          <div class="card-header">
            <div class="rank-badge" :class="'rank-'+(score.rank<=3?score.rank:'n')">
              <span class="rank-num">#{{ score.rank }}</span>
              <span class="rank-medal" v-if="score.rank<=3">{{ rankMedal(score.rank) }}</span>
            </div>
            <div class="tenant-info">
              <span class="tenant-name link-accent">{{ score.tenant_name }}</span>
              <span class="tenant-sub">{{ score.tenant_id }}</span>
            </div>
            <div class="card-stats">
              <span class="stat" title="健康分"><span class="stat-label">健康</span><span class="stat-value" :class="healthClass(score.health_score)">{{ score.health_score }}</span></span>
              <span class="stat" title="检测率"><span class="stat-label">检测</span><span class="stat-value">{{ score.redteam_score>0?score.redteam_score.toFixed(1)+'%':'N/A' }}</span></span>
              <span class="stat" title="事件数"><span class="stat-label">事件</span><span class="stat-value" :class="incidentClass(score.incident_count)">{{ score.incident_count }}</span></span>
              <span class="stat" title="拦截率"><span class="stat-label">拦截</span><span class="stat-value">{{ score.block_rate.toFixed(1) }}%</span></span>
              <span class="stat sla-badge" :class="'sla-'+score.sla_status">{{ slaIcon(score.sla_status) }} SLA</span>
              <span class="stat trend-indicator" :class="'trend-'+score.trend">
                <svg v-if="score.trend==='up'" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="18 15 12 9 6 15"/></svg>
                <svg v-else-if="score.trend==='down'" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="6 9 12 15 18 9"/></svg>
                <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/></svg>
              </span>
            </div>
          </div>
          <div class="progress-bar-container">
            <div class="progress-bar" :style="{width:sortValue(score)+'%'}" :class="progressClass(sortValue(score))"></div>
            <span class="progress-label">{{ sortValueLabel(score) }}</span>
          </div>
        </div>
      </div>
      <EmptyState v-else-if="!loading" icon="🏆" title="暂无排行数据" description="请先注入演示数据或调整筛选条件"/>
      <div v-else class="sk-list"><Skeleton type="card" v-for="i in 3" :key="i"/></div>
    </div>

    <!-- Heatmap -->
    <div class="section">
      <h2 class="section-title">🔥 攻击热力图 <span class="section-sub">近7天 · 租户×OWASP分类</span></h2>
      <div class="heatmap-wrap" v-if="heatmapData.tenants.length>0">
        <table class="heatmap-table">
          <thead><tr><th class="hm-th-t">租户</th><th v-for="cat in heatmapData.categories" :key="cat" class="hm-th-c" :title="catFullName(cat)">{{ cat }}</th></tr></thead>
          <tbody><tr v-for="t in heatmapData.tenants" :key="t.id"><td class="hm-td-n">{{ t.name }}</td><td v-for="cat in heatmapData.categories" :key="cat" class="hm-cell" :class="'intensity-'+getCellIntensity(t.id,cat)" :title="getCellCount(t.id,cat)+' 次'">{{ getCellCount(t.id,cat)||'' }}</td></tr></tbody>
        </table>
        <div class="hm-legend"><span class="lg-l">少</span><span class="lg-c intensity-none"></span><span class="lg-c intensity-low"></span><span class="lg-c intensity-medium"></span><span class="lg-c intensity-high"></span><span class="lg-c intensity-critical"></span><span class="lg-l">多</span></div>
      </div>
      <EmptyState v-else-if="!loading" icon="🔥" title="暂无热力图数据"/>
    </div>

    <!-- SLA Config -->
    <div class="section">
      <h2 class="section-title"><Icon name="settings" :size="16"/> SLA 配置</h2>
      <div class="sla-config-panel">
        <div class="cfg-row"><label>最低健康分</label><input type="number" v-model.number="editConfig.min_health_score" min="0" max="100" class="cfg-input"/></div>
        <div class="cfg-row"><label>7天最多事件</label><input type="number" v-model.number="editConfig.max_incident_count" min="0" class="cfg-input"/></div>
        <div class="cfg-row"><label>最低检测率(%)</label><input type="number" v-model.number="editConfig.min_redteam_score" min="0" max="100" step="0.1" class="cfg-input"/></div>
        <div class="cfg-row"><label>最低拦截率</label><input type="number" v-model.number="editConfig.min_block_rate" min="0" max="1" step="0.01" class="cfg-input"/></div>
        <button class="btn btn-primary" @click="saveSLAConfig" :disabled="saving">{{ saving?'保存中...':'保存配置' }}</button>
      </div>
      <div class="sla-detail" v-if="slaOverview&&slaOverview.tenants.length>0">
        <h3 class="sub-title">各租户 SLA 达标情况</h3>
        <table class="sla-table">
          <thead><tr><th>租户</th><th>健康分</th><th>事件数</th><th>检测率</th><th>SLA</th></tr></thead>
          <tbody><tr v-for="t in slaOverview.tenants" :key="t.tenant_id"><td>{{ t.tenant_name }}</td><td :class="t.health_met?'met':'unmet'">{{ t.health_score }} {{ t.health_met?'✅':'❌' }}</td><td :class="t.incident_met?'met':'unmet'">{{ t.incident_count }} {{ t.incident_met?'✅':'❌' }}</td><td :class="t.redteam_met?'met':'unmet'">{{ t.redteam_score>0?t.redteam_score.toFixed(1)+'%':'N/A' }} {{ t.redteam_met?'✅':'❌' }}</td><td><span class="sla-pill" :class="'sla-'+t.sla_status">{{ slaIcon(t.sla_status) }} {{ t.sla_status.toUpperCase() }}</span></td></tr></tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import Skeleton from '../components/Skeleton.vue'
import EmptyState from '../components/EmptyState.vue'
import { api, apiPut } from '../api.js'
import { showToast } from '../stores/app.js'

const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const scores = ref([])
const slaConfig = ref(null)
const slaOverview = ref(null)
const heatmapCells = ref([])
const searchQuery = ref('')
const activeTimeRange = ref('all')
const activeRankTab = ref('health')
const editConfig = reactive({ min_health_score:70, max_incident_count:10, min_redteam_score:80, min_block_rate:0 })
const heatmapData = reactive({ tenants:[], categories:[] })

const svgCheck = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>'
const svgWarn = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/></svg>'
const svgX = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>'
const svgBar = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>'

const timePresets = [{label:'今天',value:'today'},{label:'本周',value:'week'},{label:'本月',value:'month'},{label:'全部',value:'all'}]
const rankTabs = [{label:'健康分',value:'health'},{label:'事件数',value:'incidents'},{label:'拦截率',value:'block_rate'}]

const greenPct = computed(() => {
  if (!slaOverview.value||!slaOverview.value.summary.total) return ''
  return Math.round((slaOverview.value.summary.green/slaOverview.value.summary.total)*100)+'%'
})
const filteredScores = computed(() => {
  let list = [...scores.value]
  if (searchQuery.value.trim()) { const q=searchQuery.value.trim().toLowerCase(); list=list.filter(s=>s.tenant_name.toLowerCase().includes(q)||s.tenant_id.toLowerCase().includes(q)) }
  if (activeRankTab.value==='health') list.sort((a,b)=>b.health_score-a.health_score)
  else if (activeRankTab.value==='incidents') list.sort((a,b)=>b.incident_count-a.incident_count)
  else list.sort((a,b)=>b.block_rate-a.block_rate)
  return list
})
function sortValue(s) { return activeRankTab.value==='health'?s.health_score:activeRankTab.value==='incidents'?Math.min(s.incident_count,100):s.block_rate }
function sortValueLabel(s) { return activeRankTab.value==='health'?s.health_score+'/100':activeRankTab.value==='incidents'?s.incident_count+' 事件':s.block_rate.toFixed(1)+'%' }
function rankMedal(r) { return r===1?'🥇':r===2?'🥈':r===3?'🥉':'' }
function rankCardClass(r) { return r===1?'card-gold':r===2?'card-silver':r===3?'card-bronze':'' }
function slaIcon(s) { return s==='green'?'✅':s==='yellow'?'⚠️':'❌' }
function healthClass(s) { return s>=90?'health-excellent':s>=70?'health-good':s>=50?'health-warning':'health-danger' }
function incidentClass(c) { return c<=5?'incident-low':c<=10?'incident-medium':'incident-high' }
function progressClass(s) { return s>=90?'progress-excellent':s>=70?'progress-good':s>=50?'progress-warning':'progress-danger' }
const catNames={PI:'Prompt Injection',IO:'Insecure Output',SI:'Sensitive Info',IP:'Insecure Plugin',OR:'Overreliance',MD:'Model DoS'}
function catFullName(c) { return catNames[c]||c }
function getCellIntensity(tid,cat) { const c=heatmapCells.value.find(x=>x.tenant_id===tid&&x.category===cat); return c?c.intensity:'none' }
function getCellCount(tid,cat) { const c=heatmapCells.value.find(x=>x.tenant_id===tid&&x.category===cat); return c?c.count:0 }
function goToTenant() { router.push('/tenants') }
function setTimeRange(r) { activeTimeRange.value=r; loadAll() }
function exportData(fmt) {
  const data=filteredScores.value
  if (!data.length) { showToast('暂无数据可导出','warning'); return }
  if (fmt==='csv') {
    const hdr=['排名','租户名','租户ID','健康分','事件数','检测率','拦截率','SLA','趋势']
    const rows=data.map(s=>[s.rank,s.tenant_name,s.tenant_id,s.health_score,s.incident_count,s.redteam_score>0?s.redteam_score.toFixed(1)+'%':'N/A',s.block_rate.toFixed(1)+'%',s.sla_status,s.trend])
    dlBlob([hdr.join(','),...rows.map(r=>r.map(v=>'"'+String(v).replace(/"/g,'""')+'"').join(','))].join('\n'),'leaderboard.csv','text/csv')
  } else dlBlob(JSON.stringify(data,null,2),'leaderboard.json','application/json')
  showToast('导出成功','success')
}
function dlBlob(c,fn,m) { const b=new Blob([c],{type:m}),a=document.createElement('a'); a.href=URL.createObjectURL(b); a.download=fn; a.click(); URL.revokeObjectURL(a.href) }
async function loadLeaderboard() {
  try {
    const p=activeTimeRange.value!=='all'?'?range='+activeTimeRange.value:''
    const d=await api('/api/v1/leaderboard'+p); scores.value=d.scores||[]; slaConfig.value=d.sla||null
    if(d.sla) Object.assign(editConfig,{min_health_score:d.sla.min_health_score,max_incident_count:d.sla.max_incident_count,min_redteam_score:d.sla.min_redteam_score,min_block_rate:d.sla.min_block_rate||0})
  } catch(e){ console.error('加载排行榜失败:',e) }
}
async function loadHeatmap() {
  try {
    const d=await api('/api/v1/leaderboard/heatmap'); heatmapCells.value=d.cells||[]; heatmapData.categories=d.categories||[]
    const tm={}; for(const c of heatmapCells.value){if(!tm[c.tenant_id]){const s=scores.value.find(x=>x.tenant_id===c.tenant_id);tm[c.tenant_id]={id:c.tenant_id,name:s?s.tenant_name:c.tenant_id}}}
    heatmapData.tenants=Object.values(tm)
  } catch(e){ console.error('加载热力图失败:',e) }
}
async function loadSLA() { try{slaOverview.value=await api('/api/v1/leaderboard/sla')}catch(e){console.error('加载SLA失败:',e)} }
async function loadAll() { loading.value=true; try{await loadLeaderboard();await Promise.all([loadHeatmap(),loadSLA()])}finally{loading.value=false} }
async function saveSLAConfig() {
  saving.value=true
  try{await apiPut('/api/v1/leaderboard/sla/config',{...editConfig});showToast('SLA 配置已保存','success');await loadAll()}
  catch(e){showToast('保存失败: '+e.message,'error')}finally{saving.value=false}
}
onMounted(loadAll)
</script>

<style scoped>
.leaderboard-page{padding:var(--space-6);max-width:1200px}
.page-header{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:var(--space-6);flex-wrap:wrap;gap:var(--space-3)}
.header-left{display:flex;align-items:baseline;gap:var(--space-3)}
.page-title{font-size:1.5rem;font-weight:700;color:var(--text-primary);margin:0}
.page-subtitle{font-size:var(--text-sm);color:var(--text-tertiary)}
.header-right{display:flex;align-items:center;gap:var(--space-3);flex-wrap:wrap}
.sla-baseline{display:flex;align-items:center;gap:var(--space-2);background:var(--bg-elevated);padding:6px 12px;border-radius:var(--radius-md);font-size:var(--text-xs);color:var(--text-secondary)}
.sla-tag{font-weight:700;color:var(--color-primary)}.sla-sep{color:var(--text-tertiary)}.sla-item{font-family:var(--font-mono)}
.btn{padding:6px 14px;border-radius:var(--radius-md);border:1px solid var(--border-subtle);background:var(--bg-elevated);color:var(--text-primary);cursor:pointer;font-size:var(--text-xs);transition:all .15s;display:inline-flex;align-items:center;gap:4px;font-weight:500}
.btn:hover{background:var(--bg-surface);border-color:var(--color-primary)}.btn-sm{font-size:11px;padding:4px 10px}
.btn-primary{background:var(--color-primary);color:#fff;border-color:var(--color-primary)}.btn-primary:hover{opacity:.9}.btn:disabled{opacity:.5;cursor:not-allowed}
.btn-ghost{background:transparent;border-color:transparent;color:var(--text-secondary)}.btn-ghost:hover{background:var(--bg-elevated);color:var(--text-primary)}
.btn-active{background:rgba(99,102,241,.15);color:var(--color-primary);border-color:rgba(99,102,241,.3);font-weight:600}
.stats-row{display:grid;grid-template-columns:repeat(4,1fr);gap:var(--space-3);margin-bottom:var(--space-6)}
.sk-list{display:flex;flex-direction:column;gap:var(--space-3)}
.section{margin-bottom:var(--space-6)}
.section-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-4);flex-wrap:wrap;gap:var(--space-3)}
.section-title{font-size:1.1rem;font-weight:700;color:var(--text-primary);margin:0}
.section-sub{font-size:var(--text-xs);color:var(--text-tertiary);font-weight:400}
.section-actions{display:flex;align-items:center;gap:var(--space-3);flex-wrap:wrap}
.time-presets{display:flex;gap:var(--space-1)}
.rank-tabs{display:flex;border:1px solid var(--border-subtle);border-radius:var(--radius-md);overflow:hidden}
.rank-tab{background:var(--bg-elevated);border:none;color:var(--text-secondary);font-size:11px;padding:4px 12px;cursor:pointer;transition:all .15s;border-right:1px solid var(--border-subtle)}
.rank-tab:last-child{border-right:none}.rank-tab:hover{color:var(--text-primary);background:var(--bg-surface)}
.rank-tab.active{color:var(--color-primary);background:rgba(99,102,241,.1);font-weight:600}
.search-box{position:relative;display:flex;align-items:center}
.search-icon{position:absolute;left:8px;color:var(--text-tertiary);pointer-events:none}
.search-input{padding:4px 28px;background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:11px;width:160px;transition:all .15s}
.search-input:focus{outline:none;border-color:var(--color-primary);width:200px}
.search-clear{position:absolute;right:6px;background:none;border:none;color:var(--text-tertiary);cursor:pointer;font-size:12px}.search-clear:hover{color:var(--text-primary)}
.export-group{display:flex;gap:var(--space-1)}
.leaderboard-list{display:flex;flex-direction:column;gap:var(--space-3)}
.leaderboard-card{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:var(--space-4);transition:all .2s;cursor:pointer}
.leaderboard-card:hover{border-color:var(--color-primary);box-shadow:var(--shadow-md);transform:translateY(-1px)}
.sla-border-green{border-left:4px solid #22C55E}.sla-border-yellow{border-left:4px solid #EAB308}.sla-border-red{border-left:4px solid #EF4444}
.card-gold{background:linear-gradient(135deg,var(--bg-surface),rgba(255,215,0,.06))}.card-gold:hover{box-shadow:0 4px 20px rgba(255,215,0,.15)}
.card-silver{background:linear-gradient(135deg,var(--bg-surface),rgba(192,192,192,.06))}.card-silver:hover{box-shadow:0 4px 20px rgba(192,192,192,.15)}
.card-bronze{background:linear-gradient(135deg,var(--bg-surface),rgba(205,127,50,.06))}.card-bronze:hover{box-shadow:0 4px 20px rgba(205,127,50,.15)}
.card-header{display:flex;align-items:center;gap:var(--space-3);margin-bottom:var(--space-3);flex-wrap:wrap}
.rank-badge{display:flex;align-items:center;gap:var(--space-1);min-width:56px}
.rank-num{font-size:1.1rem;font-weight:800;font-family:var(--font-mono)}
.rank-1 .rank-num{color:#FFD700;text-shadow:0 0 8px rgba(255,215,0,.3)}.rank-2 .rank-num{color:#C0C0C0}.rank-3 .rank-num{color:#CD7F32}.rank-n .rank-num{color:var(--color-primary)}
.rank-medal{font-size:1.2rem}
.tenant-info{display:flex;flex-direction:column;gap:2px}
.tenant-name{font-size:var(--text-base);font-weight:600;color:var(--text-primary)}
.tenant-sub{font-size:10px;color:var(--text-tertiary);font-family:var(--font-mono)}
.card-stats{display:flex;align-items:center;gap:var(--space-3);margin-left:auto;flex-wrap:wrap}
.stat{display:flex;align-items:center;gap:4px;font-size:var(--text-xs)}.stat-label{color:var(--text-tertiary)}.stat-value{font-family:var(--font-mono);font-weight:600}
.health-excellent{color:#22C55E}.health-good{color:#3B82F6}.health-warning{color:#EAB308}.health-danger{color:#EF4444}
.incident-low{color:#22C55E}.incident-medium{color:#EAB308}.incident-high{color:#EF4444}
.sla-badge{padding:2px 8px;border-radius:10px;font-size:11px;font-weight:700}
.sla-green{background:rgba(34,197,94,.15);color:#22C55E}.sla-yellow{background:rgba(234,179,8,.15);color:#EAB308}.sla-red{background:rgba(239,68,68,.15);color:#EF4444}
.trend-indicator{display:flex;align-items:center}.trend-up{color:#22C55E}.trend-down{color:#EF4444}
.progress-bar-container{position:relative;height:24px;background:var(--bg-elevated);border-radius:var(--radius-md);overflow:hidden}
.progress-bar{height:100%;border-radius:var(--radius-md);transition:width .6s ease}
.progress-excellent{background:linear-gradient(90deg,#22C55E,#16A34A)}.progress-good{background:linear-gradient(90deg,#3B82F6,#2563EB)}
.progress-warning{background:linear-gradient(90deg,#EAB308,#CA8A04)}.progress-danger{background:linear-gradient(90deg,#EF4444,#DC2626)}
.progress-label{position:absolute;right:8px;top:50%;transform:translateY(-50%);font-size:11px;font-weight:700;color:var(--text-primary);font-family:var(--font-mono)}
/* Heatmap */
.heatmap-wrap{overflow-x:auto}
.heatmap-table{width:100%;border-collapse:collapse;background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);overflow:hidden}
.heatmap-table th,.heatmap-table td{padding:10px 16px;text-align:center;font-size:var(--text-xs)}
.heatmap-table thead th{background:var(--bg-elevated);color:var(--text-secondary);font-weight:700;border-bottom:1px solid var(--border-subtle)}
.hm-th-t{text-align:left}.hm-td-n{text-align:left;font-weight:600;color:var(--text-primary);white-space:nowrap}
.hm-cell{width:60px;height:40px;border-radius:4px;font-weight:700;font-family:var(--font-mono);transition:all .2s}
.intensity-none{background:rgba(34,197,94,.08);color:transparent}.intensity-low{background:rgba(34,197,94,.25);color:#22C55E}
.intensity-medium{background:rgba(234,179,8,.35);color:#CA8A04}.intensity-high{background:rgba(249,115,22,.45);color:#EA580C}
.intensity-critical{background:rgba(239,68,68,.55);color:#DC2626}
.hm-legend{display:flex;align-items:center;gap:4px;margin-top:var(--space-3);justify-content:flex-end}
.lg-l{font-size:10px;color:var(--text-tertiary)}.lg-c{width:20px;height:14px;border-radius:2px}
/* SLA Config */
.sla-config-panel{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:var(--space-4);display:flex;flex-wrap:wrap;gap:var(--space-4);align-items:flex-end}
.cfg-row{display:flex;flex-direction:column;gap:4px}
.cfg-row label{font-size:var(--text-xs);color:var(--text-secondary);font-weight:600}
.cfg-input{width:120px;padding:6px 10px;background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-family:var(--font-mono);font-size:var(--text-sm)}
.cfg-input:focus{outline:none;border-color:var(--color-primary)}
.sub-title{font-size:var(--text-sm);font-weight:600;color:var(--text-primary);margin:var(--space-4) 0 var(--space-3)}
.sla-detail{margin-top:var(--space-4)}
.sla-table{width:100%;border-collapse:collapse;background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);overflow:hidden}
.sla-table th,.sla-table td{padding:8px 14px;text-align:left;font-size:var(--text-xs);border-bottom:1px solid var(--border-subtle)}
.sla-table thead th{background:var(--bg-elevated);font-weight:700;color:var(--text-secondary)}
.met{color:#22C55E}.unmet{color:#EF4444}
.sla-pill{display:inline-block;padding:2px 8px;border-radius:10px;font-size:10px;font-weight:700}
.link-accent{color:var(--color-primary);cursor:pointer;text-decoration:none}.link-accent:hover{text-decoration:underline}
@media(max-width:768px){.stats-row{grid-template-columns:repeat(2,1fr)}.card-stats{margin-left:0}.sla-config-panel{flex-direction:column}}
</style>
