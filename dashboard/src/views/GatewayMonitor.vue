<template>
  <div class="gateway-monitor">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title"><Icon name="activity" :size="20" /> Gateway 监控中心</h1>
      <div class="page-actions">
        <button class="btn btn-sm" @click="loadOverview" :disabled="loading">
          <Icon name="refresh" :size="14" /> 刷新
        </button>
      </div>
    </div>

    <!-- 空状态引导 -->
    <div v-if="!loading && upstreams.length === 0" class="empty-guide">
      <div class="empty-guide-icon">🔗</div>
      <h2 class="empty-guide-title">连接你的 OpenClaw 实例</h2>
      <p class="empty-guide-desc">
        龙虾卫士可以监控所有上游 OpenClaw Gateway 的运行状态。<br/>
        请先在上游管理中添加上游节点。
      </p>
      <div class="empty-guide-actions">
        <router-link to="/upstream" class="btn btn-primary">
          <Icon name="server" :size="14" /> 前往上游管理
        </router-link>
      </div>
    </div>

    <!-- 所有上游都没配 Token 的引导 -->
    <div v-else-if="!loading && upstreams.length > 0 && tokenConfigured === 0" class="empty-guide">
      <div class="empty-guide-icon">🔑</div>
      <h2 class="empty-guide-title">配置 Gateway Token</h2>
      <p class="empty-guide-desc">
        检测到 {{ upstreams.length }} 个上游，但均未配置 Gateway Token。<br/>
        配置 Token 后即可监控 OpenClaw Gateway 的会话、定时任务和运行状态。
      </p>
      <div class="empty-guide-actions">
        <button class="btn btn-primary" @click="openTokenModal(upstreams[0])">
          <Icon name="settings" :size="14" /> 配置第一个 Token
        </button>
        <a href="https://docs.openclaw.dev/gateway/auth" target="_blank" class="btn btn-ghost">了解更多</a>
      </div>
    </div>

    <!-- 主内容 -->
    <template v-else>
      <!-- 聚合概览卡片 -->
      <div class="stat-cards" v-if="!loading">
        <StatCard label="上游总数" :value="overview.total || 0" color="blue"
          :iconSvg="'<path d=\'M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2\'/><circle cx=\'12\' cy=\'7\' r=\'4\'/>'" />
        <StatCard label="在线" :value="overview.online || 0" color="green"
          :iconSvg="'<path d=\'M22 11.08V12a10 10 0 1 1-5.93-9.14\'/><polyline points=\'22 4 12 14.01 9 11.01\'/>'" />
        <StatCard label="离线" :value="overview.offline || 0" color="red"
          :iconSvg="'<circle cx=\'12\' cy=\'12\' r=\'10\'/><line x1=\'15\' y1=\'9\' x2=\'9\' y2=\'15\'/><line x1=\'9\' y1=\'9\' x2=\'15\' y2=\'15\'/>'" />
        <StatCard label="已配 Token" :value="tokenConfigured" color="indigo"
          :iconSvg="'<rect x=\'3\' y=\'11\' width=\'18\' height=\'11\' rx=\'2\' ry=\'2\'/><path d=\'M7 11V7a5 5 0 0 1 10 0v4\'/>'" />
        <StatCard label="总会话数" :value="overview.total_sessions || 0" color="blue"
          :iconSvg="'<path d=\'M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z\'/>'" />
        <StatCard label="活跃会话" :value="overview.active_sessions || 0" color="green"
          :iconSvg="'<polygon points=\'13 2 3 14 12 14 11 22 21 10 12 10 13 2\'/>'" />
      </div>

      <!-- 加载骨架 -->
      <div v-if="loading" class="stat-cards">
        <Skeleton height="90px" v-for="i in 6" :key="i" />
      </div>

      <!-- 上游列表 -->
      <div class="card" style="margin-top: var(--space-3)">
        <div class="card-header">
          <h3 class="card-title">上游 Gateway 实例</h3>
        </div>
        <div v-if="loading" style="padding: var(--space-4)">
          <Skeleton height="40px" v-for="i in 3" :key="i" style="margin-bottom: var(--space-2)" />
        </div>
        <table v-else class="gw-table">
          <thead>
            <tr>
              <th>名称/ID</th>
              <th>地址</th>
              <th>健康状态</th>
              <th>Gateway 连接</th>
              <th>会话数</th>
              <th>延迟</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="up in upstreams" :key="up.id"
                :class="{ 'row-selected': selectedUpstream === up.id }"
                @click="selectUpstream(up)">
              <td>
                <div class="upstream-name">
                  <strong>{{ up.id }}</strong>
                </div>
              </td>
              <td>
                <code class="addr-code">{{ up.address }}:{{ up.port }}</code>
              </td>
              <td>
                <span class="health-dot" :class="up.healthy ? 'dot-ok' : 'dot-err'"></span>
                {{ up.healthy ? '健康' : '异常' }}
              </td>
              <td>
                <span class="status-tag" :class="'status-' + up.gateway_status">
                  {{ statusLabel(up.gateway_status) }}
                </span>
              </td>
              <td>
                <template v-if="up.gateway_status === 'connected'">
                  {{ up.session_count }}<span class="text-muted"> / {{ up.active_sessions }} 活跃</span>
                </template>
                <template v-else>--</template>
              </td>
              <td>
                <template v-if="up.latency_ms > 0">{{ up.latency_ms }}ms</template>
                <template v-else>--</template>
              </td>
              <td>
                <div class="action-btns" @click.stop>
                  <button v-if="!up.token_configured" class="btn btn-xs btn-primary" @click="openTokenModal(up)">
                    配置 Token
                  </button>
                  <button v-else-if="up.gateway_status === 'auth_failed'" class="btn btn-xs btn-warning" @click="openTokenModal(up)">
                    重新配置
                  </button>
                  <button v-else class="btn btn-xs btn-ghost" @click="openTokenModal(up)">
                    <Icon name="settings" :size="12" />
                  </button>
                  <button class="btn btn-xs btn-ghost" @click="selectUpstream(up)" :title="selectedUpstream === up.id ? '收起详情' : '查看详情'">
                    <Icon name="chevron-right" :size="12" :class="{ 'chevron-open': selectedUpstream === up.id }" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 详情面板 -->
      <div v-if="selectedUpstream && selectedDetail" class="detail-panel card" style="margin-top: var(--space-3)">
        <div class="detail-header">
          <h3 class="card-title">{{ selectedUpstream }} 详情</h3>
          <button class="btn btn-xs btn-ghost" @click="selectedUpstream = null">
            <Icon name="x-circle" :size="14" />
          </button>
        </div>

        <!-- Tabs -->
        <div class="detail-tabs">
          <button v-for="tab in detailTabs" :key="tab.key"
                  class="detail-tab" :class="{ active: activeTab === tab.key }"
                  @click="switchTab(tab.key)">
            {{ tab.label }}
          </button>
        </div>

        <!-- Tab: Sessions -->
        <div v-if="activeTab === 'sessions'" class="detail-content">
          <div v-if="detailLoading" style="padding: var(--space-4)">
            <Skeleton height="32px" v-for="i in 4" :key="i" style="margin-bottom: var(--space-2)" />
          </div>
          <div v-else-if="!sessions || sessions.length === 0" class="detail-empty">
            暂无会话数据
          </div>
          <table v-else class="gw-table gw-table-inner">
            <thead>
              <tr>
                <th>Key</th>
                <th>Agent</th>
                <th>状态</th>
                <th>Model</th>
                <th>Token 用量</th>
                <th>最后活跃</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(s, i) in sessions" :key="i">
                <td><code>{{ s.key || s.session_id || s.id || '--' }}</code></td>
                <td>{{ s.agent_id || s.agentId || '--' }}</td>
                <td>
                  <span class="session-state" :class="isActiveState(s.state) ? 'state-active' : 'state-idle'">
                    {{ s.state || 'unknown' }}
                  </span>
                </td>
                <td>{{ s.model || '--' }}</td>
                <td>{{ formatTokens(s) }}</td>
                <td>{{ fmtTime(s.last_active || s.lastActive || s.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Tab: Cron -->
        <div v-if="activeTab === 'cron'" class="detail-content">
          <div v-if="detailLoading" style="padding: var(--space-4)">
            <Skeleton height="32px" v-for="i in 3" :key="i" style="margin-bottom: var(--space-2)" />
          </div>
          <div v-else-if="!cronJobs || cronJobs.length === 0" class="detail-empty">
            暂无定时任务
          </div>
          <table v-else class="gw-table gw-table-inner">
            <thead>
              <tr>
                <th>名称</th>
                <th>状态</th>
                <th>Schedule</th>
                <th>下次运行</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(c, i) in cronJobs" :key="i">
                <td>{{ c.name || c.id || '--' }}</td>
                <td>
                  <span class="status-tag" :class="c.enabled !== false ? 'status-connected' : 'status-not_configured'">
                    {{ c.enabled !== false ? '启用' : '禁用' }}
                  </span>
                </td>
                <td><code>{{ c.schedule || c.cron || '--' }}</code></td>
                <td>{{ fmtTime(c.next_run || c.nextRun) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Tab: Config -->
        <div v-if="activeTab === 'config'" class="detail-content">
          <div class="config-section">
            <h4 class="config-title">连接信息</h4>
            <div class="config-grid">
              <div class="config-item">
                <span class="config-label">地址</span>
                <span class="config-value"><code>{{ selectedDetail.address }}:{{ selectedDetail.port }}</code></span>
              </div>
              <div class="config-item">
                <span class="config-label">延迟</span>
                <span class="config-value">{{ selectedDetail.latency_ms > 0 ? selectedDetail.latency_ms + 'ms' : '--' }}</span>
              </div>
              <div class="config-item">
                <span class="config-label">Gateway 状态</span>
                <span class="config-value">
                  <span class="status-tag" :class="'status-' + selectedDetail.gateway_status">
                    {{ statusLabel(selectedDetail.gateway_status) }}
                  </span>
                </span>
              </div>
              <div class="config-item">
                <span class="config-label">Token 配置</span>
                <span class="config-value">{{ selectedDetail.token_configured ? '已配置' : '未配置' }}</span>
              </div>
            </div>

            <h4 class="config-title" style="margin-top: var(--space-5)">Gateway Token</h4>
            <div class="token-actions">
              <button class="btn btn-sm btn-primary" @click="openTokenModal(selectedDetail)">
                {{ selectedDetail.token_configured ? '更新 Token' : '配置 Token' }}
              </button>
              <button v-if="selectedDetail.token_configured" class="btn btn-sm btn-ghost" style="color: var(--color-danger)" @click="clearToken(selectedDetail)">
                清除 Token
              </button>
              <button v-if="selectedDetail.token_configured" class="btn btn-sm btn-ghost" @click="testPing(selectedDetail.id)">
                <Icon name="zap" :size="12" /> 测试连接
              </button>
            </div>
            <div v-if="pingResult" class="ping-result" :class="pingResult.authenticated ? 'ping-ok' : 'ping-err'">
              <template v-if="pingResult.reachable && pingResult.authenticated">
                ✅ 连接成功 — 延迟 {{ pingResult.latency_ms }}ms
              </template>
              <template v-else-if="pingResult.reachable && !pingResult.authenticated">
                ⚠️ 可达但认证失败 — {{ pingResult.message }}
              </template>
              <template v-else>
                ❌ 不可达 — {{ pingResult.message || pingResult.error }}
              </template>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- Token 配置弹窗 -->
    <div v-if="tokenModal.show" class="modal-overlay" @click.self="tokenModal.show = false">
      <div class="modal-box">
        <div class="modal-header">
          <h3>{{ tokenModal.upstream?.token_configured ? '更新' : '配置' }} Gateway Token</h3>
          <button class="btn btn-ghost btn-xs" @click="tokenModal.show = false">
            <Icon name="x-circle" :size="16" />
          </button>
        </div>
        <div class="modal-body">
          <p class="modal-desc">
            为上游 <strong>{{ tokenModal.upstream?.id }}</strong> ({{ tokenModal.upstream?.address }}:{{ tokenModal.upstream?.port }}) 配置 OpenClaw Gateway Token。
          </p>
          <div class="form-group">
            <label class="form-label">Gateway Token</label>
            <div class="input-password-wrap">
              <input
                :type="tokenModal.showToken ? 'text' : 'password'"
                v-model="tokenModal.token"
                class="form-input"
                placeholder="输入 OpenClaw Gateway Auth Token"
                autocomplete="off"
              />
              <button class="btn btn-ghost btn-xs toggle-eye" @click="tokenModal.showToken = !tokenModal.showToken">
                <Icon :name="tokenModal.showToken ? 'eye' : 'eye'" :size="14" />
              </button>
            </div>
          </div>

          <!-- 测试连接结果 -->
          <div v-if="tokenModal.testResult" class="ping-result"
               :class="tokenModal.testResult.authenticated ? 'ping-ok' : 'ping-err'">
            <template v-if="tokenModal.testResult.reachable && tokenModal.testResult.authenticated">
              ✅ 连接成功 — 延迟 {{ tokenModal.testResult.latency_ms }}ms
            </template>
            <template v-else-if="tokenModal.testResult.reachable && !tokenModal.testResult.authenticated">
              ⚠️ 可达但认证失败，Token 可能无效
            </template>
            <template v-else-if="tokenModal.testResult.error === 'gateway_token_not_configured'">
              ⚠️ 请先输入 Token 再测试
            </template>
            <template v-else>
              ❌ 不可达 — {{ tokenModal.testResult.message || '网络错误' }}
            </template>
          </div>

          <div v-if="tokenModal.testResult && !tokenModal.testResult.authenticated" class="modal-warning">
            ⚠️ 测试失败不影响保存，Token 可能正确但网络暂时不通。
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="testTokenInModal" :disabled="tokenModal.testing || !tokenModal.token">
            {{ tokenModal.testing ? '测试中...' : '测试连接' }}
          </button>
          <button class="btn btn-primary" @click="saveToken" :disabled="tokenModal.saving || !tokenModal.token">
            {{ tokenModal.saving ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Toast -->
    <div v-if="toast.show" class="toast" :class="'toast-' + toast.type" @click="toast.show = false">
      {{ toast.message }}
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { api, apiPut, apiDelete } from '../api.js'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import Skeleton from '../components/Skeleton.vue'

const loading = ref(true)
const detailLoading = ref(false)
const overview = ref({})
const upstreams = ref([])
const selectedUpstream = ref(null)
const selectedDetail = ref(null)
const activeTab = ref('sessions')
const sessions = ref([])
const cronJobs = ref([])
const pingResult = ref(null)

// Toast
const toast = reactive({ show: false, message: '', type: 'info' })
let toastTimer = null

const tokenConfigured = computed(() => {
  return upstreams.value.filter(u => u.token_configured).length
})

const detailTabs = [
  { key: 'sessions', label: '会话' },
  { key: 'cron', label: '定时任务' },
  { key: 'config', label: '配置' },
]

const tokenModal = reactive({
  show: false,
  upstream: null,
  token: '',
  showToken: false,
  testing: false,
  saving: false,
  testResult: null,
})

function statusLabel(status) {
  const labels = {
    connected: '已连接',
    not_configured: '未配置',
    auth_failed: '认证失败',
    unreachable: '不可达',
    error: '错误',
  }
  return labels[status] || status || '未知'
}

function isActiveState(state) {
  return ['running', 'active', 'busy', 'working', 'in_progress', 'processing', 'streaming'].includes(state)
}

function formatTokens(s) {
  const total = s.total_tokens || s.totalTokens || s.tokens || 0
  if (!total) return '--'
  if (total > 1000000) return (total / 1000000).toFixed(1) + 'M'
  if (total > 1000) return (total / 1000).toFixed(1) + 'K'
  return total
}

function fmtTime(ts) {
  if (!ts) return '--'
  try {
    const d = new Date(ts)
    if (isNaN(d.getTime())) return '--'
    const now = new Date()
    const diff = (now - d) / 1000
    if (diff < 60) return '刚刚'
    if (diff < 3600) return Math.floor(diff / 60) + '分钟前'
    if (diff < 86400) return Math.floor(diff / 3600) + '小时前'
    return d.toLocaleDateString('zh-CN') + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  } catch {
    return '--'
  }
}

async function loadOverview() {
  loading.value = true
  try {
    const data = await api('/api/v1/upstreams/gateway/overview')
    overview.value = data
    upstreams.value = (data.upstreams || []).map(u => ({
      ...u,
      gateway_status: u.gateway_status || 'unknown',
    }))
  } catch (e) {
    console.error('加载 Gateway 概览失败:', e)
    showToast('加载失败: ' + e.message, 'error')
  } finally {
    loading.value = false
  }
}

function selectUpstream(up) {
  if (selectedUpstream.value === up.id) {
    selectedUpstream.value = null
    selectedDetail.value = null
    return
  }
  selectedUpstream.value = up.id
  selectedDetail.value = up
  activeTab.value = 'sessions'
  pingResult.value = null
  loadTabData(up.id, 'sessions')
}

async function switchTab(tab) {
  activeTab.value = tab
  if (selectedUpstream.value && tab !== 'config') {
    await loadTabData(selectedUpstream.value, tab)
  }
}

async function loadTabData(id, tab) {
  detailLoading.value = true
  try {
    if (tab === 'sessions') {
      const data = await api(`/api/v1/upstreams/${encodeURIComponent(id)}/gateway/sessions`)
      if (data.error) {
        sessions.value = []
      } else {
        sessions.value = data.sessions || data || []
        if (!Array.isArray(sessions.value)) sessions.value = []
      }
    } else if (tab === 'cron') {
      const data = await api(`/api/v1/upstreams/${encodeURIComponent(id)}/gateway/cron`)
      if (data.error) {
        cronJobs.value = []
      } else {
        cronJobs.value = data.jobs || data.crons || data || []
        if (!Array.isArray(cronJobs.value)) cronJobs.value = []
      }
    }
  } catch (e) {
    console.error(`加载 ${tab} 数据失败:`, e)
    if (tab === 'sessions') sessions.value = []
    if (tab === 'cron') cronJobs.value = []
  } finally {
    detailLoading.value = false
  }
}

async function testPing(id) {
  pingResult.value = null
  try {
    pingResult.value = await api(`/api/v1/upstreams/${encodeURIComponent(id)}/gateway/ping`)
  } catch (e) {
    pingResult.value = { reachable: false, authenticated: false, message: e.message }
  }
}

function openTokenModal(up) {
  tokenModal.upstream = up
  tokenModal.token = ''
  tokenModal.showToken = false
  tokenModal.testing = false
  tokenModal.saving = false
  tokenModal.testResult = null
  tokenModal.show = true
}

async function testTokenInModal() {
  if (!tokenModal.token || !tokenModal.upstream) return
  tokenModal.testing = true
  tokenModal.testResult = null
  try {
    // 先临时保存 token，再测试
    await apiPut(`/api/v1/upstreams/${encodeURIComponent(tokenModal.upstream.id)}/gateway-token`, {
      token: tokenModal.token
    })
    tokenModal.testResult = await api(`/api/v1/upstreams/${encodeURIComponent(tokenModal.upstream.id)}/gateway/ping`)
  } catch (e) {
    tokenModal.testResult = { reachable: false, authenticated: false, message: e.message }
  } finally {
    tokenModal.testing = false
  }
}

async function saveToken() {
  if (!tokenModal.token || !tokenModal.upstream) return
  tokenModal.saving = true
  try {
    await apiPut(`/api/v1/upstreams/${encodeURIComponent(tokenModal.upstream.id)}/gateway-token`, {
      token: tokenModal.token
    })
    showToast('Token 已保存', 'success')
    tokenModal.show = false
    await loadOverview()
  } catch (e) {
    showToast('保存失败: ' + e.message, 'error')
  } finally {
    tokenModal.saving = false
  }
}

async function clearToken(up) {
  if (!confirm(`确定清除 ${up.id} 的 Gateway Token？`)) return
  try {
    await apiDelete(`/api/v1/upstreams/${encodeURIComponent(up.id)}/gateway-token`)
    showToast('Token 已清除', 'success')
    await loadOverview()
  } catch (e) {
    showToast('清除失败: ' + e.message, 'error')
  }
}

function showToast(msg, type = 'info') {
  toast.message = msg
  toast.type = type
  toast.show = true
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toast.show = false }, 3000)
}

onMounted(() => {
  loadOverview()
})
</script>

<style scoped>
.gateway-monitor {
  padding: var(--space-4) var(--space-6);
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-4);
}
.page-title {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-xl);
  font-weight: 700;
  color: var(--text-primary);
}
.page-actions {
  display: flex;
  gap: var(--space-2);
}

/* 统计卡片 */
.stat-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}

/* 空状态引导 */
.empty-guide {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-8) var(--space-4);
  text-align: center;
  min-height: 400px;
}
.empty-guide-icon {
  font-size: 48px;
  margin-bottom: var(--space-4);
}
.empty-guide-title {
  font-size: var(--text-xl);
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: var(--space-2);
}
.empty-guide-desc {
  color: var(--text-secondary);
  font-size: var(--text-sm);
  line-height: 1.6;
  max-width: 480px;
  margin-bottom: var(--space-5);
}
.empty-guide-actions {
  display: flex;
  gap: var(--space-3);
}

/* Card */
.card {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  overflow: hidden;
}
.card-header {
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}
.card-title {
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--text-primary);
}

/* 表格 */
.gw-table {
  width: 100%;
  border-collapse: collapse;
}
.gw-table th {
  padding: var(--space-2) var(--space-3);
  text-align: left;
  font-size: var(--text-xs);
  font-weight: 600;
  color: var(--text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid var(--border-subtle);
}
.gw-table td {
  padding: var(--space-2) var(--space-3);
  font-size: var(--text-sm);
  color: var(--text-secondary);
  border-bottom: 1px solid var(--border-subtle);
}
.gw-table tbody tr {
  cursor: pointer;
  transition: background var(--transition-fast);
}
.gw-table tbody tr:hover {
  background: rgba(99, 102, 241, 0.05);
}
.gw-table tbody tr.row-selected {
  background: rgba(99, 102, 241, 0.1);
}
.gw-table-inner {
  margin: 0;
}
.gw-table-inner tbody tr {
  cursor: default;
}

.addr-code {
  font-size: var(--text-xs);
  font-family: var(--font-mono);
  background: rgba(99, 102, 241, 0.08);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  color: #a5b4fc;
}

/* 健康状态圆点 */
.health-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
}
.dot-ok {
  background: #22c55e;
  box-shadow: 0 0 6px rgba(34, 197, 94, 0.4);
}
.dot-err {
  background: #ef4444;
  box-shadow: 0 0 6px rgba(239, 68, 68, 0.4);
}

/* 状态标签 */
.status-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 9999px;
  font-size: var(--text-xs);
  font-weight: 600;
}
.status-connected {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}
.status-not_configured {
  background: rgba(100, 116, 139, 0.15);
  color: #64748b;
}
.status-auth_failed {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}
.status-unreachable {
  background: rgba(234, 179, 8, 0.15);
  color: #eab308;
}
.status-error {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}
.status-unknown {
  background: rgba(100, 116, 139, 0.1);
  color: #64748b;
}

/* 操作按钮 */
.action-btns {
  display: flex;
  gap: var(--space-1);
  align-items: center;
}
.btn-warning {
  background: rgba(234, 179, 8, 0.15);
  color: #eab308;
  border: 1px solid rgba(234, 179, 8, 0.3);
}
.btn-warning:hover {
  background: rgba(234, 179, 8, 0.25);
}
.chevron-open {
  transform: rotate(90deg);
  transition: transform var(--transition-fast);
}

.text-muted {
  color: var(--text-tertiary);
  font-size: var(--text-xs);
}

/* 详情面板 */
.detail-panel {
  animation: slideDown 0.2s ease;
}
@keyframes slideDown {
  from { opacity: 0; transform: translateY(-8px); }
  to { opacity: 1; transform: translateY(0); }
}
.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}
.detail-tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--border-subtle);
  padding: 0 var(--space-4);
}
.detail-tab {
  padding: var(--space-2) var(--space-4);
  font-size: var(--text-sm);
  font-weight: 500;
  color: var(--text-tertiary);
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: all var(--transition-fast);
}
.detail-tab:hover {
  color: var(--text-primary);
}
.detail-tab.active {
  color: #6366f1;
  border-bottom-color: #6366f1;
}
.detail-content {
  padding: var(--space-3) var(--space-4);
  min-height: 120px;
}
.detail-empty {
  text-align: center;
  color: var(--text-tertiary);
  padding: var(--space-6) 0;
  font-size: var(--text-sm);
}

/* 会话状态 */
.session-state {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 9999px;
  font-size: var(--text-xs);
  font-weight: 600;
}
.state-active {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}
.state-idle {
  background: rgba(100, 116, 139, 0.1);
  color: #64748b;
}

/* 配置面板 */
.config-section {
  padding: var(--space-2) 0;
}
.config-title {
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: var(--space-3);
}
.config-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: var(--space-3);
}
.config-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.config-label {
  font-size: var(--text-xs);
  color: var(--text-tertiary);
  font-weight: 500;
}
.config-value {
  font-size: var(--text-sm);
  color: var(--text-primary);
}
.token-actions {
  display: flex;
  gap: var(--space-2);
  margin-top: var(--space-2);
}

/* Ping 结果 */
.ping-result {
  margin-top: var(--space-3);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
  animation: fadeIn 0.2s ease;
}
@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}
.ping-ok {
  background: rgba(34, 197, 94, 0.1);
  border: 1px solid rgba(34, 197, 94, 0.2);
  color: #22c55e;
}
.ping-err {
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.2);
  color: #ef4444;
}

/* Modal */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  animation: fadeIn 0.15s ease;
}
.modal-box {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-xl);
  width: 480px;
  max-width: 95vw;
  box-shadow: 0 25px 50px rgba(0, 0, 0, 0.5);
}
.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--border-subtle);
}
.modal-header h3 {
  font-size: var(--text-base);
  font-weight: 700;
  color: var(--text-primary);
}
.modal-body {
  padding: var(--space-4) var(--space-5);
}
.modal-desc {
  font-size: var(--text-sm);
  color: var(--text-secondary);
  margin-bottom: var(--space-4);
  line-height: 1.5;
}
.modal-warning {
  font-size: var(--text-xs);
  color: #eab308;
  margin-top: var(--space-2);
  padding: var(--space-2);
  background: rgba(234, 179, 8, 0.08);
  border-radius: var(--radius-md);
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-5);
  border-top: 1px solid var(--border-subtle);
}

/* Form */
.form-group {
  margin-bottom: var(--space-3);
}
.form-label {
  display: block;
  font-size: var(--text-xs);
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: var(--space-1);
}
.form-input {
  width: 100%;
  padding: var(--space-2) var(--space-3);
  background: var(--bg-base);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  color: var(--text-primary);
  font-size: var(--text-sm);
  font-family: var(--font-mono);
  outline: none;
  transition: border-color var(--transition-fast);
}
.form-input:focus {
  border-color: #6366f1;
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.1);
}
.input-password-wrap {
  position: relative;
}
.input-password-wrap .form-input {
  padding-right: 40px;
}
.toggle-eye {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-tertiary);
}

/* Buttons */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all var(--transition-fast);
  background: var(--bg-surface);
  color: var(--text-primary);
}
.btn:hover {
  background: rgba(255, 255, 255, 0.05);
}
.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.btn-primary {
  background: #6366f1;
  color: white;
  border-color: #6366f1;
}
.btn-primary:hover {
  background: #5558e6;
}
.btn-ghost {
  background: transparent;
  color: var(--text-secondary);
}
.btn-ghost:hover {
  background: rgba(255, 255, 255, 0.05);
  color: var(--text-primary);
}
.btn-sm {
  padding: 4px 10px;
  font-size: var(--text-xs);
}
.btn-xs {
  padding: 2px 6px;
  font-size: 11px;
}
.upstream-name strong {
  color: var(--text-primary);
}

/* Toast */
.toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  padding: var(--space-3) var(--space-4);
  color: var(--text-primary);
  font-size: var(--text-sm);
  z-index: 1100;
  max-width: 360px;
  animation: toastIn 0.3s ease-out;
  cursor: pointer;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.3);
}
@keyframes toastIn {
  from { opacity: 0; transform: translateY(20px); }
  to { opacity: 1; transform: translateY(0); }
}
.toast-success {
  border-color: #22c55e;
  color: #22c55e;
}
.toast-error {
  border-color: #ef4444;
  color: #ef4444;
}
.toast-info {
  border-color: #6366f1;
  color: #a5b4fc;
}
</style>