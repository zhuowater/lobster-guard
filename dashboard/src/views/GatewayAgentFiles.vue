<template>
  <div class="gw-page">
    <!-- 顶部工具栏 -->
    <div class="gw-toolbar">
      <div class="toolbar-left">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a4 4 0 0 1 4 4c0 1.95-1.4 3.58-3.25 3.93L12 10v2"/><circle cx="12" cy="16" r="2"/><path d="M12 18v2"/><path d="M7 20h10"/></svg>
        <span class="toolbar-title">Agent 管理</span>
      </div>
      <div class="toolbar-right">
        <button class="toolbar-btn refresh-btn" @click="refresh" :class="{ spinning: agentsLoading }" title="刷新">
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
      <div class="empty-icon">🤖</div>
      <h3>选择上游实例</h3>
      <p>请先在上方选择一个上游实例以管理 Agent 文件。</p>
    </div>

    <template v-else>
      <!-- 心跳 + 设备/节点 概览卡片 -->
      <div class="info-cards">
        <!-- 心跳状态 -->
        <div class="info-card">
          <div class="info-card-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
            <span>心跳状态</span>
          </div>
          <div class="info-card-body" v-if="heartbeat">
            <div class="hb-row">
              <span class="hb-dot" :class="heartbeat.alive ? 'hb-alive' : 'hb-dead'"></span>
              <span>{{ heartbeat.alive ? '在线' : '离线' }}</span>
            </div>
            <div class="hb-detail" v-if="heartbeat.lastSeen">最后心跳: {{ fmtTime(heartbeat.lastSeen || heartbeat.last_seen) }}</div>
            <button class="btn btn-sm btn-primary" @click="doWake" :disabled="waking" style="margin-top:8px">
              {{ waking ? '唤醒中...' : '唤醒 Agent' }}
            </button>
          </div>
          <div v-else class="info-card-body text-dim">加载中...</div>
        </div>

        <!-- 设备列表 -->
        <div class="info-card">
          <div class="info-card-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
            <span>设备 ({{ devices.length }})</span>
          </div>
          <div class="info-card-body">
            <div v-if="devices.length === 0" class="text-dim">无设备</div>
            <div v-else class="device-list">
              <div v-for="d in devices" :key="d.id || d.name" class="device-item">
                <span class="device-dot" :class="d.online ? 'dev-on' : 'dev-off'"></span>
                <span>{{ d.name || d.id }}</span>
                <span class="text-dim" v-if="d.platform"> · {{ d.platform }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 节点配对 -->
        <div class="info-card">
          <div class="info-card-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M12 1v4M12 19v4M4.22 4.22l2.83 2.83M16.95 16.95l2.83 2.83M1 12h4M19 12h4M4.22 19.78l2.83-2.83M16.95 7.05l2.83-2.83"/></svg>
            <span>节点 ({{ nodes.length }})</span>
          </div>
          <div class="info-card-body">
            <div v-if="nodes.length === 0" class="text-dim">无节点</div>
            <div v-else class="device-list">
              <div v-for="n in nodes" :key="n.id || n.name" class="device-item">
                <span class="device-dot" :class="n.online || n.connected ? 'dev-on' : 'dev-off'"></span>
                <span>{{ n.name || n.id }}</span>
                <span class="text-dim" v-if="n.type"> · {{ n.type }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Agent 文件编辑器布局 -->
      <div class="editor-layout">
        <!-- 左侧：Agent 列表 + 文件列表 -->
        <div class="editor-sidebar">
          <div class="es-section">
            <div class="es-title">Agent 列表</div>
            <div v-if="agentsLoading" class="skel-lines"><div class="skel-line" v-for="i in 3" :key="i"></div></div>
            <div v-else-if="agents.length === 0" class="es-empty">暂无 Agent</div>
            <div v-else class="agent-list">
              <div
                v-for="a in agents" :key="a.id"
                class="agent-item"
                :class="{ 'agent-active': selectedAgent?.id === a.id }"
                @click="selectAgent(a)"
              >
                <div class="agent-avatar" :style="{ background: agentColor(a.id) }">
                  {{ (a.id || a.name || '?')[0].toUpperCase() }}
                </div>
                <div class="agent-info">
                  <div class="agent-name">{{ a.id || a.name }}</div>
                  <div class="agent-meta">{{ a.model || '—' }}</div>
                </div>
              </div>
            </div>
          </div>

          <div class="es-section" v-if="selectedAgent">
            <div class="es-title">文件</div>
            <div v-if="filesLoading" class="skel-lines"><div class="skel-line" v-for="i in 3" :key="i"></div></div>
            <div v-else-if="files.length === 0" class="es-empty">暂无文件</div>
            <div v-else class="file-list">
              <div
                v-for="f in files" :key="f.name || f"
                class="file-item"
                :class="{ 'file-active': selectedFile === (f.name || f) }"
                @click="selectFile(f.name || f)"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg>
                <span>{{ f.name || f }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 右侧：文件编辑器 -->
        <div class="editor-main">
          <div v-if="!selectedAgent" class="editor-placeholder">
            <div class="ep-icon">📝</div>
            <p>选择一个 Agent 开始编辑文件</p>
          </div>
          <div v-else-if="!selectedFile" class="editor-placeholder">
            <div class="ep-icon">📄</div>
            <p>选择一个文件进行编辑</p>
          </div>
          <template v-else>
            <div class="editor-header">
              <div class="editor-file-info">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg>
                <span class="editor-filename">{{ selectedFile }}</span>
                <span class="editor-agent-id">{{ selectedAgent.id }}</span>
              </div>
              <div class="editor-actions">
                <span v-if="fileDirty" class="dirty-badge">未保存</span>
                <button class="btn btn-sm btn-primary" @click="saveFile" :disabled="saving || !fileDirty">
                  {{ saving ? '保存中...' : '保存' }}
                </button>
              </div>
            </div>
            <div class="editor-body">
              <div v-if="fileLoading" class="editor-loading">
                <div class="skel-line" v-for="i in 8" :key="i"></div>
              </div>
              <textarea
                v-else
                v-model="fileContent"
                class="code-editor"
                spellcheck="false"
                @input="fileDirty = true"
              ></textarea>
            </div>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, watch, inject } from 'vue'
import { api, apiPost, apiPut } from '../api.js'
import UpstreamSelect from '../components/UpstreamSelect.vue'

const showToast = inject('showToast')

const upstreamId = ref('')
const agents = ref([])
const agentsLoading = ref(false)
const selectedAgent = ref(null)
const files = ref([])
const filesLoading = ref(false)
const selectedFile = ref(null)
const fileContent = ref('')
const fileLoading = ref(false)
const fileDirty = ref(false)
const saving = ref(false)

// Info cards
const heartbeat = ref(null)
const devices = ref([])
const nodes = ref([])
const waking = ref(false)

watch(upstreamId, () => {
  if (upstreamId.value) {
    loadAgents()
    loadHeartbeat()
    loadDevices()
    loadNodes()
  }
  selectedAgent.value = null
  selectedFile.value = null
  files.value = []
})

function refresh() {
  loadAgents()
  loadHeartbeat()
  loadDevices()
  loadNodes()
}

async function loadAgents() {
  agentsLoading.value = true
  try {
    const d = await api(`/api/v1/upstreams/${upstreamId.value}/gateway/agents`)
    agents.value = d.agents || d || []
    if (!Array.isArray(agents.value)) agents.value = []
  } catch (e) {
    showToast('加载 Agent 列表失败: ' + e.message, 'error')
    agents.value = []
  }
  agentsLoading.value = false
}

async function loadHeartbeat() {
  try {
    const d = await api(`/api/v1/upstreams/${upstreamId.value}/gateway/heartbeat`)
    heartbeat.value = d
  } catch { heartbeat.value = null }
}

async function loadDevices() {
  try {
    const d = await api(`/api/v1/upstreams/${upstreamId.value}/gateway/devices`)
    devices.value = d.devices || d || []
    if (!Array.isArray(devices.value)) devices.value = []
  } catch { devices.value = [] }
}

async function loadNodes() {
  try {
    const d = await api(`/api/v1/upstreams/${upstreamId.value}/gateway/node-pairs`)
    nodes.value = d.nodes || d.pairs || d || []
    if (!Array.isArray(nodes.value)) nodes.value = []
  } catch { nodes.value = [] }
}

async function doWake() {
  waking.value = true
  try {
    await apiPost(`/api/v1/upstreams/${upstreamId.value}/gateway/wake`, {})
    showToast('唤醒命令已发送', 'success')
    setTimeout(loadHeartbeat, 2000)
  } catch (e) { showToast('唤醒失败: ' + e.message, 'error') }
  waking.value = false
}

async function selectAgent(a) {
  if (selectedAgent.value?.id === a.id) return
  selectedAgent.value = a
  selectedFile.value = null
  fileContent.value = ''
  fileDirty.value = false
  filesLoading.value = true
  try {
    const d = await api(`/api/v1/upstreams/${upstreamId.value}/gateway/agents/files?agentId=${encodeURIComponent(a.id)}`)
    files.value = d.files || d || []
    if (!Array.isArray(files.value)) files.value = []
  } catch (e) {
    showToast('加载文件列表失败: ' + e.message, 'error')
    files.value = []
  }
  filesLoading.value = false
}

async function selectFile(name) {
  if (fileDirty.value && selectedFile.value) {
    if (!window.confirm('当前文件有未保存的修改，确定切换？')) return
  }
  selectedFile.value = name
  fileLoading.value = true
  fileDirty.value = false
  try {
    const d = await api(`/api/v1/upstreams/${upstreamId.value}/gateway/agents/file?agentId=${encodeURIComponent(selectedAgent.value.id)}&name=${encodeURIComponent(name)}`)
    fileContent.value = d.content || d.text || (typeof d === 'string' ? d : JSON.stringify(d, null, 2))
  } catch (e) {
    showToast('加载文件失败: ' + e.message, 'error')
    fileContent.value = ''
  }
  fileLoading.value = false
}

async function saveFile() {
  saving.value = true
  try {
    await apiPut(`/api/v1/upstreams/${upstreamId.value}/gateway/agents/file`, {
      agentId: selectedAgent.value.id,
      name: selectedFile.value,
      content: fileContent.value,
    })
    showToast('文件已保存', 'success')
    fileDirty.value = false
  } catch (e) { showToast('保存失败: ' + e.message, 'error') }
  saving.value = false
}

// Helpers
const colors = ['#6366f1', '#8b5cf6', '#06b6d4', '#10b981', '#f59e0b', '#ef4444', '#ec4899']
function agentColor(id) { return colors[(id || '').split('').reduce((a, c) => a + c.charCodeAt(0), 0) % colors.length] }

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
</script>

<style scoped>
.gw-page { padding: 0; display: flex; flex-direction: column; height: 100%; }

/* Toolbar */
.gw-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 16px 24px; border-bottom: 1px solid var(--border-subtle, #1e293b);
  background: var(--bg-surface, #111827); flex-shrink: 0;
}
.toolbar-left { display: flex; align-items: center; gap: 10px; color: var(--text-primary, #e2e8f0); }
.toolbar-title { font-size: 16px; font-weight: 700; }
.toolbar-right { display: flex; align-items: center; gap: 8px; }
.toolbar-btn {
  display: flex; align-items: center; justify-content: center;
  width: 32px; height: 32px; border-radius: 8px; border: 1px solid var(--border-subtle, #334155);
  background: transparent; color: var(--text-secondary, #94a3b8); cursor: pointer; transition: all 0.15s;
}
.toolbar-btn:hover { background: var(--bg-elevated, #1e293b); color: var(--text-primary, #e2e8f0); }
.refresh-btn.spinning svg { animation: spin 0.8s linear infinite; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

/* Upstream */
.gw-upstream-bar {
  display: flex; align-items: center; gap: 12px;
  padding: 16px 24px; border-bottom: 1px solid var(--border-subtle, #1e293b); flex-shrink: 0;
}
.upstream-label { font-size: 13px; font-weight: 600; color: var(--text-secondary, #94a3b8); white-space: nowrap; }

/* Empty */
.gw-empty {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 80px 24px; text-align: center; color: var(--text-tertiary, #64748b); flex: 1;
}
.gw-empty .empty-icon { font-size: 48px; margin-bottom: 16px; opacity: 0.5; }
.gw-empty h3 { font-size: 18px; font-weight: 700; color: var(--text-primary, #e2e8f0); margin: 0 0 8px; }
.gw-empty p { font-size: 14px; margin: 0; }

/* Skeleton */
.skel-lines { display: flex; flex-direction: column; gap: 8px; padding: 8px 0; }
.skel-line { height: 28px; background: rgba(99,102,241,0.08); border-radius: 6px; animation: pulse 1.5s ease-in-out infinite; }
@keyframes pulse { 0%, 100% { opacity: 0.4; } 50% { opacity: 1; } }

/* Info cards */
.info-cards {
  display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px;
  padding: 16px 24px; border-bottom: 1px solid var(--border-subtle, #1e293b); flex-shrink: 0;
}
@media (max-width: 900px) { .info-cards { grid-template-columns: 1fr; } }
.info-card {
  background: var(--bg-elevated, #1e293b); border: 1px solid var(--border-subtle, #334155);
  border-radius: 10px; padding: 14px; transition: border-color 0.15s;
}
.info-card:hover { border-color: rgba(99,102,241,0.3); }
.info-card-header {
  display: flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 700;
  color: var(--text-primary, #e2e8f0); margin-bottom: 10px;
  padding-bottom: 8px; border-bottom: 1px solid rgba(51,65,85,0.4);
}
.info-card-body { font-size: 13px; color: var(--text-secondary, #94a3b8); }
.hb-row { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.hb-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.hb-alive { background: #22c55e; box-shadow: 0 0 6px rgba(34,197,94,0.6); }
.hb-dead { background: #ef4444; box-shadow: 0 0 6px rgba(239,68,68,0.6); }
.hb-detail { font-size: 12px; color: var(--text-tertiary, #64748b); }
.device-list { display: flex; flex-direction: column; gap: 6px; }
.device-item { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.device-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
.dev-on { background: #22c55e; }
.dev-off { background: #64748b; }
.text-dim { color: var(--text-tertiary, #64748b); }

/* Editor layout */
.editor-layout {
  display: flex; flex: 1; min-height: 0; overflow: hidden;
}
.editor-sidebar {
  width: 260px; min-width: 260px; border-right: 1px solid var(--border-subtle, #1e293b);
  overflow-y: auto; background: var(--bg-surface, #111827);
  display: flex; flex-direction: column;
}
.es-section { padding: 12px 16px; border-bottom: 1px solid var(--border-subtle, #1e293b); }
.es-title {
  font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.08em;
  color: var(--text-tertiary, #64748b); margin-bottom: 10px;
}
.es-empty { font-size: 12px; color: var(--text-tertiary, #64748b); padding: 8px 0; }

/* Agent list */
.agent-list { display: flex; flex-direction: column; gap: 4px; }
.agent-item {
  display: flex; align-items: center; gap: 10px;
  padding: 8px 10px; border-radius: 8px; cursor: pointer;
  transition: all 0.15s;
}
.agent-item:hover { background: rgba(99,102,241,0.06); }
.agent-active { background: rgba(99,102,241,0.1) !important; border-left: 3px solid #6366f1; }
.agent-avatar {
  width: 32px; height: 32px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-weight: 700; font-size: 14px; flex-shrink: 0;
}
.agent-info { flex: 1; min-width: 0; }
.agent-name { font-size: 13px; font-weight: 600; color: var(--text-primary, #e2e8f0); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.agent-meta { font-size: 11px; color: var(--text-tertiary, #64748b); }

/* File list */
.file-list { display: flex; flex-direction: column; gap: 2px; }
.file-item {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 10px; border-radius: 6px; cursor: pointer;
  font-size: 13px; color: var(--text-secondary, #94a3b8);
  transition: all 0.15s;
}
.file-item:hover { background: rgba(99,102,241,0.06); color: var(--text-primary, #e2e8f0); }
.file-active { background: rgba(99,102,241,0.1) !important; color: #a5b4fc !important; }

/* Editor main */
.editor-main { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.editor-placeholder {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
  color: var(--text-tertiary, #64748b); gap: 8px;
}
.ep-icon { font-size: 48px; opacity: 0.4; }
.editor-placeholder p { font-size: 14px; margin: 0; }

.editor-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 20px; border-bottom: 1px solid var(--border-subtle, #1e293b);
  background: var(--bg-surface, #111827); flex-shrink: 0;
}
.editor-file-info { display: flex; align-items: center; gap: 8px; }
.editor-filename { font-size: 14px; font-weight: 700; color: var(--text-primary, #e2e8f0); }
.editor-agent-id { font-size: 11px; color: var(--text-tertiary, #64748b); padding: 2px 8px; background: rgba(99,102,241,0.08); border-radius: 4px; }
.editor-actions { display: flex; align-items: center; gap: 8px; }
.dirty-badge {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 9999px;
  background: rgba(245,158,11,0.12); color: #fbbf24;
}

.editor-body { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.editor-loading { padding: 20px; display: flex; flex-direction: column; gap: 8px; }
.code-editor {
  flex: 1; width: 100%; padding: 16px 20px;
  background: var(--bg-base, #0f172a); color: var(--text-primary, #e2e8f0);
  border: none; outline: none; resize: none;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', Menlo, Monaco, 'Courier New', monospace;
  font-size: 13px; line-height: 1.6; tab-size: 2;
  box-sizing: border-box;
}
.code-editor:focus { background: rgba(15,23,42,0.8); }

/* Buttons */
.btn { display: inline-flex; align-items: center; justify-content: center; gap: 4px; border: none; cursor: pointer; border-radius: 6px; font-weight: 600; transition: all 0.15s; font-family: inherit; }
.btn-sm { padding: 6px 12px; font-size: 13px; }
.btn-primary { background: #6366f1; color: #fff; border: 1px solid #6366f1; }
.btn-primary:hover { background: #4f46e5; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
