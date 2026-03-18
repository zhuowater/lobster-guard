<template>
  <div>
    <!-- Breadcrumb -->
    <div class="breadcrumb">
      <a class="breadcrumb-link" @click="$router.push('/sessions')">
        <Icon name="arrow-left" :size="14" /> 会话回放
      </a>
      <span class="breadcrumb-sep">›</span>
      <span class="breadcrumb-current mono">{{ traceId }}</span>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading-state"><Icon name="loader" :size="20" /> 加载中...</div>

    <!-- Not Found -->
    <div v-else-if="!timeline" class="empty-state">
      <div class="empty-icon">🔍</div>
      <div class="empty-title">会话不存在</div>
      <div class="empty-desc">未找到 trace_id: {{ traceId }}</div>
    </div>

    <template v-else>
      <!-- Summary Header -->
      <div class="card summary-card">
        <div class="summary-top">
          <div class="summary-left">
            <div class="summary-trace mono">{{ timeline.summary.trace_id }}</div>
            <div class="summary-meta">
              <span v-if="timeline.summary.sender_id">👤 {{ timeline.summary.sender_id }}</span>
              <span v-if="timeline.summary.model">🧠 {{ timeline.summary.model }}</span>
              <span>⏱ {{ fmtDuration(timeline.summary.duration_ms) }}</span>
            </div>
          </div>
          <div class="summary-right">
            <span class="risk-badge" :class="'badge-' + timeline.summary.risk_level">
              {{ riskLabel(timeline.summary.risk_level) }}
            </span>
          </div>
        </div>
        <div class="summary-stats">
          <div class="stat-pill"><span class="sp-label">IM</span><span class="sp-value">{{ timeline.summary.im_events }}</span></div>
          <div class="stat-pill"><span class="sp-label">LLM</span><span class="sp-value">{{ timeline.summary.llm_calls }}</span></div>
          <div class="stat-pill"><span class="sp-label">Tools</span><span class="sp-value">{{ timeline.summary.tool_calls }}</span></div>
          <div class="stat-pill"><span class="sp-label">Tokens</span><span class="sp-value">{{ fmtNum(timeline.summary.total_tokens) }}</span></div>
          <div class="stat-pill warn" v-if="timeline.summary.canary_leaked"><span class="sp-value">🔴 Canary Leaked</span></div>
          <div class="stat-pill warn" v-if="timeline.summary.budget_exceeded"><span class="sp-value">⚠️ Budget Exceeded</span></div>
          <div class="stat-pill warn" v-if="timeline.summary.blocked"><span class="sp-value">🚫 Blocked</span></div>
        </div>
      </div>

      <!-- Timeline -->
      <div class="card timeline-card">
        <div class="card-header">
          <span class="card-icon"><Icon name="play" :size="18" /></span>
          <span class="card-title">事件时间线 ({{ timeline.events.length }} 个事件)</span>
        </div>

        <div class="timeline">
          <div
            v-for="(ev, idx) in timeline.events" :key="idx"
            class="tl-item"
            :class="[
              'tl-' + ev.type,
              { 'tl-flagged': ev.flagged, 'tl-canary': ev.canary_leaked, 'tl-blocked': ev.action === 'block' }
            ]"
          >
            <!-- Node -->
            <div class="tl-node">
              <div class="tl-dot" :class="'dot-' + ev.type + (ev.flagged ? ' dot-flagged' : '') + (ev.action === 'block' ? ' dot-block' : '')">
                <span v-if="ev.type === 'im_inbound' || ev.type === 'im_outbound'">●</span>
                <span v-else-if="ev.type === 'llm_call'">◆</span>
                <span v-else-if="ev.type === 'tool_call'">▸</span>
                <span v-else-if="ev.type === 'tag'">💬</span>
              </div>
              <div class="tl-line" v-if="idx < timeline.events.length - 1"></div>
            </div>

            <!-- Content -->
            <div class="tl-content">
              <div class="tl-header">
                <span class="tl-time mono">{{ fmtTimeFull(ev.timestamp) }}</span>
                <span class="tl-type-label">{{ typeLabel(ev) }}</span>
              </div>

              <!-- IM Event -->
              <template v-if="ev.type === 'im_inbound' || ev.type === 'im_outbound'">
                <div class="tl-detail">
                  <span class="action-tag" :class="'at-' + ev.action">{{ ev.action }}</span>
                  <span v-if="ev.sender_id" class="tl-sender">{{ ev.sender_id }}</span>
                  <span v-if="ev.reason" class="tl-reason">{{ ev.reason }}</span>
                </div>
                <div class="tl-body" v-if="ev.content">
                  <div class="tl-content-text">"{{ ev.content }}"</div>
                </div>
              </template>

              <!-- LLM Call -->
              <template v-else-if="ev.type === 'llm_call'">
                <div class="tl-detail">
                  <span v-if="ev.model" class="tl-model">{{ ev.model }}</span>
                  <span class="tl-metric">tokens: {{ fmtNum(ev.tokens) }}</span>
                  <span class="tl-metric">latency: {{ Math.round(ev.latency_ms) }}ms</span>
                  <span v-if="ev.status_code && ev.status_code >= 400" class="tl-error">HTTP {{ ev.status_code }}</span>
                </div>
                <div class="tl-alert canary-alert" v-if="ev.canary_leaked">
                  🔴 Canary Token 已泄露！Prompt 注入检测触发
                </div>
                <div class="tl-alert budget-alert" v-if="ev.budget_exceeded">
                  ⚠️ 响应预算超限
                </div>
                <div class="tl-alert error-alert" v-if="ev.error_message">
                  ❌ {{ ev.error_message }}
                </div>
              </template>

              <!-- Tool Call -->
              <template v-else-if="ev.type === 'tool_call'">
                <div class="tl-detail">
                  <span class="tool-name">{{ ev.tool_name }}</span>
                  <span class="tool-risk" :class="'tr-' + ev.risk_level">{{ ev.risk_level }}</span>
                  <span class="tool-flagged-badge" v-if="ev.flagged">⚠️ flagged</span>
                </div>
                <div class="tl-code" v-if="ev.tool_input">
                  <div class="code-label">Input:</div>
                  <pre class="code-block">{{ ev.tool_input }}</pre>
                </div>
                <div class="tl-code" v-if="ev.tool_result">
                  <div class="code-label">Result:</div>
                  <pre class="code-block">{{ ev.tool_result }}</pre>
                </div>
                <div class="tl-flag-reason" v-if="ev.flag_reason">
                  🚩 {{ ev.flag_reason }}
                </div>
              </template>

              <!-- Tag -->
              <template v-else-if="ev.type === 'tag'">
                <div class="tag-bubble">
                  <span class="tag-text">{{ ev.tag_text }}</span>
                  <span class="tag-author" v-if="ev.tag_author">— {{ ev.tag_author }}</span>
                  <button class="tag-delete" @click.stop="deleteTag(ev.tag_id)" title="删除标签">×</button>
                </div>
              </template>

              <!-- Add Tag Button -->
              <div class="tl-actions" v-if="ev.type !== 'tag'">
                <button class="add-tag-btn" v-if="!ev._showTagInput" @click="ev._showTagInput = true">
                  <Icon name="tag" :size="12" /> + 标签
                </button>
                <div class="tag-input-row" v-if="ev._showTagInput">
                  <input
                    v-model="ev._tagText"
                    placeholder="输入标签内容..."
                    @keyup.enter="submitTag(ev)"
                    class="tag-input"
                  />
                  <button class="btn btn-sm" @click="submitTag(ev)">添加</button>
                  <button class="btn btn-ghost btn-sm" @click="ev._showTagInput = false">取消</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, apiPost, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'
import Icon from '../components/Icon.vue'

const route = useRoute()
const router = useRouter()
const traceId = route.params.traceId
const loading = ref(true)
const timeline = ref(null)

function fmtTimeFull(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  const h = String(d.getHours()).padStart(2, '0')
  const m = String(d.getMinutes()).padStart(2, '0')
  const s = String(d.getSeconds()).padStart(2, '0')
  const ms = String(d.getMilliseconds()).padStart(3, '0')
  return `${h}:${m}:${s}.${ms}`
}

function fmtDuration(ms) {
  if (!ms || ms <= 0) return '--'
  if (ms < 1000) return Math.round(ms) + 'ms'
  if (ms < 60000) return (ms / 1000).toFixed(1) + 's'
  const min = Math.floor(ms / 60000)
  const sec = Math.floor((ms % 60000) / 1000)
  return min + 'm ' + sec + 's'
}

function fmtNum(n) {
  if (!n) return '0'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

function riskLabel(level) {
  const map = { critical: '🔴 严重', high: '🟠 高危', medium: '🟡 中等', low: '🟢 低风险' }
  return map[level] || level
}

function typeLabel(ev) {
  const map = {
    im_inbound: 'IM 入站',
    im_outbound: 'IM 出站',
    llm_call: 'LLM 调用',
    tool_call: '工具调用',
    tag: '标签'
  }
  return map[ev.type] || ev.type
}

async function loadTimeline() {
  loading.value = true
  try {
    const d = await api('/api/v1/sessions/replay/' + encodeURIComponent(traceId))
    // Add reactive properties for tag input
    if (d.events) {
      d.events.forEach(ev => {
        ev._showTagInput = false
        ev._tagText = ''
      })
    }
    timeline.value = d
  } catch {
    timeline.value = null
  }
  loading.value = false
}

async function submitTag(ev) {
  if (!ev._tagText) return
  try {
    await apiPost('/api/v1/sessions/replay/' + encodeURIComponent(traceId) + '/tags', {
      text: ev._tagText,
      event_type: ev.type,
      event_id: ev.id || 0,
      author: 'admin'
    })
    ev._showTagInput = false
    ev._tagText = ''
    showToast('标签已添加', 'success')
    loadTimeline()
  } catch (e) {
    showToast('添加失败: ' + e.message, 'error')
  }
}

async function deleteTag(tagId) {
  if (!tagId) return
  try {
    await apiDelete('/api/v1/sessions/replay/tags/' + tagId)
    showToast('标签已删除', 'success')
    loadTimeline()
  } catch (e) {
    showToast('删除失败: ' + e.message, 'error')
  }
}

onMounted(loadTimeline)
</script>

<style scoped>
.breadcrumb {
  display: flex; align-items: center; gap: 8px; margin-bottom: 16px;
  font-size: var(--text-sm);
}
.breadcrumb-link {
  display: flex; align-items: center; gap: 4px;
  color: var(--color-primary); cursor: pointer; text-decoration: none;
}
.breadcrumb-link:hover { text-decoration: underline; }
.breadcrumb-sep { color: var(--text-tertiary); }
.breadcrumb-current { color: var(--text-secondary); }
.mono { font-family: var(--font-mono); }

.loading-state {
  display: flex; align-items: center; justify-content: center; gap: 8px;
  padding: var(--space-8); color: var(--text-tertiary);
}
.empty-state { text-align: center; padding: var(--space-8); }
.empty-icon { font-size: 3rem; margin-bottom: var(--space-2); }
.empty-title { font-size: var(--text-lg); font-weight: 600; color: var(--text-primary); margin-bottom: var(--space-1); }
.empty-desc { font-size: var(--text-sm); color: var(--text-tertiary); }

/* Summary Card */
.summary-card { margin-bottom: var(--space-4); }
.summary-top { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: var(--space-3); }
.summary-trace { font-size: var(--text-lg); font-weight: 700; color: var(--color-primary); }
.summary-meta { display: flex; gap: var(--space-3); font-size: var(--text-sm); color: var(--text-secondary); margin-top: var(--space-1); flex-wrap: wrap; }
.summary-stats { display: flex; gap: var(--space-2); flex-wrap: wrap; }
.stat-pill {
  display: flex; align-items: center; gap: 4px;
  background: var(--bg-elevated); border: 1px solid var(--border-subtle);
  border-radius: 20px; padding: 4px 12px; font-size: var(--text-xs);
}
.stat-pill.warn { background: rgba(239,68,68,0.1); border-color: rgba(239,68,68,0.3); }
.sp-label { color: var(--text-tertiary); text-transform: uppercase; font-weight: 500; letter-spacing: 0.05em; }
.sp-value { color: var(--text-primary); font-weight: 600; font-family: var(--font-mono); }

.risk-badge {
  font-size: 12px; font-weight: 700; padding: 4px 12px; border-radius: 16px;
}
.badge-critical { background: rgba(239,68,68,0.15); color: #EF4444; }
.badge-high { background: rgba(249,115,22,0.15); color: #F97316; }
.badge-medium { background: rgba(234,179,8,0.15); color: #EAB308; }
.badge-low { background: rgba(34,197,94,0.15); color: #22C55E; }

/* Timeline */
.timeline-card { position: relative; }
.timeline { padding: var(--space-2) 0; }

.tl-item {
  display: flex; gap: var(--space-4); min-height: 60px;
}
.tl-item.tl-flagged .tl-content { border-left: 3px solid #EF4444; padding-left: 12px; }
.tl-item.tl-canary .tl-content { background: rgba(239,68,68,0.08); border-radius: var(--radius-md); }
.tl-item.tl-blocked .tl-content { background: rgba(239,68,68,0.05); }

.tl-node {
  display: flex; flex-direction: column; align-items: center; width: 28px; flex-shrink: 0;
}
.tl-dot {
  width: 28px; height: 28px; border-radius: 50%; display: flex;
  align-items: center; justify-content: center; font-size: 14px;
  flex-shrink: 0; z-index: 1;
}
.dot-im_inbound, .dot-im_outbound { background: rgba(59,130,246,0.15); color: #3B82F6; }
.dot-im_inbound.dot-block, .dot-im_outbound.dot-block { background: rgba(239,68,68,0.15); color: #EF4444; }
.dot-llm_call { background: rgba(168,85,247,0.15); color: #A855F7; }
.dot-tool_call { background: rgba(34,197,94,0.15); color: #22C55E; }
.dot-tool_call.dot-flagged { background: rgba(239,68,68,0.15); color: #EF4444; animation: pulse 2s infinite; }
.dot-tag { background: rgba(255,255,255,0.08); }

@keyframes pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(239,68,68,0.4); }
  50% { box-shadow: 0 0 0 8px rgba(239,68,68,0); }
}

.tl-line {
  width: 2px; flex: 1; background: var(--border-subtle); margin: 4px 0;
}

.tl-content {
  flex: 1; padding-bottom: var(--space-4); padding: var(--space-2) var(--space-3);
  margin-bottom: var(--space-2);
}

.tl-header {
  display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-1);
}
.tl-time { font-size: 11px; color: var(--text-tertiary); }
.tl-type-label { font-size: var(--text-xs); font-weight: 600; color: var(--text-secondary); }

.tl-detail {
  display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap;
  margin-bottom: var(--space-1); font-size: var(--text-sm);
}

.action-tag {
  font-size: 11px; font-weight: 700; padding: 1px 8px; border-radius: 8px;
  text-transform: uppercase; letter-spacing: 0.05em;
}
.at-pass { background: rgba(34,197,94,0.15); color: #22C55E; }
.at-block { background: rgba(239,68,68,0.15); color: #EF4444; }
.at-warn { background: rgba(234,179,8,0.15); color: #EAB308; }

.tl-sender { color: var(--color-primary); font-weight: 500; }
.tl-reason { color: var(--text-tertiary); font-style: italic; }
.tl-model { color: #A855F7; font-weight: 600; font-family: var(--font-mono); font-size: var(--text-xs); }
.tl-metric { color: var(--text-tertiary); font-family: var(--font-mono); font-size: var(--text-xs); }
.tl-error { color: #EF4444; font-weight: 600; }

.tl-body { margin: var(--space-1) 0; }
.tl-content-text {
  font-size: var(--text-sm); color: var(--text-primary); font-style: italic;
  background: var(--bg-elevated); padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md); border-left: 3px solid var(--color-primary);
}

.tl-alert {
  font-size: var(--text-sm); font-weight: 600; padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md); margin-top: var(--space-1);
}
.canary-alert { background: rgba(239,68,68,0.15); color: #EF4444; border: 1px solid rgba(239,68,68,0.3); }
.budget-alert { background: rgba(249,115,22,0.15); color: #F97316; border: 1px solid rgba(249,115,22,0.3); }
.error-alert { background: rgba(239,68,68,0.1); color: #EF4444; }

.tool-name { font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); }
.tool-risk { font-size: 11px; font-weight: 600; padding: 1px 6px; border-radius: 8px; }
.tr-low { background: rgba(34,197,94,0.15); color: #22C55E; }
.tr-medium { background: rgba(234,179,8,0.15); color: #EAB308; }
.tr-high { background: rgba(249,115,22,0.15); color: #F97316; }
.tr-critical { background: rgba(239,68,68,0.15); color: #EF4444; }
.tool-flagged-badge { font-size: 11px; color: #EF4444; font-weight: 700; }

.tl-code { margin: var(--space-1) 0; }
.code-label { font-size: 10px; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 2px; }
.code-block {
  background: var(--bg-base); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm); padding: var(--space-2);
  font-family: var(--font-mono); font-size: 12px; color: var(--text-secondary);
  overflow-x: auto; max-height: 120px; white-space: pre-wrap; word-break: break-all;
  margin: 0;
}
.tl-flag-reason {
  font-size: var(--text-xs); color: #EF4444; font-weight: 600;
  margin-top: var(--space-1);
}

/* Tags */
.tag-bubble {
  display: inline-flex; align-items: center; gap: var(--space-2);
  background: rgba(255,255,255,0.06); padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md); border: 1px dashed var(--border-subtle);
}
.tag-text { font-size: var(--text-sm); color: var(--text-primary); }
.tag-author { font-size: var(--text-xs); color: var(--text-tertiary); }
.tag-delete {
  background: none; border: none; color: var(--text-tertiary); cursor: pointer;
  font-size: 16px; line-height: 1; padding: 0 2px;
}
.tag-delete:hover { color: #EF4444; }

/* Add tag */
.tl-actions { margin-top: var(--space-1); }
.add-tag-btn {
  display: inline-flex; align-items: center; gap: 4px;
  background: none; border: 1px dashed var(--border-subtle); border-radius: var(--radius-sm);
  color: var(--text-tertiary); font-size: 11px; padding: 2px 8px; cursor: pointer;
  transition: all var(--transition-fast);
}
.add-tag-btn:hover { color: var(--color-primary); border-color: var(--color-primary); }

.tag-input-row {
  display: flex; align-items: center; gap: var(--space-2); margin-top: var(--space-1);
}
.tag-input {
  flex: 1; max-width: 300px;
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-sm); color: var(--text-primary);
  padding: var(--space-1) var(--space-2); font-size: var(--text-xs);
}
.tag-input:focus { border-color: var(--color-primary); outline: none; }
</style>
