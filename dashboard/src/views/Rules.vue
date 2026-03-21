<template>
  <div>
    <!-- Rule Hits -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg></span><span class="card-title">规则命中率</span>
        <div class="card-actions">
          <button class="btn btn-ghost btn-sm" @click="loadRuleHits">刷新</button>
          <button class="btn btn-danger btn-sm" @click="confirmResetHits">重置</button>
        </div>
      </div>
      <DataTable :columns="hitsColumns" :data="ruleHits" :loading="hitsLoading" empty-text="规则正在保护中" empty-desc="命中数据将在检测到威胁后显示" :expandable="false">
        <template #cell-hits="{ value }"><span style="font-weight:700;color:var(--color-primary)">{{ value }}</span></template>
        <template #cell-action="{ value }"><span class="tag" :class="actTag(value)">{{ value }}</span></template>
        <template #cell-direction="{ value }">{{ value === 'inbound' ? '入站' : '出站' }}</template>
        <template #cell-last_hit="{ value }">{{ fmtTime(value) }}</template>
      </DataTable>
    </div>

    <!-- Rules Management -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg></span><span class="card-title">规则管理</span>
        <div class="card-actions">
          <button class="btn btn-sm" @click="openCreateEditor">新建规则</button>
          <button class="btn btn-ghost btn-sm" @click="exportRules" title="导出规则为 YAML">导出</button>
          <button class="btn btn-ghost btn-sm" @click="showImport = true" title="从 YAML 导入规则">导入</button>
        </div>
      </div>
      <div class="tab-header">
        <button class="tab-btn" :class="{ active: activeTab === 'inbound' }" @click="activeTab = 'inbound'">入站规则<span class="tab-badge" v-if="inboundRules.length">{{ filteredInboundRules.length }}</span></button>
        <button class="tab-btn" :class="{ active: activeTab === 'outbound' }" @click="activeTab = 'outbound'">出站规则<span class="tab-badge" v-if="outboundRules.length">{{ filteredOutboundRules.length }}</span></button>
      </div>

      <!-- ========== Inbound Tab ========== -->
      <div v-show="activeTab === 'inbound'">
        <div class="filter-bar">
          <div class="filter-search">
            <Icon name="search" :size="14" color="var(--text-tertiary)" />
            <input type="text" v-model="inboundSearch" placeholder="搜索规则名称或模式..." class="filter-input" />
            <button v-if="inboundSearch" class="filter-clear" @click="inboundSearch = ''">✕</button>
          </div>
          <div class="filter-group">
            <select v-model="inboundActionFilter" class="filter-select"><option value="">所有动作</option><option value="block">block</option><option value="warn">warn</option><option value="log">log</option></select>
            <select v-model="inboundGroupFilter" class="filter-select"><option value="">所有分组</option><option v-for="g in inboundGroups" :key="g" :value="g">{{ g }}</option></select>
          </div>
        </div>
        <div class="batch-bar" v-if="selectedInbound.length > 0">
          <span class="batch-info">已选 <b>{{ selectedInbound.length }}</b> 条</span>
          <button class="btn btn-ghost btn-sm" @click="batchActionInbound('block')">批量拦截</button>
          <button class="btn btn-ghost btn-sm" @click="batchActionInbound('warn')">批量警告</button>
          <button class="btn btn-ghost btn-sm" @click="batchActionInbound('log')">批量记录</button>
          <button class="btn btn-danger btn-sm" @click="confirmBatchDeleteInbound">批量删除</button>
          <button class="btn btn-ghost btn-sm" @click="selectedInbound = []">取消</button>
        </div>
        <DataTable :columns="inboundColumns" :data="filteredInboundRules" :loading="inboundLoading" empty-text="暂无入站规则" empty-desc="点击「新建规则」创建第一条入站规则" :expandable="true" :row-class="inboundRowClass">
          <template #cell-select="{ row }"><input type="checkbox" class="rule-checkbox" :checked="selectedInbound.includes(row.name)" @click.stop="toggleSelectInbound(row.name)" /></template>
          <template #cell-name="{ row }"><span class="rule-name" :class="{ 'high-priority': (row.priority || 0) >= 80 }">{{ row.name }}</span><span v-if="(row.priority || 0) >= 80" class="priority-badge" title="高优先级">🔥</span></template>
          <template #cell-action="{ value }"><span class="tag" :class="actTag(value)">{{ value }}</span></template>
          <template #cell-type="{ value }"><span class="tag tag-info">{{ value || 'keyword' }}</span></template>
          <template #cell-priority="{ row }"><span class="priority-num" :class="priorityClass(row.priority)">{{ row.priority ?? '--' }}</span></template>
          <template #cell-group="{ value }"><span v-if="value" class="tag" :style="{ background: groupColor(value), color: '#fff' }">{{ value }}</span><span v-else class="text-muted">--</span></template>
          <template #expand="{ row }">
            <div class="rule-expand-detail">
              <div class="expand-row"><b>名称:</b> {{ row.name }}</div>
              <div class="expand-row"><b>类型:</b> {{ row.type || 'keyword' }} | <b>动作:</b> <span class="tag" :class="actTag(row.action)" style="font-size:.72rem">{{ row.action }}</span> | <b>优先级:</b> {{ row.priority ?? '--' }}</div>
              <div class="expand-row" v-if="row.message"><b>自定义消息:</b> {{ row.message }}</div>
              <div v-if="row.patterns && row.patterns.length" class="expand-row"><b>模式 ({{ row.patterns.length }}):</b><pre class="pattern-pre">{{ row.patterns.join('\n') }}</pre></div>
            </div>
          </template>
          <template #actions="{ row }">
            <button class="btn btn-ghost btn-sm" @click.stop="openEditEditor(row, 'inbound')" title="编辑"><Icon name="edit" :size="12" /></button>
            <button class="btn btn-danger btn-sm" @click.stop="confirmDeleteRule(row, 'inbound')" style="margin-left:4px" title="删除"><Icon name="trash" :size="12" /></button>
          </template>
        </DataTable>
        <div class="rule-meta" v-if="inboundMeta">版本: {{ inboundMeta.version }} 来源: {{ inboundMeta.source }} 加载: {{ fmtTime(inboundMeta.loaded_at) }}</div>
        <div style="margin-top:12px"><button class="btn btn-sm" @click="reloadInbound" :disabled="reloadingInbound">{{ reloadingInbound ? '更新中...' : '热更新入站规则' }}</button></div>
      </div>

      <!-- ========== Outbound Tab ========== -->
      <div v-show="activeTab === 'outbound'">
        <div class="filter-bar">
          <div class="filter-search">
            <Icon name="search" :size="14" color="var(--text-tertiary)" />
            <input type="text" v-model="outboundSearch" placeholder="搜索规则名称或模式..." class="filter-input" />
            <button v-if="outboundSearch" class="filter-clear" @click="outboundSearch = ''">✕</button>
          </div>
          <div class="filter-group">
            <select v-model="outboundActionFilter" class="filter-select"><option value="">所有动作</option><option value="block">block</option><option value="warn">warn</option><option value="log">log</option></select>
          </div>
        </div>
        <div class="batch-bar" v-if="selectedOutbound.length > 0">
          <span class="batch-info">已选 <b>{{ selectedOutbound.length }}</b> 条</span>
          <button class="btn btn-danger btn-sm" @click="confirmBatchDeleteOutbound">批量删除</button>
          <button class="btn btn-ghost btn-sm" @click="selectedOutbound = []">取消</button>
        </div>
        <DataTable :columns="outboundColumns" :data="filteredOutboundRules" :loading="outboundLoading" empty-text="暂无出站规则" empty-desc="点击「新建规则」创建出站规则" :expandable="true">
          <template #cell-select="{ row }"><input type="checkbox" class="rule-checkbox" :checked="selectedOutbound.includes(row.name)" @click.stop="toggleSelectOutbound(row.name)" /></template>
          <template #cell-name="{ row }"><span class="rule-name" :class="{ 'high-priority': (row.priority || 0) >= 80 }">{{ row.name }}</span><span v-if="(row.priority || 0) >= 80" class="priority-badge" title="高优先级">🔥</span></template>
          <template #cell-action="{ value }"><span class="tag" :class="actTag(value)">{{ value }}</span></template>
          <template #cell-priority="{ row }"><span class="priority-num" :class="priorityClass(row.priority)">{{ row.priority ?? '--' }}</span></template>
          <template #expand="{ row }">
            <div class="rule-expand-detail">
              <div class="expand-row"><b>名称:</b> {{ row.name }}</div>
              <div class="expand-row"><b>动作:</b> <span class="tag" :class="actTag(row.action)" style="font-size:.72rem">{{ row.action }}</span> | <b>优先级:</b> {{ row.priority ?? 0 }}</div>
              <div class="expand-row" v-if="row.message"><b>自定义消息:</b> {{ row.message }}</div>
              <div v-if="row.patterns && row.patterns.length" class="expand-row"><b>模式 ({{ row.patterns.length }}):</b><pre class="pattern-pre">{{ row.patterns.join('\n') }}</pre></div>
            </div>
          </template>
          <template #actions="{ row }">
            <button class="btn btn-ghost btn-sm" @click.stop="openEditEditor(row, 'outbound')" title="编辑"><Icon name="edit" :size="12" /></button>
            <button class="btn btn-danger btn-sm" @click.stop="confirmDeleteRule(row, 'outbound')" style="margin-left:4px" title="删除"><Icon name="trash" :size="12" /></button>
          </template>
        </DataTable>
        <div style="margin-top:12px"><button class="btn btn-sm" @click="reloadOutbound" :disabled="reloadingOutbound">{{ reloadingOutbound ? '更新中...' : '热更新出站规则' }}</button></div>
      </div>
    </div>

    <!-- Regex Tester -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg></span><span class="card-title">正则表达式测试器</span>
      </div>
      <div style="padding:0 16px 16px"><RegexTester /></div>
    </div>

    <!-- Rule Templates -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></svg></span><span class="card-title">规则模板</span>
        <div class="card-actions"><button class="btn btn-ghost btn-sm" @click="loadTemplates">刷新</button></div>
      </div>
      <div v-if="templatesLoading" style="padding:16px;text-align:center;color:var(--text-secondary)">加载中...</div>
      <div v-else-if="templates.length === 0" style="padding:16px;text-align:center;color:var(--text-secondary)">暂无规则模板</div>
      <div v-else class="template-list">
        <div v-for="tmpl in templates" :key="tmpl.name" class="template-card">
          <div class="template-header" @click="toggleTemplate(tmpl.name)">
            <div class="template-info"><span class="template-icon">{{ templateIcon(tmpl.name) }}</span><div><div class="template-name">{{ templateDisplayName(tmpl.name) }}</div><div class="template-desc">{{ templateDescription(tmpl.name) }}</div></div></div>
            <div class="template-stats"><span class="tag tag-info">{{ tmpl.rule_count }} 条规则</span><span v-if="tmpl.groups && tmpl.groups.length" class="template-groups">{{ tmpl.groups.length }} 个分组</span><span class="expand-arrow" :class="{ expanded: expandedTemplate === tmpl.name }">▶</span></div>
          </div>
          <div v-if="expandedTemplate === tmpl.name" class="template-detail">
            <div v-if="templateDetailLoading" style="padding:12px;text-align:center;color:var(--text-secondary)">加载中...</div>
            <div v-else-if="templateRules.length === 0" style="padding:12px;text-align:center;color:var(--text-secondary)">无规则</div>
            <table v-else class="mini-table">
              <thead><tr><th>名称</th><th>类型</th><th>动作</th><th>优先级</th><th>分组</th><th>模式数</th></tr></thead>
              <tbody><tr v-for="rule in templateRules" :key="rule.name"><td>{{ rule.name }}</td><td><span class="tag tag-info" style="font-size:.7rem">{{ rule.type || 'keyword' }}</span></td><td><span class="tag" :class="actTag(rule.action)" style="font-size:.7rem">{{ rule.action }}</span></td><td>{{ rule.priority }}</td><td><span v-if="rule.group" class="tag" :style="{ background: groupColor(rule.group), color: '#fff', fontSize: '.7rem' }">{{ rule.group }}</span><span v-else>--</span></td><td>{{ (rule.patterns || []).length }}</td></tr></tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <!-- Import Modal -->
    <Teleport to="body">
      <div v-if="showImport" class="modal-overlay" @click.self="showImport = false">
        <div class="import-panel">
          <div class="import-header"><Icon name="upload" :size="16" /><span style="font-weight:600;flex:1">导入规则 (YAML)</span><button class="editor-close" @click="showImport = false">✕</button></div>
          <div class="import-body">
            <div style="margin-bottom:12px"><input type="file" accept=".yaml,.yml" @change="handleFileUpload" ref="fileInput" style="display:none" /><button class="btn btn-sm" @click="$refs.fileInput.click()"><Icon name="file-text" :size="14" /> 选择 YAML 文件</button><span v-if="importFileName" style="margin-left:8px;font-size:.82rem;color:var(--text-secondary)">{{ importFileName }}</span></div>
            <textarea v-model="importYaml" rows="10" placeholder="或直接粘贴 YAML 内容..." class="import-textarea"></textarea>
            <div v-if="importPreview" class="import-preview">
              <div><b>预览:</b> {{ importPreview.total }} 条规则</div>
              <div v-if="importPreview.new_count > 0" style="color:var(--color-success)">新增: {{ importPreview.new_count }} 条 ({{ importPreview.new_rules?.join(', ') }})</div>
              <div v-if="importPreview.override_count > 0" style="color:var(--color-warning)">覆盖: {{ importPreview.override_count }} 条 ({{ importPreview.override_rules?.join(', ') }})</div>
            </div>
          </div>
          <div class="import-footer">
            <button class="btn btn-sm" @click="previewImport" :disabled="!importYaml.trim() || importLoading">{{ importLoading ? '处理中...' : '预览' }}</button>
            <button class="btn btn-sm btn-green" @click="doImport" :disabled="!importYaml.trim() || importLoading">{{ importLoading ? '导入中...' : '确认导入' }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Rule Editor -->
    <RuleEditor :visible="editorVisible" :rule="editingRule" :direction="editingDirection" :errors="editorErrors" @close="closeEditor" @save="saveRule" />
    <!-- Confirm modal -->
    <ConfirmModal :visible="confirmVisible" :title="confirmTitle" :message="confirmMessage" :type="confirmType" @confirm="doConfirm" @cancel="confirmVisible = false" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { api, apiPost, apiPut, apiDelete, downloadFile } from '../api.js'
import { showToast } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Icon from '../components/Icon.vue'
import RuleEditor from '../components/RuleEditor.vue'
import RegexTester from '../components/RegexTester.vue'

const activeTab = ref('inbound')

// ==================== Rule Hits ====================
const ruleHits = ref([])
const hitsLoading = ref(false)
const hitsColumns = [
  { key: 'name', label: '规则', sortable: true },
  { key: 'hits', label: '命中次数', sortable: true },
  { key: 'action', label: '动作', sortable: true },
  { key: 'direction', label: '方向', sortable: true },
  { key: 'last_hit', label: '最后命中', sortable: true },
]

// ==================== Inbound Rules ====================
const inboundRules = ref([])
const inboundLoading = ref(false)
const inboundMeta = ref(null)
const reloadingInbound = ref(false)
const inboundSearch = ref('')
const inboundActionFilter = ref('')
const inboundGroupFilter = ref('')
const selectedInbound = ref([])

const inboundColumns = [
  { key: 'select', label: '', sortable: false, width: '36px' },
  { key: 'name', label: '名称', sortable: true },
  { key: 'action', label: '动作', sortable: true },
  { key: 'type', label: '类型', sortable: true },
  { key: 'priority', label: '优先级', sortable: true },
  { key: 'patterns_count', label: '模式数', sortable: true },
  { key: 'group', label: '分组', sortable: true },
]

const inboundGroups = computed(() => {
  const groups = new Set()
  inboundRules.value.forEach(r => { if (r.group) groups.add(r.group) })
  return [...groups].sort()
})

const filteredInboundRules = computed(() => {
  let list = inboundRules.value
  const q = inboundSearch.value.trim().toLowerCase()
  if (q) {
    list = list.filter(r =>
      (r.name || '').toLowerCase().includes(q) ||
      (r.patterns || []).some(p => p.toLowerCase().includes(q))
    )
  }
  if (inboundActionFilter.value) list = list.filter(r => r.action === inboundActionFilter.value)
  if (inboundGroupFilter.value) list = list.filter(r => r.group === inboundGroupFilter.value)
  return list
})

// ==================== Outbound Rules ====================
const outboundRules = ref([])
const outboundLoading = ref(false)
const reloadingOutbound = ref(false)
const outboundSearch = ref('')
const outboundActionFilter = ref('')
const selectedOutbound = ref([])

const outboundColumns = [
  { key: 'select', label: '', sortable: false, width: '36px' },
  { key: 'name', label: '名称', sortable: true },
  { key: 'action', label: '动作', sortable: true },
  { key: 'priority', label: '优先级', sortable: true },
  { key: 'patterns_count', label: '模式数', sortable: true },
]

const filteredOutboundRules = computed(() => {
  let list = outboundRules.value
  const q = outboundSearch.value.trim().toLowerCase()
  if (q) {
    list = list.filter(r =>
      (r.name || '').toLowerCase().includes(q) ||
      (r.patterns || []).some(p => p.toLowerCase().includes(q))
    )
  }
  if (outboundActionFilter.value) list = list.filter(r => r.action === outboundActionFilter.value)
  return list
})

// ==================== Templates ====================
const templates = ref([])
const templatesLoading = ref(false)
const expandedTemplate = ref(null)
const templateRules = ref([])
const templateDetailLoading = ref(false)

// ==================== Editor ====================
const editorVisible = ref(false)
const editingRule = ref(null)
const editingDirection = ref('inbound')
const editorErrors = ref({})

// ==================== Import ====================
const showImport = ref(false)
const importYaml = ref('')
const importFileName = ref('')
const importPreview = ref(null)
const importLoading = ref(false)

// ==================== Confirm ====================
const confirmVisible = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmType = ref('warning')
let confirmAction = null

// ==================== Helpers ====================
const groupColors = { jailbreak: '#ff6b6b', injection: '#ffa94d', social_engineering: '#69db7c', pii: '#74c0fc', sensitive: '#b197fc', roleplay: '#e599f7', command_injection: '#ff8787', evasion: '#845ef7', data_exfil: '#f06595' }
function groupColor(g) { return groupColors[g] || '#868e96' }
function actTag(a) { a = (a || '').toLowerCase(); return a === 'block' ? 'tag-block' : a === 'warn' ? 'tag-warn' : a === 'log' ? 'tag-log' : 'tag-pass' }
function fmtTime(ts) { if (!ts) return '--'; const d = new Date(ts); return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false }) }

function priorityClass(p) {
  if (p == null) return ''
  if (p >= 80) return 'priority-high'
  if (p >= 40) return 'priority-med'
  return 'priority-low'
}

function inboundRowClass(row) {
  return (row.priority || 0) >= 80 ? 'row-high-priority' : ''
}

const templateMeta = {
  general: { icon: '🌐', name: '通用模板', desc: '越狱、注入、社会工程、敏感信息等基础规则' },
  financial: { icon: '🏦', name: '金融模板', desc: '交易注入、账户泄露、金融欺诈、合规违规等' },
  medical: { icon: '🏥', name: '医疗模板', desc: '病历泄露、处方操纵、患者隐私等' },
  government: { icon: '🏛️', name: '政务模板', desc: '公文泄露、权限冒充、数据安全等' },
}
function templateIcon(name) { return templateMeta[name]?.icon || '📋' }
function templateDisplayName(name) { return templateMeta[name]?.name || name }
function templateDescription(name) { return templateMeta[name]?.desc || '' }

// ==================== Selection ====================
function toggleSelectInbound(name) {
  const idx = selectedInbound.value.indexOf(name)
  if (idx >= 0) selectedInbound.value.splice(idx, 1)
  else selectedInbound.value.push(name)
}

function toggleSelectOutbound(name) {
  const idx = selectedOutbound.value.indexOf(name)
  if (idx >= 0) selectedOutbound.value.splice(idx, 1)
  else selectedOutbound.value.push(name)
}

// ==================== Data Loading ====================
async function loadRuleHits() {
  hitsLoading.value = true
  try { const d = await api('/api/v1/rules/hits'); ruleHits.value = Array.isArray(d) ? d : (d.hits || []) } catch { ruleHits.value = [] }
  hitsLoading.value = false
}

async function loadInbound() {
  inboundLoading.value = true
  try {
    const d = await api('/api/v1/inbound-rules?detail=1')
    const list = d.rules || []
    inboundRules.value = list.map(r => ({ ...r, patterns_count: r.patterns_count ?? (r.patterns ? r.patterns.length : '--') }))
    if (d.version && typeof d.version === 'object') inboundMeta.value = d.version
    else inboundMeta.value = null
  } catch { inboundRules.value = [] }
  inboundLoading.value = false
}

async function loadOutbound() {
  outboundLoading.value = true
  try {
    const d = await api('/api/v1/outbound-rules?detail=1')
    const list = d.rules || []
    outboundRules.value = list.map(r => ({ ...r, patterns_count: r.patterns_count ?? (r.patterns ? r.patterns.length : '--') }))
  } catch { outboundRules.value = [] }
  outboundLoading.value = false
}

async function loadTemplates() {
  templatesLoading.value = true
  try { const d = await api('/api/v1/rule-templates'); templates.value = d.templates || [] } catch { templates.value = [] }
  templatesLoading.value = false
}

async function toggleTemplate(name) {
  if (expandedTemplate.value === name) { expandedTemplate.value = null; templateRules.value = []; return }
  expandedTemplate.value = name
  templateDetailLoading.value = true
  templateRules.value = []
  try { const d = await api('/api/v1/rule-templates/detail?name=' + encodeURIComponent(name)); templateRules.value = d.rules || [] }
  catch (e) { showToast('加载模板详情失败: ' + e.message, 'error'); templateRules.value = [] }
  templateDetailLoading.value = false
}

// ==================== Editor ====================
function openCreateEditor() {
  editingRule.value = null
  editingDirection.value = activeTab.value
  editorErrors.value = {}
  editorVisible.value = true
}

function openEditEditor(row, direction) {
  editingRule.value = row
  editingDirection.value = direction
  editorErrors.value = {}
  editorVisible.value = true
}

function closeEditor() {
  editorVisible.value = false
  editorErrors.value = {}
}

// Validate patterns for regex type
function validatePatterns(patterns, type) {
  if (type !== 'regex') return null
  for (const p of patterns) {
    try { new RegExp(p) } catch (e) { return 'Invalid regex "' + p + '": ' + e.message }
  }
  return null
}

async function saveRule(data) {
  // Validate
  const errors = {}
  if (!data.name || !data.name.trim()) errors.name = '名称不能为空'
  const patterns = (data.patterns || []).filter(p => p.trim())
  if (patterns.length === 0) errors.patterns = '至少需要一个模式'
  const regexErr = validatePatterns(patterns, data.type)
  if (regexErr) errors.patterns = regexErr
  if (Object.keys(errors).length) {
    editorErrors.value = errors
    return
  }
  editorErrors.value = {}

  const direction = editingDirection.value
  const isOutbound = direction === 'outbound'
  const basePath = isOutbound ? '/api/v1/outbound-rules' : '/api/v1/inbound-rules'

  try {
    if (editingRule.value) {
      await apiPut(basePath + '/update', data)
      showToast('规则已更新: ' + data.name, 'success')
    } else {
      await apiPost(basePath + '/add', data)
      showToast('规则已创建: ' + data.name, 'success')
    }
    editorVisible.value = false
    if (isOutbound) loadOutbound()
    else loadInbound()
  } catch (e) {
    showToast('操作失败: ' + e.message, 'error')
  }
}

// ==================== Delete ====================
function confirmDeleteRule(row, direction) {
  confirmTitle.value = '删除规则'
  confirmMessage.value = '确认删除' + (direction === 'outbound' ? '出站' : '入站') + '规则 "' + row.name + '"？此操作不可恢复。'
  confirmType.value = 'danger'
  confirmAction = async () => {
    const basePath = direction === 'outbound' ? '/api/v1/outbound-rules' : '/api/v1/inbound-rules'
    try {
      await apiDelete(basePath + '/delete', { name: row.name })
      showToast('规则已删除: ' + row.name, 'success')
      if (direction === 'outbound') loadOutbound()
      else loadInbound()
    } catch (e) {
      showToast('删除失败: ' + e.message, 'error')
    }
  }
  confirmVisible.value = true
}

// ==================== Batch Operations ====================
async function batchActionInbound(action) {
  const names = [...selectedInbound.value]
  if (!names.length) return
  let success = 0
  let failed = 0
  for (const name of names) {
    const rule = inboundRules.value.find(r => r.name === name)
    if (!rule) continue
    try {
      await apiPut('/api/v1/inbound-rules/update', { ...rule, action })
      success++
    } catch { failed++ }
  }
  showToast('批量设为 ' + action + ': 成功 ' + success + ' 条' + (failed ? ', 失败 ' + failed + ' 条' : ''), failed ? 'error' : 'success')
  selectedInbound.value = []
  loadInbound()
}

function confirmBatchDeleteInbound() {
  const names = [...selectedInbound.value]
  confirmTitle.value = '批量删除规则'
  confirmMessage.value = '确认删除 ' + names.length + ' 条入站规则？此操作不可恢复。'
  confirmType.value = 'danger'
  confirmAction = async () => {
    let success = 0, failed = 0
    for (const name of names) {
      try { await apiDelete('/api/v1/inbound-rules/delete', { name }); success++ } catch { failed++ }
    }
    showToast('批量删除: 成功 ' + success + ' 条' + (failed ? ', 失败 ' + failed + ' 条' : ''), failed ? 'error' : 'success')
    selectedInbound.value = []
    loadInbound()
  }
  confirmVisible.value = true
}

function confirmBatchDeleteOutbound() {
  const names = [...selectedOutbound.value]
  confirmTitle.value = '批量删除规则'
  confirmMessage.value = '确认删除 ' + names.length + ' 条出站规则？此操作不可恢复。'
  confirmType.value = 'danger'
  confirmAction = async () => {
    let success = 0, failed = 0
    for (const name of names) {
      try { await apiDelete('/api/v1/outbound-rules/delete', { name }); success++ } catch { failed++ }
    }
    showToast('批量删除: 成功 ' + success + ' 条' + (failed ? ', 失败 ' + failed + ' 条' : ''), failed ? 'error' : 'success')
    selectedOutbound.value = []
    loadOutbound()
  }
  confirmVisible.value = true
}

// ==================== Import/Export ====================
async function exportRules() {
  try {
    await downloadFile(location.origin + '/api/v1/rules/export', 'lobster-guard-rules.yaml')
    showToast('规则导出成功', 'success')
  } catch (e) { showToast('导出失败: ' + e.message, 'error') }
}

function handleFileUpload(e) {
  const file = e.target.files[0]
  if (!file) return
  importFileName.value = file.name
  const reader = new FileReader()
  reader.onload = (ev) => { importYaml.value = ev.target.result; importPreview.value = null }
  reader.readAsText(file)
}

async function previewImport() {
  importLoading.value = true
  try { const d = await apiPost('/api/v1/rules/import?preview=1', { yaml: importYaml.value }); importPreview.value = d }
  catch (e) { showToast('预览失败: ' + e.message, 'error') }
  importLoading.value = false
}

async function doImport() {
  importLoading.value = true
  try {
    const d = await apiPost('/api/v1/rules/import', { yaml: importYaml.value })
    showToast('导入成功: ' + d.imported + ' 条规则 (新增 ' + d.new_count + ', 覆盖 ' + d.override_count + ')', 'success')
    showImport.value = false; importYaml.value = ''; importFileName.value = ''; importPreview.value = null
    loadInbound()
  } catch (e) { showToast('导入失败: ' + e.message, 'error') }
  importLoading.value = false
}

// ==================== Reload / Reset ====================
function confirmResetHits() {
  confirmTitle.value = '重置命中统计'
  confirmMessage.value = '确认重置所有规则命中统计？此操作不可恢复。'
  confirmType.value = 'danger'
  confirmAction = async () => {
    try { await apiPost('/api/v1/rules/hits/reset', {}); showToast('命中统计已重置', 'success'); loadRuleHits() }
    catch (e) { showToast('重置失败: ' + e.message, 'error') }
  }
  confirmVisible.value = true
}

async function reloadInbound() {
  reloadingInbound.value = true
  try { await apiPost('/api/v1/inbound-rules/reload', {}); showToast('入站规则已热更新', 'success'); loadInbound() }
  catch (e) { showToast('更新失败: ' + e.message, 'error') }
  reloadingInbound.value = false
}

async function reloadOutbound() {
  reloadingOutbound.value = true
  try { await apiPost('/api/v1/outbound-rules/reload', {}); showToast('出站规则已热更新', 'success'); loadOutbound() }
  catch (e) { showToast('更新失败: ' + e.message, 'error') }
  reloadingOutbound.value = false
}

function doConfirm() {
  confirmVisible.value = false
  if (confirmAction) confirmAction()
}

onMounted(() => { loadRuleHits(); loadInbound(); loadOutbound(); loadTemplates() })
</script>

<style scoped>
/* ==================== Filter Bar ==================== */
.filter-bar {
  display: flex; align-items: center; gap: 12px; padding: 12px 16px;
  border-bottom: 1px solid var(--border-subtle); flex-wrap: wrap;
}
.filter-search {
  display: flex; align-items: center; gap: 8px; flex: 1; min-width: 200px;
  background: var(--bg-base); border: 1px solid var(--border-default);
  border-radius: var(--radius-md, 6px); padding: 6px 10px;
  transition: border-color .2s;
}
.filter-search:focus-within { border-color: var(--color-primary); }
.filter-input {
  border: none; background: transparent; color: var(--text-primary);
  font-size: .82rem; flex: 1; outline: none;
}
.filter-input::placeholder { color: var(--text-tertiary); }
.filter-clear {
  background: none; border: none; color: var(--text-tertiary); cursor: pointer;
  font-size: .85rem; padding: 0 4px; border-radius: 3px;
}
.filter-clear:hover { color: var(--text-primary); background: var(--border-subtle); }
.filter-group { display: flex; gap: 8px; }
.filter-select {
  background: var(--bg-base); border: 1px solid var(--border-default);
  border-radius: var(--radius-md, 6px); color: var(--text-primary);
  padding: 6px 10px; font-size: .82rem; cursor: pointer; outline: none;
}
.filter-select:focus { border-color: var(--color-primary); }

/* ==================== Batch Bar ==================== */
.batch-bar {
  display: flex; align-items: center; gap: 8px; padding: 8px 16px;
  background: var(--color-primary-dim, rgba(99, 102, 241, 0.1));
  border-bottom: 1px solid var(--border-subtle);
  animation: slideDown .2s ease-out;
}
@keyframes slideDown { from { opacity: 0; transform: translateY(-8px); } to { opacity: 1; transform: translateY(0); } }
.batch-info { font-size: .82rem; color: var(--text-secondary); margin-right: 4px; }
.batch-info b { color: var(--color-primary); }

/* ==================== Checkbox ==================== */
.rule-checkbox {
  accent-color: var(--color-primary); cursor: pointer;
  width: 15px; height: 15px;
}

/* ==================== Priority ==================== */
.priority-num { font-weight: 600; font-size: .82rem; }
.priority-high { color: #ff6b6b; }
.priority-med { color: #ffa94d; }
.priority-low { color: var(--text-secondary); }
.priority-badge { margin-left: 4px; font-size: .75rem; }

.rule-name { font-weight: 500; }
.rule-name.high-priority { color: #ff6b6b; }

:deep(.row-high-priority) { background: rgba(255, 107, 107, 0.04) !important; }
:deep(.row-high-priority:hover) { background: rgba(255, 107, 107, 0.08) !important; }

/* ==================== Expand Detail ==================== */
.rule-expand-detail { font-size: .82rem; }
.rule-expand-detail .expand-row { margin-bottom: 6px; }
.rule-expand-detail .expand-row b { color: var(--color-primary); }
.pattern-pre {
  background: var(--bg-base); padding: 8px; border-radius: var(--radius-md, 6px);
  margin-top: 4px; font-size: var(--text-xs, .75rem); overflow-x: auto;
  color: var(--color-success); border: 1px solid var(--border-subtle);
  font-family: 'Courier New', monospace;
}

/* ==================== Tab Badge ==================== */
.tab-badge {
  display: inline-block; background: var(--color-primary-dim, rgba(99, 102, 241, 0.15));
  color: var(--color-primary); font-size: .7rem; font-weight: 600;
  padding: 1px 6px; border-radius: 10px; margin-left: 6px;
  min-width: 20px; text-align: center;
}

/* ==================== Action Tags ==================== */
.tag-block { background: rgba(255, 68, 102, 0.15); color: #ff4466; border: 1px solid rgba(255, 68, 102, 0.3); }
.tag-warn { background: rgba(255, 169, 77, 0.15); color: #ffa94d; border: 1px solid rgba(255, 169, 77, 0.3); }
.tag-log { background: rgba(148, 163, 184, 0.15); color: #94a3b8; border: 1px solid rgba(148, 163, 184, 0.3); }
.tag-pass { background: rgba(148, 163, 184, 0.1); color: #64748b; }

.text-muted { color: var(--text-tertiary); }

/* ==================== Templates ==================== */
.template-list { padding: 0 16px 16px; }
.template-card { border: 1px solid var(--border-subtle); border-radius: var(--radius, 8px); margin-bottom: 10px; overflow: hidden; }
.template-header { display: flex; align-items: center; justify-content: space-between; padding: 12px 16px; cursor: pointer; transition: background .2s; }
.template-header:hover { background: var(--border-subtle); }
.template-info { display: flex; align-items: center; gap: 10px; }
.template-icon { font-size: 1.3rem; }
.template-name { font-weight: 600; font-size: .9rem; color: var(--text-primary); }
.template-desc { font-size: .78rem; color: var(--text-secondary); margin-top: 2px; }
.template-stats { display: flex; align-items: center; gap: 10px; }
.template-groups { font-size: .75rem; color: var(--text-secondary); }
.expand-arrow { font-size: .7rem; color: var(--text-secondary); transition: transform .2s; display: inline-block; }
.expand-arrow.expanded { transform: rotate(90deg); }
.template-detail { border-top: 1px solid var(--border-subtle); background: var(--bg-elevated); max-height: 400px; overflow-y: auto; }
.mini-table { width: 100%; border-collapse: collapse; font-size: .8rem; }
.mini-table th { text-align: left; padding: 8px 12px; color: var(--text-secondary); border-bottom: 1px solid var(--border-subtle); font-weight: 500; font-size: .75rem; text-transform: uppercase; }
.mini-table td { padding: 6px 12px; border-bottom: 1px solid var(--border-subtle); color: var(--text-primary); }
.mini-table tr:hover td { background: var(--bg-elevated); }

/* ==================== Import Modal ==================== */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,.6); z-index: 1000; display: flex; align-items: flex-start; justify-content: center; padding-top: 60px; animation: fadeIn .2s; }
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
.import-panel { background: var(--bg-surface); border: 1px solid var(--border-default); border-radius: var(--radius, 8px); width: 600px; max-width: 95vw; box-shadow: 0 16px 64px var(--shadow-lg, rgba(0,0,0,.5)); animation: slideUp .25s ease-out; }
@keyframes slideUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
.import-header { display: flex; align-items: center; gap: 8px; padding: 16px 20px; border-bottom: 1px solid var(--border-subtle); color: var(--text-primary); }
.editor-close { background: none; border: none; color: var(--text-secondary); font-size: 1.2rem; cursor: pointer; padding: 4px 8px; border-radius: 4px; }
.editor-close:hover { background: var(--border-subtle); color: var(--text-primary); }
.import-body { padding: 20px; }
.import-textarea { width: 100%; background: var(--bg-base); color: var(--text-primary); border: 1px solid var(--border-default); border-radius: 6px; padding: 10px; font-family: 'Courier New', monospace; font-size: .82rem; resize: vertical; }
.import-textarea:focus { border-color: var(--color-primary); outline: none; }
.import-preview { margin-top: 12px; padding: 10px; background: var(--bg-elevated); border-radius: 6px; font-size: .82rem; color: var(--text-primary); }
.import-footer { display: flex; justify-content: flex-end; gap: 8px; padding: 12px 20px; border-top: 1px solid var(--border-subtle); }
</style>
