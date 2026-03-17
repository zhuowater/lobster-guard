<template>
  <div>
    <!-- Realtime -->
    <div class="card" style="margin-bottom:20px">
      <div class="card-header">
        <span class="card-icon">⚡</span><span class="card-title">实时监控</span>
        <span style="margin-left:auto;font-size:.7rem;color:var(--neon-green)">每 3s 刷新</span>
      </div>
      <div style="display:flex;gap:20px;flex-wrap:wrap;margin-bottom:16px">
        <div class="stat-item"><div class="stat-num blue">{{ rt.totalRequests }}</div><div class="stat-label">总请求 (60s)</div></div>
        <div class="stat-item"><div class="stat-num red">{{ rt.totalBlocks }}</div><div class="stat-label">拦截数</div></div>
        <div class="stat-item"><div class="stat-num yellow">{{ rt.blockRate }}%</div><div class="stat-label">拦截率</div></div>
        <div class="stat-item"><div class="stat-num green">{{ rt.avgLatency }}ms</div><div class="stat-label">平均延迟</div></div>
      </div>
      <div style="display:flex;gap:20px;flex-wrap:wrap">
        <div style="flex:2;min-width:300px">
          <div style="font-size:.82rem;color:var(--text-dim);margin-bottom:6px;font-weight:600">📈 QPS 曲线（最近 60 秒）</div>
          <div style="display:flex;align-items:flex-end;gap:1px;height:100px;background:rgba(0,0,0,.2);border-radius:6px;padding:4px;overflow:hidden">
            <div v-for="(s, i) in rt.slots" :key="i" style="flex:1;display:flex;flex-direction:column;justify-content:flex-end;align-items:center;min-width:0" :title="`in=${s.inbound} out=${s.outbound} blk=${s.block}`">
              <div v-if="s.block > 0" :style="{ width: '100%', height: barH(s.block) + 'px', background: 'var(--neon-red)', borderRadius: '1px', minHeight: '1px' }"></div>
              <div v-if="s.outbound > 0" :style="{ width: '100%', height: barH(s.outbound) + 'px', background: 'var(--neon-green)', borderRadius: '1px', minHeight: '1px' }"></div>
              <div v-if="s.inbound > 0" :style="{ width: '100%', height: barH(s.inbound) + 'px', background: 'var(--neon-blue)', borderRadius: '1px', minHeight: '1px' }"></div>
              <div v-if="s.inbound + s.outbound === 0" style="width:100%;height:1px;background:rgba(255,255,255,.05)"></div>
            </div>
          </div>
          <div style="display:flex;gap:12px;margin-top:4px;font-size:.65rem;color:var(--text-dim)">
            <span><span style="display:inline-block;width:8px;height:8px;background:var(--neon-blue);border-radius:2px;margin-right:2px"></span>入站</span>
            <span><span style="display:inline-block;width:8px;height:8px;background:var(--neon-green);border-radius:2px;margin-right:2px"></span>出站</span>
            <span><span style="display:inline-block;width:8px;height:8px;background:var(--neon-red);border-radius:2px;margin-right:2px"></span>拦截</span>
          </div>
        </div>
        <div style="flex:1;min-width:280px">
          <div style="font-size:.82rem;color:var(--text-dim);margin-bottom:6px;font-weight:600">🚨 攻击实时流</div>
          <div style="max-height:160px;overflow-y:auto;background:rgba(0,0,0,.2);border-radius:6px;padding:6px">
            <div v-if="!rt.events.length" style="color:var(--text-dim);font-size:.8rem;text-align:center;padding:20px">暂无攻击事件 ✅</div>
            <div v-for="(e, i) in [...rt.events].reverse()" :key="i" style="padding:3px 6px;border-bottom:1px solid rgba(255,255,255,.04);font-size:.75rem;display:flex;gap:6px;align-items:center">
              <span style="color:var(--text-dim);font-size:.65rem">{{ e.time?.substring(11, 19) }}</span>
              <span :style="{ color: e.action === 'block' ? 'var(--neon-red)' : 'var(--neon-yellow)', fontWeight: 600, fontSize: '.7rem' }">{{ e.action }}</span>
              <span style="color:var(--text-dim)">[{{ e.direction === 'inbound' ? '入' : '出' }}]</span>
              <span>{{ e.sender_id || '--' }}</span>
              <span style="color:var(--text-dim);font-size:.7rem;font-family:monospace">{{ (e.trace_id || '--').substring(0, 8) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- WebSocket + Rate Limit -->
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:20px">
      <div class="card">
        <div class="card-header">
          <span class="card-icon">🔌</span><span class="card-title">WebSocket 连接</span>
          <div class="card-actions"><button class="btn btn-sm" @click="loadWS">🔄</button></div>
        </div>
        <div v-if="wsLoading" class="loading">加载中...</div>
        <div v-else>
          <div style="display:flex;gap:16px;flex-wrap:wrap;margin-bottom:12px">
            <div><span style="color:var(--text-dim);font-size:.75rem">活跃连接</span><br><span style="font-size:1.6rem;font-weight:700;color:var(--neon-green)">{{ ws.active }}</span></div>
            <div><span style="color:var(--text-dim);font-size:.75rem">总连接数</span><br><span style="font-size:1.6rem;font-weight:700;color:var(--neon-blue)">{{ ws.total }}</span></div>
            <div><span style="color:var(--text-dim);font-size:.75rem">模式</span><br><span style="font-size:1.1rem;font-weight:600" :style="{ color: ws.mode === 'inspect' ? 'var(--neon-yellow)' : 'var(--text-dim)' }">{{ ws.mode || '--' }}</span></div>
          </div>
          <div v-if="ws.connections && ws.connections.length" class="table-wrap">
            <table>
              <tr><th>ID</th><th>Sender</th><th>App</th><th>上游</th><th>路径</th><th>时长</th><th>入站</th><th>出站</th></tr>
              <tr v-for="c in ws.connections" :key="c.id">
                <td style="font-family:monospace;font-size:.8rem">{{ c.id }}</td>
                <td>{{ c.sender_id }}</td><td>{{ c.app_id || '-' }}</td><td>{{ c.upstream_id }}</td>
                <td style="font-family:monospace;font-size:.8rem">{{ c.path }}</td>
                <td>{{ c.duration }}</td><td style="text-align:right">{{ c.inbound_msgs }}</td><td style="text-align:right">{{ c.outbound_msgs }}</td>
              </tr>
            </table>
          </div>
          <div v-else-if="ws.active === 0" style="text-align:center;color:var(--text-dim);padding:16px">暂无活跃 WebSocket 连接</div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <span class="card-icon">⏱️</span><span class="card-title">限流统计</span>
          <div class="card-actions">
            <button class="btn btn-sm" @click="loadRateLimit">🔄</button>
            <button class="btn btn-sm btn-red" @click="confirmResetRL">重置</button>
          </div>
        </div>
        <div v-if="rlLoading" class="loading">加载中...</div>
        <div v-else-if="!rl.enabled" class="empty">限流未启用</div>
        <div v-else>
          <div class="stat-big">
            <div class="stat-item"><div class="stat-num green">{{ rl.allowed }}</div><div class="stat-label">通过数</div></div>
            <div class="stat-item"><div class="stat-num red">{{ rl.limited }}</div><div class="stat-label">限流数</div></div>
            <div class="stat-item"><div class="stat-num yellow">{{ rl.rate }}%</div><div class="stat-label">限流率</div></div>
          </div>
          <div v-if="rl.top && rl.top.length">
            <div style="font-size:.8rem;color:var(--neon-blue);margin-bottom:6px;font-weight:600">被限流 Top 5</div>
            <div v-for="(t, i) in rl.top.slice(0, 5)" :key="t.sender_id" style="display:flex;justify-content:space-between;padding:4px 8px;border-bottom:1px solid rgba(255,255,255,.04);font-size:.82rem">
              <span><span style="color:var(--neon-blue);font-weight:600;margin-right:8px">#{{ i + 1 }}</span>{{ t.sender_id }}</span>
              <span style="color:var(--neon-red);font-weight:600">{{ t.count }}</span>
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

const rt = reactive({ totalRequests: 0, totalBlocks: 0, blockRate: '0', avgLatency: '0', slots: [], events: [] })
const ws = reactive({ active: 0, total: 0, mode: '--', connections: [] })
const wsLoading = ref(false)
const rl = reactive({ enabled: false, allowed: 0, limited: 0, rate: '0', top: [] })
const rlLoading = ref(false)
const confirmVisible = ref(false)

const maxV = computed(() => { let m = 1; for (const s of rt.slots) { const t = s.inbound + s.outbound; if (t > m) m = t }; return m })
function barH(v) { return Math.max(0, Math.round(v / maxV.value * 80)) }

async function loadRealtime() {
  try {
    const d = await api('/api/v1/metrics/realtime')
    rt.totalRequests = d.total_requests || 0
    rt.totalBlocks = d.total_blocks || 0
    rt.blockRate = d.block_rate != null ? d.block_rate.toFixed(1) : '0'
    rt.avgLatency = d.avg_latency_ms != null ? d.avg_latency_ms.toFixed(1) : '0'
    rt.slots = d.slots || []
    rt.events = d.events || []
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
