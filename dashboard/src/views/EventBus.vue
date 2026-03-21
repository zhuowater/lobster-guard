<template>
  <div class="events-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="radio" :size="20" /> 事件总线</h1>
        <p class="page-subtitle">全局安全事件流 — 所有模块的事件统一收集、分发与推送</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
        <button class="btn btn-primary" @click="sendTestEvent" :disabled="testSending">
          <span v-if="testSending" class="spinner"></span>{{ testSending ? '发送中...' : '📡 测试事件' }}
        </button>
      </div>
    </div>

    <div class="stats-grid">
      <StatCard :iconSvg="svgEvents" label="总事件数" :value="stats.total_events ?? 0" color="indigo" />
      <StatCard :iconSvg="svgTarget" label="推送目标" :value="stats.targets_count ?? 0" color="blue" />
      <StatCard :iconSvg="svgDelivered" label="已投递" :value="stats.total_delivered ?? 0" color="green" />
      <StatCard :iconSvg="svgFailed" label="投递失败" :value="stats.total_failed ?? 0" color="red" />
    </div>

    <div class="tab-bar">
      <button class="tab-btn" :class="{active:activeTab==='events'}" @click="activeTab='events'"><Icon name="bar-chart" :size="14" /> 事件流 <span class="tab-count">{{events.length}}</span></button>
      <button class="tab-btn" :class="{active:activeTab==='targets'}" @click="activeTab='targets'">🔗 推送目标 <span class="tab-count">{{targets.length}}</span></button>
      <button class="tab-btn" :class="{active:activeTab==='deliveries'}" @click="activeTab='deliveries'">📬 投递记录 <span class="tab-count">{{deliveries.length}}</span></button>
      <button class="tab-btn" :class="{active:activeTab==='chains'}" @click="activeTab='chains'">⛓ 动作链 <span class="tab-count">{{chains.length}}</span></button>
    </div>

    <div v-if="activeTab==='events'" class="section">
      <div class="section-toolbar">
        <div class="search-box"><Icon name="search" :size="14" /><input v-model="evtSearch" placeholder="搜索事件类型/域/描述..." /></div>
        <select v-model="filterType" class="filter-select" @change="loadEvents"><option value="">全部类型</option><option v-for="t in eventTypes" :key="t" :value="t">{{t}}</option></select>
        <select v-model="filterSeverity" class="filter-select" @change="loadEvents"><option value="">全部严重程度</option><option value="critical">Critical</option><option value="high">High</option><option value="medium">Medium</option><option value="low">Low</option><option value="info">Info</option></select>
        <select v-model="filterTime" class="filter-select" @change="loadEvents"><option value="">全部时间</option><option value="1h">最近1h</option><option value="24h">最近24h</option><option value="7d">最近7d</option></select>
        <label class="auto-refresh-toggle"><input type="checkbox" v-model="autoRefresh" @change="toggleAutoRefresh" /><span>自动刷新</span></label>
      </div>
      <DataTable :columns="eventColumns" :data="filteredEvents" :loading="evtLoading" :expandable="true" rowKey="id" emptyText="暂无事件记录" emptyDesc="系统运行后会自动产生安全事件">
        <template #cell-timestamp="{row}"><span class="mono-cell tc">{{fmtTime(row.timestamp)}}</span></template>
        <template #cell-type="{row}"><span class="type-tag" :style="typeColor(row.type||row.event_type)">{{row.type||row.event_type||'-'}}</span></template>
        <template #cell-severity="{row}"><span class="sev-badge" :class="'sev-'+(row.severity||'low')">{{row.severity||'low'}}</span></template>
        <template #cell-description="{row}"><span class="desc-cell">{{truncate(row.description||row.message||row.summary||row.detail,60)}}</span></template>
        <template #cell-trace_id="{row}"><code class="mono-cell">{{truncate(row.trace_id,12)}}</code></template>
        <template #expand="{row}">
          <div class="expand-detail">
            <div class="dg">
              <div class="di"><span class="dl">事件 ID</span><code>{{row.id}}</code></div>
              <div class="di"><span class="dl">类型</span><span class="type-tag" :style="typeColor(row.type||row.event_type)">{{row.type||row.event_type}}</span></div>
              <div class="di"><span class="dl">严重程度</span><span class="sev-badge" :class="'sev-'+(row.severity||'low')">{{row.severity}}</span></div>
              <div class="di"><span class="dl">域</span><span>{{row.domain||'-'}}</span></div>
              <div class="di"><span class="dl">描述</span><span>{{row.description||row.message||row.summary||'-'}}</span></div>
              <div class="di"><span class="dl">时间</span><span>{{fmtFull(row.timestamp)}}</span></div>
              <div class="di"><span class="dl">TraceID</span><code>{{row.trace_id||'-'}}</code></div>
            </div>
            <div v-if="row.details||row.data" class="dp"><span class="dl">详细数据</span><pre class="dc">{{JSON.stringify(row.details||row.data||{},null,2)}}</pre></div>
          </div>
        </template>
      </DataTable>
    </div>

    <div v-if="activeTab==='targets'" class="section">
      <div class="section-toolbar">
        <div class="search-box"><Icon name="search" :size="14" /><input v-model="targetSearch" placeholder="搜索目标名称/URL..." /></div>
        <button class="btn btn-primary btn-sm" @click="openTargetForm()">+ 添加目标</button>
      </div>
      <DataTable :columns="targetColumns" :data="filteredTargets" :loading="targetLoading" rowKey="id" emptyText="暂无推送目标" emptyDesc="添加 Webhook 目标以接收安全事件通知">
        <template #cell-url="{value}"><code class="mono-cell url-cell">{{truncate(value,50)}}</code></template>
        <template #cell-enabled="{row}"><span class="status-dot" :class="(row.enabled!==false)?'dot-on':'dot-off'">{{(row.enabled!==false)?'✅ 启用':'⚠️ 关闭'}}</span></template>
        <template #cell-event_types="{row}"><span class="evt-types">{{(row.event_types||[]).join(', ')||'全部'}}</span></template>
        <template #actions="{row}">
          <div class="act-row">
            <button class="btn btn-sm btn-ghost" @click="openTargetForm(row)"><Icon name="edit" :size="12" /> 编辑</button>
            <button class="btn btn-sm btn-danger" @click="confirmDeleteTarget(row)"><Icon name="trash" :size="12" /></button>
          </div>
        </template>
      </DataTable>
    </div>

    <div v-if="activeTab==='deliveries'" class="section">
      <DataTable :columns="deliveryColumns" :data="deliveries" :loading="deliveryLoading" :expandable="true" emptyText="暂无投递记录" emptyDesc="事件触发推送后会产生投递记录">
        <template #cell-timestamp="{row}"><span class="mono-cell tc">{{fmtTime(row.timestamp||row.created_at)}}</span></template>
        <template #cell-status="{row}"><span class="delivery-status" :class="'ds-'+(row.status||'unknown')">{{row.status||'unknown'}}</span></template>
        <template #cell-target_id="{value}"><code class="mono-cell">{{truncate(value,12)}}</code></template>
        <template #cell-event_id="{value}"><code class="mono-cell">{{truncate(value,12)}}</code></template>
        <template #expand="{row}"><div class="expand-detail"><pre class="dc">{{JSON.stringify(row,null,2)}}</pre></div></template>
      </DataTable>
    </div>

    <div v-if="activeTab==='chains'" class="section">
      <DataTable :columns="chainColumns" :data="chains" :loading="chainLoading" :expandable="true" emptyText="暂无动作链" emptyDesc="配置事件驱动的自动响应动作链">
        <template #cell-enabled="{row}"><span class="status-dot" :class="(row.enabled!==false)?'dot-on':'dot-off'">{{(row.enabled!==false)?'✅':'⚠️'}}</span></template>
        <template #expand="{row}"><div class="expand-detail"><pre class="dc">{{JSON.stringify(row,null,2)}}</pre></div></template>
      </DataTable>
    </div>

    <Teleport to="body">
      <div v-if="targetFormVisible" class="modal-overlay" @click.self="targetFormVisible=false">
        <div class="modal-box">
          <div class="modal-header"><span>{{editingTarget?'编辑推送目标':'添加推送目标'}}</span><button class="btn-close" @click="targetFormVisible=false">✕</button></div>
          <div class="modal-body">
            <div class="fg"><label class="fl">名称</label><input v-model="targetForm.name" class="fi" placeholder="目标名称..." /></div>
            <div class="fg"><label class="fl">Webhook URL</label><input v-model="targetForm.url" class="fi" placeholder="https://..." /></div>
            <div class="fg"><label class="fl">事件类型过滤 (逗号分隔，留空=全部)</label><input v-model="targetForm.event_types_str" class="fi" placeholder="rule_match, anomaly" /></div>
            <div class="fg"><label class="fl">严重程度过滤 (逗号分隔，留空=全部)</label><input v-model="targetForm.severity_filter_str" class="fi" placeholder="critical, high" /></div>
            <div class="fg"><label class="fl">Secret (可选)</label><input v-model="targetForm.secret" class="fi" type="password" placeholder="Webhook 签名密钥..." /></div>
            <div class="fg"><label class="fl">启用</label>
              <label class="toggle"><input type="checkbox" v-model="targetForm.enabled" /><span class="toggle-track"><span class="toggle-thumb"></span></span><span class="toggle-txt">{{targetForm.enabled?'启用':'关闭'}}</span></label>
            </div>
          </div>
          <div class="modal-footer"><button class="btn btn-sm" @click="targetFormVisible=false">取消</button><button class="btn btn-sm btn-primary" @click="saveTarget" :disabled="targetSaving">{{targetSaving?'保存中...':'保存'}}</button></div>
        </div>
      </div>
    </Teleport>

    <ConfirmModal :visible="deleteConfirm" title="删除推送目标" :message="'确认删除 ' + (deletingTarget?.name||deletingTarget?.id||'') + '？'" type="danger" @confirm="doDeleteTarget" @cancel="deleteConfirm=false" />
    <div v-if="error" class="error-banner" @click="error=''">⚠️ {{error}} <span class="err-x">✕</span></div>
  </div>
</template>
<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'

const activeTab = ref('events')
const stats = ref({})
const events = ref([])
const targets = ref([])
const deliveries = ref([])
const chains = ref([])
const error = ref('')
const evtLoading = ref(false)
const targetLoading = ref(false)
const deliveryLoading = ref(false)
const chainLoading = ref(false)
const testSending = ref(false)
const evtSearch = ref('')
const targetSearch = ref('')
const filterType = ref('')
const filterSeverity = ref('')
const filterTime = ref('')
const eventTypes = ref([])
const autoRefresh = ref(false)
let refreshTimer = null

const targetFormVisible = ref(false)
const editingTarget = ref(null)
const targetSaving = ref(false)
const targetForm = reactive({ name: '', url: '', event_types_str: '', severity_filter_str: '', secret: '', enabled: true })
const deleteConfirm = ref(false)
const deletingTarget = ref(null)

const filteredEvents = computed(() => {
  if (!evtSearch.value.trim()) return events.value
  const q = evtSearch.value.toLowerCase().trim()
  return events.value.filter(e =>
    (e.type || '').toLowerCase().includes(q) ||
    (e.event_type || '').toLowerCase().includes(q) ||
    (e.domain || '').toLowerCase().includes(q) ||
    (e.description || e.message || e.summary || '').toLowerCase().includes(q)
  )
})
const filteredTargets = computed(() => {
  if (!targetSearch.value.trim()) return targets.value
  const q = targetSearch.value.toLowerCase().trim()
  return targets.value.filter(t => (t.name || '').toLowerCase().includes(q) || (t.url || '').toLowerCase().includes(q))
})

const eventColumns = [
  { key: 'timestamp', label: '时间', sortable: true, width: '130px' },
  { key: 'type', label: '类型', sortable: true, width: '120px' },
  { key: 'severity', label: '严重程度', sortable: true, width: '90px' },
  { key: 'domain', label: '域', sortable: true },
  { key: 'description', label: '描述' },
  { key: 'trace_id', label: 'TraceID', width: '130px' },
]
const targetColumns = [
  { key: 'name', label: '名称', sortable: true },
  { key: 'url', label: 'URL' },
  { key: 'enabled', label: '状态', width: '80px' },
  { key: 'event_types', label: '事件过滤' },
]
const deliveryColumns = [
  { key: 'timestamp', label: '时间', sortable: true, width: '130px' },
  { key: 'event_id', label: '事件ID', width: '130px' },
  { key: 'target_id', label: '目标ID', width: '130px' },
  { key: 'status', label: '状态', sortable: true, width: '90px' },
  { key: 'status_code', label: 'HTTP', width: '60px' },
]
const chainColumns = [
  { key: 'name', label: '名称', sortable: true },
  { key: 'trigger_type', label: '触发类型' },
  { key: 'enabled', label: '状态', width: '60px' },
]

const svgEvents = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="20" x2="12" y2="10"/><line x1="18" y1="20" x2="18" y2="4"/><line x1="6" y1="20" x2="6" y2="16"/></svg>'
const svgTarget = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg>'
const svgDelivered = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>'
const svgFailed = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>'

function typeColor(t) {
  if (!t) return {}
  const colors = ['#6366F1', '#8B5CF6', '#EC4899', '#F97316', '#14B8A6', '#06B6D4', '#EAB308', '#84CC16']
  let hash = 0
  for (let i = 0; i < t.length; i++) hash = t.charCodeAt(i) + ((hash << 5) - hash)
  const c = colors[Math.abs(hash) % colors.length]
  return { background: c + '20', color: c, borderColor: c + '40' }
}

async function loadStats() { try { const d = await api('/api/v1/events/stats'); stats.value = d; if (d.event_types) eventTypes.value = d.event_types } catch (e) { error.value = e.message } }
async function loadEvents() {
  evtLoading.value = true
  try {
    let url = '/api/v1/events/list?limit=50'
    if (filterType.value) url += '&type=' + encodeURIComponent(filterType.value)
    if (filterSeverity.value) url += '&severity=' + encodeURIComponent(filterSeverity.value)
    if (filterTime.value) url += '&since=' + encodeURIComponent(filterTime.value)
    const d = await api(url); events.value = d.events || d || []
  } catch (e) { error.value = e.message } finally { evtLoading.value = false }
}
async function loadTargets() { targetLoading.value = true; try { const d = await api('/api/v1/events/targets'); targets.value = d.targets || d || [] } catch (e) { error.value = e.message } finally { targetLoading.value = false } }
async function loadDeliveries() { deliveryLoading.value = true; try { const d = await api('/api/v1/events/deliveries?limit=50'); deliveries.value = d.deliveries || d || [] } catch (e) { error.value = e.message } finally { deliveryLoading.value = false } }
async function loadChains() { chainLoading.value = true; try { const d = await api('/api/v1/events/chains'); chains.value = d.chains || d || [] } catch (e) { error.value = e.message } finally { chainLoading.value = false } }
async function sendTestEvent() { testSending.value = true; try { await apiPost('/api/v1/events/test', {}); showToast('测试事件已发送 📡', 'success'); setTimeout(loadAll, 1000) } catch (e) { showToast('发送失败: ' + e.message, 'error') } finally { testSending.value = false } }

function openTargetForm(t) {
  editingTarget.value = t || null
  targetForm.name = t?.name || ''; targetForm.url = t?.url || ''; targetForm.enabled = t?.enabled !== false
  targetForm.event_types_str = (t?.event_types || []).join(', ')
  targetForm.severity_filter_str = (t?.severity_filter || []).join(', ')
  targetForm.secret = ''; targetFormVisible.value = true
}
async function saveTarget() {
  if (!targetForm.url) { showToast('URL 不能为空', 'error'); return }
  targetSaving.value = true
  const body = { name: targetForm.name, url: targetForm.url, enabled: targetForm.enabled, secret: targetForm.secret || undefined,
    event_types: targetForm.event_types_str ? targetForm.event_types_str.split(',').map(s => s.trim()).filter(Boolean) : undefined,
    severity_filter: targetForm.severity_filter_str ? targetForm.severity_filter_str.split(',').map(s => s.trim()).filter(Boolean) : undefined }
  try {
    if (editingTarget.value) await apiPut('/api/v1/events/targets/' + editingTarget.value.id, body)
    else await apiPost('/api/v1/events/targets', body)
    showToast(editingTarget.value ? '目标已更新' : '目标已添加', 'success'); targetFormVisible.value = false; loadTargets()
  } catch (e) { showToast('保存失败: ' + e.message, 'error') } finally { targetSaving.value = false }
}
function confirmDeleteTarget(t) { deletingTarget.value = t; deleteConfirm.value = true }
async function doDeleteTarget() { deleteConfirm.value = false; if (!deletingTarget.value) return; try { await apiDelete('/api/v1/events/targets/' + deletingTarget.value.id); showToast('目标已删除', 'success'); loadTargets() } catch (e) { showToast('删除失败: ' + e.message, 'error') } }
function toggleAutoRefresh() { clearInterval(refreshTimer); if (autoRefresh.value) refreshTimer = setInterval(() => { loadEvents() }, 10000) }
function loadAll() { error.value = ''; loadStats(); loadEvents(); loadTargets(); loadDeliveries(); loadChains() }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function fmtTime(ts) { if (!ts) return '-'; try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) } catch { return ts } }
function fmtFull(ts) { if (!ts) return '-'; try { return new Date(ts).toLocaleString('zh-CN') } catch { return ts } }
onMounted(loadAll)
onUnmounted(() => clearInterval(refreshTimer))
</script>
<style scoped>
.events-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; display: flex; align-items: center; gap: 8px; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }
.header-actions { display: flex; gap: var(--space-2); align-items: center; }
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.tab-bar { display: flex; gap: var(--space-1); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); flex-wrap: wrap; }
.tab-btn { display: inline-flex; align-items: center; gap: 6px; background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; border-bottom: 2px solid transparent; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom-color: var(--color-primary); font-weight: 600; }
.tab-count { padding: 0 6px; border-radius: 9999px; font-size: 10px; font-weight: 600; background: rgba(99,102,241,.12); color: var(--color-primary); line-height: 1.6; }
.section { margin-bottom: var(--space-4); }
.section-toolbar { display: flex; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; align-items: center; }
.search-box { display: flex; align-items: center; gap: 8px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 6px 12px; flex: 1; min-width: 200px; max-width: 360px; }
.search-box input { background: none; border: none; outline: none; color: var(--text-primary); font-size: var(--text-sm); width: 100%; }
.search-box input::placeholder { color: var(--text-tertiary); }
.filter-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-xs); cursor: pointer; }
.auto-refresh-toggle { display: flex; align-items: center; gap: 6px; font-size: var(--text-xs); color: var(--text-secondary); cursor: pointer; white-space: nowrap; }
.auto-refresh-toggle input { accent-color: var(--color-primary); }
.mono-cell { font-family: var(--font-mono); font-size: 11px; color: var(--text-secondary); }
.tc { color: var(--text-tertiary); }
.type-tag { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; border: 1px solid; }
.sev-badge { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: 10px; font-weight: 700; text-transform: uppercase; }
.sev-critical { background: #DC2626; color: #fff; }
.sev-high { background: #F97316; color: #fff; }
.sev-medium { background: #F59E0B; color: #1a1a2e; }
.sev-low { background: #10B981; color: #fff; }
.sev-info { background: #6B7280; color: #fff; }
.desc-cell { max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: block; font-size: var(--text-xs); color: var(--text-secondary); }
.url-cell { font-size: 10px; }
.evt-types { font-size: var(--text-xs); color: var(--text-tertiary); }
.status-dot { font-size: 11px; font-weight: 600; }
.dot-on { color: #10B981; }
.dot-off { color: var(--text-tertiary); }
.delivery-status { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 700; text-transform: uppercase; }
.ds-success, .ds-delivered { background: rgba(16,185,129,.15); color: #10B981; }
.ds-failed, .ds-error { background: rgba(239,68,68,.15); color: #EF4444; }
.ds-pending { background: rgba(99,102,241,.15); color: #6366F1; }
.ds-unknown { background: rgba(107,114,128,.15); color: #6B7280; }
.act-row { display: flex; gap: var(--space-1); align-items: center; }
.expand-detail { padding: var(--space-2) 0; }
.dg { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: var(--space-3); margin-bottom: var(--space-3); }
.di { display: flex; flex-direction: column; gap: 4px; }
.dl { font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; }
.di code { font-family: var(--font-mono); font-size: 11px; color: var(--text-secondary); word-break: break-all; }
.dp { margin-top: var(--space-2); }
.dc { background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-3); font-size: 11px; font-family: var(--font-mono); color: var(--text-secondary); overflow-x: auto; max-height: 300px; margin-top: var(--space-1); white-space: pre-wrap; word-break: break-all; }
/* Modal */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,.5); z-index: 1000; display: flex; align-items: center; justify-content: center; animation: fadeIn .2s; }
@keyframes fadeIn { from { opacity: 0 } to { opacity: 1 } }
.modal-box { background: var(--bg-surface); border: 1px solid var(--border-default); border-radius: var(--radius-lg); padding: 24px; min-width: 400px; max-width: 540px; box-shadow: 0 16px 64px rgba(0,0,0,.5); animation: slideUp .2s ease-out; }
@keyframes slideUp { from { opacity: 0; transform: translateY(20px) } to { opacity: 1; transform: translateY(0) } }
.modal-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; font-weight: 600; color: var(--text-primary); }
.modal-body { color: var(--text-secondary); font-size: var(--text-sm); margin-bottom: 20px; max-height: 60vh; overflow-y: auto; }
.modal-footer { display: flex; justify-content: flex-end; gap: 8px; }
.btn-close { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 16px; }
.btn-close:hover { color: var(--text-primary); }
.fg { margin-bottom: var(--space-4); }
.fl { display: block; font-size: var(--text-xs); font-weight: 600; color: var(--text-secondary); margin-bottom: var(--space-2); text-transform: uppercase; letter-spacing: .05em; }
.fi { width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 8px 12px; color: var(--text-primary); font-size: var(--text-sm); outline: none; box-sizing: border-box; }
.fi:focus { border-color: var(--color-primary); }
.toggle { display: flex; align-items: center; gap: 10px; cursor: pointer; }
.toggle input { display: none; }
.toggle-track { width: 36px; height: 20px; border-radius: 10px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); position: relative; transition: all .2s; }
.toggle input:checked + .toggle-track { background: var(--color-primary); border-color: var(--color-primary); }
.toggle-thumb { position: absolute; top: 2px; left: 2px; width: 14px; height: 14px; border-radius: 50%; background: #fff; transition: all .2s; }
.toggle input:checked + .toggle-track .toggle-thumb { left: 18px; }
.toggle-txt { font-size: var(--text-sm); color: var(--text-secondary); }
/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-sm { padding: 4px 10px; font-size: var(--text-xs); }
.btn-ghost { background: transparent; border-color: transparent; }
.btn-ghost:hover { background: var(--bg-elevated); }
.btn-danger { background: transparent; border-color: rgba(239,68,68,.3); color: #EF4444; }
.btn-danger:hover { background: rgba(239,68,68,.1); }
.spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.3); border-top-color: #fff; border-radius: 50%; animation: spin .6s linear infinite; margin-right: 4px; }
@keyframes spin { to { transform: rotate(360deg) } }
.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); cursor: pointer; display: flex; justify-content: space-between; }
.err-x { opacity: .5; }
@media (max-width: 768px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } .section-toolbar { flex-direction: column; } .tab-bar { overflow-x: auto; } }
</style>