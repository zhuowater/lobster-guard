<template>
  <div class="redteam-page">
    <!-- 页头 -->
    <div class="page-header">
      <div>
        <h1 class="page-title"><Icon name="crosshair" :size="20" /> Red Team Autopilot</h1>
        <p class="page-subtitle">自动化安全测试 — 龙虾卫士打自己</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-outline" @click="activeTab = 'scenarios'" v-if="activeTab !== 'scenarios'">
          <Icon name="list" :size="14" /> 场景库
        </button>
        <button class="btn btn-primary" @click="showRunModal = true" :disabled="running">
          <span v-if="running" class="spinner"></span>
          <span v-else>▶</span>
          {{ running ? '测试中...' : '开始测试' }}
        </button>
      </div>
    </div>

    <!-- 统计面板 -->
    <div class="stats-grid" v-if="statsLoaded">
      <StatCard :iconSvg="svgCrosshair" :value="stats.totalTests" label="测试总数" color="blue" :badge="stats.recentCount ? '近7天 +'+stats.recentCount : ''" />
      <StatCard :iconSvg="svgShieldX" :value="stats.totalVulns" label="发现漏洞" color="red" :badge="vulnBadge" />
      <StatCard :iconSvg="svgPercent" :value="stats.passRate + '%'" label="通过率" color="green" />
      <StatCard :iconSvg="svgClock" :value="stats.lastTestTime || '从未'" label="最近测试" color="indigo" />
    </div>
    <div class="stats-grid" v-else>
      <Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" />
    </div>

    <!-- Tab 导航 -->
    <div class="tab-bar">
      <button class="tab-btn" :class="{ active: activeTab === 'report' }" @click="activeTab = 'report'">
        <Icon name="file-text" :size="14" /> 最新报告
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'scenarios' }" @click="activeTab = 'scenarios'">
        <Icon name="list" :size="14" /> 测试场景
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'history' }" @click="activeTab = 'history'">
        <Icon name="clock" :size="14" /> 测试历史
      </button>
      <button class="tab-btn" :class="{ active: activeTab === 'vulns' }" @click="activeTab = 'vulns'">
        <Icon name="alert-triangle" :size="14" /> 漏洞报告
        <span class="tab-badge" v-if="allVulns.length">{{ allVulns.length }}</span>
      </button>
    </div>

    <!-- ==================== Tab: 最新报告 ==================== -->
    <div v-if="activeTab === 'report'">
      <div class="report-card" v-if="latestReport">
        <div class="report-card-header">
          <span class="report-card-title">最新测试</span>
          <span class="report-card-time">{{ formatTime(latestReport.timestamp) }}</span>
        </div>
        <div class="detection-rate">
          <div class="rate-ring">
            <svg viewBox="0 0 120 120" class="rate-svg">
              <circle cx="60" cy="60" r="50" fill="none" stroke="rgba(255,255,255,0.1)" stroke-width="10"/>
              <circle cx="60" cy="60" r="50" fill="none" :stroke="rateColor(latestReport.pass_rate)" stroke-width="10" stroke-linecap="round" :stroke-dasharray="rateArc(latestReport.pass_rate)" stroke-dashoffset="0" transform="rotate(-90 60 60)" class="rate-circle-animated"/>
            </svg>
            <div class="rate-text">
              <div class="rate-num" :style="{color: rateColor(latestReport.pass_rate)}">{{ latestReport.pass_rate.toFixed(1) }}%</div>
              <div class="rate-label">检测率</div>
            </div>
          </div>
          <div class="rate-stats">
            <div class="stat-item"><span class="stat-value">{{ latestReport.total_tests }}</span><span class="stat-label">测试数</span></div>
            <div class="stat-item"><span class="stat-value stat-pass">{{ latestReport.passed }}</span><span class="stat-label">通过</span></div>
            <div class="stat-item"><span class="stat-value stat-fail">{{ latestReport.failed }}</span><span class="stat-label">漏洞</span></div>
            <div class="stat-item"><span class="stat-value">{{ latestReport.duration_ms }}ms</span><span class="stat-label">耗时</span></div>
          </div>
        </div>
        <div class="category-section" v-if="latestReport.category_stats">
          <h3 class="section-title">OWASP 分类</h3>
          <div class="category-list">
            <div class="category-item" v-for="(stat, key) in latestReport.category_stats" :key="key">
              <div class="cat-header">
                <span class="cat-name">{{ categoryName(key) }}</span>
                <span class="cat-ratio">{{ stat.passed }}/{{ stat.total }}</span>
                <span class="cat-rate" :style="{color: rateColor(stat.pass_rate)}">{{ stat.pass_rate.toFixed(0) }}%</span>
              </div>
              <div class="cat-bar"><div class="cat-bar-fill" :style="{width: stat.pass_rate + '%', background: rateColor(stat.pass_rate)}"></div></div>
            </div>
          </div>
        </div>
        <div class="vuln-section" v-if="latestReport.vulnerabilities && latestReport.vulnerabilities.length > 0">
          <h3 class="section-title">⚠️ 发现 {{ latestReport.vulnerabilities.length }} 个漏洞</h3>
          <div class="vuln-list">
            <div class="vuln-item" v-for="v in latestReport.vulnerabilities" :key="v.vector_id" :class="'vuln-border-' + v.severity">
              <div class="vuln-header">
                <span class="vuln-id">{{ v.vector_id }}</span>
                <span class="vuln-name">{{ v.name }}</span>
                <span class="severity-badge" :class="'sev-' + v.severity">{{ v.severity }}</span>
              </div>
              <div class="vuln-desc">{{ v.description }}</div>
              <div class="vuln-payload" v-if="v.payload"><code>{{ v.payload }}</code></div>
              <div class="vuln-suggestion" v-if="v.suggestion"><Icon name="info" :size="14" /> {{ v.suggestion }}</div>
              <a class="vuln-fix-link" @click.stop="goFixRule(v)"><Icon name="wrench" :size="12" /> 前往规则页修复 →</a>
            </div>
          </div>
        </div>
        <div class="rec-section" v-if="latestReport.recommendations && latestReport.recommendations.length > 0">
          <h3 class="section-title"><Icon name="info" :size="14" /> 建议</h3>
          <ul class="rec-list"><li v-for="(rec, idx) in latestReport.recommendations" :key="idx">{{ rec }}</li></ul>
        </div>
        <div class="detail-section">
          <button class="btn-toggle" @click="showDetails = !showDetails">{{ showDetails ? '收起' : '展开' }}详细结果 ({{ latestReport.results?.length || 0 }} 条)</button>
          <div class="detail-table-wrap" v-if="showDetails && latestReport.results">
            <table class="data-table">
              <thead><tr><th>ID</th><th>名称</th><th>分类</th><th>期望</th><th>实际</th><th>结果</th><th>匹配规则</th><th>耗时</th></tr></thead>
              <tbody>
                <tr v-for="r in latestReport.results" :key="r.vector_id" :class="{'row-fail': !r.passed && r.expected !== 'pass'}">
                  <td class="td-mono">{{ r.vector_id }}</td><td>{{ r.name }}</td><td>{{ categoryName(r.category) }}</td>
                  <td><span class="action-badge" :class="'act-' + r.expected">{{ r.expected }}</span></td>
                  <td><span class="action-badge" :class="'act-' + r.action">{{ r.action }}</span></td>
                  <td><span :class="r.passed ? 'result-pass' : 'result-fail'">{{ r.passed ? '✅' : '❌' }}</span></td>
                  <td class="td-mono">{{ r.matched_rule || '-' }}</td><td class="td-mono">{{ r.latency_us }}μs</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <EmptyState v-else-if="!loading" icon="crosshair" title="尚未运行红队测试" subtitle="点击「开始测试」运行第一次安全扫描" />
    </div>

    <!-- ==================== Tab: 测试场景 ==================== -->
    <div v-if="activeTab === 'scenarios'" class="scenarios-tab">
      <div class="scenarios-toolbar">
        <div class="search-box"><Icon name="search" :size="14" /><input v-model="scenarioSearch" placeholder="搜索场景..." class="search-input" /></div>
        <div class="filter-group">
          <button class="filter-btn" :class="{ active: scenarioFilter === '' }" @click="scenarioFilter = ''">全部</button>
          <button class="filter-btn" :class="{ active: scenarioFilter === c }" v-for="c in scenarioCategories" :key="c" @click="scenarioFilter = c">{{ categoryName(c) }}</button>
        </div>
        <button class="btn btn-sm btn-outline" @click="showCreateScenario = true"><Icon name="plus" :size="14" /> 自定义场景</button>
      </div>
      <div class="scenario-grid">
        <div class="scenario-card" v-for="s in filteredScenarios" :key="s.id" :class="{ selected: selectedScenarios.has(s.id), custom: s.custom }" @click="toggleScenario(s.id)">
          <div class="scenario-card-top">
            <input type="checkbox" :checked="selectedScenarios.has(s.id)" @click.stop="toggleScenario(s.id)" class="scenario-check" />
            <span class="severity-badge" :class="'sev-' + s.severity">{{ s.severity }}</span>
            <span class="scenario-engine">{{ s.engine }}</span>
            <button v-if="s.custom" class="btn-icon-sm btn-danger-icon" @click.stop="removeCustomScenario(s.id)" title="删除">×</button>
          </div>
          <div class="scenario-name">{{ s.name }}</div>
          <div class="scenario-desc">{{ s.description }}</div>
          <div class="scenario-payload"><code>{{ truncate(s.payload, 80) }}</code></div>
          <div class="scenario-meta">
            <span class="scenario-cat">{{ categoryName(s.category) }}</span>
            <span class="scenario-expected">期望: <span class="action-badge" :class="'act-' + s.expected_action">{{ s.expected_action }}</span></span>
          </div>
        </div>
      </div>
      <div class="batch-bar" v-if="selectedScenarios.size > 0">
        <span>已选 <strong>{{ selectedScenarios.size }}</strong> 个场景</span>
        <button class="btn btn-sm btn-outline" @click="selectAllScenarios">全选 ({{ filteredScenarios.length }})</button>
        <button class="btn btn-sm btn-outline" @click="selectedScenarios.clear()">清空</button>
        <button class="btn btn-sm btn-primary" @click="runSelectedScenarios" :disabled="running"><span v-if="running" class="spinner spinner-sm"></span> ▶ 执行选中</button>
      </div>
      <!-- 创建场景弹窗 -->
      <div class="modal-overlay" v-if="showCreateScenario" @click.self="showCreateScenario = false">
        <div class="modal-content">
          <div class="modal-header"><h3>创建自定义测试场景</h3><button class="btn-close" @click="showCreateScenario = false">×</button></div>
          <div class="modal-body">
            <div class="form-group"><label>场景名称</label><input v-model="newScenario.name" placeholder="例：自定义SQL注入测试" class="form-input" /></div>
            <div class="form-row">
              <div class="form-group"><label>分类</label><select v-model="newScenario.category" class="form-input"><option value="prompt_injection">Prompt Injection</option><option value="insecure_output">Insecure Output</option><option value="sensitive_info">Sensitive Info</option><option value="insecure_plugin">Insecure Plugin</option><option value="overreliance">Overreliance</option><option value="model_dos">Model DoS</option></select></div>
              <div class="form-group"><label>严重性</label><select v-model="newScenario.severity" class="form-input"><option value="critical">Critical</option><option value="high">High</option><option value="medium">Medium</option><option value="low">Low</option></select></div>
            </div>
            <div class="form-group"><label>攻击 Payload</label><textarea v-model="newScenario.payload" rows="3" placeholder="输入测试 payload..." class="form-input form-textarea"></textarea></div>
            <div class="form-row">
              <div class="form-group"><label>预期结果</label><select v-model="newScenario.expected_action" class="form-input"><option value="block">Block (应拦截)</option><option value="warn">Warn (应告警)</option><option value="pass">Pass (应放行)</option></select></div>
              <div class="form-group"><label>引擎</label><select v-model="newScenario.engine" class="form-input"><option value="inbound">Inbound</option><option value="outbound">Outbound</option><option value="llm_request">LLM Request</option><option value="llm_response">LLM Response</option></select></div>
            </div>
            <div class="form-group"><label>描述（可选）</label><input v-model="newScenario.description" placeholder="简述此场景的测试目的" class="form-input" /></div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-outline" @click="showCreateScenario = false">取消</button>
            <button class="btn btn-primary" @click="addCustomScenario" :disabled="!newScenario.name || !newScenario.payload">创建场景</button>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== Tab: 测试历史 ==================== -->
    <div v-if="activeTab === 'history'" class="history-tab">
      <div class="history-toolbar">
        <div class="filter-group">
          <button class="filter-btn" :class="{ active: historyFilter === '' }" @click="historyFilter = ''">全部</button>
          <button class="filter-btn" :class="{ active: historyFilter === 'good' }" @click="historyFilter = 'good'"><span class="dot dot-green"></span> ≥90%</button>
          <button class="filter-btn" :class="{ active: historyFilter === 'warn' }" @click="historyFilter = 'warn'"><span class="dot dot-yellow"></span> 70-89%</button>
          <button class="filter-btn" :class="{ active: historyFilter === 'bad' }" @click="historyFilter = 'bad'"><span class="dot dot-red"></span> &lt;70%</button>
        </div>
      </div>
      <div class="history-list" v-if="filteredHistory.length">
        <div class="history-item" v-for="r in filteredHistory" :key="r.id" :class="{ expanded: expandedHistory === r.id }">
          <div class="history-item-main" @click="toggleHistory(r.id)">
            <div class="history-left"><span class="history-time">{{ formatTime(r.timestamp) }}</span><span class="history-tenant">{{ r.tenant_id }}</span></div>
            <div class="history-center">
              <span class="history-rate" :style="{color: rateColor(r.pass_rate)}">{{ r.pass_rate.toFixed(1) }}%</span>
              <div class="history-mini-bar"><div class="mini-bar-fill" :style="{width: r.pass_rate + '%', background: rateColor(r.pass_rate)}"></div></div>
            </div>
            <div class="history-right">
              <span class="history-stat"><span class="stat-pass">{{ r.passed }}</span>/{{ r.total_tests }}</span>
              <span class="history-vulns" v-if="r.failed > 0">{{ r.failed }} 漏洞</span>
              <span class="history-duration">{{ r.duration_ms }}ms</span>
            </div>
            <div class="history-actions">
              <button class="btn-sm" @click.stop="viewReport(r.id)">查看</button>
              <button class="btn-sm btn-danger" @click.stop="deleteReport(r.id)">删除</button>
              <Icon :name="expandedHistory === r.id ? 'chevron-up' : 'chevron-down'" :size="14" class="expand-icon" />
            </div>
          </div>
          <div class="history-detail" v-if="expandedHistory === r.id && expandedReportData">
            <div class="detail-grid">
              <div class="detail-item" v-for="res in expandedReportData.results" :key="res.vector_id" :class="res.passed ? 'detail-passed' : 'detail-failed'">
                <div class="detail-item-top"><span class="td-mono">{{ res.vector_id }}</span><span class="detail-item-name">{{ res.name }}</span><span :class="res.passed ? 'result-pass' : 'result-fail'">{{ res.passed ? '✅' : '❌' }}</span></div>
                <div class="detail-item-payload" v-if="!res.passed"><div class="detail-label">Payload:</div><code>{{ truncate(res.payload, 120) }}</code></div>
                <div class="detail-item-meta" v-if="!res.passed">
                  <span>期望: <span class="action-badge" :class="'act-' + res.expected">{{ res.expected }}</span></span>
                  <span>实际: <span class="action-badge" :class="'act-' + res.action">{{ res.action }}</span></span>
                  <span v-if="res.matched_rule">规则: {{ res.matched_rule }}</span>
                </div>
              </div>
            </div>
          </div>
          <div class="history-detail" v-else-if="expandedHistory === r.id && !expandedReportData"><div class="detail-loading"><span class="spinner"></span> 加载中...</div></div>
        </div>
      </div>
      <EmptyState v-else icon="clock" title="暂无历史记录" subtitle="运行测试后这里会显示历史报告" />
    </div>

    <!-- ==================== Tab: 漏洞报告 ==================== -->
    <div v-if="activeTab === 'vulns'" class="vulns-tab">
      <div class="vuln-severity-bar" v-if="allVulns.length">
        <div class="sev-segment sev-critical-bg" :style="{flex: sevCount('critical')}" v-if="sevCount('critical')"></div>
        <div class="sev-segment sev-high-bg" :style="{flex: sevCount('high')}" v-if="sevCount('high')"></div>
        <div class="sev-segment sev-medium-bg" :style="{flex: sevCount('medium')}" v-if="sevCount('medium')"></div>
        <div class="sev-segment sev-low-bg" :style="{flex: sevCount('low')}" v-if="sevCount('low')"></div>
      </div>
      <div class="sev-legend" v-if="allVulns.length">
        <span class="sev-legend-item" v-if="sevCount('critical')"><span class="dot sev-critical-dot"></span> Critical {{ sevCount('critical') }}</span>
        <span class="sev-legend-item" v-if="sevCount('high')"><span class="dot sev-high-dot"></span> High {{ sevCount('high') }}</span>
        <span class="sev-legend-item" v-if="sevCount('medium')"><span class="dot sev-medium-dot"></span> Medium {{ sevCount('medium') }}</span>
        <span class="sev-legend-item" v-if="sevCount('low')"><span class="dot sev-low-dot"></span> Low {{ sevCount('low') }}</span>
      </div>
      <div class="vuln-report-list" v-if="allVulns.length">
        <div class="vuln-report-item" v-for="(v, idx) in allVulns" :key="idx" :class="'vuln-border-' + v.severity">
          <div class="vuln-header"><span class="vuln-id">{{ v.vector_id }}</span><span class="vuln-name">{{ v.name }}</span><span class="severity-badge" :class="'sev-' + v.severity">{{ v.severity }}</span></div>
          <div class="vuln-desc">{{ v.description }}</div>
          <div class="vuln-payload" v-if="v.payload"><code>{{ v.payload }}</code></div>
          <div class="vuln-suggestion" v-if="v.suggestion"><Icon name="info" :size="14" /> {{ v.suggestion }}</div>
          <div class="vuln-footer"><span class="vuln-report-time">来自: {{ formatTime(v._reportTime) }}</span><a class="vuln-fix-link" @click.stop="goFixRule(v)"><Icon name="wrench" :size="12" /> 前往规则页修复 →</a></div>
        </div>
      </div>
      <EmptyState v-else icon="shield" title="未发现漏洞" subtitle="所有测试均已通过，安全状态良好 🎉" />
    </div>

    <!-- ==================== 运行测试弹窗 ==================== -->
    <div class="modal-overlay" v-if="showRunModal" @click.self="closeRunModal">
      <div class="modal-content modal-run">
        <div class="modal-header"><h3>运行红队测试</h3><button class="btn-close" @click="closeRunModal">×</button></div>
        <div class="modal-body">
          <div class="run-options" v-if="!running && !runComplete">
            <label class="radio-label"><input type="radio" v-model="runMode" value="all" /> 执行全部场景 ({{ allScenarios.length }} 个)</label>
            <label class="radio-label"><input type="radio" v-model="runMode" value="selected" :disabled="selectedScenarios.size === 0" /> 执行选中场景 ({{ selectedScenarios.size }} 个)<span class="radio-hint" v-if="selectedScenarios.size === 0"> — 请先在场景库中选择</span></label>
          </div>
          <div class="run-progress" v-if="running">
            <div class="progress-bar-wrap"><div class="progress-bar-fill pulse-blue" :style="{width: runProgress + '%'}"></div></div>
            <div class="progress-text"><span class="spinner spinner-sm"></span> {{ runProgressText }}</div>
          </div>
          <div class="run-complete" v-if="runComplete">
            <div class="complete-icon animate-pop">{{ runSuccess ? '✅' : '⚠️' }}</div>
            <div class="complete-title">{{ runSuccess ? '测试完成' : '测试完成（发现漏洞）' }}</div>
            <div class="complete-stats"><span class="stat-pass">{{ runResultPassed }} 通过</span> / <span class="stat-fail">{{ runResultFailed }} 漏洞</span></div>
          </div>
        </div>
        <div class="modal-footer" v-if="!running">
          <button class="btn btn-outline" @click="closeRunModal">{{ runComplete ? '关闭' : '取消' }}</button>
          <button class="btn btn-primary" @click="executeTest" v-if="!runComplete">▶ 开始执行</button>
          <button class="btn btn-primary" v-if="runComplete" @click="closeRunModal(); activeTab = 'report'">查看报告</button>
        </div>
      </div>
    </div>
  </div>
</template>
<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import Skeleton from '../components/Skeleton.vue'
import EmptyState from '../components/EmptyState.vue'
import { useRouter } from 'vue-router'
import { api, apiPost, apiDelete } from '../api.js'
import { showToast } from '../stores/app.js'

const router = useRouter()

const loading = ref(true)
const running = ref(false)
const statsLoaded = ref(false)
const reports = ref([])
const latestReport = ref(null)
const showDetails = ref(false)
const activeTab = ref('report')
const showRunModal = ref(false)
const runMode = ref('all')
const runProgress = ref(0)
const runProgressText = ref('')
const runComplete = ref(false)
const runSuccess = ref(false)
const runResultPassed = ref(0)
const runResultFailed = ref(0)

const CUSTOM_KEY = 'lobster_rt_custom'

const stats = reactive({ totalTests: 0, totalVulns: 0, passRate: '0.0', lastTestTime: '', recentCount: 0, criticalCount: 0, highCount: 0, mediumCount: 0, lowCount: 0 })

const allScenarios = ref([])
const customScenarios = ref([])
const selectedScenarios = ref(new Set())
const scenarioSearch = ref('')
const scenarioFilter = ref('')
const showCreateScenario = ref(false)
const newScenario = reactive({ name: '', category: 'prompt_injection', severity: 'high', payload: '', expected_action: 'block', engine: 'inbound', description: '' })

const historyFilter = ref('')
const expandedHistory = ref(null)
const expandedReportData = ref(null)

// SVG Icons
const svgCrosshair = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="22" y1="12" x2="18" y2="12"/><line x1="6" y1="12" x2="2" y2="12"/><line x1="12" y1="6" x2="12" y2="2"/><line x1="12" y1="22" x2="12" y2="18"/></svg>'
const svgShieldX = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><line x1="9" y1="9" x2="15" y2="15"/><line x1="15" y1="9" x2="9" y2="15"/></svg>'
const svgPercent = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="5" x2="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/></svg>'
const svgClock = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>'

const categoryNames = {
  prompt_injection: 'Prompt Injection', insecure_output: 'Insecure Output',
  sensitive_info: 'Sensitive Info', insecure_plugin: 'Insecure Plugin',
  overreliance: 'Overreliance', model_dos: 'Model DoS',
}
function categoryName(k) { return categoryNames[k] || k }

function rateColor(rate) {
  if (rate >= 90) return '#10B981'
  if (rate >= 70) return '#F59E0B'
  if (rate >= 50) return '#EF4444'
  return '#DC2626'
}

function rateArc(rate) {
  const c = 2 * Math.PI * 50
  return `${(rate / 100) * c} ${c}`
}

function formatTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function truncate(s, n) {
  if (!s) return ''
  return s.length > n ? s.slice(0, n) + '...' : s
}

function goFixRule(v) {
  router.push(v.category && v.category.startsWith('LLM') ? '/llm-rules' : '/rules')
}

// ─── Computed ───
const vulnBadge = computed(() => {
  const parts = []
  if (stats.criticalCount) parts.push('C:' + stats.criticalCount)
  if (stats.highCount) parts.push('H:' + stats.highCount)
  return parts.join(' ') || ''
})

const scenarioCategories = computed(() => {
  const cats = new Set(allScenarios.value.map(s => s.category))
  return [...cats]
})

const filteredScenarios = computed(() => {
  let list = allScenarios.value
  if (scenarioFilter.value) list = list.filter(s => s.category === scenarioFilter.value)
  if (scenarioSearch.value) {
    const q = scenarioSearch.value.toLowerCase()
    list = list.filter(s => s.name.toLowerCase().includes(q) || s.payload.toLowerCase().includes(q) || s.description.toLowerCase().includes(q))
  }
  return list
})

const filteredHistory = computed(() => {
  let list = reports.value
  if (historyFilter.value === 'good') list = list.filter(r => r.pass_rate >= 90)
  else if (historyFilter.value === 'warn') list = list.filter(r => r.pass_rate >= 70 && r.pass_rate < 90)
  else if (historyFilter.value === 'bad') list = list.filter(r => r.pass_rate < 70)
  return list
})

const allVulns = computed(() => {
  if (!latestReport.value || !latestReport.value.vulnerabilities) return []
  return latestReport.value.vulnerabilities.map(v => ({ ...v, _reportTime: latestReport.value.timestamp }))
})

function sevCount(sev) {
  return allVulns.value.filter(v => v.severity === sev).length
}

// ─── Scenarios ───
function loadCustomScenarios() {
  try {
    const raw = localStorage.getItem(CUSTOM_KEY)
    if (raw) customScenarios.value = JSON.parse(raw)
  } catch (e) { /* ignore */ }
}

function saveCustomScenarios() {
  localStorage.setItem(CUSTOM_KEY, JSON.stringify(customScenarios.value))
}

function addCustomScenario() {
  const id = 'CUSTOM-' + Date.now().toString(36).toUpperCase()
  const s = { ...newScenario, id, custom: true }
  customScenarios.value.push(s)
  saveCustomScenarios()
  allScenarios.value.push(s)
  showCreateScenario.value = false
  showToast('自定义场景已创建: ' + s.name)
  Object.assign(newScenario, { name: '', category: 'prompt_injection', severity: 'high', payload: '', expected_action: 'block', engine: 'inbound', description: '' })
}

function removeCustomScenario(id) {
  customScenarios.value = customScenarios.value.filter(s => s.id !== id)
  saveCustomScenarios()
  allScenarios.value = allScenarios.value.filter(s => s.id !== id)
  selectedScenarios.value.delete(id)
  showToast('场景已删除')
}

function toggleScenario(id) {
  if (selectedScenarios.value.has(id)) selectedScenarios.value.delete(id)
  else selectedScenarios.value.add(id)
  // Trigger reactivity
  selectedScenarios.value = new Set(selectedScenarios.value)
}

function selectAllScenarios() {
  filteredScenarios.value.forEach(s => selectedScenarios.value.add(s.id))
  selectedScenarios.value = new Set(selectedScenarios.value)
}

// ─── Data Loading ───
async function loadReports() {
  try {
    const data = await api('/api/v1/redteam/reports?limit=50')
    reports.value = data.reports || []
    if (reports.value.length > 0) {
      const detail = await api('/api/v1/redteam/reports/' + reports.value[0].id)
      latestReport.value = detail
    }
    computeStats()
  } catch (e) {
    console.error('Load reports error:', e)
  } finally {
    loading.value = false
  }
}

async function loadVectors() {
  try {
    const data = await api('/api/v1/redteam/vectors')
    const vectors = (data.vectors || []).map(v => ({ ...v, custom: false }))
    loadCustomScenarios()
    allScenarios.value = [...vectors, ...customScenarios.value]
  } catch (e) {
    console.error('Load vectors error:', e)
  }
}

function computeStats() {
  const reps = reports.value
  if (reps.length === 0) { statsLoaded.value = true; return }
  stats.totalTests = reps.length
  const latest = reps[0]
  stats.passRate = (latest.pass_rate || 0).toFixed(1)
  stats.lastTestTime = formatTime(latest.timestamp)

  // Vulns from latest
  const vulns = latestReport.value?.vulnerabilities || []
  stats.totalVulns = vulns.length
  stats.criticalCount = vulns.filter(v => v.severity === 'critical').length
  stats.highCount = vulns.filter(v => v.severity === 'high').length
  stats.mediumCount = vulns.filter(v => v.severity === 'medium').length
  stats.lowCount = vulns.filter(v => v.severity === 'low').length

  // Recent 7 days
  const weekAgo = Date.now() - 7 * 86400000
  stats.recentCount = reps.filter(r => new Date(r.timestamp).getTime() > weekAgo).length

  statsLoaded.value = true
}

// ─── Test Execution ───
async function executeTest() {
  running.value = true
  runComplete.value = false
  runProgress.value = 0
  runProgressText.value = '初始化测试引擎...'

  // Simulate progress
  const timer = setInterval(() => {
    if (runProgress.value < 90) {
      runProgress.value += Math.random() * 15
      if (runProgress.value > 90) runProgress.value = 90
      const stages = ['加载攻击向量...', '执行 Prompt Injection 测试...', '执行 Output Safety 测试...', '执行 Sensitive Info 测试...', '分析测试结果...']
      runProgressText.value = stages[Math.min(Math.floor(runProgress.value / 20), stages.length - 1)]
    }
  }, 300)

  try {
    const report = await apiPost('/api/v1/redteam/run', { tenant_id: 'default' })
    clearInterval(timer)
    runProgress.value = 100
    runProgressText.value = '测试完成!'
    latestReport.value = report
    runResultPassed.value = report.passed || 0
    runResultFailed.value = report.failed || 0
    runSuccess.value = (report.failed || 0) === 0

    await loadReports()

    setTimeout(() => {
      running.value = false
      runComplete.value = true
      showToast(runSuccess.value ? '红队测试通过 ✅' : `发现 ${runResultFailed.value} 个漏洞 ⚠️`, runSuccess.value ? 'success' : 'warning')
    }, 500)
  } catch (e) {
    clearInterval(timer)
    running.value = false
    showToast('测试失败: ' + e.message, 'error')
    showRunModal.value = false
  }
}

function runSelectedScenarios() {
  runMode.value = 'selected'
  showRunModal.value = true
}

function closeRunModal() {
  if (!running.value) {
    showRunModal.value = false
    runComplete.value = false
  }
}

// ─── History ───
async function toggleHistory(id) {
  if (expandedHistory.value === id) {
    expandedHistory.value = null
    expandedReportData.value = null
    return
  }
  expandedHistory.value = id
  expandedReportData.value = null
  try {
    expandedReportData.value = await api('/api/v1/redteam/reports/' + id)
  } catch (e) {
    showToast('加载详情失败', 'error')
  }
}

async function viewReport(id) {
  try {
    const report = await api('/api/v1/redteam/reports/' + id)
    latestReport.value = report
    showDetails.value = false
    activeTab.value = 'report'
    window.scrollTo(0, 0)
    showToast('已加载报告')
  } catch (e) {
    showToast('加载报告失败: ' + e.message, 'error')
  }
}

async function deleteReport(id) {
  if (!confirm('确定删除此报告？')) return
  try {
    await apiDelete('/api/v1/redteam/reports/' + id)
    await loadReports()
    showToast('报告已删除')
  } catch (e) {
    showToast('删除失败: ' + e.message, 'error')
  }
}

onMounted(() => {
  loadReports()
  loadVectors()
})
</script>
<style scoped>
.redteam-page { max-width: 960px; margin: 0 auto; padding: var(--space-4); }

.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); }
.page-title { font-size: 1.5rem; font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }
.header-actions { display: flex; gap: var(--space-2); align-items: center; }

.btn { display: inline-flex; align-items: center; gap: 6px; padding: 10px 20px; border-radius: var(--radius-md); font-weight: 700; font-size: var(--text-sm); cursor: pointer; border: none; transition: all .2s; }
.btn-primary { background: var(--color-primary); color: #fff; }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-outline { background: transparent; border: 1px solid var(--border-subtle); color: var(--text-secondary); }
.btn-outline:hover { background: var(--bg-elevated); color: var(--text-primary); }
.btn-sm { padding: 5px 12px; font-size: 12px; }

.spinner { display: inline-block; width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.3); border-top-color: #fff; border-radius: 50%; animation: spin .6s linear infinite; }
.spinner-sm { width: 12px; height: 12px; border-width: 1.5px; }
@keyframes spin { to { transform: rotate(360deg); } }

/* Stats Grid */
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3); margin-bottom: var(--space-4); }

/* Tab Bar */
.tab-bar { display: flex; gap: 2px; border-bottom: 2px solid var(--border-subtle); margin-bottom: var(--space-4); }
.tab-btn { display: inline-flex; align-items: center; gap: 6px; padding: 10px 16px; border: none; background: transparent; color: var(--text-tertiary); font-weight: 600; font-size: var(--text-sm); cursor: pointer; border-bottom: 2px solid transparent; margin-bottom: -2px; transition: all .2s; position: relative; }
.tab-btn:hover { color: var(--text-primary); }
.tab-btn.active { color: var(--color-primary); border-bottom-color: var(--color-primary); }
.tab-badge { display: inline-block; background: #EF4444; color: #fff; font-size: 10px; font-weight: 700; padding: 1px 6px; border-radius: 9999px; margin-left: 4px; }

/* Report Card */
.report-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-5); margin-bottom: var(--space-5); }
.report-card-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); }
.report-card-title { font-weight: 700; color: var(--text-primary); }
.report-card-time { font-size: var(--text-xs); color: var(--text-tertiary); font-family: var(--font-mono); }

/* Detection Rate */
.detection-rate { display: flex; align-items: center; gap: var(--space-6); margin-bottom: var(--space-5); flex-wrap: wrap; }
.rate-ring { position: relative; width: 120px; height: 120px; flex-shrink: 0; }
.rate-svg { width: 100%; height: 100%; }
.rate-circle-animated { transition: stroke-dasharray .8s ease; }
.rate-text { position: absolute; top: 50%; left: 50%; transform: translate(-50%,-50%); text-align: center; }
.rate-num { font-size: 1.5rem; font-weight: 800; line-height: 1.2; }
.rate-label { font-size: 11px; color: var(--text-tertiary); }
.rate-stats { display: flex; gap: var(--space-5); flex-wrap: wrap; }
.stat-item { display: flex; flex-direction: column; align-items: center; }
.stat-value { font-size: 1.5rem; font-weight: 800; color: var(--text-primary); }
.stat-pass { color: #10B981; }
.stat-fail { color: #EF4444; }
.stat-label { font-size: 11px; color: var(--text-tertiary); margin-top: 2px; }

/* Category Stats */
.section-title { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); }
.category-section { margin-bottom: var(--space-5); }
.category-list { display: flex; flex-direction: column; gap: var(--space-2); }
.category-item { padding: var(--space-2) 0; }
.cat-header { display: flex; align-items: center; gap: var(--space-2); margin-bottom: 4px; font-size: var(--text-sm); }
.cat-name { flex: 1; color: var(--text-secondary); font-weight: 600; }
.cat-ratio { color: var(--text-tertiary); font-family: var(--font-mono); font-size: var(--text-xs); }
.cat-rate { font-weight: 800; font-size: var(--text-sm); min-width: 40px; text-align: right; }
.cat-bar { height: 6px; background: rgba(255,255,255,0.06); border-radius: 3px; overflow: hidden; }
.cat-bar-fill { height: 100%; border-radius: 3px; transition: width .3s ease; }

/* Vulnerability */
.vuln-section { margin-bottom: var(--space-5); }
.vuln-list, .vuln-report-list { display: flex; flex-direction: column; gap: var(--space-2); }
.vuln-item, .vuln-report-item { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-3); border-left: 3px solid #EF4444; }
.vuln-border-critical { border-left-color: #DC2626; }
.vuln-border-high { border-left-color: #EF4444; }
.vuln-border-medium { border-left-color: #F59E0B; }
.vuln-border-low { border-left-color: #6B7280; }
.vuln-header { display: flex; align-items: center; gap: var(--space-2); margin-bottom: 4px; }
.vuln-id { font-family: var(--font-mono); font-size: var(--text-xs); color: var(--text-tertiary); }
.vuln-name { font-weight: 600; font-size: var(--text-sm); color: var(--text-primary); flex: 1; }
.severity-badge { display: inline-block; padding: 1px 8px; border-radius: 9999px; font-size: 10px; font-weight: 700; text-transform: uppercase; }
.sev-critical { background: #DC2626; color: #fff; }
.sev-high { background: #EF4444; color: #fff; }
.sev-medium { background: #F59E0B; color: #1a1a2e; }
.sev-low { background: #6B7280; color: #fff; }
.vuln-desc { font-size: var(--text-xs); color: var(--text-secondary); margin-bottom: 4px; }
.vuln-payload { font-size: var(--text-xs); margin-bottom: 4px; }
.vuln-payload code { background: rgba(255,255,255,0.04); padding: 2px 6px; border-radius: var(--radius-sm); color: #F59E0B; word-break: break-all; }
.vuln-suggestion { font-size: var(--text-xs); color: #F59E0B; background: rgba(245,158,11,0.08); padding: 4px 8px; border-radius: var(--radius-sm); margin-bottom: 4px; }
.vuln-fix-link { display: inline-block; margin-top: 4px; color: var(--color-primary); cursor: pointer; font-size: 12px; text-decoration: none; }
.vuln-fix-link:hover { text-decoration: underline; }
.vuln-footer { display: flex; align-items: center; justify-content: space-between; margin-top: 6px; }
.vuln-report-time { font-size: 11px; color: var(--text-tertiary); }

/* Severity Bar */
.vuln-severity-bar { display: flex; height: 8px; border-radius: 4px; overflow: hidden; margin-bottom: var(--space-2); gap: 2px; }
.sev-segment { min-width: 4px; border-radius: 2px; }
.sev-critical-bg { background: #DC2626; }
.sev-high-bg { background: #EF4444; }
.sev-medium-bg { background: #F59E0B; }
.sev-low-bg { background: #6B7280; }
.sev-legend { display: flex; gap: var(--space-3); margin-bottom: var(--space-4); font-size: var(--text-xs); color: var(--text-secondary); }
.sev-legend-item { display: flex; align-items: center; gap: 4px; }
.dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; }
.sev-critical-dot { background: #DC2626; }
.sev-high-dot { background: #EF4444; }
.sev-medium-dot { background: #F59E0B; }
.sev-low-dot { background: #6B7280; }
.dot-green { background: #10B981; }
.dot-yellow { background: #F59E0B; }
.dot-red { background: #EF4444; }

/* Recommendations */
.rec-section { margin-bottom: var(--space-5); }
.rec-list { list-style: none; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
.rec-list li { padding: var(--space-2) var(--space-3); background: rgba(245,158,11,0.06); border-left: 3px solid #F59E0B; border-radius: 0 var(--radius-md) var(--radius-md) 0; font-size: var(--text-sm); color: var(--text-secondary); }

/* Detail */
.detail-section { margin-top: var(--space-3); }
.btn-toggle { background: transparent; border: 1px solid var(--border-subtle); color: var(--text-secondary); padding: 6px 14px; border-radius: var(--radius-md); font-size: var(--text-xs); cursor: pointer; transition: all .2s; }
.btn-toggle:hover { background: var(--bg-elevated); color: var(--text-primary); }
.detail-table-wrap { margin-top: var(--space-3); overflow-x: auto; }

/* Data Table */
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th { text-align: left; padding: 8px 10px; background: var(--bg-elevated); color: var(--text-tertiary); font-weight: 600; font-size: 10px; text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle); white-space: nowrap; }
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.row-fail { background: rgba(239,68,68,0.05); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.td-fail { color: #EF4444; font-weight: 700; }

.action-badge { display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 700; }
.act-block { background: #EF4444; color: #fff; }
.act-warn { background: #F59E0B; color: #1a1a2e; }
.act-pass { background: #10B981; color: #fff; }
.act-log { background: #6B7280; color: #fff; }
.result-pass { color: #10B981; }
.result-fail { color: #EF4444; }

/* Scenarios */
.scenarios-toolbar { display: flex; align-items: center; gap: var(--space-3); margin-bottom: var(--space-4); flex-wrap: wrap; }
.search-box { display: flex; align-items: center; gap: 6px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 6px 12px; color: var(--text-tertiary); }
.search-input { background: transparent; border: none; outline: none; color: var(--text-primary); font-size: var(--text-sm); width: 160px; }
.filter-group { display: flex; gap: 4px; flex-wrap: wrap; }
.filter-btn { padding: 4px 10px; border-radius: var(--radius-md); border: 1px solid var(--border-subtle); background: transparent; color: var(--text-tertiary); font-size: 11px; cursor: pointer; transition: all .2s; display: flex; align-items: center; gap: 4px; }
.filter-btn:hover { color: var(--text-primary); background: var(--bg-elevated); }
.filter-btn.active { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }

.scenario-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: var(--space-3); margin-bottom: var(--space-4); }
.scenario-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-3); cursor: pointer; transition: all .2s; position: relative; }
.scenario-card:hover { border-color: var(--border-default); transform: translateY(-1px); box-shadow: var(--shadow-sm); }
.scenario-card.selected { border-color: var(--color-primary); background: rgba(59,130,246,0.05); }
.scenario-card.custom { border-style: dashed; }
.scenario-card-top { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-2); }
.scenario-check { accent-color: var(--color-primary); cursor: pointer; }
.scenario-engine { font-size: 10px; color: var(--text-tertiary); background: var(--bg-elevated); padding: 1px 6px; border-radius: 4px; margin-left: auto; }
.scenario-name { font-weight: 700; font-size: var(--text-sm); color: var(--text-primary); margin-bottom: 4px; }
.scenario-desc { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: 6px; line-height: 1.4; }
.scenario-payload { margin-bottom: 6px; }
.scenario-payload code { font-size: 10px; background: rgba(255,255,255,0.04); padding: 2px 6px; border-radius: var(--radius-sm); color: var(--text-secondary); word-break: break-all; display: block; }
.scenario-meta { display: flex; align-items: center; justify-content: space-between; font-size: 11px; }
.scenario-cat { color: var(--text-tertiary); }
.scenario-expected { color: var(--text-tertiary); }
.btn-icon-sm { width: 20px; height: 20px; border-radius: 4px; border: none; cursor: pointer; font-size: 14px; display: flex; align-items: center; justify-content: center; line-height: 1; }
.btn-danger-icon { background: rgba(239,68,68,0.1); color: #EF4444; }
.btn-danger-icon:hover { background: rgba(239,68,68,0.2); }

/* Batch Bar */
.batch-bar { position: sticky; bottom: 0; background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-3) var(--space-4); display: flex; align-items: center; gap: var(--space-3); box-shadow: var(--shadow-lg); z-index: 10; font-size: var(--text-sm); color: var(--text-secondary); }

/* History */
.history-toolbar { margin-bottom: var(--space-3); }
.history-list { display: flex; flex-direction: column; gap: var(--space-2); }
.history-item { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); overflow: hidden; transition: all .2s; }
.history-item.expanded { border-color: var(--border-default); }
.history-item-main { display: grid; grid-template-columns: 1.5fr 1fr 1.2fr auto; align-items: center; gap: var(--space-3); padding: var(--space-3) var(--space-4); cursor: pointer; transition: background .15s; }
.history-item-main:hover { background: var(--bg-elevated); }
.history-left { display: flex; flex-direction: column; gap: 2px; }
.history-time { font-size: var(--text-sm); font-weight: 600; color: var(--text-primary); }
.history-tenant { font-size: 11px; color: var(--text-tertiary); }
.history-center { display: flex; align-items: center; gap: var(--space-2); }
.history-rate { font-size: var(--text-base); font-weight: 800; font-family: var(--font-mono); min-width: 52px; }
.history-mini-bar { flex: 1; height: 4px; background: rgba(255,255,255,0.06); border-radius: 2px; overflow: hidden; }
.mini-bar-fill { height: 100%; border-radius: 2px; transition: width .3s; }
.history-right { display: flex; align-items: center; gap: var(--space-3); font-size: var(--text-xs); color: var(--text-secondary); }
.history-vulns { color: #EF4444; font-weight: 700; }
.history-duration { color: var(--text-tertiary); font-family: var(--font-mono); }
.history-actions { display: flex; align-items: center; gap: 4px; }
.expand-icon { color: var(--text-tertiary); transition: transform .2s; }

.history-detail { padding: 0 var(--space-4) var(--space-4); border-top: 1px solid var(--border-subtle); }
.detail-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: var(--space-2); padding-top: var(--space-3); }
.detail-item { padding: var(--space-2) var(--space-3); border-radius: var(--radius-sm); border: 1px solid var(--border-subtle); font-size: var(--text-xs); }
.detail-passed { border-left: 3px solid #10B981; }
.detail-failed { border-left: 3px solid #EF4444; background: rgba(239,68,68,0.03); }
.detail-item-top { display: flex; align-items: center; gap: var(--space-2); }
.detail-item-name { flex: 1; font-weight: 600; color: var(--text-primary); }
.detail-item-payload { margin-top: 4px; }
.detail-item-payload code { font-size: 10px; color: #F59E0B; word-break: break-all; }
.detail-label { font-size: 10px; color: var(--text-tertiary); margin-bottom: 2px; }
.detail-item-meta { display: flex; gap: var(--space-2); margin-top: 4px; flex-wrap: wrap; font-size: 11px; color: var(--text-tertiary); }
.detail-loading { padding: var(--space-4); text-align: center; color: var(--text-tertiary); display: flex; align-items: center; justify-content: center; gap: var(--space-2); }

/* Small Buttons */
.btn-sm { padding: 3px 10px; border-radius: var(--radius-sm); font-size: 11px; font-weight: 600; cursor: pointer; border: 1px solid var(--border-subtle); background: transparent; color: var(--text-secondary); transition: all .2s; }
.btn-sm:hover { background: var(--bg-elevated); color: var(--text-primary); }
.btn-danger { border-color: rgba(239,68,68,.3); color: #EF4444; }
.btn-danger:hover { background: rgba(239,68,68,.1); }

/* Modal */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.6); z-index: 1000; display: flex; align-items: center; justify-content: center; backdrop-filter: blur(4px); }
.modal-content { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); width: 520px; max-width: 95vw; max-height: 85vh; overflow-y: auto; box-shadow: var(--shadow-lg); }
.modal-run { width: 440px; }
.modal-header { display: flex; align-items: center; justify-content: space-between; padding: var(--space-4) var(--space-5); border-bottom: 1px solid var(--border-subtle); }
.modal-header h3 { margin: 0; font-size: var(--text-base); font-weight: 700; color: var(--text-primary); }
.btn-close { background: transparent; border: none; color: var(--text-tertiary); font-size: 1.2rem; cursor: pointer; padding: 4px 8px; border-radius: var(--radius-sm); }
.btn-close:hover { background: var(--bg-elevated); color: var(--text-primary); }
.modal-body { padding: var(--space-4) var(--space-5); }
.modal-footer { display: flex; justify-content: flex-end; gap: var(--space-2); padding: var(--space-3) var(--space-5); border-top: 1px solid var(--border-subtle); }

/* Form */
.form-group { margin-bottom: var(--space-3); }
.form-group label { display: block; font-size: var(--text-xs); font-weight: 600; color: var(--text-secondary); margin-bottom: 4px; }
.form-input { width: 100%; padding: 8px 12px; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-sm); outline: none; transition: border-color .2s; box-sizing: border-box; }
.form-input:focus { border-color: var(--color-primary); }
.form-textarea { resize: vertical; font-family: var(--font-mono); font-size: var(--text-xs); }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); }

/* Run modal */
.run-options { display: flex; flex-direction: column; gap: var(--space-3); }
.radio-label { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); color: var(--text-primary); cursor: pointer; padding: var(--space-2) var(--space-3); border-radius: var(--radius-md); transition: background .15s; }
.radio-label:hover { background: var(--bg-elevated); }
.radio-label input[type="radio"] { accent-color: var(--color-primary); }
.radio-hint { font-size: var(--text-xs); color: var(--text-tertiary); }

.run-progress { margin-top: var(--space-3); }
.progress-bar-wrap { height: 6px; background: rgba(255,255,255,0.06); border-radius: 3px; overflow: hidden; margin-bottom: var(--space-2); }
.progress-bar-fill { height: 100%; border-radius: 3px; transition: width .3s ease; }
.pulse-blue { background: var(--color-primary); animation: pulse-glow 1.5s ease-in-out infinite; }
@keyframes pulse-glow { 0%, 100% { opacity: 1; } 50% { opacity: 0.7; } }
.progress-text { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); color: var(--text-secondary); }

.run-complete { text-align: center; padding: var(--space-4) 0; }
.complete-icon { font-size: 3rem; margin-bottom: var(--space-2); }
.complete-title { font-size: var(--text-base); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-2); }
.complete-stats { font-size: var(--text-sm); }

.animate-pop { animation: pop .4s cubic-bezier(.34,1.56,.64,1); }
@keyframes pop { 0% { transform: scale(0); opacity: 0; } 100% { transform: scale(1); opacity: 1; } }

@media(max-width:768px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .scenario-grid { grid-template-columns: 1fr; }
  .history-item-main { grid-template-columns: 1fr; gap: var(--space-2); }
  .detection-rate { flex-direction: column; align-items: center; }
  .rate-stats { justify-content: center; }
  .page-header { flex-direction: column; gap: var(--space-3); align-items: flex-start; }
  .tab-bar { overflow-x: auto; }
  .form-row { grid-template-columns: 1fr; }
}
</style>
