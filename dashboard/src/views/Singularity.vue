<template>
  <div class="singularity-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">🌀 奇点蜜罐</h1>
        <p class="page-subtitle">拓扑感知蜜罐预算 — 以欧拉示性数约束暴露面，精准放置蜜罐</p>
      </div>
      <button class="btn btn-sm" @click="loadAll">🔄 刷新</button>
    </div>

    <!-- 超预算警告 -->
    <div v-if="budget.over_budget" class="over-budget-banner">
      🚨 <strong>预算超支！</strong>当前蜜罐暴露面超出拓扑预算限制，请减少通道暴露等级。
    </div>

    <!-- 奇点预算仪表盘 -->
    <div class="budget-dashboard">
      <div class="budget-ring-area">
        <div class="ring-container">
          <svg viewBox="0 0 120 120" class="ring-svg">
            <circle cx="60" cy="60" r="50" fill="none" stroke="rgba(255,255,255,0.08)" stroke-width="10"/>
            <circle cx="60" cy="60" r="50" fill="none"
              :stroke="budgetColor"
              stroke-width="10"
              stroke-linecap="round"
              :stroke-dasharray="budgetArc"
              stroke-dashoffset="0"
              transform="rotate(-90 60 60)"
            />
          </svg>
          <div class="ring-text">
            <div class="ring-num" :style="{ color: budgetColor }">{{ budgetPercent }}%</div>
            <div class="ring-label">已分配</div>
          </div>
        </div>
        <div class="budget-stats">
          <div class="b-stat"><span class="b-val">{{ budget.total_budget ?? '-' }}</span><span class="b-label">总预算</span></div>
          <div class="b-stat"><span class="b-val">{{ budget.allocated ?? '-' }}</span><span class="b-label">已分配</span></div>
          <div class="b-stat"><span class="b-val">{{ budget.euler_characteristic ?? '-' }}</span><span class="b-label">χ 欧拉数</span></div>
          <div class="b-stat"><span class="b-val">{{ budget.topology_lower_bound ?? '-' }}</span><span class="b-label">拓扑下限</span></div>
        </div>
      </div>

      <!-- 三通道暴露条 -->
      <div class="channels-area">
        <h3 class="section-title">通道暴露等级</h3>
        <div class="channel-bar" v-for="ch in channels" :key="ch.name">
          <div class="ch-header">
            <span class="ch-name">{{ ch.icon }} {{ ch.label }}</span>
            <span class="ch-level">Lv.{{ ch.level }} / 5</span>
          </div>
          <div class="ch-track">
            <div class="ch-fill" :style="{ width: (ch.level / 5 * 100) + '%', background: channelColor(ch.level) }"></div>
          </div>
        </div>
      </div>
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'recommend' }" @click="activeTab = 'recommend'">🎯 推荐放置</button>
      <button class="tab-btn" :class="{ active: activeTab === 'config' }" @click="activeTab = 'config'">⚙️ 配置控制</button>
      <button class="tab-btn" :class="{ active: activeTab === 'loyalty' }" @click="activeTab = 'loyalty'">🕵️ 忠诚度排行</button>
    </div>

    <!-- 推荐放置 -->
    <div v-if="activeTab === 'recommend'" class="section">
      <div class="recommend-grid">
        <div v-for="(r, idx) in recommendations" :key="idx" class="recommend-card" :class="{ pareto: r.pareto_optimal }">
          <div class="rec-header">
            <span class="rec-channel">{{ channelIcon(r.channel) }} {{ r.channel }}</span>
            <span v-if="r.pareto_optimal" class="pareto-badge">⭐ 帕累托最优</span>
          </div>
          <div class="rec-body">
            <div class="rec-row"><span class="rec-label">推荐等级</span><span class="rec-val">Lv.{{ r.level }}</span></div>
            <div class="rec-row"><span class="rec-label">误伤减少</span><span class="rec-val rec-good">↓ {{ r.false_positive_reduction ?? 0 }}%</span></div>
            <div class="rec-row"><span class="rec-label">暴露提升</span><span class="rec-val rec-warn">↑ {{ r.exposure_increase ?? 0 }}%</span></div>
          </div>
        </div>
        <div v-if="recommendations.length === 0" class="empty-state">暂无推荐</div>
      </div>
    </div>

    <!-- 配置控制 -->
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-panel">
        <h3 class="section-title">通道暴露等级配置</h3>
        <div class="slider-group" v-for="ch in configChannels" :key="ch.key">
          <div class="slider-header">
            <span class="slider-label">{{ ch.icon }} {{ ch.label }}</span>
            <span class="slider-value">{{ ch.value }}</span>
          </div>
          <input type="range" class="slider" min="0" max="5" step="1" v-model.number="ch.value">
          <div class="slider-ticks">
            <span v-for="n in 6" :key="n">{{ n - 1 }}</span>
          </div>
        </div>
        <button class="btn btn-primary" @click="saveConfig" :disabled="saving" style="margin-top: var(--space-3)">
          {{ saving ? '保存中...' : '💾 保存配置' }}
        </button>
        <div v-if="saveMsg" class="save-msg" :class="saveMsgType">{{ saveMsg }}</div>
      </div>
    </div>

    <!-- 忠诚度排行 -->
    <div v-if="activeTab === 'loyalty'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>攻击者 ID</th>
              <th>总交互</th>
              <th>忠诚度分</th>
              <th>阶段</th>
              <th>首次</th>
              <th>最近</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="l in loyaltyList" :key="l.attacker_id || l.id">
              <td class="td-mono">{{ truncate(l.attacker_id || l.id, 16) }}</td>
              <td class="td-mono">{{ l.total_interactions ?? l.interactions }}</td>
              <td>
                <span class="loyalty-score" :style="{ color: loyaltyColor(l.loyalty_score ?? l.score) }">
                  {{ (l.loyalty_score ?? l.score ?? 0).toFixed(2) }}
                </span>
              </td>
              <td><span class="phase-badge">{{ l.phase || l.stage || '-' }}</span></td>
              <td class="td-mono">{{ formatTime(l.first_seen) }}</td>
              <td class="td-mono">{{ formatTime(l.last_seen) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="loyaltyList.length === 0" class="empty-state">暂无忠诚度数据</div>
      </div>
    </div>

    <!-- 错误提示 -->
    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { api, apiPut } from '../api.js'

const activeTab = ref('recommend')
const budget = ref({})
const recommendations = ref([])
const loyaltyList = ref([])
const error = ref('')
const saving = ref(false)
const saveMsg = ref('')
const saveMsgType = ref('')

const configChannels = reactive([
  { key: 'im', icon: '💬', label: 'IM', value: 0 },
  { key: 'llm', icon: '🧠', label: 'LLM', value: 0 },
  { key: 'toolcall', icon: '🔧', label: 'ToolCall', value: 0 },
])

const channels = computed(() => {
  const b = budget.value
  return [
    { name: 'im', icon: '💬', label: 'IM', level: b.im_level ?? b.channels?.im ?? 0 },
    { name: 'llm', icon: '🧠', label: 'LLM', level: b.llm_level ?? b.channels?.llm ?? 0 },
    { name: 'toolcall', icon: '🔧', label: 'ToolCall', level: b.toolcall_level ?? b.channels?.toolcall ?? 0 },
  ]
})

const budgetPercent = computed(() => {
  const total = budget.value.total_budget || 1
  const alloc = budget.value.allocated || 0
  return Math.min(100, Math.round((alloc / total) * 100))
})

const budgetColor = computed(() => {
  if (budget.value.over_budget) return '#EF4444'
  if (budgetPercent.value >= 80) return '#F59E0B'
  return '#10B981'
})

const budgetArc = computed(() => {
  const circumference = 2 * Math.PI * 50
  const filled = (budgetPercent.value / 100) * circumference
  return `${filled} ${circumference}`
})

function channelColor(level) {
  if (level >= 4) return '#EF4444'
  if (level >= 3) return '#F59E0B'
  if (level >= 2) return '#3B82F6'
  return '#10B981'
}

function channelIcon(name) {
  const icons = { im: '💬', IM: '💬', llm: '🧠', LLM: '🧠', toolcall: '🔧', ToolCall: '🔧' }
  return icons[name] || '📡'
}

function loyaltyColor(score) {
  if (score >= 0.8) return '#EF4444'
  if (score >= 0.5) return '#F59E0B'
  return '#10B981'
}

async function loadBudget() {
  try {
    const d = await api('/api/v1/singularity/budget')
    budget.value = d
    // sync config sliders
    configChannels[0].value = d.im_level ?? d.channels?.im ?? 0
    configChannels[1].value = d.llm_level ?? d.channels?.llm ?? 0
    configChannels[2].value = d.toolcall_level ?? d.channels?.toolcall ?? 0
  } catch (e) { error.value = '加载预算失败: ' + e.message }
}

async function loadRecommendations() {
  try { const d = await api('/api/v1/singularity/recommend'); recommendations.value = d.recommendations || d || [] } catch (e) { error.value = '加载推荐失败: ' + e.message }
}

async function loadLoyalty() {
  try { const d = await api('/api/v1/honeypot/loyalty'); loyaltyList.value = d.loyalty || d.attackers || d || [] } catch (e) { error.value = '加载忠诚度失败: ' + e.message }
}

async function saveConfig() {
  saving.value = true
  saveMsg.value = ''
  try {
    const body = {
      im_level: configChannels[0].value,
      llm_level: configChannels[1].value,
      toolcall_level: configChannels[2].value,
    }
    await apiPut('/api/v1/singularity/config', body)
    saveMsg.value = '✅ 配置已保存'
    saveMsgType.value = 'success'
    loadBudget()
  } catch (e) {
    saveMsg.value = '❌ 保存失败: ' + e.message
    saveMsgType.value = 'error'
  } finally {
    saving.value = false
  }
}

function loadAll() {
  error.value = ''
  loadBudget()
  loadRecommendations()
  loadLoyalty()
}

function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function formatTime(ts) {
  if (!ts) return '-'
  try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) } catch { return ts }
}

onMounted(loadAll)
</script>

<style scoped>
.singularity-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

/* Over budget banner */
.over-budget-banner {
  background: linear-gradient(90deg, rgba(239,68,68,.15), rgba(220,38,38,.15));
  border: 1px solid rgba(239,68,68,.4); border-radius: var(--radius-md);
  padding: var(--space-3); margin-bottom: var(--space-4);
  color: #FCA5A5; font-size: var(--text-sm); font-weight: 600;
}

/* Budget Dashboard */
.budget-dashboard {
  display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4);
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-5); margin-bottom: var(--space-4);
}
.budget-ring-area { display: flex; align-items: center; gap: var(--space-5); flex-wrap: wrap; }
.ring-container { position: relative; width: 120px; height: 120px; flex-shrink: 0; }
.ring-svg { width: 100%; height: 100%; }
.ring-text { position: absolute; top: 50%; left: 50%; transform: translate(-50%,-50%); text-align: center; }
.ring-num { font-size: 1.5rem; font-weight: 800; line-height: 1.2; }
.ring-label { font-size: 11px; color: var(--text-tertiary); }
.budget-stats { display: flex; flex-direction: column; gap: var(--space-2); }
.b-stat { display: flex; flex-direction: column; }
.b-val { font-size: 1.1rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.b-label { font-size: 10px; color: var(--text-tertiary); }

/* Channel bars */
.channels-area { display: flex; flex-direction: column; justify-content: center; }
.section-title { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); }
.channel-bar { margin-bottom: var(--space-3); }
.ch-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.ch-name { font-size: var(--text-sm); color: var(--text-secondary); font-weight: 600; }
.ch-level { font-size: var(--text-xs); color: var(--text-tertiary); font-family: var(--font-mono); }
.ch-track { height: 8px; background: rgba(255,255,255,0.06); border-radius: 4px; overflow: hidden; }
.ch-fill { height: 100%; border-radius: 4px; transition: width .3s ease; }

/* Tabs */
.tab-bar { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }

.section { margin-bottom: var(--space-4); }

/* Recommend Cards */
.recommend-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: var(--space-3); }
.recommend-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-3); transition: all .2s; }
.recommend-card:hover { border-color: var(--color-primary); }
.recommend-card.pareto { border-color: rgba(245,158,11,.4); background: rgba(245,158,11,.04); }
.rec-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-2); }
.rec-channel { font-weight: 700; color: var(--text-primary); font-size: var(--text-sm); }
.pareto-badge { font-size: 10px; color: #F59E0B; font-weight: 700; padding: 1px 6px; background: rgba(245,158,11,.15); border-radius: 4px; }
.rec-body { display: flex; flex-direction: column; gap: var(--space-1); }
.rec-row { display: flex; justify-content: space-between; font-size: var(--text-xs); }
.rec-label { color: var(--text-tertiary); }
.rec-val { color: var(--text-primary); font-weight: 600; font-family: var(--font-mono); }
.rec-good { color: #10B981; }
.rec-warn { color: #F59E0B; }

/* Config Panel */
.config-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); max-width: 480px; }
.slider-group { margin-bottom: var(--space-4); }
.slider-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-1); }
.slider-label { font-size: var(--text-sm); color: var(--text-secondary); font-weight: 600; }
.slider-value { font-size: var(--text-base); font-weight: 800; color: var(--color-primary); font-family: var(--font-mono); }
.slider {
  -webkit-appearance: none; appearance: none; width: 100%; height: 6px;
  background: rgba(255,255,255,0.1); border-radius: 3px; outline: none;
}
.slider::-webkit-slider-thumb {
  -webkit-appearance: none; appearance: none; width: 18px; height: 18px;
  border-radius: 50%; background: var(--color-primary); cursor: pointer; border: 2px solid #fff;
}
.slider::-moz-range-thumb {
  width: 18px; height: 18px; border-radius: 50%; background: var(--color-primary); cursor: pointer; border: 2px solid #fff;
}
.slider-ticks { display: flex; justify-content: space-between; font-size: 10px; color: var(--text-tertiary); margin-top: 2px; padding: 0 2px; }

.save-msg { margin-top: var(--space-2); font-size: var(--text-sm); font-weight: 600; }
.save-msg.success { color: #10B981; }
.save-msg.error { color: #EF4444; }

/* Loyalty Table */
.table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th {
  text-align: left; padding: 8px 10px; background: var(--bg-elevated);
  color: var(--text-tertiary); font-weight: 600; font-size: 10px;
  text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle);
  white-space: nowrap;
}
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }

.loyalty-score { font-weight: 800; font-family: var(--font-mono); }
.phase-badge { display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 600; background: rgba(99,102,241,.15); color: #a5b4fc; }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-sm { padding: 6px 12px; font-size: var(--text-xs); }

.empty-state { text-align: center; padding: var(--space-6); color: var(--text-tertiary); }

.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); }

@media (max-width: 768px) {
  .budget-dashboard { grid-template-columns: 1fr; }
  .budget-ring-area { justify-content: center; }
  .recommend-grid { grid-template-columns: 1fr; }
}
</style>
