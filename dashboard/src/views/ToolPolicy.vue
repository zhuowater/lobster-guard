<template>
  <div class="toolpolicy-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="wrench" :size="20" /> 工具策略引擎</h1>
        <p class="page-subtitle">定义 Agent 可调用工具的策略规则 — 按工具名、参数模式精确控制</p>
      </div>
      <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
    </div>

    <!-- 顶部大数字 -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon"><Icon name="clipboard" :size="20" /></div>
        <div class="stat-value">{{ stats.rule_count ?? '-' }}</div>
        <div class="stat-label">规则数</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon"><Icon name="search" :size="20" /></div>
        <div class="stat-value">{{ stats.total_evaluations ?? '-' }}</div>
        <div class="stat-label">总评估</div>
      </div>
      <div class="stat-card stat-danger">
        <div class="stat-icon">🚫</div>
        <div class="stat-value">{{ stats.blocked ?? '-' }}</div>
        <div class="stat-label">阻断数</div>
      </div>
      <div class="stat-card stat-warn">
        <div class="stat-icon">⚠️</div>
        <div class="stat-value">{{ stats.warned ?? '-' }}</div>
        <div class="stat-label">告警数</div>
      </div>
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'test' }" @click="activeTab = 'test'"><Icon name="test" :size="14" /> 实时测试</button>
      <button class="tab-btn" :class="{ active: activeTab === 'rules' }" @click="activeTab = 'rules'"><Icon name="file-text" :size="14" /> 规则管理 ({{ rules.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'events' }" @click="activeTab = 'events'">📜 事件日志 ({{ events.length }})</button>
    </div>

    <!-- 实时测试区 -->
    <div v-if="activeTab === 'test'" class="section">
      <div class="test-panel">
        <h3 class="section-title">实时工具评估</h3>
        <div class="test-row">
          <div class="test-field">
            <label class="field-label">工具名</label>
            <input v-model="testTool" class="field-input" placeholder="e.g. shell_exec">
          </div>
        </div>
        <div class="test-field" style="margin-top: var(--space-2)">
          <label class="field-label">参数 JSON</label>
          <textarea v-model="testParams" class="test-input" rows="3" placeholder='{"cmd": "rm -rf /"}' ></textarea>
        </div>
        <button class="btn btn-primary" @click="evaluateTool" :disabled="evaluating || !testTool.trim()" style="margin-top: var(--space-2)">
          <span v-if="evaluating" class="spinner"></span>
          {{ evaluating ? '评估中...' : '评估' }}
        </button>
      </div>

      <!-- 评估结果 -->
      <div v-if="evalResult" class="eval-result">
        <div class="result-header">
          <span>评估结果</span>
          <button class="btn-close" @click="evalResult = null">✕</button>
        </div>
        <div class="eval-decision" :class="'decision-' + (evalResult.decision || evalResult.action)">
          {{ (evalResult.decision || evalResult.action || '').toUpperCase() }}
        </div>
        <div v-if="evalResult.matched_rule || evalResult.rule" class="eval-detail">
          <strong>命中规则：</strong>{{ evalResult.matched_rule || evalResult.rule }}
        </div>
        <div v-if="evalResult.risk_level != null" class="eval-detail">
          <strong>风险等级：</strong>{{ evalResult.risk_level }}
        </div>
      </div>
    </div>

    <!-- 规则管理 -->
    <div v-if="activeTab === 'rules'" class="section">
      <div style="margin-bottom: var(--space-3)">
        <button class="btn btn-primary btn-sm" @click="openNewRule">➕ 新建规则</button>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>工具模式</th>
              <th>动作</th>
              <th>优先级</th>
              <th>原因</th>
              <th>启用</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in rules" :key="r.id || r.name">
              <td class="td-mono">{{ r.name }}</td>
              <td class="td-mono">{{ r.tool_pattern || r.pattern }}</td>
              <td><span class="action-badge" :class="'action-' + r.action">{{ r.action }}</span></td>
              <td class="td-mono">{{ r.priority ?? '-' }}</td>
              <td>{{ truncate(r.reason, 40) }}</td>
              <td><span :class="r.enabled !== false ? 'badge-on' : 'badge-off'">{{ r.enabled !== false ? '启用' : '禁用' }}</span></td>
              <td class="td-actions">
                <button class="btn-icon" @click="editRule(r)" title="编辑">✏️</button>
                <button class="btn-icon" @click="deleteRule(r)" title="删除">🗑️</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="rules.length === 0" class="empty-state">暂无规则</div>
      </div>
    </div>

    <!-- 事件日志 -->
    <div v-if="activeTab === 'events'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>工具名</th>
              <th>决策</th>
              <th>风险等级</th>
              <th>规则命中</th>
              <th>TraceID</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(ev, idx) in events" :key="idx">
              <td class="td-mono">{{ formatTime(ev.timestamp || ev.time) }}</td>
              <td class="td-mono">{{ ev.tool_name || ev.tool }}</td>
              <td><span class="action-badge" :class="'action-' + (ev.decision || ev.action)">{{ ev.decision || ev.action }}</span></td>
              <td class="td-mono">{{ ev.risk_level ?? '-' }}</td>
              <td>{{ ev.matched_rule || ev.rule || '-' }}</td>
              <td class="td-mono td-trace">{{ truncate(ev.trace_id, 16) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="events.length === 0" class="empty-state">暂无事件</div>
      </div>
    </div>

    <!-- 新建/编辑规则对话框 -->
    <div v-if="showDialog" class="dialog-overlay" @click.self="showDialog = false">
      <div class="dialog">
        <div class="dialog-header">{{ editingRule ? '编辑规则' : '新建规则' }}</div>
        <div class="dialog-body">
          <div class="config-field">
            <label class="field-label">名称</label>
            <input v-model="form.name" class="field-input" placeholder="规则名称">
          </div>
          <div class="config-field">
            <label class="field-label">工具模式</label>
            <input v-model="form.tool_pattern" class="field-input" placeholder="e.g. shell_*, file_write">
          </div>
          <div class="config-field">
            <label class="field-label">动作</label>
            <select v-model="form.action" class="field-select">
              <option value="block">block</option>
              <option value="warn">warn</option>
              <option value="allow">allow</option>
            </select>
          </div>
          <div class="config-field">
            <label class="field-label">优先级</label>
            <input v-model.number="form.priority" class="field-input" type="number" placeholder="0">
          </div>
          <div class="config-field">
            <label class="field-label">原因</label>
            <input v-model="form.reason" class="field-input" placeholder="规则原因说明">
          </div>
        </div>
        <div class="dialog-footer">
          <button class="btn btn-sm" @click="showDialog = false">取消</button>
          <button class="btn btn-primary btn-sm" @click="saveRule" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button>
        </div>
      </div>
    </div>

    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'

const activeTab = ref('test')
const stats = ref({})
const rules = ref([])
const events = ref([])
const error = ref('')
const testTool = ref('')
const testParams = ref('')
const evaluating = ref(false)
const evalResult = ref(null)
const showDialog = ref(false)
const editingRule = ref(null)
const saving = ref(false)
const form = reactive({ name: '', tool_pattern: '', action: 'block', priority: 0, reason: '' })

async function loadStats() {
  try {
    const d = await api('/api/v1/tools/rules')
    const r = d.rules || d || []
    rules.value = r
    stats.value = { rule_count: r.length, total_evaluations: d.total_evaluations, blocked: d.blocked, warned: d.warned }
  } catch (e) { error.value = '加载规则失败: ' + e.message }
}

async function loadEvents() {
  try { const d = await api('/api/v1/tools/events?limit=50'); events.value = d.events || d || [] } catch (e) { error.value = '加载事件失败: ' + e.message }
}

async function evaluateTool() {
  evaluating.value = true; evalResult.value = null
  try {
    let params = {}
    if (testParams.value.trim()) { try { params = JSON.parse(testParams.value) } catch { params = { raw: testParams.value } } }
    evalResult.value = await apiPost('/api/v1/tools/evaluate', { tool_name: testTool.value, parameters: params })
  } catch (e) { error.value = '评估失败: ' + e.message }
  finally { evaluating.value = false }
}

function openNewRule() {
  editingRule.value = null
  Object.assign(form, { name: '', tool_pattern: '', action: 'block', priority: 0, reason: '' })
  showDialog.value = true
}

function editRule(r) {
  editingRule.value = r
  Object.assign(form, { name: r.name, tool_pattern: r.tool_pattern || r.pattern, action: r.action, priority: r.priority || 0, reason: r.reason || '' })
  showDialog.value = true
}

async function saveRule() {
  saving.value = true
  try {
    const body = { name: form.name, tool_pattern: form.tool_pattern, action: form.action, priority: form.priority, reason: form.reason }
    if (editingRule.value) {
      await apiPut('/api/v1/tools/rules/' + (editingRule.value.id || editingRule.value.name), body)
    } else {
      await apiPost('/api/v1/tools/rules', body)
    }
    showDialog.value = false; loadStats()
  } catch (e) { error.value = '保存失败: ' + e.message }
  finally { saving.value = false }
}

async function deleteRule(r) {
  if (!confirm('确定删除规则 "' + r.name + '" 吗？')) return
  try { await apiDelete('/api/v1/tools/rules/' + (r.id || r.name)); loadStats() } catch (e) { error.value = '删除失败: ' + e.message }
}

function loadAll() { error.value = ''; loadStats(); loadEvents() }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function formatTime(ts) {
  if (!ts) return '-'
  try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) } catch { return ts }
}
onMounted(loadAll)
</script>

<style scoped>
.toolpolicy-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); text-align: center; }
.stat-icon { font-size: 1.5rem; margin-bottom: var(--space-1); }
.stat-value { font-size: 1.75rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }
.stat-danger .stat-value { color: #EF4444; }
.stat-warn .stat-value { color: #F59E0B; }

.tab-bar { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }

.section { margin-bottom: var(--space-4); }
.section-title { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); }

/* Test Panel */
.test-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); margin-bottom: var(--space-3); }
.test-row { display: flex; gap: var(--space-3); flex-wrap: wrap; }
.test-field { flex: 1; min-width: 200px; }
.test-input {
  width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md);
  color: var(--text-primary); padding: var(--space-3); font-size: var(--text-sm); resize: vertical; font-family: var(--font-mono);
}
.test-input:focus { outline: none; border-color: var(--color-primary); }

/* Eval Result */
.eval-result { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.result-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-3); font-weight: 700; color: var(--text-primary); }
.btn-close { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 16px; }
.btn-close:hover { color: var(--text-primary); }
.eval-decision { font-size: 2rem; font-weight: 800; text-align: center; padding: var(--space-3); border-radius: var(--radius-md); margin-bottom: var(--space-2); }
.decision-block { color: #EF4444; background: rgba(239,68,68,.1); }
.decision-warn { color: #F59E0B; background: rgba(245,158,11,.1); }
.decision-allow { color: #10B981; background: rgba(16,185,129,.1); }
.eval-detail { font-size: var(--text-sm); color: var(--text-secondary); margin-bottom: var(--space-1); }

/* Action badges */
.action-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.action-block { background: rgba(239,68,68,.15); color: #FCA5A5; }
.action-warn { background: rgba(245,158,11,.15); color: #FCD34D; }
.action-allow { background: rgba(16,185,129,.15); color: #6EE7B7; }

.badge-on { color: #10B981; font-weight: 600; font-size: 11px; }
.badge-off { color: var(--text-tertiary); font-size: 11px; }

/* Table */
.table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th { text-align: left; padding: 8px 10px; background: var(--bg-elevated); color: var(--text-tertiary); font-weight: 600; font-size: 10px; text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle); white-space: nowrap; }
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.td-trace { max-width: 150px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.td-actions { display: flex; gap: 4px; }
.btn-icon { background: none; border: none; cursor: pointer; font-size: 14px; padding: 2px 4px; border-radius: 4px; transition: background .2s; }
.btn-icon:hover { background: var(--bg-elevated); }

/* Dialog */
.dialog-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.dialog { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: 0; width: 420px; max-width: 90vw; box-shadow: var(--shadow-lg); }
.dialog-header { padding: var(--space-4); border-bottom: 1px solid var(--border-subtle); font-weight: 700; color: var(--text-primary); font-size: var(--text-base); }
.dialog-body { padding: var(--space-4); display: flex; flex-direction: column; gap: var(--space-3); }
.dialog-footer { padding: var(--space-3) var(--space-4); border-top: 1px solid var(--border-subtle); display: flex; justify-content: flex-end; gap: var(--space-2); }

.config-field { display: flex; flex-direction: column; gap: 4px; }
.field-label { font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; }
.field-input { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); }
.field-input:focus { outline: none; border-color: var(--color-primary); }
.field-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); }

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
}
</style>
