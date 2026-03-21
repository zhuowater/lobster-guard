<template>
  <div class="tenants-page">
    <div class="page-header">
      <div class="page-header-left">
        <h1 class="page-title"><Icon name="building" :size="20" /> 租户管理</h1>
        <p class="page-desc">安全域隔离 — 每个租户独立管理成员、安全策略和审计数据</p>
      </div>
      <div class="page-header-right">
        <div class="view-toggle">
          <button :class="['vt-btn', {active: viewMode==='card'}]" @click="viewMode='card'" title="卡片视图"><Icon name="grid" :size="14" /></button>
          <button :class="['vt-btn', {active: viewMode==='list'}]" @click="viewMode='list'" title="列表视图"><Icon name="list" :size="14" /></button>
        </div>
        <button class="btn-primary" @click="openCreateModal"><Icon name="plus" :size="14" /> 创建租户</button>
      </div>
    </div>

    <!-- 搜索栏 -->
    <div class="search-bar">
      <Icon name="search" :size="16" class="search-icon" />
      <input v-model="searchQuery" class="search-input" placeholder="搜索租户名、ID..." />
      <span class="search-count" v-if="searchQuery">{{ filteredTenants.length }} / {{ tenants.length }}</span>
    </div>

    <!-- Loading -->
    <div class="loading-state" v-if="pageLoading">
      <div class="loading-spinner"></div>
      <span>加载中...</span>
    </div>

    <!-- 卡片视图 -->
    <div class="tenant-grid" v-if="!pageLoading && viewMode==='card'">
      <div class="tenant-card" v-for="t in filteredTenants" :key="t.id" :class="{'card-disabled': !t.enabled}">
        <div class="card-header">
          <div class="card-title"><span class="card-icon"><Icon :name="t.strict_mode ? 'shield' : 'building'" :size="18" /></span><span>{{ t.name }}</span></div>
          <span class="card-badge" :class="t.enabled ? 'badge-active' : 'badge-inactive'">{{ t.enabled ? '启用' : '停用' }}</span>
        </div>
        <div class="card-id">ID: {{ t.id }}</div>
        <div class="card-desc" v-if="t.description">{{ t.description }}</div>

        <div class="card-stats">
          <div class="stat-item"><span class="stat-value">{{ fmtNum(t.im_calls || 0) }}</span><span class="stat-label">IM 调用</span></div>
          <div class="stat-item"><span class="stat-value">{{ fmtNum(t.llm_calls || 0) }}</span><span class="stat-label">LLM 调用</span></div>
          <div class="stat-item"><span class="stat-value">{{ t.member_count || 0 }}</span><span class="stat-label">成员</span></div>
          <div class="stat-item"><span class="stat-value">{{ fmtNum(t.block_count || 0) }}</span><span class="stat-label">拦截</span></div>
        </div>

        <div class="card-tabs">
          <button class="tab-btn" :class="{active: activeTab[t.id]==='members'}" @click="setTab(t.id,'members')"><Icon name="file-text" :size="14" /> 成员</button>
          <button class="tab-btn" :class="{active: activeTab[t.id]==='security'}" @click="setTab(t.id,'security'); loadTenantConfig(t.id)"><Icon name="lock" :size="14" /> 安全策略</button>
        </div>

        <!-- 成员 Tab -->
        <div class="card-section" v-if="activeTab[t.id]==='members'">
          <div class="section-header">
            <span class="section-title">成员映射 ({{ (members[t.id]||[]).length }})</span>
            <div class="section-actions">
              <input v-model="memberSearch[t.id]" class="member-search" placeholder="搜索成员..." v-if="(members[t.id]||[]).length > 3" />
              <button class="btn-xs btn-primary" @click="openAddMember(t.id)">+ 添加</button>
            </div>
          </div>
          <div class="member-list" v-if="filteredMembers(t.id).length > 0">
            <div class="member-item" v-for="m in filteredMembers(t.id)" :key="m.id">
              <label class="checkbox-wrap" @click.stop v-if="memberSelectMode[t.id]"><input type="checkbox" :checked="(memberSelected[t.id]||new Set()).has(m.id)" @change="toggleMemberSelect(t.id, m.id)" /></label>
              <span class="member-icon">{{ m.match_type==='sender_id'?'👤':m.match_type==='app_id'?'📱':'🔣' }}</span>
              <span class="member-type">{{ matchTypeLabel(m.match_type) }}</span>
              <span class="member-value">{{ m.match_value }}</span>
              <span class="member-desc" v-if="m.description">{{ m.description }}</span>
              <button class="btn-icon btn-danger-icon" @click="confirmRemoveMember(t.id, m)" title="删除">✕</button>
            </div>
          </div>
          <div class="empty-hint" v-else>暂无成员映射，点击"添加"绑定用户</div>
          <div class="member-batch-bar" v-if="(members[t.id]||[]).length > 0">
            <button class="btn-xs btn-outline" @click="toggleMemberSelectMode(t.id)">{{ memberSelectMode[t.id] ? '取消选择' : '批量管理' }}</button>
            <button class="btn-xs btn-danger" v-if="memberSelectMode[t.id] && (memberSelected[t.id]||new Set()).size > 0" @click="batchRemoveMembers(t.id)">删除选中 ({{ (memberSelected[t.id]||new Set()).size }})</button>
          </div>
        </div>

        <!-- 安全策略 Tab -->
        <div class="card-section" v-if="activeTab[t.id]==='security'">
          <div class="config-section" v-if="tenantConfigs[t.id]">
            <div class="config-inherit-hint"><Icon name="info" :size="12" /> 租户配置覆盖全局配置。未设置的项将使用全局默认值。</div>
            <div class="config-group">
              <div class="config-group-title"><Icon name="shield" :size="12" /> 检测规则</div>
              <div class="config-field">
                <label>禁用的规则</label>
                <input class="form-input" v-model="tenantConfigs[t.id].disabled_rules" placeholder="逗号分隔，如 roleplay_cn,roleplay_en" />
                <div class="field-hint">这些全局规则将对此租户禁用</div>
              </div>
            </div>
            <div class="config-group">
              <div class="config-group-title"><Icon name="lock" :size="12" /> LLM 安全</div>
              <div class="config-row">
                <label class="check-label"><input type="checkbox" v-model="tenantConfigs[t.id].canary_enabled" /> Canary Token</label>
                <label class="check-label"><input type="checkbox" v-model="tenantConfigs[t.id].budget_enabled" /> Response Budget</label>
              </div>
              <div class="config-row" v-if="tenantConfigs[t.id].budget_enabled">
                <div class="config-field compact"><label>Token 上限</label><input class="form-input" type="number" v-model.number="tenantConfigs[t.id].budget_max_tokens" placeholder="0=全局" /></div>
                <div class="config-field compact"><label>工具数上限</label><input class="form-input" type="number" v-model.number="tenantConfigs[t.id].budget_max_tools" placeholder="0=全局" /></div>
              </div>
              <div class="config-field"><label>工具黑名单</label><input class="form-input" v-model="tenantConfigs[t.id].tool_blacklist" placeholder="逗号分隔，如 exec,shell,curl" /></div>
            </div>
            <div class="config-group">
              <div class="config-group-title"><Icon name="bell" :size="12" /> 限流与告警</div>
              <div class="config-row">
                <div class="config-field compact"><label>通知阈值</label><select class="form-input" v-model="tenantConfigs[t.id].alert_level"><option value="low">低</option><option value="medium">中</option><option value="high">高</option><option value="critical">严重</option></select></div>
                <div class="config-field compact flex1"><label>Webhook</label><input class="form-input" v-model="tenantConfigs[t.id].alert_webhook" placeholder="https://..." /></div>
              </div>
            </div>
            <div class="config-actions">
              <button class="btn-sm btn-primary" @click="saveTenantConfig(t.id)" :disabled="savingConfig[t.id]">{{ savingConfig[t.id] ? '保存中...' : '保存配置' }}</button>
              <span class="save-hint">运行时联动 v14.1 实施</span>
            </div>
          </div>
          <div class="empty-hint" v-else>加载中...</div>
        </div>

        <div class="card-meta">
          <span v-if="t.strict_mode" class="meta-tag tag-strict">严格模式</span>
          <span v-if="t.max_agents" class="meta-tag tag-coming-soon">Agent ≤{{ t.max_agents }} 🚧</span>
          <span v-if="t.max_rules" class="meta-tag tag-coming-soon">规则 ≤{{ t.max_rules }} 🚧</span>
        </div>

        <div class="card-actions">
          <button class="btn-sm btn-primary" @click="switchToTenant(t.id)">管理</button>
          <button class="btn-sm btn-outline" @click="openEdit(t)">编辑</button>
          <button class="btn-sm btn-danger" v-if="t.id !== 'default'" @click="openDeleteTenant(t)">删除</button>
        </div>
      </div>
    </div>

    <!-- 列表视图 -->
    <div class="tenant-list-view" v-if="!pageLoading && viewMode==='list'">
      <table class="tenant-table">
        <thead><tr><th>租户</th><th>ID</th><th>成员</th><th>IM调用</th><th>LLM调用</th><th>拦截</th><th>状态</th><th>操作</th></tr></thead>
        <tbody>
          <tr v-for="t in filteredTenants" :key="t.id" :class="{'row-disabled': !t.enabled}">
            <td><div class="list-tenant-name"><Icon :name="t.strict_mode?'shield':'building'" :size="14" /> {{ t.name }}</div></td>
            <td><code class="list-id">{{ t.id }}</code></td>
            <td>{{ t.member_count || 0 }}</td>
            <td>{{ fmtNum(t.im_calls || 0) }}</td>
            <td>{{ fmtNum(t.llm_calls || 0) }}</td>
            <td>{{ fmtNum(t.block_count || 0) }}</td>
            <td><span class="card-badge" :class="t.enabled?'badge-active':'badge-inactive'">{{ t.enabled?'启用':'停用' }}</span></td>
            <td>
              <div class="action-btns">
                <button class="btn-sm btn-primary" @click="switchToTenant(t.id)">管理</button>
                <button class="btn-sm btn-outline" @click="openEdit(t)">编辑</button>
                <button class="btn-sm btn-danger" v-if="t.id!=='default'" @click="openDeleteTenant(t)">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create/Edit Modal -->
    <Transition name="modal-fade">
      <div class="modal-overlay" v-if="showCreateModal || showEditModal" @click.self="closeModals">
        <div class="modal-content">
          <h3 class="modal-title">{{ showEditModal ? '编辑租户' : '创建租户' }}</h3>
          <div class="form-group" v-if="!showEditModal">
            <label>租户 ID <span class="required">*</span></label>
            <input v-model="form.id" placeholder="如 security-team（创建后不可修改）" class="form-input" :class="{'input-error': formErrors.id}" @input="clearFormError('id')" />
            <div class="field-error" v-if="formErrors.id">{{ formErrors.id }}</div>
          </div>
          <div class="form-group">
            <label>名称 <span class="required">*</span></label>
            <input v-model="form.name" placeholder="如 安全团队" class="form-input" :class="{'input-error': formErrors.name}" @input="clearFormError('name')" />
            <div class="field-error" v-if="formErrors.name">{{ formErrors.name }}</div>
          </div>
          <div class="form-group"><label>描述</label><input v-model="form.description" placeholder="可选描述" class="form-input" /></div>
          <div class="form-group form-check"><label class="check-label"><input type="checkbox" v-model="form.strict_mode" /><Icon name="shield" :size="14" /> 严格模式</label></div>
          <div class="modal-actions"><button class="btn-outline" @click="closeModals">取消</button><button class="btn-primary" @click="submitForm" :disabled="submitting">{{ submitting ? '提交中...' : (showEditModal ? '保存' : '创建') }}</button></div>
          <div class="form-error" v-if="formError">{{ formError }}</div>
        </div>
      </div>
    </Transition>

    <!-- Add Member Modal -->
    <Transition name="modal-fade">
      <div class="modal-overlay" v-if="showMemberModal" @click.self="showMemberModal=false">
        <div class="modal-content">
          <h3 class="modal-title">添加成员映射</h3>
          <div class="form-group"><label>匹配类型</label><select v-model="memberForm.match_type" class="form-input"><option value="sender_id">👤 用户 ID (sender_id)</option><option value="app_id">📱 应用 ID (app_id)</option><option value="pattern">🔣 模式匹配 (pattern)</option></select></div>
          <div class="form-group">
            <label>匹配值 <span class="required">*</span></label>
            <textarea v-model="memberForm.match_value" :placeholder="memberPlaceholder" class="form-input form-textarea" rows="3"></textarea>
            <div class="field-hint">批量添加：每行一个值</div>
          </div>
          <div class="form-group"><label>备注（可选）</label><input v-model="memberForm.description" placeholder="如 安全团队-张三" class="form-input" /></div>
          <div class="modal-actions"><button class="btn-outline" @click="showMemberModal=false">取消</button><button class="btn-primary" @click="submitMember" :disabled="submitting">{{ submitting ? '添加中...' : '确认添加' }}</button></div>
          <div class="form-error" v-if="memberError">{{ memberError }}</div>
        </div>
      </div>
    </Transition>

    <!-- Delete confirmations -->
    <ConfirmModal :visible="showDeleteTenantModal" title="删除租户" :message="deleteTenantMsg" type="danger" confirm-text="删除" @confirm="confirmDeleteTenant" @cancel="showDeleteTenantModal=false" />
    <ConfirmModal :visible="showDeleteMemberModal" title="删除成员" :message="deleteMemberMsg" type="danger" confirm-text="删除" @confirm="confirmRemoveMemberAction" @cancel="showDeleteMemberModal=false" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, inject } from 'vue'
import Icon from '../components/Icon.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import { setTenant } from '../stores/app.js'
import { useRouter } from 'vue-router'

const router = useRouter()
const showToast = inject('showToast')
const tenants = ref([])
const members = reactive({})
const tenantConfigs = reactive({})
const savingConfig = reactive({})
const activeTab = reactive({})
const pageLoading = ref(true)
const viewMode = ref('card')
const searchQuery = ref('')
const memberSearch = reactive({})
const memberSelectMode = reactive({})
const memberSelected = reactive({})

// Modal state
const showCreateModal = ref(false), showEditModal = ref(false), showMemberModal = ref(false)
const submitting = ref(false), formError = ref(''), memberError = ref(''), formErrors = ref({})
const form = ref({ id: '', name: '', description: '', max_agents: 0, max_rules: 0, strict_mode: false })
const memberForm = ref({ tenant_id: '', match_type: 'sender_id', match_value: '', description: '' })

// Delete state
const showDeleteTenantModal = ref(false), deleteTenantTarget = ref(null), deleteTenantMsg = ref('')
const showDeleteMemberModal = ref(false), deleteMemberTarget = ref(null), deleteMemberTenantId = ref(''), deleteMemberMsg = ref('')

const filteredTenants = computed(() => {
  if (!searchQuery.value) return tenants.value
  const q = searchQuery.value.toLowerCase()
  return tenants.value.filter(t => (t.name||'').toLowerCase().includes(q) || (t.id||'').toLowerCase().includes(q) || (t.description||'').toLowerCase().includes(q))
})

function filteredMembers(tid) {
  const list = members[tid] || []
  const q = (memberSearch[tid] || '').toLowerCase()
  if (!q) return list
  return list.filter(m => (m.match_value||'').toLowerCase().includes(q) || (m.description||'').toLowerCase().includes(q))
}

const memberPlaceholder = computed(() => {
  switch (memberForm.value.match_type) {
    case 'sender_id': return '如 user-001\n可每行一个批量添加'
    case 'app_id': return '如 bot-security'
    case 'pattern': return '如 sec-* 或 admin-?'
    default: return ''
  }
})

function clearFormError(f) { const v = {...formErrors.value}; delete v[f]; formErrors.value = v }

async function loadTenants() {
  pageLoading.value = true
  try {
    const d = await api('/api/v1/tenants'); tenants.value = d.tenants || []
    for (const t of tenants.value) { if (!activeTab[t.id]) activeTab[t.id] = 'members'; loadMembers(t.id) }
  } catch { tenants.value = [] } finally { pageLoading.value = false }
}

async function loadMembers(tid) { try { const d = await api('/api/v1/tenants/' + tid + '/members'); members[tid] = d.members || [] } catch { members[tid] = [] } }

async function loadTenantConfig(tid) {
  if (tenantConfigs[tid]) return
  try { const d = await api('/api/v1/tenants/' + tid + '/config'); tenantConfigs[tid] = d.config || {} }
  catch { tenantConfigs[tid] = { canary_enabled: true, budget_enabled: true, alert_level: 'high' } }
}

async function saveTenantConfig(tid) {
  savingConfig[tid] = true
  try { await apiPut('/api/v1/tenants/' + tid + '/config', tenantConfigs[tid]); showToast('安全配置已保存') }
  catch(e) { showToast('保存失败: ' + e.message) } finally { savingConfig[tid] = false }
}

function setTab(tid, tab) { activeTab[tid] = tab }
function matchTypeLabel(t) { return {sender_id:'用户',app_id:'应用',pattern:'模式'}[t]||t }
function fmtNum(n) { if (n >= 10000) return (n/1000).toFixed(1)+'k'; if (n >= 1000) return n.toLocaleString(); return String(n) }

function switchToTenant(id) { setTenant(id); router.push('/overview'); setTimeout(() => router.go(0), 100) }

function openCreateModal() {
  form.value = { id: '', name: '', description: '', max_agents: 0, max_rules: 0, strict_mode: false }
  formError.value = ''; formErrors.value = {}; showCreateModal.value = true
}

function openEdit(t) {
  form.value = { id: t.id, name: t.name, description: t.description||'', max_agents: t.max_agents||0, max_rules: t.max_rules||0, strict_mode: t.strict_mode||false }
  formError.value = ''; formErrors.value = {}; showEditModal.value = true
}

function openAddMember(tid) {
  memberForm.value = { tenant_id: tid, match_type: 'sender_id', match_value: '', description: '' }
  memberError.value = ''; showMemberModal.value = true
}

function closeModals() { showCreateModal.value = false; showEditModal.value = false; formError.value = ''; formErrors.value = {}; form.value = { id: '', name: '', description: '', max_agents: 0, max_rules: 0, strict_mode: false } }

function validateTenantForm() {
  const e = {}
  if (!showEditModal.value) {
    const id = (form.value.id||'').trim()
    if (!id) e.id = 'ID 不能为空'; else if (!/^[a-zA-Z0-9_-]+$/.test(id)) e.id = '只能含字母数字下划线横线'
    else if (tenants.value.some(t => t.id === id)) e.id = 'ID 已存在'
  }
  if (!(form.value.name||'').trim()) e.name = '名称不能为空'
  else if (!showEditModal.value && tenants.value.some(t => t.name === form.value.name.trim())) e.name = '名称已存在'
  formErrors.value = e; return Object.keys(e).length === 0
}

async function submitForm() {
  formError.value = ''; if (!validateTenantForm()) return; submitting.value = true
  try {
    if (showEditModal.value) { await apiPut('/api/v1/tenants/' + form.value.id, form.value); showToast('租户已更新') }
    else { await apiPost('/api/v1/tenants', { ...form.value, enabled: true }); showToast('租户已创建') }
    closeModals(); await loadTenants()
  } catch(e) { formError.value = e.message||'操作失败' } finally { submitting.value = false }
}

async function submitMember() {
  memberError.value = ''
  const values = memberForm.value.match_value.split('\n').map(v => v.trim()).filter(Boolean)
  if (values.length === 0) { memberError.value = '匹配值不能为空'; return }
  submitting.value = true; let ok = 0, fail = 0
  try {
    for (const val of values) {
      try { await apiPost('/api/v1/tenants/' + memberForm.value.tenant_id + '/members', { match_type: memberForm.value.match_type, match_value: val, description: memberForm.value.description }); ok++ }
      catch { fail++ }
    }
    showToast(ok + ' 个成员已添加' + (fail > 0 ? '，' + fail + ' 个失败' : ''))
    showMemberModal.value = false
    await loadMembers(memberForm.value.tenant_id); await loadTenants()
  } catch(e) { memberError.value = e.message||'添加失败' } finally { submitting.value = false }
}

// Delete tenant
function openDeleteTenant(t) { deleteTenantTarget.value = t; deleteTenantMsg.value = '确定要删除租户「' + t.name + '」(' + t.id + ') 吗？\n\n⚠️ 删除后该租户数据将无法通过 Dashboard 访问。'; showDeleteTenantModal.value = true }
async function confirmDeleteTenant() {
  showDeleteTenantModal.value = false
  try { await apiDelete('/api/v1/tenants/' + deleteTenantTarget.value.id); showToast('租户已删除'); await loadTenants() }
  catch(e) { showToast('删除失败: ' + e.message) }
}

// Delete member
function confirmRemoveMember(tid, m) { deleteMemberTenantId.value = tid; deleteMemberTarget.value = m; deleteMemberMsg.value = '确定要删除成员映射「' + m.match_value + '」吗？'; showDeleteMemberModal.value = true }
async function confirmRemoveMemberAction() {
  showDeleteMemberModal.value = false
  try { await apiDelete('/api/v1/tenants/' + deleteMemberTenantId.value + '/members/' + deleteMemberTarget.value.id); showToast('成员已删除'); await loadMembers(deleteMemberTenantId.value); await loadTenants() }
  catch(e) { showToast('删除失败: ' + e.message) }
}

// Batch member operations
function toggleMemberSelectMode(tid) {
  if (memberSelectMode[tid]) { memberSelectMode[tid] = false; memberSelected[tid] = new Set() }
  else { memberSelectMode[tid] = true; memberSelected[tid] = new Set() }
}
function toggleMemberSelect(tid, mid) {
  const s = new Set(memberSelected[tid] || [])
  if (s.has(mid)) s.delete(mid); else s.add(mid)
  memberSelected[tid] = s
}
async function batchRemoveMembers(tid) {
  const ids = memberSelected[tid]; if (!ids || ids.size === 0) return
  let ok = 0, fail = 0
  for (const mid of ids) { try { await apiDelete('/api/v1/tenants/' + tid + '/members/' + mid); ok++ } catch { fail++ } }
  showToast(ok + ' 个成员已删除' + (fail > 0 ? '，' + fail + ' 个失败' : ''))
  memberSelectMode[tid] = false; memberSelected[tid] = new Set()
  await loadMembers(tid); await loadTenants()
}

onMounted(loadTenants)
</script>

<style scoped>
.tenants-page { padding: var(--space-6); max-width: 1200px; }
.page-header { margin-bottom: var(--space-5); display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-4); }
.page-header-left { flex: 1; }
.page-header-right { display: flex; align-items: center; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 700; color: var(--text-primary); margin: 0; }
.page-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin: var(--space-1) 0 0 0; }

/* View toggle */
.view-toggle { display: flex; border: 1px solid var(--border-default); border-radius: var(--radius-md); overflow: hidden; }
.vt-btn { background: transparent; border: none; padding: 6px 10px; cursor: pointer; color: var(--text-tertiary); transition: all var(--transition-fast); display: flex; align-items: center; }
.vt-btn.active { background: var(--color-primary); color: #fff; }
.vt-btn:hover:not(.active) { background: var(--bg-elevated); }

/* Search */
.search-bar { display: flex; align-items: center; gap: var(--space-3); margin-bottom: var(--space-4); position: relative; }
.search-icon { position: absolute; left: 12px; color: var(--text-tertiary); pointer-events: none; }
.search-input { flex: 1; padding: 8px 12px 8px 36px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm); outline: none; font-family: var(--font-sans); transition: border-color var(--transition-fast); }
.search-input:focus { border-color: var(--color-primary); }
.search-input::placeholder { color: var(--text-tertiary); }
.search-count { font-size: var(--text-xs); color: var(--text-tertiary); }

/* Loading */
.loading-state { display: flex; align-items: center; justify-content: center; gap: var(--space-3); padding: var(--space-8); color: var(--text-tertiary); }
.loading-spinner { width: 20px; height: 20px; border: 2px solid var(--border-default); border-top-color: var(--color-primary); border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* Grid */
.tenant-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(380px, 1fr)); gap: var(--space-4); }
.tenant-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-5); transition: all var(--transition-fast); display: flex; flex-direction: column; gap: var(--space-3); }
.tenant-card:hover { border-color: var(--color-primary); box-shadow: var(--shadow-md); }
.card-disabled { opacity: 0.6; }

.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-title { display: flex; align-items: center; gap: var(--space-2); font-weight: 700; font-size: var(--text-base); color: var(--text-primary); }
.card-icon { font-size: 1.2em; }
.card-badge { font-size: 10px; font-weight: 700; padding: 2px 8px; border-radius: 12px; text-transform: uppercase; letter-spacing: 0.05em; }
.badge-active { background: rgba(16, 185, 129, 0.15); color: #10B981; }
.badge-inactive { background: rgba(239, 68, 68, 0.15); color: #EF4444; }
.card-id { font-family: var(--font-mono); font-size: var(--text-xs); color: var(--text-tertiary); }
.card-desc { font-size: var(--text-xs); color: var(--text-secondary); line-height: 1.5; }

.card-stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-2); padding: var(--space-3) 0; border-top: 1px solid var(--border-subtle); border-bottom: 1px solid var(--border-subtle); }
.stat-item { text-align: center; }
.stat-value { display: block; font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: 10px; color: var(--text-tertiary); }

.card-tabs { display: flex; gap: 2px; border-bottom: 1px solid var(--border-subtle); }
.tab-btn { background: transparent; border: none; padding: 6px 12px; font-size: var(--text-xs); color: var(--text-tertiary); cursor: pointer; border-bottom: 2px solid transparent; transition: all var(--transition-fast); display: inline-flex; align-items: center; gap: 4px; }
.tab-btn.active { color: var(--color-primary); border-bottom-color: var(--color-primary); }
.tab-btn:hover { color: var(--text-primary); }

.card-section { min-height: 60px; }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-2); gap: var(--space-2); flex-wrap: wrap; }
.section-title { font-size: var(--text-xs); font-weight: 600; color: var(--text-secondary); }
.section-actions { display: flex; align-items: center; gap: var(--space-2); }
.member-search { padding: 4px 8px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-sm); color: var(--text-primary); font-size: 11px; outline: none; width: 120px; }
.member-search:focus { border-color: var(--color-primary); }

.member-list { display: flex; flex-direction: column; gap: 4px; }
.member-item { display: flex; align-items: center; gap: var(--space-2); padding: 4px 8px; background: var(--bg-elevated); border-radius: var(--radius-sm); font-size: var(--text-xs); }
.member-icon { flex-shrink: 0; }
.member-type { color: var(--text-tertiary); min-width: 28px; }
.member-value { font-family: var(--font-mono); color: var(--text-primary); font-weight: 600; }
.member-desc { color: var(--text-tertiary); margin-left: auto; }
.btn-icon { background: none; border: none; cursor: pointer; padding: 2px 4px; font-size: 12px; opacity: 0.5; transition: opacity 0.15s; }
.btn-icon:hover { opacity: 1; }
.btn-danger-icon { color: #EF4444; }
.empty-hint { font-size: var(--text-xs); color: var(--text-tertiary); padding: var(--space-2) 0; text-align: center; }
.member-batch-bar { display: flex; gap: var(--space-2); margin-top: var(--space-2); padding-top: var(--space-2); border-top: 1px solid var(--border-subtle); }
.checkbox-wrap { display: inline-flex; align-items: center; cursor: pointer; }
.checkbox-wrap input { accent-color: var(--color-primary); width: 14px; height: 14px; cursor: pointer; }

/* Config */
.config-inherit-hint { display: flex; align-items: center; gap: var(--space-2); padding: 6px 10px; background: rgba(99, 102, 241, 0.08); border: 1px solid rgba(99, 102, 241, 0.2); border-radius: var(--radius-sm); font-size: 10px; color: var(--color-primary); margin-bottom: var(--space-3); }
.config-group { margin-bottom: var(--space-3); }
.config-group-title { font-size: 11px; font-weight: 700; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: var(--space-2); display: flex; align-items: center; gap: 4px; }
.config-field { margin-bottom: var(--space-2); }
.config-field label { display: block; font-size: 11px; font-weight: 600; color: var(--text-secondary); margin-bottom: 2px; }
.config-field.compact { flex: 1; min-width: 0; }
.flex1 { flex: 2 !important; }
.config-row { display: flex; gap: var(--space-3); align-items: flex-start; flex-wrap: wrap; margin-bottom: var(--space-2); }
.field-hint { font-size: 10px; color: var(--text-tertiary); margin-top: 2px; }
.config-actions { display: flex; align-items: center; gap: var(--space-3); padding-top: var(--space-2); border-top: 1px solid var(--border-subtle); }
.save-hint { font-size: 10px; color: var(--text-tertiary); }

.card-meta { display: flex; flex-wrap: wrap; gap: var(--space-1); }
.meta-tag { font-size: 10px; padding: 2px 6px; border-radius: var(--radius-sm); background: var(--bg-elevated); color: var(--text-secondary); border: 1px solid var(--border-subtle); }
.tag-strict { background: rgba(239, 68, 68, 0.1); color: #EF4444; border-color: rgba(239, 68, 68, 0.3); }
.tag-coming-soon { opacity: 0.5; text-decoration: line-through; }
.card-actions { display: flex; gap: var(--space-2); margin-top: auto; }

/* List view */
.tenant-list-view { overflow-x: auto; }
.tenant-table { width: 100%; border-collapse: collapse; font-size: var(--text-sm); }
.tenant-table th { text-align: left; padding: 8px 12px; font-size: var(--text-xs); font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: 0.05em; border-bottom: 1px solid var(--border-subtle); }
.tenant-table td { padding: 10px 12px; border-bottom: 1px solid var(--border-subtle); color: var(--text-primary); }
.tenant-table tr:hover { background: var(--bg-elevated); }
.row-disabled { opacity: 0.5; }
.list-tenant-name { display: flex; align-items: center; gap: var(--space-2); font-weight: 600; }
.list-id { font-family: var(--font-mono); font-size: var(--text-xs); color: var(--text-tertiary); background: var(--bg-elevated); padding: 1px 6px; border-radius: var(--radius-sm); }
.action-btns { display: flex; gap: var(--space-2); }

/* Buttons */
.btn-primary { background: var(--color-primary); color: #fff; border: none; padding: 8px 16px; border-radius: var(--radius-md); font-size: var(--text-sm); font-weight: 600; cursor: pointer; transition: all var(--transition-fast); display: inline-flex; align-items: center; gap: var(--space-2); }
.btn-primary:hover { opacity: 0.9; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-outline { background: transparent; color: var(--text-secondary); border: 1px solid var(--border-default); padding: 8px 16px; border-radius: var(--radius-md); font-size: var(--text-sm); cursor: pointer; transition: all var(--transition-fast); }
.btn-outline:hover { border-color: var(--color-primary); color: var(--text-primary); }
.btn-sm { padding: 4px 12px; font-size: var(--text-xs); }
.btn-xs { padding: 2px 8px; font-size: 10px; }
.btn-danger { background: transparent; color: #EF4444; border: 1px solid rgba(239, 68, 68, 0.3); padding: 4px 12px; border-radius: var(--radius-md); font-size: var(--text-xs); cursor: pointer; transition: all var(--transition-fast); }
.btn-danger:hover { background: rgba(239, 68, 68, 0.1); }
.check-label { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); color: var(--text-secondary); cursor: pointer; }
.check-label input[type="checkbox"] { accent-color: var(--color-primary); }

/* Modal */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center; z-index: 500; }
.modal-content { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-6); width: 480px; max-width: 90vw; box-shadow: var(--shadow-lg); }
.modal-title { font-size: var(--text-lg); font-weight: 700; color: var(--text-primary); margin: 0 0 var(--space-4) 0; }
.form-group { margin-bottom: var(--space-3); }
.form-group label { display: block; font-size: var(--text-xs); font-weight: 600; color: var(--text-secondary); margin-bottom: 4px; }
.required { color: #EF4444; }
.form-input { width: 100%; padding: 8px 12px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm); outline: none; box-sizing: border-box; font-family: var(--font-sans); transition: border-color var(--transition-fast); }
.form-input:focus { border-color: var(--color-primary); }
.form-textarea { resize: vertical; min-height: 60px; font-family: var(--font-mono); }
.input-error { border-color: #EF4444 !important; }
.field-error { font-size: 11px; color: #EF4444; margin-top: 2px; }
.form-check { padding-top: var(--space-1); }
.modal-actions { display: flex; justify-content: flex-end; gap: var(--space-2); margin-top: var(--space-4); }
.form-error { margin-top: var(--space-2); font-size: var(--text-xs); color: #EF4444; }

.modal-fade-enter-active { transition: all .2s ease; }
.modal-fade-leave-active { transition: all .2s ease; }
.modal-fade-enter-from, .modal-fade-leave-to { opacity: 0; }
.modal-fade-enter-from .modal-content { transform: scale(0.95); }
</style>
