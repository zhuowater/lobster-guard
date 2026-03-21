<template>
  <div class="taint-page">
    <!-- 页头 -->
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="biohazard" :size="20" /> 污染追踪</h1>
        <p class="page-subtitle">数据流污染标签传播追踪 + 自动逆转引擎 — 阻止 PII 与凭据泄漏</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-sm btn-danger-outline" @click="showCleanup = true" :disabled="!stats.active_count">
          <Icon name="trash" :size="14" /> 清理过期
        </button>
        <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
      </div>
    </div>

    <!-- 统计面板 StatCard -->
    <div class="ov-cards" v-if="loaded">
      <StatCard :iconSvg="svgBiohazard" :value="stats.active_count ?? 0" label="活跃标记数" color="green" />
      <StatCard :iconSvg="svgAlert" :value="stats.total_blocked ?? 0" label="泄露拦截数" color="red" />
      <StatCard :iconSvg="svgUsers" :value="stats.total_marked ?? 0" label="累计标记数" color="blue" />
      <StatCard :iconSvg="svgClock" :value="lastLeakDisplay" label="最近泄露" color="yellow" />
    </div>
    <div class="ov-cards" v-else>
      <Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" />
    </div>

    <!-- 标签分布 -->
    <div class="label-dist" v-if="loaded && labelDistItems.length">
      <span class="label-dist-title">标签分布</span>
      <span v-for="item in labelDistItems" :key="item.label" class="taint-badge" :class="'taint-' + normLabel(item.label)">
        {{ item.label }} <b>{{ item.count }}</b>
      </span>
    </div>

    <!-- Tab 切换 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'active' }" @click="activeTab = 'active'">
        <Icon name="biohazard" :size="14" /> 活跃标记 <span class="tab-count">{{ activeTaints.length }}</span>
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'leaks' }" @click="activeTab = 'leaks'">
        <Icon name="alert" :size="14" /> 泄露检测 <span class="tab-count">{{ leakRecords.length }}</span>
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'scan' }" @click="activeTab = 'scan'">
        <Icon name="search" :size="14" /> 实时扫描
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'reversal' }" @click="activeTab = 'reversal'">
        <Icon name="refresh" :size="14" /> 逆转记录 <span class="tab-count">{{ reversals.length }}</span>
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'config' }" @click="activeTab = 'config'">
        <Icon name="settings" :size="14" /> 配置
      </button>
    </div>

    <!-- Tab: 活跃标记 -->
    <div v-if="activeTab === 'active'" class="section">
      <div class="toolbar">
        <div class="toolbar-left">
          <div class="search-box">
            <Icon name="search" :size="14" />
            <input v-model="activeFilter.keyword" class="search-input" placeholder="搜索 TraceID / 来源 / 详情..." />
          </div>
          <select v-model="activeFilter.label" class="filter-select">
            <option value="">全部标签</option>
            <option v-for="l in allLabels" :key="l" :value="l">{{ l }}</option>
          </select>
          <select v-model="activeFilter.source" class="filter-select">
            <option value="">全部来源</option>
            <option v-for="s in allSources" :key="s" :value="s">{{ s }}</option>
          </select>
        </div>
        <button class="btn btn-sm btn-primary" @click="showInjectModal = true">
          <Icon name="plus" :size="14" /> 注入标记
        </button>
      </div>

      <div class="table-wrap">
        <table class="data-table" v-if="filteredActive.length">
          <thead>
            <tr>
              <th style="width:24px"></th>
              <th>TraceID</th>
              <th>标签</th>
              <th>来源</th>
              <th>详情</th>
              <th>时间</th>
              <th>传播</th>
              <th>状态</th>
              <th style="width:60px">操作</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="(t, idx) in filteredActive" :key="t.trace_id || idx">
              <tr @click="toggleExpand(idx)" class="row-clickable">
                <td class="td-expand">{{ expandedIdx === idx ? '▼' : '▶' }}</td>
                <td class="td-mono">{{ truncate(t.trace_id, 16) }}</td>
                <td>
                  <span v-for="l in (t.labels || [t.label])" :key="l" class="taint-badge" :class="'taint-' + normLabel(l)">{{ l }}</span>
                </td>
                <td>{{ t.source || '-' }}</td>
                <td>{{ truncate(t.source_detail || t.detail || t.details, 30) }}</td>
                <td class="td-mono">{{ formatTime(t.timestamp || t.time) }}</td>
                <td class="td-mono">{{ (t.propagations || []).length }}</td>
                <td>
                  <span class="status-badge status-active" v-if="!isExpired(t)">活跃</span>
                  <span class="status-badge status-expired" v-else>过期</span>
                </td>
                <td>
                  <button class="btn-icon btn-icon-danger" @click.stop="confirmDeleteEntry(t)" title="删除">
                    <Icon name="trash" :size="14" />
                  </button>
                </td>
              </tr>
              <tr v-if="expandedIdx === idx" class="row-expanded">
                <td colspan="9">
                  <div class="propagation-detail">
                    <h4 class="detail-title">传播路径</h4>
                    <div class="propagation-chain" v-if="t.propagations && t.propagations.length">
                      <div v-for="(p, pi) in t.propagations" :key="pi" class="chain-node">
                        <div class="chain-dot" :class="'chain-dot-' + stageColor(p.stage)"></div>
                        <div class="chain-info">
                          <div class="chain-stage-name">{{ stageDisplayName(p.stage) }}</div>
                          <div class="chain-meta">
                            <span class="action-badge" :class="'action-' + p.action">{{ p.action }}</span>
                            <span v-if="p.label" class="taint-badge taint-sm" :class="'taint-' + normLabel(p.label)">{{ p.label }}</span>
                            <span class="chain-time">{{ formatTime(p.timestamp) }}</span>
                          </div>
                          <div class="chain-detail" v-if="p.detail">{{ p.detail }}</div>
                        </div>
                        <div v-if="pi < t.propagations.length - 1" class="chain-connector"></div>
                      </div>
                    </div>
                    <div v-else class="no-chain">暂无传播记录</div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
        <EmptyState v-else :iconSvg="svgBiohazard" title="暂无活跃标记" description="当 LLM 代理拦截到包含 PII/凭据的数据时，标记将自动出现" />
      </div>
    </div>

    <!-- Tab: 泄露检测 -->
    <div v-if="activeTab === 'leaks'" class="section">
      <div class="toolbar">
        <div class="toolbar-left">
          <div class="search-box">
            <Icon name="search" :size="14" />
            <input v-model="leakFilter.keyword" class="search-input" placeholder="搜索..." />
          </div>
          <select v-model="leakFilter.label" class="filter-select">
            <option value="">全部标签</option>
            <option v-for="l in allLabels" :key="l" :value="l">{{ l }}</option>
          </select>
          <select v-model="leakFilter.action" class="filter-select">
            <option value="">全部动作</option>
            <option value="block">拦截 (block)</option>
            <option value="warn">告警 (warn)</option>
          </select>
        </div>
      </div>

      <div class="table-wrap">
        <table class="data-table" v-if="filteredLeaks.length">
          <thead>
            <tr>
              <th>TraceID</th>
              <th>标签</th>
              <th>检测位置</th>
              <th>动作</th>
              <th>原始注入</th>
              <th>检测时间</th>
              <th>详情</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(leak, idx) in filteredLeaks" :key="idx" class="leak-row">
              <td class="td-mono">{{ truncate(leak.trace_id, 16) }}</td>
              <td>
                <span v-for="l in (leak.labels || [])" :key="l" class="taint-badge" :class="'taint-' + normLabel(l)">{{ l }}</span>
              </td>
              <td>{{ leak.detected_at || 'outbound' }}</td>
              <td>
                <span class="action-badge" :class="'action-' + leak.action">{{ leak.action }}</span>
              </td>
              <td>{{ leak.source || '-' }}</td>
              <td class="td-mono">{{ formatTime(leak.timestamp) }}</td>
              <td>{{ truncate(leak.detail || leak.reason, 40) }}</td>
            </tr>
          </tbody>
        </table>
        <EmptyState v-else :iconSvg="svgShield" title="未检测到泄露" description="所有污染标记均在安全范围内，未发生出站泄露" />
      </div>
    </div>

    <!-- Tab: 实时扫描 -->
    <div v-if="activeTab === 'scan'" class="section">
      <div class="test-panel">
        <h3 class="section-title"><Icon name="search" :size="16" /> 实时污染扫描</h3>
        <p class="section-desc">输入文本检测是否包含 PII/凭据等敏感信息</p>
        <textarea v-model="scanText" class="test-input" rows="4" placeholder="粘贴待检测的文本内容..."></textarea>
        <button class="btn btn-primary" @click="scanTaint" :disabled="scanning || !scanText.trim()" style="margin-top: var(--space-2)">
          <span v-if="scanning" class="spinner"></span>
          {{ scanning ? '扫描中...' : '开始扫描' }}
        </button>
      </div>
      <div v-if="scanResult" class="scan-result">
        <div class="result-header">
          <span class="result-title">扫描结果</span>
          <button class="btn-close" @click="scanResult = null">✕</button>
        </div>
        <div class="taint-status" :class="scanResult.tainted ? 'tainted-yes' : 'tainted-no'">
          {{ scanResult.tainted ? '⚠️ 检测到污染' : '✅ 未检测到污染' }}
        </div>
        <div v-if="scanResult.labels && scanResult.labels.length" class="result-tags">
          <strong>匹配标签：</strong>
          <span v-for="l in scanResult.labels" :key="l" class="taint-badge" :class="'taint-' + normLabel(l)">{{ l }}</span>
        </div>
        <div v-if="scanResult.matches && scanResult.matches.length" class="result-tags">
          <strong>PII 类型：</strong>
          <span v-for="p in scanResult.matches" :key="p" class="pii-badge">{{ p }}</span>
        </div>
        <div class="result-meta">匹配 {{ scanResult.patterns || 0 }} 个模式规则</div>
      </div>
    </div>

    <!-- Tab: 逆转记录 -->
    <div v-if="activeTab === 'reversal'" class="section">
      <div class="table-wrap">
        <table class="data-table" v-if="reversals.length">
          <thead>
            <tr>
              <th>TraceID</th>
              <th>模板</th>
              <th>逆转模式</th>
              <th>原始长度</th>
              <th>逆转后长度</th>
              <th>时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, idx) in reversals" :key="idx">
              <td class="td-mono">{{ truncate(r.trace_id, 16) }}</td>
              <td>{{ r.template || '-' }}</td>
              <td><span class="mode-badge" :class="'mode-' + r.mode">{{ modeDisplayName(r.mode) }}</span></td>
              <td class="td-mono">{{ r.original_length ?? '-' }}</td>
              <td class="td-mono">{{ r.reversed_length ?? '-' }}</td>
              <td class="td-mono">{{ formatTime(r.timestamp || r.time) }}</td>
            </tr>
          </tbody>
        </table>
        <EmptyState v-else :iconSvg="svgRefresh" title="暂无逆转记录" description="当污染数据被逆转引擎处理后，记录将在此显示" />
      </div>
    </div>

    <!-- Tab: 配置 -->
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-columns">
        <div class="config-panel">
          <h3 class="section-title"><Icon name="biohazard" :size="16" /> 污染追踪配置</h3>
          <div class="config-field">
            <label class="field-label">检测动作</label>
            <select v-model="taintConfig.action" class="field-select">
              <option value="block">拦截 (block) — 直接阻断</option>
              <option value="warn">告警 (warn) — 放行但告警</option>
              <option value="log">记录 (log) — 仅记录</option>
            </select>
            <span class="field-hint">出站检测到污染时的处理方式</span>
          </div>
          <div class="config-field">
            <label class="field-label">标记存活时间 (TTL)</label>
            <div class="input-group">
              <input v-model.number="taintConfig.ttl_minutes" class="field-input" type="number" min="1" max="10080" placeholder="60">
              <span class="input-suffix">分钟</span>
            </div>
            <span class="field-hint">污染标记的最大存活时间，超过后自动过期</span>
          </div>
          <div class="config-field">
            <label class="field-label">自动清理策略</label>
            <select v-model="autoCleanStrategy" class="field-select">
              <option value="auto">自动清理 — 每分钟清理过期标记</option>
              <option value="manual">手动清理 — 需手动触发清理</option>
            </select>
            <span class="field-hint">系统内置每分钟自动清理，建议保持默认</span>
          </div>
          <button class="btn btn-primary btn-sm" @click="saveTaintConfig" :disabled="savingTaint" style="margin-top: var(--space-2)">
            <span v-if="savingTaint" class="spinner"></span>
            {{ savingTaint ? '保存中...' : '保存配置' }}
          </button>
        </div>
        <div class="config-panel">
          <h3 class="section-title"><Icon name="refresh" :size="16" /> 逆转引擎配置</h3>
          <div class="config-field">
            <label class="field-label">逆转模式</label>
            <select v-model="reversalConfig.mode" class="field-select">
              <option value="soft">soft — 标记脱敏（保留结构，替换敏感值）</option>
              <option value="hard">hard — 完全移除（彻底删除污染内容）</option>
              <option value="stealth">stealth — 静默替换（无感知替换）</option>
            </select>
            <span class="field-hint">检测到泄露时对数据的处理方式</span>
          </div>
          <div class="config-field" v-if="reversalConfig.templates && reversalConfig.templates.length">
            <label class="field-label">逆转模板</label>
            <div class="template-list">
              <span v-for="t in reversalConfig.templates" :key="t" class="template-tag">{{ t }}</span>
            </div>
          </div>
          <button class="btn btn-primary btn-sm" @click="saveReversalConfig" :disabled="savingReversal" style="margin-top: var(--space-2)">
            <span v-if="savingReversal" class="spinner"></span>
            {{ savingReversal ? '保存中...' : '保存配置' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 错误 Banner -->
    <div v-if="error" class="error-banner">
      <span>⚠️ {{ error }}</span>
      <button class="btn-close" @click="error = ''">✕</button>
    </div>

    <!-- 清理确认 Modal -->
    <ConfirmModal
      :visible="showCleanup"
      title="清理过期标记"
      message="将立即清理所有已超过 TTL 存活时间的污染标记。此操作不可逆，确认继续？"
      type="danger"
      confirmText="确认清理"
      @confirm="doCleanup"
      @cancel="showCleanup = false"
    />

    <!-- 删除确认 Modal -->
    <ConfirmModal
      :visible="showDeleteConfirm"
      title="删除污染标记"
      :message="deleteConfirmMsg"
      type="danger"
      confirmText="确认删除"
      @confirm="doDeleteEntry"
      @cancel="showDeleteConfirm = false"
    />

    <!-- 注入标记 Modal -->
    <Teleport to="body">
      <div v-if="showInjectModal" class="modal-overlay" @click.self="showInjectModal = false">
        <div class="modal-box">
          <div class="modal-header">
            <span class="modal-icon">🏷️</span>
            <span class="modal-title">注入污染标记</span>
          </div>
          <div class="modal-body-form">
            <div class="config-field">
              <label class="field-label">标签（多选）</label>
              <div class="inject-labels">
                <label v-for="l in injectLabelOptions" :key="l" class="checkbox-label">
                  <input type="checkbox" :value="l" v-model="injectForm.labels" />
                  <span class="taint-badge" :class="'taint-' + normLabel(l)">{{ l }}</span>
                </label>
              </div>
            </div>
            <div class="config-field">
              <label class="field-label">来源</label>
              <select v-model="injectForm.source" class="field-select">
                <option value="manual">手动注入</option>
                <option value="inbound">入站检测</option>
                <option value="llm">LLM 输出</option>
                <option value="toolcall">工具调用</option>
              </select>
            </div>
            <div class="config-field">
              <label class="field-label">详情</label>
              <input v-model="injectForm.detail" class="field-input" placeholder="补充说明（可选）" />
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-sm" @click="showInjectModal = false">取消</button>
            <button class="btn btn-sm btn-primary" @click="doInject" :disabled="!injectForm.labels.length || injecting">
              <span v-if="injecting" class="spinner"></span>
              {{ injecting ? '注入中...' : '注入' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import Skeleton from '../components/Skeleton.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import EmptyState from '../components/EmptyState.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'

// SVG Icons for StatCard
const svgBiohazard = '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="2"/><path d="M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10 10-4.5 10-10S17.5 2 12 2"/></svg>'
const svgAlert = '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgUsers = '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>'
const svgClock = '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>'
const svgShield = '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>'
const svgRefresh = '<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>'

// State
const activeTab = ref('active')
const loaded = ref(false)
const stats = ref({})
const activeTaints = ref([])
const leakRecords = ref([])
const reversals = ref([])
const error = ref('')
const expandedIdx = ref(-1)

const scanText = ref('')
const scanning = ref(false)
const scanResult = ref(null)

const taintConfig = reactive({ action: 'warn', ttl_minutes: 60 })
const reversalConfig = reactive({ mode: 'soft', templates: [] })
const autoCleanStrategy = ref('auto')
const savingTaint = ref(false)
const savingReversal = ref(false)

const showCleanup = ref(false)
const showDeleteConfirm = ref(false)
const deleteTarget = ref(null)
const showInjectModal = ref(false)
const injecting = ref(false)
const injectForm = reactive({ labels: [], source: 'manual', detail: '' })
const injectLabelOptions = ['PII-TAINTED', 'CREDENTIAL-TAINTED', 'CONFIDENTIAL', 'INTERNAL-ONLY']

const activeFilter = reactive({ keyword: '', label: '', source: '' })
const leakFilter = reactive({ keyword: '', label: '', action: '' })

// Computed
const labelDistItems = computed(() => {
  const dist = stats.value.label_distribution || {}
  return Object.entries(dist).map(([label, count]) => ({ label, count })).sort((a, b) => b.count - a.count)
})

const allLabels = computed(() => {
  const s = new Set()
  activeTaints.value.forEach(t => (t.labels || []).forEach(l => s.add(l)))
  return [...s]
})

const allSources = computed(() => {
  const s = new Set()
  activeTaints.value.forEach(t => { if (t.source) s.add(t.source) })
  return [...s]
})

const lastLeakDisplay = computed(() => {
  if (!leakRecords.value.length) return '无'
  return formatTimeShort(leakRecords.value[0].timestamp)
})

const deleteConfirmMsg = computed(() => {
  const tid = deleteTarget.value ? deleteTarget.value.trace_id : ''
  return '确认删除标记 ' + truncate(tid, 20) + ' ？此操作不可逆。'
})

const filteredActive = computed(() => {
  let list = activeTaints.value
  const kw = activeFilter.keyword.toLowerCase()
  if (kw) {
    list = list.filter(t =>
      (t.trace_id || '').toLowerCase().includes(kw) ||
      (t.source || '').toLowerCase().includes(kw) ||
      (t.source_detail || '').toLowerCase().includes(kw)
    )
  }
  if (activeFilter.label) {
    list = list.filter(t => (t.labels || []).includes(activeFilter.label))
  }
  if (activeFilter.source) {
    list = list.filter(t => t.source === activeFilter.source)
  }
  return list
})

const filteredLeaks = computed(() => {
  let list = leakRecords.value
  const kw = leakFilter.keyword.toLowerCase()
  if (kw) {
    list = list.filter(t =>
      (t.trace_id || '').toLowerCase().includes(kw) ||
      (t.detail || '').toLowerCase().includes(kw)
    )
  }
  if (leakFilter.label) {
    list = list.filter(t => (t.labels || []).includes(leakFilter.label))
  }
  if (leakFilter.action) {
    list = list.filter(t => t.action === leakFilter.action)
  }
  return list
})

// Data Loading
async function loadStats() {
  try {
    const d = await api('/api/v1/taint/stats')
    stats.value = d || {}
    if (d.action) taintConfig.action = d.action
    if (d.ttl_minutes) taintConfig.ttl_minutes = d.ttl_minutes
  } catch (e) { error.value = '加载统计失败: ' + e.message }
}

async function loadActive() {
  try {
    const d = await api('/api/v1/taint/active')
    activeTaints.value = d.entries || d.taints || d || []
    // Extract leak records from propagations with block/warn actions
    const leaks = []
    for (const t of activeTaints.value) {
      if (!t.propagations) continue
      for (const p of t.propagations) {
        if (p.action === 'block' || p.action === 'warn') {
          leaks.push({
            trace_id: t.trace_id,
            labels: t.labels,
            detected_at: p.stage,
            action: p.action,
            source: t.source,
            timestamp: p.timestamp,
            detail: p.detail,
            reason: p.detail
          })
        }
      }
    }
    leakRecords.value = leaks.sort((a, b) => {
      const ta = new Date(a.timestamp || 0).getTime()
      const tb = new Date(b.timestamp || 0).getTime()
      return tb - ta
    })
  } catch (e) { error.value = '加载活跃污染失败: ' + e.message }
}

async function loadReversals() {
  try {
    const d = await api('/api/v1/reversal/records')
    reversals.value = d.records || d || []
  } catch (e) { error.value = '加载逆转记录失败: ' + e.message }
}

async function loadReversalConfig() {
  try {
    const d = await api('/api/v1/reversal/config')
    if (d.mode) reversalConfig.mode = d.mode
    if (d.templates) reversalConfig.templates = d.templates
  } catch (_) {}
}

// Actions
async function scanTaint() {
  scanning.value = true
  scanResult.value = null
  try {
    scanResult.value = await apiPost('/api/v1/taint/scan', { text: scanText.value })
    showToast(scanResult.value.tainted ? '⚠️ 检测到污染标记' : '✅ 文本安全', scanResult.value.tainted ? 'warning' : 'success')
  } catch (e) {
    error.value = '扫描失败: ' + e.message
    showToast('扫描失败: ' + e.message, 'error')
  } finally { scanning.value = false }
}

async function saveTaintConfig() {
  savingTaint.value = true
  try {
    await apiPut('/api/v1/taint/config', { action: taintConfig.action, ttl_minutes: taintConfig.ttl_minutes })
    showToast('✅ 污染追踪配置已保存', 'success')
    loadStats()
  } catch (e) {
    showToast('❌ 保存失败: ' + e.message, 'error')
  } finally { savingTaint.value = false }
}

async function saveReversalConfig() {
  savingReversal.value = true
  try {
    await apiPut('/api/v1/reversal/config', { mode: reversalConfig.mode })
    showToast('✅ 逆转引擎配置已保存', 'success')
  } catch (e) {
    showToast('❌ 保存失败: ' + e.message, 'error')
  } finally { savingReversal.value = false }
}

async function doCleanup() {
  showCleanup.value = false
  try {
    const d = await apiPost('/api/v1/taint/cleanup', {})
    showToast('✅ 清理完成，剩余活跃标记: ' + (d.active_count ?? 0), 'success')
    loadAll()
  } catch (e) {
    showToast('❌ 清理失败: ' + e.message, 'error')
  }
}

function confirmDeleteEntry(t) {
  deleteTarget.value = t
  showDeleteConfirm.value = true
}

async function doDeleteEntry() {
  showDeleteConfirm.value = false
  if (!deleteTarget.value) return
  const tid = deleteTarget.value.trace_id
  try {
    await apiDelete('/api/v1/taint/entry/' + encodeURIComponent(tid))
    showToast('✅ 标记已删除: ' + truncate(tid, 16), 'success')
    expandedIdx.value = -1
    loadAll()
  } catch (e) {
    showToast('❌ 删除失败: ' + e.message, 'error')
  }
}

async function doInject() {
  injecting.value = true
  try {
    const d = await apiPost('/api/v1/taint/inject', {
      labels: injectForm.labels,
      source: injectForm.source,
      detail: injectForm.detail
    })
    showToast('✅ 标记已注入: ' + (d.trace_id || ''), 'success')
    showInjectModal.value = false
    injectForm.labels = []
    injectForm.detail = ''
    loadAll()
  } catch (e) {
    showToast('❌ 注入失败: ' + e.message, 'error')
  } finally { injecting.value = false }
}

// Helpers
function toggleExpand(idx) { expandedIdx.value = expandedIdx.value === idx ? -1 : idx }

function normLabel(l) {
  const m = { 'PII-TAINTED': 'pii', 'CREDENTIAL-TAINTED': 'credential', 'CONFIDENTIAL': 'confidential', 'INTERNAL-ONLY': 'internal' }
  return m[l] || 'default'
}

function isExpired(t) {
  if (!t.timestamp) return false
  const ttl = taintConfig.ttl_minutes || 60
  const exp = new Date(t.timestamp).getTime() + ttl * 60 * 1000
  return Date.now() > exp
}

function stageColor(stage) {
  const m = { inbound: 'blue', llm_request: 'indigo', llm_response: 'purple', outbound: 'red', manual_inject: 'green' }
  return m[stage] || 'gray'
}

function stageDisplayName(stage) {
  const m = { inbound: '入站', llm_request: 'LLM 请求', llm_response: 'LLM 响应', outbound: '出站', manual_inject: '手动注入' }
  return m[stage] || stage
}

function modeDisplayName(mode) {
  const m = { soft: '标记脱敏', hard: '完全移除', stealth: '静默替换' }
  return m[mode] || mode
}

function loadAll() {
  error.value = ''
  loaded.value = false
  Promise.all([loadStats(), loadActive(), loadReversals(), loadReversalConfig()])
    .finally(() => { loaded.value = true })
}

function truncate(s, max) { return s && s.length > max ? s.slice(0, max) + '…' : s || '-' }

function formatTime(ts) {
  if (!ts) return '-'
  try {
    const d = new Date(ts)
    return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch { return ts }
}

function formatTimeShort(ts) {
  if (!ts) return '-'
  try {
    const d = new Date(ts)
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    if (diff < 60000) return '刚刚'
    if (diff < 3600000) return Math.floor(diff / 60000) + '分前'
    if (diff < 86400000) return Math.floor(diff / 3600000) + '小时前'
    return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
  } catch { return ts }
}

onMounted(loadAll)
</script>

<style scoped>
.taint-page { padding: var(--space-4); max-width: 1200px; }

/* Header */
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); flex-wrap: wrap; gap: var(--space-3); }
.page-title { font-size: var(--text-xl); font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }
.header-actions { display: flex; gap: var(--space-2); align-items: center; }

/* StatCard row */
.ov-cards { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }

/* Label distribution */
.label-dist { display: flex; flex-wrap: wrap; gap: var(--space-2); align-items: center; margin-bottom: var(--space-4); padding: var(--space-3); background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); }
.label-dist-title { font-size: var(--text-xs); font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; margin-right: var(--space-2); }

/* Tab bar */
.tab-bar { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); border-bottom: 1px solid var(--border-subtle); padding-bottom: var(--space-2); flex-wrap: wrap; }
.tab-btn { background: none; border: none; color: var(--text-secondary); font-size: var(--text-sm); padding: var(--space-2) var(--space-3); cursor: pointer; border-radius: var(--radius-md) var(--radius-md) 0 0; transition: all .2s; display: inline-flex; align-items: center; gap: 6px; }
.tab-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.tab-btn.active { color: var(--color-primary); border-bottom: 2px solid var(--color-primary); font-weight: 600; }
.tab-count { font-size: 10px; background: rgba(255,255,255,.08); padding: 1px 6px; border-radius: 9999px; font-weight: 600; }

/* Section */
.section { margin-bottom: var(--space-4); }
.section-title { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); display: flex; align-items: center; gap: 6px; }
.section-desc { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: var(--space-3); }

/* Toolbar */
.toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-3); gap: var(--space-2); flex-wrap: wrap; }
.toolbar-left { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; }
.search-box { display: flex; align-items: center; gap: 6px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 4px 10px; color: var(--text-tertiary); }
.search-input { background: none; border: none; color: var(--text-primary); font-size: var(--text-sm); outline: none; width: 200px; }
.search-input::placeholder { color: var(--text-tertiary); }
.filter-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 5px 10px; font-size: var(--text-xs); }

/* Test Panel */
.test-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); margin-bottom: var(--space-3); }
.test-input { width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-3); font-size: var(--text-sm); resize: vertical; font-family: var(--font-mono); }
.test-input:focus { outline: none; border-color: var(--color-primary); }

/* Scan Result */
.scan-result { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.result-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-3); }
.result-title { font-weight: 700; color: var(--text-primary); }
.btn-close { background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 16px; }
.btn-close:hover { color: var(--text-primary); }
.taint-status { font-size: 1.25rem; font-weight: 700; text-align: center; padding: var(--space-3); border-radius: var(--radius-md); margin-bottom: var(--space-3); }
.tainted-yes { color: #EF4444; background: rgba(239,68,68,.1); }
.tainted-no { color: #10B981; background: rgba(16,185,129,.1); }
.result-tags { display: flex; flex-wrap: wrap; gap: var(--space-2); align-items: center; font-size: var(--text-sm); color: var(--text-secondary); margin-bottom: var(--space-2); }
.result-meta { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-2); }

/* Taint badges */
.taint-badge { display: inline-flex; align-items: center; gap: 4px; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.taint-badge b { font-weight: 800; }
.taint-sm { font-size: 9px; padding: 1px 6px; }
.taint-pii { background: rgba(239,68,68,.15); color: #FCA5A5; }
.taint-credential { background: rgba(168,85,247,.15); color: #C4B5FD; }
.taint-confidential { background: rgba(245,158,11,.15); color: #FCD34D; }
.taint-internal { background: rgba(59,130,246,.15); color: #93C5FD; }
.taint-default { background: rgba(255,255,255,.1); color: var(--text-secondary); }
.pii-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; background: rgba(239,68,68,.1); color: #FCA5A5; }

/* Status badges */
.status-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.status-active { background: rgba(16,185,129,.15); color: #6EE7B7; }
.status-expired { background: rgba(107,114,128,.15); color: #9CA3AF; }

/* Action badges */
.action-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.action-block { background: rgba(239,68,68,.15); color: #FCA5A5; }
.action-warn { background: rgba(245,158,11,.15); color: #FCD34D; }
.action-log, .action-pass, .action-inject { background: rgba(107,114,128,.15); color: #9CA3AF; }

/* Mode badges */
.mode-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.mode-soft { background: rgba(16,185,129,.15); color: #6EE7B7; }
.mode-hard { background: rgba(239,68,68,.15); color: #FCA5A5; }
.mode-stealth { background: rgba(168,85,247,.15); color: #C4B5FD; }

/* Table */
.table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th { text-align: left; padding: 8px 10px; background: var(--bg-elevated); color: var(--text-tertiary); font-weight: 600; font-size: 10px; text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle); white-space: nowrap; }
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.td-expand { width: 20px; cursor: pointer; color: var(--text-tertiary); user-select: none; }
.row-clickable { cursor: pointer; }
.row-expanded td { background: var(--bg-elevated); padding: var(--space-3); }
.leak-row:hover { background: rgba(239,68,68,.05); }

/* Propagation chain (vertical timeline) */
.propagation-detail { padding: var(--space-2); }
.detail-title { font-size: var(--text-xs); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); }
.propagation-chain { position: relative; padding-left: 20px; }
.chain-node { position: relative; display: flex; gap: var(--space-3); margin-bottom: var(--space-3); }
.chain-dot { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; margin-top: 3px; }
.chain-dot-blue { background: #3B82F6; box-shadow: 0 0 8px rgba(59,130,246,.4); }
.chain-dot-indigo { background: #6366F1; box-shadow: 0 0 8px rgba(99,102,241,.4); }
.chain-dot-purple { background: #8B5CF6; box-shadow: 0 0 8px rgba(139,92,246,.4); }
.chain-dot-red { background: #EF4444; box-shadow: 0 0 8px rgba(239,68,68,.4); }
.chain-dot-green { background: #10B981; box-shadow: 0 0 8px rgba(16,185,129,.4); }
.chain-dot-gray { background: #6B7280; }
.chain-info { flex: 1; min-width: 0; }
.chain-stage-name { font-size: var(--text-xs); font-weight: 700; color: var(--text-primary); margin-bottom: 2px; }
.chain-meta { display: flex; flex-wrap: wrap; gap: var(--space-1); align-items: center; }
.chain-time { font-size: 10px; color: var(--text-tertiary); font-family: var(--font-mono); }
.chain-detail { font-size: 10px; color: var(--text-tertiary); margin-top: 2px; }
.chain-connector { position: absolute; left: 5px; top: 18px; bottom: -12px; width: 2px; background: var(--border-subtle); }
.no-chain { font-size: var(--text-xs); color: var(--text-tertiary); padding: var(--space-2); }

/* Config */
.config-columns { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
.config-panel { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); }
.config-field { margin-bottom: var(--space-3); display: flex; flex-direction: column; gap: 4px; }
.field-label { font-size: 10px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; }
.field-input { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); flex: 1; }
.field-input:focus { outline: none; border-color: var(--color-primary); }
.field-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); padding: 6px 10px; font-size: var(--text-sm); }
.field-hint { font-size: 10px; color: var(--text-tertiary); }
.input-group { display: flex; align-items: center; gap: var(--space-2); }
.input-suffix { font-size: var(--text-xs); color: var(--text-tertiary); white-space: nowrap; }
.template-list { display: flex; flex-wrap: wrap; gap: var(--space-1); }
.template-tag { display: inline-block; padding: 2px 8px; background: rgba(255,255,255,.08); border-radius: 4px; font-size: 10px; color: var(--text-secondary); }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: var(--radius-md); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); transition: all .2s; }
.btn:hover { background: var(--bg-surface); color: var(--text-primary); }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-sm { padding: 6px 12px; font-size: var(--text-xs); }
.btn-danger-outline { border-color: rgba(239,68,68,.4); color: #FCA5A5; }
.btn-danger-outline:hover:not(:disabled) { background: rgba(239,68,68,.1); border-color: #EF4444; color: #FCA5A5; }
.btn-danger-outline:disabled { opacity: .4; cursor: not-allowed; }
.btn-icon { background: none; border: none; color: var(--text-tertiary); cursor: pointer; padding: 4px; border-radius: var(--radius-sm); transition: all .2s; }
.btn-icon:hover { background: var(--bg-elevated); }
.btn-icon-danger:hover { color: #EF4444; background: rgba(239,68,68,.1); }

.spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.3); border-top-color: #fff; border-radius: 50%; animation: spin .6s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* Error banner */
.error-banner { margin-top: var(--space-3); padding: var(--space-3); background: rgba(239,68,68,.1); border: 1px solid rgba(239,68,68,.3); border-radius: var(--radius-md); color: #FCA5A5; font-size: var(--text-sm); display: flex; align-items: center; justify-content: space-between; }

/* Modal overlay (for inject) */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,.5); z-index: 1000; display: flex; align-items: center; justify-content: center; animation: fadeIn .2s; }
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
.modal-box { background: var(--bg-surface); border: 1px solid var(--border-default); border-radius: var(--radius-lg); padding: 24px; min-width: 400px; max-width: 520px; box-shadow: 0 16px 64px rgba(0,0,0,.5); animation: slideUp .2s ease-out; }
@keyframes slideUp { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
.modal-header { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.modal-icon { font-size: 1.4rem; }
.modal-title { font-size: 1.1rem; font-weight: 600; color: var(--text-primary); }
.modal-body-form { margin-bottom: 20px; }
.modal-footer { display: flex; justify-content: flex-end; gap: 8px; }

/* Inject labels */
.inject-labels { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.checkbox-label { display: flex; align-items: center; gap: 6px; cursor: pointer; }
.checkbox-label input[type="checkbox"] { accent-color: var(--color-primary); }

/* Responsive */
@media (max-width: 768px) {
  .ov-cards { grid-template-columns: repeat(2, 1fr); }
  .config-columns { grid-template-columns: 1fr; }
  .search-input { width: 140px; }
}
</style>
