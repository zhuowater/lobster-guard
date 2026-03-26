<template>
  <div>
    <!-- 顶部统计 -->
    <div class="stat-row" style="margin-bottom:20px">
      <div class="stat-card">
        <div class="stat-value">{{ templates.length }}</div>
        <div class="stat-label">模板总数</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--color-primary)">{{ builtInCount }}</div>
        <div class="stat-label">内置模板</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--color-success)">{{ customCount }}</div>
        <div class="stat-label">自定义模板</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--color-warning)">{{ totalRuleCount }}</div>
        <div class="stat-label">规则总数</div>
      </div>
    </div>

    <!-- 操作栏 -->
    <div class="filter-bar card" style="margin-bottom:16px">
      <div class="filter-bar-inner">
        <div class="search-box">
          <input v-model="searchText" class="search-input" placeholder="搜索模板名称或描述..." />
          <button v-if="searchText" class="search-clear" @click="searchText = ''" title="清除">&times;</button>
        </div>
        <div class="filter-selects">
          <select v-model="filterCategory" class="filter-select">
            <option value="">全部类别</option>
            <option value="industry">行业</option>
            <option value="security">安全</option>
            <option value="compliance">合规</option>
          </select>
          <button class="btn btn-primary" @click="openCreate">+ 创建模板</button>
        </div>
      </div>
    </div>

    <!-- 模板列表 -->
    <div class="template-grid">
      <div v-for="tpl in filteredTemplates" :key="tpl.id" class="card template-card" :class="{ 'card-builtin': tpl.built_in }">
        <div class="template-header">
          <div class="template-title">
            <span class="template-name">{{ tpl.name }}</span>
            <span v-if="tpl.built_in" class="badge badge-builtin">内置</span>
            <span v-else class="badge badge-custom">自定义</span>
            <span class="badge badge-category">{{ categoryLabel(tpl.category) }}</span>
          </div>
          <div class="template-actions">
            <button class="btn-xs btn-outline" @click="toggleExpand(tpl.id)">
              {{ expandedIds[tpl.id] ? '收起' : '展开规则' }}
            </button>
            <button class="btn-xs btn-primary" @click="openEdit(tpl)">编辑</button>
            <button class="btn-xs btn-danger" :disabled="tpl.built_in" @click="confirmDelete(tpl)">删除</button>
          </div>
        </div>
        <div class="template-desc">{{ tpl.description || '暂无描述' }}</div>
        <div class="template-meta">
          <span class="meta-tag">ID: {{ tpl.id }}</span>
          <span class="meta-tag">{{ (tpl.rules || []).length }} 条规则</span>
        </div>
        <!-- 展开规则详情 -->
        <div v-if="expandedIds[tpl.id]" class="template-rules">
          <div class="rules-title">规则列表</div>
          <div v-if="!tpl.rules || tpl.rules.length === 0" class="empty-hint">暂无规则</div>
          <div v-for="(rule, idx) in (tpl.rules || [])" :key="idx" class="rule-item">
            <div class="rule-header">
              <span class="rule-name">{{ rule.name || rule.id }}</span>
              <span class="rule-action-badge" :class="'action-' + rule.action">{{ rule.action }}</span>
              <span class="rule-type-badge">{{ rule.type || 'keyword' }}</span>
              <span class="rule-category-tag">{{ rule.category }}</span>
              <span v-if="rule.direction" class="rule-dir-tag">{{ dirLabel(rule.direction) }}</span>
            </div>
            <div v-if="rule.description" class="rule-desc-text">{{ rule.description }}</div>
            <div class="rule-patterns">
              模式: <code v-for="(p, pi) in (rule.patterns || []).slice(0, 5)" :key="pi">{{ p }}</code>
              <span v-if="(rule.patterns || []).length > 5" class="more-tag">+{{ rule.patterns.length - 5 }} 更多</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="filteredTemplates.length === 0 && !loading" class="empty-state card">
      <div class="empty-icon">📋</div>
      <div class="empty-text">暂无匹配的 LLM 规则模板</div>
    </div>

    <!-- 创建/编辑模态框 -->
    <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal-content modal-lg">
        <div class="modal-header">
          <h3>{{ isEdit ? '编辑 LLM 规则模板' : '创建 LLM 规则模板' }}</h3>
          <button class="modal-close" @click="closeModal">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>模板 ID</label>
            <input class="form-input" v-model="form.id" :disabled="isEdit" placeholder="如 my-custom-tpl" />
          </div>
          <div class="form-group">
            <label>名称</label>
            <input class="form-input" v-model="form.name" placeholder="模板名称" />
          </div>
          <div class="form-group">
            <label>描述</label>
            <input class="form-input" v-model="form.description" placeholder="模板描述" />
          </div>
          <div class="form-group">
            <label>类别</label>
            <select class="form-input" v-model="form.category">
              <option value="industry">行业</option>
              <option value="security">安全</option>
              <option value="compliance">合规</option>
            </select>
          </div>
          <div class="form-group">
            <label>规则 (JSON 数组)</label>
            <textarea class="form-input rules-json-editor" v-model="form.rulesJSON" rows="12"
              placeholder='[{"id":"r1","name":"规则1","category":"pii_leak","direction":"both","type":"keyword","patterns":["关键词"],"action":"block","enabled":true,"priority":10}]'></textarea>
            <div v-if="jsonError" class="form-error">{{ jsonError }}</div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-outline" @click="closeModal">取消</button>
          <button class="btn btn-primary" @click="submitForm" :disabled="submitting">
            {{ submitting ? '提交中...' : (isEdit ? '保存' : '创建') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 删除确认 -->
    <ConfirmModal v-if="showDeleteModal" :message="deleteMsg" @confirm="doDelete" @cancel="showDeleteModal = false" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, inject } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import ConfirmModal from '../components/ConfirmModal.vue'

const showToast = inject('showToast')
const templates = ref([])
const loading = ref(true)
const searchText = ref('')
const filterCategory = ref('')
const expandedIds = reactive({})

// Modal
const showModal = ref(false)
const isEdit = ref(false)
const submitting = ref(false)
const jsonError = ref('')
const form = ref({ id: '', name: '', description: '', category: 'security', rulesJSON: '[]' })

// Delete
const showDeleteModal = ref(false)
const deleteMsg = ref('')
const deleteTarget = ref(null)

const builtInCount = computed(() => templates.value.filter(t => t.built_in).length)
const customCount = computed(() => templates.value.filter(t => !t.built_in).length)
const totalRuleCount = computed(() => templates.value.reduce((sum, t) => sum + (t.rules || []).length, 0))

const filteredTemplates = computed(() => {
  let list = templates.value
  if (searchText.value) {
    const q = searchText.value.toLowerCase()
    list = list.filter(t => (t.name || '').toLowerCase().includes(q) || (t.description || '').toLowerCase().includes(q) || (t.id || '').toLowerCase().includes(q))
  }
  if (filterCategory.value) {
    list = list.filter(t => t.category === filterCategory.value)
  }
  return list
})

function categoryLabel(c) { return { industry: '行业', security: '安全', compliance: '合规' }[c] || c }
function dirLabel(d) { return { request: '请求', response: '响应', both: '双向' }[d] || d }

function toggleExpand(id) { expandedIds[id] = !expandedIds[id] }

async function loadTemplates() {
  loading.value = true
  try {
    const d = await api('/api/v1/llm/templates')
    templates.value = d.templates || []
  } catch { templates.value = [] }
  loading.value = false
}

function openCreate() {
  isEdit.value = false
  form.value = { id: '', name: '', description: '', category: 'security', rulesJSON: '[]' }
  jsonError.value = ''
  showModal.value = true
}

function openEdit(tpl) {
  isEdit.value = true
  form.value = {
    id: tpl.id,
    name: tpl.name,
    description: tpl.description || '',
    category: tpl.category || 'security',
    rulesJSON: JSON.stringify(tpl.rules || [], null, 2)
  }
  jsonError.value = ''
  showModal.value = true
}

function closeModal() { showModal.value = false }

async function submitForm() {
  let rules
  try {
    rules = JSON.parse(form.value.rulesJSON)
    if (!Array.isArray(rules)) throw new Error('必须是数组')
    jsonError.value = ''
  } catch (e) {
    jsonError.value = 'JSON 格式错误: ' + e.message
    return
  }
  submitting.value = true
  try {
    const body = { id: form.value.id, name: form.value.name, description: form.value.description, category: form.value.category, rules }
    if (isEdit.value) {
      await apiPut('/api/v1/llm/templates/' + form.value.id, body)
      showToast('模板已更新', 'success')
    } else {
      await apiPost('/api/v1/llm/templates', body)
      showToast('模板已创建', 'success')
    }
    closeModal()
    loadTemplates()
  } catch (e) { showToast('操作失败: ' + e.message, 'error') }
  submitting.value = false
}

function confirmDelete(tpl) {
  deleteTarget.value = tpl
  deleteMsg.value = `确定删除模板「${tpl.name}」？此操作不可恢复。`
  showDeleteModal.value = true
}

async function doDelete() {
  showDeleteModal.value = false
  if (!deleteTarget.value) return
  try {
    await apiDelete('/api/v1/llm/templates/' + deleteTarget.value.id)
    showToast('模板已删除', 'success')
    loadTemplates()
  } catch (e) { showToast('删除失败: ' + e.message, 'error') }
}

onMounted(loadTemplates)
</script>

<style scoped>
.template-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(480px, 1fr)); gap: var(--space-4); }
.template-card { padding: var(--space-4); transition: box-shadow 0.2s; }
.template-card:hover { box-shadow: 0 4px 16px rgba(99,102,241,0.10); }
.card-builtin { border-left: 3px solid var(--color-primary); }
.template-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: var(--space-2); flex-wrap: wrap; gap: var(--space-2); }
.template-title { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; }
.template-name { font-weight: 600; font-size: 1.05rem; color: var(--text-primary); }
.template-actions { display: flex; gap: var(--space-1); }
.template-desc { color: var(--text-secondary); font-size: 0.875rem; margin-bottom: var(--space-2); }
.template-meta { display: flex; gap: var(--space-2); flex-wrap: wrap; }
.meta-tag { font-size: 0.75rem; padding: 2px 8px; border-radius: var(--radius-sm); background: var(--bg-elevated); color: var(--text-tertiary); }
.badge { padding: 2px 8px; border-radius: var(--radius-sm); font-size: 0.7rem; font-weight: 600; }
.badge-builtin { background: rgba(99,102,241,0.12); color: var(--color-primary); }
.badge-custom { background: rgba(16,185,129,0.12); color: var(--color-success); }
.badge-category { background: rgba(245,158,11,0.12); color: var(--color-warning); }
.template-rules { margin-top: var(--space-3); border-top: 1px solid var(--border-subtle); padding-top: var(--space-3); }
.rules-title { font-weight: 600; font-size: 0.875rem; margin-bottom: var(--space-2); color: var(--text-primary); }
.rule-item { padding: var(--space-2); margin-bottom: var(--space-2); background: var(--bg-elevated); border-radius: var(--radius-sm); border: 1px solid var(--border-subtle); }
.rule-header { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; }
.rule-name { font-weight: 500; font-size: 0.875rem; }
.rule-action-badge { padding: 1px 6px; border-radius: 3px; font-size: 0.7rem; font-weight: 600; }
.action-block { background: rgba(239,68,68,0.12); color: #EF4444; }
.action-warn { background: rgba(245,158,11,0.12); color: #F59E0B; }
.action-log { background: rgba(59,130,246,0.12); color: #3B82F6; }
.action-rewrite { background: rgba(139,92,246,0.12); color: #8B5CF6; }
.rule-type-badge { padding: 1px 6px; border-radius: 3px; font-size: 0.7rem; background: rgba(99,102,241,0.08); color: var(--text-tertiary); }
.rule-category-tag { font-size: 0.7rem; color: var(--text-tertiary); }
.rule-dir-tag { font-size: 0.7rem; padding: 1px 6px; border-radius: 3px; background: rgba(16,185,129,0.08); color: var(--color-success); }
.rule-desc-text { font-size: 0.8rem; color: var(--text-secondary); margin: 4px 0; }
.rule-patterns { font-size: 0.75rem; color: var(--text-tertiary); margin-top: 4px; }
.rule-patterns code { background: var(--bg-surface); padding: 1px 4px; border-radius: 2px; margin: 0 2px; font-size: 0.7rem; }
.more-tag { color: var(--color-primary); font-style: italic; }

/* Modal */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal-content { background: var(--bg-surface); border-radius: var(--radius-lg); width: 90%; max-width: 720px; max-height: 90vh; overflow-y: auto; box-shadow: 0 20px 60px rgba(0,0,0,0.3); }
.modal-lg { max-width: 800px; }
.modal-header { display: flex; justify-content: space-between; align-items: center; padding: var(--space-4); border-bottom: 1px solid var(--border-subtle); }
.modal-header h3 { margin: 0; font-size: 1.1rem; }
.modal-close { background: none; border: none; font-size: 1.5rem; cursor: pointer; color: var(--text-tertiary); }
.modal-body { padding: var(--space-4); }
.modal-footer { display: flex; justify-content: flex-end; gap: var(--space-2); padding: var(--space-4); border-top: 1px solid var(--border-subtle); }
.form-group { margin-bottom: var(--space-3); }
.form-group label { display: block; font-size: 0.85rem; font-weight: 500; margin-bottom: 4px; color: var(--text-secondary); }
.form-input { width: 100%; padding: 8px 12px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--bg-elevated); color: var(--text-primary); font-size: 0.9rem; }
.rules-json-editor { font-family: monospace; font-size: 0.8rem; line-height: 1.5; min-height: 200px; resize: vertical; }
.form-error { color: #EF4444; font-size: 0.8rem; margin-top: 4px; }

/* Buttons */
.btn { padding: 8px 16px; border-radius: var(--radius-sm); font-size: 0.875rem; font-weight: 500; cursor: pointer; border: none; transition: all 0.15s; }
.btn-primary { background: var(--color-primary); color: white; }
.btn-primary:hover { opacity: 0.9; }
.btn-outline { background: transparent; border: 1px solid var(--border-subtle); color: var(--text-secondary); }
.btn-outline:hover { border-color: var(--color-primary); color: var(--color-primary); }
.btn-xs { padding: 3px 8px; font-size: 0.75rem; border-radius: 3px; cursor: pointer; border: none; }
.btn-xs.btn-outline { background: transparent; border: 1px solid var(--border-subtle); color: var(--text-tertiary); }
.btn-xs.btn-primary { background: var(--color-primary); color: white; }
.btn-xs.btn-danger { background: rgba(239,68,68,0.12); color: #EF4444; }
.btn-xs.btn-danger:disabled { opacity: 0.4; cursor: not-allowed; }
.btn-danger { background: #EF4444; color: white; }

/* Empty state */
.empty-state { text-align: center; padding: var(--space-8); }
.empty-icon { font-size: 3rem; margin-bottom: var(--space-2); }
.empty-text { color: var(--text-tertiary); }
.empty-hint { color: var(--text-tertiary); font-size: 0.85rem; padding: var(--space-2) 0; }

/* Stat row */
.stat-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: var(--space-3); }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-3); text-align: center; }
.stat-value { font-size: 1.5rem; font-weight: 700; color: var(--color-primary); }
.stat-label { font-size: 0.8rem; color: var(--text-tertiary); margin-top: 2px; }

/* Filter bar */
.filter-bar { padding: var(--space-3); }
.filter-bar-inner { display: flex; gap: var(--space-3); align-items: center; flex-wrap: wrap; }
.search-box { position: relative; flex: 1; min-width: 200px; }
.search-input { width: 100%; padding: 8px 12px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--bg-elevated); color: var(--text-primary); }
.search-clear { position: absolute; right: 8px; top: 50%; transform: translateY(-50%); background: none; border: none; cursor: pointer; color: var(--text-tertiary); font-size: 1.1rem; }
.filter-selects { display: flex; gap: var(--space-2); align-items: center; }
.filter-select { padding: 8px 12px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--bg-elevated); color: var(--text-primary); font-size: 0.85rem; }
</style>
