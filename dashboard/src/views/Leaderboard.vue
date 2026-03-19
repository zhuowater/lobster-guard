<template>
  <div class="leaderboard-page">
    <!-- 标题栏 -->
    <div class="page-header">
      <div class="header-left">
        <h1 class="page-title"><Icon name="trophy" :size="20" /> 安全排行榜</h1>
        <span class="page-subtitle">租户安全态势全局视图</span>
      </div>
      <div class="header-right">
        <div class="sla-baseline" v-if="slaConfig">
          <span class="sla-tag">SLA 基线:</span>
          <span class="sla-item">健康≥{{ slaConfig.min_health_score }}</span>
          <span class="sla-sep">|</span>
          <span class="sla-item">事件≤{{ slaConfig.max_incident_count }}</span>
          <span class="sla-sep">|</span>
          <span class="sla-item">检测≥{{ slaConfig.min_redteam_score }}%</span>
        </div>
        <button class="btn btn-sm" @click="loadAll" :disabled="loading">
          <span v-if="loading">刷新中...</span>
          <span v-else><Icon name="refresh" :size="14" /> 刷新</span>
        </button>
      </div>
    </div>

    <!-- SLA 汇总 -->
    <div class="sla-summary" v-if="slaOverview">
      <div class="sla-card sla-green">
        <div class="sla-count">{{ slaOverview.summary.green }}</div>
        <div class="sla-label">✅ 达标</div>
      </div>
      <div class="sla-card sla-yellow">
        <div class="sla-count">{{ slaOverview.summary.yellow }}</div>
        <div class="sla-label">⚠️ 警告</div>
      </div>
      <div class="sla-card sla-red">
        <div class="sla-count">{{ slaOverview.summary.red }}</div>
        <div class="sla-label">❌ 未达标</div>
      </div>
      <div class="sla-card sla-total">
        <div class="sla-count">{{ slaOverview.summary.total }}</div>
        <div class="sla-label"><Icon name="bar-chart" :size="14" /> 总计</div>
      </div>
    </div>

    <!-- 排行榜列表 -->
    <div class="section">
      <h2 class="section-title">排行榜</h2>
      <div class="leaderboard-list">
        <div
          v-for="score in scores" :key="score.tenant_id"
          class="leaderboard-card"
          :class="'sla-border-' + score.sla_status"
        >
          <div class="card-header">
            <div class="rank-badge">
              <span class="rank-num">#{{ score.rank }}</span>
              <span class="rank-medal">{{ rankMedal(score.rank) }}</span>
            </div>
            <a class="tenant-name link-accent" @click.stop="$router.push('/tenants')">{{ score.tenant_name }}</a>
            <div class="card-stats">
              <span class="stat" title="健康分">
                <span class="stat-label">健康</span>
                <span class="stat-value" :class="healthClass(score.health_score)">{{ score.health_score }}</span>
              </span>
              <span class="stat" title="红队检测率">
                <span class="stat-label">检测</span>
                <span class="stat-value">{{ score.redteam_score > 0 ? score.redteam_score.toFixed(1) + '%' : 'N/A' }}</span>
              </span>
              <span class="stat" title="近7天安全事件数">
                <span class="stat-label">事件</span>
                <span class="stat-value" :class="incidentClass(score.incident_count)">{{ score.incident_count }}</span>
              </span>
              <span class="stat" title="拦截率">
                <span class="stat-label">拦截</span>
                <span class="stat-value">{{ score.block_rate.toFixed(1) }}%</span>
              </span>
              <span class="stat sla-badge" :class="'sla-' + score.sla_status" title="SLA 状态">
                {{ slaIcon(score.sla_status) }} SLA
              </span>
              <span class="stat trend-badge" :title="'趋势: ' + score.trend">
                {{ trendIcon(score.trend) }}
              </span>
            </div>
          </div>
          <div class="progress-bar-container">
            <div class="progress-bar" :style="{ width: score.health_score + '%' }" :class="progressClass(score.health_score)">
            </div>
            <span class="progress-label">{{ score.health_score }}/100</span>
          </div>
        </div>
        <div v-if="scores.length === 0 && !loading" class="empty-state">
          暂无排行数据，请先注入演示数据
        </div>
      </div>
    </div>

    <!-- 攻击热力图 -->
    <div class="section">
      <h2 class="section-title">🔥 攻击热力图 <span class="section-sub">近7天 · 租户×OWASP分类</span></h2>
      <div class="heatmap-container" v-if="heatmapData.tenants.length > 0">
        <table class="heatmap-table">
          <thead>
            <tr>
              <th class="heatmap-tenant-header">租户</th>
              <th v-for="cat in heatmapData.categories" :key="cat" class="heatmap-cat-header" :title="catFullName(cat)">{{ cat }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="tenant in heatmapData.tenants" :key="tenant.id">
              <td class="heatmap-tenant-name">{{ tenant.name }}</td>
              <td v-for="cat in heatmapData.categories" :key="cat"
                class="heatmap-cell"
                :class="'intensity-' + getCellIntensity(tenant.id, cat)"
                :title="getCellCount(tenant.id, cat) + ' 次攻击'"
              >
                {{ getCellCount(tenant.id, cat) || '' }}
              </td>
            </tr>
          </tbody>
        </table>
        <div class="heatmap-legend">
          <span class="legend-label">少</span>
          <span class="legend-cell intensity-none"></span>
          <span class="legend-cell intensity-low"></span>
          <span class="legend-cell intensity-medium"></span>
          <span class="legend-cell intensity-high"></span>
          <span class="legend-cell intensity-critical"></span>
          <span class="legend-label">多</span>
        </div>
      </div>
      <div v-else-if="!loading" class="empty-state">暂无热力图数据</div>
    </div>

    <!-- SLA 配置面板 -->
    <div class="section">
      <h2 class="section-title"><Icon name="settings" :size="16" /> SLA 配置</h2>
      <div class="sla-config-panel">
        <div class="config-row">
          <label>最低健康分</label>
          <input type="number" v-model.number="editConfig.min_health_score" min="0" max="100" class="config-input" />
        </div>
        <div class="config-row">
          <label>7天最多事件数</label>
          <input type="number" v-model.number="editConfig.max_incident_count" min="0" class="config-input" />
        </div>
        <div class="config-row">
          <label>红队最低检测率 (%)</label>
          <input type="number" v-model.number="editConfig.min_redteam_score" min="0" max="100" step="0.1" class="config-input" />
        </div>
        <div class="config-row">
          <label>最低拦截率</label>
          <input type="number" v-model.number="editConfig.min_block_rate" min="0" max="1" step="0.01" class="config-input" />
        </div>
        <button class="btn btn-primary" @click="saveSLAConfig" :disabled="saving">
          {{ saving ? '保存中...' : '保存配置' }}
        </button>
      </div>

      <!-- SLA 达标详情 -->
      <div class="sla-detail" v-if="slaOverview && slaOverview.tenants.length > 0">
        <h3 class="subsection-title">各租户 SLA 达标情况</h3>
        <table class="sla-table">
          <thead>
            <tr>
              <th>租户</th>
              <th>健康分</th>
              <th>事件数</th>
              <th>检测率</th>
              <th>SLA</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in slaOverview.tenants" :key="t.tenant_id">
              <td>{{ t.tenant_name }}</td>
              <td :class="t.health_met ? 'met' : 'unmet'">{{ t.health_score }} {{ t.health_met ? '✅' : '❌' }}</td>
              <td :class="t.incident_met ? 'met' : 'unmet'">{{ t.incident_count }} {{ t.incident_met ? '✅' : '❌' }}</td>
              <td :class="t.redteam_met ? 'met' : 'unmet'">{{ t.redteam_score > 0 ? t.redteam_score.toFixed(1) + '%' : 'N/A' }} {{ t.redteam_met ? '✅' : '❌' }}</td>
              <td><span class="sla-pill" :class="'sla-' + t.sla_status">{{ slaIcon(t.sla_status) }} {{ t.sla_status.toUpperCase() }}</span></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import Icon from '../components/Icon.vue'
import { api, apiPut } from '../api.js'

const loading = ref(false)
const saving = ref(false)
const scores = ref([])
const slaConfig = ref(null)
const slaOverview = ref(null)
const heatmapCells = ref([])
const heatmapCategories = ref([])

const editConfig = reactive({
  min_health_score: 70,
  max_incident_count: 10,
  min_redteam_score: 80,
  min_block_rate: 0,
})

const heatmapData = reactive({
  tenants: [],
  categories: [],
})

function rankMedal(rank) {
  if (rank === 1) return '🥇'
  if (rank === 2) return '🥈'
  if (rank === 3) return '🥉'
  return ''
}

function slaIcon(status) {
  if (status === 'green') return '✅'
  if (status === 'yellow') return '⚠️'
  return '❌'
}

function trendIcon(trend) {
  if (trend === 'up') return '📈'
  if (trend === 'down') return '📉'
  return '➡️'
}

function healthClass(score) {
  if (score >= 90) return 'health-excellent'
  if (score >= 70) return 'health-good'
  if (score >= 50) return 'health-warning'
  return 'health-danger'
}

function incidentClass(count) {
  if (count <= 5) return 'incident-low'
  if (count <= 10) return 'incident-medium'
  return 'incident-high'
}

function progressClass(score) {
  if (score >= 90) return 'progress-excellent'
  if (score >= 70) return 'progress-good'
  if (score >= 50) return 'progress-warning'
  return 'progress-danger'
}

const catNames = {
  PI: 'Prompt Injection',
  IO: 'Insecure Output',
  SI: 'Sensitive Info',
  IP: 'Insecure Plugin',
  OR: 'Overreliance',
  MD: 'Model DoS',
}
function catFullName(cat) { return catNames[cat] || cat }

function getCellIntensity(tenantId, cat) {
  const cell = heatmapCells.value.find(c => c.tenant_id === tenantId && c.category === cat)
  return cell ? cell.intensity : 'none'
}
function getCellCount(tenantId, cat) {
  const cell = heatmapCells.value.find(c => c.tenant_id === tenantId && c.category === cat)
  return cell ? cell.count : 0
}

async function loadLeaderboard() {
  try {
    const d = await api('/api/v1/leaderboard')
    scores.value = d.scores || []
    slaConfig.value = d.sla || null
    if (d.sla) {
      editConfig.min_health_score = d.sla.min_health_score
      editConfig.max_incident_count = d.sla.max_incident_count
      editConfig.min_redteam_score = d.sla.min_redteam_score
      editConfig.min_block_rate = d.sla.min_block_rate || 0
    }
  } catch (e) {
    console.error('加载排行榜失败:', e)
  }
}

async function loadHeatmap() {
  try {
    const d = await api('/api/v1/leaderboard/heatmap')
    heatmapCells.value = d.cells || []
    heatmapCategories.value = d.categories || []
    heatmapData.categories = d.categories || []
    // 提取唯一租户
    const tenantMap = {}
    for (const c of heatmapCells.value) {
      if (!tenantMap[c.tenant_id]) {
        const score = scores.value.find(s => s.tenant_id === c.tenant_id)
        tenantMap[c.tenant_id] = { id: c.tenant_id, name: score ? score.tenant_name : c.tenant_id }
      }
    }
    heatmapData.tenants = Object.values(tenantMap)
  } catch (e) {
    console.error('加载热力图失败:', e)
  }
}

async function loadSLA() {
  try {
    const d = await api('/api/v1/leaderboard/sla')
    slaOverview.value = d
  } catch (e) {
    console.error('加载 SLA 失败:', e)
  }
}

async function loadAll() {
  loading.value = true
  try {
    await loadLeaderboard()
    await Promise.all([loadHeatmap(), loadSLA()])
  } finally {
    loading.value = false
  }
}

async function saveSLAConfig() {
  saving.value = true
  try {
    await apiPut('/api/v1/leaderboard/sla/config', {
      min_health_score: editConfig.min_health_score,
      max_incident_count: editConfig.max_incident_count,
      min_redteam_score: editConfig.min_redteam_score,
      min_block_rate: editConfig.min_block_rate,
    })
    await loadAll()
  } catch (e) {
    alert('保存失败: ' + e.message)
  } finally {
    saving.value = false
  }
}

onMounted(loadAll)
</script>

<style scoped>
.leaderboard-page {
  padding: var(--space-6);
  max-width: 1200px;
}

/* 头部 */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: var(--space-6);
  flex-wrap: wrap;
  gap: var(--space-3);
}
.header-left { display: flex; align-items: baseline; gap: var(--space-3); }
.page-title { font-size: 1.5rem; font-weight: 700; color: var(--text-primary); margin: 0; }
.page-subtitle { font-size: var(--text-sm); color: var(--text-tertiary); }
.header-right { display: flex; align-items: center; gap: var(--space-3); flex-wrap: wrap; }
.sla-baseline {
  display: flex; align-items: center; gap: var(--space-2);
  background: var(--bg-elevated); padding: 6px 12px; border-radius: var(--radius-md);
  font-size: var(--text-xs); color: var(--text-secondary);
}
.sla-tag { font-weight: 700; color: var(--color-primary); }
.sla-sep { color: var(--text-tertiary); }
.sla-item { font-family: var(--font-mono); }

.btn { padding: 6px 14px; border-radius: var(--radius-md); border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-primary); cursor: pointer; font-size: var(--text-xs); transition: all .15s; }
.btn:hover { background: var(--bg-surface); border-color: var(--color-primary); }
.btn-sm { font-size: 11px; padding: 4px 10px; }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover { opacity: .9; }
.btn:disabled { opacity: .5; cursor: not-allowed; }

/* SLA 汇总卡片 */
.sla-summary {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-3);
  margin-bottom: var(--space-6);
}
.sla-card {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-4); text-align: center;
}
.sla-count { font-size: 2rem; font-weight: 800; font-family: var(--font-mono); }
.sla-label { font-size: var(--text-xs); color: var(--text-secondary); margin-top: var(--space-1); }
.sla-green .sla-count { color: #22C55E; }
.sla-yellow .sla-count { color: #EAB308; }
.sla-red .sla-count { color: #EF4444; }
.sla-total .sla-count { color: var(--color-primary); }

/* Section */
.section { margin-bottom: var(--space-6); }
.section-title { font-size: 1.1rem; font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-4); }
.section-sub { font-size: var(--text-xs); color: var(--text-tertiary); font-weight: 400; }

/* 排行榜卡片 */
.leaderboard-list { display: flex; flex-direction: column; gap: var(--space-3); }
.leaderboard-card {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-4);
  transition: all .2s;
}
.leaderboard-card:hover { border-color: var(--color-primary); box-shadow: var(--shadow-md); }
.sla-border-green { border-left: 4px solid #22C55E; }
.sla-border-yellow { border-left: 4px solid #EAB308; }
.sla-border-red { border-left: 4px solid #EF4444; }

.card-header { display: flex; align-items: center; gap: var(--space-3); margin-bottom: var(--space-3); flex-wrap: wrap; }
.rank-badge { display: flex; align-items: center; gap: var(--space-1); }
.rank-num { font-size: 1.1rem; font-weight: 800; color: var(--color-primary); font-family: var(--font-mono); }
.rank-medal { font-size: 1.2rem; }
.tenant-name { font-size: var(--text-base); font-weight: 600; color: var(--text-primary); }
.card-stats { display: flex; align-items: center; gap: var(--space-3); margin-left: auto; flex-wrap: wrap; }
.stat { display: flex; align-items: center; gap: 4px; font-size: var(--text-xs); }
.stat-label { color: var(--text-tertiary); }
.stat-value { font-family: var(--font-mono); font-weight: 600; }

.health-excellent { color: #22C55E; }
.health-good { color: #3B82F6; }
.health-warning { color: #EAB308; }
.health-danger { color: #EF4444; }
.incident-low { color: #22C55E; }
.incident-medium { color: #EAB308; }
.incident-high { color: #EF4444; }

.sla-badge { padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: 700; }
.sla-green { background: rgba(34,197,94,.15); color: #22C55E; }
.sla-yellow { background: rgba(234,179,8,.15); color: #EAB308; }
.sla-red { background: rgba(239,68,68,.15); color: #EF4444; }

.trend-badge { font-size: 1rem; }

/* 进度条 */
.progress-bar-container {
  position: relative; height: 24px; background: var(--bg-elevated);
  border-radius: var(--radius-md); overflow: hidden;
}
.progress-bar {
  height: 100%; border-radius: var(--radius-md); transition: width .6s ease;
}
.progress-excellent { background: linear-gradient(90deg, #22C55E, #16A34A); }
.progress-good { background: linear-gradient(90deg, #3B82F6, #2563EB); }
.progress-warning { background: linear-gradient(90deg, #EAB308, #CA8A04); }
.progress-danger { background: linear-gradient(90deg, #EF4444, #DC2626); }
.progress-label {
  position: absolute; right: 8px; top: 50%; transform: translateY(-50%);
  font-size: 11px; font-weight: 700; color: var(--text-primary);
  font-family: var(--font-mono);
}

/* 热力图 */
.heatmap-container { overflow-x: auto; }
.heatmap-table {
  width: 100%; border-collapse: collapse;
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); overflow: hidden;
}
.heatmap-table th, .heatmap-table td {
  padding: 10px 16px; text-align: center; font-size: var(--text-xs);
}
.heatmap-table thead th {
  background: var(--bg-elevated); color: var(--text-secondary);
  font-weight: 700; border-bottom: 1px solid var(--border-subtle);
}
.heatmap-tenant-header { text-align: left; }
.heatmap-tenant-name { text-align: left; font-weight: 600; color: var(--text-primary); white-space: nowrap; }
.heatmap-cell {
  width: 60px; height: 40px; border-radius: 4px; font-weight: 700;
  font-family: var(--font-mono); transition: all .2s;
}
.intensity-none { background: rgba(34,197,94,.08); color: transparent; }
.intensity-low { background: rgba(34,197,94,.25); color: #22C55E; }
.intensity-medium { background: rgba(234,179,8,.35); color: #CA8A04; }
.intensity-high { background: rgba(249,115,22,.45); color: #EA580C; }
.intensity-critical { background: rgba(239,68,68,.55); color: #DC2626; }

.heatmap-legend {
  display: flex; align-items: center; gap: 4px; margin-top: var(--space-3);
  justify-content: flex-end;
}
.legend-label { font-size: 10px; color: var(--text-tertiary); }
.legend-cell {
  width: 20px; height: 14px; border-radius: 2px;
}

/* SLA 配置面板 */
.sla-config-panel {
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); padding: var(--space-4);
  display: flex; flex-wrap: wrap; gap: var(--space-4); align-items: flex-end;
}
.config-row { display: flex; flex-direction: column; gap: 4px; }
.config-row label { font-size: var(--text-xs); color: var(--text-secondary); font-weight: 600; }
.config-input {
  width: 120px; padding: 6px 10px; background: var(--bg-elevated);
  border: 1px solid var(--border-subtle); border-radius: var(--radius-md);
  color: var(--text-primary); font-family: var(--font-mono); font-size: var(--text-sm);
}
.config-input:focus { outline: none; border-color: var(--color-primary); }

/* SLA 达标详情表 */
.subsection-title { font-size: var(--text-sm); font-weight: 600; color: var(--text-primary); margin: var(--space-4) 0 var(--space-3); }
.sla-detail { margin-top: var(--space-4); }
.sla-table {
  width: 100%; border-collapse: collapse;
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); overflow: hidden;
}
.sla-table th, .sla-table td {
  padding: 8px 14px; text-align: left; font-size: var(--text-xs);
  border-bottom: 1px solid var(--border-subtle);
}
.sla-table thead th { background: var(--bg-elevated); font-weight: 700; color: var(--text-secondary); }
.met { color: #22C55E; }
.unmet { color: #EF4444; }
.sla-pill {
  display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 10px; font-weight: 700;
}

.empty-state {
  text-align: center; padding: var(--space-6); color: var(--text-tertiary);
  font-size: var(--text-sm);
}

@media (max-width: 768px) {
  .sla-summary { grid-template-columns: repeat(2, 1fr); }
  .card-stats { margin-left: 0; }
  .sla-config-panel { flex-direction: column; }
}
.link-accent{color:var(--color-primary);cursor:pointer;text-decoration:none}.link-accent:hover{text-decoration:underline}
</style>
