<template>
  <div>
    <!-- 顶部统计栏 -->
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

    <!-- 规则列表 -->
    <div class="card">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg></span>
        <span class="card-title">LLM 规则引擎</span>
        <div class="card-actions">
          <button class="btn btn-ghost btn-sm" @click="loadData">刷新</button>
          <button class="btn btn-sm" @click="openCreate">新建规则</button>
        </div>
      </div>
      <!-- 筛选状态 badge -->
      <div v-if="filterCategory" class="filter-badge-wrap">
        <span class="filter-badge">筛选: {{ catLabel(filterCategory) }} <button class="filter-clear" @click="clearFilter">✕</button></span>
      </div>
      <DataTable :columns="columns" :data="rulesWithHits" :loading="loading" empty-text="暂无 LLM 规则" empty-desc="点击右上角新建规则" :expandable="true">
        <!-- 状态列 -->
        <template #cell-status="{ row }">
          <span v-if="row.shadow_mode" class="status-icon" title="影子模式">👻</span>
          <span v-else-if="row.enabled" class="status-icon" title="启用">🟢</span>
          <span v-else class="status-icon" title="禁用">🔴</span>
        </template>
        <!-- 类别 badge -->
        <template #cell-category="{ value }">
          <span class="tag" :style="catStyle(value)">{{ catLabel(value) }}</span>
        </template>
        <!-- 方向 badge -->
        <template #cell-direction="{ value }">
          <span class="tag" :style="dirStyle(value)">{{ dirLabel(value) }}</span>
        </template>
        <!-- 类型 -->
        <template #cell-type="{ value }">
          <span class="tag tag-info">{{ value }}</span>
        </template>
        <!-- 动作 badge -->
        <template #cell-action="{ value }">
          <span class="tag" :class="actTag(value)">{{ value }}</span>
        </template>
        <!-- 命中数 -->
        <template #cell-hits="{ row }">
          <span v-if="row.shadow_mode" style="color:var(--text-tertiary);border-bottom:1px dashed var(--text-tertiary)" :title="'如果激活会影响 '+row._shadow_hits+' 条'">{{ row._shadow_hits || 0 }}</span>
          <span v-else style="font-weight:700;color:var(--color-primary)">{{ row._hits || 0 }}</span>
        </template>
        <!-- 影子命中 -->
        <template #cell-shadow_hits="{ row }">
          <span v-if="row.shadow_mode" style="color:var(--color-warning)">{{ row._shadow_hits || 0 }}</span>
          <span v-else>-</span>
        </template>
        <!-- 展开详情 -->
        <template #expand="{ row }">
          <div style="font-size:.82rem">
            <div><b style="color:var(--color-primary)">ID:</b> {{ row.id }}</div>
            <div v-if="row.description"><b style="color:var(--color-primary)">描述:</b> {{ row.description }}</div>
            <div><b style="color:var(--color-primary)">方向:</b> {{ dirLabel(row.direction) }} | <b style="color:var(--color-primary)">类型:</b> {{ row.type }} | <b style="color:var(--color-primary)">动作:</b> {{ row.action }} | <b style="color:var(--color-primary)">优先级:</b> {{ row.priority }}</div>
            <div v-if="row.rewrite_to"><b style="color:var(--color-primary)">Rewrite 目标:</b> <code>{{ row.rewrite_to }}</code></div>
            <div v-if="row.patterns && row.patterns.length"><b style="color:var(--color-primary)">模式 ({{ row.patterns.length }}):</b>
              <pre style="background:var(--bg-base);padding:8px;border-radius:var(--radius-md);margin-top:4px;font-size:var(--text-xs);overflow-x:auto;color:var(--color-success);border:1px solid var(--border-subtle)">{{ row.patterns.join('\n') }}</pre>
            </div>
          </div>
        </template>
        <!-- 操作按钮 -->
        <template #actions="{ row }">
          <button class="btn btn-ghost btn-sm" @click.stop="openEdit(row)" title="编辑">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
          </button>
          <button class="btn btn-ghost btn-sm" @click.stop="toggleShadow(row)" :title="row.shadow_mode?'切换为激活':'切换为影子模式'" style="margin-left:2px">
            <span v-if="row.shadow_mode">🔄</span>
            <span v-else>👻</span>
          </button>
          <button class="btn btn-danger btn-sm" @click.stop="confirmDelete(row)" style="margin-left:2px" title="删除">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
          </button>
        </template>
      </DataTable>
    </div>

    <!-- 新建/编辑模态框 -->
    <div v-if="showModal" class="modal-overlay" @click.self="showModal=false">
      <div class="modal-box" style="max-width:600px">
        <div class="modal-header">
          <h3>{{ editMode ? '编辑规则' : '新建规则' }}</h3>
          <button class="modal-close" @click="showModal=false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>ID</label>
            <input v-model="form.id" :disabled="editMode" class="input" placeholder="llm-custom-xxx (自动生成)">
          </div>
          <div class="form-group">
            <label>名称 <span style="color:var(--color-danger)">*</span></label>
            <input v-model="form.name" class="input" placeholder="规则名称">
          </div>
          <div class="form-group">
            <label>描述</label>
            <input v-model="form.description" class="input" placeholder="规则描述（可选）">
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>类别</label>
              <select v-model="form.category" class="input">
                <option value="prompt_injection">Prompt Injection</option>
                <option value="pii_leak">PII Leak</option>
                <option value="sensitive_topic">Sensitive Topic</option>
                <option value="token_abuse">Token Abuse</option>
                <option value="custom">Custom</option>
              </select>
            </div>
            <div class="form-group">
              <label>方向</label>
              <select v-model="form.direction" class="input">
                <option value="request">Request →</option>
                <option value="response">← Response</option>
                <option value="both">Both ↔</option>
              </select>
            </div>
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>类型</label>
              <select v-model="form.type" class="input">
                <option value="keyword">Keyword</option>
                <option value="regex">Regex</option>
              </select>
            </div>
            <div class="form-group">
              <label>动作</label>
              <select v-model="form.action" class="input">
                <option value="log">Log</option>
                <option value="warn">Warn</option>
                <option value="block">Block</option>
                <option value="rewrite">Rewrite</option>
              </select>
            </div>
          </div>
          <div class="form-group">
            <label>模式列表（每行一个）<span style="color:var(--color-danger)">*</span></label>
            <textarea v-model="patternsText" class="input" rows="5" placeholder="每行一个关键词或正则表达式" style="font-family:var(--font-mono);font-size:var(--text-xs)"></textarea>
          </div>
          <div v-if="form.action==='rewrite'" class="form-group">
            <label>Rewrite 目标</label>
            <input v-model="form.rewrite_to" class="input" placeholder="[REDACTED]">
          </div>
          <div class="form-group">
            <label>优先级</label>
            <input v-model.number="form.priority" type="number" class="input" min="0" max="100">
          </div>
          <div class="form-row" style="gap:20px">
            <label class="toggle-label"><input type="checkbox" v-model="form.enabled"> 启用</label>
            <label class="toggle-label"><input type="checkbox" v-model="form.shadow_mode"> 影子模式 <small style="color:var(--text-tertiary)">（只记录不执行，用于测试新规则）</small></label>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="showModal=false">取消</button>
          <button class="btn" @click="saveRule" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button>
        </div>
      </div>
    </div>

    <!-- 删除确认 -->
    <ConfirmModal v-if="deleteTarget" title="删除规则" :message="'确定删除规则 '+deleteTarget.id+'？'" @confirm="doDelete" @cancel="deleteTarget=null"/>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, apiPost } from '../api.js'
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
const filterCategory = ref('')

const defaultForm = () => ({
  id: '', name: '', description: '', category: 'custom', direction: 'request',
  type: 'keyword', patterns: [], action: 'log', rewrite_to: '', priority: 10,
  enabled: true, shadow_mode: false,
})
const form = ref(defaultForm())

const columns = [
  { key: 'status', label: '状态', width: '50px' },
  { key: 'id', label: 'ID', width: '130px' },
  { key: 'name', label: '名称' },
  { key: 'category', label: '类别', width: '140px' },
  { key: 'direction', label: '方向', width: '100px' },
  { key: 'type', label: '类型', width: '80px' },
  { key: 'action', label: '动作', width: '80px' },
  { key: 'hits', label: '命中', width: '70px' },
  { key: 'shadow_hits', label: '影子命中', width: '85px' },
]

const filteredRules = computed(() => {
  if (!filterCategory.value) return rules.value
  return rules.value.filter(r => r.category === filterCategory.value)
})

const rulesWithHits = computed(() => {
  return filteredRules.value.map(r => ({
    ...r,
    _hits: hits.value[r.id]?.count || 0,
    _shadow_hits: hits.value[r.id]?.shadow_hits || 0,
  }))
})

function clearFilter() {
  filterCategory.value = ''
  router.replace({ path: '/llm-rules' })
}

const totalRules = computed(() => rules.value.length)
const enabledRules = computed(() => rules.value.filter(r => r.enabled && !r.shadow_mode).length)
const shadowRules = computed(() => rules.value.filter(r => r.shadow_mode).length)
const totalHits = computed(() => {
  let sum = 0
  for (const h of Object.values(hits.value)) {
    sum += (h.count || 0) + (h.shadow_hits || 0)
  }
  return sum
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
    console.error('Load LLM rules failed:', e)
  }
  loading.value = false
}

function catStyle(cat) {
  const colors = {
    prompt_injection: { background: 'var(--color-danger-dim, #3a1a1a)', color: 'var(--color-danger)' },
    pii_leak: { background: '#3a2a1a', color: '#f0a050' },
    sensitive_topic: { background: '#2a1a3a', color: '#c080f0' },
    token_abuse: { background: '#3a3a1a', color: '#e0c040' },
    custom: { background: 'var(--bg-elevated)', color: 'var(--text-secondary)' },
  }
  return colors[cat] || colors.custom
}

function catLabel(cat) {
  const labels = {
    prompt_injection: 'Prompt Injection',
    pii_leak: 'PII Leak',
    sensitive_topic: 'Sensitive Topic',
    token_abuse: 'Token Abuse',
    custom: 'Custom',
  }
  return labels[cat] || cat
}

function dirStyle(dir) {
  const styles = {
    request: { background: 'var(--color-primary-dim, #1a2a3a)', color: 'var(--color-primary)' },
    response: { background: '#1a3a2a', color: 'var(--color-success)' },
    both: { background: '#2a1a3a', color: '#c080f0' },
  }
  return styles[dir] || {}
}

function dirLabel(dir) {
  return { request: '请求 →', response: '← 响应', both: '↔ 双向' }[dir] || dir
}

function actTag(action) {
  return {
    log: 'tag-secondary',
    warn: 'tag-warning',
    block: 'tag-danger',
    rewrite: 'tag-info',
  }[action] || 'tag-secondary'
}

function openCreate() {
  editMode.value = false
  form.value = defaultForm()
  patternsText.value = ''
  showModal.value = true
}

function openEdit(row) {
  editMode.value = true
  form.value = { ...row }
  patternsText.value = (row.patterns || []).join('\n')
  showModal.value = true
}

async function saveRule() {
  saving.value = true
  const data = { ...form.value }
  data.patterns = patternsText.value.split('\n').map(s => s.trim()).filter(Boolean)
  if (!data.name || !data.patterns.length) {
    alert('名称和模式不能为空')
    saving.value = false
    return
  }
  try {
    if (editMode.value) {
      await api('/api/v1/llm/rules/' + data.id, { method: 'PUT', body: JSON.stringify(data) })
    } else {
      await apiPost('/api/v1/llm/rules', data)
    }
    showModal.value = false
    await loadData()
  } catch (e) {
    alert('保存失败: ' + e.message)
  }
  saving.value = false
}

function confirmDelete(row) {
  deleteTarget.value = row
}

async function doDelete() {
  if (!deleteTarget.value) return
  try {
    await api('/api/v1/llm/rules/' + deleteTarget.value.id, { method: 'DELETE' })
    deleteTarget.value = null
    await loadData()
  } catch (e) {
    alert('删除失败: ' + e.message)
  }
}

async function toggleShadow(row) {
  try {
    await apiPost('/api/v1/llm/rules/' + row.id + '/toggle-shadow', {})
    await loadData()
  } catch (e) {
    alert('切换失败: ' + e.message)
  }
}

onMounted(() => {
  if (route.query.category) {
    filterCategory.value = route.query.category
  }
  loadData()
})

// Watch for route query changes (e.g. navigating from OWASP matrix)
watch(() => route.query.category, (val) => {
  filterCategory.value = val || ''
})
</script>

<style scoped>
.stat-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}
.stat-card {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  padding: 16px;
  text-align: center;
}
.stat-value {
  font-size: 1.8rem;
  font-weight: 700;
  color: var(--text-primary);
  font-family: var(--font-mono);
}
.stat-label {
  font-size: var(--text-xs);
  color: var(--text-tertiary);
  margin-top: 4px;
}
.status-icon {
  font-size: 1rem;
}

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
select.input { appearance: auto; }
textarea.input { resize: vertical; }
.toggle-label {
  display: inline-flex; align-items: center; gap: 6px; font-size: var(--text-sm);
  color: var(--text-secondary); cursor: pointer;
}
/* 筛选 badge */
.filter-badge-wrap { margin-bottom: 12px; }
.filter-badge {
  display: inline-flex; align-items: center; gap: 8px;
  padding: 4px 12px; background: rgba(99,102,241,0.15); color: var(--color-primary);
  border-radius: 9999px; font-size: var(--text-xs); font-weight: 600;
}
.filter-clear {
  background: none; border: none; color: var(--color-primary); cursor: pointer;
  font-size: 14px; line-height: 1; padding: 0 2px; font-weight: 700;
  opacity: 0.7; transition: opacity .2s;
}
.filter-clear:hover { opacity: 1; }
@media (max-width:768px) {
  .stat-row { grid-template-columns: repeat(2,1fr); }
  .form-row { grid-template-columns: 1fr; }
}
</style>
