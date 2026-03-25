<template>
  <div class="pathpolicy-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="git-branch" :size="20" /> Path Policy Engine</h1>
        <p class="page-subtitle">Runtime path-level governance: sequence, cumulative, and degradation rules for agent execution paths</p>
      </div>
      <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> Refresh</button>
    </div>

    <div class="stats-grid" v-if="!initialLoading">
      <StatCard :iconSvg="svgPath" :value="stats.active_contexts!=null?stats.active_contexts:'-'" label="Active Paths" color="indigo" />
      <StatCard :iconSvg="svgRules" :value="stats.total_rules!=null?stats.total_rules:'-'" label="Total Rules" color="blue" />
      <StatCard :iconSvg="svgBlock" :value="stats.block_count!=null?stats.block_count:'-'" label="Blocked" color="red" />
      <StatCard :iconSvg="svgWarn" :value="stats.warn_count!=null?stats.warn_count:'-'" label="Warned" color="yellow" />
    </div>
    <div class="stats-grid" v-else><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>

    <div class="tab-bar">
      <button class="tab-btn" :class="{active:activeTab==='rules'}" @click="activeTab='rules'"><Icon name="file-text" :size="14"/> Policy Rules ({{ rules.length }})</button>
      <button class="tab-btn" :class="{active:activeTab==='paths'}" @click="activeTab='paths'; loadContexts()"><Icon name="git-branch" :size="14"/> Active Paths ({{ contexts.length }})</button>
      <button class="tab-btn" :class="{active:activeTab==='events'}" @click="activeTab='events'; loadEvents()"><Icon name="clipboard" :size="14"/> Decision Log ({{ events.length }})</button>
    </div>

    <!-- Rules Tab -->
    <div v-if="activeTab==='rules'" class="section">
      <div class="rules-toolbar">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="ruleSearch" placeholder="Search rules..." class="search-input"/>
        </div>
        <button class="btn btn-primary btn-sm" @click="openNewRule"><Icon name="plus" :size="14"/> New Rule</button>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>Name</th><th>Type</th><th>Action</th><th>Priority</th><th>Description</th><th>Enabled</th><th>Actions</th></tr></thead>
          <tbody>
            <tr v-for="r in filteredRules" :key="r.id">
              <td class="td-mono">{{ r.name }}</td>
              <td><span class="type-badge" :class="'type-'+r.rule_type">{{ r.rule_type }}</span></td>
              <td><span class="action-badge" :class="'action-'+r.action">{{ r.action }}</span></td>
              <td class="td-mono">{{ r.priority }}</td>
              <td class="td-desc">{{ r.description }}</td>
              <td><label class="switch"><input type="checkbox" :checked="r.enabled" @change="toggleEnabled(r)"/><span class="slider"></span></label></td>
              <td class="td-actions">
                <button class="btn-icon" @click="editRule(r)" title="Edit"><Icon name="edit" :size="14"/></button>
                <button class="btn-icon btn-icon-danger" @click="confirmDeleteRule(r)" title="Delete"><Icon name="trash" :size="14"/></button>
              </td>
            </tr>
            <tr v-if="filteredRules.length===0"><td colspan="7" class="empty-row">No rules found</td></tr>
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
            <span>Steps: {{ (ctx.steps||[]).length }}</span>
            <span>Tools: {{ (ctx.tool_history||[]).length }}</span>
          </div>
        </div>
      </div>
      <div v-else class="empty-state"><Icon name="git-branch" :size="48" color="#6366F1"/><p>No active paths</p></div>
    </div>

    <!-- Events Tab -->
    <div v-if="activeTab==='events'" class="section">
      <div class="events-toolbar">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="eventSearch" placeholder="Search trace ID..." class="search-input"/>
        </div>
        <select v-model="eventSince" class="select-sm" @change="loadEvents">
          <option value="">All time</option>
          <option value="1h">Last 1h</option>
          <option value="24h">Last 24h</option>
          <option value="7d">Last 7d</option>
        </select>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>Time</th><th>Trace ID</th><th>Rule</th><th>Decision</th><th>Risk Score</th><th>Reason</th><th>Path Len</th></tr></thead>
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
            <tr v-if="filteredEvents.length===0"><td colspan="7" class="empty-row">No events found</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Rule Modal -->
    <div class="modal-overlay" v-if="showRuleModal" @click.self="showRuleModal=false">
      <div class="modal">
        <div class="modal-header">
          <h3>{{ editingRule ? 'Edit Rule' : 'New Rule' }}</h3>
          <button class="btn-close" @click="showRuleModal=false"><Icon name="x-circle" :size="18"/></button>
        </div>
        <div class="modal-body">
          <div class="form-row"><label>ID</label><input v-model="ruleForm.id" class="field-input" :disabled="!!editingRule" placeholder="pp-xxx"/></div>
          <div class="form-row"><label>Name</label><input v-model="ruleForm.name" class="field-input" placeholder="rule_name"/></div>
          <div class="form-row"><label>Type</label>
            <select v-model="ruleForm.rule_type" class="field-input">
              <option value="sequence">Sequence</option><option value="cumulative">Cumulative</option><option value="degradation">Degradation</option>
            </select>
          </div>
          <div class="form-row"><label>Conditions (JSON)</label><textarea v-model="ruleForm.conditions" class="field-input" rows="3"></textarea></div>
          <div class="form-row"><label>Action</label>
            <select v-model="ruleForm.action" class="field-input"><option value="block">Block</option><option value="warn">Warn</option><option value="log">Log</option></select>
          </div>
          <div class="form-row"><label>Priority</label><input v-model.number="ruleForm.priority" class="field-input" type="number"/></div>
          <div class="form-row"><label>Description</label><input v-model="ruleForm.description" class="field-input"/></div>
          <div class="form-row"><label><input type="checkbox" v-model="ruleForm.enabled"/> Enabled</label></div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="showRuleModal=false">Cancel</button>
          <button class="btn btn-primary" @click="saveRule" :disabled="saving">{{ saving?'Saving...': editingRule?'Update':'Create' }}</button>
        </div>
      </div>
    </div>

    <!-- Delete Confirm -->
    <div class="modal-overlay" v-if="deleteTarget" @click.self="deleteTarget=null">
      <div class="modal modal-sm">
        <div class="modal-header"><h3>Confirm Delete</h3></div>
        <div class="modal-body"><p>Delete rule <strong>{{ deleteTarget.name }}</strong>?</p></div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="deleteTarget=null">Cancel</button>
          <button class="btn btn-danger" @click="doDelete">Delete</button>
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
      ruleSearch: '', eventSearch: '', eventSince: '',
      showRuleModal: false, editingRule: null, ruleForm: this.emptyForm(), saving: false, deleteTarget: null,
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
  }
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

@media (max-width: 768px) {
  .pathpolicy-page { padding: var(--space-3); }
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .paths-grid { grid-template-columns: 1fr; }
  .rules-toolbar, .events-toolbar { flex-direction: column; }
}
</style>