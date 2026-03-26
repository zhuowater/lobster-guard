<template>
  <div class="apikeys-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="key" :size="20" /> API Key 管理</h1>
        <p class="page-desc">管理 API Key 与用户身份绑定 — 追踪谁在使用 AI 模型</p>
      </div>
      <button class="btn-primary" @click="showCreate=true"><Icon name="plus" :size="14" /> 创建 API Key</button>
    </div>

    <!-- 统计卡片 -->
    <div class="stat-row">
      <div class="stat-card"><span class="stat-value">{{ keys.length }}</span><span class="stat-label">总密钥数</span></div>
      <div class="stat-card"><span class="stat-value accent">{{ keys.filter(k=>k.status==='active'||(!k.status&&k.enabled)).length }}</span><span class="stat-label">已绑定</span></div>
      <div class="stat-card"><span class="stat-value pending-val">{{ pendingKeys.length }}</span><span class="stat-label">待绑定</span></div>
      <div class="stat-card"><span class="stat-value">{{ uniqueDepts }}</span><span class="stat-label">覆盖部门</span></div>
    </div>

    <!-- 🔍 待绑定 API Key -->
    <div class="pending-section" v-if="pendingKeys.length">
      <div class="pending-header"><span class="pending-icon">🔍</span> 自动发现的 API Key（待绑定）<span class="pending-count">{{ pendingKeys.length }}</span></div>
      <div class="pending-list">
        <div class="pending-card" v-for="pk in pendingKeys" :key="pk.id">
          <div class="pending-info">
            <code class="key-prefix">{{ pk.key_prefix }}...</code>
            <span class="pending-meta">发现于 {{ fmtTime(pk.discovered_at||pk.created_at) }}</span>
            <span class="pending-meta">{{ pk.request_count||0 }} 次请求</span>
            <span class="pending-meta" v-if="pk.last_seen_at">最后活动 {{ fmtTime(pk.last_seen_at) }}</span>
          </div>
          <button class="btn-primary btn-sm" @click="openBind(pk)">绑定用户</button>
        </div>
      </div>
    </div>

    <!-- 搜索与筛选 -->
    <div class="filter-bar">
      <div class="search-wrap"><Icon name="search" :size="14" /><input v-model="search" placeholder="搜索用户/部门/Key前缀..." class="search-input" /></div>
      <select v-model="filterTenant" class="filter-select"><option value="">全部租户</option><option v-for="t in tenants" :key="t" :value="t">{{ t }}</option></select>
    </div>

    <!-- 密钥列表 -->
    <div class="table-wrap">
      <table class="data-table">
        <thead><tr>
          <th>Key 前缀</th><th>用户</th><th>部门</th><th>租户</th>
          <th>日配额</th><th>今日用量</th><th>状态</th><th>最后使用</th><th>操作</th>
        </tr></thead>
        <tbody>
          <tr v-for="k in filtered" :key="k.id" :class="{'row-disabled':!k.enabled}">
            <td><code class="key-prefix">{{ k.key_prefix }}</code></td>
            <td><strong>{{ k.user_name || k.user_id }}</strong><div class="sub-text" v-if="k.user_name">{{ k.user_id }}</div></td>
            <td>{{ k.department || '-' }}</td>
            <td><span class="badge-tenant">{{ k.tenant_id }}</span></td>
            <td>{{ k.quota_daily || '∞' }}</td>
            <td><span :class="quotaClass(k)">{{ k.used_today || 0 }}</span></td>
            <td><span v-if="k.status==='pending'" class="badge-pending">待绑定</span><span v-else :class="k.enabled?'badge-on':'badge-off'">{{ k.enabled?'启用':'禁用' }}</span></td>
            <td class="td-time">{{ fmtTime(k.last_used_at) }}</td>
            <td class="td-actions">
              <button class="btn-xs btn-outline" @click="openEdit(k)">编辑</button>
              <button class="btn-xs btn-warn" @click="rotateKey(k)">轮换</button>
              <button class="btn-xs btn-danger" @click="deleteKey(k)">删除</button>
            </td>
          </tr>
          <tr v-if="filtered.length===0"><td colspan="9" class="empty">暂无 API Key，点击"创建"开始</td></tr>
        </tbody>
      </table>
    </div>

    <!-- 创建弹窗 -->
    <div class="modal-overlay" v-if="showCreate" @click.self="showCreate=false">
      <div class="modal">
        <h2>创建 API Key</h2>
        <div class="form-group"><label>用户 ID *</label><input v-model="form.user_id" placeholder="工号或邮箱" /></div>
        <div class="form-group"><label>用户名</label><input v-model="form.user_name" placeholder="姓名" /></div>
        <div class="form-group"><label>部门</label><input v-model="form.department" placeholder="所属部门" /></div>
        <div class="form-row">
          <div class="form-group"><label>租户</label><input v-model="form.tenant_id" placeholder="default" /></div>
          <div class="form-group"><label>日配额</label><input v-model.number="form.quota_daily" type="number" placeholder="0=不限" /></div>
        </div>
        <div class="form-group"><label>过期时间</label><input v-model="form.expires_at" type="datetime-local" /></div>
        <div class="modal-actions">
          <button class="btn-outline" @click="showCreate=false">取消</button>
          <button class="btn-primary" @click="createKey" :disabled="!form.user_id||creating">{{ creating?'创建中...':'创建' }}</button>
        </div>
        <!-- 创建成功后显示密钥 -->
        <div class="key-reveal" v-if="newRawKey">
          <div class="key-reveal-title">⚠️ 请妥善保管，仅显示一次</div>
          <code class="key-reveal-value">{{ newRawKey }}</code>
          <button class="btn-xs btn-outline" @click="copyKey">复制</button>
        </div>
      </div>
    </div>

    <!-- 编辑弹窗 -->
    <div class="modal-overlay" v-if="showEdit" @click.self="showEdit=false">
      <div class="modal">
        <h2>编辑 API Key</h2>
        <div class="form-group"><label>用户名</label><input v-model="editForm.user_name" /></div>
        <div class="form-group"><label>部门</label><input v-model="editForm.department" /></div>
        <div class="form-row">
          <div class="form-group"><label>租户</label><input v-model="editForm.tenant_id" /></div>
          <div class="form-group"><label>日配额</label><input v-model.number="editForm.quota_daily" type="number" /></div>
        </div>
        <div class="form-group"><label>状态</label><label class="check-label"><input type="checkbox" v-model="editForm.enabled" /> 启用</label></div>
        <div class="modal-actions">
          <button class="btn-outline" @click="showEdit=false">取消</button>
          <button class="btn-primary" @click="saveEdit" :disabled="saving">{{ saving?'保存中...':'保存' }}</button>
        </div>
      </div>
    </div>
    <!-- 绑定弹窗 -->
    <div class="modal-overlay" v-if="showBind" @click.self="showBind=false">
      <div class="modal">
        <h2>🔗 绑定 API Key 到用户</h2>
        <div class="bind-key-info"><code>{{ bindForm.key_prefix }}...</code> — 已发出 {{ bindForm.request_count||0 }} 次请求</div>
        <div class="form-group"><label>用户 ID *</label><input v-model="bindForm.user_id" placeholder="工号或邮箱" /></div>
        <div class="form-group"><label>用户名</label><input v-model="bindForm.user_name" placeholder="姓名" /></div>
        <div class="form-group"><label>部门</label><input v-model="bindForm.department" placeholder="所属部门" /></div>
        <div class="form-group"><label>租户</label>
          <select v-model="bindForm.tenant_id" class="form-select">
            <option value="default">默认租户</option>
            <option v-for="t in allTenants" :key="t.id" :value="t.id">{{ t.name }} ({{ t.id }})</option>
          </select>
        </div>
        <div class="modal-actions">
          <button class="btn-outline" @click="showBind=false">取消</button>
          <button class="btn-primary" @click="doBind" :disabled="!bindForm.user_id||binding">{{ binding?'绑定中...':'确认绑定' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api'
import Icon from '../components/Icon.vue'

const keys = ref([])
const search = ref('')
const filterTenant = ref('')
const showCreate = ref(false)
const showEdit = ref(false)
const creating = ref(false)
const saving = ref(false)
const newRawKey = ref('')
const form = ref({ user_id:'', user_name:'', department:'', tenant_id:'default', quota_daily:0, expires_at:'' })
const editForm = ref({})
let editingId = ''

// v27.1: 自动发现绑定
const showBind = ref(false)
const binding = ref(false)
const bindForm = ref({ id:'', key_prefix:'', request_count:0, user_id:'', user_name:'', department:'', tenant_id:'default' })
const allTenants = ref([])

const pendingKeys = computed(()=>keys.value.filter(k=>k.status==='pending'))

const tenants = computed(()=>[...new Set(keys.value.map(k=>k.tenant_id))])
const uniqueDepts = computed(()=>new Set(keys.value.map(k=>k.department).filter(Boolean)).size)
const filtered = computed(()=>{
  let list = keys.value
  if (filterTenant.value) list = list.filter(k=>k.tenant_id===filterTenant.value)
  if (search.value) {
    const q = search.value.toLowerCase()
    list = list.filter(k=>(k.user_id+k.user_name+k.department+k.key_prefix).toLowerCase().includes(q))
  }
  return list
})

function quotaClass(k) { return k.quota_daily>0 && k.used_today>=k.quota_daily ? 'usage-exceeded' : '' }
function fmtTime(t) { if (!t) return '-'; try { return new Date(t).toLocaleString('zh-CN',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}) } catch { return t } }

async function loadKeys() { try { const d=await api('/api/v1/apikeys'); keys.value=d.keys||[] } catch(e) { console.error(e) } }

async function createKey() {
  creating.value=true; newRawKey.value=''
  try {
    const body = { ...form.value }
    if (body.expires_at) body.expires_at = new Date(body.expires_at).toISOString()
    else delete body.expires_at
    const d = await apiPost('/api/v1/apikeys', body)
    newRawKey.value = d.raw_key
    loadKeys()
  } catch(e) { alert('创建失败: '+e.message) }
  creating.value=false
}

function openEdit(k) { editingId=k.id; editForm.value={user_name:k.user_name,department:k.department,tenant_id:k.tenant_id,quota_daily:k.quota_daily,enabled:k.enabled}; showEdit.value=true }
async function saveEdit() {
  saving.value=true
  try { await apiPut('/api/v1/apikeys/'+editingId, editForm.value); showEdit.value=false; loadKeys() } catch(e) { alert('保存失败: '+e.message) }
  saving.value=false
}

async function rotateKey(k) {
  if (!confirm(`确定轮换 ${k.key_prefix}... 的密钥？旧密钥将立即失效。`)) return
  try { const d=await apiPost('/api/v1/apikeys/'+k.id+'/rotate',{}); alert('新密钥: '+d.raw_key+'\n请妥善保管！'); loadKeys() } catch(e) { alert('轮换失败: '+e.message) }
}

async function deleteKey(k) {
  if (!confirm(`确定删除 ${k.user_name||k.user_id} 的 API Key？`)) return
  try { await apiDelete('/api/v1/apikeys/'+k.id); loadKeys() } catch(e) { alert('删除失败: '+e.message) }
}

function copyKey() { navigator.clipboard.writeText(newRawKey.value).then(()=>alert('已复制到剪贴板')) }

// v27.1: 绑定逻辑
function openBind(pk) {
  bindForm.value = { id:pk.id, key_prefix:pk.key_prefix, request_count:pk.request_count||0, user_id:'', user_name:'', department:'', tenant_id:'default' }
  showBind.value = true
}
async function doBind() {
  binding.value = true
  try {
    await apiPost('/api/v1/apikeys/'+bindForm.value.id+'/bind', {
      user_id: bindForm.value.user_id, user_name: bindForm.value.user_name,
      department: bindForm.value.department, tenant_id: bindForm.value.tenant_id
    })
    showBind.value = false; loadKeys()
  } catch(e) { alert('绑定失败: '+e.message) }
  binding.value = false
}
async function loadTenantList() { try { const d=await api('/api/v1/tenants'); allTenants.value=d.tenants||[] } catch{} }

onMounted(()=>{ loadKeys(); loadTenantList() })
</script>

<style scoped>
.apikeys-page { padding: var(--space-4); max-width: 1400px; }
.page-header { display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:var(--space-4); }
.page-title { font-size:1.5rem; font-weight:700; color:var(--text-primary); display:flex; align-items:center; gap:var(--space-2); margin:0; }
.page-desc { color:var(--text-secondary); margin:var(--space-1) 0 0; font-size:0.875rem; }
.stat-row { display:grid; grid-template-columns:repeat(4,1fr); gap:var(--space-3); margin-bottom:var(--space-4); }
.stat-card { background:var(--bg-card); border:1px solid var(--border); border-radius:var(--radius); padding:var(--space-3); text-align:center; }
.stat-value { display:block; font-size:1.75rem; font-weight:700; color:var(--text-primary); }
.stat-value.accent { color:var(--accent); }
.stat-value.warn { color:var(--warning); }
.stat-label { font-size:0.75rem; color:var(--text-secondary); text-transform:uppercase; letter-spacing:0.05em; }
.filter-bar { display:flex; gap:var(--space-2); margin-bottom:var(--space-3); }
.search-wrap { display:flex; align-items:center; gap:var(--space-1); background:var(--bg-card); border:1px solid var(--border); border-radius:var(--radius); padding:0 var(--space-2); flex:1; }
.search-input { border:none; background:transparent; outline:none; padding:var(--space-2) 0; width:100%; color:var(--text-primary); }
.filter-select { background:var(--bg-card); border:1px solid var(--border); border-radius:var(--radius); padding:var(--space-2); color:var(--text-primary); }
.table-wrap { overflow-x:auto; }
.data-table { width:100%; border-collapse:collapse; }
.data-table th { text-align:left; padding:var(--space-2); font-size:0.75rem; text-transform:uppercase; color:var(--text-secondary); border-bottom:2px solid var(--border); }
.data-table td { padding:var(--space-2); border-bottom:1px solid var(--border); font-size:0.875rem; }
.key-prefix { background:var(--bg-muted); padding:2px 6px; border-radius:4px; font-size:0.8rem; }
.sub-text { font-size:0.75rem; color:var(--text-secondary); }
.badge-tenant { background:var(--accent-muted,#EEF2FF); color:var(--accent); padding:2px 8px; border-radius:10px; font-size:0.75rem; }
.badge-on { color:var(--success); font-weight:600; }
.badge-off { color:var(--text-secondary); }
.usage-exceeded { color:var(--danger); font-weight:700; }
.td-time { font-size:0.8rem; color:var(--text-secondary); white-space:nowrap; }
.td-actions { white-space:nowrap; display:flex; gap:4px; }
.row-disabled { opacity:0.5; }
.empty { text-align:center; color:var(--text-secondary); padding:var(--space-6)!important; }
.modal-overlay { position:fixed; inset:0; background:rgba(0,0,0,0.5); display:flex; align-items:center; justify-content:center; z-index:100; }
.modal { background:var(--bg-card); border-radius:var(--radius-lg,12px); padding:var(--space-4); width:480px; max-width:90vw; }
.modal h2 { margin:0 0 var(--space-3); font-size:1.25rem; }
.form-group { margin-bottom:var(--space-3); }
.form-group label { display:block; font-size:0.8rem; color:var(--text-secondary); margin-bottom:4px; }
.form-group input { width:100%; padding:var(--space-2); border:1px solid var(--border); border-radius:var(--radius); background:var(--bg-main); color:var(--text-primary); }
.form-row { display:grid; grid-template-columns:1fr 1fr; gap:var(--space-2); }
.modal-actions { display:flex; justify-content:flex-end; gap:var(--space-2); margin-top:var(--space-3); }
.check-label { display:flex; align-items:center; gap:var(--space-1); cursor:pointer; }
.key-reveal { margin-top:var(--space-3); padding:var(--space-3); background:var(--bg-muted); border-radius:var(--radius); border:1px solid var(--warning); }
.key-reveal-title { font-size:0.85rem; color:var(--warning); font-weight:600; margin-bottom:var(--space-1); }
.key-reveal-value { display:block; word-break:break-all; font-size:0.8rem; margin-bottom:var(--space-2); }
.btn-primary { background:var(--accent); color:#fff; border:none; padding:var(--space-2) var(--space-3); border-radius:var(--radius); cursor:pointer; font-weight:600; display:flex; align-items:center; gap:var(--space-1); }
.btn-primary:disabled { opacity:0.5; cursor:not-allowed; }
.btn-outline { background:transparent; border:1px solid var(--border); color:var(--text-primary); padding:var(--space-2) var(--space-3); border-radius:var(--radius); cursor:pointer; }
.btn-xs { padding:4px 8px; font-size:0.75rem; border-radius:4px; cursor:pointer; border:1px solid var(--border); background:transparent; color:var(--text-primary); }
.btn-warn { border-color:var(--warning); color:var(--warning); }
.btn-danger { border-color:var(--danger); color:var(--danger); }
.btn-sm { padding:6px 12px; font-size:0.8rem; }
.pending-val { color:#F59E0B; }
.badge-pending { color:#F59E0B; font-weight:700; background:rgba(245,158,11,0.1); padding:2px 8px; border-radius:10px; font-size:0.75rem; }
.pending-section { margin-bottom:var(--space-4); background:rgba(245,158,11,0.05); border:1px solid rgba(245,158,11,0.3); border-radius:var(--radius-lg,12px); padding:var(--space-3); }
.pending-header { font-size:1rem; font-weight:700; color:#F59E0B; display:flex; align-items:center; gap:var(--space-2); margin-bottom:var(--space-2); }
.pending-icon { font-size:1.2em; }
.pending-count { background:#F59E0B; color:#000; font-size:0.75rem; padding:2px 8px; border-radius:10px; font-weight:700; }
.pending-list { display:flex; flex-direction:column; gap:var(--space-2); }
.pending-card { display:flex; align-items:center; justify-content:space-between; padding:var(--space-2) var(--space-3); background:var(--bg-card); border:1px solid var(--border); border-radius:var(--radius); border-left:3px solid #F59E0B; }
.pending-info { display:flex; align-items:center; gap:var(--space-3); flex-wrap:wrap; }
.pending-meta { font-size:0.75rem; color:var(--text-secondary); }
.bind-key-info { padding:var(--space-2); background:var(--bg-muted); border-radius:var(--radius); margin-bottom:var(--space-3); font-size:0.85rem; color:var(--text-secondary); }
.form-select { width:100%; padding:var(--space-2); border:1px solid var(--border); border-radius:var(--radius); background:var(--bg-main); color:var(--text-primary); }
</style>
