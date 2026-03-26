<template>
<div class="page"><div class="page-header"><div><h1 class="page-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.78 7.78 5.5 5.5 0 0 1 7.78-7.78m0 0L12 16m0 0l3-3m-3 3l-3 3m9-15l2 2m-2-2v3.5m0 0h3.5"/></svg> Capability Engine</h1>
<p class="page-subtitle">Data-level capability tagging — track what operations each data source is allowed to trigger</p></div>
<button class="btn btn-sm" @click="loadAll"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg> Refresh</button></div>

<div class="stats-grid">
<div class="stat-card" v-for="s in statCards" :key="s.label"><div class="stat-val">{{ s.value }}</div><div class="stat-label">{{ s.label }}</div></div>
</div>

<div class="tab-bar">
<button class="tab-btn" :class="{active:tab==='mappings'}" @click="tab='mappings'">Tool Mappings ({{ mappings.length }})</button>
<button class="tab-btn" :class="{active:tab==='contexts'}" @click="tab='contexts'">Active Contexts</button>
<button class="tab-btn" :class="{active:tab==='evals'}" @click="tab='evals'">Evaluations</button>
</div>

<div v-if="tab==='mappings'" class="section">
<table class="data-table" v-if="mappings.length"><thead><tr><th>Tool</th><th>Category</th><th>Level</th><th>Allowed Caps</th><th>Denied Caps</th><th>Trust</th></tr></thead>
<tbody><tr v-for="m in mappings" :key="m.tool_name"><td class="mono">{{ m.tool_name }}</td><td>{{ m.category }}</td><td><span class="badge" :class="'lvl-'+m.default_level">{{ m.default_level }}</span></td>
<td>{{ (m.allowed_caps||[]).join(', ')||'-' }}</td><td>{{ (m.denied_caps||[]).join(', ')||'-' }}</td><td>{{ (m.trust_factor||0).toFixed(2) }}</td></tr></tbody></table>
<div v-else class="empty">No tool mappings configured</div>
</div>

<div v-if="tab==='contexts'" class="section">
<table class="data-table" v-if="contexts.length"><thead><tr><th>Trace ID</th><th>User</th><th>Status</th><th>Created</th></tr></thead>
<tbody><tr v-for="c in contexts" :key="c.trace_id"><td class="mono">{{ c.trace_id }}</td><td>{{ c.user_id||'-' }}</td><td><span class="badge" :class="'st-'+c.status">{{ c.status }}</span></td><td class="mono">{{ c.created_at }}</td></tr></tbody></table>
<div v-else class="empty">No active contexts</div>
</div>

<div v-if="tab==='evals'" class="section">
<table class="data-table" v-if="evals.length"><thead><tr><th>Time</th><th>Tool</th><th>Action</th><th>Decision</th><th>Reason</th><th>Trace</th></tr></thead>
<tbody><tr v-for="e in evals" :key="e.created_at+e.tool_name"><td class="mono">{{ e.created_at }}</td><td class="mono">{{ e.tool_name }}</td><td>{{ e.action }}</td>
<td><span class="badge" :class="'dec-'+e.decision">{{ e.decision }}</span></td><td>{{ e.reason||'-' }}</td><td class="mono">{{ (e.trace_id||'').substring(0,12) }}</td></tr></tbody></table>
<div v-else class="empty">No evaluations recorded</div>
</div>
</div>
</template>
<script>
import { api } from '../api.js'
export default {
  name: 'Capability',
  data() { return { tab: 'mappings', mappings: [], contexts: [], evals: [], stats: {} } },
  computed: {
    statCards() { const s = this.stats; return [
      {label:'Tool Mappings', value: s.tool_mapping_count??0}, {label:'Total Contexts', value: s.total_contexts??0},
      {label:'Active Contexts', value: s.active_contexts??0}, {label:'Blocked', value: s.deny_count??0}
    ]}
  },
  mounted() { this.loadAll() },
  methods: {
    async loadAll() {
      try { const d = await api('/api/v1/capabilities/mappings'); this.mappings = d.mappings||[] } catch(e){}
      try { const d = await api('/api/v1/capabilities/contexts'); this.contexts = d.contexts||[] } catch(e){}
      try { const d = await api('/api/v1/capabilities/evaluations'); this.evals = d.evaluations||[] } catch(e){}
      try { this.stats = await api('/api/v1/capabilities/stats') } catch(e){}
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
.stat-val { font-size:var(--text-2xl); font-weight:700; color:#6366F1; }
.stat-label { font-size:var(--text-xs); color:var(--text-tertiary); margin-top:var(--space-1); }
.tab-bar { display:flex; gap:var(--space-1); margin-bottom:var(--space-4); border-bottom:1px solid var(--border-subtle); padding-bottom:var(--space-2); }
.tab-btn { padding:var(--space-2) var(--space-3); border:none; background:none; cursor:pointer; font-size:var(--text-sm); color:var(--text-secondary); border-radius:var(--radius-md); }
.tab-btn.active { color:#6366F1; background:rgba(99,102,241,0.08); font-weight:600; }
.section { margin-top:var(--space-4); }
.data-table { width:100%; border-collapse:collapse; font-size:var(--text-sm); }
.data-table th,.data-table td { padding:var(--space-2) var(--space-3); text-align:left; border-bottom:1px solid var(--border-subtle); }
.data-table th { font-weight:600; color:var(--text-secondary); font-size:var(--text-xs); text-transform:uppercase; }
.mono { font-family:var(--font-mono); font-size:var(--text-xs); }
.badge { padding:2px 8px; border-radius:var(--radius-full); font-size:10px; font-weight:600; }
.lvl-critical,.dec-block { background:rgba(239,68,68,0.1); color:#EF4444; }
.lvl-high,.dec-warn { background:rgba(245,158,11,0.1); color:#D97706; }
.lvl-medium,.dec-allow,.st-active { background:rgba(34,197,94,0.1); color:#16A34A; }
.lvl-low { background:rgba(99,102,241,0.1); color:#6366F1; }
.st-completed { background:rgba(107,114,128,0.1); color:#6B7280; }
.empty { text-align:center; padding:var(--space-8); color:var(--text-tertiary); }
.btn { display:inline-flex; align-items:center; gap:var(--space-1); padding:var(--space-1) var(--space-3); border:1px solid var(--border-subtle); border-radius:var(--radius-md); background:var(--bg-card); cursor:pointer; font-size:var(--text-sm); }
</style>
