<template>
  <div class="cache-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="save" :size="20" /> 响应缓存</h1>
        <p class="page-subtitle">LLM 响应智能缓存 — 节省 Token 开销、降低延迟、支持租户隔离</p>
      </div>
      <button class="btn btn-sm" @click="loadAll">🔄 刷新</button>
    </div>

    <!-- 顶部大数字 -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon">📦</div>
        <div class="stat-value">{{ stats.entries ?? '-' }}</div>
        <div class="stat-label">缓存条目</div>
      </div>
      <div class="stat-card stat-hit">
        <div class="stat-icon"><Icon name="crosshair" :size="18" /></div>
        <div class="stat-value">{{ stats.hit_rate != null ? (stats.hit_rate * 100).toFixed(1) + '%' : '-' }}</div>
        <div class="stat-label">命中率</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🪙</div>
        <div class="stat-value">{{ stats.tokens_saved != null ? formatNum(stats.tokens_saved) : '-' }}</div>
        <div class="stat-label">节省 Token</div>
      </div>
      <div class="stat-card stat-cost">
        <div class="stat-icon">💰</div>
        <div class="stat-value">{{ stats.cost_saved != null ? '$' + stats.cost_saved.toFixed(2) : '-' }}</div>
        <div class="stat-label">节省成本</div>
      </div>
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'entries' }" @click="activeTab = 'entries'">📦 缓存条目 ({{ entries.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'lookup' }" @click="activeTab = 'lookup'"><Icon name="search" :size="14" /> 测试查询</button>
      <button class="tab-btn" :class="{ active: activeTab === 'manage' }" @click="activeTab = 'manage'">🗑️ 管理</button>
      <button class="tab-btn" :class="{ active: activeTab === 'config' }" @click="activeTab = 'config'"><Icon name="settings" :size="14" /> 配置</button>
    </div>

    <!-- 缓存条目表格 -->
    <div v-if="activeTab === 'entries'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>查询</th>
              <th>模型</th>
              <th>租户</th>
              <th>命中次数</th>
              <th>节省 Token</th>
              <th>创建时间</th>
              <th>最近命中</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(e, idx) in entries" :key="idx">
              <td class="td-payload">{{ truncate(e.query, 50) }}</td>
              <td class="td-mono">{{ e.model || '-' }}</td>
              <td class="td-mono">{{ e.tenant || e.tenant_id || '-' }}</td>
              <td class="td-mono">{{ e.hit_count ?? 0 }}</td>
              <td class="td-mono">{{ formatNum(e.tokens_saved ?? 0) }}</td>
              <td class="td-mono">{{ formatTime(e.created_at || e.created) }}</td>
              <td class="td-mono">{{ formatTime(e.last_hit || e.last_accessed) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="entries.length === 0" class="empty-state">暂无缓存条目</div>
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
        <div class="test-row" style="margin-top: var(--space-2)">
          <div class="test-field" style="max-width: 200px">
            <label class="field-label">租户 ID</label>
            <input v-model="lookupTenant" class="field-input" placeholder="可选">
          </div>
        </div>
        <button class="btn btn-primary" @click="lookupCache" :disabled="looking || !lookupQuery.trim()" style="margin-top: var(--space-2)">
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
          <button class="btn btn-danger" @click="clearAll">🗑️ 清除全部缓存</button>
          <div class="tenant-clear">
            <input v-model="clearTenantId" class="field-input" placeholder="输入租户 ID" style="width: 200px">
            <button class="btn btn-warn" @click="clearTenant" :disabled="!clearTenantId.trim()">清除指定租户</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 配置区 -->
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-panel">
        <h3 class="section-title">缓存配置</h3>
        <div class="config-field">
          <label class="field-label">最大条目数</label>
          <input v-model.number="config.max_entries" class="field-input" type="number" placeholder="10000">
        </div>
        <div class="config-field">
          <label class="field-label">TTL (秒)</label>
          <input v-model.number="config.ttl_seconds" class="field-input" type="number" placeholder="3600">
        </div>
        <div class="slider-group">
          <div class="slider-header">
            <span class="slider-label">相似度阈值</span>
            <span class="slider-value">{{ config.similarity_threshold }}</span>
          </div>
          <input type="range" class="slider" min="0" max="1" step="0.05" v-model.number="config.similarity_threshold">
        </div>
        <div class="config-field">
          <label class="toggle-row">
            <input type="checkbox" v-model="config.tenant_isolation">
            <span>租户隔离</span>
          </label>
        </div>
        <div class="config-field">
          <label class="toggle-row">
            <input type="checkbox" v-model="config.skip_tainted">
            <span>跳过污染数据</span>
          </label>
        </div>
        <button class="btn btn-primary" @click="saveConfig" :disabled="saving" style="margin-top: var(--space-3)">
          <template v-if="!saving"><Icon name="save" :size="14" /> 保存配置</template><template v-else>保存中...</template>
        </button>
        <div v-if="saveMsg" class="save-msg" :class="saveMsgType">{{ saveMsg }}</div>
      </div>
    </div>

    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'

const activeTab = ref('entries')
const stats = ref({})
const entries = ref([])
const error = ref('')
const lookupQuery = ref('')
const lookupTenant = ref('')
const looking = ref(false)
const lookupResult = ref(null)
const clearTenantId = ref('')
const saving = ref(false)
const saveMsg = ref('')
const saveMsgType = ref('')

const config = reactive({ max_entries: 10000, ttl_seconds: 3600, similarity_threshold: 0.85, tenant_isolation: true, skip_tainted: true })

async function loadStats() {
  try {
    const d = await api('/api/v1/cache/config')
    stats.value = { entries: d.entries ?? d.entry_count, hit_rate: d.hit_rate, tokens_saved: d.tokens_saved, cost_saved: d.cost_saved }
    if (d.max_entries != null) config.max_entries = d.max_entries
    if (d.ttl_seconds != null) config.ttl_seconds = d.ttl_seconds
    if (d.similarity_threshold != null) config.similarity_threshold = d.similarity_threshold
    if (d.tenant_isolation != null) config.tenant_isolation = d.tenant_isolation
    if (d.skip_tainted != null) config.skip_tainted = d.skip_tainted
  } catch (e) { error.value = '加载统计失败: ' + e.message }
}

async function loadEntries() {
  try { const d = await api('/api/v1/cache/entries?limit=50'); entries.value = d.entries || d || [] } catch (e) { error.value = '加载缓存条目失败: ' + e.message }
}

async function lookupCache() {
  looking.value = true; lookupResult.value = null
  try {
    const body = { query: lookupQuery.value }
    if (lookupTenant.value.trim()) body.tenant_id = lookupTenant.value.trim()
    lookupResult.value = await apiPost('/api/v1/cache/lookup', body)
  } catch (e) { error.value = '查询失败: ' + e.message }
  finally { looking.value = false }
}

async function clearAll() {
  if (!confirm('确定清除全部缓存吗？此操作不可恢复！')) return
  try { await apiDelete('/api/v1/cache/entries'); loadStats(); loadEntries() } catch (e) { error.value = '清除失败: ' + e.message }
}

async function clearTenant() {
  if (!confirm('确定清除租户 "' + clearTenantId.value + '" 的缓存吗？')) return
  try { await apiDelete('/api/v1/cache/tenant/' + encodeURIComponent(clearTenantId.value)); loadStats(); loadEntries() } catch (e) { error.value = '清除失败: ' + e.message }
}

async function saveConfig() {
  saving.value = true; saveMsg.value = ''
  try {
    await apiPut('/api/v1/cache/config', { max_entries: config.max_entries, ttl_seconds: config.ttl_seconds, similarity_threshold: config.similarity_threshold, tenant_isolation: config.tenant_isolation, skip_tainted: config.skip_tainted })
    saveMsg.value = '✅ 配置已保存'; saveMsgType.value = 'success'; loadStats()
  } catch (e) { saveMsg.value = '❌ 保存失败: ' + e.message; saveMsgType.value = 'error' }
  finally { saving.value = false }
}

function formatNum(n) {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

function loadAll() { error.value = ''; loadStats(); loadEntries() }
function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function formatTime(ts) {
  if (!ts) return '-'
  try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) } catch { return ts }
}
onMounted(loadAll)
</script>

<style scoped>
.cache-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); text-align: center; }
.stat-icon { font-size: 1.5rem; margin-bottom: var(--space-1); }
.stat-value { font-size: 1.75rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }
.stat-hit .stat-value { color: #10B981; }
.stat-cost .stat-value { color: #10B981; }

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
.td-payload { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* Test Panel */
.test-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); margin-bottom: var(--space-3); }
.test-row { display: flex; gap: var(--space-3); flex-wrap: wrap; }
.test-field { flex: 1; min-width: 200px; }
.test-input {
  width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md);
  color: var(--text-primary); padding: var(--space-3); font-size: var(--text-sm); resize: vertical; font-family: var(--font-mono);
}
.test-input:focus { outline: none; border-color: var(--color-primary); }

/* Lookup Result */
.lookup-result { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.result-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-3); font-weight: 700; color: var(--text-primary); }
.btn-close { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 16px; }
.btn-close:hover { color: var(--text-primary); }
.hit-status { font-size: 1.25rem; font-weight: 700; text-align: center; padding: var(--space-3); border-radius: var(--radius-md); margin-bottom: var(--space-3); }
.hit-yes { color: #10B981; background: rgba(16,185,129,.1); }
.hit-no { color: #F59E0B; background: rgba(245,158,11,.1); }
.cached-response { margin-top: var(--space-2); }
.response-pre { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-3); font-size: var(--text-xs); font-family: var(--font-mono); color: var(--text-secondary); overflow-x: auto; white-space: pre-wrap; word-wrap: break-word; max-height: 200px; overflow-y: auto; }

/* Manage */
.manage-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.manage-actions { display: flex; flex-direction: column; gap: var(--space-3); }
.tenant-clear { display: flex; gap: var(--space-2); align-items: center; flex-wrap: wrap; }

/* Config */
.config-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); max-width: 480px; }
.config-field { margin-bottom: var(--space-3); display: flex; flex-direction: column; gap: 4px; }
.field-label { font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; }
.field-input { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); }
.field-input:focus { outline: none; border-color: var(--color-primary); }
.toggle-row { display: flex; align-items: center; gap: 8px; font-size: var(--text-sm); color: var(--text-secondary); cursor: pointer; }
.toggle-row input { accent-color: var(--color-primary); }

.slider-group { margin-bottom: var(--space-4); }
.slider-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-1); }
.slider-label { font-size: var(--text-sm); color: var(--text-secondary); font-weight: 600; }
.slider-value { font-size: var(--text-base); font-weight: 800; color: var(--color-primary); font-family: var(--font-mono); }
.slider { -webkit-appearance: none; appearance: none; width: 100%; height: 6px; background: rgba(255,255,255,0.1); border-radius: 3px; outline: none; }
.slider::-webkit-slider-thumb { -webkit-appearance: none; appearance: none; width: 18px; height: 18px; border-radius: 50%; background: var(--color-primary); cursor: pointer; border: 2px solid #fff; }
.slider::-moz-range-thumb { width: 18px; height: 18px; border-radius: 50%; background: var(--color-primary); cursor: pointer; border: 2px solid #fff; }

.save-msg { margin-top: var(--space-2); font-size: var(--text-sm); font-weight: 600; }
.save-msg.success { color: #10B981; }
.save-msg.error { color: #EF4444; }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-sm { padding: 6px 12px; font-size: var(--text-xs); }
.btn-danger { background: rgba(239,68,68,.15); color: #FCA5A5; border-color: rgba(239,68,68,.3); }
.btn-danger:hover { background: rgba(239,68,68,.25); color: #FCA5A5; }
.btn-warn { background: rgba(245,158,11,.15); color: #FCD34D; border-color: rgba(245,158,11,.3); }
.btn-warn:hover { background: rgba(245,158,11,.25); color: #FCD34D; }
.spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.3); border-top-color: #fff; border-radius: 50%; animation: spin .6s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.empty-state { text-align: center; padding: var(--space-6); color: var(--text-tertiary); }
.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); }

@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
}
</style>
