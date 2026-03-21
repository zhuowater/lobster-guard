<template>
  <div class="prompt-tracker-page">
    <!-- 统计面板 -->
    <div class="stats-grid" v-if="!loading">
      <StatCard :iconSvg="svgLayers" :value="stats.total || versions.length" label="版本总数" color="blue" />
      <StatCard :iconSvg="svgCheck" :value="stats.active || activeCount" label="活跃版本" color="green" badge="近7天" />
      <StatCard :iconSvg="svgHash" :value="avgTokensDisplay" label="平均 Token" color="indigo" />
      <StatCard :iconSvg="svgClock" :value="lastChangeDisplay" label="最近变更" color="yellow" />
    </div>
    <div class="stats-grid" v-else>
      <Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" />
    </div>

    <!-- 当前版本卡片 -->
    <div class="card current-card" v-if="currentVersion">
      <div class="card-header">
        <span class="card-icon"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg></span>
        <span class="card-title">当前 Prompt 版本</span>
        <span class="version-badge">v{{ versions.length }}</span>
        <span v-if="currentVersion.tag" class="tag-pill" :class="'tag-' + currentVersion.tag">{{ tagLabel(currentVersion.tag) }}</span>
      </div>
      <div class="current-info">
        <div class="current-row">
          <div class="current-item"><div class="current-label">Hash</div><div class="current-value mono">{{ currentVersion.hash }}</div></div>
          <div class="current-item"><div class="current-label">模型</div><div class="current-value">{{ currentVersion.model || '-' }}</div></div>
          <div class="current-item"><div class="current-label">首次出现</div><div class="current-value">{{ fmtTime(currentVersion.first_seen) }}</div></div>
          <div class="current-item"><div class="current-label">调用次数</div><div class="current-value">{{ currentVersion.total_calls || currentVersion.call_count }}</div></div>
        </div>
        <div class="current-metrics">
          <div class="metric-chip" :class="metricClass(currentVersion, 'canary')"><span class="metric-label">Canary 泄露率</span><span class="metric-value">{{ canaryRate(currentVersion) }}%</span></div>
          <div class="metric-chip" :class="metricClass(currentVersion, 'error')"><span class="metric-label">错误率</span><span class="metric-value">{{ (currentVersion.error_rate * 100).toFixed(1) }}%</span></div>
          <div class="metric-chip"><span class="metric-label">平均 Token</span><span class="metric-value">{{ Math.round(currentVersion.avg_tokens) }}</span></div>
          <div class="metric-chip" :class="metricClass(currentVersion, 'flagged')"><span class="metric-label">高危工具</span><span class="metric-value">{{ currentVersion.flagged_tools || 0 }}</span></div>
        </div>
      </div>
    </div>

    <div v-if="loading" class="loading-state"><div class="loading-spinner"></div><span>加载中...</span></div>

    <div v-else-if="!versions.length" class="empty-state">
      <div class="empty-icon">📝</div>
      <div class="empty-title">暂无 Prompt 版本</div>
      <div class="empty-desc">当 LLM 代理拦截到 System Prompt 后，版本将自动追踪。<br>可点击"注入演示数据"快速体验。</div>
    </div>

    <!-- 版本历史列表 -->
    <div class="card" v-if="versions.length" style="margin-top: 16px;">
      <div class="card-header">
        <span class="card-icon"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg></span>
        <span class="card-title">版本历史</span>
        <span class="version-count">共 {{ versions.length }} 个版本</span>
        <div class="card-actions">
          <button class="btn btn-ghost btn-sm" @click="loadData" :disabled="loading">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg> 刷新
          </button>
        </div>
      </div>

      <div class="filter-bar">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="searchQuery" placeholder="搜索内容、Hash、标签..." class="search-input" />
          <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''">✕</button>
        </div>
        <div class="filter-chips">
          <button v-for="t in tagFilters" :key="t.value" class="chip" :class="{ 'chip-active': activeTagFilter === t.value }" @click="activeTagFilter = activeTagFilter === t.value ? '' : t.value">
            <span class="chip-dot" :class="'dot-' + (t.value || 'all')"></span>{{ t.label }}
          </button>
        </div>
        <div class="sort-control">
          <select v-model="sortOrder" class="sort-select">
            <option value="newest">最新优先</option>
            <option value="oldest">最早优先</option>
            <option value="calls">调用最多</option>
          </select>
        </div>
      </div>

      <div class="version-list">
        <div v-if="!filteredVersions.length" class="empty-filter"><span class="empty-filter-icon">🔍</span><span>没有匹配的版本</span></div>
        <div v-for="v in filteredVersions" :key="v.hash" class="version-item" :class="{ 'version-current': v.hash === currentHash, 'version-expanded': expandedHash === v.hash, 'version-deprecated': v.tag === 'deprecated' }">
          <div class="version-header" @click="toggleExpand(v.hash)">
            <div class="version-left">
              <span class="expand-arrow" :class="{ 'arrow-open': expandedHash === v.hash }"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="9 18 15 12 9 6"/></svg></span>
              <span class="version-num">v{{ getVersionNum(v) }}</span>
              <span class="version-hash mono">{{ v.hash }}</span>
              <span class="tag tag-current" v-if="v.hash === currentHash">当前</span>
              <span v-if="v.tag" class="tag-pill tag-sm" :class="'tag-' + v.tag">{{ tagLabel(v.tag) }}</span>
            </div>
            <div class="version-right">
              <span class="version-time" :title="fmtTime(v.first_seen)">{{ fmtTimeRelative(v.first_seen) }}</span>
              <span class="version-calls">{{ v.total_calls || v.call_count }} 次调用</span>
              <span class="version-tokens mono">{{ Math.round(v.avg_tokens) }} tok</span>
            </div>
          </div>

          <div class="version-metrics" v-if="getPrevVersion(v)">
            <div class="vm-item" :class="compareClass(v, getPrevVersion(v), 'canary')"><span>Canary: {{ canaryRate(v) }}%</span><span class="vm-arrow">{{ canaryArrow(v, getPrevVersion(v)) }}</span></div>
            <div class="vm-item" :class="compareClass(v, getPrevVersion(v), 'error')"><span>Error: {{ (v.error_rate * 100).toFixed(1) }}%</span><span class="vm-arrow">{{ errorArrow(v, getPrevVersion(v)) }}</span></div>
            <div class="vm-verdict" :class="'verdict-' + getVerdict(v, getPrevVersion(v))">{{ verdictLabel(getVerdict(v, getPrevVersion(v))) }}</div>
          </div>
          <div class="version-metrics" v-else><span class="vm-initial">（初始版本，无对比）</span></div>

          <div class="version-actions">
            <button class="btn-sm btn-outline" @click.stop="showDetail(v)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg> 详情</button>
            <button class="btn-sm btn-outline" @click.stop="showDiff(v)" v-if="v.prev_hash"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg> 对比</button>
            <div class="tag-dropdown" @click.stop>
              <button class="btn-sm btn-outline" @click.stop="toggleTagMenu(v.hash)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/></svg> 标签</button>
              <div class="tag-menu" v-if="tagMenuHash === v.hash">
                <button v-for="opt in tagOptions" :key="opt.value" class="tag-menu-item" :class="{ 'tag-menu-active': v.tag === opt.value }" @click="setTag(v, opt.value)">
                  <span class="tag-dot" :class="'dot-' + (opt.value || 'none')"></span>{{ opt.label }}<span v-if="v.tag === opt.value" class="tag-check">✓</span>
                </button>
              </div>
            </div>
            <button class="btn-sm btn-outline btn-warn" @click.stop="confirmRollback(v)" v-if="v.hash !== currentHash"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="1 4 1 10 7 10"/><path d="M3.51 15a9 9 0 1 0 2.13-9.36L1 10"/></svg> 回滚</button>
          </div>

          <div class="version-expand" v-if="expandedHash === v.hash">
            <div class="expand-section"><div class="expand-title">Prompt 内容</div><div class="prompt-content-block"><pre>{{ v.content }}</pre></div></div>
            <div class="expand-metrics-grid">
              <div class="em-item"><span class="em-label">Token 数</span><span class="em-value">{{ Math.round(v.avg_tokens) }}</span></div>
              <div class="em-item"><span class="em-label">总调用</span><span class="em-value">{{ v.total_calls || v.call_count }}</span></div>
              <div class="em-item"><span class="em-label">Canary 泄露</span><span class="em-value" :class="(v.canary_leaks||0) > 0 ? 'em-danger' : 'em-safe'">{{ v.canary_leaks || 0 }}</span></div>
              <div class="em-item"><span class="em-label">预算超限</span><span class="em-value">{{ v.budget_exceeds || 0 }}</span></div>
              <div class="em-item"><span class="em-label">高危工具</span><span class="em-value" :class="(v.flagged_tools||0) > 0 ? 'em-warn' : 'em-safe'">{{ v.flagged_tools || 0 }}</span></div>
              <div class="em-item"><span class="em-label">错误率</span><span class="em-value">{{ (v.error_rate * 100).toFixed(1) }}%</span></div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 详情 Modal -->
    <div class="modal-overlay" v-if="detailModal" @click.self="detailModal = null">
      <div class="modal-box modal-lg">
        <div class="modal-header">
          <span>Prompt 详情 — {{ detailModal.hash }}</span>
          <div class="modal-header-actions">
            <span v-if="detailModal.tag" class="tag-pill" :class="'tag-' + detailModal.tag">{{ tagLabel(detailModal.tag) }}</span>
            <button class="modal-close" @click="detailModal = null">✕</button>
          </div>
        </div>
        <div class="modal-body">
          <div class="detail-meta"><span>模型: {{ detailModal.model }}</span><span>首次: {{ fmtTime(detailModal.first_seen) }}</span><span>末次: {{ fmtTime(detailModal.last_seen) }}</span><span>调用: {{ detailModal.total_calls || detailModal.call_count }} 次</span></div>
          <div class="prompt-content">
            <div class="prompt-content-header"><span>Prompt 内容</span><button class="btn-sm btn-ghost" @click="copyContent(detailModal.content)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg> 复制</button></div>
            <pre>{{ detailModal.content }}</pre>
          </div>
          <div class="detail-metrics">
            <div class="dm-item"><span class="dm-label">Canary 泄露</span><span class="dm-value" :class="(detailModal.canary_leaks||0) > 0 ? 'dm-danger' : ''">{{ detailModal.canary_leaks || 0 }} 次 ({{ canaryRate(detailModal) }}%)</span></div>
            <div class="dm-item"><span class="dm-label">预算超限</span><span class="dm-value">{{ detailModal.budget_exceeds || 0 }} 次</span></div>
            <div class="dm-item"><span class="dm-label">高危工具</span><span class="dm-value" :class="(detailModal.flagged_tools||0) > 0 ? 'dm-warn' : ''">{{ detailModal.flagged_tools || 0 }} 次</span></div>
            <div class="dm-item"><span class="dm-label">错误率</span><span class="dm-value">{{ (detailModal.error_rate * 100).toFixed(1) }}%</span></div>
            <div class="dm-item"><span class="dm-label">平均 Token</span><span class="dm-value">{{ Math.round(detailModal.avg_tokens) }}</span></div>
            <div class="dm-item"><span class="dm-label">内容长度</span><span class="dm-value">{{ (detailModal.content || '').length }} 字符</span></div>
          </div>
        </div>
      </div>
    </div>

    <!-- Diff Modal -->
    <div class="modal-overlay" v-if="diffModal" @click.self="diffModal = null">
      <div class="modal-box modal-xl">
        <div class="modal-header">
          <span>版本对比</span>
          <div class="diff-header-pills">
            <span class="diff-version-pill diff-old">{{ diffModal.old_version?.hash?.slice(0,8) || '无' }}</span>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></svg>
            <span class="diff-version-pill diff-new">{{ diffModal.new_version?.hash?.slice(0,8) }}</span>
          </div>
          <button class="modal-close" @click="diffModal = null">✕</button>
        </div>
        <div class="modal-body">
          <div class="diff-view-toggle">
            <button class="btn-sm" :class="diffView === 'split' ? 'btn-active' : 'btn-outline'" @click="diffView = 'split'">左右对比</button>
            <button class="btn-sm" :class="diffView === 'unified' ? 'btn-active' : 'btn-outline'" @click="diffView = 'unified'">统一视图</button>
          </div>

          <div class="diff-split" v-if="diffView === 'split'">
            <div class="diff-pane diff-pane-old">
              <div class="diff-pane-header">旧版本 <span class="mono">{{ diffModal.old_version?.hash?.slice(0,8) || '初始' }}</span></div>
              <div class="diff-pane-body">
                <div v-for="(line, i) in splitDiffLeft" :key="'l-'+i" class="diff-line" :class="'diff-' + line.type"><span class="diff-linenum">{{ line.num || '' }}</span><span class="diff-content">{{ line.content }}</span></div>
              </div>
            </div>
            <div class="diff-pane diff-pane-new">
              <div class="diff-pane-header">新版本 <span class="mono">{{ diffModal.new_version?.hash?.slice(0,8) }}</span></div>
              <div class="diff-pane-body">
                <div v-for="(line, i) in splitDiffRight" :key="'r-'+i" class="diff-line" :class="'diff-' + line.type"><span class="diff-linenum">{{ line.num || '' }}</span><span class="diff-content">{{ line.content }}</span></div>
              </div>
            </div>
          </div>

          <div class="diff-block" v-else>
            <div v-for="(line, i) in (diffModal.lines || [])" :key="i" class="diff-line" :class="'diff-' + line.type"><span class="diff-linenum">{{ line.line_num }}</span><span class="diff-prefix">{{ line.type === 'added' ? '+' : line.type === 'removed' ? '-' : ' ' }}</span><span class="diff-content">{{ line.content }}</span></div>
          </div>

          <div class="diff-summary">
            <span class="diff-stat diff-stat-add">+{{ diffAddedCount }} 新增</span>
            <span class="diff-stat diff-stat-del">-{{ diffRemovedCount }} 删除</span>
            <span class="diff-stat diff-stat-eq">{{ diffUnchangedCount }} 不变</span>
          </div>

          <div class="metrics-compare" v-if="diffModal.metrics_diff">
            <div class="mc-title">安全指标对比</div>
            <div class="mc-grid">
              <div class="mc-card"><div class="mc-card-label">Canary 泄露率</div><div class="mc-card-values"><span class="mc-old">{{ diffModal.metrics_diff.old_canary_rate?.toFixed(1) }}%</span><span class="mc-arrow">→</span><span class="mc-new" :class="rateImproved(diffModal.metrics_diff.old_canary_rate, diffModal.metrics_diff.new_canary_rate)">{{ diffModal.metrics_diff.new_canary_rate?.toFixed(1) }}%</span></div><span class="mc-change">{{ changeLabel(diffModal.metrics_diff.old_canary_rate, diffModal.metrics_diff.new_canary_rate) }}</span></div>
              <div class="mc-card"><div class="mc-card-label">错误率</div><div class="mc-card-values"><span class="mc-old">{{ diffModal.metrics_diff.old_error_rate?.toFixed(1) }}%</span><span class="mc-arrow">→</span><span class="mc-new" :class="rateImproved(diffModal.metrics_diff.old_error_rate, diffModal.metrics_diff.new_error_rate)">{{ diffModal.metrics_diff.new_error_rate?.toFixed(1) }}%</span></div><span class="mc-change">{{ changeLabel(diffModal.metrics_diff.old_error_rate, diffModal.metrics_diff.new_error_rate) }}</span></div>
              <div class="mc-card"><div class="mc-card-label">平均 Token</div><div class="mc-card-values"><span class="mc-old">{{ Math.round(diffModal.metrics_diff.old_avg_tokens || 0) }}</span><span class="mc-arrow">→</span><span class="mc-new">{{ Math.round(diffModal.metrics_diff.new_avg_tokens || 0) }}</span></div></div>
              <div class="mc-card"><div class="mc-card-label">高危工具率</div><div class="mc-card-values"><span class="mc-old">{{ diffModal.metrics_diff.old_flagged_rate?.toFixed(1) }}%</span><span class="mc-arrow">→</span><span class="mc-new" :class="rateImproved(diffModal.metrics_diff.old_flagged_rate, diffModal.metrics_diff.new_flagged_rate)">{{ diffModal.metrics_diff.new_flagged_rate?.toFixed(1) }}%</span></div><span class="mc-change">{{ changeLabel(diffModal.metrics_diff.old_flagged_rate, diffModal.metrics_diff.new_flagged_rate) }}</span></div>
            </div>
            <div class="mc-verdict" :class="'verdict-' + diffModal.metrics_diff.verdict">判定: {{ verdictLabel(diffModal.metrics_diff.verdict) }}</div>
          </div>
        </div>
      </div>
    </div>

    <ConfirmModal :visible="!!rollbackTarget" title="确认回滚" :message="rollbackMessage" type="warning" confirmText="确认回滚" cancelText="取消" @confirm="doRollback" @cancel="rollbackTarget = null" />
  </div>
</template>

<script setup>
import { ref, onMounted, computed, onBeforeUnmount } from 'vue'
import StatCard from '../components/StatCard.vue'
import Skeleton from '../components/Skeleton.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { api, apiPost } from '../api.js'
import { showToast } from '../stores/app.js'

const svgLayers = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg>'
const svgCheck = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>'
const svgHash = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="4" y1="9" x2="20" y2="9"/><line x1="4" y1="15" x2="20" y2="15"/><line x1="10" y1="3" x2="8" y2="21"/><line x1="16" y1="3" x2="14" y2="21"/></svg>'
const svgClock = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>'

const loading = ref(true)
const versions = ref([])
const currentVersion = ref(null)
const stats = ref({})
const detailModal = ref(null)
const diffModal = ref(null)
const diffView = ref('split')
const searchQuery = ref('')
const activeTagFilter = ref('')
const sortOrder = ref('newest')
const expandedHash = ref(null)
const tagMenuHash = ref(null)
const rollbackTarget = ref(null)

const currentHash = computed(() => currentVersion.value?.hash || '')

const tagFilters = [
  { label: '全部', value: '' },
  { label: 'Production', value: 'production' },
  { label: 'Staging', value: 'staging' },
  { label: 'Deprecated', value: 'deprecated' },
]

const tagOptions = [
  { label: 'Production', value: 'production' },
  { label: 'Staging', value: 'staging' },
  { label: 'Deprecated', value: 'deprecated' },
  { label: '清除标签', value: '' },
]

const activeCount = computed(() => {
  const week = Date.now() - 7 * 24 * 60 * 60 * 1000
  return versions.value.filter(v => new Date(v.last_seen).getTime() > week).length
})

const avgTokensDisplay = computed(() => {
  if (stats.value.avg_tokens) return Math.round(stats.value.avg_tokens)
  if (!versions.value.length) return 0
  const sum = versions.value.reduce((a, v) => a + (v.avg_tokens || 0), 0)
  return Math.round(sum / versions.value.length)
})

const lastChangeDisplay = computed(() => {
  const lc = stats.value.last_change || (versions.value.length ? versions.value[0].first_seen : '')
  return lc ? fmtTimeRelative(lc) : '--'
})

const filteredVersions = computed(() => {
  let list = [...versions.value]
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(v =>
      (v.content || '').toLowerCase().includes(q) ||
      (v.hash || '').toLowerCase().includes(q) ||
      (v.tag || '').toLowerCase().includes(q) ||
      (v.model || '').toLowerCase().includes(q)
    )
  }
  if (activeTagFilter.value) {
    list = list.filter(v => v.tag === activeTagFilter.value)
  }
  if (sortOrder.value === 'oldest') {
    list.sort((a, b) => new Date(a.first_seen) - new Date(b.first_seen))
  } else if (sortOrder.value === 'calls') {
    list.sort((a, b) => (b.total_calls || b.call_count) - (a.total_calls || a.call_count))
  }
  return list
})

const rollbackMessage = computed(() => {
  if (!rollbackTarget.value) return ''
  return `确定要回滚到版本 ${rollbackTarget.value.hash.slice(0,8)}...（v${getVersionNum(rollbackTarget.value)}）吗？\n当前版本将不再是活跃版本。`
})

// Diff computed
const splitDiffLeft = computed(() => {
  if (!diffModal.value?.lines) return []
  let num = 0
  return diffModal.value.lines.filter(l => l.type !== 'added').map(l => {
    if (l.type === 'removed') { num++; return { type: 'removed', content: l.content, num } }
    num++
    return { type: 'unchanged', content: l.content, num }
  })
})
const splitDiffRight = computed(() => {
  if (!diffModal.value?.lines) return []
  let num = 0
  return diffModal.value.lines.filter(l => l.type !== 'removed').map(l => {
    if (l.type === 'added') { num++; return { type: 'added', content: l.content, num } }
    num++
    return { type: 'unchanged', content: l.content, num }
  })
})
const diffAddedCount = computed(() => (diffModal.value?.lines || []).filter(l => l.type === 'added').length)
const diffRemovedCount = computed(() => (diffModal.value?.lines || []).filter(l => l.type === 'removed').length)
const diffUnchangedCount = computed(() => (diffModal.value?.lines || []).filter(l => l.type === 'unchanged').length)

// Version number helper - versions are sorted newest first by default
function getVersionNum(v) {
  const idx = versions.value.findIndex(x => x.hash === v.hash)
  return idx >= 0 ? versions.value.length - idx : '?'
}

function getPrevVersion(v) {
  if (!v.prev_hash) return null
  return versions.value.find(x => x.hash === v.prev_hash) || null
}

async function loadData() {
  loading.value = true
  try {
    const [listRes, curRes, statsRes] = await Promise.allSettled([
      api('/api/v1/prompts'),
      api('/api/v1/prompts/current'),
      api('/api/v1/prompts/stats')
    ])
    if (listRes.status === 'fulfilled') versions.value = listRes.value.versions || []
    if (curRes.status === 'fulfilled' && curRes.value.hash) currentVersion.value = curRes.value
    else if (versions.value.length > 0) currentVersion.value = versions.value[0]
    if (statsRes.status === 'fulfilled') stats.value = statsRes.value
  } catch (e) {
    console.error('Load prompt versions failed:', e)
  } finally {
    loading.value = false
  }
}

function toggleExpand(hash) { expandedHash.value = expandedHash.value === hash ? null : hash }

function showDetail(v) { detailModal.value = v }

async function showDiff(v) {
  try {
    const data = await api(`/api/v1/prompts/${v.hash}/diff`)
    diffModal.value = data
    diffView.value = 'split'
  } catch (e) {
    console.error('Load diff failed:', e)
    showToast('加载 Diff 失败', 'error')
  }
}

function toggleTagMenu(hash) { tagMenuHash.value = tagMenuHash.value === hash ? null : hash }

async function setTag(v, tag) {
  try {
    await apiPost(`/api/v1/prompts/${v.hash}/tag`, { tag })
    v.tag = tag
    if (currentVersion.value?.hash === v.hash) currentVersion.value.tag = tag
    tagMenuHash.value = null
    showToast(tag ? `标签已设为 ${tagLabel(tag)}` : '标签已清除', 'success')
  } catch (e) {
    showToast('设置标签失败: ' + e.message, 'error')
  }
}

function confirmRollback(v) { rollbackTarget.value = v }

async function doRollback() {
  const v = rollbackTarget.value
  if (!v) return
  try {
    await apiPost(`/api/v1/prompts/${v.hash}/rollback`, {})
    showToast(`已回滚到版本 ${v.hash.slice(0,8)}`, 'success')
    rollbackTarget.value = null
    await loadData()
  } catch (e) {
    showToast('回滚失败: ' + e.message, 'error')
    rollbackTarget.value = null
  }
}

function copyContent(text) {
  navigator.clipboard.writeText(text).then(() => showToast('已复制到剪贴板', 'success')).catch(() => showToast('复制失败', 'error'))
}

function tagLabel(tag) {
  const m = { production: 'Production', staging: 'Staging', deprecated: 'Deprecated' }
  return m[tag] || tag
}

function canaryRate(v) {
  const total = v.total_calls || v.call_count || 1
  return ((v.canary_leaks || 0) / total * 100).toFixed(1)
}

function metricClass(v, type) {
  if (type === 'canary') { const r = parseFloat(canaryRate(v)); return r > 1 ? 'metric-bad' : r > 0 ? 'metric-warn' : 'metric-good' }
  if (type === 'error') { const r = v.error_rate * 100; return r > 5 ? 'metric-bad' : r > 1 ? 'metric-warn' : 'metric-good' }
  if (type === 'flagged') { return (v.flagged_tools || 0) > 3 ? 'metric-bad' : (v.flagged_tools || 0) > 0 ? 'metric-warn' : 'metric-good' }
  return ''
}

function compareClass(newer, older, type) {
  if (type === 'canary') { const n = parseFloat(canaryRate(newer)), o = parseFloat(canaryRate(older)); return n < o ? 'vm-improved' : n > o ? 'vm-degraded' : '' }
  if (type === 'error') { return newer.error_rate < older.error_rate ? 'vm-improved' : newer.error_rate > older.error_rate ? 'vm-degraded' : '' }
  return ''
}

function canaryArrow(newer, older) { const n = parseFloat(canaryRate(newer)), o = parseFloat(canaryRate(older)); return n < o ? '↓' : n > o ? '↑' : '→' }
function errorArrow(newer, older) { return newer.error_rate < older.error_rate ? '↓' : newer.error_rate > older.error_rate ? '↑' : '→' }

function getVerdict(newer, older) {
  let imp = 0, deg = 0
  if (parseFloat(canaryRate(newer)) < parseFloat(canaryRate(older))) imp++; else if (parseFloat(canaryRate(newer)) > parseFloat(canaryRate(older))) deg++
  if (newer.error_rate < older.error_rate) imp++; else if (newer.error_rate > older.error_rate) deg++
  return imp > deg ? 'improved' : deg > imp ? 'degraded' : 'neutral'
}

function verdictLabel(v) { return v === 'improved' ? '✅ 改善' : v === 'degraded' ? '⚠️ 退化' : '➡️ 持平' }
function rateImproved(o, n) { if (n < o) return 'mc-improved'; if (n > o) return 'mc-degraded'; return '' }

function changeLabel(oldRate, newRate) {
  if (!oldRate && !newRate) return ''
  if (oldRate === 0) return newRate > 0 ? '⚠️' : ''
  const pct = ((newRate - oldRate) / oldRate * 100).toFixed(0)
  return newRate < oldRate ? `✅ ${pct}%` : newRate > oldRate ? `⚠️ +${pct}%` : ''
}

function fmtTime(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? ts : d.toLocaleString('zh-CN', { hour12: false })
}

function fmtTimeRelative(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  const diff = Date.now() - d.getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return '刚刚'
  if (mins < 60) return `${mins}分钟前`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}小时前`
  const days = Math.floor(hours / 24)
  if (days === 0) return '今天'
  if (days === 1) return '昨天'
  return `${days}天前`
}

// Close tag menu on outside click
function onDocClick(e) {
  if (tagMenuHash.value && !e.target.closest('.tag-dropdown')) tagMenuHash.value = null
}
onMounted(() => { loadData(); document.addEventListener('click', onDocClick) })
onBeforeUnmount(() => { document.removeEventListener('click', onDocClick) })
</script>

<style scoped>
.prompt-tracker-page { }
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 16px; }
@media (max-width: 900px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } }

.current-card { border-left: 3px solid var(--color-primary); }
.current-info { padding: 16px; }
.current-row { display: flex; gap: 24px; flex-wrap: wrap; margin-bottom: 12px; }
.current-label { font-size: 11px; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 4px; }
.current-value { font-size: 14px; color: var(--text-primary); font-weight: 600; }
.current-metrics { display: flex; gap: 12px; flex-wrap: wrap; }
.metric-chip { display: flex; flex-direction: column; align-items: center; padding: 8px 16px; border-radius: var(--radius-md); background: var(--bg-elevated); border: 1px solid var(--border-subtle); }
.metric-label { font-size: 10px; color: var(--text-tertiary); margin-bottom: 2px; }
.metric-value { font-size: 16px; font-weight: 700; color: var(--text-primary); }
.metric-good { border-color: var(--color-success); } .metric-good .metric-value { color: var(--color-success); }
.metric-warn { border-color: var(--color-warning); } .metric-warn .metric-value { color: var(--color-warning); }
.metric-bad { border-color: var(--color-error); } .metric-bad .metric-value { color: var(--color-error); }

.version-badge { margin-left: auto; padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: 600; background: var(--color-primary-dim); color: var(--color-primary); }
.version-count { margin-left: auto; font-size: 12px; color: var(--text-tertiary); }

/* Tag Pills */
.tag-pill { padding: 2px 10px; border-radius: 12px; font-size: 11px; font-weight: 600; display: inline-flex; align-items: center; gap: 4px; }
.tag-pill.tag-sm { padding: 1px 8px; font-size: 10px; }
.tag-production { background: rgba(34, 197, 94, 0.15); color: #22c55e; }
.tag-staging { background: rgba(59, 130, 246, 0.15); color: #3b82f6; }
.tag-deprecated { background: rgba(107, 114, 128, 0.15); color: #6b7280; }

/* Filter Bar */
.filter-bar { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; padding: 12px 16px; border-bottom: 1px solid var(--border-subtle); }
.search-box { position: relative; flex: 1; min-width: 200px; }
.search-icon { position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: var(--text-tertiary); pointer-events: none; }
.search-input { width: 100%; padding: 6px 30px 6px 32px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); background: var(--bg-elevated); color: var(--text-primary); font-size: 13px; outline: none; transition: border-color var(--transition-fast); }
.search-input:focus { border-color: var(--color-primary); }
.search-input::placeholder { color: var(--text-tertiary); }
.search-clear { position: absolute; right: 8px; top: 50%; transform: translateY(-50%); background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 14px; padding: 2px 4px; }
.search-clear:hover { color: var(--text-primary); }

.filter-chips { display: flex; gap: 6px; }
.chip { padding: 4px 12px; border-radius: 16px; font-size: 12px; border: 1px solid var(--border-subtle); background: transparent; color: var(--text-secondary); cursor: pointer; display: flex; align-items: center; gap: 5px; transition: all var(--transition-fast); }
.chip:hover { border-color: var(--color-primary); color: var(--text-primary); }
.chip-active { background: var(--color-primary-dim); border-color: var(--color-primary); color: var(--color-primary); font-weight: 600; }
.chip-dot { width: 6px; height: 6px; border-radius: 50%; }
.dot-all { background: var(--text-tertiary); }
.dot-production { background: #22c55e; }
.dot-staging { background: #3b82f6; }
.dot-deprecated { background: #6b7280; }
.dot-none { background: var(--border-subtle); }

.sort-control { margin-left: auto; }
.sort-select { padding: 4px 8px; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--bg-elevated); color: var(--text-secondary); font-size: 12px; outline: none; cursor: pointer; }

/* Version List */
.version-list { padding: 0 16px 16px; }
.version-item { padding: 12px 16px; margin-bottom: 8px; border-radius: var(--radius-md); border: 1px solid var(--border-subtle); background: var(--bg-elevated); transition: all var(--transition-fast); }
.version-item:hover { border-color: var(--color-primary); }
.version-current { border-left: 3px solid var(--color-primary); }
.version-deprecated { opacity: 0.65; }
.version-expanded { border-color: var(--color-primary); box-shadow: 0 0 0 1px var(--color-primary-dim); }

.version-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; flex-wrap: wrap; gap: 8px; cursor: pointer; user-select: none; }
.version-left { display: flex; align-items: center; gap: 8px; }
.version-right { display: flex; align-items: center; gap: 12px; font-size: 12px; color: var(--text-tertiary); }
.version-num { font-weight: 700; font-size: 14px; color: var(--text-primary); }
.version-hash { font-size: 12px; color: var(--text-secondary); }
.version-tokens { font-size: 11px; color: var(--text-tertiary); background: var(--bg-surface); padding: 1px 6px; border-radius: 4px; }
.version-calls { }

.expand-arrow { display: flex; transition: transform 0.2s ease; color: var(--text-tertiary); }
.arrow-open { transform: rotate(90deg); }

.tag { padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.tag-current { background: var(--color-primary-dim); color: var(--color-primary); }

.version-metrics { display: flex; align-items: center; gap: 16px; margin-bottom: 8px; flex-wrap: wrap; }
.vm-item { font-size: 12px; color: var(--text-secondary); display: flex; gap: 4px; }
.vm-arrow { font-weight: 700; }
.vm-improved { color: var(--color-success); } .vm-improved .vm-arrow { color: var(--color-success); }
.vm-degraded { color: var(--color-error); } .vm-degraded .vm-arrow { color: var(--color-error); }
.vm-initial { font-size: 12px; color: var(--text-tertiary); font-style: italic; }
.vm-verdict { font-size: 12px; font-weight: 600; }
.verdict-improved { color: var(--color-success); }
.verdict-degraded { color: var(--color-error); }
.verdict-neutral { color: var(--text-tertiary); }

.version-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.btn-sm { padding: 4px 12px; font-size: 11px; border-radius: var(--radius-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: transparent; color: var(--text-secondary); transition: all var(--transition-fast); display: inline-flex; align-items: center; gap: 4px; }
.btn-sm:hover { background: var(--bg-surface); color: var(--text-primary); border-color: var(--color-primary); }
.btn-sm.btn-active { background: var(--color-primary-dim); color: var(--color-primary); border-color: var(--color-primary); }
.btn-sm.btn-ghost { border-color: transparent; }
.btn-sm.btn-ghost:hover { background: var(--bg-elevated); }
.btn-sm.btn-warn { color: var(--color-warning); border-color: rgba(245, 158, 11, 0.3); }
.btn-sm.btn-warn:hover { background: rgba(245, 158, 11, 0.1); border-color: var(--color-warning); }

/* Tag Dropdown */
.tag-dropdown { position: relative; }
.tag-menu { position: absolute; top: 100%; left: 0; z-index: 100; margin-top: 4px; min-width: 160px; background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); box-shadow: var(--shadow-lg); padding: 4px; }
.tag-menu-item { display: flex; align-items: center; gap: 8px; width: 100%; padding: 6px 10px; border: none; background: transparent; color: var(--text-secondary); font-size: 12px; cursor: pointer; border-radius: var(--radius-sm); transition: background var(--transition-fast); }
.tag-menu-item:hover { background: var(--bg-elevated); color: var(--text-primary); }
.tag-menu-active { color: var(--color-primary); font-weight: 600; }
.tag-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.tag-check { margin-left: auto; color: var(--color-primary); }

/* Version Expand */
.version-expand { margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--border-subtle); animation: slideDown 0.2s ease; }
@keyframes slideDown { from { opacity: 0; max-height: 0; } to { opacity: 1; max-height: 500px; } }
.expand-title { font-size: 12px; font-weight: 600; color: var(--text-secondary); margin-bottom: 8px; text-transform: uppercase; letter-spacing: 0.05em; }
.prompt-content-block { background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 12px; overflow-x: auto; max-height: 200px; overflow-y: auto; }
.prompt-content-block pre { font-family: var(--font-mono); font-size: 12px; color: var(--text-primary); white-space: pre-wrap; word-break: break-word; margin: 0; }
.expand-metrics-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin-top: 12px; }
@media (max-width: 600px) { .expand-metrics-grid { grid-template-columns: repeat(2, 1fr); } }
.em-item { display: flex; justify-content: space-between; padding: 6px 10px; background: var(--bg-base); border-radius: var(--radius-sm); border: 1px solid var(--border-subtle); }
.em-label { font-size: 11px; color: var(--text-tertiary); }
.em-value { font-size: 12px; font-weight: 600; color: var(--text-primary); font-family: var(--font-mono); }
.em-danger { color: var(--color-error); }
.em-warn { color: var(--color-warning); }
.em-safe { color: var(--color-success); }

.empty-filter { display: flex; align-items: center; justify-content: center; gap: 8px; padding: 32px; color: var(--text-tertiary); font-size: 13px; }
.empty-filter-icon { font-size: 20px; }

/* Modal */
.modal-overlay { position: fixed; inset: 0; z-index: 1000; background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center; animation: fadeIn 0.2s; }
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
.modal-box { background: var(--bg-surface); border-radius: var(--radius-lg); border: 1px solid var(--border-subtle); box-shadow: var(--shadow-lg); max-height: 85vh; overflow: auto; width: 90%; max-width: 700px; animation: slideUp 0.2s ease-out; }
@keyframes slideUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
.modal-lg { max-width: 800px; }
.modal-xl { max-width: 1100px; }
.modal-header { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid var(--border-subtle); font-weight: 600; font-size: 15px; gap: 12px; }
.modal-header-actions { display: flex; align-items: center; gap: 8px; margin-left: auto; }
.modal-close { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 18px; padding: 4px 8px; border-radius: var(--radius-sm); }
.modal-close:hover { background: var(--bg-elevated); color: var(--text-primary); }
.modal-body { padding: 20px; }

/* Detail Modal */
.detail-meta { display: flex; gap: 16px; font-size: 12px; color: var(--text-secondary); margin-bottom: 16px; flex-wrap: wrap; }
.prompt-content { background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); margin-bottom: 16px; overflow-x: auto; }
.prompt-content-header { display: flex; justify-content: space-between; align-items: center; padding: 8px 16px; border-bottom: 1px solid var(--border-subtle); font-size: 12px; font-weight: 600; color: var(--text-secondary); }
.prompt-content pre { font-family: var(--font-mono); font-size: 13px; color: var(--text-primary); white-space: pre-wrap; word-break: break-word; margin: 0; padding: 16px; }
.detail-metrics { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 8px; }
.dm-item { display: flex; justify-content: space-between; padding: 8px 12px; background: var(--bg-elevated); border-radius: var(--radius-sm); }
.dm-label { font-size: 12px; color: var(--text-secondary); }
.dm-value { font-size: 12px; font-weight: 600; color: var(--text-primary); }
.dm-danger { color: var(--color-error); }
.dm-warn { color: var(--color-warning); }

/* Diff View */
.diff-view-toggle { display: flex; gap: 6px; margin-bottom: 12px; }
.diff-split { display: grid; grid-template-columns: 1fr 1fr; gap: 0; margin-bottom: 16px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); overflow: hidden; }
.diff-pane { min-width: 0; }
.diff-pane-old { border-right: 1px solid var(--border-subtle); }
.diff-pane-header { padding: 8px 12px; font-size: 12px; font-weight: 600; color: var(--text-secondary); background: var(--bg-elevated); border-bottom: 1px solid var(--border-subtle); }
.diff-pane-body { font-family: var(--font-mono); font-size: 12px; overflow-x: auto; max-height: 400px; overflow-y: auto; }

.diff-block { background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 8px 0; margin-bottom: 16px; font-family: var(--font-mono); font-size: 13px; overflow-x: auto; max-height: 400px; overflow-y: auto; }
.diff-line { display: flex; padding: 1px 12px; line-height: 1.6; }
.diff-linenum { width: 28px; flex-shrink: 0; text-align: right; color: var(--text-tertiary); font-size: 11px; padding-right: 8px; user-select: none; }
.diff-prefix { width: 16px; flex-shrink: 0; font-weight: 700; user-select: none; }
.diff-content { white-space: pre-wrap; word-break: break-word; }
.diff-added { background: rgba(34, 197, 94, 0.12); color: #22c55e; }
.diff-added .diff-prefix { color: #22c55e; }
.diff-removed { background: rgba(239, 68, 68, 0.12); color: #ef4444; }
.diff-removed .diff-prefix { color: #ef4444; }
.diff-unchanged { color: var(--text-secondary); }

.diff-summary { display: flex; gap: 16px; margin-bottom: 16px; font-size: 13px; }
.diff-stat { font-weight: 600; font-family: var(--font-mono); }
.diff-stat-add { color: #22c55e; }
.diff-stat-del { color: #ef4444; }
.diff-stat-eq { color: var(--text-tertiary); }

.diff-header-pills { display: flex; align-items: center; gap: 8px; }
.diff-version-pill { padding: 2px 10px; border-radius: 12px; font-size: 11px; font-weight: 600; font-family: var(--font-mono); }
.diff-old { background: rgba(239, 68, 68, 0.12); color: #ef4444; }
.diff-new { background: rgba(34, 197, 94, 0.12); color: #22c55e; }

/* Metrics Comparison */
.metrics-compare { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 16px; }
.mc-title { font-size: 13px; font-weight: 600; color: var(--text-primary); margin-bottom: 12px; }
.mc-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
@media (max-width: 600px) { .mc-grid { grid-template-columns: 1fr; } }
.mc-card { background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); padding: 10px 14px; }
.mc-card-label { font-size: 11px; color: var(--text-tertiary); margin-bottom: 4px; }
.mc-card-values { display: flex; align-items: center; gap: 6px; font-size: 14px; }
.mc-old { color: var(--text-tertiary); }
.mc-arrow { color: var(--text-tertiary); font-size: 12px; }
.mc-new { font-weight: 600; color: var(--text-primary); }
.mc-improved { color: var(--color-success) !important; }
.mc-degraded { color: var(--color-error) !important; }
.mc-change { font-size: 11px; font-weight: 600; }
.mc-verdict { margin-top: 12px; padding: 8px 16px; text-align: center; font-size: 14px; font-weight: 700; border-radius: var(--radius-md); }
.mc-verdict.verdict-improved { background: rgba(34,197,94,0.12); color: #22c55e; }
.mc-verdict.verdict-degraded { background: rgba(239,68,68,0.12); color: #ef4444; }
.mc-verdict.verdict-neutral { background: var(--bg-surface); color: var(--text-tertiary); }

/* Loading & Empty */
.loading-state { display: flex; align-items: center; justify-content: center; gap: 8px; padding: 60px 0; color: var(--text-secondary); }
.loading-spinner { width: 20px; height: 20px; border: 2px solid var(--border-subtle); border-top-color: var(--color-primary); border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.empty-state { text-align: center; padding: 60px 20px; }
.empty-icon { font-size: 48px; margin-bottom: 12px; }
.empty-title { font-size: 16px; font-weight: 600; color: var(--text-primary); margin-bottom: 8px; }
.empty-desc { font-size: 13px; color: var(--text-secondary); line-height: 1.6; }
.mono { font-family: var(--font-mono); }
</style>
