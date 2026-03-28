<template>
  <div class="gw-page">
    <!-- 顶部工具栏 -->
    <div class="gw-toolbar">
      <div class="toolbar-left">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
        <span class="toolbar-title">会话管理</span>
      </div>
      <div class="toolbar-right">
        <button class="toolbar-btn refresh-btn" @click="loadSessions" :class="{ spinning: loading }" title="刷新">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
        </button>
      </div>
    </div>

    <!-- 上游选择 -->
    <div class="gw-upstream-bar">
      <label class="upstream-label">上游实例</label>
      <UpstreamSelect v-model="upstreamId" placeholder="选择上游..." />
    </div>

    <!-- 空状态 -->
    <div v-if="!upstreamId" class="gw-empty">
      <div class="empty-icon">🗨️</div>
      <h3>选择上游实例</h3>
      <p>请先在上方选择一个上游实例以查看会话列表。</p>
    </div>

    <!-- 加载中 -->
    <div v-else-if="loading && sessions.length === 0" class="skel-wrap">
      <div class="skel-line" v-for="i in 6" :key="i"></div>
    </div>

    <!-- 会话列表 -->
    <template v-else>
      <div v-if="sessions.length === 0" class="gw-empty">
        <div class="empty-icon">💬</div>
        <h3>暂无会话</h3>
        <p>该上游实例当前没有活跃会话。</p>
      </div>
      <div v-else class="table-wrap">
        <table class="gw-table">
          <thead>
            <tr>
              <th style="width:28px"></th>
              <th>Key</th>
              <th>Channel</th>
              <th>Model</th>
              <th>Tokens (In/Out)</th>
              <th>上下文 (已用/最大)</th>
              <th>最后活跃</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="s in sessions" :key="s.key || s.sessionId">
              <tr class="session-row" :class="{ 'row-active': expandedKey === (s.key || s.sessionId) }" @click="toggleExpand(s)">
                <td>
                  <svg class="expand-chevron" :class="{ open: expandedKey === (s.key || s.sessionId) }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
                </td>
                <td><code class="mono-sm">{{ truncKey(s.key || s.sessionId) }}</code></td>
                <td>{{ s.channel || s.lastChannel || '—' }}</td>
                <td class="text-dim">{{ s.model || '—' }}</td>
                <td>
                  <span class="num-p">{{ fmtNum(s.tokensIn || s.tokens_in || 0) }}</span>
                  <span class="num-s"> / {{ fmtNum(s.tokensOut || s.tokens_out || 0) }}</span>
                </td>
                <td>
                  <span class="num-p">{{ fmtNum(s.contextUsed || s.context_used || 0) }}</span>
                  <span class="num-s"> / {{ fmtNum(s.contextMax || s.context_max || 0) }}</span>
                </td>
                <td>{{ fmtTime(s.updatedAt || s.updated_at) }}</td>
                <td @click.stop>
                  <div class="act-group">
                    <button class="btn btn-xs btn-ghost" @click="toggleExpand(s)" title="查看历史">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                    </button>
                    <button class="btn btn-xs btn-ghost" @click="openModelModal(s)" title="修改模型">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                    </button>
                    <button class="btn btn-xs btn-ghost" @click="confirmReset(s)" title="重置">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/></svg>
                    </button>
                    <button class="btn btn-xs btn-ghost" @click="doCompact(s)" title="压缩">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 14 10 14 10 20"/><polyline points="20 10 14 10 14 4"/><line x1="14" y1="10" x2="21" y2="3"/><line x1="3" y1="21" x2="10" y2="14"/></svg>
                    </button>
                    <button class="btn btn-xs btn-danger-ghost" @click="confirmDelete(s)" title="删除">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                    </button>
                  </div>
                </td>
              </tr>
              <!-- 展开行 — 聊天记录 + 发消息 -->
              <tr v-if="expandedKey === (s.key || s.sessionId)" class="expand-row">
                <td colspan="8">
                  <div class="chat-panel">
                    <div v-if="historyLoading" class="skel-lines"><div class="skel-line" v-for="i in 4" :key="i"></div></div>
                    <div v-else-if="messages.length === 0" class="dtab-empty">暂无消息</div>
                    <div v-else class="chat-messages">
                      <div v-for="(msg, idx) in messages" :key="idx" class="chat-msg" :class="'msg-' + msg.role">
                        <div class="msg-role">{{ msg.role }}</div>
                        <div class="msg-content">{{ extractContent(msg.content) }}</div>
                        <div class="msg-meta" v-if="msg.timestamp">{{ fmtTime(msg.timestamp) }}</div>
                      </div>
                    </div>
                    <!-- 发送消息 -->
                    <div class="chat-input-bar">
                      <input
                        v-model="chatInput"
                        class="chat-input"
                        placeholder="输入消息..."
                        @keydown.enter="sendMessage(s)"
                        :disabled="sending"
                      />
                      <button class="btn btn-sm btn-primary" @click="sendMessage(s)" :disabled="sending || !chatInput.trim()">
                        {{ sending ? '发送中...' : '发送' }}
                      </button>
                      <button class="btn btn-sm btn-danger-ghost" @click="abortChat(s)" title="中止生成">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/></svg>
                        中止
                      </button>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </template>

    <!-- 修改模型模态框 -->
    <Teleport to="body">
      <div v-if="modelModal.visible" class="modal-overlay" @click.self="modelModal.visible = false">
        <div class="modal-box">
          <div class="modal-header">
            <span class="modal-icon">✏️</span>
            <span class="modal-title">修改模型</span>
          </div>
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Session Key</label>
              <code class="mono-sm">{{ modelModal.key }}</code>
            </div>
            <div class="form-group">
              <label class="form-label">模型名称</label>
              <input v-model="modelModal.model" class="form-input" placeholder="例如: gpt-4o, claude-3.5-sonnet" />
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-sm" @click="modelModal.visible = false">取消</button>
            <button class="btn btn-sm btn-primary" @click="doChangeModel" :disabled="modelModal.saving">
              {{ modelModal.saving ? '保存中...' : '保存' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ConfirmModal -->
    <ConfirmModal
      :visible="confirm.visible"
      :title="confirm.title"
      :message="confirm.message"
      :type="confirm.type"
      :confirm-text="confirm.confirmText"
      @confirm="confirm.onConfirm"
      @cancel="confirm.visible = false"
    />
  </div>
</template>

<script setup>
import { ref, watch, inject } from 'vue'
import { api, apiPost, apiPatch, apiDelete } from '../api.js'
import UpstreamSelect from '../components/UpstreamSelect.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const showToast = inject('showToast')

const upstreamId = ref('')
const sessions = ref([])
const loading = ref(false)
const expandedKey = ref(null)
const messages = ref([])
const historyLoading = ref(false)
const chatInput = ref('')
const sending = ref(false)

// 模型修改模态框
const modelModal = ref({ visible: false, key: '', model: '', saving: false })

// 确认模态框
const confirm = ref({ visible: false, title: '', message: '', type: 'warning', confirmText: '确认', onConfirm: () => {} })

watch(upstreamId, () => { if (upstreamId.value) loadSessions() })

async function loadSessions() {
  if (!upstreamId.value) return
  loading.value = true
  try {
    const d = await api(`/api/v1/upstreams/${upstreamId.value}/gateway/sessions`)
    sessions.value = d.sessions || d || []
    if (!Array.isArray(sessions.value)) sessions.value = []
  } catch (e) {
    showToast('加载会话失败: ' + e.message, 'error')
    sessions.value = []
  }
  loading.value = false
}

async function toggleExpand(s) {
  const key = s.key || s.sessionId
  if (expandedKey.value === key) { expandedKey.value = null; return }
  expandedKey.value = key
  await loadHistory(key)
}

async function loadHistory(key) {
  historyLoading.value = true
  messages.value = []
  try {
    const d = await api(`/api/v1/upstreams/${upstreamId.value}/gateway/session-history?key=${encodeURIComponent(key)}&limit=50`)
    messages.value = d.messages || d || []
    if (!Array.isArray(messages.value)) messages.value = []
  } catch (e) {
    showToast('加载聊天记录失败: ' + e.message, 'error')
  }
  historyLoading.value = false
}

function openModelModal(s) {
  modelModal.value = { visible: true, key: s.key || s.sessionId, model: s.model || '', saving: false }
}

async function doChangeModel() {
  modelModal.value.saving = true
  try {
    await apiPatch(`/api/v1/upstreams/${upstreamId.value}/gateway/session`, {
      key: modelModal.value.key,
      model: modelModal.value.model,
    })
    showToast('模型已更新', 'success')
    modelModal.value.visible = false
    loadSessions()
  } catch (e) {
    showToast('修改模型失败: ' + e.message, 'error')
  }
  modelModal.value.saving = false
}

function confirmReset(s) {
  const key = s.key || s.sessionId
  confirm.value = {
    visible: true,
    title: '重置会话',
    message: `确定要重置会话 "${truncKey(key)}" 吗？这将清除所有上下文。`,
    type: 'warning',
    confirmText: '重置',
    onConfirm: async () => {
      confirm.value.visible = false
      try {
        await apiPost(`/api/v1/upstreams/${upstreamId.value}/gateway/session/reset`, { key })
        showToast('会话已重置', 'success')
        loadSessions()
      } catch (e) { showToast('重置失败: ' + e.message, 'error') }
    }
  }
}

async function doCompact(s) {
  const key = s.key || s.sessionId
  try {
    await apiPost(`/api/v1/upstreams/${upstreamId.value}/gateway/session/compact`, { key })
    showToast('会话已压缩', 'success')
    loadSessions()
  } catch (e) { showToast('压缩失败: ' + e.message, 'error') }
}

function confirmDelete(s) {
  const key = s.key || s.sessionId
  confirm.value = {
    visible: true,
    title: '删除会话',
    message: `确定要删除会话 "${truncKey(key)}" 吗？此操作不可恢复！`,
    type: 'danger',
    confirmText: '删除',
    onConfirm: async () => {
      confirm.value.visible = false
      try {
        await apiDelete(`/api/v1/upstreams/${upstreamId.value}/gateway/session?key=${encodeURIComponent(key)}`)
        showToast('会话已删除', 'success')
        loadSessions()
        if (expandedKey.value === key) expandedKey.value = null
      } catch (e) { showToast('删除失败: ' + e.message, 'error') }
    }
  }
}

async function sendMessage(s) {
  const key = s.key || s.sessionId
  if (!chatInput.value.trim()) return
  sending.value = true
  try {
    await apiPost(`/api/v1/upstreams/${upstreamId.value}/gateway/chat/send`, {
      sessionKey: key,
      message: chatInput.value.trim(),
    })
    chatInput.value = ''
    showToast('消息已发送', 'success')
    setTimeout(() => loadHistory(key), 1000)
  } catch (e) { showToast('发送失败: ' + e.message, 'error') }
  sending.value = false
}

async function abortChat(s) {
  const key = s.key || s.sessionId
  try {
    await apiPost(`/api/v1/upstreams/${upstreamId.value}/gateway/chat/abort`, { sessionKey: key })
    showToast('已中止生成', 'success')
  } catch (e) { showToast('中止失败: ' + e.message, 'error') }
}

function truncKey(k) { return k && k.length > 24 ? k.slice(0, 12) + '…' + k.slice(-8) : k }
function fmtNum(n) { return n != null ? n.toLocaleString() : '0' }
function fmtTime(t) {
  if (!t) return '—'
  const d = new Date(t)
  if (isNaN(d.getTime())) return t
  const now = Date.now()
  const diff = now - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return Math.floor(diff / 60000) + ' 分钟前'
  if (diff < 86400000) return Math.floor(diff / 3600000) + ' 小时前'
  return d.toLocaleDateString('zh-CN') + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}
function extractContent(c) {
  if (!c) return ''
  if (typeof c === 'string') return c
  if (Array.isArray(c)) return c.map(p => p.text || p.content || JSON.stringify(p)).join('\n')
  return JSON.stringify(c)
}
</script>

<style scoped>
.gw-page { padding: 0; }

/* Toolbar */
.gw-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 16px 24px; border-bottom: 1px solid var(--border-subtle, #1e293b);
  background: var(--bg-surface, #111827);
}
.toolbar-left { display: flex; align-items: center; gap: 10px; color: var(--text-primary, #e2e8f0); }
.toolbar-title { font-size: 16px; font-weight: 700; }
.toolbar-right { display: flex; align-items: center; gap: 8px; }
.toolbar-btn {
  display: flex; align-items: center; justify-content: center;
  width: 32px; height: 32px; border-radius: 8px; border: 1px solid var(--border-subtle, #334155);
  background: transparent; color: var(--text-secondary, #94a3b8); cursor: pointer;
  transition: all 0.15s;
}
.toolbar-btn:hover { background: var(--bg-elevated, #1e293b); color: var(--text-primary, #e2e8f0); }
.refresh-btn.spinning svg { animation: spin 0.8s linear infinite; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

/* Upstream bar */
.gw-upstream-bar {
  display: flex; align-items: center; gap: 12px;
  padding: 16px 24px; border-bottom: 1px solid var(--border-subtle, #1e293b);
}
.upstream-label { font-size: 13px; font-weight: 600; color: var(--text-secondary, #94a3b8); white-space: nowrap; }

/* Empty */
.gw-empty {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 80px 24px; text-align: center; color: var(--text-tertiary, #64748b);
}
.gw-empty .empty-icon { font-size: 48px; margin-bottom: 16px; opacity: 0.5; }
.gw-empty h3 { font-size: 18px; font-weight: 700; color: var(--text-primary, #e2e8f0); margin: 0 0 8px; }
.gw-empty p { font-size: 14px; margin: 0; }

/* Skeleton */
.skel-wrap { padding: 24px; display: flex; flex-direction: column; gap: 12px; }
.skel-line { height: 36px; background: rgba(99,102,241,0.08); border-radius: 6px; animation: pulse 1.5s ease-in-out infinite; }
.skel-lines { display: flex; flex-direction: column; gap: 8px; padding: 12px 0; }
@keyframes pulse { 0%, 100% { opacity: 0.4; } 50% { opacity: 1; } }

/* Table */
.table-wrap { overflow-x: auto; padding: 0 24px 24px; }
.gw-table {
  width: 100%; border-collapse: collapse; font-size: 13px;
}
.gw-table th {
  padding: 10px 12px; text-align: left; font-size: 11px; font-weight: 700;
  text-transform: uppercase; letter-spacing: 0.05em;
  color: var(--text-tertiary, #64748b); border-bottom: 1px solid var(--border-subtle, #1e293b);
  white-space: nowrap; position: sticky; top: 0; background: var(--bg-surface, #111827);
}
.gw-table td {
  padding: 10px 12px; border-bottom: 1px solid rgba(51,65,85,0.3);
  color: var(--text-primary, #e2e8f0); vertical-align: middle;
}
.session-row { cursor: pointer; transition: background 0.15s; }
.session-row:hover { background: rgba(99,102,241,0.06); }
.row-active { background: rgba(99,102,241,0.08) !important; }
.expand-chevron { transition: transform 0.2s; }
.expand-chevron.open { transform: rotate(90deg); }
.mono-sm { font-family: 'SF Mono', Menlo, monospace; font-size: 12px; color: #a5b4fc; }
.text-dim { color: var(--text-tertiary, #64748b); }
.num-p { font-weight: 600; }
.num-s { color: var(--text-tertiary, #64748b); font-size: 12px; }

/* Actions */
.act-group { display: flex; gap: 4px; }
.btn { display: inline-flex; align-items: center; justify-content: center; gap: 4px; border: none; cursor: pointer; border-radius: 6px; font-weight: 600; transition: all 0.15s; font-family: inherit; }
.btn-xs { padding: 4px 6px; font-size: 12px; }
.btn-sm { padding: 6px 12px; font-size: 13px; }
.btn-ghost { background: transparent; color: var(--text-secondary, #94a3b8); border: 1px solid transparent; }
.btn-ghost:hover { background: rgba(99,102,241,0.1); color: #a5b4fc; }
.btn-danger-ghost { background: transparent; color: #f87171; border: 1px solid transparent; }
.btn-danger-ghost:hover { background: rgba(239,68,68,0.1); }
.btn-primary { background: #6366f1; color: #fff; border: 1px solid #6366f1; }
.btn-primary:hover { background: #4f46e5; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-red { background: #ef4444; color: #fff; }

/* Expand row / Chat panel */
.expand-row td { padding: 0 !important; border-bottom: 1px solid rgba(51,65,85,0.3); }
.chat-panel {
  max-height: 500px; overflow-y: auto; padding: 16px 20px;
  background: rgba(15,23,42,0.5); animation: slideDown 0.2s ease;
}
@keyframes slideDown { from { opacity: 0; max-height: 0; } to { opacity: 1; max-height: 500px; } }
.dtab-empty { padding: 24px; text-align: center; color: var(--text-tertiary, #64748b); font-size: 13px; }

/* Chat messages */
.chat-messages { display: flex; flex-direction: column; gap: 10px; margin-bottom: 16px; }
.chat-msg { padding: 8px 12px; border-radius: 8px; }
.msg-user { background: rgba(99,102,241,0.08); border-left: 3px solid #6366f1; }
.msg-assistant { background: rgba(34,197,94,0.06); border-left: 3px solid #22c55e; }
.msg-system { background: rgba(245,158,11,0.06); border-left: 3px solid #eab308; font-size: 12px; }
.msg-tool { background: rgba(6,182,212,0.06); border-left: 3px solid #06b6d4; font-size: 12px; }
.msg-role { font-size: 11px; font-weight: 600; text-transform: uppercase; margin-bottom: 4px; color: var(--text-tertiary, #64748b); letter-spacing: 0.03em; }
.msg-content { font-size: 13px; line-height: 1.5; white-space: pre-wrap; word-break: break-word; color: var(--text-primary, #e2e8f0); }
.msg-meta { font-size: 10px; color: var(--text-tertiary, #64748b); margin-top: 4px; }

/* Chat input */
.chat-input-bar { display: flex; gap: 8px; align-items: center; padding-top: 12px; border-top: 1px solid rgba(51,65,85,0.4); }
.chat-input {
  flex: 1; padding: 8px 12px; background: var(--bg-elevated, #1e293b);
  border: 1px solid var(--border-subtle, #334155); border-radius: 8px;
  color: var(--text-primary, #e2e8f0); font-size: 13px; outline: none;
  font-family: inherit; transition: border-color 0.15s;
}
.chat-input:focus { border-color: #6366f1; }
.chat-input:disabled { opacity: 0.5; }

/* Modal */
.modal-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.5); z-index: 1000;
  display: flex; align-items: center; justify-content: center;
  animation: fadeIn 0.2s;
}
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
.modal-box {
  background: var(--bg-surface, #111827); border: 1px solid var(--border-default, #334155);
  border-radius: 12px; padding: 24px; min-width: 400px; max-width: 520px;
  box-shadow: 0 16px 64px rgba(0,0,0,0.5); animation: slideUp 0.2s ease-out;
}
@keyframes slideUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
.modal-header { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.modal-icon { font-size: 1.4rem; }
.modal-title { font-size: 16px; font-weight: 700; color: var(--text-primary, #e2e8f0); }
.modal-body { margin-bottom: 20px; }
.modal-footer { display: flex; justify-content: flex-end; gap: 8px; }
.form-group { margin-bottom: 16px; }
.form-label { display: block; font-size: 12px; font-weight: 600; color: var(--text-secondary, #94a3b8); margin-bottom: 6px; }
.form-input {
  width: 100%; padding: 8px 12px; background: var(--bg-elevated, #1e293b);
  border: 1px solid var(--border-subtle, #334155); border-radius: 8px;
  color: var(--text-primary, #e2e8f0); font-size: 13px; outline: none;
  font-family: inherit; transition: border-color 0.15s; box-sizing: border-box;
}
.form-input:focus { border-color: #6366f1; }
</style>