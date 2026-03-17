<template>
  <div>
    <!-- Realtime Stats -->
    <div class="ov-cards" style="margin-bottom:20px">
      <StatCard :iconSvg="svgActivity" :value="rt.totalRequests" label="总请求 (60s)" color="blue" />
      <StatCard :iconSvg="svgShieldX" :value="rt.totalBlocks" label="拦截数" color="red" />
      <StatCard :iconSvg="svgPercent" :value="rt.blockRate + '%'" label="拦截率" color="yellow" />
      <StatCard :iconSvg="svgClock" :value="rt.avgLatency + 'ms'" label="平均延迟" color="green" />
    </div>

    <!-- QPS + Attack Timeline -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span>
        <span class="card-title">实时监控</span>
        <span style="margin-left:auto;font-size:var(--text-xs);color:var(--color-success);display:flex;align-items:center;gap:4px">
          <span class="dot dot-sm dot-healthy"></span>每 3s 刷新
        </span>
      </div>
      <div style="display:flex;gap:20px;flex-wrap:wrap">
        <div style="flex:2;min-width:300px">
          <div style="font-size:var(--text-sm);color:var(--text-secondary);margin-bottom:var(--space-2);font-weight:500;display:flex;align-items:center;gap:6px">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
            QPS 曲线（最近 60 秒）
          </div>
          <TrendChart
            :data="qpsChartData"
            :lines="qpsLines"
            :xLabels="qpsXLabels"
            :height="130"
          />
        </div>
        <div style="flex:1;min-width:280px">
          <div style="font-size:var(--text-sm);color:var(--text-secondary);margin-bottom:var(--space-2);font-weight:500;display:flex;align-items:center;gap:6px">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
            攻击实时流
          </div>
          <div class="attack-timeline" style="max-height:220px;overflow-y:auto;background:var(--bg-elevated);border-radius:var(--radius-md);padding:var(--space-2)">
            <EmptyState v-if="!rt.events.length"
              :iconSvg="svgShieldCheck" title="当前环境安全" description="没有检测到攻击事件"
            />
            <div v-for="(e, i) in [...rt.events].reverse()" :key="i" class="timeline-item" :class="{ 'timeline-new': i < newEventCount }">
              <div class="timeline-time">{{ e.time?.substring(11, 19) }}</div>
              <div class="timeline-dot" :style="{ background: e.action === 'block' ? 'var(--color-danger)' : 'var(--color-warning)' }"></div>
              <div class="timeline-card">
                <div class="timeline-card-header">
                  <span :style="{ color: e.action === 'block' ? 'var(--color-danger)' : 'var(--color-warning)', fontWeight: 600, fontSize: 'var(--text-xs)' }">{{ e.action === 'block' ? 'BLOCK' : 'WARN' }}</span>
                  <span class="timeline-direction">[{{ e.direction === 'inbound' ? '入站' : '出站' }}]</span>
                </div>
                <div class="timeline-card-body">
                  <span v-if="e.sender_id" class="timeline-sender">{{ e.sender_id }}</span>
                  <span v-if="e.reason" class="timeline-reason">{{ e.reason }}</span>
                  <span v-if="e.trace_id" class="timeline-trace">{{ (e.trace_id || '').substring(0, 8) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- WebSocket + Rate Limit -->
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:20px">
      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12.55a11 11 0 0 1 14.08 0"/><path d="M1.42 9a16 16 0 0 1 21.16 0"/><path d="M8.53 16.11a6 6 0 0 1 6.95 0"/><line x1="12" y1="20" x2="12.01" y2="20"/></svg></span>
          <span class="card-title">WebSocket 连接</span>
          <div class="card-actions"><button class="btn btn-ghost btn-sm" @click="loadWS">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
          </button></div>
        </div>
        <div v-if="wsLoading" class="loading">加载中...</div>
        <div v-else>
          <div style="display:flex;gap:16px;flex-wrap:wrap;margin-bottom:12px">
            <div><span style="color:var(--text-secondary);font-size:var(--text-xs)">活跃连接</span><br><span style="font-size:var(--text-2xl);font-weight:700;color:var(--color-success);font-family:var(--font-mono)">{{ ws.active }}</span></div>
            <div><span style="color:var(--text-secondary);font-size:var(--text-xs)">总连接数</span><br><span style="font-size:var(--text-2xl);font-weight:700;color:var(--color-primary);font-family:var(--font-mono)">{{ ws.total }}</span></div>
            <div><span style="color:var(--text-secondary);font-size:var(--text-xs)">模式</span><br><span style="font-size:var(--text-lg);font-weight:600" :style="{ color: ws.mode === 'inspect' ? 'var(--color-warning)' : 'var(--text-tertiary)' }">{{ ws.mode || '--' }}</span></div>
          </div>
          <div v-if="ws.connections && ws.connections.length" class="table-wrap">
            <table>
              <tr><th>ID</th><th>Sender</th><th>App</th><th>上游</th><th>路径</th><th>时长</th><th>入站</th><th>出站</th></tr>
              <tr v-for="c in ws.connections" :key="c.id">
                <td style="font-family:var(--font-mono);font-size:var(--text-xs)">{{ c.id }}</td>
                <td>{{ c.sender_id }}</td><td>{{ c.app_id || '-' }}</td><td>{{ c.upstream_id }}</td>
                <td style="font-family:var(--font-mono);font-size:var(--text-xs)">{{ c.path }}</td>
                <td>{{ c.duration }}</td><td style="text-align:right">{{ c.inbound_msgs }}</td><td style="text-align:right">{{ c.outbound_msgs }}</td>
              </tr>
            </table>
          </div>
          <EmptyState v-else-if="ws.active === 0"
            title="暂无活跃连接" description="WebSocket 连接将在客户端接入后显示"
          />
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></span>
          <span class="card-title">限流统计</span>
          <div class="card-actions">
            <button class="btn btn-ghost btn-sm" @click="loadRateLimit">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
            </button>
            <button class="btn btn-danger btn-sm" @click="confirmResetRL">重置</button>
          </div>
        </div>
        <div v-if="rlLoading" class="loading">加载中...</div>
        <EmptyState v-else-if="!rl.enabled" title="限流未启用" description="在配置文件中启用限流功能" />
        <div v-else>
          <div class="stat-big">
            <div class="stat-item"><div class="stat-num green">{{ rl.allowed }}</div><div class="stat-label">通过数</div></div>
            <div class="stat-item"><div class="stat-num red">{{ rl.limited }}</div><div class="stat-label">限流数</div></div>
            <div class="stat-item"><div class="stat-num yellow">{{ rl.rate }}%</div><div class="stat-label">限流率</div></div>
          </div>
          <div v-if="rl.top && rl.top.length">
            <div style="font-size:var(--text-sm);color:var(--color-primary);margin-bottom:var(--space-2);font-weight:500">被限流 Top 5</div>
            <div v-for="(t, i) in rl.top.slice(0, 5)" :key="t.sender_id" style="display:flex;justify-content:space-between;padding:var(--space-1) var(--space-2);border-bottom:1px solid var(--border-subtle);font-size:var(--text-sm)">
              <span><span style="color:var(--color-primary);font-weight:600;margin-right:var(--space-2)">#{{ i + 1 }}</span>{{ t.sender_id }}</span>
              <span style="color:var(--color-danger);font-weight:600;font-family:var(--font-mono)">{{ t.count }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ConfirmModal :visible="confirmVisible" title="重置限流" message="确认重置限流统计？" type="warning" @confirm="doResetRL" @cancel="confirmVisible = false" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { api, apiPost } from '../api.js'
import { showToast } from '../stores/app.js'
import ConfirmModal from '../components/ConfirmModal.vue'
import TrendChart from '../components/TrendChart.vue'
import StatCard from '../components/StatCard.vue'
import EmptyState from '../components/EmptyState.vue'

// SVG icons
const svgActivity = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgShieldX = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><line x1="9.5" y1="9.5" x2="14.5" y2="14.5"/><line x1="14.5" y1="9.5" x2="9.5" y2="14.5"/></svg>'
const svgPercent = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" y1="5" x2="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/></svg>'
const svgClock = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>'
const svgShieldCheck = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>'

const rt = reactive({ totalRequests: 0, totalBlocks: 0, blockRate: '0', avgLatency: '0', slots: [], events: [] })
const newEventCount = ref(0)
const ws = reactive({ active: 0, total: 0, mode: '--', connections: [] })
const wsLoading = ref(false)
const rl = reactive({ enabled: false, allowed: 0, limited: 0, rate: '0', top: [] })
const rlLoading = ref(false)
const confirmVisible = ref(false)

const qpsChartData = computed(() => {
  return rt.slots.map(s => ({
    inbound: s.inbound || 0,
    outbound: s.outbound || 0,
    block: s.block || 0,
  }))
})

const qpsLines = [
  { key: 'inbound', color: '#3B82F6', label: '入站' },
  { key: 'outbound', color: '#10B981', label: '出站' },
  { key: 'block', color: '#EF4444', label: '拦截' },
]

const qpsXLabels = computed(() => {
  const n = rt.slots.length
  return rt.slots.map((_, i) => {
    if (i % 10 === 0 || i === n - 1) return (n - i) + 's'
    return ''
  })
})

let prevEventCount = 0

async function loadRealtime() {
  try {
    const d = await api('/api/v1/metrics/realtime')
    rt.totalRequests = d.total_requests || 0
    rt.totalBlocks = d.total_blocks || 0
    rt.blockRate = d.block_rate != null ? d.block_rate.toFixed(1) : '0'
    rt.avgLatency = d.avg_latency_ms != null ? d.avg_latency_ms.toFixed(1) : '0'
    rt.slots = d.slots || []
    const events = d.events || []
    const newCount = events.length - prevEventCount
    newEventCount.value = Math.max(0, newCount)
    prevEventCount = events.length
    rt.events = events
  } catch { /* ignore */ }
}

async function loadWS() {
  wsLoading.value = true
  try {
    const d = await api('/api/v1/ws/connections')
    ws.active = d.active || 0; ws.total = d.total || 0; ws.mode = d.mode || '--'; ws.connections = d.connections || []
  } catch { /* ignore */ }
  wsLoading.value = false
}

async function loadRateLimit() {
  rlLoading.value = true
  try {
    const d = await api('/api/v1/rate-limit/stats')
    rl.enabled = d.enabled !== false
    rl.allowed = d.total_allowed || 0
    rl.limited = d.total_limited || 0
    rl.rate = d.limit_rate_percent != null ? d.limit_rate_percent.toFixed(2) : '0'
    rl.top = d.top_limited || []
  } catch { /* ignore */ }
  rlLoading.value = false
}

function confirmResetRL() { confirmVisible.value = true }
async function doResetRL() {
  confirmVisible.value = false
  try { await apiPost('/api/v1/rate-limit/reset', {}); showToast('限流已重置', 'success'); loadRateLimit() } catch (e) { showToast('重置失败: ' + e.message, 'error') }
}

let realtimeTimer = null
onMounted(() => {
  loadRealtime(); loadWS(); loadRateLimit()
  realtimeTimer = setInterval(loadRealtime, 3000)
})
onUnmounted(() => clearInterval(realtimeTimer))
</script>

<style scoped>
.timeline-item {
  display: flex; align-items: flex-start; gap: var(--space-2); padding: var(--space-2) var(--space-1);
  border-bottom: 1px solid var(--border-subtle); animation: timeline-in .4s ease-out both;
}
.timeline-new { animation: timeline-flash .6s ease-out; }
@keyframes timeline-in {
  from { opacity: 0; transform: translateX(20px); }
  to { opacity: 1; transform: translateX(0); }
}
@keyframes timeline-flash {
  0% { background: var(--color-primary-dim); }
  100% { background: transparent; }
}
.timeline-time {
  font-size: var(--text-xs); color: var(--text-tertiary); font-family: var(--font-mono);
  min-width: 55px; flex-shrink: 0; padding-top: 2px;
}
.timeline-dot {
  width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; margin-top: 4px;
}
.timeline-card {
  flex: 1; min-width: 0; background: var(--bg-surface); border-radius: var(--radius-md);
  padding: var(--space-1) var(--space-2); border-left: 2px solid var(--border-subtle);
}
.timeline-card-header { display: flex; align-items: center; gap: var(--space-2); margin-bottom: 2px; }
.timeline-direction { font-size: var(--text-xs); color: var(--text-tertiary); }
.timeline-card-body { display: flex; gap: var(--space-2); flex-wrap: wrap; font-size: var(--text-xs); }
.timeline-sender { color: var(--text-primary); font-weight: 500; }
.timeline-reason { color: var(--text-secondary); }
.timeline-trace { font-family: var(--font-mono); color: var(--color-primary); font-size: 0.68rem; }
</style>
