<template>
<div class="page"><div class="page-header"><div><h1 class="page-title"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg> 计划偏差检测器</h1>
<p class="page-subtitle">检测并自动修复实际工具调用与执行计划模板之间的偏差</p></div>
<button class="btn btn-sm" @click="loadAll"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg> 刷新</button></div>

<div class="stats-grid">
<div class="stat-card" v-for="s in statCards" :key="s.label"><div class="stat-val" :style="{color:s.color}">{{ s.value }}</div><div class="stat-label">{{ s.label }}</div></div>
</div>

<div class="tab-bar">
<button class="tab-btn" :class="{active:tab==='list'}" @click="tab='list'">偏差列表 ({{ deviations.length }})</button>
<button class="tab-btn" :class="{active:tab==='config'}" @click="tab='config'">配置</button>
</div>

<div v-if="tab==='list'" class="section">
<table class="data-table" v-if="deviations.length"><thead><tr><th>ID</th><th>类型</th><th>工具</th><th>严重度</th><th>决策</th><th>已修复</th><th>Trace</th></tr></thead>
<tbody><tr v-for="d in deviations" :key="d.id"><td class="mono">{{ (d.id||'').substring(0,12) }}</td>
<td><span class="type-badge">{{ d.type }}</span></td><td class="mono">{{ d.tool_name }}</td>
<td><span class="badge" :class="'sev-'+d.severity">{{ d.severity }}</span></td>
<td><span class="badge" :class="'dec-'+d.decision">{{ d.decision }}</span></td>
<td>{{ d.repaired ? '是' : '-' }}</td>
<td class="mono">{{ (d.trace_id||'').substring(0,12) }}</td></tr></tbody></table>
<div v-else class="empty">暂无偏差检测</div>
</div>

<div v-if="tab==='config'" class="section">
<div class="config-form">
<div class="config-row"><label>启用</label><label class="toggle"><input type="checkbox" v-model="configForm.enabled"/><span class="slider"></span></label></div>
<div class="config-row"><label>自动修复</label><label class="toggle"><input type="checkbox" v-model="configForm.auto_repair"/><span class="slider"></span></label></div>
<div class="config-row"><label>每 Trace 最大修复数</label><input v-model.number="configForm.max_repairs" type="number" class="field-input" min="0"/></div>
<button class="btn btn-primary" @click="saveConfig" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button>
</div>
</div>
</div>
</template>
<script>
import { api, apiPut } from '../api.js'
export default {
  name: 'PlanDeviation',
  data() { return { tab: 'list', deviations: [], stats: {}, configForm: { enabled: false, auto_repair: false, max_repairs: 5 }, saving: false } },
  computed: {
    statCards() { const s = this.stats; return [
      {label:'总检查数', value: s.total_checks??0, color:'#6366F1'},
      {label:'偏差数', value: s.total_deviations??0, color:'#F59E0B'},
      {label:'严重', value: s.critical_count??0, color:'#EF4444'},
      {label:'已修复', value: s.repairs_applied??0, color:'#10B981'}
    ]}
  },
  mounted() { this.loadAll() },
  methods: {
    async loadAll() {
      try { const d = await api('/api/v1/deviations'); this.deviations = d.deviations||[] } catch(e){}
      try { this.stats = await api('/api/v1/deviations/stats') } catch(e){}
      try { const c = await api('/api/v1/deviations/config'); Object.assign(this.configForm, c) } catch(e){}
    },
    async saveConfig() {
      this.saving = true
      try { await apiPut('/api/v1/deviations/config', this.configForm) } catch(e) { alert(e.message||e) }
      this.saving = false
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
.data-table { width:100%; border-collapse:collapse; font-size:var(--text-sm); }
.data-table th,.data-table td { padding:var(--space-2) var(--space-3); text-align:left; border-bottom:1px solid var(--border-subtle); }
.data-table th { font-weight:600; color:var(--text-secondary); font-size:var(--text-xs); text-transform:uppercase; }
.mono { font-family:var(--font-mono); font-size:var(--text-xs); }
.badge,.type-badge { padding:2px 8px; border-radius:var(--radius-full); font-size:10px; font-weight:600; }
.type-badge { background:rgba(99,102,241,0.08); color:#6366F1; }
.sev-critical,.dec-block { background:rgba(239,68,68,0.1); color:#EF4444; }
.sev-moderate,.dec-warn { background:rgba(245,158,11,0.1); color:#D97706; }
.sev-minor,.dec-allow { background:rgba(34,197,94,0.1); color:#16A34A; }
.config-form { max-width:480px; }
.config-row { display:flex; justify-content:space-between; align-items:center; padding:var(--space-3) 0; border-bottom:1px solid var(--border-subtle); }
.config-row label { font-size:var(--text-sm); font-weight:500; }
.field-input { padding:var(--space-2); border:1px solid var(--border-subtle); border-radius:var(--radius-md); font-size:var(--text-sm); width:100px; }
.toggle { position:relative; display:inline-block; width:40px; height:22px; }
.toggle input { opacity:0; width:0; height:0; }
.slider { position:absolute; cursor:pointer; top:0; left:0; right:0; bottom:0; background:#CBD5E1; border-radius:22px; transition:.3s; }
.slider:before { content:''; position:absolute; height:16px; width:16px; left:3px; bottom:3px; background:#fff; border-radius:50%; transition:.3s; }
.toggle input:checked+.slider { background:#6366F1; }
.toggle input:checked+.slider:before { transform:translateX(18px); }
.btn { display:inline-flex; align-items:center; gap:var(--space-1); padding:var(--space-2) var(--space-4); border:1px solid var(--border-subtle); border-radius:var(--radius-md); background:var(--bg-card); cursor:pointer; font-size:var(--text-sm); margin-top:var(--space-4); }
.btn-primary { background:#6366F1; color:#fff; border-color:#6366F1; }
.empty { text-align:center; padding:var(--space-8); color:var(--text-tertiary); }
</style>
