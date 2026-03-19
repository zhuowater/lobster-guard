<template>
  <div class="redteam-page">
    <!-- 页头 -->
    <div class="page-header">
      <div>
        <h1 class="page-title">🎯 Red Team Autopilot</h1>
        <p class="page-subtitle">自动化安全测试 — 龙虾卫士打自己</p>
      </div>
      <button class="btn btn-primary" @click="runTest" :disabled="running">
        <span v-if="running" class="spinner"></span>
        <span v-else>▶</span>
        {{ running ? '测试中...' : '开始测试' }}
      </button>
    </div>

    <!-- 最新报告概要 -->
    <div class="report-card" v-if="latestReport">
      <div class="report-card-header">
        <span class="report-card-title">最新测试</span>
        <span class="report-card-time">{{ formatTime(latestReport.timestamp) }}</span>
      </div>

      <!-- 检测率环形图 -->
      <div class="detection-rate">
        <div class="rate-ring">
          <svg viewBox="0 0 120 120" class="rate-svg">
            <circle cx="60" cy="60" r="50" fill="none" stroke="rgba(255,255,255,0.1)" stroke-width="10"/>
            <circle cx="60" cy="60" r="50" fill="none"
              :stroke="rateColor(latestReport.pass_rate)"
              stroke-width="10"
              stroke-linecap="round"
              :stroke-dasharray="rateArc(latestReport.pass_rate)"
              stroke-dashoffset="0"
              transform="rotate(-90 60 60)"
            />
          </svg>
          <div class="rate-text">
            <div class="rate-num" :style="{color: rateColor(latestReport.pass_rate)}">{{ latestReport.pass_rate.toFixed(1) }}%</div>
            <div class="rate-label">检测率</div>
          </div>
        </div>
        <div class="rate-stats">
          <div class="stat-item">
            <span class="stat-value">{{ latestReport.total_tests }}</span>
            <span class="stat-label">测试数</span>
          </div>
          <div class="stat-item">
            <span class="stat-value stat-pass">{{ latestReport.passed }}</span>
            <span class="stat-label">通过</span>
          </div>
          <div class="stat-item">
            <span class="stat-value stat-fail">{{ latestReport.failed }}</span>
            <span class="stat-label">漏洞</span>
          </div>
          <div class="stat-item">
            <span class="stat-value">{{ latestReport.duration_ms }}ms</span>
            <span class="stat-label">耗时</span>
          </div>
        </div>
      </div>

      <!-- OWASP 分类统计 -->
      <div class="category-section" v-if="latestReport.category_stats">
        <h3 class="section-title">OWASP 分类</h3>
        <div class="category-list">
          <div class="category-item" v-for="(stat, key) in latestReport.category_stats" :key="key">
            <div class="cat-header">
              <span class="cat-name">{{ categoryName(key) }}</span>
              <span class="cat-ratio">{{ stat.passed }}/{{ stat.total }}</span>
              <span class="cat-rate" :style="{color: rateColor(stat.pass_rate)}">{{ stat.pass_rate.toFixed(0) }}%</span>
            </div>
            <div class="cat-bar">
              <div class="cat-bar-fill" :style="{width: stat.pass_rate + '%', background: rateColor(stat.pass_rate)}"></div>
            </div>
          </div>
        </div>
      </div>

      <!-- 漏洞列表 -->
      <div class="vuln-section" v-if="latestReport.vulnerabilities && latestReport.vulnerabilities.length > 0">
        <h3 class="section-title">⚠️ 发现 {{ latestReport.vulnerabilities.length }} 个漏洞</h3>
        <div class="vuln-list">
          <div class="vuln-item" v-for="v in latestReport.vulnerabilities" :key="v.vector_id">
            <div class="vuln-header">
              <span class="vuln-id">{{ v.vector_id }}</span>
              <span class="vuln-name">{{ v.name }}</span>
              <span class="severity-badge" :class="'sev-' + v.severity">{{ v.severity }}</span>
            </div>
            <div class="vuln-desc">{{ v.description }}</div>
            <div class="vuln-suggestion" v-if="v.suggestion">💡 {{ v.suggestion }}</div>
            <a class="vuln-fix-link" @click.stop="$router.push(v.category && v.category.startsWith('LLM') ? '/llm-rules' : '/rules')"><Icon name="wrench" :size="12" /> 前往规则页修复 →</a>
          </div>
        </div>
      </div>

      <!-- 建议 -->
      <div class="rec-section" v-if="latestReport.recommendations && latestReport.recommendations.length > 0">
        <h3 class="section-title">💡 建议</h3>
        <ul class="rec-list">
          <li v-for="(rec, idx) in latestReport.recommendations" :key="idx">{{ rec }}</li>
        </ul>
      </div>

      <!-- 详细结果 (折叠) -->
      <div class="detail-section">
        <button class="btn-toggle" @click="showDetails = !showDetails">
          {{ showDetails ? '收起' : '展开' }}详细结果 ({{ latestReport.results?.length || 0 }} 条)
        </button>
        <div class="detail-table-wrap" v-if="showDetails && latestReport.results">
          <table class="data-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>名称</th>
                <th>分类</th>
                <th>期望</th>
                <th>实际</th>
                <th>结果</th>
                <th>匹配规则</th>
                <th>耗时</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in latestReport.results" :key="r.vector_id" :class="{'row-fail': !r.passed && r.expected !== 'pass'}">
                <td class="td-mono">{{ r.vector_id }}</td>
                <td>{{ r.name }}</td>
                <td>{{ categoryName(r.category) }}</td>
                <td><span class="action-badge" :class="'act-' + r.expected">{{ r.expected }}</span></td>
                <td><span class="action-badge" :class="'act-' + r.action">{{ r.action }}</span></td>
                <td><span :class="r.passed ? 'result-pass' : 'result-fail'">{{ r.passed ? '✅' : '❌' }}</span></td>
                <td class="td-mono">{{ r.matched_rule || '-' }}</td>
                <td class="td-mono">{{ r.latency_us }}μs</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- 无数据 -->
    <div class="empty-state" v-else-if="!loading">
      <div class="empty-icon">🎯</div>
      <div class="empty-text">尚未运行红队测试</div>
      <div class="empty-sub">点击「开始测试」运行第一次安全扫描</div>
    </div>

    <!-- 历史报告 -->
    <div class="history-section" v-if="reports.length > 1">
      <h2 class="section-title-lg"><Icon name="file-text" :size="16" /> 历史报告</h2>
      <table class="data-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>租户</th>
            <th>检测率</th>
            <th>通过</th>
            <th>漏洞</th>
            <th>耗时</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in reports.slice(1)" :key="r.id">
            <td>{{ formatTime(r.timestamp) }}</td>
            <td>{{ r.tenant_id }}</td>
            <td :style="{color: rateColor(r.pass_rate), fontWeight: 700}">{{ r.pass_rate.toFixed(1) }}%</td>
            <td>{{ r.passed }}/{{ r.total_tests }}</td>
            <td class="td-fail">{{ r.failed }}</td>
            <td>{{ r.duration_ms }}ms</td>
            <td class="td-actions">
              <button class="btn-sm" @click="viewReport(r.id)">查看</button>
              <button class="btn-sm btn-danger" @click="deleteReport(r.id)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import { useRouter } from 'vue-router'
import { api, apiPost, apiDelete } from '../api.js'

const router = useRouter()

const loading = ref(true)
const running = ref(false)
const reports = ref([])
const latestReport = ref(null)
const showDetails = ref(false)

const categoryNames = {
  prompt_injection: 'Prompt Injection',
  insecure_output: 'Insecure Output',
  sensitive_info: 'Sensitive Info',
  insecure_plugin: 'Insecure Plugin',
  overreliance: 'Overreliance',
  model_dos: 'Model DoS',
}

function categoryName(key) {
  return categoryNames[key] || key
}

function rateColor(rate) {
  if (rate >= 90) return '#10B981'
  if (rate >= 70) return '#F59E0B'
  if (rate >= 50) return '#EF4444'
  return '#DC2626'
}

function rateArc(rate) {
  const circumference = 2 * Math.PI * 50
  const filled = (rate / 100) * circumference
  return `${filled} ${circumference}`
}

function formatTime(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

async function loadReports() {
  try {
    const data = await api('/api/v1/redteam/reports?limit=20')
    reports.value = data.reports || []
    if (reports.value.length > 0) {
      // Load full detail of the latest report
      const detail = await api('/api/v1/redteam/reports/' + reports.value[0].id)
      latestReport.value = detail
    }
  } catch (e) {
    console.error('Load reports error:', e)
  } finally {
    loading.value = false
  }
}

async function runTest() {
  running.value = true
  try {
    const report = await apiPost('/api/v1/redteam/run', { tenant_id: 'default' })
    latestReport.value = report
    await loadReports()
    showDetails.value = false
  } catch (e) {
    alert('测试失败: ' + e.message)
  } finally {
    running.value = false
  }
}

async function viewReport(id) {
  try {
    const report = await api('/api/v1/redteam/reports/' + id)
    latestReport.value = report
    showDetails.value = false
    window.scrollTo(0, 0)
  } catch (e) {
    alert('加载报告失败: ' + e.message)
  }
}

async function deleteReport(id) {
  if (!confirm('确定删除此报告？')) return
  try {
    await apiDelete('/api/v1/redteam/reports/' + id)
    await loadReports()
  } catch (e) {
    alert('删除失败: ' + e.message)
  }
}

onMounted(loadReports)
</script>

<style scoped>
.redteam-page { max-width: 960px; margin: 0 auto; padding: var(--space-4); }

.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: var(--space-5);
}
.page-title { font-size: 1.5rem; font-weight: 800; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 2px; }

.btn { display: inline-flex; align-items: center; gap: 6px; padding: 10px 20px; border-radius: var(--radius-md); font-weight: 700; font-size: var(--text-sm); cursor: pointer; border: none; transition: all .2s; }
.btn-primary { background: var(--color-primary); color: #fff; }
.btn-primary:hover:not(:disabled) { filter: brightness(1.15); }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }

.spinner {
  display: inline-block; width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.3);
  border-top-color: #fff; border-radius: 50%; animation: spin .6s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* Report Card */
.report-card {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-5); margin-bottom: var(--space-5);
}
.report-card-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); }
.report-card-title { font-weight: 700; color: var(--text-primary); }
.report-card-time { font-size: var(--text-xs); color: var(--text-tertiary); font-family: var(--font-mono); }

/* Detection Rate */
.detection-rate { display: flex; align-items: center; gap: var(--space-6); margin-bottom: var(--space-5); flex-wrap: wrap; }
.rate-ring { position: relative; width: 120px; height: 120px; flex-shrink: 0; }
.rate-svg { width: 100%; height: 100%; }
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

/* Vulnerability List */
.vuln-section { margin-bottom: var(--space-5); }
.vuln-list { display: flex; flex-direction: column; gap: var(--space-2); }
.vuln-item {
  background: var(--bg-elevated); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md); padding: var(--space-3);
  border-left: 3px solid #EF4444;
}
.vuln-header { display: flex; align-items: center; gap: var(--space-2); margin-bottom: 4px; }
.vuln-id { font-family: var(--font-mono); font-size: var(--text-xs); color: var(--text-tertiary); }
.vuln-name { font-weight: 600; font-size: var(--text-sm); color: var(--text-primary); flex: 1; }
.severity-badge {
  display: inline-block; padding: 1px 8px; border-radius: 9999px;
  font-size: 10px; font-weight: 700; text-transform: uppercase;
}
.sev-critical { background: #DC2626; color: #fff; }
.sev-high { background: #EF4444; color: #fff; }
.sev-medium { background: #F59E0B; color: #1a1a2e; }
.sev-low { background: #6B7280; color: #fff; }
.vuln-desc { font-size: var(--text-xs); color: var(--text-secondary); margin-bottom: 4px; }
.vuln-suggestion { font-size: var(--text-xs); color: #F59E0B; background: rgba(245,158,11,0.08); padding: 4px 8px; border-radius: var(--radius-sm); }

/* Recommendations */
.rec-section { margin-bottom: var(--space-5); }
.rec-list { list-style: none; padding: 0; display: flex; flex-direction: column; gap: var(--space-2); }
.rec-list li {
  padding: var(--space-2) var(--space-3); background: rgba(245,158,11,0.06);
  border-left: 3px solid #F59E0B; border-radius: 0 var(--radius-md) var(--radius-md) 0;
  font-size: var(--text-sm); color: var(--text-secondary);
}

/* Detail Toggle */
.detail-section { margin-top: var(--space-3); }
.btn-toggle {
  background: transparent; border: 1px solid var(--border-subtle); color: var(--text-secondary);
  padding: 6px 14px; border-radius: var(--radius-md); font-size: var(--text-xs); cursor: pointer;
  transition: all .2s;
}
.btn-toggle:hover { background: var(--bg-elevated); color: var(--text-primary); }
.detail-table-wrap { margin-top: var(--space-3); overflow-x: auto; }

/* Data Table */
.data-table { width: 100%; border-collapse: collapse; font-size: var(--text-xs); }
.data-table th {
  text-align: left; padding: 8px 10px; background: var(--bg-elevated);
  color: var(--text-tertiary); font-weight: 600; font-size: 10px;
  text-transform: uppercase; letter-spacing: .05em; border-bottom: 2px solid var(--border-subtle);
  white-space: nowrap;
}
.data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-secondary); }
.data-table tr:hover { background: var(--bg-elevated); }
.row-fail { background: rgba(239,68,68,0.05); }
.td-mono { font-family: var(--font-mono); font-size: 11px; }
.td-fail { color: #EF4444; font-weight: 700; }
.td-actions { display: flex; gap: 4px; }

.action-badge {
  display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 700;
}
.act-block { background: #EF4444; color: #fff; }
.act-warn { background: #F59E0B; color: #1a1a2e; }
.act-pass { background: #10B981; color: #fff; }
.act-log { background: #6B7280; color: #fff; }

.result-pass { color: #10B981; }
.result-fail { color: #EF4444; }

/* Small Buttons */
.btn-sm {
  padding: 3px 10px; border-radius: var(--radius-sm); font-size: 11px; font-weight: 600;
  cursor: pointer; border: 1px solid var(--border-subtle); background: transparent;
  color: var(--text-secondary); transition: all .2s;
}
.btn-sm:hover { background: var(--bg-elevated); color: var(--text-primary); }
.btn-danger { border-color: rgba(239,68,68,.3); color: #EF4444; }
.btn-danger:hover { background: rgba(239,68,68,.1); }

/* Empty State */
.empty-state { text-align: center; padding: var(--space-8) 0; }
.empty-icon { font-size: 3rem; margin-bottom: var(--space-3); }
.empty-text { font-size: var(--text-base); font-weight: 700; color: var(--text-primary); }
.empty-sub { font-size: var(--text-sm); color: var(--text-tertiary); margin-top: 4px; }

/* History */
.history-section { margin-top: var(--space-5); }
.section-title-lg { font-size: var(--text-base); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-3); }

@media(max-width:600px) {
  .detection-rate { flex-direction: column; align-items: center; }
  .rate-stats { justify-content: center; }
  .page-header { flex-direction: column; gap: var(--space-3); align-items: flex-start; }
}
.vuln-fix-link{display:inline-block;margin-top:6px;color:var(--color-primary);cursor:pointer;font-size:12px;text-decoration:none}.vuln-fix-link:hover{text-decoration:underline}
</style>
