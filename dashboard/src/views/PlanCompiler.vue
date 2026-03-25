<template>
  <div class="plan-compiler-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="git-branch" :size="20" /> 执行计划编译器</h1>
        <p class="page-subtitle">CaMeL 网关级程序解释器 -- 编译用户意图为执行计划模板，实时比对 LLM tool_call 序列</p>
      </div>
      <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
    </div>

    <div class="stats-grid" v-if="!initialLoading">
      <StatCard :iconSvg="svgTemplate" :value="stats.total_templates??'-'" label="模板数" color="blue" />
      <StatCard :iconSvg="svgActive" :value="stats.active_plans??'-'" label="活跃计划" color="green" />
      <StatCard :iconSvg="svgCheck" :value="stats.total_evaluated??'-'" label="总评估" color="indigo" />
      <StatCard :iconSvg="svgAlert" :value="stats.total_violations??'-'" label="总违规" color="red" />
    </div>
    <div class="stats-grid" v-else><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>

    <div class="tab-bar">
      <button class="tab-btn" :class="{active:activeTab==='templates'}" @click="activeTab='templates'"><Icon name="file-text" :size="14"/> 模板 ({{ templates.length }})</button>
      <button class="tab-btn" :class="{active:activeTab==='active'}" @click="activeTab='active'"><Icon name="zap" :size="14"/> 活跃计划 ({{ activePlans.length }})</button>
      <button class="tab-btn" :class="{active:activeTab==='history'}" @click="switchTab('history')"><Icon name="clipboard" :size="14"/> 历史</button>
      <button class="tab-btn" :class="{active:activeTab==='stats'}" @click="switchTab('stats')"><Icon name="bar-chart" :size="14"/> 统计</button>
    </div>

    <!-- Templates Tab -->
    <div v-if="activeTab==='templates'" class="section">
      <div class="rules-toolbar">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="templateSearch" placeholder="搜索模板名/分类..." class="search-input"/>
        </div>
        <div class="action-filters">
          <button v-for="cat in categoryOpts" :key="cat" class="btn btn-sm" :class="categoryFilter===cat?'btn-active':'btn-ghost'" @click="categoryFilter=cat">{{ cat||'全部' }}</button>
        </div>
        <button class="btn btn-primary btn-sm" @click="showNewTemplate=true">+ 新建模板</button>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>名称</th><th>分类</th><th>意图模式</th><th>允许工具</th><th>禁止工具</th><th>内置</th><th>启用</th><th>操作</th></tr></thead>
          <tbody><tr v-for="t in filteredTemplates" :key="t.id">
            <td class="td-mono">{{ t.name }}</td>
            <td><span class="cat-badge" :class="'cat-'+t.category">{{ t.category }}</span></td>
            <td class="td-mono td-truncate" :title="t.intent_pattern">{{ truncate(t.intent_pattern, 40) }}</td>
            <td>{{ (t.allowed_sequence||[]).map(s=>s.tool).join(', ') }}</td>
            <td class="td-danger">{{ (t.forbidden_tools||[]).join(', ') }}</td>
            <td><span :class="t.built_in?'badge-on':'badge-off'">{{ t.built_in?'Y':'N' }}</span></td>
            <td><span :class="t.enabled?'badge-on':'badge-off'">{{ t.enabled?'Y':'N' }}</span></td>
            <td class="td-actions">
              <button class="btn-icon" @click="editTemplate(t)" title="编辑"><Icon name="edit" :size="14"/></button>
              <button v-if="!t.built_in" class="btn-icon btn-icon-danger" @click="deleteTemplate(t)" title="删除"><Icon name="trash" :size="14"/></button>
            </td>
          </tr></tbody>
        </table>
        <EmptyState v-if="filteredTemplates.length===0" :iconSvg="svgTemplate" title="暂无模板" description="添加执行计划模板来约束 LLM 工具调用"/>
      </div>
    </div>

    <!-- Active Plans Tab -->
    <div v-if="activeTab==='active'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>Trace ID</th><th>模板</th><th>意图</th><th>状态</th><th>已执行步骤</th><th>违规数</th><th>开始时间</th></tr></thead>
          <tbody><tr v-for="p in activePlans" :key="p.trace_id">
            <td class="td-mono">{{ truncate(p.trace_id, 16) }}</td>
            <td>{{ p.template_name }}</td>
            <td class="td-truncate" :title="p.intent">{{ truncate(p.intent, 40) }}</td>
            <td><span class="status-badge" :class="'status-'+p.status">{{ p.status }}</span></td>
            <td>{{ (p.steps_executed||[]).length }}</td>
            <td :class="{'td-danger':(p.violations||[]).length>0}">{{ (p.violations||[]).length }}</td>
            <td class="td-mono">{{ formatTime(p.started_at) }}</td>
          </tr></tbody>
        </table>
        <EmptyState v-if="activePlans.length===0" :iconSvg="svgActive" title="无活跃计划" description="当 LLM 请求匹配模板时会创建活跃计划"/>
      </div>
    </div>

    <!-- History Tab -->
    <div v-if="activeTab==='history'" class="section">
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>Trace ID</th><th>模板</th><th>意图</th><th>状态</th><th>步骤</th><th>违规</th><th>开始</th><th>完成</th></tr></thead>
          <tbody><tr v-for="p in historyPlans" :key="p.trace_id">
            <td class="td-mono">{{ truncate(p.trace_id, 16) }}</td>
            <td>{{ p.template_name }}</td>
            <td class="td-truncate" :title="p.intent">{{ truncate(p.intent, 35) }}</td>
            <td><span class="status-badge" :class="'status-'+p.status">{{ p.status }}</span></td>
            <td>{{ (p.steps||[]).length }}</td>
            <td :class="{'td-danger':(p.violations||[]).length>0}">{{ (p.violations||[]).length }}</td>
            <td class="td-mono">{{ formatTime(p.started_at) }}</td>
            <td class="td-mono">{{ p.completed_at ? formatTime(p.completed_at) : '--' }}</td>
          </tr></tbody>
        </table>
        <EmptyState v-if="historyPlans.length===0" :iconSvg="svgCheck" title="暂无历史" description="完成或过期的计划将显示在这里"/>
      </div>
    </div>

    <!-- Statistics Tab -->
    <div v-if="activeTab==='stats'" class="section">
      <div class="stats-detail">
        <div class="stat-block">
          <h3 class="section-title">评估统计</h3>
          <div class="stat-row"><span>总评估</span><span class="stat-val">{{ stats.total_evaluated??0 }}</span></div>
          <div class="stat-row"><span>允许</span><span class="stat-val text-green">{{ stats.total_allowed??0 }}</span></div>
          <div class="stat-row"><span>告警</span><span class="stat-val text-yellow">{{ stats.total_warned??0 }}</span></div>
          <div class="stat-row"><span>阻断</span><span class="stat-val text-red">{{ stats.total_blocked??0 }}</span></div>
          <div class="stat-row"><span>总违规</span><span class="stat-val text-red">{{ stats.total_violations??0 }}</span></div>
        </div>
        <div class="stat-block">
          <h3 class="section-title">分类分布</h3>
          <div v-for="(count, cat) in (stats.by_category||{})" :key="cat" class="stat-row">
            <span><span class="cat-badge" :class="'cat-'+cat">{{ cat }}</span></span>
            <span class="stat-val">{{ count }} 模板</span>
          </div>
        </div>
      </div>
    </div>

    <!-- New/Edit Template Modal -->
    <div v-if="showNewTemplate" class="modal-overlay" @click.self="showNewTemplate=false">
      <div class="modal">
        <div class="modal-header"><h3>{{ editingTemplate ? '编辑模板' : '新建模板' }}</h3><button class="btn-close" @click="showNewTemplate=false">X</button></div>
        <div class="modal-body">
          <div class="form-row"><label>名称</label><input v-model="formTemplate.name" class="field-input" placeholder="Plan Name"/></div>
          <div class="form-row"><label>分类</label><input v-model="formTemplate.category" class="field-input" placeholder="query/email/file/code/web/admin"/></div>
          <div class="form-row"><label>意图模式 (|分隔)</label><input v-model="formTemplate.intent_pattern" class="field-input" placeholder="search|find|look up"/></div>
          <div class="form-row"><label>描述</label><textarea v-model="formTemplate.description" class="field-input" rows="2"></textarea></div>
          <div class="form-row"><label>禁止工具 (逗号分隔)</label><input v-model="forbiddenStr" class="field-input" placeholder="shell_exec,send_email"/></div>
          <div class="form-row"><label>启用</label><input type="checkbox" v-model="formTemplate.enabled"/></div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="showNewTemplate=false">取消</button>
          <button class="btn btn-primary" @click="saveTemplate" :disabled="saving">{{ saving?'保存中...':'保存' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import Skeleton from '../components/Skeleton.vue'
import EmptyState from '../components/EmptyState.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'

const svgTemplate = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>'
const svgActive = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>'
const svgCheck = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>'
const svgAlert = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'

export default {
  name: 'PlanCompiler',
  components: { Icon, StatCard, Skeleton, EmptyState },
  data() {
    return {
      activeTab: 'templates',
      initialLoading: true,
      stats: {},
      templates: [],
      activePlans: [],
      historyPlans: [],
      templateSearch: '',
      categoryFilter: '',
      showNewTemplate: false,
      editingTemplate: null,
      formTemplate: { name: '', category: '', intent_pattern: '', description: '', enabled: true },
      forbiddenStr: '',
      saving: false,
      svgTemplate, svgActive, svgCheck, svgAlert,
    }
  },
  computed: {
    categoryOpts() {
      const cats = new Set(this.templates.map(t => t.category).filter(Boolean))
      return ['', ...Array.from(cats).sort()]
    },
    filteredTemplates() {
      let list = this.templates
      if (this.categoryFilter) list = list.filter(t => t.category === this.categoryFilter)
      if (this.templateSearch) {
        const q = this.templateSearch.toLowerCase()
        list = list.filter(t => (t.name||'').toLowerCase().includes(q) || (t.category||'').toLowerCase().includes(q) || (t.intent_pattern||'').toLowerCase().includes(q))
      }
      return list
    },
  },
  methods: {
    async loadAll() {
      this.initialLoading = true
      try {
        const [statsRes, tmplRes, activeRes] = await Promise.all([
          api('/api/v1/plans/stats'),
          api('/api/v1/plans/templates'),
          api('/api/v1/plans/active'),
        ])
        this.stats = statsRes||{}
        this.templates = (tmplRes||{}).templates||[]
        this.activePlans = (activeRes||{}).plans||[]
      } catch(e) { console.error('load error', e) }
      this.initialLoading = false
    },
    async switchTab(tab) {
      this.activeTab = tab
      if (tab === 'history') {
        try {
          const res = await api('/api/v1/plans/history?limit=100')
          this.historyPlans = (res||{}).plans||[]
        } catch(e) { console.error(e) }
      } else if (tab === 'stats') {
        try {
          const res = await api('/api/v1/plans/stats')
          this.stats = res||{}
        } catch(e) { console.error(e) }
      }
    },
    editTemplate(t) {
      this.editingTemplate = t
      this.formTemplate = { ...t }
      this.forbiddenStr = (t.forbidden_tools||[]).join(', ')
      this.showNewTemplate = true
    },
    async saveTemplate() {
      this.saving = true
      try {
        const body = { ...this.formTemplate, forbidden_tools: this.forbiddenStr.split(',').map(s=>s.trim()).filter(Boolean) }
        if (this.editingTemplate) {
          await apiPut('/api/v1/plans/templates/' + this.editingTemplate.id, body)
        } else {
          await apiPost('/api/v1/plans/templates', body)
        }
        this.showNewTemplate = false
        this.editingTemplate = null
        this.formTemplate = { name: '', category: '', intent_pattern: '', description: '', enabled: true }
        this.forbiddenStr = ''
        await this.loadAll()
      } catch(e) { alert('Error: ' + (e.message||e)) }
      this.saving = false
    },
    async deleteTemplate(t) {
      if (!confirm('Delete template "' + t.name + '"?')) return
      try {
        await apiDelete('/api/v1/plans/templates/' + t.id)
        await this.loadAll()
      } catch(e) { alert('Error: ' + (e.message||e)) }
    },
    truncate(s, n) { return (s||'').length > n ? (s||'').slice(0, n) + '...' : (s||'') },
    formatTime(t) {
      if (!t) return '--'
      try { return new Date(t).toLocaleString() } catch { return t }
    },
  },
  mounted() { this.loadAll() },
}
</script>

<style scoped>
.plan-compiler-page { padding: var(--space-4); }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: var(--space-4); }
.page-title { font-size: 1.25rem; font-weight: 700; display: flex; align-items: center; gap: 8px; }
.page-subtitle { color: var(--text-secondary); font-size: 0.85rem; margin-top: 4px; }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: var(--space-3); margin-bottom: var(--space-4); }
.tab-bar { display: flex; gap: 4px; border-bottom: 1px solid var(--border); margin-bottom: var(--space-4); }
.tab-btn { padding: 8px 16px; border: none; background: none; cursor: pointer; font-size: 0.85rem; color: var(--text-secondary); border-bottom: 2px solid transparent; display: flex; align-items: center; gap: 6px; }
.tab-btn.active { color: var(--primary); border-bottom-color: var(--primary); font-weight: 600; }
.section { margin-bottom: var(--space-4); }
.rules-toolbar { display: flex; gap: var(--space-2); align-items: center; margin-bottom: var(--space-3); flex-wrap: wrap; }
.search-box { position: relative; }
.search-icon { position: absolute; left: 8px; top: 50%; transform: translateY(-50%); color: var(--text-tertiary); }
.search-input { padding: 6px 8px 6px 28px; border: 1px solid var(--border); border-radius: 6px; font-size: 0.85rem; background: var(--bg-secondary); color: var(--text-primary); width: 200px; }
.action-filters { display: flex; gap: 4px; }
.table-wrap { overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
.data-table th { padding: 8px 12px; text-align: left; font-weight: 600; border-bottom: 2px solid var(--border); white-space: nowrap; }
.data-table td { padding: 8px 12px; border-bottom: 1px solid var(--border-light, var(--border)); }
.td-mono { font-family: var(--font-mono, monospace); font-size: 0.8rem; }
.td-truncate { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.td-danger { color: var(--red, #e53e3e); }
.td-actions { display: flex; gap: 4px; }
.btn-icon { background: none; border: none; cursor: pointer; padding: 4px; border-radius: 4px; color: var(--text-secondary); }
.btn-icon:hover { background: var(--bg-hover, rgba(0,0,0,0.05)); }
.btn-icon-danger:hover { color: var(--red, #e53e3e); }
.cat-badge { padding: 2px 8px; border-radius: 10px; font-size: 0.75rem; font-weight: 600; }
.cat-query { background: #ebf5fb; color: #2980b9; }
.cat-email { background: #fef9e7; color: #f39c12; }
.cat-file { background: #eafaf1; color: #27ae60; }
.cat-code { background: #f4ecf7; color: #8e44ad; }
.cat-web { background: #fdedec; color: #e74c3c; }
.cat-admin { background: #ebedef; color: #2c3e50; }
.status-badge { padding: 2px 8px; border-radius: 10px; font-size: 0.75rem; font-weight: 600; }
.status-active { background: #eafaf1; color: #27ae60; }
.status-completed { background: #ebf5fb; color: #2980b9; }
.status-violated { background: #fdedec; color: #e74c3c; }
.status-expired { background: #ebedef; color: #7f8c8d; }
.badge-on { color: #27ae60; font-weight: 600; }
.badge-off { color: #95a5a6; }
.stats-detail { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: var(--space-4); }
.stat-block { background: var(--bg-secondary); border-radius: 8px; padding: var(--space-3); }
.section-title { font-size: 0.95rem; font-weight: 700; margin-bottom: var(--space-2); }
.stat-row { display: flex; justify-content: space-between; padding: 6px 0; border-bottom: 1px solid var(--border-light, var(--border)); font-size: 0.85rem; }
.stat-val { font-weight: 600; font-family: var(--font-mono, monospace); }
.text-green { color: #27ae60; }
.text-yellow { color: #f39c12; }
.text-red { color: #e74c3c; }
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: var(--bg-primary, #fff); border-radius: 12px; width: 500px; max-width: 90vw; max-height: 80vh; overflow-y: auto; }
.modal-header { padding: 16px 20px; border-bottom: 1px solid var(--border); display: flex; justify-content: space-between; align-items: center; }
.modal-header h3 { font-size: 1rem; font-weight: 700; }
.modal-body { padding: 20px; }
.modal-footer { padding: 12px 20px; border-top: 1px solid var(--border); display: flex; justify-content: flex-end; gap: 8px; }
.form-row { margin-bottom: var(--space-2); }
.form-row label { display: block; font-size: 0.8rem; font-weight: 600; margin-bottom: 4px; color: var(--text-secondary); }
.field-input { width: 100%; padding: 8px 12px; border: 1px solid var(--border); border-radius: 6px; font-size: 0.85rem; background: var(--bg-secondary); color: var(--text-primary); box-sizing: border-box; }
.btn-close { background: none; border: none; cursor: pointer; font-size: 1.1rem; color: var(--text-tertiary); }
</style>
