<template>
  <div class="cf-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
          反事实验证引擎
        </h1>
        <p class="page-subtitle">AttriGuard 对照验证 — 构造无外部数据的对照请求，比对行为差异判断是否注入驱动</p>
      </div>
      <button class="btn btn-sm" @click="loadAll">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
        刷新
      </button>
    </div>

    <!-- Stat Cards -->
    <div class="stats-grid" v-if="stats">
      <div class="stat-card">
        <div class="stat-icon" style="background:#EEF2FF;color:#6366F1">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
        </div>
        <div class="stat-content"><div class="stat-value">{{ stats.total_verifications ?? 0 }}</div><div class="stat-label">总验证数</div></div>
      </div>
      <div class="stat-card">
        <div class="stat-icon" style="background:#FEF2F2;color:#EF4444">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
        </div>
        <div class="stat-content"><div class="stat-value">{{ stats.blocked_count ?? 0 }}</div><div class="stat-label">注入阻断</div></div>
      </div>
      <div class="stat-card">
        <div class="stat-icon" style="background:#ECFDF5;color:#10B981">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
        </div>
        <div class="stat-content"><div class="stat-value">{{ stats.allowed_count ?? 0 }}</div><div class="stat-label">用户驱动</div></div>
      </div>
      <div class="stat-card">
        <div class="stat-icon" style="background:#F5F3FF;color:#8B5CF6">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ (stats.cache_hit_rate * 100).toFixed(1) }}%</div>
          <div class="stat-label">缓存命中率</div>
        </div>
      </div>
    </div>

    <!-- Budget Bar -->
    <div class="budget-bar" v-if="stats">
      <span class="budget-label">小时预算</span>
      <div class="budget-track">
        <div class="budget-fill" :style="{width: budgetPct + '%', background: budgetPct > 80 ? '#EF4444' : '#6366F1'}"></div>
      </div>
      <span class="budget-text">{{ stats.hourly_used }} / {{ stats.hourly_budget }}</span>
    </div>

    <!-- Tabs -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{active: activeTab === 'verifications'}" @click="activeTab='verifications'">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="9" y1="21" x2="9" y2="9"/></svg>
        验证记录 ({{ verifications.length }})
      </button>
      <button class="tab-btn" :class="{active: activeTab === 'statistics'}" @click="activeTab='statistics'">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>
        统计分析
      </button>
      <button class="tab-btn" :class="{active: activeTab === 'config'}" @click="activeTab='config'">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
        配置
      </button>
    </div>

    <!-- Tab 1: Verifications -->
    <div v-if="activeTab === 'verifications'" class="section">
      <div class="filter-row">
        <select v-model="verdictFilter" class="field-select" @change="loadVerifications">
          <option value="">全部判定</option>
          <option value="USER_DRIVEN">USER_DRIVEN</option>
          <option value="INJECTION_DRIVEN">INJECTION_DRIVEN</option>
          <option value="INCONCLUSIVE">INCONCLUSIVE</option>
        </select>
        <input v-model="traceFilter" class="field-input" placeholder="Trace ID..." @keyup.enter="loadVerifications" />
        <button class="btn btn-sm btn-primary" @click="loadVerifications">查询</button>
      </div>

      <div class="data-table" v-if="verifications.length">
        <table>
          <thead>
            <tr>
              <th>时间</th>
              <th>Trace ID</th>
              <th>工具名</th>
              <th>判定</th>
              <th>归因分数</th>
              <th>延迟</th>
              <th>缓存</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="v in verifications" :key="v.id">
              <tr @click="toggleExpand(v.id)" class="row-clickable">
                <td class="text-mono text-sm">{{ formatTime(v.created_at) }}</td>
                <td class="text-mono text-sm">{{ (v.trace_id || '-').substring(0, 12) }}</td>
                <td><span class="tool-badge">{{ v.tool_name }}</span></td>
                <td><span class="verdict-badge" :class="verdictClass(v.verdict)">{{ v.verdict }}</span></td>
                <td>
                  <div class="attr-bar-wrap">
                    <div class="attr-bar" :style="{width: (v.attribution_score * 100) + '%', background: attrColor(v.attribution_score)}"></div>
                    <span class="attr-val">{{ v.attribution_score.toFixed(2) }}</span>
                  </div>
                </td>
                <td>{{ v.latency_ms }}ms</td>
                <td>
                  <svg v-if="v.cached" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#10B981" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>
                  <span v-else class="text-muted">-</span>
                </td>
              </tr>
              <tr v-if="expandedId === v.id" class="expand-row">
                <td colspan="7">
                  <div class="diff-view">
                    <div class="diff-panel">
                      <div class="diff-title">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                        原始 Messages
                      </div>
                      <pre class="diff-pre diff-original">{{ formatJSON(v.original_messages) }}</pre>
                    </div>
                    <div class="diff-panel">
                      <div class="diff-title">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><rect x="8" y="2" width="8" height="4" rx="1" ry="1"/></svg>
                        对照 Messages (移除外部数据)
                      </div>
                      <pre class="diff-pre diff-control">{{ formatJSON(v.control_messages) }}</pre>
                    </div>
                  </div>
                  <div class="detail-row">
                    <div class="detail-item"><strong>原始结果:</strong> <code>{{ v.original_result }}</code></div>
                    <div class="detail-item"><strong>对照结果:</strong> <code>{{ v.control_result }}</code></div>
                    <div class="detail-item"><strong>因果来源:</strong> {{ v.causal_driver || '-' }}</div>
                    <div class="detail-item"><strong>决策:</strong> <span :class="'decision-'+v.decision">{{ v.decision }}</span></div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
      <div v-else class="empty-state">
        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="#9CA3AF" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
        <p>暂无验证记录</p>
      </div>
    </div>

    <!-- Tab 2: Statistics -->
    <div v-if="activeTab === 'statistics'" class="section">
      <div class="chart-grid">
        <div class="chart-card">
          <h3 class="chart-title">归因分布</h3>
          <div class="pie-chart">
            <svg viewBox="0 0 200 200" width="180" height="180">
              <circle cx="100" cy="100" :r="80" fill="none" stroke="#10B981" stroke-width="30"
                :stroke-dasharray="pieSlice(stats.allowed_count, pieTotal)" stroke-dashoffset="0" />
              <circle cx="100" cy="100" :r="80" fill="none" stroke="#EF4444" stroke-width="30"
                :stroke-dasharray="pieSlice(stats.blocked_count, pieTotal)"
                :stroke-dashoffset="'-' + pieOffset(stats.allowed_count, pieTotal)" />
              <circle cx="100" cy="100" :r="80" fill="none" stroke="#F59E0B" stroke-width="30"
                :stroke-dasharray="pieSlice(stats.inconclusive_count, pieTotal)"
                :stroke-dashoffset="'-' + pieOffset(stats.allowed_count + stats.blocked_count, pieTotal)" />
            </svg>
            <div class="pie-legend">
              <div class="legend-item"><span class="legend-dot" style="background:#10B981"></span> 用户驱动 ({{ stats.allowed_count ?? 0 }})</div>
              <div class="legend-item"><span class="legend-dot" style="background:#EF4444"></span> 注入驱动 ({{ stats.blocked_count ?? 0 }})</div>
              <div class="legend-item"><span class="legend-dot" style="background:#F59E0B"></span> 不确定 ({{ stats.inconclusive_count ?? 0 }})</div>
            </div>
          </div>
        </div>
        <div class="chart-card">
          <h3 class="chart-title">关键指标</h3>
          <div class="metric-list">
            <div class="metric-item">
              <span class="metric-label">平均延迟</span>
              <span class="metric-value">{{ (stats.avg_latency_ms ?? 0).toFixed(1) }}ms</span>
            </div>
            <div class="metric-item">
              <span class="metric-label">平均归因分数</span>
              <span class="metric-value">{{ (stats.avg_attribution_score ?? 0).toFixed(3) }}</span>
            </div>
            <div class="metric-item">
              <span class="metric-label">缓存命中率</span>
              <span class="metric-value">{{ ((stats.cache_hit_rate ?? 0) * 100).toFixed(1) }}%</span>
            </div>
            <div class="metric-item">
              <span class="metric-label">预算使用率</span>
              <span class="metric-value">{{ budgetPct.toFixed(1) }}%</span>
            </div>
            <div class="metric-item">
              <span class="metric-label">模式</span>
              <span class="metric-value">{{ mode }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Tab 3: Configuration -->
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-form">
        <div class="config-row">
          <label class="config-label">启用</label>
          <label class="toggle">
            <input type="checkbox" v-model="configForm.enabled" />
            <span class="toggle-slider"></span>
          </label>
          <span class="config-hint">关闭后不再触发反事实验证</span>
        </div>
        <div class="config-row">
          <label class="config-label">模式</label>
          <select v-model="configForm.mode" class="field-select">
            <option value="sync">同步 (sync) — 验证完再决定放行/阻断</option>
            <option value="async">异步 (async) — 先放行，后台验证</option>
          </select>
        </div>
        <div class="config-row">
          <label class="config-label">每小时预算</label>
          <input v-model.number="configForm.max_per_hour" type="number" class="field-input" min="1" max="10000" />
          <span class="config-hint">滑动窗口内最大验证次数</span>
        </div>
        <div class="config-row">
          <label class="config-label">风险阈值</label>
          <input v-model.number="configForm.risk_threshold" type="number" class="field-input" min="0" max="100" step="5" />
          <span class="config-hint">风险分 >= 此值时触发验证 (0-100)</span>
        </div>
        <div class="config-row">
          <label class="config-label">缓存 TTL (秒)</label>
          <input v-model.number="configForm.cache_ttl_sec" type="number" class="field-input" min="0" max="86400" />
        </div>
        <div class="config-row">
          <label class="config-label">超时 (秒)</label>
          <input v-model.number="configForm.timeout_sec" type="number" class="field-input" min="1" max="120" />
        </div>
        <div class="config-row">
          <label class="config-label">模糊匹配</label>
          <label class="toggle">
            <input type="checkbox" v-model="configForm.fuzzy_match" />
            <span class="toggle-slider"></span>
          </label>
          <span class="config-hint">同名不同参数的 tool call 视为部分匹配</span>
        </div>
        <div class="config-actions">
          <button class="btn btn-primary" @click="saveConfig" :disabled="saving">
            {{ saving ? '保存中...' : '保存配置' }}
          </button>
          <button class="btn btn-ghost" @click="clearCache" :disabled="clearingCache">
            {{ clearingCache ? '清除中...' : '清除缓存' }}
          </button>
        </div>
        <div v-if="configMsg" class="config-msg" :class="configMsgType">{{ configMsg }}</div>
      </div>
    </div>
  </div>
</template>

<script>
import { api, apiPut, apiDelete } from '../api.js'

export default {
  name: 'Counterfactual',
  data() {
    return {
      activeTab: 'verifications',
      stats: { total_verifications: 0, blocked_count: 0, allowed_count: 0, inconclusive_count: 0, cache_hit_rate: 0, avg_latency_ms: 0, avg_attribution_score: 0, hourly_used: 0, hourly_budget: 100 },
      mode: 'async',
      verifications: [],
      expandedId: null,
      verdictFilter: '',
      traceFilter: '',
      configForm: { enabled: false, mode: 'async', max_per_hour: 100, risk_threshold: 50, cache_ttl_sec: 300, timeout_sec: 10, fuzzy_match: true },
      saving: false,
      clearingCache: false,
      configMsg: '',
      configMsgType: '',
    }
  },
  computed: {
    pieTotal() { return (this.stats.allowed_count || 0) + (this.stats.blocked_count || 0) + (this.stats.inconclusive_count || 0) || 1 },
    budgetPct() { return this.stats.hourly_budget > 0 ? (this.stats.hourly_used / this.stats.hourly_budget) * 100 : 0 },
  },
  methods: {
    async loadAll() {
      await Promise.all([this.loadStats(), this.loadVerifications(), this.loadConfig()])
    },
    async loadStats() {
      try {
        const d = await api('/api/v1/counterfactual/stats')
        if (d.stats) this.stats = d.stats
        if (d.mode) this.mode = d.mode
      } catch {}
    },
    async loadVerifications() {
      try {
        let q = '/api/v1/counterfactual/verifications?limit=100'
        if (this.verdictFilter) q += '&verdict=' + this.verdictFilter
        if (this.traceFilter) q += '&trace_id=' + encodeURIComponent(this.traceFilter)
        const d = await api(q)
        this.verifications = d.verifications || []
      } catch {}
    },
    async loadConfig() {
      try {
        const d = await api('/api/v1/counterfactual/config')
        this.configForm = { ...d }
      } catch {}
    },
    async saveConfig() {
      this.saving = true
      this.configMsg = ''
      try {
        await apiPut('/api/v1/counterfactual/config', this.configForm)
        this.configMsg = '配置已保存'
        this.configMsgType = 'msg-success'
        this.loadStats()
      } catch (e) {
        this.configMsg = '保存失败: ' + e.message
        this.configMsgType = 'msg-error'
      }
      this.saving = false
    },
    async clearCache() {
      this.clearingCache = true
      try {
        const d = await apiDelete('/api/v1/counterfactual/cache')
        this.configMsg = '缓存已清除: ' + (d.cleared || 0) + ' 条'
        this.configMsgType = 'msg-success'
      } catch (e) {
        this.configMsg = '清除失败: ' + e.message
        this.configMsgType = 'msg-error'
      }
      this.clearingCache = false
    },
    toggleExpand(id) { this.expandedId = this.expandedId === id ? null : id },
    verdictClass(v) {
      if (v === 'USER_DRIVEN') return 'verdict-user'
      if (v === 'INJECTION_DRIVEN') return 'verdict-injection'
      return 'verdict-inconclusive'
    },
    attrColor(score) {
      if (score <= 0.3) return '#10B981'
      if (score <= 0.6) return '#F59E0B'
      return '#EF4444'
    },
    formatTime(t) {
      if (!t) return '-'
      try { return new Date(t).toLocaleString('zh-CN', { hour12: false }) } catch { return t }
    },
    formatJSON(s) {
      if (!s) return ''
      try { return JSON.stringify(JSON.parse(s), null, 2) } catch { return s }
    },
    pieSlice(val, total) {
      const pct = (val || 0) / total
      const circ = 2 * Math.PI * 80
      return (circ * pct) + ' ' + circ
    },
    pieOffset(val, total) {
      const pct = (val || 0) / total
      return (2 * Math.PI * 80 * pct).toFixed(2)
    },
  },
  mounted() { this.loadAll() },
}
</script>

<style scoped>
.cf-page { max-width: 1200px; margin: 0 auto; padding: var(--space-4); }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: var(--space-4); }
.page-title { font-size: 1.5rem; font-weight: 700; display: flex; align-items: center; gap: 8px; color: var(--text-primary); }
.page-subtitle { color: var(--text-secondary); font-size: 0.875rem; margin-top: 4px; }

/* Stats */
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-card); border: 1px solid var(--border); border-radius: 12px; padding: var(--space-3); display: flex; align-items: center; gap: var(--space-3); }
.stat-icon { width: 44px; height: 44px; border-radius: 10px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.stat-value { font-size: 1.5rem; font-weight: 700; color: var(--text-primary); }
.stat-label { font-size: 0.75rem; color: var(--text-secondary); }

/* Budget bar */
.budget-bar { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-4); padding: var(--space-2) var(--space-3); background: var(--bg-card); border: 1px solid var(--border); border-radius: 8px; }
.budget-label { font-size: 0.8rem; color: var(--text-secondary); white-space: nowrap; }
.budget-track { flex: 1; height: 8px; background: var(--bg-hover); border-radius: 4px; overflow: hidden; }
.budget-fill { height: 100%; border-radius: 4px; transition: width 0.3s; }
.budget-text { font-size: 0.8rem; font-weight: 600; color: var(--text-primary); white-space: nowrap; }

/* Tabs */
.tab-bar { display: flex; gap: 2px; margin-bottom: var(--space-3); border-bottom: 2px solid var(--border); }
.tab-btn { display: flex; align-items: center; gap: 6px; padding: 10px 16px; font-size: 0.875rem; border: none; background: none; cursor: pointer; color: var(--text-secondary); border-bottom: 2px solid transparent; margin-bottom: -2px; transition: all 0.2s; }
.tab-btn:hover { color: var(--text-primary); }
.tab-btn.active { color: #6366F1; border-bottom-color: #6366F1; font-weight: 600; }
.section { margin-top: var(--space-3); }

/* Filters */
.filter-row { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); flex-wrap: wrap; }
.field-select, .field-input { padding: 8px 12px; border: 1px solid var(--border); border-radius: 8px; font-size: 0.875rem; background: var(--bg-card); color: var(--text-primary); }
.field-select:focus, .field-input:focus { outline: none; border-color: #6366F1; box-shadow: 0 0 0 3px rgba(99,102,241,0.1); }

/* Table */
.data-table { overflow-x: auto; }
.data-table table { width: 100%; border-collapse: collapse; }
.data-table th { padding: 10px 12px; text-align: left; font-size: 0.75rem; font-weight: 600; color: var(--text-secondary); text-transform: uppercase; border-bottom: 2px solid var(--border); }
.data-table td { padding: 10px 12px; border-bottom: 1px solid var(--border); font-size: 0.875rem; }
.row-clickable { cursor: pointer; transition: background 0.15s; }
.row-clickable:hover { background: var(--bg-hover); }
.text-mono { font-family: var(--font-mono, 'SF Mono', Monaco, Consolas, monospace); }
.text-sm { font-size: 0.8rem; }
.text-muted { color: var(--text-secondary); }

/* Badges */
.tool-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 0.8rem; font-family: var(--font-mono, monospace); background: #EEF2FF; color: #6366F1; }
.verdict-badge { display: inline-block; padding: 2px 10px; border-radius: 12px; font-size: 0.75rem; font-weight: 600; }
.verdict-user { background: #ECFDF5; color: #059669; }
.verdict-injection { background: #FEF2F2; color: #DC2626; }
.verdict-inconclusive { background: #FFFBEB; color: #D97706; }

/* Attribution bar */
.attr-bar-wrap { display: flex; align-items: center; gap: 6px; min-width: 100px; }
.attr-bar { height: 6px; border-radius: 3px; transition: width 0.3s; min-width: 2px; }
.attr-val { font-size: 0.8rem; font-weight: 600; color: var(--text-primary); }

/* Diff view */
.expand-row td { padding: var(--space-3) !important; background: var(--bg-hover); }
.diff-view { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); margin-bottom: var(--space-2); }
.diff-panel { background: var(--bg-card); border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.diff-title { padding: 8px 12px; font-size: 0.8rem; font-weight: 600; color: var(--text-secondary); border-bottom: 1px solid var(--border); display: flex; align-items: center; gap: 6px; }
.diff-pre { padding: 12px; font-size: 0.75rem; line-height: 1.5; overflow-x: auto; max-height: 300px; overflow-y: auto; margin: 0; white-space: pre-wrap; word-break: break-all; }
.diff-original { background: #FEFCE8; }
.diff-control { background: #F0FDF4; }
.detail-row { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.detail-item { font-size: 0.8rem; color: var(--text-secondary); flex: 1 1 45%; min-width: 200px; }
.detail-item code { font-size: 0.75rem; background: var(--bg-hover); padding: 2px 6px; border-radius: 4px; max-width: 300px; display: inline-block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; vertical-align: bottom; }
.decision-allow { color: #059669; font-weight: 600; }
.decision-block { color: #DC2626; font-weight: 600; }
.decision-warn { color: #D97706; font-weight: 600; }

/* Empty */
.empty-state { text-align: center; padding: var(--space-6); color: var(--text-secondary); }
.empty-state p { margin-top: var(--space-2); }

/* Charts */
.chart-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
.chart-card { background: var(--bg-card); border: 1px solid var(--border); border-radius: 12px; padding: var(--space-4); }
.chart-title { font-size: 0.9rem; font-weight: 600; color: var(--text-primary); margin-bottom: var(--space-3); }
.pie-chart { display: flex; align-items: center; gap: var(--space-4); justify-content: center; flex-wrap: wrap; }
.pie-legend { display: flex; flex-direction: column; gap: 8px; }
.legend-item { display: flex; align-items: center; gap: 8px; font-size: 0.8rem; color: var(--text-primary); }
.legend-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.metric-list { display: flex; flex-direction: column; gap: 12px; }
.metric-item { display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid var(--border); }
.metric-label { font-size: 0.85rem; color: var(--text-secondary); }
.metric-value { font-size: 0.95rem; font-weight: 600; color: var(--text-primary); }

/* Config */
.config-form { background: var(--bg-card); border: 1px solid var(--border); border-radius: 12px; padding: var(--space-4); max-width: 600px; }
.config-row { display: flex; align-items: center; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; }
.config-label { font-size: 0.875rem; font-weight: 600; color: var(--text-primary); min-width: 120px; }
.config-hint { font-size: 0.75rem; color: var(--text-secondary); }
.config-actions { display: flex; gap: var(--space-2); margin-top: var(--space-4); }
.config-msg { margin-top: var(--space-2); padding: 8px 12px; border-radius: 8px; font-size: 0.85rem; }
.msg-success { background: #ECFDF5; color: #059669; }
.msg-error { background: #FEF2F2; color: #DC2626; }

/* Toggle */
.toggle { position: relative; display: inline-block; width: 44px; height: 24px; }
.toggle input { opacity: 0; width: 0; height: 0; }
.toggle-slider { position: absolute; cursor: pointer; top: 0; left: 0; right: 0; bottom: 0; background: #D1D5DB; border-radius: 24px; transition: 0.3s; }
.toggle-slider::before { content: ''; position: absolute; height: 18px; width: 18px; left: 3px; bottom: 3px; background: white; border-radius: 50%; transition: 0.3s; }
.toggle input:checked + .toggle-slider { background: #6366F1; }
.toggle input:checked + .toggle-slider::before { transform: translateX(20px); }

/* Buttons */
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border: 1px solid var(--border); border-radius: 8px; font-size: 0.875rem; cursor: pointer; background: var(--bg-card); color: var(--text-primary); transition: all 0.2s; }
.btn:hover { background: var(--bg-hover); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-sm { padding: 6px 12px; font-size: 0.8rem; }
.btn-primary { background: #6366F1; color: white; border-color: #6366F1; }
.btn-primary:hover { background: #4F46E5; }
.btn-ghost { background: transparent; border-color: transparent; }
.btn-ghost:hover { background: var(--bg-hover); }

@media (max-width: 768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .chart-grid { grid-template-columns: 1fr; }
  .diff-view { grid-template-columns: 1fr; }
  .config-row { flex-direction: column; align-items: flex-start; }
}
</style>