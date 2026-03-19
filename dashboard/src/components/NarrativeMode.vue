<template>
  <div class="narrative">
    <header class="narrative-status">
      <div class="status-left">
        <span class="brand">🦞 龙虾卫士</span>
        <button class="mode-switch-btn" @click="switchBack">📋 经典模式</button>
      </div>
      <div class="status-center">
        <Transition name="status-fade" mode="out-in">
          <div v-if="statusLevel === 'safe'" key="safe" class="status-text safe breathing">
            ✅ 一切平静。过去 {{ quietHours }}h 无安全事件。
          </div>
          <div v-else-if="statusLevel === 'warn'" key="warn" class="status-text warn">
            ⚠️ {{ warnCount }} 个事件需要关注
          </div>
          <div v-else key="danger" class="status-text danger">
            🔴 {{ blockCount }} 个攻击已拦截，{{ passReviewCount }} 个放行待审查
          </div>
        </Transition>
      </div>
      <div class="status-right">
        <span class="clock">{{ currentTime }}</span>
        <span class="online-dot" :class="connected ? 'on' : 'off'" :title="connected ? '在线' : '离线'"></span>
      </div>
    </header>
    <main class="narrative-timeline">
      <div class="timeline-controls">
        <button class="review-btn" :class="{ active: reviewMode }" @click="reviewMode = !reviewMode">
          {{ reviewMode ? '🔙 回到默认' : '🔍 审查放行' }}
        </button>
        <span class="event-count">{{ events.length }} 条事件</span>
        <span class="auto-refresh-hint">每 10s 自动刷新</span>
      </div>
      <div class="timeline-list" ref="timelineRef">
        <TransitionGroup name="slide">
          <div v-for="event in sortedEvents" :key="event.id" class="timeline-card"
               :class="[actionClass(event), { expanded: event._expanded, 'review-highlight': reviewMode && event._action === 'pass' }]"
               @click="event._expanded = !event._expanded">
            <div class="card-row">
              <span class="ev-time">{{ fmtTime(event.timestamp || event.time) }}</span>
              <span class="ev-direction">{{ dirIcon(event.direction) }}</span>
              <span class="ev-badge" :class="event._action">{{ badgeText(event._action) }}</span>
              <span class="ev-summary">{{ eventSummary(event) }}</span>
              <span class="ev-expand">{{ event._expanded ? '▾' : '▸' }}</span>
            </div>
            <div v-if="reviewMode && event._action === 'pass' && !event._expanded" class="review-reason">
              <span class="reason-tag" v-if="event.matched_rules">📏 规则匹配: {{ event.matched_rules }}</span>
              <span class="reason-tag" v-if="event.semantic_score != null">🧠 语义分: {{ event.semantic_score }}</span>
              <span class="reason-tag" v-if="event.user_risk_score != null">👤 用户画像: {{ event.user_risk_score }}</span>
              <span class="reason-tag" v-if="!event.matched_rules && event.semantic_score == null && event.user_risk_score == null">✅ 默认放行</span>
            </div>
            <Transition name="detail-expand">
              <div v-if="event._expanded" class="card-detail">
                <div class="detail-section" v-if="event.content || event.request_body">
                  <div class="detail-label">请求内容</div>
                  <pre class="detail-pre">{{ event.content || event.request_body || '--' }}</pre>
                </div>
                <div class="detail-section" v-if="event.response_body">
                  <div class="detail-label">响应内容</div>
                  <pre class="detail-pre">{{ event.response_body }}</pre>
                </div>
                <div class="detail-section">
                  <div class="detail-label">检测链路</div>
                  <div class="detection-chain">
                    <span class="chain-step" :class="stepStatus(event, 'keyword')">关键词{{ stepResult(event, 'keyword') }}</span>
                    <span class="chain-arrow">→</span>
                    <span class="chain-step" :class="stepStatus(event, 'regex')">正则{{ stepResult(event, 'regex') }}</span>
                    <span class="chain-arrow">→</span>
                    <span class="chain-step" :class="stepStatus(event, 'pii')">PII{{ stepResult(event, 'pii') }}</span>
                    <span class="chain-arrow">→</span>
                    <span class="chain-step" :class="stepStatus(event, 'semantic')">语义{{ stepResult(event, 'semantic') }}</span>
                  </div>
                </div>
                <div class="detail-section" v-if="event.envelope_status">
                  <div class="detail-label">信封验证</div>
                  <span class="envelope-badge" :class="event.envelope_status">{{ event.envelope_status }}</span>
                </div>
                <div class="detail-meta">
                  <span v-if="event.reason" class="meta-item">原因: {{ event.reason }}</span>
                  <span v-if="event.sender_id" class="meta-item">发送者: {{ event.sender_id }}</span>
                  <span v-if="event.rule_id" class="meta-item">规则: {{ event.rule_id }}</span>
                  <span v-if="event.trace_id" class="meta-item trace-link" @click.stop="goTrace(event.trace_id)">
                    TraceID: {{ event.trace_id.substring(0, 12) }}… →
                  </span>
                </div>
              </div>
            </Transition>
          </div>
        </TransitionGroup>
        <div v-if="events.length === 0 && loaded" class="timeline-empty">
          <div class="empty-icon">🌊</div>
          <div class="empty-text">风平浪静，无安全事件</div>
        </div>
        <div v-if="!loaded" class="timeline-loading">
          <div class="loading-pulse"></div>
          <div class="loading-text">加载事件流...</div>
        </div>
      </div>
    </main>
    <footer class="narrative-footer">
      <div class="footer-stat" @click="goClassic('/audit')">
        <div class="stat-number"><AnimNum :value="footerStats.total" /></div>
        <div class="stat-label">24h 总流量</div>
      </div>
      <div class="footer-divider"></div>
      <div class="footer-stat" @click="goClassic('/audit?action=block')">
        <div class="stat-number red"><AnimNum :value="footerStats.blocked" /></div>
        <div class="stat-label">拦截数</div>
      </div>
      <div class="footer-divider"></div>
      <div class="footer-stat" @click="goClassic('/audit?action=warn')">
        <div class="stat-number orange"><AnimNum :value="footerStats.warned" /></div>
        <div class="stat-label">告警数</div>
      </div>
      <div class="footer-divider"></div>
      <div class="footer-stat" @click="goClassic('/audit?action=pass')">
        <div class="stat-number green"><AnimNum :value="footerStats.passReview" /></div>
        <div class="stat-label">放行待审查</div>
      </div>
      <div class="footer-divider"></div>
      <div class="footer-stat" @click="goClassic('/overview')">
        <div class="stat-number" :class="scoreClass"><AnimNum :value="footerStats.score" /></div>
        <div class="stat-label">安全分</div>
      </div>
    </footer>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, watch, h, defineComponent } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import { navStore } from '../stores/navigation.js'

const router = useRouter()

/* ---- Animated Number inline component ---- */
const AnimNum = defineComponent({
  props: { value: { type: Number, default: 0 }, duration: { type: Number, default: 600 } },
  setup(props) {
    const displayed = ref(props.value)
    let raf = null, prev = props.value
    function animate(from, to) {
      const start = performance.now(), diff = to - from
      function step(ts) {
        const p = Math.min((ts - start) / props.duration, 1)
        displayed.value = Math.round(from + diff * p)
        if (p < 1) raf = requestAnimationFrame(step)
      }
      cancelAnimationFrame(raf)
      raf = requestAnimationFrame(step)
    }
    let iv = null
    onMounted(() => {
      iv = setInterval(() => {
        if (props.value !== prev) { animate(prev, props.value); prev = props.value }
      }, 250)
    })
    onUnmounted(() => { cancelAnimationFrame(raf); clearInterval(iv) })
    return () => h('span', null, String(displayed.value))
  }
})

/* ---- State ---- */
const events = ref([])
const loaded = ref(false)
const reviewMode = ref(false)
const connected = ref(true)
const currentTime = ref('')

const footerStats = reactive({ total: 0, blocked: 0, warned: 0, passReview: 0, score: 0 })

/* ---- Computed ---- */
const blockCount = computed(() => events.value.filter(e => e._action === 'block').length)
const warnCount = computed(() => events.value.filter(e => e._action === 'warn').length)
const passReviewCount = computed(() => events.value.filter(e => e._action === 'pass').length)

const statusLevel = computed(() => {
  if (blockCount.value > 0) return 'danger'
  if (warnCount.value > 0) return 'warn'
  return 'safe'
})

const quietHours = computed(() => {
  const threats = events.value.filter(e => e._action === 'block' || e._action === 'warn')
  if (!threats.length) return 24
  const latest = new Date(threats[0].timestamp || threats[0].time)
  if (isNaN(latest.getTime())) return 24
  return Math.max(0, Math.floor((Date.now() - latest.getTime()) / 3600000))
})

const scoreClass = computed(() => {
  const s = footerStats.score
  return s >= 80 ? 'green' : s >= 60 ? 'orange' : 'red'
})

const sortedEvents = computed(() => {
  const arr = [...events.value]
  arr.sort((a, b) => {
    const order = { block: 0, warn: 1, info: 2, pass: 3 }
    const oa = order[a._action] ?? 2, ob = order[b._action] ?? 2
    if (oa !== ob) return oa - ob
    return new Date(b.timestamp || b.time || 0) - new Date(a.timestamp || a.time || 0)
  })
  return arr
})

/* ---- Helpers ---- */
function switchBack() { navStore.setMode('classic') }

function fmtTime(ts) {
  if (!ts) return '--:--'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return String(ts).substring(11, 16) || '--:--'
  return d.toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit' })
}

function dirIcon(dir) {
  const m = { inbound: '←入站', outbound: '→出站', tool: '🔧工具', llm: '🧠LLM' }
  return m[dir] || '📡'
}

function normalizeAction(ev) {
  const a = (ev.action || '').toLowerCase()
  if (a.includes('block')) return 'block'
  if (a.includes('warn')) return 'warn'
  if (a.includes('pass') || a.includes('allow')) return 'pass'
  return 'info'
}

function actionClass(ev) { return 'action-' + ev._action }

function badgeText(action) {
  return { block: '🔴拦截', warn: '🟡告警', pass: '🟢放行', info: '🔵信息' }[action] || '🔵信息'
}

function eventSummary(ev) {
  const parts = []
  const c = ev.content || ev.request_body || ''
  if (c) parts.push('"' + (c.length > 60 ? c.substring(0, 60) + '…' : c) + '"')
  if (ev.reason) parts.push('→ ' + ev.reason)
  if (ev.sender_id) parts.push('from ' + ev.sender_id)
  if (ev.semantic_score != null) parts.push('score=' + ev.semantic_score)
  return parts.join(' ') || ev.reason || ev.rule_id || '(无摘要)'
}

function stepStatus(ev, step) {
  const v = (ev.checks || ev.detection_chain || {})[step]
  if (v === true || v === 'hit' || v === 'blocked') return 'hit'
  if (v === false || v === 'pass' || v === 'clean') return 'clean'
  return 'na'
}

function stepResult(ev, step) {
  const v = (ev.checks || ev.detection_chain || {})[step]
  if (v === true || v === 'hit' || v === 'blocked') return ' ✗'
  if (v === false || v === 'pass' || v === 'clean') return ' ✓'
  return ''
}

function goTrace(tid) { navStore.setMode('classic'); router.push('/sessions/' + tid) }
function goClassic(path) { navStore.setMode('classic'); router.push(path) }

function updateClock() {
  currentTime.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
}

/* ---- Data ---- */
async function loadEvents() {
  try {
    const d = await api('/api/v1/audit/logs?limit=50')
    const logs = d.logs || []
    const expandSet = new Set(events.value.filter(e => e._expanded).map(e => e.id))
    events.value = logs.map((log, i) => {
      const id = log.id || log.trace_id || ('ev-' + i + '-' + (log.timestamp || ''))
      return { ...log, id, _action: normalizeAction(log), _expanded: expandSet.has(id) }
    })
    loaded.value = true; connected.value = true
  } catch { connected.value = false; loaded.value = true }
}

async function loadStats() {
  try {
    const d = await api('/api/v1/stats?since=24h')
    const total = d.total || 0, bk = d.breakdown || {}
    let blocked = 0, warned = 0
    for (const k of Object.keys(bk)) {
      if (k.includes('block')) blocked += bk[k]
      if (k.includes('warn')) warned += bk[k]
    }
    Object.assign(footerStats, { total, blocked, warned, passReview: Math.max(0, total - blocked - warned) })
  } catch {}
}

async function loadScore() {
  try { footerStats.score = (await api('/api/v1/health/score')).score || 0 } catch { footerStats.score = 0 }
}

async function loadAll() { await Promise.all([loadEvents(), loadStats(), loadScore()]) }

/* ---- Lifecycle ---- */
let refreshTimer = null, clockTimer = null
onMounted(() => { updateClock(); clockTimer = setInterval(updateClock, 1000); loadAll(); refreshTimer = setInterval(loadAll, 10000) })
onUnmounted(() => { clearInterval(refreshTimer); clearInterval(clockTimer) })
</script>

<style scoped>
.narrative {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: #0a0a0f; color: #c8ccd4;
  display: flex; flex-direction: column; z-index: 500;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  overflow: hidden;
}

/* ====== Status Bar ====== */
.narrative-status {
  height: 48px; min-height: 48px;
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 20px;
  background: rgba(10, 10, 15, 0.95);
  border-bottom: 1px solid rgba(255,255,255,0.06);
  backdrop-filter: blur(8px);
}
.status-left { display: flex; align-items: center; gap: 12px; flex-shrink: 0; }
.brand { font-size: 14px; font-weight: 700; color: #e2e8f0; }
.mode-switch-btn {
  background: rgba(99,102,241,0.12); border: 1px solid rgba(99,102,241,0.25);
  color: #a5b4fc; padding: 4px 12px; border-radius: 6px;
  font-size: 12px; font-weight: 500; cursor: pointer; transition: all .2s; font-family: inherit;
}
.mode-switch-btn:hover { background: rgba(99,102,241,0.25); border-color: rgba(99,102,241,0.5); color: #c7d2fe; }
.status-center { flex: 1; text-align: center; min-width: 0; }
.status-text { font-size: 15px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.status-text.safe { color: #34d399; }
.status-text.warn { color: #fbbf24; }
.status-text.danger { color: #f87171; }
.breathing { animation: breathe 4s ease-in-out infinite; }
@keyframes breathe { 0%,100%{ opacity:.7; } 50%{ opacity:1; } }
.status-fade-enter-active { transition: opacity .5s ease, transform .5s ease; }
.status-fade-leave-active { transition: opacity .3s ease, transform .3s ease; }
.status-fade-enter-from { opacity: 0; transform: translateY(-4px); }
.status-fade-leave-to { opacity: 0; transform: translateY(4px); }
.status-right { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.clock { font-family: 'SF Mono','Fira Code','Cascadia Code','Menlo',monospace; font-size: 13px; color: #64748b; letter-spacing: .05em; }
.online-dot { width: 8px; height: 8px; border-radius: 50%; }
.online-dot.on { background: #34d399; box-shadow: 0 0 6px rgba(52,211,153,.5); }
.online-dot.off { background: #ef4444; box-shadow: 0 0 6px rgba(239,68,68,.5); }

/* ====== Timeline ====== */
.narrative-timeline { flex: 1; display: flex; flex-direction: column; min-height: 0; padding: 12px 20px 0; }
.timeline-controls { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; flex-shrink: 0; }
.review-btn {
  background: rgba(255,255,255,.05); border: 1px solid rgba(255,255,255,.1);
  color: #94a3b8; padding: 5px 14px; border-radius: 6px;
  font-size: 12px; font-weight: 500; cursor: pointer; transition: all .2s; font-family: inherit;
}
.review-btn:hover { background: rgba(255,255,255,.08); color: #e2e8f0; }
.review-btn.active { background: rgba(52,211,153,.12); border-color: rgba(52,211,153,.3); color: #34d399; }
.event-count { font-size: 12px; color: #64748b; font-family: 'SF Mono','Fira Code','Cascadia Code','Menlo',monospace; }
.auto-refresh-hint { font-size: 11px; color: #475569; margin-left: auto; }

.timeline-list { flex: 1; overflow-y: auto; overflow-x: hidden; padding-bottom: 8px; }
.timeline-list::-webkit-scrollbar { width: 4px; }
.timeline-list::-webkit-scrollbar-track { background: transparent; }
.timeline-list::-webkit-scrollbar-thumb { background: rgba(255,255,255,.08); border-radius: 2px; }

/* Cards */
.timeline-card {
  background: rgba(255,255,255,.02); border: 1px solid rgba(255,255,255,.04);
  border-radius: 6px; margin-bottom: 4px; padding: 8px 14px;
  cursor: pointer; transition: all .15s ease;
  font-family: 'SF Mono','Fira Code','Cascadia Code','Menlo',monospace; font-size: 13px;
}
.timeline-card:hover { background: rgba(255,255,255,.04); border-color: rgba(255,255,255,.08); }
.timeline-card.action-block { border-left: 3px solid #ef4444; background: rgba(239,68,68,.04); }
.timeline-card.action-warn { border-left: 3px solid #f59e0b; background: rgba(245,158,11,.03); }
.timeline-card.action-pass { border-left: 3px solid rgba(52,211,153,.15); opacity: .55; }
.timeline-card.action-pass:hover { opacity: .75; }
.timeline-card.action-info { border-left: 3px solid rgba(59,130,246,.3); }
.timeline-card.review-highlight { opacity: 1 !important; border-left-color: #34d399; background: rgba(52,211,153,.06); }
.timeline-card.review-highlight .ev-badge { color: #34d399; font-weight: 700; }

.card-row { display: flex; align-items: center; gap: 12px; min-height: 24px; }
.ev-time { color: #64748b; font-size: 12px; flex-shrink: 0; min-width: 40px; }
.ev-direction { font-size: 12px; color: #94a3b8; flex-shrink: 0; min-width: 48px; }
.ev-badge { font-size: 12px; flex-shrink: 0; min-width: 52px; }
.ev-badge.block { color: #f87171; }
.ev-badge.warn { color: #fbbf24; }
.ev-badge.pass { color: #6b7280; }
.ev-badge.info { color: #60a5fa; }
.ev-summary { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #94a3b8; font-size: 12px; }
.ev-expand { color: #475569; font-size: 11px; flex-shrink: 0; }

/* Slide transition for new events */
.slide-enter-active { animation: slideDown .3s ease-out; }
.slide-leave-active { animation: slideDown .2s ease-in reverse; }
.slide-move { transition: transform .3s ease; }
@keyframes slideDown { from { opacity: 0; transform: translateY(-16px); } to { opacity: 1; transform: translateY(0); } }

/* Review reason tags */
.review-reason { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; padding-top: 6px; border-top: 1px solid rgba(52,211,153,.1); }
.reason-tag { font-size: 11px; color: #6ee7b7; background: rgba(52,211,153,.08); padding: 2px 8px; border-radius: 4px; font-family: 'SF Mono','Fira Code','Cascadia Code','Menlo',monospace; }

/* Expand detail */
.detail-expand-enter-active { transition: all .25s ease-out; }
.detail-expand-leave-active { transition: all .15s ease-in; }
.detail-expand-enter-from, .detail-expand-leave-to { opacity: 0; max-height: 0; overflow: hidden; }
.card-detail { margin-top: 10px; padding-top: 10px; border-top: 1px solid rgba(255,255,255,.06); }
.detail-section { margin-bottom: 10px; }
.detail-label {
  font-size: 11px; color: #64748b; font-weight: 600; margin-bottom: 4px;
  text-transform: uppercase; letter-spacing: .05em;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
.detail-pre {
  background: rgba(0,0,0,.3); border: 1px solid rgba(255,255,255,.04);
  border-radius: 4px; padding: 8px 10px; font-size: 12px; color: #cbd5e1;
  white-space: pre-wrap; word-break: break-all; max-height: 200px; overflow-y: auto;
  margin: 0;
}
.detection-chain { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.chain-step {
  font-size: 11px; padding: 2px 8px; border-radius: 4px;
  font-family: 'SF Mono','Fira Code','Cascadia Code','Menlo',monospace;
  background: rgba(255,255,255,.04); color: #64748b;
}
.chain-step.hit { background: rgba(239,68,68,.12); color: #f87171; }
.chain-step.clean { background: rgba(52,211,153,.08); color: #6ee7b7; }
.chain-arrow { color: #475569; font-size: 11px; }
.envelope-badge {
  font-size: 11px; padding: 2px 8px; border-radius: 4px;
  font-family: 'SF Mono','Fira Code','Cascadia Code','Menlo',monospace;
}
.envelope-badge.valid, .envelope-badge.ok { background: rgba(52,211,153,.1); color: #6ee7b7; }
.envelope-badge.invalid, .envelope-badge.fail { background: rgba(239,68,68,.1); color: #f87171; }
.detail-meta { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 8px; }
.meta-item { font-size: 11px; color: #64748b; }
.trace-link { color: #818cf8; cursor: pointer; text-decoration: underline; text-underline-offset: 2px; }
.trace-link:hover { color: #a5b4fc; }

/* Empty / Loading */
.timeline-empty { text-align: center; padding: 60px 0; }
.empty-icon { font-size: 3rem; margin-bottom: 12px; opacity: .6; }
.empty-text { font-size: 14px; color: #475569; }
.timeline-loading { text-align: center; padding: 60px 0; }
.loading-pulse { width: 32px; height: 32px; border-radius: 50%; background: rgba(99,102,241,.2); margin: 0 auto 12px; animation: pulse 1.5s ease-in-out infinite; }
@keyframes pulse { 0%,100%{ transform:scale(.8); opacity:.5; } 50%{ transform:scale(1.2); opacity:1; } }
.loading-text { font-size: 13px; color: #475569; }

/* ====== Footer Stats ====== */
.narrative-footer {
  height: 64px; min-height: 64px;
  display: flex; align-items: center; justify-content: center; gap: 0;
  background: rgba(15,15,25,.85);
  border-top: 1px solid rgba(255,255,255,.06);
  backdrop-filter: blur(8px);
  padding: 0 20px;
}
.footer-stat { display: flex; flex-direction: column; align-items: center; cursor: pointer; padding: 4px 24px; transition: all .15s; }
.footer-stat:hover { background: rgba(255,255,255,.04); border-radius: 6px; }
.stat-number { font-size: 22px; font-weight: 700; color: #e2e8f0; font-family: 'SF Mono','Fira Code','Cascadia Code','Menlo',monospace; line-height: 1.2; }
.stat-number.red { color: #f87171; }
.stat-number.orange { color: #fbbf24; }
.stat-number.green { color: #34d399; }
.stat-label { font-size: 10px; color: #64748b; margin-top: 2px; white-space: nowrap; }
.footer-divider { width: 1px; height: 28px; background: rgba(255,255,255,.06); margin: 0 4px; }

/* ====== Responsive ====== */
@media (max-width: 768px) {
  .narrative-status { padding: 0 12px; }
  .status-text { font-size: 12px; }
  .brand { display: none; }
  .narrative-timeline { padding: 8px 10px 0; }
  .card-row { gap: 6px; }
  .ev-direction { display: none; }
  .ev-summary { font-size: 11px; }
  .narrative-footer { padding: 0 8px; gap: 0; flex-wrap: wrap; height: auto; min-height: 56px; }
  .footer-stat { padding: 4px 10px; }
  .stat-number { font-size: 18px; }
  .footer-divider { height: 20px; }
}
</style>