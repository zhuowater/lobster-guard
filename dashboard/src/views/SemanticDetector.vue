<template>
  <div class="semantic-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="microscope" :size="20" /> 语义检测引擎</h1>
        <p class="page-subtitle">基于 TF-IDF / 句法 / 异常 / 意图 四维分析，精准识别 Prompt 注入与越狱攻击</p>
      </div>
      <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
    </div>

    <!-- 顶部大数字 -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon">📚</div>
        <div class="stat-value">{{ stats.pattern_count ?? '-' }}</div>
        <div class="stat-label">模式库数量</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon"><Icon name="search" :size="20" /></div>
        <div class="stat-value">{{ stats.total_analyses ?? '-' }}</div>
        <div class="stat-label">总分析数</div>
      </div>
      <div class="stat-card stat-danger">
        <div class="stat-icon"><Icon name="alert-triangle" :size="20" /></div>
        <div class="stat-value">{{ stats.detections ?? '-' }}</div>
        <div class="stat-label">检测数</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">📏</div>
        <div class="stat-value">{{ stats.threshold ?? '-' }}</div>
        <div class="stat-label">当前阈值</div>
      </div>
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'test' }" @click="activeTab = 'test'"><Icon name="test" :size="14" /> 实时测试</button>
      <button class="tab-btn" :class="{ active: activeTab === 'patterns' }" @click="activeTab = 'patterns'">📚 攻击模式库 ({{ patterns.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'config' }" @click="activeTab = 'config'"><Icon name="settings" :size="14" /> 配置</button>
    </div>

    <!-- 实时测试区 -->
    <div v-if="activeTab === 'test'" class="section">
      <div class="test-panel">
        <h3 class="section-title">实时语义分析</h3>
        <textarea v-model="testText" class="test-input" rows="4" placeholder="输入要分析的文本..."></textarea>
        <button class="btn btn-primary" @click="analyzeText" :disabled="analyzing || !testText.trim()" style="margin-top: var(--space-2)">
          <span v-if="analyzing" class="spinner"></span>
          {{ analyzing ? '分析中...' : '分析' }}
        </button>
      </div>

      <!-- 分析结果 -->
      <div v-if="analyzeResult" class="analyze-result">
        <div class="result-header">
          <span>分析结果</span>
          <button class="btn-close" @click="analyzeResult = null">✕</button>
        </div>
        <div class="result-top">
          <div class="score-big" :style="{ color: scoreColor(analyzeResult.score ?? analyzeResult.composite_score) }">
            {{ ((analyzeResult.score ?? analyzeResult.composite_score ?? 0) * 100).toFixed(0) }}
            <span class="score-unit">分</span>
          </div>
          <div class="radar-area">
            <svg viewBox="0 0 200 200" class="radar-svg">
              <!-- 背景网格 -->
              <polygon v-for="n in 4" :key="'grid'+n" :points="radarGrid(n/4)" fill="none" stroke="rgba(255,255,255,0.08)" stroke-width="1"/>
              <!-- 轴线 -->
              <line v-for="(_, i) in 4" :key="'axis'+i" x1="100" y1="100" :x2="axisPoint(i, 1).x" :y2="axisPoint(i, 1).y" stroke="rgba(255,255,255,0.1)" stroke-width="1"/>
              <!-- 数据区 -->
              <polygon :points="radarData" fill="rgba(99,102,241,0.25)" stroke="#6366F1" stroke-width="2"/>
              <!-- 数据点 -->
              <circle v-for="(dim, i) in radarDims" :key="'dot'+i" :cx="axisPoint(i, dim.val).x" :cy="axisPoint(i, dim.val).y" r="4" fill="#6366F1"/>
              <!-- 标签 -->
              <text v-for="(dim, i) in radarDims" :key="'label'+i" :x="axisPoint(i, 1.2).x" :y="axisPoint(i, 1.2).y" fill="rgba(255,255,255,0.6)" font-size="11" text-anchor="middle" dominant-baseline="middle">{{ dim.label }}</text>
            </svg>
          </div>
        </div>
        <div v-if="analyzeResult.explanation" class="result-explain">
          <strong>解释：</strong>{{ analyzeResult.explanation }}
        </div>
        <div v-if="analyzeResult.matched_patterns && analyzeResult.matched_patterns.length" class="result-patterns">
          <strong>匹配模式：</strong>
          <span v-for="p in analyzeResult.matched_patterns" :key="p" class="cat-badge" :class="'cat-' + (p.category || p)">{{ p.text || p }}</span>
        </div>
      </div>
    </div>

    <!-- 攻击模式库 -->
    <div v-if="activeTab === 'patterns'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>类别</th>
              <th>文本</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in patterns" :key="p.id">
              <td class="td-mono">{{ p.id }}</td>
              <td><span class="cat-badge" :class="'cat-' + p.category">{{ p.category }}</span></td>
              <td class="td-payload">{{ truncate(p.text || p.pattern, 80) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="patterns.length === 0" class="empty-state">暂无攻击模式</div>
      </div>
    </div>

    <!-- 配置区 -->
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-panel">
        <h3 class="section-title">语义检测配置</h3>
        <div class="slider-group">
          <div class="slider-header">
            <span class="slider-label">检测阈值</span>
            <span class="slider-value">{{ config.threshold }}</span>
          </div>
          <input type="range" class="slider" min="0" max="1" step="0.05" v-model.number="config.threshold">
        </div>
        <div class="slider-group" v-for="w in weightKeys" :key="w.key">
          <div class="slider-header">
            <span class="slider-label">{{ w.label }} 权重</span>
            <span class="slider-value">{{ config.weights[w.key] }}</span>
          </div>
          <input type="range" class="slider" min="0" max="1" step="0.05" v-model.number="config.weights[w.key]">
        </div>
        <div class="config-field">
          <label class="field-label">检测动作</label>
          <select v-model="config.action" class="field-select">
            <option value="block">拦截 (block)</option>
            <option value="warn">告警 (warn)</option>
            <option value="log">记录 (log)</option>
          </select>
        </div>
        <button class="btn btn-primary" @click="saveConfig" :disabled="saving" style="margin-top: var(--space-3)">
          {{ saving ? '保存中...' : '保存配置' }}
        </button>
        <div v-if="saveMsg" class="save-msg" :class="saveMsgType">{{ saveMsg }}</div>
      </div>
    </div>

    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import { api, apiPost, apiPut } from '../api.js'

const activeTab = ref('test')
const stats = ref({})
const patterns = ref([])
const error = ref('')
const testText = ref('')
const analyzing = ref(false)
const analyzeResult = ref(null)
const saving = ref(false)
const saveMsg = ref('')
const saveMsgType = ref('')

const config = reactive({ threshold: 0.7, weights: { tfidf: 0.25, syntax: 0.25, anomaly: 0.25, intent: 0.25 }, action: 'warn' })

const weightKeys = [
  { key: 'tfidf', label: 'TF-IDF' },
  { key: 'syntax', label: '句法' },
  { key: 'anomaly', label: '异常' },
  { key: 'intent', label: '意图' },
]

const radarDims = computed(() => {
  const r = analyzeResult.value
  if (!r) return []
  const d = r.dimensions || r.scores || {}
  return [
    { label: 'TF-IDF', val: d.tfidf ?? d.tf_idf ?? 0 },
    { label: '句法', val: d.syntax ?? 0 },
    { label: '异常', val: d.anomaly ?? 0 },
    { label: '意图', val: d.intent ?? 0 },
  ]
})

function axisPoint(i, scale) {
  const angle = (Math.PI * 2 * i / 4) - Math.PI / 2
  return { x: 100 + Math.cos(angle) * 70 * scale, y: 100 + Math.sin(angle) * 70 * scale }
}

function radarGrid(scale) {
  return [0,1,2,3].map(i => { const p = axisPoint(i, scale); return `${p.x},${p.y}` }).join(' ')
}

const radarData = computed(() => {
  return radarDims.value.map((d, i) => { const p = axisPoint(i, d.val); return `${p.x},${p.y}` }).join(' ')
})

function scoreColor(s) {
  if (s >= 0.7) return '#EF4444'
  if (s >= 0.4) return '#F59E0B'
  return '#10B981'
}

async function loadStats() {
  try {
    const d = await api('/api/v1/semantic/config')
    stats.value = { pattern_count: d.pattern_count, total_analyses: d.total_analyses, detections: d.detections, threshold: d.threshold }
    config.threshold = d.threshold ?? 0.7
    if (d.weights) Object.assign(config.weights, d.weights)
    if (d.action) config.action = d.action
  } catch (e) { error.value = '加载统计失败: ' + e.message }
}

async function loadPatterns() {
  try { const d = await api('/api/v1/semantic/patterns'); patterns.value = d.patterns || d || [] } catch (e) { error.value = '加载模式库失败: ' + e.message }
}

async function analyzeText() {
  analyzing.value = true
  analyzeResult.value = null
  try {
    analyzeResult.value = await apiPost('/api/v1/semantic/analyze', { text: testText.value })
  } catch (e) { error.value = '分析失败: ' + e.message }
  finally { analyzing.value = false }
}

async function saveConfig() {
  saving.value = true; saveMsg.value = ''
  try {
    await apiPut('/api/v1/semantic/config', { threshold: config.threshold, weights: { ...config.weights }, action: config.action })
    saveMsg.value = '✅ 配置已保存'; saveMsgType.value = 'success'; loadStats()
  } catch (e) { saveMsg.value = '❌ 保存失败: ' + e.message; saveMsgType.value = 'error' }
  finally { saving.value = false }
}

function loadAll() { error.value = ''; loadStats(); loadPatterns() }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
onMounted(loadAll)
</script>

<style scoped>
.semantic-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); text-align: center; }
.stat-icon { font-size: 1.5rem; margin-bottom: var(--space-1); }
.stat-value { font-size: 1.75rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }
.stat-danger .stat-value { color: #EF4444; }

.tab-bar { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }

.section { margin-bottom: var(--space-4); }
.section-title { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); }

/* Test Panel */
.test-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); margin-bottom: var(--space-3); }
.test-input {
  width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md);
  color: var(--text-primary); padding: var(--space-3); font-size: var(--text-sm); resize: vertical; font-family: var(--font-mono);
}
.test-input:focus { outline: none; border-color: var(--color-primary); }

/* Analyze Result */
.analyze-result { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.result-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-3); font-weight: 700; color: var(--text-primary); }
.btn-close { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 16px; }
.btn-close:hover { color: var(--text-primary); }
.result-top { display: flex; align-items: center; gap: var(--space-5); flex-wrap: wrap; margin-bottom: var(--space-3); }
.score-big { font-size: 3rem; font-weight: 800; font-family: var(--font-mono); line-height: 1; }
.score-unit { font-size: 1rem; font-weight: 600; opacity: .6; }
.radar-area { width: 200px; height: 200px; flex-shrink: 0; }
.radar-svg { width: 100%; height: 100%; }
.result-explain { font-size: var(--text-sm); color: var(--text-secondary); line-height: 1.6; margin-bottom: var(--space-2); }
.result-patterns { display: flex; flex-wrap: wrap; gap: var(--space-2); align-items: center; font-size: var(--text-sm); color: var(--text-secondary); }

/* Category badges */
.cat-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.cat-prompt_injection { background: rgba(239,68,68,.15); color: #FCA5A5; }
.cat-jailbreak { background: rgba(245,158,11,.15); color: #FCD34D; }
.cat-data_exfil { background: rgba(168,85,247,.15); color: #C4B5FD; }
.cat-role_play { background: rgba(59,130,246,.15); color: #93C5FD; }

/* Patterns Table */
.table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th { text-align: left; padding: 8px 10px; background: var(--bg-elevated); color: var(--text-tertiary); font-weight: 600; font-size: 10px; text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle); white-space: nowrap; }
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.td-payload { max-width: 500px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* Config Panel */
.config-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); max-width: 480px; }
.slider-group { margin-bottom: var(--space-4); }
.slider-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-1); }
.slider-label { font-size: var(--text-sm); color: var(--text-secondary); font-weight: 600; }
.slider-value { font-size: var(--text-base); font-weight: 800; color: var(--color-primary); font-family: var(--font-mono); }
.slider { -webkit-appearance: none; appearance: none; width: 100%; height: 6px; background: rgba(255,255,255,0.1); border-radius: 3px; outline: none; }
.slider::-webkit-slider-thumb { -webkit-appearance: none; appearance: none; width: 18px; height: 18px; border-radius: 50%; background: var(--color-primary); cursor: pointer; border: 2px solid #fff; }
.slider::-moz-range-thumb { width: 18px; height: 18px; border-radius: 50%; background: var(--color-primary); cursor: pointer; border: 2px solid #fff; }

.config-field { margin-bottom: var(--space-3); }
.field-label { display: block; font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; margin-bottom: 4px; }
.field-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); width: 100%; }

.save-msg { margin-top: var(--space-2); font-size: var(--text-sm); font-weight: 600; }
.save-msg.success { color: #10B981; }
.save-msg.error { color: #EF4444; }

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
  .result-top { flex-direction: column; align-items: flex-start; }
}
</style>
