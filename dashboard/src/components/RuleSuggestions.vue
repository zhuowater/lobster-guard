<template>
  <div class="card suggestion-card">
    <div class="card-header">
      <h3>💡 规则建议队列</h3>
      <div class="header-actions">
        <select v-model="statusFilter" class="filter-select" @change="loadSuggestions">
          <option value="">全部</option>
          <option value="pending">待审批</option>
          <option value="accepted">已接受</option>
          <option value="rejected">已拒绝</option>
        </select>
        <button class="btn btn-sm" @click="loadSuggestions">刷新</button>
      </div>
    </div>

    <!-- 统计 -->
    <div class="suggestion-stats">
      <div class="stat-item pending">
        <span class="stat-num">{{ stats.pending }}</span>
        <span class="stat-label">待审批</span>
      </div>
      <div class="stat-item accepted">
        <span class="stat-num">{{ stats.accepted }}</span>
        <span class="stat-label">已接受</span>
      </div>
      <div class="stat-item rejected">
        <span class="stat-num">{{ stats.rejected }}</span>
        <span class="stat-label">已拒绝</span>
      </div>
      <div class="stat-item total">
        <span class="stat-num">{{ stats.total }}</span>
        <span class="stat-label">总计</span>
      </div>
    </div>

    <!-- 列表 -->
    <div v-if="loading" class="skeleton" style="height: 100px"></div>
    <div v-else-if="suggestions.length === 0" class="empty-state">
      <p>暂无规则建议</p>
      <p class="hint">运行红队测试或自进化引擎后，发现的绕过将生成规则建议</p>
    </div>
    <div v-else class="suggestion-list">
      <div v-for="s in suggestions" :key="s.id" class="suggestion-item" :class="s.status">
        <div class="suggestion-header">
          <span class="suggestion-name">{{ s.rule_name }}</span>
          <span class="tag" :class="'tag-' + s.rule_type">{{ s.rule_type }}</span>
          <span class="tag" :class="'tag-' + s.action">{{ s.action }}</span>
          <span class="tag tag-source">{{ s.source }}</span>
          <span class="status-badge" :class="s.status">{{ statusLabel(s.status) }}</span>
        </div>
        <div class="suggestion-reason">{{ s.reason }}</div>
        <div class="suggestion-patterns">
          <code v-for="(p, i) in s.patterns.slice(0, 5)" :key="i" class="pattern-chip">{{ p }}</code>
          <span v-if="s.patterns.length > 5" class="more">+{{ s.patterns.length - 5 }} 更多</span>
        </div>
        <div class="suggestion-meta">
          <span>{{ formatTime(s.created_at) }}</span>
          <span v-if="s.source_detail">· {{ s.source_detail }}</span>
          <span v-if="s.reviewed_by">· 审批人: {{ s.reviewed_by }}</span>
        </div>
        <div v-if="s.status === 'pending'" class="suggestion-actions">
          <button class="btn btn-sm btn-success" @click="accept(s.id)">✓ 接受</button>
          <button class="btn btn-sm btn-danger" @click="showReject(s)">✗ 拒绝</button>
        </div>
        <div v-if="s.status === 'rejected' && s.reject_reason" class="reject-reason">
          拒绝原因: {{ s.reject_reason }}
        </div>
      </div>
    </div>

    <!-- 拒绝弹窗 -->
    <div v-if="rejectModal" class="modal-overlay" @click.self="rejectModal = false">
      <div class="modal-box">
        <h3>拒绝规则建议</h3>
        <p>{{ rejectTarget?.rule_name }}</p>
        <textarea v-model="rejectReason" placeholder="拒绝原因（可选）" class="input-area"></textarea>
        <div class="modal-actions">
          <button class="btn btn-sm" @click="rejectModal = false">取消</button>
          <button class="btn btn-sm btn-danger" @click="reject">确认拒绝</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api.js'

const suggestions = ref([])
const stats = ref({ pending: 0, accepted: 0, rejected: 0, total: 0 })
const statusFilter = ref('pending')
const loading = ref(false)
const rejectModal = ref(false)
const rejectTarget = ref(null)
const rejectReason = ref('')

async function loadSuggestions() {
  loading.value = true
  try {
    const params = statusFilter.value ? `?status=${statusFilter.value}` : ''
    const data = await api(`/api/v1/suggestions${params}`)
    suggestions.value = data.suggestions || []
    stats.value = data.stats || { pending: 0, accepted: 0, rejected: 0, total: 0 }
  } catch (e) {
    console.error('load suggestions:', e)
  }
  loading.value = false
}

async function accept(id) {
  try {
    await api(`/api/v1/suggestions/${id}/accept`, { method: 'POST' })
    loadSuggestions()
  } catch (e) {
    console.error('accept:', e)
  }
}

function showReject(s) {
  rejectTarget.value = s
  rejectReason.value = ''
  rejectModal.value = true
}

async function reject() {
  if (!rejectTarget.value) return
  try {
    await api(`/api/v1/suggestions/${rejectTarget.value.id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason: rejectReason.value })
    })
    rejectModal.value = false
    loadSuggestions()
  } catch (e) {
    console.error('reject:', e)
  }
}

function statusLabel(s) {
  return { pending: '待审批', accepted: '已接受', rejected: '已拒绝' }[s] || s
}

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

onMounted(loadSuggestions)
</script>

<style scoped>
.suggestion-card { padding: 20px; }
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.card-header h3 { margin: 0; font-size: 1.1rem; }
.header-actions { display: flex; gap: 8px; align-items: center; }
.filter-select {
  background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: 6px;
  padding: 4px 8px; color: var(--text-primary); font-size: .82rem;
}

.suggestion-stats {
  display: flex; gap: 16px; margin-bottom: 16px; padding: 12px;
  background: var(--bg-base); border-radius: var(--radius-md);
}
.stat-item { text-align: center; flex: 1; }
.stat-num { display: block; font-size: 1.4rem; font-weight: 700; }
.stat-label { font-size: .72rem; color: var(--text-tertiary); }
.stat-item.pending .stat-num { color: var(--color-warning); }
.stat-item.accepted .stat-num { color: var(--color-success); }
.stat-item.rejected .stat-num { color: var(--text-tertiary); }
.stat-item.total .stat-num { color: var(--color-primary); }

.suggestion-list { display: flex; flex-direction: column; gap: 10px; }
.suggestion-item {
  padding: 14px; background: var(--bg-base); border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle); transition: border-color .2s;
}
.suggestion-item.pending { border-left: 3px solid var(--color-warning); }
.suggestion-item.accepted { border-left: 3px solid var(--color-success); opacity: .7; }
.suggestion-item.rejected { border-left: 3px solid var(--text-tertiary); opacity: .5; }

.suggestion-header { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.suggestion-name { font-weight: 600; font-size: .9rem; color: var(--text-primary); }
.tag { padding: 2px 8px; border-radius: 4px; font-size: .72rem; }
.tag-keyword { background: rgba(99, 102, 241, .15); color: #818cf8; }
.tag-regex { background: rgba(245, 158, 11, .15); color: #f59e0b; }
.tag-block { background: rgba(239, 68, 68, .15); color: #ef4444; }
.tag-warn { background: rgba(245, 158, 11, .15); color: #f59e0b; }
.tag-review { background: rgba(59, 130, 246, .15); color: #3b82f6; }
.tag-source { background: rgba(99, 102, 241, .1); color: var(--color-primary); }

.status-badge { margin-left: auto; font-size: .72rem; padding: 2px 8px; border-radius: 4px; }
.status-badge.pending { background: rgba(245, 158, 11, .2); color: #f59e0b; }
.status-badge.accepted { background: rgba(16, 185, 129, .2); color: #10b981; }
.status-badge.rejected { background: rgba(107, 114, 128, .2); color: #6b7280; }

.suggestion-reason { margin-top: 8px; font-size: .82rem; color: var(--text-secondary); }
.suggestion-patterns { margin-top: 8px; display: flex; flex-wrap: wrap; gap: 6px; }
.pattern-chip {
  background: rgba(99, 102, 241, .1); color: #a5b4fc; padding: 2px 8px;
  border-radius: 4px; font-size: .75rem; font-family: monospace;
}
.more { font-size: .72rem; color: var(--text-tertiary); align-self: center; }
.suggestion-meta { margin-top: 8px; font-size: .72rem; color: var(--text-tertiary); }
.suggestion-actions { margin-top: 10px; display: flex; gap: 8px; }
.reject-reason { margin-top: 8px; font-size: .78rem; color: var(--text-tertiary); font-style: italic; }

.empty-state { text-align: center; padding: 32px; color: var(--text-tertiary); }
.empty-state .hint { font-size: .78rem; margin-top: 8px; }

.modal-overlay {
  position: fixed; top: 0; left: 0; width: 100%; height: 100%;
  background: rgba(0,0,0,.6); display: flex; align-items: center; justify-content: center; z-index: 999;
}
.modal-box {
  background: var(--bg-card); border-radius: var(--radius-lg); padding: 24px;
  width: 400px; max-width: 90vw;
}
.modal-box h3 { margin: 0 0 12px; }
.input-area {
  width: 100%; min-height: 80px; background: var(--bg-base); border: 1px solid var(--border-subtle);
  border-radius: 6px; padding: 8px; color: var(--text-primary); font-size: .85rem; resize: vertical;
}
.modal-actions { margin-top: 16px; display: flex; gap: 8px; justify-content: flex-end; }

.btn-success { background: var(--color-success); color: white; }
.btn-success:hover { opacity: .9; }
.btn-danger { background: var(--color-danger); color: white; }
.btn-danger:hover { opacity: .9; }
</style>
