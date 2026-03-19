<template>
  <div class="custom-dashboard">
    <!-- Toolbar -->
    <div class="cd-toolbar">
      <div class="cd-toolbar-left">
        <span class="cd-title"><Icon name="grid" :size="16" /> 自定义布局</span>
        <select v-model="selectedPreset" @change="applyPreset" class="cd-preset-select">
          <option value="">选择预设...</option>
          <option v-for="p in presets" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
        <select v-if="savedLayouts.length" v-model="activeLayoutId" @change="loadSavedLayout" class="cd-layout-select">
          <option value="">我的布局...</option>
          <option v-for="l in savedLayouts" :key="l.id" :value="l.id">{{ l.name }} {{ l.is_default ? '⭐' : '' }}</option>
        </select>
      </div>
      <div class="cd-toolbar-right">
        <button class="cd-btn cd-btn-add" @click="showPanelPicker = !showPanelPicker" title="添加面板">
          + 添加面板
        </button>
        <button class="cd-btn cd-btn-save" @click="saveLayout" :disabled="saving" title="保存当前布局">
          {{ saving ? '保存中...' : '保存' }}
        </button>
        <button class="cd-btn cd-btn-reset" @click="resetLayout" title="重置为默认布局">
          <Icon name="refresh" :size="14" /> 重置
        </button>
        <button
          class="cd-btn"
          :class="editMode ? 'cd-btn-edit-on' : 'cd-btn-edit-off'"
          @click="editMode = !editMode"
        >
          {{ editMode ? '✅ 完成编辑' : '✏️ 编辑模式' }}
        </button>
      </div>
    </div>

    <!-- Panel Picker Dropdown -->
    <Transition name="picker-fade">
      <div v-if="showPanelPicker" class="cd-panel-picker">
        <div class="picker-header">
          <span>选择要显示的面板</span>
          <button @click="showPanelPicker = false" class="picker-close">✕</button>
        </div>
        <div class="picker-grid">
          <label
            v-for="ap in allPanelDefs"
            :key="ap.id"
            class="picker-item"
            :class="{ active: isPanelVisible(ap.id) }"
          >
            <input
              type="checkbox"
              :checked="isPanelVisible(ap.id)"
              @change="togglePanelVisibility(ap.id)"
            />
            <span>{{ ap.title }}</span>
          </label>
        </div>
      </div>
    </Transition>

    <!-- Save Dialog -->
    <Transition name="picker-fade">
      <div v-if="showSaveDialog" class="cd-save-dialog-overlay" @click.self="showSaveDialog = false">
        <div class="cd-save-dialog">
          <h3>保存布局</h3>
          <input
            v-model="saveName"
            class="cd-save-input"
            placeholder="布局名称"
            @keyup.enter="confirmSave"
          />
          <textarea
            v-model="saveDesc"
            class="cd-save-textarea"
            placeholder="布局描述（可选）"
            rows="2"
          ></textarea>
          <div class="cd-save-actions">
            <button class="cd-btn cd-btn-save" @click="confirmSave" :disabled="!saveName.trim()">保存</button>
            <button class="cd-btn cd-btn-reset" @click="showSaveDialog = false">取消</button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Draggable Grid -->
    <DraggableGrid
      :panels="panels"
      :editMode="editMode"
      @update:panels="onPanelsUpdate"
      @remove="onPanelRemove"
      @reorder="onPanelReorder"
    >
      <!-- 安全健康分 -->
      <template #health-score>
        <div class="panel-content">
          <div class="mini-score" v-if="healthScore">
            <div class="mini-score-ring">
              <svg viewBox="0 0 80 80" class="mini-ring-svg">
                <circle cx="40" cy="40" r="34" fill="none" :stroke="scoreColor" stroke-width="6" opacity="0.15"/>
                <circle cx="40" cy="40" r="34" fill="none" :stroke="scoreColor" stroke-width="6" stroke-linecap="round" :stroke-dasharray="scoreDash" stroke-dashoffset="0" transform="rotate(-90 40 40)"/>
              </svg>
              <div class="mini-score-num" :style="{color: scoreColor}">{{ healthScore.score }}</div>
            </div>
            <div class="mini-score-info">
              <span class="mini-score-label" :class="'badge-' + healthScore.level">{{ healthScore.level_label }}</span>
              <div class="mini-score-trend" v-if="healthScore.trend && healthScore.trend.length">
                <svg :viewBox="'0 0 120 30'" class="mini-trend-svg">
                  <polyline :points="trendPoints" fill="none" :stroke="scoreColor" stroke-width="1.5" stroke-linejoin="round"/>
                </svg>
              </div>
            </div>
          </div>
          <div v-else class="panel-loading">加载中...</div>
        </div>
      </template>

      <!-- 实时统计 -->
      <template #realtime-stats>
        <div class="panel-content">
          <div class="mini-stats" v-if="stats">
            <div class="mini-stat"><span class="mini-stat-val">{{ stats.total }}</span><span class="mini-stat-label">总请求</span></div>
            <div class="mini-stat"><span class="mini-stat-val stat-red">{{ stats.blocked }}</span><span class="mini-stat-label">拦截</span></div>
            <div class="mini-stat"><span class="mini-stat-val stat-yellow">{{ stats.warned }}</span><span class="mini-stat-label">告警</span></div>
            <div class="mini-stat"><span class="mini-stat-val stat-green">{{ stats.rate }}</span><span class="mini-stat-label">拦截率</span></div>
          </div>
          <div v-else class="panel-loading">加载中...</div>
        </div>
      </template>

      <!-- OWASP 矩阵 -->
      <template #owasp-matrix>
        <div class="panel-content">
          <div v-if="owaspData && owaspData.length" class="owasp-mini-grid">
            <div v-for="item in owaspData.slice(0, 10)" :key="item.id" class="owasp-mini-item" :class="'severity-' + (item.severity || 'low')">
              <span class="owasp-mini-id">{{ item.id }}</span>
              <span class="owasp-mini-name">{{ item.name }}</span>
              <span class="owasp-mini-count">{{ item.hits || 0 }}</span>
            </div>
          </div>
          <div v-else class="panel-empty">暂无 OWASP 数据</div>
        </div>
      </template>

      <!-- 攻击趋势图 -->
      <template #attack-trend>
        <div class="panel-content">
          <div v-if="trendData && trendData.length" class="trend-placeholder">
            <svg :viewBox="'0 0 400 100'" class="trend-full-svg" preserveAspectRatio="none">
              <polyline :points="fullTrendPoints" fill="none" stroke="#3B82F6" stroke-width="2" stroke-linejoin="round"/>
              <polyline :points="fullBlockPoints" fill="none" stroke="#EF4444" stroke-width="2" stroke-linejoin="round"/>
            </svg>
            <div class="trend-legend">
              <span class="legend-item"><span class="legend-dot" style="background:#3B82F6"></span>总量</span>
              <span class="legend-item"><span class="legend-dot" style="background:#EF4444"></span>拦截</span>
            </div>
          </div>
          <div v-else class="panel-empty">暂无趋势数据</div>
        </div>
      </template>

      <!-- 审计日志 -->
      <template #audit-log>
        <div class="panel-content">
          <div v-if="recentLogs && recentLogs.length" class="mini-log-list">
            <div v-for="log in recentLogs.slice(0, 6)" :key="log.id" class="mini-log-item" :class="'log-' + log.action">
              <span class="mini-log-time">{{ fmtTime(log.timestamp) }}</span>
              <span class="mini-log-action" :class="'action-' + log.action">{{ log.action }}</span>
              <span class="mini-log-sender">{{ log.sender_id || '--' }}</span>
              <span class="mini-log-reason">{{ log.reason || '--' }}</span>
            </div>
          </div>
          <div v-else class="panel-empty">暂无审计日志</div>
        </div>
      </template>

      <!-- 攻击链摘要 -->
      <template #attack-chain>
        <div class="panel-content">
          <div v-if="attackChains && attackChains.length" class="mini-chains">
            <div v-for="c in attackChains.slice(0, 5)" :key="c.id" class="mini-chain-item">
              <span class="chain-severity" :class="'sev-' + c.risk_level">{{ c.risk_level }}</span>
              <span class="chain-name">{{ c.summary || c.pattern_name || '未命名攻击链' }}</span>
              <span class="chain-steps">{{ c.step_count || 0 }} 步</span>
            </div>
          </div>
          <div v-else class="panel-empty">暂无攻击链数据</div>
        </div>
      </template>

      <!-- 蜜罐统计 -->
      <template #honeypot-stats>
        <div class="panel-content">
          <div v-if="honeypotData" class="mini-stats">
            <div class="mini-stat"><span class="mini-stat-val">{{ honeypotData.total_templates || 0 }}</span><span class="mini-stat-label">模板</span></div>
            <div class="mini-stat"><span class="mini-stat-val stat-red">{{ honeypotData.total_triggers || 0 }}</span><span class="mini-stat-label">触发</span></div>
            <div class="mini-stat"><span class="mini-stat-val stat-yellow">{{ honeypotData.unique_agents || 0 }}</span><span class="mini-stat-label">Agent</span></div>
          </div>
          <div v-else class="panel-empty">暂无蜜罐数据</div>
        </div>
      </template>

      <!-- 红队结果 -->
      <template #redteam-results>
        <div class="panel-content">
          <div v-if="redteamData" class="mini-stats">
            <div class="mini-stat"><span class="mini-stat-val">{{ redteamData.total || 0 }}</span><span class="mini-stat-label">测试总数</span></div>
            <div class="mini-stat"><span class="mini-stat-val stat-green">{{ redteamData.passed || 0 }}</span><span class="mini-stat-label">通过</span></div>
            <div class="mini-stat"><span class="mini-stat-val stat-red">{{ redteamData.failed || 0 }}</span><span class="mini-stat-label">失败</span></div>
          </div>
          <div v-else class="panel-empty">暂无红队数据</div>
        </div>
      </template>

      <!-- 排行榜 -->
      <template #leaderboard>
        <div class="panel-content">
          <div v-if="leaderboardData && leaderboardData.length" class="mini-lb">
            <div v-for="(item, i) in leaderboardData.slice(0, 5)" :key="i" class="mini-lb-row">
              <span class="lb-rank">#{{ i + 1 }}</span>
              <span class="lb-name">{{ item.tenant_name || item.tenant_id || '--' }}</span>
              <span class="lb-score">{{ item.total_score?.toFixed(1) || '--' }}</span>
            </div>
          </div>
          <div v-else class="panel-empty">暂无排行榜数据</div>
        </div>
      </template>

      <!-- A/B 测试 -->
      <template #ab-testing>
        <div class="panel-content">
          <div v-if="abTestData && abTestData.length" class="mini-ab">
            <div v-for="ab in abTestData.slice(0, 3)" :key="ab.id" class="mini-ab-item">
              <span class="ab-status" :class="'ab-' + ab.status">{{ ab.status }}</span>
              <span class="ab-name">{{ ab.name }}</span>
            </div>
          </div>
          <div v-else class="panel-empty">暂无 A/B 测试</div>
        </div>
      </template>

      <!-- 行为异常 -->
      <template #behavior-anomaly>
        <div class="panel-content">
          <div v-if="anomalyData && anomalyData.length" class="mini-anomalies">
            <div v-for="a in anomalyData.slice(0, 5)" :key="a.id" class="mini-anomaly-item">
              <span class="anomaly-sev" :class="'sev-' + a.severity">{{ a.severity }}</span>
              <span class="anomaly-metric">{{ a.metric_name }}</span>
              <span class="anomaly-dev">{{ a.deviation?.toFixed(1) || '--' }}σ</span>
            </div>
          </div>
          <div v-else class="panel-empty">暂无异常数据</div>
        </div>
      </template>
    </DraggableGrid>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, inject } from 'vue'
import Icon from '../components/Icon.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import DraggableGrid from '../components/DraggableGrid.vue'

const showToast = inject('showToast', (msg) => alert(msg))

// State
const editMode = ref(false)
const panels = ref([])
const presets = ref([])
const allPanelDefs = ref([])
const savedLayouts = ref([])
const activeLayoutId = ref('')
const selectedPreset = ref('')
const showPanelPicker = ref(false)
const saving = ref(false)
const showSaveDialog = ref(false)
const saveName = ref('')
const saveDesc = ref('')

// Data for panels
const healthScore = ref(null)
const stats = ref(null)
const owaspData = ref([])
const trendData = ref([])
const recentLogs = ref([])
const attackChains = ref([])
const honeypotData = ref(null)
const redteamData = ref(null)
const leaderboardData = ref([])
const abTestData = ref([])
const anomalyData = ref([])

// Computed
const scoreColorMap = { excellent: '#10B981', good: '#3B82F6', warning: '#F59E0B', danger: '#EF4444', critical: '#DC2626' }
const scoreColor = computed(() => healthScore.value ? (scoreColorMap[healthScore.value.level] || '#6B7280') : '#6B7280')
const scoreDash = computed(() => {
  if (!healthScore.value) return '0 214'
  const c = 2 * Math.PI * 34
  const p = healthScore.value.score / 100
  return `${c * p} ${c * (1 - p)}`
})

const trendPoints = computed(() => {
  if (!healthScore.value?.trend?.length) return ''
  const d = healthScore.value.trend
  return d.map((v, i) => `${5 + (i / Math.max(d.length - 1, 1)) * 110},${3 + (1 - v.score / 100) * 24}`).join(' ')
})

const fullTrendPoints = computed(() => {
  if (!trendData.value?.length) return ''
  const d = trendData.value
  const max = Math.max(...d.map(t => (t.pass || 0) + (t.block || 0) + (t.warn || 0)), 1)
  return d.map((t, i) => `${(i / Math.max(d.length - 1, 1)) * 400},${100 - ((t.pass || 0) + (t.block || 0) + (t.warn || 0)) / max * 90}`).join(' ')
})

const fullBlockPoints = computed(() => {
  if (!trendData.value?.length) return ''
  const d = trendData.value
  const max = Math.max(...d.map(t => (t.pass || 0) + (t.block || 0) + (t.warn || 0)), 1)
  return d.map((t, i) => `${(i / Math.max(d.length - 1, 1)) * 400},${100 - (t.block || 0) / max * 90}`).join(' ')
})

function isPanelVisible(id) {
  const p = panels.value.find(p => p.id === id)
  return p ? p.visible !== false : false
}

function togglePanelVisibility(id) {
  const idx = panels.value.findIndex(p => p.id === id)
  if (idx >= 0) {
    panels.value[idx].visible = !panels.value[idx].visible
  } else {
    // Add panel from defs
    const def = allPanelDefs.value.find(d => d.id === id)
    if (def) {
      panels.value.push({ ...def, visible: true, order: panels.value.length })
    }
  }
}

function onPanelsUpdate(newPanels) {
  panels.value = newPanels
}

function onPanelRemove(id) {
  showToast(`已隐藏面板，可通过"添加面板"重新显示`)
}

function onPanelReorder() {
  // reorder handled automatically
}

function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? String(ts) : d.toLocaleTimeString('zh-CN', { hour12: false })
}

// Preset & Layout management
async function loadPresets() {
  try {
    const d = await api('/api/v1/layouts/presets')
    presets.value = d.presets || []
    allPanelDefs.value = d.panels || []
  } catch { presets.value = [] }
}

async function loadSavedLayouts() {
  try {
    const d = await api('/api/v1/layouts')
    savedLayouts.value = d.layouts || []
  } catch { savedLayouts.value = [] }
}

function applyPreset() {
  const p = presets.value.find(p => p.id === selectedPreset.value)
  if (p) {
    panels.value = JSON.parse(JSON.stringify(p.panels))
    showToast(`已应用预设: ${p.name}`)
  }
  selectedPreset.value = ''
}

async function loadSavedLayout() {
  if (!activeLayoutId.value) return
  try {
    const layout = await api(`/api/v1/layouts/${activeLayoutId.value}`)
    panels.value = layout.panels || []
    showToast(`已加载布局: ${layout.name}`)
  } catch {
    showToast('加载布局失败')
  }
}

function saveLayout() {
  showSaveDialog.value = true
  saveName.value = ''
  saveDesc.value = ''
}

async function confirmSave() {
  if (!saveName.value.trim()) return
  saving.value = true
  try {
    const body = {
      name: saveName.value.trim(),
      description: saveDesc.value.trim(),
      panels: panels.value,
    }
    // If we have an active layout, update it
    if (activeLayoutId.value) {
      await apiPut(`/api/v1/layouts/${activeLayoutId.value}`, body)
      showToast('布局已更新')
    } else {
      const result = await apiPost('/api/v1/layouts', body)
      activeLayoutId.value = result.id
      showToast('布局已保存')
    }
    await loadSavedLayouts()
  } catch (e) {
    showToast('保存失败: ' + e.message)
  } finally {
    saving.value = false
    showSaveDialog.value = false
  }
}

function resetLayout() {
  if (!confirm('确定要重置为默认布局吗？')) return
  // Apply SOC preset as default
  const soc = presets.value.find(p => p.id === 'preset-soc')
  if (soc) {
    panels.value = JSON.parse(JSON.stringify(soc.panels))
  }
  activeLayoutId.value = ''
  showToast('已重置为默认布局')
}

// Load panel data
async function loadPanelData() {
  // Health Score
  try { healthScore.value = await api('/api/v1/health/score') } catch { healthScore.value = null }

  // Stats
  try {
    const d = await api('/api/v1/stats?since=24h')
    const total = d.total || 0
    const breakdown = d.breakdown || {}
    let blocked = 0, warned = 0
    for (const k of Object.keys(breakdown)) {
      if (k.indexOf('block') >= 0) blocked += breakdown[k]
      if (k.indexOf('warn') >= 0) warned += breakdown[k]
    }
    const rate = total > 0 ? (blocked / total * 100).toFixed(1) : '0.0'
    stats.value = { total, blocked, warned, rate: rate + '%' }
  } catch { stats.value = null }

  // OWASP
  try { const d = await api('/api/v1/llm/owasp-matrix'); owaspData.value = d.items || [] } catch { owaspData.value = [] }

  // Trend
  try { const d = await api('/api/v1/audit/timeline?hours=24'); trendData.value = d.timeline || [] } catch { trendData.value = [] }

  // Audit logs
  try { const d = await api('/api/v1/audit/logs?action=block&limit=6'); recentLogs.value = d.logs || [] } catch { recentLogs.value = [] }

  // Attack chains
  try { const d = await api('/api/v1/attack-chains?limit=5'); attackChains.value = d.chains || [] } catch { attackChains.value = [] }

  // Honeypot
  try { honeypotData.value = await api('/api/v1/honeypot/stats') } catch { honeypotData.value = null }

  // Red team
  try {
    const d = await api('/api/v1/redteam/reports?limit=1')
    const reports = d.reports || []
    if (reports.length > 0) {
      redteamData.value = { total: reports[0].total_tests, passed: reports[0].passed, failed: reports[0].failed }
    }
  } catch { redteamData.value = null }

  // Leaderboard
  try { const d = await api('/api/v1/leaderboard'); leaderboardData.value = d.entries || [] } catch { leaderboardData.value = [] }

  // A/B tests
  try { const d = await api('/api/v1/ab-tests'); abTestData.value = d.tests || [] } catch { abTestData.value = [] }

  // Anomaly
  try { const d = await api('/api/v1/anomaly/alerts?limit=5'); anomalyData.value = d.alerts || [] } catch { anomalyData.value = [] }
}

onMounted(async () => {
  await loadPresets()
  await loadSavedLayouts()

  // Try to load active layout, else apply SOC preset
  const activeLayout = savedLayouts.value.find(l => l.is_default)
  if (activeLayout) {
    activeLayoutId.value = activeLayout.id
    panels.value = activeLayout.panels || []
  } else {
    const soc = presets.value.find(p => p.id === 'preset-soc')
    if (soc) {
      panels.value = JSON.parse(JSON.stringify(soc.panels))
    }
  }

  loadPanelData()
})
</script>

<style scoped>
.custom-dashboard {
  padding: 0;
}

/* Toolbar */
.cd-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-3, 12px) 0;
  margin-bottom: var(--space-4, 16px);
  border-bottom: 1px solid var(--border-subtle, #2a2a40);
  flex-wrap: wrap;
  gap: var(--space-2, 8px);
}

.cd-toolbar-left,
.cd-toolbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-2, 8px);
  flex-wrap: wrap;
}

.cd-title {
  font-size: var(--text-base, 16px);
  font-weight: 700;
  color: var(--text-primary, #e0e0e0);
  white-space: nowrap;
}

.cd-preset-select,
.cd-layout-select {
  background: var(--bg-elevated, #1e1e38);
  border: 1px solid var(--border-default, #333);
  border-radius: var(--radius-sm, 4px);
  color: var(--text-primary, #e0e0e0);
  font-size: var(--text-xs, 12px);
  padding: 4px 8px;
  cursor: pointer;
}

.cd-btn {
  background: var(--bg-elevated, #1e1e38);
  border: 1px solid var(--border-default, #333);
  border-radius: var(--radius-sm, 4px);
  color: var(--text-primary, #e0e0e0);
  font-size: var(--text-xs, 12px);
  font-weight: 600;
  padding: 4px 12px;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.cd-btn:hover {
  background: var(--bg-surface, #22223b);
}

.cd-btn-save {
  border-color: var(--color-primary, #6366f1);
  color: var(--color-primary, #6366f1);
}

.cd-btn-save:hover {
  background: var(--color-primary, #6366f1);
  color: #fff;
}

.cd-btn-reset {
  border-color: var(--color-warning, #f59e0b);
  color: var(--color-warning, #f59e0b);
}

.cd-btn-add {
  border-color: var(--color-success, #10b981);
  color: var(--color-success, #10b981);
}

.cd-btn-edit-on {
  background: rgba(99, 102, 241, 0.15);
  border-color: #6366f1;
  color: #6366f1;
}

.cd-btn-edit-off {
  border-color: #666;
}

/* Panel Picker */
.cd-panel-picker {
  background: var(--bg-surface, #1a1a2e);
  border: 1px solid var(--border-default, #333);
  border-radius: var(--radius-lg, 12px);
  padding: var(--space-4, 16px);
  margin-bottom: var(--space-4, 16px);
  box-shadow: var(--shadow-lg, 0 8px 24px rgba(0, 0, 0, 0.3));
}

.picker-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--space-3, 12px);
  font-weight: 600;
  color: var(--text-primary, #e0e0e0);
}

.picker-close {
  background: none;
  border: none;
  color: var(--text-tertiary, #666);
  cursor: pointer;
  font-size: 16px;
}

.picker-grid {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2, 8px);
}

.picker-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: var(--radius-sm, 4px);
  border: 1px solid var(--border-subtle, #2a2a40);
  font-size: var(--text-xs, 12px);
  color: var(--text-secondary, #aaa);
  cursor: pointer;
  transition: all 0.2s;
}

.picker-item:hover {
  border-color: var(--color-primary, #6366f1);
}

.picker-item.active {
  background: rgba(99, 102, 241, 0.1);
  border-color: var(--color-primary, #6366f1);
  color: var(--color-primary, #6366f1);
}

/* Picker transition */
.picker-fade-enter-active, .picker-fade-leave-active { transition: all 0.2s ease; }
.picker-fade-enter-from, .picker-fade-leave-to { opacity: 0; transform: translateY(-8px); }

/* Save Dialog */
.cd-save-dialog-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.5); z-index: 1000;
  display: flex; align-items: center; justify-content: center;
}
.cd-save-dialog {
  background: var(--bg-surface, #1a1a2e);
  border: 1px solid var(--border-default, #333);
  border-radius: var(--radius-lg, 12px);
  padding: var(--space-4, 16px); width: 360px; max-width: 90vw;
}
.cd-save-dialog h3 { margin: 0 0 12px; font-size: var(--text-base, 16px); color: var(--text-primary, #e0e0e0); }
.cd-save-input, .cd-save-textarea {
  width: 100%; background: var(--bg-elevated, #1e1e38);
  border: 1px solid var(--border-default, #333); border-radius: var(--radius-sm, 4px);
  color: var(--text-primary, #e0e0e0); font-size: var(--text-sm, 14px);
  padding: 8px; margin-bottom: 8px; box-sizing: border-box;
}
.cd-save-textarea { resize: vertical; font-family: inherit; }
.cd-save-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 8px; }

/* Panel content styles */
.panel-content { min-height: 60px; }
.panel-loading { color: var(--text-tertiary, #666); font-size: var(--text-sm, 14px); text-align: center; padding: 20px 0; }
.panel-empty { color: var(--text-tertiary, #666); font-size: var(--text-xs, 12px); text-align: center; padding: 20px 0; }

/* Mini score */
.mini-score { display: flex; align-items: center; gap: 16px; }
.mini-score-ring { position: relative; width: 80px; height: 80px; flex-shrink: 0; }
.mini-ring-svg { width: 100%; height: 100%; }
.mini-score-num { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); font-size: 1.5rem; font-weight: 800; font-family: var(--font-mono); }
.mini-score-info { flex: 1; }
.mini-score-label { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: 11px; font-weight: 700; color: #fff; }
.badge-excellent { background: #10B981; } .badge-good { background: #3B82F6; } .badge-warning { background: #F59E0B; } .badge-danger { background: #EF4444; } .badge-critical { background: #DC2626; }
.mini-trend-svg { width: 100%; height: 30px; margin-top: 8px; }

/* Mini stats */
.mini-stats { display: flex; gap: 16px; flex-wrap: wrap; }
.mini-stat { display: flex; flex-direction: column; align-items: center; flex: 1; min-width: 60px; }
.mini-stat-val { font-size: 1.25rem; font-weight: 800; font-family: var(--font-mono); color: var(--text-primary, #e0e0e0); }
.mini-stat-label { font-size: 10px; color: var(--text-tertiary, #666); margin-top: 2px; }
.stat-red { color: #EF4444; } .stat-yellow { color: #F59E0B; } .stat-green { color: #10B981; }

/* OWASP mini */
.owasp-mini-grid { display: flex; flex-direction: column; gap: 4px; }
.owasp-mini-item { display: flex; align-items: center; gap: 8px; padding: 4px 0; border-bottom: 1px solid var(--border-subtle, #2a2a40); font-size: var(--text-xs, 12px); }
.owasp-mini-id { font-weight: 700; color: var(--text-tertiary, #666); min-width: 60px; }
.owasp-mini-name { flex: 1; color: var(--text-secondary, #aaa); }
.owasp-mini-count { font-weight: 700; font-family: var(--font-mono); color: var(--text-primary, #e0e0e0); }
.severity-critical .owasp-mini-count { color: #DC2626; }
.severity-high .owasp-mini-count { color: #EF4444; }
.severity-medium .owasp-mini-count { color: #F59E0B; }

/* Trend full */
.trend-placeholder { position: relative; }
.trend-full-svg { width: 100%; height: 100px; }
.trend-legend { display: flex; gap: 12px; justify-content: center; margin-top: 4px; }
.legend-item { display: flex; align-items: center; gap: 4px; font-size: 10px; color: var(--text-tertiary, #666); }
.legend-dot { width: 8px; height: 8px; border-radius: 50%; }

/* Mini log */
.mini-log-list { display: flex; flex-direction: column; gap: 2px; }
.mini-log-item { display: flex; align-items: center; gap: 8px; padding: 3px 0; border-bottom: 1px solid var(--border-subtle, #2a2a40); font-size: var(--text-xs, 12px); }
.mini-log-time { color: var(--text-tertiary, #666); min-width: 60px; font-family: var(--font-mono); }
.mini-log-action { font-weight: 700; min-width: 40px; }
.action-block { color: #EF4444; } .action-warn { color: #F59E0B; } .action-pass { color: #10B981; }
.mini-log-sender { color: var(--text-secondary, #aaa); min-width: 60px; }
.mini-log-reason { color: var(--text-tertiary, #666); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* Mini chains */
.mini-chains { display: flex; flex-direction: column; gap: 4px; }
.mini-chain-item { display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: var(--text-xs, 12px); border-bottom: 1px solid var(--border-subtle, #2a2a40); }
.chain-severity { font-weight: 700; min-width: 50px; text-transform: uppercase; font-size: 10px; }
.sev-critical { color: #DC2626; } .sev-high { color: #EF4444; } .sev-medium { color: #F59E0B; } .sev-low { color: #10B981; }
.chain-name { flex: 1; color: var(--text-secondary, #aaa); }
.chain-steps { color: var(--text-tertiary, #666); font-family: var(--font-mono); }

/* Mini leaderboard */
.mini-lb { display: flex; flex-direction: column; gap: 2px; }
.mini-lb-row { display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: var(--text-xs, 12px); border-bottom: 1px solid var(--border-subtle, #2a2a40); }
.lb-rank { font-weight: 700; color: var(--color-primary, #6366f1); min-width: 24px; }
.lb-name { flex: 1; color: var(--text-secondary, #aaa); }
.lb-score { font-weight: 700; font-family: var(--font-mono); color: var(--text-primary, #e0e0e0); }

/* Mini A/B */
.mini-ab { display: flex; flex-direction: column; gap: 4px; }
.mini-ab-item { display: flex; align-items: center; gap: 8px; font-size: var(--text-xs, 12px); padding: 4px 0; border-bottom: 1px solid var(--border-subtle, #2a2a40); }
.ab-status { font-weight: 700; min-width: 50px; text-transform: uppercase; font-size: 10px; }
.ab-running { color: #10B981; } .ab-draft { color: #F59E0B; } .ab-completed { color: #6366f1; } .ab-stopped { color: #666; }
.ab-name { flex: 1; color: var(--text-secondary, #aaa); }

/* Mini anomaly */
.mini-anomalies { display: flex; flex-direction: column; gap: 2px; }
.mini-anomaly-item { display: flex; align-items: center; gap: 8px; font-size: var(--text-xs, 12px); padding: 3px 0; border-bottom: 1px solid var(--border-subtle, #2a2a40); }
.anomaly-sev { font-weight: 700; min-width: 50px; text-transform: uppercase; font-size: 10px; }
.anomaly-metric { flex: 1; color: var(--text-secondary, #aaa); }
.anomaly-dev { font-weight: 700; font-family: var(--font-mono); color: #EF4444; }

@media (max-width: 768px) {
  .cd-toolbar { flex-direction: column; align-items: flex-start; }
}
</style>