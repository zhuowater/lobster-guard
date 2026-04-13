<template>
  <div>
    <div v-if="loaded && alertCount > 0" class="alert-panel" style="margin-bottom:20px">
      <div class="alert-header"><span class="alert-icon">⚠️</span><span class="alert-title">检测到 {{ alertCount }} 个高危工具调用</span></div>
      <div class="alert-items">
        <div v-for="a in alertPreview" :key="a.id" class="alert-item">
          <code>{{ a.tool_name }}</code><span class="alert-time">{{ fmtTime(a.timestamp) }}</span><span class="alert-reason">{{ a.flag_reason || a.risk_level }}</span>
        </div>
      </div>
      <div class="alert-footer"><a class="alert-link" @click="scrollToHighRisk">查看全部 →</a><a class="alert-config-link" @click="goToSettings('security')">前往安全策略配置 →</a></div>
    </div>

    <div v-if="loaded && canaryLeakCount > 0" class="alert-panel canary-alert" style="margin-bottom:20px">
      <div class="alert-header"><span class="alert-icon">🐤</span><span class="alert-title" style="color:#D97706">检测到 {{ canaryLeakCount }} 次 Prompt 泄露</span></div>
      <div style="font-size:var(--text-sm);color:var(--text-secondary);padding:4px 0">Agent 响应中包含了 Canary Token，表明 System Prompt 内容被泄露（OWASP LLM06）</div>
      <div class="alert-footer" style="justify-content:flex-end"><a class="alert-config-link" @click="goToSettings('canary')">配置 Canary Token →</a></div>
    </div>

    <div v-if="loaded && budgetViolationCount > 0" class="alert-panel budget-alert" style="margin-bottom:20px">
      <div class="alert-header"><span class="alert-icon"><Icon name="bar-chart" :size="16" /></span><span class="alert-title" style="color:#EA580C">检测到 {{ budgetViolationCount }} 次预算超限</span></div>
      <div style="font-size:var(--text-sm);color:var(--text-secondary);padding:4px 0">Agent 工具调用或 Token 使用量超出预算限制（OWASP LLM08 Excessive Agency）</div>
      <div class="alert-footer" style="justify-content:flex-end"><a class="alert-config-link" @click="goToSettings('budget')">配置 Response Budget →</a></div>
    </div>

    <!-- 统计卡片 -->
    <div class="ov-cards" v-if="loaded">
      <StatCard :iconSvg="svgRobot" :value="stats.total" label="总工具调用" color="blue" />
      <StatCard :iconSvg="svgAlert" :value="stats.high_risk_count" label="高危调用" color="red" />
      <StatCard :iconSvg="svgFlag" :value="stats.flagged_count" label="已标记" color="yellow" />
      <StatCard :iconSvg="svgPercent" :value="stats.high_risk_rate" label="24h高危率" color="purple" />
      <StatCard :iconSvg="svgGlobe" :value="stats.source_category_count" label="来源分类数" color="green" />
      <StatCard v-if="canaryLeakCount > 0" :iconSvg="svgCanary" :value="canaryLeakCount" label="Prompt 泄露" color="yellow" />
      <StatCard v-if="budgetViolationCount > 0" :iconSvg="svgBudget" :value="budgetViolationCount" label="预算超限" color="red" />
    </div>
    <div class="ov-cards" v-else><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /></div>

    <!-- 行为规则管理 -->
    <div v-if="showRules" class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon">📋</span><span class="card-title">行为规则管理</span>
        <button class="btn btn-primary btn-sm" style="margin-left:auto" @click="addRule">+ 新建规则</button>
      </div>
      <div v-if="rules.length" class="table-wrap">
        <table>
          <thead><tr><th>规则名称</th><th>条件</th><th>动作</th><th>状态</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="(rule,i) in rules" :key="i" :class="{'row-disabled': !rule.enabled}">
              <td><b>{{ rule.name }}</b></td>
              <td class="td-mono">{{ rule.condition }}</td>
              <td><span class="action-badge" :class="'action-'+rule.action">{{ rule.action }}</span></td>
              <td><button class="toggle-btn" :class="{active:rule.enabled}" @click="rule.enabled=!rule.enabled">{{ rule.enabled ? 'ON' : 'OFF' }}</button></td>
              <td><button class="btn-sm btn-ghost" @click="editRule(i)">编辑</button><button class="btn-sm btn-ghost" style="color:#EF4444" @click="deleteRule(i)">删除</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState v-else :iconSvg="svgShieldCheck" title="暂无行为规则" description="点击「新建规则」创建行为检测规则" />
    </div>

    <!-- 搜索过滤 -->
    <div class="filter-bar card" style="margin-bottom:16px">
      <div class="filter-bar-inner">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="searchText" class="search-input" placeholder="搜索工具名称..." />
          <button v-if="searchText" class="search-clear" @click="searchText=''">&times;</button>
        </div>
        <div class="filter-selects">
          <select v-model="filterRisk" class="filter-select"><option value="">全部风险</option><option value="critical">极危</option><option value="high">高危</option><option value="medium">中危</option><option value="low">低危</option></select>
          <button class="btn btn-ghost btn-sm" @click="showRules=!showRules">{{ showRules ? '隐藏规则' : '📋 规则管理' }}</button>
        </div>
      </div>
    </div>

    <!-- 趋势图 -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span>
        <span class="card-title">工具调用趋势</span>
      </div>
      <Skeleton v-if="!loaded" type="chart" />
      <EmptyState v-else-if="!timelineData.length" :iconSvg="svgTrend" title="暂无趋势数据" description="Agent 运行后将自动收集工具调用数据" />
      <TrendChart v-else :data="trendChartData" :lines="trendLines" :xLabels="trendXLabels" :height="170"
        :timeRanges="[{label:'24h',value:'24h'},{label:'7d',value:'7d'}]" :currentRange="trendRange" @rangeChange="onTrendRangeChange" />
    </div>

    <!-- Top10 + Pie -->
    <div class="ov-row">
      <div class="card">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 9l6 6 6-6"/></svg></span><span class="card-title">工具调用 TOP10</span></div>
        <Skeleton v-if="!loaded" type="text" />
        <EmptyState v-else-if="!topTools.length" :iconSvg="svgBar" title="暂无工具调用数据" description="Agent 运行后将自动收集调用统计" />
        <div v-else>
          <TransitionGroup name="list-anim" tag="div">
            <div class="hbar-row" v-for="(t, i) in topTools" :key="t.name">
              <span class="hbar-rank">#{{ i + 1 }}</span>
              <span class="hbar-name" :title="t.name">{{ t.name }}</span>
              <div class="hbar-track"><div class="hbar-fill hbar-fill-anim" :style="{ '--target-w': Math.max(5, t.pct) + '%', background: getRiskColor(classifyRisk(t.name)) }">{{ t.count }}</div></div>
              <span class="risk-badge" :class="'risk-' + classifyRisk(t.name)" style="margin-left:8px;font-size:10px;">{{ classifyRisk(t.name) }}</span>
            </div>
          </TransitionGroup>
        </div>
      </div>
      <div class="card">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21.21 15.89A10 10 0 1 1 8 2.83"/><path d="M22 12A10 10 0 0 0 12 2v10z"/></svg></span><span class="card-title">风险等级分布</span></div>
        <Skeleton v-if="!loaded" type="chart" />
        <PieChart v-else :data="pieData" :size="180" />
      </div>
    </div>

    <div class="card" style="margin-bottom:20px">
      <div class="card-header"><span class="card-icon"><Icon name="globe" :size="16" /></span><span class="card-title">来源分类分布</span></div>
      <EmptyState v-if="loaded && !sourceCategoryRows.length" :iconSvg="svgGlobeLarge" title="暂无来源分类数据" description="当 tool call 被来源分类后，这里会显示 public_web / internal_api / external_api 等分布" />
      <div v-else class="source-category-list">
        <div v-for="row in sourceCategoryRows" :key="row.category" class="source-category-row source-category-row-clickable" @click="goToSourceSessions(row.category)">
          <span class="source-badge">{{ row.category }}</span>
          <div class="hbar-track source-track"><div class="hbar-fill hbar-fill-anim source-fill" :style="{ '--target-w': Math.max(6, row.pct) + '%' }">{{ row.count }}</div></div>
          <span class="source-pct">{{ row.pct.toFixed(1) }}%</span>
        </div>
      </div>
    </div>

    <!-- 高危调用表 -->
    <div class="card" ref="highRiskRef">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg></span>
        <span class="card-title">最近高危调用</span>
        <button v-if="highRiskRecords.length" class="btn btn-ghost btn-sm" style="margin-left:auto" @click="exportHighRisk">📤 导出</button>
      </div>
      <Skeleton v-if="!loaded" type="table" />
      <EmptyState v-else-if="!filteredHighRisk.length" :iconSvg="svgShieldCheck" title="暂无高危调用" description="系统运行正常，未检测到高危工具调用" />
      <div v-else class="table-wrap">
        <table>
          <thead><tr><th style="width:30px"></th><th>时间</th><th>工具名</th><th>来源分类</th><th>风险等级</th><th>参数摘要</th><th>标记原因</th><th>操作</th></tr></thead>
          <tbody>
            <template v-for="rec in filteredHighRisk" :key="rec.id">
              <tr :class="{'row-critical': rec.risk_level==='critical','row-high': rec.risk_level==='high','row-expanded': expandedIds.has(rec.id)}" @click="toggleExpand(rec.id)" style="cursor:pointer">
                <td class="expand-toggle"><svg :class="{'rotated': expandedIds.has(rec.id)}" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg></td>
                <td>{{ fmtTime(rec.timestamp) }}</td>
                <td><code>{{ rec.tool_name }}</code></td>
                <td>
                  <span v-if="rec.source_category" class="source-badge source-badge-clickable" @click.stop="goToSourceAudit(rec.source_category)">{{ rec.source_category }}</span>
                  <span v-else class="text-muted">--</span>
                </td>
                <td><span class="risk-badge" :class="'risk-'+rec.risk_level">{{ rec.risk_level }}</span></td>
                <td class="td-preview" :title="rec.tool_input_preview">{{ rec.tool_input_preview || '--' }}</td>
                <td>{{ rec.flag_reason || '--' }}</td>
                <td @click.stop><button class="btn-sm btn-ghost" :class="{'btn-flagged': rec.flagged}" @click="toggleFlag(rec)">{{ rec.flagged ? '✅ 已标记' : '🚩 标记' }}</button></td>
              </tr>
              <tr v-if="expandedIds.has(rec.id)" class="detail-row">
                <td colspan="8">
                  <div class="detail-grid">
                    <div class="detail-section"><div class="detail-label">工具输入参数</div><JsonHighlight :content="rec.tool_input_preview || '(无数据)'" /></div>
                    <div class="detail-section"><div class="detail-label">工具返回结果</div><JsonHighlight :content="rec.tool_result_preview || '(无数据)'" /></div>
                    <div v-if="rec.source_category || rec.source_key || rec.source_descriptor_json" class="detail-section">
                      <div class="detail-label">来源分类</div>
                      <div class="risk-assessment source-meta">
                        <span v-if="rec.source_category" class="source-badge source-badge-clickable" @click.stop="goToSourceAudit(rec.source_category)">{{ rec.source_category }}</span>
                        <span v-if="rec.source_key" class="source-key">{{ rec.source_key }}</span>
                      </div>
                      <JsonHighlight v-if="rec.source_descriptor_json" :content="formatSourceDescriptor(rec.source_descriptor_json)" />
                    </div>
                    <div class="detail-section" v-if="rec.risk_level==='critical'||rec.flag_reason">
                      <div class="detail-label">风险评估</div>
                      <div class="risk-assessment">
                        <span class="risk-badge" :class="'risk-'+rec.risk_level" style="font-size:11px;">{{ rec.risk_level }}</span>
                        <span v-if="rec.flag_reason">{{ rec.flag_reason }}</span>
                        <span v-if="rec.risk_level==='critical'" class="risk-desc">此工具可直接执行系统命令，存在远程代码执行（RCE）风险。建议配合安全策略拦截。</span>
                        <span v-else-if="rec.risk_level==='high'" class="risk-desc">此工具可能修改文件或发送数据，需审查操作内容。</span>
                      </div>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 规则编辑弹窗 -->
    <div v-if="editingRule" class="modal-overlay" @click.self="editingRule=null">
      <div class="modal-content" style="max-width:500px">
        <div class="modal-header"><h3>{{ editingRule._isNew ? '新建规则' : '编辑规则' }}</h3><button class="btn-close" @click="editingRule=null">✕</button></div>
        <div class="modal-body">
          <div class="form-group"><label class="form-label">规则名称</label><input v-model="editingRule.name" class="form-input" placeholder="如: 禁止执行命令" /></div>
          <div class="form-group"><label class="form-label">匹配条件</label><input v-model="editingRule.condition" class="form-input" placeholder="如: tool_name == 'exec'" /></div>
          <div class="form-group"><label class="form-label">触发动作</label>
            <select v-model="editingRule.action" class="form-input"><option value="log">记录</option><option value="warn">告警</option><option value="block">阻断</option></select>
          </div>
          <div class="form-group"><label class="form-label"><input type="checkbox" v-model="editingRule.enabled" style="margin-right:6px;accent-color:var(--color-primary)" />启用</label></div>
        </div>
        <div class="modal-footer"><button class="btn btn-ghost btn-sm" @click="editingRule=null">取消</button><button class="btn btn-primary btn-sm" @click="saveRule">保存</button></div>
      </div>
    </div>
    <ConfirmModal :visible="cfmVisible" :title="cfmTitle" :message="cfmMsg" :type="cfmType" @confirm="doConfirmAction" @cancel="cfmVisible = false" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import { showToast } from '../stores/app.js'
import Icon from '../components/Icon.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const cfmVisible = ref(false), cfmTitle = ref(''), cfmMsg = ref(''), cfmType = ref('danger')
let cfmAction = null
function doConfirmAction() { cfmVisible.value = false; if (cfmAction) cfmAction() }
import StatCard from '../components/StatCard.vue'
import TrendChart from '../components/TrendChart.vue'
import PieChart from '../components/PieChart.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'
import JsonHighlight from '../components/JsonHighlight.vue'

const svgRobot='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><line x1="12" y1="7" x2="12" y2="11"/><line x1="8" y1="16" x2="8" y2="16.01"/><line x1="16" y1="16" x2="16" y2="16.01"/></svg>'
const svgAlert='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgFlag='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"/><line x1="4" y1="22" x2="4" y2="15"/></svg>'
const svgPercent='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="5" x2="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/></svg>'
const svgTrend='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgBar='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="12" width="4" height="8"/><rect x="10" y="8" width="4" height="12"/><rect x="17" y="4" width="4" height="16"/></svg>'
const svgShieldCheck='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>'
const svgCanary='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M8 15h8"/><circle cx="9" cy="9" r="1"/><circle cx="15" cy="9" r="1"/></svg>'
const svgBudget='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>'
const svgGlobe='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M2 12h20"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10Z"/></svg>'
const svgGlobeLarge='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><path d="M2 12h20"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10Z"/></svg>'

const riskColors={low:'#6B7280',medium:'#3B82F6',high:'#F59E0B',critical:'#EF4444'}
const criticalTools=new Set(['exec','shell','bash','run_command','execute_command'])
const highTools=new Set(['write_file','edit_file','delete_file','write','edit','http_request','curl','web_fetch','send_email','send_message','message'])
const mediumTools=new Set(['read_file','read','list_directory','web_search','browser'])

function classifyRisk(n){return criticalTools.has(n)?'critical':highTools.has(n)?'high':mediumTools.has(n)?'medium':'low'}
function getRiskColor(r){return riskColors[r]||'#6B7280'}

const loaded=ref(false)
const stats=ref({total:0,high_risk_count:0,flagged_count:0,high_risk_rate:'0%',source_category_count:0})
const timelineData=ref([]),trendRange=ref('24h'),topTools=ref([]),pieData=ref([])
const highRiskRecords=ref([]),expandedIds=ref(new Set()),highRiskRef=ref(null)
const canaryLeakCount=ref(0),budgetViolationCount=ref(0)
const sourceCategoryRows=ref([])
const searchText=ref(''),filterRisk=ref('')
const showRules=ref(false)
const rules=ref([
  {name:'禁止执行系统命令',condition:'tool_name in [exec,shell,bash]',action:'block',enabled:true},
  {name:'文件写入告警',condition:'tool_name in [write_file,edit_file]',action:'warn',enabled:true},
  {name:'外部请求记录',condition:'tool_name in [http_request,curl,web_fetch]',action:'log',enabled:false},
])
const editingRule=ref(null)

const alertCount=computed(()=>highRiskRecords.value.filter(r=>r.flagged||r.risk_level==='critical').length)
const alertPreview=computed(()=>highRiskRecords.value.filter(r=>r.flagged||r.risk_level==='critical').slice(0,3))
const filteredHighRisk=computed(()=>{
  let list=[...highRiskRecords.value]
  if(searchText.value){const q=searchText.value.toLowerCase();list=list.filter(r=>(r.tool_name||'').toLowerCase().includes(q))}
  if(filterRisk.value)list=list.filter(r=>r.risk_level===filterRisk.value)
  return list
})

function formatSourceDescriptor(raw){
  if(!raw) return '(无数据)'
  try{return JSON.stringify(JSON.parse(raw), null, 2)}catch{return raw}
}

function toggleExpand(id){const s=new Set(expandedIds.value);s.has(id)?s.delete(id):s.add(id);expandedIds.value=s}
function scrollToHighRisk(){nextTick(()=>{highRiskRef.value?.scrollIntoView({behavior:'smooth',block:'start'})})}
function toggleFlag(rec){rec.flagged=!rec.flagged;showToast(rec.flagged?'已标记异常':'已取消标记','success')}

const router=useRouter()
function goToSettings(section){router.push({path:'/settings',query:{section}})}
function fmtTime(ts){if(!ts)return'--';const d=new Date(ts);return isNaN(d.getTime())?String(ts):d.toLocaleString('zh-CN',{hour12:false})}

function addRule(){editingRule.value={name:'',condition:'',action:'log',enabled:true,_isNew:true}}
function editRule(i){editingRule.value={...rules.value[i],_idx:i}}
function deleteRule(i){
  cfmTitle.value='删除规则';cfmMsg.value='确认删除此规则？该操作不可恢复。';cfmType.value='danger'
  cfmAction=()=>{rules.value.splice(i,1);showToast('规则已删除','success')};cfmVisible.value=true
}
function saveRule(){
  if(!editingRule.value.name){showToast('请输入规则名称','error');return}
  if(editingRule.value._isNew){rules.value.push({...editingRule.value});delete rules.value[rules.value.length-1]._isNew}
  else{const i=editingRule.value._idx;rules.value[i]={...editingRule.value};delete rules.value[i]._idx}
  editingRule.value=null;showToast('规则已保存','success')
}

function exportHighRisk(){
  const blob=new Blob([JSON.stringify(highRiskRecords.value,null,2)],{type:'application/json'})
  const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download='high-risk-calls-'+new Date().toISOString().slice(0,10)+'.json';a.click();URL.revokeObjectURL(a.href)
  showToast('导出成功','success')
}

const trendChartData=computed(()=>timelineData.value.map(t=>({total:t.total||0,critical:t.critical||0,high:t.high||0,medium:t.medium||0})))
const trendLines=[{key:'total',color:'#3B82F6',label:'总调用'},{key:'critical',color:'#EF4444',label:'极危'},{key:'high',color:'#F59E0B',label:'高危'},{key:'medium',color:'#8B5CF6',label:'中危'}]
const trendXLabels=computed(()=>timelineData.value.map(t=>{const h=t.hour||'';if(trendRange.value==='7d')return h.substring(5,10)+' '+h.substring(11,13)+':00';const hp=h.substring(11,13);return hp?hp+':00':''}))

function onTrendRangeChange(range){trendRange.value=range;loadTimeline()}

async function loadTimeline(){try{const d=await api('/api/v1/llm/tools/timeline?hours='+(trendRange.value==='7d'?168:24));timelineData.value=d.timeline||[]}catch{timelineData.value=[]}}

async function loadData(){
  try{
    const d=await api('/api/v1/llm/tools/stats')
    const hrc=(d.by_risk?.high||0)+(d.by_risk?.critical||0),total=d.total||0
    const rate=total>0?((d.high_risk_24h||0)/total*100).toFixed(1):'0.0'
    const bySource=Array.isArray(d.by_source_category)?d.by_source_category:[]
    stats.value={total,high_risk_count:hrc,flagged_count:d.flagged_count||0,high_risk_rate:rate+'%',source_category_count:bySource.length}
    const byTool=d.by_tool||[],maxC=byTool.length?byTool[0].count:1
    topTools.value=byTool.slice(0,10).map(t=>({name:t.name,count:t.count,pct:(t.count/maxC)*100}))
    const byRisk=d.by_risk||{}
    pieData.value=[{label:'critical',value:byRisk.critical||0,color:'#EF4444'},{label:'high',value:byRisk.high||0,color:'#F59E0B'},{label:'medium',value:byRisk.medium||0,color:'#3B82F6'},{label:'low',value:byRisk.low||0,color:'#6B7280'}].filter(d=>d.value>0)
    const sourceMax=bySource.length?bySource[0].count:1
    sourceCategoryRows.value=bySource.map(s=>({category:s.category,count:s.count,pct:(s.count/sourceMax)*100}))
  }catch{stats.value={total:0,high_risk_count:0,flagged_count:0,high_risk_rate:'0%',source_category_count:0};topTools.value=[];pieData.value=[];sourceCategoryRows.value=[]}
  await loadTimeline()
  try{const d=await api('/api/v1/llm/tools?risk_level=critical&limit=10'),d2=await api('/api/v1/llm/tools?risk_level=high&limit=10');highRiskRecords.value=[...(d.records||[]),...(d2.records||[])].sort((a,b)=>b.id-a.id).slice(0,20)}catch{highRiskRecords.value=[]}
  try{canaryLeakCount.value=(await api('/api/v1/llm/canary/status')).leak_count||0}catch{canaryLeakCount.value=0}
  try{budgetViolationCount.value=(await api('/api/v1/llm/budget/status')).violations_24h||0}catch{budgetViolationCount.value=0}
  loaded.value=true
}

let timer=null
onMounted(()=>{loadData();timer=setInterval(loadData,30000)})
onUnmounted(()=>clearInterval(timer))
</script>

<style scoped>
.alert-panel{background:rgba(239,68,68,.08);border:1px solid rgba(239,68,68,.25);border-radius:var(--radius-lg);padding:var(--space-4)}
.alert-header{display:flex;align-items:center;gap:var(--space-2);margin-bottom:var(--space-3)}
.alert-icon{font-size:1.2rem}.alert-title{font-size:var(--text-base);font-weight:700;color:#EF4444}
.alert-items{display:flex;flex-direction:column;gap:6px;margin-bottom:var(--space-2)}
.alert-item{display:flex;align-items:center;gap:var(--space-2);font-size:var(--text-sm);padding:6px 10px;background:rgba(239,68,68,.06);border-radius:var(--radius-sm)}
.alert-item code{background:rgba(239,68,68,.12);padding:1px 6px;border-radius:3px;font-size:var(--text-xs);font-family:var(--font-mono);color:#EF4444}
.alert-time{color:var(--text-tertiary);font-size:var(--text-xs)}.alert-reason{color:var(--text-secondary);font-size:var(--text-xs);margin-left:auto}
.alert-link{font-size:var(--text-sm);color:#EF4444;cursor:pointer;text-decoration:underline}
.alert-footer{display:flex;align-items:center;justify-content:space-between;margin-top:var(--space-2)}
.alert-config-link{font-size:var(--text-sm);color:var(--color-primary);cursor:pointer;text-decoration:none;transition:text-decoration .15s}
.alert-config-link:hover{text-decoration:underline}
.canary-alert{background:rgba(217,119,6,.08);border:1px solid rgba(217,119,6,.25)}
.budget-alert{background:rgba(234,88,12,.08);border:1px solid rgba(234,88,12,.25)}
.source-badge{display:inline-flex;align-items:center;padding:2px 8px;border-radius:999px;background:rgba(34,197,94,.12);border:1px solid rgba(34,197,94,.28);color:#86efac;font-size:11px;font-family:var(--font-mono)}.source-badge-clickable{cursor:pointer}.source-badge-clickable:hover{filter:brightness(1.08)}
.source-meta{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.source-key{font-family:var(--font-mono);font-size:11px;color:var(--text-secondary);word-break:break-all}
.source-category-list{display:flex;flex-direction:column;gap:10px}
.source-category-row{display:grid;grid-template-columns:160px 1fr 64px;gap:10px;align-items:center}.source-category-row-clickable{cursor:pointer;border-radius:8px;padding:4px 6px;transition:background .15s ease}.source-category-row-clickable:hover{background:rgba(34,197,94,.06)}
.source-track{height:18px}
.source-fill{background:linear-gradient(90deg,#22c55e,#14b8a6);font-size:11px}
.source-pct{font-size:12px;color:var(--text-secondary);text-align:right}

.filter-bar{padding:12px 16px}.filter-bar-inner{display:flex;gap:12px;align-items:center;flex-wrap:wrap}
.search-box{position:relative;flex:1;min-width:200px}
.search-icon{position:absolute;left:10px;top:50%;transform:translateY(-50%);color:var(--text-tertiary);pointer-events:none}
.search-input{width:100%;padding:8px 30px 8px 32px;background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-sm)}
.search-input:focus{outline:none;border-color:var(--color-primary)}
.search-clear{position:absolute;right:8px;top:50%;transform:translateY(-50%);background:none;border:none;color:var(--text-tertiary);font-size:1.1rem;cursor:pointer;padding:0 4px}
.search-clear:hover{color:var(--text-primary)}
.filter-selects{display:flex;gap:8px;flex-wrap:wrap}
.filter-select{padding:6px 10px;background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-xs);cursor:pointer}
.filter-select:focus{outline:none;border-color:var(--color-primary)}

.expand-toggle{width:30px;text-align:center}.expand-toggle svg{transition:transform .2s ease;color:var(--text-tertiary)}.expand-toggle svg.rotated{transform:rotate(90deg)}
.row-expanded{background:rgba(99,102,241,.04)!important}
.row-critical{background:rgba(239,68,68,.06)!important}.row-high{background:rgba(245,158,11,.04)!important}
.row-disabled{opacity:.5}
.detail-row td{padding:0!important}
.detail-grid{padding:var(--space-3);display:flex;flex-direction:column;gap:var(--space-3);border-top:1px solid var(--border-subtle);background:var(--bg-base)}
.detail-label{font-size:var(--text-xs);font-weight:600;color:var(--text-tertiary);text-transform:uppercase;letter-spacing:0.05em;margin-bottom:4px}
.risk-assessment{display:flex;flex-direction:column;gap:4px;font-size:var(--text-sm);color:var(--text-secondary)}
.risk-desc{font-size:var(--text-xs);color:var(--text-tertiary);font-style:italic}
.risk-badge{display:inline-block;padding:2px 8px;border-radius:9999px;font-size:var(--text-xs);font-weight:600;text-transform:uppercase;letter-spacing:0.02em}
.risk-low{background:rgba(107,114,128,.15);color:#6B7280}.risk-medium{background:rgba(59,130,246,.15);color:#3B82F6}
.risk-high{background:rgba(245,158,11,.15);color:#F59E0B}.risk-critical{background:rgba(239,68,68,.15);color:#EF4444}
.td-preview{max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:var(--text-xs);font-family:var(--font-mono)}
.td-mono{font-family:var(--font-mono);font-size:var(--text-xs);max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
code{background:var(--bg-elevated);padding:2px 6px;border-radius:4px;font-size:var(--text-xs);font-family:var(--font-mono)}
.hbar-rank{width:24px;font-size:var(--text-xs);color:var(--color-primary);font-weight:700;text-align:center;flex-shrink:0}
.hbar-fill-anim{width:0;animation:hbar-grow .8s ease-out forwards}
@keyframes hbar-grow{from{width:0}to{width:var(--target-w)}}
.list-anim-enter-active{animation:list-in .2s ease-out}.list-anim-leave-active{animation:list-out .2s ease-in}
.list-anim-move{transition:transform .2s ease}
@keyframes list-in{from{opacity:0;transform:translateY(-10px)}to{opacity:1;transform:translateY(0)}}
@keyframes list-out{from{opacity:1;transform:translateY(0)}to{opacity:0;transform:translateY(10px)}}

.action-badge{display:inline-block;padding:2px 8px;border-radius:9999px;font-size:var(--text-xs);font-weight:600;text-transform:uppercase}
.action-log{background:rgba(107,114,128,.15);color:#6B7280}
.action-warn{background:rgba(245,158,11,.15);color:#F59E0B}
.action-block{background:rgba(239,68,68,.15);color:#EF4444}
.toggle-btn{padding:2px 10px;border-radius:9999px;font-size:var(--text-xs);font-weight:600;cursor:pointer;border:1px solid var(--border-subtle);background:var(--bg-elevated);color:var(--text-tertiary);transition:all .15s}
.toggle-btn.active{background:rgba(34,197,94,.15);color:#22C55E;border-color:rgba(34,197,94,.3)}
.btn-flagged{background:rgba(34,197,94,.1);color:#22C55E;border-color:rgba(34,197,94,.3)}
.btn-sm{padding:4px 12px;font-size:var(--text-xs);border-radius:var(--radius-md);cursor:pointer;border:1px solid var(--border-subtle);transition:all .15s}
.btn-ghost{background:transparent;color:var(--text-secondary)}.btn-ghost:hover{background:var(--bg-elevated);color:var(--text-primary)}

.modal-overlay{position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.6);z-index:9999;display:flex;align-items:center;justify-content:center}
.modal-content{background:var(--bg-surface);border-radius:var(--radius-lg);width:90%;max-width:700px;max-height:85vh;overflow-y:auto;box-shadow:var(--shadow-lg)}
.modal-header{display:flex;justify-content:space-between;align-items:center;padding:16px 20px;border-bottom:1px solid var(--border-subtle)}
.modal-header h3{margin:0;font-size:var(--text-base)}.btn-close{background:none;border:none;font-size:18px;cursor:pointer;color:var(--text-secondary);padding:4px 8px}.btn-close:hover{color:var(--text-primary)}
.modal-body{padding:20px}.modal-footer{display:flex;justify-content:flex-end;gap:8px;padding:12px 20px;border-top:1px solid var(--border-subtle)}
.form-group{margin-bottom:14px}.form-label{display:block;font-size:var(--text-sm);font-weight:600;color:var(--text-secondary);margin-bottom:4px}
.form-input{width:100%;padding:8px 12px;background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-sm)}
.form-input:focus{outline:none;border-color:var(--color-primary)}
</style>