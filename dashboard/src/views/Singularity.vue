<template>
  <div class="singularity-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="orbit" :size="20" /> 奇点蜜罐</h1>
        <p class="page-subtitle">拓扑感知蜜罐预算 — 以欧拉示性数约束暴露面，精准放置蜜罐</p>
      </div>
      <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
    </div>

    <!-- 超预算警告 -->
    <div v-if="budget.over_budget" class="over-budget-banner">
      <Icon name="alert-triangle" :size="14" /> <strong>预算超支！</strong>当前蜜罐暴露面超出拓扑预算限制，请减少通道暴露等级。
    </div>

    <!-- 统计面板 -->
    <div class="stats-grid">
      <StatCard :iconSvg="svgDeploy" label="部署数" :value="budget.allocated ?? 0" color="indigo" />
      <StatCard :iconSvg="svgTrigger" label="触发数" :value="triggerCount" color="yellow" />
      <StatCard :iconSvg="svgAgent" label="捕获 Agent" :value="capturedAgents" color="red" />
      <StatCard :iconSvg="svgEuler" label="χ 欧拉数" :value="budget.euler_characteristic ?? '-'" color="green" />
    </div>

    <!-- 奇点预算仪表盘 -->
    <div class="budget-dashboard">
      <div class="budget-ring-area">
        <div class="ring-container">
          <svg viewBox="0 0 120 120" class="ring-svg">
            <circle cx="60" cy="60" r="50" fill="none" stroke="rgba(255,255,255,0.08)" stroke-width="10"/>
            <circle cx="60" cy="60" r="50" fill="none" :stroke="budgetColor" stroke-width="10" stroke-linecap="round" :stroke-dasharray="budgetArc" stroke-dashoffset="0" transform="rotate(-90 60 60)"/>
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
      <div class="channels-area">
        <h3 class="section-title">通道暴露等级</h3>
        <div class="channel-bar" v-for="ch in channels" :key="ch.name">
          <div class="ch-header">
            <span class="ch-name"><Icon :name="ch.icon" :size="14" /> {{ ch.label }}</span>
            <span class="ch-level">Lv.{{ ch.level }} / 5</span>
          </div>
          <div class="ch-track"><div class="ch-fill" :style="{ width: (ch.level / 5 * 100) + '%', background: channelColor(ch.level) }"></div></div>
        </div>
      </div>
    </div>

    <!-- Tab -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'recommend' }" @click="activeTab = 'recommend'"><Icon name="crosshair" :size="14" /> 推荐放置</button>
      <button class="tab-btn" :class="{ active: activeTab === 'config' }" @click="activeTab = 'config'"><Icon name="settings" :size="14" /> 配置控制</button>
      <button class="tab-btn" :class="{ active: activeTab === 'captures' }" @click="activeTab = 'captures'">🕵️ 捕获记录 <span class="tab-count">{{ loyaltyList.length }}</span></button>
      <button class="tab-btn" :class="{ active: activeTab === 'history' }" @click="activeTab = 'history'"><Icon name="clock" :size="14" /> 历史 <span class="tab-count">{{ historyList.length }}</span></button>
    </div>

    <!-- 推荐放置 -->
    <div v-if="activeTab === 'recommend'" class="section">
      <div class="recommend-grid" v-if="recommendations.length">
        <div v-for="(r, idx) in recommendations" :key="idx" class="recommend-card" :class="{ pareto: r.pareto_optimal }">
          <div class="rec-header">
            <span class="rec-channel"><Icon :name="channelIcon(r.channel)" :size="14" /> {{ r.channel }}</span>
            <span v-if="r.pareto_optimal" class="pareto-badge">⭐ 帕累托最优</span>
          </div>
          <div class="rec-body">
            <div class="rec-row"><span class="rec-label">推荐等级</span><span class="rec-val">Lv.{{ r.level }}</span></div>
            <div class="rec-row"><span class="rec-label">误伤减少</span><span class="rec-val rec-good">↓ {{ r.false_positive_reduction ?? 0 }}%</span></div>
            <div class="rec-row"><span class="rec-label">暴露提升</span><span class="rec-val rec-warn">↑ {{ r.exposure_increase ?? 0 }}%</span></div>
          </div>
          <button class="btn btn-sm btn-primary" style="margin-top: var(--space-2); width: 100%;" @click="applyRecommendation(r)">应用此配置</button>
        </div>
      </div>
      <EmptyState v-else icon="🎯" title="暂无推荐" description="系统将根据拓扑分析生成最优放置建议" />
    </div>

    <!-- 配置控制 -->
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-grid">
        <div class="config-panel">
          <h3 class="section-title">通道暴露等级配置</h3>
          <div class="slider-group" v-for="ch in configChannels" :key="ch.key">
            <div class="slider-header">
              <span class="slider-label"><Icon :name="ch.icon" :size="14" /> {{ ch.label }}</span>
              <span class="slider-value">{{ ch.value }}</span>
            </div>
            <input type="range" class="slider" min="0" max="5" step="1" v-model.number="ch.value">
            <div class="slider-ticks"><span v-for="n in 6" :key="n">{{ n - 1 }}</span></div>
          </div>
          <button class="btn btn-primary" @click="saveConfig" :disabled="saving" style="margin-top: var(--space-3); width: 100%;">
            {{ saving ? '保存中...' : '保存配置' }}
          </button>
        </div>
        <div class="config-panel">
          <h3 class="section-title">诱饵内容配置</h3>
          <div class="form-group"><label>诱饵类型</label>
            <select v-model="baitConfig.type" class="form-input"><option value="fake_credential">假凭据</option><option value="canary_token">金丝雀令牌</option><option value="honeydoc">蜜罐文档</option><option value="fake_endpoint">假端点</option></select>
          </div>
          <div class="form-group"><label>诱饵内容</label>
            <textarea v-model="baitConfig.content" rows="4" class="form-input" placeholder="设置诱饵内容..."></textarea>
          </div>
          <div class="form-group"><label>触发条件</label>
            <select v-model="baitConfig.trigger_condition" class="form-input"><option value="access">被访问时</option><option value="extract">被提取时</option><option value="modify">被修改时</option><option value="exfiltrate">被外传时</option></select>
          </div>
          <div class="form-group"><label>告警级别</label>
            <select v-model="baitConfig.alert_level" class="form-input"><option value="info">信息</option><option value="warning">警告</option><option value="critical">严重</option></select>
          </div>
          <button class="btn btn-primary" @click="saveBaitConfig" :disabled="savingBait" style="width: 100%;">
            {{ savingBait ? '保存中...' : '保存诱饵配置' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 捕获记录 -->
    <div v-if="activeTab === 'captures'" class="section">
      <div class="section-toolbar">
        <div class="search-box"><Icon name="search" :size="14" /><input v-model="captureSearch" placeholder="搜索攻击者ID..." /></div>
      </div>
      <div class="table-wrap" v-if="filteredLoyalty.length">
        <table class="data-table">
          <thead><tr><th>攻击者 ID</th><th>总交互</th><th>忠诚度分</th><th>阶段</th><th>首次</th><th>最近</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="l in filteredLoyalty" :key="l.attacker_id || l.id" @click="toggleCapture(l)" style="cursor:pointer">
              <td class="td-mono">{{ truncate(l.attacker_id || l.id, 20) }}</td>
              <td class="td-mono">{{ l.total_interactions ?? l.interactions }}</td>
              <td><span class="loyalty-score" :style="{ color: loyaltyColor(l.loyalty_score ?? l.score) }">{{ (l.loyalty_score ?? l.score ?? 0).toFixed(2) }}</span></td>
              <td><span class="phase-badge">{{ l.phase || l.stage || '-' }}</span></td>
              <td class="td-mono">{{ formatTime(l.first_seen) }}</td>
              <td class="td-mono">{{ formatTime(l.last_seen) }}</td>
              <td><button class="btn btn-sm btn-ghost" @click.stop="feedbackAttacker(l)"><Icon name="refresh" :size="12" /> 回馈</button></td>
            </tr>
            <template v-if="expandedCapture === (l => l.attacker_id || l.id)"><!-- handled below --></template>
          </tbody>
        </table>
        <!-- Expanded detail -->
        <div v-if="expandedCaptureData" class="capture-detail">
          <div class="detail-header">
            <h4>攻击者详情: {{ expandedCaptureData.attacker_id || expandedCaptureData.id }}</h4>
            <button class="btn btn-sm btn-ghost" @click="expandedCapture = null; expandedCaptureData = null">✕</button>
          </div>
          <div class="detail-grid">
            <div class="detail-item"><span class="detail-label">忠诚度分</span><span class="detail-val" :style="{ color: loyaltyColor(expandedCaptureData.loyalty_score ?? expandedCaptureData.score) }">{{ (expandedCaptureData.loyalty_score ?? expandedCaptureData.score ?? 0).toFixed(4) }}</span></div>
            <div class="detail-item"><span class="detail-label">总交互</span><span class="detail-val">{{ expandedCaptureData.total_interactions ?? expandedCaptureData.interactions }}</span></div>
            <div class="detail-item"><span class="detail-label">阶段</span><span class="detail-val">{{ expandedCaptureData.phase || expandedCaptureData.stage || '-' }}</span></div>
            <div class="detail-item"><span class="detail-label">首次发现</span><span class="detail-val">{{ formatTime(expandedCaptureData.first_seen) }}</span></div>
            <div class="detail-item"><span class="detail-label">最近活动</span><span class="detail-val">{{ formatTime(expandedCaptureData.last_seen) }}</span></div>
          </div>
        </div>
      </div>
      <EmptyState v-else icon="🕵️" title="暂无捕获记录" description="当攻击者与蜜罐交互后，记录将出现在这里" />
    </div>

    <!-- 历史记录 -->
    <div v-if="activeTab === 'history'" class="section">
      <div class="history-list" v-if="historyList.length">
        <div v-for="h in historyList" :key="h.id || h.timestamp" class="history-card">
          <div class="history-header">
            <span class="history-time mono">{{ formatTime(h.timestamp || h.created_at) }}</span>
            <span class="history-type badge">{{ h.event_type || h.type || 'event' }}</span>
          </div>
          <div class="history-body">{{ h.description || h.message || h.details || JSON.stringify(h) }}</div>
        </div>
      </div>
      <EmptyState v-else icon="📜" title="暂无历史" description="奇点蜜罐操作历史将在这里显示" />
    </div>

    <!-- 确认弹窗 -->
    <ConfirmModal :visible="confirmAction.show" :title="confirmAction.title" :message="confirmAction.message" :type="confirmAction.type" :confirm-text="confirmAction.confirmText" @confirm="confirmAction.onConfirm(); confirmAction.show = false" @cancel="confirmAction.show = false" />

    <!-- 错误提示 -->
    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { api, apiPut } from '../api.js'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import EmptyState from '../components/EmptyState.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { showToast } from '../stores/app.js'

const activeTab = ref('recommend')
const budget = ref({})
const recommendations = ref([])
const loyaltyList = ref([])
const historyList = ref([])
const error = ref('')
const saving = ref(false)
const savingBait = ref(false)
const captureSearch = ref('')
const expandedCapture = ref(null)
const expandedCaptureData = ref(null)

const confirmAction = reactive({ show: false, title: '', message: '', type: 'warning', confirmText: '确认', onConfirm: () => {} })

const baitConfig = reactive({ type: 'fake_credential', content: '', trigger_condition: 'access', alert_level: 'warning' })

const configChannels = reactive([
  { key: 'im', icon: 'message-circle', label: 'IM', value: 0 },
  { key: 'llm', icon: 'brain', label: 'LLM', value: 0 },
  { key: 'toolcall', icon: 'wrench', label: 'ToolCall', value: 0 },
])

const svgDeploy = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
const svgTrigger = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>'
const svgAgent = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="16" height="16" rx="2" ry="2"/><circle cx="9" cy="10" r="1.5"/><circle cx="15" cy="10" r="1.5"/><path d="M9 16h6"/></svg>'
const svgEuler = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>'

const channels = computed(() => {
  const b = budget.value
  return [
    { name: 'im', icon: 'message-circle', label: 'IM', level: b.im_level ?? b.channels?.im ?? 0 },
    { name: 'llm', icon: 'brain', label: 'LLM', level: b.llm_level ?? b.channels?.llm ?? 0 },
    { name: 'toolcall', icon: 'wrench', label: 'ToolCall', level: b.toolcall_level ?? b.channels?.toolcall ?? 0 },
  ]
})

const budgetPercent = computed(() => { const total = budget.value.total_budget || 1; const alloc = budget.value.allocated || 0; return Math.min(100, Math.round((alloc / total) * 100)) })
const budgetColor = computed(() => { if (budget.value.over_budget) return '#EF4444'; if (budgetPercent.value >= 80) return '#F59E0B'; return '#10B981' })
const budgetArc = computed(() => { const c = 2 * Math.PI * 50; return `${(budgetPercent.value / 100) * c} ${c}` })

const triggerCount = computed(() => { let c = 0; for (const l of loyaltyList.value) c += (l.total_interactions ?? l.interactions ?? 0); return c })
const capturedAgents = computed(() => loyaltyList.value.length)

const filteredLoyalty = computed(() => {
  if (!captureSearch.value) return loyaltyList.value
  const q = captureSearch.value.toLowerCase()
  return loyaltyList.value.filter(l => ((l.attacker_id||l.id||'') + '').toLowerCase().includes(q))
})

function channelColor(level) { if (level >= 4) return '#EF4444'; if (level >= 3) return '#F59E0B'; if (level >= 2) return '#3B82F6'; return '#10B981' }
function channelIcon(name) { return { im: 'message-circle', IM: 'message-circle', llm: 'brain', LLM: 'brain', toolcall: 'wrench', ToolCall: 'wrench' }[name] || 'radio' }
function loyaltyColor(score) { if (score >= 0.8) return '#EF4444'; if (score >= 0.5) return '#F59E0B'; return '#10B981' }

async function loadBudget() {
  try { const d = await api('/api/v1/singularity/budget'); budget.value = d; configChannels[0].value = d.im_level ?? d.channels?.im ?? 0; configChannels[1].value = d.llm_level ?? d.channels?.llm ?? 0; configChannels[2].value = d.toolcall_level ?? d.channels?.toolcall ?? 0 }
  catch (e) { error.value = '加载预算失败: ' + e.message }
}
async function loadRecommendations() { try { const d = await api('/api/v1/singularity/recommend'); recommendations.value = d.recommendations || d || [] } catch (e) { error.value = '加载推荐失败: ' + e.message } }
async function loadLoyalty() { try { const d = await api('/api/v1/honeypot/loyalty'); loyaltyList.value = d.loyalty || d.attackers || d.curves || d || [] } catch (e) { error.value = '加载捕获记录失败: ' + e.message } }
async function loadHistory() { try { const d = await api('/api/v1/singularity/history?limit=100'); historyList.value = Array.isArray(d) ? d : d.history || [] } catch {} }

async function saveConfig() {
  saving.value = true
  try {
    await apiPut('/api/v1/singularity/config', { im_level: configChannels[0].value, llm_level: configChannels[1].value, toolcall_level: configChannels[2].value })
    showToast('配置已保存', 'success'); loadBudget()
  } catch (e) { showToast('保存失败: ' + e.message, 'error') }
  finally { saving.value = false }
}

async function saveBaitConfig() {
  savingBait.value = true
  try {
    await apiPut('/api/v1/singularity/config', { ...baitConfig, im_level: configChannels[0].value, llm_level: configChannels[1].value, toolcall_level: configChannels[2].value })
    showToast('诱饵配置已保存', 'success')
  } catch (e) { showToast('保存失败: ' + e.message, 'error') }
  finally { savingBait.value = false }
}

function applyRecommendation(r) {
  Object.assign(confirmAction, { show: true, title: '应用推荐配置', message: '将 ' + r.channel + ' 通道设置为 Lv.' + r.level + '？', type: 'info', confirmText: '应用', onConfirm: () => {
    const ch = configChannels.find(c => c.label.toLowerCase() === r.channel.toLowerCase() || c.key === r.channel.toLowerCase())
    if (ch) { ch.value = r.level; saveConfig() }
  }})
}

function toggleCapture(l) {
  const id = l.attacker_id || l.id
  if (expandedCapture.value === id) { expandedCapture.value = null; expandedCaptureData.value = null }
  else { expandedCapture.value = id; expandedCaptureData.value = l }
}

async function feedbackAttacker(l) {
  const id = l.attacker_id || l.id
  try { await api('/api/v1/honeypot/feedback/' + encodeURIComponent(id), { method: 'POST' }); showToast('回馈已触发', 'success'); loadLoyalty() }
  catch (e) { showToast('回馈失败: ' + e.message, 'error') }
}

function loadAll() { error.value = ''; loadBudget(); loadRecommendations(); loadLoyalty(); loadHistory() }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '...' : s || '-' }
function formatTime(ts) { if (!ts) return '-'; try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) } catch { return ts } }

onMounted(loadAll)
</script>

<style scoped>
.singularity-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

.over-budget-banner { background: linear-gradient(90deg, rgba(239,68,68,.15), rgba(220,38,38,.15)); border: 1px solid rgba(239,68,68,.4); border-radius: var(--radius-md); padding: var(--space-3); margin-bottom: var(--space-4); color: #FCA5A5; font-size: var(--text-sm); font-weight: 600; display: flex; align-items: center; gap: var(--space-2); }
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }

/* Budget Dashboard */
.budget-dashboard { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-5); margin-bottom: var(--space-4); }
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
.channels-area { display: flex; flex-direction: column; justify-content: center; }
.section-title { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); }
.channel-bar { margin-bottom: var(--space-3); }
.ch-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.ch-name { font-size: var(--text-sm); color: var(--text-secondary); font-weight: 600; display: flex; align-items: center; gap: 4px; }
.ch-level { font-size: var(--text-xs); color: var(--text-tertiary); font-family: var(--font-mono); }
.ch-track { height: 8px; background: rgba(255,255,255,0.06); border-radius: 4px; overflow: hidden; }
.ch-fill { height: 100%; border-radius: 4px; transition: width .3s ease; }

/* Tabs */
.tab-bar { display: flex; gap: var(--space-1); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); overflow-x: auto; }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; white-space: nowrap; display: flex; align-items: center; gap: 6px; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }
.tab-count { font-size: 10px; background: var(--bg-elevated); padding: 1px 6px; border-radius: 9999px; color: var(--text-tertiary); font-weight: 600; }
.tab-btn.active .tab-count { background: rgba(99,102,241,.15); color: var(--color-primary); }
.section { margin-bottom: var(--space-4); }
.section-toolbar { display: flex; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; align-items: center; }
.search-box { display: flex; align-items: center; gap: var(--space-2); background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-1) var(--space-2); flex: 1; min-width: 200px; }
.search-box input { background: none; border: none; color: var(--text-primary); font-size: var(--text-sm); outline: none; width: 100%; }

/* Recommend Cards */
.recommend-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: var(--space-3); }
.recommend-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-3); transition: all .2s; }
.recommend-card:hover { border-color: var(--color-primary); box-shadow: 0 2px 12px rgba(99,102,241,.08); }
.recommend-card.pareto { border-color: rgba(245,158,11,.4); background: rgba(245,158,11,.04); }
.rec-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-2); }
.rec-channel { font-weight: 700; color: var(--text-primary); font-size: var(--text-sm); display: flex; align-items: center; gap: 4px; }
.pareto-badge { font-size: 10px; color: #F59E0B; font-weight: 700; padding: 1px 6px; background: rgba(245,158,11,.15); border-radius: 4px; }
.rec-body { display: flex; flex-direction: column; gap: var(--space-1); }
.rec-row { display: flex; justify-content: space-between; font-size: var(--text-xs); }
.rec-label { color: var(--text-tertiary); }
.rec-val { color: var(--text-primary); font-weight: 600; font-family: var(--font-mono); }
.rec-good { color: #10B981; }
.rec-warn { color: #F59E0B; }

/* Config */
.config-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
.config-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.slider-group { margin-bottom: var(--space-4); }
.slider-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-1); }
.slider-label { font-size: var(--text-sm); color: var(--text-secondary); font-weight: 600; display: flex; align-items: center; gap: 4px; }
.slider-value { font-size: var(--text-base); font-weight: 800; color: var(--color-primary); font-family: var(--font-mono); }
.slider { -webkit-appearance: none; appearance: none; width: 100%; height: 6px; background: rgba(255,255,255,0.1); border-radius: 3px; outline: none; }
.slider::-webkit-slider-thumb { -webkit-appearance: none; appearance: none; width: 18px; height: 18px; border-radius: 50%; background: var(--color-primary); cursor: pointer; border: 2px solid #fff; }
.slider::-moz-range-thumb { width: 18px; height: 18px; border-radius: 50%; background: var(--color-primary); cursor: pointer; border: 2px solid #fff; }
.slider-ticks { display: flex; justify-content: space-between; font-size: 10px; color: var(--text-tertiary); margin-top: 2px; padding: 0 2px; }
.form-group { margin-bottom: var(--space-3); }
.form-group label { display: block; font-size: var(--text-xs); color: var(--text-secondary); margin-bottom: var(--space-1); font-weight: 600; }
.form-input { width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-2); font-size: var(--text-sm); box-sizing: border-box; }
.form-input:focus { border-color: var(--color-primary); outline: none; box-shadow: 0 0 0 2px rgba(99,102,241,.2); }

/* Capture table */
.table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th { text-align: left; padding: 8px 10px; background: var(--bg-elevated); color: var(--text-tertiary); font-weight: 600; font-size: 10px; text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle); white-space: nowrap; }
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.loyalty-score { font-weight: 800; font-family: var(--font-mono); }
.phase-badge { display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 600; background: rgba(99,102,241,.15); color: #a5b4fc; }
.badge { font-size: 10px; padding: 2px 6px; border-radius: 4px; background: rgba(99,102,241,.15); color: #a5b4fc; font-weight: 600; }

/* Capture detail */
.capture-detail { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); margin-top: var(--space-3); }
.detail-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-3); }
.detail-header h4 { margin: 0; font-size: var(--text-sm); color: var(--text-primary); }
.detail-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: var(--space-3); }
.detail-item { display: flex; flex-direction: column; gap: 2px; }
.detail-label { font-size: 10px; color: var(--text-tertiary); font-weight: 600; text-transform: uppercase; letter-spacing: .05em; }
.detail-val { font-size: var(--text-sm); color: var(--text-primary); font-weight: 600; }

/* History */
.history-list { display: flex; flex-direction: column; gap: var(--space-2); }
.history-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-3); }
.history-header { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-1); }
.history-time { font-size: var(--text-xs); color: var(--text-tertiary); }
.history-body { font-size: var(--text-sm); color: var(--text-secondary); }
.mono { font-family: var(--font-mono); font-size: 11px; }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-sm { padding: 6px 12px; font-size: var(--text-xs); }
.btn-ghost { background: transparent; border-color: transparent; color: var(--text-secondary); }
.btn-ghost:hover { background: var(--bg-elevated); color: var(--text-primary); }
.btn-danger { color: #EF4444; border-color: rgba(239,68,68,.3); }
.btn-danger:hover { background: rgba(239,68,68,.1); }

.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); }

@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .budget-dashboard { grid-template-columns: 1fr; }
  .budget-ring-area { justify-content: center; }
  .recommend-grid { grid-template-columns: 1fr; }
  .config-grid { grid-template-columns: 1fr; }
}
</style>
