<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="split" :size="20" /> Prompt A/B 测试</h1>
        <p class="page-desc">同时运行两个 Prompt 版本，量化比较安全性</p>
      </div>
      <button class="btn btn-primary" @click="showCreateModal = true">+ 创建测试</button>
    </div>

    <!-- StatCards -->
    <div class="stats-row" v-if="!loading">
      <StatCard :iconSvg="svgLab" :value="tests.length" label="测试总数" color="blue" />
      <StatCard :iconSvg="svgPlay" :value="runningCount" label="运行中" color="indigo" :badge="runningCount>0?'Active':''" />
      <StatCard :iconSvg="svgCheck" :value="completedCount" label="已完成" color="green" />
      <StatCard :iconSvg="svgTrend" :value="avgLift" label="平均提升率" color="yellow" />
    </div>
    <div class="stats-row" v-else><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>

    <!-- Filters -->
    <div class="filter-bar">
      <div class="search-box">
        <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
        <input v-model="searchQuery" placeholder="搜索测试名称..." class="search-input"/>
        <button v-if="searchQuery" class="search-clear" @click="searchQuery=''">✕</button>
      </div>
      <div class="status-filters">
        <button v-for="sf in statusFilters" :key="sf.value" class="btn btn-sm" :class="activeStatusFilter===sf.value?'btn-active':'btn-ghost'" @click="activeStatusFilter=sf.value">{{ sf.label }}</button>
      </div>
    </div>

    <!-- Active Tests -->
    <div v-for="test in filteredActiveTests" :key="test.id" class="ab-card" :class="['ab-card-'+test.status, {'ab-pulse':test.status==='running'}]">
      <div class="ab-card-header">
        <div class="ab-card-title">
          <span class="ab-name">{{ test.name }}</span>
          <span class="ab-badge" :class="'badge-'+test.status">{{ statusLabel(test.status) }}</span>
          <span v-if="test.status==='running'&&test.started_at" class="ab-duration">已进行 {{ duration(test.started_at) }}</span>
          <span v-if="test.confidence>0" class="ab-confidence">置信度 {{ test.confidence.toFixed(1) }}%</span>
        </div>
        <button class="btn btn-ghost btn-sm" @click="toggleExpand(test.id)">{{ expandedIds.has(test.id)?'收起':'详情' }}</button>
      </div>

      <div class="ab-card-body">
        <div class="ab-comparison">
          <div class="ab-version ab-version-a">
            <div class="ab-version-header">
              <span class="ab-version-label">版本 A (对照组 {{ test.traffic_a }}%)</span>
              <span v-if="test.winner==='A'" class="ab-winner">🏆</span>
            </div>
            <div class="ab-metrics" v-if="test.result_a">
              <div class="ab-metric-row"><span class="ab-metric-label">请求</span><span class="ab-metric-value">{{ test.result_a.total_requests }}</span></div>
              <div class="ab-metric-row"><span class="ab-metric-label">拦截率</span><span class="ab-metric-value">{{ pct(test.result_a.block_rate) }}</span></div>
              <div class="ab-metric-row"><span class="ab-metric-label">Canary</span><span class="ab-metric-value" :class="betterClass(test.result_a.canary_leak_rate,test.result_b?.canary_leak_rate,true)">{{ pct(test.result_a.canary_leak_rate) }}</span></div>
              <div class="ab-metric-row"><span class="ab-metric-label">错误</span><span class="ab-metric-value">{{ pct(test.result_a.error_rate) }}</span></div>
              <div class="ab-metric-row ab-metric-score"><span class="ab-metric-label">安全分</span><span class="ab-metric-value" :class="scoreClass(test.result_a.security_score)">{{ test.result_a.security_score.toFixed(1) }}</span></div>
            </div>
            <div v-else class="ab-no-data">暂无数据</div>
          </div>
          <div class="ab-divider"></div>
          <div class="ab-version ab-version-b">
            <div class="ab-version-header">
              <span class="ab-version-label">版本 B (实验组 {{ test.traffic_b }}%)</span>
              <span v-if="test.winner==='B'" class="ab-winner">🏆</span>
            </div>
            <div class="ab-metrics" v-if="test.result_b">
              <div class="ab-metric-row"><span class="ab-metric-label">请求</span><span class="ab-metric-value">{{ test.result_b.total_requests }}</span></div>
              <div class="ab-metric-row"><span class="ab-metric-label">拦截率</span><span class="ab-metric-value">{{ pct(test.result_b.block_rate) }}</span></div>
              <div class="ab-metric-row"><span class="ab-metric-label">Canary</span><span class="ab-metric-value" :class="betterClass(test.result_b?.canary_leak_rate,test.result_a?.canary_leak_rate,true)">{{ pct(test.result_b.canary_leak_rate) }}</span></div>
              <div class="ab-metric-row"><span class="ab-metric-label">错误</span><span class="ab-metric-value">{{ pct(test.result_b.error_rate) }}</span></div>
              <div class="ab-metric-row ab-metric-score"><span class="ab-metric-label">安全分</span><span class="ab-metric-value" :class="scoreClass(test.result_b.security_score)">{{ test.result_b.security_score.toFixed(1) }}</span></div>
            </div>
            <div v-else class="ab-no-data">暂无数据</div>
          </div>
        </div>

        <!-- Expanded Detail -->
        <div v-if="expandedIds.has(test.id)" class="ab-expand-detail">
          <div class="ab-detail-grid">
            <div class="ab-detail-item"><span class="ab-detail-label">版本 A</span><span class="ab-detail-val mono">{{ test.version_a }}</span></div>
            <div class="ab-detail-item"><span class="ab-detail-label">版本 B</span><span class="ab-detail-val mono">{{ test.version_b }}</span></div>
            <div class="ab-detail-item"><span class="ab-detail-label">Prompt Hash A</span><span class="ab-detail-val mono">{{ test.prompt_hash_a||'—' }}</span></div>
            <div class="ab-detail-item"><span class="ab-detail-label">Prompt Hash B</span><span class="ab-detail-val mono">{{ test.prompt_hash_b||'—' }}</span></div>
            <div class="ab-detail-item" v-if="test.confidence>0"><span class="ab-detail-label">显著性</span><span class="ab-detail-val" :class="test.confidence>=95?'sig-yes':'sig-no'">{{ test.confidence>=95?'✅ 显著 (p<0.05)':'⚠️ 不显著' }}</span></div>
          </div>
        </div>

        <div class="ab-conclusion" v-if="test.confidence>0||test.recommendation">
          <div v-if="test.confidence>0" class="ab-conclusion-line">
            <Icon name="bar-chart" :size="12"/> 置信度: {{ test.confidence.toFixed(1) }}%
            <span v-if="test.winner==='B'"> — 版本 B 在安全性上显著优于版本 A</span>
            <span v-else-if="test.winner==='A'"> — 版本 A 在安全性上更优</span>
            <span v-else-if="test.winner==='tie'"> — 两个版本差异不显著</span>
          </div>
          <div v-if="test.recommendation" class="ab-conclusion-line">💡 {{ test.recommendation }}</div>
        </div>
      </div>

      <div class="ab-card-actions">
        <button v-if="test.status==='draft'" class="btn btn-sm btn-success" @click="startTest(test.id)">开始测试</button>
        <button v-if="test.status==='running'" class="btn btn-sm btn-warning" @click="stopTest(test.id)">停止测试</button>
        <button v-if="test.winner&&test.winner!=='tie'" class="btn btn-sm btn-primary" @click="applyWinner(test)">应用胜出方案</button>
        <button class="btn btn-sm btn-danger" @click="confirmDeleteTest(test)">删除</button>
      </div>
    </div>

    <EmptyState v-if="filteredActiveTests.length===0&&filteredHistoryTests.length===0&&!loading" icon="🧪" title="暂无 A/B 测试" description="创建一个测试，比较不同 Prompt 版本的安全表现" actionText="+ 创建测试" @action="showCreateModal=true"/>

    <!-- History -->
    <div v-if="filteredHistoryTests.length>0" class="history-section">
      <h2 class="section-title"><Icon name="file-text" :size="16"/> 历史测试</h2>
      <div class="data-table-wrap">
        <table class="data-table">
          <thead><tr><th>名称</th><th>状态</th><th>版本 A</th><th>版本 B</th><th>赢家</th><th>置信度</th><th>安全分 A</th><th>安全分 B</th><th>创建时间</th><th>操作</th></tr></thead>
          <tbody><tr v-for="t in filteredHistoryTests" :key="t.id"><td>{{ t.name }}</td><td><span class="ab-badge" :class="'badge-'+t.status">{{ statusLabel(t.status) }}</span></td><td>{{ t.version_a }}</td><td>{{ t.version_b }}</td><td><span v-if="t.winner==='A'" class="winner-a">🏆 A</span><span v-else-if="t.winner==='B'" class="winner-b">🏆 B</span><span v-else-if="t.winner==='tie'" class="winner-tie">⚖️ 平局</span><span v-else>—</span></td><td>{{ t.confidence>0?t.confidence.toFixed(1)+'%':'—' }}</td><td>{{ t.result_a?t.result_a.security_score.toFixed(1):'—' }}</td><td>{{ t.result_b?t.result_b.security_score.toFixed(1):'—' }}</td><td>{{ formatTime(t.created_at) }}</td><td><button class="btn btn-sm btn-danger" @click="confirmDeleteTest(t)">删除</button></td></tr></tbody>
        </table>
      </div>
    </div>

    <!-- Create Modal -->
    <div v-if="showCreateModal" class="modal-overlay" @click.self="showCreateModal=false">
      <div class="modal-box">
        <h3>创建 A/B 测试</h3>
        <div class="form-group"><label>测试名称 <span class="required">*</span></label><input v-model="newTest.name" placeholder="如：v4.0 安全指令优化"/></div>
        <div class="form-row">
          <div class="form-group"><label>版本 A 标签</label><input v-model="newTest.version_a" placeholder="如：v3.2-当前版"/></div>
          <div class="form-group"><label>版本 A Prompt Hash</label><input v-model="newTest.prompt_hash_a" placeholder="prompt_versions 的 hash"/></div>
        </div>
        <div class="form-row">
          <div class="form-group"><label>版本 B 标签</label><input v-model="newTest.version_b" placeholder="如：v4.0-新指令"/></div>
          <div class="form-group"><label>版本 B Prompt Hash</label><input v-model="newTest.prompt_hash_b" placeholder="prompt_versions 的 hash"/></div>
        </div>
        <div class="form-group">
          <label>版本 A 流量比例: {{ newTest.traffic_a }}%</label>
          <input type="range" v-model.number="newTest.traffic_a" min="10" max="90" step="5"/>
          <div class="traffic-labels"><span>A: {{ newTest.traffic_a }}%</span><span>B: {{ 100-newTest.traffic_a }}%</span></div>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showCreateModal=false">取消</button>
          <button class="btn btn-primary" @click="createTest" :disabled="!newTest.name.trim()">创建</button>
        </div>
      </div>
    </div>

    <!-- Confirm Modal -->
    <ConfirmModal :visible="confirmModal.show" :title="confirmModal.title" :message="confirmModal.message" :type="confirmModal.type" @confirm="confirmModal.onConfirm" @cancel="confirmModal.show=false"/>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import Skeleton from '../components/Skeleton.vue'
import EmptyState from '../components/EmptyState.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { api, apiPost, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'

const tests = ref([])
const loading = ref(true)
const showCreateModal = ref(false)
const searchQuery = ref('')
const activeStatusFilter = ref('all')
const expandedIds = ref(new Set())
const newTest = ref({ name:'', version_a:'A', prompt_hash_a:'', version_b:'B', prompt_hash_b:'', traffic_a:50 })
const confirmModal = reactive({ show:false, title:'', message:'', type:'danger', onConfirm:()=>{} })

const svgLab = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 3h6v5l4 7H5l4-7V3z"/><line x1="9" y1="3" x2="15" y2="3"/></svg>'
const svgPlay = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>'
const svgCheck = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>'
const svgTrend = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 6 13.5 15.5 8.5 10.5 1 18"/><polyline points="17 6 23 6 23 12"/></svg>'

const statusFilters = [{label:'全部',value:'all'},{label:'运行中',value:'running'},{label:'已完成',value:'completed'},{label:'草稿',value:'draft'}]

const runningCount = computed(()=>tests.value.filter(t=>t.status==='running').length)
const completedCount = computed(()=>tests.value.filter(t=>t.status==='completed').length)
const avgLift = computed(()=>{
  const done = tests.value.filter(t=>t.status==='completed'&&t.result_a&&t.result_b)
  if (!done.length) return '—'
  const avg = done.reduce((s,t)=>{
    const diff = t.result_b.security_score - t.result_a.security_score
    return s + diff
  },0)/done.length
  return (avg>=0?'+':'')+avg.toFixed(1)
})

const filteredActiveTests = computed(()=>{
  let list = tests.value.filter(t=>t.status==='running'||t.status==='draft')
  if (activeStatusFilter.value!=='all') list = list.filter(t=>t.status===activeStatusFilter.value)
  if (searchQuery.value.trim()) { const q=searchQuery.value.trim().toLowerCase(); list=list.filter(t=>t.name.toLowerCase().includes(q)) }
  return list
})
const filteredHistoryTests = computed(()=>{
  let list = tests.value.filter(t=>t.status==='completed'||t.status==='cancelled')
  if (activeStatusFilter.value!=='all'&&activeStatusFilter.value!=='running'&&activeStatusFilter.value!=='draft') list = list.filter(t=>t.status===activeStatusFilter.value)
  if (searchQuery.value.trim()) { const q=searchQuery.value.trim().toLowerCase(); list=list.filter(t=>t.name.toLowerCase().includes(q)) }
  return list
})

function toggleExpand(id) { expandedIds.value.has(id)?expandedIds.value.delete(id):expandedIds.value.add(id); expandedIds.value=new Set(expandedIds.value) }

async function loadTests() {
  loading.value=true
  try { const d=await api('/api/v1/ab-tests?tenant=all'); tests.value=d.tests||[] }
  catch{ tests.value=[] } loading.value=false
}
async function createTest() {
  if (!newTest.value.name.trim()) { showToast('请输入测试名称','warning'); return }
  try { await apiPost('/api/v1/ab-tests',newTest.value); showCreateModal.value=false; newTest.value={name:'',version_a:'A',prompt_hash_a:'',version_b:'B',prompt_hash_b:'',traffic_a:50}; showToast('测试已创建','success'); loadTests() }
  catch(e){ showToast('创建失败: '+e.message,'error') }
}
async function startTest(id) {
  confirmModal.title='开始测试'; confirmModal.message='确定开始此A/B测试？测试开始后将分流流量到两个版本。'; confirmModal.type='info'
  confirmModal.onConfirm=async()=>{ confirmModal.show=false; try{await apiPost('/api/v1/ab-tests/'+id+'/start');showToast('测试已启动','success');loadTests()}catch(e){showToast('启动失败: '+e.message,'error')} }
  confirmModal.show=true
}
async function stopTest(id) {
  confirmModal.title='停止测试'; confirmModal.message='确定停止测试？将计算最终结果。'; confirmModal.type='warning'
  confirmModal.onConfirm=async()=>{ confirmModal.show=false; try{await apiPost('/api/v1/ab-tests/'+id+'/stop');showToast('测试已停止','success');loadTests()}catch(e){showToast('停止失败: '+e.message,'error')} }
  confirmModal.show=true
}
function confirmDeleteTest(test) {
  confirmModal.title='删除测试'; confirmModal.message='确定删除测试 "'+test.name+'" 吗？此操作不可恢复。'; confirmModal.type='danger'
  confirmModal.onConfirm=async()=>{ confirmModal.show=false; try{await apiDelete('/api/v1/ab-tests/'+test.id);showToast('已删除','success');loadTests()}catch(e){showToast('删除失败: '+e.message,'error')} }
  confirmModal.show=true
}
function applyWinner(test) {
  confirmModal.title='应用胜出方案'; confirmModal.message='将版本 '+test.winner+' 设为默认 Prompt。确认应用？'; confirmModal.type='info'
  confirmModal.onConfirm=async()=>{ confirmModal.show=false; showToast('已应用版本 '+test.winner+' 为默认方案','success') }
  confirmModal.show=true
}
function statusLabel(s) { return {draft:'草稿',running:'运行中',completed:'已完成',cancelled:'已取消'}[s]||s }
function pct(v) { return v==null?'—':(v*100).toFixed(1)+'%' }
function scoreClass(s) { return s>=80?'score-good':s>=60?'score-warn':'score-bad' }
function betterClass(my,other,lowerBetter) { if(my==null||other==null) return ''; if(lowerBetter) return my<other?'metric-better':my>other?'metric-worse':''; return my>other?'metric-better':my<other?'metric-worse':'' }
function duration(startedAt) { if(!startedAt) return ''; const ms=Date.now()-new Date(startedAt).getTime(); const h=Math.floor(ms/3600000),m=Math.floor((ms%3600000)/60000); return h>0?h+'h '+m+'m':m+'m' }
function formatTime(t) { if(!t) return '—'; return new Date(t).toLocaleString('zh-CN',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}) }
onMounted(loadTests)
</script>

<style scoped>
.page-container{padding:var(--space-6);max-width:1200px}
.page-header{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:var(--space-6)}
.page-title{font-size:var(--text-2xl);font-weight:700;color:var(--text-primary);margin:0}
.page-desc{font-size:var(--text-sm);color:var(--text-tertiary);margin-top:var(--space-1)}
.stats-row{display:grid;grid-template-columns:repeat(4,1fr);gap:var(--space-3);margin-bottom:var(--space-5)}
/* Filter bar */
.filter-bar{display:flex;align-items:center;gap:var(--space-3);margin-bottom:var(--space-5);flex-wrap:wrap}
.search-box{position:relative;display:flex;align-items:center}
.search-icon{position:absolute;left:8px;color:var(--text-tertiary);pointer-events:none}
.search-input{padding:6px 28px;background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-sm);width:220px;transition:all .15s}
.search-input:focus{outline:none;border-color:var(--color-primary);width:280px}
.search-clear{position:absolute;right:6px;background:none;border:none;color:var(--text-tertiary);cursor:pointer;font-size:12px}.search-clear:hover{color:var(--text-primary)}
.status-filters{display:flex;gap:var(--space-1)}
/* AB Card */
.ab-card{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:var(--space-5);margin-bottom:var(--space-4);transition:all var(--transition-fast)}
.ab-card-running{border-left:4px solid var(--color-primary)}.ab-card-completed{border-left:4px solid var(--color-success)}.ab-card-draft{border-left:4px solid var(--text-tertiary)}
.ab-pulse{animation:abPulse 2s ease-in-out infinite}
@keyframes abPulse{0%,100%{box-shadow:0 0 0 0 rgba(99,102,241,0)}50%{box-shadow:0 0 0 4px rgba(99,102,241,.15)}}
.ab-card-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-4)}
.ab-card-title{display:flex;align-items:center;gap:var(--space-3);flex-wrap:wrap}
.ab-name{font-size:var(--text-lg);font-weight:600;color:var(--text-primary)}
.ab-badge{font-size:11px;padding:2px 8px;border-radius:12px;font-weight:600}
.badge-draft{background:rgba(255,255,255,0.1);color:var(--text-tertiary)}.badge-running{background:rgba(99,102,241,.15);color:#818CF8}
.badge-completed{background:rgba(34,197,94,0.15);color:#4ADE80}.badge-cancelled{background:rgba(239,68,68,0.15);color:#F87171}
.ab-duration{font-size:var(--text-xs);color:var(--text-tertiary)}.ab-confidence{font-size:var(--text-xs);color:var(--color-primary);font-weight:600}
/* Comparison */
.ab-comparison{display:flex;gap:var(--space-4);margin-bottom:var(--space-4)}
.ab-version{flex:1;background:var(--bg-elevated);border-radius:var(--radius-md);padding:var(--space-4)}
.ab-version-a{border-top:3px solid #6366F1}.ab-version-b{border-top:3px solid #22C55E}
.ab-version-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-3)}
.ab-version-label{font-size:var(--text-sm);font-weight:600;color:var(--text-secondary)}.ab-winner{font-size:1.2rem}
.ab-divider{width:1px;background:var(--border-subtle);flex-shrink:0}
.ab-metrics{display:flex;flex-direction:column;gap:var(--space-2)}
.ab-metric-row{display:flex;justify-content:space-between;align-items:center;padding:var(--space-1) 0}
.ab-metric-label{font-size:var(--text-sm);color:var(--text-tertiary)}.ab-metric-value{font-size:var(--text-sm);font-weight:600;color:var(--text-primary);font-family:var(--font-mono)}
.ab-metric-score{border-top:1px solid var(--border-subtle);padding-top:var(--space-2);margin-top:var(--space-1)}
.ab-no-data{font-size:var(--text-sm);color:var(--text-tertiary);text-align:center;padding:var(--space-4)}
.metric-better{color:#4ADE80 !important}.metric-better::after{content:' ← 更好';font-size:10px;color:#4ADE80}
.metric-worse{color:#F87171 !important}
.score-good{color:#4ADE80 !important}.score-warn{color:#FBBF24 !important}.score-bad{color:#F87171 !important}
/* Expanded detail */
.ab-expand-detail{background:var(--bg-elevated);border-radius:var(--radius-md);padding:var(--space-3) var(--space-4);margin-bottom:var(--space-3);animation:slideDown .2s ease}
@keyframes slideDown{from{opacity:0;max-height:0}to{opacity:1;max-height:200px}}
.ab-detail-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:var(--space-2)}
.ab-detail-item{display:flex;justify-content:space-between;font-size:var(--text-xs);padding:var(--space-1) 0}
.ab-detail-label{color:var(--text-tertiary)}.ab-detail-val{color:var(--text-primary);font-weight:500}
.mono{font-family:var(--font-mono)}.sig-yes{color:#4ADE80}.sig-no{color:#FBBF24}
/* Conclusion */
.ab-conclusion{background:var(--bg-elevated);border-radius:var(--radius-md);padding:var(--space-3) var(--space-4);margin-bottom:var(--space-3)}
.ab-conclusion-line{font-size:var(--text-sm);color:var(--text-secondary);line-height:1.6}
.ab-card-actions{display:flex;gap:var(--space-2)}
/* History */
.history-section{margin-top:var(--space-8)}.section-title{font-size:var(--text-lg);font-weight:600;color:var(--text-primary);margin-bottom:var(--space-4)}
.data-table-wrap{overflow-x:auto}
.data-table{width:100%;border-collapse:collapse;font-size:var(--text-sm)}
.data-table th{text-align:left;padding:var(--space-2) var(--space-3);color:var(--text-tertiary);font-weight:600;border-bottom:1px solid var(--border-subtle);white-space:nowrap}
.data-table td{padding:var(--space-2) var(--space-3);border-bottom:1px solid var(--border-subtle);color:var(--text-secondary);white-space:nowrap}
.data-table tr:hover{background:var(--bg-elevated)}
.winner-a{color:#818CF8;font-weight:600}.winner-b{color:#4ADE80;font-weight:600}.winner-tie{color:#FBBF24}
/* Buttons */
.btn{padding:var(--space-2) var(--space-4);border-radius:var(--radius-md);font-size:var(--text-sm);font-weight:600;cursor:pointer;border:1px solid var(--border-subtle);background:var(--bg-elevated);color:var(--text-secondary);transition:all var(--transition-fast);display:inline-flex;align-items:center;gap:4px}
.btn:hover{background:var(--bg-surface);color:var(--text-primary)}
.btn-primary{background:var(--color-primary);color:#fff;border-color:var(--color-primary)}.btn-primary:hover{opacity:0.9}.btn-primary:disabled{opacity:0.5;cursor:not-allowed}
.btn-success{background:rgba(34,197,94,0.15);color:#4ADE80;border-color:rgba(34,197,94,0.3)}
.btn-warning{background:rgba(251,191,36,0.15);color:#FBBF24;border-color:rgba(251,191,36,0.3)}
.btn-danger{background:rgba(239,68,68,0.1);color:#F87171;border-color:rgba(239,68,68,0.2)}
.btn-ghost{background:transparent;border-color:transparent;color:var(--text-secondary)}.btn-ghost:hover{background:var(--bg-elevated);color:var(--text-primary)}
.btn-active{background:rgba(99,102,241,.15);color:var(--color-primary);border-color:rgba(99,102,241,.3);font-weight:600}
.btn-sm{padding:var(--space-1) var(--space-3);font-size:var(--text-xs)}
/* Modal */
.modal-overlay{position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.6);z-index:1000;display:flex;align-items:center;justify-content:center}
.modal-box{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:var(--space-6);width:560px;max-width:90vw;max-height:90vh;overflow-y:auto}
.modal-box h3{margin:0 0 var(--space-5);font-size:var(--text-lg);color:var(--text-primary)}
.form-group{margin-bottom:var(--space-4)}.form-group label{display:block;font-size:var(--text-sm);font-weight:600;color:var(--text-secondary);margin-bottom:var(--space-1)}
.form-group input,.form-group select{width:100%;padding:var(--space-2) var(--space-3);background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-sm)}
.form-group input[type="range"]{padding:0}.form-row{display:flex;gap:var(--space-4)}.form-row .form-group{flex:1}
.required{color:#EF4444}
.traffic-labels{display:flex;justify-content:space-between;font-size:var(--text-xs);color:var(--text-tertiary);margin-top:var(--space-1)}
.modal-actions{display:flex;justify-content:flex-end;gap:var(--space-2);margin-top:var(--space-5)}
@media(max-width:768px){.ab-comparison{flex-direction:column}.ab-divider{height:1px;width:100%}.form-row{flex-direction:column}.stats-row{grid-template-columns:repeat(2,1fr)}.ab-detail-grid{grid-template-columns:1fr}}
</style>
