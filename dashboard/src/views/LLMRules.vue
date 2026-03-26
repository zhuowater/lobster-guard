<template>
  <div>
    <!-- Top Stats -->
    <div class="stat-row" style="margin-bottom:20px">
      <div class="stat-card">
        <div class="stat-value">{{ totalRules }}</div>
        <div class="stat-label">总规则数</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--color-success)">{{ enabledRules }}</div>
        <div class="stat-label">启用中</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--color-warning)">{{ shadowRules }}</div>
        <div class="stat-label">影子模式</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--color-primary)">{{ totalHits }}</div>
        <div class="stat-label">总命中数</div>
      </div>
    </div>

    <!-- Search & Filter Bar -->
    <div class="filter-bar card" style="margin-bottom:16px">
      <div class="filter-bar-inner">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="searchText" class="search-input" placeholder="搜索名称、描述、ID 或模式..." />
          <button v-if="searchText" class="search-clear" @click="searchText = ''" title="清除">&times;</button>
        </div>
        <div class="filter-selects">
          <select v-model="filterCategory" class="filter-select">
            <option value="">全部类别</option>
            <option v-for="c in categoryOptions" :key="c.value" :value="c.value">{{ c.label }}</option>
          </select>
          <select v-model="filterAction" class="filter-select">
            <option value="">全部动作</option>
            <option value="log">记录</option>
            <option value="warn">告警</option>
            <option value="block">阻断</option>
            <option value="rewrite">改写</option>
          </select>
          <select v-model="filterDirection" class="filter-select">
            <option value="">全部方向</option>
            <option value="request">请求</option>
            <option value="response">响应</option>
            <option value="both">双向</option>
          </select>
          <select v-model="filterEnabled" class="filter-select">
            <option value="">全部状态</option>
            <option value="enabled">已启用</option>
            <option value="disabled">已禁用</option>
            <option value="shadow">影子模式</option>
          </select>
        </div>
      </div>
      <div v-if="hasActiveFilters" class="active-filters">
        <span class="filter-badge" v-if="searchText">搜索: {{ searchText }} <button class="filter-clear-btn" @click="searchText = ''">&times;</button></span>
        <span class="filter-badge" v-if="filterCategory">类别: {{ catLabel(filterCategory) }} <button class="filter-clear-btn" @click="filterCategory = ''">&times;</button></span>
        <span class="filter-badge" v-if="filterAction">动作: {{ filterAction }} <button class="filter-clear-btn" @click="filterAction = ''">&times;</button></span>
        <span class="filter-badge" v-if="filterDirection">方向: {{ dirLabel(filterDirection) }} <button class="filter-clear-btn" @click="filterDirection = ''">&times;</button></span>
        <span class="filter-badge" v-if="filterEnabled">状态: {{ enabledLabel(filterEnabled) }} <button class="filter-clear-btn" @click="filterEnabled = ''">&times;</button></span>
        <button class="btn btn-ghost btn-sm" @click="clearAllFilters" style="font-size:.75rem">清除全部</button>
      </div>
    </div>

    <!-- Rule List Card -->
    <div class="card">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg></span>
        <span class="card-title">LLM 规则引擎</span>
        <div class="card-actions">
          <button class="btn btn-ghost btn-sm" @click="showTester = !showTester">{{ showTester ? '关闭测试器' : '规则测试器' }}</button>
          <button class="btn btn-ghost btn-sm" @click="loadData">刷新</button>
          <button class="btn btn-sm" @click="openCreate">新建规则</button>
        </div>
      </div>

      <!-- Rule Tester Panel -->
      <div v-if="showTester" class="rule-tester-panel">
        <div class="rule-tester-header">
          <span style="font-weight:600;font-size:.9rem">LLM 规则测试器</span>
          <span style="font-size:.75rem;color:var(--text-tertiary);margin-left:8px">输入文本，前端模拟测试匹配</span>
        </div>
        <div class="tester-input-wrap">
          <select v-model="testerDirection" class="filter-select" style="width:auto;min-width:100px">
            <option value="request">测试请求</option>
            <option value="response">测试响应</option>
          </select>
          <textarea v-model="testerText" class="tester-textarea" rows="3" placeholder="输入要测试的文本..."></textarea>
          <button class="btn btn-sm" @click="runTest" :disabled="!testerText.trim()">运行测试</button>
        </div>
        <div v-if="testerResults !== null" class="tester-results">
          <div v-if="testerResults.length === 0"><span class="tag tag-pass" style="font-size:.8rem">未命中任何规则</span></div>
          <div v-else>
            <div style="margin-bottom:8px"><span class="tag tag-danger" style="font-size:.8rem">命中 {{ testerResults.length }} 条规则</span></div>
            <div class="tester-match-list">
              <div v-for="(m, mi) in testerResults" :key="mi" class="tester-match-item" :class="{ 'tester-shadow': m.shadowMode }">
                <div class="tester-match-meta">
                  <span class="tag" :class="actTag(m.action)" style="font-size:.7rem">{{ m.action }}</span>
                  <span class="tag" :style="catStyle(m.category)" style="font-size:.7rem">{{ catLabel(m.category) }}</span>
                  <span v-if="m.shadowMode" class="tag tag-ghost" style="font-size:.7rem">影子</span>
                  <span style="font-size:.75rem;color:var(--text-tertiary)">优先级: {{ m.priority }}</span>
                </div>
                <div style="font-weight:600;font-size:.85rem">{{ m.ruleName }}</div>
                <div style="font-size:.75rem;color:var(--text-tertiary)">ID: {{ m.ruleId }} · 匹配: <code style="background:rgba(255,68,102,.15);padding:1px 4px;border-radius:3px;color:var(--color-danger)">{{ m.pattern }}</code></div>
                <div v-if="m.matchedText" style="font-size:.75rem;margin-top:2px">命中: <mark class="tester-highlight">{{ m.matchedText }}</mark></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Batch Action Bar -->
      <div v-if="selectedIds.size > 0" class="batch-bar">
        <span class="batch-info">已选择 <b>{{ selectedIds.size }}</b> 条规则</span>
        <div class="batch-actions">
          <button class="btn btn-ghost btn-sm" @click="batchEnable(true)">批量启用</button>
          <button class="btn btn-ghost btn-sm" @click="batchEnable(false)">批量禁用</button>
          <select class="filter-select" v-model="batchActionVal" @change="batchChangeAction">
            <option value="">批量改动作</option>
            <option value="log">记录</option>
            <option value="warn">告警</option>
            <option value="block">阻断</option>
            <option value="rewrite">改写</option>
          </select>
          <button class="btn btn-danger btn-sm" @click="batchDeleteConfirm">批量删除</button>
          <button class="btn btn-ghost btn-sm" @click="selectedIds.clear()">取消</button>
        </div>
      </div>

      <DataTable :columns="columns" :data="rulesWithHits" :loading="loading" empty-text="暂无 LLM 规则" empty-desc="点击右上角新建规则" :expandable="true" :row-class="rowClass">
        <template #cell-_select="{ row }"><input type="checkbox" :checked="selectedIds.has(row.id)" @click.stop="toggleSelect(row.id)" class="row-checkbox" /></template>
        <template #cell-status="{ row }">
          <span v-if="row.shadow_mode" class="status-icon" title="影子模式">👻</span>
          <span v-else-if="row.enabled" class="status-icon" title="启用">🟢</span>
          <span v-else class="status-icon" title="禁用">🔴</span>
        </template>
        <template #cell-name="{ row }">
          <div class="rule-name-cell"><span class="rule-name">{{ row.name }}</span><span v-if="row.shadow_mode" class="shadow-badge">SHADOW</span></div>
        </template>
        <template #cell-category="{ value }"><span class="tag" :style="catStyle(value)">{{ catLabel(value) }}</span></template>
        <template #cell-direction="{ value }"><span class="tag" :style="dirStyle(value)">{{ dirLabel(value) }}</span></template>
        <template #cell-type="{ value }"><span class="tag tag-info">{{ value }}</span></template>
        <template #cell-action="{ value }"><span class="tag" :class="actTag(value)">{{ value }}</span></template>
        <template #cell-priority="{ value }">
          <div class="priority-cell">
            <div class="priority-bar-bg"><div class="priority-bar-fill" :style="{ width: Math.min(value||0,100)+'%', background: priorityColor(value||0) }"></div></div>
            <span class="priority-value">{{ value }}</span>
          </div>
        </template>
        <template #cell-hits="{ row }">
          <span v-if="row.shadow_mode" style="color:var(--text-tertiary);border-bottom:1px dashed var(--text-tertiary)">{{ row._shadow_hits || 0 }}</span>
          <span v-else style="font-weight:700;color:var(--color-primary)">{{ row._hits || 0 }}</span>
        </template>
        <template #expand="{ row }">
          <div style="font-size:.82rem">
            <div><b style="color:var(--color-primary)">ID:</b> {{ row.id }}</div>
            <div v-if="row.description"><b style="color:var(--color-primary)">描述:</b> {{ row.description }}</div>
            <div><b style="color:var(--color-primary)">方向:</b> {{ dirLabel(row.direction) }} | <b style="color:var(--color-primary)">类型:</b> {{ row.type }} | <b style="color:var(--color-primary)">动作:</b> {{ row.action }} | <b style="color:var(--color-primary)">优先级:</b> {{ row.priority }}</div>
            <div v-if="row.rewrite_to"><b style="color:var(--color-primary)">Rewrite:</b> <code>{{ row.rewrite_to }}</code></div>
            <div v-if="row.patterns && row.patterns.length"><b style="color:var(--color-primary)">模式 ({{ row.patterns.length }}):</b>
              <pre style="background:var(--bg-base);padding:8px;border-radius:var(--radius-md);margin-top:4px;font-size:var(--text-xs);overflow-x:auto;color:var(--color-success);border:1px solid var(--border-subtle)">{{ row.patterns.join('\n') }}</pre>
            </div>
          </div>
        </template>
        <template #actions="{ row }">
          <div class="action-btns">
            <button class="btn btn-ghost btn-sm" @click.stop="toggleEnabled(row)" :title="row.enabled?'禁用':'启用'"><span v-if="row.enabled" style="font-size:.8rem">🟢</span><span v-else style="font-size:.8rem">🔴</span></button>
            <button class="btn btn-ghost btn-sm" @click.stop="openEdit(row)" title="编辑"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg></button>
            <button class="btn btn-ghost btn-sm" @click.stop="toggleShadow(row)" :title="row.shadow_mode?'切换为激活':'切换为影子模式'"><span v-if="row.shadow_mode"><Icon name="refresh" :size="14" /></span><span v-else>👻</span></button>
            <button class="btn btn-danger btn-sm" @click.stop="confirmDelete(row)" title="删除"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @click.self="showModal=false">
      <div class="modal-box" style="max-width:640px">
        <div class="modal-header">
          <h3>{{ editMode ? '编辑规则' : '新建规则' }}</h3>
          <button class="modal-close" @click="showModal=false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>ID <span v-if="!editMode" class="form-hint">（留空自动生成）</span></label>
            <input v-model="form.id" :disabled="editMode" class="input" :class="{ 'input-error': formErrors.id }" placeholder="llm-custom-xxx">
            <div v-if="formErrors.id" class="form-error">{{ formErrors.id }}</div>
          </div>
          <div class="form-group">
            <label>名称 <span class="required">*</span></label>
            <input v-model="form.name" class="input" :class="{ 'input-error': formErrors.name }" placeholder="规则名称">
            <div v-if="formErrors.name" class="form-error">{{ formErrors.name }}</div>
          </div>
          <div class="form-group">
            <label>描述</label>
            <input v-model="form.description" class="input" placeholder="规则描述">
          </div>
          <div class="form-row">
            <div class="form-group"><label>类别</label>
              <select v-model="form.category" class="input"><option v-for="c in categoryOptions" :key="c.value" :value="c.value">{{ c.label }}</option></select>
            </div>
            <div class="form-group"><label>方向</label>
              <select v-model="form.direction" class="input"><option value="request">Request</option><option value="response">Response</option><option value="both">Both</option></select>
            </div>
          </div>
          <div class="form-row">
            <div class="form-group"><label>类型</label>
              <select v-model="form.type" class="input"><option value="keyword">Keyword</option><option value="regex">Regex</option></select>
            </div>
            <div class="form-group"><label>动作</label>
              <select v-model="form.action" class="input"><option value="log">记录</option><option value="warn">告警</option><option value="block">阻断</option><option value="rewrite">改写</option></select>
            </div>
          </div>
          <div class="form-group">
            <label>模式列表（每行一个） <span class="required">*</span></label>
            <textarea v-model="patternsText" class="input" :class="{ 'input-error': formErrors.patterns }" rows="5" placeholder="每行一个关键词或正则表达式" style="font-family:var(--font-mono);font-size:var(--text-xs)"></textarea>
            <div v-if="formErrors.patterns" class="form-error">{{ formErrors.patterns }}</div>
            <div v-if="form.type==='regex' && regexValidation.length" class="pattern-validation">
              <div v-for="(err,i) in regexValidation" :key="i" class="pattern-error-item"><span style="color:var(--color-danger)">✗</span> 行 {{ err.line }}: <code>{{ err.pattern }}</code> — {{ err.error }}</div>
            </div>
            <div v-if="form.type==='regex' && patternsText.trim() && !regexValidation.length" class="pattern-valid"><span style="color:var(--color-success)">✓</span> 所有正则语法有效</div>
          </div>
          <div v-if="form.action==='rewrite'" class="form-group">
            <label>Rewrite 目标 <span class="required">*</span></label>
            <input v-model="form.rewrite_to" class="input" :class="{ 'input-error': formErrors.rewrite_to }" placeholder="[REDACTED]">
            <div v-if="formErrors.rewrite_to" class="form-error">{{ formErrors.rewrite_to }}</div>
          </div>
          <div class="form-group">
            <label>优先级 <span class="form-hint">(0-100)</span></label>
            <div class="priority-input-row">
              <input v-model.number="form.priority" type="range" min="0" max="100" class="priority-slider" />
              <input v-model.number="form.priority" type="number" class="input priority-num" min="0" max="100">
            </div>
          </div>
          <div class="form-row" style="gap:20px">
            <label class="toggle-label"><input type="checkbox" v-model="form.enabled"> 启用</label>
            <label class="toggle-label"><input type="checkbox" v-model="form.shadow_mode"> 影子模式</label>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="showModal=false">取消</button>
          <button class="btn" @click="saveRule" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button>
        </div>
      </div>
    </div>

    <ConfirmModal v-if="deleteTarget" :visible="!!deleteTarget" type="danger" title="删除规则" :message="deleteMessage" confirm-text="删除" @confirm="doDelete" @cancel="deleteTarget=null"/>
    <ConfirmModal v-if="showBatchDeleteConfirm" :visible="showBatchDeleteConfirm" type="danger" title="批量删除" :message="batchDeleteMessage" confirm-text="删除" @confirm="doBatchDelete" @cancel="showBatchDeleteConfirm=false"/>
  </div>
</template>

<script setup>
import { ref, computed, reactive, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, apiPost, apiPut } from '../api.js'
import { showToast } from '../stores/app.js'
import Icon from '../components/Icon.vue'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const route = useRoute()
const router = useRouter()

const rules = ref([])
const hits = ref({})
const loading = ref(true)
const showModal = ref(false)
const editMode = ref(false)
const saving = ref(false)
const deleteTarget = ref(null)
const patternsText = ref('')
const showBatchDeleteConfirm = ref(false)
const batchActionVal = ref('')

const searchText = ref('')
const filterCategory = ref('')
const filterAction = ref('')
const filterDirection = ref('')
const filterEnabled = ref('')

const selectedIds = reactive(new Set())

const showTester = ref(false)
const testerText = ref('')
const testerDirection = ref('request')
const testerResults = ref(null)

const categoryOptions = [
  { value: 'prompt_injection', label: 'Prompt Injection' },
  { value: 'pii_leak', label: 'PII Leak' },
  { value: 'sensitive_topic', label: 'Sensitive Topic' },
  { value: 'token_abuse', label: 'Token Abuse' },
  { value: 'jailbreak', label: 'Jailbreak' },
  { value: 'exfiltration', label: 'Exfiltration' },
  { value: 'custom', label: 'Custom' },
]

const defaultForm = () => ({
  id: '', name: '', description: '', category: 'custom', direction: 'request',
  type: 'keyword', patterns: [], action: 'log', rewrite_to: '', priority: 10,
  enabled: true, shadow_mode: false,
})
const form = ref(defaultForm())
const formErrors = reactive({ id: '', name: '', patterns: '', rewrite_to: '' })

const columns = [
  { key: '_select', label: '', width: '36px' },
  { key: 'status', label: '状态', width: '50px' },
  { key: 'name', label: '名称' },
  { key: 'category', label: '类别', width: '140px', sortable: true },
  { key: 'direction', label: '方向', width: '90px' },
  { key: 'type', label: '类型', width: '75px' },
  { key: 'action', label: '动作', width: '75px', sortable: true },
  { key: 'priority', label: '优先级', width: '100px', sortable: true },
  { key: 'hits', label: '命中', width: '65px', sortable: true },
]

const hasActiveFilters = computed(() =>
  !!(searchText.value || filterCategory.value || filterAction.value || filterDirection.value || filterEnabled.value)
)

const filteredRules = computed(() => {
  let list = rules.value
  if (searchText.value) {
    const q = searchText.value.toLowerCase()
    list = list.filter(r =>
      (r.name || '').toLowerCase().includes(q) ||
      (r.description || '').toLowerCase().includes(q) ||
      (r.id || '').toLowerCase().includes(q) ||
      (r.patterns || []).some(p => p.toLowerCase().includes(q))
    )
  }
  if (filterCategory.value) list = list.filter(r => r.category === filterCategory.value)
  if (filterAction.value) list = list.filter(r => r.action === filterAction.value)
  if (filterDirection.value) list = list.filter(r => r.direction === filterDirection.value)
  if (filterEnabled.value === 'enabled') list = list.filter(r => r.enabled && !r.shadow_mode)
  else if (filterEnabled.value === 'disabled') list = list.filter(r => !r.enabled)
  else if (filterEnabled.value === 'shadow') list = list.filter(r => r.shadow_mode)
  return list
})

const rulesWithHits = computed(() =>
  filteredRules.value.map(r => ({
    ...r,
    _hits: hits.value[r.id]?.count || 0,
    _shadow_hits: hits.value[r.id]?.shadow_hits || 0,
  }))
)

const deleteMessage = computed(() => {
  if (!deleteTarget.value) return ''
  return '确定删除规则 "' + (deleteTarget.value.name || deleteTarget.value.id) + '"？此操作不可恢复。'
})
const batchDeleteMessage = computed(() =>
  '确定删除选中的 ' + selectedIds.size + ' 条规则？此操作不可恢复。'
)

const totalRules = computed(() => rules.value.length)
const enabledRules = computed(() => rules.value.filter(r => r.enabled && !r.shadow_mode).length)
const shadowRules = computed(() => rules.value.filter(r => r.shadow_mode).length)
const totalHits = computed(() => {
  let sum = 0
  for (const h of Object.values(hits.value)) sum += (h.count || 0) + (h.shadow_hits || 0)
  return sum
})

const regexValidation = computed(() => {
  if (form.value.type !== 'regex' || !patternsText.value.trim()) return []
  const errors = []
  patternsText.value.split('\n').forEach((line, i) => {
    const t = line.trim()
    if (!t) return
    try { new RegExp(t) }
    catch (e) { errors.push({ line: i + 1, pattern: t, error: e.message }) }
  })
  return errors
})

async function loadData() {
  loading.value = true
  try {
    const [rulesData, hitsData] = await Promise.all([
      api('/api/v1/llm/rules'),
      api('/api/v1/llm/rules/hits'),
    ])
    rules.value = rulesData.rules || []
    hits.value = hitsData.hits || {}
  } catch (e) {
    showToast('加载规则失败: ' + e.message, 'error')
  }
  loading.value = false
}

function catStyle(cat) {
  const colors = {
    prompt_injection: { background: 'var(--color-danger-dim, #3a1a1a)', color: 'var(--color-danger)' },
    pii_leak: { background: '#3a2a1a', color: '#f0a050' },
    sensitive_topic: { background: '#2a1a3a', color: '#c080f0' },
    token_abuse: { background: '#3a3a1a', color: '#e0c040' },
    jailbreak: { background: '#3a1a2a', color: '#f06080' },
    exfiltration: { background: '#1a2a3a', color: '#60a0f0' },
    custom: { background: 'var(--bg-elevated)', color: 'var(--text-secondary)' },
  }
  return colors[cat] || colors.custom
}
function catLabel(cat) {
  const m = { prompt_injection:'Prompt Injection', pii_leak:'PII Leak', sensitive_topic:'Sensitive Topic', token_abuse:'Token Abuse', jailbreak:'Jailbreak', exfiltration:'Exfiltration', custom:'Custom' }
  return m[cat] || cat
}
function dirStyle(dir) {
  return { request: { background:'var(--color-primary-dim,#1a2a3a)', color:'var(--color-primary)' }, response: { background:'#1a3a2a', color:'var(--color-success)' }, both: { background:'#2a1a3a', color:'#c080f0' } }[dir] || {}
}
function dirLabel(dir) { return { request:'请求 →', response:'← 响应', both:'↔ 双向' }[dir] || dir }
function enabledLabel(v) { return { enabled:'已启用', disabled:'已禁用', shadow:'影子模式' }[v] || v }
function actTag(a) { return { log:'tag-secondary', warn:'tag-warning', block:'tag-danger', rewrite:'tag-info' }[a] || 'tag-secondary' }
function priorityColor(v) {
  if (v >= 80) return 'var(--color-danger)'
  if (v >= 50) return 'var(--color-warning)'
  if (v >= 20) return 'var(--color-primary)'
  return 'var(--text-tertiary)'
}
function rowClass(row) {
  if (row.shadow_mode) return 'row-shadow'
  if (!row.enabled) return 'row-disabled'
  return ''
}

function clearAllFilters() {
  searchText.value = ''; filterCategory.value = ''; filterAction.value = ''; filterDirection.value = ''; filterEnabled.value = ''
  router.replace({ path: '/llm-rules' })
}
function toggleSelect(id) { if (selectedIds.has(id)) selectedIds.delete(id); else selectedIds.add(id) }

function openCreate() {
  editMode.value = false; form.value = defaultForm(); patternsText.value = ''; clearFormErrors(); showModal.value = true
}
function openEdit(row) {
  editMode.value = true; form.value = { ...row }; patternsText.value = (row.patterns||[]).join('\n'); clearFormErrors(); showModal.value = true
}
function clearFormErrors() { formErrors.id = ''; formErrors.name = ''; formErrors.patterns = ''; formErrors.rewrite_to = '' }

function validateForm() {
  clearFormErrors()
  let valid = true
  if (!form.value.name.trim()) { formErrors.name = '名称不能为空'; valid = false }
  const patterns = patternsText.value.split('\n').map(s=>s.trim()).filter(Boolean)
  if (!patterns.length) { formErrors.patterns = '至少需要一个模式'; valid = false }
  if (form.value.type === 'regex' && regexValidation.value.length) { formErrors.patterns = '存在无效正则'; valid = false }
  if (form.value.action === 'rewrite' && !form.value.rewrite_to.trim()) { formErrors.rewrite_to = 'Rewrite 必须指定替换目标'; valid = false }
  if (!editMode.value && form.value.id.trim() && rules.value.some(r => r.id === form.value.id.trim())) { formErrors.id = 'ID 已存在'; valid = false }
  return valid
}

async function saveRule() {
  if (!validateForm()) return
  saving.value = true
  const data = { ...form.value }
  data.patterns = patternsText.value.split('\n').map(s=>s.trim()).filter(Boolean)
  try {
    if (editMode.value) {
      await api('/api/v1/llm/rules/' + data.id, { method: 'PUT', body: JSON.stringify(data) })
      showToast('规则已更新: ' + data.name, 'success')
    } else {
      await apiPost('/api/v1/llm/rules', data)
      showToast('规则已创建: ' + data.name, 'success')
    }
    showModal.value = false
    await loadData()
  } catch (e) {
    if (e.message.includes('409')) formErrors.id = 'ID 已存在'
    showToast('保存失败: ' + e.message, 'error')
  }
  saving.value = false
}

function confirmDelete(row) { deleteTarget.value = row }
async function doDelete() {
  if (!deleteTarget.value) return
  const name = deleteTarget.value.name || deleteTarget.value.id
  try {
    await api('/api/v1/llm/rules/' + deleteTarget.value.id, { method: 'DELETE' })
    showToast('规则已删除: ' + name, 'success')
    deleteTarget.value = null
    await loadData()
  } catch (e) { showToast('删除失败: ' + e.message, 'error') }
}

async function toggleEnabled(row) {
  try {
    await apiPut('/api/v1/llm/rules/' + row.id, { enabled: !row.enabled })
    showToast(row.name + (row.enabled ? ' 已禁用' : ' 已启用'), 'success')
    await loadData()
  } catch (e) { showToast('切换失败: ' + e.message, 'error') }
}

async function toggleShadow(row) {
  try {
    await apiPost('/api/v1/llm/rules/' + row.id + '/toggle-shadow', {})
    showToast(row.name + (row.shadow_mode ? ' 已切换为激活' : ' 已切换为影子模式'), 'success')
    await loadData()
  } catch (e) { showToast('切换失败: ' + e.message, 'error') }
}

async function batchEnable(enabled) {
  const ids = [...selectedIds]; let ok = 0
  for (const id of ids) { try { await apiPut('/api/v1/llm/rules/' + id, { enabled }); ok++ } catch(e){} }
  showToast((enabled?'启用':'禁用') + ' ' + ok + ' 条规则', 'success')
  selectedIds.clear(); await loadData()
}

async function batchChangeAction() {
  const action = batchActionVal.value; if (!action) return
  const ids = [...selectedIds]; let ok = 0
  for (const id of ids) { try { await apiPut('/api/v1/llm/rules/' + id, { action }); ok++ } catch(e){} }
  showToast('修改 ' + ok + ' 条规则动作为 ' + action, 'success')
  batchActionVal.value = ''; selectedIds.clear(); await loadData()
}

function batchDeleteConfirm() { showBatchDeleteConfirm.value = true }
async function doBatchDelete() {
  const ids = [...selectedIds]; let ok = 0
  for (const id of ids) { try { await api('/api/v1/llm/rules/' + id, { method: 'DELETE' }); ok++ } catch(e){} }
  showToast('已删除 ' + ok + ' 条规则', 'success')
  showBatchDeleteConfirm.value = false; selectedIds.clear(); await loadData()
}

function runTest() {
  const text = testerText.value; const dir = testerDirection.value; const results = []
  for (const rule of rules.value) {
    if (!rule.enabled && !rule.shadow_mode) continue
    if (rule.direction !== 'both' && rule.direction !== dir) continue
    for (const pattern of (rule.patterns || [])) {
      let matched = false, matchedText = ''
      if (rule.type === 'regex') {
        try { const re = new RegExp(pattern, 'gi'); const m = re.exec(text); if (m) { matched = true; matchedText = m[0] } } catch(e){}
      } else {
        const idx = text.toLowerCase().indexOf(pattern.toLowerCase())
        if (idx !== -1) { matched = true; matchedText = text.substring(idx, idx + pattern.length) }
      }
      if (matched) {
        results.push({ ruleId: rule.id, ruleName: rule.name, category: rule.category, action: rule.action, pattern, matchedText, shadowMode: rule.shadow_mode, priority: rule.priority })
        break
      }
    }
  }
  results.sort((a, b) => (b.priority||0) - (a.priority||0))
  testerResults.value = results
}

onMounted(() => { if (route.query.category) filterCategory.value = route.query.category; loadData() })
watch(() => route.query.category, v => { filterCategory.value = v || '' })
</script>

<style scoped>
.stat-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 16px; text-align: center; }
.stat-value { font-size: 1.8rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: 4px; }
.status-icon { font-size: 1rem; }

/* Filter Bar */
.filter-bar { padding: 12px 16px; }
.filter-bar-inner { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
.search-box { position: relative; flex: 1; min-width: 200px; }
.search-icon { position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: var(--text-tertiary); pointer-events: none; }
.search-input {
  width: 100%; padding: 8px 30px 8px 32px; background: var(--bg-base); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm);
}
.search-input:focus { outline: none; border-color: var(--color-primary); }
.search-clear {
  position: absolute; right: 8px; top: 50%; transform: translateY(-50%);
  background: none; border: none; color: var(--text-tertiary); font-size: 1.1rem;
  cursor: pointer; line-height: 1; padding: 0 4px;
}
.search-clear:hover { color: var(--text-primary); }
.filter-selects { display: flex; gap: 8px; flex-wrap: wrap; }
.filter-select {
  padding: 6px 10px; background: var(--bg-base); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-xs);
  appearance: auto; cursor: pointer;
}
.filter-select:focus { outline: none; border-color: var(--color-primary); }

/* Active Filters */
.active-filters { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin-top: 10px; padding-top: 10px; border-top: 1px solid var(--border-subtle); }
.filter-badge {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 3px 10px; background: rgba(99,102,241,0.15); color: var(--color-primary);
  border-radius: 9999px; font-size: var(--text-xs); font-weight: 600;
}
.filter-clear-btn {
  background: none; border: none; color: var(--color-primary); cursor: pointer;
  font-size: 14px; line-height: 1; padding: 0 2px; font-weight: 700; opacity: 0.7;
}
.filter-clear-btn:hover { opacity: 1; }

/* Batch Bar */
.batch-bar {
  display: flex; align-items: center; justify-content: space-between; gap: 12px;
  padding: 10px 16px; margin: 0 -1px; background: rgba(99,102,241,0.08);
  border-top: 1px solid rgba(99,102,241,0.2); border-bottom: 1px solid rgba(99,102,241,0.2);
}
.batch-info { font-size: var(--text-sm); color: var(--color-primary); }
.batch-actions { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
.batch-action-select { min-width: 110px; }

/* Row styles */
:deep(.row-shadow) { opacity: 0.7; border-left: 3px dashed var(--color-warning) !important; }
:deep(.row-disabled) { opacity: 0.5; }

/* Rule name cell */
.rule-name-cell { display: flex; align-items: center; gap: 6px; }
.rule-name { font-weight: 500; }
.shadow-badge {
  display: inline-block; font-size: .6rem; font-weight: 700; letter-spacing: .05em;
  padding: 1px 5px; border-radius: 3px; color: var(--color-warning);
  background: rgba(234,179,8,0.15); border: 1px dashed var(--color-warning);
}

/* Priority */
.priority-cell { display: flex; align-items: center; gap: 6px; }
.priority-bar-bg {
  flex: 1; height: 4px; background: var(--bg-elevated); border-radius: 2px; overflow: hidden;
}
.priority-bar-fill { height: 100%; border-radius: 2px; transition: width .3s; }
.priority-value { font-size: var(--text-xs); font-weight: 600; color: var(--text-secondary); min-width: 20px; text-align: right; }

/* Action buttons */
.action-btns { display: flex; gap: 2px; align-items: center; }

/* Checkbox */
.row-checkbox { accent-color: var(--color-primary); cursor: pointer; width: 14px; height: 14px; }

/* Rule Tester */
.rule-tester-panel {
  padding: 16px; margin: 0 -1px; background: rgba(0,0,0,.15);
  border-top: 1px solid var(--border-subtle); border-bottom: 1px solid var(--border-subtle);
}
.rule-tester-header { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.tester-row { margin-bottom: 8px; }
.tester-input-wrap { display: flex; gap: 8px; align-items: flex-start; }
.tester-textarea {
  flex: 1; padding: 8px; background: var(--bg-base); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm);
  font-family: var(--font-mono); resize: vertical;
}
.tester-textarea:focus { outline: none; border-color: var(--color-primary); }
.tester-results { margin-top: 12px; }
.tester-no-match { padding: 8px 0; }
.tester-match-header { margin-bottom: 8px; }
.tester-match-list { display: flex; flex-direction: column; gap: 8px; }
.tester-match-item {
  padding: 10px 12px; background: rgba(255,68,102,.05); border: 1px solid rgba(255,68,102,.2);
  border-radius: var(--radius-md);
}
.tester-match-item.tester-shadow {
  background: rgba(234,179,8,.05); border-color: rgba(234,179,8,.2);
  border-style: dashed;
}
.tester-match-meta { display: flex; gap: 6px; align-items: center; margin-bottom: 4px; flex-wrap: wrap; }
.tester-highlight { background: rgba(255,68,102,.25); color: #fff; padding: 1px 3px; border-radius: 2px; }
.tag-ghost { background: rgba(234,179,8,0.15); color: var(--color-warning); }

/* Modal */
.modal-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,.6); z-index: 1000;
  display: flex; align-items: center; justify-content: center;
}
.modal-box {
  background: var(--bg-surface); border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle); width: 90%; box-shadow: 0 20px 60px rgba(0,0,0,.4);
}
.modal-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 16px 20px; border-bottom: 1px solid var(--border-subtle);
}
.modal-header h3 { margin: 0; font-size: var(--text-base); }
.modal-close {
  background: none; border: none; color: var(--text-tertiary); font-size: 1.5rem;
  cursor: pointer; line-height: 1;
}
.modal-body { padding: 20px; max-height: 60vh; overflow-y: auto; }
.modal-footer {
  padding: 12px 20px; border-top: 1px solid var(--border-subtle);
  display: flex; justify-content: flex-end; gap: 8px;
}
.form-group { margin-bottom: 12px; }
.form-group label { display: block; font-size: var(--text-sm); color: var(--text-secondary); margin-bottom: 4px; }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.input {
  width: 100%; padding: 8px 10px; background: var(--bg-base); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm);
}
.input:focus { outline: none; border-color: var(--color-primary); }
.input-error { border-color: var(--color-danger) !important; }
select.input { appearance: auto; }
textarea.input { resize: vertical; }
.toggle-label {
  display: inline-flex; align-items: center; gap: 6px; font-size: var(--text-sm);
  color: var(--text-secondary); cursor: pointer;
}
.required { color: var(--color-danger); font-weight: 700; }
.form-hint { color: var(--text-tertiary); font-size: var(--text-xs); font-weight: 400; }
.form-error { color: var(--color-danger); font-size: var(--text-xs); margin-top: 4px; }

/* Regex validation */
.pattern-validation { margin-top: 6px; }
.pattern-error-item { font-size: var(--text-xs); color: var(--color-danger); margin-bottom: 2px; }
.pattern-error-item code { background: rgba(255,68,102,.1); padding: 1px 4px; border-radius: 2px; }
.pattern-valid { font-size: var(--text-xs); color: var(--color-success); margin-top: 6px; }

/* Priority input */
.priority-input-row { display: flex; gap: 12px; align-items: center; }
.priority-slider { flex: 1; accent-color: var(--color-primary); }
.priority-num { width: 70px !important; flex: none; text-align: center; }

@media (max-width:768px) {
  .stat-row { grid-template-columns: repeat(2,1fr); }
  .form-row { grid-template-columns: 1fr; }
  .filter-bar-inner { flex-direction: column; }
  .search-box { min-width: 100%; }
  .batch-bar { flex-direction: column; align-items: flex-start; }
  .tester-input-wrap { flex-direction: column; }
}
</style>
