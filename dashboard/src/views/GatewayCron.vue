<template>
  <div class="gw-page">
    <!-- 顶部工具栏 -->
    <div class="gw-toolbar">
      <div class="toolbar-left">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
        <span class="toolbar-title">定时任务管理</span>
      </div>
      <div class="toolbar-right">
        <button class="btn btn-sm btn-primary" @click="openAddModal">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          新建任务
        </button>
        <button class="toolbar-btn refresh-btn" @click="loadCronJobs" :class="{ spinning: loading }" title="刷新">
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
      <div class="empty-icon">⏰</div>
      <h3>选择上游实例</h3>
      <p>请先在上方选择一个上游实例以查看定时任务。</p>
    </div>

    <!-- 加载中 -->
    <div v-else-if="loading && cronJobs.length === 0" class="skel-wrap">
      <div class="skel-line" v-for="i in 5" :key="i"></div>
    </div>

    <!-- Cron 列表 -->
    <template v-else>
      <div v-if="cronJobs.length === 0" class="gw-empty">
        <div class="empty-icon">📋</div>
        <h3>暂无定时任务</h3>
        <p>点击"新建任务"来创建第一个定时任务。</p>
      </div>
      <div v-else class="table-wrap">
        <table class="gw-table">
          <thead>
            <tr>
              <th style="width:28px"></th>
              <th>名称</th>
              <th>调度类型</th>
              <th>调度表达式</th>
              <th>Payload</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="c in cronJobs" :key="c.id">
              <tr class="cron-row" :class="{ 'row-active': expandedId === c.id }" @click="toggleExpand(c)">
                <td>
                  <svg class="expand-chevron" :class="{ open: expandedId === c.id }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
                </td>
                <td><strong>{{ c.name || c.id || '—' }}</strong></td>
                <td><span class="schedule-badge" :class="'sched-' + scheduleKind(c)">{{ scheduleKind(c) }}</span></td>
                <td><code class="mono-sm">{{ scheduleExpr(c) }}</code></td>
                <td>
                  <span class="payload-badge" :class="'pl-' + payloadKind(c)">{{ payloadKind(c) }}</span>
                  <span class="text-dim payload-text">{{ payloadPreview(c) }}</span>
                </td>
                <td>
                  <span class="status-badge" :class="c.enabled !== false ? 'st-on' : 'st-off'" @click.stop="toggleEnabled(c)">
                    {{ c.enabled !== false ? '启用' : '禁用' }}
                  </span>
                </td>
                <td @click.stop>
                  <div class="act-group">
                    <button class="btn btn-xs btn-ghost" @click="openEditModal(c)" title="编辑">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                    </button>
                    <button class="btn btn-xs btn-ghost" @click="triggerRun(c)" title="立即运行">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                    </button>
                    <button class="btn btn-xs btn-ghost" @click="toggleExpand(c)" title="运行历史">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                    </button>
                    <button class="btn btn-xs btn-danger-ghost" @click="confirmDeleteCron(c)" title="删除">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                    </button>
                  </div>
                </td>
              </tr>
              <!-- 展开行：运行历史 -->
              <tr v-if="expandedId === c.id" class="expand-row">
                <td colspan="7">
                  <div class="runs-panel">
                    <h4 class="runs-title">运行历史</h4>
                    <div v-if="runsLoading" class="skel-lines"><div class="skel-line" v-for="i in 3" :key="i"></div></div>
                    <div v-else-if="runs.length === 0" class="dtab-empty">暂无运行记录</div>
                    <table v-else class="inner-table">
                      <thead><tr><th>运行时间</th><th>状态</th><th>耗时</th><th>结果</th></tr></thead>
                      <tbody>
                        <tr v-for="r in runs" :key="r.id || r.startedAt">
                          <td>{{ fmtTime(r.startedAt || r.started_at || r.time) }}</td>
                          <td><span class="run-status" :class="r.success !== false ? 'rs-ok' : 'rs-fail'">{{ r.success !== false ? '成功' : '失败' }}</span></td>
                          <td>{{ r.durationMs || r.duration_ms || '—' }}ms</td>
                          <td class="text-dim run-result">{{ truncResult(r.result || r.error || '—') }}</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </template>

    <!-- 创建/编辑 Cron 模态框 -->
    <Teleport to="body">
      <div v-if="cronModal.visible" class="modal-overlay" @click.self="cronModal.visible = false">
        <div class="modal-box modal-wide">
          <div class="modal-header">
            <span class="modal-icon">{{ cronModal.editing ? '✏️' : '➕' }}</span>
            <span class="modal-title">{{ cronModal.editing ? '编辑任务' : '新建任务' }}</span>
          </div>
          <div class="modal-body">
            <!-- 名称 -->
            <div class="form-group">
              <label class="form-label">任务名称</label>
              <input v-model="cronModal.form.name" class="form-input" placeholder="例如: daily-summary" />
            </div>

            <!-- 调度类型 -->
            <div class="form-group">
              <label class="form-label">调度类型</label>
              <div class="radio-group">
                <label class="radio-item" v-for="sk in scheduleKinds" :key="sk.value">
                  <input type="radio" v-model="cronModal.form.scheduleKind" :value="sk.value" />
                  <span class="radio-label">{{ sk.label }}</span>
                </label>
              </div>
            </div>

            <!-- 调度表达式 -->
            <div class="form-group" v-if="cronModal.form.scheduleKind === 'cron'">
              <label class="form-label">Cron 表达式</label>
              <input v-model="cronModal.form.scheduleExpr" class="form-input font-mono" placeholder="0 */6 * * *" />
            </div>
            <div class="form-group" v-if="cronModal.form.scheduleKind === 'every'">
              <label class="form-label">间隔</label>
              <input v-model="cronModal.form.scheduleEvery" class="form-input" placeholder="例如: 30m, 2h, 1d" />
            </div>
            <div class="form-group" v-if="cronModal.form.scheduleKind === 'at'">
              <label class="form-label">指定时间</label>
              <input v-model="cronModal.form.scheduleExpr" class="form-input" placeholder="例如: 09:00, 2024-03-28T09:00:00Z" />
            </div>

            <!-- Payload 类型 -->
            <div class="form-group">
              <label class="form-label">Payload 类型</label>
              <div class="radio-group">
                <label class="radio-item" v-for="pk in payloadKinds" :key="pk.value">
                  <input type="radio" v-model="cronModal.form.payloadKind" :value="pk.value" />
                  <span class="radio-label">{{ pk.label }}</span>
                </label>
              </div>
            </div>

            <!-- Payload 内容 -->
            <div class="form-group">
              <label class="form-label">Payload 内容</label>
              <textarea v-model="cronModal.form.payloadText" class="form-textarea" rows="4" placeholder="消息内容或事件数据..."></textarea>
            </div>

            <!-- 启用 -->
            <div class="form-group">
              <label class="toggle-inline">
                <input type="checkbox" v-model="cronModal.form.enabled" />
                <span>启用该任务</span>
              </label>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-sm" @click="cronModal.visible = false">取消</button>
            <button class="btn btn-sm btn-primary" @click="saveCron" :disabled="cronModal.saving">
              {{ cronModal.saving ? '保存中...' : '保存' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ConfirmModal -->
    <ConfirmModal
      :visible="confirmDlg.visible"
      :title="confirmDlg.title"
      :message="confirmDlg.message"
      :type="confirmDlg.type"
      :confirm-text="confirmDlg.confirmText"
      @confirm="confirmDlg.onConfirm"
      @cancel="confirmDlg.visible = false"
    />
  </div>
</template>

<script setup>
import { ref, watch, inject } from 'vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import UpstreamSelect from '../components/UpstreamSelect.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const showToast = inject('showToast')

const upstreamId = ref('')
const cronJobs = ref([])
const loading = ref(false)
const expandedId = ref(null)
const runs = ref([])
const runsLoading = ref(false)

const scheduleKinds = [
  { value: 'cron', label: 'Cron 表达式' },
  { value: 'every', label: '固定间隔 (every)' },
  { value: 'at', label: '指定时间 (at)' },
]
const payloadKinds = [
  { value: 'systemEvent', label: 'System Event' },
  { value: 'agentTurn', label: 'Agent Turn' },
]

// Cron 模态框
const cronModal = ref({
  visible: false, editing: false, saving: false,
  form: { id: '', name: '', scheduleKind: 'cron', scheduleExpr: '', scheduleEvery: '', payloadKind: 'agentTurn', payloadText: '', enabled: true },
})

// 确认对话框
const confirmDlg = ref({ visible: false, title: '', message: '', type: 'warning', confirmText: '确认', onConfirm: () => {} })

watch(upstreamId, () => { if (upstreamId.value) loadCronJobs() })

async function loadCronJobs() {
  if (!upstreamId.value) return
  loading.value = true
  try {
    const d = await api(`/api/v1/upstreams/${upstreamId.value}/gateway/cron`)
    cronJobs.value = d.jobs || d.crons || d || []
    if (!Array.isArray(cronJobs.value)) cronJobs.value = []
  } catch (e) {
    showToast('加载定时任务失败: ' + e.message, 'error')
    cronJobs.value = []
  }
  loading.value = false
}

async function toggleExpand(c) {
  if (expandedId.value === c.id) { expandedId.value = null; return }
  expandedId.value = c.id
  await loadRuns(c.id)
}

async function loadRuns(id) {
  runsLoading.value = true
  runs.value = []
  try {
    const d = await api(`/api/v1/upstreams/${upstreamId.value}/gateway/cron/runs?id=${encodeURIComponent(id)}&limit=10`)
    runs.value = d.runs || d || []
    if (!Array.isArray(runs.value)) runs.value = []
  } catch (e) {
    showToast('加载运行历史失败: ' + e.message, 'error')
  }
  runsLoading.value = false
}

function openAddModal() {
  cronModal.value = {
    visible: true, editing: false, saving: false,
    form: { id: '', name: '', scheduleKind: 'cron', scheduleExpr: '', scheduleEvery: '', payloadKind: 'agentTurn', payloadText: '', enabled: true },
  }
}

function openEditModal(c) {
  const sk = c.schedule?.kind || (c.cron ? 'cron' : (c.every ? 'every' : 'cron'))
  cronModal.value = {
    visible: true, editing: true, saving: false,
    form: {
      id: c.id,
      name: c.name || '',
      scheduleKind: sk,
      scheduleExpr: c.schedule?.expr || c.cron || c.at || '',
      scheduleEvery: c.schedule?.every || c.every || '',
      payloadKind: c.payload?.kind || 'agentTurn',
      payloadText: c.payload?.text || c.payload?.message || '',
      enabled: c.enabled !== false,
    },
  }
}

async function saveCron() {
  const f = cronModal.value.form
  cronModal.value.saving = true
  const body = {
    id: f.id || undefined,
    name: f.name,
    schedule: { kind: f.scheduleKind },
    payload: { kind: f.payloadKind, text: f.payloadText },
    enabled: f.enabled,
  }
  if (f.scheduleKind === 'cron') body.schedule.expr = f.scheduleExpr
  else if (f.scheduleKind === 'every') body.schedule.every = f.scheduleEvery
  else if (f.scheduleKind === 'at') body.schedule.expr = f.scheduleExpr

  try {
    if (cronModal.value.editing) {
      await apiPut(`/api/v1/upstreams/${upstreamId.value}/gateway/cron/update`, body)
      showToast('任务已更新', 'success')
    } else {
      await apiPost(`/api/v1/upstreams/${upstreamId.value}/gateway/cron/add`, body)
      showToast('任务已创建', 'success')
    }
    cronModal.value.visible = false
    loadCronJobs()
  } catch (e) {
    showToast('保存失败: ' + e.message, 'error')
  }
  cronModal.value.saving = false
}

async function triggerRun(c) {
  try {
    await apiPost(`/api/v1/upstreams/${upstreamId.value}/gateway/cron/run`, { id: c.id })
    showToast('任务已触发运行', 'success')
    if (expandedId.value === c.id) setTimeout(() => loadRuns(c.id), 1500)
  } catch (e) { showToast('触发运行失败: ' + e.message, 'error') }
}

async function toggleEnabled(c) {
  const newEnabled = c.enabled === false
  try {
    await apiPut(`/api/v1/upstreams/${upstreamId.value}/gateway/cron/update`, { id: c.id, enabled: newEnabled })
    c.enabled = newEnabled
    showToast(newEnabled ? '任务已启用' : '任务已禁用', 'success')
  } catch (e) { showToast('操作失败: ' + e.message, 'error') }
}

function confirmDeleteCron(c) {
  confirmDlg.value = {
    visible: true,
    title: '删除定时任务',
    message: `确定要删除任务 "${c.name || c.id}" 吗？此操作不可恢复！`,
    type: 'danger',
    confirmText: '删除',
    onConfirm: async () => {
      confirmDlg.value.visible = false
      try {
        await apiDelete(`/api/v1/upstreams/${upstreamId.value}/gateway/cron/remove?id=${encodeURIComponent(c.id)}`)
        showToast('任务已删除', 'success')
        loadCronJobs()
        if (expandedId.value === c.id) expandedId.value = null
      } catch (e) { showToast('删除失败: ' + e.message, 'error') }
    }
  }
}

// Helpers
function scheduleKind(c) { return c.schedule?.kind || (c.cron ? 'cron' : (c.every ? 'every' : (c.at ? 'at' : 'cron'))) }
function scheduleExpr(c) { return c.schedule?.expr || c.schedule?.every || c.cron || c.every || c.at || '—' }
function payloadKind(c) { return c.payload?.kind || 'agentTurn' }
function payloadPreview(c) {
  const text = c.payload?.text || c.payload?.message || ''
  return text.length > 40 ? text.slice(0, 40) + '…' : text
}
function truncResult(r) { return r && r.length > 60 ? r.slice(0, 60) + '…' : r }
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
  background: transparent; color: var(--text-secondary, #94a3b8); cursor: pointer; transition: all 0.15s;
}
.toolbar-btn:hover { background: var(--bg-elevated, #1e293b); color: var(--text-primary, #e2e8f0); }
.refresh-btn.spinning svg { animation: spin 0.8s linear infinite; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

/* Upstream */
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
.cron-row { cursor: pointer; transition: background 0.15s; }
.cron-row:hover { background: rgba(99,102,241,0.06); }
.row-active { background: rgba(99,102,241,0.08) !important; }
.expand-chevron { transition: transform 0.2s; }
.expand-chevron.open { transform: rotate(90deg); }
.mono-sm { font-family: 'SF Mono', Menlo, monospace; font-size: 12px; color: #a5b4fc; }
.text-dim { color: var(--text-tertiary, #64748b); }

/* Badges */
.schedule-badge {
  display: inline-block; font-size: 10px; font-weight: 700; padding: 2px 8px;
  border-radius: 9999px; text-transform: uppercase; letter-spacing: 0.04em;
}
.sched-cron { background: rgba(99,102,241,0.12); color: #a5b4fc; }
.sched-every { background: rgba(34,197,94,0.12); color: #4ade80; }
.sched-at { background: rgba(245,158,11,0.12); color: #fbbf24; }

.payload-badge {
  display: inline-block; font-size: 10px; font-weight: 600; padding: 2px 6px;
  border-radius: 4px; margin-right: 6px;
}
.pl-systemEvent { background: rgba(6,182,212,0.12); color: #22d3ee; }
.pl-agentTurn { background: rgba(139,92,246,0.12); color: #c4b5fd; }
.payload-text { font-size: 12px; }

.status-badge {
  display: inline-block; font-size: 11px; font-weight: 600; padding: 3px 10px;
  border-radius: 9999px; cursor: pointer; transition: all 0.15s;
}
.st-on { background: rgba(34,197,94,0.12); color: #4ade80; }
.st-on:hover { background: rgba(34,197,94,0.2); }
.st-off { background: rgba(100,116,139,0.12); color: #94a3b8; }
.st-off:hover { background: rgba(100,116,139,0.2); }

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

/* Expand row / Runs */
.expand-row td { padding: 0 !important; border-bottom: 1px solid rgba(51,65,85,0.3); }
.runs-panel {
  padding: 16px 20px; background: rgba(15,23,42,0.5);
  animation: slideDown 0.2s ease;
}
@keyframes slideDown { from { opacity: 0; max-height: 0; } to { opacity: 1; max-height: 400px; } }
.runs-title { font-size: 13px; font-weight: 700; color: var(--text-primary, #e2e8f0); margin: 0 0 12px; }
.dtab-empty { padding: 24px; text-align: center; color: var(--text-tertiary, #64748b); font-size: 13px; }

.inner-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.inner-table th {
  padding: 6px 10px; text-align: left; font-size: 10px; font-weight: 700;
  text-transform: uppercase; letter-spacing: 0.05em;
  color: var(--text-tertiary, #64748b); border-bottom: 1px solid rgba(51,65,85,0.4);
}
.inner-table td { padding: 6px 10px; color: var(--text-primary, #e2e8f0); border-bottom: 1px solid rgba(51,65,85,0.2); }
.run-status { font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 9999px; }
.rs-ok { background: rgba(34,197,94,0.12); color: #4ade80; }
.rs-fail { background: rgba(239,68,68,0.12); color: #f87171; }
.run-result { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

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
  border-radius: 12px; padding: 24px; min-width: 400px; max-width: 560px;
  box-shadow: 0 16px 64px rgba(0,0,0,0.5); animation: slideUp 0.2s ease-out;
}
.modal-wide { min-width: 480px; }
@keyframes slideUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
.modal-header { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.modal-icon { font-size: 1.4rem; }
.modal-title { font-size: 16px; font-weight: 700; color: var(--text-primary, #e2e8f0); }
.modal-body { margin-bottom: 20px; }
.modal-footer { display: flex; justify-content: flex-end; gap: 8px; }

/* Form */
.form-group { margin-bottom: 16px; }
.form-label { display: block; font-size: 12px; font-weight: 600; color: var(--text-secondary, #94a3b8); margin-bottom: 6px; }
.form-input {
  width: 100%; padding: 8px 12px; background: var(--bg-elevated, #1e293b);
  border: 1px solid var(--border-subtle, #334155); border-radius: 8px;
  color: var(--text-primary, #e2e8f0); font-size: 13px; outline: none;
  font-family: inherit; transition: border-color 0.15s; box-sizing: border-box;
}
.form-input:focus { border-color: #6366f1; }
.form-input.font-mono { font-family: 'SF Mono', Menlo, monospace; }
.form-textarea {
  width: 100%; padding: 8px 12px; background: var(--bg-elevated, #1e293b);
  border: 1px solid var(--border-subtle, #334155); border-radius: 8px;
  color: var(--text-primary, #e2e8f0); font-size: 13px; outline: none;
  font-family: inherit; transition: border-color 0.15s; box-sizing: border-box;
  resize: vertical; min-height: 80px;
}
.form-textarea:focus { border-color: #6366f1; }
.radio-group { display: flex; gap: 16px; flex-wrap: wrap; }
.radio-item { display: flex; align-items: center; gap: 6px; cursor: pointer; font-size: 13px; color: var(--text-primary, #e2e8f0); }
.radio-item input[type="radio"] { accent-color: #6366f1; }
.toggle-inline { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--text-primary, #e2e8f0); cursor: pointer; }
.toggle-inline input[type="checkbox"] { accent-color: #6366f1; width: 16px; height: 16px; }
</style>