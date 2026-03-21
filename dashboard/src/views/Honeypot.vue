<template>
  <div class="honeypot-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">🍯 Agent 蜜罐</h1>
        <p class="page-subtitle">检测到信息提取行为时，返回带追踪水印的假数据 — 攻击者以为得手，实则暴露身份</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
        <button class="btn btn-primary" @click="openCreate">+ 创建模板</button>
      </div>
    </div>
    <div class="stats-grid">
      <StatCard :iconSvg="svgHoneypot" label="活跃蜜罐" :value="stats.active_templates ?? 0" color="indigo" />
      <StatCard :iconSvg="svgTrigger" label="触发次数" :value="stats.total_triggers ?? 0" color="yellow" />
      <StatCard :iconSvg="svgUser" label="捕获用户" :value="stats.captured_users ?? stats.total_detonated ?? 0" color="red" />
      <StatCard :iconSvg="svgClock" label="最近触发" :value="lastTriggerText" color="green" />
    </div>
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'templates' }" @click="activeTab = 'templates'"><Icon name="file-text" :size="14" /> 蜜罐模板 <span class="tab-count">{{ filteredTemplates.length }}</span></button>
      <button class="tab-btn" :class="{ active: activeTab === 'deployments' }" @click="activeTab = 'deployments'"><Icon name="radio" :size="14" /> 部署管理 <span class="tab-count">{{ deployments.length }}</span></button>
      <button class="tab-btn" :class="{ active: activeTab === 'triggers' }" @click="activeTab = 'triggers'">⏰ 触发记录 <span class="tab-count">{{ filteredTriggers.length }}</span></button>
      <button class="tab-btn" :class="{ active: activeTab === 'test' }" @click="activeTab = 'test'"><Icon name="test" :size="14" /> 测试蜜罐</button>
    </div>

    <!-- 模板列表 -->
    <div v-if="activeTab === 'templates'" class="section">
      <div class="section-toolbar">
        <div class="search-box"><Icon name="search" :size="14" /><input v-model="tplSearch" placeholder="搜索模板名称或模式..." /></div>
        <div class="filter-group">
          <select v-model="tplTypeFilter" class="filter-select"><option value="">全部类型</option><option value="credential_request">凭据请求</option><option value="info_extraction">信息提取</option><option value="system_probe">系统探测</option><option value="custom">自定义</option></select>
          <select v-model="tplStatusFilter" class="filter-select"><option value="">全部状态</option><option value="enabled">已启用</option><option value="disabled">已关闭</option></select>
        </div>
      </div>
      <div class="template-grid" v-if="filteredTemplates.length">
        <div v-for="tpl in filteredTemplates" :key="tpl.id" class="template-card" :class="{ disabled: !tpl.enabled }">
          <div class="tpl-header">
            <span class="tpl-icon"><Icon :name="typeIcon(tpl.trigger_type)" :size="16" /></span>
            <span class="tpl-name">{{ tpl.name }}</span>
            <span class="tpl-status" :class="tpl.enabled ? 'status-on' : 'status-off'">{{ tpl.enabled ? '✅ 启用' : '⚠️ 关闭' }}</span>
          </div>
          <div class="tpl-meta">
            <span class="badge">{{ typeLabel(tpl.trigger_type) }}</span>
            <span class="badge badge-alt">{{ respLabel(tpl.response_type) }}</span>
            <span v-if="tpl.watermark_prefix" class="badge badge-gold">🔖 {{ tpl.watermark_prefix }}</span>
          </div>
          <div class="tpl-pattern"><code>{{ tpl.trigger_pattern }}</code></div>
          <div class="tpl-response"><small>响应: {{ truncate(tpl.response_template, 60) }}</small></div>
          <div v-if="expandedTpl === tpl.id" class="tpl-detail">
            <div class="detail-row"><span class="detail-label">完整响应模板</span></div>
            <pre class="detail-code">{{ tpl.response_template }}</pre>
            <div class="detail-row"><span class="detail-label">水印前缀</span><span class="detail-val">{{ tpl.watermark_prefix || '-' }}</span></div>
            <div class="detail-row"><span class="detail-label">创建时间</span><span class="detail-val">{{ formatTime(tpl.created_at) || '-' }}</span></div>
            <div class="detail-row"><span class="detail-label">ID</span><code class="detail-val mono">{{ tpl.id }}</code></div>
          </div>
          <div class="tpl-actions">
            <button class="btn btn-sm btn-ghost" @click="expandedTpl = expandedTpl === tpl.id ? null : tpl.id">{{ expandedTpl === tpl.id ? '收起' : '详情' }}</button>
            <button class="btn btn-sm" @click="openEdit(tpl)"><Icon name="edit" :size="12" /> 编辑</button>
            <button class="btn btn-sm" @click="toggleTemplate(tpl)">{{ tpl.enabled ? '关闭' : '启用' }}</button>
            <button class="btn btn-sm btn-danger" @click="confirmDeleteTpl(tpl)"><Icon name="trash" :size="12" /></button>
          </div>
        </div>
      </div>
      <EmptyState v-else icon="🍯" title="暂无蜜罐模板" description="创建第一个蜜罐模板，开始捕获恶意Agent" action-text="创建模板" @action="openCreate" />
    </div>

    <!-- 部署管理 -->
    <div v-if="activeTab === 'deployments'" class="section">
      <div class="section-toolbar">
        <div class="search-box"><Icon name="search" :size="14" /><input v-model="deploySearch" placeholder="搜索部署名称..." /></div>
        <select v-model="deployStatusFilter" class="filter-select"><option value="">全部状态</option><option value="active">活跃</option><option value="triggered">已触发</option><option value="expired">已过期</option></select>
      </div>
      <div class="deploy-list" v-if="filteredDeployments.length">
        <div v-for="d in filteredDeployments" :key="d.id" class="deploy-card" :class="'deploy-' + (d.status || 'active')">
          <div class="deploy-header">
            <span class="deploy-status-dot" :class="'dot-' + (d.status || 'active')"></span>
            <span class="deploy-name">{{ d.template_name || d.name || d.id }}</span>
            <span class="deploy-status-label">{{ deployStatusLabel(d.status) }}</span>
            <span class="deploy-time mono">{{ formatTime(d.deployed_at || d.created_at) }}</span>
          </div>
          <div class="deploy-meta">
            <span v-if="d.trigger_count != null"><Icon name="zap" :size="12" /> 触发 {{ d.trigger_count }} 次</span>
            <span v-if="d.watermark"><Icon name="bookmark" :size="12" /> {{ truncate(d.watermark, 20) }}</span>
          </div>
          <div class="deploy-actions">
            <button v-if="d.status === 'active'" class="btn btn-sm btn-danger" @click="confirmRevoke(d)"><Icon name="x-circle" :size="12" /> 撤回</button>
          </div>
        </div>
      </div>
      <EmptyState v-else icon="📡" title="暂无部署" description="蜜罐模板启用后将出现在这里" />
    </div>

    <!-- 触发记录 -->
    <div v-if="activeTab === 'triggers'" class="section">
      <div class="section-toolbar">
        <div class="search-box"><Icon name="search" :size="14" /><input v-model="trigSearch" placeholder="搜索用户ID或模板..." /></div>
        <div class="filter-group">
          <select v-model="trigTypeFilter" class="filter-select"><option value="">全部类型</option><option value="credential_request">凭据请求</option><option value="info_extraction">信息提取</option><option value="system_probe">系统探测</option><option value="custom">自定义</option></select>
          <select v-model="trigDetonatedFilter" class="filter-select"><option value="">全部状态</option><option value="detonated">已引爆</option><option value="active">活跃</option></select>
        </div>
      </div>
      <div class="trigger-list" v-if="filteredTriggers.length">
        <div v-for="t in filteredTriggers" :key="t.id" class="trigger-card" :class="{ detonated: t.detonated }">
          <div class="trigger-header">
            <span class="trigger-time mono">{{ formatTime(t.timestamp) }}</span>
            <a class="trigger-sender link-accent" @click.stop="$router.push('/user-profiles/' + encodeURIComponent(t.sender_id))">{{ t.sender_id }}</a>
            <span class="trigger-tpl"><Icon :name="typeIcon(t.trigger_type)" :size="14" /> {{ t.template_name }}</span>
            <span v-if="t.detonated" class="trigger-boom">💣 已引爆</span>
            <span v-else class="trigger-active-badge">🔖 活跃</span>
          </div>
          <div v-if="expandedTrigger === t.id" class="trigger-body">
            <div class="trigger-row"><span class="label">输入:</span><span class="value">{{ t.original_input }}</span></div>
            <div class="trigger-row"><span class="label">假响应:</span><code class="value">{{ t.fake_response }}</code></div>
            <div class="trigger-row"><span class="label">水印:</span><code class="watermark">{{ t.watermark }}</code></div>
            <div v-if="t.detonated" class="trigger-row"><span class="label">引爆时间:</span><span class="value danger">{{ formatTime(t.detonated_at) }}</span></div>
            <div v-if="t.trace_id" class="trigger-row"><span class="label">会话:</span><a class="link-accent" @click.stop="$router.push('/sessions/' + t.trace_id)">查看会话回放 →</a></div>
            <div v-if="t.user_agent || t.ip || t.context" class="trigger-context">
              <div class="context-title">触发上下文</div>
              <div v-if="t.user_agent" class="trigger-row"><span class="label">UA:</span><code class="value">{{ t.user_agent }}</code></div>
              <div v-if="t.ip" class="trigger-row"><span class="label">IP:</span><span class="value mono">{{ t.ip }}</span></div>
              <div v-if="t.context" class="trigger-row"><span class="label">上下文:</span><span class="value">{{ t.context }}</span></div>
            </div>
          </div>
          <div class="trigger-peek" @click="expandedTrigger = expandedTrigger === t.id ? null : t.id">
            <span v-if="expandedTrigger !== t.id" class="peek-text">{{ truncate(t.original_input, 80) }}</span>
            <span class="peek-toggle">{{ expandedTrigger === t.id ? '收起 ▲' : '展开 ▼' }}</span>
          </div>
        </div>
      </div>
      <EmptyState v-else icon="⏰" title="暂无触发记录" description="当蜜罐被触发时，记录将出现在这里" />
    </div>

    <!-- 测试蜜罐 -->
    <div v-if="activeTab === 'test'" class="section">
      <div class="test-panel">
        <h3 class="section-title"><Icon name="test" :size="16" /> 测试蜜罐</h3>
        <p class="test-desc">输入模拟文本，验证哪个蜜罐模板会被触发。</p>
        <div class="test-input">
          <textarea v-model="testText" placeholder="输入待测试的文本，例如: What is the API key?" rows="3"></textarea>
          <button class="btn btn-primary" @click="runTest" :disabled="!testText.trim() || testing">{{ testing ? '测试中...' : '测试' }}</button>
        </div>
        <div v-if="testResult" class="test-result" :class="{ triggered: testResult.triggered }">
          <div v-if="testResult.triggered" class="test-hit">
            <div class="test-hit-title"><Icon name="crosshair" :size="14" /> 蜜罐已触发！</div>
            <div class="test-row"><span>匹配模板:</span><strong>{{ testResult.template_name }}</strong></div>
            <div class="test-row"><span>触发类型:</span><span class="badge">{{ typeLabel(testResult.trigger_type) }}</span></div>
            <div class="test-row"><span>响应类型:</span><span class="badge badge-alt">{{ respLabel(testResult.response_type) }}</span></div>
            <div class="test-row"><span>假响应:</span><code>{{ testResult.fake_response }}</code></div>
            <div class="test-row"><span>水印:</span><code class="watermark">{{ testResult.watermark }}</code></div>
          </div>
          <div v-else class="test-miss"><div class="test-miss-title">✅ 未触发蜜罐</div><div>该输入不匹配任何蜜罐模板</div></div>
        </div>
      </div>
    </div>

    <!-- 创建/编辑弹窗 -->
    <Teleport to="body">
      <div v-if="showFormModal" class="modal-overlay" @click.self="showFormModal = false">
        <div class="modal">
          <h3>{{ editingTpl ? '编辑蜜罐模板' : '创建蜜罐模板' }}</h3>
          <div class="form-group"><label>模板名称 <span class="required">*</span></label><input v-model="formTpl.name" placeholder="例: 假 API Key" /></div>
          <div class="form-row">
            <div class="form-group"><label>触发类型</label><select v-model="formTpl.trigger_type"><option value="credential_request">凭据请求</option><option value="info_extraction">信息提取</option><option value="system_probe">系统探测</option><option value="custom">自定义</option></select></div>
            <div class="form-group"><label>响应类型</label><select v-model="formTpl.response_type"><option value="fake_credential">假凭据</option><option value="fake_data">假数据</option><option value="canary_document">金丝雀文档</option><option value="tracked_url">追踪链接</option></select></div>
          </div>
          <div class="form-group"><label>触发模式 <span class="hint">(竖线分隔关键词或正则)</span></label><input v-model="formTpl.trigger_pattern" placeholder="api_key|secret|password" /></div>
          <div class="form-group"><label>响应模板</label><textarea v-model="formTpl.response_template" rows="3" placeholder="sk-honey-{{watermark}}-fake123"></textarea></div>
          <div class="form-group"><label>水印前缀</label><input v-model="formTpl.watermark_prefix" placeholder="HONEY" /></div>
          <div class="modal-actions">
            <button class="btn" @click="showFormModal = false">取消</button>
            <button class="btn btn-primary" @click="submitTemplate" :disabled="!formTpl.name.trim()">{{ editingTpl ? '保存修改' : '创建' }}</button>
          </div>
        </div>
      </div>
    </Teleport>
    <ConfirmModal :visible="confirmAction.show" :title="confirmAction.title" :message="confirmAction.message" :type="confirmAction.type" :confirm-text="confirmAction.confirmText" @confirm="confirmAction.onConfirm(); confirmAction.show = false" @cancel="confirmAction.show = false" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import EmptyState from '../components/EmptyState.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { useRouter } from 'vue-router'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'

const router = useRouter()
const activeTab = ref('templates')
const stats = ref({ active_templates: 0, total_triggers: 0, total_detonated: 0, captured_users: 0 })
const templates = ref([])
const triggers = ref([])
const deployments = ref([])
const testText = ref('')
const testResult = ref(null)
const testing = ref(false)
const tplSearch = ref('')
const tplTypeFilter = ref('')
const tplStatusFilter = ref('')
const expandedTpl = ref(null)
const trigSearch = ref('')
const trigTypeFilter = ref('')
const trigDetonatedFilter = ref('')
const expandedTrigger = ref(null)
const deploySearch = ref('')
const deployStatusFilter = ref('')
const showFormModal = ref(false)
const editingTpl = ref(null)
const formTpl = ref(emptyForm())
const confirmAction = reactive({ show: false, title: '', message: '', type: 'danger', confirmText: '确认', onConfirm: () => {} })

const svgHoneypot = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a7 7 0 0 0-7 7c0 5 7 13 7 13s7-8 7-13a7 7 0 0 0-7-7z"/><circle cx="12" cy="9" r="2.5"/></svg>'
const svgTrigger = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>'
const svgUser = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>'
const svgClock = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>'

const filteredTemplates = computed(() => {
  let list = templates.value
  if (tplSearch.value) { const q = tplSearch.value.toLowerCase(); list = list.filter(t => (t.name||'').toLowerCase().includes(q) || (t.trigger_pattern||'').toLowerCase().includes(q)) }
  if (tplTypeFilter.value) list = list.filter(t => t.trigger_type === tplTypeFilter.value)
  if (tplStatusFilter.value) list = list.filter(t => tplStatusFilter.value === 'enabled' ? t.enabled : !t.enabled)
  return list
})
const filteredTriggers = computed(() => {
  let list = triggers.value
  if (trigSearch.value) { const q = trigSearch.value.toLowerCase(); list = list.filter(t => (t.sender_id||'').toLowerCase().includes(q) || (t.template_name||'').toLowerCase().includes(q)) }
  if (trigTypeFilter.value) list = list.filter(t => t.trigger_type === trigTypeFilter.value)
  if (trigDetonatedFilter.value) list = list.filter(t => trigDetonatedFilter.value === 'detonated' ? t.detonated : !t.detonated)
  return list
})
const filteredDeployments = computed(() => {
  let list = deployments.value
  if (deploySearch.value) { const q = deploySearch.value.toLowerCase(); list = list.filter(d => ((d.template_name||d.name||'')+d.id).toLowerCase().includes(q)) }
  if (deployStatusFilter.value) list = list.filter(d => (d.status||'active') === deployStatusFilter.value)
  return list
})
const lastTriggerText = computed(() => { if (!triggers.value.length) return '无'; return formatTimeShort(triggers.value[0].timestamp) })

async function loadStats() { try { stats.value = await api('/api/v1/honeypot/stats') } catch {} }
async function loadTemplates() { try { templates.value = await api('/api/v1/honeypot/templates') } catch {} }
async function loadTriggers() { try { triggers.value = await api('/api/v1/honeypot/triggers?limit=200') } catch {} }
async function loadDeployments() {
  const byTpl = {}
  for (const tr of triggers.value) { const key = tr.template_id||tr.template_name||tr.id; if (!byTpl[key]) byTpl[key] = { id: key, template_name: tr.template_name, status: 'active', trigger_count: 0, deployed_at: tr.timestamp, watermark: tr.watermark }; byTpl[key].trigger_count++; if (tr.detonated) byTpl[key].status = 'triggered' }
  for (const t of templates.value) { if (!byTpl[t.id]) byTpl[t.id] = { id: t.id, template_name: t.name, status: t.enabled ? 'active' : 'expired', trigger_count: 0, deployed_at: t.created_at } }
  deployments.value = Object.values(byTpl).sort((a,b) => a.status==='active'&&b.status!=='active'?-1:b.status==='active'&&a.status!=='active'?1:0)
}
function loadAll() { loadStats(); loadTemplates().then(() => loadTriggers().then(loadDeployments)) }

async function submitTemplate() {
  try {
    const body = { ...formTpl.value, enabled: editingTpl.value ? formTpl.value.enabled : true }
    if (editingTpl.value) { await apiPut('/api/v1/honeypot/templates/' + editingTpl.value.id, body); showToast('模板已更新', 'success') }
    else { await apiPost('/api/v1/honeypot/templates', body); showToast('模板已创建', 'success') }
    showFormModal.value = false; editingTpl.value = null; formTpl.value = emptyForm(); loadTemplates(); loadStats()
  } catch (e) { showToast('操作失败: ' + e.message, 'error') }
}
async function toggleTemplate(tpl) {
  try { await apiPut('/api/v1/honeypot/templates/' + tpl.id, { ...tpl, enabled: !tpl.enabled }); showToast(tpl.enabled ? '已关闭模板' : '已启用模板', 'success'); loadTemplates(); loadStats() }
  catch (e) { showToast('操作失败: ' + e.message, 'error') }
}
async function doDeleteTpl(id) { try { await apiDelete('/api/v1/honeypot/templates/' + id); showToast('模板已删除', 'success'); loadTemplates(); loadStats() } catch (e) { showToast('删除失败: ' + e.message, 'error') } }
async function doRevoke(d) { try { const tpl = templates.value.find(t => t.id === d.id || t.name === d.template_name); if (tpl) await apiPut('/api/v1/honeypot/templates/' + tpl.id, { ...tpl, enabled: false }); showToast('已撤回蜜罐部署', 'success'); loadAll() } catch (e) { showToast('撤回失败: ' + e.message, 'error') } }
async function runTest() { testing.value = true; try { testResult.value = await apiPost('/api/v1/honeypot/test', { text: testText.value }) } catch (e) { showToast('测试失败: ' + e.message, 'error') } finally { testing.value = false } }

function openCreate() { editingTpl.value = null; formTpl.value = emptyForm(); showFormModal.value = true }
function openEdit(tpl) { editingTpl.value = tpl; formTpl.value = { ...tpl }; showFormModal.value = true }
function confirmDeleteTpl(tpl) { Object.assign(confirmAction, { show: true, title: '删除蜜罐模板', message: '确定删除模板 "' + tpl.name + '"？', type: 'danger', confirmText: '删除', onConfirm: () => doDeleteTpl(tpl.id) }) }
function confirmRevoke(d) { Object.assign(confirmAction, { show: true, title: '撤回蜜罐部署', message: '确定撤回 "' + (d.template_name||d.id) + '" 的部署？', type: 'warning', confirmText: '撤回', onConfirm: () => doRevoke(d) }) }

function emptyForm() { return { name: '', trigger_type: 'credential_request', trigger_pattern: '', response_type: 'fake_credential', response_template: '', watermark_prefix: 'HONEY' } }
function typeIcon(type) { return { credential_request: 'key', info_extraction: 'link', system_probe: 'file-text', custom: 'zap' }[type] || 'flame' }
function typeLabel(type) { return { credential_request: '凭据请求', info_extraction: '信息提取', system_probe: '系统探测', custom: '自定义' }[type] || type }
function respLabel(type) { return { fake_credential: '假凭据', fake_data: '假数据', canary_document: '金丝雀文档', tracked_url: '追踪链接' }[type] || type }
function deployStatusLabel(s) { return { active: '🟢 活跃', triggered: '💣 已触发', expired: '⚪ 已过期' }[s] || s || '🟢 活跃' }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '...' : s || '' }
function formatTime(ts) { if (!ts) return ''; try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) } catch { return ts } }
function formatTimeShort(ts) { if (!ts) return '无'; try { const d = new Date(ts); const now = new Date(); const diff = now - d; if (diff < 60000) return '刚刚'; if (diff < 3600000) return Math.floor(diff/60000) + '分钟前'; if (diff < 86400000) return Math.floor(diff/3600000) + '小时前'; return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) } catch { return ts } }

onMounted(loadAll)
</script>

<style scoped>
.honeypot-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: var(--space-4); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }
.header-actions { display: flex; gap: var(--space-2); align-items: center; }
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.tab-bar { display: flex; gap: var(--space-1); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); overflow-x: auto; }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; white-space: nowrap; display: flex; align-items: center; gap: 6px; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }
.tab-count { font-size: 10px; background: var(--bg-elevated); padding: 1px 6px; border-radius: 9999px; color: var(--text-tertiary); font-weight: 600; }
.tab-btn.active .tab-count { background: rgba(99,102,241,.15); color: var(--color-primary); }
.section { margin-bottom: var(--space-4); }
.section-toolbar { display: flex; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; align-items: center; }
.search-box { display: flex; align-items: center; gap: var(--space-2); background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-1) var(--space-2); flex: 1; min-width: 200px; }
.search-box input { background: none; border: none; color: var(--text-primary); font-size: var(--text-sm); outline: none; width: 100%; }
.filter-group { display: flex; gap: var(--space-2); }
.filter-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-1) var(--space-2); font-size: var(--text-xs); outline: none; }
.filter-select option { background: var(--bg-elevated); }
.section-title { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-2); }

/* Template cards */
.template-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: var(--space-3); }
.template-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-3); transition: all .2s; }
.template-card:hover { border-color: var(--color-primary); box-shadow: 0 2px 12px rgba(99,102,241,.08); }
.template-card.disabled { opacity: 0.55; }
.tpl-header { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-2); }
.tpl-icon { font-size: 1.1rem; color: var(--color-primary); }
.tpl-name { font-weight: 600; color: var(--text-primary); flex: 1; font-size: var(--text-sm); }
.tpl-status { font-size: var(--text-xs); }
.status-on { color: #22C55E; }
.status-off { color: #F59E0B; }
.tpl-meta { display: flex; gap: var(--space-2); margin-bottom: var(--space-2); flex-wrap: wrap; }
.badge { font-size: 10px; padding: 2px 6px; border-radius: 4px; background: rgba(99,102,241,.15); color: #a5b4fc; font-weight: 600; }
.badge-alt { background: rgba(168,85,247,.15); color: #C084FC; }
.badge-gold { background: rgba(234,179,8,.15); color: #FBBF24; }
.tpl-pattern { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: var(--space-1); }
.tpl-pattern code { background: var(--bg-elevated); padding: 2px 4px; border-radius: 3px; font-size: 11px; }
.tpl-response { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: var(--space-2); }
.tpl-detail { background: var(--bg-elevated); border-radius: var(--radius-md); padding: var(--space-3); margin: var(--space-2) 0; border: 1px solid var(--border-subtle); }
.detail-row { display: flex; justify-content: space-between; padding: var(--space-1) 0; font-size: var(--text-xs); }
.detail-label { color: var(--text-tertiary); font-weight: 600; }
.detail-val { color: var(--text-secondary); }
.detail-code { background: var(--bg-surface); padding: var(--space-2); border-radius: var(--radius-sm); font-size: 11px; color: var(--text-secondary); overflow-x: auto; white-space: pre-wrap; word-break: break-all; margin: var(--space-1) 0; }
.mono { font-family: var(--font-mono); font-size: 11px; }
.tpl-actions { display: flex; gap: var(--space-2); flex-wrap: wrap; }

/* Deploy cards */
.deploy-list { display: flex; flex-direction: column; gap: var(--space-2); }
.deploy-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-3); transition: all .2s; }
.deploy-card:hover { border-color: var(--border-default); }
.deploy-active { border-left: 3px solid #22C55E; }
.deploy-triggered { border-left: 3px solid #EF4444; }
.deploy-expired { border-left: 3px solid var(--text-disabled); opacity: 0.7; }
.deploy-header { display: flex; align-items: center; gap: var(--space-3); flex-wrap: wrap; }
.deploy-status-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.dot-active { background: #22C55E; box-shadow: 0 0 6px rgba(34,197,94,.4); }
.dot-triggered { background: #EF4444; }
.dot-expired { background: var(--text-disabled); }
.deploy-name { font-weight: 600; color: var(--text-primary); font-size: var(--text-sm); }
.deploy-status-label { font-size: var(--text-xs); color: var(--text-secondary); }
.deploy-time { font-size: var(--text-xs); color: var(--text-tertiary); margin-left: auto; }
.deploy-meta { display: flex; gap: var(--space-3); margin-top: var(--space-2); font-size: var(--text-xs); color: var(--text-tertiary); }
.deploy-meta span { display: flex; align-items: center; gap: 4px; }
.deploy-actions { margin-top: var(--space-2); }

/* Trigger cards */
.trigger-list { display: flex; flex-direction: column; gap: var(--space-2); }
.trigger-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-3); border-left: 3px solid #60A5FA; transition: all .2s; }
.trigger-card.detonated { border-left-color: #EF4444; }
.trigger-header { display: flex; align-items: center; gap: var(--space-3); flex-wrap: wrap; }
.trigger-time { font-size: var(--text-xs); color: var(--text-tertiary); }
.trigger-sender { font-size: var(--text-sm); color: var(--text-secondary); font-weight: 500; }
.link-accent { color: var(--color-primary); cursor: pointer; text-decoration: none; }
.link-accent:hover { text-decoration: underline; }
.trigger-tpl { font-size: var(--text-sm); color: var(--text-primary); display: flex; align-items: center; gap: 4px; }
.trigger-boom { font-size: var(--text-xs); color: #EF4444; font-weight: 600; }
.trigger-active-badge { font-size: var(--text-xs); color: #22C55E; }
.trigger-body { padding: var(--space-3); background: var(--bg-elevated); border-radius: var(--radius-md); margin-top: var(--space-2); }
.trigger-row { margin-bottom: var(--space-1); display: flex; gap: var(--space-2); font-size: var(--text-sm); }
.trigger-row .label { color: var(--text-tertiary); min-width: 60px; flex-shrink: 0; font-weight: 600; font-size: var(--text-xs); }
.trigger-row .value { color: var(--text-secondary); word-break: break-all; }
.trigger-row .danger { color: #EF4444; }
.watermark { background: rgba(234,179,8,.15); color: #FBBF24; padding: 1px 4px; border-radius: 3px; font-size: var(--text-xs); }
.trigger-context { margin-top: var(--space-2); padding-top: var(--space-2); border-top: 1px dashed var(--border-subtle); }
.context-title { font-size: var(--text-xs); font-weight: 700; color: var(--text-tertiary); margin-bottom: var(--space-1); text-transform: uppercase; letter-spacing: .05em; }
.trigger-peek { display: flex; align-items: center; justify-content: space-between; margin-top: var(--space-2); cursor: pointer; padding: var(--space-1) 0; }
.peek-text { font-size: var(--text-xs); color: var(--text-tertiary); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.peek-toggle { font-size: var(--text-xs); color: var(--color-primary); font-weight: 600; flex-shrink: 0; margin-left: var(--space-2); }

/* Test panel */
.test-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.test-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin-bottom: var(--space-3); }
.test-input { display: flex; gap: var(--space-3); align-items: flex-start; margin-bottom: var(--space-3); }
.test-input textarea { flex: 1; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-2); font-size: var(--text-sm); resize: vertical; font-family: var(--font-mono); }
.test-result { padding: var(--space-3); border-radius: var(--radius-md); }
.test-result.triggered { background: rgba(239,68,68,.08); border: 1px solid rgba(239,68,68,.2); }
.test-result:not(.triggered) { background: rgba(34,197,94,.08); border: 1px solid rgba(34,197,94,.2); }
.test-hit-title { font-size: var(--text-base); font-weight: 700; color: #EF4444; margin-bottom: var(--space-2); display: flex; align-items: center; gap: 6px; }
.test-miss-title { font-size: var(--text-base); font-weight: 700; color: #22C55E; margin-bottom: var(--space-2); }
.test-row { margin-bottom: var(--space-1); font-size: var(--text-sm); display: flex; gap: var(--space-2); align-items: baseline; }
.test-row span { color: var(--text-tertiary); flex-shrink: 0; }
.test-row code { background: var(--bg-elevated); padding: 1px 4px; border-radius: 3px; word-break: break-all; }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); color: var(--text-primary); padding: var(--space-2) var(--space-3); border-radius: var(--radius-md); cursor: pointer; font-size: var(--text-sm); transition: all .2s; font-weight: 500; }
.btn:hover { background: var(--bg-surface); border-color: var(--text-tertiary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-danger { color: #EF4444; border-color: rgba(239,68,68,.3); }
.btn-danger:hover { background: rgba(239,68,68,.1); }
.btn-ghost { background: transparent; border-color: transparent; color: var(--text-secondary); }
.btn-ghost:hover { background: var(--bg-elevated); color: var(--text-primary); }
.btn-sm { padding: var(--space-1) var(--space-2); font-size: var(--text-xs); }

/* Modal */
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,.5); z-index: 1000; display: flex; align-items: center; justify-content: center; animation: fadeIn .2s; }
@keyframes fadeIn { from { opacity: 0 } to { opacity: 1 } }
.modal { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); width: 520px; max-width: 92vw; max-height: 85vh; overflow-y: auto; box-shadow: 0 16px 64px rgba(0,0,0,.5); animation: slideUp .2s ease-out; }
@keyframes slideUp { from { opacity: 0; transform: translateY(20px) } to { opacity: 1; transform: translateY(0) } }
.modal h3 { margin: 0 0 var(--space-3); color: var(--text-primary); font-size: var(--text-base); }
.form-group { margin-bottom: var(--space-3); }
.form-group label { display: block; font-size: var(--text-xs); color: var(--text-secondary); margin-bottom: var(--space-1); font-weight: 600; }
.form-group input, .form-group select, .form-group textarea { width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-2); font-size: var(--text-sm); box-sizing: border-box; }
.form-group input:focus, .form-group select:focus, .form-group textarea:focus { border-color: var(--color-primary); outline: none; box-shadow: 0 0 0 2px rgba(99,102,241,.2); }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); }
.required { color: #EF4444; }
.hint { color: var(--text-tertiary); font-weight: 400; }
.modal-actions { display: flex; justify-content: flex-end; gap: var(--space-2); margin-top: var(--space-3); padding-top: var(--space-3); border-top: 1px solid var(--border-subtle); }

.spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.3); border-top-color: #fff; border-radius: 50%; animation: spin .6s linear infinite; }
@keyframes spin { to { transform: rotate(360deg) } }

@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .template-grid { grid-template-columns: 1fr; }
  .form-row { grid-template-columns: 1fr; }
  .trigger-header { flex-direction: column; align-items: flex-start; gap: var(--space-1); }
}
</style>
