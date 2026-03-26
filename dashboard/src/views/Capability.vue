<template>
<div class="page"><div class="page-header"><div><h1 class="page-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.78 7.78 5.5 5.5 0 0 1 7.78-7.78m0 0L12 16m0 0l3-3m-3 3l-3 3m9-15l2 2m-2-2v3.5m0 0h3.5"/></svg> 能力标签引擎</h1>
<p class="page-subtitle">数据级能力标签 — 追踪每个数据源允许触发的操作</p></div>
<button class="btn btn-sm" @click="loadAll"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg> 刷新</button></div>

<div class="stats-grid">
<div class="stat-card" v-for="s in statCards" :key="s.label"><div class="stat-val">{{ s.value }}</div><div class="stat-label">{{ s.label }}</div></div>
</div>

<div class="tab-bar">
<button class="tab-btn" :class="{active:tab==='mappings'}" @click="tab='mappings'">工具映射 ({{ mappings.length }})</button>
<button class="tab-btn" :class="{active:tab==='contexts'}" @click="tab='contexts'">活跃上下文</button>
<button class="tab-btn" :class="{active:tab==='evals'}" @click="tab='evals'">评估记录</button>
</div>

<div v-if="tab==='mappings'" class="section">
<table class="data-table" v-if="mappings.length"><thead><tr><th>工具</th><th>分类</th><th>级别</th><th>允许能力</th><th>拒绝能力</th><th>信任度</th></tr></thead>
<tbody><tr v-for="m in mappings" :key="m.tool_name"><td class="mono">{{ m.tool_name }}</td><td>{{ m.category }}</td><td><span class="badge" :class="'lvl-'+m.default_level">{{ m.default_level }}</span></td>
<td>{{ (m.allowed_caps||[]).join(', ')||'-' }}</td><td>{{ (m.denied_caps||[]).join(', ')||'-' }}</td><td>{{ (m.trust_factor||0).toFixed(2) }}</td></tr></tbody></table>
<EmptyState v-else :iconSvg="emptyIcons.mappings" title="暂无工具映射配置" description="配置工具的能力标签来追踪数据源允许的操作" />
</div>

<div v-if="tab==='contexts'" class="section">
<table class="data-table" v-if="contexts.length"><thead><tr><th>Trace ID</th><th>用户</th><th>状态</th><th>创建时间</th></tr></thead>
<tbody><tr v-for="c in contexts" :key="c.trace_id"><td class="mono">{{ c.trace_id }}</td><td>{{ c.user_id||'-' }}</td><td><span class="badge" :class="'st-'+c.status">{{ c.status }}</span></td><td class="mono">{{ c.created_at }}</td></tr></tbody></table>
<EmptyState v-else :iconSvg="emptyIcons.contexts" title="暂无活跃上下文" description="活跃的能力评估上下文将显示在这里" />
</div>

<div v-if="tab==='evals'" class="section">
<table class="data-table" v-if="evals.length"><thead><tr><th>时间</th><th>工具</th><th>动作</th><th>决策</th><th>原因</th><th>Trace</th></tr></thead>
<tbody><tr v-for="e in evals" :key="e.created_at+e.tool_name"><td class="mono">{{ e.created_at }}</td><td class="mono">{{ e.tool_name }}</td><td>{{ e.action }}</td>
<td><span class="badge" :class="'dec-'+e.decision">{{ e.decision }}</span></td><td>{{ e.reason||'-' }}</td><td class="mono">{{ (e.trace_id||'').substring(0,12) }}</td></tr></tbody></table>
<EmptyState v-else :iconSvg="emptyIcons.evals" title="暂无评估记录" description="工具调用的能力评估记录将显示在这里" />
</div>
</div>
</template>
<script>
import { api } from '../api.js'
import EmptyState from '../components/EmptyState.vue'
export default {
  name: 'Capability',
  components: { EmptyState },
  data() { return { tab: 'mappings', mappings: [], contexts: [], evals: [], stats: {},
    emptyIcons: {
      mappings: '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.78 7.78 5.5 5.5 0 0 1 7.78-7.78m0 0L12 16m0 0l3-3m-3 3l-3 3m9-15l2 2m-2-2v3.5m0 0h3.5"/></svg>',
      contexts: '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>',
      evals: '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>'
    }
  } },
  computed: {
    statCards() { const s = this.stats; return [
      {label:'工具映射', value: s.tool_mapping_count??0}, {label:'活跃上下文', value: s.total_contexts??0},
      {label:'活跃上下文', value: s.active_contexts??0}, {label:'已拒绝', value: s.deny_count??0}
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
