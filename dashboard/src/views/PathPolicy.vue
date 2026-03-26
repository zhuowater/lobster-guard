<template>
  <div class="pathpolicy-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="git-branch" :size="20" /> 路径治理引擎</h1>
        <p class="page-subtitle">运行时路径级治理：序列、累积与降级规则</p>
      </div>
      <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
    </div>

    <div class="stats-grid" v-if="!initialLoading">
      <StatCard :iconSvg="svgPath" :value="stats.active_contexts!=null?stats.active_contexts:'-'" label="活跃路径" color="indigo" />
      <StatCard :iconSvg="svgRules" :value="stats.total_rules!=null?stats.total_rules:'-'" label="规则总数" color="blue" />
      <StatCard :iconSvg="svgBlock" :value="stats.block_count!=null?stats.block_count:'-'" label="已拦截" color="red" />
      <StatCard :iconSvg="svgWarn" :value="stats.warn_count!=null?stats.warn_count:'-'" label="已告警" color="yellow" />
    </div>
    <div class="stats-grid" v-else><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>

    <div class="tab-bar">
      <button class="tab-btn" :class="{active:activeTab==='rules'}" @click="activeTab='rules'"><Icon name="file-text" :size="14"/> 策略规则 ({{ rules.length }})</button>
      <button class="tab-btn" :class="{active:activeTab==='gauge'}" @click="activeTab='gauge'; loadGauge()"><Icon name="activity" :size="14"/> 风险仪表</button>
      <button class="tab-btn" :class="{active:activeTab==='paths'}" @click="activeTab='paths'; loadContexts()"><Icon name="git-branch" :size="14"/> 活跃路径 ({{ contexts.length }})</button>
      <button class="tab-btn" :class="{active:activeTab==='events'}" @click="activeTab='events'; loadEvents()"><Icon name="clipboard" :size="14"/> 决策日志 ({{ events.length }})</button>
      <button class="tab-btn" :class="{active:activeTab==='templates'}" @click="activeTab='templates'; loadTemplates()"><Icon name="book" :size="14"/> 模板</button>
    </div>

    <!-- Rules Tab -->
    <div v-if="activeTab==='rules'" class="section">
      <div class="rules-toolbar">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="ruleSearch" placeholder="搜索规则..." class="search-input"/>
        </div>
        <button class="btn btn-primary btn-sm" @click="openNewRule"><Icon name="plus" :size="14"/> 新建规则</button>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>名称</th><th>类型</th><th>动作</th><th>优先级</th><th>描述</th><th>启用</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="r in filteredRules" :key="r.id">
              <td class="td-mono">{{ r.name }}</td>
              <td><span class="type-badge" :class="'type-'+r.rule_type">{{ r.rule_type }}</span></td>
              <td><span class="action-badge" :class="'action-'+r.action">{{ r.action }}</span></td>
              <td class="td-mono">{{ r.priority }}</td>
              <td class="td-desc">{{ r.description }}</td>
              <td><label class="switch"><input type="checkbox" :checked="r.enabled" @change="toggleEnabled(r)"/><span class="slider"></span></label></td>
              <td class="td-actions">
                <button class="btn-icon" @click="editRule(r)" title="编辑"><Icon name="edit" :size="14"/></button>
                <button class="btn-icon btn-icon-danger" @click="confirmDeleteRule(r)" title="删除"><Icon name="trash" :size="14"/></button>
              </td>
            </tr>
            <tr v-if="filteredRules.length===0"><td colspan="7" class="empty-row">暂无规则</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Active Paths Tab -->
    <div v-if="activeTab==='paths'" class="section">
      <div class="paths-grid" v-if="contexts.length">
        <div class="path-card" v-for="ctx in contexts" :key="ctx.trace_id">
          <div class="path-card-header">
            <div class="path-ids">
              <span class="trace-id">{{ ctx.trace_id }}</span>
              <span class="session-id" v-if="ctx.session_id">{{ ctx.session_id }}</span>
            </div>
            <div class="risk-ring">
              <svg width="60" height="60" viewBox="0 0 60 60">
                <circle cx="30" cy="30" r="26" fill="none" :stroke="riskBgColor(ctx.risk_score)" stroke-width="5" />
                <circle cx="30" cy="30" r="26" fill="none" :stroke="riskColor(ctx.risk_score)" stroke-width="5"
                  stroke-linecap="round" :stroke-dasharray="riskDash(ctx.risk_score)"
                  transform="rotate(-90 30 30)" class="risk-arc" />
              </svg>
              <span class="risk-value" :style="{color: riskColor(ctx.risk_score)}">{{ Math.round(ctx.risk_score) }}</span>
            </div>
          </div>
          <div class="taint-badges" v-if="ctx.taint_labels && ctx.taint_labels.length">
            <span class="taint-badge" v-for="t in ctx.taint_labels" :key="t">{{ t }}</span>
          </div>
          <div class="path-timeline" v-if="ctx.steps && ctx.steps.length">
            <div class="timeline-step" v-for="(step, idx) in ctx.steps.slice(-6)" :key="idx">
              <div class="timeline-dot" :class="'stage-'+step.stage"></div>
              <div class="timeline-info">
                <span class="step-action">{{ step.action }}</span>
                <span class="step-stage">{{ step.stage }}</span>
              </div>
            </div>
          </div>
          <div class="path-meta">
            <span>步骤: {{ (ctx.steps||[]).length }}</span>
            <span>工具: {{ (ctx.tool_history||[]).length }}</span>
          </div>
        </div>
      </div>
      <div v-else class="empty-state"><Icon name="git-branch" :size="48" color="#6366F1"/><p>暂无活跃路径</p></div>
    </div>

    <!-- Events Tab -->
    <div v-if="activeTab==='events'" class="section">
      <div class="events-toolbar">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="eventSearch" placeholder="搜索 Trace ID..." class="search-input"/>
        </div>
        <select v-model="eventSince" class="select-sm" @change="loadEvents">
          <option value="">全部时间</option>
          <option value="1h">最近 1 小时</option>
          <option value="24h">最近 24 小时</option>
          <option value="7d">最近 7 天</option>
        </select>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>时间</th><th>Trace ID</th><th>规则</th><th>决策</th><th>风险分</th><th>原因</th><th>路径长度</th></tr></thead>
          <tbody>
            <tr v-for="ev in filteredEvents" :key="ev.id">
              <td class="td-mono td-time">{{ formatTime(ev.created_at) }}</td>
              <td class="td-mono">{{ ev.trace_id }}</td>
              <td>{{ ev.rule_name || '-' }}</td>
              <td><span class="action-badge" :class="'action-'+ev.decision">{{ ev.decision }}</span></td>
              <td class="td-mono">{{ typeof ev.risk_score==='number'? ev.risk_score.toFixed(1) : '-' }}</td>
              <td class="td-desc">{{ ev.reason }}</td>
              <td class="td-mono">{{ ev.path_length }}</td>
            </tr>
            <tr v-if="filteredEvents.length===0"><td colspan="7" class="empty-row">暂无事件</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Risk Gauge Tab (v23.1) -->
    <div v-if="activeTab==='gauge'" class="section">
      <div class="gauge-header">
        <p class="gauge-desc">每个活跃 Agent 会话的实时风险评分，分数随时间指数衰减。</p>
        <button class="btn btn-sm" @click="loadGauge"><Icon name="refresh" :size="14"/> 刷新</button>
      </div>
      <div class="gauge-grid" v-if="gauges.length">
        <div class="gauge-card" v-for="g in gauges" :key="g.trace_id" :class="gaugeLevel(g.risk_score)">
          <div class="gauge-top">
            <div class="gauge-ring-wrap">
              <svg width="80" height="80" viewBox="0 0 80 80">
                <circle cx="40" cy="40" r="34" fill="none" stroke="rgba(100,116,139,0.15)" stroke-width="6" />
                <circle cx="40" cy="40" r="34" fill="none" :stroke="riskColor(g.risk_score)" stroke-width="6"
                  stroke-linecap="round" :stroke-dasharray="gaugeDash(g.risk_score)"
                  transform="rotate(-90 40 40)" class="gauge-arc" />
              </svg>
              <span class="gauge-value" :style="{color: riskColor(g.risk_score)}">{{ Math.round(g.risk_score) }}</span>
            </div>
            <div class="gauge-info">
              <div class="gauge-trace">{{ g.trace_id }}</div>
              <div class="gauge-session" v-if="g.session_id">{{ g.session_id }}</div>
              <div class="gauge-meta-row">
                <span>{{ g.step_count }} 步骤</span>
                <span>{{ g.tool_count }} 工具</span>
                <span>{{ g.age_sec }}秒前</span>
              </div>
            </div>
          </div>
          <div class="gauge-taints" v-if="g.taint_labels && g.taint_labels.length">
            <span class="taint-badge" v-for="t in g.taint_labels" :key="t">{{ t }}</span>
          </div>
          <div class="gauge-last" v-if="g.last_action">最近: <span class="td-mono">{{ g.last_action }}</span></div>
        </div>
      </div>
      <div v-else class="empty-state"><Icon name="activity" :size="48" color="#6366F1"/><p>暂无活跃会话</p></div>
    </div>

    <!-- Templates Tab (v23.2 CRUD) -->
    <div v-if="activeTab==='templates'" class="section">
      <div class="template-toolbar">
        <p class="template-desc">合规、安全与行业场景的策略模板。激活以启用规则，停用以禁用。</p>
        <button class="btn btn-primary btn-sm" @click="openNewTemplate"><Icon name="plus" :size="14"/> 新建模板</button>
      </div>
      <div class="template-grid" v-if="templates.length">
        <div class="template-card" v-for="t in templates" :key="t.id" :class="{'tpl-disabled': !t.enabled}">
          <div class="template-header">
            <div>
              <h3 class="template-name">{{ t.name }}</h3>
              <span class="category-badge" :class="'cat-'+t.category">{{ t.category }}</span>
              <span class="builtin-badge" v-if="t.built_in">built-in</span>
            </div>
            <div class="tpl-actions">
              <button class="btn btn-sm" @click="activateTemplate(t)" :disabled="t._busy" title="启用模板中的所有规则">
                <Icon name="play" :size="12"/> 激活
              </button>
              <button class="btn btn-sm btn-ghost" @click="deactivateTemplate(t)" :disabled="t._busy" title="禁用模板中的所有规则">
                <Icon name="pause" :size="12"/> 停用
              </button>
              <button class="btn-icon" @click="editTemplate(t)" title="编辑"><Icon name="edit" :size="14"/></button>
              <button class="btn-icon btn-icon-danger" v-if="!t.built_in" @click="confirmDeleteTemplate(t)" title="删除"><Icon name="trash" :size="14"/></button>
            </div>
          </div>
          <p class="template-text">{{ t.description }}</p>
          <div class="template-rules">
            <span class="template-rule-badge" v-for="rid in t.rule_ids" :key="rid">{{ rid }}</span>
          </div>
        </div>
      </div>
      <div v-else class="empty-state"><Icon name="book" :size="48" color="#6366F1"/><p>暂无模板</p></div>
    </div>

    <!-- Template Modal -->
    <div class="modal-overlay" v-if="showTemplateModal" @click.self="showTemplateModal=false">
      <div class="modal">
        <div class="modal-header">
          <h3>{{ editingTemplate ? '编辑模板' : '新建模板' }}</h3>
          <button class="btn-close" @click="showTemplateModal=false"><Icon name="x-circle" :size="18"/></button>
        </div>
        <div class="modal-body">
          <div class="form-row"><label>ID</label><input v-model="tplForm.id" class="field-input" :disabled="!!editingTemplate" placeholder="tpl-xxx"/></div>
          <div class="form-row"><label>名称</label><input v-model="tplForm.name" class="field-input" placeholder="模板名称"/></div>
          <div class="form-row"><label>分类</label>
            <select v-model="tplForm.category" class="field-input">
              <option value="compliance">合规</option><option value="security">安全</option>
              <option value="industry">行业</option><option value="custom">自定义</option>
            </select>
          </div>
          <div class="form-row"><label>描述</label><textarea v-model="tplForm.description" class="field-input" rows="3"></textarea></div>
          <div class="form-row">
            <label>规则 ID（选择要包含的规则）</label>
            <div class="rule-checkboxes">
              <label class="rule-checkbox" v-for="r in rules" :key="r.id">
                <input type="checkbox" :value="r.id" v-model="tplForm.rule_ids"/>
                <span class="rc-label">{{ r.id }} <span class="rc-name">{{ r.name }}</span></span>
              </label>
            </div>
          </div>
          <div class="form-row"><label><input type="checkbox" v-model="tplForm.enabled"/> 启用</label></div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="showTemplateModal=false">取消</button>
          <button class="btn btn-primary" @click="saveTemplate" :disabled="tplSaving">{{ tplSaving ? '保存中...' : editingTemplate ? '更新' : '创建' }}</button>
        </div>
      </div>
    </div>

    <!-- Template Delete Confirm -->
    <div class="modal-overlay" v-if="deleteTemplateTarget" @click.self="deleteTemplateTarget=null">
      <div class="modal modal-sm">
        <div class="modal-header"><h3>确认删除</h3></div>
        <div class="modal-body"><p>确定删除模板 <strong>{{ deleteTemplateTarget.name }}</strong>？</p></div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="deleteTemplateTarget=null">取消</button>
          <button class="btn btn-danger" @click="doDeleteTemplate">删除</button>
        </div>
      </div>
    </div>

    <!-- Rule Modal -->
    <div class="modal-overlay" v-if="showRuleModal" @click.self="showRuleModal=false">
      <div class="modal">
        <div class="modal-header">
          <h3>{{ editingRule ? '编辑规则' : '新建规则' }}</h3>
          <button class="btn-close" @click="showRuleModal=false"><Icon name="x-circle" :size="18"/></button>
        </div>
        <div class="modal-body">
          <div class="form-row"><label>ID</label><input v-model="ruleForm.id" class="field-input" :disabled="!!editingRule" placeholder="pp-xxx"/></div>
          <div class="form-row"><label>名称</label><input v-model="ruleForm.name" class="field-input" placeholder="规则名称"/></div>
          <div class="form-row"><label>类型</label>
            <select v-model="ruleForm.rule_type" class="field-input">
              <option value="sequence">序列</option><option value="cumulative">累积</option><option value="degradation">降级</option>
            </select>
          </div>
          <div class="form-row"><label>条件 (JSON)</label><textarea v-model="ruleForm.conditions" class="field-input" rows="3"></textarea></div>
          <div class="form-row"><label>动作</label>
            <select v-model="ruleForm.action" class="field-input"><option value="block">拦截</option><option value="warn">告警</option><option value="log">记录</option></select>
          </div>
          <div class="form-row"><label>优先级</label><input v-model.number="ruleForm.priority" class="field-input" type="number"/></div>
          <div class="form-row"><label>描述</label><input v-model="ruleForm.description" class="field-input"/></div>
          <div class="form-row"><label><input type="checkbox" v-model="ruleForm.enabled"/> 启用</label></div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="showRuleModal=false">取消</button>
          <button class="btn btn-primary" @click="saveRule" :disabled="saving">{{ saving?'保存中...': editingRule?'更新':'创建' }}</button>
        </div>
      </div>
    </div>

    <!-- Delete Confirm -->
    <div class="modal-overlay" v-if="deleteTarget" @click.self="deleteTarget=null">
      <div class="modal modal-sm">
        <div class="modal-header"><h3>确认删除</h3></div>
        <div class="modal-body"><p>确定删除规则 <strong>{{ deleteTarget.name }}</strong>？</p></div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="deleteTarget=null">取消</button>
          <button class="btn btn-danger" @click="doDelete">删除</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import Skeleton from '../components/Skeleton.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'

export default {
  name: 'PathPolicy',
  components: { Icon, StatCard, Skeleton },
  data() {
    return {
      activeTab: 'rules', initialLoading: true, stats: {}, rules: [], contexts: [], events: [],
      gauges: [], templates: [],
      ruleSearch: '', eventSearch: '', eventSince: '',
      showRuleModal: false, editingRule: null, ruleForm: this.emptyForm(), saving: false, deleteTarget: null,
      showTemplateModal: false, editingTemplate: null, tplForm: this.emptyTplForm(), tplSaving: false, deleteTemplateTarget: null,
      gaugeTimer: null,
      svgPath: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="4"/><path d="M16 8v5a3 3 0 0 0 6 0v-1a10 10 0 1 0-3.92 7.94"/></svg>',
      svgRules: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>',
      svgBlock: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>',
      svgWarn: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>',
    }
  },
  computed: {
    filteredRules() {
      if (!this.ruleSearch) return this.rules
      const q = this.ruleSearch.toLowerCase()
      return this.rules.filter(r => r.name.toLowerCase().includes(q) || r.rule_type.includes(q) || (r.description||'').toLowerCase().includes(q))
    },
    filteredEvents() {
      if (!this.eventSearch) return this.events
      const q = this.eventSearch.toLowerCase()
      return this.events.filter(e => (e.trace_id||'').toLowerCase().includes(q) || (e.rule_name||'').toLowerCase().includes(q))
    },
  },
  mounted() { this.loadAll() },
  methods: {
    emptyForm() { return { id: '', name: '', rule_type: 'sequence', conditions: '{}', action: 'warn', priority: 50, description: '', enabled: true } },
    async loadAll() { this.initialLoading = true; await Promise.all([this.loadStats(), this.loadRules()]); this.initialLoading = false },
    async loadStats() { try { this.stats = await api('/api/v1/path-policies/stats') } catch(e) { console.error(e) } },
    async loadRules() { try { const d = await api('/api/v1/path-policies'); this.rules = d.rules || [] } catch(e) { console.error(e) } },
    async loadContexts() { try { const d = await api('/api/v1/path-policies/contexts'); this.contexts = d.contexts || [] } catch(e) { console.error(e) } },
    async loadEvents() {
      try { let url = '/api/v1/path-policies/events?limit=200'; if (this.eventSince) url += '&since=' + this.eventSince; const d = await api(url); this.events = d.events || [] } catch(e) { console.error(e) }
    },
    openNewRule() { this.editingRule = null; this.ruleForm = this.emptyForm(); this.showRuleModal = true },
    editRule(r) { this.editingRule = r; this.ruleForm = { id: r.id, name: r.name, rule_type: r.rule_type, conditions: r.conditions, action: r.action, priority: r.priority, description: r.description||'', enabled: r.enabled }; this.showRuleModal = true },
    async saveRule() {
      this.saving = true
      try { if (this.editingRule) { await apiPut('/api/v1/path-policies/' + this.ruleForm.id, this.ruleForm) } else { await apiPost('/api/v1/path-policies', this.ruleForm) }; this.showRuleModal = false; await this.loadRules(); await this.loadStats() } catch(e) { alert(e.message||e) }
      this.saving = false
    },
    confirmDeleteRule(r) { this.deleteTarget = r },
    async doDelete() { try { await apiDelete('/api/v1/path-policies/' + this.deleteTarget.id); this.deleteTarget = null; await this.loadRules(); await this.loadStats() } catch(e) { alert(e.message||e) } },
    async toggleEnabled(r) { try { await apiPut('/api/v1/path-policies/' + r.id, { ...r, enabled: !r.enabled }); await this.loadRules() } catch(e) { alert(e.message||e) } },
    formatTime(t) { if (!t) return '-'; try { return new Date(t).toLocaleString() } catch(e) { return t } },
    riskColor(s) { if (s > 80) return '#EF4444'; if (s > 60) return '#F59E0B'; return '#22C55E' },
    riskBgColor(s) { if (s > 80) return '#FEE2E2'; if (s > 60) return '#FEF3C7'; return '#DCFCE7' },
    riskDash(s) { const c = 2 * Math.PI * 26; return (c * Math.min(s / 100, 1)) + ' ' + c },
    // v23.1: Risk Gauge
    async loadGauge() {
      try { const d = await api('/api/v1/path-policies/risk-gauge'); this.gauges = d.gauges || [] } catch(e) { console.error(e) }
    },
    gaugeDash(s) { const c = 2 * Math.PI * 34; return (c * Math.min(s / 100, 1)) + ' ' + c },
    gaugeLevel(s) { if (s > 80) return 'gauge-danger'; if (s > 60) return 'gauge-warning'; return 'gauge-normal' },
    // v23.2 CRUD: Templates
    emptyTplForm() { return { id: '', name: '', category: 'custom', description: '', rule_ids: [], enabled: true } },
    async loadTemplates() {
      try { const d = await api('/api/v1/path-policies/templates'); this.templates = (d.templates || []).map(t => ({...t, _busy: false})) } catch(e) { console.error(e) }
    },
    async activateTemplate(t) {
      t._busy = true
      try { await apiPost('/api/v1/path-policies/templates/' + t.id + '/activate', {}); await this.loadRules(); await this.loadStats() } catch(e) { alert(e.message||e) }
      t._busy = false
    },
    async deactivateTemplate(t) {
      t._busy = true
      try { await apiPost('/api/v1/path-policies/templates/' + t.id + '/deactivate', {}); await this.loadRules(); await this.loadStats() } catch(e) { alert(e.message||e) }
      t._busy = false
    },
    openNewTemplate() { this.editingTemplate = null; this.tplForm = this.emptyTplForm(); this.showTemplateModal = true },
    editTemplate(t) {
      this.editingTemplate = t
      this.tplForm = { id: t.id, name: t.name, category: t.category || 'custom', description: t.description || '', rule_ids: [...(t.rule_ids || [])], enabled: t.enabled }
      this.showTemplateModal = true
    },
    async saveTemplate() {
      this.tplSaving = true
      try {
        if (this.editingTemplate) { await apiPut('/api/v1/path-policies/templates/' + this.tplForm.id, this.tplForm) }
        else { await apiPost('/api/v1/path-policies/templates', this.tplForm) }
        this.showTemplateModal = false; await this.loadTemplates()
      } catch(e) { alert(e.message||e) }
      this.tplSaving = false
    },
    confirmDeleteTemplate(t) { this.deleteTemplateTarget = t },
    async doDeleteTemplate() {
      try { await apiDelete('/api/v1/path-policies/templates/' + this.deleteTemplateTarget.id); this.deleteTemplateTarget = null; await this.loadTemplates() } catch(e) { alert(e.message||e) }
    },
  },
  beforeUnmount() { if (this.gaugeTimer) clearInterval(this.gaugeTimer) }
}
</script>

<style scoped>
.pathpolicy-page { padding: var(--space-6); max-width: 1400px; margin: 0 auto; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: var(--space-6); }
.page-title { font-size: var(--text-xl); font-weight: 700; display: flex; align-items: center; gap: var(--space-2); color: var(--text-primary); }
.page-subtitle { font-size: var(--text-sm); color: var(--text-secondary); margin-top: var(--space-1); }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: var(--space-4); margin-bottom: var(--space-6); }
.tab-bar { display: flex; gap: var(--space-1); border-bottom: 1px solid var(--border-subtle); margin-bottom: var(--space-5); }
.tab-btn { padding: var(--space-2) var(--space-4); border: none; background: none; cursor: pointer; font-size: var(--text-sm); font-weight: 500; color: var(--text-secondary); border-bottom: 2px solid transparent; display: flex; align-items: center; gap: var(--space-1); transition: all var(--transition-fast); }
.tab-btn:hover { color: var(--text-primary); }
.tab-btn.active { color: #6366F1; border-bottom-color: #6366F1; }
.section { animation: fadeIn 0.2s ease; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: translateY(0); } }

.rules-toolbar, .events-toolbar { display: flex; gap: var(--space-3); align-items: center; margin-bottom: var(--space-4); flex-wrap: wrap; }
.search-box { position: relative; flex: 1; min-width: 200px; }
.search-icon { position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: var(--text-tertiary); }
.search-input { width: 100%; padding: var(--space-2) var(--space-2) var(--space-2) 32px; border: 1px solid var(--border-subtle); border-radius: var(--radius-md); font-size: var(--text-sm); background: var(--bg-surface); color: var(--text-primary); }
.search-input:focus { outline: none; border-color: #6366F1; box-shadow: 0 0 0 2px rgba(99,102,241,0.15); }
.select-sm { padding: var(--space-2); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); font-size: var(--text-sm); background: var(--bg-surface); color: var(--text-primary); }

.table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-sm); }
.data-table th { padding: var(--space-2) var(--space-3); text-align: left; font-weight: 600; color: var(--text-secondary); border-bottom: 2px solid var(--border-subtle); font-size: var(--text-xs); text-transform: uppercase; letter-spacing: 0.05em; }
.data-table td { padding: var(--space-2) var(--space-3); border-bottom: 1px solid var(--border-subtle); color: var(--text-primary); }
.data-table tr:hover td { background: var(--bg-hover); }
.td-mono { font-family: var(--font-mono); font-size: var(--text-xs); }
.td-desc { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-secondary); }
.td-time { white-space: nowrap; }
.td-actions { display: flex; gap: var(--space-1); }
.empty-row { text-align: center; padding: var(--space-8) !important; color: var(--text-tertiary); }

.type-badge { display: inline-block; padding: 2px 8px; border-radius: var(--radius-full); font-size: var(--text-xs); font-weight: 600; }
.type-sequence { background: rgba(99,102,241,0.12); color: #6366F1; }
.type-cumulative { background: rgba(245,158,11,0.12); color: #D97706; }
.type-degradation { background: rgba(239,68,68,0.12); color: #EF4444; }
.action-badge { display: inline-block; padding: 2px 8px; border-radius: var(--radius-full); font-size: var(--text-xs); font-weight: 600; text-transform: uppercase; }
.action-block { background: rgba(239,68,68,0.12); color: #EF4444; }
.action-warn { background: rgba(245,158,11,0.12); color: #D97706; }
.action-log { background: rgba(107,114,128,0.12); color: #6B7280; }
.action-allow { background: rgba(34,197,94,0.12); color: #22C55E; }
.action-isolate { background: rgba(168,85,247,0.12); color: #A855F7; }

.switch { position: relative; display: inline-block; width: 36px; height: 20px; }
.switch input { opacity: 0; width: 0; height: 0; }
.slider { position: absolute; cursor: pointer; top: 0; left: 0; right: 0; bottom: 0; background-color: #CBD5E1; border-radius: 20px; transition: 0.3s; }
.slider::before { content: ""; position: absolute; height: 16px; width: 16px; left: 2px; bottom: 2px; background-color: white; border-radius: 50%; transition: 0.3s; }
input:checked + .slider { background-color: #6366F1; }
input:checked + .slider::before { transform: translateX(16px); }

.btn { padding: var(--space-2) var(--space-4); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); cursor: pointer; font-size: var(--text-sm); font-weight: 500; display: inline-flex; align-items: center; gap: var(--space-1); transition: all var(--transition-fast); background: var(--bg-surface); color: var(--text-primary); }
.btn:hover { border-color: var(--border-default); }
.btn-primary { background: #6366F1; color: white; border-color: #6366F1; }
.btn-primary:hover { background: #4F46E5; }
.btn-danger { background: #EF4444; color: white; border-color: #EF4444; }
.btn-danger:hover { background: #DC2626; }
.btn-ghost { background: transparent; border-color: transparent; }
.btn-ghost:hover { background: var(--bg-hover); }
.btn-sm { padding: var(--space-1) var(--space-3); font-size: var(--text-xs); }
.btn-icon { background: none; border: none; cursor: pointer; padding: 4px; border-radius: var(--radius-sm); color: var(--text-secondary); transition: all var(--transition-fast); }
.btn-icon:hover { background: var(--bg-hover); color: var(--text-primary); }
.btn-icon-danger:hover { color: #EF4444; background: rgba(239,68,68,0.08); }
.btn-close { background: none; border: none; cursor: pointer; color: var(--text-tertiary); }
.btn-close:hover { color: var(--text-primary); }

.paths-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: var(--space-4); }
.path-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); transition: all var(--transition-fast); }
.path-card:hover { border-color: #6366F1; box-shadow: 0 0 0 2px rgba(99,102,241,0.1); }
.path-card-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: var(--space-3); }
.path-ids { display: flex; flex-direction: column; gap: 2px; }
.trace-id { font-family: var(--font-mono); font-size: var(--text-xs); color: #6366F1; font-weight: 600; }
.session-id { font-family: var(--font-mono); font-size: 10px; color: var(--text-tertiary); }
.risk-ring { position: relative; display: flex; align-items: center; justify-content: center; }
.risk-arc { transition: stroke-dasharray 0.8s ease; }
.risk-value { position: absolute; font-family: var(--font-mono); font-size: var(--text-lg); font-weight: 700; transition: color 0.3s; }
.taint-badges { display: flex; flex-wrap: wrap; gap: var(--space-1); margin-bottom: var(--space-3); }
.taint-badge { display: inline-block; padding: 1px 6px; border-radius: var(--radius-full); font-size: 10px; font-weight: 600; background: rgba(239,68,68,0.1); color: #EF4444; border: 1px solid rgba(239,68,68,0.2); }

.path-timeline { margin: var(--space-3) 0; padding-left: var(--space-4); position: relative; }
.path-timeline::before { content: ''; position: absolute; left: 7px; top: 4px; bottom: 4px; width: 2px; background: var(--border-subtle); }
.timeline-step { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-2); position: relative; }
.timeline-dot { width: 10px; height: 10px; border-radius: 50%; border: 2px solid var(--border-subtle); background: var(--bg-surface); position: relative; z-index: 1; flex-shrink: 0; }
.stage-inbound .timeline-dot, .timeline-dot.stage-inbound { border-color: #6366F1; background: rgba(99,102,241,0.2); }
.stage-tool_call .timeline-dot, .timeline-dot.stage-tool_call { border-color: #F59E0B; background: rgba(245,158,11,0.2); }
.stage-llm_request .timeline-dot, .timeline-dot.stage-llm_request { border-color: #22C55E; background: rgba(34,197,94,0.2); }
.stage-llm_response .timeline-dot, .timeline-dot.stage-llm_response { border-color: #14B8A6; background: rgba(20,184,166,0.2); }
.stage-outbound .timeline-dot, .timeline-dot.stage-outbound { border-color: #8B5CF6; background: rgba(139,92,246,0.2); }
.timeline-info { display: flex; flex-direction: column; }
.step-action { font-size: var(--text-xs); font-weight: 600; color: var(--text-primary); font-family: var(--font-mono); }
.step-stage { font-size: 10px; color: var(--text-tertiary); }
.path-meta { display: flex; gap: var(--space-3); font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-2); padding-top: var(--space-2); border-top: 1px solid var(--border-subtle); }
.empty-state { display: flex; flex-direction: column; align-items: center; gap: var(--space-3); padding: var(--space-12); color: var(--text-tertiary); }

.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: var(--bg-surface); border-radius: var(--radius-lg); width: 520px; max-width: 90vw; max-height: 90vh; overflow-y: auto; box-shadow: var(--shadow-xl); }
.modal-sm { width: 400px; }
.modal-header { display: flex; justify-content: space-between; align-items: center; padding: var(--space-4) var(--space-5); border-bottom: 1px solid var(--border-subtle); }
.modal-header h3 { font-size: var(--text-lg); font-weight: 600; }
.modal-body { padding: var(--space-5); }
.modal-footer { display: flex; justify-content: flex-end; gap: var(--space-2); padding: var(--space-4) var(--space-5); border-top: 1px solid var(--border-subtle); }
.form-row { margin-bottom: var(--space-3); }
.form-row label { display: block; font-size: var(--text-sm); font-weight: 500; color: var(--text-secondary); margin-bottom: var(--space-1); }
.field-input { width: 100%; padding: var(--space-2); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); font-size: var(--text-sm); background: var(--bg-surface); color: var(--text-primary); font-family: inherit; }
.field-input:focus { outline: none; border-color: #6366F1; box-shadow: 0 0 0 2px rgba(99,102,241,0.15); }
textarea.field-input { font-family: var(--font-mono); resize: vertical; }

/* v23.1: Risk Gauge */
.gauge-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-4); }
.gauge-desc { font-size: var(--text-sm); color: var(--text-secondary); }
.gauge-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: var(--space-4); }
.gauge-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); transition: all var(--transition-fast); }
.gauge-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.1); }
.gauge-card.gauge-danger { border-left: 3px solid #EF4444; }
.gauge-card.gauge-warning { border-left: 3px solid #F59E0B; }
.gauge-card.gauge-normal { border-left: 3px solid #22C55E; }
.gauge-top { display: flex; gap: var(--space-4); align-items: center; }
.gauge-ring-wrap { position: relative; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.gauge-arc { transition: stroke-dasharray 0.8s ease; }
.gauge-value { position: absolute; font-family: var(--font-mono); font-size: var(--text-2xl); font-weight: 800; transition: color 0.3s; }
.gauge-info { flex: 1; min-width: 0; }
.gauge-trace { font-family: var(--font-mono); font-size: var(--text-xs); color: #6366F1; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.gauge-session { font-family: var(--font-mono); font-size: 10px; color: var(--text-tertiary); margin-top: 2px; }
.gauge-meta-row { display: flex; gap: var(--space-3); margin-top: var(--space-2); font-size: var(--text-xs); color: var(--text-secondary); }
.gauge-taints { display: flex; flex-wrap: wrap; gap: var(--space-1); margin-top: var(--space-2); }
.gauge-last { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: var(--space-2); }

/* v23.2 CRUD: Templates */
.template-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-4); }
.template-desc { font-size: var(--text-sm); color: var(--text-secondary); flex: 1; }
.template-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: var(--space-4); }
.template-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-5); transition: all var(--transition-fast); }
.template-card:hover { border-color: #6366F1; box-shadow: 0 0 0 2px rgba(99,102,241,0.1); }
.template-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-2); }
.template-name { font-size: var(--text-base); font-weight: 700; color: var(--text-primary); }
.template-text { font-size: var(--text-sm); color: var(--text-secondary); line-height: 1.5; margin-bottom: var(--space-3); }
.template-rules { display: flex; flex-wrap: wrap; gap: var(--space-1); }
.template-rule-badge { display: inline-block; padding: 2px 8px; border-radius: var(--radius-full); font-size: 10px; font-weight: 600; font-family: var(--font-mono); background: rgba(99,102,241,0.08); color: #6366F1; }
.tpl-disabled { opacity: 0.5; }
.tpl-actions { display: flex; gap: var(--space-1); align-items: center; flex-shrink: 0; }
.category-badge { display: inline-block; padding: 1px 6px; border-radius: var(--radius-full); font-size: 10px; font-weight: 600; margin-left: var(--space-2); }
.cat-compliance { background: rgba(34,197,94,0.1); color: #16A34A; }
.cat-security { background: rgba(99,102,241,0.1); color: #6366F1; }
.cat-industry { background: rgba(245,158,11,0.1); color: #D97706; }
.cat-custom { background: rgba(107,114,128,0.1); color: #6B7280; }
.builtin-badge { display: inline-block; padding: 1px 6px; border-radius: var(--radius-full); font-size: 10px; font-weight: 500; margin-left: var(--space-1); background: rgba(100,116,139,0.1); color: #64748B; }
.rule-checkboxes { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: var(--space-1); max-height: 200px; overflow-y: auto; padding: var(--space-2); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); }
.rule-checkbox { display: flex; align-items: center; gap: var(--space-1); font-size: var(--text-xs); cursor: pointer; padding: 2px 4px; border-radius: var(--radius-sm); }
.rule-checkbox:hover { background: var(--bg-hover); }
.rc-label { font-family: var(--font-mono); }
.rc-name { color: var(--text-tertiary); font-family: inherit; }

@media (max-width: 768px) {
  .pathpolicy-page { padding: var(--space-3); }
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .paths-grid { grid-template-columns: 1fr; }
  .gauge-grid { grid-template-columns: 1fr; }
  .template-grid { grid-template-columns: 1fr; }
  .rules-toolbar, .events-toolbar { flex-direction: column; }
}
</style>