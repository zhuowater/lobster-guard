<template>
  <div class="cf-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg> 反事实验证引擎</h1>
        <p class="page-subtitle">AttriGuard 对照验证 -- 构造无外部数据的对照请求，比对行为差异判断是否注入驱动</p>
      </div>
      <button class="btn btn-sm" @click="loadAll"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg> 刷新</button>
    </div>
    <div class="stats-grid" v-if="stats">
      <div class="stat-card"><div class="stat-icon" style="background:#EEF2FF;color:#6366F1"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg></div><div class="stat-content"><div class="stat-value">{{ stats.total_verifications ?? 0 }}</div><div class="stat-label">总验证数</div></div></div>
      <div class="stat-card"><div class="stat-icon" style="background:#FEF2F2;color:#EF4444"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg></div><div class="stat-content"><div class="stat-value">{{ stats.blocked_count ?? 0 }}</div><div class="stat-label">注入阻断</div></div></div>
      <div class="stat-card"><div class="stat-icon" style="background:#ECFDF5;color:#10B981"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg></div><div class="stat-content"><div class="stat-value">{{ stats.allowed_count ?? 0 }}</div><div class="stat-label">用户驱动</div></div></div>
      <div class="stat-card"><div class="stat-icon" style="background:#F5F3FF;color:#8B5CF6"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg></div><div class="stat-content"><div class="stat-value">{{ (stats.cache_hit_rate * 100).toFixed(1) }}%</div><div class="stat-label">缓存命中率</div></div></div>
    </div>
    <div class="budget-bar" v-if="stats"><span class="budget-label">小时预算</span><div class="budget-track"><div class="budget-fill" :style="{width: budgetPct + '%', background: budgetPct > 80 ? '#EF4444' : '#6366F1'}"></div></div><span class="budget-text">{{ stats.hourly_used }} / {{ stats.hourly_budget }}</span></div>
    <div class="tab-bar">
      <button class="tab-btn" :class="{active: activeTab === 'verifications'}" @click="activeTab='verifications'">验证记录 ({{ verifications.length }})</button>
      <button class="tab-btn" :class="{active: activeTab === 'statistics'}" @click="activeTab='statistics'">统计分析</button>
      <button class="tab-btn" :class="{active: activeTab === 'config'}" @click="activeTab='config'">配置</button>
    </div>
    <div v-if="activeTab === 'verifications'" class="section">
      <div class="filter-row"><select v-model="verdictFilter" class="field-select" @change="loadVerifications"><option value="">全部判定</option><option value="USER_DRIVEN">USER_DRIVEN</option><option value="INJECTION_DRIVEN">INJECTION_DRIVEN</option><option value="INCONCLUSIVE">INCONCLUSIVE</option></select><input v-model="traceFilter" class="field-input" placeholder="Trace ID..." @keyup.enter="loadVerifications" /><button class="btn btn-sm btn-primary" @click="loadVerifications">查询</button></div>
      <div class="data-table" v-if="verifications.length"><table><thead><tr><th>时间</th><th>Trace ID</th><th>工具名</th><th>判定</th><th>归因分数</th><th>延迟</th><th>缓存</th><th v-if="adaptiveConfig.feedback_enabled">反馈</th><th>操作</th></tr></thead><tbody>
        <template v-for="v in verifications" :key="v.id">
          <tr @click="toggleExpand(v.id)" class="row-clickable"><td class="text-mono text-sm">{{ formatTime(v.created_at) }}</td><td class="text-mono text-sm">{{ (v.trace_id || '-').substring(0, 12) }}</td><td><span class="tool-badge">{{ v.tool_name }}</span></td><td><span class="verdict-badge" :class="verdictClass(v.verdict)">{{ v.verdict }}</span></td><td><div class="attr-bar-wrap"><div class="attr-bar" :style="{width: (v.attribution_score * 100) + '%', background: attrColor(v.attribution_score)}"></div><span class="attr-val">{{ v.attribution_score.toFixed(2) }}</span></div></td><td>{{ v.latency_ms }}ms</td><td><svg v-if="v.cached" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#10B981" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg><span v-else class="text-muted">-</span></td>
            <td v-if="adaptiveConfig.feedback_enabled" @click.stop><div class="feedback-btns"><button class="fb-btn fb-correct" :class="{active: feedbackMap[v.id] === true}" @click="submitFeedback(v.id, true)" title="正确"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg></button><button class="fb-btn fb-wrong" :class="{active: feedbackMap[v.id] === false}" @click="submitFeedback(v.id, false)" title="误报"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button></div></td>
            <td @click.stop><button v-if="v.trace_id" class="link-btn" @click="$router.push('/audit?trace_id=' + v.trace_id)">📋 查看原始事件</button></td></tr>
          <tr v-if="expandedId === v.id" class="expand-row"><td :colspan="adaptiveConfig.feedback_enabled ? 9 : 8"><div class="diff-view"><div class="diff-panel"><div class="diff-title">原始 Messages</div><pre class="diff-pre diff-original">{{ formatJSON(v.original_messages) }}</pre></div><div class="diff-panel"><div class="diff-title">对照 Messages</div><pre class="diff-pre diff-control">{{ formatJSON(v.control_messages) }}</pre></div></div><div class="detail-row"><div class="detail-item"><strong>原始结果:</strong> <code>{{ v.original_result }}</code></div><div class="detail-item"><strong>对照结果:</strong> <code>{{ v.control_result }}</code></div><div class="detail-item"><strong>因果来源:</strong> {{ v.causal_driver || '-' }}</div><div class="detail-item"><strong>决策:</strong> <span :class="'decision-'+v.decision">{{ v.decision }}</span></div></div></td></tr>
        </template></tbody></table></div>
      <EmptyState v-else :iconSvg="emptyIcons.verifications" title="暂无验证记录" description="当系统执行反事实验证时将显示在这里" />
    </div>
    <div v-if="activeTab === 'statistics'" class="section">
      <div class="chart-grid">
        <div class="chart-card"><h3 class="chart-title">归因分布</h3><div class="pie-chart"><svg viewBox="0 0 200 200" width="180" height="180"><circle cx="100" cy="100" :r="80" fill="none" stroke="#10B981" stroke-width="30" :stroke-dasharray="pieSlice(stats.allowed_count, pieTotal)" stroke-dashoffset="0" /><circle cx="100" cy="100" :r="80" fill="none" stroke="#EF4444" stroke-width="30" :stroke-dasharray="pieSlice(stats.blocked_count, pieTotal)" :stroke-dashoffset="'-' + pieOffset(stats.allowed_count, pieTotal)" /><circle cx="100" cy="100" :r="80" fill="none" stroke="#F59E0B" stroke-width="30" :stroke-dasharray="pieSlice(stats.inconclusive_count, pieTotal)" :stroke-dashoffset="'-' + pieOffset(stats.allowed_count + stats.blocked_count, pieTotal)" /></svg><div class="pie-legend"><div class="legend-item"><span class="legend-dot" style="background:#10B981"></span> 用户驱动 ({{ stats.allowed_count ?? 0 }})</div><div class="legend-item"><span class="legend-dot" style="background:#EF4444"></span> 注入驱动 ({{ stats.blocked_count ?? 0 }})</div><div class="legend-item"><span class="legend-dot" style="background:#F59E0B"></span> 不确定 ({{ stats.inconclusive_count ?? 0 }})</div></div></div></div>
        <div class="chart-card"><h3 class="chart-title">关键指标</h3><div class="metric-list"><div class="metric-item"><span class="metric-label">平均延迟</span><span class="metric-value">{{ (stats.avg_latency_ms ?? 0).toFixed(1) }}ms</span></div><div class="metric-item"><span class="metric-label">平均归因分数</span><span class="metric-value">{{ (stats.avg_attribution_score ?? 0).toFixed(3) }}</span></div><div class="metric-item"><span class="metric-label">缓存命中率</span><span class="metric-value">{{ ((stats.cache_hit_rate ?? 0) * 100).toFixed(1) }}%</span></div><div class="metric-item"><span class="metric-label">预算使用率</span><span class="metric-value">{{ budgetPct.toFixed(1) }}%</span></div><div class="metric-item"><span class="metric-label">模式</span><span class="metric-value">{{ mode }}</span></div></div></div>
      </div>
      <div class="chart-card cost-section" v-if="costSummary"><h3 class="chart-title">成本趋势 (30天)</h3><div class="cost-budget-row"><div class="cost-budget-info"><span class="cost-budget-label">月预算</span><span class="cost-budget-val">${{ costSummary.monthly_used.toFixed(2) }} / ${{ costSummary.monthly_budget.toFixed(2) }}</span></div><div class="cost-budget-track"><div class="cost-budget-fill" :style="{width: Math.min(costSummary.usage_pct, 100) + '%', background: costSummary.usage_pct > 80 ? '#EF4444' : '#6366F1'}"></div></div><div class="cost-budget-info"><span class="cost-budget-label">预测月底</span><span class="cost-budget-val" :style="{color: costSummary.predicted_total > costSummary.monthly_budget ? '#EF4444' : '#10B981'}">${{ costSummary.predicted_total.toFixed(2) }}</span></div></div><div class="cost-chart" v-if="costSummary.daily_history && costSummary.daily_history.length"><div class="cost-chart-bars"><div class="cost-bar-col" v-for="d in costSummary.daily_history" :key="d.date" :title="d.date"><div class="cost-bar" :style="{height: costBarHeight(d.cost_usd) + '%', background: d.blocked_count > 0 ? '#EF4444' : '#6366F1'}"></div><span class="cost-bar-label">{{ d.date.substring(5) }}</span></div></div></div><div v-else class="empty-hint">暂无成本数据</div></div>
      <div class="chart-card effect-section" v-if="effectMetrics"><h3 class="chart-title">效果指标</h3><div class="gauge-grid"><div class="gauge-item" v-for="g in gaugeItems" :key="g.label"><svg viewBox="0 0 120 70" width="120" height="70"><path d="M 10 60 A 50 50 0 0 1 110 60" fill="none" stroke="#E5E7EB" stroke-width="8" stroke-linecap="round"/><path d="M 10 60 A 50 50 0 0 1 110 60" fill="none" :stroke="gaugeColor(g.value)" stroke-width="8" stroke-linecap="round" :stroke-dasharray="gaugeArc(g.value)" /><text x="60" y="52" text-anchor="middle" font-size="16" font-weight="700" :fill="gaugeColor(g.value)">{{ (g.value * 100).toFixed(1) }}%</text></svg><div class="gauge-label">{{ g.label }}</div></div></div><div class="effect-detail"><span class="effect-item">TP: {{ effectMetrics.true_positive }}</span><span class="effect-item">FP: {{ effectMetrics.false_positive }}</span><span class="effect-item">TN: {{ effectMetrics.true_negative }}</span><span class="effect-item">FN: {{ effectMetrics.false_negative }}</span><span class="effect-item">Total: {{ effectMetrics.total_checked }}</span></div></div>
    </div>
    <div v-if="activeTab === 'config'" class="section">
      <div class="config-form"><h3 class="config-section-title">基础配置</h3>
        <div class="config-row"><label class="config-label">启用</label><label class="toggle"><input type="checkbox" v-model="configForm.enabled" /><span class="toggle-slider"></span></label></div>
        <div class="config-row"><label class="config-label">模式</label><select v-model="configForm.mode" class="field-select"><option value="sync">同步</option><option value="async">异步</option></select></div>
        <div class="config-row"><label class="config-label">每小时预算</label><input v-model.number="configForm.max_per_hour" type="number" class="field-input" min="1" max="10000" /></div>
        <div class="config-row"><label class="config-label">风险阈值</label><input v-model.number="configForm.risk_threshold" type="number" class="field-input" min="0" max="100" /></div>
        <div class="config-row"><label class="config-label">缓存 TTL</label><input v-model.number="configForm.cache_ttl_sec" type="number" class="field-input" min="0" /></div>
        <div class="config-row"><label class="config-label">超时</label><input v-model.number="configForm.timeout_sec" type="number" class="field-input" min="1" /></div>
        <div class="config-row"><label class="config-label">模糊匹配</label><label class="toggle"><input type="checkbox" v-model="configForm.fuzzy_match" /><span class="toggle-slider"></span></label></div>
        <div class="config-actions"><button class="btn btn-primary" @click="saveConfig" :disabled="saving">{{ saving ? '保存中...' : '保存配置' }}</button><button class="btn btn-ghost" @click="clearCache" :disabled="clearingCache">{{ clearingCache ? '清除中...' : '清除缓存' }}</button></div>
        <div v-if="configMsg" class="config-msg" :class="configMsgType">{{ configMsg }}</div>
      </div>
      <div class="config-form adaptive-config"><h3 class="config-section-title">自适应策略</h3>
        <div class="config-row"><label class="config-label">启用</label><label class="toggle"><input type="checkbox" v-model="adaptiveForm.enabled" /><span class="toggle-slider"></span></label></div>
        <div class="config-row"><label class="config-label">月预算 (USD)</label><input v-model.number="adaptiveForm.monthly_budget_usd" type="number" class="field-input" min="0" step="10" /></div>
        <div class="config-row"><label class="config-label">单次成本</label><input v-model.number="adaptiveForm.cost_per_verification" type="number" class="field-input" min="0" step="0.01" /></div>
        <div class="config-row"><label class="config-label">优先级模式</label><select v-model="adaptiveForm.priority_mode" class="field-select"><option value="risk_score">风险分</option><option value="tool_severity">工具等级</option><option value="hybrid">混合</option></select></div>
        <div class="config-row"><label class="config-label">同步阈值</label><input v-model.number="adaptiveForm.min_risk_for_sync" type="number" class="field-input" min="0" max="100" /></div>
        <div class="config-row"><label class="config-label">人类反馈</label><label class="toggle"><input type="checkbox" v-model="adaptiveForm.feedback_enabled" /><span class="toggle-slider"></span></label></div>
        <div class="config-actions"><button class="btn btn-primary" @click="saveAdaptiveConfig" :disabled="savingAdaptive">{{ savingAdaptive ? '保存中...' : '保存自适应配置' }}</button></div>
        <div v-if="adaptiveMsg" class="config-msg" :class="adaptiveMsgType">{{ adaptiveMsg }}</div>
      </div>
    </div>
    <!-- Tab 4: Attribution (v24.1) -->
    <div v-if="activeTab === 'attribution'" class="section">
      <div class="attr-timeline" v-if="timelineEvents.length">
        <div class="timeline-line"></div>
        <div class="timeline-node" v-for="(ev, idx) in timelineEvents" :key="ev.id + '-' + idx">
          <div class="timeline-dot" :class="'dot-' + verdictToClass(ev.verdict)"></div>
          <div class="timeline-card" :class="{'expanded': expandedTimelineId === ev.id}" @click="toggleTimelineExpand(ev)">
            <div class="timeline-header">
              <span class="timeline-time">{{ formatTime(ev.timestamp) }}</span>
              <span class="timeline-type-badge" :class="'type-' + ev.event_type">
                <svg v-if="ev.event_type === 'verification'" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
                <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 20V10"/><path d="M18 20V4"/><path d="M6 20v-4"/></svg>
                {{ ev.event_type === 'verification' ? '验证' : '归因' }}
              </span>
            </div>
            <div class="timeline-body">
              <span class="tool-badge">{{ ev.tool_name }}</span>
              <span class="verdict-badge" :class="verdictClass(ev.verdict)">{{ ev.verdict }}</span>
              <div class="attr-ring-wrap"><svg class="attr-ring" width="36" height="36" viewBox="0 0 36 36"><circle cx="18" cy="18" r="14" fill="none" stroke="#E5E7EB" stroke-width="3" /><circle cx="18" cy="18" r="14" fill="none" :stroke="attrColor(ev.attribution_score)" stroke-width="3" :stroke-dasharray="(ev.attribution_score * 87.96) + ' 87.96'" stroke-dashoffset="0" transform="rotate(-90 18 18)" stroke-linecap="round" /><text x="18" y="21" text-anchor="middle" font-size="9" font-weight="600" :fill="attrColor(ev.attribution_score)">{{ (ev.attribution_score * 100).toFixed(0) }}</text></svg></div>
            </div>
            <div v-if="expandedTimelineId === ev.id" class="timeline-detail">
              <div v-if="ev.evidence_summary" class="evidence-box">
                <div class="evidence-title"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg> 证据摘要</div>
                <p class="evidence-text">{{ ev.evidence_summary }}</p>
              </div>
              <div v-if="ev.causal_chain && ev.causal_chain.length" class="chain-box">
                <div class="chain-title"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg> 因果链</div>
                <div class="chain-steps">
                  <div class="chain-step" v-for="step in ev.causal_chain" :key="step.step_index" :class="{'step-removed': step.was_removed}">
                    <span class="step-idx">#{{ step.step_index }}</span>
                    <span class="step-role">{{ step.role }}</span>
                    <span class="step-type">{{ step.content_type }}</span>
                    <span class="step-impact" :class="'impact-' + step.impact">{{ step.impact }}</span>
                    <span v-if="step.was_removed" class="step-removed-badge"><svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> 已移除</span>
                    <span v-else class="step-kept-badge"><svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg> 保留</span>
                  </div>
                </div>
              </div>
              <div v-if="ev.decision" class="decision-info"><strong>决策:</strong> <span :class="'decision-' + ev.decision">{{ ev.decision }}</span></div>
            </div>
          </div>
        </div>
      </div>
      <EmptyState v-else :iconSvg="emptyIcons.attribution" title="暂无因果归因事件" description="因果归因分析结果将显示在这里" />
    </div>
  </div>
</template>

<script>
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import EmptyState from '../components/EmptyState.vue'
export default {
  name: 'Counterfactual',
  components: { EmptyState },
  data() {
    return {
      activeTab: 'verifications',
      stats: { total_verifications: 0, blocked_count: 0, allowed_count: 0, inconclusive_count: 0, cache_hit_rate: 0, avg_latency_ms: 0, avg_attribution_score: 0, hourly_used: 0, hourly_budget: 100 },
      mode: 'async', verifications: [], expandedId: null, verdictFilter: '', traceFilter: '',
      configForm: { enabled: false, mode: 'async', max_per_hour: 100, risk_threshold: 50, cache_ttl_sec: 300, timeout_sec: 10, fuzzy_match: true },
      saving: false, clearingCache: false, configMsg: '', configMsgType: '',
      costSummary: null, effectMetrics: null, adaptiveConfig: { feedback_enabled: false },
      adaptiveForm: { enabled: false, monthly_budget_usd: 100, cost_per_verification: 0.05, priority_mode: 'hybrid', min_risk_for_sync: 80, feedback_enabled: true },
      savingAdaptive: false, adaptiveMsg: '', adaptiveMsgType: '', feedbackMap: {},
      timelineEvents: [], expandedTimelineId: null,
      emptyIcons: {
        verifications: '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>',
        attribution: '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 20V10"/><path d="M18 20V4"/><path d="M6 20v-4"/></svg>'
      },
    }
  },
  computed: {
    pieTotal() { return (this.stats.allowed_count || 0) + (this.stats.blocked_count || 0) + (this.stats.inconclusive_count || 0) || 1 },
    budgetPct() { return this.stats.hourly_budget > 0 ? (this.stats.hourly_used / this.stats.hourly_budget) * 100 : 0 },
    gaugeItems() {
      if (!this.effectMetrics) return []
      return [{ label: '准确率', value: this.effectMetrics.accuracy || 0 }, { label: '精确率', value: this.effectMetrics.precision || 0 }, { label: '召回率', value: this.effectMetrics.recall || 0 }, { label: 'F1 Score', value: this.effectMetrics.f1_score || 0 }]
    },
  },
  methods: {
    async loadAll() { await Promise.all([this.loadStats(), this.loadVerifications(), this.loadConfig(), this.loadCost(), this.loadEffectiveness(), this.loadAdaptiveConfig()]) },
    async loadStats() { try { const d = await api('/api/v1/counterfactual/stats'); if (d.stats) this.stats = d.stats; if (d.mode) this.mode = d.mode } catch(e) {} },
    async loadVerifications() { try { let q = '/api/v1/counterfactual/verifications?limit=100'; if (this.verdictFilter) q += '&verdict=' + this.verdictFilter; if (this.traceFilter) q += '&trace_id=' + encodeURIComponent(this.traceFilter); const d = await api(q); this.verifications = d.verifications || [] } catch(e) {} },
    async loadConfig() { try { const d = await api('/api/v1/counterfactual/config'); this.configForm = { ...d } } catch(e) {} },
    async loadCost() { try { this.costSummary = await api('/api/v1/counterfactual/cost') } catch(e) {} },
    async loadEffectiveness() { try { this.effectMetrics = await api('/api/v1/counterfactual/effectiveness') } catch(e) {} },
    async loadAdaptiveConfig() { try { const d = await api('/api/v1/counterfactual/adaptive-config'); this.adaptiveConfig = d; this.adaptiveForm = { ...d } } catch(e) {} },
    async saveConfig() { this.saving = true; this.configMsg = ''; try { await apiPut('/api/v1/counterfactual/config', this.configForm); this.configMsg = '配置已保存'; this.configMsgType = 'msg-success'; this.loadStats() } catch (e) { this.configMsg = '保存失败: ' + e.message; this.configMsgType = 'msg-error' } this.saving = false },
    async saveAdaptiveConfig() { this.savingAdaptive = true; this.adaptiveMsg = ''; try { await apiPut('/api/v1/counterfactual/adaptive-config', this.adaptiveForm); this.adaptiveMsg = '自适应配置已保存'; this.adaptiveMsgType = 'msg-success' } catch (e) { this.adaptiveMsg = '保存失败: ' + e.message; this.adaptiveMsgType = 'msg-error' } this.savingAdaptive = false },
    async submitFeedback(vid, wasCorrect) { try { await apiPost('/api/v1/counterfactual/feedback', { verification_id: vid, was_correct: wasCorrect }); this.feedbackMap = { ...this.feedbackMap, [vid]: wasCorrect } } catch(e) {} },
    async clearCache() { this.clearingCache = true; try { const d = await apiDelete('/api/v1/counterfactual/cache'); this.configMsg = '缓存已清除: ' + (d.cleared || 0) + ' 条'; this.configMsgType = 'msg-success' } catch (e) { this.configMsg = '清除失败: ' + e.message; this.configMsgType = 'msg-error' } this.clearingCache = false },
    async loadTimeline() { try { const d = await api('/api/v1/counterfactual/timeline?limit=200'); this.timelineEvents = d.events || [] } catch(e) {} },
    toggleExpand(id) { this.expandedId = this.expandedId === id ? null : id },
    toggleTimelineExpand(ev) {
      if (this.expandedTimelineId === ev.id) { this.expandedTimelineId = null; return }
      this.expandedTimelineId = ev.id
      if (ev.event_type === 'attribution' && (!ev.causal_chain || !ev.causal_chain.length)) {
        api('/api/v1/counterfactual/reports/' + ev.id).then(function(d) { if (d) { ev.causal_chain = d.causal_chain || []; ev.evidence_summary = d.evidence_summary || '' } }).catch(function() {})
      }
    },
    verdictClass(v) { if (v === 'USER_DRIVEN') return 'verdict-user'; if (v === 'INJECTION_DRIVEN') return 'verdict-injection'; return 'verdict-inconclusive' },
    verdictToClass(v) { if (v === 'USER_DRIVEN') return 'user'; if (v === 'INJECTION_DRIVEN') return 'injection'; return 'inconclusive' },
    attrColor(score) { if (score <= 0.3) return '#10B981'; if (score <= 0.6) return '#F59E0B'; return '#EF4444' },
    formatTime(t) { if (!t) return '-'; try { return new Date(t).toLocaleString('zh-CN', { hour12: false }) } catch(e) { return t } },
    formatJSON(s) { if (!s) return ''; try { return JSON.stringify(JSON.parse(s), null, 2) } catch(e) { return s } },
    pieSlice(val, total) { var pct = (val || 0) / total; var circ = 2 * Math.PI * 80; return (circ * pct) + ' ' + circ },
    pieOffset(val, total) { var pct = (val || 0) / total; return (2 * Math.PI * 80 * pct).toFixed(2) },
    gaugeColor(v) { if (v >= 0.8) return '#10B981'; if (v >= 0.5) return '#F59E0B'; return '#EF4444' },
    gaugeArc(v) { var totalArc = 157; return (totalArc * (v || 0)) + ' ' + totalArc },
    costBarHeight(cost) { if (!this.costSummary || !this.costSummary.daily_history) return 0; var max = Math.max.apply(null, this.costSummary.daily_history.map(function(d) { return d.cost_usd }).concat([0.01])); return Math.max((cost / max) * 100, 2) },
  },
  mounted() { this.loadAll() },
}
</script>

<style scoped>
.cf-page { max-width: 1200px; margin: 0 auto; padding: var(--space-4); }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: var(--space-4); }
.page-title { font-size: 1.5rem; font-weight: 700; display: flex; align-items: center; gap: 8px; color: var(--text-primary); }
.page-subtitle { color: var(--text-secondary); font-size: 0.875rem; margin-top: 4px; }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: var(--space-3); margin-bottom: var(--space-4); }
.stat-card { background: var(--bg-card); border: 1px solid var(--border); border-radius: 12px; padding: var(--space-3); display: flex; align-items: center; gap: var(--space-3); }
.stat-icon { width: 44px; height: 44px; border-radius: 10px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.stat-value { font-size: 1.5rem; font-weight: 700; color: var(--text-primary); }
.stat-label { font-size: 0.75rem; color: var(--text-secondary); }
.budget-bar { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-4); padding: var(--space-2) var(--space-3); background: var(--bg-card); border: 1px solid var(--border); border-radius: 8px; }
.budget-label { font-size: 0.8rem; color: var(--text-secondary); white-space: nowrap; }
.budget-track { flex: 1; height: 8px; background: var(--bg-hover); border-radius: 4px; overflow: hidden; }
.budget-fill { height: 100%; border-radius: 4px; transition: width 0.3s; }
.budget-text { font-size: 0.8rem; font-weight: 600; color: var(--text-primary); white-space: nowrap; }
.tab-bar { display: flex; gap: 2px; margin-bottom: var(--space-3); border-bottom: 2px solid var(--border); flex-wrap: wrap; }
.tab-btn { display: flex; align-items: center; gap: 6px; padding: 10px 16px; font-size: 0.875rem; border: none; background: none; cursor: pointer; color: var(--text-secondary); border-bottom: 2px solid transparent; margin-bottom: -2px; transition: all 0.2s; }
.tab-btn:hover { color: var(--text-primary); }
.tab-btn.active { color: #6366F1; border-bottom-color: #6366F1; font-weight: 600; }
.section { margin-top: var(--space-3); }
.filter-row { display: flex; gap: var(--space-2); margin-bottom: var(--space-3); flex-wrap: wrap; }
.field-select, .field-input { padding: 8px 12px; border: 1px solid var(--border); border-radius: 8px; font-size: 0.875rem; background: var(--bg-card); color: var(--text-primary); }
.data-table { overflow-x: auto; }
.data-table table { width: 100%; border-collapse: collapse; }
.data-table th { padding: 10px 12px; text-align: left; font-size: 0.75rem; font-weight: 600; color: var(--text-secondary); text-transform: uppercase; border-bottom: 2px solid var(--border); }
.data-table td { padding: 10px 12px; border-bottom: 1px solid var(--border); font-size: 0.875rem; }
.row-clickable { cursor: pointer; transition: background 0.15s; }
.row-clickable:hover { background: var(--bg-hover); }
.text-mono { font-family: var(--font-mono, monospace); }
.text-sm { font-size: 0.8rem; }
.text-muted { color: var(--text-secondary); }
.tool-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 0.8rem; font-family: var(--font-mono, monospace); background: #EEF2FF; color: #6366F1; }
.verdict-badge { display: inline-block; padding: 2px 10px; border-radius: 12px; font-size: 0.75rem; font-weight: 600; }
.verdict-user { background: #ECFDF5; color: #059669; }
.verdict-injection { background: #FEF2F2; color: #DC2626; }
.verdict-inconclusive { background: #FFFBEB; color: #D97706; }
.attr-bar-wrap { display: flex; align-items: center; gap: 6px; min-width: 100px; }
.attr-bar { height: 6px; border-radius: 3px; transition: width 0.3s; min-width: 2px; }
.attr-val { font-size: 0.8rem; font-weight: 600; color: var(--text-primary); }
.expand-row td { padding: var(--space-3) !important; background: var(--bg-hover); }
.diff-view { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); margin-bottom: var(--space-2); }
.diff-panel { background: var(--bg-card); border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.diff-title { padding: 8px 12px; font-size: 0.8rem; font-weight: 600; color: var(--text-secondary); border-bottom: 1px solid var(--border); }
.diff-pre { padding: 12px; font-size: 0.75rem; line-height: 1.5; overflow-x: auto; max-height: 300px; overflow-y: auto; margin: 0; white-space: pre-wrap; word-break: break-all; }
.diff-original { background: #FEFCE8; }
.diff-control { background: #F0FDF4; }
.detail-row { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.detail-item { font-size: 0.8rem; color: var(--text-secondary); flex: 1 1 45%; min-width: 200px; }
.detail-item code { font-size: 0.75rem; background: var(--bg-hover); padding: 2px 6px; border-radius: 4px; }
.decision-allow { color: #059669; font-weight: 600; }
.decision-block { color: #DC2626; font-weight: 600; }
.decision-warn { color: #D97706; font-weight: 600; }
.empty-state { text-align: center; padding: var(--space-6); color: var(--text-secondary); }
.empty-state p { margin-top: var(--space-2); }
.chart-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
.chart-card { background: var(--bg-card); border: 1px solid var(--border); border-radius: 12px; padding: var(--space-4); }
.chart-title { font-size: 0.9rem; font-weight: 600; color: var(--text-primary); margin-bottom: var(--space-3); display: flex; align-items: center; gap: 6px; }
.pie-chart { display: flex; align-items: center; gap: var(--space-4); justify-content: center; flex-wrap: wrap; }
.pie-legend { display: flex; flex-direction: column; gap: 8px; }
.legend-item { display: flex; align-items: center; gap: 8px; font-size: 0.8rem; }
.legend-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.metric-list { display: flex; flex-direction: column; gap: 12px; }
.metric-item { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid var(--border); }
.metric-label { font-size: 0.85rem; color: var(--text-secondary); }
.metric-value { font-size: 0.95rem; font-weight: 600; color: var(--text-primary); }
.config-form { background: var(--bg-card); border: 1px solid var(--border); border-radius: 12px; padding: var(--space-4); max-width: 600px; margin-bottom: var(--space-3); }
.config-section-title { font-size: 1rem; font-weight: 700; margin-bottom: var(--space-3); display: flex; align-items: center; gap: 6px; }
.config-row { display: flex; align-items: center; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; }
.config-label { font-size: 0.875rem; font-weight: 600; min-width: 120px; }
.config-hint { font-size: 0.75rem; color: var(--text-secondary); }
.config-actions { display: flex; gap: var(--space-2); margin-top: var(--space-4); }
.config-msg { margin-top: var(--space-2); padding: 8px 12px; border-radius: 8px; font-size: 0.85rem; }
.msg-success { background: #ECFDF5; color: #059669; }
.msg-error { background: #FEF2F2; color: #DC2626; }
.toggle { position: relative; display: inline-block; width: 44px; height: 24px; }
.toggle input { opacity: 0; width: 0; height: 0; }
.toggle-slider { position: absolute; cursor: pointer; top: 0; left: 0; right: 0; bottom: 0; background: #D1D5DB; border-radius: 24px; transition: 0.3s; }
.toggle-slider::before { content: ''; position: absolute; height: 18px; width: 18px; left: 3px; bottom: 3px; background: white; border-radius: 50%; transition: 0.3s; }
.toggle input:checked + .toggle-slider { background: #6366F1; }
.toggle input:checked + .toggle-slider::before { transform: translateX(20px); }
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border: 1px solid var(--border); border-radius: 8px; font-size: 0.875rem; cursor: pointer; background: var(--bg-card); color: var(--text-primary); }
.btn:hover { background: var(--bg-hover); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-sm { padding: 6px 12px; font-size: 0.8rem; }
.btn-primary { background: #6366F1; color: white; border-color: #6366F1; }
.btn-primary:hover { background: #4F46E5; }
.btn-ghost { background: transparent; border-color: transparent; }
.cost-section { margin-top: var(--space-3); grid-column: 1 / -1; }
.cost-budget-row { display: flex; align-items: center; gap: var(--space-3); flex-wrap: wrap; }
.cost-budget-info { display: flex; flex-direction: column; gap: 2px; }
.cost-budget-label { font-size: 0.75rem; color: var(--text-secondary); }
.cost-budget-val { font-size: 0.95rem; font-weight: 700; }
.cost-budget-track { flex: 1; height: 8px; background: var(--bg-hover); border-radius: 4px; overflow: hidden; min-width: 100px; }
.cost-budget-fill { height: 100%; border-radius: 4px; }
.cost-chart-bars { display: flex; align-items: flex-end; gap: 2px; height: 120px; }
.cost-bar-col { flex: 1; display: flex; flex-direction: column; align-items: center; height: 100%; justify-content: flex-end; }
.cost-bar { width: 100%; min-height: 2px; border-radius: 3px 3px 0 0; }
.cost-bar-label { font-size: 0.55rem; color: var(--text-secondary); margin-top: 4px; }
.empty-hint { text-align: center; padding: var(--space-3); color: var(--text-secondary); }
.gauge-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); }
.gauge-item { text-align: center; }
.gauge-label { font-size: 0.8rem; color: var(--text-secondary); margin-top: 4px; }
.effect-section { margin-top: var(--space-3); grid-column: 1 / -1; }
.effect-detail { display: flex; gap: var(--space-3); flex-wrap: wrap; }
.effect-item { font-size: 0.8rem; color: var(--text-secondary); background: var(--bg-hover); padding: 4px 10px; border-radius: 6px; }
.adaptive-config { margin-top: var(--space-3); }
.feedback-btns { display: flex; gap: 4px; }
.fb-btn { padding: 4px 8px; border-radius: 6px; border: 1px solid var(--border); background: var(--bg-card); cursor: pointer; display: flex; align-items: center; }
.fb-btn:hover { background: var(--bg-hover); }
.fb-btn.active { border-color: #6366F1; background: #EEF2FF; }
/* v24.1 Attribution Timeline */
.attr-timeline { position: relative; padding-left: 32px; }
.timeline-line { position: absolute; left: 15px; top: 0; bottom: 0; width: 2px; background: linear-gradient(180deg, #6366F1 0%, #A5B4FC 50%, #E5E7EB 100%); }
.timeline-node { position: relative; margin-bottom: var(--space-3); }
.timeline-dot { position: absolute; left: -25px; top: 12px; width: 12px; height: 12px; border-radius: 50%; border: 2px solid white; z-index: 1; }
.dot-user { background: #10B981; box-shadow: 0 0 0 2px #10B981; }
.dot-injection { background: #EF4444; box-shadow: 0 0 0 2px #EF4444; }
.dot-inconclusive { background: #F59E0B; box-shadow: 0 0 0 2px #F59E0B; }
.timeline-card { background: var(--bg-card); border: 1px solid var(--border); border-radius: 10px; padding: var(--space-3); cursor: pointer; transition: all 0.2s; }
.timeline-card:hover { border-color: #6366F1; box-shadow: 0 2px 8px rgba(99,102,241,0.1); }
.timeline-card.expanded { border-color: #6366F1; }
.timeline-header { display: flex; align-items: center; gap: var(--space-2); margin-bottom: 8px; }
.timeline-time { font-size: 0.75rem; color: var(--text-secondary); font-family: var(--font-mono, monospace); }
.timeline-type-badge { display: inline-flex; align-items: center; gap: 4px; padding: 2px 8px; border-radius: 10px; font-size: 0.7rem; font-weight: 600; }
.type-verification { background: #EEF2FF; color: #6366F1; }
.type-attribution { background: #F5F3FF; color: #8B5CF6; }
.timeline-body { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; }
.attr-ring-wrap { margin-left: auto; }
.timeline-detail { margin-top: var(--space-3); padding-top: var(--space-3); border-top: 1px solid var(--border); }
.evidence-box { background: #F5F3FF; border-radius: 8px; padding: var(--space-3); margin-bottom: var(--space-2); }
.evidence-title { font-size: 0.85rem; font-weight: 600; color: #6366F1; margin-bottom: 8px; display: flex; align-items: center; gap: 6px; }
.evidence-text { font-size: 0.85rem; line-height: 1.6; color: var(--text-primary); margin: 0; }
.chain-box { background: var(--bg-card); border: 1px solid var(--border); border-radius: 8px; padding: var(--space-3); }
.chain-title { font-size: 0.85rem; font-weight: 600; color: #6366F1; margin-bottom: 8px; display: flex; align-items: center; gap: 6px; }
.chain-steps { display: flex; flex-direction: column; gap: 6px; }
.chain-step { display: flex; align-items: center; gap: 8px; padding: 6px 10px; border-radius: 6px; font-size: 0.8rem; background: var(--bg-hover); }
.chain-step.step-removed { background: #FEF2F2; }
.step-idx { font-weight: 700; color: #6366F1; min-width: 28px; }
.step-role { font-weight: 600; color: var(--text-primary); }
.step-type { color: var(--text-secondary); font-family: var(--font-mono, monospace); font-size: 0.75rem; }
.step-impact { padding: 1px 6px; border-radius: 4px; font-size: 0.7rem; font-weight: 600; }
.impact-high { background: #FEE2E2; color: #DC2626; }
.impact-medium { background: #FEF3C7; color: #D97706; }
.impact-low { background: #ECFDF5; color: #059669; }
.step-removed-badge { color: #DC2626; font-size: 0.7rem; display: inline-flex; align-items: center; gap: 2px; font-weight: 600; }
.step-kept-badge { color: #059669; font-size: 0.7rem; display: inline-flex; align-items: center; gap: 2px; font-weight: 600; }
.decision-info { margin-top: var(--space-2); font-size: 0.85rem; }
.link-btn { background:none; border:1px solid var(--border, #e5e7eb); border-radius:8px; cursor:pointer; padding:2px 8px; font-size:11px; color:#6366F1; transition:all .2s; white-space:nowrap; }
.link-btn:hover { background:rgba(99,102,241,0.08); border-color:#6366F1; }
</style>
