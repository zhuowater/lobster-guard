<template>
  <div>
    <div class="page-header">
      <div class="page-header-left">
        <h1 class="page-title">🔗 攻击链分析</h1>
        <span class="page-desc">跨 Agent 关联分析 — 发现协同攻击链</span>
      </div>
      <button class="btn btn-primary" @click="triggerAnalyze" :disabled="analyzing">
        {{ analyzing ? '分析中...' : '🔍 分析' }}
      </button>
    </div>

    <!-- 统计卡片 -->
    <div class="stat-grid" v-if="stats">
      <div class="stat-card">
        <div class="stat-value">{{ stats.active_chains }}</div>
        <div class="stat-label">活跃链 Active</div>
      </div>
      <div class="stat-card stat-danger">
        <div class="stat-value">{{ stats.critical_chains + stats.high_chains }}</div>
        <div class="stat-label">高危 Critical+High</div>
      </div>
      <div class="stat-card stat-info">
        <div class="stat-value">{{ stats.agents_involved }}</div>
        <div class="stat-label">涉及 Agent</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ stats.total_events }}</div>
        <div class="stat-label">事件 Events</div>
      </div>
    </div>

    <!-- 过滤器 -->
    <div class="filter-bar">
      <select v-model="filterSeverity" @change="loadChains" class="filter-select">
        <option value="">全部严重级别</option>
        <option value="critical">🔴 严重</option>
        <option value="high">🟠 高危</option>
        <option value="medium">🟡 中等</option>
        <option value="low">🟢 低风险</option>
      </select>
      <select v-model="filterStatus" @change="loadChains" class="filter-select">
        <option value="">全部状态</option>
        <option value="active">活跃</option>
        <option value="resolved">已处理</option>
        <option value="false_positive">误报</option>
      </select>
    </div>

    <!-- 攻击链卡片列表 -->
    <div v-if="loading" class="loading-state">加载中...</div>
    <div v-else-if="chains.length === 0" class="empty-state">
      <div class="empty-icon">🔗</div>
      <div class="empty-title">暂无攻击链</div>
      <div class="empty-desc">点击"分析"按钮从审计数据中发现攻击链</div>
    </div>
    <div v-else class="chain-list">
      <div v-for="chain in chains" :key="chain.id" class="chain-card" :class="'chain-' + chain.severity">
        <!-- 卡片头 -->
        <div class="chain-header">
          <div class="chain-header-left">
            <span class="severity-dot" :class="'dot-' + chain.severity"></span>
            <span class="chain-name">{{ chain.name }}</span>
            <span class="severity-badge" :class="'badge-' + chain.severity">{{ chain.severity }}</span>
            <span class="chain-meta">{{ chain.total_events }} 事件</span>
            <span class="chain-meta">{{ chain.agents.length }} Agent</span>
          </div>
          <div class="chain-header-right">
            <span class="chain-status" :class="'status-' + chain.status">{{ statusLabel(chain.status) }}</span>
          </div>
        </div>

        <!-- 描述 -->
        <div class="chain-desc">{{ chain.description }}</div>

        <!-- 事件时间线 -->
        <div class="chain-timeline" v-if="chain.events && chain.events.length > 0">
          <div class="tl-label">时间线:</div>
          <div v-for="(ev, idx) in chain.events" :key="idx" class="tl-row">
            <div class="tl-connector">
              <span class="tl-dot-icon" :class="ev.agent_id !== (chain.events[idx-1]||{}).agent_id ? 'tl-dot-diamond' : 'tl-dot-circle'">
                {{ ev.agent_id !== (chain.events[idx-1]||{}).agent_id ? '◆' : '●' }}
              </span>
              <div class="tl-line-v" v-if="idx < chain.events.length - 1"></div>
            </div>
            <div class="tl-event-content">
              <div class="tl-event-header">
                <span class="tl-time mono">{{ fmtTime(ev.timestamp) }}</span>
                <a class="tl-agent link-accent" @click.stop="$router.push('/user-profiles/' + encodeURIComponent(ev.agent_id))">{{ ev.agent_id }}</a>
                <span class="tl-type-tag" :class="'tag-' + ev.event_type">{{ ev.event_type }}</span>
                <a v-if="ev.source === 'honeypot' || ev.event_type === 'honeypot_trigger'" class="link-accent" style="font-size:11px;margin-left:6px" @click.stop="$router.push('/honeypot')">🍯 蜜罐 →</a>
              </div>
              <div class="tl-event-detail">"{{ ev.detail }}"</div>
            </div>
          </div>
        </div>

        <!-- 卡片底部 -->
        <div class="chain-footer">
          <div class="chain-footer-info">
            <span>涉及 Agent: <b>{{ chain.agents.join(', ') }}</b></span>
            <span class="chain-score">风险分: <b>{{ chain.risk_score }}</b></span>
          </div>
          <div class="chain-actions">
            <button v-if="chain.status === 'active'" class="btn btn-sm btn-ghost" @click="updateStatus(chain.id, 'resolved')">标记已处理</button>
            <button v-if="chain.status === 'active'" class="btn btn-sm btn-ghost" @click="updateStatus(chain.id, 'false_positive')">标记误报</button>
            <button v-if="chain.events.some(e => e.trace_id)" class="btn btn-sm" @click="goToSession(chain)">查看会话回放</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 攻击模式统计 -->
    <div class="card pattern-card" v-if="stats && Object.keys(stats.pattern_counts || {}).length > 0">
      <div class="card-header">
        <span class="card-title">📊 攻击模式分布</span>
      </div>
      <div class="pattern-bars">
        <div v-for="(count, name) in stats.pattern_counts" :key="name" class="pattern-row">
          <span class="pattern-name">{{ name }}</span>
          <div class="pattern-bar-bg">
            <div class="pattern-bar-fill" :style="{ width: barWidth(count) }"></div>
          </div>
          <span class="pattern-count">{{ count }}</span>
        </div>
      </div>
    </div>

    <!-- Toast -->
    <Transition name="toast-fade">
      <div class="toast" v-if="toastMsg">{{ toastMsg }}</div>
    </Transition>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api, apiPost, apiPut } from '../api.js'

const router = useRouter()
const chains = ref([])
const stats = ref(null)
const loading = ref(true)
const analyzing = ref(false)
const filterSeverity = ref('')
const filterStatus = ref('')
const toastMsg = ref('')

function showToast(msg) {
  toastMsg.value = msg
  setTimeout(() => { toastMsg.value = '' }, 3000)
}

function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  const pad = n => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function statusLabel(s) {
  const m = { active: '活跃', resolved: '已处理', false_positive: '误报' }
  return m[s] || s
}

function barWidth(count) {
  if (!stats.value || !stats.value.pattern_counts) return '0%'
  const max = Math.max(...Object.values(stats.value.pattern_counts), 1)
  return Math.round(count / max * 100) + '%'
}

async function loadChains() {
  loading.value = true
  try {
    let url = '/api/v1/attack-chains?tenant=all'
    if (filterSeverity.value) url += '&severity=' + filterSeverity.value
    if (filterStatus.value) url += '&status=' + filterStatus.value
    chains.value = await api(url)
  } catch { chains.value = [] }
  loading.value = false
}

async function loadStats() {
  try {
    stats.value = await api('/api/v1/attack-chains/stats?tenant=all')
  } catch { stats.value = null }
}

async function triggerAnalyze() {
  analyzing.value = true
  try {
    const res = await apiPost('/api/v1/attack-chains/analyze', { tenant_id: 'default', hours: 48 })
    showToast(`分析完成，发现 ${res.count || 0} 条攻击链`)
    loadChains()
    loadStats()
  } catch (e) {
    showToast('分析失败: ' + e.message)
  }
  analyzing.value = false
}

async function updateStatus(id, status) {
  try {
    await apiPut('/api/v1/attack-chains/' + id + '/status', { status })
    showToast('状态已更新')
    loadChains()
    loadStats()
  } catch (e) {
    showToast('更新失败: ' + e.message)
  }
}

function goToSession(chain) {
  const ev = chain.events.find(e => e.trace_id)
  if (ev) {
    router.push('/sessions/' + ev.trace_id)
  }
}

onMounted(() => {
  loadChains()
  loadStats()
})
</script>

<style scoped>
.page-header {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: var(--space-4);
}
.page-header-left { display: flex; flex-direction: column; gap: 2px; }
.page-title { font-size: var(--text-xl); font-weight: 700; color: var(--text-primary); margin: 0; }
.page-desc { font-size: var(--text-sm); color: var(--text-tertiary); }

.stat-grid {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3);
  margin-bottom: var(--space-4);
}
.stat-card {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-4); text-align: center;
}
.stat-value { font-size: 2rem; font-weight: 800; color: var(--text-primary); font-family: var(--font-mono); }
.stat-label { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-1); text-transform: uppercase; letter-spacing: 0.05em; }
.stat-danger .stat-value { color: #EF4444; }
.stat-info .stat-value { color: #3B82F6; }

.filter-bar {
  display: flex; gap: var(--space-2); margin-bottom: var(--space-4);
}
.filter-select {
  background: var(--bg-surface); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary);
  padding: var(--space-2) var(--space-3); font-size: var(--text-sm);
}
.filter-select:focus { border-color: var(--color-primary); outline: none; }

.loading-state, .empty-state {
  text-align: center; padding: var(--space-8); color: var(--text-tertiary);
}
.empty-icon { font-size: 3rem; margin-bottom: var(--space-2); }
.empty-title { font-size: var(--text-lg); font-weight: 600; color: var(--text-primary); }
.empty-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: var(--space-1); }

.chain-list { display: flex; flex-direction: column; gap: var(--space-3); }

.chain-card {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-4);
  border-left: 4px solid var(--border-subtle);
}
.chain-critical { border-left-color: #EF4444; }
.chain-high { border-left-color: #F97316; }
.chain-medium { border-left-color: #EAB308; }
.chain-low { border-left-color: #22C55E; }

.chain-header {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: var(--space-2);
}
.chain-header-left { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; }
.severity-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.dot-critical { background: #EF4444; }
.dot-high { background: #F97316; }
.dot-medium { background: #EAB308; }
.dot-low { background: #22C55E; }

.chain-name { font-weight: 700; color: var(--text-primary); font-size: var(--text-base); }
.severity-badge {
  font-size: 11px; font-weight: 700; padding: 2px 8px; border-radius: 10px;
  text-transform: uppercase; letter-spacing: 0.05em;
}
.badge-critical { background: rgba(239,68,68,0.15); color: #EF4444; }
.badge-high { background: rgba(249,115,22,0.15); color: #F97316; }
.badge-medium { background: rgba(234,179,8,0.15); color: #EAB308; }
.badge-low { background: rgba(34,197,94,0.15); color: #22C55E; }
.chain-meta { font-size: var(--text-xs); color: var(--text-tertiary); }

.chain-status {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 8px;
}
.status-active { background: rgba(239,68,68,0.1); color: #EF4444; }
.status-resolved { background: rgba(34,197,94,0.1); color: #22C55E; }
.status-false_positive { background: rgba(156,163,175,0.1); color: #9CA3AF; }

.chain-desc {
  font-size: var(--text-sm); color: var(--text-secondary);
  margin-bottom: var(--space-3);
}

/* Timeline */
.chain-timeline {
  margin: var(--space-3) 0; padding: var(--space-3);
  background: var(--bg-base); border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
}
.tl-label { font-size: var(--text-xs); color: var(--text-tertiary); font-weight: 600; margin-bottom: var(--space-2); text-transform: uppercase; letter-spacing: 0.05em; }

.tl-row { display: flex; gap: var(--space-3); min-height: 40px; }
.tl-connector {
  display: flex; flex-direction: column; align-items: center; width: 20px; flex-shrink: 0;
}
.tl-dot-icon { font-size: 14px; z-index: 1; }
.tl-dot-circle { color: #3B82F6; }
.tl-dot-diamond { color: #F97316; }
.tl-line-v { width: 2px; flex: 1; background: var(--border-subtle); margin: 2px 0; }

.tl-event-content { flex: 1; padding-bottom: var(--space-2); }
.tl-event-header { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; }
.tl-time { font-size: 11px; color: var(--text-tertiary); }
.mono { font-family: var(--font-mono); }
.tl-agent { font-size: var(--text-xs); color: var(--color-primary); font-weight: 600; }
.tl-type-tag {
  font-size: 10px; font-weight: 700; padding: 1px 6px; border-radius: 6px;
  text-transform: uppercase; letter-spacing: 0.03em;
}
.tag-probe { background: rgba(59,130,246,0.15); color: #3B82F6; }
.tag-extraction { background: rgba(234,179,8,0.15); color: #EAB308; }
.tag-execution { background: rgba(249,115,22,0.15); color: #F97316; }
.tag-exfiltration { background: rgba(239,68,68,0.15); color: #EF4444; }
.tag-honeypot_trigger { background: rgba(168,85,247,0.15); color: #A855F7; }

.tl-event-detail {
  font-size: var(--text-xs); color: var(--text-secondary); font-style: italic;
  margin-top: 2px;
}

/* Footer */
.chain-footer {
  display: flex; justify-content: space-between; align-items: center;
  padding-top: var(--space-3); border-top: 1px solid var(--border-subtle);
  margin-top: var(--space-3); flex-wrap: wrap; gap: var(--space-2);
}
.chain-footer-info { display: flex; gap: var(--space-4); font-size: var(--text-xs); color: var(--text-secondary); flex-wrap: wrap; }
.chain-score { color: var(--text-primary); }
.chain-actions { display: flex; gap: var(--space-2); }

/* Pattern chart */
.pattern-card { margin-top: var(--space-4); }
.pattern-bars { padding: var(--space-2) 0; }
.pattern-row { display: flex; align-items: center; gap: var(--space-3); padding: var(--space-1) 0; }
.pattern-name { font-size: var(--text-sm); color: var(--text-secondary); width: 180px; flex-shrink: 0; }
.pattern-bar-bg {
  flex: 1; height: 20px; background: var(--bg-elevated); border-radius: var(--radius-sm); overflow: hidden;
}
.pattern-bar-fill {
  height: 100%; background: linear-gradient(90deg, var(--color-primary), #3B82F6);
  border-radius: var(--radius-sm); transition: width 0.5s ease;
}
.pattern-count { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); width: 30px; text-align: right; }

/* Toast */
.toast {
  position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%);
  background: var(--bg-surface); border: 1px solid var(--color-primary);
  color: var(--text-primary); padding: 10px 20px; border-radius: var(--radius-md);
  font-size: var(--text-sm); font-weight: 600; box-shadow: var(--shadow-lg); z-index: 9999;
}
.toast-fade-enter-active, .toast-fade-leave-active { transition: all .3s ease; }
.toast-fade-enter-from, .toast-fade-leave-to { opacity: 0; transform: translateX(-50%) translateY(10px); }

@media (max-width: 768px) {
  .stat-grid { grid-template-columns: repeat(2, 1fr); }
  .chain-footer { flex-direction: column; align-items: flex-start; }
}
.link-accent{color:var(--color-primary);cursor:pointer;text-decoration:none}.link-accent:hover{text-decoration:underline}
</style>
