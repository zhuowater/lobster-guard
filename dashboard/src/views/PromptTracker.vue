<template>
  <div>
    <!-- 当前版本卡片 -->
    <div class="card current-card" v-if="currentVersion">
      <div class="card-header">
        <span class="card-icon">📝</span>
        <span class="card-title">当前 Prompt 版本</span>
        <span class="version-badge">v{{ versions.length }}</span>
      </div>
      <div class="current-info">
        <div class="current-row">
          <div class="current-item">
            <div class="current-label">Hash</div>
            <div class="current-value mono">{{ currentVersion.hash }}</div>
          </div>
          <div class="current-item">
            <div class="current-label">模型</div>
            <div class="current-value">{{ currentVersion.model || '-' }}</div>
          </div>
          <div class="current-item">
            <div class="current-label">首次出现</div>
            <div class="current-value">{{ fmtTime(currentVersion.first_seen) }}</div>
          </div>
          <div class="current-item">
            <div class="current-label">调用次数</div>
            <div class="current-value">{{ currentVersion.total_calls || currentVersion.call_count }}</div>
          </div>
        </div>
        <div class="current-metrics">
          <div class="metric-chip" :class="metricClass(currentVersion, 'canary')">
            <span class="metric-label">Canary 泄露率</span>
            <span class="metric-value">{{ canaryRate(currentVersion) }}%</span>
          </div>
          <div class="metric-chip" :class="metricClass(currentVersion, 'error')">
            <span class="metric-label">错误率</span>
            <span class="metric-value">{{ (currentVersion.error_rate * 100).toFixed(1) }}%</span>
          </div>
          <div class="metric-chip">
            <span class="metric-label">平均 Token</span>
            <span class="metric-value">{{ Math.round(currentVersion.avg_tokens) }}</span>
          </div>
          <div class="metric-chip" :class="metricClass(currentVersion, 'flagged')">
            <span class="metric-label">高危工具</span>
            <span class="metric-value">{{ currentVersion.flagged_tools || 0 }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 加载中 -->
    <div v-if="loading" class="loading-state">
      <div class="loading-spinner"></div>
      <span>加载中...</span>
    </div>

    <!-- 空状态 -->
    <div v-else-if="!versions.length" class="empty-state">
      <div class="empty-icon">📝</div>
      <div class="empty-title">暂无 Prompt 版本</div>
      <div class="empty-desc">当 LLM 代理拦截到 System Prompt 后，版本将自动追踪。<br>可点击"注入演示数据"快速体验。</div>
    </div>

    <!-- 版本历史列表 -->
    <div class="card" v-if="versions.length" style="margin-top: 16px;">
      <div class="card-header">
        <span class="card-icon">📋</span>
        <span class="card-title">版本历史</span>
        <span class="version-count">共 {{ versions.length }} 个版本</span>
      </div>
      <div class="version-list">
        <div
          v-for="(v, idx) in versions"
          :key="v.hash"
          class="version-item"
          :class="{ 'version-current': idx === 0 }"
        >
          <div class="version-header">
            <div class="version-left">
              <span class="version-num">v{{ versions.length - idx }}</span>
              <span class="version-hash mono">{{ v.hash }}</span>
              <span class="tag tag-current" v-if="idx === 0">当前</span>
            </div>
            <div class="version-right">
              <span class="version-time">{{ fmtTimeRelative(v.first_seen) }}</span>
              <span class="version-calls">{{ v.total_calls || v.call_count }} 次调用</span>
            </div>
          </div>

          <!-- 安全指标 + Verdict -->
          <div class="version-metrics" v-if="idx < versions.length - 1">
            <div class="vm-item" :class="compareClass(v, versions[idx+1], 'canary')">
              <span>Canary: {{ canaryRate(v) }}%</span>
              <span class="vm-arrow">{{ canaryArrow(v, versions[idx+1]) }}</span>
            </div>
            <div class="vm-item" :class="compareClass(v, versions[idx+1], 'error')">
              <span>Error: {{ (v.error_rate * 100).toFixed(1) }}%</span>
              <span class="vm-arrow">{{ errorArrow(v, versions[idx+1]) }}</span>
            </div>
            <div class="vm-verdict" :class="'verdict-' + getVerdict(v, versions[idx+1])">
              {{ verdictLabel(getVerdict(v, versions[idx+1])) }}
            </div>
          </div>
          <div class="version-metrics" v-else>
            <span class="vm-initial">（初始版本，无对比）</span>
          </div>

          <!-- 操作按钮 -->
          <div class="version-actions">
            <button class="btn-sm btn-outline" @click="showDetail(v)">查看详情</button>
            <button class="btn-sm btn-outline" @click="showDiff(v)" v-if="v.prev_hash">查看 Diff</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 详情 Modal -->
    <div class="modal-overlay" v-if="detailModal" @click.self="detailModal = null">
      <div class="modal-box modal-lg">
        <div class="modal-header">
          <span>Prompt 详情 — {{ detailModal.hash }}</span>
          <button class="modal-close" @click="detailModal = null">✕</button>
        </div>
        <div class="modal-body">
          <div class="detail-meta">
            <span>模型: {{ detailModal.model }}</span>
            <span>首次: {{ fmtTime(detailModal.first_seen) }}</span>
            <span>调用: {{ detailModal.total_calls || detailModal.call_count }} 次</span>
          </div>
          <div class="prompt-content">
            <pre>{{ detailModal.content }}</pre>
          </div>
          <div class="detail-metrics">
            <div class="dm-item">
              <span class="dm-label">Canary 泄露</span>
              <span class="dm-value">{{ detailModal.canary_leaks }} 次 ({{ canaryRate(detailModal) }}%)</span>
            </div>
            <div class="dm-item">
              <span class="dm-label">预算超限</span>
              <span class="dm-value">{{ detailModal.budget_exceeds }} 次</span>
            </div>
            <div class="dm-item">
              <span class="dm-label">高危工具</span>
              <span class="dm-value">{{ detailModal.flagged_tools }} 次</span>
            </div>
            <div class="dm-item">
              <span class="dm-label">错误率</span>
              <span class="dm-value">{{ (detailModal.error_rate * 100).toFixed(1) }}%</span>
            </div>
            <div class="dm-item">
              <span class="dm-label">平均 Token</span>
              <span class="dm-value">{{ Math.round(detailModal.avg_tokens) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Diff Modal -->
    <div class="modal-overlay" v-if="diffModal" @click.self="diffModal = null">
      <div class="modal-box modal-lg">
        <div class="modal-header">
          <span>Prompt Diff — {{ diffModal.new_version?.hash }}</span>
          <button class="modal-close" @click="diffModal = null">✕</button>
        </div>
        <div class="modal-body">
          <!-- Diff 行 -->
          <div class="diff-block">
            <div
              v-for="(line, i) in diffModal.lines"
              :key="i"
              class="diff-line"
              :class="'diff-' + line.type"
            >
              <span class="diff-prefix">{{ line.type === 'added' ? '+' : line.type === 'removed' ? '-' : ' ' }}</span>
              <span class="diff-content">{{ line.content }}</span>
            </div>
          </div>

          <!-- 指标对比 -->
          <div class="metrics-compare" v-if="diffModal.metrics_diff">
            <div class="mc-title">指标对比 ({{ diffModal.old_version?.hash?.slice(0,8) || '无' }} → {{ diffModal.new_version?.hash?.slice(0,8) }})</div>
            <div class="mc-row">
              <span class="mc-label">Canary 泄露率:</span>
              <span class="mc-old">{{ diffModal.metrics_diff.old_canary_rate?.toFixed(1) }}%</span>
              <span class="mc-arrow">→</span>
              <span class="mc-new" :class="rateImproved(diffModal.metrics_diff.old_canary_rate, diffModal.metrics_diff.new_canary_rate)">{{ diffModal.metrics_diff.new_canary_rate?.toFixed(1) }}%</span>
              <span class="mc-change">{{ changeLabel(diffModal.metrics_diff.old_canary_rate, diffModal.metrics_diff.new_canary_rate) }}</span>
            </div>
            <div class="mc-row">
              <span class="mc-label">错误率:</span>
              <span class="mc-old">{{ diffModal.metrics_diff.old_error_rate?.toFixed(1) }}%</span>
              <span class="mc-arrow">→</span>
              <span class="mc-new" :class="rateImproved(diffModal.metrics_diff.old_error_rate, diffModal.metrics_diff.new_error_rate)">{{ diffModal.metrics_diff.new_error_rate?.toFixed(1) }}%</span>
              <span class="mc-change">{{ changeLabel(diffModal.metrics_diff.old_error_rate, diffModal.metrics_diff.new_error_rate) }}</span>
            </div>
            <div class="mc-row">
              <span class="mc-label">平均 Token:</span>
              <span class="mc-old">{{ Math.round(diffModal.metrics_diff.old_avg_tokens || 0) }}</span>
              <span class="mc-arrow">→</span>
              <span class="mc-new">{{ Math.round(diffModal.metrics_diff.new_avg_tokens || 0) }}</span>
            </div>
            <div class="mc-row">
              <span class="mc-label">高危工具率:</span>
              <span class="mc-old">{{ diffModal.metrics_diff.old_flagged_rate?.toFixed(1) }}%</span>
              <span class="mc-arrow">→</span>
              <span class="mc-new" :class="rateImproved(diffModal.metrics_diff.old_flagged_rate, diffModal.metrics_diff.new_flagged_rate)">{{ diffModal.metrics_diff.new_flagged_rate?.toFixed(1) }}%</span>
              <span class="mc-change">{{ changeLabel(diffModal.metrics_diff.old_flagged_rate, diffModal.metrics_diff.new_flagged_rate) }}</span>
            </div>
            <div class="mc-verdict" :class="'verdict-' + diffModal.metrics_diff.verdict">
              判定: {{ verdictLabel(diffModal.metrics_diff.verdict) }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { api } from '../api.js'

const loading = ref(true)
const versions = ref([])
const currentVersion = ref(null)
const detailModal = ref(null)
const diffModal = ref(null)

async function loadData() {
  loading.value = true
  try {
    const [listRes, curRes] = await Promise.allSettled([
      api('/api/v1/prompts'),
      api('/api/v1/prompts/current')
    ])
    if (listRes.status === 'fulfilled') {
      versions.value = listRes.value.versions || []
    }
    if (curRes.status === 'fulfilled' && curRes.value.hash) {
      currentVersion.value = curRes.value
    } else if (versions.value.length > 0) {
      currentVersion.value = versions.value[0]
    }
  } catch (e) {
    console.error('Load prompt versions failed:', e)
  } finally {
    loading.value = false
  }
}

function showDetail(v) {
  detailModal.value = v
}

async function showDiff(v) {
  try {
    const data = await api(`/api/v1/prompts/${v.hash}/diff`)
    diffModal.value = data
  } catch (e) {
    console.error('Load diff failed:', e)
    alert('加载 Diff 失败')
  }
}

function canaryRate(v) {
  const total = v.total_calls || v.call_count || 1
  return ((v.canary_leaks || 0) / total * 100).toFixed(1)
}

function metricClass(v, type) {
  if (type === 'canary') {
    const rate = parseFloat(canaryRate(v))
    return rate > 1 ? 'metric-bad' : rate > 0 ? 'metric-warn' : 'metric-good'
  }
  if (type === 'error') {
    const rate = v.error_rate * 100
    return rate > 5 ? 'metric-bad' : rate > 1 ? 'metric-warn' : 'metric-good'
  }
  if (type === 'flagged') {
    return (v.flagged_tools || 0) > 3 ? 'metric-bad' : (v.flagged_tools || 0) > 0 ? 'metric-warn' : 'metric-good'
  }
  return ''
}

function compareClass(newer, older, type) {
  if (type === 'canary') {
    const n = parseFloat(canaryRate(newer))
    const o = parseFloat(canaryRate(older))
    return n < o ? 'vm-improved' : n > o ? 'vm-degraded' : ''
  }
  if (type === 'error') {
    return newer.error_rate < older.error_rate ? 'vm-improved' : newer.error_rate > older.error_rate ? 'vm-degraded' : ''
  }
  return ''
}

function canaryArrow(newer, older) {
  const n = parseFloat(canaryRate(newer))
  const o = parseFloat(canaryRate(older))
  return n < o ? '↓' : n > o ? '↑' : '→'
}

function errorArrow(newer, older) {
  return newer.error_rate < older.error_rate ? '↓' : newer.error_rate > older.error_rate ? '↑' : '→'
}

function getVerdict(newer, older) {
  let imp = 0, deg = 0
  if (parseFloat(canaryRate(newer)) < parseFloat(canaryRate(older))) imp++
  else if (parseFloat(canaryRate(newer)) > parseFloat(canaryRate(older))) deg++
  if (newer.error_rate < older.error_rate) imp++
  else if (newer.error_rate > older.error_rate) deg++
  if (imp > deg) return 'improved'
  if (deg > imp) return 'degraded'
  return 'neutral'
}

function verdictLabel(v) {
  return v === 'improved' ? '✅ 改善' : v === 'degraded' ? '⚠️ 退化' : '➡️ 持平'
}

function rateImproved(oldRate, newRate) {
  if (newRate < oldRate) return 'mc-improved'
  if (newRate > oldRate) return 'mc-degraded'
  return ''
}

function changeLabel(oldRate, newRate) {
  if (!oldRate && !newRate) return ''
  if (oldRate === 0) return newRate > 0 ? '⚠️' : ''
  const pct = ((newRate - oldRate) / oldRate * 100).toFixed(0)
  if (newRate < oldRate) return `✅ ${pct}%`
  if (newRate > oldRate) return `⚠️ +${pct}%`
  return ''
}

function fmtTime(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? ts : d.toLocaleString('zh-CN', { hour12: false })
}

function fmtTimeRelative(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  const diff = Date.now() - d.getTime()
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  if (days === 0) return '今天'
  if (days === 1) return '昨天'
  return `${days}天前`
}

onMounted(loadData)
</script>

<style scoped>
.current-card { border-left: 3px solid var(--color-primary); }
.current-info { padding: 16px; }
.current-row { display: flex; gap: 24px; flex-wrap: wrap; margin-bottom: 12px; }
.current-item { }
.current-label { font-size: 11px; color: var(--text-tertiary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 4px; }
.current-value { font-size: 14px; color: var(--text-primary); font-weight: 600; }
.current-metrics { display: flex; gap: 12px; flex-wrap: wrap; }
.metric-chip {
  display: flex; flex-direction: column; align-items: center;
  padding: 8px 16px; border-radius: var(--radius-md);
  background: var(--bg-elevated); border: 1px solid var(--border-subtle);
}
.metric-label { font-size: 10px; color: var(--text-tertiary); margin-bottom: 2px; }
.metric-value { font-size: 16px; font-weight: 700; color: var(--text-primary); }
.metric-good { border-color: var(--color-success); }
.metric-good .metric-value { color: var(--color-success); }
.metric-warn { border-color: var(--color-warning); }
.metric-warn .metric-value { color: var(--color-warning); }
.metric-bad { border-color: var(--color-error); }
.metric-bad .metric-value { color: var(--color-error); }

.version-badge {
  margin-left: auto; padding: 2px 8px; border-radius: 10px;
  font-size: 11px; font-weight: 600;
  background: var(--color-primary-dim); color: var(--color-primary);
}
.version-count {
  margin-left: auto; font-size: 12px; color: var(--text-tertiary);
}

.version-list { padding: 0 16px 16px; }
.version-item {
  padding: 12px 16px; margin-bottom: 8px;
  border-radius: var(--radius-md); border: 1px solid var(--border-subtle);
  background: var(--bg-elevated);
  transition: all var(--transition-fast);
}
.version-item:hover { border-color: var(--color-primary); }
.version-current { border-left: 3px solid var(--color-primary); }

.version-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; flex-wrap: wrap; gap: 8px; }
.version-left { display: flex; align-items: center; gap: 8px; }
.version-right { display: flex; align-items: center; gap: 12px; font-size: 12px; color: var(--text-tertiary); }
.version-num { font-weight: 700; font-size: 14px; color: var(--text-primary); }
.version-hash { font-size: 12px; color: var(--text-secondary); }
.version-time { }
.version-calls { }

.tag { padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.tag-current { background: var(--color-primary-dim); color: var(--color-primary); }

.version-metrics { display: flex; align-items: center; gap: 16px; margin-bottom: 8px; flex-wrap: wrap; }
.vm-item { font-size: 12px; color: var(--text-secondary); display: flex; gap: 4px; }
.vm-arrow { font-weight: 700; }
.vm-improved { color: var(--color-success); }
.vm-improved .vm-arrow { color: var(--color-success); }
.vm-degraded { color: var(--color-error); }
.vm-degraded .vm-arrow { color: var(--color-error); }
.vm-initial { font-size: 12px; color: var(--text-tertiary); font-style: italic; }
.vm-verdict { font-size: 12px; font-weight: 600; }
.verdict-improved { color: var(--color-success); }
.verdict-degraded { color: var(--color-error); }
.verdict-neutral { color: var(--text-tertiary); }

.version-actions { display: flex; gap: 8px; }
.btn-sm {
  padding: 4px 12px; font-size: 11px; border-radius: var(--radius-sm); cursor: pointer;
  border: 1px solid var(--border-subtle); background: transparent; color: var(--text-secondary);
  transition: all var(--transition-fast);
}
.btn-sm:hover { background: var(--bg-surface); color: var(--text-primary); border-color: var(--color-primary); }

/* Modal */
.modal-overlay {
  position: fixed; inset: 0; z-index: 1000;
  background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center;
}
.modal-box {
  background: var(--bg-surface); border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle); box-shadow: var(--shadow-lg);
  max-height: 85vh; overflow: auto; width: 90%; max-width: 700px;
}
.modal-lg { max-width: 800px; }
.modal-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 16px 20px; border-bottom: 1px solid var(--border-subtle);
  font-weight: 600; font-size: 15px;
}
.modal-close {
  background: none; border: none; color: var(--text-tertiary); cursor: pointer; font-size: 18px;
  padding: 4px 8px; border-radius: var(--radius-sm);
}
.modal-close:hover { background: var(--bg-elevated); color: var(--text-primary); }
.modal-body { padding: 20px; }

/* Detail Modal */
.detail-meta { display: flex; gap: 16px; font-size: 12px; color: var(--text-secondary); margin-bottom: 16px; flex-wrap: wrap; }
.prompt-content {
  background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: var(--radius-md);
  padding: 16px; margin-bottom: 16px; overflow-x: auto;
}
.prompt-content pre {
  font-family: var(--font-mono); font-size: 13px; color: var(--text-primary);
  white-space: pre-wrap; word-break: break-word; margin: 0;
}
.detail-metrics { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 8px; }
.dm-item { display: flex; justify-content: space-between; padding: 8px 12px; background: var(--bg-elevated); border-radius: var(--radius-sm); }
.dm-label { font-size: 12px; color: var(--text-secondary); }
.dm-value { font-size: 12px; font-weight: 600; color: var(--text-primary); }

/* Diff Modal */
.diff-block {
  background: var(--bg-base); border: 1px solid var(--border-subtle); border-radius: var(--radius-md);
  padding: 8px 0; margin-bottom: 16px; font-family: var(--font-mono); font-size: 13px;
  overflow-x: auto;
}
.diff-line { display: flex; padding: 2px 16px; line-height: 1.6; }
.diff-prefix { width: 16px; flex-shrink: 0; font-weight: 700; user-select: none; }
.diff-content { white-space: pre-wrap; word-break: break-word; }
.diff-added { background: rgba(34, 197, 94, 0.12); color: #22c55e; }
.diff-added .diff-prefix { color: #22c55e; }
.diff-removed { background: rgba(239, 68, 68, 0.12); color: #ef4444; }
.diff-removed .diff-prefix { color: #ef4444; }
.diff-unchanged { color: var(--text-secondary); }

/* Metrics Comparison */
.metrics-compare {
  background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md);
  padding: 16px;
}
.mc-title { font-size: 13px; font-weight: 600; color: var(--text-primary); margin-bottom: 12px; }
.mc-row { display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: 13px; }
.mc-label { color: var(--text-secondary); min-width: 100px; }
.mc-old { color: var(--text-tertiary); }
.mc-arrow { color: var(--text-tertiary); }
.mc-new { font-weight: 600; color: var(--text-primary); }
.mc-improved { color: var(--color-success) !important; }
.mc-degraded { color: var(--color-error) !important; }
.mc-change { font-size: 11px; font-weight: 600; }
.mc-verdict {
  margin-top: 12px; padding: 8px 16px; text-align: center;
  font-size: 14px; font-weight: 700; border-radius: var(--radius-md);
}
.mc-verdict.verdict-improved { background: rgba(34,197,94,0.12); color: #22c55e; }
.mc-verdict.verdict-degraded { background: rgba(239,68,68,0.12); color: #ef4444; }
.mc-verdict.verdict-neutral { background: var(--bg-surface); color: var(--text-tertiary); }

/* Loading & Empty */
.loading-state { display: flex; align-items: center; justify-content: center; gap: 8px; padding: 60px 0; color: var(--text-secondary); }
.loading-spinner { width: 20px; height: 20px; border: 2px solid var(--border-subtle); border-top-color: var(--color-primary); border-radius: 50%; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.empty-state { text-align: center; padding: 60px 20px; }
.empty-icon { font-size: 48px; margin-bottom: 12px; }
.empty-title { font-size: 16px; font-weight: 600; color: var(--text-primary); margin-bottom: 8px; }
.empty-desc { font-size: 13px; color: var(--text-secondary); line-height: 1.6; }

.mono { font-family: var(--font-mono); }
</style>
