<template>
  <div>
    <!-- Top Stats -->
    <div class="stat-row" style="margin-bottom:20px">
      <div class="stat-card">
        <div class="stat-value">{{ targets.length }}</div>
        <div class="stat-label">上游目标总数</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--color-success)">{{ targets.filter(t => t.path_prefix).length }}</div>
        <div class="stat-label">路径前缀路由</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color:var(--color-primary)">{{ targets.filter(t => t.api_key_header).length }}</div>
        <div class="stat-label">自定义 API Key Header</div>
      </div>
    </div>

    <!-- Target List Card -->
    <div class="card">
      <div class="card-header">
        <span class="card-icon">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
          </svg>
        </span>
        <span class="card-title">LLM 上游目标管理</span>
        <div class="card-actions">
          <button class="btn btn-ghost btn-sm" @click="loadData">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
            刷新
          </button>
          <button class="btn btn-sm" @click="openCreate">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            新建目标
          </button>
        </div>
      </div>

      <!-- Table -->
      <div v-if="loading" class="loading-wrap">
        <div class="loading-spinner"></div>
        <span style="color:var(--text-tertiary);font-size:var(--text-sm)">加载中...</span>
      </div>
      <div v-else-if="!targets.length" class="empty-state">
        <div class="empty-icon">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--text-quaternary)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
          </svg>
        </div>
        <div class="empty-text">暂无上游目标</div>
        <div class="empty-desc">点击右上角「新建目标」添加 LLM 上游</div>
      </div>
      <div v-else class="targets-table-wrap">
        <table class="targets-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>上游地址</th>
              <th>路径前缀</th>
              <th>API Key Header</th>
              <th style="width:100px;text-align:right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in targets" :key="t.name" class="target-row">
              <td>
                <div class="target-name-cell">
                  <span class="target-name">{{ t.name }}</span>
                </div>
              </td>
              <td>
                <code class="upstream-url">{{ t.upstream }}</code>
              </td>
              <td>
                <code v-if="t.path_prefix" class="path-prefix">{{ t.path_prefix }}</code>
                <span v-else class="no-value">—</span>
              </td>
              <td>
                <code v-if="t.api_key_header" class="api-key-hdr">{{ t.api_key_header }}</code>
                <span v-else class="no-value">默认 (Authorization)</span>
              </td>
              <td class="actions-cell">
                <div class="action-btns">
                  <button class="btn btn-ghost btn-sm" @click="openEdit(t)" title="编辑">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                  </button>
                  <button class="btn btn-danger btn-sm" @click="confirmDelete(t)" title="删除">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @click.self="showModal = false">
      <div class="modal-box" style="max-width:560px">
        <div class="modal-header">
          <h3>{{ editMode ? '编辑目标' : '新建目标' }}</h3>
          <button class="modal-close" @click="showModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>名称 <span class="required">*</span></label>
            <input
              v-model="form.name"
              :disabled="editMode"
              class="input"
              :class="{ 'input-error': formErrors.name, 'input-disabled': editMode }"
              placeholder="如: openai, deepseek, qax"
            />
            <div v-if="formErrors.name" class="form-error">{{ formErrors.name }}</div>
            <div v-if="editMode" class="form-hint">名称创建后不可修改</div>
          </div>
          <div class="form-group">
            <label>上游地址 <span class="required">*</span></label>
            <input
              v-model="form.upstream"
              class="input"
              :class="{ 'input-error': formErrors.upstream }"
              placeholder="https://api.openai.com"
            />
            <div v-if="formErrors.upstream" class="form-error">{{ formErrors.upstream }}</div>
          </div>
          <div class="form-group">
            <label>路径前缀</label>
            <input
              v-model="form.path_prefix"
              class="input"
              placeholder="/v1/chat （留空则为默认目标）"
            />
            <div class="form-hint">匹配请求路径前缀，用于路由到此上游</div>
          </div>
          <div class="form-group">
            <label>API Key Header</label>
            <input
              v-model="form.api_key_header"
              class="input"
              placeholder="Authorization（留空使用默认）"
            />
            <div class="form-hint">自定义 API Key 传递的 Header 名</div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="showModal = false">取消</button>
          <button class="btn" @click="saveTarget" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button>
        </div>
      </div>
    </div>

    <!-- Delete Confirm -->
    <ConfirmModal
      v-if="deleteItem"
      :visible="!!deleteItem"
      type="danger"
      title="删除目标"
      :message="deleteMessage"
      confirm-text="删除"
      @confirm="doDelete"
      @cancel="deleteItem = null"
    />
  </div>
</template>

<script setup>
import { ref, computed, reactive, onMounted } from 'vue'
import { api, apiPost, apiPut } from '../api.js'
import { showToast } from '../stores/app.js'
import ConfirmModal from '../components/ConfirmModal.vue'

const targets = ref([])
const loading = ref(true)
const showModal = ref(false)
const editMode = ref(false)
const saving = ref(false)
const deleteItem = ref(null)

const defaultForm = () => ({
  name: '',
  upstream: '',
  path_prefix: '',
  api_key_header: '',
})
const form = ref(defaultForm())
const formErrors = reactive({ name: '', upstream: '' })

const deleteMessage = computed(() => {
  if (!deleteItem.value) return ''
  return `确定删除上游目标「${deleteItem.value.name}」？此目标的路由配置将被永久移除。`
})

async function loadData() {
  loading.value = true
  try {
    const data = await api('/api/v1/llm/targets')
    targets.value = data.targets || []
  } catch (e) {
    showToast('加载目标失败: ' + e.message, 'error')
  }
  loading.value = false
}

function openCreate() {
  editMode.value = false
  form.value = defaultForm()
  clearFormErrors()
  showModal.value = true
}

function openEdit(t) {
  editMode.value = true
  form.value = { ...t }
  clearFormErrors()
  showModal.value = true
}

function clearFormErrors() {
  formErrors.name = ''
  formErrors.upstream = ''
}

function validateForm() {
  clearFormErrors()
  let valid = true
  if (!form.value.name.trim()) {
    formErrors.name = '名称不能为空'
    valid = false
  } else if (!editMode.value && targets.value.some(t => t.name === form.value.name.trim())) {
    formErrors.name = '名称已存在'
    valid = false
  }
  if (!form.value.upstream.trim()) {
    formErrors.upstream = '上游地址不能为空'
    valid = false
  } else if (!/^https?:\/\//.test(form.value.upstream.trim())) {
    formErrors.upstream = '必须以 http:// 或 https:// 开头'
    valid = false
  }
  return valid
}

async function saveTarget() {
  if (!validateForm()) return
  saving.value = true
  const data = { ...form.value }
  data.name = data.name.trim()
  data.upstream = data.upstream.trim()
  data.path_prefix = data.path_prefix.trim()
  data.api_key_header = data.api_key_header.trim()

  try {
    if (editMode.value) {
      await api('/api/v1/llm/targets/' + encodeURIComponent(data.name), {
        method: 'PUT',
        body: JSON.stringify(data),
      })
      showToast('目标已更新: ' + data.name, 'success')
    } else {
      await apiPost('/api/v1/llm/targets', data)
      showToast('目标已创建: ' + data.name, 'success')
    }
    showModal.value = false
    await loadData()
  } catch (e) {
    if (e.message.includes('409')) {
      formErrors.name = '名称已存在'
    }
    showToast('保存失败: ' + e.message, 'error')
  }
  saving.value = false
}

function confirmDelete(t) {
  deleteItem.value = t
}

async function doDelete() {
  if (!deleteItem.value) return
  const name = deleteItem.value.name
  try {
    await api('/api/v1/llm/targets/' + encodeURIComponent(name), { method: 'DELETE' })
    showToast('目标已删除: ' + name, 'success')
    deleteItem.value = null
    await loadData()
  } catch (e) {
    showToast('删除失败: ' + e.message, 'error')
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.stat-row { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 16px; text-align: center; }
.stat-value { font-size: 1.8rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: 4px; }

/* Loading */
.loading-wrap { display: flex; align-items: center; justify-content: center; gap: 10px; padding: 40px; }
.loading-spinner {
  width: 20px; height: 20px; border: 2px solid var(--border-subtle);
  border-top-color: var(--color-primary); border-radius: 50%;
  animation: spin 0.6s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* Empty State */
.empty-state { text-align: center; padding: 40px 20px; }
.empty-icon { margin-bottom: 12px; }
.empty-text { font-size: var(--text-base); font-weight: 600; color: var(--text-secondary); margin-bottom: 4px; }
.empty-desc { font-size: var(--text-sm); color: var(--text-tertiary); }

/* Table */
.targets-table-wrap { overflow-x: auto; }
.targets-table {
  width: 100%; border-collapse: collapse;
}
.targets-table thead th {
  text-align: left; padding: 10px 14px; font-size: var(--text-xs); font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.04em; color: var(--text-tertiary);
  border-bottom: 1px solid var(--border-subtle); background: var(--bg-elevated);
}
.targets-table tbody td {
  padding: 12px 14px; border-bottom: 1px solid var(--border-subtle);
  font-size: var(--text-sm); vertical-align: middle;
}
.target-row { transition: background var(--transition-fast); }
.target-row:hover { background: var(--bg-hover); }
.target-row:last-child td { border-bottom: none; }

.target-name-cell { display: flex; align-items: center; gap: 8px; }
.target-name { font-weight: 600; color: var(--text-primary); }

.upstream-url {
  font-family: var(--font-mono); font-size: var(--text-xs);
  background: rgba(99,102,241,0.08); color: var(--color-primary);
  padding: 2px 8px; border-radius: 4px; word-break: break-all;
}
.path-prefix {
  font-family: var(--font-mono); font-size: var(--text-xs);
  background: rgba(34,197,94,0.08); color: var(--color-success);
  padding: 2px 8px; border-radius: 4px;
}
.api-key-hdr {
  font-family: var(--font-mono); font-size: var(--text-xs);
  background: rgba(245,158,11,0.08); color: var(--color-warning);
  padding: 2px 8px; border-radius: 4px;
}
.no-value { color: var(--text-quaternary); font-size: var(--text-xs); }

.actions-cell { text-align: right; }
.action-btns { display: flex; gap: 4px; justify-content: flex-end; }

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
.modal-body { padding: 20px; }
.modal-footer {
  padding: 12px 20px; border-top: 1px solid var(--border-subtle);
  display: flex; justify-content: flex-end; gap: 8px;
}
.form-group { margin-bottom: 16px; }
.form-group label {
  display: block; font-size: var(--text-sm); color: var(--text-secondary);
  margin-bottom: 6px; font-weight: 500;
}
.input {
  width: 100%; padding: 8px 12px; background: var(--bg-base); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm);
  font-family: var(--font-mono);
}
.input:focus { outline: none; border-color: var(--color-primary); box-shadow: 0 0 0 2px var(--color-primary-dim, rgba(99,102,241,0.15)); }
.input-error { border-color: var(--color-danger) !important; }
.input-disabled { opacity: 0.6; cursor: not-allowed; }
.required { color: var(--color-danger); font-weight: 700; }
.form-error { color: var(--color-danger); font-size: var(--text-xs); margin-top: 4px; }
.form-hint { color: var(--text-tertiary); font-size: var(--text-xs); margin-top: 4px; }

@media (max-width: 768px) {
  .stat-row { grid-template-columns: 1fr; }
  .targets-table { font-size: var(--text-xs); }
  .targets-table thead th, .targets-table tbody td { padding: 8px 10px; }
}
</style>
