<template>
  <div class="users-page">
    <div class="page-header">
      <div class="page-header-left">
        <h1 class="page-title">👥 用户管理</h1>
        <p class="page-desc">管理系统用户账号、角色和权限</p>
      </div>
      <button class="btn-primary" @click="openCreate">
        <Icon name="plus" :size="14" /> 创建用户
      </button>
    </div>

    <!-- 搜索栏 -->
    <div class="search-bar">
      <Icon name="search" :size="16" class="search-icon" />
      <input
        v-model="searchQuery"
        class="search-input"
        placeholder="搜索用户名、显示名..."
      />
      <select v-model="filterRole" class="filter-select">
        <option value="">全部角色</option>
        <option value="admin">管理员</option>
        <option value="operator">操作员</option>
        <option value="viewer">观察者</option>
      </select>
    </div>

    <!-- 数据表格 -->
    <DataTable
      :columns="columns"
      :data="filteredUsers"
      :loading="loading"
      :page-size="20"
      row-key="id"
      empty-text="暂无用户"
      empty-desc="点击「创建用户」添加第一个用户"
    >
      <template #cell-username="{ row }">
        <div class="cell-user">
          <div class="user-avatar" :class="'avatar-' + row.role">
            {{ (row.display_name || row.username || '?')[0].toUpperCase() }}
          </div>
          <div class="user-info">
            <span class="user-name">{{ row.username }}</span>
            <span class="user-display" v-if="row.display_name">{{ row.display_name }}</span>
          </div>
        </div>
      </template>

      <template #cell-role="{ row }">
        <span class="role-tag" :class="'role-' + row.role">{{ roleLabel(row.role) }}</span>
      </template>

      <template #cell-tenant_id="{ row }">
        <span class="tenant-tag" v-if="row.tenant_id">{{ row.tenant_id }}</span>
        <span class="text-muted" v-else>—</span>
      </template>

      <template #cell-enabled="{ row }">
        <label class="toggle-switch-sm" @click.stop>
          <input type="checkbox" :checked="row.enabled" @change="toggleEnabled(row)" />
          <span class="toggle-slider-sm" :class="{ 'toggle-active-sm': row.enabled }"></span>
        </label>
      </template>

      <template #cell-created_at="{ row }">
        <span class="text-mono text-muted">{{ fmtTime(row.created_at) }}</span>
      </template>

      <template #actions="{ row }">
        <div class="action-btns">
          <button class="btn-action" @click="openEdit(row)" title="编辑">
            <Icon name="edit" :size="14" />
          </button>
          <button class="btn-action" @click="openResetPwd(row)" title="重置密码">
            <Icon name="key" :size="14" />
          </button>
          <button class="btn-action btn-action-danger" @click="openDelete(row)" title="删除">
            <Icon name="trash" :size="14" />
          </button>
        </div>
      </template>
    </DataTable>

    <!-- 创建/编辑用户弹窗 -->
    <Transition name="modal-fade">
      <div class="modal-overlay" v-if="showUserModal" @click.self="closeUserModal">
        <div class="modal-content">
          <h3 class="modal-title">{{ isEditing ? '编辑用户' : '创建用户' }}</h3>

          <div class="form-group" v-if="!isEditing">
            <label>用户名 <span class="required">*</span></label>
            <input v-model="userForm.username" placeholder="登录用户名" class="form-input" autocomplete="off" />
          </div>
          <div class="form-group" v-if="!isEditing">
            <label>密码 <span class="required">*</span></label>
            <input v-model="userForm.password" type="password" placeholder="至少 6 位" class="form-input" autocomplete="new-password" />
          </div>
          <div class="form-group">
            <label>显示名</label>
            <input v-model="userForm.display_name" placeholder="可选，如 张三" class="form-input" />
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>角色 <span class="required">*</span></label>
              <select v-model="userForm.role" class="form-input">
                <option value="admin">管理员 (admin)</option>
                <option value="operator">操作员 (operator)</option>
                <option value="viewer">观察者 (viewer)</option>
              </select>
            </div>
            <div class="form-group">
              <label>所属租户</label>
              <select v-model="userForm.tenant_id" class="form-input">
                <option value="">无（全局）</option>
                <option v-for="t in tenants" :key="t.id" :value="t.id">{{ t.name || t.id }}</option>
              </select>
            </div>
          </div>
          <div class="form-group form-check">
            <label class="check-label">
              <input type="checkbox" v-model="userForm.enabled" />
              启用账号
            </label>
          </div>

          <div class="modal-actions">
            <button class="btn-outline" @click="closeUserModal">取消</button>
            <button class="btn-primary" @click="submitUser" :disabled="submitting">
              {{ submitting ? '提交中...' : (isEditing ? '保存' : '创建') }}
            </button>
          </div>
          <div class="form-error" v-if="formError">{{ formError }}</div>
        </div>
      </div>
    </Transition>

    <!-- 重置密码弹窗 -->
    <Transition name="modal-fade">
      <div class="modal-overlay" v-if="showPwdModal" @click.self="closePwdModal">
        <div class="modal-content modal-sm">
          <h3 class="modal-title">🔑 重置密码</h3>
          <p class="modal-desc">为用户 <strong>{{ pwdTarget?.username }}</strong> 设置新密码</p>

          <div class="form-group">
            <label>新密码 <span class="required">*</span></label>
            <input v-model="newPassword" type="password" placeholder="至少 6 位" class="form-input" autocomplete="new-password" />
          </div>
          <div class="form-group">
            <label>确认密码 <span class="required">*</span></label>
            <input v-model="confirmPassword" type="password" placeholder="再次输入" class="form-input" autocomplete="new-password" />
          </div>

          <div class="modal-actions">
            <button class="btn-outline" @click="closePwdModal">取消</button>
            <button class="btn-primary" @click="submitResetPwd" :disabled="submitting">
              {{ submitting ? '重置中...' : '重置密码' }}
            </button>
          </div>
          <div class="form-error" v-if="pwdError">{{ pwdError }}</div>
        </div>
      </div>
    </Transition>

    <!-- 删除确认弹窗 -->
    <ConfirmModal
      :visible="showDeleteModal"
      title="删除用户"
      :message="'确定要删除用户「' + (deleteTarget?.username || '') + '」吗？此操作不可恢复。'"
      type="danger"
      confirm-text="删除"
      @confirm="confirmDeleteUser"
      @cancel="showDeleteModal = false"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, inject } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Icon from '../components/Icon.vue'

const showToast = inject('showToast')

// === 数据状态 ===
const loading = ref(false)
const users = ref([])
const tenants = ref([])
const searchQuery = ref('')
const filterRole = ref('')

// === 表格列定义 ===
const columns = [
  { key: 'username', label: '用户', sortable: true },
  { key: 'role', label: '角色', sortable: true, width: '120px' },
  { key: 'tenant_id', label: '所属租户', sortable: true, width: '140px' },
  { key: 'enabled', label: '状态', width: '80px' },
  { key: 'created_at', label: '创建时间', sortable: true, width: '170px' },
]

// === 过滤后的用户 ===
const filteredUsers = computed(() => {
  let list = users.value
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(u =>
      (u.username || '').toLowerCase().includes(q) ||
      (u.display_name || '').toLowerCase().includes(q)
    )
  }
  if (filterRole.value) {
    list = list.filter(u => u.role === filterRole.value)
  }
  return list
})

// === 创建/编辑弹窗 ===
const showUserModal = ref(false)
const isEditing = ref(false)
const editingId = ref(null)
const submitting = ref(false)
const formError = ref('')
const userForm = ref(defaultForm())

function defaultForm() {
  return { username: '', password: '', display_name: '', role: 'viewer', tenant_id: '', enabled: true }
}

function openCreate() {
  isEditing.value = false
  editingId.value = null
  userForm.value = defaultForm()
  formError.value = ''
  showUserModal.value = true
}

function openEdit(row) {
  isEditing.value = true
  editingId.value = row.id
  userForm.value = {
    username: row.username,
    password: '',
    display_name: row.display_name || '',
    role: row.role || 'viewer',
    tenant_id: row.tenant_id || '',
    enabled: row.enabled !== false,
  }
  formError.value = ''
  showUserModal.value = true
}

function closeUserModal() {
  showUserModal.value = false
  formError.value = ''
}

async function submitUser() {
  formError.value = ''
  if (!isEditing.value) {
    if (!userForm.value.username.trim()) { formError.value = '用户名不能为空'; return }
    if (!userForm.value.password || userForm.value.password.length < 6) { formError.value = '密码至少 6 位'; return }
  }
  if (!userForm.value.role) { formError.value = '请选择角色'; return }

  submitting.value = true
  try {
    if (isEditing.value) {
      await apiPut('/api/v1/auth/users/' + editingId.value, {
        display_name: userForm.value.display_name,
        role: userForm.value.role,
        tenant_id: userForm.value.tenant_id,
        enabled: userForm.value.enabled,
      })
      showToast('用户已更新')
    } else {
      await apiPost('/api/v1/auth/users', {
        username: userForm.value.username.trim(),
        password: userForm.value.password,
        display_name: userForm.value.display_name,
        role: userForm.value.role,
        tenant_id: userForm.value.tenant_id,
        enabled: userForm.value.enabled,
      })
      showToast('用户已创建')
    }
    closeUserModal()
    await loadUsers()
  } catch (e) {
    formError.value = e.message || '操作失败'
  } finally {
    submitting.value = false
  }
}

// === 重置密码弹窗 ===
const showPwdModal = ref(false)
const pwdTarget = ref(null)
const newPassword = ref('')
const confirmPassword = ref('')
const pwdError = ref('')

function openResetPwd(row) {
  pwdTarget.value = row
  newPassword.value = ''
  confirmPassword.value = ''
  pwdError.value = ''
  showPwdModal.value = true
}

function closePwdModal() {
  showPwdModal.value = false
  pwdError.value = ''
}

async function submitResetPwd() {
  pwdError.value = ''
  if (!newPassword.value || newPassword.value.length < 6) { pwdError.value = '密码至少 6 位'; return }
  if (newPassword.value !== confirmPassword.value) { pwdError.value = '两次密码不一致'; return }

  submitting.value = true
  try {
    await apiPut('/api/v1/auth/users/' + pwdTarget.value.id, {
      password: newPassword.value,
    })
    showToast('密码已重置')
    closePwdModal()
  } catch (e) {
    pwdError.value = e.message || '重置失败'
  } finally {
    submitting.value = false
  }
}

// === 删除弹窗 ===
const showDeleteModal = ref(false)
const deleteTarget = ref(null)

function openDelete(row) {
  deleteTarget.value = row
  showDeleteModal.value = true
}

async function confirmDeleteUser() {
  showDeleteModal.value = false
  try {
    await apiDelete('/api/v1/auth/users/' + deleteTarget.value.id)
    showToast('用户已删除')
    await loadUsers()
  } catch (e) {
    showToast('删除失败: ' + e.message)
  }
}

// === 启用/禁用快速切换 ===
async function toggleEnabled(row) {
  try {
    await apiPut('/api/v1/auth/users/' + row.id, { enabled: !row.enabled })
    row.enabled = !row.enabled
    showToast(row.enabled ? '已启用' : '已禁用')
  } catch (e) {
    showToast('操作失败: ' + e.message)
  }
}

// === 辅助函数 ===
function roleLabel(role) {
  switch (role) {
    case 'admin': return '管理员'
    case 'operator': return '操作员'
    case 'viewer': return '观察者'
    default: return role || '--'
  }
}

function fmtTime(ts) {
  if (!ts) return '--'
  try {
    const d = new Date(ts)
    if (isNaN(d.getTime())) return ts
    return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  } catch { return ts }
}

// === 数据加载 ===
async function loadUsers() {
  loading.value = true
  try {
    const d = await api('/api/v1/auth/users')
    users.value = d.users || d || []
  } catch (e) {
    users.value = []
  } finally {
    loading.value = false
  }
}

async function loadTenants() {
  try {
    const d = await api('/api/v1/tenants')
    tenants.value = d.tenants || []
  } catch {
    tenants.value = []
  }
}

onMounted(() => {
  loadUsers()
  loadTenants()
})
</script>

<style scoped>
.users-page { padding: var(--space-6); max-width: 1200px; }

/* Header */
.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: var(--space-5); gap: var(--space-4);
}
.page-header-left { flex: 1; }
.page-title { font-size: var(--text-xl); font-weight: 700; color: var(--text-primary); margin: 0; }
.page-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin: var(--space-1) 0 0 0; }

/* Search */
.search-bar {
  display: flex; align-items: center; gap: var(--space-3);
  margin-bottom: var(--space-4); position: relative;
}
.search-icon {
  position: absolute; left: 12px; color: var(--text-tertiary); pointer-events: none;
}
.search-input {
  flex: 1; padding: 8px 12px 8px 36px;
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm);
  outline: none; font-family: var(--font-sans); transition: border-color var(--transition-fast);
}
.search-input:focus { border-color: var(--color-primary); }
.search-input::placeholder { color: var(--text-tertiary); }
.filter-select {
  padding: 8px 12px; background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm);
  outline: none; font-family: var(--font-sans); min-width: 130px; cursor: pointer;
}
.filter-select:focus { border-color: var(--color-primary); }
.filter-select option { background: var(--bg-elevated); }

/* User cell */
.cell-user { display: flex; align-items: center; gap: var(--space-3); }
.user-avatar {
  width: 32px; height: 32px; border-radius: 50%; display: flex; align-items: center; justify-content: center;
  font-size: var(--text-sm); font-weight: 700; color: #fff; flex-shrink: 0;
  background: var(--color-primary);
}
.avatar-admin { background: #EF4444; }
.avatar-operator { background: #6366F1; }
.avatar-viewer { background: #6B7280; }
.user-info { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
.user-name { font-weight: 600; color: var(--text-primary); font-size: var(--text-sm); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.user-display { font-size: var(--text-xs); color: var(--text-tertiary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

/* Role tag */
.role-tag {
  display: inline-flex; align-items: center; padding: 2px 10px;
  border-radius: 12px; font-size: 11px; font-weight: 600;
  letter-spacing: 0.03em;
}
.role-admin { background: rgba(239, 68, 68, 0.15); color: #EF4444; }
.role-operator { background: rgba(99, 102, 241, 0.15); color: #818CF8; }
.role-viewer { background: rgba(107, 114, 128, 0.15); color: #9CA3AF; }

/* Tenant tag */
.tenant-tag {
  display: inline-flex; align-items: center; padding: 2px 8px;
  background: var(--bg-elevated); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm); font-size: var(--text-xs); color: var(--text-secondary);
  font-family: var(--font-mono);
}

/* Toggle switch (small) */
.toggle-switch-sm { position: relative; display: inline-block; width: 32px; height: 18px; cursor: pointer; }
.toggle-switch-sm input { opacity: 0; width: 0; height: 0; position: absolute; }
.toggle-slider-sm {
  position: absolute; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(255,255,255,0.1); border-radius: 18px; transition: all .25s;
}
.toggle-slider-sm::before {
  content: ''; position: absolute; height: 14px; width: 14px; left: 2px; bottom: 2px;
  background: #fff; border-radius: 50%; transition: all .25s;
}
.toggle-active-sm { background: #10B981; }
.toggle-active-sm::before { transform: translateX(14px); }

/* Action buttons */
.action-btns { display: flex; gap: var(--space-1); }
.btn-action {
  background: transparent; border: 1px solid transparent; padding: 4px 6px;
  border-radius: var(--radius-sm); color: var(--text-tertiary); cursor: pointer;
  transition: all var(--transition-fast); display: flex; align-items: center;
}
.btn-action:hover { background: var(--bg-elevated); color: var(--text-primary); border-color: var(--border-subtle); }
.btn-action-danger:hover { color: #EF4444; border-color: rgba(239, 68, 68, 0.3); background: rgba(239, 68, 68, 0.08); }

/* Text helpers */
.text-mono { font-family: var(--font-mono); }
.text-muted { color: var(--text-tertiary); font-size: var(--text-xs); }

/* Buttons (consistent with Tenants.vue) */
.btn-primary {
  background: var(--color-primary); color: #fff; border: none; padding: 8px 16px;
  border-radius: var(--radius-md); font-size: var(--text-sm); font-weight: 600;
  cursor: pointer; transition: all var(--transition-fast);
  display: inline-flex; align-items: center; gap: var(--space-2);
}
.btn-primary:hover { opacity: 0.9; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-outline {
  background: transparent; color: var(--text-secondary); border: 1px solid var(--border-default);
  padding: 8px 16px; border-radius: var(--radius-md); font-size: var(--text-sm);
  cursor: pointer; transition: all var(--transition-fast);
}
.btn-outline:hover { border-color: var(--color-primary); color: var(--text-primary); }

/* Modal */
.modal-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center;
  z-index: 500;
}
.modal-content {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-6); width: 520px; max-width: 90vw;
  box-shadow: var(--shadow-lg);
}
.modal-sm { width: 420px; }
.modal-title { font-size: var(--text-lg); font-weight: 700; color: var(--text-primary); margin: 0 0 var(--space-4) 0; }
.modal-desc { font-size: var(--text-sm); color: var(--text-secondary); margin: 0 0 var(--space-4) 0; line-height: 1.5; }
.modal-desc strong { color: var(--text-primary); font-weight: 600; }
.form-group { margin-bottom: var(--space-3); }
.form-group label { display: block; font-size: var(--text-xs); font-weight: 600; color: var(--text-secondary); margin-bottom: 4px; }
.required { color: #EF4444; }
.form-input {
  width: 100%; padding: 8px 12px; background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm);
  outline: none; box-sizing: border-box; font-family: var(--font-sans); transition: border-color var(--transition-fast);
}
.form-input:focus { border-color: var(--color-primary); }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); }
.form-check { padding-top: var(--space-1); }
.check-label { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); color: var(--text-secondary); cursor: pointer; }
.check-label input[type="checkbox"] { accent-color: var(--color-primary); }
.modal-actions { display: flex; justify-content: flex-end; gap: var(--space-2); margin-top: var(--space-4); }
.form-error { margin-top: var(--space-2); font-size: var(--text-xs); color: #EF4444; }

/* Transitions */
.modal-fade-enter-active { transition: all .2s ease; }
.modal-fade-leave-active { transition: all .2s ease; }
.modal-fade-enter-from, .modal-fade-leave-to { opacity: 0; }
.modal-fade-enter-from .modal-content { transform: scale(0.95); }
</style>
