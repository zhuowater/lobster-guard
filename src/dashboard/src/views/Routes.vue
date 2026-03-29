<template>
  <div>
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/><polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/><line x1="4" y1="4" x2="9" y2="9"/></svg></span>
        <span class="card-title">策略路由管理</span>
        <div class="card-actions">
          <button class="btn btn-sm" @click="openPolicyCreate"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> 新建策略</button>
          <button class="btn btn-ghost btn-sm" @click="loadPolicies"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg> 刷新</button>
        </div>
      </div>
      <div>
        <div v-if="policiesLoading" style="padding:24px;text-align:center;color:var(--text-secondary)">加载中...</div>
        <div v-else-if="policies.length === 0" class="policy-empty">
          <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="var(--text-disabled)" stroke-width="1.5"><polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/></svg>
          <div style="margin-top:12px;color:var(--text-secondary)">暂无路由策略配置</div>
          <div style="font-size:var(--text-xs);color:var(--text-tertiary);margin-top:4px">点击「新建策略」按钮添加</div>
        </div>
        <table v-else class="policy-table">
          <thead><tr><th style="width:40px">#</th><th>匹配条件</th><th>目标上游</th><th>类型</th><th style="width:100px;text-align:right">操作</th></tr></thead>
          <tbody>
            <tr v-for="(p, idx) in policies" :key="idx" :class="{ 'policy-matched': policyTestResult && policyTestResult.matched && policyTestResult.policy_index === idx }">
              <td style="color:var(--text-tertiary);font-size:.75rem;font-weight:600">{{ idx + 1 }}</td>
              <td><div class="policy-conditions">
                <span v-if="gmf(p,'department')" class="tag tag-info">部门: {{ gmf(p,'department') }}</span>
                <span v-if="gmf(p,'email')" class="tag tag-info">邮箱: {{ gmf(p,'email') }}</span>
                <span v-if="gmf(p,'email_suffix')" class="tag tag-info">后缀: {{ gmf(p,'email_suffix') }}</span>
                <span v-if="gmf(p,'app_id')" class="tag tag-info">App: {{ gmf(p,'app_id') }}</span>
                <span v-if="gmf(p,'default')" class="tag tag-pass">默认策略</span>
              </div></td>
              <td><span class="tag" style="background:var(--color-primary-dim);color:var(--color-primary);font-weight:600">{{ p.upstream_id || '(默认分配)' }}</span></td>
              <td><span v-if="gmf(p,'default')" class="tag" style="background:var(--color-warning-dim);color:var(--color-warning)">默认</span><span v-else class="tag tag-info">条件</span></td>
              <td style="text-align:right">
                <button class="btn btn-ghost btn-sm" @click="openPolicyEdit(idx, p)" title="编辑"><Icon name="edit" :size="14" /></button>
                <button class="btn btn-ghost btn-sm" @click="confirmDeletePolicy(idx, p)" style="margin-left:4px" title="删除"><Icon name="trash" :size="14" /></button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="policy-test-section">
        <div class="policy-test-header"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg><span>策略匹配测试</span></div>
        <div class="policy-test-form">
          <input v-model="policyTestForm.email" placeholder="邮箱" style="flex:1;min-width:120px" />
          <input v-model="policyTestForm.department" placeholder="部门" style="flex:1;min-width:100px" />
          <input v-model="policyTestForm.app_id" placeholder="App ID" style="flex:1;min-width:100px" />
          <button class="btn btn-sm" @click="testPolicy">测试</button>
        </div>
        <div v-if="policyTestResult" class="policy-test-result" :class="{ matched: policyTestResult.matched }">
          <div v-if="policyTestResult.matched">
            <span style="color:var(--color-success);font-weight:600">✅ 命中策略 #{{ policyTestResult.policy_index + 1 }}</span>
            <span style="margin-left:12px">→ <span class="tag" style="background:var(--color-primary-dim);color:var(--color-primary);font-weight:700">{{ policyTestResult.upstream_id }}</span></span>
          </div>
          <div v-else><span style="color:var(--color-danger);font-weight:600">❌ 未命中</span><span v-if="policyTestResult.message" style="margin-left:8px;font-size:.82rem;color:var(--text-secondary)">{{ policyTestResult.message }}</span></div>
        </div>
      </div>
    </div>
    <div class="card">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></svg></span>
        <span class="card-title">亲和路由管理</span>
        <div class="card-actions">
          <button class="btn btn-sm" @click="showBindModal = true"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> 绑定用户</button>
          <button class="btn btn-ghost btn-sm" @click="showBatchModal = true"><Icon name="import" :size="14" /> 批量绑定</button>
          <button class="btn btn-ghost btn-sm" @click="showMigrateModal = true"><Icon name="refresh" :size="14" /> 迁移用户</button>
          <button class="btn btn-ghost btn-sm" @click="refresh"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg></button>
        </div>
      </div>
      <div v-if="routeStats" class="route-stats-bar">
        <div class="route-stat-item"><span class="route-stat-label">Bot</span><span class="route-stat-value" style="color:var(--color-primary)">{{ routeStats.appCount }}</span></div><div class="route-stat-divider"></div>
        <div class="route-stat-item"><span class="route-stat-label">用户</span><span class="route-stat-value" style="color:var(--color-success)">{{ routeStats.senderCount }}</span></div><div class="route-stat-divider"></div>
        <div class="route-stat-item"><span class="route-stat-label">上游</span><span class="route-stat-value" style="color:var(--color-info)">{{ routeStats.upstreamCount }}</span></div><div class="route-stat-divider"></div>
        <div class="route-stat-item"><span class="route-stat-label">路由</span><span class="route-stat-value" style="color:var(--color-warning)">{{ routeStats.total }}</span></div>
      </div>
      <div class="filters">
        <select v-model="filterApp"><option value="">全部 Bot</option><option v-for="a in apps" :key="a" :value="a">{{ a.length > 20 ? a.substring(0, 20) + '...' : a }}</option></select>
        <select v-model="filterDept"><option value="">全部部门</option><option v-for="d in depts" :key="d" :value="d">{{ d }}</option></select>
        <div class="search-input-wrap"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg><input type="text" v-model="searchText" placeholder="搜索用户ID/名称..." /></div>
      </div>
      <DataTable :columns="columns" :data="filteredRoutes" :loading="loading" empty-text="尚未绑定用户" empty-desc="通过上方按钮绑定用户到指定上游" :expandable="true">
        <template #cell-sender_id="{ row }"><span style="font-size:.75rem;font-family:var(--font-mono)">{{ row.sender_id }}</span></template>
        <template #cell-app_id="{ row }"><span style="font-size:.75rem" :title="row.app_id">{{ (row.app_id||'--').length > 16 ? row.app_id.substring(0,16)+'...' : (row.app_id||'--') }}</span></template>
        <template #cell-upstream_id="{ row }"><span class="tag" style="background:var(--color-primary-dim);color:var(--color-primary);font-weight:500">{{ row.upstream_id }}</span></template>
        <template #expand="{ row }">
          <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:12px 24px;font-size:.82rem">
            <div><span class="expand-label">用户ID</span><span class="expand-value">{{ row.sender_id }}</span></div>
            <div><span class="expand-label">姓名</span><span class="expand-value">{{ getUserInfo(row,'name') }}</span></div>
            <div><span class="expand-label">邮箱</span><span class="expand-value">{{ getUserInfo(row,'email') }}</span></div>
            <div><span class="expand-label">手机</span><span class="expand-value">{{ getUserInfo(row,'mobile') }}</span></div>
            <div><span class="expand-label">部门</span><span class="expand-value">{{ getUserInfo(row,'department') }}</span></div>
            <div><span class="expand-label">App</span><span class="expand-value" style="font-family:var(--font-mono);font-size:.75rem">{{ row.app_id||'--' }}</span></div>
            <div><span class="expand-label">上游</span><span class="expand-value">{{ row.upstream_id }}</span></div>
          </div>
        </template>
        <template #actions="{ row }">
          <button class="btn btn-ghost btn-sm" @click.stop="openMigrateFor(row)" title="迁移"><Icon name="refresh" :size="14" /></button>
          <button class="btn btn-danger btn-sm" @click.stop="confirmUnbind(row)" style="margin-left:4px">解绑</button>
        </template>
      </DataTable>
    </div>
    <BindModal :visible="showBindModal" title="绑定用户" icon="🔗" description="将用户绑定到指定上游服务" :fields="bindFields" v-model="bindForm" confirm-text="确认绑定" @confirm="doBind" @cancel="showBindModal=false"><template #field-upstream="{value,update}"><UpstreamSelect :modelValue="value" @update:modelValue="update"/></template></BindModal>
    <BindModal :visible="showBatchModal" title="批量绑定" icon="📥" description="批量绑定多个用户到同一上游" :fields="batchFields" v-model="batchForm" confirm-text="确认绑定" @confirm="doBatchBind" @cancel="showBatchModal=false"><template #field-upstream="{value,update}"><UpstreamSelect :modelValue="value" @update:modelValue="update"/></template><template #preview><div v-if="batchPreview" class="batch-preview">解析预览: <strong style="color:var(--color-info)">{{ batchPreview }}</strong> 条有效记录</div></template></BindModal>
    <BindModal :visible="showMigrateModal" title="迁移用户" icon="🔄" warning="迁移会将用户从当前上游移到新上游" type="warning" :fields="migrateFields" v-model="migrateForm" confirm-text="确认迁移" @confirm="doMigrate" @cancel="showMigrateModal=false"><template #field-upstream="{value,update}"><UpstreamSelect :modelValue="value" @update:modelValue="update"/></template></BindModal>
    <BindModal :visible="showPolicyModal" :title="policyEditIdx>=0?'编辑策略':'新建策略'" :icon="policyEditIdx>=0?'✏️':'➕'" :description="policyEditIdx>=0?'修改路由策略的匹配条件和目标上游':'添加新的路由策略规则'" :fields="policyFields" v-model="policyForm" :confirm-text="policyEditIdx>=0?'保存修改':'创建策略'" @confirm="doPolicySave" @cancel="showPolicyModal=false"><template #field-upstream="{value,update}"><UpstreamSelect :modelValue="value" @update:modelValue="update"/></template></BindModal>
    <ConfirmModal :visible="confirmVisible" :title="confirmTitle" :message="confirmMsg" type="danger" :confirm-text="confirmBtnText" @confirm="doConfirmAction" @cancel="confirmVisible=false"/>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import Icon from '../components/Icon.vue'
import BindModal from '../components/BindModal.vue'
import UpstreamSelect from '../components/UpstreamSelect.vue'

const policies = ref([])
const policiesLoading = ref(false)
const policyTestForm = reactive({ email: '', department: '', app_id: '' })
const policyTestResult = ref(null)
const showPolicyModal = ref(false)
const policyEditIdx = ref(-1)
const policyForm = ref({ matchType: 'department', matchValue: '', upstream: '' })

function gmf(p, field) {
  if (!p) return ''
  const match = p.match || p.Match || {}
  if (field === 'default') return match.default || match.Default || false
  return match[field] || ''
}

const MATCH_TYPE_OPTIONS = [
  { value: 'department', label: '部门' },
  { value: 'email', label: '邮箱' },
  { value: 'email_suffix', label: '邮箱后缀' },
  { value: 'app_id', label: 'App ID' },
  { value: 'default', label: '默认策略' },
]

const policyFields = computed(() => {
  const fields = [{ key: 'matchType', label: '匹配类型', type: 'select', required: true, options: MATCH_TYPE_OPTIONS }]
  if (policyForm.value.matchType !== 'default') {
    const ph = { department: '输入部门名称', email: '输入完整邮箱', email_suffix: '输入邮箱后缀，如 @qianxin.com', app_id: '输入 App ID' }
    fields.push({ key: 'matchValue', label: '匹配值', type: 'text', required: true, placeholder: ph[policyForm.value.matchType] || '' })
  }
  fields.push({ key: 'upstream', label: '目标上游', type: 'component', required: false, hint: '留空表示使用默认上游分配' })
  return fields
})

function openPolicyCreate() { policyEditIdx.value = -1; policyForm.value = { matchType: 'department', matchValue: '', upstream: '' }; showPolicyModal.value = true }
function openPolicyEdit(idx, p) {
  policyEditIdx.value = idx
  const match = p.match || p.Match || {}
  let mt = 'default', mv = ''
  if (match.department) { mt = 'department'; mv = match.department }
  else if (match.email) { mt = 'email'; mv = match.email }
  else if (match.email_suffix) { mt = 'email_suffix'; mv = match.email_suffix }
  else if (match.app_id) { mt = 'app_id'; mv = match.app_id }
  policyForm.value = { matchType: mt, matchValue: mv, upstream: p.upstream_id || '' }
  showPolicyModal.value = true
}

async function doPolicySave() {
  const { matchType, matchValue, upstream } = policyForm.value
  const match = {}
  if (matchType === 'default') { match.default = true }
  else { if (!matchValue || !matchValue.trim()) { showToast('请填写匹配值', 'error'); return }; match[matchType] = matchValue.trim() }
  const body = { match, upstream_id: upstream || '' }
  try {
    let result
    if (policyEditIdx.value >= 0) result = await apiPut('/api/v1/route-policies/' + policyEditIdx.value, body)
    else result = await apiPost('/api/v1/route-policies', body)
    policies.value = result.policies || []
    showToast(policyEditIdx.value >= 0 ? '策略修改成功' : '策略创建成功', 'success')
    showPolicyModal.value = false
  } catch (e) { showToast('保存失败: ' + e.message, 'error') }
}

const confirmVisible = ref(false)
const confirmTitle = ref('确认操作')
const confirmMsg = ref('')
const confirmBtnText = ref('确认')
const pendingConfirmAction = ref(null)
function doConfirmAction() { confirmVisible.value = false; if (pendingConfirmAction.value) { pendingConfirmAction.value(); pendingConfirmAction.value = null } }

function confirmDeletePolicy(idx, p) {
  const match = p.match || p.Match || {}
  let desc = '策略 #' + (idx + 1)
  if (match.department) desc += ' (部门: ' + match.department + ')'
  else if (match.email_suffix) desc += ' (后缀: ' + match.email_suffix + ')'
  else if (match.email) desc += ' (邮箱: ' + match.email + ')'
  else if (match.app_id) desc += ' (App: ' + match.app_id + ')'
  else if (match.default) desc += ' (默认策略)'
  confirmTitle.value = '删除策略'; confirmMsg.value = '确认删除 ' + desc + '？此操作不可撤销。'; confirmBtnText.value = '删除'
  pendingConfirmAction.value = async () => { try { const r = await apiDelete('/api/v1/route-policies/' + idx); policies.value = r.policies || []; showToast('策略删除成功', 'success') } catch (e) { showToast('删除失败: ' + e.message, 'error') } }
  confirmVisible.value = true
}

async function loadPolicies() { policiesLoading.value = true; try { const d = await api('/api/v1/route-policies'); policies.value = d.policies || [] } catch { policies.value = [] } policiesLoading.value = false }
async function testPolicy() {
  if (!policyTestForm.email && !policyTestForm.department && !policyTestForm.app_id) { showToast('请至少填写一个匹配条件', 'error'); return }
  try { policyTestResult.value = await apiPost('/api/v1/route-policies/test', { email: policyTestForm.email, department: policyTestForm.department, app_id: policyTestForm.app_id }) } catch (e) { showToast('测试失败: ' + e.message, 'error') }
}

const loading = ref(false)
const allRoutes = ref([])
const userCache = ref({})
const routeStats = ref(null)
const filterApp = ref('')
const filterDept = ref('')
const searchText = ref('')
const showBindModal = ref(false)
const showBatchModal = ref(false)
const showMigrateModal = ref(false)
const bindForm = ref({ sender: '', app: '', upstream: '', name: '', dept: '' })
const batchForm = ref({ app: '', upstream: '', text: '' })
const migrateForm = ref({ sender: '', app: '', upstream: '' })
const pendingUnbind = ref(null)

const bindFields = [
  { key: 'sender', label: '用户 ID', type: 'text', required: true, placeholder: '输入用户 ID' },
  { key: 'app', label: 'App ID (Bot)', type: 'text', placeholder: '输入 App ID（可选）' },
  { key: 'upstream', label: '目标上游', type: 'component', required: true },
  { key: 'name', label: '显示名', type: 'text', placeholder: '用户姓名（可选）' },
  { key: 'dept', label: '部门', type: 'text', placeholder: '所属部门（可选）' },
]
const batchFields = [
  { key: 'app', label: 'App ID (Bot)', type: 'text', placeholder: '输入 App ID（可选）' },
  { key: 'upstream', label: '目标上游', type: 'component', required: true },
  { key: 'text', label: '用户列表', type: 'textarea', required: true, placeholder: '每行: 用户ID,显示名,部门', rows: 6, hint: '格式: 用户ID,显示名,部门' },
]
const migrateFields = [
  { key: 'sender', label: '用户 ID', type: 'text', required: true, placeholder: '输入要迁移的用户 ID' },
  { key: 'app', label: 'App ID', type: 'text', placeholder: '输入 App ID（可选）' },
  { key: 'upstream', label: '目标上游', type: 'component', required: true },
]

const batchPreview = computed(() => { const t = batchForm.value.text; if (!t || !t.trim()) return null; return t.trim().split('\n').filter(l => l.split(',')[0]?.trim()).length })
const columns = [
  { key: 'sender_id', label: '用户 ID', sortable: true },
  { key: 'display_name', label: '姓名', sortable: true },
  { key: 'department', label: '部门', sortable: true },
  { key: 'app_id', label: 'Bot', sortable: true },
  { key: 'upstream_id', label: '上游', sortable: true },
]
const apps = computed(() => { const s = new Set(); allRoutes.value.forEach(r => { if (r.app_id) s.add(r.app_id) }); return [...s].sort() })
const depts = computed(() => { const s = new Set(); allRoutes.value.forEach(r => { if (r.department) s.add(r.department) }); return [...s].sort() })
const filteredRoutes = computed(() => {
  let list = allRoutes.value
  if (filterApp.value) list = list.filter(r => r.app_id === filterApp.value)
  if (filterDept.value) list = list.filter(r => r.department === filterDept.value)
  if (searchText.value) { const q = searchText.value.toLowerCase(); list = list.filter(r => (r.sender_id||'').toLowerCase().includes(q) || (r.display_name||'').toLowerCase().includes(q)) }
  return list.map(r => { const u = userCache.value[r.sender_id] || {}; return { ...r, display_name: u.name || r.display_name || '--', department: u.department || r.department || '--' } })
})
function getUserInfo(row, field) {
  const u = userCache.value[row.sender_id] || {}
  if (field === 'name') return u.name || row.display_name || '--'
  if (field === 'email') return u.email || '--'
  if (field === 'mobile') return u.mobile || '--'
  if (field === 'department') return u.department || row.department || '--'
  return '--'
}

async function loadRoutes() { loading.value = true; try { allRoutes.value = (await api('/api/v1/routes')).routes || [] } catch { allRoutes.value = [] } loading.value = false }
async function loadRouteStats() { try { const d = await api('/api/v1/routes/stats'); routeStats.value = { appCount: d.by_app ? Object.keys(d.by_app).length : 0, senderCount: d.unique_senders || 0, upstreamCount: d.by_upstream ? Object.keys(d.by_upstream).length : 0, total: d.total || 0 } } catch {} }
async function loadUsers() { try { const d = await api('/api/v1/users'); const m = {}; (d.users || []).forEach(u => { m[u.sender_id] = u }); userCache.value = m } catch {} }
function refresh() { loadRoutes(); loadRouteStats(); loadUsers(); loadPolicies() }

function confirmUnbind(row) {
  pendingUnbind.value = row; confirmTitle.value = '确认解绑'; confirmMsg.value = '确认解绑用户 ' + row.sender_id + ' (' + (row.display_name || '--') + ') ?'; confirmBtnText.value = '解绑'
  pendingConfirmAction.value = async () => { try { await apiPost('/api/v1/routes/unbind', { sender_id: row.sender_id, app_id: row.app_id }); showToast('解绑成功', 'success'); refresh() } catch (e) { showToast('解绑失败: ' + e.message, 'error') } }
  confirmVisible.value = true
}
async function doBind(data) {
  const body = { sender_id: data.sender, upstream_id: data.upstream }
  if (data.app) body.app_id = data.app; if (data.name) body.display_name = data.name; if (data.dept) body.department = data.dept
  try { await apiPost('/api/v1/routes/bind', body); showToast('绑定成功', 'success'); showBindModal.value = false; bindForm.value = { sender: '', app: '', upstream: '', name: '', dept: '' }; refresh() } catch (e) { showToast('绑定失败: ' + e.message, 'error') }
}
async function doBatchBind(data) {
  if (!data.upstream) { showToast('请选择上游', 'error'); return }
  const lines = data.text.trim().split('\n').filter(l => l.trim())
  if (!lines.length) { showToast('请输入用户列表', 'error'); return }
  const entries = lines.map(l => { const p = l.split(','); return { sender_id: p[0]?.trim(), display_name: p[1]?.trim(), department: p[2]?.trim() } }).filter(e => e.sender_id)
  try { const d = await apiPost('/api/v1/routes/batch-bind', { app_id: data.app, upstream_id: data.upstream, entries }); showToast('批量绑定 ' + (d.count || entries.length) + ' 条成功', 'success'); showBatchModal.value = false; batchForm.value = { app: '', upstream: '', text: '' }; refresh() } catch (e) { showToast('批量绑定失败: ' + e.message, 'error') }
}
async function doMigrate(data) {
  if (!data.sender || !data.upstream) { showToast('请填写用户ID和目标上游', 'error'); return }
  const body = { sender_id: data.sender, to: data.upstream }; if (data.app) body.app_id = data.app
  try { await apiPost('/api/v1/routes/migrate', body); showToast('迁移成功', 'success'); showMigrateModal.value = false; migrateForm.value = { sender: '', app: '', upstream: '' }; refresh() } catch (e) { showToast('迁移失败: ' + e.message, 'error') }
}
function openMigrateFor(row) { migrateForm.value = { sender: row.sender_id, app: row.app_id || '', upstream: '' }; showMigrateModal.value = true }

onMounted(refresh)
</script>

<style scoped>
.policy-table { width: 100%; border-collapse: collapse; }
.policy-table th { text-align: left; padding: 10px 14px; color: var(--text-secondary); border-bottom: 1px solid var(--border-default); font-weight: 500; font-size: .78rem; text-transform: uppercase; }
.policy-table td { padding: 10px 14px; border-bottom: 1px solid var(--border-subtle); color: var(--text-primary); font-size: .85rem; }
.policy-table tr:hover td { background: var(--bg-hover); }
.policy-table tr.policy-matched td { background: rgba(34,197,94,.08) !important; border-color: rgba(34,197,94,.15); }
.policy-conditions { display: flex; gap: 6px; flex-wrap: wrap; }
.policy-empty { text-align: center; padding: 32px; }
.policy-test-section { padding: 16px; border-top: 1px solid var(--border-subtle); margin-top: 4px; }
.policy-test-header { display: flex; align-items: center; gap: 8px; font-size: var(--text-sm); color: var(--color-primary); font-weight: 600; margin-bottom: 10px; }
.policy-test-form { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
.policy-test-form input { background: var(--bg-elevated); color: var(--text-primary); border: 1px solid var(--border-default); border-radius: var(--radius-md); padding: 6px 10px; font-size: .82rem; outline: none; transition: border-color var(--transition-fast); }
.policy-test-form input:focus { border-color: var(--color-primary); box-shadow: 0 0 0 3px var(--color-primary-dim); }
.policy-test-result { margin-top: 10px; padding: 10px 14px; background: var(--bg-elevated); border-radius: var(--radius-md); border-left: 3px solid var(--color-danger); font-size: .85rem; }
.policy-test-result.matched { border-left-color: var(--color-success); }
.route-stats-bar { display: flex; align-items: center; gap: 0; margin-bottom: 16px; padding: 12px 16px; background: var(--bg-elevated); border-radius: var(--radius-md); }
.route-stat-item { display: flex; align-items: center; gap: 8px; flex: 1; justify-content: center; }
.route-stat-label { font-size: var(--text-xs); color: var(--text-tertiary); }
.route-stat-value { font-size: var(--text-lg); font-weight: 700; font-family: var(--font-mono); }
.route-stat-divider { width: 1px; height: 24px; background: var(--border-default); }
.search-input-wrap { position: relative; flex: 2; min-width: 150px; }
.search-input-wrap svg { position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: var(--text-tertiary); pointer-events: none; }
.search-input-wrap input { width: 100%; padding-left: 32px; background: var(--bg-elevated); border: 1px solid var(--border-default); border-radius: var(--radius-md); color: var(--text-primary); padding-top: 8px; padding-bottom: 8px; padding-right: 12px; font-size: var(--text-sm); outline: none; }
.search-input-wrap input:focus { border-color: var(--color-primary); }
.expand-label { color: var(--text-tertiary); font-size: var(--text-xs); display: block; margin-bottom: 2px; }
.expand-value { color: var(--text-primary); font-weight: 500; }
.batch-preview { margin-top: 12px; padding: 10px 14px; background: var(--bg-elevated); border-radius: var(--radius-md); font-size: var(--text-sm); color: var(--text-secondary); display: flex; align-items: center; gap: 8px; }
</style>
