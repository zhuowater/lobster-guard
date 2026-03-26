<template>
<div class="page"><div class="page-header"><div><h1 class="page-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg> IFC 信息流控制</h1>
<p class="page-subtitle">Bell-LaPadula 信息流控制引擎 — 机密性上行、完整性下行</p></div>
<button class="btn btn-sm" @click="loadAll"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg> Refresh</button></div>

<div class="stats-grid">
<div class="stat-card" v-for="s in statCards" :key="s.label"><div class="stat-val" :style="{color:s.color}">{{ s.value }}</div><div class="stat-label">{{ s.label }}</div></div>
</div>

<div class="tab-bar">
<button class="tab-btn" :class="{active:tab==='source-rules'}" @click="tab='source-rules'">Source Rules ({{ sourceRules.length }})</button>
<button class="tab-btn" :class="{active:tab==='tool-reqs'}" @click="tab='tool-reqs'">Tool Requirements ({{ toolReqs.length }})</button>
<button class="tab-btn" :class="{active:tab==='variables'}" @click="tab='variables'">Variables</button>
<button class="tab-btn" :class="{active:tab==='violations'}" @click="tab='violations'">Violations ({{ violations.length }})</button>
<button class="tab-btn" :class="{active:tab==='check'}" @click="tab='check'">Live Check</button>
</div>

<!-- Source Rules Tab -->
<div v-if="tab==='source-rules'" class="section">
<div class="section-header"><h3>Source Rules</h3><button class="btn btn-primary btn-sm" @click="showAddSource=true">+ Add</button></div>
<table class="data-table" v-if="sourceRules.length"><thead><tr><th>Source</th><th>Confidentiality</th><th>Integrity</th><th>Actions</th></tr></thead>
<tbody><tr v-for="r in sourceRules" :key="r.source"><td class="mono">{{ r.source }}</td>
<td><span class="badge" :class="'conf-'+r.label.confidentiality">{{ confLabel(r.label.confidentiality) }}</span></td>
<td><span class="badge" :class="'integ-'+r.label.integrity">{{ integLabel(r.label.integrity) }}</span></td>
<td><button class="btn-icon" @click="editSource(r)" title="Edit">✏️</button>
<button class="btn-icon" @click="deleteSource(r.source)" title="Delete">🗑️</button></td></tr></tbody></table>
<div v-else class="empty">No source rules</div>

<!-- Add/Edit Source Rule Modal -->
<div v-if="showAddSource" class="modal-overlay" @click.self="showAddSource=false"><div class="modal">
<h3>{{ editingSource ? 'Edit' : 'Add' }} Source Rule</h3>
<div class="form-row"><label>Source</label><input v-model="sourceForm.source" :disabled="!!editingSource" class="field-input"/></div>
<div class="form-row"><label>Confidentiality</label><select v-model.number="sourceForm.confidentiality" class="field-input"><option :value="0">PUBLIC</option><option :value="1">INTERNAL</option><option :value="2">CONFIDENTIAL</option><option :value="3">SECRET</option></select></div>
<div class="form-row"><label>Integrity</label><select v-model.number="sourceForm.integrity" class="field-input"><option :value="0">TAINT</option><option :value="1">LOW</option><option :value="2">MEDIUM</option><option :value="3">HIGH</option></select></div>
<div class="modal-actions"><button class="btn" @click="showAddSource=false">Cancel</button><button class="btn btn-primary" @click="saveSource">Save</button></div>
</div></div>
</div>

<!-- Tool Requirements Tab -->
<div v-if="tab==='tool-reqs'" class="section">
<div class="section-header"><h3>Tool Requirements</h3><button class="btn btn-primary btn-sm" @click="showAddTool=true">+ Add</button></div>
<table class="data-table" v-if="toolReqs.length"><thead><tr><th>Tool</th><th>Required Integrity</th><th>Max Confidentiality</th><th>Actions</th></tr></thead>
<tbody><tr v-for="r in toolReqs" :key="r.tool"><td class="mono">{{ r.tool }}</td>
<td><span class="badge" :class="'integ-'+r.required_integrity">{{ integLabel(r.required_integrity) }}</span></td>
<td><span class="badge" :class="'conf-'+r.max_confidentiality">{{ confLabel(r.max_confidentiality) }}</span></td>
<td><button class="btn-icon" @click="editTool(r)" title="Edit">✏️</button>
<button class="btn-icon" @click="deleteTool(r.tool)" title="Delete">🗑️</button></td></tr></tbody></table>
<div v-else class="empty">No tool requirements</div>

<!-- Add/Edit Tool Modal -->
<div v-if="showAddTool" class="modal-overlay" @click.self="showAddTool=false"><div class="modal">
<h3>{{ editingTool ? 'Edit' : 'Add' }} Tool Requirement</h3>
<div class="form-row"><label>Tool</label><input v-model="toolForm.tool" :disabled="!!editingTool" class="field-input"/></div>
<div class="form-row"><label>Required Integrity</label><select v-model.number="toolForm.required_integrity" class="field-input"><option :value="0">TAINT</option><option :value="1">LOW</option><option :value="2">MEDIUM</option><option :value="3">HIGH</option></select></div>
<div class="form-row"><label>Max Confidentiality</label><select v-model.number="toolForm.max_confidentiality" class="field-input"><option :value="0">PUBLIC</option><option :value="1">INTERNAL</option><option :value="2">CONFIDENTIAL</option><option :value="3">SECRET</option></select></div>
<div class="modal-actions"><button class="btn" @click="showAddTool=false">Cancel</button><button class="btn btn-primary" @click="saveTool">Save</button></div>
</div></div>
</div>

<!-- Variables Tab -->
<div v-if="tab==='variables'" class="section">
<div class="section-header"><h3>Variables</h3>
<div class="inline-form"><input v-model="varTraceId" placeholder="trace_id" class="field-input" @keyup.enter="loadVars"/>
<button class="btn btn-sm" @click="loadVars">Search</button></div></div>
<table class="data-table" v-if="variables.length"><thead><tr><th>ID</th><th>Name</th><th>Source</th><th>Conf</th><th>Integ</th><th>Parents</th></tr></thead>
<tbody><tr v-for="v in variables" :key="v.id"><td class="mono">{{ (v.id||'').substring(0,16) }}</td><td>{{ v.name }}</td><td class="mono">{{ v.source }}</td>
<td><span class="badge" :class="'conf-'+v.label.confidentiality">{{ confLabel(v.label.confidentiality) }}</span></td>
<td><span class="badge" :class="'integ-'+v.label.integrity">{{ integLabel(v.label.integrity) }}</span></td>
<td class="mono">{{ (v.parents||[]).map(p=>p.substring(0,8)).join(', ') || '-' }}</td></tr></tbody></table>
<div v-else class="empty">Enter a trace_id to search variables</div>
</div>

<!-- Violations Tab -->
<div v-if="tab==='violations'" class="section">
<table class="data-table" v-if="violations.length"><thead><tr><th>Type</th><th>Variable</th><th>Tool</th><th>Label</th><th>Action</th><th>Time</th></tr></thead>
<tbody><tr v-for="v in violations" :key="v.id">
<td><span class="badge" :class="'viol-'+v.type">{{ v.type }}</span></td>
<td class="mono">{{ (v.variable||'').substring(0,12) }}</td>
<td class="mono">{{ v.tool }}</td>
<td>conf={{ confLabel(v.var_label.confidentiality) }}, integ={{ integLabel(v.var_label.integrity) }}</td>
<td><span class="badge" :class="'dec-'+v.action">{{ v.action }}</span></td>
<td>{{ formatTime(v.timestamp) }}</td></tr></tbody></table>
<div v-else class="empty">No violations</div>
</div>

<!-- Live Check Tab -->
<div v-if="tab==='check'" class="section">
<div class="check-form">
<div class="form-row"><label>Trace ID</label><input v-model="checkForm.trace_id" class="field-input"/></div>
<div class="form-row"><label>Tool</label><input v-model="checkForm.tool" class="field-input" placeholder="e.g. send_email, shell_exec"/></div>
<div class="form-row"><label>Input Var IDs (comma-sep)</label><input v-model="checkForm.var_ids_str" class="field-input" placeholder="ifc-xxxx,ifc-yyyy"/></div>
<button class="btn btn-primary" @click="runCheck" :disabled="checking">{{ checking ? 'Checking...' : 'Check' }}</button>
</div>
<div v-if="checkResult" class="check-result" :class="'result-'+checkResult.decision">
<div class="result-header"><span class="result-icon">{{ checkResult.allowed ? '✅' : '🚫' }}</span>
<span class="result-decision">{{ checkResult.decision.toUpperCase() }}</span></div>
<div class="result-reason">{{ checkResult.reason }}</div>
<pre v-if="checkResult.violation" class="result-violation">{{ JSON.stringify(checkResult.violation, null, 2) }}</pre>
</div>
</div>
</div>
</template>
<script>
import { api, apiPost, apiPut, apiDelete } from '../api.js'
export default {
  name: 'IFC',
  data() { return {
    tab: 'source-rules', stats: {}, sourceRules: [], toolReqs: [], variables: [], violations: [],
    varTraceId: '', showAddSource: false, showAddTool: false, editingSource: null, editingTool: null,
    sourceForm: { source: '', confidentiality: 1, integrity: 2 },
    toolForm: { tool: '', required_integrity: 2, max_confidentiality: 1 },
    checkForm: { trace_id: '', tool: '', var_ids_str: '' },
    checkResult: null, checking: false
  }},
  computed: {
    statCards() { const s = this.stats; return [
      {label:'Total Variables', value: s.total_variables??0, color:'#6366F1'},
      {label:'Active Traces', value: s.active_traces??0, color:'#8B5CF6'},
      {label:'Violations', value: s.total_violations??0, color:'#F59E0B'},
      {label:'Blocked', value: s.total_blocked??0, color:'#EF4444'}
    ]}
  },
  mounted() { this.loadAll() },
  methods: {
    confLabel(v) { return ['PUBLIC','INTERNAL','CONFIDENTIAL','SECRET'][v]||'?' },
    integLabel(v) { return ['TAINT','LOW','MEDIUM','HIGH'][v]||'?' },
    formatTime(t) { if(!t) return '-'; return new Date(t).toLocaleString() },
    async loadAll() {
      try { this.stats = await api('/api/v1/ifc/stats') } catch(e){}
      try { const d = await api('/api/v1/ifc/source-rules'); this.sourceRules = d.rules||[] } catch(e){}
      try { const d = await api('/api/v1/ifc/tool-requirements'); this.toolReqs = d.requirements||[] } catch(e){}
      try { const d = await api('/api/v1/ifc/violations?limit=50'); this.violations = d.violations||[] } catch(e){}
    },
    async loadVars() {
      if(!this.varTraceId) return
      try { const d = await api('/api/v1/ifc/variables?trace_id='+encodeURIComponent(this.varTraceId)); this.variables = d.variables||[] } catch(e){ alert(e.message||e) }
    },
    editSource(r) { this.editingSource = r.source; this.sourceForm = { source: r.source, confidentiality: r.label.confidentiality, integrity: r.label.integrity }; this.showAddSource = true },
    async saveSource() {
      try {
        if(this.editingSource) { await apiPut('/api/v1/ifc/source-rules/'+encodeURIComponent(this.editingSource), { confidentiality: this.sourceForm.confidentiality, integrity: this.sourceForm.integrity }) }
        else { await apiPost('/api/v1/ifc/source-rules', { source: this.sourceForm.source, label: { confidentiality: this.sourceForm.confidentiality, integrity: this.sourceForm.integrity }}) }
        this.showAddSource = false; this.editingSource = null; this.loadAll()
      } catch(e) { alert(e.message||e) }
    },
    async deleteSource(src) { if(!confirm('Delete source rule: '+src+'?')) return; try { await apiDelete('/api/v1/ifc/source-rules/'+encodeURIComponent(src)); this.loadAll() } catch(e){ alert(e.message||e) } },
    editTool(r) { this.editingTool = r.tool; this.toolForm = { tool: r.tool, required_integrity: r.required_integrity, max_confidentiality: r.max_confidentiality }; this.showAddTool = true },
    async saveTool() {
      try {
        if(this.editingTool) { await apiPut('/api/v1/ifc/tool-requirements/'+encodeURIComponent(this.editingTool), { required_integrity: this.toolForm.required_integrity, max_confidentiality: this.toolForm.max_confidentiality }) }
        else { await apiPost('/api/v1/ifc/tool-requirements', { tool: this.toolForm.tool, required_integrity: this.toolForm.required_integrity, max_confidentiality: this.toolForm.max_confidentiality }) }
        this.showAddTool = false; this.editingTool = null; this.loadAll()
      } catch(e) { alert(e.message||e) }
    },
    async deleteTool(tool) { if(!confirm('Delete tool requirement: '+tool+'?')) return; try { await apiDelete('/api/v1/ifc/tool-requirements/'+encodeURIComponent(tool)); this.loadAll() } catch(e){ alert(e.message||e) } },
    async runCheck() {
      this.checking = true; this.checkResult = null
      try {
        const ids = this.checkForm.var_ids_str.split(',').map(s=>s.trim()).filter(Boolean)
        this.checkResult = await apiPost('/api/v1/ifc/check', { trace_id: this.checkForm.trace_id, tool: this.checkForm.tool, input_var_ids: ids })
      } catch(e) { alert(e.message||e) }
      this.checking = false
    }
  }
}
</script>
<style scoped>
.page { padding: var(--space-6); }
.page-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:var(--space-6); }
.page-title { font-size:var(--text-xl); font-weight:700; display:flex; align-items:center; gap:var(--space-2); }
.page-subtitle { font-size:var(--text-sm); color:var(--text-secondary); margin-top:var(--space-1); }
.stats-grid { display:grid; grid-template-columns:repeat(4,1fr); gap:var(--space-4); margin-bottom:var(--space-6); }
.stat-card { background:var(--bg-card); border:1px solid var(--border-subtle); border-radius:var(--radius-lg); padding:var(--space-4); }
.stat-val { font-size:var(--text-2xl); font-weight:700; }
.stat-label { font-size:var(--text-xs); color:var(--text-tertiary); margin-top:var(--space-1); }
.tab-bar { display:flex; gap:var(--space-1); margin-bottom:var(--space-4); border-bottom:1px solid var(--border-subtle); padding-bottom:var(--space-2); }
.tab-btn { padding:var(--space-2) var(--space-3); border:none; background:none; cursor:pointer; font-size:var(--text-sm); color:var(--text-secondary); border-radius:var(--radius-md); }
.tab-btn.active { color:#6366F1; background:rgba(99,102,241,0.08); font-weight:600; }
.section { margin-top:var(--space-4); }
.section-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:var(--space-3); }
.section-header h3 { font-size:var(--text-lg); font-weight:600; }
.data-table { width:100%; border-collapse:collapse; font-size:var(--text-sm); }
.data-table th,.data-table td { padding:var(--space-2) var(--space-3); text-align:left; border-bottom:1px solid var(--border-subtle); }
.data-table th { font-weight:600; color:var(--text-secondary); font-size:var(--text-xs); text-transform:uppercase; }
.mono { font-family:var(--font-mono); font-size:var(--text-xs); }
.badge { padding:2px 8px; border-radius:var(--radius-full); font-size:10px; font-weight:600; }
.conf-0 { background:rgba(34,197,94,0.1); color:#16A34A; }
.conf-1 { background:rgba(59,130,246,0.1); color:#2563EB; }
.conf-2 { background:rgba(245,158,11,0.1); color:#D97706; }
.conf-3 { background:rgba(239,68,68,0.1); color:#EF4444; }
.integ-0 { background:rgba(239,68,68,0.1); color:#EF4444; }
.integ-1 { background:rgba(245,158,11,0.1); color:#D97706; }
.integ-2 { background:rgba(59,130,246,0.1); color:#2563EB; }
.integ-3 { background:rgba(34,197,94,0.1); color:#16A34A; }
.viol-confidentiality { background:rgba(239,68,68,0.1); color:#EF4444; }
.viol-integrity { background:rgba(245,158,11,0.1); color:#D97706; }
.dec-block { background:rgba(239,68,68,0.1); color:#EF4444; }
.dec-warn { background:rgba(245,158,11,0.1); color:#D97706; }
.dec-allow { background:rgba(34,197,94,0.1); color:#16A34A; }
.dec-log { background:rgba(99,102,241,0.08); color:#6366F1; }
.btn { display:inline-flex; align-items:center; gap:var(--space-1); padding:var(--space-2) var(--space-4); border:1px solid var(--border-subtle); border-radius:var(--radius-md); background:var(--bg-card); cursor:pointer; font-size:var(--text-sm); }
.btn-sm { padding:var(--space-1) var(--space-3); font-size:var(--text-xs); }
.btn-primary { background:#6366F1; color:#fff; border-color:#6366F1; }
.btn-icon { background:none; border:none; cursor:pointer; padding:2px 4px; font-size:14px; }
.empty { text-align:center; padding:var(--space-8); color:var(--text-tertiary); }
.inline-form { display:flex; gap:var(--space-2); align-items:center; }
.field-input { padding:var(--space-2); border:1px solid var(--border-subtle); border-radius:var(--radius-md); font-size:var(--text-sm); min-width:160px; background:var(--bg-card); color:var(--text-primary); }
.modal-overlay { position:fixed; top:0; left:0; right:0; bottom:0; background:rgba(0,0,0,0.5); display:flex; align-items:center; justify-content:center; z-index:1000; }
.modal { background:var(--bg-card); border-radius:var(--radius-lg); padding:var(--space-6); min-width:400px; max-width:500px; }
.modal h3 { margin-bottom:var(--space-4); }
.form-row { display:flex; justify-content:space-between; align-items:center; padding:var(--space-2) 0; gap:var(--space-3); }
.form-row label { font-size:var(--text-sm); font-weight:500; white-space:nowrap; }
.modal-actions { display:flex; gap:var(--space-2); justify-content:flex-end; margin-top:var(--space-4); }
.check-form { max-width:500px; }
.check-result { margin-top:var(--space-4); padding:var(--space-4); border-radius:var(--radius-lg); border:1px solid var(--border-subtle); }
.result-allow { border-color:#16A34A; }
.result-warn { border-color:#D97706; }
.result-block { border-color:#EF4444; }
.result-header { display:flex; align-items:center; gap:var(--space-2); font-size:var(--text-lg); font-weight:700; margin-bottom:var(--space-2); }
.result-reason { font-size:var(--text-sm); color:var(--text-secondary); }
.result-violation { margin-top:var(--space-3); font-size:var(--text-xs); background:var(--bg-muted,#f1f5f9); padding:var(--space-3); border-radius:var(--radius-md); overflow-x:auto; }
</style>
