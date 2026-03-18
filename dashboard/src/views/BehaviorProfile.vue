<template>
  <div>
    <div class="page-header">
      <h2 class="page-title">🧠 Agent 行为画像</h2>
      <p class="page-desc">学习 Agent 正常行为模式，检测语义行为突变</p>
    </div>

    <!-- 顶部统计卡片 -->
    <div class="ov-cards" v-if="loaded">
      <StatCard :iconSvg="svgAgent" :value="stats.totalProfiles" label="Agent Profiled" color="blue" />
      <StatCard :iconSvg="svgAlert" :value="stats.totalAnomalies" label="突变 Anomalies" color="red" />
      <StatCard :iconSvg="svgRisk" :value="stats.highRiskCount" label="高风险 High Risk" color="yellow" />
      <StatCard :iconSvg="svgPattern" :value="stats.totalPatterns" label="行为模式 Patterns" color="purple" />
    </div>
    <div class="ov-cards" v-else>
      <Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" />
    </div>

    <!-- Agent 画像卡片列表 -->
    <div v-if="loaded && profiles.length > 0">
      <div class="profile-card" v-for="p in profiles" :key="p.agent_id"
           :class="{'profile-critical': p.risk_level==='critical', 'profile-high': p.risk_level==='high', 'profile-elevated': p.risk_level==='elevated'}">
        <div class="profile-header">
          <div class="profile-id">
            <span class="profile-name">{{ p.agent_id }}</span>
            <span class="profile-display" v-if="p.display_name && p.display_name !== p.agent_id">{{ p.display_name }}</span>
          </div>
          <span class="risk-badge" :class="'risk-' + p.risk_level">
            {{ riskIcon(p.risk_level) }} {{ riskLabel(p.risk_level) }}
          </span>
        </div>

        <div class="profile-stats">
          <span>总请求: <b>{{ p.total_requests }}</b></span>
          <span v-if="p.avg_tokens > 0">平均Token: <b>{{ Math.round(p.avg_tokens) }}</b></span>
          <span v-if="p.avg_tools_per_req > 0">工具/请求: <b>{{ p.avg_tools_per_req.toFixed(1) }}</b></span>
          <span v-if="p.peak_hours && p.peak_hours.length">活跃: <b>{{ p.peak_hours.map(h => h+':00').join('-') }}</b></span>
        </div>

        <!-- 常用工具条形图 -->
        <div v-if="p.typical_tools && p.typical_tools.length" class="profile-section">
          <div class="section-label">常用工具</div>
          <div class="tool-bar" v-for="tool in p.typical_tools.slice(0,6)" :key="tool.tool_name">
            <span class="tool-name" :title="tool.tool_name">{{ tool.tool_name }}</span>
            <div class="tool-track">
              <div class="tool-fill" :style="{ width: Math.max(3, tool.percentage) + '%', background: toolColor(tool.tool_name) }">
              </div>
            </div>
            <span class="tool-pct">{{ tool.percentage.toFixed(0) }}%</span>
          </div>
        </div>

        <!-- 行为模式 -->
        <div v-if="p.common_patterns && p.common_patterns.length" class="profile-section">
          <div class="section-label">行为模式</div>
          <div class="pattern-item" v-for="(pat, pi) in p.common_patterns.slice(0,5)" :key="pi">
            <span class="pattern-dot" :style="{ background: riskColor(pat.risk_score) }"></span>
            <span class="pattern-seq">{{ pat.sequence.join(' → ') }}</span>
            <span class="pattern-count">({{ pat.count }}次</span>
            <span class="pattern-risk" :style="{ color: riskColor(pat.risk_score) }">
              {{ pat.risk_score > 50 ? '高风险' : pat.risk_score > 20 ? '中风险' : '低风险' }})
            </span>
          </div>
        </div>

        <!-- 突变告警 -->
        <div v-if="p.anomalies && p.anomalies.length" class="profile-section anomaly-section">
          <div class="section-label">⚠️ 突变告警</div>
          <div class="anomaly-item" v-for="a in p.anomalies.slice(0,5)" :key="a.id">
            <span class="anomaly-severity" :class="'sev-' + a.severity">{{ sevIcon(a.severity) }}</span>
            <span class="anomaly-desc">{{ a.description }}</span>
          </div>
        </div>

        <!-- 操作按钮 -->
        <div class="profile-actions">
          <button class="btn-sm btn-primary" @click="scanAgent(p.agent_id)" :disabled="scanning === p.agent_id">
            {{ scanning === p.agent_id ? '扫描中...' : '扫描' }}
          </button>
          <button class="btn-sm btn-ghost" @click="showDetail(p.agent_id)">查看详情</button>
          <button v-if="p.anomalies && p.anomalies.length && p.anomalies.some(a => a.trace_id)"
                  class="btn-sm btn-ghost" @click="goToReplay(p.anomalies.find(a => a.trace_id)?.trace_id)">
            查看会话回放
          </button>
        </div>
      </div>
    </div>

    <EmptyState v-else-if="loaded && !profiles.length"
      :iconSvg="svgBrain" title="暂无 Agent 画像" description="注入演示数据或等待 Agent 活动后自动生成画像"
    />
    <Skeleton v-else type="table" />

    <!-- 详情弹窗 -->
    <div v-if="detailProfile" class="modal-overlay" @click.self="detailProfile = null">
      <div class="modal-content">
        <div class="modal-header">
          <h3>{{ detailProfile.agent_id }} — 详细画像</h3>
          <button class="btn-close" @click="detailProfile = null">✕</button>
        </div>
        <div class="modal-body">
          <div class="detail-row"><span class="dl">风险等级</span><span class="risk-badge" :class="'risk-' + detailProfile.risk_level">{{ riskLabel(detailProfile.risk_level) }}</span></div>
          <div class="detail-row"><span class="dl">总请求</span><span>{{ detailProfile.total_requests }}</span></div>
          <div class="detail-row"><span class="dl">平均 Token</span><span>{{ Math.round(detailProfile.avg_tokens) }}</span></div>
          <div class="detail-row"><span class="dl">平均工具/请求</span><span>{{ detailProfile.avg_tools_per_req?.toFixed(2) }}</span></div>
          <div class="detail-row"><span class="dl">活跃时段</span><span>{{ (detailProfile.peak_hours || []).map(h => h+':00').join(', ') || '无数据' }}</span></div>
          <div class="detail-row"><span class="dl">首次活跃</span><span>{{ fmtTime(detailProfile.profiled_since) }}</span></div>
          <div class="detail-row"><span class="dl">最近活跃</span><span>{{ fmtTime(detailProfile.last_seen) }}</span></div>

          <div v-if="detailProfile.typical_tools?.length" style="margin-top:16px">
            <div class="section-label">全部工具使用</div>
            <div class="tool-bar" v-for="tool in detailProfile.typical_tools" :key="tool.tool_name">
              <span class="tool-name">{{ tool.tool_name }}</span>
              <div class="tool-track">
                <div class="tool-fill" :style="{ width: Math.max(3, tool.percentage) + '%', background: toolColor(tool.tool_name) }"></div>
              </div>
              <span class="tool-pct">{{ tool.count }} ({{ tool.percentage.toFixed(1) }}%)</span>
            </div>
          </div>

          <div v-if="detailProfile.common_patterns?.length" style="margin-top:16px">
            <div class="section-label">全部行为模式</div>
            <div class="pattern-item" v-for="(pat, pi) in detailProfile.common_patterns" :key="pi">
              <span class="pattern-dot" :style="{ background: riskColor(pat.risk_score) }"></span>
              <span class="pattern-seq">{{ pat.sequence.join(' → ') }}</span>
              <span class="pattern-count">({{ pat.count }}次, 风险={{ pat.risk_score }})</span>
            </div>
          </div>

          <div v-if="detailProfile.anomalies?.length" style="margin-top:16px">
            <div class="section-label">全部突变记录</div>
            <div class="anomaly-item" v-for="a in detailProfile.anomalies" :key="a.id">
              <span class="anomaly-severity" :class="'sev-' + a.severity">{{ sevIcon(a.severity) }}</span>
              <span class="anomaly-desc">{{ a.description }}</span>
              <span class="anomaly-time">{{ fmtTime(a.timestamp) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { api, apiPost } from '../api.js'
import StatCard from '../components/StatCard.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'

const svgAgent = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><line x1="12" y1="7" x2="12" y2="11"/><line x1="8" y1="16" x2="8" y2="16.01"/><line x1="16" y1="16" x2="16" y2="16.01"/></svg>'
const svgAlert = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgRisk = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>'
const svgPattern = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgBrain = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 2a4 4 0 014 4c0 2.5-2 4-2 6h-4c0-2-2-3.5-2-6a4 4 0 014-4z"/><path d="M10 12h4"/><path d="M10 16h4"/></svg>'

const criticalTools = new Set(['exec', 'shell', 'bash', 'run_command', 'execute_command'])
const highTools = new Set(['write_file', 'edit_file', 'delete_file', 'write', 'edit', 'http_request', 'curl', 'send_email', 'send_message', 'message'])

function toolColor(name) {
  if (criticalTools.has(name)) return '#EF4444'
  if (highTools.has(name)) return '#F59E0B'
  return '#3B82F6'
}

function riskColor(score) {
  if (score >= 50) return '#EF4444'
  if (score >= 20) return '#F59E0B'
  return '#3B82F6'
}

function riskIcon(level) {
  const m = { critical: '🔴', high: '🟠', elevated: '🟡', normal: '🟢' }
  return m[level] || '⚪'
}

function riskLabel(level) {
  const m = { critical: '极危', high: '高风险', elevated: '轻微异常', normal: '正常' }
  return m[level] || level
}

function sevIcon(sev) {
  const m = { critical: '🔴', high: '🟠', medium: '🟡', low: '🔵' }
  return m[sev] || '⚪'
}

function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false })
}

const router = useRouter()
const loaded = ref(false)
const profiles = ref([])
const stats = ref({ totalProfiles: 0, totalAnomalies: 0, highRiskCount: 0, totalPatterns: 0 })
const scanning = ref(null)
const detailProfile = ref(null)

async function loadData() {
  try {
    const d = await api('/api/v1/behavior/profiles')
    profiles.value = d.profiles || []
    stats.value = {
      totalProfiles: d.total || 0,
      totalAnomalies: d.total_anomalies || 0,
      highRiskCount: d.high_risk_count || 0,
      totalPatterns: d.total_patterns || 0,
    }
  } catch {
    profiles.value = []
  }
  loaded.value = true
}

async function scanAgent(agentID) {
  scanning.value = agentID
  try {
    await apiPost('/api/v1/behavior/profiles/' + encodeURIComponent(agentID) + '/scan')
    await loadData()
  } catch { /* ignore */ }
  scanning.value = null
}

async function showDetail(agentID) {
  try {
    const d = await api('/api/v1/behavior/profiles/' + encodeURIComponent(agentID))
    detailProfile.value = d
  } catch { /* ignore */ }
}

function goToReplay(traceID) {
  if (traceID) router.push('/sessions/' + traceID)
}

let timer = null
onMounted(() => { loadData(); timer = setInterval(loadData, 30000) })
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.page-header { margin-bottom: 20px; }
.page-title { font-size: var(--text-xl); font-weight: 700; margin: 0 0 4px 0; }
.page-desc { font-size: var(--text-sm); color: var(--text-tertiary); margin: 0; }

.profile-card {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
  margin-bottom: 16px;
  transition: border-color .2s;
}
.profile-card:hover { border-color: var(--color-primary); }
.profile-critical { border-left: 3px solid #EF4444; }
.profile-high { border-left: 3px solid #F59E0B; }
.profile-elevated { border-left: 3px solid #8B5CF6; }

.profile-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.profile-id { display: flex; align-items: center; gap: 8px; }
.profile-name { font-size: var(--text-base); font-weight: 700; font-family: var(--font-mono); }
.profile-display { font-size: var(--text-sm); color: var(--text-tertiary); }

.risk-badge { display: inline-block; padding: 2px 10px; border-radius: 9999px; font-size: var(--text-xs); font-weight: 600; }
.risk-normal { background: rgba(34,197,94,.15); color: #22C55E; }
.risk-elevated { background: rgba(139,92,246,.15); color: #8B5CF6; }
.risk-high { background: rgba(245,158,11,.15); color: #F59E0B; }
.risk-critical { background: rgba(239,68,68,.15); color: #EF4444; }

.profile-stats {
  display: flex; flex-wrap: wrap; gap: 16px;
  font-size: var(--text-sm); color: var(--text-secondary);
  margin-bottom: 16px; padding-bottom: 12px; border-bottom: 1px solid var(--border-subtle);
}
.profile-stats b { color: var(--text-primary); }

.profile-section { margin-bottom: 12px; }
.section-label { font-size: 11px; font-weight: 600; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: .05em; margin-bottom: 8px; }

.tool-bar { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.tool-name { width: 120px; font-size: var(--text-xs); font-family: var(--font-mono); color: var(--text-secondary); text-align: right; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex-shrink: 0; }
.tool-track { flex: 1; height: 16px; background: rgba(255,255,255,.05); border-radius: 4px; overflow: hidden; }
.tool-fill { height: 100%; border-radius: 4px; transition: width .6s ease-out; min-width: 2px; }
.tool-pct { width: 40px; font-size: var(--text-xs); color: var(--text-tertiary); text-align: right; flex-shrink: 0; }

.pattern-item { display: flex; align-items: center; gap: 6px; font-size: var(--text-sm); margin-bottom: 4px; }
.pattern-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.pattern-seq { font-family: var(--font-mono); font-size: var(--text-xs); color: var(--text-primary); }
.pattern-count { font-size: var(--text-xs); color: var(--text-tertiary); }
.pattern-risk { font-size: var(--text-xs); font-weight: 600; }

.anomaly-section { background: rgba(239,68,68,.04); border-radius: var(--radius-md); padding: 10px; }
.anomaly-item { display: flex; align-items: flex-start; gap: 6px; font-size: var(--text-sm); margin-bottom: 4px; }
.anomaly-severity { flex-shrink: 0; font-size: 12px; }
.anomaly-desc { color: var(--text-secondary); }
.anomaly-time { font-size: var(--text-xs); color: var(--text-tertiary); margin-left: auto; white-space: nowrap; }

.profile-actions { display: flex; gap: 8px; margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--border-subtle); }
.btn-sm { padding: 4px 12px; font-size: var(--text-xs); border-radius: var(--radius-md); cursor: pointer; border: 1px solid var(--border-subtle); transition: all .15s; }
.btn-primary { background: var(--color-primary); color: #fff; border-color: var(--color-primary); }
.btn-primary:hover { opacity: .85; }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-ghost { background: transparent; color: var(--text-secondary); }
.btn-ghost:hover { background: var(--bg-elevated); color: var(--text-primary); }

/* 弹窗 */
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,.6); z-index: 9999; display: flex; align-items: center; justify-content: center; }
.modal-content { background: var(--bg-surface); border-radius: var(--radius-lg); width: 90%; max-width: 700px; max-height: 85vh; overflow-y: auto; box-shadow: var(--shadow-lg); }
.modal-header { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid var(--border-subtle); }
.modal-header h3 { margin: 0; font-size: var(--text-base); }
.btn-close { background: none; border: none; font-size: 18px; cursor: pointer; color: var(--text-secondary); padding: 4px 8px; }
.btn-close:hover { color: var(--text-primary); }
.modal-body { padding: 20px; }
.detail-row { display: flex; justify-content: space-between; padding: 6px 0; font-size: var(--text-sm); border-bottom: 1px solid rgba(255,255,255,.04); }
.dl { color: var(--text-tertiary); font-weight: 600; }
</style>
