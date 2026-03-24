<template>
  <div class="gateway-monitor">
    <!-- 顶部工具栏 -->
    <div class="gm-toolbar">
      <div class="toolbar-left">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="M12 8v4l3 3"/></svg>
        <span class="toolbar-title">Gateway 监控中心</span>
        <span class="last-refresh" :title="'上次刷新: ' + lastRefreshTime">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
          {{ lastRefreshDisplay }}
        </span>
      </div>
      <div class="toolbar-right">
        <button class="toolbar-btn refresh-btn" @click="refresh" :class="{ spinning: loading }" title="刷新">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
        </button>
        <div class="refresh-interval-wrap">
          <select v-model="refreshInterval" @change="onRefreshChange" class="refresh-select" title="自动刷新间隔">
            <option value="15000">15s</option><option value="30000">30s</option><option value="60000">1m</option><option value="0">手动</option>
          </select>
        </div>
      </div>
    </div>

    <!-- 空状态：无上游 -->
    <div v-if="!initialLoading && allUpstreams.length === 0" class="empty-state">
      <div class="empty-visual">
        <svg width="80" height="80" viewBox="0 0 80 80" fill="none">
          <circle cx="40" cy="40" r="36" stroke="#6366f1" stroke-width="2" stroke-dasharray="6 4" opacity="0.3"/>
          <circle cx="40" cy="40" r="12" fill="#6366f1" opacity="0.15"/>
          <path d="M40 28v24M28 40h24" stroke="#6366f1" stroke-width="2" stroke-linecap="round"/>
        </svg>
      </div>
      <h2 class="empty-title">尚未发现上游实例</h2>
      <p class="empty-desc">添加 OpenClaw Gateway 上游后，即可在此监控所有实例的运行状态。</p>
      <router-link to="/upstream" class="btn btn-primary btn-md">添加上游</router-link>
    </div>

    <!-- 空状态：有上游但全部未配 Token -->
    <div v-else-if="!initialLoading && allUpstreams.length > 0 && configuredCount === 0" class="empty-state">
      <div class="empty-visual">
        <svg width="80" height="80" viewBox="0 0 80 80" fill="none">
          <rect x="24" y="32" width="32" height="24" rx="4" stroke="#6366f1" stroke-width="2" opacity="0.4"/>
          <path d="M32 32v-6a8 8 0 0 1 16 0v6" stroke="#eab308" stroke-width="2" stroke-linecap="round"/>
          <circle cx="40" cy="44" r="3" fill="#eab308"/>
        </svg>
      </div>
      <h2 class="empty-title">配置 Gateway Token 以开始监控</h2>
      <p class="empty-desc">已发现 <strong>{{ allUpstreams.length }}</strong> 个上游，但均未配置认证 Token。</p>
      <button class="btn btn-primary btn-md" @click="openTokenModal(allUpstreams[0])">配置第一个 Token</button>
    </div>

    <!-- 加载骨架 -->
    <div v-else-if="initialLoading" class="skeleton-wrap">
      <div class="stat-row"><div class="skeleton-card" v-for="i in 5" :key="i"></div></div>
      <div class="skeleton-table"><div class="skeleton-line" v-for="i in 6" :key="i"></div></div>
    </div>

    <!-- ====== 主内容区 ====== -->
    <template v-else>
      <!-- 聚合统计卡片 -->
      <div class="stat-row">
        <div class="stat-card stat-blue">
          <div class="stat-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg></div>
          <div class="stat-body"><div class="stat-value">{{ overview.total || 0 }}</div><div class="stat-label">上游总数</div></div>
        </div>
        <div class="stat-card stat-green">
          <div class="stat-icon pulse-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></div>
          <div class="stat-body"><div class="stat-value">{{ overview.online || 0 }}</div><div class="stat-label">在线</div></div>
        </div>
        <div class="stat-card" :class="(overview.offline || 0) > 0 ? 'stat-red' : 'stat-dim'">
          <div class="stat-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg></div>
          <div class="stat-body"><div class="stat-value">{{ overview.offline || 0 }}</div><div class="stat-label">离线/异常</div></div>
        </div>
        <div class="stat-card stat-indigo">
          <div class="stat-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg></div>
          <div class="stat-body"><div class="stat-value">{{ configuredCount }}<span class="stat-sub"> / {{ allUpstreams.length }}</span></div><div class="stat-label">Token 已配</div></div>
        </div>
        <div class="stat-card stat-cyan">
          <div class="stat-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg></div>
          <div class="stat-body"><div class="stat-value">{{ overview.total_sessions || 0 }}<span class="stat-sub"> / {{ overview.active_sessions || 0 }} 活跃</span></div><div class="stat-label">会话</div></div>
        </div>
      </div>

      <!-- 拓扑视图 -->
      <div class="topo-section">
        <div class="section-header">
          <h3 class="section-title">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M12 1v4M12 19v4M4.22 4.22l2.83 2.83M16.95 16.95l2.83 2.83M1 12h4M19 12h4M4.22 19.78l2.83-2.83M16.95 7.05l2.83-2.83"/></svg>
            实时拓扑
          </h3>
          <div class="topo-legend">
            <span class="legend-item"><span class="legend-dot lg-green"></span>已连接</span>
            <span class="legend-item"><span class="legend-dot lg-gray"></span>未配置</span>
            <span class="legend-item"><span class="legend-dot lg-red"></span>异常</span>
          </div>
        </div>
        <div class="topo-canvas">
          <!-- 连线层 (SVG) -->
          <svg class="topo-lines" viewBox="0 0 600 300" preserveAspectRatio="xMidYMid meet">
            <line v-for="(up, i) in allUpstreams" :key="'line-'+up.id"
                  :x1="300" :y1="150"
                  :x2="nodeX(i, allUpstreams.length, 600, 300)"
                  :y2="nodeY(i, allUpstreams.length, 600, 300)"
                  class="topo-link" :class="'link-' + up.gateway_status" />
            <!-- 数据流粒子（已连接的线上） -->
            <circle v-for="(up, i) in connectedUpstreams" :key="'particle-'+up.id"
                    r="3" class="flow-particle">
              <animateMotion :dur="(1.5 + i * 0.3) + 's'" repeatCount="indefinite">
                <mpath :href="'#path-'+i" />
              </animateMotion>
            </circle>
            <path v-for="(up, i) in connectedUpstreams" :key="'path-'+up.id"
                  :id="'path-'+i"
                  :d="`M300,150 L${nodeX(allUpstreams.indexOf(up), allUpstreams.length, 600, 300)},${nodeY(allUpstreams.indexOf(up), allUpstreams.length, 600, 300)}`"
                  fill="none" stroke="none" />
          </svg>
          <!-- 中心节点 -->
          <div class="topo-center-node">
            <div class="center-glow"></div>
            <div class="center-body">🦞</div>
            <div class="center-text">Lobster Guard</div>
          </div>
          <!-- 外围节点 -->
          <div v-for="(up, i) in allUpstreams" :key="'node-'+up.id"
               class="topo-outer-node" :class="'tn-' + up.gateway_status"
               :style="outerNodeStyle(i, allUpstreams.length)"
               @click="toggleExpand(up)">
            <div class="outer-dot" :class="{ 'dot-pulse': up.gateway_status === 'connected' }"></div>
            <div class="outer-label">{{ up.id }}</div>
          </div>
        </div>
      </div>

      <!-- 上游列表 -->
      <div class="list-section">
        <div class="section-header">
          <h3 class="section-title">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>
            上游实例
          </h3>
        </div>
        <div class="table-wrap">
          <table class="gm-table">
            <thead><tr>
              <th style="width:28px"></th><th>ID</th><th>地址</th><th>健康</th><th>Gateway</th><th>会话</th><th>延迟</th><th>操作</th>
            </tr></thead>
            <tbody>
              <template v-for="up in allUpstreams" :key="up.id">
                <tr :class="{ 'row-active': expandedId === up.id }" @click="toggleExpand(up)">
                  <td><svg class="expand-chevron" :class="{ open: expandedId === up.id }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg></td>
                  <td><strong class="id-text">{{ up.id }}</strong></td>
                  <td><code class="mono-text">{{ up.address }}:{{ up.port }}</code></td>
                  <td><span class="health-ind" :class="up.healthy ? 'h-ok' : 'h-err'"><span class="h-dot"></span>{{ up.healthy ? '健康' : '异常' }}</span></td>
                  <td><span class="gw-badge" :class="'gw-' + up.gateway_status">{{ statusLabel(up.gateway_status) }}</span></td>
                  <td>
                    <template v-if="up.gateway_status === 'connected'"><span class="num-p">{{ up.session_count }}</span><span class="num-s"> / {{ up.active_sessions }} 活跃</span></template>
                    <span v-else class="text-dim">—</span>
                  </td>
                  <td>
                    <span v-if="up.latency_ms > 0" :class="latencyClass(up.latency_ms)">{{ up.latency_ms }}ms</span>
                    <span v-else class="text-dim">—</span>
                  </td>
                  <td @click.stop>
                    <div class="act-group">
                      <button v-if="!up.token_configured" class="btn btn-xs btn-primary" @click="openTokenModal(up)">配置 Token</button>
                      <button v-else-if="up.gateway_status === 'auth_failed'" class="btn btn-xs btn-warn" @click="openTokenModal(up)">重新配置</button>
                      <template v-else>
                        <button class="btn btn-xs btn-ghost" @click="quickPing(up)" title="Ping" :disabled="up._pinging">⚡</button>
                        <button class="btn btn-xs btn-ghost" @click="openTokenModal(up)" title="Token">⚙️</button>
                      </template>
                    </div>
                  </td>
                </tr>
                <!-- 展开行 -->
                <tr v-if="expandedId === up.id" class="expand-row">
                  <td colspan="8">
                    <div class="detail-panel" :key="'detail-'+up.id">
                      <div class="dtabs">
                        <button v-for="t in tabs" :key="t.key" class="dtab" :class="{ active: activeTab === t.key }" @click="switchTab(t.key, up.id)">{{ t.icon }} {{ t.label }}</button>
                      </div>
                      <!-- Sessions -->
                      <div v-if="activeTab === 'sessions'" class="dtab-body">
                        <div v-if="detailLoading" class="skel-lines"><div class="skel-line" v-for="i in 4" :key="i"></div></div>
                        <div v-else-if="sessions.length === 0" class="dtab-empty">暂无会话</div>
                        <table v-else class="inner-table">
                          <thead><tr><th>Key</th><th>Channel</th><th>Model</th><th>Token</th><th>上下文</th><th>最后活跃</th></tr></thead>
                          <tbody>
                            <tr v-for="s in sessions" :key="s.key||s.sessionId">
                              <td><code class="mono-sm">{{ truncKey(s.key||s.sessionId) }}</code></td>
                              <td>{{ s.channel||s.lastChannel||'—' }}</td>
                              <td class="text-dim">{{ s.model||'—' }}</td>
                              <td>{{ fmtTokens(s) }}</td>
                              <td>{{ fmtContext(s) }}</td>
                              <td>{{ fmtTime(s.updatedAt||s.updated_at) }}</td>
                            </tr>
                          </tbody>
                        </table>
                      </div>
                      <!-- Cron -->
                      <div v-if="activeTab === 'cron'" class="dtab-body">
                        <div v-if="detailLoading" class="skel-lines"><div class="skel-line" v-for="i in 3" :key="i"></div></div>
                        <div v-else-if="cronJobs.length === 0" class="dtab-empty">暂无定时任务</div>
                        <table v-else class="inner-table">
                          <thead><tr><th>名称</th><th>状态</th><th>计划</th><th>下次运行</th></tr></thead>
                          <tbody>
                            <tr v-for="c in cronJobs" :key="c.id||c.name">
                              <td>{{ c.name||c.id||'—' }}</td>
                              <td><span class="gw-badge" :class="c.enabled!==false?'gw-connected':'gw-not_configured'">{{ c.enabled!==false?'启用':'禁用' }}</span></td>
                              <td><code class="mono-sm">{{ c.schedule||c.cron||'—' }}</code></td>
                              <td>{{ fmtTime(c.next_run||c.nextRun) }}</td>
                            </tr>
                          </tbody>
                        </table>
                      </div>
                      <!-- 诊断 Tab -->
                      <div v-if="activeTab === 'diag'" class="dtab-body diag-body">
                        <div class="diag-card">
                          <h4>三步快速诊断</h4>
                          <button class="btn btn-sm btn-primary" @click="runDiag(up)" :disabled="diagRunning">{{ diagRunning ? '诊断中...' : '🔍 开始诊断' }}</button>
                          <div v-if="diagResult" class="diag-steps">
                            <div class="diag-step" :class="diagResult.reach ? 'ds-ok' : 'ds-fail'">
                              <span class="ds-icon">{{ diagResult.reach ? '✅' : '❌' }}</span>
                              <div><strong>网络连通</strong><br/><span class="ds-detail">{{ diagResult.reach ? `延迟 ${diagResult.latency}ms` : (diagResult.err || '无法连接') }}</span></div>
                            </div>
                            <div class="diag-step" :class="diagResult.auth ? 'ds-ok' : (diagResult.reach ? 'ds-fail' : 'ds-skip')">
                              <span class="ds-icon">{{ diagResult.auth ? '✅' : (diagResult.reach ? '❌' : '⏭️') }}</span>
                              <div><strong>Token 认证</strong><br/><span class="ds-detail">{{ diagResult.auth ? '验证通过' : (diagResult.reach ? 'Token 无效' : '跳过') }}</span></div>
                            </div>
                            <div class="diag-step" :class="diagResult.api ? 'ds-ok' : (diagResult.auth ? 'ds-fail' : 'ds-skip')">
                              <span class="ds-icon">{{ diagResult.api ? '✅' : (diagResult.auth ? '⚠️' : '⏭️') }}</span>
                              <div><strong>API 可用</strong><br/><span class="ds-detail">{{ diagResult.api ? `${diagResult.sessions} 个会话` : (diagResult.auth ? '调用异常' : '跳过') }}</span></div>
                            </div>
                          </div>
                        </div>
                        <div class="diag-card">
                          <h4>Token 管理</h4>
                          <div class="tk-row">
                            <span>状态：</span>
                            <span class="gw-badge" :class="up.token_configured ? 'gw-connected' : 'gw-not_configured'">{{ up.token_configured ? '已配置' : '未配置' }}</span>
                          </div>
                          <div class="tk-actions">
                            <button class="btn btn-sm btn-primary" @click="openTokenModal(up)">{{ up.token_configured ? '更新' : '配置' }} Token</button>
                            <button v-if="up.token_configured" class="btn btn-sm btn-danger-ghost" @click="clearToken(up)">清除</button>
                          </div>
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

      <!-- 跨 Gateway Agent 画廊 -->
      <div v-if="allAgents.length > 0" class="agents-section">
        <div class="section-header">
          <h3 class="section-title">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
            Agent 画廊 <span class="badge-count">{{ allAgents.length }}</span>
          </h3>
        </div>
        <div class="agent-grid">
          <div v-for="ag in allAgents" :key="ag.id+ag.gateway" class="agent-card" :class="{ 'ag-active': ag.active }">
            <div class="ag-avatar" :style="{ background: agentColor(ag.id) }">{{ agentInitial(ag.id) }}</div>
            <div class="ag-info">
              <div class="ag-name">{{ ag.id }}</div>
              <div class="ag-meta">
                <span class="ag-gateway">{{ ag.gateway }}</span>
                <span class="ag-model" v-if="ag.model">{{ ag.model }}</span>
              </div>
            </div>
            <div class="ag-status" :class="ag.active ? 'ag-on' : 'ag-off'">{{ ag.active ? '活跃' : '空闲' }}</div>
          </div>
        </div>
      </div>
    </template>

    <!-- Token 配置弹窗 -->
    <Teleport to="body">
      <div v-if="tokenModal.show" class="modal-overlay" @click.self="tokenModal.show = false">
        <div class="modal-box">
          <div class="modal-head">
            <h3>{{ tokenModal.data?.token_configured ? '更新' : '配置' }} Gateway Token</h3>
            <button class="btn btn-xs btn-ghost" @click="tokenModal.show = false">✕</button>
          </div>
          <div class="modal-body">
            <p class="modal-desc">为 <strong>{{ tokenModal.data?.id }}</strong> ({{ tokenModal.data?.address }}:{{ tokenModal.data?.port }}) 配置 OpenClaw Gateway 认证 Token。</p>
            <label class="form-label">Token</label>
            <div class="input-wrap">
              <input :type="tokenModal.showPwd ? 'text' : 'password'" v-model="tokenModal.token" class="form-input" placeholder="粘贴 Gateway Auth Token" autocomplete="off" @keydown.enter="saveToken" />
              <button class="eye-btn" @click="tokenModal.showPwd = !tokenModal.showPwd">{{ tokenModal.showPwd ? '🙈' : '👁️' }}</button>
            </div>
            <div v-if="tokenModal.testResult" class="test-result" :class="tokenModal.testResult.ok ? 'tr-ok' : 'tr-err'">
              {{ tokenModal.testResult.ok ? `✅ 连接成功 · ${tokenModal.testResult.latency}ms` : `❌ ${tokenModal.testResult.msg}` }}
            </div>
            <div v-if="tokenModal.testResult && !tokenModal.testResult.ok" class="modal-hint">测试失败不影响保存 — Token 可能正确但网络暂时不通</div>
          </div>
          <div class="modal-foot">
            <button class="btn btn-ghost" @click="testInModal" :disabled="tokenModal.testing || !tokenModal.token">{{ tokenModal.testing ? '测试中...' : '测试连接' }}</button>
            <button class="btn btn-primary" @click="saveToken" :disabled="tokenModal.saving || !tokenModal.token">{{ tokenModal.saving ? '保存中...' : '保存' }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Toast -->
    <Teleport to="body">
      <Transition name="toast">
        <div v-if="toast.show" class="toast" :class="'toast-' + toast.type" @click="toast.show = false">{{ toast.msg }}</div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { api, apiPut, apiDelete } from '../api.js'

// === State ===
const initialLoading = ref(true)
const loading = ref(false)
const detailLoading = ref(false)
const overview = ref({})
const allUpstreams = ref([])
const expandedId = ref(null)
const activeTab = ref('sessions')
const sessions = ref([])
const cronJobs = ref([])
const diagRunning = ref(false)
const diagResult = ref(null)
const lastRefreshTime = ref('')
const lastRefreshDisplay = ref('')
const refreshInterval = ref(localStorage.getItem('gm_refresh') || '30000')
let refreshTimer = null
let displayTimer = null

const toast = reactive({ show: false, msg: '', type: 'info' })
let toastTimer = null

const tokenModal = reactive({ show: false, data: null, token: '', showPwd: false, testing: false, saving: false, testResult: null })

const tabs = [
  { key: 'sessions', icon: '💬', label: '会话' },
  { key: 'cron', icon: '⏰', label: '定时任务' },
  { key: 'diag', icon: '🔍', label: '诊断' },
]

// === Computed ===
const configuredCount = computed(() => allUpstreams.value.filter(u => u.token_configured).length)
const connectedUpstreams = computed(() => allUpstreams.value.filter(u => u.gateway_status === 'connected'))

const allAgents = computed(() => {
  const agents = []
  for (const up of allUpstreams.value) {
    // 优先从 overview 返回的 agents 列表
    if (up.gateway_status === 'connected' && up._agents && up._agents.length > 0) {
      for (const ag of up._agents) {
        const aid = ag.id || ag.agentId
        if (aid && !agents.find(a => a.id === aid && a.gateway === up.id)) {
          agents.push({ id: aid, gateway: up.id, model: null, active: !!ag.configured, configured: !!ag.configured })
        }
      }
    }
    // 补充从 sessions 提取的 agent + model 信息
    if (up.gateway_status === 'connected' && up._sessions) {
      for (const s of up._sessions) {
        // OpenClaw sessions 用 key 格式 "agent:<agentId>:<sessionType>"
        const aid = extractAgentFromKey(s.key) || s.agentId || s.agent_id
        if (!aid) continue
        const existing = agents.find(a => a.id === aid && a.gateway === up.id)
        if (existing) {
          if (!existing.model && s.model) existing.model = s.model
          if (s.totalTokens > 0 || s.total_tokens > 0) existing.active = true
        } else {
          agents.push({ id: aid, gateway: up.id, model: s.model, active: true, configured: true })
        }
      }
    }
  }
  return agents
})

// === Data Loading ===
async function refresh() {
  loading.value = true
  try {
    const data = await api('/api/v1/upstreams/gateway/overview')
    overview.value = data
    allUpstreams.value = (data.upstreams || []).map(u => ({
      ...u,
      gateway_status: u.gateway_status || 'unknown',
      _pinging: false,
      _sessions: extractSessions(u),
      _agents: u.agents || [],
    }))
    lastRefreshTime.value = new Date().toLocaleTimeString('zh-CN')
    updateRefreshDisplay()
  } catch (e) {
    showToast('加载失败: ' + e.message, 'error')
  } finally {
    loading.value = false
    initialLoading.value = false
  }
}

function extractSessions(up) {
  if (!up.sessions) return []
  if (Array.isArray(up.sessions)) return up.sessions
  if (up.sessions && up.sessions.sessions) return up.sessions.sessions
  return []
}

async function toggleExpand(up) {
  if (expandedId.value === up.id) { expandedId.value = null; return }
  expandedId.value = up.id; activeTab.value = 'sessions'; diagResult.value = null
  if (up.token_configured && up.gateway_status !== 'not_configured') await loadTabData(up.id, 'sessions')
}
async function switchTab(tab, id) { activeTab.value = tab; if (tab !== 'diag') await loadTabData(id, tab) }
async function loadTabData(id, tab) {
  detailLoading.value = true
  try {
    if (tab === 'sessions') {
      const d = await api(`/api/v1/upstreams/${encodeURIComponent(id)}/gateway/sessions`)
      sessions.value = d.error ? [] : (Array.isArray(d.sessions) ? d.sessions : [])
    } else if (tab === 'cron') {
      const d = await api(`/api/v1/upstreams/${encodeURIComponent(id)}/gateway/cron`)
      cronJobs.value = d.error ? [] : (Array.isArray(d.jobs) ? d.jobs : [])
    }
  } catch { sessions.value = []; cronJobs.value = [] } finally { detailLoading.value = false }
}
async function runDiag(up) {
  diagRunning.value = true; diagResult.value = null
  try {
    const p = await api(`/api/v1/upstreams/${encodeURIComponent(up.id)}/gateway/ping`)
    let apiOk = !!p.api_ok, sc = 0
    if (apiOk) {
      try {
        const sd = await api(`/api/v1/upstreams/${encodeURIComponent(up.id)}/gateway/sessions`)
        if (!sd.error && sd.sessions) { sc = sd.sessions.length }
      } catch {}
    }
    diagResult.value = { reach: !!p.reachable, auth: !!p.authenticated, api: apiOk, latency: p.latency_ms, sessions: sc, err: p.message, statusText: p.status_text }
  } catch (e) { diagResult.value = { reach:false, auth:false, api:false, err:e.message } } finally { diagRunning.value = false }
}
async function quickPing(up) {
  up._pinging = true
  try { const r = await api(`/api/v1/upstreams/${encodeURIComponent(up.id)}/gateway/ping`); showToast(r.api_ok ? `${up.id}: ✅ ${r.latency_ms}ms` : `${up.id}: ❌ ${r.message||'连接失败'}`, r.api_ok?'success':'error') }
  catch (e) { showToast(`${up.id}: ❌ ${e.message}`, 'error') } finally { up._pinging = false }
}
function openTokenModal(up) { tokenModal.data=up; tokenModal.token=''; tokenModal.showPwd=false; tokenModal.testing=false; tokenModal.saving=false; tokenModal.testResult=null; tokenModal.show=true }
async function testInModal() {
  if (!tokenModal.token||!tokenModal.data) return; tokenModal.testing=true; tokenModal.testResult=null
  try { await apiPut(`/api/v1/upstreams/${encodeURIComponent(tokenModal.data.id)}/gateway-token`,{token:tokenModal.token}); const r=await api(`/api/v1/upstreams/${encodeURIComponent(tokenModal.data.id)}/gateway/ping`); tokenModal.testResult={ok:!!r.api_ok,latency:r.latency_ms,msg:r.message||'连接失败'} }
  catch (e) { tokenModal.testResult={ok:false,msg:e.message} } finally { tokenModal.testing=false }
}
async function saveToken() {
  if (!tokenModal.token||!tokenModal.data) return; tokenModal.saving=true
  try { await apiPut(`/api/v1/upstreams/${encodeURIComponent(tokenModal.data.id)}/gateway-token`,{token:tokenModal.token}); showToast('Token 已保存','success'); tokenModal.show=false; await refresh() }
  catch (e) { showToast('保存失败: '+e.message,'error') } finally { tokenModal.saving=false }
}
async function clearToken(up) {
  if (!confirm(`确定清除 ${up.id} 的 Gateway Token？`)) return
  try { await apiDelete(`/api/v1/upstreams/${encodeURIComponent(up.id)}/gateway-token`); showToast('已清除','success'); await refresh() } catch(e) { showToast('失败: '+e.message,'error') }
}

function statusLabel(s) { return {connected:'已连接',not_configured:'未配置',auth_failed:'认证失败',unreachable:'不可达',error:'错误'}[s]||s||'未知' }
function isActive(s) { return ['running','active','busy','working','in_progress','processing','streaming'].includes(s) }
function latencyClass(ms) { return ms<100?'lat-fast':ms<500?'lat-mid':'lat-slow' }
function truncKey(k) { return k&&k.length>32?k.slice(0,14)+'…'+k.slice(-14):(k||'—') }
function fmtTokens(s) { const t=s.totalTokens||s.total_tokens||0; return !t?'—':t>1e6?(t/1e6).toFixed(1)+'M':t>1e3?(t/1e3).toFixed(1)+'K':String(t) }
function fmtContext(s) { const ctx=s.contextTokens||0; const total=s.totalTokens||s.total_tokens||0; if(!ctx) return '—'; const pct=total>0?Math.round(total/ctx*100):0; return `${fmtTokens({totalTokens:total})}/${fmtTokens({totalTokens:ctx})} (${pct}%)` }
function extractAgentFromKey(key) { if(!key) return null; const m=/^agent:([^:]+):/.exec(key); return m?m[1]:null }
function fmtTime(ts) { if(!ts) return '—'; const d=new Date(typeof ts==='number'&&ts<1e12?ts*1000:ts); if(isNaN(d)) return '—'; const s=(Date.now()-d)/1000; if(s<60) return '刚刚'; if(s<3600) return Math.floor(s/60)+'分钟前'; if(s<86400) return Math.floor(s/3600)+'小时前'; return d.toLocaleDateString('zh-CN')+' '+d.toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'}) }
function nodeX(i,n,w) { return w/2+(w*0.38)*Math.cos(2*Math.PI*i/Math.max(n,1)-Math.PI/2) }
function nodeY(i,n,w,h) { return h/2+(h*0.38)*Math.sin(2*Math.PI*i/Math.max(n,1)-Math.PI/2) }
function outerNodeStyle(i,n) { const a=2*Math.PI*i/Math.max(n,1)-Math.PI/2,r=42; return {left:(50+r*Math.cos(a))+'%',top:(50+r*Math.sin(a))+'%',transform:'translate(-50%,-50%)'} }
function agentColor(id) { let h=0; for(let i=0;i<(id||'').length;i++) h=(id||'').charCodeAt(i)+((h<<5)-h); return `hsl(${Math.abs(h)%360},60%,45%)` }
function agentInitial(id) { return (id||'?').charAt(0).toUpperCase() }
function showToast(m,t='info') { toast.msg=m; toast.type=t; toast.show=true; if(toastTimer)clearTimeout(toastTimer); toastTimer=setTimeout(()=>{toast.show=false},3500) }
function updateRefreshDisplay() { lastRefreshDisplay.value = lastRefreshTime.value || '—' }
function onRefreshChange() { localStorage.setItem('gm_refresh',refreshInterval.value); setupTimer() }
function setupTimer() { if(refreshTimer)clearInterval(refreshTimer); const ms=parseInt(refreshInterval.value); if(ms>0) refreshTimer=setInterval(refresh,ms) }

onMounted(()=>{ refresh(); setupTimer(); displayTimer=setInterval(updateRefreshDisplay,5000) })
onUnmounted(()=>{ if(refreshTimer)clearInterval(refreshTimer); if(displayTimer)clearInterval(displayTimer) })
</script>

<style scoped>
.gateway-monitor { padding:20px 28px; max-width:1440px; margin:0 auto; }
.gm-toolbar { display:flex; align-items:center; justify-content:space-between; margin-bottom:20px; }
.toolbar-left { display:flex; align-items:center; gap:10px; color:var(--text-primary,#e2e8f0); }
.toolbar-title { font-size:18px; font-weight:700; }
.toolbar-right { display:flex; align-items:center; gap:8px; }
.last-refresh { display:flex; align-items:center; gap:4px; font-size:12px; color:var(--text-tertiary,#64748b); }
.refresh-btn { padding:6px; border-radius:6px; background:transparent; border:1px solid var(--border-subtle,#1e293b); color:var(--text-secondary,#94a3b8); cursor:pointer; transition:all .15s; display:flex; align-items:center; }
.refresh-btn:hover { background:rgba(99,102,241,.1); border-color:#6366f1; color:#a5b4fc; }
.refresh-btn.spinning svg { animation:spin .8s linear infinite; }
@keyframes spin { to{transform:rotate(360deg)} }
.refresh-select { background:var(--bg-surface,#1e293b); border:1px solid var(--border-subtle,#334155); border-radius:6px; color:var(--text-secondary,#94a3b8); font-size:12px; padding:4px 8px; cursor:pointer; }
.refresh-interval-wrap { display:flex; }
.toolbar-btn { display:flex; align-items:center; }
.empty-state { display:flex; flex-direction:column; align-items:center; justify-content:center; padding:80px 20px; text-align:center; }
.empty-visual { margin-bottom:24px; }
.empty-title { font-size:20px; font-weight:700; color:var(--text-primary,#e2e8f0); margin-bottom:8px; }
.empty-desc { font-size:14px; color:var(--text-secondary,#94a3b8); line-height:1.6; max-width:440px; margin-bottom:24px; }
.skeleton-wrap { animation:fadeIn .3s; }
.skeleton-card { height:80px; background:var(--bg-surface,#1e293b); border-radius:12px; animation:shimmer 1.5s infinite; }
.skeleton-table { margin-top:16px; background:var(--bg-surface,#1e293b); border-radius:12px; padding:16px; }
.skel-lines { padding:12px 0; }
.skel-line { height:32px; background:linear-gradient(90deg,rgba(99,102,241,.05) 25%,rgba(99,102,241,.1) 50%,rgba(99,102,241,.05) 75%); background-size:200% 100%; border-radius:6px; margin-bottom:8px; animation:shimmer 1.5s infinite; }
@keyframes shimmer { 0%{background-position:200% 0} 100%{background-position:-200% 0} }
@keyframes fadeIn { from{opacity:0} to{opacity:1} }
.stat-row { display:grid; grid-template-columns:repeat(auto-fill,minmax(170px,1fr)); gap:12px; margin-bottom:20px; }
.stat-card { display:flex; align-items:center; gap:12px; padding:16px; background:var(--bg-surface,#1e293b); border:1px solid var(--border-subtle,#334155); border-radius:12px; transition:all .2s; }
.stat-card:hover { border-color:rgba(99,102,241,.3); transform:translateY(-1px); }
.stat-icon { width:36px; height:36px; border-radius:10px; display:flex; align-items:center; justify-content:center; flex-shrink:0; }
.stat-blue .stat-icon { background:rgba(59,130,246,.15); color:#60a5fa; }
.stat-green .stat-icon { background:rgba(34,197,94,.15); color:#4ade80; }
.stat-red .stat-icon { background:rgba(239,68,68,.15); color:#f87171; }
.stat-dim .stat-icon { background:rgba(100,116,139,.1); color:#64748b; }
.stat-indigo .stat-icon { background:rgba(99,102,241,.15); color:#a5b4fc; }
.stat-cyan .stat-icon { background:rgba(6,182,212,.15); color:#22d3ee; }
.stat-value { font-size:22px; font-weight:700; color:var(--text-primary,#e2e8f0); line-height:1.2; }
.stat-sub { font-size:13px; font-weight:400; color:var(--text-tertiary,#64748b); }
.stat-label { font-size:12px; color:var(--text-tertiary,#64748b); margin-top:2px; }
.pulse-icon { animation:pulse-glow 2s ease-in-out infinite; }
@keyframes pulse-glow { 0%,100%{opacity:1} 50%{opacity:.6} }
.section-header { display:flex; align-items:center; justify-content:space-between; margin-bottom:12px; }
.section-title { display:flex; align-items:center; gap:8px; font-size:15px; font-weight:600; color:var(--text-primary,#e2e8f0); }
.topo-section { background:var(--bg-surface,#1e293b); border:1px solid var(--border-subtle,#334155); border-radius:12px; padding:20px; margin-bottom:20px; }
.topo-legend { display:flex; gap:16px; }
.legend-item { display:flex; align-items:center; gap:5px; font-size:12px; color:var(--text-tertiary,#64748b); }
.legend-dot { width:8px; height:8px; border-radius:50%; }
.lg-green { background:#22c55e; box-shadow:0 0 6px rgba(34,197,94,.4); }
.lg-gray { background:#64748b; }
.lg-red { background:#ef4444; box-shadow:0 0 6px rgba(239,68,68,.4); }
.topo-canvas { position:relative; height:300px; margin-top:12px; }
.topo-lines { position:absolute; inset:0; width:100%; height:100%; pointer-events:none; }
.topo-link { stroke-width:1.5; stroke-dasharray:4 3; }
.link-connected { stroke:#22c55e; stroke-opacity:.5; stroke-dasharray:none; }
.link-not_configured { stroke:#64748b; stroke-opacity:.25; }
.link-auth_failed { stroke:#ef4444; stroke-opacity:.4; }
.link-unreachable { stroke:#eab308; stroke-opacity:.3; }
.link-error,.link-unknown { stroke:#64748b; stroke-opacity:.2; }
.flow-particle { fill:#22c55e; opacity:.8; filter:drop-shadow(0 0 3px #22c55e); }
.topo-center-node { position:absolute; left:50%; top:50%; transform:translate(-50%,-50%); text-align:center; z-index:2; }
.center-glow { position:absolute; left:50%; top:50%; transform:translate(-50%,-50%); width:80px; height:80px; border-radius:50%; background:radial-gradient(circle,rgba(99,102,241,.2) 0%,transparent 70%); animation:center-pulse 3s ease-in-out infinite; }
@keyframes center-pulse { 0%,100%{transform:translate(-50%,-50%) scale(1);opacity:.6} 50%{transform:translate(-50%,-50%) scale(1.2);opacity:1} }
.center-body { width:48px; height:48px; border-radius:50%; background:linear-gradient(135deg,#6366f1,#8b5cf6); display:flex; align-items:center; justify-content:center; font-size:24px; margin:0 auto; box-shadow:0 0 20px rgba(99,102,241,.4); position:relative; z-index:1; }
.center-text { font-size:11px; font-weight:600; color:var(--text-tertiary,#94a3b8); margin-top:6px; letter-spacing:.5px; }
.topo-outer-node { position:absolute; text-align:center; cursor:pointer; z-index:2; transition:transform .2s; }
.topo-outer-node:hover { transform:translate(-50%,-50%) scale(1.1) !important; }
.outer-dot { width:20px; height:20px; border-radius:50%; margin:0 auto 4px; border:2px solid; transition:all .2s; }
.tn-connected .outer-dot { background:rgba(34,197,94,.2); border-color:#22c55e; }
.tn-not_configured .outer-dot { background:rgba(100,116,139,.15); border-color:#475569; }
.tn-auth_failed .outer-dot { background:rgba(239,68,68,.15); border-color:#ef4444; }
.tn-unreachable .outer-dot { background:rgba(234,179,8,.15); border-color:#eab308; }
.tn-error .outer-dot,.tn-unknown .outer-dot { background:rgba(100,116,139,.1); border-color:#475569; }
.dot-pulse { animation:dot-breathe 2s ease-in-out infinite; }
@keyframes dot-breathe { 0%,100%{box-shadow:0 0 0 0 rgba(34,197,94,.4)} 50%{box-shadow:0 0 0 6px rgba(34,197,94,0)} }
.outer-label { font-size:11px; color:var(--text-secondary,#94a3b8); white-space:nowrap; max-width:100px; overflow:hidden; text-overflow:ellipsis; }
.list-section { background:var(--bg-surface,#1e293b); border:1px solid var(--border-subtle,#334155); border-radius:12px; padding:20px; margin-bottom:20px; }
.table-wrap { overflow-x:auto; margin-top:8px; }
.gm-table { width:100%; border-collapse:collapse; }
.gm-table th { padding:8px 12px; text-align:left; font-size:11px; font-weight:600; color:var(--text-tertiary,#64748b); text-transform:uppercase; letter-spacing:.05em; border-bottom:1px solid var(--border-subtle,#334155); }
.gm-table td { padding:10px 12px; font-size:13px; color:var(--text-secondary,#94a3b8); border-bottom:1px solid rgba(51,65,85,.5); }
.gm-table tbody tr { cursor:pointer; transition:background .15s; }
.gm-table tbody tr:hover { background:rgba(99,102,241,.04); }
.gm-table tbody tr.row-active { background:rgba(99,102,241,.08); }
.expand-row { cursor:default !important; }
.expand-row:hover { background:transparent !important; }
.expand-chevron { transition:transform .2s; color:var(--text-tertiary,#64748b); }
.expand-chevron.open { transform:rotate(90deg); }
.id-text { color:var(--text-primary,#e2e8f0); }
.mono-text { font-size:12px; font-family:'SF Mono',Menlo,monospace; background:rgba(99,102,241,.08); padding:2px 6px; border-radius:4px; color:#a5b4fc; }
.mono-sm { font-size:11px; font-family:'SF Mono',Menlo,monospace; color:#94a3b8; }
.health-ind { display:inline-flex; align-items:center; gap:5px; font-size:12px; }
.h-dot { width:7px; height:7px; border-radius:50%; }
.h-ok .h-dot { background:#22c55e; box-shadow:0 0 5px rgba(34,197,94,.4); }
.h-err .h-dot { background:#ef4444; box-shadow:0 0 5px rgba(239,68,68,.4); }
.h-ok { color:#4ade80; } .h-err { color:#f87171; }
.gw-badge { display:inline-block; padding:2px 8px; border-radius:9999px; font-size:11px; font-weight:600; }
.gw-connected { background:rgba(34,197,94,.12); color:#4ade80; }
.gw-not_configured { background:rgba(100,116,139,.12); color:#64748b; }
.gw-auth_failed { background:rgba(239,68,68,.12); color:#f87171; }
.gw-unreachable { background:rgba(234,179,8,.12); color:#facc15; }
.gw-error,.gw-unknown { background:rgba(100,116,139,.08); color:#64748b; }
.num-p { font-weight:600; color:var(--text-primary,#e2e8f0); }
.num-s { font-size:12px; color:var(--text-tertiary,#64748b); }
.text-dim { color:var(--text-tertiary,#475569); }
.lat-fast { color:#4ade80; font-weight:500; }
.lat-mid { color:#facc15; font-weight:500; }
.lat-slow { color:#f87171; font-weight:500; }
.act-group { display:flex; gap:4px; align-items:center; }
.btn { display:inline-flex; align-items:center; gap:4px; border-radius:6px; font-size:12px; font-weight:500; cursor:pointer; border:1px solid transparent; transition:all .15s; padding:4px 10px; background:var(--bg-surface,#1e293b); color:var(--text-primary,#e2e8f0); }
.btn-xs { padding:2px 8px; font-size:11px; }
.btn-sm { padding:4px 10px; font-size:12px; }
.btn-md { padding:8px 16px; font-size:13px; }
.btn-primary { background:#6366f1; color:#fff; border-color:#6366f1; }
.btn-primary:hover { background:#5558e6; }
.btn-primary:disabled { opacity:.5; cursor:not-allowed; }
.btn-ghost { background:transparent; color:var(--text-secondary,#94a3b8); border:1px solid var(--border-subtle,#334155); }
.btn-ghost:hover { background:rgba(255,255,255,.05); color:var(--text-primary,#e2e8f0); }
.btn-warn { background:rgba(234,179,8,.12); color:#facc15; border:1px solid rgba(234,179,8,.3); }
.btn-warn:hover { background:rgba(234,179,8,.2); }
.btn-danger-ghost { background:transparent; color:#f87171; border:1px solid rgba(239,68,68,.3); }
.btn-danger-ghost:hover { background:rgba(239,68,68,.08); }
.detail-panel { background:var(--bg-base,#0f172a); border-radius:8px; margin:8px 0; overflow:hidden; animation:slideDown .2s; }
@keyframes slideDown { from{opacity:0;transform:translateY(-4px)} to{opacity:1;transform:translateY(0)} }
.dtabs { display:flex; border-bottom:1px solid var(--border-subtle,#334155); padding:0 12px; }
.dtab { padding:8px 14px; font-size:13px; font-weight:500; color:var(--text-tertiary,#64748b); background:none; border:none; border-bottom:2px solid transparent; cursor:pointer; transition:all .15s; }
.dtab:hover { color:var(--text-primary,#e2e8f0); }
.dtab.active { color:#a5b4fc; border-bottom-color:#6366f1; }
.dtab-body { padding:12px 16px; min-height:80px; }
.dtab-empty { text-align:center; color:var(--text-tertiary,#475569); padding:32px 0; font-size:13px; }
.inner-table { width:100%; border-collapse:collapse; }
.inner-table th { padding:6px 10px; text-align:left; font-size:11px; font-weight:600; color:var(--text-tertiary,#64748b); text-transform:uppercase; letter-spacing:.04em; border-bottom:1px solid rgba(51,65,85,.5); }
.inner-table td { padding:6px 10px; font-size:12px; color:var(--text-secondary,#94a3b8); border-bottom:1px solid rgba(51,65,85,.3); }
.inner-table tbody tr { cursor:default; }
.ss { display:inline-block; padding:1px 6px; border-radius:9999px; font-size:11px; font-weight:600; }
.ss-on { background:rgba(34,197,94,.12); color:#4ade80; }
.ss-off { background:rgba(100,116,139,.08); color:#64748b; }
.diag-body { display:grid; grid-template-columns:1fr 1fr; gap:16px; }
@media(max-width:768px) { .diag-body{grid-template-columns:1fr} }
.diag-card { background:var(--bg-surface,#1e293b); border:1px solid var(--border-subtle,#334155); border-radius:8px; padding:16px; }
.diag-card h4 { font-size:14px; font-weight:600; color:var(--text-primary,#e2e8f0); margin-bottom:12px; }
.diag-steps { margin-top:16px; display:flex; flex-direction:column; gap:10px; }
.diag-step { display:flex; align-items:flex-start; gap:10px; padding:8px 10px; border-radius:6px; font-size:13px; color:var(--text-secondary,#94a3b8); }
.ds-icon { font-size:16px; flex-shrink:0; margin-top:1px; }
.ds-detail { font-size:12px; color:var(--text-tertiary,#64748b); }
.ds-ok { background:rgba(34,197,94,.06); }
.ds-fail { background:rgba(239,68,68,.06); }
.ds-skip { background:rgba(100,116,139,.04); opacity:.6; }
.tk-row { display:flex; align-items:center; gap:8px; margin-bottom:12px; font-size:13px; color:var(--text-secondary,#94a3b8); }
.tk-actions { display:flex; gap:8px; }
.agents-section { background:var(--bg-surface,#1e293b); border:1px solid var(--border-subtle,#334155); border-radius:12px; padding:20px; margin-bottom:20px; }
.badge-count { background:rgba(99,102,241,.15); color:#a5b4fc; font-size:11px; font-weight:600; padding:1px 7px; border-radius:9999px; margin-left:6px; }
.agent-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(240px,1fr)); gap:10px; margin-top:12px; }
.agent-card { display:flex; align-items:center; gap:10px; padding:10px 14px; background:var(--bg-base,#0f172a); border:1px solid var(--border-subtle,#334155); border-radius:8px; transition:all .2s; }
.agent-card:hover { border-color:rgba(99,102,241,.3); }
.agent-card.ag-active { border-color:rgba(34,197,94,.3); }
.ag-avatar { width:32px; height:32px; border-radius:8px; display:flex; align-items:center; justify-content:center; color:#fff; font-weight:700; font-size:14px; flex-shrink:0; }
.ag-info { flex:1; min-width:0; }
.ag-name { font-size:13px; font-weight:600; color:var(--text-primary,#e2e8f0); white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
.ag-meta { display:flex; gap:6px; margin-top:2px; }
.ag-gateway { font-size:11px; color:var(--text-tertiary,#64748b); }
.ag-model { font-size:11px; color:var(--text-tertiary,#475569); }
.ag-status { font-size:11px; font-weight:600; padding:1px 6px; border-radius:9999px; }
.ag-on { background:rgba(34,197,94,.12); color:#4ade80; }
.ag-off { background:rgba(100,116,139,.08); color:#64748b; }

/* Modal */
.modal-overlay { position:fixed; inset:0; background:rgba(0,0,0,.6); backdrop-filter:blur(4px); display:flex; align-items:center; justify-content:center; z-index:1000; animation:fadeIn .15s; }
.modal-box { background:var(--bg-surface,#1e293b); border:1px solid var(--border-subtle,#334155); border-radius:16px; width:460px; max-width:95vw; box-shadow:0 25px 50px rgba(0,0,0,.5); }
.modal-head { display:flex; align-items:center; justify-content:space-between; padding:16px 20px; border-bottom:1px solid var(--border-subtle,#334155); }
.modal-head h3 { font-size:15px; font-weight:700; color:var(--text-primary,#e2e8f0); }
.modal-body { padding:16px 20px; }
.modal-desc { font-size:13px; color:var(--text-secondary,#94a3b8); margin-bottom:16px; line-height:1.5; }
.form-label { display:block; font-size:11px; font-weight:600; color:var(--text-secondary,#94a3b8); margin-bottom:6px; text-transform:uppercase; letter-spacing:.04em; }
.input-wrap { position:relative; }
.form-input { width:100%; padding:8px 40px 8px 12px; background:var(--bg-base,#0f172a); border:1px solid var(--border-subtle,#334155); border-radius:8px; color:var(--text-primary,#e2e8f0); font-size:13px; font-family:'SF Mono',Menlo,monospace; outline:none; transition:border-color .15s; box-sizing:border-box; }
.form-input:focus { border-color:#6366f1; box-shadow:0 0 0 3px rgba(99,102,241,.1); }
.eye-btn { position:absolute; right:8px; top:50%; transform:translateY(-50%); background:none; border:none; cursor:pointer; font-size:14px; padding:2px; }
.test-result { margin-top:12px; padding:8px 12px; border-radius:6px; font-size:13px; }
.tr-ok { background:rgba(34,197,94,.08); border:1px solid rgba(34,197,94,.2); color:#4ade80; }
.tr-err { background:rgba(239,68,68,.08); border:1px solid rgba(239,68,68,.2); color:#f87171; }
.modal-hint { font-size:11px; color:#facc15; margin-top:8px; padding:6px 8px; background:rgba(234,179,8,.06); border-radius:4px; }
.modal-foot { display:flex; justify-content:flex-end; gap:8px; padding:12px 20px; border-top:1px solid var(--border-subtle,#334155); }

/* Toast */
.toast { position:fixed; bottom:24px; right:24px; background:var(--bg-surface,#1e293b); border:1px solid var(--border-subtle,#334155); border-radius:8px; padding:10px 16px; font-size:13px; z-index:1100; max-width:360px; box-shadow:0 10px 25px rgba(0,0,0,.3); cursor:pointer; }
.toast-enter-active { animation:toastIn .3s ease-out; }
.toast-leave-active { animation:toastIn .2s reverse; }
@keyframes toastIn { from{opacity:0;transform:translateY(20px)} to{opacity:1;transform:translateY(0)} }
.toast-success { border-color:#22c55e; color:#4ade80; }
.toast-error { border-color:#ef4444; color:#f87171; }
.toast-info { border-color:#6366f1; color:#a5b4fc; }
</style>