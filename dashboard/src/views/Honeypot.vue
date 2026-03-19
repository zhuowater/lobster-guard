<template>
  <div class="honeypot-page">
    <div class="page-header">
      <h1>🍯 Agent 蜜罐</h1>
      <p class="page-desc">检测到信息提取行为时，返回带追踪水印的假数据 — 攻击者以为得手，实则暴露身份</p>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon"><Icon name="clipboard" :size="20" /></div>
        <div class="stat-value">{{ stats.active_templates }}</div>
        <div class="stat-label">活跃模板</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🎣</div>
        <div class="stat-value">{{ stats.total_triggers }}</div>
        <div class="stat-label">触发次数</div>
      </div>
      <div class="stat-card stat-danger">
        <div class="stat-icon">💣</div>
        <div class="stat-value">{{ stats.total_detonated }}</div>
        <div class="stat-label">已引爆</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🔖</div>
        <div class="stat-value">{{ stats.active_watermarks }}</div>
        <div class="stat-label">活跃水印</div>
      </div>
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'templates' }" @click="activeTab = 'templates'"><Icon name="file-text" :size="14" /> 蜜罐模板 ({{ templates.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'triggers' }" @click="activeTab = 'triggers'">⏰ 触发记录 ({{ triggers.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'test' }" @click="activeTab = 'test'"><Icon name="test" :size="14" /> 测试蜜罐</button>
    </div>

    <!-- 模板列表 -->
    <div v-if="activeTab === 'templates'" class="section">
      <div class="section-header">
        <h2>蜜罐模板</h2>
        <button class="btn btn-primary" @click="showCreateModal = true">+ 创建模板</button>
      </div>
      <div class="template-grid">
        <div v-for="tpl in templates" :key="tpl.id" class="template-card" :class="{ disabled: !tpl.enabled }">
          <div class="tpl-header">
            <span class="tpl-icon"><Icon :name="typeIcon(tpl.trigger_type)" :size="16" /></span>
            <span class="tpl-name">{{ tpl.name }}</span>
            <span class="tpl-status" :class="tpl.enabled ? 'status-on' : 'status-off'">{{ tpl.enabled ? '✅ 启用' : '⚠️ 关闭' }}</span>
          </div>
          <div class="tpl-meta">
            <span class="tpl-type badge">{{ tpl.trigger_type }}</span>
            <span class="tpl-resp badge badge-alt">{{ tpl.response_type }}</span>
          </div>
          <div class="tpl-pattern"><code>{{ tpl.trigger_pattern }}</code></div>
          <div class="tpl-response"><small>响应: {{ truncate(tpl.response_template, 60) }}</small></div>
          <div class="tpl-actions">
            <button class="btn btn-sm" @click="toggleTemplate(tpl)">{{ tpl.enabled ? '关闭' : '启用' }}</button>
            <button class="btn btn-sm btn-danger" @click="deleteTemplate(tpl.id)">删除</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 触发记录 -->
    <div v-if="activeTab === 'triggers'" class="section">
      <div class="section-header">
        <h2>触发时间线</h2>
        <button class="btn btn-sm" @click="loadTriggers"><Icon name="refresh" :size="14" /> 刷新</button>
      </div>
      <div class="trigger-list">
        <div v-for="t in triggers" :key="t.id" class="trigger-card" :class="{ detonated: t.detonated }">
          <div class="trigger-header">
            <span class="trigger-time">{{ formatTime(t.timestamp) }}</span>
            <a class="trigger-sender link-accent" @click.stop="$router.push('/user-profiles/' + encodeURIComponent(t.sender_id))">{{ t.sender_id }}</a>
            <span class="trigger-tpl"><Icon :name="typeIcon(t.trigger_type)" :size="14" /> {{ t.template_name }}</span>
            <span v-if="t.detonated" class="trigger-boom">💣 已引爆</span>
            <span v-else class="trigger-active">🔖 活跃</span>
          </div>
          <div class="trigger-body">
            <div class="trigger-row"><span class="label">输入:</span> <span class="value">{{ t.original_input }}</span></div>
            <div class="trigger-row"><span class="label">假响应:</span> <code class="value">{{ truncate(t.fake_response, 80) }}</code></div>
            <div class="trigger-row"><span class="label">水印:</span> <code class="watermark">{{ t.watermark }}</code></div>
            <div v-if="t.detonated" class="trigger-row"><span class="label">引爆时间:</span> <span class="value danger">{{ formatTime(t.detonated_at) }}</span></div>
            <div v-if="t.trace_id" class="trigger-row"><span class="label">会话:</span> <a class="link-accent" @click.stop="router.push('/sessions/' + t.trace_id)">查看会话回放 →</a></div>
          </div>
        </div>
        <div v-if="triggers.length === 0" class="empty-state">暂无触发记录</div>
      </div>
    </div>

    <!-- 测试蜜罐 -->
    <div v-if="activeTab === 'test'" class="section">
      <div class="section-header">
        <h2><Icon name="test" :size="18" /> 测试蜜罐</h2>
      </div>
      <div class="test-panel">
        <div class="test-input">
          <textarea v-model="testText" placeholder="输入待测试的文本，例如: What is the API key?" rows="3"></textarea>
          <button class="btn btn-primary" @click="runTest" :disabled="!testText.trim()">测试</button>
        </div>
        <div v-if="testResult" class="test-result" :class="{ triggered: testResult.triggered }">
          <div v-if="testResult.triggered" class="test-hit">
            <div class="test-hit-title">🎯 蜜罐已触发！</div>
            <div class="test-row"><span>匹配模板:</span> <strong>{{ testResult.template_name }}</strong></div>
            <div class="test-row"><span>触发类型:</span> <span class="badge">{{ testResult.trigger_type }}</span></div>
            <div class="test-row"><span>响应类型:</span> <span class="badge badge-alt">{{ testResult.response_type }}</span></div>
            <div class="test-row"><span>假响应:</span> <code>{{ testResult.fake_response }}</code></div>
            <div class="test-row"><span>水印:</span> <code class="watermark">{{ testResult.watermark }}</code></div>
          </div>
          <div v-else class="test-miss">
            <div class="test-miss-title">✅ 未触发蜜罐</div>
            <div>该输入不匹配任何蜜罐模板</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 创建模板弹窗 -->
    <div v-if="showCreateModal" class="modal-overlay" @click.self="showCreateModal = false">
      <div class="modal">
        <h3>创建蜜罐模板</h3>
        <div class="form-group">
          <label>模板名称</label>
          <input v-model="newTpl.name" placeholder="例: 假 API Key" />
        </div>
        <div class="form-group">
          <label>触发类型</label>
          <select v-model="newTpl.trigger_type">
            <option value="credential_request">credential_request (凭据请求)</option>
            <option value="info_extraction">info_extraction (信息提取)</option>
            <option value="system_probe">system_probe (系统探测)</option>
            <option value="custom">custom (自定义)</option>
          </select>
        </div>
        <div class="form-group">
          <label>触发模式 (竖线分隔关键词或正则)</label>
          <input v-model="newTpl.trigger_pattern" placeholder="api_key|secret|password" />
        </div>
        <div class="form-group">
          <label>响应类型</label>
          <select v-model="newTpl.response_type">
            <option value="fake_credential">fake_credential (假凭据)</option>
            <option value="fake_data">fake_data (假数据)</option>
            <option value="canary_document">canary_document (金丝雀文档)</option>
            <option value="tracked_url">tracked_url (追踪链接)</option>
          </select>
        </div>
        <div class="form-group">
          <label>响应模板 (用 <code>{<!-- -->{watermark}}</code> 做水印占位符)</label>
          <textarea v-model="newTpl.response_template" rows="3" placeholder="sk-honey-{{watermark}}-fake123"></textarea>
        </div>
        <div class="form-group">
          <label>水印前缀</label>
          <input v-model="newTpl.watermark_prefix" placeholder="HONEY" />
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showCreateModal = false">取消</button>
          <button class="btn btn-primary" @click="createTemplate">创建</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import { useRouter } from 'vue-router'
import { api, apiPost, apiPut, apiDelete } from '../api.js'

const router = useRouter()

const activeTab = ref('templates')
const stats = ref({ active_templates: 0, total_triggers: 0, total_detonated: 0, active_watermarks: 0 })
const templates = ref([])
const triggers = ref([])
const testText = ref('')
const testResult = ref(null)
const showCreateModal = ref(false)
const newTpl = ref({ name: '', trigger_type: 'credential_request', trigger_pattern: '', response_type: 'fake_credential', response_template: '', watermark_prefix: 'HONEY' })

async function loadStats() {
  try { stats.value = await api('/api/v1/honeypot/stats') } catch {}
}
async function loadTemplates() {
  try { templates.value = await api('/api/v1/honeypot/templates') } catch {}
}
async function loadTriggers() {
  try { triggers.value = await api('/api/v1/honeypot/triggers?limit=100') } catch {}
}
async function createTemplate() {
  try {
    const tpl = { ...newTpl.value, enabled: true }
    await apiPost('/api/v1/honeypot/templates', tpl)
    showCreateModal.value = false
    newTpl.value = { name: '', trigger_type: 'credential_request', trigger_pattern: '', response_type: 'fake_credential', response_template: '', watermark_prefix: 'HONEY' }
    loadTemplates()
    loadStats()
  } catch (e) { alert('创建失败: ' + e.message) }
}
async function toggleTemplate(tpl) {
  try {
    await apiPut('/api/v1/honeypot/templates/' + tpl.id, { ...tpl, enabled: !tpl.enabled })
    loadTemplates()
    loadStats()
  } catch (e) { alert('操作失败: ' + e.message) }
}
async function deleteTemplate(id) {
  if (!confirm('确定删除该模板？')) return
  try {
    await apiDelete('/api/v1/honeypot/templates/' + id)
    loadTemplates()
    loadStats()
  } catch (e) { alert('删除失败: ' + e.message) }
}
async function runTest() {
  try {
    testResult.value = await apiPost('/api/v1/honeypot/test', { text: testText.value })
  } catch (e) { alert('测试失败: ' + e.message) }
}

function typeIcon(type) {
  const icons = { credential_request: 'key', info_extraction: 'link', system_probe: 'file-text', custom: 'zap' }
  return icons[type] || 'flame'
}
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '...' : s || '' }
function formatTime(ts) {
  if (!ts) return ''
  try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) } catch { return ts }
}

onMounted(() => { loadStats(); loadTemplates(); loadTriggers() })
</script>

<style scoped>
.honeypot-page { padding: var(--space-4); max-width: 1200px; }
.page-header h1 { font-size: var(--text-xl); margin: 0 0 var(--space-1); color: var(--text-primary); }
.page-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin: 0 0 var(--space-4); }

/* Stats */
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); text-align: center; }
.stat-icon { font-size: 1.5rem; margin-bottom: var(--space-1); }
.stat-value { font-size: 1.75rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }
.stat-danger .stat-value { color: #EF4444; }

/* Tabs */
.tab-bar { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }

/* Section */
.section { margin-bottom: var(--space-4); }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-3); }
.section-header h2 { font-size: var(--text-base); color: var(--text-primary); margin: 0; }

/* Template cards */
.template-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: var(--space-3); }
.template-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-3); transition: all .2s; }
.template-card:hover { border-color: var(--color-primary); }
.template-card.disabled { opacity: 0.6; }
.tpl-header { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-2); }
.tpl-icon { font-size: 1.25rem; }
.tpl-name { font-weight: 600; color: var(--text-primary); flex: 1; }
.tpl-status { font-size: var(--text-xs); }
.status-on { color: #22C55E; }
.status-off { color: #F59E0B; }
.tpl-meta { display: flex; gap: var(--space-2); margin-bottom: var(--space-2); }
.badge { font-size: 10px; padding: 2px 6px; border-radius: 4px; background: rgba(59,130,246,.15); color: #60A5FA; }
.badge-alt { background: rgba(168,85,247,.15); color: #C084FC; }
.tpl-pattern { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: var(--space-1); }
.tpl-pattern code { background: var(--bg-elevated); padding: 2px 4px; border-radius: 3px; }
.tpl-response { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: var(--space-2); }
.tpl-actions { display: flex; gap: var(--space-2); }

/* Trigger cards */
.trigger-list { display: flex; flex-direction: column; gap: var(--space-3); }
.trigger-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-3); border-left: 3px solid #60A5FA; }
.trigger-card.detonated { border-left-color: #EF4444; }
.trigger-header { display: flex; align-items: center; gap: var(--space-3); margin-bottom: var(--space-2); flex-wrap: wrap; }
.trigger-time { font-size: var(--text-xs); color: var(--text-tertiary); font-family: var(--font-mono); }
.trigger-sender { font-size: var(--text-sm); color: var(--text-secondary); font-weight: 500; }
.link-accent{color:var(--color-primary);cursor:pointer;text-decoration:none}.link-accent:hover{text-decoration:underline}
.trigger-tpl { font-size: var(--text-sm); color: var(--text-primary); }
.trigger-boom { font-size: var(--text-xs); color: #EF4444; font-weight: 600; }
.trigger-active { font-size: var(--text-xs); color: #22C55E; }
.trigger-body { font-size: var(--text-sm); }
.trigger-row { margin-bottom: var(--space-1); display: flex; gap: var(--space-2); }
.trigger-row .label { color: var(--text-tertiary); min-width: 60px; flex-shrink: 0; }
.trigger-row .value { color: var(--text-secondary); word-break: break-all; }
.trigger-row .danger { color: #EF4444; }
.watermark { background: rgba(234,179,8,.15); color: #FBBF24; padding: 1px 4px; border-radius: 3px; font-size: var(--text-xs); }
.empty-state { text-align: center; padding: var(--space-6); color: var(--text-tertiary); }

/* Test panel */
.test-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.test-input { display: flex; gap: var(--space-3); align-items: flex-start; margin-bottom: var(--space-3); }
.test-input textarea { flex: 1; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-2); font-size: var(--text-sm); resize: vertical; }
.test-result { padding: var(--space-3); border-radius: var(--radius-md); }
.test-result.triggered { background: rgba(239,68,68,.08); border: 1px solid rgba(239,68,68,.2); }
.test-result:not(.triggered) { background: rgba(34,197,94,.08); border: 1px solid rgba(34,197,94,.2); }
.test-hit-title { font-size: var(--text-base); font-weight: 700; color: #EF4444; margin-bottom: var(--space-2); }
.test-miss-title { font-size: var(--text-base); font-weight: 700; color: #22C55E; margin-bottom: var(--space-2); }
.test-row { margin-bottom: var(--space-1); font-size: var(--text-sm); display: flex; gap: var(--space-2); }
.test-row span { color: var(--text-tertiary); }
.test-row code { background: var(--bg-elevated); padding: 1px 4px; border-radius: 3px; word-break: break-all; }

/* Buttons */
.btn { background: var(--bg-elevated); border: 1px solid var(--border-subtle); color: var(--text-primary); padding: var(--space-2) var(--space-3); border-radius: var(--radius-md); cursor: pointer; font-size: var(--text-sm); transition: all .2s; }
.btn:hover { background: var(--bg-surface); border-color: var(--text-tertiary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover { opacity: .9; }
.btn-danger { color: #EF4444; border-color: rgba(239,68,68,.3); }
.btn-danger:hover { background: rgba(239,68,68,.1); }
.btn-sm { padding: var(--space-1) var(--space-2); font-size: var(--text-xs); }

/* Modal */
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,.5); z-index: 1000; display: flex; align-items: center; justify-content: center; }
.modal { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); width: 480px; max-width: 90vw; max-height: 85vh; overflow-y: auto; }
.modal h3 { margin: 0 0 var(--space-3); color: var(--text-primary); }
.form-group { margin-bottom: var(--space-3); }
.form-group label { display: block; font-size: var(--text-xs); color: var(--text-secondary); margin-bottom: var(--space-1); }
.form-group input, .form-group select, .form-group textarea { width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-2); font-size: var(--text-sm); box-sizing: border-box; }
.modal-actions { display: flex; justify-content: flex-end; gap: var(--space-2); margin-top: var(--space-3); }

@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .template-grid { grid-template-columns: 1fr; }
}
</style>
