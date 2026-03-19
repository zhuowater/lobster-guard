<template>
  <div class="envelopes-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">🔐 执行信封 + Merkle Tree</h1>
        <p class="page-subtitle">不可篡改的决策信封 — 每条拦截决策都有密封凭证</p>
      </div>
      <button class="btn btn-sm" @click="loadAll">🔄 刷新</button>
    </div>

    <!-- StatCards -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon">📨</div>
        <div class="stat-value">{{ stats.total_envelopes ?? '-' }}</div>
        <div class="stat-label">总信封数</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🌲</div>
        <div class="stat-value">{{ stats.total_batches ?? '-' }}</div>
        <div class="stat-label">Merkle 批次</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">🍃</div>
        <div class="stat-value">{{ stats.pending_leaves ?? '-' }}</div>
        <div class="stat-label">待处理叶子</div>
      </div>
      <div class="stat-card">
        <div class="stat-icon">📦</div>
        <div class="stat-value">{{ stats.batch_size ?? '-' }}</div>
        <div class="stat-label">批次大小</div>
      </div>
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'envelopes' }" @click="activeTab = 'envelopes'">📨 信封列表 ({{ envelopes.length }})</button>
      <button class="tab-btn" :class="{ active: activeTab === 'batches' }" @click="activeTab = 'batches'">🌲 Merkle 批次 ({{ batches.length }})</button>
    </div>

    <!-- 信封列表 -->
    <div v-if="activeTab === 'envelopes'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>域</th>
              <th>决策</th>
              <th>TraceID</th>
              <th>时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in envelopes" :key="e.id">
              <td class="td-mono">{{ truncate(e.id, 12) }}</td>
              <td>{{ e.domain || '-' }}</td>
              <td>
                <span class="action-badge" :class="'act-' + e.decision">{{ e.decision }}</span>
              </td>
              <td class="td-mono">{{ truncate(e.trace_id, 12) }}</td>
              <td class="td-mono">{{ formatTime(e.created_at || e.timestamp) }}</td>
              <td>
                <button class="btn-sm" @click="verifyEnvelope(e.id)" :disabled="verifying[e.id]">
                  {{ verifying[e.id] ? '验证中...' : '🔍 验证' }}
                </button>
                <span v-if="verifyResults[e.id] !== undefined" class="verify-result" :class="verifyResults[e.id] ? 'valid' : 'invalid'">
                  {{ verifyResults[e.id] ? '✅ 有效' : '❌ 无效' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="envelopes.length === 0" class="empty-state">暂无信封记录</div>
      </div>
    </div>

    <!-- Merkle 批次列表 -->
    <div v-if="activeTab === 'batches'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Merkle Root</th>
              <th>叶子数</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="b in batches" :key="b.id">
              <td class="td-mono">{{ truncate(b.id, 12) }}</td>
              <td class="td-mono">{{ truncate(b.root || b.merkle_root, 16) }}</td>
              <td>{{ b.leaf_count ?? b.leaves?.length ?? '-' }}</td>
              <td class="td-mono">{{ formatTime(b.created_at || b.timestamp) }}</td>
              <td>
                <button class="btn-sm" @click="verifyBatch(b.id)" :disabled="batchVerifying[b.id]">
                  {{ batchVerifying[b.id] ? '验证中...' : '🌲 验证批次' }}
                </button>
                <span v-if="batchVerifyResults[b.id] !== undefined" class="verify-result" :class="batchVerifyResults[b.id] ? 'valid' : 'invalid'">
                  {{ batchVerifyResults[b.id] ? '✅ 完整' : '❌ 损坏' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="batches.length === 0" class="empty-state">暂无 Merkle 批次</div>
      </div>
    </div>

    <!-- 错误提示 -->
    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { api } from '../api.js'

const activeTab = ref('envelopes')
const stats = ref({})
const envelopes = ref([])
const batches = ref([])
const error = ref('')
const verifying = reactive({})
const verifyResults = reactive({})
const batchVerifying = reactive({})
const batchVerifyResults = reactive({})

async function loadStats() {
  try { stats.value = await api('/api/v1/envelopes/stats') } catch (e) { error.value = '加载统计失败: ' + e.message }
}

async function loadEnvelopes() {
  try { const d = await api('/api/v1/envelopes/list'); envelopes.value = d.envelopes || d || [] } catch (e) { error.value = '加载信封列表失败: ' + e.message }
}

async function loadBatches() {
  try { const d = await api('/api/v1/envelopes/batches'); batches.value = d.batches || d || [] } catch (e) { error.value = '加载批次失败: ' + e.message }
}

async function verifyEnvelope(id) {
  verifying[id] = true
  try {
    const d = await api('/api/v1/envelopes/verify/' + id)
    verifyResults[id] = d.valid !== false
  } catch {
    verifyResults[id] = false
  } finally {
    verifying[id] = false
  }
}

async function verifyBatch(id) {
  batchVerifying[id] = true
  try {
    const d = await api('/api/v1/envelopes/batch/' + id)
    batchVerifyResults[id] = d.valid !== false
  } catch {
    batchVerifyResults[id] = false
  } finally {
    batchVerifying[id] = false
  }
}

function loadAll() {
  error.value = ''
  loadStats()
  loadEnvelopes()
  loadBatches()
}

function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }
function formatTime(ts) {
  if (!ts) return '-'
  try { const d = new Date(ts); return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) } catch { return ts }
}

onMounted(loadAll)
</script>

<style scoped>
.envelopes-page { padding: var(--space-4); max-width: 1200px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

/* Stats */
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); text-align: center; }
.stat-icon { font-size: 1.5rem; margin-bottom: var(--space-1); }
.stat-value { font-size: 1.75rem; font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); }

/* Tabs */
.tab-bar { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }

.section { margin-bottom: var(--space-4); }
.table-wrap { overflow-x: auto; }

/* Data Table */
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th {
  text-align: left; padding: 8px 10px; background: var(--bg-elevated);
  color: var(--text-tertiary); font-weight: 600; font-size: 10px;
  text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle);
  white-space: nowrap;
}
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }

.action-badge { display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 700; }
.act-block { background: #EF4444; color: #fff; }
.act-warn { background: #F59E0B; color: #1a1a2e; }
.act-pass { background: #10B981; color: #fff; }
.act-log { background: #6B7280; color: #fff; }
.act-allow { background: #10B981; color: #fff; }
.act-deny { background: #EF4444; color: #fff; }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-sm { padding: var(--space-1) var(--space-2); font-size: var(--text-xs); }
.btn-sm { padding: 3px 10px; border-radius: var(--radius-sm); font-size: 11px; font-weight: 600; cursor: pointer; border: 1px solid var(--border-subtle); background: transparent; color: var(--text-secondary); transition: all .2s; }
.btn-sm:hover { background: var(--bg-elevated); color: var(--text-primary); }
.btn-sm:disabled { opacity: .5; cursor: not-allowed; }

.verify-result { margin-left: 8px; font-size: 11px; font-weight: 700; }
.verify-result.valid { color: #10B981; }
.verify-result.invalid { color: #EF4444; }

.empty-state { text-align: center; padding: var(--space-6); color: var(--text-tertiary); }

.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); }

@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
}
</style>
