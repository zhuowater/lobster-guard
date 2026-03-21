<template>
  <div class="evolution-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="dna" :size="20" /> 对抗性自进化</h1>
        <p class="page-subtitle">自动变异攻击向量、寻找规则绕过、生成修补规则 — 让防御永远领先一步</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
        <button class="btn btn-sm" @click="openConfig"><Icon name="settings" :size="14" /> 配置</button>
        <button class="btn btn-primary" @click="runEvolution" :disabled="running">
          <span v-if="running" class="spinner"></span>
          {{ running ? '进化中...' : '🧬 运行一轮进化' }}
        </button>
      </div>
    </div>

    <!-- 进化结果摘要 -->
    <div v-if="runResult" class="run-result">
      <div class="rr-header">
        <span><Icon name="dna" :size="14" /> 进化完成 — 第 {{ runResult.generation }} 代</span>
        <button class="btn-close" @click="runResult = null">✕</button>
      </div>
      <div class="rr-stats">
        <div class="rr-item"><span class="rr-num">{{ runResult.mutations ?? 0 }}</span><span class="rr-label">变异</span></div>
        <div class="rr-item"><span class="rr-num rr-danger">{{ runResult.bypasses ?? 0 }}</span><span class="rr-label">绕过</span></div>
        <div class="rr-item"><span class="rr-num rr-success">{{ runResult.new_rules ?? 0 }}</span><span class="rr-label">新规则</span></div>
      </div>
    </div>

    <div class="stats-grid">
      <StatCard :iconSvg="svgGen" label="当前代数" :value="stats.current_generation ?? 0" color="indigo" />
      <StatCard :iconSvg="svgMut" label="总变异数" :value="stats.total_mutations ?? 0" color="blue" />
      <StatCard :iconSvg="svgBypass" label="总绕过数" :value="stats.total_bypasses ?? 0" color="red" />
      <StatCard :iconSvg="svgRule" label="自动生成规则" :value="stats.auto_rules ?? 0" color="green" />
    </div>

    <div class="tab-bar">
      <button class="tab-btn" :class="{active:activeTab==='log'}" @click="activeTab='log'"><Icon name="file-text" :size="14" /> 进化日志 <span class="tab-count">{{logs.length}}</span></button>
      <button class="tab-btn" :class="{active:activeTab==='strategies'}" @click="activeTab='strategies'"><Icon name="zap" :size="14" /> 变异策略 <span class="tab-count">{{strategies.length}}</span></button>
      <button class="tab-btn" :class="{active:activeTab==='trend'}" @click="activeTab='trend'; loadTrend()"><Icon name="trending-up" :size="14" /> 学习曲线</button>
    </div>

    <!-- 进化日志 -->
    <div v-if="activeTab==='log'" class="section">
      <div class="section-toolbar">
        <div class="search-box"><Icon name="search" :size="14" /><input v-model="logSearch" placeholder="搜索策略/载荷/规则..." /></div>
        <div class="filter-group">
          <input type="number" v-model.number="filterGen" placeholder="代数" min="0" class="filter-input" @change="loadLog" />
        </div>
        <select v-model="filterPhase" class="filter-select" @change="loadLog"><option value="">全部阶段</option><option value="mutate">mutate</option><option value="test">test</option><option value="generate">generate</option></select>
        <select v-model="filterBypassed" class="filter-select" @change="loadLog"><option value="">全部</option><option value="true">仅绕过</option><option value="false">仅未绕过</option></select>
      </div>
      <DataTable :columns="logColumns" :data="filteredLogs" :loading="logLoading" :expandable="true" emptyText="暂无进化日志" emptyDesc="运行进化后会产生日志">
        <template #cell-generation="{value}"><span class="gen-badge">G{{value}}</span></template>
        <template #cell-phase="{value}"><span class="phase-badge">{{value}}</span></template>
        <template #cell-original="{row}"><code class="mono-cell payload-cell">{{truncate(row.original_vector||row.original,30)}}</code></template>
        <template #cell-mutated="{row}"><code class="mono-cell payload-cell">{{truncate(row.mutated_payload||row.mutated,30)}}</code></template>
        <template #cell-bypassed="{row}">
          <span v-if="row.bypassed" class="bypass-yes">⚠️ 绕过</span>
          <span v-else class="bypass-no">✅ 拦截</span>
        </template>
        <template #cell-generated_rule="{row}"><code class="mono-cell" v-if="row.generated_rule||row.new_rule">{{truncate(row.generated_rule||row.new_rule,30)}}</code><span v-else class="text-muted">-</span></template>
        <template #expand="{row}">
          <div class="expand-detail">
            <div class="dg">
              <div class="di"><span class="dl">代数</span><span>{{row.generation}}</span></div>
              <div class="di"><span class="dl">阶段</span><span class="phase-badge">{{row.phase}}</span></div>
              <div class="di"><span class="dl">策略</span><span>{{row.strategy||'-'}}</span></div>
              <div class="di"><span class="dl">绕过</span><span :class="row.bypassed?'bypass-yes':'bypass-no'">{{row.bypassed?'是':'否'}}</span></div>
            </div>
            <div class="dp"><span class="dl">原始向量</span><pre class="dc">{{row.original_vector||row.original||'-'}}</pre></div>
            <div class="dp"><span class="dl">变异载荷</span><pre class="dc">{{row.mutated_payload||row.mutated||'-'}}</pre></div>
            <div v-if="row.generated_rule||row.new_rule" class="dp"><span class="dl">生成规则</span><pre class="dc">{{row.generated_rule||row.new_rule}}</pre></div>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- 变异策略 -->
    <div v-if="activeTab==='strategies'" class="section">
      <div class="strategy-grid">
        <div v-for="s in strategies" :key="s.name||s.id" class="strategy-card">
          <div class="sc-header">
            <span class="sc-icon">⚡</span>
            <span class="sc-name">{{ s.name }}</span>
          </div>
          <div class="sc-desc">{{ s.description || '-' }}</div>
          <div v-if="s.params" class="sc-params">
            <span v-for="(v,k) in s.params" :key="k" class="sc-param"><b>{{k}}</b>: {{v}}</span>
          </div>
        </div>
      </div>
      <EmptyState v-if="strategies.length===0" icon="⚡" title="暂无变异策略" description="变异策略在进化引擎初始化时自动注册" />
    </div>

    <!-- 学习曲线 -->
    <div v-if="activeTab==='trend'" class="section">
      <div class="trend-panel">
        <div class="trend-header"><Icon name="trending-up" :size="16" /> 进化学习曲线</div>
        <TrendChart v-if="trendData.length" :data="trendData" :lines="trendLines" :xLabels="trendLabels" :height="200" />
        <EmptyState v-else icon="📈" title="暂无趋势数据" description="运行多轮进化后会显示学习曲线" />
      </div>
    </div>

    <!-- Config Modal -->
    <Teleport to="body">
      <div v-if="configVisible" class="modal-overlay" @click.self="configVisible=false">
        <div class="modal-box">
          <div class="modal-header"><span><Icon name="settings" :size="16" /> 自进化配置</span><button class="btn-close" @click="configVisible=false">✕</button></div>
          <div class="modal-body">
            <div class="fg"><label class="fl">自动进化</label>
              <label class="toggle"><input type="checkbox" v-model="configForm.enabled" /><span class="toggle-track"><span class="toggle-thumb"></span></span><span class="toggle-txt">{{configForm.enabled?'已启用':'已关闭'}}</span></label>
            </div>
            <div class="fg"><label class="fl">进化间隔 (分钟)</label><input v-model.number="configForm.interval_min" type="number" class="fi" min="1" placeholder="360" /><span class="fh">建议 60~1440 分钟</span></div>
          </div>
          <div class="modal-footer"><button class="btn btn-sm" @click="configVisible=false">取消</button><button class="btn btn-sm btn-primary" @click="saveConfig" :disabled="configSaving">{{configSaving?'保存中...':'保存'}}</button></div>
        </div>
      </div>
    </Teleport>

    <div v-if="error" class="error-banner" @click="error=''">⚠️ {{error}} <span class="err-x">✕</span></div>
  </div>
</template>
<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { api, apiPost, apiPut } from '../api.js'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import DataTable from '../components/DataTable.vue'
import EmptyState from '../components/EmptyState.vue'
import TrendChart from '../components/TrendChart.vue'
import { showToast } from '../stores/app.js'

const activeTab = ref('log')
const stats = ref({})
const logs = ref([])
const strategies = ref([])
const error = ref('')
const running = ref(false)
const runResult = ref(null)
const logLoading = ref(false)
const logSearch = ref('')
const filterGen = ref(null)
const filterPhase = ref('')
const filterBypassed = ref('')

// Config
const configVisible = ref(false)
const configSaving = ref(false)
const configForm = reactive({ enabled: false, interval_min: 360 })

// Trend
const trendData = ref([])
const trendLabels = ref([])
const trendLines = [
  { key: 'mutations', label: '变异数', color: '#6366F1', width: 2 },
  { key: 'bypasses', label: '绕过数', color: '#EF4444', width: 2 },
  { key: 'new_rules', label: '新规则', color: '#10B981', width: 2 },
]

const filteredLogs = computed(() => {
  if (!logSearch.value.trim()) return logs.value
  const q = logSearch.value.toLowerCase().trim()
  return logs.value.filter(e =>
    (e.strategy || '').toLowerCase().includes(q) ||
    (e.original_vector || e.original || '').toLowerCase().includes(q) ||
    (e.mutated_payload || e.mutated || '').toLowerCase().includes(q) ||
    (e.generated_rule || e.new_rule || '').toLowerCase().includes(q)
  )
})

const logColumns = [
  { key: 'generation', label: '代数', sortable: true, width: '70px' },
  { key: 'phase', label: '阶段', width: '80px' },
  { key: 'strategy', label: '策略', sortable: true },
  { key: 'original', label: '原始向量' },
  { key: 'mutated', label: '变异载荷' },
  { key: 'bypassed', label: '绕过?', sortable: true, width: '80px' },
  { key: 'generated_rule', label: '生成规则' },
]

const svgGen = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>'
const svgMut = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/><path d="M8 13h2"/><path d="M8 17h2"/><path d="M14 13l-2 4h4l-2 4"/></svg>'
const svgBypass = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>'
const svgRule = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>'

async function loadStats() { try { stats.value = await api('/api/v1/evolution/stats') } catch (e) { error.value = '统计加载失败: ' + e.message } }

async function loadLog() {
  logLoading.value = true
  try {
    let url = '/api/v1/evolution/log?limit=50'
    if (filterGen.value != null && filterGen.value !== '') url += '&generation=' + filterGen.value
    if (filterPhase.value) url += '&phase=' + encodeURIComponent(filterPhase.value)
    if (filterBypassed.value) url += '&bypassed=' + filterBypassed.value
    const d = await api(url)
    logs.value = d.logs || d.entries || d.log || d || []
  } catch (e) { error.value = '日志加载失败: ' + e.message } finally { logLoading.value = false }
}

async function loadStrategies() {
  try { const d = await api('/api/v1/evolution/strategies'); strategies.value = d.strategies || d || [] }
  catch (e) { error.value = '策略加载失败: ' + e.message }
}

async function runEvolution() {
  running.value = true; runResult.value = null
  try {
    const d = await apiPost('/api/v1/evolution/run', {})
    runResult.value = d
    showToast('进化完成 — 第 ' + (d.generation || '?') + ' 代 🧬', 'success')
    loadStats(); loadLog()
  } catch (e) { showToast('进化运行失败: ' + e.message, 'error'); error.value = e.message }
  finally { running.value = false }
}

async function loadTrend() {
  // Build trend from log data grouped by generation
  try {
    const d = await api('/api/v1/evolution/log?limit=200')
    const entries = d.logs || d.entries || d.log || d || []
    const genMap = {}
    for (const e of entries) {
      const g = e.generation || 0
      if (!genMap[g]) genMap[g] = { mutations: 0, bypasses: 0, new_rules: 0 }
      genMap[g].mutations++
      if (e.bypassed) genMap[g].bypasses++
      if (e.generated_rule || e.new_rule) genMap[g].new_rules++
    }
    const gens = Object.keys(genMap).map(Number).sort((a, b) => a - b)
    trendData.value = gens.map(g => genMap[g])
    trendLabels.value = gens.map(g => 'G' + g)
  } catch (e) { /* ignore trend errors */ }
}

function openConfig() {
  configForm.enabled = stats.value.auto_enabled !== false && stats.value.enabled !== false
  configForm.interval_min = stats.value.interval_min || 360
  configVisible.value = true
}

async function saveConfig() {
  configSaving.value = true
  try {
    await apiPut('/api/v1/evolution/config', { enabled: configForm.enabled, interval_min: configForm.interval_min })
    showToast('配置已更新', 'success'); configVisible.value = false; loadStats()
  } catch (e) { showToast('保存失败: ' + e.message, 'error') }
  finally { configSaving.value = false }
}

function loadAll() { error.value = ''; loadStats(); loadLog(); loadStrategies() }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }

onMounted(loadAll)
</script>
<style scoped>
.evolution-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; display: flex; align-items: center; gap: 8px; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }
.header-actions { display: flex; gap: var(--space-2); align-items: center; }

/* Run result */
.run-result { background: var(--bg-surface); border: 1px solid var(--color-primary); border-radius: var(--radius-lg); padding: var(--space-4); margin-bottom: var(--space-4); animation: slideDown .3s ease-out; }
@keyframes slideDown { from { opacity: 0; transform: translateY(-10px) } to { opacity: 1; transform: translateY(0) } }
.rr-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-3); font-weight: 700; color: var(--text-primary); }
.rr-stats { display: flex; gap: var(--space-5); }
.rr-item { display: flex; flex-direction: column; align-items: center; }
.rr-num { font-size: 1.75rem; font-weight: 800; color: var(--text-primary); font-family: var(--font-mono); }
.rr-danger { color: #EF4444; }
.rr-success { color: #10B981; }
.rr-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: 2px; }

.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.tab-bar { display: flex; gap: var(--space-1); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); }
.tab-btn { display: inline-flex; align-items: center; gap: 6px; background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; border-bottom: 2px solid transparent; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom-color: var(--color-primary); font-weight: 600; }
.tab-count { padding: 0 6px; border-radius: 9999px; font-size: 10px; font-weight: 600; background: rgba(99,102,241,.12); color: var(--color-primary); line-height: 1.6; }

.section { margin-bottom: var(--space-4); }
.section-toolbar { display: flex; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; align-items: center; }
.search-box { display: flex; align-items: center; gap: 8px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 6px 12px; flex: 1; min-width: 200px; max-width: 360px; }
.search-box input { background: none; border: none; outline: none; color: var(--text-primary); font-size: var(--text-sm); width: 100%; }
.search-box input::placeholder { color: var(--text-tertiary); }
.filter-group { display: flex; gap: var(--space-2); }
.filter-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-xs); cursor: pointer; }
.filter-input { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-xs); width: 80px; }

.mono-cell { font-family: var(--font-mono); font-size: 11px; color: var(--text-secondary); }
.payload-cell { max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: inline-block; }
.gen-badge { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: 10px; font-weight: 700; background: rgba(99,102,241,.12); color: var(--color-primary); font-family: var(--font-mono); }
.phase-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; background: rgba(99,102,241,.15); color: #a5b4fc; }
.bypass-yes { color: #EF4444; font-weight: 700; font-size: 11px; }
.bypass-no { color: #10B981; font-size: 11px; }
.text-muted { color: var(--text-tertiary); font-size: 11px; }

/* Strategy cards */
.strategy-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: var(--space-3); }
.strategy-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); transition: transform .2s, box-shadow .2s; }
.strategy-card:hover { transform: translateY(-2px); box-shadow: var(--shadow-md); }
.sc-header { display: flex; align-items: center; gap: 8px; margin-bottom: var(--space-2); }
.sc-icon { font-size: 1.2rem; }
.sc-name { font-weight: 700; color: var(--text-primary); font-size: var(--text-sm); }
.sc-desc { font-size: var(--text-xs); color: var(--text-secondary); line-height: 1.5; margin-bottom: var(--space-2); }
.sc-params { display: flex; flex-wrap: wrap; gap: var(--space-1); }
.sc-param { font-size: 10px; padding: 2px 6px; border-radius: 4px; background: var(--bg-elevated); color: var(--text-tertiary); }
.sc-param b { color: var(--text-secondary); font-weight: 600; }

/* Trend */
.trend-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.trend-header { display: flex; align-items: center; gap: 8px; font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); }

/* Expand */
.expand-detail { padding: var(--space-2) 0; }
.dg { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: var(--space-3); margin-bottom: var(--space-3); }
.di { display: flex; flex-direction: column; gap: 4px; }
.dl { font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; }
.di code { font-family: var(--font-mono); font-size: 11px; color: var(--text-secondary); word-break: break-all; }
.dp { margin-top: var(--space-2); }
.dc { background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-3); font-size: 11px; font-family: var(--font-mono); color: var(--text-secondary); overflow-x: auto; max-height: 200px; margin-top: var(--space-1); white-space: pre-wrap; word-break: break-all; }

/* Modal */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,.5); z-index: 1000; display: flex; align-items: center; justify-content: center; animation: fadeIn .2s; }
@keyframes fadeIn { from { opacity: 0 } to { opacity: 1 } }
.modal-box { background: var(--bg-surface); border: 1px solid var(--border-default); border-radius: var(--radius-lg); padding: 24px; min-width: 380px; max-width: 500px; box-shadow: 0 16px 64px rgba(0,0,0,.5); animation: slideUp .2s ease-out; }
@keyframes slideUp { from { opacity: 0; transform: translateY(20px) } to { opacity: 1; transform: translateY(0) } }
.modal-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; font-weight: 600; color: var(--text-primary); }
.modal-body { color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: 20px; }
.modal-footer { display: flex; justify-content: flex-end; gap: 8px; }
.btn-close { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 16px; }
.btn-close:hover { color: var(--text-primary); }
.fg { margin-bottom: var(--space-4); }
.fl { display: block; font-size: var(--text-xs); font-weight: 600; color: var(--text-secondary); margin-bottom: var(--space-2); text-transform: uppercase; letter-spacing: .05em; }
.fi { width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 8px 12px; color: var(--text-primary); font-size: var(--text-sm); outline: none; box-sizing: border-box; }
.fi:focus { border-color: var(--color-primary); }
.fh { display: block; font-size: 11px; color: var(--text-tertiary); margin-top: 4px; }
.toggle { display: flex; align-items: center; gap: 10px; cursor: pointer; }
.toggle input { display: none; }
.toggle-track { width: 36px; height: 20px; border-radius: 10px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); position: relative; transition: all .2s; }
.toggle input:checked + .toggle-track { background: var(--color-primary); border-color: var(--color-primary); }
.toggle-thumb { position: absolute; top: 2px; left: 2px; width: 14px; height: 14px; border-radius: 50%; background: #fff; transition: all .2s; }
.toggle input:checked + .toggle-track .toggle-thumb { left: 18px; }
.toggle-txt { font-size: var(--text-sm); color: var(--text-secondary); }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-sm { padding: 4px 10px; font-size: var(--text-xs); }
.spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.3); border-top-color: #fff; border-radius: 50%; animation: spin .6s linear infinite; margin-right: 4px; }
@keyframes spin { to { transform: rotate(360deg) } }
.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); cursor: pointer; display: flex; justify-content: space-between; }
.err-x { opacity: .5; }
@media (max-width: 768px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } .strategy-grid { grid-template-columns: 1fr; } .section-toolbar { flex-direction: column; } }
</style>