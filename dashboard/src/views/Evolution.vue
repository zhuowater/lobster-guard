<template>
  <div class="evolution-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">🧬 对抗性自进化</h1>
        <p class="page-subtitle">自动变异攻击向量、寻找规则绕过、生成修补规则 — 让防御永远领先一步</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-sm" @click="loadAll">🔄 刷新</button>
        <button class="btn btn-primary" @click="runEvolution" :disabled="running">
          <span v-if="running" class="spinner"></span>
          {{ running ? '进化中...' : '🧬 运行一轮进化' }}
        </button>
      </div>
    </div>

    <!-- 进化结果摘要 (运行后显示) -->
    <div v-if="runResult" class="run-result">
      <div class="run-result-header">
        <span>🧬 进化完成 — 第 {{ runResult.generation }} 代</span>
        <button class="btn-close" @click="runResult = null">✕</button>
      </div>
      <div class="run-result-stats">
        <div class="result-stat"><span class="result-num">{{ runResult.mutations ?? 0 }}</span><span class="result-label">变异</span></div>
        <div class="result-stat"><span class="result-num result-danger">{{ runResult.bypasses ?? 0 }}</span><span class="result-label">绕过</span></div>
        <div class="result-stat"><span class="result-num result-success">{{ runResult.new_rules ?? 0 }}</span><span class="result-label">新规则</span></div>
      </div>
    </div>

    <!-- 顶部大数字 -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon">🔄</div>
        <div class="stat-value">{{ stats.current_generation ?? '-' }}</div>
        <div class="stat-label">当前代数</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🧪</div>
        <div class="stat-value">{{ stats.total_mutations ?? '-' }}</div>
        <div class="stat-label">总变异数</div>
      </div>
      <div class="stat-card stat-danger">
        <div class="stat-icon">💥</div>
        <div class="stat-value">{{ stats.total_bypasses ?? '-' }}</div>
        <div class="stat-label">总绕过数</div>
      </div>
      <div class="stat-card stat-success">
        <div class="stat-icon">🛡️</div>
        <div class="stat-value">{{ stats.auto_rules ?? '-' }}</div>
        <div class="stat-label">自动生成规则</div>
      </div>
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'log' }" @click="activeTab = 'log'">📋 进化日志 ({{ logs.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'strategies' }" @click="activeTab = 'strategies'">⚡ 变异策略 ({{ strategies.length }})</button>
    </div>

    <!-- 进化日志 -->
    <div v-if="activeTab === 'log'" class="section">
      <!-- 筛选 -->
      <div class="filter-bar">
        <div class="filter-group">
          <label>代数</label>
          <input type="number" v-model.number="filterGen" placeholder="全部" min="0" @change="loadLog">
        </div>
        <div class="filter-group">
          <label>阶段</label>
          <select v-model="filterPhase" @change="loadLog">
            <option value="">全部</option>
            <option value="mutate">mutate</option>
            <option value="test">test</option>
            <option value="generate">generate</option>
          </select>
        </div>
        <div class="filter-group">
          <label>绕过</label>
          <select v-model="filterBypassed" @change="loadLog">
            <option value="">全部</option>
            <option value="true">仅绕过</option>
            <option value="false">仅未绕过</option>
          </select>
        </div>
      </div>

      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>代数</th>
              <th>阶段</th>
              <th>策略</th>
              <th>原始向量</th>
              <th>变异载荷</th>
              <th>绕过?</th>
              <th>生成规则</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(entry, idx) in logs" :key="idx" :class="{ 'row-bypass': entry.bypassed }">
              <td class="td-mono">{{ entry.generation }}</td>
              <td><span class="phase-badge">{{ entry.phase }}</span></td>
              <td>{{ entry.strategy || '-' }}</td>
              <td class="td-mono td-payload">{{ truncate(entry.original_vector || entry.original, 30) }}</td>
              <td class="td-mono td-payload">{{ truncate(entry.mutated_payload || entry.mutated, 30) }}</td>
              <td>
                <span v-if="entry.bypassed" class="bypass-yes">⚠️ 是</span>
                <span v-else class="bypass-no">✅ 否</span>
              </td>
              <td class="td-mono">{{ entry.generated_rule || entry.new_rule || '-' }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="logs.length === 0" class="empty-state">暂无进化日志</div>
      </div>
    </div>

    <!-- 变异策略 -->
    <div v-if="activeTab === 'strategies'" class="section">
      <div class="strategy-grid">
        <div v-for="s in strategies" :key="s.name || s.id" class="strategy-card">
          <div class="strategy-name">⚡ {{ s.name }}</div>
          <div class="strategy-desc">{{ s.description || '-' }}</div>
        </div>
      </div>
      <div v-if="strategies.length === 0" class="empty-state">暂无变异策略</div>
    </div>

    <!-- 错误提示 -->
    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api, apiPost } from '../api.js'

const activeTab = ref('log')
const stats = ref({})
const logs = ref([])
const strategies = ref([])
const error = ref('')
const running = ref(false)
const runResult = ref(null)
const filterGen = ref(null)
const filterPhase = ref('')
const filterBypassed = ref('')

async function loadStats() {
  try { stats.value = await api('/api/v1/evolution/stats') } catch (e) { error.value = '加载统计失败: ' + e.message }
}

async function loadLog() {
  try {
    let url = '/api/v1/evolution/log?limit=50'
    if (filterGen.value != null && filterGen.value !== '') url += '&generation=' + filterGen.value
    if (filterPhase.value) url += '&phase=' + encodeURIComponent(filterPhase.value)
    if (filterBypassed.value) url += '&bypassed=' + filterBypassed.value
    const d = await api(url)
    logs.value = d.entries || d.log || d || []
  } catch (e) { error.value = '加载进化日志失败: ' + e.message }
}

async function loadStrategies() {
  try { const d = await api('/api/v1/evolution/strategies'); strategies.value = d.strategies || d || [] } catch (e) { error.value = '加载策略失败: ' + e.message }
}

async function runEvolution() {
  running.value = true
  runResult.value = null
  try {
    const d = await apiPost('/api/v1/evolution/run', {})
    runResult.value = d
    loadStats()
    loadLog()
  } catch (e) {
    error.value = '进化运行失败: ' + e.message
  } finally {
    running.value = false
  }
}

function loadAll() {
  error.value = ''
  loadStats()
  loadLog()
  loadStrategies()
}

function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }

onMounted(loadAll)
</script>

<style scoped>
.evolution-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }
.header-actions { display: flex; gap: var(--space-2); align-items: center; }

/* Run Result */
.run-result { background: var(--bg-surface); border: 1px solid var(--color-primary); border-radius: var(--radius-lg); padding: var(--space-4); margin-bottom: var(--space-4); }
.run-result-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-3); font-weight: 700; color: var(--text-primary); }
.btn-close { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 16px; }
.btn-close:hover { color: var(--text-primary); }
.run-result-stats { display: flex; gap: var(--space-5); }
.result-stat { display: flex; flex-direction: column; align-items: center; }
.result-num { font-size: 1.75rem; font-weight: 800; color: var(--text-primary); font-family: var(--font-mono); }
.result-danger { color: #EF4444; }
.result-success { color: #10B981; }
.result-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: 2px; }

/* Stats */
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); text-align: center; }
.stat-icon { font-size: 1.5rem; margin-bottom: var(--space-1); }
.stat-value { font-size: 1.75rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }
.stat-danger .stat-value { color: #EF4444; }
.stat-success .stat-value { color: #10B981; }

/* Tabs */
.tab-bar { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }

/* Filter */
.filter-bar { display: flex; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; align-items: flex-end; }
.filter-group { display: flex; flex-direction: column; gap: 4px; }
.filter-group label { font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; }
.filter-group select, .filter-group input {
  background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md);
  color: var(--text-primary); padding: 6px 10px; font-size: var(--text-xs); width: 120px;
}

.section { margin-bottom: var(--space-4); }
.table-wrap { overflow-x: auto; }

/* Data Table */
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th {
  text-align: left; padding: 8px 10px; background: var(--bg-elevated);
  color: var(--text-tertiary); font-weight: 600; font-size: 10px;
  text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle);
  white-space: nowrap;
}
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.row-bypass { background: rgba(239,68,68,.06); }
.row-bypass:hover { background: rgba(239,68,68,.1); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.td-payload { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.phase-badge { display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 600; background: rgba(99,102,241,.15); color: #a5b4fc; }
.bypass-yes { color: #EF4444; font-weight: 700; font-size: 11px; }
.bypass-no { color: #10B981; font-size: 11px; }

/* Strategy Cards */
.strategy-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: var(--space-3); }
.strategy-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-3); }
.strategy-name { font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-1); font-size: var(--text-sm); }
.strategy-desc { font-size: var(--text-xs); color: var(--text-secondary); line-height: 1.5; }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-sm { padding: 6px 12px; font-size: var(--text-xs); }

.spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.3); border-top-color: #fff; border-radius: 50%; animation: spin .6s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.empty-state { text-align: center; padding: var(--space-6); color: var(--text-tertiary); }

.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); }

@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .filter-bar { flex-direction: column; }
  .strategy-grid { grid-template-columns: 1fr; }
}
</style>
