<template>
  <div class="users-page">
    <div class="page-header">
      <div class="page-header-left">
        <h1 class="page-title"><Icon name="users" :size="20" /> 用户管理</h1>
        <p class="page-desc">管理系统用户账号、角色和权限</p>
      </div>
      <div class="page-header-right">
        <div class="batch-wrapper" v-if="selectedIds.size > 0">
          <button class="btn-outline btn-batch" @click="showBatchMenu = !showBatchMenu">
            <Icon name="check-square" :size="14" /> 批量操作 ({{ selectedIds.size }}) ▾
          </button>
          <div class="batch-menu" v-if="showBatchMenu">
            <button class="batch-item" @click="batchEnable(true)">✓ 批量启用</button>
            <button class="batch-item" @click="batchEnable(false)">✗ 批量禁用</button>
            <div class="batch-divider"></div>
            <button class="batch-item batch-danger" @click="openBatchDelete">🗑 批量删除</button>
          </div>
        </div>
        <button class="btn-primary" @click="openCreate"><Icon name="plus" :size="14" /> 创建用户</button>
      </div>
    </div>

    <div class="search-bar">
      <Icon name="search" :size="16" class="search-icon" />
      <input v-model="searchQuery" class="search-input" placeholder="搜索用户名、邮箱、显示名..." />
      <select v-model="filterRole" class="filter-select"><option value="">全部角色</option><option value="admin">管理员</option><option value="operator">操作员</option><option value="viewer">观察者</option></select>
      <select v-model="filterStatus" class="filter-select"><option value="">全部状态</option><option value="enabled">已启用</option><option value="disabled">已禁用</option></select>
      <select v-model="sortField" class="filter-select"><option value="">默认排序</option><option value="last_login">最后登录</option><option value="created_at">创建时间</option><option value="username">用户名</option></select>
    </div>

    <Transition name="fade">
      <div class="role-legend" v-if="showRoleLegend">
        <div class="role-legend-item"><span class="role-tag role-admin">管理员</span><span class="rld">全部权限 — 用户/租户/系统配置/安全策略</span></div>
        <div class="role-legend-item"><span class="role-tag role-operator">操作员</span><span class="rld">操作权限 — 事件处理/规则/告警响应</span></div>
        <div class="role-legend-item"><span class="role-tag role-viewer">观察者</span><span class="rld">只读权限 — 仪表盘/事件/日志</span></div>
        <button class="role-legend-close" @click="showRoleLegend = false">✕</button>
      </div>
    </Transition>

    <DataTable :columns="columns" :data="filteredUsers" :loading="loading" :page-size="20" row-key="id" empty-text="暂无用户" empty-desc="点击「创建用户」添加第一个用户">
      <template #toolbar><button class="btn-ghost btn-sm" @click="showRoleLegend = !showRoleLegend"><Icon name="info" :size="14" /> 角色说明</button></template>
      <template #cell-checkbox="{ row }"><label class="checkbox-wrap" @click.stop><input type="checkbox" :checked="selectedIds.has(row.id)" @change="toggleSelect(row.id)" /><span class="checkbox-mark"></span></label></template>
      <template #cell-username="{ row }">
        <div class="cell-user">
          <div class="user-avatar" :class="'avatar-' + row.role">{{ (row.display_name || row.username || '?')[0].toUpperCase() }}</div>
          <div class="user-info"><span class="user-name">{{ row.username }}</span><span class="user-display" v-if="row.display_name">{{ row.display_name }}</span></div>
        </div>
      </template>
      <template #cell-role="{ row }"><span class="role-tag" :class="'role-' + row.role">{{ roleLabel(row.role) }}</span></template>
      <template #cell-tenant_id="{ row }"><span class="tenant-tag" v-if="row.tenant_id">{{ tenantName(row.tenant_id) }}</span><span class="text-muted" v-else>全局</span></template>
      <template #cell-enabled="{ row }"><label class="toggle-switch-sm" @click.stop><input type="checkbox" :checked="row.enabled" @change="toggleEnabled(row)" /><span class="toggle-slider-sm" :class="{ 'toggle-active-sm': row.enabled }"></span></label></template>
      <template #cell-last_login="{ row }"><span class="text-mono text-muted" v-if="row.last_login">{{ fmtTime(row.last_login) }}</span><span class="text-muted" v-else>从未登录</span></template>
      <template #cell-created_at="{ row }"><span class="text-mono text-muted">{{ fmtTime(row.created_at) }}</span></template>
      <template #actions="{ row }">
        <div class="action-btns">
          <button class="btn-action" @click="openEdit(row)" title="编辑"><Icon name="edit" :size="14" /></button>
          <button class="btn-action" @click="openResetPwd(row)" title="重置密码"><Icon name="key" :size="14" /></button>
          <button class="btn-action" @click="openAuditLog(row)" title="操作历史"><Icon name="file-text" :size="14" /></button>
          <button class="btn-action btn-action-danger" @click="openDelete(row)" title="删除"><Icon name="trash" :size="14" /></button>
        </div>
      </template>
    </DataTable>

    <!-- Create/Edit Modal -->
    <Transition name="modal-fade">
      <div class="modal-overlay" v-if="showUserModal" @click.self="closeUserModal">
        <div class="modal-content">
          <h3 class="modal-title">{{ isEditing ? '编辑用户' : '创建用户' }}</h3>
          <div class="form-group" v-if="!isEditing">
            <label>用户名 <span class="required">*</span></label>
            <input v-model="userForm.username" placeholder="3-32位字母数字下划线" class="form-input" :class="{'input-error': validationErrors.username}" autocomplete="off" @input="clearValidation('username')" />
            <div class="field-error" v-if="validationErrors.username">{{ validationErrors.username }}</div>
          </div>
          <div class="form-group" v-if="!isEditing">
            <label>密码 <span class="required">*</span></label>
            <div class="password-field">
              <input v-model="userForm.password" :type="showPwdCreate ? 'text' : 'password'" placeholder="至少8位含大小写和数字" class="form-input" :class="{'input-error': validationErrors.password}" autocomplete="new-password" @input="clearValidation('password')" />
              <button type="button" class="pwd-toggle" @click="showPwdCreate = !showPwdCreate">{{ showPwdCreate ? '隐' : '显' }}</button>
              <button type="button" class="pwd-generate" @click="generatePassword" title="随机密码">🎲</button>
            </div>
            <div class="password-strength" v-if="userForm.password">
              <div class="strength-bar"><div class="strength-fill" :class="'str-' + passwordStrength.level" :style="{width: passwordStrength.percent + '%'}"></div></div>
              <span class="strength-text" :class="'str-' + passwordStrength.level">{{ passwordStrength.text }}</span>
            </div>
            <div class="field-error" v-if="validationErrors.password">{{ validationErrors.password }}</div>
          </div>
          <div class="form-group"><label>邮箱</label><input v-model="userForm.email" placeholder="user@example.com" class="form-input" :class="{'input-error': validationErrors.email}" @input="clearValidation('email')" /><div class="field-error" v-if="validationErrors.email">{{ validationErrors.email }}</div></div>
          <div class="form-group"><label>显示名</label><input v-model="userForm.display_name" placeholder="可选" class="form-input" /></div>
          <div class="form-row">
            <div class="form-group"><label>角色 <span class="required">*</span></label><select v-model="userForm.role" class="form-input"><option value="admin">管理员</option><option value="operator">操作员</option><option value="viewer">观察者</option></select></div>
            <div class="form-group"><label>所属租户</label><select v-model="userForm.tenant_id" class="form-input"><option value="">无（全局）</option><option v-for="t in tenants" :key="t.id" :value="t.id">{{ t.name || t.id }}</option></select></div>
          </div>
          <div class="form-group form-check"><label class="check-label"><input type="checkbox" v-model="userForm.enabled" /> 启用账号</label></div>
          <div class="modal-actions"><button class="btn-outline" @click="closeUserModal">取消</button><button class="btn-primary" @click="submitUser" :disabled="submitting">{{ submitting ? '提交中...' : (isEditing ? '保存' : '创建') }}</button></div>
          <div class="form-error" v-if="formError">{{ formError }}</div>
        </div>
      </div>
    </Transition>

    <!-- Reset Password Modal -->
    <Transition name="modal-fade">
      <div class="modal-overlay" v-if="showPwdModal" @click.self="closePwdModal">
        <div class="modal-content modal-sm">
          <h3 class="modal-title"><Icon name="key" :size="16" /> 重置密码</h3>
          <p class="modal-desc">为 <strong>{{ pwdTarget?.username }}</strong> 设置新密码</p>
          <div class="pwd-mode-switch">
            <button :class="['pwd-mode-btn', {active: pwdMode==='manual'}]" @click="pwdMode='manual'">手动输入</button>
            <button :class="['pwd-mode-btn', {active: pwdMode==='random'}]" @click="pwdMode='random';generateResetPassword()">随机生成</button>
          </div>
          <template v-if="pwdMode==='manual'">
            <div class="form-group"><label>新密码 <span class="required">*</span></label><div class="password-field"><input v-model="newPassword" :type="showPwdReset?'text':'password'" placeholder="至少8位" class="form-input" autocomplete="new-password" /><button type="button" class="pwd-toggle" @click="showPwdReset=!showPwdReset">{{ showPwdReset?'隐':'显' }}</button></div></div>
            <div class="form-group"><label>确认密码 <span class="required">*</span></label><input v-model="confirmPassword" type="password" placeholder="再次输入" class="form-input" autocomplete="new-password" /></div>
          </template>
          <template v-else>
            <div class="form-group"><label>已生成密码</label><div class="generated-pwd"><code class="pwd-display">{{ generatedPwd }}</code><button class="btn-copy" @click="copyToClipboard(generatedPwd)">📋</button><button class="btn-copy" @click="generateResetPassword">🔄</button></div><div class="field-hint">⚠️ 请通过安全渠道告知用户</div></div>
          </template>
          <div class="modal-actions"><button class="btn-outline" @click="closePwdModal">取消</button><button class="btn-primary" @click="submitResetPwd" :disabled="submitting">{{ submitting ? '重置中...' : '重置密码' }}</button></div>
          <div class="form-error" v-if="pwdError">{{ pwdError }}</div>
        </div>
      </div>
    </Transition>

    <!-- Audit Modal -->
    <Transition name="modal-fade">
      <div class="modal-overlay" v-if="showAuditModal" @click.self="showAuditModal=false">
        <div class="modal-content modal-lg">
          <h3 class="modal-title"><Icon name="file-text" :size="16" /> 操作历史 — {{ auditTarget?.username }}</h3>
          <div v-if="auditLoading" class="audit-loading">加载中...</div>
          <div v-else-if="auditEntries.length===0" class="audit-empty">暂无操作记录</div>
          <div v-else class="audit-list">
            <div class="audit-item" v-for="e in auditEntries" :key="e.id">
              <div class="audit-time">{{ fmtTime(e.timestamp) }}</div>
              <span class="audit-action-tag" :class="'audit-'+auditActionType(e.action)">{{ e.action }}</span>
              <div class="audit-detail">{{ e.detail }}</div>
              <div class="audit-ip" v-if="e.ip">IP: {{ e.ip }}</div>
            </div>
          </div>
          <div class="modal-actions"><button class="btn-outline" @click="showAuditModal=false">关闭</button></div>
        </div>
      </div>
    </Transition>

    <ConfirmModal :visible="showDeleteModal" title="删除用户" :message="deleteMessage" type="danger" confirm-text="删除" @confirm="confirmDeleteAction" @cancel="showDeleteModal=false" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, inject } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Icon from '../components/Icon.vue'

const showToast = inject('showToast')
const loading = ref(false), users = ref([]), tenants = ref([])
const searchQuery = ref(''), filterRole = ref(''), filterStatus = ref(''), sortField = ref('')
const showRoleLegend = ref(false), selectedIds = ref(new Set()), showBatchMenu = ref(false), batchDeleteMode = ref(false)

function toggleSelect(id) { const s = new Set(selectedIds.value); if (s.has(id)) s.delete(id); else s.add(id); selectedIds.value = s }

const columns = [
  { key: 'checkbox', label: '', width: '40px' },
  { key: 'username', label: '用户', sortable: true },
  { key: 'role', label: '角色', sortable: true, width: '120px' },
  { key: 'tenant_id', label: '所属租户', sortable: true, width: '140px' },
  { key: 'enabled', label: '状态', width: '80px' },
  { key: 'last_login', label: '最后登录', sortable: true, width: '170px' },
  { key: 'created_at', label: '创建时间', sortable: true, width: '170px' },
]

const filteredUsers = computed(() => {
  let list = users.value
  if (searchQuery.value) { const q = searchQuery.value.toLowerCase(); list = list.filter(u => (u.username||'').toLowerCase().includes(q) || (u.display_name||'').toLowerCase().includes(q) || (u.email||'').toLowerCase().includes(q)) }
  if (filterRole.value) list = list.filter(u => u.role === filterRole.value)
  if (filterStatus.value) list = list.filter(u => filterStatus.value === 'enabled' ? u.enabled : !u.enabled)
  if (sortField.value) { list = [...list].sort((a, b) => { const va = a[sortField.value]||'', vb = b[sortField.value]||''; return (sortField.value === 'last_login' || sortField.value === 'created_at') ? (vb||'').localeCompare(va||'') : (va||'').localeCompare(vb||'') }) }
  return list
})

const showUserModal = ref(false), isEditing = ref(false), editingId = ref(null), submitting = ref(false), formError = ref(''), showPwdCreate = ref(false), validationErrors = ref({})
const userForm = ref(defaultForm())
function defaultForm() { return { username: '', password: '', display_name: '', email: '', role: 'viewer', tenant_id: '', enabled: true } }
function clearValidation(f) { const v = { ...validationErrors.value }; delete v[f]; validationErrors.value = v }

const passwordStrength = computed(() => {
  const p = userForm.value.password || ''; if (!p) return { level: 'none', text: '', percent: 0 }
  let s = 0; if (p.length >= 8) s++; if (p.length >= 12) s++; if (/[a-z]/.test(p)) s++; if (/[A-Z]/.test(p)) s++; if (/[0-9]/.test(p)) s++; if (/[^a-zA-Z0-9]/.test(p)) s++
  if (s <= 2) return { level: 'weak', text: '弱', percent: 25 }; if (s <= 3) return { level: 'fair', text: '一般', percent: 50 }
  if (s <= 4) return { level: 'good', text: '良好', percent: 75 }; return { level: 'strong', text: '强', percent: 100 }
})

function makeRandomPwd() { const c = 'abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%&*'; let p = ''; for (let i = 0; i < 12; i++) p += c[Math.floor(Math.random() * c.length)]; p += 'Aa3!'; return p.split('').sort(() => Math.random() - 0.5).join('') }
function generatePassword() { userForm.value.password = makeRandomPwd(); showPwdCreate.value = true; clearValidation('password') }

function validateForm() {
  const e = {}
  if (!isEditing.value) {
    const u = (userForm.value.username||'').trim()
    if (!u) e.username = '用户名不能为空'; else if (u.length < 3) e.username = '至少3个字符'; else if (u.length > 32) e.username = '不能超过32字符'; else if (!/^[a-zA-Z0-9_-]+$/.test(u)) e.username = '只能含字母数字下划线横线'; else if (users.value.some(x => x.username.toLowerCase() === u.toLowerCase())) e.username = '用户名已存在'
    const p = userForm.value.password||''; if (!p) e.password = '密码不能为空'; else if (p.length < 8) e.password = '至少8位'; else if (!/[A-Z]/.test(p)) e.password = '需含大写字母'; else if (!/[a-z]/.test(p)) e.password = '需含小写字母'; else if (!/[0-9]/.test(p)) e.password = '需含数字'
  }
  if (userForm.value.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(userForm.value.email)) e.email = '邮箱格式不正确'
  if (!userForm.value.role) e.role = '请选择角色'; validationErrors.value = e; return Object.keys(e).length === 0
}

function openCreate() { isEditing.value = false; editingId.value = null; userForm.value = defaultForm(); formError.value = ''; validationErrors.value = {}; showPwdCreate.value = false; showUserModal.value = true }
function openEdit(row) { isEditing.value = true; editingId.value = row.id; userForm.value = { username: row.username, password: '', display_name: row.display_name||'', email: row.email||'', role: row.role||'viewer', tenant_id: row.tenant_id||'', enabled: row.enabled !== false }; formError.value = ''; validationErrors.value = {}; showUserModal.value = true }
function closeUserModal() { showUserModal.value = false; formError.value = ''; validationErrors.value = {} }

async function submitUser() {
  formError.value = ''; if (!validateForm()) return; submitting.value = true
  try {
    if (isEditing.value) { await apiPut('/api/v1/auth/users/' + editingId.value, { display_name: userForm.value.display_name, email: userForm.value.email, role: userForm.value.role, tenant_id: userForm.value.tenant_id, enabled: userForm.value.enabled }); showToast('用户已更新') }
    else { await apiPost('/api/v1/auth/users', { username: userForm.value.username.trim(), password: userForm.value.password, display_name: userForm.value.display_name, email: userForm.value.email, role: userForm.value.role, tenant_id: userForm.value.tenant_id, enabled: userForm.value.enabled }); showToast('用户已创建') }
    closeUserModal(); await loadUsers()
  } catch(e) { formError.value = e.message||'操作失败' } finally { submitting.value = false }
}

const showPwdModal = ref(false), pwdTarget = ref(null), newPassword = ref(''), confirmPassword = ref(''), pwdError = ref(''), pwdMode = ref('manual'), generatedPwd = ref(''), showPwdReset = ref(false)
function openResetPwd(row) { pwdTarget.value = row; newPassword.value = ''; confirmPassword.value = ''; pwdError.value = ''; pwdMode.value = 'manual'; generatedPwd.value = ''; showPwdReset.value = false; showPwdModal.value = true }
function closePwdModal() { showPwdModal.value = false; pwdError.value = '' }
function generateResetPassword() { generatedPwd.value = makeRandomPwd() }
async function copyToClipboard(text) { try { await navigator.clipboard.writeText(text); showToast('已复制到剪贴板') } catch { showToast('复制失败') } }
async function submitResetPwd() {
  pwdError.value = ''; const pwd = pwdMode.value === 'random' ? generatedPwd.value : newPassword.value
  if (!pwd || pwd.length < 8) { pwdError.value = '密码至少 8 位'; return }
  if (pwdMode.value === 'manual' && newPassword.value !== confirmPassword.value) { pwdError.value = '两次密码不一致'; return }
  submitting.value = true
  try { await apiPut('/api/v1/auth/users/' + pwdTarget.value.id, { password: pwd }); showToast('密码已重置'); closePwdModal() }
  catch(e) { pwdError.value = e.message||'重置失败' } finally { submitting.value = false }
}

const showAuditModal = ref(false), auditTarget = ref(null), auditEntries = ref([]), auditLoading = ref(false)
async function openAuditLog(row) {
  auditTarget.value = row; auditEntries.value = []; auditLoading.value = true; showAuditModal.value = true
  try { const d = await api('/api/v1/op-audit?username=' + encodeURIComponent(row.username) + '&limit=50'); auditEntries.value = d.entries || [] }
  catch { auditEntries.value = [] } finally { auditLoading.value = false }
}
function auditActionType(a) { if (!a) return 'info'; const l = a.toLowerCase(); if (l.includes('delete')||l.includes('remove')) return 'danger'; if (l.includes('create')||l.includes('add')) return 'success'; if (l.includes('update')||l.includes('reset')) return 'warning'; return 'info' }

const showDeleteModal = ref(false), deleteTarget = ref(null), deleteMessage = ref('')
function openDelete(row) { batchDeleteMode.value = false; deleteTarget.value = row; deleteMessage.value = '确定要删除用户「' + row.username + '」吗？此操作不可恢复。'; showDeleteModal.value = true }
function openBatchDelete() { showBatchMenu.value = false; batchDeleteMode.value = true; deleteMessage.value = '确定要删除选中的 ' + selectedIds.value.size + ' 个用户吗？此操作不可恢复。'; showDeleteModal.value = true }
async function confirmDeleteAction() {
  showDeleteModal.value = false
  if (batchDeleteMode.value) { let ok=0,fail=0; for (const id of selectedIds.value) { try { await apiDelete('/api/v1/auth/users/'+id); ok++ } catch { fail++ } }; showToast(ok+' 个用户已删除'+(fail>0 ? '，'+fail+' 个失败':'')); selectedIds.value = new Set() }
  else { try { await apiDelete('/api/v1/auth/users/' + deleteTarget.value.id); showToast('用户已删除') } catch(e) { showToast('删除失败: '+e.message) } }
  await loadUsers()
}

async function toggleEnabled(row) { try { await apiPut('/api/v1/auth/users/'+row.id, {enabled:!row.enabled}); row.enabled=!row.enabled; showToast(row.enabled?'已启用':'已禁用') } catch(e) { showToast('操作失败: '+e.message) } }
async function batchEnable(enable) {
  showBatchMenu.value = false; let ok=0,fail=0
  for (const id of selectedIds.value) { try { await apiPut('/api/v1/auth/users/'+id, {enabled:enable}); ok++ } catch { fail++ } }
  showToast(ok+' 个用户已'+(enable?'启用':'禁用')+(fail>0 ? '，'+fail+' 个失败':'')); selectedIds.value = new Set(); await loadUsers()
}

function roleLabel(r) { return {admin:'管理员',operator:'操作员',viewer:'观察者'}[r]||r||'--' }
function tenantName(id) { const t = tenants.value.find(x=>x.id===id); return t?(t.name||t.id):id }
function fmtTime(ts) { if (!ts) return '--'; try { const d = new Date(ts); if (isNaN(d.getTime())) return ts; return d.toLocaleString('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}) } catch { return ts } }

async function loadUsers() { loading.value = true; try { const d = await api('/api/v1/auth/users'); users.value = d.users||d||[] } catch { users.value = [] } finally { loading.value = false } }
async function loadTenants() { try { const d = await api('/api/v1/tenants'); tenants.value = d.tenants||[] } catch { tenants.value = [] } }
onMounted(() => { loadUsers(); loadTenants() })
</script>

<style scoped>
.users-page { padding: var(--space-6); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-5); gap: var(--space-4); }
.page-header-left { flex: 1; }
.page-header-right { display: flex; align-items: center; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 700; color: var(--text-primary); margin: 0; }
.page-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin: var(--space-1) 0 0 0; }

.search-bar { display: flex; align-items: center; gap: var(--space-3); margin-bottom: var(--space-4); position: relative; flex-wrap: wrap; }
.search-icon { position: absolute; left: 12px; color: var(--text-tertiary); pointer-events: none; }
.search-input { flex: 1; min-width: 200px; padding: 8px 12px 8px 36px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm); outline: none; font-family: var(--font-sans); transition: border-color var(--transition-fast); }
.search-input:focus { border-color: var(--color-primary); }
.search-input::placeholder { color: var(--text-tertiary); }
.filter-select { padding: 8px 12px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm); outline: none; font-family: var(--font-sans); min-width: 110px; cursor: pointer; }
.filter-select:focus { border-color: var(--color-primary); }
.filter-select option { background: var(--bg-elevated); }

/* Role Legend */
.role-legend { display: flex; align-items: center; gap: var(--space-4); padding: var(--space-3) var(--space-4); background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); margin-bottom: var(--space-4); position: relative; flex-wrap: wrap; }
.role-legend-item { display: flex; align-items: center; gap: var(--space-2); }
.rld { font-size: var(--text-xs); color: var(--text-secondary); }
.role-legend-close { position: absolute; right: 8px; top: 8px; background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 14px; }

/* Batch */
.batch-wrapper { position: relative; }
.btn-batch { display: inline-flex; align-items: center; gap: var(--space-2); }
.batch-menu { position: absolute; top: 100%; right: 0; margin-top: 4px; background: var(--bg-surface); border: 1px solid var(--border-default); border-radius: var(--radius-md); box-shadow: var(--shadow-lg); z-index: 100; min-width: 160px; padding: 4px 0; }
.batch-item { display: block; width: 100%; padding: 8px 16px; background: none; border: none; color: var(--text-primary); font-size: var(--text-sm); text-align: left; cursor: pointer; transition: background var(--transition-fast); }
.batch-item:hover { background: var(--bg-elevated); }
.batch-danger { color: #EF4444; }
.batch-danger:hover { background: rgba(239, 68, 68, 0.08); }
.batch-divider { height: 1px; background: var(--border-subtle); margin: 4px 0; }

/* Checkbox */
.checkbox-wrap { display: inline-flex; align-items: center; cursor: pointer; }
.checkbox-wrap input { accent-color: var(--color-primary); width: 16px; height: 16px; cursor: pointer; }
.checkbox-mark { display: none; }

/* User cell */
.cell-user { display: flex; align-items: center; gap: var(--space-3); }
.user-avatar { width: 32px; height: 32px; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-size: var(--text-sm); font-weight: 700; color: #fff; flex-shrink: 0; background: var(--color-primary); }
.avatar-admin { background: #EF4444; }
.avatar-operator { background: #6366F1; }
.avatar-viewer { background: #6B7280; }
.user-info { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
.user-name { font-weight: 600; color: var(--text-primary); font-size: var(--text-sm); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.user-display { font-size: var(--text-xs); color: var(--text-tertiary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

/* Role tag */
.role-tag { display: inline-flex; align-items: center; padding: 2px 10px; border-radius: 12px; font-size: 11px; font-weight: 600; letter-spacing: 0.03em; }
.role-admin { background: rgba(239, 68, 68, 0.15); color: #EF4444; }
.role-operator { background: rgba(99, 102, 241, 0.15); color: #818CF8; }
.role-viewer { background: rgba(107, 114, 128, 0.15); color: #9CA3AF; }

.tenant-tag { display: inline-flex; align-items: center; padding: 2px 8px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); font-size: var(--text-xs); color: var(--text-secondary); font-family: var(--font-mono); }

/* Toggle */
.toggle-switch-sm { position: relative; display: inline-block; width: 32px; height: 18px; cursor: pointer; }
.toggle-switch-sm input { opacity: 0; width: 0; height: 0; position: absolute; }
.toggle-slider-sm { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(255,255,255,0.1); border-radius: 18px; transition: all .25s; }
.toggle-slider-sm::before { content: ''; position: absolute; height: 14px; width: 14px; left: 2px; bottom: 2px; background: #fff; border-radius: 50%; transition: all .25s; }
.toggle-active-sm { background: #10B981; }
.toggle-active-sm::before { transform: translateX(14px); }

/* Actions */
.action-btns { display: flex; gap: var(--space-1); }
.btn-action { background: transparent; border: 1px solid transparent; padding: 4px 6px; border-radius: var(--radius-sm); color: var(--text-tertiary); cursor: pointer; transition: all var(--transition-fast); display: flex; align-items: center; }
.btn-action:hover { background: var(--bg-elevated); color: var(--text-primary); border-color: var(--border-subtle); }
.btn-action-danger:hover { color: #EF4444; border-color: rgba(239, 68, 68, 0.3); background: rgba(239, 68, 68, 0.08); }

.text-mono { font-family: var(--font-mono); }
.text-muted { color: var(--text-tertiary); font-size: var(--text-xs); }

/* Buttons */
.btn-primary { background: var(--color-primary); color: #fff; border: none; padding: 8px 16px; border-radius: var(--radius-md); font-size: var(--text-sm); font-weight: 600; cursor: pointer; transition: all var(--transition-fast); display: inline-flex; align-items: center; gap: var(--space-2); }
.btn-primary:hover { opacity: 0.9; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-outline { background: transparent; color: var(--text-secondary); border: 1px solid var(--border-default); padding: 8px 16px; border-radius: var(--radius-md); font-size: var(--text-sm); cursor: pointer; transition: all var(--transition-fast); }
.btn-outline:hover { border-color: var(--color-primary); color: var(--text-primary); }
.btn-ghost { background: transparent; border: 1px solid transparent; color: var(--text-tertiary); cursor: pointer; padding: 4px 10px; border-radius: var(--radius-sm); font-size: var(--text-xs); display: inline-flex; align-items: center; gap: 4px; transition: all var(--transition-fast); }
.btn-ghost:hover { color: var(--text-primary); background: var(--bg-elevated); }
.btn-sm { font-size: var(--text-xs); padding: 4px 10px; }

/* Password */
.password-field { display: flex; gap: 4px; align-items: center; }
.password-field .form-input { flex: 1; }
.pwd-toggle, .pwd-generate { background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); padding: 6px 8px; cursor: pointer; font-size: 12px; color: var(--text-secondary); transition: all var(--transition-fast); }
.pwd-toggle:hover, .pwd-generate:hover { border-color: var(--color-primary); color: var(--text-primary); }
.password-strength { display: flex; align-items: center; gap: var(--space-2); margin-top: 4px; }
.strength-bar { flex: 1; height: 4px; background: var(--bg-elevated); border-radius: 2px; overflow: hidden; }
.strength-fill { height: 100%; border-radius: 2px; transition: width .3s, background .3s; }
.str-weak { background: #EF4444; color: #EF4444; }
.str-fair { background: #F59E0B; color: #F59E0B; }
.str-good { background: #10B981; color: #10B981; }
.str-strong { background: #059669; color: #059669; }
.strength-text { font-size: 10px; font-weight: 600; }

/* Reset pwd mode */
.pwd-mode-switch { display: flex; gap: 0; margin-bottom: var(--space-3); border: 1px solid var(--border-default); border-radius: var(--radius-md); overflow: hidden; }
.pwd-mode-btn { flex: 1; padding: 6px 12px; background: transparent; border: none; font-size: var(--text-xs); color: var(--text-tertiary); cursor: pointer; transition: all var(--transition-fast); }
.pwd-mode-btn.active { background: var(--color-primary); color: #fff; }
.generated-pwd { display: flex; align-items: center; gap: var(--space-2); padding: 8px 12px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); }
.pwd-display { flex: 1; font-family: var(--font-mono); font-size: var(--text-sm); color: var(--text-primary); letter-spacing: 0.05em; word-break: break-all; background: transparent; }
.btn-copy { background: none; border: none; cursor: pointer; font-size: 14px; padding: 2px 4px; opacity: 0.7; transition: opacity .15s; }
.btn-copy:hover { opacity: 1; }
.field-hint { font-size: 10px; color: var(--text-tertiary); margin-top: 4px; }

/* Audit */
.audit-loading, .audit-empty { text-align: center; padding: var(--space-6); color: var(--text-tertiary); font-size: var(--text-sm); }
.audit-list { max-height: 400px; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.audit-item { display: flex; align-items: center; gap: var(--space-3); padding: 8px 12px; background: var(--bg-elevated); border-radius: var(--radius-sm); font-size: var(--text-xs); }
.audit-time { color: var(--text-tertiary); font-family: var(--font-mono); min-width: 130px; }
.audit-action-tag { padding: 2px 8px; border-radius: 10px; font-weight: 600; font-size: 10px; white-space: nowrap; }
.audit-danger { background: rgba(239, 68, 68, 0.15); color: #EF4444; }
.audit-success { background: rgba(16, 185, 129, 0.15); color: #10B981; }
.audit-warning { background: rgba(245, 158, 11, 0.15); color: #F59E0B; }
.audit-info { background: rgba(99, 102, 241, 0.15); color: #818CF8; }
.audit-detail { flex: 1; color: var(--text-secondary); min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.audit-ip { color: var(--text-tertiary); font-family: var(--font-mono); }

/* Modal */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center; z-index: 500; }
.modal-content { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-6); width: 520px; max-width: 90vw; box-shadow: var(--shadow-lg); }
.modal-sm { width: 420px; }
.modal-lg { width: 680px; }
.modal-title { font-size: var(--text-lg); font-weight: 700; color: var(--text-primary); margin: 0 0 var(--space-4) 0; }
.modal-desc { font-size: var(--text-sm); color: var(--text-secondary); margin: 0 0 var(--space-4) 0; line-height: 1.5; }
.modal-desc strong { color: var(--text-primary); font-weight: 600; }
.form-group { margin-bottom: var(--space-3); }
.form-group label { display: block; font-size: var(--text-xs); font-weight: 600; color: var(--text-secondary); margin-bottom: 4px; }
.required { color: #EF4444; }
.form-input { width: 100%; padding: 8px 12px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm); outline: none; box-sizing: border-box; font-family: var(--font-sans); transition: border-color var(--transition-fast); }
.form-input:focus { border-color: var(--color-primary); }
.input-error { border-color: #EF4444 !important; }
.field-error { font-size: 11px; color: #EF4444; margin-top: 2px; }
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
.fade-enter-active, .fade-leave-active { transition: opacity .2s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
