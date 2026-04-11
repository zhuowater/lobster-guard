<template>
  <div class="toolpolicy-page">
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="wrench" :size="20" /> 工具策略引擎</h1>
        <p class="page-subtitle">定义 Agent 可调用工具的策略规则 — 按工具名、参数模式精确控制</p>
      </div>
      <button class="btn btn-sm" @click="loadAll"><Icon name="refresh" :size="14" /> 刷新</button>
    </div>

    <!-- StatCards -->
    <div class="stats-grid" v-if="!initialLoading">
      <StatCard :iconSvg="svgClip" :value="stats.rule_count??'-'" label="规则数" color="blue" />
      <StatCard :iconSvg="svgSearch" :value="stats.total_evaluations??'-'" label="总评估" color="indigo" />
      <StatCard :iconSvg="svgBlock" :value="stats.blocked??'-'" label="阻断数" color="red" />
      <StatCard :iconSvg="svgAlert" :value="stats.warned??'-'" label="告警数" color="yellow" />
    </div>
    <div class="stats-grid" v-else><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>

    <!-- Tab -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{active:activeTab==='test'}" @click="activeTab='test'"><Icon name="test" :size="14"/> 实时测试</button>
      <button class="tab-btn" :class="{active:activeTab==='rules'}" @click="activeTab='rules'"><Icon name="file-text" :size="14"/> 规则管理 ({{ rules.length }})</button>
      <button class="tab-btn" :class="{active:activeTab==='semantic'}" @click="activeTab='semantic'">🧠 语义规则 ({{ semanticRules.length }})</button>
      <button class="tab-btn" :class="{active:activeTab==='context'}" @click="activeTab='context'">🔗 上下文策略 ({{ contextPolicies.length }})</button>
      <button class="tab-btn" :class="{active:activeTab==='events'}" @click="activeTab='events'">📜 事件日志 ({{ events.length }})</button>
    </div>

    <!-- Test Panel -->
    <div v-if="activeTab==='test'" class="section">
      <div class="test-panel">
        <h3 class="section-title">实时工具评估</h3>
        <div class="test-row">
          <div class="test-field"><label class="field-label">工具名 <span class="required">*</span></label><input v-model="testTool" class="field-input" placeholder="e.g. shell_exec"></div>
          <div class="test-field"><label class="field-label">样例库</label><select v-model="selectedSample" class="field-select" @change="applySample"><option value="">选择内置样例</option><option v-for="s in testSamples" :key="s.name" :value="s.name">{{ s.name }}</option></select></div>
        </div>
        <div class="test-field" style="margin-top:var(--space-2)"><label class="field-label">参数 JSON</label><textarea v-model="testParams" class="test-input" rows="3" placeholder='{"cmd": "rm -rf /"}'></textarea></div>
        <button class="btn btn-primary" @click="evaluateTool" :disabled="evaluating||!testTool.trim()" style="margin-top:var(--space-2)">
          <span v-if="evaluating" class="spinner"></span>{{ evaluating?'评估中...':'评估' }}
        </button>
      </div>
      <div v-if="evalResult" class="eval-result">
        <div class="result-header"><span>评估结果</span><button class="btn-close" @click="evalResult=null">✕</button></div>
        <div class="eval-decision" :class="'decision-'+(evalResult.decision||evalResult.action)">{{ (evalResult.decision||evalResult.action||'').toUpperCase() }}</div>
        <div v-if="evalResult.matched_rule||evalResult.rule_hit||evalResult.rule" class="eval-detail"><strong>命中规则：</strong>{{ evalResult.matched_rule||evalResult.rule_hit||evalResult.rule }}</div>
        <div v-if="evalResult.risk_level!=null" class="eval-detail"><strong>风险等级：</strong>{{ evalResult.risk_level }}</div>
        <div v-if="evalResult.semantic_class" class="eval-detail"><strong>语义分类：</strong>{{ evalResult.semantic_class }}</div>
        <div v-if="evalResult.context_signals?.length" class="eval-detail"><strong>上下文信号：</strong>{{ evalResult.context_signals.join(' / ') }}</div>
      </div>
    </div>

    <!-- Rules Tab -->
    <div v-if="activeTab==='rules'" class="section">
      <div class="rules-toolbar">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="ruleSearch" placeholder="搜索规则名/工具名..." class="search-input"/>
          <button v-if="ruleSearch" class="search-clear" @click="ruleSearch=''">✕</button>
        </div>
        <div class="action-filters">
          <button v-for="af in actionFilterOpts" :key="af.value" class="btn btn-sm" :class="actionFilter===af.value?'btn-active':'btn-ghost'" @click="actionFilter=af.value">{{ af.label }}</button>
        </div>
        <div class="toolbar-right">
          <span v-if="selectedIds.size>0" class="batch-info">已选 {{ selectedIds.size }} 项</span>
          <button v-if="selectedIds.size>0" class="btn btn-sm btn-success" @click="batchAction('enable')">批量启用</button>
          <button v-if="selectedIds.size>0" class="btn btn-sm btn-warning" @click="batchAction('disable')">批量禁用</button>
          <button v-if="selectedIds.size>0" class="btn btn-sm btn-danger" @click="batchAction('delete')">批量删除</button>
          <button class="btn btn-primary btn-sm" @click="openNewRule">➕ 新建规则</button>
        </div>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th class="th-check"><input type="checkbox" :checked="allSelected" @change="toggleAll"/></th><th>名称</th><th>工具模式</th><th>动作</th><th>优先级</th><th>原因</th><th>启用</th><th>操作</th></tr></thead>
          <tbody><tr v-for="r in filteredRules" :key="r.id||r.name" :class="{'row-expanded':expandedRuleId===(r.id||r.name)}">
            <td class="td-check"><input type="checkbox" :checked="selectedIds.has(r.id||r.name)" @change="toggleSelect(r.id||r.name)"/></td>
            <td class="td-mono">{{ r.name }}</td><td class="td-mono">{{ r.tool_pattern||r.pattern }}</td>
            <td><span class="action-badge" :class="'action-'+r.action">{{ r.action }}</span></td>
            <td class="td-mono">{{ r.priority??'-' }}</td><td>{{ truncate(r.reason,40) }}</td>
            <td><span :class="r.enabled!==false?'badge-on':'badge-off'">{{ r.enabled!==false?'启用':'禁用' }}</span></td>
            <td class="td-actions">
              <button class="btn-icon" @click="toggleExpandRule(r.id||r.name)" title="详情">📋</button>
              <button class="btn-icon" @click="editRule(r)" title="编辑">✏️</button>
              <button class="btn-icon" @click="confirmDeleteRule(r)" title="删除">🗑️</button>
            </td>
          </tr></tbody>
        </table>
        <EmptyState v-if="filteredRules.length===0" icon="📋" title="暂无规则" description="创建策略规则来控制工具调用"/>
        <!-- Expanded Rule Detail -->
        <div v-if="expandedRuleId" class="rule-detail-panel">
          <div v-for="r in filteredRules.filter(x=>(x.id||x.name)===expandedRuleId)" :key="r.id||r.name" class="rule-detail">
            <div class="detail-row"><span class="detail-label">名称</span><span class="detail-val">{{ r.name }}</span></div>
            <div class="detail-row"><span class="detail-label">工具模式</span><span class="detail-val mono">{{ r.tool_pattern||r.pattern }}</span></div>
            <div class="detail-row"><span class="detail-label">动作</span><span class="action-badge" :class="'action-'+r.action">{{ r.action }}</span></div>
            <div class="detail-row"><span class="detail-label">优先级</span><span class="detail-val">{{ r.priority??0 }}</span></div>
            <div class="detail-row"><span class="detail-label">原因</span><span class="detail-val">{{ r.reason||'—' }}</span></div>
            <div class="detail-row"><span class="detail-label">状态</span><span :class="r.enabled!==false?'badge-on':'badge-off'">{{ r.enabled!==false?'启用':'禁用' }}</span></div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="activeTab==='semantic'" class="section">
      <div class="rules-toolbar">
        <div class="toolbar-right">
          <button class="btn btn-sm" @click="exportSemanticRules">导出 JSON</button>
          <button class="btn btn-sm" @click="openImportSemanticRules">导入 JSON</button>
          <button class="btn btn-primary btn-sm" @click="openNewSemanticRule">➕ 新建语义规则</button>
        </div>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>名称</th><th>工具模式</th><th>参数键</th><th>匹配类型</th><th>语义类</th><th>动作</th><th>风险</th><th>优先级</th><th>操作</th></tr></thead>
          <tbody><tr v-for="r in semanticRules" :key="r.id||r.name"><td>{{ r.name }}</td><td class="td-mono">{{ r.tool_pattern }}</td><td class="td-mono">{{ (r.param_keys||[]).join(', ') }}</td><td>{{ r.match_type }}</td><td class="td-mono">{{ r.class }}</td><td><span class="action-badge" :class="'action-'+r.action">{{ r.action }}</span></td><td>{{ r.risk_level||'-' }}</td><td>{{ r.priority??'-' }}</td><td class="td-actions"><button class="btn-icon" @click="editSemanticRule(r)">✏️</button><button class="btn-icon" @click="deleteSemanticRule(r)">🗑️</button></td></tr></tbody>
        </table>
        <EmptyState v-if="semanticRules.length===0" icon="🧠" title="暂无语义规则" description="定义参数如何被解释为语义类"/>
      </div>
    </div>

    <div v-if="activeTab==='context'" class="section">
      <div class="rules-toolbar">
        <div class="toolbar-right">
          <button class="btn btn-primary btn-sm" @click="openNewContextPolicy">➕ 新建上下文策略</button>
        </div>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>名称</th><th>源语义类</th><th>目标语义类</th><th>目标工具</th><th>动作</th><th>风险</th><th>窗口</th><th>优先级</th><th>操作</th></tr></thead>
          <tbody><tr v-for="p in contextPolicies" :key="p.id||p.name"><td>{{ p.name }}</td><td class="td-mono">{{ (p.source_classes||[]).join(', ') }}</td><td class="td-mono">{{ (p.target_classes||[]).join(', ') }}</td><td class="td-mono">{{ (p.target_tools||[]).join(', ')||'-' }}</td><td><span class="action-badge" :class="'action-'+p.action">{{ p.action }}</span></td><td>{{ p.risk_level||'-' }}</td><td>{{ p.window_size||'-' }}</td><td>{{ p.priority??'-' }}</td><td class="td-actions"><button class="btn-icon" @click="editContextPolicy(p)">✏️</button><button class="btn-icon" @click="deleteContextPolicy(p)">🗑️</button></td></tr></tbody>
        </table>
        <EmptyState v-if="contextPolicies.length===0" icon="🔗" title="暂无上下文策略" description="定义多步行为链如何升级风险"/>
      </div>
    </div>

    <!-- Events Tab -->
    <div v-if="activeTab==='events'" class="section">
      <div class="rules-toolbar">
        <div class="search-box">
          <input v-model="eventSemanticFilter" placeholder="按语义类筛选，如 command:build_test" class="search-input"/>
        </div>
        <div class="search-box">
          <input v-model="eventContextFilter" placeholder="按上下文信号筛选，如 source:path:sensitive" class="search-input"/>
        </div>
        <div class="toolbar-right">
          <button class="btn btn-sm" @click="loadEvents">应用筛选</button>
          <button class="btn btn-sm btn-ghost" @click="eventSemanticFilter=''; eventContextFilter=''; loadEvents()">清空</button>
        </div>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>时间</th><th>工具名</th><th>决策</th><th>风险等级</th><th>语义分类</th><th>规则命中</th><th>上下文信号</th><th>TraceID</th></tr></thead>
          <tbody><tr v-for="(ev,idx) in events" :key="idx"><td class="td-mono">{{ formatTime(ev.timestamp||ev.time) }}</td><td class="td-mono">{{ ev.tool_name||ev.tool }}</td><td><span class="action-badge" :class="'action-'+(ev.decision||ev.action)">{{ ev.decision||ev.action }}</span></td><td class="td-mono">{{ ev.risk_level??'-' }}</td><td class="td-mono">{{ ev.semantic_class||'-' }}</td><td>{{ ev.matched_rule||ev.rule_hit||ev.rule||'-' }}</td><td>{{ Array.isArray(ev.context_signals)?ev.context_signals.join(' / '):'-' }}</td><td class="td-mono td-trace">{{ truncate(ev.trace_id,16) }}</td></tr></tbody>
        </table>
        <EmptyState v-if="events.length===0" icon="📜" title="暂无事件"/>
      </div>
    </div>

    <!-- Rule Dialog -->
    <div v-if="showDialog" class="dialog-overlay" @click.self="showDialog=false">
      <div class="dialog">
        <div class="dialog-header">{{ editingRule?'编辑规则':'新建规则' }}</div>
        <div class="dialog-body">
          <div class="config-field"><label class="field-label">名称 <span class="required">*</span></label><input v-model="form.name" class="field-input" placeholder="规则名称"><div v-if="formErrors.name" class="field-error">{{ formErrors.name }}</div></div>
          <div class="config-field"><label class="field-label">工具模式 <span class="required">*</span></label><input v-model="form.tool_pattern" class="field-input" placeholder="e.g. shell_*, file_write"><div v-if="formErrors.tool_pattern" class="field-error">{{ formErrors.tool_pattern }}</div></div>
          <div class="config-field"><label class="field-label">动作</label><select v-model="form.action" class="field-select"><option value="block">block</option><option value="warn">warn</option><option value="allow">allow</option></select></div>
          <div class="config-field"><label class="field-label">优先级</label><input v-model.number="form.priority" class="field-input" type="number" placeholder="0"></div>
          <div class="config-field"><label class="field-label">原因</label><input v-model="form.reason" class="field-input" placeholder="规则原因说明"></div>
        </div>
        <div class="dialog-footer">
          <button class="btn btn-sm" @click="showDialog=false">取消</button>
          <button class="btn btn-primary btn-sm" @click="saveRule" :disabled="saving">{{ saving?'保存中...':'保存' }}</button>
        </div>
      </div>
    </div>

    <div v-if="showSemanticDialog" class="dialog-overlay" @click.self="showSemanticDialog=false">
      <div class="dialog">
        <div class="dialog-header">{{ editingSemanticRule?'编辑语义规则':'新建语义规则' }}</div>
        <div class="dialog-body">
          <div class="config-field"><label class="field-label">名称</label><input v-model="semanticForm.name" class="field-input"></div>
          <div class="config-field"><label class="field-label">工具模式</label><input v-model="semanticForm.tool_pattern" class="field-input" placeholder="*command*"></div>
          <div class="config-field"><label class="field-label">参数键（逗号分隔）</label><input v-model="semanticForm.param_keys" class="field-input" placeholder="command,cmd"></div>
          <div class="config-field"><label class="field-label">匹配类型</label><select v-model="semanticForm.match_type" class="field-select"><option value="regex">regex</option><option value="exists">exists</option><option value="always">always</option></select></div>
          <div class="config-field"><label class="field-label">匹配模式</label><textarea v-model="semanticForm.pattern" class="test-input" rows="3"></textarea></div>
          <div class="config-field"><label class="field-label">语义类</label><input v-model="semanticForm.class" class="field-input" placeholder="command:build_test"></div>
          <div class="config-field"><label class="field-label">动作</label><select v-model="semanticForm.action" class="field-select"><option value="allow">allow</option><option value="warn">warn</option><option value="block">block</option></select></div>
          <div class="config-field"><label class="field-label">风险等级</label><input v-model="semanticForm.risk_level" class="field-input" placeholder="low/medium/high"></div>
          <div class="config-field"><label class="field-label">优先级</label><input v-model.number="semanticForm.priority" class="field-input" type="number"></div>
        </div>
        <div class="dialog-footer"><button class="btn btn-sm" @click="showSemanticDialog=false">取消</button><button class="btn btn-primary btn-sm" @click="saveSemanticRule">保存</button></div>
      </div>
    </div>

    <div v-if="showContextDialog" class="dialog-overlay" @click.self="showContextDialog=false">
      <div class="dialog">
        <div class="dialog-header">{{ editingContextPolicy?'编辑上下文策略':'新建上下文策略' }}</div>
        <div class="dialog-body">
          <div class="config-field"><label class="field-label">名称</label><input v-model="contextForm.name" class="field-input"></div>
          <div class="config-field"><label class="field-label">源语义类（逗号分隔）</label><input v-model="contextForm.source_classes" class="field-input" placeholder="path:sensitive"></div>
          <div class="config-field"><label class="field-label">目标语义类（逗号分隔）</label><input v-model="contextForm.target_classes" class="field-input" placeholder="url:external"></div>
          <div class="config-field"><label class="field-label">目标工具（逗号分隔，可选）</label><input v-model="contextForm.target_tools" class="field-input" placeholder="http_request,send_email"></div>
          <div class="config-field"><label class="field-label">动作</label><select v-model="contextForm.action" class="field-select"><option value="warn">warn</option><option value="block">block</option><option value="allow">allow</option></select></div>
          <div class="config-field"><label class="field-label">风险等级</label><input v-model="contextForm.risk_level" class="field-input" placeholder="medium/high"></div>
          <div class="config-field"><label class="field-label">窗口大小</label><input v-model.number="contextForm.window_size" class="field-input" type="number"></div>
          <div class="config-field"><label class="field-label">优先级</label><input v-model.number="contextForm.priority" class="field-input" type="number"></div>
        </div>
        <div class="dialog-footer"><button class="btn btn-sm" @click="showContextDialog=false">取消</button><button class="btn btn-primary btn-sm" @click="saveContextPolicy">保存</button></div>
      </div>
    </div>

    <ConfirmModal :visible="confirmModal.show" :title="confirmModal.title" :message="confirmModal.message" :type="confirmModal.type" @confirm="confirmModal.onConfirm" @cancel="confirmModal.show=false"/>
    <div v-if="error" class="error-banner">⚠️ {{ error }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import Skeleton from '../components/Skeleton.vue'
import EmptyState from '../components/EmptyState.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { api, apiPost, apiPut, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'

const activeTab = ref('test')
const stats = ref({})
const rules = ref([])
const semanticRules = ref([])
const contextPolicies = ref([])
const events = ref([])
const error = ref('')
const testTool = ref('')
const testParams = ref('')
const evaluating = ref(false)
const evalResult = ref(null)
const showDialog = ref(false)
const showSemanticDialog = ref(false)
const showContextDialog = ref(false)
const editingRule = ref(null)
const editingSemanticRule = ref(null)
const editingContextPolicy = ref(null)
const saving = ref(false)
const initialLoading = ref(true)
const ruleSearch = ref('')
const actionFilter = ref('all')
const selectedIds = ref(new Set())
const expandedRuleId = ref(null)
const eventSemanticFilter = ref('')
const eventContextFilter = ref('')
const form = reactive({ name:'', tool_pattern:'', action:'block', priority:0, reason:'' })
const semanticForm = reactive({ name:'', tool_pattern:'*', param_keys:'', match_type:'regex', pattern:'', class:'', action:'allow', risk_level:'low', priority:100 })
const contextForm = reactive({ name:'', source_classes:'', target_classes:'', target_tools:'', action:'block', risk_level:'high', window_size:12, priority:100 })
const formErrors = reactive({ name:'', tool_pattern:'' })
const confirmModal = reactive({ show:false, title:'', message:'', type:'danger', onConfirm:()=>{} })

const svgClip = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><rect x="8" y="2" width="8" height="4" rx="1" ry="1"/></svg>'
const svgSearch = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>'
const svgBlock = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>'
const svgAlert = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'

const actionFilterOpts = [{label:'全部',value:'all'},{label:'Block',value:'block'},{label:'Warn',value:'warn'},{label:'Allow',value:'allow'}]

const filteredRules = computed(()=>{
  let list = [...rules.value]
  if (ruleSearch.value.trim()) { const q=ruleSearch.value.trim().toLowerCase(); list=list.filter(r=>(r.name||'').toLowerCase().includes(q)||(r.tool_pattern||r.pattern||'').toLowerCase().includes(q)) }
  if (actionFilter.value!=='all') list=list.filter(r=>r.action===actionFilter.value)
  return list
})
const allSelected = computed(()=>filteredRules.value.length>0&&filteredRules.value.every(r=>selectedIds.value.has(r.id||r.name)))

function toggleSelect(id) { selectedIds.value.has(id)?selectedIds.value.delete(id):selectedIds.value.add(id); selectedIds.value=new Set(selectedIds.value) }
function toggleAll() { if(allSelected.value){selectedIds.value=new Set()}else{selectedIds.value=new Set(filteredRules.value.map(r=>r.id||r.name))} }
function toggleExpandRule(id) { expandedRuleId.value=expandedRuleId.value===id?null:id }

async function loadStats() {
  try {
    const d=await api('/api/v1/tools/rules'); const r=d.rules||d||[]
    rules.value=r
    // Load stats from dedicated stats endpoint
    try {
      const s=await api('/api/v1/tools/stats')
      const bd=s.by_decision||{}
      stats.value={rule_count:r.length,total_evaluations:s.total_events||0,blocked:bd.block||s.blocked_24h||0,warned:bd.warn||s.warned_24h||0}
    } catch(e2) {
      stats.value={rule_count:r.length,total_evaluations:'-',blocked:'-',warned:'-'}
    }
  } catch(e){ error.value='加载规则失败: '+e.message }
}
async function loadSemanticRules() { try{const d=await api('/api/v1/tools/semantic-rules');semanticRules.value=d.rules||[]}catch(e){error.value='加载语义规则失败: '+e.message} }
async function loadContextPolicies() { try{const d=await api('/api/v1/tools/context-policies');contextPolicies.value=d.policies||[]}catch(e){error.value='加载上下文策略失败: '+e.message} }
async function loadEvents() { try{const qs=new URLSearchParams({limit:'50'}); if(eventSemanticFilter.value.trim()) qs.set('semantic_class', eventSemanticFilter.value.trim()); if(eventContextFilter.value.trim()) qs.set('context_signal', eventContextFilter.value.trim()); const d=await api('/api/v1/tools/events?'+qs.toString());events.value=d.events||d||[]}catch(e){error.value='加载事件失败: '+e.message} }

async function evaluateTool() {
  if(!testTool.value.trim()){showToast('请输入工具名','warning');return}
  evaluating.value=true; evalResult.value=null
  try {
    let params={}; if(testParams.value.trim()){try{params=JSON.parse(testParams.value)}catch{params={raw:testParams.value}}}
    evalResult.value=await apiPost('/api/v1/tools/evaluate',{tool_name:testTool.value,parameters:params})
    showToast('评估完成','success')
  } catch(e){showToast('评估失败: '+e.message,'error')}finally{evaluating.value=false}
}

function openNewRule() { editingRule.value=null; Object.assign(form,{name:'',tool_pattern:'',action:'block',priority:0,reason:''}); Object.assign(formErrors,{name:'',tool_pattern:''}); showDialog.value=true }
function editRule(r) { editingRule.value=r; Object.assign(form,{name:r.name,tool_pattern:r.tool_pattern||r.pattern,action:r.action,priority:r.priority||0,reason:r.reason||''}); Object.assign(formErrors,{name:'',tool_pattern:''}); showDialog.value=true }

function validateForm() {
  let ok=true; formErrors.name=''; formErrors.tool_pattern=''
  if(!form.name.trim()){formErrors.name='规则名称不能为空';ok=false}
  if(!form.tool_pattern.trim()){formErrors.tool_pattern='工具模式不能为空';ok=false}
  return ok
}

async function saveRule() {
  if(!validateForm()) return
  saving.value=true
  try {
    const body={name:form.name,tool_pattern:form.tool_pattern,action:form.action,priority:form.priority,reason:form.reason}
    if(editingRule.value){await apiPut('/api/v1/tools/rules/'+(editingRule.value.id||editingRule.value.name),body)}
    else{await apiPost('/api/v1/tools/rules',body)}
    showDialog.value=false; showToast(editingRule.value?'规则已更新':'规则已创建','success'); loadStats()
  } catch(e){showToast('保存失败: '+e.message,'error')}finally{saving.value=false}
}

function confirmDeleteRule(r) {
  confirmModal.title='删除规则'; confirmModal.message='确定删除规则 "'+r.name+'" 吗？此操作不可恢复。'; confirmModal.type='danger'
  confirmModal.onConfirm=async()=>{confirmModal.show=false;try{await apiDelete('/api/v1/tools/rules/'+(r.id||r.name));showToast('已删除','success');loadStats()}catch(e){showToast('删除失败: '+e.message,'error')}}
  confirmModal.show=true
}

function csvToList(v){return (v||'').split(',').map(s=>s.trim()).filter(Boolean)}
function openNewSemanticRule(){editingSemanticRule.value=null;Object.assign(semanticForm,{name:'',tool_pattern:'*',param_keys:'',match_type:'regex',pattern:'',class:'',action:'allow',risk_level:'low',priority:100});showSemanticDialog.value=true}
function editSemanticRule(r){editingSemanticRule.value=r;Object.assign(semanticForm,{name:r.name||'',tool_pattern:r.tool_pattern||'*',param_keys:(r.param_keys||[]).join(', '),match_type:r.match_type||'regex',pattern:r.pattern||'',class:r.class||'',action:r.action||'allow',risk_level:r.risk_level||'low',priority:r.priority??100});showSemanticDialog.value=true}
async function saveSemanticRule(){
  const body={name:semanticForm.name,tool_pattern:semanticForm.tool_pattern,param_keys:csvToList(semanticForm.param_keys),match_type:semanticForm.match_type,pattern:semanticForm.pattern,class:semanticForm.class,action:semanticForm.action,risk_level:semanticForm.risk_level,priority:semanticForm.priority,enabled:true}
  try{
    if(editingSemanticRule.value) await apiPut('/api/v1/tools/semantic-rules/'+(editingSemanticRule.value.id||editingSemanticRule.value.name),body)
    else await apiPost('/api/v1/tools/semantic-rules',body)
    showSemanticDialog.value=false;showToast('语义规则已保存','success');loadSemanticRules()
  }catch(e){showToast('保存语义规则失败: '+e.message,'error')}
}
function deleteSemanticRule(r){confirmModal.title='删除语义规则';confirmModal.message='确定删除语义规则 "'+r.name+'" 吗？';confirmModal.type='danger';confirmModal.onConfirm=async()=>{confirmModal.show=false;try{await apiDelete('/api/v1/tools/semantic-rules/'+(r.id||r.name));showToast('已删除语义规则','success');loadSemanticRules()}catch(e){showToast('删除失败: '+e.message,'error')}};confirmModal.show=true}

function openNewContextPolicy(){editingContextPolicy.value=null;Object.assign(contextForm,{name:'',source_classes:'',target_classes:'',target_tools:'',action:'block',risk_level:'high',window_size:12,priority:100});showContextDialog.value=true}
function editContextPolicy(p){editingContextPolicy.value=p;Object.assign(contextForm,{name:p.name||'',source_classes:(p.source_classes||[]).join(', '),target_classes:(p.target_classes||[]).join(', '),target_tools:(p.target_tools||[]).join(', '),action:p.action||'block',risk_level:p.risk_level||'high',window_size:p.window_size??12,priority:p.priority??100});showContextDialog.value=true}
async function saveContextPolicy(){
  const body={name:contextForm.name,source_classes:csvToList(contextForm.source_classes),target_classes:csvToList(contextForm.target_classes),target_tools:csvToList(contextForm.target_tools),action:contextForm.action,risk_level:contextForm.risk_level,window_size:contextForm.window_size,priority:contextForm.priority,enabled:true}
  try{
    if(editingContextPolicy.value) await apiPut('/api/v1/tools/context-policies/'+(editingContextPolicy.value.id||editingContextPolicy.value.name),body)
    else await apiPost('/api/v1/tools/context-policies',body)
    showContextDialog.value=false;showToast('上下文策略已保存','success');loadContextPolicies()
  }catch(e){showToast('保存上下文策略失败: '+e.message,'error')}
}
function deleteContextPolicy(p){confirmModal.title='删除上下文策略';confirmModal.message='确定删除上下文策略 "'+p.name+'" 吗？';confirmModal.type='danger';confirmModal.onConfirm=async()=>{confirmModal.show=false;try{await apiDelete('/api/v1/tools/context-policies/'+(p.id||p.name));showToast('已删除上下文策略','success');loadContextPolicies()}catch(e){showToast('删除失败: '+e.message,'error')}};confirmModal.show=true}

async function batchAction(action) {
  const ids=[...selectedIds.value]; if(!ids.length) return
  if(action==='delete'){
    confirmModal.title='批量删除'; confirmModal.message='确定删除选中的 '+ids.length+' 条规则吗？'; confirmModal.type='danger'
    confirmModal.onConfirm=async()=>{confirmModal.show=false;let ok=0;for(const id of ids){try{await apiDelete('/api/v1/tools/rules/'+id);ok++}catch{}}showToast('已删除 '+ok+' 条规则','success');selectedIds.value=new Set();loadStats()}
    confirmModal.show=true
  } else {
    const enabled=action==='enable'
    let ok=0;for(const id of ids){try{await apiPut('/api/v1/tools/rules/'+id,{enabled});ok++}catch{}}
    showToast((enabled?'已启用':'已禁用')+' '+ok+' 条规则','success');selectedIds.value=new Set();loadStats()
  }
}

function loadAll() { error.value=''; Promise.all([loadStats(),loadSemanticRules(),loadContextPolicies(),loadEvents()]).finally(()=>{initialLoading.value=false}) }
function truncate(s,max) { return s&&s.length>max?s.slice(0,max)+'…':s||'-' }
function formatTime(ts) { if(!ts) return '-'; try{const d=new Date(ts);return d.toLocaleDateString('zh-CN',{month:'2-digit',day:'2-digit'})+' '+d.toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit',second:'2-digit'})}catch{return ts} }
onMounted(loadAll)
</script>

<style scoped>
.toolpolicy-page{padding:var(--space-4);max-width:1200px}
.page-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:var(--space-4);flex-wrap:wrap;gap:var(--space-3)}
.page-title{font-size:var(--text-xl);font-weight:800;color:var(--text-primary);margin:0}
.page-subtitle{font-size:var(--text-sm);color:var(--text-tertiary);margin-top:2px}
.stats-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:var(--space-3);margin-bottom:var(--space-4)}
.tab-bar{display:flex;gap:var(--space-2);margin-bottom:var(--space-3);border-bottom:1px solid var(--border-subtle);padding-bottom:var(--space-2)}
.tab-btn{background:none;border:none;color:var(--text-secondary);font-size:var(--text-sm);padding:var(--space-2) var(--space-3);cursor:pointer;border-radius:var(--radius-md) var(--radius-md) 0 0;transition:all .2s;display:inline-flex;align-items:center;gap:4px}
.tab-btn:hover{color:var(--text-primary);background:var(--bg-elevated)}
.tab-btn.active{color:var(--color-primary);border-bottom:2px solid var(--color-primary);font-weight:600}
.section{margin-bottom:var(--space-4)}.section-title{font-size:var(--text-sm);font-weight:700;color:var(--text-primary);margin-bottom:var(--space-3)}
.required{color:#EF4444}
/* Test Panel */
.test-panel{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:var(--space-4);margin-bottom:var(--space-3)}
.test-row{display:flex;gap:var(--space-3);flex-wrap:wrap}.test-field{flex:1;min-width:200px}
.test-input{width:100%;background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);padding:var(--space-3);font-size:var(--text-sm);resize:vertical;font-family:var(--font-mono)}
.test-input:focus{outline:none;border-color:var(--color-primary)}
.eval-result{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:var(--space-4)}
.result-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:var(--space-3);font-weight:700;color:var(--text-primary)}
.btn-close{background:none;border:none;color:var(--text-tertiary);cursor:pointer;font-size:16px}.btn-close:hover{color:var(--text-primary)}
.eval-decision{font-size:2rem;font-weight:800;text-align:center;padding:var(--space-3);border-radius:var(--radius-md);margin-bottom:var(--space-2)}
.decision-block{color:#EF4444;background:rgba(239,68,68,.1)}.decision-warn{color:#F59E0B;background:rgba(245,158,11,.1)}.decision-allow{color:#10B981;background:rgba(16,185,129,.1)}
.eval-detail{font-size:var(--text-sm);color:var(--text-secondary);margin-bottom:var(--space-1)}
/* Rules toolbar */
.rules-toolbar{display:flex;align-items:center;gap:var(--space-3);margin-bottom:var(--space-3);flex-wrap:wrap}
.search-box{position:relative;display:flex;align-items:center}
.search-icon{position:absolute;left:8px;color:var(--text-tertiary);pointer-events:none}
.search-input{padding:6px 28px;background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-sm);width:200px;transition:all .15s}
.search-input:focus{outline:none;border-color:var(--color-primary);width:260px}
.search-clear{position:absolute;right:6px;background:none;border:none;color:var(--text-tertiary);cursor:pointer;font-size:12px}.search-clear:hover{color:var(--text-primary)}
.action-filters{display:flex;gap:var(--space-1)}
.toolbar-right{display:flex;align-items:center;gap:var(--space-2);margin-left:auto;flex-wrap:wrap}
.batch-info{font-size:var(--text-xs);color:var(--color-primary);font-weight:600}
/* Action badges */
.action-badge{display:inline-block;padding:2px 8px;border-radius:4px;font-size:10px;font-weight:600}
.action-block{background:rgba(239,68,68,.15);color:#FCA5A5}.action-warn{background:rgba(245,158,11,.15);color:#FCD34D}.action-allow{background:rgba(16,185,129,.15);color:#6EE7B7}
.badge-on{color:#10B981;font-weight:600;font-size:11px}.badge-off{color:var(--text-tertiary);font-size:11px}
/* Table */
.table-wrap{overflow-x:auto}
.data-table{width:100%;border-collapse:collapse;font-size:var(--text-xs)}
.data-table th{text-align:left;padding:8px 10px;background:var(--bg-elevated);color:var(--text-tertiary);font-weight:600;font-size:10px;text-transform:uppercase;letter-spacing:.05em;border-bottom:2px solid var(--border-subtle);white-space:nowrap}
.data-table td{padding:6px 10px;border-bottom:1px solid var(--border-subtle);color:var(--text-secondary)}
.data-table tr:hover{background:var(--bg-elevated)}
.td-mono{font-family:var(--font-mono);font-size:11px}
.td-trace{max-width:150px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.td-actions{display:flex;gap:4px}.td-check{width:32px}.th-check{width:32px}
.btn-icon{background:none;border:none;cursor:pointer;font-size:14px;padding:2px 4px;border-radius:4px;transition:background .2s}.btn-icon:hover{background:var(--bg-elevated)}
.row-expanded{background:rgba(99,102,241,.05)}
/* Rule detail */
.rule-detail-panel{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-md);padding:var(--space-3);margin-top:var(--space-2);animation:slideDown .2s ease}
@keyframes slideDown{from{opacity:0;max-height:0}to{opacity:1;max-height:300px}}
.detail-row{display:flex;gap:var(--space-3);padding:var(--space-1) 0;font-size:var(--text-xs)}
.detail-label{color:var(--text-tertiary);min-width:80px}.detail-val{color:var(--text-primary)}.mono{font-family:var(--font-mono)}
/* Dialog */
.dialog-overlay{position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.5);display:flex;align-items:center;justify-content:center;z-index:1000}
.dialog{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:0;width:420px;max-width:90vw;box-shadow:var(--shadow-lg)}
.dialog-header{padding:var(--space-4);border-bottom:1px solid var(--border-subtle);font-weight:700;color:var(--text-primary);font-size:var(--text-base)}
.dialog-body{padding:var(--space-4);display:flex;flex-direction:column;gap:var(--space-3)}
.dialog-footer{padding:var(--space-3) var(--space-4);border-top:1px solid var(--border-subtle);display:flex;justify-content:flex-end;gap:var(--space-2)}
.config-field{display:flex;flex-direction:column;gap:4px}
.field-label{font-size:10px;font-weight:600;color:var(--text-tertiary);text-transform:uppercase;letter-spacing:.05em}
.field-input{background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);padding:6px 10px;font-size:var(--text-sm)}
.field-input:focus{outline:none;border-color:var(--color-primary)}
.field-select{background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);padding:6px 10px;font-size:var(--text-sm)}
.field-error{font-size:10px;color:#EF4444;margin-top:2px}
/* Buttons */
.btn{display:inline-flex;align-items:center;gap:6px;padding:8px 16px;border-radius:var(--radius-md);font-weight:600;font-size:var(--text-sm);cursor:pointer;border:1px solid var(--border-subtle);background:var(--bg-elevated);color:var(--text-secondary);transition:all .2s}
.btn:hover{background:var(--bg-surface);color:var(--text-primary)}
.btn-primary{background:var(--color-primary);color:#fff;border-color:var(--color-primary)}.btn-primary:hover:not(:disabled){filter:brightness(1.15)}.btn-primary:disabled{opacity:.5;cursor:not-allowed}
.btn-success{background:rgba(34,197,94,.15);color:#4ADE80;border-color:rgba(34,197,94,.3)}
.btn-warning{background:rgba(251,191,36,.15);color:#FBBF24;border-color:rgba(251,191,36,.3)}
.btn-danger{background:rgba(239,68,68,.1);color:#F87171;border-color:rgba(239,68,68,.2)}
.btn-ghost{background:transparent;border-color:transparent;color:var(--text-secondary)}.btn-ghost:hover{background:var(--bg-elevated);color:var(--text-primary)}
.btn-active{background:rgba(99,102,241,.15);color:var(--color-primary);border-color:rgba(99,102,241,.3);font-weight:600}
.btn-sm{padding:6px 12px;font-size:var(--text-xs)}
.spinner{display:inline-block;width:14px;height:14px;border:2px solid rgba(255,255,255,.3);border-top-color:#fff;border-radius:50%;animation:spin .6s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}
.error-banner{margin-top:var(--space-3);padding:var(--space-3);background:rgba(239,68,68,.1);border:1px solid rgba(239,68,68,.3);border-radius:var(--radius-md);color:#FCA5A5;font-size:var(--text-sm)}
@media(max-width:768px){.stats-grid{grid-template-columns:repeat(2,1fr)}.rules-toolbar{flex-direction:column;align-items:stretch}.toolbar-right{margin-left:0}}
</style>
