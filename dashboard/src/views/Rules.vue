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
          <template v-if="activeTab === 'rules'">
            <button class="btn btn-sm" @click="openCreateEditor">新建规则</button>
            <button class="btn btn-ghost btn-sm" @click="exportRules" title="导出规则为 YAML">导出</button>
            <button class="btn btn-ghost btn-sm" @click="showImport = true" title="从 YAML 导入规则">导入</button>
          </template>
          <template v-else>
            <button class="btn btn-sm" @click="openCreateTemplate">创建模板</button>
            <button class="btn btn-ghost btn-sm" @click="loadInboundTemplates">刷新</button>
          </template>
        </div>
      </div>
      <div class="tab-header">
        <button class="tab-btn" :class="{ active: activeTab === 'rules' }" @click="activeTab = 'rules'">规则管理<span class="tab-badge">{{ filteredAllRules.length }}</span></button>
        <button class="tab-btn" :class="{ active: activeTab === 'templates' }" @click="activeTab = 'templates'">规则模板<span class="tab-badge" v-if="inboundTemplates.length">{{ inboundTemplates.length }}</span></button>
      </div>

      <!-- ========== Rules Tab (Merged Inbound + Outbound) ========== -->
      <div v-show="activeTab === 'rules'">
        <div class="filter-bar">
          <div class="filter-search">
            <Icon name="search" :size="14" color="var(--text-tertiary)" />
            <input type="text" v-model="ruleSearch" placeholder="搜索规则名称或模式..." class="filter-input" />
            <button v-if="ruleSearch" class="filter-clear" @click="ruleSearch = ''">✕</button>
          </div>
          <div class="filter-group">
            <select v-model="filterDirection" class="filter-select"><option value="">全部方向</option><option value="inbound">入站</option><option value="outbound">出站</option></select>
            <select v-model="filterAction" class="filter-select"><option value="">所有动作</option><option value="block">block</option><option value="warn">warn</option><option value="log">log</option></select>
            <select v-model="filterGroup" class="filter-select"><option value="">所有分组</option><option v-for="g in allGroups" :key="g" :value="g">{{ g }}</option></select>
          </div>
        </div>
        <div class="batch-bar" v-if="selectedRules.length > 0">
          <span class="batch-info">已选 <b>{{ selectedRules.length }}</b> 条</span>
          <button class="btn btn-ghost btn-sm" @click="batchAction('block')">批量拦截</button>
          <button class="btn btn-ghost btn-sm" @click="batchAction('warn')">批量警告</button>
          <button class="btn btn-ghost btn-sm" @click="batchAction('log')">批量记录</button>
          <button class="btn btn-danger btn-sm" @click="confirmBatchDelete">批量删除</button>
          <button class="btn btn-ghost btn-sm" @click="selectedRules = []">取消</button>
        </div>
        <DataTable :columns="allColumns" :data="filteredAllRules" :loading="inboundLoading || outboundLoading" empty-text="暂无规则" empty-desc="点击「新建规则」创建第一条规则" :expandable="true" :row-class="ruleRowClass">
          <template #cell-select="{ row }"><input type="checkbox" class="rule-checkbox" :checked="selectedRules.includes(row._key)" @click.stop="toggleSelectRule(row._key)" /></template>
          <template #cell-direction="{ row }"><span class="tag" :class="row._direction === 'inbound' ? 'tag-success' : 'tag-info'">{{ row._direction === 'inbound' ? '入站' : '出站' }}</span></template>
          <template #cell-name="{ row }"><span class="rule-name" :class="{ 'high-priority': (row.priority || 0) >= 80 }">{{ row.name }}</span><span v-if="(row.priority || 0) >= 80" class="priority-badge" title="高优先级">🔥</span></template>
          <template #cell-action="{ value }"><span class="tag" :class="actTag(value)">{{ value }}</span></template>
          <template #cell-type="{ value }"><span class="tag tag-info">{{ value || 'keyword' }}</span></template>
          <template #cell-priority="{ row }"><span class="priority-num" :class="priorityClass(row.priority)">{{ row.priority ?? '--' }}</span></template>
          <template #cell-group="{ value }"><span v-if="value" class="tag" :style="{ background: groupColor(value), color: '#fff' }">{{ value }}</span><span v-else class="text-muted">--</span></template>
          <template #expand="{ row }">
            <div class="rule-expand-detail">
              <div class="expand-row"><b>名称:</b> {{ row.name }}</div>
              <div class="expand-row"><b>方向:</b> <span class="tag" :class="row._direction === 'inbound' ? 'tag-success' : 'tag-info'" style="font-size:.72rem">{{ row._direction === 'inbound' ? '入站' : '出站' }}</span> | <b>类型:</b> {{ row.type || 'keyword' }} | <b>动作:</b> <span class="tag" :class="actTag(row.action)" style="font-size:.72rem">{{ row.action }}</span> | <b>优先级:</b> {{ row.priority ?? '--' }}</div>
              <div class="expand-row" v-if="row.message"><b>自定义消息:</b> {{ row.message }}</div>
              <div v-if="row.patterns && row.patterns.length" class="expand-row"><b>模式 ({{ row.patterns.length }}):</b><pre class="pattern-pre">{{ row.patterns.join('\n') }}</pre></div>
            </div>
          </template>
          <template #actions="{ row }">
            <button class="btn btn-ghost btn-sm" @click.stop="openEditEditor(row, row._direction)" title="编辑"><Icon name="edit" :size="12" /></button>
            <button class="btn btn-danger btn-sm" @click.stop="confirmDeleteRule(row, row._direction)" style="margin-left:4px" title="删除"><Icon name="trash" :size="12" /></button>
          </template>
        </DataTable>
        <div class="rule-meta" v-if="inboundMeta">版本: {{ inboundMeta.version }} 来源: {{ inboundMeta.source }} 加载: {{ fmtTime(inboundMeta.loaded_at) }}</div>
        <div style="margin-top:12px;display:flex;gap:8px">
          <button class="btn btn-sm" @click="reloadInbound" :disabled="reloadingInbound">{{ reloadingInbound ? '更新中...' : '热更新入站规则' }}</button>
          <button class="btn btn-sm" @click="reloadOutbound" :disabled="reloadingOutbound">{{ reloadingOutbound ? '更新中...' : '热更新出站规则' }}</button>
        </div>
      </div>
      <!-- ========== Templates Tab ========== -->
      <div v-show="activeTab === 'templates'">
        <div class="tpl-stat-row">
          <div class="tpl-stat-card"><div class="tpl-stat-value">{{ inboundTemplates.length }}</div><div class="tpl-stat-label">模板总数</div></div>
          <div class="tpl-stat-card"><div class="tpl-stat-value" style="color:var(--color-primary)">{{ tplBuiltInCount }}</div><div class="tpl-stat-label">内置模板</div></div>
          <div class="tpl-stat-card"><div class="tpl-stat-value" style="color:var(--color-success)">{{ tplCustomCount }}</div><div class="tpl-stat-label">自定义模板</div></div>
          <div class="tpl-stat-card"><div class="tpl-stat-value" style="color:var(--color-warning)">{{ tplTotalRuleCount }}</div><div class="tpl-stat-label">规则总数</div></div>
        </div>
        <div class="filter-bar">
          <div class="filter-search">
            <Icon name="search" :size="14" color="var(--text-tertiary)" />
            <input type="text" v-model="tplSearchText" placeholder="搜索模板名称或描述..." class="filter-input" />
            <button v-if="tplSearchText" class="filter-clear" @click="tplSearchText = ''">✕</button>
          </div>
          <div class="filter-group">
            <select v-model="tplFilterCategory" class="filter-select">
              <option value="">全部类别</option>
              <option value="industry">行业</option>
              <option value="security">安全</option>
              <option value="compliance">合规</option>
            </select>
          </div>
        </div>
        <div class="tpl-grid">
          <div v-for="tpl in filteredInboundTemplates" :key="tpl.id" class="tpl-card" :class="{ 'tpl-card-builtin': tpl.built_in }">
            <div class="tpl-card-header">
              <div class="tpl-card-title">
                <span class="tpl-name">{{ tpl.name }}</span>
                <span v-if="tpl.built_in" class="tpl-badge tpl-badge-builtin">内置</span>
                <span v-else class="tpl-badge tpl-badge-custom">自定义</span>
                <span class="tpl-badge tpl-badge-category">{{ tplCategoryLabel(tpl.category) }}</span>
              </div>
              <div class="tpl-card-actions">
                <button class="btn btn-ghost btn-sm" @click="toggleTplExpand(tpl.id)">{{ tplExpandedIds[tpl.id] ? '收起' : '展开规则' }}</button>
                <button class="btn btn-ghost btn-sm" @click="openEditTemplate(tpl)">编辑</button>
                <button class="btn btn-danger btn-sm" :disabled="tpl.built_in" @click="confirmDeleteTemplate(tpl)">删除</button>
              </div>
            </div>
            <div class="tpl-card-desc">{{ tpl.description || '暂无描述' }}</div>
            <div class="tpl-card-meta">
              <span class="tpl-meta-tag">ID: {{ tpl.id }}</span>
              <span class="tpl-meta-tag">{{ (tpl.rules || []).length }} 条规则</span>
            </div>
            <div v-if="tplExpandedIds[tpl.id]" class="tpl-rules-detail">
              <div class="tpl-rules-title">规则列表</div>
              <div v-if="!tpl.rules || tpl.rules.length === 0" style="padding:8px 0;color:var(--text-tertiary);font-size:.85rem">暂无规则</div>
              <div v-for="(rule, idx) in (tpl.rules || [])" :key="idx" class="tpl-rule-item">
                <div class="tpl-rule-header">
                  <span class="tpl-rule-name">{{ rule.name }}</span>
                  <span class="tag" :class="actTag(rule.action)" style="font-size:.7rem">{{ rule.action }}</span>
                  <span class="tag tag-info" style="font-size:.7rem">{{ rule.type || 'keyword' }}</span>
                  <span style="font-size:.7rem;color:var(--text-tertiary)">{{ rule.category }}</span>
                  <span v-if="rule.group" class="tag" :style="{ background: groupColor(rule.group), color: '#fff', fontSize: '.7rem' }">{{ rule.group }}</span>
                </div>
                <div class="tpl-rule-patterns">
                  模式: <code v-for="(p, pi) in (rule.patterns || []).slice(0, 5)" :key="pi">{{ p }}</code>
                  <span v-if="(rule.patterns || []).length > 5" style="color:var(--color-primary);font-style:italic">+{{ rule.patterns.length - 5 }} 更多</span>
                </div>
                <div v-if="rule.message" class="tpl-rule-msg">提示: {{ rule.message }}</div>
              </div>
            </div>
          </div>
        </div>
        <div v-if="filteredInboundTemplates.length === 0 && !tplLoading" style="text-align:center;padding:32px;color:var(--text-tertiary)">
          <div style="font-size:2rem;margin-bottom:8px">📋</div>
          <div>暂无匹配的入站规则模板</div>
        </div>
      </div>
    </div>

    <!-- Regex Tester -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg></span><span class="card-title">正则表达式测试器</span>
      </div>
      <div style="padding:0 16px 16px"><RegexTester /></div>
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

    <!-- Template Create/Edit Modal -->
    <Teleport to="body">
      <div v-if="showTplModal" class="modal-overlay" @click.self="showTplModal = false">
        <div class="import-panel" style="max-width:800px">
          <div class="import-header">
            <span style="font-weight:600;flex:1">{{ tplIsEdit ? '编辑入站规则模板' : '创建入站规则模板' }}</span>
            <button class="editor-close" @click="showTplModal = false">✕</button>
          </div>
          <div class="import-body">
            <div style="margin-bottom:12px"><label style="display:block;font-size:.82rem;color:var(--text-secondary);margin-bottom:4px">模板 ID</label><input v-model="tplForm.id" :disabled="tplIsEdit" class="import-textarea" style="height:auto;min-height:0;padding:8px 10px;font-family:inherit;resize:none" placeholder="如 my-inbound-tpl" /></div>
            <div style="margin-bottom:12px"><label style="display:block;font-size:.82rem;color:var(--text-secondary);margin-bottom:4px">名称</label><input v-model="tplForm.name" class="import-textarea" style="height:auto;min-height:0;padding:8px 10px;font-family:inherit;resize:none" placeholder="模板名称" /></div>
            <div style="margin-bottom:12px"><label style="display:block;font-size:.82rem;color:var(--text-secondary);margin-bottom:4px">描述</label><input v-model="tplForm.description" class="import-textarea" style="height:auto;min-height:0;padding:8px 10px;font-family:inherit;resize:none" placeholder="模板描述" /></div>
            <div style="margin-bottom:12px"><label style="display:block;font-size:.82rem;color:var(--text-secondary);margin-bottom:4px">类别</label><select v-model="tplForm.category" class="filter-select" style="width:100%"><option value="industry">行业</option><option value="security">安全</option><option value="compliance">合规</option></select></div>
            <div style="margin-bottom:12px"><label style="display:block;font-size:.82rem;color:var(--text-secondary);margin-bottom:4px">规则 (JSON 数组)</label><textarea v-model="tplForm.rulesJSON" rows="12" class="import-textarea" style="font-family:'Courier New',monospace;font-size:.8rem" placeholder='[{"name":"规则1","patterns":["关键词1"],"action":"block"}]'></textarea><div v-if="tplJsonError" style="color:var(--color-danger);font-size:.8rem;margin-top:4px">{{ tplJsonError }}</div></div>
          </div>
          <div class="import-footer">
            <button class="btn btn-ghost btn-sm" @click="showTplModal = false">取消</button>
            <button class="btn btn-sm" @click="submitTemplate" :disabled="tplSubmitting">{{ tplSubmitting ? '提交中...' : (tplIsEdit ? '保存' : '创建') }}</button>
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
import { ref, reactive, computed, onMounted } from 'vue'
import { api, apiPost, apiPut, apiDelete, downloadFile } from '../api.js'
import { showToast } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Icon from '../components/Icon.vue'
import RuleEditor from '../components/RuleEditor.vue'
import RegexTester from '../components/RegexTester.vue'

const activeTab = ref('rules')

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

// ==================== Unified Rules (Merged) ====================
const ruleSearch = ref('')
const filterDirection = ref('')
const filterAction = ref('')
const filterGroup = ref('')
const selectedRules = ref([])

const allColumns = [
  { key: 'select', label: '', sortable: false, width: '36px' },
  { key: 'direction', label: '方向', sortable: true, width: '72px' },
  { key: 'name', label: '名称', sortable: true },
  { key: 'action', label: '动作', sortable: true },
  { key: 'type', label: '类型', sortable: true },
  { key: 'priority', label: '优先级', sortable: true },
  { key: 'patterns_count', label: '模式数', sortable: true },
  { key: 'group', label: '分组', sortable: true },
]

const allRules = computed(() => {
  const inList = inboundRules.value.map(r => ({ ...r, _direction: 'inbound', _key: 'in_' + r.name }))
  const outList = outboundRules.value.map(r => ({ ...r, _direction: 'outbound', _key: 'out_' + r.name }))
  return [...inList, ...outList]
})

const allGroups = computed(() => {
  const groups = new Set()
  allRules.value.forEach(r => { if (r.group) groups.add(r.group) })
  return [...groups].sort()
})

const filteredAllRules = computed(() => {
  let list = allRules.value
  const q = ruleSearch.value.trim().toLowerCase()
  if (q) {
    list = list.filter(r =>
      (r.name || '').toLowerCase().includes(q) ||
      (r.patterns || []).some(p => p.toLowerCase().includes(q))
    )
  }
  if (filterDirection.value) list = list.filter(r => r._direction === filterDirection.value)
  if (filterAction.value) list = list.filter(r => r.action === filterAction.value)
  if (filterGroup.value) list = list.filter(r => r.group === filterGroup.value)
  return list
})

function ruleRowClass(row) { return (row.priority || 0) >= 80 ? 'row-high-priority' : '' }
function toggleSelectRule(key) { const idx = selectedRules.value.indexOf(key); if (idx >= 0) selectedRules.value.splice(idx, 1); else selectedRules.value.push(key) }

// ==================== Inbound Templates ====================
const inboundTemplates = ref([])
const tplLoading = ref(false)
const tplSearchText = ref('')
const tplFilterCategory = ref('')
const tplExpandedIds = reactive({})

const tplBuiltInCount = computed(() => inboundTemplates.value.filter(t => t.built_in).length)
const tplCustomCount = computed(() => inboundTemplates.value.filter(t => !t.built_in).length)
const tplTotalRuleCount = computed(() => inboundTemplates.value.reduce((sum, t) => sum + (t.rules || []).length, 0))

const filteredInboundTemplates = computed(() => {
  let list = inboundTemplates.value
  if (tplSearchText.value) {
    const q = tplSearchText.value.toLowerCase()
    list = list.filter(t => (t.name || '').toLowerCase().includes(q) || (t.description || '').toLowerCase().includes(q) || (t.id || '').toLowerCase().includes(q))
  }
  if (tplFilterCategory.value) list = list.filter(t => t.category === tplFilterCategory.value)
  return list
})

function tplCategoryLabel(c) { return { industry: '行业', security: '安全', compliance: '合规' }[c] || c }
function toggleTplExpand(id) { tplExpandedIds[id] = !tplExpandedIds[id] }

// Template Modal
const showTplModal = ref(false)
const tplIsEdit = ref(false)
const tplSubmitting = ref(false)
const tplJsonError = ref('')
const tplForm = ref({ id: '', name: '', description: '', category: 'security', rulesJSON: '[]' })

function openCreateTemplate() {
  tplIsEdit.value = false
  tplForm.value = { id: '', name: '', description: '', category: 'security', rulesJSON: '[]' }
  tplJsonError.value = ''
  showTplModal.value = true
}

function openEditTemplate(tpl) {
  tplIsEdit.value = true
  tplForm.value = { id: tpl.id, name: tpl.name, description: tpl.description || '', category: tpl.category || 'security', rulesJSON: JSON.stringify(tpl.rules || [], null, 2) }
  tplJsonError.value = ''
  showTplModal.value = true
}

async function submitTemplate() {
  let rules
  try { rules = JSON.parse(tplForm.value.rulesJSON); if (!Array.isArray(rules)) throw new Error('必须是数组'); tplJsonError.value = '' }
  catch (e) { tplJsonError.value = 'JSON 格式错误: ' + e.message; return }
  tplSubmitting.value = true
  try {
    const body = { id: tplForm.value.id, name: tplForm.value.name, description: tplForm.value.description, category: tplForm.value.category, rules }
    if (tplIsEdit.value) { await apiPut('/api/v1/inbound-templates/' + tplForm.value.id, body); showToast('模板已更新', 'success') }
    else { await apiPost('/api/v1/inbound-templates', body); showToast('模板已创建', 'success') }
    showTplModal.value = false
    loadInboundTemplates()
  } catch (e) { showToast('操作失败: ' + e.message, 'error') }
  tplSubmitting.value = false
}

function confirmDeleteTemplate(tpl) {
  confirmTitle.value = '删除模板'
  confirmMessage.value = '确定删除模板「' + tpl.name + '」？此操作不可恢复。'
  confirmType.value = 'danger'
  confirmAction = async () => {
    try { await apiDelete('/api/v1/inbound-templates/' + tpl.id); showToast('模板已删除', 'success'); loadInboundTemplates() }
    catch (e) { showToast('删除失败: ' + e.message, 'error') }
  }
  confirmVisible.value = true
}

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
function priorityClass(p) { if (p == null) return ''; if (p >= 80) return 'priority-high'; if (p >= 40) return 'priority-med'; return 'priority-low' }
function inboundRowClass(row) { return (row.priority || 0) >= 80 ? 'row-high-priority' : '' }

// ==================== Selection ====================
function toggleSelectInbound(name) { const idx = selectedInbound.value.indexOf(name); if (idx >= 0) selectedInbound.value.splice(idx, 1); else selectedInbound.value.push(name) }
function toggleSelectOutbound(name) { const idx = selectedOutbound.value.indexOf(name); if (idx >= 0) selectedOutbound.value.splice(idx, 1); else selectedOutbound.value.push(name) }

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
    if (d.version && typeof d.version === 'object') inboundMeta.value = d.version; else inboundMeta.value = null
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

async function loadInboundTemplates() {
  tplLoading.value = true
  try { const d = await api('/api/v1/inbound-templates'); inboundTemplates.value = d.templates || [] } catch { inboundTemplates.value = [] }
  tplLoading.value = false
}

// ==================== Editor ====================
function openCreateEditor() { editingRule.value = null; editingDirection.value = 'inbound'; editorErrors.value = {}; editorVisible.value = true }
function openEditEditor(row, direction) { editingRule.value = row; editingDirection.value = direction; editorErrors.value = {}; editorVisible.value = true }
function closeEditor() { editorVisible.value = false; editorErrors.value = {} }

function validatePatterns(patterns, type) {
  if (type !== 'regex') return null
  for (const p of patterns) { try { new RegExp(p) } catch (e) { return 'Invalid regex "' + p + '": ' + e.message } }
  return null
}

async function saveRule(data) {
  const errors = {}
  if (!data.name || !data.name.trim()) errors.name = '名称不能为空'
  const patterns = (data.patterns || []).filter(p => p.trim())
  if (patterns.length === 0) errors.patterns = '至少需要一个模式'
  const regexErr = validatePatterns(patterns, data.type)
  if (regexErr) errors.patterns = regexErr
  if (Object.keys(errors).length) { editorErrors.value = errors; return }
  editorErrors.value = {}
  const direction = editingDirection.value
  const isOutbound = direction === 'outbound'
  const basePath = isOutbound ? '/api/v1/outbound-rules' : '/api/v1/inbound-rules'
  try {
    if (editingRule.value) { await apiPut(basePath + '/update', data); showToast('规则已更新: ' + data.name, 'success') }
    else { await apiPost(basePath + '/add', data); showToast('规则已创建: ' + data.name, 'success') }
    editorVisible.value = false
    if (isOutbound) loadOutbound(); else loadInbound()
  } catch (e) { showToast('操作失败: ' + e.message, 'error') }
}

// ==================== Delete ====================
function confirmDeleteRule(row, direction) {
  confirmTitle.value = '删除规则'
  confirmMessage.value = '确认删除' + (direction === 'outbound' ? '出站' : '入站') + '规则 "' + row.name + '"？此操作不可恢复。'
  confirmType.value = 'danger'
  confirmAction = async () => {
    const basePath = direction === 'outbound' ? '/api/v1/outbound-rules' : '/api/v1/inbound-rules'
    try { await apiDelete(basePath + '/delete', { name: row.name }); showToast('规则已删除: ' + row.name, 'success'); if (direction === 'outbound') loadOutbound(); else loadInbound() }
    catch (e) { showToast('删除失败: ' + e.message, 'error') }
  }
  confirmVisible.value = true
}

// ==================== Batch Operations ====================
async function batchActionInbound(action) {
  const names = [...selectedInbound.value]; if (!names.length) return; let success = 0; let failed = 0
  for (const name of names) { const rule = inboundRules.value.find(r => r.name === name); if (!rule) continue; try { await apiPut('/api/v1/inbound-rules/update', { ...rule, action }); success++ } catch { failed++ } }
  showToast('批量设为 ' + action + ': 成功 ' + success + ' 条' + (failed ? ', 失败 ' + failed + ' 条' : ''), failed ? 'error' : 'success')
  selectedInbound.value = []; loadInbound()
}

function confirmBatchDeleteInbound() {
  const names = [...selectedInbound.value]
  confirmTitle.value = '批量删除规则'; confirmMessage.value = '确认删除 ' + names.length + ' 条入站规则？此操作不可恢复。'; confirmType.value = 'danger'
  confirmAction = async () => { let success = 0, failed = 0; for (const name of names) { try { await apiDelete('/api/v1/inbound-rules/delete', { name }); success++ } catch { failed++ } }; showToast('批量删除: 成功 ' + success + ' 条' + (failed ? ', 失败 ' + failed + ' 条' : ''), failed ? 'error' : 'success'); selectedInbound.value = []; loadInbound() }
  confirmVisible.value = true
}

function confirmBatchDeleteOutbound() {
  const names = [...selectedOutbound.value]
  confirmTitle.value = '批量删除规则'; confirmMessage.value = '确认删除 ' + names.length + ' 条出站规则？此操作不可恢复。'; confirmType.value = 'danger'
  confirmAction = async () => { let success = 0, failed = 0; for (const name of names) { try { await apiDelete('/api/v1/outbound-rules/delete', { name }); success++ } catch { failed++ } }; showToast('批量删除: 成功 ' + success + ' 条' + (failed ? ', 失败 ' + failed + ' 条' : ''), failed ? 'error' : 'success'); selectedOutbound.value = []; loadOutbound() }
  confirmVisible.value = true
}

// ==================== Unified Batch Operations ====================
async function batchAction(action) {
  const keys = [...selectedRules.value]; if (!keys.length) return; let success = 0; let failed = 0
  for (const key of keys) {
    const rule = allRules.value.find(r => r._key === key); if (!rule) continue
    const basePath = rule._direction === 'outbound' ? '/api/v1/outbound-rules' : '/api/v1/inbound-rules'
    try { await apiPut(basePath + '/update', { ...rule, action }); success++ } catch { failed++ }
  }
  showToast('批量设为 ' + action + ': 成功 ' + success + ' 条' + (failed ? ', 失败 ' + failed + ' 条' : ''), failed ? 'error' : 'success')
  selectedRules.value = []; loadInbound(); loadOutbound()
}

function confirmBatchDelete() {
  const keys = [...selectedRules.value]
  confirmTitle.value = '批量删除规则'; confirmMessage.value = '确认删除 ' + keys.length + ' 条规则？此操作不可恢复。'; confirmType.value = 'danger'
  confirmAction = async () => {
    let success = 0, failed = 0
    for (const key of keys) {
      const rule = allRules.value.find(r => r._key === key); if (!rule) continue
      const basePath = rule._direction === 'outbound' ? '/api/v1/outbound-rules' : '/api/v1/inbound-rules'
      try { await apiDelete(basePath + '/delete', { name: rule.name }); success++ } catch { failed++ }
    }
    showToast('批量删除: 成功 ' + success + ' 条' + (failed ? ', 失败 ' + failed + ' 条' : ''), failed ? 'error' : 'success')
    selectedRules.value = []; loadInbound(); loadOutbound()
  }
  confirmVisible.value = true
}

// ==================== Import/Export ====================
async function exportRules() { try { await downloadFile(location.origin + '/api/v1/rules/export', 'lobster-guard-rules.yaml'); showToast('规则导出成功', 'success') } catch (e) { showToast('导出失败: ' + e.message, 'error') } }

function handleFileUpload(e) { const file = e.target.files[0]; if (!file) return; importFileName.value = file.name; const reader = new FileReader(); reader.onload = (ev) => { importYaml.value = ev.target.result; importPreview.value = null }; reader.readAsText(file) }

async function previewImport() { importLoading.value = true; try { const d = await apiPost('/api/v1/rules/import?preview=1', { yaml: importYaml.value }); importPreview.value = d } catch (e) { showToast('预览失败: ' + e.message, 'error') }; importLoading.value = false }

async function doImport() {
  importLoading.value = true
  try { const d = await apiPost('/api/v1/rules/import', { yaml: importYaml.value }); showToast('导入成功: ' + d.imported + ' 条规则 (新增 ' + d.new_count + ', 覆盖 ' + d.override_count + ')', 'success'); showImport.value = false; importYaml.value = ''; importFileName.value = ''; importPreview.value = null; loadInbound() }
  catch (e) { showToast('导入失败: ' + e.message, 'error') }
  importLoading.value = false
}

// ==================== Reload / Reset ====================
function confirmResetHits() {
  confirmTitle.value = '重置命中统计'; confirmMessage.value = '确认重置所有规则命中统计？此操作不可恢复。'; confirmType.value = 'danger'
  confirmAction = async () => { try { await apiPost('/api/v1/rules/hits/reset', {}); showToast('命中统计已重置', 'success'); loadRuleHits() } catch (e) { showToast('重置失败: ' + e.message, 'error') } }
  confirmVisible.value = true
}

async function reloadInbound() { reloadingInbound.value = true; try { await apiPost('/api/v1/inbound-rules/reload', {}); showToast('入站规则已热更新', 'success'); loadInbound() } catch (e) { showToast('更新失败: ' + e.message, 'error') }; reloadingInbound.value = false }
async function reloadOutbound() { reloadingOutbound.value = true; try { await apiPost('/api/v1/outbound-rules/reload', {}); showToast('出站规则已热更新', 'success'); loadOutbound() } catch (e) { showToast('更新失败: ' + e.message, 'error') }; reloadingOutbound.value = false }

function doConfirm() { confirmVisible.value = false; if (confirmAction) confirmAction() }

onMounted(() => { loadRuleHits(); loadInbound(); loadOutbound(); loadInboundTemplates() })
</script>

<style scoped>
/* ==================== Filter Bar ==================== */
.filter-bar { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border-bottom: 1px solid var(--border-subtle); flex-wrap: wrap; }
.filter-search { display: flex; align-items: center; gap: 8px; flex: 1; min-width: 200px; background: var(--bg-base); border: 1px solid var(--border-default); border-radius: var(--radius-md, 6px); padding: 6px 10px; transition: border-color .2s; }
.filter-search:focus-within { border-color: var(--color-primary); }
.filter-input { border: none; background: transparent; color: var(--text-primary); font-size: .82rem; flex: 1; outline: none; }
.filter-input::placeholder { color: var(--text-tertiary); }
.filter-clear { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: .85rem; padding: 0 4px; border-radius: 3px; }
.filter-clear:hover { color: var(--text-primary); background: var(--border-subtle); }
.filter-group { display: flex; gap: 8px; }
.filter-select { background: var(--bg-base); border: 1px solid var(--border-default); border-radius: var(--radius-md, 6px); color: var(--text-primary); padding: 6px 10px; font-size: .82rem; cursor: pointer; outline: none; }
.filter-select:focus { border-color: var(--color-primary); }

/* ==================== Batch Bar ==================== */
.batch-bar { display: flex; align-items: center; gap: 8px; padding: 8px 16px; background: var(--color-primary-dim, rgba(99, 102, 241, 0.1)); border-bottom: 1px solid var(--border-subtle); animation: slideDown .2s ease-out; }
@keyframes slideDown { from { opacity: 0; transform: translateY(-8px); } to { opacity: 1; transform: translateY(0); } }
.batch-info { font-size: .82rem; color: var(--text-secondary); margin-right: 4px; }
.batch-info b { color: var(--color-primary); }

/* ==================== Checkbox ==================== */
.rule-checkbox { accent-color: var(--color-primary); cursor: pointer; width: 15px; height: 15px; }

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
.pattern-pre { background: var(--bg-base); padding: 8px; border-radius: var(--radius-md, 6px); margin-top: 4px; font-size: var(--text-xs, .75rem); overflow-x: auto; color: var(--color-success); border: 1px solid var(--border-subtle); font-family: 'Courier New', monospace; }

/* ==================== Tab Badge ==================== */
.tab-badge { display: inline-block; background: var(--color-primary-dim, rgba(99, 102, 241, 0.15)); color: var(--color-primary); font-size: .7rem; font-weight: 600; padding: 1px 6px; border-radius: 10px; margin-left: 6px; min-width: 20px; text-align: center; }

/* ==================== Action Tags ==================== */
.tag-block { background: rgba(255, 68, 102, 0.15); color: #ff4466; border: 1px solid rgba(255, 68, 102, 0.3); }
.tag-warn { background: rgba(255, 169, 77, 0.15); color: #ffa94d; border: 1px solid rgba(255, 169, 77, 0.3); }
.tag-log { background: rgba(148, 163, 184, 0.15); color: #94a3b8; border: 1px solid rgba(148, 163, 184, 0.3); }
.tag-pass { background: rgba(148, 163, 184, 0.1); color: #64748b; }
.tag-success { background: rgba(16, 185, 129, 0.15); color: #10b981; border: 1px solid rgba(16, 185, 129, 0.3); }
.tag-info { background: rgba(99, 102, 241, 0.15); color: #6366f1; border: 1px solid rgba(99, 102, 241, 0.3); }
.text-muted { color: var(--text-tertiary); }

/* ==================== Templates Tab ==================== */
.tpl-stat-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; padding: 16px; }
.tpl-stat-card { background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: var(--radius-md, 6px); padding: 12px; text-align: center; }
.tpl-stat-value { font-size: 1.4rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.tpl-stat-label { font-size: .75rem; color: var(--text-tertiary); margin-top: 2px; }

.tpl-grid { padding: 0 16px 16px; }
.tpl-card { border: 1px solid var(--border-subtle); border-radius: var(--radius, 8px); margin-bottom: 10px; padding: 14px 16px; transition: box-shadow .2s; }
.tpl-card:hover { box-shadow: 0 4px 16px rgba(99,102,241,0.08); }
.tpl-card-builtin { border-left: 3px solid var(--color-primary); }
.tpl-card-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 6px; flex-wrap: wrap; gap: 8px; }
.tpl-card-title { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.tpl-name { font-weight: 600; font-size: .95rem; color: var(--text-primary); }
.tpl-card-actions { display: flex; gap: 4px; }
.tpl-card-desc { color: var(--text-secondary); font-size: .82rem; margin-bottom: 6px; }
.tpl-card-meta { display: flex; gap: 8px; flex-wrap: wrap; }
.tpl-meta-tag { font-size: .72rem; padding: 2px 8px; border-radius: 4px; background: var(--bg-elevated, var(--bg-base)); color: var(--text-tertiary); }
.tpl-badge { padding: 2px 8px; border-radius: 4px; font-size: .68rem; font-weight: 600; }
.tpl-badge-builtin { background: rgba(99,102,241,0.12); color: var(--color-primary); }
.tpl-badge-custom { background: rgba(16,185,129,0.12); color: var(--color-success); }
.tpl-badge-category { background: rgba(245,158,11,0.12); color: var(--color-warning); }

.tpl-rules-detail { margin-top: 10px; border-top: 1px solid var(--border-subtle); padding-top: 10px; }
.tpl-rules-title { font-weight: 600; font-size: .85rem; margin-bottom: 8px; color: var(--text-primary); }
.tpl-rule-item { padding: 8px; margin-bottom: 6px; background: var(--bg-elevated, var(--bg-base)); border-radius: 4px; border: 1px solid var(--border-subtle); }
.tpl-rule-header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.tpl-rule-name { font-weight: 500; font-size: .85rem; }
.tpl-rule-patterns { font-size: .75rem; color: var(--text-tertiary); margin-top: 4px; }
.tpl-rule-patterns code { background: var(--bg-surface, var(--bg-base)); padding: 1px 4px; border-radius: 2px; margin: 0 2px; font-size: .7rem; }
.tpl-rule-msg { font-size: .78rem; color: var(--text-secondary); margin-top: 2px; }

/* ==================== Import Modal ==================== */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,.6); z-index: 1000; display: flex; align-items: flex-start; justify-content: center; padding-top: 60px; animation: fadeIn .2s; }
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
.import-panel { background: var(--bg-surface); border: 1px solid var(--border-default); border-radius: var(--radius, 8px); width: 600px; max-width: 95vw; box-shadow: 0 16px 64px var(--shadow-lg, rgba(0,0,0,.5)); animation: slideUp .25s ease-out; }
@keyframes slideUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
.import-header { display: flex; align-items: center; gap: 8px; padding: 16px 20px; border-bottom: 1px solid var(--border-subtle); color: var(--text-primary); }
.editor-close { background: none; border: none; color: var(--text-secondary); font-size: 1.2rem; cursor: pointer; padding: 4px 8px; border-radius: 4px; }
.editor-close:hover { background: var(--border-subtle); color: var(--text-primary); }
.import-body { padding: 20px; max-height: 60vh; overflow-y: auto; }
.import-textarea { width: 100%; background: var(--bg-base); color: var(--text-primary); border: 1px solid var(--border-default); border-radius: 6px; padding: 10px; font-family: 'Courier New', monospace; font-size: .82rem; resize: vertical; }
.import-textarea:focus { border-color: var(--color-primary); outline: none; }
.import-preview { margin-top: 12px; padding: 10px; background: var(--bg-elevated); border-radius: 6px; font-size: .82rem; color: var(--text-primary); }
.import-footer { display: flex; justify-content: flex-end; gap: 8px; padding: 12px 20px; border-top: 1px solid var(--border-subtle); }

@media (max-width: 768px) {
  .tpl-stat-row { grid-template-columns: repeat(2, 1fr); }
}
</style>
