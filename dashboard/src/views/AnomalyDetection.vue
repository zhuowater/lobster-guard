<template>
  <div>
    <div class="page-header">
      <h2><Icon name="bar-chart" :size="18" /> 异常基线检测</h2>
      <p class="page-desc">连续运行 {{ anomalyConfig.min_ready_days || 3 }} 天后自动建立正常行为基线，偏离 >{{ anomalyConfig.warning_threshold || 2 }}σ 自动告警</p>
    </div>

    <!-- 顶部 StatCard 行 -->
    <div class="ov-cards" v-if="loaded">
      <StatCard :iconSvg="svgWave" :value="status.metrics_count||0" label="监控指标" color="blue"/>
      <StatCard :iconSvg="svgCheck" :value="status.baselines_ready||0" label="基线就绪" color="green"/>
      <StatCard :iconSvg="svgAlert" :value="status.alerts_24h||0" label="24h 异常告警" color="red"/>
      <StatCard :iconSvg="svgPulse" :value="status.enabled?'运行中':'未启用'" label="检测器状态" :color="status.enabled?'green':'yellow'"/>
    </div>
    <div class="ov-cards" v-else><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>

    <!-- 基线状态网格 -->
    <div class="section-title">基线状态</div>
    <div class="baseline-grid" v-if="loaded">
      <div class="baseline-card" :class="{ 'bc-anomaly-active': m.anomaly }" v-for="m in metricsData" :key="m.metric_name" @click="onBaselineCardClick(m)" style="cursor:pointer">
        <div class="bc-header">
          <span class="bc-icon"><Icon name="bar-chart" :size="16" /></span>
          <span class="bc-name">{{ metricDisplayName(m.metric_name) }}</span>
        </div>
        <div class="bc-body" v-if="m.baseline && m.baseline.ready">
          <div class="bc-row"><span class="bc-label">基线状态</span><span class="bc-badge bc-ready">✅ 就绪</span></div>
          <div class="bc-row"><span class="bc-label">样本数</span><span class="bc-val">{{ m.baseline.sample_count }}h ({{ Math.round(m.baseline.sample_count/24) }}天)</span></div>
          <div class="bc-row"><span class="bc-label">当前小时均值</span><span class="bc-val">{{ formatNum(m.baseline.hourly_mean[currentHour]) }}</span></div>
          <div class="bc-row"><span class="bc-label">当前小时标准差</span><span class="bc-val">{{ formatNum(m.baseline.hourly_std[currentHour]) }}</span></div>
          <div class="bc-row">
            <span class="bc-label">当前值</span>
            <span class="bc-val" :class="m.anomaly?'anomaly-val':'normal-val'">{{ formatNum(m.current_value) }} {{ m.anomaly?'⚠️ 异常':'✅ 正常' }}</span>
          </div>
          <!-- 24 小时基线图 -->
          <div class="bc-chart">
            <svg viewBox="0 0 360 100" class="baseline-svg" preserveAspectRatio="none">
              <!-- ±2σ 灰色带 -->
              <polygon :points="sigma2BandPoints(m.baseline)" fill="rgba(99,102,241,0.12)" stroke="none"/>
              <!-- 均值线 -->
              <polyline :points="meanLinePoints(m.baseline)" fill="none" stroke="#6366F1" stroke-width="2" stroke-linejoin="round"/>
              <!-- 当前值点 -->
              <circle :cx="15*currentHour" :cy="mapY(m.current_value, m.baseline)" :r="4" :fill="m.anomaly?'#EF4444':'#10B981'" stroke="#fff" stroke-width="1.5"/>
            </svg>
            <div class="bc-chart-labels"><span>0h</span><span>6h</span><span>12h</span><span>18h</span><span>23h</span></div>
          </div>
        </div>
        <div class="bc-body bc-learning" v-else>
          <div class="bc-learning-icon"><Icon name="radio" :size="20" /></div>
          <div class="bc-learning-text">数据收集中...</div>
          <div class="bc-learning-sub">需要至少 3 天数据建立基线</div>
          <div class="bc-progress-wrap">
            <div class="bc-progress-bar"><div class="bc-progress-fill" :style="{width: learningProgress(m)+'%'}"></div></div>
            <span class="bc-progress-text">{{ learningProgress(m) }}%</span>
          </div>
        </div>
      </div>
    </div>
    <div v-else class="baseline-grid"><Skeleton type="card" v-for="i in 6" :key="i"/></div>

    <!-- 检测参数配置 -->
    <div class="section-title" style="margin-top:24px">
      检测参数
      <button class="btn btn-sm btn-secondary" @click="showConfig = !showConfig" style="margin-left:auto;font-size:12px">
        {{ showConfig ? '收起' : '展开配置' }}
      </button>
    </div>
    <div v-if="showConfig" class="config-panel card">
      <div class="config-grid">
        <div class="config-item">
          <label>基线窗口 (天)</label>
          <input type="number" v-model.number="anomalyConfig.window_days" min="1" max="90" class="config-input"/>
        </div>
        <div class="config-item">
          <label>告警阈值 (σ)</label>
          <input type="number" v-model.number="anomalyConfig.warning_threshold" min="0.5" max="10" step="0.1" class="config-input"/>
        </div>
        <div class="config-item">
          <label>严重阈值 (σ)</label>
          <input type="number" v-model.number="anomalyConfig.critical_threshold" min="1" max="20" step="0.1" class="config-input"/>
        </div>
        <div class="config-item">
          <label>最小标准差</label>
          <input type="number" v-model.number="anomalyConfig.min_std_dev" min="0.1" max="100" step="0.1" class="config-input"/>
        </div>
        <div class="config-item">
          <label>基线就绪最少天数</label>
          <input type="number" v-model.number="anomalyConfig.min_ready_days" min="1" max="30" class="config-input"/>
        </div>
        <div class="config-item">
          <label>基线更新间隔 (分钟)</label>
          <input type="number" v-model.number="anomalyConfig.baseline_interval_min" min="1" max="1440" class="config-input"/>
        </div>
        <div class="config-item">
          <label>异常检查间隔 (分钟)</label>
          <input type="number" v-model.number="anomalyConfig.check_interval_min" min="1" max="1440" class="config-input"/>
        </div>
        <div class="config-item">
          <label>最大告警数</label>
          <input type="number" v-model.number="anomalyConfig.max_alerts" min="10" max="10000" class="config-input"/>
        </div>
      </div>
      <div class="config-actions">
        <button class="btn btn-primary" @click="saveConfig" :disabled="configSaving">{{ configSaving ? '保存中...' : '保存配置' }}</button>
        <button class="btn btn-secondary" @click="resetConfig">恢复默认</button>
        <span v-if="configMsg" class="config-msg" :class="configMsgType">{{ configMsg }}</span>
      </div>
    </div>

    <!-- 异常告警列表 -->
    <div class="section-title" style="margin-top:24px">异常告警</div>
    <div class="card" v-if="loaded">
      <EmptyState v-if="!alerts.length" :iconSvg="svgShieldOk" title="未检测到异常" description="所有指标在正常基线范围内"/>
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr><th>时间</th><th>指标</th><th>期望值</th><th>实际值</th><th>偏离</th><th>方向</th><th>严重度</th></tr>
          </thead>
          <tbody>
            <tr v-for="a in alerts" :key="a.id" :class="'row-'+a.severity">
              <td>{{ fmtTime(a.timestamp) }}</td>
              <td><a class="metric-link" @click.stop="onAlertMetricClick(a.metric_name)">{{ metricDisplayName(a.metric_name) }}</a></td>
              <td class="mono">{{ formatNum(a.expected) }}</td>
              <td class="mono fw-bold">{{ formatNum(a.actual) }}</td>
              <td>
                <div class="deviation-cell">
                  <div class="deviation-bar-wrap">
                    <div class="deviation-bar" :style="{width: Math.min(100, a.deviation/5*100)+'%', background: a.severity==='critical'?'#EF4444':'#F59E0B'}"></div>
                    <div class="deviation-mark-2s" title="2σ"></div>
                    <div class="deviation-mark-3s" title="3σ"></div>
                  </div>
                  <span class="deviation-text">{{ a.deviation.toFixed(1) }}σ</span>
                </div>
              </td>
              <td><span :class="'dir-'+a.direction">{{ a.direction==='above'?'↑ 高于':'↓ 低于' }}</span></td>
              <td><span class="severity-badge" :class="'sev-'+a.severity">{{ a.severity==='critical'?'🔴 critical':'🟡 warning' }}</span></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <Skeleton v-else type="table"/>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { api, apiPut } from '../api.js'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'

const router = useRouter()

const svgWave='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M2 12h2l3-7 4 14 4-10 3 3h4"/></svg>'
const svgCheck='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>'
const svgAlert='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgPulse='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgShieldOk='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>'

const loaded = ref(false)
const status = ref({})
const baselines = ref({})
const alerts = ref([])
const metricsData = ref([])
const currentHour = new Date().getUTCHours()

// 配置面板
const showConfig = ref(false)
const configSaving = ref(false)
const configMsg = ref('')
const configMsgType = ref('success')
const defaultConfig = { window_days: 7, warning_threshold: 2.0, critical_threshold: 3.0, min_std_dev: 1.0, min_ready_days: 3, baseline_interval_min: 60, check_interval_min: 5, max_alerts: 100 }
const anomalyConfig = ref({ ...defaultConfig })

async function loadConfig() {
  try { anomalyConfig.value = await api('/api/v1/anomaly/config') } catch { anomalyConfig.value = { ...defaultConfig } }
}
async function saveConfig() {
  configSaving.value = true; configMsg.value = ''
  try {
    const d = await apiPut('/api/v1/anomaly/config', anomalyConfig.value)
    anomalyConfig.value = d.config || anomalyConfig.value
    configMsg.value = '✅ 配置已保存'; configMsgType.value = 'success'
  } catch (e) { configMsg.value = '❌ ' + e.message; configMsgType.value = 'error' }
  configSaving.value = false
  setTimeout(() => { configMsg.value = '' }, 3000)
}
function resetConfig() { anomalyConfig.value = { ...defaultConfig } }

const metricNames = [
  'im_requests_per_hour', 'im_blocks_per_hour', 'llm_calls_per_hour',
  'llm_tokens_per_hour', 'tool_calls_per_hour', 'high_risk_tools_per_hour'
]
const metricLabels = {
  'im_requests_per_hour': 'IM 每小时请求数',
  'im_blocks_per_hour': 'IM 每小时拦截数',
  'llm_calls_per_hour': 'LLM 每小时调用数',
  'llm_tokens_per_hour': 'LLM 每小时 Token 消耗',
  'tool_calls_per_hour': '每小时工具调用数',
  'high_risk_tools_per_hour': '每小时高危工具调用数',
}
function metricDisplayName(name) { return metricLabels[name] || name }

// v11.3: metric → navigation target
const metricRouteMap = {
  'im_requests_per_hour': '/audit',
  'im_blocks_per_hour': '/audit',
  'llm_calls_per_hour': '/agent',
  'llm_tokens_per_hour': '/agent',
  'tool_calls_per_hour': '/agent',
  'high_risk_tools_per_hour': '/agent',
}
function onBaselineCardClick(m) {
  const target = metricRouteMap[m.metric_name]
  if (target) router.push(target)
}
function onAlertMetricClick(metricName) {
  const target = metricRouteMap[metricName]
  if (target) router.push(target)
}
function formatNum(v) { if (v == null) return '--'; return typeof v === 'number' ? (v >= 1000 ? (v/1000).toFixed(1)+'k' : Number(v.toFixed(1)).toString()) : String(v) }
function fmtTime(ts) { if (!ts) return '--'; const d = new Date(ts); return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false }) }

function learningProgress(m) {
  if (!m.baseline) return 0
  const samples = m.baseline.sample_count || 0
  return Math.min(100, Math.round(samples / (3 * 24) * 100))
}

// SVG 基线图 helpers
function getMinMax(baseline) {
  let min = Infinity, max = -Infinity
  for (let h = 0; h < 24; h++) {
    const lo = (baseline.hourly_mean[h] || 0) - 2 * (baseline.hourly_std[h] || 1)
    const hi = (baseline.hourly_mean[h] || 0) + 2 * (baseline.hourly_std[h] || 1)
    if (lo < min) min = lo
    if (hi > max) max = hi
  }
  if (min === max) { min = 0; max = 10 }
  return { min: Math.max(0, min - (max-min)*0.1), max: max + (max-min)*0.1 }
}

function mapY(val, baseline) {
  const { min, max } = getMinMax(baseline)
  const pct = (val - min) / (max - min)
  return 90 - pct * 80
}

function meanLinePoints(baseline) {
  const { min, max } = getMinMax(baseline)
  let pts = []
  for (let h = 0; h < 24; h++) {
    const x = h * 15
    const y = 90 - ((baseline.hourly_mean[h] || 0) - min) / (max - min) * 80
    pts.push(`${x},${y}`)
  }
  return pts.join(' ')
}

function sigma2BandPoints(baseline) {
  const { min, max } = getMinMax(baseline)
  let upper = [], lower = []
  for (let h = 0; h < 24; h++) {
    const x = h * 15
    const mean = baseline.hourly_mean[h] || 0
    const std = baseline.hourly_std[h] || 1
    const hi = 90 - (mean + 2 * std - min) / (max - min) * 80
    const lo = 90 - (Math.max(0, mean - 2 * std) - min) / (max - min) * 80
    upper.push(`${x},${hi}`)
    lower.unshift(`${x},${lo}`)
  }
  return [...upper, ...lower].join(' ')
}

async function loadData() {
  try { status.value = await api('/api/v1/anomaly/status') } catch { status.value = { enabled: false } }
  try {
    const d = await api('/api/v1/anomaly/baselines')
    baselines.value = d.baselines || {}
  } catch { baselines.value = {} }
  try {
    const d = await api('/api/v1/anomaly/alerts?limit=50')
    alerts.value = d.alerts || []
  } catch { alerts.value = [] }

  // 构建 metricsData
  const data = []
  for (const name of metricNames) {
    try {
      const detail = await api('/api/v1/anomaly/metric/' + name)
      data.push(detail)
    } catch {
      data.push({ metric_name: name, baseline: baselines.value[name] || null, current_value: 0, anomaly: false })
    }
  }
  metricsData.value = data
  loaded.value = true
}

let timer = null
onMounted(() => { loadData(); loadConfig(); timer = setInterval(loadData, 60000) })
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.page-header { margin-bottom: 20px }
.page-header h2 { margin: 0 0 4px 0; font-size: var(--text-lg); color: var(--text-primary) }
.page-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin: 0 }
.section-title { font-size: var(--text-base); font-weight: 700; color: var(--text-primary); margin-bottom: 12px; display: flex; align-items: center; gap: 8px }

.baseline-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 16px }
.baseline-card { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: 16px; transition: border-color .2s }
.baseline-card:hover { border-color: var(--color-primary) }
.bc-anomaly-active { border-color: #EF4444 !important; animation: anomaly-pulse 2s ease-in-out infinite; }
@keyframes anomaly-pulse { 0%,100% { box-shadow: 0 0 0 0 rgba(239,68,68,0.3); } 50% { box-shadow: 0 0 12px 2px rgba(239,68,68,0.4); } }
.metric-link { color: var(--color-primary); cursor: pointer; font-weight: 600; text-decoration: none; }
.metric-link:hover { text-decoration: underline; }
.bc-header { display: flex; align-items: center; gap: 8px; margin-bottom: 12px }
.bc-icon { font-size: 1.2rem }
.bc-name { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary) }
.bc-row { display: flex; justify-content: space-between; align-items: center; padding: 3px 0; font-size: var(--text-xs) }
.bc-label { color: var(--text-tertiary) }
.bc-val { color: var(--text-primary); font-family: var(--font-mono); font-weight: 600 }
.bc-badge { padding: 1px 8px; border-radius: 9999px; font-size: 11px; font-weight: 700 }
.bc-ready { background: rgba(16,185,129,0.15); color: #10B981 }
.anomaly-val { color: #EF4444 !important }
.normal-val { color: #10B981 !important }

/* 基线图 */
.bc-chart { margin-top: 10px }
.baseline-svg { width: 100%; height: 80px }
.bc-chart-labels { display: flex; justify-content: space-between; font-size: 9px; color: var(--text-disabled); padding: 0 2px }

/* 学习中状态 */
.bc-learning { text-align: center; padding: 20px 0 }
.bc-learning-icon { font-size: 2rem; margin-bottom: 8px }
.bc-learning-text { font-size: var(--text-sm); font-weight: 700; color: var(--text-primary); margin-bottom: 4px }
.bc-learning-sub { font-size: var(--text-xs); color: var(--text-tertiary); margin-bottom: 12px }
.bc-progress-wrap { display: flex; align-items: center; gap: 8px }
.bc-progress-bar { flex: 1; height: 6px; background: rgba(255,255,255,0.06); border-radius: 3px; overflow: hidden }
.bc-progress-fill { height: 100%; background: linear-gradient(90deg, #6366F1, #818CF8); border-radius: 3px; transition: width .5s }
.bc-progress-text { font-size: 11px; color: var(--text-tertiary); font-family: var(--font-mono); min-width: 30px }

/* 告警表格 */
.mono { font-family: var(--font-mono) }
.fw-bold { font-weight: 700 }
.row-critical { background: rgba(239,68,68,0.06) }
.row-warning { background: rgba(245,158,11,0.06) }

.deviation-cell { display: flex; align-items: center; gap: 8px }
.deviation-bar-wrap { flex: 1; height: 8px; background: rgba(255,255,255,0.06); border-radius: 4px; position: relative; overflow: hidden; min-width: 60px }
.deviation-bar { height: 100%; border-radius: 4px; transition: width .5s }
.deviation-mark-2s { position: absolute; left: 40%; top: 0; bottom: 0; width: 2px; background: #F59E0B; opacity: .5 }
.deviation-mark-3s { position: absolute; left: 60%; top: 0; bottom: 0; width: 2px; background: #EF4444; opacity: .5 }
.deviation-text { font-size: var(--text-xs); font-weight: 700; font-family: var(--font-mono); min-width: 36px }

.dir-above { color: #EF4444; font-weight: 600 }
.dir-below { color: #3B82F6; font-weight: 600 }
.severity-badge { display: inline-block; padding: 2px 8px; border-radius: 9999px; font-size: 11px; font-weight: 700 }
.sev-critical { background: rgba(239,68,68,0.15); color: #EF4444 }
.sev-warning { background: rgba(245,158,11,0.15); color: #F59E0B }

/* 配置面板 */
.config-panel { padding: 20px; margin-bottom: 16px }
.config-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 16px; margin-bottom: 16px }
.config-item { display: flex; flex-direction: column; gap: 4px }
.config-item label { font-size: var(--text-xs); color: var(--text-tertiary); font-weight: 600 }
.config-input { background: var(--bg-primary); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 6px 10px; color: var(--text-primary); font-size: var(--text-sm); font-family: var(--font-mono); width: 100% }
.config-input:focus { border-color: var(--color-primary); outline: none; box-shadow: 0 0 0 2px rgba(99,102,241,0.2) }
.config-actions { display: flex; align-items: center; gap: 12px }
.config-msg { font-size: var(--text-xs); font-weight: 600 }
.config-msg.success { color: #10B981 }
.config-msg.error { color: #EF4444 }
.btn-sm { padding: 4px 10px; font-size: 12px }
.btn-secondary { background: var(--bg-surface); border: 1px solid var(--border-subtle); color: var(--text-secondary); border-radius: var(--radius-md); cursor: pointer }
.btn-secondary:hover { border-color: var(--color-primary); color: var(--text-primary) }
</style>
