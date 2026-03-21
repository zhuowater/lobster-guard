<template>
  <div class="attackchain-page">
    <!-- Page Header -->
    <div class="page-header">
      <div class="page-header-left">
        <h1 class="page-title">🔗 攻击链分析</h1>
        <span class="page-desc">跨 Agent 关联分析 — 发现协同攻击链路与模式</span>
      </div>
      <div class="page-header-actions">
        <button class="btn btn-ghost btn-sm" @click="showConfigPanel = !showConfigPanel" title="分析配置">
          <Icon name="settings" :size="14" /> 配置
        </button>
        <button class="btn btn-primary" @click="triggerAnalyze" :disabled="analyzing">
          <Icon v-if="!analyzing" name="refresh" :size="14" />
          <Icon v-else name="loader" :size="14" />
          {{ analyzing ? '分析中...' : '运行分析' }}
        </button>
      </div>
    </div>

    <!-- 分析配置面板 -->
    <Transition name="slide">
      <div v-if="showConfigPanel" class="config-panel card">
        <div class="config-panel-header">
          <span class="config-panel-title"><Icon name="settings" :size="14" /> 分析配置</span>
          <button class="btn-icon" @click="showConfigPanel = false">✕</button>
        </div>
        <div class="config-panel-body">
          <div class="config-field">
            <label>分析时间窗口</label>
            <select v-model="analyzeHours" class="filter-select">
              <option :value="6">最近 6 小时</option>
              <option :value="12">最近 12 小时</option>
              <option :value="24">最近 24 小时</option>
              <option :value="48">最近 48 小时</option>
              <option :value="72">最近 3 天</option>
              <option :value="168">最近 7 天</option>
            </select>
          </div>
          <div class="config-field">
            <label>最小链长度</label>
            <select v-model="minChainLength" class="filter-select">
              <option :value="2">≥ 2 个事件</option>
              <option :value="3">≥ 3 个事件</option>
              <option :value="4">≥ 4 个事件</option>
              <option :value="5">≥ 5 个事件</option>
            </select>
          </div>
          <div class="config-hint">
            <Icon name="info" :size="12" /> 时间窗口越大分析越全面，但耗时更长
          </div>
        </div>
      </div>
    </Transition>

    <!-- 统计卡片 -->
    <div class="stats-grid" v-if="stats">
      <StatCard :iconSvg="svgLink" label="攻击链总数" :value="chainTotal" color="blue" />
      <StatCard :iconSvg="svgActivity" label="活跃链" :value="stats.active_chains" color="red" :badge="stats.active_chains > 0 ? '需处理' : ''" />
      <StatCard :iconSvg="svgUsers" label="涉及 Agent" :value="stats.agents_involved" color="indigo" />
      <StatCard :iconSvg="svgShield" label="最高风险" :value="highestRiskLabel" :color="highestRiskColor" />
    </div>
    <div v-else class="stats-grid">
      <div class="stat-placeholder" v-for="i in 4" :key="i"><div class="ph-bar"></div><div class="ph-text"></div></div>
    </div>

    <!-- 过滤器 -->
    <div class="filter-bar">
      <div class="search-box">
        <Icon name="search" :size="14" class="search-icon" />
        <input v-model="searchQuery" type="text" class="search-input" placeholder="搜索 Agent / 规则名 / 链名称..." @input="debouncedFilter" />
        <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''; loadChains()">✕</button>
      </div>
      <select v-model="filterSeverity" @change="loadChains" class="filter-select">
        <option value="">全部风险</option>
        <option value="critical">🔴 严重</option>
        <option value="high">🟠 高危</option>
        <option value="medium">🟡 中等</option>
        <option value="low">⚪ 低风险</option>
      </select>
      <select v-model="filterStatus" @change="loadChains" class="filter-select">
        <option value="">全部状态</option>
        <option value="active">🟢 活跃</option>
        <option value="resolved">✅ 已处理</option>
        <option value="false_positive">⚪ 误报</option>
      </select>
      <select v-model="filterTime" class="filter-select">
        <option value="">全部时间</option>
        <option value="1h">最近 1 小时</option>
        <option value="6h">最近 6 小时</option>
        <option value="24h">最近 24 小时</option>
        <option value="7d">最近 7 天</option>
      </select>
      <span class="filter-count" v-if="filteredChains.length !== chains.length">显示 {{ filteredChains.length }} / {{ chains.length }}</span>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading-center"><Icon name="loader" :size="24" /><span>加载攻击链数据...</span></div>

    <!-- 空状态 -->
    <EmptyState v-else-if="chains.length === 0" icon="🔗" title="暂无攻击链数据" description="点击「运行分析」从审计日志中自动发现跨 Agent 攻击链路" actionText="立即分析" @action="triggerAnalyze" />

    <!-- 搜索无结果 -->
    <EmptyState v-else-if="filteredChains.length === 0" icon="🔍" title="无匹配结果" description="尝试调整搜索条件或筛选器" actionText="清除筛选" @action="clearFilters" />

    <!-- 攻击链列表 -->
    <div v-else class="chain-list">
      <div v-for="chain in filteredChains" :key="chain.id" class="chain-card" :class="['chain-' + chain.severity, { 'chain-expanded': expandedId === chain.id }]">
        <!-- 卡片头 -->
        <div class="chain-header" @click="toggleExpand(chain.id)">
          <div class="chain-header-left">
            <span class="severity-dot" :class="'dot-' + chain.severity"></span>
            <span class="chain-name">{{ chain.name }}</span>
            <span class="severity-badge" :class="'badge-' + chain.severity">{{ severityText(chain.severity) }}</span>
            <span class="chain-meta"><Icon name="activity" :size="11" /> {{ chain.total_events }} 事件</span>
            <span class="chain-meta"><Icon name="users" :size="11" /> {{ chain.agents.length }} Agent</span>
            <span class="chain-meta mono">{{ fmtRelative(chain.last_seen) }}</span>
          </div>
          <div class="chain-header-right">
            <span class="chain-status-pill" :class="'st-' + chain.status">{{ statusText(chain.status) }}</span>
            <span class="risk-score" :class="'score-' + chain.severity">{{ chain.risk_score }}</span>
            <span class="expand-chevron" :class="{ rotated: expandedId === chain.id }">
              <Icon name="chevron-right" :size="16" />
            </span>
          </div>
        </div>

        <!-- 描述 + 标签 -->
        <div class="chain-summary">
          <span class="chain-desc">{{ chain.description }}</span>
          <div class="chain-tags" v-if="chain.pattern && chain.pattern !== 'Unknown'">
            <span class="pattern-tag" @click.stop="goToRules(chain.pattern)" :title="'查看相关规则'">
              <Icon name="tag" :size="11" /> {{ chain.pattern }}
            </span>
            <span v-for="src in uniqueSources(chain)" :key="src" class="source-tag" :class="'src-' + src">{{ sourceText(src) }}</span>
          </div>
        </div>

        <!-- 展开详情 -->
        <Transition name="detail-expand">
          <div v-if="expandedId === chain.id" class="chain-detail">
            <!-- 涉及 Agent -->
            <div class="detail-block">
              <div class="detail-label"><Icon name="users" :size="12" /> 涉及 Agent</div>
              <div class="agent-chips">
                <a v-for="agent in chain.agents" :key="agent" class="agent-chip" @click.stop="$router.push('/user-profiles/' + encodeURIComponent(agent))">{{ agent }}</a>
              </div>
            </div>

            <!-- 时间线 -->
            <div class="detail-block">
              <div class="detail-label"><Icon name="clock" :size="12" /> 攻击时间线</div>
              <div class="timeline">
                <div v-for="(ev, idx) in chain.events" :key="idx" class="tl-item">
                  <div class="tl-rail">
                    <div class="tl-dot" :class="'tl-dot--' + (ev.severity || 'medium')"></div>
                    <div v-if="idx < chain.events.length - 1" class="tl-line"></div>
                  </div>
                  <div class="tl-body">
                    <div class="tl-row-top">
                      <span class="tl-time mono">{{ fmtFull(ev.timestamp) }}</span>
                      <a class="tl-agent-link" @click.stop="$router.push('/user-profiles/' + encodeURIComponent(ev.agent_id))">{{ ev.agent_id || 'unknown' }}</a>
                      <span class="tl-type-badge" :class="'etype-' + ev.event_type">{{ eventLabel(ev.event_type) }}</span>
                      <span class="tl-action-badge" :class="'eact-' + ev.action">{{ ev.action }}</span>
                      <a v-if="ev.source === 'honeypot' || ev.event_type === 'honeypot_trigger'" class="tl-hp-link" @click.stop="$router.push('/honeypot')">🍯 蜜罐 →</a>
                    </div>
                    <div class="tl-detail-text">{{ ev.detail }}</div>
                    <div class="tl-trace" v-if="ev.trace_id">
                      <a class="trace-link" @click.stop="$router.push('/sessions/' + ev.trace_id)"><Icon name="play" :size="10" /> trace: {{ ev.trace_id.slice(0, 16) }}…</a>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- 操作 -->
            <div class="detail-actions">
              <template v-if="chain.status === 'active'">
                <button class="btn btn-sm" @click.stop="askConfirm('resolved', chain)"><Icon name="check-circle" :size="13" /> 标记已处理</button>
                <button class="btn btn-sm btn-ghost" @click.stop="askConfirm('false_positive', chain)"><Icon name="x-circle" :size="13" /> 标记误报</button>
                <button class="btn btn-sm btn-red" @click.stop="askBan(chain)"><Icon name="lock" :size="13" /> 封禁 Agent</button>
              </template>
              <template v-else>
                <button class="btn btn-sm btn-ghost" @click.stop="askConfirm('active', chain)"><Icon name="refresh" :size="13" /> 重新激活</button>
              </template>
              <button v-if="chain.events.some(e => e.trace_id)" class="btn btn-sm" @click.stop="goToSession(chain)"><Icon name="play" :size="13" /> 会话回放</button>
            </div>
          </div>
        </Transition>
      </div>
    </div>

    <!-- 攻击模式分布 -->
    <div class="card pattern-card" v-if="stats && Object.keys(stats.pattern_counts || {}).length > 0">
      <div class="card-header"><span class="card-title"><Icon name="bar-chart" :size="14" /> 攻击模式分布</span></div>
      <div class="pattern-bars">
        <div v-for="(count, name) in stats.pattern_counts" :key="name" class="pattern-row">
          <span class="pattern-name clickable" @click="goToRules(name)"><Icon name="tag" :size="11" /> {{ name }}</span>
          <div class="pattern-bar-bg"><div class="pattern-bar-fill" :style="{ width: barPct(count) }"></div></div>
          <span class="pattern-count">{{ count }}</span>
        </div>
      </div>
    </div>

    <!-- ConfirmModal -->
    <ConfirmModal :visible="cmVis" :title="cmTitle" :message="cmMsg" :type="cmType" :confirmText="cmBtn" @confirm="onCmConfirm" @cancel="cmVis = false" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api, apiPost, apiPut } from '../api.js'
import { showToast } from '../stores/app.js'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import EmptyState from '../components/EmptyState.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const router = useRouter()

// State
const chains = ref([])
const stats = ref(null)
const loading = ref(true)
const analyzing = ref(false)
const expandedId = ref(null)
const showConfigPanel = ref(false)
const searchQuery = ref('')
const filterSeverity = ref('')
const filterStatus = ref('')
const filterTime = ref('')
const analyzeHours = ref(48)
const minChainLength = ref(2)

// Confirm modal state
const cmVis = ref(false)
const cmTitle = ref('')
const cmMsg = ref('')
const cmType = ref('warning')
const cmBtn = ref('确认')
const cmCb = ref(null)

// SVG strings for StatCard icons
const svgLink = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>'
const svgActivity = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgUsers = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>'
const svgShield = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>'

// Computed
const chainTotal = computed(() => chains.value.length)

const highestRiskLabel = computed(() => {
  if (!stats.value) return '--'
  if (stats.value.critical_chains > 0) return 'CRITICAL'
  if (stats.value.high_chains > 0) return 'HIGH'
  if (stats.value.medium_chains > 0) return 'MEDIUM'
  if (stats.value.low_chains > 0) return 'LOW'
  return '安全'
})

const highestRiskColor = computed(() => {
  if (!stats.value) return 'blue'
  if (stats.value.critical_chains > 0) return 'red'
  if (stats.value.high_chains > 0) return 'yellow'
  if (stats.value.medium_chains > 0) return 'yellow'
  return 'green'
})

const filteredChains = computed(() => {
  let r = chains.value
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase().trim()
    r = r.filter(c =>
      c.name.toLowerCase().includes(q) ||
      c.description.toLowerCase().includes(q) ||
      (c.pattern || '').toLowerCase().includes(q) ||
      c.agents.some(a => a.toLowerCase().includes(q)) ||
      c.events.some(e => (e.detail || '').toLowerCase().includes(q))
    )
  }
  if (filterSeverity.value) r = r.filter(c => c.severity === filterSeverity.value)
  if (filterStatus.value) r = r.filter(c => c.status === filterStatus.value)
  if (filterTime.value) {
    const ms = { '1h': 36e5, '6h': 216e5, '24h': 864e5, '7d': 6048e5 }
    const cutoff = Date.now() - (ms[filterTime.value] || 0)
    r = r.filter(c => { const t = new Date(c.last_seen).getTime(); return !isNaN(t) && t >= cutoff })
  }
  if (minChainLength.value > 2) r = r.filter(c => c.total_events >= minChainLength.value)
  return r
})

// Helpers
function fmtFull(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  const p = n => String(n).padStart(2, '0')
  return `${p(d.getMonth()+1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}
function fmtRelative(ts) {
  if (!ts) return ''
  const d = new Date(ts); if (isNaN(d.getTime())) return ts
  const diff = Date.now() - d.getTime()
  if (diff < 6e4) return '刚刚'
  if (diff < 36e5) return Math.floor(diff / 6e4) + ' 分钟前'
  if (diff < 864e5) return Math.floor(diff / 36e5) + ' 小时前'
  return Math.floor(diff / 864e5) + ' 天前'
}
function statusText(s) { return { active: '活跃', resolved: '已处理', false_positive: '误报' }[s] || s }
function severityText(s) { return { critical: 'CRITICAL', high: 'HIGH', medium: 'MEDIUM', low: 'LOW' }[s] || s }
function eventLabel(t) { return { probe: '探测', extraction: '提取', execution: '执行', exfiltration: '外传', honeypot_trigger: '蜜罐触发' }[t] || t }
function sourceText(s) { return { im_audit: 'IM', llm_audit: 'LLM', honeypot: '蜜罐' }[s] || s }
function uniqueSources(chain) { const s = new Set(); chain.events.forEach(e => { if (e.source) s.add(e.source) }); return [...s] }
function barPct(count) {
  if (!stats.value?.pattern_counts) return '0%'
  const max = Math.max(...Object.values(stats.value.pattern_counts), 1)
  return Math.round(count / max * 100) + '%'
}

// Debounce for search
let _searchTimer = null
function debouncedFilter() { clearTimeout(_searchTimer); _searchTimer = setTimeout(() => {}, 200) }

// Actions
function toggleExpand(id) { expandedId.value = expandedId.value === id ? null : id }

function clearFilters() {
  searchQuery.value = ''; filterSeverity.value = ''; filterStatus.value = ''; filterTime.value = ''
  loadChains()
}

async function loadChains() {
  loading.value = true
  try {
    let url = '/api/v1/attack-chains?tenant=all'
    if (filterSeverity.value) url += '&severity=' + filterSeverity.value
    if (filterStatus.value) url += '&status=' + filterStatus.value
    const data = await api(url)
    chains.value = Array.isArray(data) ? data : []
  } catch { chains.value = [] }
  loading.value = false
}

async function loadStats() {
  try { stats.value = await api('/api/v1/attack-chains/stats?tenant=all') } catch { stats.value = null }
}

async function triggerAnalyze() {
  analyzing.value = true
  showToast('正在分析攻击链...')
  try {
    const res = await apiPost('/api/v1/attack-chains/analyze', { tenant_id: 'default', hours: analyzeHours.value })
    const count = res.count || 0
    showToast(count > 0 ? `分析完成，发现 ${count} 条新攻击链` : '分析完成，未发现新攻击链')
    await Promise.all([loadChains(), loadStats()])
  } catch (e) { showToast('分析失败: ' + (e.message || '未知错误')) }
  analyzing.value = false
}

// Confirm modal
function askConfirm(newStatus, chain) {
  const map = {
    resolved: { t: '标记已处理', m: `确认将「${chain.name}」标记为已处理？`, tp: 'info', b: '确认处理' },
    false_positive: { t: '标记为误报', m: `确认将「${chain.name}」标记为误报？`, tp: 'warning', b: '确认误报' },
    active: { t: '重新激活', m: `确认将「${chain.name}」重新激活？`, tp: 'warning', b: '确认激活' },
  }
  const c = map[newStatus] || { t: '确认', m: '确认操作？', tp: 'warning', b: '确认' }
  cmTitle.value = c.t; cmMsg.value = c.m; cmType.value = c.tp; cmBtn.value = c.b
  cmCb.value = () => doUpdateStatus(chain.id, newStatus)
  cmVis.value = true
}
function askBan(chain) {
  cmTitle.value = '⚠️ 封禁 Agent'
  cmMsg.value = `确认封禁攻击链涉及的 Agent：${chain.agents.join(', ')}？封禁后将无法继续交互。`
  cmType.value = 'danger'; cmBtn.value = '确认封禁'
  cmCb.value = () => doBan(chain)
  cmVis.value = true
}
async function onCmConfirm() {
  cmVis.value = false
  if (cmCb.value) { await cmCb.value(); cmCb.value = null }
}

async function doUpdateStatus(id, status) {
  try {
    await apiPut('/api/v1/attack-chains/' + id + '/status', { status })
    showToast('状态已更新')
    await Promise.all([loadChains(), loadStats()])
  } catch (e) { showToast('更新失败: ' + (e.message || '')) }
}

async function doBan(chain) {
  let ok = 0, fail = 0
  for (const a of chain.agents) {
    try { await apiPost('/api/v1/user-profiles/' + encodeURIComponent(a) + '/block', { blocked: true }); ok++ } catch { fail++ }
  }
  if (ok) showToast(`已封禁 ${ok} 个 Agent`)
  if (fail) showToast(`${fail} 个 Agent 封禁失败`)
  try { await apiPut('/api/v1/attack-chains/' + chain.id + '/status', { status: 'resolved' }) } catch {}
  await Promise.all([loadChains(), loadStats()])
}

function goToSession(chain) { const ev = chain.events.find(e => e.trace_id); if (ev) router.push('/sessions/' + ev.trace_id) }
function goToRules(pattern) { router.push('/rules?q=' + encodeURIComponent(pattern)) }

onMounted(() => { Promise.all([loadChains(), loadStats()]) })
</script>

<style scoped>
/* ═══════ Page ═══════ */
.attackchain-page { max-width: 1200px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-4); }
.page-header-left { display: flex; flex-direction: column; gap: 2px; }
.page-title { font-size: var(--text-xl); font-weight: 700; color: var(--text-primary); margin: 0; }
.page-desc { font-size: var(--text-sm); color: var(--text-tertiary); }
.page-header-actions { display: flex; gap: var(--space-2); align-items: center; }

/* ═══════ Config Panel ═══════ */
.config-panel { margin-bottom: var(--space-4); border: 1px solid var(--border-default); }
.config-panel-header { display: flex; justify-content: space-between; align-items: center; padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--border-subtle); }
.config-panel-title { font-weight: 600; font-size: var(--text-sm); color: var(--text-primary); display: flex; align-items: center; gap: var(--space-2); }
.config-panel-body { padding: var(--space-4); display: flex; gap: var(--space-4); align-items: flex-end; flex-wrap: wrap; }
.config-field { display: flex; flex-direction: column; gap: var(--space-1); }
.config-field label { font-size: var(--text-xs); color: var(--text-secondary); font-weight: 600; text-transform: uppercase; letter-spacing: .04em; }
.config-hint { font-size: var(--text-xs); color: var(--text-tertiary); display: flex; align-items: center; gap: 4px; }
.btn-icon { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 16px; padding: 4px; }
.btn-icon:hover { color: var(--text-primary); }

.slide-enter-active, .slide-leave-active { transition: all .25s ease; overflow: hidden; }
.slide-enter-from, .slide-leave-to { opacity: 0; max-height: 0; margin-bottom: 0; }
.slide-enter-to, .slide-leave-from { max-height: 200px; }

/* ═══════ Stats ═══════ */
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-placeholder { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-5); }
.ph-bar { width: 40%; height: 12px; background: var(--bg-elevated); border-radius: 4px; margin-bottom: var(--space-3); animation: pulse 1.5s infinite; }
.ph-text { width: 60%; height: 24px; background: var(--bg-elevated); border-radius: 4px; animation: pulse 1.5s infinite; }
@keyframes pulse { 0%,100% { opacity: .4; } 50% { opacity: .7; } }

/* ═══════ Filter Bar ═══════ */
.filter-bar { display: flex; gap: var(--space-2); margin-bottom: var(--space-4); align-items: center; flex-wrap: wrap; }
.search-box { position: relative; flex: 1; min-width: 200px; max-width: 360px; }
.search-icon { position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: var(--text-tertiary); pointer-events: none; }
.search-input {
  width: 100%; background: var(--bg-surface); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary);
  padding: var(--space-2) var(--space-3) var(--space-2) 32px; font-size: var(--text-sm);
}
.search-input:focus { border-color: var(--color-primary); outline: none; box-shadow: 0 0 0 2px rgba(59,130,246,.15); }
.search-input::placeholder { color: var(--text-disabled); }
.search-clear {
  position: absolute; right: 8px; top: 50%; transform: translateY(-50%);
  background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 14px; padding: 2px;
}
.search-clear:hover { color: var(--text-primary); }
.filter-select {
  background: var(--bg-surface); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary);
  padding: var(--space-2) var(--space-3); font-size: var(--text-sm); cursor: pointer;
}
.filter-select:focus { border-color: var(--color-primary); outline: none; }
.filter-count { font-size: var(--text-xs); color: var(--text-tertiary); margin-left: var(--space-2); white-space: nowrap; }

/* ═══════ Loading ═══════ */
.loading-center { display: flex; align-items: center; justify-content: center; gap: var(--space-3); padding: var(--space-12); color: var(--text-tertiary); font-size: var(--text-sm); }

/* ═══════ Chain List ═══════ */
.chain-list { display: flex; flex-direction: column; gap: var(--space-3); }

.chain-card {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); border-left: 4px solid var(--border-subtle);
  transition: border-color .2s, box-shadow .2s;
}
.chain-card:hover { box-shadow: var(--shadow-sm); }
.chain-card.chain-expanded { box-shadow: var(--shadow-md); border-color: var(--border-default); }
.chain-critical { border-left-color: #EF4444; }
.chain-high { border-left-color: #F97316; }
.chain-medium { border-left-color: #EAB308; }
.chain-low { border-left-color: #6B7280; }

/* ─── Header ─── */
.chain-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: var(--space-3) var(--space-4); cursor: pointer;
  transition: background .15s;
}
.chain-header:hover { background: var(--bg-elevated); }
.chain-header-left { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; flex: 1; min-width: 0; }
.chain-header-right { display: flex; align-items: center; gap: var(--space-3); flex-shrink: 0; }

.severity-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.dot-critical { background: #EF4444; box-shadow: 0 0 6px rgba(239,68,68,.4); }
.dot-high { background: #F97316; }
.dot-medium { background: #EAB308; }
.dot-low { background: #6B7280; }

.chain-name { font-weight: 700; color: var(--text-primary); font-size: var(--text-sm); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 260px; }
.severity-badge {
  font-size: 10px; font-weight: 700; padding: 2px 8px; border-radius: 10px;
  text-transform: uppercase; letter-spacing: .05em; flex-shrink: 0;
}
.badge-critical { background: rgba(239,68,68,.12); color: #EF4444; }
.badge-high { background: rgba(249,115,22,.12); color: #F97316; }
.badge-medium { background: rgba(234,179,8,.12); color: #EAB308; }
.badge-low { background: rgba(107,114,128,.12); color: #6B7280; }

.chain-meta { font-size: var(--text-xs); color: var(--text-tertiary); display: inline-flex; align-items: center; gap: 3px; white-space: nowrap; }
.mono { font-family: var(--font-mono); }

.chain-status-pill {
  font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 8px; white-space: nowrap;
}
.st-active { background: rgba(239,68,68,.1); color: #EF4444; }
.st-resolved { background: rgba(34,197,94,.1); color: #22C55E; }
.st-false_positive { background: rgba(107,114,128,.1); color: #9CA3AF; }

.risk-score {
  font-size: var(--text-sm); font-weight: 800; font-family: var(--font-mono); min-width: 32px; text-align: center;
}
.score-critical { color: #EF4444; }
.score-high { color: #F97316; }
.score-medium { color: #EAB308; }
.score-low { color: #6B7280; }

.expand-chevron { display: flex; align-items: center; transition: transform .2s ease; color: var(--text-tertiary); }
.expand-chevron.rotated { transform: rotate(90deg); }

/* ─── Summary ─── */
.chain-summary { padding: 0 var(--space-4) var(--space-3); }
.chain-desc { font-size: var(--text-sm); color: var(--text-secondary); line-height: 1.5; }
.chain-tags { display: flex; gap: var(--space-2); margin-top: var(--space-2); flex-wrap: wrap; align-items: center; }
.pattern-tag {
  display: inline-flex; align-items: center; gap: 4px;
  font-size: 11px; font-weight: 600; padding: 2px 10px; border-radius: 10px;
  background: rgba(59,130,246,.1); color: var(--color-primary); cursor: pointer;
  transition: background .15s;
}
.pattern-tag:hover { background: rgba(59,130,246,.2); }
.source-tag {
  font-size: 10px; font-weight: 600; padding: 2px 8px; border-radius: 8px;
  background: rgba(107,114,128,.08); color: var(--text-tertiary);
}
.src-honeypot { background: rgba(168,85,247,.1); color: #A855F7; }
.src-llm_audit { background: rgba(59,130,246,.1); color: #3B82F6; }
.src-im_audit { background: rgba(34,197,94,.1); color: #22C55E; }

/* ─── Detail expand ─── */
.detail-expand-enter-active, .detail-expand-leave-active { transition: all .3s ease; overflow: hidden; }
.detail-expand-enter-from, .detail-expand-leave-to { opacity: 0; max-height: 0; }
.detail-expand-enter-to, .detail-expand-leave-from { max-height: 2000px; }

.chain-detail {
  border-top: 1px solid var(--border-subtle);
  padding: var(--space-4);
  background: var(--bg-base);
  border-radius: 0 0 var(--radius-lg) var(--radius-lg);
}

.detail-block { margin-bottom: var(--space-4); }
.detail-block:last-child { margin-bottom: 0; }
.detail-label {
  font-size: var(--text-xs); font-weight: 700; color: var(--text-tertiary);
  text-transform: uppercase; letter-spacing: .05em; margin-bottom: var(--space-2);
  display: flex; align-items: center; gap: 6px;
}

/* Agent chips */
.agent-chips { display: flex; gap: var(--space-2); flex-wrap: wrap; }
.agent-chip {
  display: inline-block; padding: 4px 12px; border-radius: var(--radius-md);
  background: rgba(59,130,246,.08); color: var(--color-primary);
  font-size: var(--text-xs); font-weight: 600; cursor: pointer;
  border: 1px solid rgba(59,130,246,.15); transition: all .15s;
  text-decoration: none;
}
.agent-chip:hover { background: rgba(59,130,246,.15); border-color: var(--color-primary); }

/* ═══════ Timeline ═══════ */
.timeline { padding-left: var(--space-1); }
.tl-item { display: flex; gap: var(--space-3); min-height: 56px; }
.tl-rail { display: flex; flex-direction: column; align-items: center; width: 16px; flex-shrink: 0; padding-top: 4px; }
.tl-dot {
  width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0;
  border: 2px solid var(--bg-base); box-shadow: 0 0 0 2px var(--border-subtle);
  z-index: 1;
}
.tl-dot--critical { background: #EF4444; box-shadow: 0 0 0 2px rgba(239,68,68,.3); }
.tl-dot--high { background: #F97316; box-shadow: 0 0 0 2px rgba(249,115,22,.3); }
.tl-dot--medium { background: #EAB308; box-shadow: 0 0 0 2px rgba(234,179,8,.3); }
.tl-dot--low { background: #6B7280; box-shadow: 0 0 0 2px rgba(107,114,128,.3); }
.tl-line { width: 2px; flex: 1; background: var(--border-subtle); margin: 3px 0; }

.tl-body { flex: 1; padding-bottom: var(--space-3); }
.tl-row-top { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; }
.tl-time { font-size: 11px; color: var(--text-tertiary); }
.tl-agent-link { font-size: var(--text-xs); color: var(--color-primary); font-weight: 600; cursor: pointer; text-decoration: none; }
.tl-agent-link:hover { text-decoration: underline; }

.tl-type-badge {
  font-size: 10px; font-weight: 700; padding: 1px 7px; border-radius: 6px;
  text-transform: uppercase; letter-spacing: .03em;
}
.etype-probe { background: rgba(59,130,246,.12); color: #3B82F6; }
.etype-extraction { background: rgba(234,179,8,.12); color: #EAB308; }
.etype-execution { background: rgba(249,115,22,.12); color: #F97316; }
.etype-exfiltration { background: rgba(239,68,68,.12); color: #EF4444; }
.etype-honeypot_trigger { background: rgba(168,85,247,.12); color: #A855F7; }

.tl-action-badge {
  font-size: 10px; font-weight: 600; padding: 1px 6px; border-radius: 6px;
}
.eact-block { background: rgba(239,68,68,.1); color: #EF4444; }
.eact-warn { background: rgba(234,179,8,.1); color: #EAB308; }
.eact-pass { background: rgba(34,197,94,.1); color: #22C55E; }

.tl-hp-link { font-size: 11px; color: #A855F7; cursor: pointer; text-decoration: none; }
.tl-hp-link:hover { text-decoration: underline; }

.tl-detail-text {
  font-size: var(--text-xs); color: var(--text-secondary); margin-top: 4px;
  font-style: italic; line-height: 1.5; word-break: break-all;
}
.tl-trace { margin-top: 4px; }
.trace-link {
  font-size: 11px; color: var(--color-primary); cursor: pointer; text-decoration: none;
  display: inline-flex; align-items: center; gap: 4px; font-family: var(--font-mono);
}
.trace-link:hover { text-decoration: underline; }

/* ─── Detail Actions ─── */
.detail-actions {
  display: flex; gap: var(--space-2); padding-top: var(--space-3);
  border-top: 1px solid var(--border-subtle); margin-top: var(--space-3); flex-wrap: wrap;
}

/* ═══════ Pattern Card ═══════ */
.pattern-card { margin-top: var(--space-4); }
.card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); }
.card-header { padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--border-subtle); }
.card-title { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); display: flex; align-items: center; gap: var(--space-2); }
.pattern-bars { padding: var(--space-3) var(--space-4); }
.pattern-row { display: flex; align-items: center; gap: var(--space-3); padding: var(--space-1) 0; }
.pattern-name { font-size: var(--text-sm); color: var(--text-secondary); width: 180px; flex-shrink: 0; display: flex; align-items: center; gap: 6px; }
.clickable { cursor: pointer; }
.clickable:hover { color: var(--color-primary); }
.pattern-bar-bg { flex: 1; height: 20px; background: var(--bg-elevated); border-radius: var(--radius-sm); overflow: hidden; }
.pattern-bar-fill {
  height: 100%; background: linear-gradient(90deg, var(--color-primary), #3B82F6);
  border-radius: var(--radius-sm); transition: width .5s ease;
}
.pattern-count { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); font-family: var(--font-mono); width: 32px; text-align: right; }

/* ═══════ Responsive ═══════ */
@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .filter-bar { flex-direction: column; align-items: stretch; }
  .search-box { max-width: 100%; }
  .chain-header { flex-direction: column; align-items: flex-start; gap: var(--space-2); }
  .chain-header-right { width: 100%; justify-content: flex-end; }
  .detail-actions { flex-direction: column; }
  .pattern-name { width: 120px; }
}
</style>