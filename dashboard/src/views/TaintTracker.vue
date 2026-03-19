<template>
  <div class="taint-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="biohazard" :size="20" /> 污染追踪</h1>
        <p class="page-subtitle">数据流污染标签传播追踪 + 自动逆转引擎 — 阻止 PII 与凭据泄漏</p>
      </div>
      <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
    </div>

    <!-- 顶部大数字 -->
    <div class="stats-grid">
      <div class="stat-card stat-danger">
        <div class="stat-icon"><Icon name="biohazard" :size="20" /></div>
        <div class="stat-value">{{ stats.active_taints ?? '-' }}</div>
        <div class="stat-label">活跃污染数</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🏷️</div>
        <div class="stat-value">{{ stats.label_count ?? '-' }}</div>
        <div class="stat-label">标签分布</div>
      </div>
      <div class="stat-card stat-success">
        <div class="stat-icon"><Icon name="refresh" :size="20" /></div>
        <div class="stat-value">{{ stats.reversals ?? '-' }}</div>
        <div class="stat-label">逆转数</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon"><Icon name="lock" :size="20" /></div>
        <div class="stat-value">{{ stats.pii_patterns ?? '-' }}</div>
        <div class="stat-label">PII 模式数</div>
      </div>
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'scan' }" @click="activeTab = 'scan'"><Icon name="test" :size="14" /> 实时扫描</button>
      <button class="tab-btn" :class="{ active: activeTab === 'active' }" @click="activeTab = 'active'"><Icon name="biohazard" :size="14" /> 活跃污染 ({{ activeTaints.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'reversal' }" @click="activeTab = 'reversal'"><Icon name="refresh" :size="14" /> 逆转记录 ({{ reversals.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'config' }" @click="activeTab = 'config'"><Icon name="settings" :size="14" /> 配置</button>
    </div>

    <!-- 实时扫描区 -->
    <div v-if="activeTab === 'scan'" class="section">
      <div class="test-panel">
        <h3 class="section-title">实时污染扫描</h3>
        <textarea v-model="scanText" class="test-input" rows="4" placeholder="输入要扫描的文本..."></textarea>
        <button class="btn btn-primary" @click="scanTaint" :disabled="scanning || !scanText.trim()" style="margin-top: var(--space-2)">
          <span v-if="scanning" class="spinner"></span>
          {{ scanning ? '扫描中...' : '扫描' }}
        </button>
      </div>

      <div v-if="scanResult" class="scan-result">
        <div class="result-header">
          <span>扫描结果</span>
          <button class="btn-close" @click="scanResult = null">✕</button>
        </div>
        <div class="taint-status" :class="scanResult.tainted ? 'tainted-yes' : 'tainted-no'">
          {{ scanResult.tainted ? '⚠️ 检测到污染' : '✅ 未检测到污染' }}
        </div>
        <div v-if="scanResult.labels && scanResult.labels.length" class="result-tags">
          <strong>匹配标签：</strong>
          <span v-for="l in scanResult.labels" :key="l" class="taint-badge" :class="'taint-' + normLabel(l)">{{ l }}</span>
        </div>
        <div v-if="scanResult.pii_types && scanResult.pii_types.length" class="result-tags">
          <strong>PII 类型：</strong>
          <span v-for="p in scanResult.pii_types" :key="p" class="pii-badge">{{ p }}</span>
        </div>
      </div>
    </div>

    <!-- 活跃污染列表 -->
    <div v-if="activeTab === 'active'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th></th>
              <th>TraceID</th>
              <th>标签</th>
              <th>来源</th>
              <th>详情</th>
              <th>时间</th>
              <th>传播次数</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="(t, idx) in activeTaints" :key="idx">
              <tr @click="toggleExpand(idx)" class="row-clickable">
                <td class="td-expand">{{ expandedIdx === idx ? '▼' : '▶' }}</td>
                <td class="td-mono">{{ truncate(t.trace_id, 16) }}</td>
                <td>
                  <span v-for="l in (t.labels || [t.label])" :key="l" class="taint-badge" :class="'taint-' + normLabel(l)">{{ l }}</span>
                </td>
                <td>{{ t.source || '-' }}</td>
                <td>{{ truncate(t.detail || t.details, 30) }}</td>
                <td class="td-mono">{{ formatTime(t.timestamp || t.time) }}</td>
                <td class="td-mono">{{ t.propagation_count ?? t.propagations ?? 0 }}</td>
              </tr>
              <!-- 展开: 传播链路 -->
              <tr v-if="expandedIdx === idx && t.chain" class="row-expanded">
                <td colspan="7">
                  <div class="chain-view">
                    <span v-for="(stage, si) in t.chain" :key="si" class="chain-stage">
                      {{ stage }}<span v-if="si < t.chain.length - 1" class="chain-arrow"> → </span>
                    </span>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
        <div v-if="activeTaints.length === 0" class="empty-state">暂无活跃污染</div>
      </div>
    </div>

    <!-- 逆转记录 -->
    <div v-if="activeTab === 'reversal'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>TraceID</th>
              <th>模板</th>
              <th>模式</th>
              <th>原始长度</th>
              <th>逆转后长度</th>
              <th>时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, idx) in reversals" :key="idx">
              <td class="td-mono">{{ truncate(r.trace_id, 16) }}</td>
              <td>{{ r.template || '-' }}</td>
              <td><span class="mode-badge" :class="'mode-' + r.mode">{{ r.mode }}</span></td>
              <td class="td-mono">{{ r.original_length ?? '-' }}</td>
              <td class="td-mono">{{ r.reversed_length ?? '-' }}</td>
              <td class="td-mono">{{ formatTime(r.timestamp || r.time) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="reversals.length === 0" class="empty-state">暂无逆转记录</div>
      </div>
    </div>

    <!-- 配置区 -->
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-columns">
        <!-- 污染追踪配置 -->
        <div class="config-panel">
          <h3 class="section-title">污染追踪配置</h3>
          <div class="config-field">
            <label class="field-label">检测动作</label>
            <select v-model="taintConfig.action" class="field-select">
              <option value="block">拦截 (block)</option>
              <option value="warn">告警 (warn)</option>
              <option value="log">记录 (log)</option>
            </select>
          </div>
          <div class="config-field">
            <label class="field-label">TTL (分钟)</label>
            <input v-model.number="taintConfig.ttl_minutes" class="field-input" type="number" placeholder="60">
          </div>
          <button class="btn btn-primary btn-sm" @click="saveTaintConfig" :disabled="saving" style="margin-top: var(--space-2)">
            {{ saving ? '保存中...' : '保存' }}
          </button>
        </div>

        <!-- 逆转引擎配置 -->
        <div class="config-panel">
          <h3 class="section-title">逆转引擎配置</h3>
          <div class="config-field">
            <label class="field-label">逆转模式</label>
            <select v-model="reversalConfig.mode" class="field-select">
              <option value="soft">soft — 标记脱敏</option>
              <option value="hard">hard — 完全移除</option>
              <option value="stealth">stealth — 静默替换</option>
            </select>
          </div>
          <div class="config-field" v-if="reversalConfig.templates && reversalConfig.templates.length">
            <label class="field-label">模板列表</label>
            <div class="template-list">
              <span v-for="t in reversalConfig.templates" :key="t" class="template-tag">{{ t }}</span>
            </div>
          </div>
          <button class="btn btn-primary btn-sm" @click="saveReversalConfig" :disabled="saving2" style="margin-top: var(--space-2)">
            {{ saving2 ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
      <div v-if="saveMsg" class="save-msg" :class="saveMsgType">{{ saveMsg }}</div>
    </div>

    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import { api, apiPost, apiPut } from '../api.js'

const activeTab = ref('scan')
const stats = ref({})
const activeTaints = ref([])
const reversals = ref([])
const error = ref('')
const scanText = ref('')
const scanning = ref(false)
const scanResult = ref(null)
const expandedIdx = ref(-1)
const saving = ref(false)
const saving2 = ref(false)
const saveMsg = ref('')
const saveMsgType = ref('')

const taintConfig = reactive({ action: 'warn', ttl_minutes: 60 })
const reversalConfig = reactive({ mode: 'soft', templates: [] })

async function loadStats() {
  try {
    const d = await api('/api/v1/taint/config')
    stats.value = { active_taints: d.active_taints, label_count: d.label_count, reversals: d.reversals, pii_patterns: d.pii_patterns }
    if (d.action) taintConfig.action = d.action
    if (d.ttl_minutes) taintConfig.ttl_minutes = d.ttl_minutes
  } catch (e) { error.value = '加载统计失败: ' + e.message }
}

async function loadActive() {
  try { const d = await api('/api/v1/taint/active'); activeTaints.value = d.taints || d || [] } catch (e) { error.value = '加载活跃污染失败: ' + e.message }
}

async function loadReversals() {
  try { const d = await api('/api/v1/reversal/records'); reversals.value = d.records || d || [] } catch (e) { error.value = '加载逆转记录失败: ' + e.message }
}

async function loadReversalConfig() {
  try { const d = await api('/api/v1/reversal/config'); if (d.mode) reversalConfig.mode = d.mode; if (d.templates) reversalConfig.templates = d.templates } catch {}
}

async function scanTaint() {
  scanning.value = true; scanResult.value = null
  try { scanResult.value = await apiPost('/api/v1/taint/scan', { text: scanText.value }) }
  catch (e) { error.value = '扫描失败: ' + e.message }
  finally { scanning.value = false }
}

async function saveTaintConfig() {
  saving.value = true; saveMsg.value = ''
  try { await apiPut('/api/v1/taint/config', { action: taintConfig.action, ttl_minutes: taintConfig.ttl_minutes }); saveMsg.value = '✅ 污染追踪配置已保存'; saveMsgType.value = 'success'; loadStats() }
  catch (e) { saveMsg.value = '❌ 保存失败: ' + e.message; saveMsgType.value = 'error' }
  finally { saving.value = false }
}

async function saveReversalConfig() {
  saving2.value = true; saveMsg.value = ''
  try { await apiPut('/api/v1/reversal/config', { mode: reversalConfig.mode }); saveMsg.value = '✅ 逆转引擎配置已保存'; saveMsgType.value = 'success' }
  catch (e) { saveMsg.value = '❌ 保存失败: ' + e.message; saveMsgType.value = 'error' }
  finally { saving2.value = false }
}

function toggleExpand(idx) { expandedIdx.value = expandedIdx.value === idx ? -1 : idx }

function normLabel(l) {
  const m = { 'PII-TAINTED': 'pii', 'CREDENTIAL-TAINTED': 'credential', 'CONFIDENTIAL': 'confidential', 'INTERNAL-ONLY': 'internal' }
  return m[l] || 'default'
}

function loadAll() { error.value = ''; loadStats(); loadActive(); loadReversals(); loadReversalConfig() }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function formatTime(ts) {
  if (!ts) return '-'
  try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) } catch { return ts }
}
onMounted(loadAll)
</script>

<style scoped>
.taint-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); text-align: center; }
.stat-icon { font-size: 1.5rem; margin-bottom: var(--space-1); }
.stat-value { font-size: 1.75rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }
.stat-danger .stat-value { color: #EF4444; }
.stat-success .stat-value { color: #10B981; }

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

/* Scan Result */
.scan-result { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.result-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-3); font-weight: 700; color: var(--text-primary); }
.btn-close { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 16px; }
.btn-close:hover { color: var(--text-primary); }
.taint-status { font-size: 1.25rem; font-weight: 700; text-align: center; padding: var(--space-3); border-radius: var(--radius-md); margin-bottom: var(--space-3); }
.tainted-yes { color: #EF4444; background: rgba(239,68,68,.1); }
.tainted-no { color: #10B981; background: rgba(16,185,129,.1); }
.result-tags { display: flex; flex-wrap: wrap; gap: var(--space-2); align-items: center; font-size: var(--text-sm); color: var(--text-secondary); margin-bottom: var(--space-2); }

/* Taint badges */
.taint-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.taint-pii { background: rgba(239,68,68,.15); color: #FCA5A5; }
.taint-credential { background: rgba(168,85,247,.15); color: #C4B5FD; }
.taint-confidential { background: rgba(245,158,11,.15); color: #FCD34D; }
.taint-internal { background: rgba(59,130,246,.15); color: #93C5FD; }
.taint-default { background: rgba(255,255,255,.1); color: var(--text-secondary); }
.pii-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; background: rgba(239,68,68,.1); color: #FCA5A5; }

/* Mode badges */
.mode-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.mode-soft { background: rgba(16,185,129,.15); color: #6EE7B7; }
.mode-hard { background: rgba(239,68,68,.15); color: #FCA5A5; }
.mode-stealth { background: rgba(168,85,247,.15); color: #C4B5FD; }

/* Table */
.table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th { text-align: left; padding: 8px 10px; background: var(--bg-elevated); color: var(--text-tertiary); font-weight: 600; font-size: 10px; text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle); white-space: nowrap; }
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.td-expand { width: 20px; cursor: pointer; color: var(--text-tertiary); user-select: none; }
.row-clickable { cursor: pointer; }
.row-expanded td { background: var(--bg-elevated); padding: var(--space-3); }

/* Chain view */
.chain-view { display: flex; flex-wrap: wrap; gap: var(--space-1); align-items: center; font-size: var(--text-xs); }
.chain-stage { display: inline-block; padding: 2px 8px; background: rgba(99,102,241,.15); color: #a5b4fc; border-radius: 4px; font-weight: 600; }
.chain-arrow { color: var(--text-tertiary); margin: 0 2px; }

/* Config */
.config-columns { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
.config-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.config-field { margin-bottom: var(--space-3); display: flex; flex-direction: column; gap: 4px; }
.field-label { font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; }
.field-input { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); }
.field-input:focus { outline: none; border-color: var(--color-primary); }
.field-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); }
.template-list { display: flex; flex-wrap: wrap; gap: var(--space-1); }
.template-tag { display: inline-block; padding: 2px 8px; background: rgba(255,255,255,.08); border-radius: 4px; font-size: 10px; color: var(--text-secondary); }

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
  .config-columns { grid-template-columns: 1fr; }
}
</style>
