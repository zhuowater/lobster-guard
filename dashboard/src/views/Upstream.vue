<template>
  <div class="upstream-page">
    <!-- 页面标题区 -->
    <div class="page-header">
      <h1 class="page-title"><Icon name="server" :size="20" /> 上游管理</h1>
      <div class="page-actions">
        <button class="btn btn-sm" @click="loadUpstreams"><Icon name="refresh" :size="14" /> 刷新</button>
        <button class="btn btn-sm btn-primary" @click="openAddModal"><Icon name="plus" :size="14" /> 添加上游</button>
      </div>
    </div>

    <!-- 统计卡片行 -->
    <div class="stat-cards">
      <div class="stat-card">
        <div class="stat-value">{{ stats.total }}</div>
        <div class="stat-label">总上游数</div>
      </div>
      <div class="stat-card stat-healthy">
        <div class="stat-value">{{ stats.healthy }}</div>
        <div class="stat-label">健康</div>
      </div>
      <div class="stat-card" :class="{ 'stat-unhealthy': stats.total - stats.healthy > 0 }">
        <div class="stat-value">{{ stats.total - stats.healthy }}</div>
        <div class="stat-label">异常</div>
      </div>
      <div class="stat-card stat-users">
        <div class="stat-value">{{ stats.total_users }}</div>
        <div class="stat-label">总用户数</div>
      </div>
    </div>

    <!-- K8s 发现状态条 -->
    <div v-if="discovery.enabled" class="discovery-bar" :class="discovery.connected ? 'discovery-ok' : 'discovery-err'">
      <div class="discovery-header" @click="discoveryExpanded = !discoveryExpanded">
        <div class="discovery-left">
          <span class="discovery-dot">{{ discovery.connected ? '🟢' : '🔴' }}</span>
          <span class="discovery-title">K8s 服务发现</span>
          <span class="discovery-status-text">{{ discovery.connected ? '已连接' : '连接异常' }}</span>
        </div>
        <div class="discovery-summary">
          <span v-if="discovery.namespace">ns: <strong>{{ discovery.namespace }}</strong></span>
          <span v-if="discovery.service">svc: <strong>{{ discovery.service }}</strong></span>
          <span v-if="discovery.pod_count != null">Pod: <strong>{{ discovery.pod_count }}</strong></span>
        </div>
        <svg :class="{ 'expand-chevron': true, expanded: discoveryExpanded }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
      </div>
      <div v-if="discoveryExpanded" class="discovery-detail">
        <div class="discovery-grid">
          <div><span class="detail-label">Namespace</span><span class="detail-value">{{ discovery.namespace || '--' }}</span></div>
          <div><span class="detail-label">Service</span><span class="detail-value">{{ discovery.service || '--' }}</span></div>
          <div><span class="detail-label">上次同步</span><span class="detail-value">{{ fmtTime(discovery.last_sync) }}</span></div>
          <div><span class="detail-label">发现 Pod 数</span><span class="detail-value">{{ discovery.pod_count ?? '--' }}</span></div>
          <div v-if="discovery.error"><span class="detail-label">错误</span><span class="detail-value" style="color:var(--color-danger)">{{ discovery.error }}</span></div>
        </div>
      </div>
    </div>

    <!-- 上游列表表格 -->
    <div class="card" style="margin-top: var(--space-3)">
      <DataTable
        :columns="columns"
        :data="upstreams"
        :loading="loading"
        empty-text="暂无上游节点"
        empty-desc="请配置 upstream 或等待服务注册"
        :expandable="true"
        row-key="id"
      >
        <template #cell-address_port="{ row }">
          <code class="addr-code">{{ row.address || row.addr || row.host }}:{{ row.port }}</code>
          <span v-if="row.path_prefix" class="prefix-tag">{{ row.path_prefix }}</span>
        </template>

        <template #cell-healthy="{ row }">
          <div class="health-badge" :class="row.healthy ? 'health-ok' : 'health-err'">
            <span class="dot dot-sm" :class="row.healthy ? 'dot-healthy' : 'dot-unhealthy'"></span>
            {{ row.healthy ? '健康' : '异常' }}
          </div>
        </template>

        <template #cell-source="{ row }">
          <span class="source-tag source-k8s" v-if="getSource(row) === 'k8s'">K8s</span>
          <span class="source-tag source-static" v-else-if="row.static">静态</span>
          <span class="source-tag source-api" v-else-if="getSource(row) === 'api'">API</span>
          <span class="source-tag source-legacy" v-else>兼容</span>
        </template>

        <template #cell-last_heartbeat="{ row }">
          {{ fmtTime(row.last_heartbeat) }}
        </template>

        <template #cell-actions="{ row }">
          <div class="action-btns">
            <button class="btn btn-ghost btn-xs" title="编辑" @click.stop="openEditModal(row)">
              <Icon name="edit" :size="14" />
            </button>
            <button
              class="btn btn-ghost btn-xs"
              :title="row.static ? '静态上游不可删除' : '删除'"
              :disabled="row.static"
              @click.stop="confirmDelete(row)"
            >
              <Icon name="trash" :size="14" />
            </button>
            <button class="btn btn-ghost btn-xs" title="健康检查" @click.stop="doHealthCheck(row)">
              <Icon name="activity" :size="14" />
            </button>
          </div>
        </template>

        <template #expand="{ row }">
          <div class="expand-detail">
            <div><span class="detail-label">ID</span><span class="detail-value" style="font-weight:500">{{ row.id }}</span></div>
            <div><span class="detail-label">地址</span><span class="detail-value mono">{{ row.address || row.addr || row.host }}:{{ row.port }}</span></div>
            <div><span class="detail-label">路径前缀</span><span class="detail-value mono">{{ row.path_prefix || '(无)' }}</span></div>
            <div><span class="detail-label">静态</span><span class="detail-value">{{ row.static ? '是' : '否' }}</span></div>
            <div><span class="detail-label">用户数</span><span class="detail-value" style="color:var(--color-primary)">{{ row.user_count || 0 }}</span></div>
            <div v-if="row.tags"><span class="detail-label">Tags</span><span class="detail-value mono">{{ JSON.stringify(row.tags) }}</span></div>
            <div v-if="row.load"><span class="detail-label">负载</span><span class="detail-value mono">{{ JSON.stringify(row.load) }}</span></div>
            <div><span class="detail-label">最后心跳</span><span class="detail-value">{{ fmtTime(row.last_heartbeat) }}</span></div>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- 添加/编辑弹窗 -->
    <div v-if="modalVisible" class="modal-overlay" @click.self="closeModal">
      <div class="modal-box">
        <div class="modal-header">
          <h3>{{ modalMode === 'add' ? '添加上游' : '编辑上游' }}</h3>
          <button class="btn btn-ghost btn-xs modal-close" @click="closeModal">&times;</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>ID</label>
            <input
              v-model="form.id"
              :disabled="modalMode === 'edit'"
              :class="{ 'input-disabled': modalMode === 'edit' }"
              placeholder="例如: my-upstream-1"
            />
          </div>
          <div class="form-group">
            <label>地址</label>
            <input v-model="form.address" placeholder="例如: 10.0.0.1" />
          </div>
          <div class="form-group">
            <label>端口</label>
            <input v-model.number="form.port" type="number" placeholder="例如: 18789" />
          </div>
          <div class="form-group">
            <label>路径前缀 <span class="label-hint">(可选，如 /api/v1)</span></label>
            <input v-model="form.path_prefix" placeholder="例如: /api/v1" />
          </div>
          <div class="form-group">
            <label>Tags</label>
            <div class="tags-editor">
              <div v-for="(tag, idx) in form.tags" :key="idx" class="tag-row">
                <input v-model="tag.key" placeholder="key" class="tag-input" />
                <input v-model="tag.value" placeholder="value" class="tag-input" />
                <button class="btn btn-ghost btn-xs" @click="removeTag(idx)" title="删除">
                  <Icon name="trash" :size="12" />
                </button>
              </div>
              <button class="btn btn-ghost btn-xs" @click="addTag">
                <Icon name="plus" :size="12" /> 添加 Tag
              </button>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-sm" @click="closeModal">取消</button>
          <button class="btn btn-sm btn-primary" @click="saveUpstream" :disabled="saving">
            {{ saving ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 删除确认弹窗 -->
    <div v-if="deleteConfirmVisible" class="modal-overlay" @click.self="deleteConfirmVisible = false">
      <div class="modal-box modal-sm">
        <div class="modal-header">
          <h3>确认删除</h3>
          <button class="btn btn-ghost btn-xs modal-close" @click="deleteConfirmVisible = false">&times;</button>
        </div>
        <div class="modal-body">
          <p>确定要删除上游 <strong>{{ deleteTarget?.id }}</strong> 吗？</p>
          <p v-if="getSource(deleteTarget) === 'k8s'" class="warn-text">
            ⚠️ 此上游由 K8s 自动管理，删除后可能被重新发现。
          </p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-sm" @click="deleteConfirmVisible = false">取消</button>
          <button class="btn btn-sm btn-danger" @click="doDelete" :disabled="deleting">
            {{ deleting ? '删除中...' : '确认删除' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 健康检查结果提示 -->
    <div v-if="toast.show" class="toast" :class="'toast-' + toast.type" @click="toast.show = false">
      {{ toast.message }}
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import DataTable from '../components/DataTable.vue'
import Icon from '../components/Icon.vue'

// === State ===
const loading = ref(false)
const upstreams = ref([])
const stats = reactive({ total: 0, healthy: 0, total_users: 0 })
const discovery = reactive({ enabled: false, connected: false, namespace: '', service: '', last_sync: '', pod_count: null, error: '' })
const discoveryExpanded = ref(false)

// Modal
const modalVisible = ref(false)
const modalMode = ref('add') // 'add' | 'edit'
const saving = ref(false)
const form = reactive({
  id: '',
  address: '',
  port: 18789,
  path_prefix: '',
  tags: [] // [{key, value}]
})

// Delete confirm
const deleteConfirmVisible = ref(false)
const deleteTarget = ref(null)
const deleting = ref(false)

// Toast
const toast = reactive({ show: false, message: '', type: 'info' })
let toastTimer = null

const columns = [
  { key: 'id', label: 'ID', sortable: true },
  { key: 'address_port', label: '地址', sortable: false },
  { key: 'healthy', label: '状态', sortable: true },
  { key: 'source', label: '来源', sortable: true },
  { key: 'user_count', label: '用户数', sortable: true },
  { key: 'last_heartbeat', label: '最后心跳', sortable: true },
  { key: 'actions', label: '操作', sortable: false },
]

// === Helpers ===
function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false })
}

function getSource(row) {
  if (!row) return ''
  return row.tags?.source || (row.static ? 'static' : '')
}

function showToast(message, type = 'info', duration = 3000) {
  toast.message = message
  toast.type = type
  toast.show = true
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toast.show = false }, duration)
}

// === Data Loading ===
async function loadUpstreams() {
  loading.value = true
  try {
    const d = await api('/api/v1/upstreams')
    upstreams.value = (d.upstreams || []).map(u => ({
      ...u,
      source: u.tags?.source || (u.static ? 'static' : 'legacy'),
    }))
    stats.total = d.total || upstreams.value.length
    stats.healthy = d.healthy ?? upstreams.value.filter(u => u.healthy).length
    stats.total_users = d.total_users ?? upstreams.value.reduce((s, u) => s + (u.user_count || 0), 0)
  } catch (e) {
    upstreams.value = []
    stats.total = 0
    stats.healthy = 0
    stats.total_users = 0
  }
  loading.value = false
}

async function loadDiscovery() {
  try {
    const d = await api('/api/v1/discovery/status')
    discovery.enabled = !!d.enabled
    discovery.connected = !!d.connected
    discovery.namespace = d.namespace || ''
    discovery.service = d.service || ''
    discovery.last_sync = d.last_sync || ''
    discovery.pod_count = d.pod_count ?? null
    discovery.error = d.error || ''
  } catch {
    discovery.enabled = false
  }
}

// === Modal ===
function openAddModal() {
  modalMode.value = 'add'
  form.id = ''
  form.address = ''
  form.port = 18789
  form.path_prefix = ''
  form.tags = []
  modalVisible.value = true
}

function openEditModal(row) {
  modalMode.value = 'edit'
  form.id = row.id
  form.address = row.address || row.addr || row.host || ''
  form.port = row.port || 18789
  form.path_prefix = row.path_prefix || ''
  // Convert tags object to array
  form.tags = []
  if (row.tags && typeof row.tags === 'object') {
    for (const [k, v] of Object.entries(row.tags)) {
      form.tags.push({ key: k, value: String(v) })
    }
  }
  modalVisible.value = true
}

function closeModal() {
  modalVisible.value = false
}

function addTag() {
  form.tags.push({ key: '', value: '' })
}

function removeTag(idx) {
  form.tags.splice(idx, 1)
}

async function saveUpstream() {
  if (!form.id.trim()) { showToast('ID 不能为空', 'error'); return }
  if (!form.address.trim()) { showToast('地址不能为空', 'error'); return }
  if (!form.port || form.port <= 0) { showToast('端口无效', 'error'); return }

  // Build tags object
  const tags = {}
  for (const t of form.tags) {
    if (t.key.trim()) tags[t.key.trim()] = t.value
  }

  saving.value = true
  try {
    if (modalMode.value === 'add') {
      await apiPost('/api/v1/upstreams', {
        id: form.id.trim(),
        address: form.address.trim(),
        port: form.port,
        path_prefix: form.path_prefix.trim(),
        tags
      })
      showToast('上游添加成功', 'success')
    } else {
      await apiPut('/api/v1/upstreams/' + encodeURIComponent(form.id), {
        address: form.address.trim(),
        port: form.port,
        path_prefix: form.path_prefix.trim(),
        tags
      })
      showToast('上游更新成功', 'success')
    }
    closeModal()
    await loadUpstreams()
  } catch (e) {
    showToast('保存失败: ' + (e.message || e), 'error')
  }
  saving.value = false
}

// === Delete ===
function confirmDelete(row) {
  if (row.static) return
  deleteTarget.value = row
  deleteConfirmVisible.value = true
}

async function doDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await apiDelete('/api/v1/upstreams/' + encodeURIComponent(deleteTarget.value.id))
    showToast('上游已删除', 'success')
    deleteConfirmVisible.value = false
    await loadUpstreams()
  } catch (e) {
    showToast('删除失败: ' + (e.message || e), 'error')
  }
  deleting.value = false
}

// === Health Check ===
async function doHealthCheck(row) {
  showToast('正在检查 ' + row.id + ' ...', 'info')
  try {
    const res = await apiPost('/api/v1/upstreams/' + encodeURIComponent(row.id) + '/health-check', {})
    const ok = res.healthy ?? res.ok ?? true
    showToast(row.id + (ok ? ' 健康 ✓' : ' 异常 ✗'), ok ? 'success' : 'error')
    await loadUpstreams()
  } catch (e) {
    showToast('健康检查失败: ' + (e.message || e), 'error')
  }
}

// === Init ===
onMounted(() => {
  loadUpstreams()
  loadDiscovery()
})
</script>

<style scoped>
/* === 页面标题区 === */
.upstream-page {
  padding: 0;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--space-4);
}
.page-title {
  font-size: 1.25rem;
  font-weight: 700;
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-primary);
  margin: 0;
}
.page-actions {
  display: flex;
  gap: var(--space-2);
}

/* === 统计卡片 === */
.stat-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}
.stat-card {
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  padding: var(--space-3) var(--space-4);
  text-align: center;
}
.stat-value {
  font-size: 1.75rem;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1.2;
}
.stat-label {
  font-size: var(--text-xs);
  color: var(--text-tertiary);
  margin-top: 4px;
}
.stat-healthy .stat-value {
  color: var(--color-success, #22c55e);
}
.stat-unhealthy .stat-value {
  color: var(--color-danger, #ef4444);
}
.stat-users .stat-value {
  color: #a78bfa;
}

/* === K8s 发现状态条 === */
.discovery-bar {
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: hidden;
}
.discovery-ok {
  border-left: 3px solid var(--color-success, #22c55e);
}
.discovery-err {
  border-left: 3px solid var(--color-danger, #ef4444);
}
.discovery-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-2) var(--space-3);
  cursor: pointer;
  user-select: none;
}
.discovery-header:hover {
  background: var(--bg-hover, rgba(255,255,255,0.03));
}
.discovery-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.discovery-dot {
  font-size: 0.75rem;
}
.discovery-title {
  font-weight: 600;
  font-size: var(--text-sm);
  color: var(--text-primary);
}
.discovery-status-text {
  font-size: var(--text-xs);
  color: var(--text-secondary);
}
.discovery-summary {
  display: flex;
  gap: var(--space-3);
  font-size: var(--text-xs);
  color: var(--text-secondary);
}
.discovery-summary strong {
  color: var(--text-primary);
}
.expand-chevron {
  transition: transform 0.2s;
  color: var(--text-tertiary);
}
.expand-chevron.expanded {
  transform: rotate(90deg);
}
.discovery-detail {
  padding: var(--space-2) var(--space-3) var(--space-3);
  border-top: 1px solid var(--border-subtle);
}
.discovery-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 8px 24px;
  font-size: var(--text-sm);
}
.detail-label {
  display: block;
  color: var(--text-tertiary);
  font-size: var(--text-xs);
  margin-bottom: 2px;
}
.detail-value {
  font-weight: 500;
  color: var(--text-primary);
}

/* === 表格自定义 === */
.addr-code {
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  background: var(--bg-inset, rgba(0,0,0,0.15));
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  color: var(--text-primary);
}
.prefix-tag {
  display: inline-block;
  margin-left: 6px;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 0.7rem;
  font-family: var(--font-mono);
  background: rgba(99, 102, 241, 0.12);
  color: #818cf8;
}
.label-hint {
  font-weight: 400;
  color: var(--text-tertiary);
  font-size: var(--text-xs);
}
.health-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 10px;
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  font-weight: 500;
}
.health-ok {
  background: var(--color-success-dim, rgba(34,197,94,0.12));
  color: var(--color-success, #22c55e);
}
.health-err {
  background: var(--color-danger-dim, rgba(239,68,68,0.12));
  color: var(--color-danger, #ef4444);
}

/* === 来源标签 === */
.source-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.02em;
}
.source-k8s {
  background: rgba(129, 140, 248, 0.15);
  color: #818cf8;
}
.source-static {
  background: rgba(148, 163, 184, 0.15);
  color: #94a3b8;
}
.source-api {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}
.source-legacy {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
}

/* === 操作按钮组 === */
.action-btns {
  display: flex;
  gap: 4px;
  align-items: center;
}
.action-btns .btn {
  padding: 4px 6px;
}
.action-btns .btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

/* === 展开行详情 === */
.expand-detail {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px 24px;
  font-size: 0.82rem;
}
.expand-detail .mono {
  font-family: var(--font-mono);
}

/* === Modal === */
.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
}
.modal-box {
  background: var(--bg-elevated, #1e1e2e);
  border: 1px solid var(--border-subtle, rgba(255,255,255,0.08));
  border-radius: var(--radius-lg, 12px);
  min-width: 420px;
  max-width: 560px;
  width: 90vw;
  max-height: 85vh;
  overflow-y: auto;
  box-shadow: 0 20px 60px rgba(0,0,0,0.4);
}
.modal-sm {
  min-width: 340px;
  max-width: 440px;
}
.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}
.modal-header h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}
.modal-close {
  font-size: 1.25rem;
  line-height: 1;
  padding: 4px 8px;
}
.modal-body {
  padding: var(--space-4);
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  border-top: 1px solid var(--border-subtle);
}

/* === Form === */
.form-group {
  margin-bottom: var(--space-3);
}
.form-group label {
  display: block;
  font-size: var(--text-sm);
  font-weight: 500;
  color: var(--text-secondary);
  margin-bottom: 6px;
}
.form-group input {
  width: 100%;
  padding: 8px 12px;
  background: var(--bg-inset, rgba(0,0,0,0.2));
  border: 1px solid var(--border-default, rgba(255,255,255,0.1));
  border-radius: var(--radius-md);
  color: var(--text-primary);
  font-size: var(--text-sm);
  font-family: var(--font-sans);
  outline: none;
  transition: border-color 0.15s;
  box-sizing: border-box;
}
.form-group input:focus {
  border-color: var(--color-primary);
}
.form-group input.input-disabled {
  opacity: 0.5;
  cursor: not-allowed;
  background: var(--bg-elevated);
}

/* Tags editor */
.tags-editor {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.tag-row {
  display: flex;
  gap: 6px;
  align-items: center;
}
.tag-input {
  flex: 1;
  padding: 6px 10px;
  background: var(--bg-inset, rgba(0,0,0,0.2));
  border: 1px solid var(--border-default, rgba(255,255,255,0.1));
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: var(--text-xs);
  font-family: var(--font-mono);
  outline: none;
}
.tag-input:focus {
  border-color: var(--color-primary);
}

/* === Buttons === */
.btn-primary {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}
.btn-primary:hover {
  opacity: 0.9;
}
.btn-danger {
  background: var(--color-danger, #ef4444);
  color: #fff;
  border-color: var(--color-danger, #ef4444);
}
.btn-danger:hover {
  opacity: 0.9;
}
.btn-xs {
  padding: 2px 6px;
  font-size: var(--text-xs);
}

/* === Warn text === */
.warn-text {
  color: var(--color-warning, #f59e0b);
  font-size: var(--text-sm);
  margin-top: 8px;
}

/* === Toast === */
.toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 2000;
  padding: 10px 20px;
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
  font-weight: 500;
  cursor: pointer;
  box-shadow: 0 4px 20px rgba(0,0,0,0.3);
  animation: toast-in 0.3s ease;
}
.toast-info {
  background: var(--bg-elevated);
  color: var(--text-primary);
  border: 1px solid var(--border-default);
}
.toast-success {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
  border: 1px solid rgba(34, 197, 94, 0.3);
}
.toast-error {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.3);
}
@keyframes toast-in {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

/* === Responsive === */
@media (max-width: 768px) {
  .stat-cards {
    grid-template-columns: repeat(2, 1fr);
  }
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-2);
  }
  .discovery-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }
  .modal-box {
    min-width: auto;
    width: 95vw;
  }
}
</style>