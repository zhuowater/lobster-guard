<template>
  <div class="gateway-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="door" :size="20" /> API 网关</h1>
        <p class="page-subtitle">统一入口路由 + JWT/APIKey 认证 + 灰度发布 — 安全访问控制</p>
      </div>
      <button class="btn btn-sm" @click="loadAll">🔄 刷新</button>
    </div>

    <!-- StatCard 统计 -->
    <div class="stats-grid-5">
      <StatCard label="总请求" :value="stats.total_requests ?? '--'" color="blue" :iconSvg="svgBar" />
      <StatCard label="认证失败" :value="stats.auth_failures ?? '--'" color="red" :iconSvg="svgLock" />
      <StatCard label="灰度请求" :value="stats.canary_requests ?? '--'" color="yellow" :iconSvg="svgGlobe" />
      <StatCard label="路由数" :value="stats.route_count ?? '--'" color="indigo" :iconSvg="svgBranch" :badge="activeRouteCount > 0 ? activeRouteCount + ' 活跃' : ''" />
      <StatCard label="认证路由" :value="authRouteCount" color="green" :iconSvg="svgKey" />
    </div>

    <!-- Tab -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'routes' }" @click="activeTab = 'routes'"><Icon name="git-branch" :size="14" /> 路由管理 ({{ routes.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'jwt' }" @click="activeTab = 'jwt'"><Icon name="key" :size="14" /> JWT 工具</button>
      <button class="tab-btn" :class="{ active: activeTab === 'test' }" @click="activeTab = 'test'"><Icon name="zap" :size="14" /> 路由测试</button>
      <button class="tab-btn" :class="{ active: activeTab === 'log' }" @click="activeTab = 'log'">📜 日志 ({{ logs.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'config' }" @click="activeTab = 'config'"><Icon name="settings" :size="14" /> 配置</button>
    </div>

    <!-- 路由管理 -->
    <div v-if="activeTab === 'routes'" class="section">
      <div class="filter-bar">
        <div class="filter-left">
          <div class="search-box">
            <Icon name="search" :size="14" class="search-icon" />
            <input v-model="searchQuery" class="search-input" placeholder="搜索路由名称/路径..." />
            <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''">✕</button>
          </div>
          <div class="method-pills">
            <button v-for="m in ['ALL','GET','POST','PUT','DELETE']" :key="m" class="mpill" :class="{ active: filterMethod === m, ['mp-'+m.toLowerCase()]: true }" @click="filterMethod = m">{{ m }}</button>
          </div>
        </div>
        <div class="filter-right">
          <template v-if="selectedIds.length">
            <button class="btn btn-sm btn-outline-ok" @click="batchEnable">✅ 启用 ({{ selectedIds.length }})</button>
            <button class="btn btn-sm btn-outline-red" @click="batchDisable">⛔ 禁用 ({{ selectedIds.length }})</button>
          </template>
          <button class="btn btn-primary btn-sm" @click="openNewRoute">➕ 新建路由</button>
        </div>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr>
            <th style="width:36px"><input type="checkbox" :checked="allSelected" @change="toggleSelectAll" class="row-cb" /></th>
            <th>名称</th><th>方法</th><th>路径</th><th>上游</th><th>认证</th><th>灰度</th><th>状态</th><th>操作</th>
          </tr></thead>
          <tbody>
            <tr v-for="r in filteredRoutes" :key="r.id||r.name" :class="{'row-off':r.enabled===false}">
              <td><input type="checkbox" :checked="selectedIds.includes(rid(r))" @change="toggleSelect(r)" class="row-cb" /></td>
              <td class="td-mono td-name">{{ r.name }}</td>
              <td><span class="method-badge" :class="'method-'+(r.method||'any').toLowerCase()">{{ r.method||'ANY' }}</span></td>
              <td class="td-mono">{{ r.path_pattern||r.path }}</td>
              <td class="td-mono td-url">{{ truncate(r.upstream_url||r.upstream, 36) }}</td>
              <td><span class="auth-badge" :class="'auth-'+(r.auth_method||r.auth||'none')">{{ r.auth_method||r.auth||'none' }}</span></td>
              <td class="td-mono">{{ r.canary_percent??r.canary??0 }}%</td>
              <td><button class="tog" :class="{on:r.enabled!==false}" @click="toggleRouteEnabled(r)"><span class="tog-k"></span></button></td>
              <td class="td-actions">
                <button class="btn-icon" @click="editRoute(r)" title="编辑"><Icon name="edit" :size="14" /></button>
                <button class="btn-icon btn-icon-danger" @click="confirmDeleteRoute(r)" title="删除"><Icon name="trash" :size="14" /></button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="filteredRoutes.length===0 && routes.length>0" class="empty-state">没有匹配的路由</div>
        <div v-if="routes.length===0" class="empty-state">暂无路由，点击"新建路由"开始</div>
      </div>
    </div>

    <!-- JWT 工具 -->
    <div v-if="activeTab === 'jwt'" class="section">
      <div class="jwt-panels">
        <div class="test-panel">
          <h3 class="section-title"><Icon name="key" :size="14" /> 生成 JWT</h3>
          <div class="config-field"><label class="field-label">租户 ID</label><input v-model="jwtForm.tenant_id" class="field-input" placeholder="tenant-001"></div>
          <div class="config-field"><label class="field-label">角色</label><select v-model="jwtForm.role" class="field-select"><option value="admin">admin</option><option value="user">user</option><option value="readonly">readonly</option></select></div>
          <div class="config-field"><label class="field-label">自定义 Claims</label><textarea v-model="jwtForm.custom_claims" class="test-input" rows="2" placeholder='{"scope":"read"}'></textarea></div>
          <div class="config-field"><label class="field-label">过期 (小时)</label><input v-model.number="jwtForm.expire_hours" class="field-input" type="number" placeholder="24"></div>
          <button class="btn btn-primary btn-sm" @click="generateToken" :disabled="generating">
            <template v-if="!generating"><Icon name="key" :size="14" /> 生成</template><template v-else><span class="spinner"></span> 生成中...</template>
          </button>
          <div v-if="generatedToken" class="token-output">
            <label class="field-label">JWT Token</label>
            <div class="token-box"><code class="token-text">{{ generatedToken }}</code><button class="btn-icon" @click="copyText(generatedToken)" title="复制"><Icon name="clipboard" :size="14" /></button></div>
            <div v-if="decodedJwt" class="jwt-decoded">
              <div class="jwt-sec"><span class="jwt-lbl">Header</span><pre class="code-pre">{{ JSON.stringify(decodedJwt.header,null,2) }}</pre></div>
              <div class="jwt-sec"><span class="jwt-lbl">Payload</span><pre class="code-pre">{{ JSON.stringify(decodedJwt.payload,null,2) }}</pre></div>
            </div>
          </div>
        </div>
        <div class="test-panel">
          <h3 class="section-title"><Icon name="search" :size="14" /> 验证 JWT</h3>
          <div class="config-field"><label class="field-label">JWT Token</label><textarea v-model="validateJwt" class="test-input" rows="4" placeholder="粘贴 JWT..."></textarea></div>
          <button class="btn btn-primary btn-sm" @click="doValidateToken" :disabled="validating||!validateJwt.trim()">
            <template v-if="!validating"><Icon name="search" :size="14" /> 验证</template><template v-else><span class="spinner"></span></template>
          </button>
          <div v-if="validateResult" class="validate-output">
            <div class="validate-status" :class="validateResult.valid?'valid-yes':'valid-no'">
              {{ validateResult.valid ? '✅ Token 有效' : '❌ Token 无效' }}
              <span v-if="validateResult.valid && validateResult.claims && validateResult.claims.exp" class="validate-exp">过期: {{ fmtExp(validateResult.claims.exp) }}</span>
            </div>
            <pre v-if="validateResult.claims" class="code-pre">{{ JSON.stringify(validateResult.claims,null,2) }}</pre>
            <div v-if="validateResult.error" class="error-text">{{ validateResult.error }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 路由测试 -->
    <div v-if="activeTab === 'test'" class="section">
      <div class="test-panel" style="max-width:600px">
        <h3 class="section-title"><Icon name="zap" :size="14" /> 路由匹配测试</h3>
        <p class="section-desc">输入路径和方法，测试匹配结果及认证状态</p>
        <div class="test-row">
          <select v-model="testForm.method" class="field-select" style="width:100px;flex-shrink:0"><option value="GET">GET</option><option value="POST">POST</option><option value="PUT">PUT</option><option value="DELETE">DELETE</option></select>
          <input v-model="testForm.path" class="field-input" placeholder="/api/v1/users" @keyup.enter="testRoute" style="flex:1" />
          <button class="btn btn-primary btn-sm" @click="testRoute" :disabled="testing||!testForm.path.trim()"><template v-if="!testing"><Icon name="zap" :size="14" /> 测试</template><template v-else><span class="spinner"></span></template></button>
        </div>
        <Transition name="fade">
          <div v-if="testResult" class="test-result" :class="testResult.matched?'tr-ok':'tr-fail'">
            <div class="tr-hd">{{ testResult.matched ? '✅ 匹配成功' : '❌ 无匹配路由' }}</div>
            <div v-if="testResult.matched" class="tr-body">
              <div class="tr-row"><span>路由</span><span class="td-mono">{{ testResult.route_name }}</span></div>
              <div class="tr-row"><span>上游</span><span class="td-mono">{{ testResult.upstream_url }}</span></div>
              <div class="tr-row"><span>认证</span><span class="auth-badge" :class="'auth-'+(testResult.auth_method||'none')">{{ testResult.auth_method||'none' }}</span></div>
              <div class="tr-row"><span>灰度</span><span>{{ testResult.canary_percent||0 }}%</span></div>
            </div>
            <div v-else class="tr-hint">没有匹配 <code>{{ testForm.method }} {{ testForm.path }}</code> 的路由</div>
          </div>
        </Transition>
      </div>
    </div>

    <!-- 日志 -->
    <div v-if="activeTab === 'log'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>时间</th><th>路径</th><th>方法</th><th>路由</th><th>租户</th><th>上游</th><th>状态码</th><th>延迟</th></tr></thead>
          <tbody>
            <tr v-for="(l,idx) in logs" :key="idx">
              <td class="td-mono">{{ formatTime(l.timestamp||l.time) }}</td>
              <td class="td-mono">{{ truncate(l.path,30) }}</td>
              <td><span class="method-badge" :class="'method-'+(l.method||'').toLowerCase()">{{ l.method }}</span></td>
              <td class="td-mono">{{ l.route||'-' }}</td>
              <td class="td-mono">{{ l.tenant||l.tenant_id||'-' }}</td>
              <td class="td-mono td-url">{{ truncate(l.upstream||l.upstream_url,30) }}</td>
              <td><span class="status-code" :class="statusClass(l.status_code||l.status)">{{ l.status_code||l.status }}</span></td>
              <td class="td-mono">{{ (l.latency_ms!=null||l.latency!=null)?(l.latency_ms||l.latency)+'ms':'-' }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="logs.length===0" class="empty-state">暂无日志</div>
      </div>
    </div>

    <!-- 配置 -->
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-panels">
        <div class="config-panel">
          <h3 class="section-title"><Icon name="key" :size="14" /> JWT 认证</h3>
          <div class="config-field"><label class="toggle-row"><input type="checkbox" v-model="gwConfig.jwt_enabled"><span>启用 JWT</span></label></div>
          <div class="config-field"><label class="field-label">Secret</label><div class="secret-row"><input v-model="gwConfig.jwt_secret" class="field-input" :type="showSecret?'text':'password'" placeholder="••••••••"><button class="btn-icon" @click="showSecret=!showSecret"><Icon :name="showSecret?'eye-off':'eye'" :size="14" /></button></div></div>
          <div class="config-field"><label class="field-label">算法</label><select v-model="gwConfig.jwt_algorithm" class="field-select"><option value="HS256">HS256</option><option value="HS384">HS384</option><option value="HS512">HS512</option><option value="RS256">RS256</option></select></div>
          <div class="config-field"><label class="field-label">有效期 (小时)</label><input v-model.number="gwConfig.jwt_ttl_hours" class="field-input" type="number" placeholder="24"></div>
        </div>
        <div class="config-panel">
          <h3 class="section-title">🗝️ API Key</h3>
          <div class="config-field"><label class="toggle-row"><input type="checkbox" v-model="gwConfig.apikey_enabled"><span>启用 API Key</span></label></div>
          <div class="config-field" v-if="gwConfig.api_keys && gwConfig.api_keys.length"><label class="field-label">已配置</label><div class="key-list"><div v-for="(k,i) in gwConfig.api_keys" :key="i" class="key-tag"><code>{{ maskKey(k) }}</code><button class="key-rm" @click="removeApiKey(i)">✕</button></div></div></div>
          <div class="config-field"><label class="field-label">添加 Key</label><div class="add-key-row"><input v-model="newApiKey" class="field-input" placeholder="输入或生成" /><button class="btn btn-sm" @click="genApiKey"><Icon name="refresh" :size="14" /></button><button class="btn btn-sm btn-primary" @click="addApiKey" :disabled="!newApiKey.trim()">添加</button></div></div>
        </div>
      </div>
      <button class="btn btn-primary" @click="saveGwConfig" :disabled="saving" style="margin-top:var(--space-3)">
        <template v-if="!saving"><Icon name="save" :size="14" /> 保存配置</template><template v-else><span class="spinner"></span> 保存中...</template>
      </button>
    </div>

    <!-- 路由对话框 -->
    <Transition name="fade">
      <div v-if="showDialog" class="dialog-overlay" @click.self="showDialog=false">
        <div class="dialog">
          <div class="dialog-header"><span>{{ editingRoute?'编辑路由':'新建路由' }}</span><button class="dlg-close" @click="showDialog=false">✕</button></div>
          <div class="dialog-body">
            <div class="config-field"><label class="field-label">名称 <span class="req">*</span></label><input v-model="routeForm.name" class="field-input" :class="{'has-err':routeErrors.name}" placeholder="路由名称"><span v-if="routeErrors.name" class="field-err">{{ routeErrors.name }}</span></div>
            <div class="config-field"><label class="field-label">方法</label><select v-model="routeForm.method" class="field-select"><option value="">ANY</option><option value="GET">GET</option><option value="POST">POST</option><option value="PUT">PUT</option><option value="DELETE">DELETE</option></select></div>
            <div class="config-field"><label class="field-label">路径 <span class="req">*</span></label><input v-model="routeForm.path_pattern" class="field-input" :class="{'has-err':routeErrors.path}" placeholder="/api/v1/*"><span v-if="routeErrors.path" class="field-err">{{ routeErrors.path }}</span></div>
            <div class="config-field"><label class="field-label">上游 <span class="req">*</span></label><input v-model="routeForm.upstream_url" class="field-input" :class="{'has-err':routeErrors.upstream}" placeholder="http://backend:8080"><span v-if="routeErrors.upstream" class="field-err">{{ routeErrors.upstream }}</span></div>
            <div class="config-field"><label class="field-label">认证</label><select v-model="routeForm.auth_method" class="field-select"><option value="none">none</option><option value="jwt">jwt</option><option value="apikey">apikey</option><option value="both">both</option></select></div>
            <div class="config-field"><label class="field-label">灰度</label><div class="range-row"><input v-model.number="routeForm.canary_percent" type="range" min="0" max="100" class="field-range"><span class="range-val">{{ routeForm.canary_percent||0 }}%</span></div></div>
          </div>
          <div class="dialog-footer">
            <button class="btn btn-sm" @click="showDialog=false">取消</button>
            <button class="btn btn-primary btn-sm" @click="saveRoute" :disabled="savingRoute">{{ savingRoute?'保存中...':'保存' }}</button>
          </div>
        </div>
      </div>
    </Transition>

    <ConfirmModal :visible="confirmVis" :title="confirmTitle" :message="confirmMsg" :type="confirmType" @confirm="onConfirm" @cancel="confirmVis=false" />
    <Transition name="fade"><div v-if="error" class="error-banner">⚠️ {{ error }} <button class="err-x" @click="error=''">✕</button></div></Transition>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, inject } from 'vue'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'

const showToast = inject('showToast', () => {})
const svgBar = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="12" width="4" height="9"/><rect x="10" y="7" width="4" height="14"/><rect x="17" y="3" width="4" height="18"/></svg>'
const svgLock = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>'
const svgGlobe = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M2 12h20"/></svg>'
const svgBranch = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></svg>'
const svgKey = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>'

const activeTab = ref('routes')
const stats = ref({})
const routes = ref([])
const logs = ref([])
const error = ref('')
const showDialog = ref(false)
const editingRoute = ref(null)
const savingRoute = ref(false)
const saving = ref(false)
const showSecret = ref(false)
const searchQuery = ref('')
const filterMethod = ref('ALL')
const selectedIds = ref([])
const routeForm = reactive({ name:'', method:'', path_pattern:'', upstream_url:'', auth_method:'none', canary_percent:0 })
const routeErrors = reactive({ name:'', path:'', upstream:'' })
const jwtForm = reactive({ tenant_id:'', role:'user', expire_hours:24, custom_claims:'' })
const generating = ref(false)
const generatedToken = ref('')
const validateJwt = ref('')
const validating = ref(false)
const validateResult = ref(null)
const testForm = reactive({ method:'GET', path:'' })
const testing = ref(false)
const testResult = ref(null)
const gwConfig = reactive({ jwt_enabled:true, apikey_enabled:false, jwt_secret:'', jwt_algorithm:'HS256', jwt_ttl_hours:24, api_keys:[] })
const newApiKey = ref('')
const confirmVis = ref(false)
const confirmTitle = ref('')
const confirmMsg = ref('')
const confirmType = ref('warning')
let confirmAction = null

const activeRouteCount = computed(() => routes.value.filter(r => r.enabled !== false).length)
const authRouteCount = computed(() => routes.value.filter(r => (r.auth_method || r.auth || 'none') !== 'none').length)
const filteredRoutes = computed(() => {
  let res = routes.value
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase()
    res = res.filter(r => (r.name||'').toLowerCase().includes(q) || (r.path_pattern||r.path||'').toLowerCase().includes(q) || (r.upstream_url||r.upstream||'').toLowerCase().includes(q))
  }
  if (filterMethod.value !== 'ALL') res = res.filter(r => (r.method||'ANY').toUpperCase() === filterMethod.value)
  return res
})
const allSelected = computed(() => filteredRoutes.value.length > 0 && filteredRoutes.value.every(r => selectedIds.value.includes(rid(r))))
const decodedJwt = computed(() => {
  if (!generatedToken.value) return null
  try { const p = generatedToken.value.split('.'); if (p.length !== 3) return null; const d = s => JSON.parse(atob(s.replace(/-/g,'+').replace(/_/g,'/'))); return { header: d(p[0]), payload: d(p[1]) } } catch { return null }
})

function rid(r) { return r.id || r.name }
function toggleSelectAll() { allSelected.value ? selectedIds.value = [] : selectedIds.value = filteredRoutes.value.map(r => rid(r)) }
function toggleSelect(r) { const id = rid(r), i = selectedIds.value.indexOf(id); i >= 0 ? selectedIds.value.splice(i, 1) : selectedIds.value.push(id) }

async function loadStats() {
  try { const d = await api('/api/v1/gateway/routes'); const r = d.routes || d || []; routes.value = r; stats.value = { route_count: r.length, total_requests: d.total_requests, auth_failures: d.auth_failures, canary_requests: d.canary_requests } } catch (e) { error.value = '加载路由失败: ' + e.message }
}
async function loadLogs() { try { const d = await api('/api/v1/gateway/log?limit=50'); logs.value = d.logs || d.entries || d || [] } catch (e) { error.value = '加载日志失败: ' + e.message } }
async function loadConfig() {
  try { const d = await api('/api/v1/gateway/config'); if (d.jwt_enabled != null) gwConfig.jwt_enabled = d.jwt_enabled; if (d.apikey_enabled != null) gwConfig.apikey_enabled = d.apikey_enabled; if (d.jwt_secret) gwConfig.jwt_secret = d.jwt_secret; if (d.jwt_algorithm) gwConfig.jwt_algorithm = d.jwt_algorithm; if (d.jwt_ttl_hours) gwConfig.jwt_ttl_hours = d.jwt_ttl_hours; if (d.api_keys) gwConfig.api_keys = d.api_keys } catch {}
}

function openNewRoute() { editingRoute.value = null; Object.assign(routeForm, { name:'', method:'', path_pattern:'', upstream_url:'', auth_method:'none', canary_percent:0 }); Object.assign(routeErrors, { name:'', path:'', upstream:'' }); showDialog.value = true }
function editRoute(r) { editingRoute.value = r; Object.assign(routeForm, { name: r.name, method: r.method||'', path_pattern: r.path_pattern||r.path, upstream_url: r.upstream_url||r.upstream, auth_method: r.auth_method||r.auth||'none', canary_percent: r.canary_percent??r.canary??0 }); Object.assign(routeErrors, { name:'', path:'', upstream:'' }); showDialog.value = true }

function validateForm() {
  let ok = true; Object.assign(routeErrors, { name:'', path:'', upstream:'' })
  if (!routeForm.name.trim()) { routeErrors.name = '名称不能为空'; ok = false }
  if (!routeForm.path_pattern.trim()) { routeErrors.path = '路径不能为空'; ok = false }
  if (!routeForm.upstream_url.trim()) { routeErrors.upstream = '上游URL不能为空'; ok = false }
  return ok
}

async function saveRoute() {
  if (!validateForm()) return
  savingRoute.value = true
  try {
    const body = { name: routeForm.name, method: routeForm.method||undefined, path_pattern: routeForm.path_pattern, upstream_url: routeForm.upstream_url, auth_method: routeForm.auth_method, canary_percent: routeForm.canary_percent }
    if (editingRoute.value) { await apiPut('/api/v1/gateway/routes/' + (editingRoute.value.id||editingRoute.value.name), body); showToast('路由已更新') }
    else { await apiPost('/api/v1/gateway/routes', body); showToast('路由已创建') }
    showDialog.value = false; loadStats()
  } catch (e) { error.value = '保存失败: ' + e.message } finally { savingRoute.value = false }
}

function confirmDeleteRoute(r) {
  confirmTitle.value = '删除路由'; confirmMsg.value = '确定删除路由 "' + r.name + '" 吗？此操作不可恢复。'; confirmType.value = 'danger'
  confirmAction = async () => { try { await apiDelete('/api/v1/gateway/routes/' + (r.id||r.name)); showToast('路由已删除'); loadStats() } catch (e) { error.value = '删除失败: ' + e.message }; confirmVis.value = false }
  confirmVis.value = true
}
function onConfirm() { if (confirmAction) confirmAction() }

async function toggleRouteEnabled(r) {
  try { await apiPut('/api/v1/gateway/routes/' + (r.id||r.name), { ...r, enabled: r.enabled === false }); showToast(r.enabled === false ? '路由已启用' : '路由已禁用'); loadStats() } catch (e) { error.value = '操作失败: ' + e.message }
}

async function batchEnable() {
  for (const id of selectedIds.value) { const r = routes.value.find(x => rid(x) === id); if (r && r.enabled === false) { try { await apiPut('/api/v1/gateway/routes/' + id, { ...r, enabled: true }) } catch {} } }
  showToast('批量启用完成'); selectedIds.value = []; loadStats()
}
async function batchDisable() {
  for (const id of selectedIds.value) { const r = routes.value.find(x => rid(x) === id); if (r && r.enabled !== false) { try { await apiPut('/api/v1/gateway/routes/' + id, { ...r, enabled: false }) } catch {} } }
  showToast('批量禁用完成'); selectedIds.value = []; loadStats()
}

async function generateToken() {
  generating.value = true; generatedToken.value = ''
  try {
    let extraClaims = {}; if (jwtForm.custom_claims.trim()) { try { extraClaims = JSON.parse(jwtForm.custom_claims) } catch { error.value = 'Claims JSON 格式错误'; generating.value = false; return } }
    const d = await apiPost('/api/v1/gateway/token', { tenant_id: jwtForm.tenant_id, role: jwtForm.role, expire_hours: jwtForm.expire_hours, ...extraClaims })
    generatedToken.value = d.token || d.jwt || ''; showToast('Token 已生成')
  } catch (e) { error.value = '生成失败: ' + e.message } finally { generating.value = false }
}

async function doValidateToken() {
  validating.value = true; validateResult.value = null
  try { validateResult.value = await apiPost('/api/v1/gateway/validate', { token: validateJwt.value }) } catch (e) { error.value = '验证失败: ' + e.message } finally { validating.value = false }
}

function copyText(t) { navigator.clipboard?.writeText(t); showToast('已复制到剪贴板') }

async function testRoute() {
  testing.value = true; testResult.value = null
  try { testResult.value = await apiPost('/api/v1/gateway/test', { method: testForm.method, path: testForm.path }) } catch (e) {
    // Client-side matching fallback
    const matched = routes.value.find(r => {
      const pat = r.path_pattern || r.path || ''
      if (r.method && r.method.toUpperCase() !== testForm.method) return false
      if (pat.endsWith('*')) return testForm.path.startsWith(pat.slice(0, -1))
      return testForm.path === pat
    })
    testResult.value = matched ? { matched: true, route_name: matched.name, upstream_url: matched.upstream_url || matched.upstream, auth_method: matched.auth_method || matched.auth || 'none', canary_percent: matched.canary_percent ?? matched.canary ?? 0 } : { matched: false }
  } finally { testing.value = false }
}

async function saveGwConfig() {
  saving.value = true
  try { await apiPut('/api/v1/gateway/config', { jwt_enabled: gwConfig.jwt_enabled, apikey_enabled: gwConfig.apikey_enabled, jwt_secret: gwConfig.jwt_secret, jwt_algorithm: gwConfig.jwt_algorithm, jwt_ttl_hours: gwConfig.jwt_ttl_hours, api_keys: gwConfig.api_keys }); showToast('配置已保存') } catch (e) { error.value = '保存失败: ' + e.message } finally { saving.value = false }
}

function genApiKey() { const c = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'; let r = 'lgk_'; for (let i = 0; i < 32; i++) r += c[Math.floor(Math.random() * c.length)]; newApiKey.value = r }
function addApiKey() { if (!newApiKey.value.trim()) return; gwConfig.api_keys = [...(gwConfig.api_keys || []), newApiKey.value.trim()]; newApiKey.value = ''; showToast('Key 已添加，保存配置后生效') }
function removeApiKey(i) { gwConfig.api_keys.splice(i, 1); showToast('Key 已移除，保存配置后生效') }

function statusClass(code) { if (!code) return ''; if (code >= 500) return 'status-5xx'; if (code >= 400) return 'status-4xx'; if (code >= 300) return 'status-3xx'; return 'status-2xx' }
function maskKey(k) { if (!k || k.length < 8) return k; return k.slice(0, 6) + '••••' + k.slice(-4) }
function loadAll() { error.value = ''; loadStats(); loadLogs(); loadConfig() }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function formatTime(ts) { if (!ts) return '-'; try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month:'2-digit', day:'2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour:'2-digit', minute:'2-digit', second:'2-digit' }) } catch { return ts } }
function fmtExp(exp) { if (!exp) return ''; try { return new Date(exp * 1000).toLocaleString('zh-CN') } catch { return exp } }

onMounted(loadAll)
</script>

<style scoped>
.gateway-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

.stats-grid-5 { display: grid; grid-template-columns: repeat(5, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }

.tab-bar { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); overflow-x: auto; }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; white-space: nowrap; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }

.section { margin-bottom: var(--space-4); }
.section-title { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); display: flex; align-items: center; gap: 6px; }
.section-desc { font-size: var(--text-xs); color: var(--text-tertiary); margin: -8px 0 12px; }

/* Filter bar */
.filter-bar { display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; }
.filter-left, .filter-right { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; }
.search-box { position: relative; display: flex; align-items: center; }
.search-icon { position: absolute; left: 10px; color: var(--text-tertiary); pointer-events: none; }
.search-input { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 30px 6px 32px; font-size: var(--text-sm); width: 240px; transition: border-color .2s; }
.search-input:focus { outline: none; border-color: var(--color-primary); }
.search-clear { position: absolute; right: 8px; background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 12px; }
.method-pills { display: flex; gap: 2px; }
.mpill { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-tertiary); font-size: 10px; font-weight: 700; padding: 4px 8px; cursor: pointer; transition: all .2s; font-family: var(--font-mono); }
.mpill:hover { color: var(--text-primary); }
.mpill.active { border-color: var(--color-primary); color: var(--color-primary); background: rgba(99,102,241,.1); }
.mpill.active.mp-get { border-color: #10B981; color: #10B981; background: rgba(16,185,129,.1); }
.mpill.active.mp-post { border-color: #3B82F6; color: #3B82F6; background: rgba(59,130,246,.1); }
.mpill.active.mp-put { border-color: #F59E0B; color: #F59E0B; background: rgba(245,158,11,.1); }
.mpill.active.mp-delete { border-color: #EF4444; color: #EF4444; background: rgba(239,68,68,.1); }

/* Table */
.table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th { text-align: left; padding: 8px 10px; background: var(--bg-elevated); color: var(--text-tertiary); font-weight: 600; font-size: 10px; text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle); white-space: nowrap; }
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.row-off { opacity: .5; }
.row-cb { accent-color: var(--color-primary); cursor: pointer; }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.td-name { font-weight: 600; color: var(--text-primary); }
.td-url { max-width: 250px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.td-actions { display: flex; gap: 4px; }
.btn-icon { background: none; border: none; cursor: pointer; font-size: 14px; padding: 4px; border-radius: 4px; transition: all .2s; color: var(--text-tertiary); }
.btn-icon:hover { background: var(--bg-elevated); color: var(--text-primary); }
.btn-icon-danger:hover { color: #EF4444; background: rgba(239,68,68,.1); }

/* Toggle switch */
.tog { width: 34px; height: 18px; border-radius: 9px; border: none; background: rgba(255,255,255,.1); cursor: pointer; position: relative; transition: background .2s; padding: 0; }
.tog.on { background: #10B981; }
.tog-k { position: absolute; top: 2px; left: 2px; width: 14px; height: 14px; border-radius: 50%; background: #fff; transition: transform .2s; box-shadow: 0 1px 3px rgba(0,0,0,.3); }
.tog.on .tog-k { transform: translateX(16px); }

/* Badges */
.method-badge { display: inline-block; padding: 1px 6px; border-radius: 3px; font-size: 10px; font-weight: 700; font-family: var(--font-mono); }
.method-get { background: rgba(16,185,129,.15); color: #6EE7B7; }
.method-post { background: rgba(59,130,246,.15); color: #93C5FD; }
.method-put { background: rgba(245,158,11,.15); color: #FCD34D; }
.method-delete { background: rgba(239,68,68,.15); color: #FCA5A5; }
.method-any { background: rgba(99,102,241,.15); color: #a5b4fc; }
.auth-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.auth-none { background: rgba(100,116,139,.15); color: #94a3b8; }
.auth-jwt { background: rgba(99,102,241,.15); color: #a5b4fc; }
.auth-apikey { background: rgba(245,158,11,.15); color: #fcd34d; }
.auth-both { background: rgba(16,185,129,.15); color: #6ee7b7; }
.status-code { font-family: var(--font-mono); font-weight: 700; font-size: 11px; }
.status-2xx { color: #10B981; } .status-3xx { color: #3B82F6; } .status-4xx { color: #F59E0B; } .status-5xx { color: #EF4444; }

/* JWT */
.jwt-panels { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
.test-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.test-input { width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-3); font-size: var(--text-sm); resize: vertical; font-family: var(--font-mono); box-sizing: border-box; }
.test-input:focus { outline: none; border-color: var(--color-primary); }
.token-output { margin-top: var(--space-3); }
.token-box { display: flex; align-items: flex-start; gap: var(--space-2); background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-2); }
.token-text { flex: 1; font-size: 10px; font-family: var(--font-mono); color: #10B981; word-break: break-all; line-height: 1.4; }
.jwt-decoded { margin-top: var(--space-2); display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-2); }
.jwt-sec { }
.jwt-lbl { font-size: 10px; font-weight: 700; color: var(--text-tertiary); text-transform: uppercase; display: block; margin-bottom: 4px; }
.code-pre { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-2); font-size: 10px; font-family: var(--font-mono); color: var(--text-secondary); overflow-x: auto; white-space: pre-wrap; word-wrap: break-word; max-height: 160px; overflow-y: auto; margin: 0; }
.validate-output { margin-top: var(--space-3); }
.validate-status { font-weight: 700; font-size: var(--text-sm); margin-bottom: var(--space-2); display: flex; align-items: center; gap: var(--space-2); }
.valid-yes { color: #10B981; } .valid-no { color: #EF4444; }
.validate-exp { font-size: var(--text-xs); font-weight: 400; color: var(--text-tertiary); }
.error-text { color: #EF4444; font-size: var(--text-sm); }

/* Route test */
.test-row { display: flex; gap: var(--space-2); align-items: center; margin-bottom: var(--space-3); }
.test-result { padding: var(--space-3); border-radius: var(--radius-md); margin-top: var(--space-2); }
.tr-ok { background: rgba(16,185,129,.08); border: 1px solid rgba(16,185,129,.2); }
.tr-fail { background: rgba(239,68,68,.08); border: 1px solid rgba(239,68,68,.2); }
.tr-hd { font-weight: 700; font-size: var(--text-sm); margin-bottom: var(--space-2); }
.tr-body { display: flex; flex-direction: column; gap: 6px; }
.tr-row { display: flex; align-items: center; gap: var(--space-3); font-size: var(--text-xs); }
.tr-row > span:first-child { color: var(--text-tertiary); min-width: 40px; }
.tr-hint { font-size: var(--text-xs); color: var(--text-tertiary); }
.tr-hint code { background: var(--bg-elevated); padding: 1px 4px; border-radius: 3px; font-size: 10px; }

/* Config */
.config-panels { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
.config-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.config-field { margin-bottom: var(--space-3); display: flex; flex-direction: column; gap: 4px; }
.field-label { font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; }
.req { color: #EF4444; }
.field-input { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); box-sizing: border-box; }
.field-input:focus { outline: none; border-color: var(--color-primary); }
.field-input.has-err { border-color: #EF4444; }
.field-err { font-size: 10px; color: #EF4444; }
.field-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); }
.field-range { flex: 1; accent-color: var(--color-primary); }
.range-row { display: flex; align-items: center; gap: var(--space-2); }
.range-val { font-family: var(--font-mono); font-size: var(--text-sm); color: var(--color-primary); font-weight: 700; min-width: 36px; }
.toggle-row { display: flex; align-items: center; gap: 8px; font-size: var(--text-sm); color: var(--text-secondary); cursor: pointer; }
.toggle-row input { accent-color: var(--color-primary); }
.secret-row { display: flex; gap: 4px; align-items: center; }
.secret-row .field-input { flex: 1; }
.key-list { display: flex; flex-wrap: wrap; gap: var(--space-1); }
.key-tag { display: inline-flex; align-items: center; gap: 6px; padding: 2px 8px; background: rgba(255,255,255,.06); border: 1px solid var(--border-subtle); border-radius: 4px; font-size: 10px; color: var(--text-secondary); }
.key-rm { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 10px; padding: 0; }
.key-rm:hover { color: #EF4444; }
.add-key-row { display: flex; gap: var(--space-2); align-items: center; }
.add-key-row .field-input { flex: 1; }

/* Dialog */
.dialog-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.dialog { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); width: 440px; max-width: 90vw; box-shadow: var(--shadow-lg); }
.dialog-header { padding: var(--space-4); border-bottom: 1px solid var(--border-subtle); font-weight: 700; color: var(--text-primary); font-size: var(--text-base); display: flex; justify-content: space-between; align-items: center; }
.dlg-close { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 16px; }
.dlg-close:hover { color: var(--text-primary); }
.dialog-body { padding: var(--space-4); display: flex; flex-direction: column; gap: var(--space-3); }
.dialog-footer { padding: var(--space-3) var(--space-4); border-top: 1px solid var(--border-subtle); display: flex; justify-content: flex-end; gap: var(--space-2); }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-sm { padding: 6px 12px; font-size: var(--text-xs); }
.btn-outline-ok { border-color: #10B981; color: #10B981; }
.btn-outline-ok:hover { background: rgba(16,185,129,.1); }
.btn-outline-red { border-color: #EF4444; color: #EF4444; }
.btn-outline-red:hover { background: rgba(239,68,68,.1); }
.spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.3); border-top-color: #fff; border-radius: 50%; animation: spin .6s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.empty-state { text-align: center; padding: var(--space-6); color: var(--text-tertiary); font-size: var(--text-sm); }
.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); display: flex; justify-content: space-between; align-items: center; }
.err-x { background: none; border: none; color: #FCA5A5; cursor: pointer; font-size: 14px; }

.fade-enter-active { animation: fadeIn .2s ease; }
.fade-leave-active { animation: fadeIn .15s reverse; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: translateY(0); } }

@media (max-width: 768px) {
  .stats-grid-5 { grid-template-columns: repeat(2, 1fr); }
  .jwt-panels, .config-panels { grid-template-columns: 1fr; }
  .filter-bar { flex-direction: column; align-items: stretch; }
}
</style>
