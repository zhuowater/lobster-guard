<template>
  <div class="card">
    <div class="card-header">
      <span class="card-icon">🗺️</span>
      <span class="card-title">亲和路由管理</span>
      <div class="card-actions">
        <button class="btn btn-sm" @click="refresh">🔄 刷新</button>
      </div>
    </div>

    <!-- Filters -->
    <div class="filters">
      <select v-model="filterApp" @change="applyFilter">
        <option value="">全部 Bot</option>
        <option v-for="a in apps" :key="a" :value="a">{{ a.length > 20 ? a.substring(0, 20) + '...' : a }}</option>
      </select>
      <select v-model="filterDept" @change="applyFilter">
        <option value="">全部部门</option>
        <option v-for="d in depts" :key="d" :value="d">{{ d }}</option>
      </select>
      <input type="text" v-model="searchText" placeholder="搜索用户ID/名称..." style="flex:2;min-width:150px" />
    </div>

    <!-- Stats bar -->
    <div v-if="routeStats" style="display:flex;gap:16px;margin-bottom:12px;flex-wrap:wrap">
      <span class="tag tag-pass" style="font-size:.8rem">Bot 数: {{ routeStats.appCount }}</span>
      <span class="tag tag-pass" style="font-size:.8rem">用户数: {{ routeStats.senderCount }}</span>
      <span class="tag tag-info" style="font-size:.8rem">上游数: {{ routeStats.upstreamCount }}</span>
      <span class="tag" style="font-size:.8rem;background:rgba(255,204,0,.15);color:var(--neon-yellow)">路由数: {{ routeStats.total }}</span>
    </div>

    <!-- DataTable -->
    <DataTable
      :columns="columns"
      :data="filteredRoutes"
      :loading="loading"
      empty-text="暂无匹配路由"
      empty-icon="🗺️"
      :expandable="true"
    >
      <template #empty-hint>请先绑定用户到上游</template>
      <template #cell-sender_id="{ row }">
        <span style="font-size:.75rem">{{ row.sender_id }}</span>
      </template>
      <template #cell-app_id="{ row }">
        <span style="font-size:.75rem" :title="row.app_id">{{ (row.app_id || '--').length > 16 ? row.app_id.substring(0, 16) + '...' : (row.app_id || '--') }}</span>
      </template>
      <template #expand="{ row }">
        <div style="display:flex;gap:20px;flex-wrap:wrap;font-size:.82rem">
          <div><b style="color:var(--neon-blue)">用户ID:</b> {{ row.sender_id }}</div>
          <div><b style="color:var(--neon-blue)">姓名:</b> {{ getUserInfo(row, 'name') }}</div>
          <div><b style="color:var(--neon-blue)">邮箱:</b> {{ getUserInfo(row, 'email') }}</div>
          <div><b style="color:var(--neon-blue)">手机:</b> {{ getUserInfo(row, 'mobile') }}</div>
          <div><b style="color:var(--neon-blue)">部门:</b> {{ getUserInfo(row, 'department') }}</div>
          <div><b style="color:var(--neon-blue)">App ID:</b> {{ row.app_id || '--' }}</div>
          <div><b style="color:var(--neon-blue)">上游:</b> {{ row.upstream_id }}</div>
        </div>
      </template>
      <template #actions="{ row }">
        <button class="btn btn-sm btn-red" @click.stop="confirmUnbind(row)">解绑</button>
      </template>
    </DataTable>

    <!-- Bind forms -->
    <details style="margin-top:12px">
      <summary style="cursor:pointer;color:var(--neon-green);font-weight:600">➕ 单用户绑定</summary>
      <div class="inline-form" style="margin-top:8px">
        <input v-model="bindForm.sender" placeholder="用户 ID" style="flex:1;min-width:100px" />
        <input v-model="bindForm.app" placeholder="App ID (Bot)" style="flex:1;min-width:100px" />
        <input v-model="bindForm.upstream" placeholder="上游 ID" style="flex:1;min-width:100px" />
        <input v-model="bindForm.name" placeholder="显示名 (可选)" style="flex:1;min-width:80px" />
        <input v-model="bindForm.dept" placeholder="部门 (可选)" style="flex:1;min-width:80px" />
        <button class="btn btn-green" @click="bindRoute">绑定</button>
      </div>
    </details>

    <details style="margin-top:8px">
      <summary style="cursor:pointer;color:var(--neon-yellow);font-weight:600">📋 批量绑定</summary>
      <div style="margin-top:8px">
        <div class="inline-form" style="margin-bottom:8px">
          <input v-model="batchForm.app" placeholder="App ID (Bot)" style="flex:1" />
          <input v-model="batchForm.upstream" placeholder="上游 ID" style="flex:1" />
        </div>
        <textarea v-model="batchForm.text" rows="4" placeholder="每行一条: 用户ID,显示名,部门" style="width:100%;background:rgba(0,0,0,.3);color:var(--text);border:1px solid rgba(0,212,255,.2);border-radius:6px;padding:8px;font-size:.8rem;font-family:monospace;resize:vertical"></textarea>
        <button class="btn btn-green" @click="batchBind" style="margin-top:8px">批量绑定</button>
      </div>
    </details>

    <details style="margin-top:8px">
      <summary style="cursor:pointer;color:var(--neon-blue);font-weight:600">🔀 迁移用户</summary>
      <div class="inline-form" style="margin-top:8px">
        <input v-model="migrateForm.sender" placeholder="用户 ID" />
        <input v-model="migrateForm.app" placeholder="App ID (可选)" />
        <input v-model="migrateForm.upstream" placeholder="目标上游 ID" />
        <button class="btn" @click="migrateRoute">迁移</button>
      </div>
    </details>

    <!-- Confirm modal -->
    <ConfirmModal
      :visible="confirmVisible"
      :title="'确认解绑'"
      :message="confirmMsg"
      type="danger"
      confirm-text="解绑"
      @confirm="doUnbind"
      @cancel="confirmVisible = false"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, inject } from 'vue'
import { api, apiPost } from '../api.js'
import { showToast } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const loading = ref(false)
const allRoutes = ref([])
const userCache = ref({})
const routeStats = ref(null)
const filterApp = ref('')
const filterDept = ref('')
const searchText = ref('')

const bindForm = reactive({ sender: '', app: '', upstream: '', name: '', dept: '' })
const batchForm = reactive({ app: '', upstream: '', text: '' })
const migrateForm = reactive({ sender: '', app: '', upstream: '' })

const confirmVisible = ref(false)
const confirmMsg = ref('')
const pendingUnbind = ref(null)

const columns = [
  { key: 'sender_id', label: '用户 ID', sortable: true },
  { key: 'display_name', label: '姓名', sortable: true },
  { key: 'department', label: '部门', sortable: true },
  { key: 'app_id', label: 'Bot', sortable: true },
  { key: 'upstream_id', label: '上游', sortable: true },
]

const apps = computed(() => {
  const set = new Set()
  allRoutes.value.forEach(r => { if (r.app_id) set.add(r.app_id) })
  return [...set].sort()
})
const depts = computed(() => {
  const set = new Set()
  allRoutes.value.forEach(r => { if (r.department) set.add(r.department) })
  return [...set].sort()
})

const filteredRoutes = computed(() => {
  let list = allRoutes.value
  if (filterApp.value) list = list.filter(r => r.app_id === filterApp.value)
  if (filterDept.value) list = list.filter(r => r.department === filterDept.value)
  if (searchText.value) {
    const q = searchText.value.toLowerCase()
    list = list.filter(r => (r.sender_id || '').toLowerCase().includes(q) || (r.display_name || '').toLowerCase().includes(q))
  }
  return list.map(r => {
    const u = userCache.value[r.sender_id] || {}
    return { ...r, display_name: u.name || r.display_name || '--', department: u.department || r.department || '--' }
  })
})

function getUserInfo(row, field) {
  const u = userCache.value[row.sender_id] || {}
  if (field === 'name') return u.name || row.display_name || '--'
  if (field === 'email') return u.email || '--'
  if (field === 'mobile') return u.mobile || '--'
  if (field === 'department') return u.department || row.department || '--'
  return '--'
}

async function loadRoutes() {
  loading.value = true
  try { const d = await api('/api/v1/routes'); allRoutes.value = d.routes || [] } catch { allRoutes.value = [] }
  loading.value = false
}
async function loadRouteStats() {
  try {
    const d = await api('/api/v1/routes/stats')
    routeStats.value = {
      appCount: d.by_app ? Object.keys(d.by_app).length : 0,
      senderCount: d.unique_senders || 0,
      upstreamCount: d.by_upstream ? Object.keys(d.by_upstream).length : 0,
      total: d.total || 0,
    }
  } catch { /* ignore */ }
}
async function loadUsers() {
  try { const d = await api('/api/v1/users'); const list = d.users || []; const m = {}; list.forEach(u => { m[u.sender_id] = u }); userCache.value = m } catch { /* ignore */ }
}

function refresh() { loadRoutes(); loadRouteStats(); loadUsers() }

function confirmUnbind(row) {
  pendingUnbind.value = row
  confirmMsg.value = `确认解绑用户 ${row.sender_id} (${row.display_name || '--'}) ?`
  confirmVisible.value = true
}

async function doUnbind() {
  const row = pendingUnbind.value
  confirmVisible.value = false
  if (!row) return
  try {
    await apiPost('/api/v1/routes/unbind', { sender_id: row.sender_id, app_id: row.app_id })
    showToast('解绑成功', 'success')
    refresh()
  } catch (e) { showToast('解绑失败: ' + e.message, 'error') }
}

async function bindRoute() {
  if (!bindForm.sender || !bindForm.upstream) { showToast('请填写用户ID和上游ID', 'error'); return }
  const body = { sender_id: bindForm.sender, upstream_id: bindForm.upstream }
  if (bindForm.app) body.app_id = bindForm.app
  if (bindForm.name) body.display_name = bindForm.name
  if (bindForm.dept) body.department = bindForm.dept
  try { await apiPost('/api/v1/routes/bind', body); showToast('绑定成功', 'success'); refresh() } catch (e) { showToast('绑定失败: ' + e.message, 'error') }
}

async function batchBind() {
  if (!batchForm.upstream) { showToast('请填写上游ID', 'error'); return }
  const lines = batchForm.text.trim().split('\n').filter(l => l.trim())
  if (!lines.length) { showToast('请输入用户列表', 'error'); return }
  const entries = lines.map(l => { const p = l.split(','); return { sender_id: p[0]?.trim(), display_name: p[1]?.trim(), department: p[2]?.trim() } }).filter(e => e.sender_id)
  try { const d = await apiPost('/api/v1/routes/batch-bind', { app_id: batchForm.app, upstream_id: batchForm.upstream, entries }); showToast('批量绑定 ' + (d.count || entries.length) + ' 条成功', 'success'); refresh() } catch (e) { showToast('批量绑定失败: ' + e.message, 'error') }
}

async function migrateRoute() {
  if (!migrateForm.sender || !migrateForm.upstream) { showToast('请填写用户ID和目标上游ID', 'error'); return }
  const body = { sender_id: migrateForm.sender, to: migrateForm.upstream }
  if (migrateForm.app) body.app_id = migrateForm.app
  try { await apiPost('/api/v1/routes/migrate', body); showToast('迁移成功', 'success'); refresh() } catch (e) { showToast('迁移失败: ' + e.message, 'error') }
}

function applyFilter() { /* reactive, no action needed */ }

onMounted(refresh)
</script>
