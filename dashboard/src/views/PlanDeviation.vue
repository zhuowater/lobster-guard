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
<table class="data-table" v-if="deviations.length"><thead><tr><th>ID</th><th>类型</th><th>工具</th><th>严重度</th><th>决策</th><th>修复状态</th><th>Trace</th><th>操作</th></tr></thead>
<tbody><tr v-for="d in deviations" :key="d.id"><td class="mono">{{ (d.id||'').substring(0,12) }}</td>
<td><span class="type-badge">{{ d.type }}</span></td><td class="mono">{{ d.tool_name }}</td>
<td><span class="badge" :class="'sev-'+d.severity">{{ d.severity }}</span></td>
<td><span class="badge" :class="'dec-'+d.decision">{{ d.decision }}</span></td>
<td><span v-if="d.repaired" class="badge repaired-badge">✅ 已修复</span><span v-if="d.repaired && d.repaired_tool" class="repair-detail">{{ d.tool_name }} → {{ d.repaired_tool }}</span><span v-if="d.repaired && !d.repaired_tool && d.repaired_args" class="repair-detail">参数已修正</span><span v-if="!d.repaired">-</span></td>
<td class="mono">{{ (d.trace_id||'').substring(0,12) }}</td>
<td><button v-if="d.trace_id" class="link-btn" @click="$router.push('/audit?trace_id=' + d.trace_id)">📋 查看审计日志</button></td></tr></tbody></table>
<EmptyState v-else :iconSvg="emptyIcon" title="暂无偏差检测" description="当检测到执行计划偏差时将显示在这里" />
</div>

<div v-if="tab==='config'" class="section">
<div class="config-form">
<h3 class="config-section-title">基础配置</h3>
<div class="config-row"><label>启用</label><label class="toggle"><input type="checkbox" v-model="configForm.enabled"/><span class="slider"></span></label></div>
<div class="config-row"><label>自动修复</label><label class="toggle"><input type="checkbox" v-model="configForm.auto_repair"/><span class="slider"></span></label></div>
<div class="config-row"><label>每 Trace 最大修复数</label><input v-model.number="configForm.max_repairs" type="number" class="field-input" min="0"/></div>
<button class="btn btn-primary" @click="saveConfig" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button>
</div>

<div class="policies-section">
<div class="section-header"><h3 class="config-section-title">修复策略</h3>
<button class="btn btn-primary btn-sm" @click="openPolicyModal(null)"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg> 添加策略</button></div>
<p class="section-hint">定义不同偏差类型和严重度下的修复动作。匹配时按顺序取第一个命中的已启用策略。</p>
<table class="data-table" v-if="policies.length"><thead><tr><th>ID</th><th>名称</th><th>偏差类型</th><th>严重度</th><th>修复动作</th><th>状态</th><th>操作</th></tr></thead>
<tbody><tr v-for="p in policies" :key="p.id" :class="{'row-disabled':!p.enabled}">
<td class="mono">{{ p.id }}</td><td>{{ p.name }}</td>
<td><span class="type-badge">{{ devTypeLabel(p.deviation_type) }}</span></td>
<td><span class="badge" :class="'sev-'+p.severity">{{ p.severity }}</span></td>
<td><span class="action-badge" :class="'act-'+p.action">{{ actionLabel(p.action) }}</span></td>
<td><span class="badge" :class="p.enabled?'sev-minor':'sev-critical'">{{ p.enabled?'启用':'禁用' }}</span></td>
<td class="actions-cell">
<button class="link-btn" @click="openPolicyModal(p)">编辑</button>
<button class="link-btn link-danger" v-if="!p.builtin" @click="confirmDeletePolicy(p.id)">删除</button>
<span v-if="p.builtin" class="builtin-tag">内置</span>
</td></tr></tbody></table>
<EmptyState v-else :iconSvg="emptyIcon" title="暂无修复策略" description="点击「添加策略」定义偏差修复规则" />
</div>

<!-- Policy Modal -->
<div class="modal-overlay" v-if="showPolicyModal" @click.self="showPolicyModal=false">
<div class="modal-box">
<h3 class="modal-title">{{ editingPolicy ? '编辑修复策略' : '添加修复策略' }}</h3>
<div class="form-group"><label class="form-label">策略 ID</label><input class="field-input-full" v-model="policyForm.id" :disabled="!!editingPolicy" placeholder="如 rp-custom-001" /></div>
<div class="form-group"><label class="form-label">名称</label><input class="field-input-full" v-model="policyForm.name" placeholder="如：生产环境工具阻断" /></div>
<div class="form-group"><label class="form-label">偏差类型</label><select class="field-select" v-model="policyForm.deviation_type">
<option value="*">全部类型</option><option value="out_of_order">乱序 (out_of_order)</option><option value="unexpected">意外工具 (unexpected)</option><option value="capability_violation">权限违规 (capability_violation)</option></select></div>
<div class="form-group"><label class="form-label">严重度</label><select class="field-select" v-model="policyForm.severity">
<option value="*">全部等级</option><option value="minor">minor</option><option value="moderate">moderate</option><option value="critical">critical</option></select></div>
<div class="form-group"><label class="form-label">修复动作</label><select class="field-select" v-model="policyForm.action">
<option value="replace_tool">🔄 工具替换 (replace_tool)</option><option value="sanitize_args">🧹 参数修正 (sanitize_args)</option>
<option value="block">🚫 阻断 (block)</option><option value="log">📝 仅记录 (log)</option><option value="skip">⏭️ 跳过 (skip)</option></select></div>
<div class="form-group"><label class="form-label">描述</label><input class="field-input-full" v-model="policyForm.description" placeholder="策略说明" /></div>
<div class="form-group"><label class="form-label">启用</label><label class="toggle"><input type="checkbox" v-model="policyForm.enabled"/><span class="slider"></span></label></div>
<div class="modal-actions"><button class="btn btn-primary" @click="savePolicy" :disabled="policySaving">{{ policySaving ? '保存中...' : '保存' }}</button><button class="btn btn-ghost" @click="showPolicyModal=false">取消</button></div>
<div v-if="policyMsg" class="config-msg" :class="policyMsgType">{{ policyMsg }}</div>
</div></div>

<!-- Policy Delete Confirm -->
<div class="modal-overlay" v-if="showPolicyDelete" @click.self="showPolicyDelete=false">
<div class="modal-box modal-sm">
<h3 class="modal-title">确认删除</h3><p>确定删除策略 <strong>{{ policyDeleteTarget }}</strong> 吗？</p>
<div class="modal-actions"><button class="btn btn-danger" @click="doDeletePolicy">删除</button><button class="btn btn-ghost" @click="showPolicyDelete=false">取消</button></div>
</div></div>
</div>
</div>
</template>
<script>
import { api, apiPut } from '../api.js'
import EmptyState from '../components/EmptyState.vue'
export default {
  name: 'PlanDeviation',
  components: { EmptyState },
  data() { return { tab: 'list', deviations: [], stats: {}, configForm: { enabled: false, auto_repair: false, max_repairs: 5 }, saving: false,
    policies: [], showPolicyModal: false, editingPolicy: null, policySaving: false, policyMsg: '', policyMsgType: 'success',
    policyForm: { id: '', name: '', deviation_type: '*', severity: '*', action: 'replace_tool', description: '', enabled: true },
    showPolicyDelete: false, policyDeleteTarget: '',
    emptyIcon: '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
  } },
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
      try { const p = await api('/api/v1/deviations/repair-policies'); this.policies = p.policies||[] } catch(e){}
    },
    async saveConfig() {
      this.saving = true
      try { await apiPut('/api/v1/deviations/config', this.configForm) } catch(e) { alert(e.message||e) }
      this.saving = false
    },
    devTypeLabel(t) { return {'out_of_order':'乱序','unexpected':'意外工具','capability_violation':'权限违规','*':'全部'}[t]||t },
    actionLabel(a) { return {'replace_tool':'工具替换','sanitize_args':'参数修正','block':'阻断','log':'仅记录','skip':'跳过'}[a]||a },
    openPolicyModal(p) {
      this.policyMsg = '';
      if (p) {
        this.editingPolicy = p;
        this.policyForm = { ...p };
      } else {
        this.editingPolicy = null;
        this.policyForm = { id: '', name: '', deviation_type: '*', severity: '*', action: 'replace_tool', description: '', enabled: true };
      }
      this.showPolicyModal = true;
    },
    async savePolicy() {
      this.policySaving = true; this.policyMsg = '';
      try {
        if (this.editingPolicy) {
          await apiPut(`/api/v1/deviations/repair-policies/${this.policyForm.id}`, this.policyForm);
        } else {
          await api('/api/v1/deviations/repair-policies', { method: 'POST', body: JSON.stringify(this.policyForm) });
        }
        this.policyMsg = '保存成功'; this.policyMsgType = 'success';
        this.showPolicyModal = false; this.loadAll();
      } catch(e) { this.policyMsg = '失败: ' + (e.message||e); this.policyMsgType = 'error'; }
      this.policySaving = false;
    },
    confirmDeletePolicy(id) { this.policyDeleteTarget = id; this.showPolicyDelete = true; },
    async doDeletePolicy() {
      try {
        await api(`/api/v1/deviations/repair-policies/${this.policyDeleteTarget}`, { method: 'DELETE' });
        this.showPolicyDelete = false; this.loadAll();
      } catch(e) { alert('删除失败: ' + (e.message||e)); }
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
.link-btn { background:none; border:1px solid var(--border-subtle); border-radius:var(--radius-md); cursor:pointer; padding:2px 8px; font-size:11px; color:#6366F1; transition:all .2s; white-space:nowrap; }
.link-btn:hover { background:rgba(99,102,241,0.08); border-color:#6366F1; }
.repaired-badge { background:rgba(16,185,129,0.12); color:#059669; }
.repair-detail { display:block; font-size:10px; color:var(--text-secondary); margin-top:2px; font-family:var(--font-mono); }
.config-section-title { font-size:var(--text-base); font-weight:700; margin-bottom:var(--space-3); margin-top:var(--space-2); }
.policies-section { margin-top:var(--space-6); }
.section-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:var(--space-2); }
.section-hint { font-size:var(--text-xs); color:var(--text-tertiary); margin-bottom:var(--space-3); }
.actions-cell { display:flex; gap:var(--space-2); align-items:center; }
.link-btn { background:none; border:none; color:#6366F1; cursor:pointer; font-size:var(--text-xs); padding:2px 4px; }
.link-btn:hover { text-decoration:underline; }
.link-danger { color:#EF4444; }
.builtin-tag { font-size:9px; color:var(--text-tertiary); background:var(--bg-surface); padding:1px 6px; border-radius:var(--radius-full); }
.row-disabled { opacity:0.5; }
.action-badge { padding:2px 8px; border-radius:var(--radius-full); font-size:10px; font-weight:600; }
.act-replace_tool { background:rgba(99,102,241,0.1); color:#6366F1; }
.act-sanitize_args { background:rgba(34,197,94,0.1); color:#16A34A; }
.act-block { background:rgba(239,68,68,0.1); color:#EF4444; }
.act-log { background:rgba(107,114,128,0.1); color:#6B7280; }
.act-skip { background:rgba(245,158,11,0.1); color:#D97706; }
.btn-sm { padding:var(--space-1) var(--space-3); font-size:var(--text-xs); margin-top:0; }
.btn-danger { background:#EF4444; color:#fff; border-color:#EF4444; }
.btn-ghost { background:transparent; border-color:var(--border-subtle); }
.modal-overlay { position:fixed; top:0; left:0; right:0; bottom:0; background:rgba(0,0,0,0.5); display:flex; align-items:center; justify-content:center; z-index:1000; }
.modal-box { background:var(--bg-card); border-radius:var(--radius-xl); padding:var(--space-6); width:480px; max-height:80vh; overflow-y:auto; border:1px solid var(--border-subtle); }
.modal-sm { width:360px; }
.modal-title { font-size:var(--text-lg); font-weight:700; margin-bottom:var(--space-4); }
.form-group { margin-bottom:var(--space-3); }
.form-label { display:block; font-size:var(--text-xs); font-weight:600; color:var(--text-secondary); margin-bottom:var(--space-1); }
.field-input-full { width:100%; padding:var(--space-2); border:1px solid var(--border-subtle); border-radius:var(--radius-md); background:var(--bg-surface); color:var(--text-primary); font-size:var(--text-sm); box-sizing:border-box; }
.field-select { width:100%; padding:var(--space-2); border:1px solid var(--border-subtle); border-radius:var(--radius-md); background:var(--bg-surface); color:var(--text-primary); font-size:var(--text-sm); box-sizing:border-box; }
.modal-actions { display:flex; gap:var(--space-2); justify-content:flex-end; margin-top:var(--space-4); }
.config-msg { margin-top:var(--space-2); font-size:var(--text-xs); padding:var(--space-2); border-radius:var(--radius-md); }
.config-msg.success { background:rgba(34,197,94,0.1); color:#16A34A; }
.config-msg.error { background:rgba(239,68,68,0.1); color:#EF4444; }
</style>
