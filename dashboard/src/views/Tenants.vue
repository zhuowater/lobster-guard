<template>
  <div class="tenants-page">
    <div class="page-header">
      <h1 class="page-title">🏢 租户管理</h1>
      <p class="page-desc">安全域隔离 — 每个租户独立管理 Agent、规则和审计数据</p>
      <button class="btn-primary" @click="showCreateModal = true">+ 创建租户</button>
    </div>

    <div class="tenant-grid">
      <div class="tenant-card" v-for="t in tenants" :key="t.id" :class="{'card-disabled': !t.enabled}">
        <div class="card-header">
          <div class="card-title">
            <span class="card-icon">{{ t.strict_mode ? '🛡️' : '🏢' }}</span>
            <span>{{ t.name }}</span>
          </div>
          <span class="card-badge" :class="t.enabled ? 'badge-active' : 'badge-inactive'">
            {{ t.enabled ? '启用' : '停用' }}
          </span>
        </div>
        <div class="card-id">ID: {{ t.id }}</div>
        <div class="card-desc" v-if="t.description">{{ t.description }}</div>

        <div class="card-stats">
          <div class="stat-item">
            <span class="stat-value">{{ fmtNum(t.im_calls || 0) }}</span>
            <span class="stat-label">IM 调用</span>
          </div>
          <div class="stat-item">
            <span class="stat-value">{{ fmtNum(t.llm_calls || 0) }}</span>
            <span class="stat-label">LLM 调用</span>
          </div>
          <div class="stat-item">
            <span class="stat-value">{{ t.user_count || 0 }}</span>
            <span class="stat-label">用户</span>
          </div>
          <div class="stat-item">
            <span class="stat-value">{{ fmtNum(t.block_count || 0) }}</span>
            <span class="stat-label">拦截</span>
          </div>
        </div>

        <div class="card-meta">
          <span v-if="t.strict_mode" class="meta-tag tag-strict">严格模式</span>
          <span v-if="t.max_agents" class="meta-tag">Agent ≤{{ t.max_agents }}</span>
          <span v-if="t.max_rules" class="meta-tag">规则 ≤{{ t.max_rules }}</span>
        </div>

        <div class="card-actions">
          <button class="btn-sm btn-primary" @click="switchToTenant(t.id)">管理</button>
          <button class="btn-sm btn-outline" @click="openEdit(t)">编辑</button>
          <button class="btn-sm btn-danger" v-if="t.id !== 'default'" @click="confirmDelete(t)">删除</button>
        </div>
      </div>
    </div>

    <!-- 创建/编辑 Modal -->
    <Transition name="modal-fade">
      <div class="modal-overlay" v-if="showCreateModal || showEditModal" @click.self="closeModals">
        <div class="modal-content">
          <h3 class="modal-title">{{ showEditModal ? '编辑租户' : '创建租户' }}</h3>
          <div class="form-group" v-if="!showEditModal">
            <label>租户 ID</label>
            <input v-model="form.id" placeholder="如 security-team（创建后不可修改）" class="form-input" />
          </div>
          <div class="form-group">
            <label>名称</label>
            <input v-model="form.name" placeholder="如 安全团队" class="form-input" />
          </div>
          <div class="form-group">
            <label>描述</label>
            <input v-model="form.description" placeholder="可选描述" class="form-input" />
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>最大 Agent 数</label>
              <input v-model.number="form.max_agents" type="number" min="0" class="form-input" placeholder="0=无限" />
            </div>
            <div class="form-group">
              <label>最大规则数</label>
              <input v-model.number="form.max_rules" type="number" min="0" class="form-input" placeholder="0=无限" />
            </div>
          </div>
          <div class="form-group form-check">
            <label class="check-label">
              <input type="checkbox" v-model="form.strict_mode" />
              🛡️ 严格模式
            </label>
          </div>
          <div class="modal-actions">
            <button class="btn-outline" @click="closeModals">取消</button>
            <button class="btn-primary" @click="submitForm" :disabled="submitting">
              {{ submitting ? '提交中...' : (showEditModal ? '保存' : '创建') }}
            </button>
          </div>
          <div class="form-error" v-if="formError">{{ formError }}</div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup>
import { ref, onMounted, inject } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import { setTenant } from '../stores/app.js'
import { useRouter } from 'vue-router'

const router = useRouter()
const showToast = inject('showToast')
const tenants = ref([])
const showCreateModal = ref(false)
const showEditModal = ref(false)
const submitting = ref(false)
const formError = ref('')
const form = ref({ id: '', name: '', description: '', max_agents: 0, max_rules: 0, strict_mode: false })

async function loadTenants() {
  try {
    const d = await api('/api/v1/tenants')
    tenants.value = d.tenants || []
  } catch (e) {
    tenants.value = []
  }
}

function fmtNum(n) {
  if (n >= 10000) return (n / 1000).toFixed(1) + 'k'
  if (n >= 1000) return n.toLocaleString()
  return String(n)
}

function switchToTenant(id) {
  setTenant(id)
  router.push('/overview')
  setTimeout(() => router.go(0), 100) // refresh
}

function openEdit(t) {
  form.value = { id: t.id, name: t.name, description: t.description || '', max_agents: t.max_agents || 0, max_rules: t.max_rules || 0, strict_mode: t.strict_mode || false }
  formError.value = ''
  showEditModal.value = true
}

function closeModals() {
  showCreateModal.value = false
  showEditModal.value = false
  formError.value = ''
  form.value = { id: '', name: '', description: '', max_agents: 0, max_rules: 0, strict_mode: false }
}

async function submitForm() {
  formError.value = ''
  submitting.value = true
  try {
    if (showEditModal.value) {
      await apiPut('/api/v1/tenants/' + form.value.id, form.value)
      showToast('租户已更新')
    } else {
      if (!form.value.id || !form.value.name) {
        formError.value = 'ID 和名称必填'
        submitting.value = false
        return
      }
      await apiPost('/api/v1/tenants', { ...form.value, enabled: true })
      showToast('租户已创建')
    }
    closeModals()
    await loadTenants()
  } catch (e) {
    formError.value = e.message || '操作失败'
  } finally {
    submitting.value = false
  }
}

async function confirmDelete(t) {
  if (!confirm(`确定要删除租户 "${t.name}" (${t.id}) 吗？\n\n⚠️ 删除后该租户的数据不会被自动清除，但将无法通过 Dashboard 访问。`)) return
  try {
    await apiDelete('/api/v1/tenants/' + t.id)
    showToast('租户已删除')
    await loadTenants()
  } catch (e) {
    alert('删除失败: ' + e.message)
  }
}

onMounted(loadTenants)
</script>

<style scoped>
.tenants-page { padding: var(--space-6); max-width: 1200px; }
.page-header { margin-bottom: var(--space-6); display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-4); }
.page-title { font-size: var(--text-xl); font-weight: 700; color: var(--text-primary); margin: 0; }
.page-desc { flex: 1; font-size: var(--text-sm); color: var(--text-tertiary); min-width: 200px; margin: 0; }

.tenant-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: var(--space-4); }

.tenant-card {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-5);
  transition: all var(--transition-fast); display: flex; flex-direction: column; gap: var(--space-3);
}
.tenant-card:hover { border-color: var(--color-primary); box-shadow: var(--shadow-md); }
.card-disabled { opacity: 0.6; }

.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-title { display: flex; align-items: center; gap: var(--space-2); font-weight: 700; font-size: var(--text-base); color: var(--text-primary); }
.card-icon { font-size: 1.2em; }
.card-badge {
  font-size: 10px; font-weight: 700; padding: 2px 8px; border-radius: 12px;
  text-transform: uppercase; letter-spacing: 0.05em;
}
.badge-active { background: rgba(16, 185, 129, 0.15); color: #10B981; }
.badge-inactive { background: rgba(239, 68, 68, 0.15); color: #EF4444; }

.card-id { font-family: var(--font-mono); font-size: var(--text-xs); color: var(--text-tertiary); }
.card-desc { font-size: var(--text-xs); color: var(--text-secondary); line-height: 1.5; }

.card-stats {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-2);
  padding: var(--space-3) 0; border-top: 1px solid var(--border-subtle); border-bottom: 1px solid var(--border-subtle);
}
.stat-item { text-align: center; }
.stat-value { display: block; font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: 10px; color: var(--text-tertiary); }

.card-meta { display: flex; flex-wrap: wrap; gap: var(--space-1); }
.meta-tag {
  font-size: 10px; padding: 2px 6px; border-radius: var(--radius-sm);
  background: var(--bg-elevated); color: var(--text-secondary); border: 1px solid var(--border-subtle);
}
.tag-strict { background: rgba(239, 68, 68, 0.1); color: #EF4444; border-color: rgba(239, 68, 68, 0.3); }

.card-actions { display: flex; gap: var(--space-2); margin-top: auto; }

/* Buttons */
.btn-primary {
  background: var(--color-primary); color: #fff; border: none; padding: 8px 16px;
  border-radius: var(--radius-md); font-size: var(--text-sm); font-weight: 600;
  cursor: pointer; transition: all var(--transition-fast);
}
.btn-primary:hover { opacity: 0.9; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-outline {
  background: transparent; color: var(--text-secondary); border: 1px solid var(--border-default);
  padding: 8px 16px; border-radius: var(--radius-md); font-size: var(--text-sm);
  cursor: pointer; transition: all var(--transition-fast);
}
.btn-outline:hover { border-color: var(--color-primary); color: var(--text-primary); }
.btn-sm { padding: 4px 12px; font-size: var(--text-xs); }
.btn-danger {
  background: transparent; color: #EF4444; border: 1px solid rgba(239, 68, 68, 0.3);
  padding: 4px 12px; border-radius: var(--radius-md); font-size: var(--text-xs);
  cursor: pointer; transition: all var(--transition-fast);
}
.btn-danger:hover { background: rgba(239, 68, 68, 0.1); }

/* Modal */
.modal-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center;
  z-index: 500;
}
.modal-content {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-6); width: 480px; max-width: 90vw;
  box-shadow: var(--shadow-lg);
}
.modal-title { font-size: var(--text-lg); font-weight: 700; color: var(--text-primary); margin: 0 0 var(--space-4) 0; }
.form-group { margin-bottom: var(--space-3); }
.form-group label { display: block; font-size: var(--text-xs); font-weight: 600; color: var(--text-secondary); margin-bottom: 4px; }
.form-input {
  width: 100%; padding: 8px 12px; background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm);
  outline: none; box-sizing: border-box; font-family: var(--font-sans);
}
.form-input:focus { border-color: var(--color-primary); }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); }
.form-check { padding-top: var(--space-1); }
.check-label { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); color: var(--text-secondary); cursor: pointer; }
.modal-actions { display: flex; justify-content: flex-end; gap: var(--space-2); margin-top: var(--space-4); }
.form-error { margin-top: var(--space-2); font-size: var(--text-xs); color: #EF4444; }

.modal-fade-enter-active { transition: all .2s ease; }
.modal-fade-leave-active { transition: all .2s ease; }
.modal-fade-enter-from, .modal-fade-leave-to { opacity: 0; }
.modal-fade-enter-from .modal-content { transform: scale(0.95); }
</style>
