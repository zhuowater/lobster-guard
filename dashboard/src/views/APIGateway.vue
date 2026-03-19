<template>
  <div class="gateway-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">🚪 API 网关</h1>
        <p class="page-subtitle">统一入口路由 + JWT/APIKey 认证 + 灰度发布 — 安全访问控制</p>
      </div>
      <button class="btn btn-sm" @click="loadAll">🔄 刷新</button>
    </div>

    <!-- 顶部大数字 -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon">📊</div>
        <div class="stat-value">{{ stats.total_requests ?? '-' }}</div>
        <div class="stat-label">总请求</div>
      </div>
      <div class="stat-card stat-danger">
        <div class="stat-icon">🔒</div>
        <div class="stat-value">{{ stats.auth_failures ?? '-' }}</div>
        <div class="stat-label">认证失败</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🎲</div>
        <div class="stat-value">{{ stats.canary_requests ?? '-' }}</div>
        <div class="stat-label">灰度请求</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🗺️</div>
        <div class="stat-value">{{ stats.route_count ?? '-' }}</div>
        <div class="stat-label">路由数</div>
      </div>
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'routes' }" @click="activeTab = 'routes'">🗺️ 路由管理 ({{ routes.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'jwt' }" @click="activeTab = 'jwt'">🔑 JWT 工具</button>
      <button class="tab-btn" :class="{ active: activeTab === 'log' }" @click="activeTab = 'log'">📜 网关日志 ({{ logs.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'config' }" @click="activeTab = 'config'">⚙️ 配置</button>
    </div>

    <!-- 路由管理 -->
    <div v-if="activeTab === 'routes'" class="section">
      <div style="margin-bottom: var(--space-3)">
        <button class="btn btn-primary btn-sm" @click="openNewRoute">➕ 新建路由</button>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>路径模式</th>
              <th>上游 URL</th>
              <th>认证方式</th>
              <th>灰度%</th>
              <th>启用</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in routes" :key="r.id || r.name">
              <td class="td-mono">{{ r.name }}</td>
              <td class="td-mono">{{ r.path_pattern || r.path }}</td>
              <td class="td-mono td-url">{{ truncate(r.upstream_url || r.upstream, 40) }}</td>
              <td><span class="auth-badge">{{ r.auth_method || r.auth || 'none' }}</span></td>
              <td class="td-mono">{{ r.canary_percent ?? r.canary ?? 0 }}%</td>
              <td><span :class="r.enabled !== false ? 'badge-on' : 'badge-off'">{{ r.enabled !== false ? '启用' : '禁用' }}</span></td>
              <td class="td-actions">
                <button class="btn-icon" @click="editRoute(r)" title="编辑">✏️</button>
                <button class="btn-icon" @click="deleteRoute(r)" title="删除">🗑️</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="routes.length === 0" class="empty-state">暂无路由</div>
      </div>
    </div>

    <!-- JWT 工具区 -->
    <div v-if="activeTab === 'jwt'" class="section">
      <div class="jwt-panels">
        <!-- 生成 Token -->
        <div class="test-panel">
          <h3 class="section-title">🔑 生成 JWT Token</h3>
          <div class="config-field">
            <label class="field-label">租户 ID</label>
            <input v-model="jwtForm.tenant_id" class="field-input" placeholder="e.g. tenant-001">
          </div>
          <div class="config-field">
            <label class="field-label">角色</label>
            <select v-model="jwtForm.role" class="field-select">
              <option value="admin">admin</option>
              <option value="user">user</option>
              <option value="readonly">readonly</option>
            </select>
          </div>
          <div class="config-field">
            <label class="field-label">过期时间 (小时)</label>
            <input v-model.number="jwtForm.expire_hours" class="field-input" type="number" placeholder="24">
          </div>
          <button class="btn btn-primary btn-sm" @click="generateToken" :disabled="generating">
            {{ generating ? '生成中...' : '🔑 生成 Token' }}
          </button>
          <div v-if="generatedToken" class="token-output">
            <label class="field-label">生成的 JWT</label>
            <div class="token-box">
              <code class="token-text">{{ generatedToken }}</code>
              <button class="btn-icon" @click="copyToken(generatedToken)" title="复制">📋</button>
            </div>
          </div>
        </div>

        <!-- 验证 Token -->
        <div class="test-panel">
          <h3 class="section-title">🔍 验证 JWT Token</h3>
          <div class="config-field">
            <label class="field-label">JWT Token</label>
            <textarea v-model="validateJwt" class="test-input" rows="3" placeholder="粘贴 JWT..."></textarea>
          </div>
          <button class="btn btn-primary btn-sm" @click="validateToken" :disabled="validating || !validateJwt.trim()">
            {{ validating ? '验证中...' : '🔍 验证' }}
          </button>
          <div v-if="validateResult" class="validate-output">
            <div class="validate-status" :class="validateResult.valid ? 'valid-yes' : 'valid-no'">
              {{ validateResult.valid ? '✅ Token 有效' : '❌ Token 无效' }}
            </div>
            <pre v-if="validateResult.claims" class="response-pre">{{ JSON.stringify(validateResult.claims, null, 2) }}</pre>
            <div v-if="validateResult.error" class="error-text">{{ validateResult.error }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 网关日志 -->
    <div v-if="activeTab === 'log'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>路径</th>
              <th>方法</th>
              <th>路由</th>
              <th>租户</th>
              <th>上游</th>
              <th>状态码</th>
              <th>延迟</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(l, idx) in logs" :key="idx">
              <td class="td-mono">{{ formatTime(l.timestamp || l.time) }}</td>
              <td class="td-mono">{{ truncate(l.path, 30) }}</td>
              <td><span class="method-badge" :class="'method-' + (l.method || '').toLowerCase()">{{ l.method }}</span></td>
              <td class="td-mono">{{ l.route || '-' }}</td>
              <td class="td-mono">{{ l.tenant || l.tenant_id || '-' }}</td>
              <td class="td-mono td-url">{{ truncate(l.upstream || l.upstream_url, 30) }}</td>
              <td><span class="status-code" :class="statusClass(l.status_code || l.status)">{{ l.status_code || l.status }}</span></td>
              <td class="td-mono">{{ l.latency_ms ?? l.latency ? (l.latency_ms ?? l.latency) + 'ms' : '-' }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="logs.length === 0" class="empty-state">暂无日志</div>
      </div>
    </div>

    <!-- 配置 -->
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-panel">
        <h3 class="section-title">网关配置</h3>
        <div class="config-field">
          <label class="toggle-row">
            <input type="checkbox" v-model="gwConfig.jwt_enabled">
            <span>JWT 认证</span>
          </label>
        </div>
        <div class="config-field">
          <label class="toggle-row">
            <input type="checkbox" v-model="gwConfig.apikey_enabled">
            <span>API Key 认证</span>
          </label>
        </div>
        <div class="config-field">
          <label class="field-label">JWT Secret (脱敏)</label>
          <input v-model="gwConfig.jwt_secret" class="field-input" type="password" placeholder="••••••••">
        </div>
        <div class="config-field" v-if="gwConfig.api_keys && gwConfig.api_keys.length">
          <label class="field-label">API Keys</label>
          <div class="key-list">
            <span v-for="k in gwConfig.api_keys" :key="k" class="key-tag">{{ maskKey(k) }}</span>
          </div>
        </div>
        <button class="btn btn-primary" @click="saveGwConfig" :disabled="saving" style="margin-top: var(--space-3)">
          {{ saving ? '保存中...' : '💾 保存配置' }}
        </button>
        <div v-if="saveMsg" class="save-msg" :class="saveMsgType">{{ saveMsg }}</div>
      </div>
    </div>

    <!-- 新建/编辑路由对话框 -->
    <div v-if="showDialog" class="dialog-overlay" @click.self="showDialog = false">
      <div class="dialog">
        <div class="dialog-header">{{ editingRoute ? '编辑路由' : '新建路由' }}</div>
        <div class="dialog-body">
          <div class="config-field">
            <label class="field-label">名称</label>
            <input v-model="routeForm.name" class="field-input" placeholder="路由名称">
          </div>
          <div class="config-field">
            <label class="field-label">路径模式</label>
            <input v-model="routeForm.path_pattern" class="field-input" placeholder="e.g. /api/v1/*">
          </div>
          <div class="config-field">
            <label class="field-label">上游 URL</label>
            <input v-model="routeForm.upstream_url" class="field-input" placeholder="http://backend:8080">
          </div>
          <div class="config-field">
            <label class="field-label">认证方式</label>
            <select v-model="routeForm.auth_method" class="field-select">
              <option value="none">none</option>
              <option value="jwt">jwt</option>
              <option value="apikey">apikey</option>
              <option value="both">both</option>
            </select>
          </div>
          <div class="config-field">
            <label class="field-label">灰度比例 (%)</label>
            <input v-model.number="routeForm.canary_percent" class="field-input" type="number" min="0" max="100" placeholder="0">
          </div>
        </div>
        <div class="dialog-footer">
          <button class="btn btn-sm" @click="showDialog = false">取消</button>
          <button class="btn btn-primary btn-sm" @click="saveRoute" :disabled="savingRoute">{{ savingRoute ? '保存中...' : '保存' }}</button>
        </div>
      </div>
    </div>

    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'

const activeTab = ref('routes')
const stats = ref({})
const routes = ref([])
const logs = ref([])
const error = ref('')
const showDialog = ref(false)
const editingRoute = ref(null)
const savingRoute = ref(false)
const saving = ref(false)
const saveMsg = ref('')
const saveMsgType = ref('')

const routeForm = reactive({ name: '', path_pattern: '', upstream_url: '', auth_method: 'none', canary_percent: 0 })

// JWT
const jwtForm = reactive({ tenant_id: '', role: 'user', expire_hours: 24 })
const generating = ref(false)
const generatedToken = ref('')
const validateJwt = ref('')
const validating = ref(false)
const validateResult = ref(null)

// Config
const gwConfig = reactive({ jwt_enabled: true, apikey_enabled: false, jwt_secret: '', api_keys: [] })

async function loadStats() {
  try {
    const d = await api('/api/v1/gateway/routes')
    const r = d.routes || d || []
    routes.value = r
    stats.value = { route_count: r.length, total_requests: d.total_requests, auth_failures: d.auth_failures, canary_requests: d.canary_requests }
  } catch (e) { error.value = '加载路由失败: ' + e.message }
}

async function loadLogs() {
  try { const d = await api('/api/v1/gateway/log?limit=50'); logs.value = d.logs || d.entries || d || [] } catch (e) { error.value = '加载日志失败: ' + e.message }
}

async function loadConfig() {
  try {
    const d = await api('/api/v1/gateway/config')
    if (d.jwt_enabled != null) gwConfig.jwt_enabled = d.jwt_enabled
    if (d.apikey_enabled != null) gwConfig.apikey_enabled = d.apikey_enabled
    if (d.jwt_secret) gwConfig.jwt_secret = d.jwt_secret
    if (d.api_keys) gwConfig.api_keys = d.api_keys
  } catch {}
}

function openNewRoute() {
  editingRoute.value = null
  Object.assign(routeForm, { name: '', path_pattern: '', upstream_url: '', auth_method: 'none', canary_percent: 0 })
  showDialog.value = true
}

function editRoute(r) {
  editingRoute.value = r
  Object.assign(routeForm, { name: r.name, path_pattern: r.path_pattern || r.path, upstream_url: r.upstream_url || r.upstream, auth_method: r.auth_method || r.auth || 'none', canary_percent: r.canary_percent ?? r.canary ?? 0 })
  showDialog.value = true
}

async function saveRoute() {
  savingRoute.value = true
  try {
    const body = { name: routeForm.name, path_pattern: routeForm.path_pattern, upstream_url: routeForm.upstream_url, auth_method: routeForm.auth_method, canary_percent: routeForm.canary_percent }
    if (editingRoute.value) {
      await apiPut('/api/v1/gateway/routes/' + (editingRoute.value.id || editingRoute.value.name), body)
    } else {
      await apiPost('/api/v1/gateway/routes', body)
    }
    showDialog.value = false; loadStats()
  } catch (e) { error.value = '保存路由失败: ' + e.message }
  finally { savingRoute.value = false }
}

async function deleteRoute(r) {
  if (!confirm('确定删除路由 "' + r.name + '" 吗？')) return
  try { await apiDelete('/api/v1/gateway/routes/' + (r.id || r.name)); loadStats() } catch (e) { error.value = '删除失败: ' + e.message }
}

async function generateToken() {
  generating.value = true; generatedToken.value = ''
  try {
    const d = await apiPost('/api/v1/gateway/token', { tenant_id: jwtForm.tenant_id, role: jwtForm.role, expire_hours: jwtForm.expire_hours })
    generatedToken.value = d.token || d.jwt || ''
  } catch (e) { error.value = '生成失败: ' + e.message }
  finally { generating.value = false }
}

async function validateToken() {
  validating.value = true; validateResult.value = null
  try { validateResult.value = await apiPost('/api/v1/gateway/validate', { token: validateJwt.value }) }
  catch (e) { error.value = '验证失败: ' + e.message }
  finally { validating.value = false }
}

function copyToken(t) {
  navigator.clipboard?.writeText(t)
}

async function saveGwConfig() {
  saving.value = true; saveMsg.value = ''
  try {
    await apiPut('/api/v1/gateway/config', { jwt_enabled: gwConfig.jwt_enabled, apikey_enabled: gwConfig.apikey_enabled, jwt_secret: gwConfig.jwt_secret })
    saveMsg.value = '✅ 配置已保存'; saveMsgType.value = 'success'
  } catch (e) { saveMsg.value = '❌ 保存失败: ' + e.message; saveMsgType.value = 'error' }
  finally { saving.value = false }
}

function statusClass(code) {
  if (!code) return ''
  if (code >= 500) return 'status-5xx'
  if (code >= 400) return 'status-4xx'
  if (code >= 300) return 'status-3xx'
  return 'status-2xx'
}

function maskKey(k) {
  if (!k || k.length < 8) return k
  return k.slice(0, 4) + '••••' + k.slice(-4)
}

function loadAll() { error.value = ''; loadStats(); loadLogs(); loadConfig() }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function formatTime(ts) {
  if (!ts) return '-'
  try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) } catch { return ts }
}
onMounted(loadAll)
</script>

<style scoped>
.gateway-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); text-align: center; }
.stat-icon { font-size: 1.5rem; margin-bottom: var(--space-1); }
.stat-value { font-size: 1.75rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }
.stat-danger .stat-value { color: #EF4444; }

.tab-bar { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }

.section { margin-bottom: var(--space-4); }
.section-title { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); }

/* Table */
.table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th { text-align: left; padding: 8px 10px; background: var(--bg-elevated); color: var(--text-tertiary); font-weight: 600; font-size: 10px; text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle); white-space: nowrap; }
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.td-url { max-width: 250px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.td-actions { display: flex; gap: 4px; }
.btn-icon { background: none; border: none; cursor: pointer; font-size: 14px; padding: 2px 4px; border-radius: 4px; transition: background .2s; }
.btn-icon:hover { background: var(--bg-elevated); }

.badge-on { color: #10B981; font-weight: 600; font-size: 11px; }
.badge-off { color: var(--text-tertiary); font-size: 11px; }
.auth-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; background: rgba(99,102,241,.15); color: #a5b4fc; }

/* Method badges */
.method-badge { display: inline-block; padding: 1px 6px; border-radius: 3px; font-size: 10px; font-weight: 700; font-family: var(--font-mono); }
.method-get { background: rgba(16,185,129,.15); color: #6EE7B7; }
.method-post { background: rgba(59,130,246,.15); color: #93C5FD; }
.method-put { background: rgba(245,158,11,.15); color: #FCD34D; }
.method-delete { background: rgba(239,68,68,.15); color: #FCA5A5; }

/* Status codes */
.status-code { font-family: var(--font-mono); font-weight: 700; font-size: 11px; }
.status-2xx { color: #10B981; }
.status-3xx { color: #3B82F6; }
.status-4xx { color: #F59E0B; }
.status-5xx { color: #EF4444; }

/* JWT Panels */
.jwt-panels { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
.test-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.test-input {
  width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md);
  color: var(--text-primary); padding: var(--space-3); font-size: var(--text-sm); resize: vertical; font-family: var(--font-mono);
}
.test-input:focus { outline: none; border-color: var(--color-primary); }

.token-output { margin-top: var(--space-3); }
.token-box { display: flex; align-items: flex-start; gap: var(--space-2); background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-2); }
.token-text { flex: 1; font-size: 10px; font-family: var(--font-mono); color: #10B981; word-break: break-all; line-height: 1.4; }

.validate-output { margin-top: var(--space-3); }
.validate-status { font-weight: 700; font-size: var(--text-sm); margin-bottom: var(--space-2); }
.valid-yes { color: #10B981; }
.valid-no { color: #EF4444; }
.response-pre { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-3); font-size: var(--text-xs); font-family: var(--font-mono); color: var(--text-secondary); overflow-x: auto; white-space: pre-wrap; word-wrap: break-word; max-height: 200px; overflow-y: auto; }
.error-text { color: #EF4444; font-size: var(--text-sm); }

/* Config */
.config-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); max-width: 480px; }
.config-field { margin-bottom: var(--space-3); display: flex; flex-direction: column; gap: 4px; }
.field-label { font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; }
.field-input { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); }
.field-input:focus { outline: none; border-color: var(--color-primary); }
.field-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); }
.toggle-row { display: flex; align-items: center; gap: 8px; font-size: var(--text-sm); color: var(--text-secondary); cursor: pointer; }
.toggle-row input { accent-color: var(--color-primary); }
.key-list { display: flex; flex-wrap: wrap; gap: var(--space-1); }
.key-tag { display: inline-block; padding: 2px 8px; background: rgba(255,255,255,.08); border-radius: 4px; font-size: 10px; font-family: var(--font-mono); color: var(--text-secondary); }

.save-msg { margin-top: var(--space-2); font-size: var(--text-sm); font-weight: 600; }
.save-msg.success { color: #10B981; }
.save-msg.error { color: #EF4444; }

/* Dialog */
.dialog-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.dialog { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: 0; width: 420px; max-width: 90vw; box-shadow: var(--shadow-lg); }
.dialog-header { padding: var(--space-4); border-bottom: 1px solid var(--border-subtle); font-weight: 700; color: var(--text-primary); font-size: var(--text-base); }
.dialog-body { padding: var(--space-4); display: flex; flex-direction: column; gap: var(--space-3); }
.dialog-footer { padding: var(--space-3) var(--space-4); border-top: 1px solid var(--border-subtle); display: flex; justify-content: flex-end; gap: var(--space-2); }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-sm { padding: 6px 12px; font-size: var(--text-xs); }
.spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.3); border-top-color: #fff; border-radius: 50%; animation: spin .6s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.empty-state { text-align: center; padding: var(--space-6); color: var(--text-tertiary); }
.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); }

@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .jwt-panels { grid-template-columns: 1fr; }
}
</style>