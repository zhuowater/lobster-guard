<template>
  <div class="cache-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="save" :size="20" /> 响应缓存</h1>
        <p class="page-subtitle">LLM 响应智能缓存 — 节省 Token 开销、降低延迟、支持租户隔离</p>
      </div>
      <div class="header-actions">
        <label class="auto-refresh-toggle" :class="{ on: autoRefresh }">
          <input type="checkbox" v-model="autoRefresh" @change="onAutoRefreshChange" />
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
          30s
        </label>
        <button class="btn btn-sm" @click="loadAll">🔄 刷新</button>
      </div>
    </div>

    <!-- 统计面板 StatCards -->
    <div class="stats-grid">
      <StatCard :iconSvg="svgEntries" :value="stats.entries ?? '-'" label="缓存条目数" color="indigo" class="stat-clickable" @click="activeTab = 'entries'" />
      <StatCard :iconSvg="svgHit" :value="stats.hit_rate != null ? (stats.hit_rate * 100).toFixed(1) + '%' : '-'" label="缓存命中率" color="green" class="stat-clickable" @click="activeTab = 'entries'" />
      <StatCard :iconSvg="svgSize" :value="cacheSizeDisplay" label="缓存大小" color="blue" />
      <StatCard :iconSvg="svgSave" :value="stats.tokens_saved != null ? formatNum(stats.tokens_saved) : '-'" label="节省 Token" color="yellow" />
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'entries' }" @click="activeTab = 'entries'">📦 缓存条目 ({{ filteredEntries.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'lookup' }" @click="activeTab = 'lookup'"><Icon name="search" :size="14" /> 测试查询</button>
      <button class="tab-btn" :class="{ active: activeTab === 'manage' }" @click="activeTab = 'manage'">🗑️ 管理</button>
      <button class="tab-btn" :class="{ active: activeTab === 'config' }" @click="activeTab = 'config'"><Icon name="settings" :size="14" /> 策略配置</button>
    </div>

    <!-- 缓存条目列表 -->
    <div v-if="activeTab === 'entries'" class="section">
      <!-- 搜索栏 -->
      <div class="search-bar">
        <div class="search-input-wrap">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="searchQuery" class="search-input" placeholder="按 key / 查询文本搜索…" @input="onSearch" />
          <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''; onSearch()">✕</button>
        </div>
        <div class="search-meta">
          <span v-if="selectedKeys.size > 0" class="selected-count">已选 {{ selectedKeys.size }} 项</span>
          <button v-if="selectedKeys.size > 0" class="btn btn-sm btn-danger" @click="showBatchClearConfirm = true">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/></svg>
            批量删除
          </button>
          <button class="btn btn-sm btn-danger" @click="showClearAllConfirm = true">🗑️ 清空全部</button>
        </div>
      </div>

      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th style="width:36px"><input type="checkbox" :checked="isAllSelected" @change="toggleSelectAll" :indeterminate="isPartialSelected" /></th>
              <th>查询 / Key</th>
              <th>模型</th>
              <th>命中次数</th>
              <th>节省 Token</th>
              <th>最后命中</th>
              <th>创建时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(e, idx) in filteredEntries" :key="idx" :class="{ 'row-selected': selectedKeys.has(entryKey(e)) }">
              <td><input type="checkbox" :checked="selectedKeys.has(entryKey(e))" @change="toggleSelect(e)" /></td>
              <td class="td-payload" :title="e.query || e.key">{{ truncate(e.query || e.key, 60) }}</td>
              <td class="td-mono">{{ e.model || '-' }}</td>
              <td class="td-mono"><span class="hit-count-badge">{{ e.hit_count ?? 0 }}</span></td>
              <td class="td-mono">{{ formatNum(e.tokens_saved ?? 0) }}</td>
              <td class="td-mono">{{ formatTime(e.last_hit || e.last_accessed) }}</td>
              <td class="td-mono">{{ formatTime(e.created_at || e.created) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="filteredEntries.length === 0" class="empty-state">
          <template v-if="searchQuery">未找到匹配 "{{ searchQuery }}" 的缓存条目</template>
          <template v-else>暂无缓存条目</template>
        </div>
      </div>
    </div>

    <!-- 测试查询区 -->
    <div v-if="activeTab === 'lookup'" class="section">
      <div class="test-panel">
        <h3 class="section-title">缓存查询测试</h3>
        <div class="test-row">
          <div class="test-field">
            <label class="field-label">查询文本</label>
            <textarea v-model="lookupQuery" class="test-input" rows="3" placeholder="输入查询文本..."></textarea>
          </div>
        </div>
        <div class="test-row" style="margin-top:var(--space-2)">
          <div class="test-field" style="max-width:200px">
            <label class="field-label">租户 ID</label>
            <input v-model="lookupTenant" class="field-input" placeholder="可选">
          </div>
        </div>
        <button class="btn btn-primary" @click="lookupCache" :disabled="looking || !lookupQuery.trim()" style="margin-top:var(--space-2)">
          <span v-if="looking" class="spinner"></span>
          <template v-if="!looking"><Icon name="search" :size="14" /> 查询缓存</template><template v-else>查询中...</template>
        </button>
      </div>
      <div v-if="lookupResult" class="lookup-result">
        <div class="result-header">
          <span>查询结果</span>
          <button class="btn-close" @click="lookupResult = null">✕</button>
        </div>
        <div class="hit-status" :class="lookupResult.hit ? 'hit-yes' : 'hit-no'">
          {{ lookupResult.hit ? '🎯 命中缓存' : '❌ 未命中' }}
        </div>
        <div v-if="lookupResult.hit && lookupResult.response" class="cached-response">
          <strong>缓存响应：</strong>
          <pre class="response-pre">{{ lookupResult.response }}</pre>
        </div>
      </div>
    </div>

    <!-- 管理 -->
    <div v-if="activeTab === 'manage'" class="section">
      <div class="manage-panel">
        <h3 class="section-title">缓存管理</h3>
        <div class="manage-actions">
          <button class="btn btn-danger" @click="showClearAllConfirm = true">🗑️ 清除全部缓存</button>
          <div class="tenant-clear">
            <input v-model="clearTenantId" class="field-input" placeholder="输入租户 ID" style="width:200px">
            <button class="btn btn-warn" @click="clearTenant" :disabled="!clearTenantId.trim()">清除指定租户</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 策略配置 -->
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-panel">
        <h3 class="section-title">缓存策略配置</h3>
        <div class="config-grid">
          <div class="config-field">
            <label class="field-label">TTL (秒)</label>
            <input v-model.number="config.ttl_seconds" class="field-input" type="number" placeholder="3600">
            <span class="field-hint">缓存条目过期时间，0 表示不过期</span>
          </div>
          <div class="config-field">
            <label class="field-label">最大缓存大小 (条目数)</label>
            <input v-model.number="config.max_entries" class="field-input" type="number" placeholder="10000">
            <span class="field-hint">达到上限后按策略淘汰旧条目</span>
          </div>
          <div class="config-field">
            <label class="field-label">缓存策略</label>
            <div class="strategy-group">
              <button v-for="s in strategyOptions" :key="s.value" class="strategy-chip" :class="{ active: config.eviction_policy === s.value }" @click="config.eviction_policy = s.value">
                {{ s.label }}
              </button>
            </div>
            <span class="field-hint">LRU: 最近最少使用 / LFU: 最不经常使用</span>
          </div>
        </div>
        <div class="slider-group">
          <div class="slider-header">
            <span class="slider-label">相似度阈值</span>
            <span class="slider-value">{{ config.similarity_threshold }}</span>
          </div>
          <input type="range" class="slider" min="0" max="1" step="0.05" v-model.number="config.similarity_threshold">
        </div>
        <div class="config-toggles">
          <label class="toggle-row"><input type="checkbox" v-model="config.tenant_isolation"><span>租户隔离</span></label>
          <label class="toggle-row"><input type="checkbox" v-model="config.skip_tainted"><span>跳过污染数据</span></label>
        </div>
        <button class="btn btn-primary" @click="saveConfig" :disabled="saving" style="margin-top:var(--space-3)">
          <template v-if="!saving"><Icon name="save" :size="14" /> 保存配置</template><template v-else>保存中...</template>
        </button>
      </div>
    </div>

    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>

    <!-- Confirm Modals -->
    <ConfirmModal :visible="showClearAllConfirm" type="danger" title="清空全部缓存"
      message="此操作将清除全部缓存条目，不可恢复。确认继续？"
      confirmText="确认清空" @confirm="doClearAll" @cancel="showClearAllConfirm = false" />
    <ConfirmModal :visible="showBatchClearConfirm" type="warning" :title="'批量删除 ' + selectedKeys.size + ' 条缓存'"
      :message="'将删除选中的 ' + selectedKeys.size + ' 条缓存条目，此操作不可恢复。'"
      confirmText="确认删除" @confirm="doBatchClear" @cancel="showBatchClearConfirm = false" />
  </div>
</template>
<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import Icon from '../components/Icon.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'
import StatCard from '../components/StatCard.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

/* SVG Icons for StatCard */
const svgEntries = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>'
const svgHit = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg>'
const svgSize = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>'
const svgSave = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 6v12"/><path d="M15.5 9h-5a2.5 2.5 0 0 0 0 5h3a2.5 2.5 0 0 1 0 5h-5"/></svg>'

const activeTab = ref('entries')
const stats = ref({})
const entries = ref([])
const error = ref('')
const searchQuery = ref('')
const selectedKeys = ref(new Set())

/* Auto Refresh */
const autoRefresh = ref(localStorage.getItem('cache_auto_refresh') === '1')
let refreshTimer = null
function onAutoRefreshChange() {
  localStorage.setItem('cache_auto_refresh', autoRefresh.value ? '1' : '0')
  setupTimer()
}
function setupTimer() {
  clearInterval(refreshTimer)
  if (autoRefresh.value) refreshTimer = setInterval(loadAll, 30000)
}

/* Strategy options */
const strategyOptions = [
  { label: 'LRU (最近最少使用)', value: 'lru' },
  { label: 'LFU (最不经常使用)', value: 'lfu' },
]

/* Lookup */
const lookupQuery = ref('')
const lookupTenant = ref('')
const looking = ref(false)
const lookupResult = ref(null)

/* Manage */
const clearTenantId = ref('')
const showClearAllConfirm = ref(false)
const showBatchClearConfirm = ref(false)

/* Config */
const saving = ref(false)
const config = reactive({
  max_entries: 10000,
  ttl_seconds: 3600,
  similarity_threshold: 0.85,
  tenant_isolation: true,
  skip_tainted: true,
  eviction_policy: 'lru',
})

/* Computed */
const cacheSizeDisplay = computed(() => {
  const bytes = stats.value.size_bytes || stats.value.cache_size || 0
  if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(1) + ' GB'
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + ' MB'
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes > 0) return bytes + ' B'
  return (stats.value.entries || 0) + ' 条'
})

const filteredEntries = computed(() => {
  if (!searchQuery.value.trim()) return entries.value
  const q = searchQuery.value.toLowerCase()
  return entries.value.filter(e => {
    const key = (e.query || e.key || '').toLowerCase()
    const model = (e.model || '').toLowerCase()
    return key.includes(q) || model.includes(q)
  })
})

const isAllSelected = computed(() => filteredEntries.value.length > 0 && filteredEntries.value.every(e => selectedKeys.value.has(entryKey(e))))
const isPartialSelected = computed(() => {
  const s = selectedKeys.value.size
  return s > 0 && s < filteredEntries.value.length
})

/* Helpers */
function formatNum(n) { if (n >= 1000000) return (n/1000000).toFixed(1)+'M'; if (n >= 1000) return (n/1000).toFixed(1)+'K'; return String(n) }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function formatTime(ts) {
  if (!ts) return '-'
  try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month:'2-digit', day:'2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour:'2-digit', minute:'2-digit' }) } catch { return ts }
}
function entryKey(e) { return e.id || e.key || e.query || JSON.stringify(e) }
function onSearch() { /* reactive via computed */ }

/* Selection */
function toggleSelect(e) {
  const key = entryKey(e)
  const next = new Set(selectedKeys.value)
  if (next.has(key)) next.delete(key); else next.add(key)
  selectedKeys.value = next
}
function toggleSelectAll() {
  if (isAllSelected.value) {
    selectedKeys.value = new Set()
  } else {
    selectedKeys.value = new Set(filteredEntries.value.map(e => entryKey(e)))
  }
}

/* Data loading */
async function loadStats() {
  try {
    const d = await api('/api/v1/cache/config')
    stats.value = { entries: d.entries ?? d.entry_count, hit_rate: d.hit_rate, tokens_saved: d.tokens_saved, cost_saved: d.cost_saved, size_bytes: d.size_bytes, cache_size: d.cache_size }
    if (d.max_entries != null) config.max_entries = d.max_entries
    if (d.ttl_seconds != null) config.ttl_seconds = d.ttl_seconds
    if (d.similarity_threshold != null) config.similarity_threshold = d.similarity_threshold
    if (d.tenant_isolation != null) config.tenant_isolation = d.tenant_isolation
    if (d.skip_tainted != null) config.skip_tainted = d.skip_tainted
    if (d.eviction_policy) config.eviction_policy = d.eviction_policy
  } catch (e) { error.value = '加载统计失败: ' + e.message }
}
async function loadEntries() {
  try { const d = await api('/api/v1/cache/entries?limit=100'); entries.value = d.entries || d || [] } catch (e) { error.value = '加载缓存条目失败: ' + e.message }
}
function loadAll() { error.value = ''; selectedKeys.value = new Set(); loadStats(); loadEntries() }

/* Lookup */
async function lookupCache() {
  looking.value = true; lookupResult.value = null
  try {
    const body = { query: lookupQuery.value }
    if (lookupTenant.value.trim()) body.tenant_id = lookupTenant.value.trim()
    lookupResult.value = await apiPost('/api/v1/cache/lookup', body)
    showToast(lookupResult.value.hit ? '🎯 命中缓存' : '❌ 未命中', lookupResult.value.hit ? 'success' : '')
  } catch (e) { error.value = '查询失败: ' + e.message; showToast('查询失败', 'error') }
  finally { looking.value = false }
}

/* Clear All */
async function doClearAll() {
  showClearAllConfirm.value = false
  try { await apiDelete('/api/v1/cache/entries'); showToast('全部缓存已清除', 'success'); loadAll() }
  catch (e) { showToast('清除失败: ' + e.message, 'error') }
}

/* Batch Clear */
async function doBatchClear() {
  showBatchClearConfirm.value = false
  const keys = Array.from(selectedKeys.value)
  try {
    await apiPost('/api/v1/cache/entries/batch-delete', { keys })
    showToast('已删除 ' + keys.length + ' 条缓存', 'success')
    selectedKeys.value = new Set()
    loadAll()
  } catch (e) {
    // Fallback: try individual deletes
    let ok = 0
    for (const k of keys) {
      try { await apiDelete('/api/v1/cache/entries/' + encodeURIComponent(k)); ok++ } catch { /* ignore */ }
    }
    showToast('已删除 ' + ok + '/' + keys.length + ' 条', ok > 0 ? 'success' : 'error')
    selectedKeys.value = new Set()
    loadAll()
  }
}

/* Clear Tenant */
async function clearTenant() {
  if (!confirm('确定清除租户 "' + clearTenantId.value + '" 的缓存吗？')) return
  try { await apiDelete('/api/v1/cache/tenant/' + encodeURIComponent(clearTenantId.value)); showToast('租户缓存已清除', 'success'); loadAll() }
  catch (e) { showToast('清除失败: ' + e.message, 'error') }
}

/* Save Config */
async function saveConfig() {
  saving.value = true
  try {
    await apiPut('/api/v1/cache/config', {
      max_entries: config.max_entries, ttl_seconds: config.ttl_seconds,
      similarity_threshold: config.similarity_threshold, tenant_isolation: config.tenant_isolation,
      skip_tainted: config.skip_tainted, eviction_policy: config.eviction_policy,
    })
    showToast('配置已保存', 'success'); loadStats()
  } catch (e) { showToast('保存失败: ' + e.message, 'error') }
  finally { saving.value = false }
}

onMounted(() => { loadAll(); setupTimer() })
onUnmounted(() => clearInterval(refreshTimer))
</script>
<style scoped>
.cache-page { padding:var(--space-4); max-width:1200px; }
.page-header { display:flex; align-items:center; justify-content:space-between; margin-bottom:var(--space-4); flex-wrap:wrap; gap:var(--space-3); }
.page-title { font-size:var(--text-xl); font-weight:800; color:var(--text-primary); margin:0; }
.page-subtitle { font-size:var(--text-sm); color:var(--text-tertiary); margin-top:2px; }
.header-actions { display:flex; align-items:center; gap:var(--space-2); }

/* Auto Refresh Toggle */
.auto-refresh-toggle {
  display:inline-flex; align-items:center; gap:4px; padding:4px 10px; border-radius:var(--radius-md);
  font-size:var(--text-xs); font-weight:600; cursor:pointer; border:1px solid var(--border-subtle);
  background:var(--bg-elevated); color:var(--text-tertiary); transition:all .2s; user-select:none;
}
.auto-refresh-toggle input { display:none; }
.auto-refresh-toggle.on { border-color:var(--color-primary); color:var(--color-primary); background:rgba(99,102,241,.08); }
.auto-refresh-toggle.on svg { animation:spin-slow 2s linear infinite; }
@keyframes spin-slow { to { transform:rotate(360deg); } }

/* Stats Grid - now uses StatCard */
.stats-grid { display:grid; grid-template-columns:repeat(4,1fr); gap:var(--space-3); margin-bottom:var(--space-4); }
.stat-clickable { cursor:pointer !important; }
.stat-clickable:hover { transform:translateY(-3px) !important; box-shadow:var(--shadow-lg) !important; border-color:var(--color-primary) !important; }

/* Tab Bar */
.tab-bar { display:flex; gap:var(--space-2); margin-bottom:var(--space-3); border-bottom:1px solid var(--border-subtle); padding-bottom:var(--space-2); }
.tab-btn { background:none; border:none; color:var(--text-secondary); font-size:var(--text-sm); padding:var(--space-2) var(--space-3); cursor:pointer; border-radius:var(--radius-md) var(--radius-md) 0 0; transition:all .2s; }
.tab-btn:hover { color:var(--text-primary); background:var(--bg-elevated); }
.tab-btn.active { color:var(--color-primary); border-bottom:2px solid var(--color-primary); font-weight:600; }

.section { margin-bottom:var(--space-4); }
.section-title { font-size:var(--text-sm); font-weight:700; color:var(--text-primary); margin-bottom:var(--space-3); }

/* Search Bar */
.search-bar { display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); margin-bottom:var(--space-3); flex-wrap:wrap; }
.search-input-wrap { position:relative; flex:1; min-width:200px; max-width:400px; }
.search-icon { position:absolute; left:10px; top:50%; transform:translateY(-50%); color:var(--text-tertiary); pointer-events:none; }
.search-input {
  width:100%; padding:8px 32px 8px 32px; background:var(--bg-elevated); border:1px solid var(--border-subtle);
  border-radius:var(--radius-md); color:var(--text-primary); font-size:var(--text-sm); transition:border-color .2s;
}
.search-input:focus { outline:none; border-color:var(--color-primary); }
.search-input::placeholder { color:var(--text-disabled); }
.search-clear { position:absolute; right:8px; top:50%; transform:translateY(-50%); background:none; border:none; color:var(--text-tertiary); cursor:pointer; font-size:14px; padding:2px; }
.search-clear:hover { color:var(--text-primary); }
.search-meta { display:flex; align-items:center; gap:var(--space-2); }
.selected-count { font-size:var(--text-xs); color:var(--color-primary); font-weight:600; padding:4px 10px; background:rgba(99,102,241,.1); border-radius:var(--radius-sm); }

/* Table */
.table-wrap { overflow-x:auto; }
.data-table { width:100%; border-collapse:collapse; font-size:var(--text-xs); }
.data-table th { text-align:left; padding:8px 10px; background:var(--bg-elevated); color:var(--text-tertiary); font-weight:600; font-size:10px; text-transform:uppercase; letter-spacing:.05em; border-bottom:2px solid var(--border-subtle); white-space:nowrap; }
.data-table td { padding:6px 10px; border-bottom:1px solid var(--border-subtle); color:var(--text-secondary); }
.data-table tr:hover { background:var(--bg-elevated); }
.data-table input[type="checkbox"] { accent-color:var(--color-primary); cursor:pointer; }
.td-mono { font-family:var(--font-mono); font-size:11px; }
.td-payload { max-width:300px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.row-selected { background:rgba(99,102,241,.06) !important; }
.hit-count-badge { display:inline-block; padding:1px 8px; border-radius:9999px; font-size:10px; font-weight:600; background:rgba(16,185,129,.12); color:#10B981; }

/* Test Panel */
.test-panel { background:var(--bg-surface); border:1px solid var(--border-subtle); border-radius:var(--radius-lg); padding:var(--space-4); margin-bottom:var(--space-3); }
.test-row { display:flex; gap:var(--space-3); flex-wrap:wrap; }
.test-field { flex:1; min-width:200px; }
.test-input {
  width:100%; background:var(--bg-elevated); border:1px solid var(--border-subtle); border-radius:var(--radius-md);
  color:var(--text-primary); padding:var(--space-3); font-size:var(--text-sm); resize:vertical; font-family:var(--font-mono);
}
.test-input:focus { outline:none; border-color:var(--color-primary); }

/* Lookup Result */
.lookup-result { background:var(--bg-surface); border:1px solid var(--border-subtle); border-radius:var(--radius-lg); padding:var(--space-4); }
.result-header { display:flex; align-items:center; justify-content:space-between; margin-bottom:var(--space-3); font-weight:700; color:var(--text-primary); }
.btn-close { background:none; border:none; color:var(--text-tertiary); cursor:pointer; font-size:16px; }
.btn-close:hover { color:var(--text-primary); }
.hit-status { font-size:1.25rem; font-weight:700; text-align:center; padding:var(--space-3); border-radius:var(--radius-md); margin-bottom:var(--space-3); }
.hit-yes { color:#10B981; background:rgba(16,185,129,.1); }
.hit-no { color:#F59E0B; background:rgba(245,158,11,.1); }
.cached-response { margin-top:var(--space-2); }
.response-pre { background:var(--bg-elevated); border:1px solid var(--border-subtle); border-radius:var(--radius-md); padding:var(--space-3); font-size:var(--text-xs); font-family:var(--font-mono); color:var(--text-secondary); overflow-x:auto; white-space:pre-wrap; word-wrap:break-word; max-height:200px; overflow-y:auto; }

/* Manage */
.manage-panel { background:var(--bg-surface); border:1px solid var(--border-subtle); border-radius:var(--radius-lg); padding:var(--space-4); }
.manage-actions { display:flex; flex-direction:column; gap:var(--space-3); }
.tenant-clear { display:flex; gap:var(--space-2); align-items:center; flex-wrap:wrap; }

/* Config */
.config-panel { background:var(--bg-surface); border:1px solid var(--border-subtle); border-radius:var(--radius-lg); padding:var(--space-4); max-width:560px; }
.config-grid { display:flex; flex-direction:column; gap:var(--space-3); }
.config-field { display:flex; flex-direction:column; gap:4px; }
.field-label { font-size:10px; font-weight:600; color:var(--text-tertiary); text-transform:uppercase; letter-spacing:.05em; }
.field-hint { font-size:10px; color:var(--text-disabled); margin-top:2px; }
.field-input { background:var(--bg-elevated); border:1px solid var(--border-subtle); border-radius:var(--radius-md); color:var(--text-primary); padding:6px 10px; font-size:var(--text-sm); }
.field-input:focus { outline:none; border-color:var(--color-primary); }

/* Strategy selector */
.strategy-group { display:flex; gap:4px; }
.strategy-chip {
  padding:6px 14px; border:1px solid var(--border-subtle); border-radius:var(--radius-md); background:var(--bg-elevated);
  color:var(--text-secondary); font-size:var(--text-xs); font-weight:600; cursor:pointer; transition:all .2s;
}
.strategy-chip.active { background:var(--color-primary); color:#fff; border-color:var(--color-primary); }
.strategy-chip:hover:not(.active) { border-color:var(--color-primary); color:var(--color-primary); }

.config-toggles { display:flex; flex-direction:column; gap:var(--space-2); margin-top:var(--space-3); }
.toggle-row { display:flex; align-items:center; gap:8px; font-size:var(--text-sm); color:var(--text-secondary); cursor:pointer; }
.toggle-row input { accent-color:var(--color-primary); }

/* Slider */
.slider-group { margin:var(--space-3) 0; }
.slider-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:var(--space-1); }
.slider-label { font-size:var(--text-sm); color:var(--text-secondary); font-weight:600; }
.slider-value { font-size:var(--text-base); font-weight:800; color:var(--color-primary); font-family:var(--font-mono); }
.slider { -webkit-appearance:none; appearance:none; width:100%; height:6px; background:rgba(255,255,255,0.1); border-radius:3px; outline:none; }
.slider::-webkit-slider-thumb { -webkit-appearance:none; appearance:none; width:18px; height:18px; border-radius:50%; background:var(--color-primary); cursor:pointer; border:2px solid #fff; }
.slider::-moz-range-thumb { width:18px; height:18px; border-radius:50%; background:var(--color-primary); cursor:pointer; border:2px solid #fff; }

/* Buttons */
.btn { display:inline-flex; align-items:center; gap:6px; padding:8px 16px; border-radius:var(--radius-md); font-weight:600; font-size:var(--text-sm); cursor:pointer; border:1px solid var(--border-subtle); background:var(--bg-elevated); color:var(--text-secondary); transition:all .2s; }
.btn:hover { background:var(--bg-surface); color:var(--text-primary); }
.btn-primary { background:var(--color-primary); color:#fff; border-color:var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter:brightness(1.15); }
.btn-primary:disabled { opacity:.5; cursor:not-allowed; }
.btn-sm { padding:6px 12px; font-size:var(--text-xs); }
.btn-danger { background:rgba(239,68,68,.15); color:#FCA5A5; border-color:rgba(239,68,68,.3); }
.btn-danger:hover { background:rgba(239,68,68,.25); color:#FCA5A5; }
.btn-warn { background:rgba(245,158,11,.15); color:#FCD34D; border-color:rgba(245,158,11,.3); }
.btn-warn:hover { background:rgba(245,158,11,.25); color:#FCD34D; }
.spinner { display:inline-block; width:14px; height:14px; border:2px solid rgba(255,255,255,.3); border-top-color:#fff; border-radius:50%; animation:spin .6s linear infinite; }
@keyframes spin { to { transform:rotate(360deg); } }
.empty-state { text-align:center; padding:var(--space-6); color:var(--text-tertiary); }
.error-banner { margin-top:var(--space-3); padding:var(--space-3); background:rgba(239,68,68,.1); border:1px solid rgba(239,68,68,.3); border-radius:var(--radius-md); color:#FCA5A5; font-size:var(--text-sm); }

@media (max-width:768px) {
  .stats-grid { grid-template-columns:repeat(2,1fr); }
  .search-bar { flex-direction:column; align-items:stretch; }
  .search-input-wrap { max-width:none; }
}
</style>
