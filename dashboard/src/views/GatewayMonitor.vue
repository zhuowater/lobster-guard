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
                        <button class="btn btn-xs btn-ghost" @click="quickPing(up)" title="Ping" :disabled="up._pinging"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="13 2 3 14 12 14 11 22"/></svg></button>
                        <button class="btn btn-xs btn-ghost" @click="openTokenModal(up)" title="Token"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg></button>
                      </template>
                    </div>
                  </td>
                </tr>
                <!-- 展开行 -->
                <tr v-if="expandedId === up.id" class="expand-row">
                  <td colspan="8">
                    <div class="detail-panel" :key="'detail-'+up.id">
                      <div class="dtabs">
                        <button v-for="t in tabs" :key="t.key" class="dtab" :class="{ active: activeTab === t.key }" @click="switchTab(t.key, up.id)" v-show="t.key !== 'agent' || up.gateway_status === 'connected'"><span class="dtab-icon" v-html="t.icon"></span> {{ t.label }}</button>
                      </div>
                      <!-- Sessions -->
                      <div v-if="activeTab === 'sessions'" class="dtab-body">
                        <div v-if="detailLoading" class="skel-lines"><div class="skel-line" v-for="i in 4" :key="i"></div></div>
                        <div v-else-if="sessions.length === 0" class="dtab-empty">暂无会话</div>
                        <table v-else class="inner-table">
                          <thead><tr><th style="width:24px"></th><th>Key</th><th>Channel</th><th>Model</th><th>Token</th><th>上下文</th><th>最后活跃</th><th>操作</th></tr></thead>
                          <tbody>
                            <template v-for="s in sessions" :key="s.key||s.sessionId">
                              <tr class="session-row" :class="{ 'row-active': expandedSessionKey === (s.key||s.sessionId) }" @click="toggleSessionHistory(s)">
                                <td><svg class="expand-chevron" :class="{ open: expandedSessionKey === (s.key||s.sessionId) }" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg></td>
                                <td><code class="mono-sm">{{ truncKey(s.key||s.sessionId) }}</code></td>
                                <td>{{ s.channel||s.lastChannel||'—' }}</td>
                                <td class="text-dim">{{ s.model||'—' }}</td>
                                <td>{{ fmtTokens(s) }}</td>
                                <td>{{ fmtContext(s) }}</td>
                                <td>{{ fmtTime(s.updatedAt||s.updated_at) }}</td>
                                <td @click.stop>
                                  <div class="act-group">
                                    <button class="btn btn-xs btn-ghost" title="压缩上下文" @click="sessionCompact(s)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 14 10 14 10 20"/><polyline points="20 10 14 10 14 4"/><line x1="14" y1="10" x2="21" y2="3"/><line x1="3" y1="21" x2="10" y2="14"/></svg></button>
                                    <button class="btn btn-xs btn-ghost" title="重置会话" @click="sessionReset(s)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg></button>
                                    <button class="btn btn-xs btn-ghost btn-danger-ghost" title="删除会话" @click="sessionDelete(s)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></button>
                                  </div>
                                </td>
                              </tr>
                              <tr v-if="expandedSessionKey === (s.key||s.sessionId)" class="session-detail-row">
                                <td colspan="8">
                                  <div class="session-replay">
                                    <div v-if="sessionHistoryLoading" class="skel-lines"><div class="skel-line" v-for="i in 4" :key="i"></div></div>
                                    <div v-else-if="sessionMessages.length === 0" class="dtab-empty">暂无消息</div>
                                    <div v-else class="chat-messages">
                                      <div v-for="(msg, idx) in sessionMessages" :key="idx"
                                           class="chat-msg" :class="'msg-' + msg.role">
                                        <div class="msg-role">{{ msg.role }}</div>
                                        <div class="msg-content">{{ msgExpanded[idx] ? extractContentFull(msg.content) : extractContent(msg.content) }}</div>
                                        <button v-if="extractContentFull(msg.content).length > 500 && !msgExpanded[idx]" class="msg-expand-btn" @click.stop="msgExpanded[idx] = true">展开全文</button>
                                        <button v-if="msgExpanded[idx]" class="msg-expand-btn" @click.stop="msgExpanded[idx] = false">收起</button>
                                        <div class="msg-meta" v-if="msg.timestamp">{{ fmtTime(msg.timestamp) }}</div>
                                      </div>
                                    </div>
                                    <!-- 发消息 + 中止 -->
                                    <div class="chat-actions">
                                      <input v-model="chatInput" class="chat-input" placeholder="向此 session 发送消息…" @keyup.enter="chatSend(s)" />
                                      <button class="btn btn-xs btn-primary" @click="chatSend(s)" :disabled="!chatInput.trim() || chatSending">{{ chatSending ? '发送中…' : '发送' }}</button>
                                      <button class="btn btn-xs btn-warn" @click="chatAbort(s)" title="中止生成">中止</button>
                                    </div>
                                  </div>
                                </td>
                              </tr>
                            </template>
                          </tbody>
                        </table>
                      </div>
                      <!-- Cron -->
                      <div v-if="activeTab === 'cron'" class="dtab-body">
                        <div style="display:flex;justify-content:flex-end;margin-bottom:8px">
                          <button class="btn btn-sm btn-primary" @click="openCronModal(null)">+ 新建任务</button>
                        </div>
                        <div v-if="detailLoading" class="skel-lines"><div class="skel-line" v-for="i in 3" :key="i"></div></div>
                        <div v-else-if="cronJobs.length === 0" class="dtab-empty">暂无定时任务</div>
                        <template v-else>
                        <table class="inner-table">
                          <thead><tr><th>名称</th><th>状态</th><th>计划</th><th>下次运行</th><th>操作</th></tr></thead>
                          <tbody>
                            <template v-for="c in cronJobs" :key="c.id||c.name">
                            <tr>
                              <td>{{ c.name||c.id||'—' }}</td>
                              <td><span class="gw-badge" :class="c.enabled!==false?'gw-connected':'gw-not_configured'">{{ c.enabled!==false?'启用':'禁用' }}</span></td>
                              <td><code class="mono-sm">{{ fmtCronSchedule(c) }}</code></td>
                              <td>{{ fmtTime(c.next_run||c.nextRun||c.nextRunAt) }}</td>
                              <td @click.stop>
                                <div class="act-group">
                                  <button class="btn btn-xs btn-ghost" title="编辑" @click="openCronModal(c)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg></button>
                                  <button class="btn btn-xs btn-ghost" title="立即运行" @click="cronTrigger(c)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg></button>
                                  <button class="btn btn-xs btn-ghost" :title="c.enabled!==false?'禁用':'启用'" @click="cronToggle(c)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line v-if="c.enabled!==false" x1="4.93" y1="4.93" x2="19.07" y2="19.07"/><polyline v-else points="9 12 12 15 16 10"/></svg></button>
                                  <button class="btn btn-xs btn-ghost" title="运行历史" @click="toggleCronRuns(c)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></button>
                                  <button class="btn btn-xs btn-ghost btn-danger-ghost" title="删除" @click="cronRemove(c)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></button>
                                </div>
                              </td>
                            </tr>
                            <!-- cron 运行历史展开 -->
                            <tr v-if="expandedCronId === (c.id||c.jobId)" class="session-detail-row">
                              <td colspan="5">
                                <div v-if="cronRunsLoading" class="skel-lines"><div class="skel-line" v-for="i in 3" :key="i"></div></div>
                                <div v-else-if="cronRuns.length === 0" class="dtab-empty">暂无运行记录</div>
                                <table v-else class="inner-table" style="font-size:12px">
                                  <thead><tr><th>时间</th><th>状态</th><th>耗时</th></tr></thead>
                                  <tbody><tr v-for="(run, ri) in cronRuns" :key="ri">
                                    <td>{{ fmtTime(run.startedAt||run.ts) }}</td>
                                    <td><span class="gw-badge" :class="run.ok||run.status==='ok'?'gw-connected':'gw-error'">{{ run.ok||run.status==='ok'?'成功':'失败' }}</span></td>
                                    <td>{{ run.durationMs ? run.durationMs + 'ms' : '—' }}</td>
                                  </tr></tbody>
                                </table>
                              </td>
                            </tr>
                            </template>
                          </tbody>
                        </table>
                        </template>
                      </div>
                      <!-- 诊断 Tab -->
                      <div v-if="activeTab === 'diag'" class="dtab-body diag-body">
                        <div class="diag-card">
                          <h4>三步快速诊断</h4>
                          <button class="btn btn-sm btn-primary" @click="runDiag(up)" :disabled="diagRunning">{{ diagRunning ? '诊断中...' : '' }}<svg v-if="!diagRunning" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-2px;margin-right:2px"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>{{ diagRunning ? '' : ' 开始诊断' }}</button>
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
                        <!-- Gateway 远程配置 -->
                        <div class="diag-card diag-card-full" v-if="up.gateway_status === 'connected'">
                          <h4>⚙️ Gateway 配置
                            <button class="btn btn-xs btn-ghost" style="float:right" @click="loadGatewayConfig(up)" title="加载配置">
                              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
                            </button>
                          </h4>
                          <div v-if="!gwConfigLoaded" class="tk-actions">
                            <button class="btn btn-sm btn-primary" @click="loadGatewayConfig(up)">加载当前配置</button>
                          </div>
                          <template v-else>
                            <textarea v-model="gwConfigRaw" class="file-textarea" style="min-height:60vh;max-height:80vh;border:1px solid var(--border-subtle,#334155);border-radius:6px;margin:8px 0;width:100%" spellcheck="false" @input="gwConfigDirty=true"></textarea>
                            <div class="tk-actions" style="gap:8px">
                              <button class="btn btn-sm btn-primary" @click="patchGatewayConfig(up)" :disabled="gwConfigSaving || !gwConfigDirty">{{ gwConfigSaving ? '保存中…' : '保存并应用' }}</button>
                              <span v-if="gwConfigDirty" class="badge-unsaved">未保存</span>
                            </div>
                          </template>
                        </div>
                      </div>
                      <!-- Agent Tab (AOC per-upstream) -->
                      <div v-if="activeTab === 'agent'" class="dtab-body aoc-section aoc-inline">
                        <div class="section-header">
                          <h3 class="section-title">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
                            Agent 运营中心 <span class="badge-count">{{ expandedAgents.length }}</span>
                          </h3>
                          <button class="btn btn-xs btn-primary" style="margin-left:auto;margin-right:12px" @click="openAgentModal(null)">+ 新建 Agent</button>
                          <div class="aoc-view-toggle">
                            <button class="aoc-vbtn" :class="{ active: aocView === 'dashboard' }" @click="aocView='dashboard'" title="仪表盘"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg></button>
                            <button class="aoc-vbtn" :class="{ active: aocView === 'cards' }" @click="aocView='cards'" title="详情卡片"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg></button>
                            <button class="aoc-vbtn" :class="{ active: aocView === 'collab' }" @click="aocView='collab'" title="协作视图"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></svg></button>
                            <button class="aoc-vbtn" :class="{ active: aocView === 'users' }" @click="aocView='users'" title="用户归因"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg></button>
                            <button class="aoc-vbtn" :class="{ active: aocView === 'skills' }" @click="switchToSkills" title="Skill 目录"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg></button>
                            <button class="aoc-vbtn" :class="{ active: aocView === 'files' }" @click="switchToFiles" title="文件编辑"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg></button>
                            <button class="aoc-vbtn" :class="{ active: aocView === 'heartbeat' }" @click="switchToHeartbeat" title="心跳/设备"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></button>
                          </div>
                        </div>

                        <!-- ===== 1. 总览仪表盘 ===== -->
                        <div v-if="aocView === 'dashboard'" class="aoc-dashboard">
                          <div class="aoc-stat-strip">
                            <div class="aoc-mini-stat">
                              <div class="ams-value">{{ expandedAgentStats.total }}</div>
                              <div class="ams-label">Agent 总数</div>
                            </div>
                            <div class="aoc-mini-stat">
                              <div class="ams-value ams-green">{{ expandedAgentStats.active }}</div>
                              <div class="ams-label">活跃 Agent</div>
                            </div>
                            <div class="aoc-mini-stat">
                              <div class="ams-value ams-cyan">{{ fmtTokensShort(expandedAgentStats.totalTokens) }}</div>
                              <div class="ams-label">总 Token 消耗</div>
                            </div>
                            <div class="aoc-mini-stat">
                              <div class="ams-value ams-indigo">{{ expandedAgentStats.totalSessions }}</div>
                              <div class="ams-label">总会话数</div>
                            </div>
                            <div class="aoc-mini-stat">
                              <div class="ams-value" :class="expandedAgentStats.abortedCount > 0 ? 'ams-red' : 'ams-green'">{{ expandedAgentStats.abortedCount }}</div>
                              <div class="ams-label">异常中断</div>
                            </div>
                          </div>

                          <div class="aoc-charts-row">
                            <div class="aoc-chart-card">
                              <h4 class="aoc-chart-title">Token 消耗分布</h4>
                              <div class="aoc-pie-wrap">
                                <svg viewBox="0 0 120 120" class="aoc-pie-svg">
                                  <circle v-for="(seg, idx) in expandedTokenPieSegments" :key="idx"
                                    cx="60" cy="60" r="48" fill="none"
                                    :stroke="seg.color" stroke-width="22"
                                    :stroke-dasharray="seg.dash" :stroke-dashoffset="seg.offset"
                                    :transform="'rotate(-90 60 60)'" />
                                  <text x="60" y="56" text-anchor="middle" fill="#e2e8f0" font-size="12" font-weight="700">{{ fmtTokensShort(expandedAgentStats.totalTokens) }}</text>
                                  <text x="60" y="70" text-anchor="middle" fill="#64748b" font-size="8">总 Token</text>
                                </svg>
                                <div class="aoc-pie-legend">
                                  <div v-for="(seg, idx) in expandedTokenPieSegments" :key="idx" class="aoc-legend-row">
                                    <span class="aoc-legend-dot" :style="{ background: seg.color }"></span>
                                    <span class="aoc-legend-name">{{ seg.name }}</span>
                                    <span class="aoc-legend-val">{{ seg.pctLabel }}</span>
                                  </div>
                                </div>
                              </div>
                            </div>

                            <div class="aoc-chart-card">
                              <h4 class="aoc-chart-title">上下文使用率</h4>
                              <div class="aoc-bars-wrap">
                                <div v-for="ag in expandedAgents" :key="'bar-'+ag.id" class="aoc-bar-row">
                                  <div class="aoc-bar-label" :title="ag.id">{{ agentShortId(ag.id) }}</div>
                                  <div class="aoc-bar-track">
                                    <div class="aoc-bar-fill" :style="{ width: ag.contextPct + '%', background: contextBarColor(ag.contextPct) }"></div>
                                  </div>
                                  <div class="aoc-bar-pct" :style="{ color: contextBarColor(ag.contextPct) }">{{ ag.contextPct }}%</div>
                                </div>
                                <div v-if="expandedAgents.length === 0" class="dtab-empty" style="padding:16px 0">暂无数据</div>
                              </div>
                            </div>
                          </div>

                          <div class="aoc-ops-strip">
                            <div class="aoc-ops-item" v-for="gw in expandedGatewayOpsInfo" :key="gw.id">
                              <div class="aoc-ops-head">
                                <span class="aoc-ops-gw">{{ gw.id }}</span>
                                <span v-if="gw.version" class="aoc-ops-ver">v{{ gw.version }}</span>
                              </div>
                              <div class="aoc-ops-tags">
                                <span v-if="gw.mode" class="aoc-ops-tag" :class="'aot-' + gw.mode">{{ gw.mode }}</span>
                                <span v-if="gw.elevated" class="aoc-ops-tag aot-warn">elevated</span>
                                <span v-if="gw.compactions > 0" class="aoc-ops-tag aot-info">{{ gw.compactions }} 次压缩</span>
                                <span v-if="gw.queueDepth > 0" class="aoc-ops-tag aot-warn">队列 {{ gw.queueDepth }}</span>
                              </div>
                            </div>
                          </div>
                        </div>

                        <!-- ===== 2. Agent 详情卡片 ===== -->
                        <div v-if="aocView === 'cards'" class="aoc-cards-view">
                          <div class="aoc-agent-grid">
                            <div v-for="ag in expandedAgents" :key="ag.id + ag.gateway" class="aoc-agent-card" :class="{ 'aoc-card-active': ag.isActive, 'aoc-card-error': ag.hasError }">
                              <div class="aoc-card-top">
                                <div class="aoc-card-avatar" :style="{ background: agentColor(ag.id) }">
                                  {{ agentInitial(ag.id) }}
                                  <span class="aoc-status-dot" :class="ag.isActive ? 'asd-active' : (ag.hasError ? 'asd-error' : 'asd-idle')"></span>
                                </div>
                                <div class="aoc-card-ids">
                                  <div class="aoc-card-name" :title="ag.id">{{ agentShortId(ag.id) }}</div>
                                  <div class="aoc-card-gw">{{ ag.gateway }}</div>
                                </div>
                                <div class="aoc-card-status-badge" :class="ag.isActive ? 'acsb-active' : (ag.hasError ? 'acsb-error' : 'acsb-idle')">
                                  {{ ag.isActive ? '活跃' : (ag.hasError ? '异常' : '空闲') }}
                                </div>
                              </div>
                              <div class="aoc-card-model" v-if="ag.model">
                                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/></svg>
                                <span>{{ ag.model }}</span>
                                <span v-if="ag.provider" class="aoc-provider-tag">{{ ag.provider }}</span>
                              </div>
                              <div class="aoc-card-token">
                                <div class="aoc-token-head">
                                  <span class="aoc-token-label">Token</span>
                                  <span class="aoc-token-nums">{{ fmtTokensShort(ag.totalTokens) }} / {{ fmtTokensShort(ag.contextTokens) }}</span>
                                </div>
                                <div class="aoc-token-bar">
                                  <div class="aoc-token-fill" :style="{ width: Math.min(ag.contextPct, 100) + '%', background: contextBarColor(ag.contextPct) }"></div>
                                </div>
                                <div class="aoc-token-pct" :style="{ color: contextBarColor(ag.contextPct) }">{{ ag.contextPct }}%</div>
                              </div>
                              <div class="aoc-card-sessions">
                                <span class="aoc-sess-tag" v-if="ag.sessionBreakdown.main > 0">main ×{{ ag.sessionBreakdown.main }}</span>
                                <span class="aoc-sess-tag aoc-sess-iso" v-if="ag.sessionBreakdown.isolated > 0">isolated ×{{ ag.sessionBreakdown.isolated }}</span>
                                <span class="aoc-sess-tag aoc-sess-sub" v-if="ag.sessionBreakdown.sub > 0">sub ×{{ ag.sessionBreakdown.sub }}</span>
                                <span class="aoc-sess-tag aoc-sess-other" v-if="ag.sessionBreakdown.other > 0">other ×{{ ag.sessionBreakdown.other }}</span>
                              </div>
                              <div class="aoc-card-footer">
                                <div class="aoc-footer-item" :title="'通信渠道: ' + ag.channels.join(', ')">
                                  <span class="aoc-channel-icons">
                                    <span v-for="ch in ag.channels" :key="ch" class="aoc-ch-icon" :title="ch" v-html="channelIcon(ch)"></span>
                                  </span>
                                </div>
                                <div class="aoc-footer-item" v-if="ag.users.length > 0" :title="'用户: ' + ag.users.join(', ')">
                                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
                                  <span>{{ ag.users.length }}</span>
                                </div>
                                <div class="aoc-footer-item aoc-footer-time" :title="'最后活跃: ' + new Date(ag.lastActive).toLocaleString()">{{ fmtTime(ag.lastActive) }}</div>
                              </div>
                              <div class="aoc-card-ops">
                                <button class="btn btn-xs btn-ghost" title="编辑 Agent" @click="openAgentModal(ag)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg></button>
                                <button class="btn btn-xs btn-ghost btn-danger-ghost" title="删除 Agent" @click="agentDelete(ag)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></button>
                              </div>
                              <div v-if="ag.hasError" class="aoc-card-warn">
                                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#ef4444" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
                                abortedLastRun
                              </div>
                            </div>
                          </div>
                        </div>

                        <!-- ===== 3. 协作视图 ===== -->
                        <div v-if="aocView === 'collab'" class="aoc-collab-view">
                          <div v-for="gw in expandedCollabTree" :key="gw.gateway" class="aoc-collab-gw">
                            <div class="aoc-collab-gw-head" :style="{ borderLeftColor: gatewayColor(gw.gateway) }">
                              <span class="aoc-collab-gw-name">{{ gw.gateway }}</span>
                              <span class="aoc-collab-gw-count">{{ gw.agents.length }} agent{{ gw.agents.length > 1 ? 's' : '' }}</span>
                            </div>
                            <div v-for="agent in gw.agents" :key="agent.id" class="aoc-collab-agent">
                              <div class="aoc-collab-agent-head">
                                <div class="aoc-collab-avatar" :style="{ background: agentColor(agent.id) }">{{ agentInitial(agent.id) }}</div>
                                <span class="aoc-collab-agent-name">{{ agentShortId(agent.id) }}</span>
                                <span class="aoc-collab-model" v-if="agent.model">{{ agent.model }}</span>
                              </div>
                              <div class="aoc-collab-sessions">
                                <div v-for="sess in agent.sessions" :key="sess.key" class="aoc-collab-sess" :class="'acs-' + sess.kind">
                                  <span class="aoc-collab-sess-icon" v-html="sessionKindIcon(sess.kind)"></span>
                                  <span class="aoc-collab-sess-type">{{ sess.kind }}</span>
                                  <span class="aoc-collab-sess-ch">{{ sess.channel || '—' }}</span>
                                  <span class="aoc-collab-sess-model">{{ sess.model || '—' }}</span>
                                  <span class="aoc-collab-sess-tokens">{{ fmtTokensShort(sess.totalTokens || 0) }}</span>
                                  <span class="aoc-collab-sess-time">{{ fmtTime(sess.updatedAt) }}</span>
                                </div>
                              </div>
                            </div>
                            <div v-if="gw.cronJobs && gw.cronJobs.length > 0" class="aoc-collab-cron">
                              <div class="aoc-collab-cron-title"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:-2px"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg> 定时任务</div>
                              <div v-for="cj in gw.cronJobs" :key="cj.id || cj.name" class="aoc-collab-cron-item">
                                <span class="aoc-cron-name">{{ cj.name || cj.id }}</span>
                                <span class="gw-badge" :class="cj.enabled !== false ? 'gw-connected' : 'gw-not_configured'">{{ cj.enabled !== false ? '启用' : '禁用' }}</span>
                                <code class="mono-sm">{{ cj.schedule || cj.cron || '—' }}</code>
                              </div>
                            </div>
                          </div>
                          <div v-if="expandedCollabTree.length === 0" class="dtab-empty">暂无协作数据</div>
                        </div>

                        <!-- ===== 4. 用户归因视图 ===== -->
                        <div v-if="aocView === 'users'" class="aoc-users-view">
                          <div class="aoc-users-grid">
                            <div v-for="user in expandedUserAttribution" :key="user.id" class="aoc-user-card">
                              <div class="aoc-user-top">
                                <div class="aoc-user-avatar" :style="{ background: agentColor(user.id) }">{{ (user.displayName || user.id).charAt(0).toUpperCase() }}</div>
                                <div class="aoc-user-info">
                                  <div class="aoc-user-name">{{ user.displayName || user.id }}</div>
                                  <div class="aoc-user-channel">{{ user.channels.join(', ') }}</div>
                                </div>
                              </div>
                              <div class="aoc-user-ring-row">
                                <svg viewBox="0 0 80 80" class="aoc-user-ring-svg">
                                  <circle cx="40" cy="40" r="32" fill="none" stroke="#1e293b" stroke-width="6" />
                                  <circle cx="40" cy="40" r="32" fill="none" :stroke="contextBarColor(user.tokenPct)" stroke-width="6"
                                    :stroke-dasharray="(user.tokenPct / 100 * 201.06) + ' ' + (201.06 - user.tokenPct / 100 * 201.06)"
                                    stroke-dashoffset="0" transform="rotate(-90 40 40)" stroke-linecap="round" />
                                  <text x="40" y="38" text-anchor="middle" fill="#e2e8f0" font-size="11" font-weight="700">{{ user.tokenPct }}%</text>
                                  <text x="40" y="50" text-anchor="middle" fill="#64748b" font-size="7">{{ fmtTokensShort(user.totalTokens) }}</text>
                                </svg>
                                <div class="aoc-user-ring-detail">
                                  <div class="aoc-urd-row"><span class="aoc-urd-label">会话</span><span class="aoc-urd-val">{{ user.sessionCount }}</span></div>
                                  <div class="aoc-urd-row"><span class="aoc-urd-label">Agent</span><span class="aoc-urd-val">{{ user.agentIds.length }}</span></div>
                                  <div class="aoc-urd-row"><span class="aoc-urd-label">最后活跃</span><span class="aoc-urd-val">{{ fmtTime(user.lastActive) }}</span></div>
                                </div>
                              </div>
                              <div class="aoc-user-agents">
                                <div v-for="aid in user.agentIds" :key="aid" class="aoc-user-agent-tag">
                                  <span class="aoc-ua-dot" :style="{ background: agentColor(aid) }"></span>
                                  {{ agentShortId(aid) }}
                                </div>
                              </div>
                            </div>
                          </div>
                          <div v-if="expandedUserAttribution.length === 0" class="dtab-empty">暂无用户数据</div>
                        </div>

                        <!-- ===== 5. Skills 视图 ===== -->
                        <div v-if="aocView === 'skills'" class="aoc-skills-view">
                          <div class="skills-stat-strip">
                            <div class="ams-item"><div class="ams-value ams-indigo">{{ skillData.count }}</div><div class="ams-label">总 Skills</div></div>
                            <div class="ams-item"><div class="ams-value ams-cyan">{{ skillData.summary.global }}</div><div class="ams-label">全局</div></div>
                            <div class="ams-item"><div class="ams-value ams-green">{{ skillData.summary.user }}</div><div class="ams-label">用户</div></div>
                            <div class="ams-item"><div class="ams-value" :class="skillData.summary.workspace > 0 ? 'ams-amber' : ''">{{ skillData.summary.workspace }}</div><div class="ams-label">Workspace</div></div>
                          </div>
                          <div class="skills-search">
                            <input v-model="skillSearch" placeholder="搜索 skill 名称或描述..." class="skills-search-input" />
                            <span class="skills-search-count" v-if="skillSearch">{{ filteredSkills.length }} / {{ skillData.skills.length }}</span>
                            <button class="btn btn-xs btn-primary" style="margin-left:8px" @click="openSkillInstall">+ 安装</button>
                          </div>
                          <div v-if="skillsLoading" class="skel-lines"><div class="skel-line" v-for="i in 6" :key="i"></div></div>
                          <template v-else>
                            <div v-for="group in groupedSkills" :key="group.category" class="skill-group">
                              <div class="skill-group-header" @click="group.expanded = !group.expanded">
                                <span class="skill-cat-tag" :class="'scat-' + group.catKey" v-html="group.label"></span>
                                <span class="skill-group-count">{{ group.skills.length }}</span>
                                <svg :class="{ 'expand-chevron': true, open: group.expanded }" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
                              </div>
                              <div v-if="group.expanded" class="skill-group-body">
                                <div v-for="sk in group.skills" :key="sk.name + sk.category" class="skill-item">
                                  <div class="skill-icon"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg></div>
                                  <div class="skill-info">
                                    <div class="skill-name">{{ sk.name }}</div>
                                    <div class="skill-desc" v-if="sk.description">{{ sk.description }}</div>
                                  </div>
                                  <div class="skill-badges">
                                    <span v-if="sk.has_skill_md" class="skill-badge sb-ok">SKILL.md</span>
                                    <span v-if="sk.workspace" class="skill-badge sb-ws" :title="sk.workspace">{{ sk.workspace.substring(0, 12) }}...</span>
                                  </div>
                                  <div class="skill-actions">
                                    <button class="btn btn-xs btn-ghost" title="更新" @click.stop="skillUpdate(sk)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg></button>
                                    <button class="btn btn-xs btn-ghost btn-danger-ghost" title="卸载" @click.stop="skillUninstall(sk)"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></button>
                                  </div>
                                </div>
                              </div>
                            </div>
                            <div v-if="filteredSkills.length === 0 && !skillsLoading" class="dtab-empty">{{ skillSearch ? '无匹配 skill' : '暂无 skill 数据' }}</div>
                          </template>
                        </div>

                        <!-- ===== 6. 文件编辑器 ===== -->
                        <div v-if="aocView === 'files'" class="aoc-files-view">
                          <div class="files-layout">
                            <div class="files-sidebar">
                              <div class="files-agent-select">
                                <select v-model="fileAgentId" @change="loadAgentFiles" class="refresh-select" style="width:100%">
                                  <option value="">选择 Agent</option>
                                  <option v-for="a in expandedAgents" :key="a.id" :value="a.id">{{ a.id }}</option>
                                </select>
                              </div>
                              <div class="files-list">
                                <div v-for="f in agentFiles" :key="f.name" class="file-item" :class="{ active: f.name === editingFileName }" @click="openFile(f)">
                                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg>
                                  <span>{{ f.name }}</span>
                                </div>
                                <div v-if="fileAgentId && agentFiles.length === 0" class="dtab-empty" style="padding:12px;font-size:12px">暂无文件</div>
                              </div>
                            </div>
                            <div class="files-editor">
                              <div v-if="!editingFileName" class="dtab-empty">选择文件开始编辑</div>
                              <template v-else>
                                <div class="editor-header">
                                  <code class="mono-sm">{{ editingFileName }}</code>
                                  <span v-if="fileUnsaved" class="badge-unsaved">未保存</span>
                                  <button class="btn btn-xs btn-primary" @click="saveFile" :disabled="fileSaving">{{ fileSaving ? '保存中…' : '保存' }}</button>
                                </div>
                                <textarea v-model="fileContent" class="file-textarea" spellcheck="false" @input="fileUnsaved=true"></textarea>
                              </template>
                            </div>
                          </div>
                        </div>

                        <!-- ===== 7. 心跳/设备/节点 ===== -->
                        <div v-if="aocView === 'heartbeat'" class="aoc-heartbeat-view">
                          <div class="hb-cards">
                            <div class="hb-card">
                              <h4>💓 心跳状态</h4>
                              <div v-if="heartbeatData">
                                <div class="hb-row"><span>状态：</span><span class="gw-badge" :class="heartbeatData.status==='ok'||heartbeatData.status==='skipped'?'gw-connected':'gw-error'">{{ heartbeatData.status }}</span></div>
                                <div class="hb-row" v-if="heartbeatData.ts"><span>时间：</span><span>{{ fmtTime(heartbeatData.ts) }}</span></div>
                                <div class="hb-row" v-if="heartbeatData.reason"><span>原因：</span><span class="text-dim">{{ heartbeatData.reason }}</span></div>
                              </div>
                              <div v-else class="text-dim" style="font-size:13px">加载中…</div>
                              <div class="hb-actions" style="margin-top:8px">
                                <button class="btn btn-sm btn-primary" @click="wakeAgent">⚡ 唤醒 Agent</button>
                              </div>
                            </div>
                            <div class="hb-card">
                              <h4>📱 已配对设备</h4>
                              <div v-if="devicesList.length === 0" class="text-dim" style="font-size:13px">暂无设备</div>
                              <div v-for="d in devicesList" :key="d.id||d.deviceId" class="hb-device">
                                <span>{{ d.name||d.label||d.id||d.deviceId }}</span>
                                <span class="gw-badge gw-connected" style="font-size:10px">已配对</span>
                              </div>
                            </div>
                            <div class="hb-card">
                              <h4>🖥️ 已配对节点</h4>
                              <div v-if="nodePairList.length === 0" class="text-dim" style="font-size:13px">暂无节点</div>
                              <div v-for="n in nodePairList" :key="n.id||n.nodeId" class="hb-device">
                                <span>{{ n.name||n.id||n.nodeId }}</span>
                                <span class="gw-badge gw-connected" style="font-size:10px">在线</span>
                              </div>
                            </div>
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
              <button class="eye-btn" @click="tokenModal.showPwd = !tokenModal.showPwd"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><template v-if="tokenModal.showPwd"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></template><template v-else><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></template></svg></button>
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

    <!-- Cron CRUD 弹窗 -->
    <Teleport to="body">
      <div v-if="cronModalOpen" class="modal-overlay" @click.self="cronModalOpen=false">
        <div class="modal-box" style="max-width:560px">
          <h3>{{ cronEditing ? '编辑定时任务' : '新建定时任务' }}</h3>
          <div class="form-group">
            <label>名称</label>
            <input v-model="cronForm.name" class="form-input" placeholder="任务名称" />
          </div>
          <div class="form-group">
            <label>调度类型</label>
            <select v-model="cronForm.scheduleKind" class="form-input">
              <option value="cron">Cron 表达式</option>
              <option value="every">固定间隔</option>
              <option value="at">一次性定时</option>
            </select>
          </div>
          <div class="form-group" v-if="cronForm.scheduleKind === 'cron'">
            <label>Cron 表达式</label>
            <input v-model="cronForm.cronExpr" class="form-input" placeholder="*/30 * * * *" />
          </div>
          <div class="form-group" v-if="cronForm.scheduleKind === 'every'">
            <label>间隔（分钟）</label>
            <input v-model.number="cronForm.everyMin" type="number" class="form-input" placeholder="30" min="1" />
          </div>
          <div class="form-group" v-if="cronForm.scheduleKind === 'at'">
            <label>执行时间 (ISO)</label>
            <input v-model="cronForm.atTime" class="form-input" placeholder="2026-03-28T12:00:00Z" />
          </div>
          <div class="form-group">
            <label>Session 目标</label>
            <select v-model="cronForm.sessionTarget" class="form-input">
              <option value="isolated">isolated (独立会话)</option>
              <option value="main">main (主会话)</option>
            </select>
          </div>
          <div class="form-group" v-if="cronForm.sessionTarget === 'main'">
            <label>系统事件文本</label>
            <textarea v-model="cronForm.eventText" class="form-input" rows="3" placeholder="注入到主会话的系统事件文本"></textarea>
          </div>
          <div class="form-group" v-if="cronForm.sessionTarget === 'isolated'">
            <label>Agent 消息</label>
            <textarea v-model="cronForm.agentMessage" class="form-input" rows="3" placeholder="发送给 Agent 的消息"></textarea>
          </div>
          <div class="form-group">
            <label><input type="checkbox" v-model="cronForm.enabled" /> 启用</label>
          </div>
          <div class="modal-actions">
            <button class="btn btn-sm" @click="cronModalOpen=false">取消</button>
            <button class="btn btn-sm btn-primary" @click="cronSave" :disabled="cronSaving">{{ cronSaving ? '保存中…' : '保存' }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Agent CRUD 弹窗 -->
    <Teleport to="body">
      <div v-if="agentModalOpen" class="modal-overlay" @click.self="agentModalOpen=false">
        <div class="modal-box" style="max-width:500px">
          <h3>{{ agentEditing ? '编辑 Agent' : '新建 Agent' }}</h3>
          <div class="form-group">
            <label>Agent ID</label>
            <input v-model="agentForm.id" class="form-input" placeholder="my-agent" :disabled="!!agentEditing" />
          </div>
          <div class="form-group">
            <label>显示名称</label>
            <input v-model="agentForm.name" class="form-input" placeholder="My Agent (可选)" />
          </div>
          <div class="form-group">
            <label>Model</label>
            <input v-model="agentForm.model" class="form-input" placeholder="claude-sonnet-4-20250514 (可选)" />
          </div>
          <div class="form-group">
            <label>System Prompt</label>
            <textarea v-model="agentForm.systemPrompt" class="form-input" rows="4" placeholder="系统提示词 (可选)"></textarea>
          </div>
          <div class="modal-actions">
            <button class="btn btn-sm" @click="agentModalOpen=false">取消</button>
            <button class="btn btn-sm btn-primary" @click="agentSave" :disabled="agentSaving">{{ agentSaving ? '保存中…' : '保存' }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Skill 安装弹窗 -->
    <Teleport to="body">
      <div v-if="skillInstallOpen" class="modal-overlay" @click.self="skillInstallOpen=false">
        <div class="modal-box" style="max-width:480px">
          <h3>安装 Skill</h3>
          <div class="form-group">
            <label>Skill 名称 (slug)</label>
            <input v-model="skillInstallSlug" class="form-input" placeholder="skill-name 或 @author/skill-name" />
          </div>
          <div class="modal-actions">
            <button class="btn btn-sm" @click="skillInstallOpen=false">取消</button>
            <button class="btn btn-sm btn-primary" @click="skillInstall" :disabled="skillInstalling">{{ skillInstalling ? '安装中…' : '安装' }}</button>
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
import { api, apiPost, apiPut, apiPatch, apiDelete } from '../api.js'

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
// Session history state
const expandedSessionKey = ref(null)
const sessionMessages = ref([])
const sessionHistoryLoading = ref(false)
const msgExpanded = reactive({})
const lastRefreshTime = ref('')
const lastRefreshDisplay = ref('')
const refreshInterval = ref(localStorage.getItem('gm_refresh') || '30000')
let refreshTimer = null
let displayTimer = null

const toast = reactive({ show: false, msg: '', type: 'info' })
let toastTimer = null

const tokenModal = reactive({ show: false, data: null, token: '', showPwd: false, testing: false, saving: false, testResult: null })

const SVG_ATTRS = 'width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"'
const svgIcon = (inner) => `<svg ${SVG_ATTRS}>${inner}</svg>`

const tabs = [
  { key: 'sessions', icon: svgIcon('<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>'), label: '会话' },
  { key: 'cron', icon: svgIcon('<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>'), label: '定时任务' },
  { key: 'diag', icon: svgIcon('<circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>'), label: '诊断' },
  { key: 'agent', icon: svgIcon('<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>'), label: 'Agent' },
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

// === AOC (Agent Operations Center) State ===
const aocView = ref('dashboard')
const allCronJobs = ref([])

// Skill 相关状态
const skillData = reactive({ count: 0, summary: { global: 0, user: 0, workspace: 0 }, skills: [] })
const skillsLoading = ref(false)
const skillSearch = ref('')
const skillGroupExpanded = reactive({})

// v29.0: session 操作
const chatInput = ref('')
const chatSending = ref(false)

// v29.0: cron 操作
const expandedCronId = ref(null)
const cronRuns = ref([])
const cronRunsLoading = ref(false)
const cronModalOpen = ref(false)
const cronEditing = ref(null) // null=新建, object=编辑
const cronSaving = ref(false)
const cronForm = reactive({
  name: '', scheduleKind: 'cron', cronExpr: '', everyMin: 30, atTime: '',
  sessionTarget: 'isolated', eventText: '', agentMessage: '', enabled: true
})

// v29.0: agent 文件编辑
const fileAgentId = ref('')
const agentFiles = ref([])
const editingFileName = ref('')
const fileContent = ref('')
const fileUnsaved = ref(false)
const fileSaving = ref(false)

// v29.0: Agent CRUD
const agentModalOpen = ref(false)
const agentEditing = ref(null)
const agentSaving = ref(false)
const agentForm = reactive({ id: '', name: '', model: '', systemPrompt: '' })

// v29.0: Skill 安装/卸载
const skillInstallOpen = ref(false)
const skillInstallSlug = ref('')
const skillInstalling = ref(false)

// v29.0: Gateway 远程配置
const gwConfigLoaded = ref(false)
const gwConfigRaw = ref('')
const gwConfigDirty = ref(false)
const gwConfigSaving = ref(false)

// v29.0: 心跳/设备/节点
const heartbeatData = ref(null)
const devicesList = ref([])
const nodePairList = ref([])

const filteredSkills = computed(() => {
  if (!skillSearch.value) return skillData.skills
  const q = skillSearch.value.toLowerCase()
  return skillData.skills.filter(s => s.name.toLowerCase().includes(q) || (s.description || '').toLowerCase().includes(q))
})

const groupedSkills = computed(() => {
  const groups = {}
  for (const sk of filteredSkills.value) {
    const catKey = sk.category.startsWith('workspace:') ? 'workspace' : sk.category
    if (!groups[catKey]) {
      const label = catKey === 'global'
        ? svgIcon('<circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>') + ' 全局 Skills'
        : catKey === 'user'
        ? svgIcon('<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>') + ' 用户 Skills'
        : svgIcon('<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>') + ' Workspace Skills'
      groups[catKey] = { category: sk.category, catKey, label, skills: [], expanded: skillGroupExpanded[catKey] !== false }
    }
    groups[catKey].skills.push(sk)
  }
  return Object.values(groups).sort((a, b) => {
    const order = { global: 0, user: 1, workspace: 2 }
    return (order[a.catKey] ?? 9) - (order[b.catKey] ?? 9)
  })
})

async function loadSkills() {
  if (!expandedId.value) return
  skillsLoading.value = true
  try {
    const upId = expandedId.value
    const d = await api(`/api/v1/upstreams/${encodeURIComponent(upId)}/gateway/skills`)
    if (!d.error) {
      skillData.count = d.count || 0
      skillData.summary = d.summary || { global: 0, user: 0, workspace: 0 }
      skillData.skills = d.skills || []
    }
  } catch (e) { console.error('loadSkills error:', e) }
  finally { skillsLoading.value = false }
}

function switchToSkills() {
  aocView.value = 'skills'
  loadSkills()
}

function switchToFiles() {
  aocView.value = 'files'
  if (expandedAgents.value.length > 0 && !fileAgentId.value) {
    fileAgentId.value = expandedAgents.value[0].id
    loadAgentFiles()
  }
}

function switchToHeartbeat() {
  aocView.value = 'heartbeat'
  loadHeartbeatData()
}

// v29.0: Session 操作
async function sessionCompact(s) {
  if (!confirm(`压缩会话 ${truncKey(s.key||s.sessionId)}？`)) return
  try {
    await apiPost(`/api/v1/upstreams/${expandedId.value}/gateway/session/compact`, { key: s.key||s.sessionId })
    toastMsg.value = '会话已压缩'; toastType.value = 'success'; showToast.value = true
    loadTabData(expandedId.value, 'sessions')
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

async function sessionReset(s) {
  if (!confirm(`重置会话 ${truncKey(s.key||s.sessionId)}？所有历史将被清空。`)) return
  try {
    await apiPost(`/api/v1/upstreams/${expandedId.value}/gateway/session/reset`, { key: s.key||s.sessionId })
    toastMsg.value = '会话已重置'; toastType.value = 'success'; showToast.value = true
    loadTabData(expandedId.value, 'sessions')
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

async function sessionDelete(s) {
  if (!confirm(`删除会话 ${truncKey(s.key||s.sessionId)}？此操作不可恢复！`)) return
  try {
    await apiDelete(`/api/v1/upstreams/${expandedId.value}/gateway/session?key=${encodeURIComponent(s.key||s.sessionId)}`)
    toastMsg.value = '会话已删除'; toastType.value = 'success'; showToast.value = true
    loadTabData(expandedId.value, 'sessions')
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

async function chatSend(s) {
  if (!chatInput.value.trim()) return
  chatSending.value = true
  try {
    await apiPost(`/api/v1/upstreams/${expandedId.value}/gateway/chat/send`, { sessionKey: s.key||s.sessionId, message: chatInput.value.trim() })
    chatInput.value = ''
    toastMsg.value = '消息已发送'; toastType.value = 'success'; showToast.value = true
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
  chatSending.value = false
}

async function chatAbort(s) {
  try {
    await apiPost(`/api/v1/upstreams/${expandedId.value}/gateway/chat/abort`, { sessionKey: s.key||s.sessionId })
    toastMsg.value = '已发送中止请求'; toastType.value = 'success'; showToast.value = true
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

// v29.0: Cron 操作
function fmtCronSchedule(c) {
  const sched = c.schedule || c.cron || {}
  if (typeof sched === 'string') return sched
  if (sched.kind === 'cron') return `cron: ${sched.expr || '?'}`
  if (sched.kind === 'every') return `every ${sched.everyMs ? (sched.everyMs/1000/60)+'m' : '?'}`
  if (sched.kind === 'at') return `at: ${sched.at || '?'}`
  return JSON.stringify(sched).slice(0, 40)
}

async function cronTrigger(c) {
  try {
    await apiPost(`/api/v1/upstreams/${expandedId.value}/gateway/cron/run`, { id: c.id||c.jobId, mode: 'force' })
    toastMsg.value = '已触发运行'; toastType.value = 'success'; showToast.value = true
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

async function cronToggle(c) {
  try {
    await apiPut(`/api/v1/upstreams/${expandedId.value}/gateway/cron/update`, { id: c.id||c.jobId, enabled: c.enabled === false })
    toastMsg.value = c.enabled === false ? '已启用' : '已禁用'; toastType.value = 'success'; showToast.value = true
    loadTabData(expandedId.value, 'cron')
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

async function cronRemove(c) {
  if (!confirm(`删除定时任务 ${c.name||c.id}？`)) return
  try {
    await apiDelete(`/api/v1/upstreams/${expandedId.value}/gateway/cron/remove?id=${encodeURIComponent(c.id||c.jobId)}`)
    toastMsg.value = '已删除'; toastType.value = 'success'; showToast.value = true
    loadTabData(expandedId.value, 'cron')
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

function openCronModal(c) {
  if (c) {
    cronEditing.value = c
    cronForm.name = c.name || ''
    const sched = c.schedule || {}
    if (typeof sched === 'object') {
      cronForm.scheduleKind = sched.kind || 'cron'
      cronForm.cronExpr = sched.expr || ''
      cronForm.everyMin = sched.everyMs ? sched.everyMs / 60000 : 30
      cronForm.atTime = sched.at || ''
    }
    cronForm.sessionTarget = c.sessionTarget || 'isolated'
    const pl = c.payload || {}
    cronForm.eventText = pl.text || ''
    cronForm.agentMessage = pl.message || ''
    cronForm.enabled = c.enabled !== false
  } else {
    cronEditing.value = null
    cronForm.name = ''; cronForm.scheduleKind = 'cron'; cronForm.cronExpr = ''
    cronForm.everyMin = 30; cronForm.atTime = ''
    cronForm.sessionTarget = 'isolated'; cronForm.eventText = ''; cronForm.agentMessage = ''
    cronForm.enabled = true
  }
  cronModalOpen.value = true
}

async function cronSave() {
  cronSaving.value = true
  try {
    const schedule = cronForm.scheduleKind === 'cron' ? { kind: 'cron', expr: cronForm.cronExpr }
      : cronForm.scheduleKind === 'every' ? { kind: 'every', everyMs: cronForm.everyMin * 60000 }
      : { kind: 'at', at: cronForm.atTime }
    const payload = cronForm.sessionTarget === 'main'
      ? { kind: 'systemEvent', text: cronForm.eventText }
      : { kind: 'agentTurn', message: cronForm.agentMessage }
    const body = { name: cronForm.name, schedule, payload, sessionTarget: cronForm.sessionTarget, enabled: cronForm.enabled }

    if (cronEditing.value) {
      body.id = cronEditing.value.id || cronEditing.value.jobId
      await apiPut(`/api/v1/upstreams/${expandedId.value}/gateway/cron/update`, body)
    } else {
      await apiPost(`/api/v1/upstreams/${expandedId.value}/gateway/cron/add`, body)
    }
    cronModalOpen.value = false
    toastMsg.value = cronEditing.value ? '任务已更新' : '任务已创建'; toastType.value = 'success'; showToast.value = true
    loadTabData(expandedId.value, 'cron')
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
  cronSaving.value = false
}

async function toggleCronRuns(c) {
  const cid = c.id || c.jobId
  if (expandedCronId.value === cid) { expandedCronId.value = null; return }
  expandedCronId.value = cid
  cronRunsLoading.value = true
  try {
    const res = await api(`/api/v1/upstreams/${expandedId.value}/gateway/cron/runs?id=${encodeURIComponent(cid)}&limit=10`)
    cronRuns.value = res.runs || res.history || []
  } catch { cronRuns.value = [] }
  cronRunsLoading.value = false
}

// v29.0: Agent 文件编辑
async function loadAgentFiles() {
  if (!fileAgentId.value || !expandedId.value) return
  try {
    const res = await api(`/api/v1/upstreams/${expandedId.value}/gateway/agents/files?agentId=${encodeURIComponent(fileAgentId.value)}`)
    agentFiles.value = res.files || []
  } catch { agentFiles.value = [] }
  editingFileName.value = ''
  fileContent.value = ''
  fileUnsaved.value = false
}

async function openFile(f) {
  if (fileUnsaved.value && !confirm('有未保存的修改，确认切换？')) return
  editingFileName.value = f.name
  fileContent.value = ''
  fileUnsaved.value = false
  try {
    const res = await api(`/api/v1/upstreams/${expandedId.value}/gateway/agents/file?agentId=${encodeURIComponent(fileAgentId.value)}&name=${encodeURIComponent(f.name)}`)
    fileContent.value = res.content || ''
  } catch (e) { fileContent.value = '// 加载失败: ' + e.message }
}

async function saveFile() {
  fileSaving.value = true
  try {
    await apiPut(`/api/v1/upstreams/${expandedId.value}/gateway/agents/file`, { agentId: fileAgentId.value, name: editingFileName.value, content: fileContent.value })
    fileUnsaved.value = false
    toastMsg.value = '文件已保存'; toastType.value = 'success'; showToast.value = true
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
  fileSaving.value = false
}

// v29.0: 心跳/设备/节点
async function loadHeartbeatData() {
  if (!expandedId.value) return
  try {
    const [hb, dev, np] = await Promise.all([
      api(`/api/v1/upstreams/${expandedId.value}/gateway/heartbeat`),
      api(`/api/v1/upstreams/${expandedId.value}/gateway/devices`),
      api(`/api/v1/upstreams/${expandedId.value}/gateway/node-pairs`),
    ])
    heartbeatData.value = hb
    devicesList.value = hb.paired || dev.paired || []
    nodePairList.value = np.paired || []
  } catch { /* ignore */ }
}

async function wakeAgent() {
  try {
    await apiPost(`/api/v1/upstreams/${expandedId.value}/gateway/wake`, { mode: 'now' })
    toastMsg.value = '已发送唤醒'; toastType.value = 'success'; showToast.value = true
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

// v29.0: Agent CRUD
function openAgentModal(ag) {
  if (ag) {
    agentEditing.value = ag
    agentForm.id = ag.id
    agentForm.name = ag.name || ''
    agentForm.model = ag.model || ''
    agentForm.systemPrompt = ag.systemPrompt || ''
  } else {
    agentEditing.value = null
    agentForm.id = ''; agentForm.name = ''; agentForm.model = ''; agentForm.systemPrompt = ''
  }
  agentModalOpen.value = true
}

async function agentSave() {
  agentSaving.value = true
  try {
    const body = { id: agentForm.id, name: agentForm.name || undefined, model: agentForm.model || undefined, systemPrompt: agentForm.systemPrompt || undefined }
    if (agentEditing.value) {
      await apiPut(`/api/v1/upstreams/${expandedId.value}/gateway/agents/update`, body)
    } else {
      await apiPost(`/api/v1/upstreams/${expandedId.value}/gateway/agents/create`, body)
    }
    agentModalOpen.value = false
    toastMsg.value = agentEditing.value ? 'Agent 已更新' : 'Agent 已创建'; toastType.value = 'success'; showToast.value = true
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
  agentSaving.value = false
}

async function agentDelete(ag) {
  if (!confirm(`删除 Agent "${ag.id}"？此操作不可恢复！`)) return
  try {
    await apiDelete(`/api/v1/upstreams/${expandedId.value}/gateway/agents/delete?id=${encodeURIComponent(ag.id)}`)
    toastMsg.value = 'Agent 已删除'; toastType.value = 'success'; showToast.value = true
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

// v29.0: Skill 安装/更新/卸载
function openSkillInstall() { skillInstallSlug.value = ''; skillInstallOpen.value = true }

async function skillInstall() {
  if (!skillInstallSlug.value.trim()) return
  skillInstalling.value = true
  try {
    await apiPost(`/api/v1/upstreams/${expandedId.value}/gateway/skills/install`, { slug: skillInstallSlug.value.trim() })
    skillInstallOpen.value = false
    toastMsg.value = 'Skill 安装成功'; toastType.value = 'success'; showToast.value = true
    loadSkills()
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
  skillInstalling.value = false
}

async function skillUpdate(sk) {
  try {
    await apiPost(`/api/v1/upstreams/${expandedId.value}/gateway/skills/update`, { slug: sk.name })
    toastMsg.value = `${sk.name} 已更新`; toastType.value = 'success'; showToast.value = true
    loadSkills()
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

async function skillUninstall(sk) {
  if (!confirm(`卸载 Skill "${sk.name}"？`)) return
  try {
    await apiDelete(`/api/v1/upstreams/${expandedId.value}/gateway/skills/uninstall?slug=${encodeURIComponent(sk.name)}`)
    toastMsg.value = `${sk.name} 已卸载`; toastType.value = 'success'; showToast.value = true
    loadSkills()
  } catch (e) { toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true }
}

// v29.0: Gateway 远程配置
async function loadGatewayConfig(up) {
  try {
    const res = await api(`/api/v1/upstreams/${up.id}/gateway/config`)
    gwConfigRaw.value = JSON.stringify(res, null, 2)
    gwConfigLoaded.value = true
    gwConfigDirty.value = false
  } catch (e) { toastMsg.value = '加载配置失败: ' + e.message; toastType.value = 'error'; showToast.value = true }
}

async function patchGatewayConfig(up) {
  if (!confirm('确认修改 Gateway 配置？这将触发 Gateway 重启。')) return
  gwConfigSaving.value = true
  try {
    let parsed = JSON.parse(gwConfigRaw.value)
    await apiPatch(`/api/v1/upstreams/${up.id}/gateway/config`, parsed)
    gwConfigDirty.value = false
    toastMsg.value = '配置已保存，Gateway 正在重启'; toastType.value = 'success'; showToast.value = true
  } catch (e) {
    toastMsg.value = e.message; toastType.value = 'error'; showToast.value = true
  }
  gwConfigSaving.value = false
}

// 加载所有 Gateway 的 cron jobs
async function loadAllCronJobs() {
  const jobs = []
  for (const up of allUpstreams.value) {
    if (up.gateway_status === 'connected' && up.token_configured) {
      try {
        const d = await api(`/api/v1/upstreams/${encodeURIComponent(up.id)}/gateway/cron`)
        if (!d.error && Array.isArray(d.jobs)) {
          for (const j of d.jobs) jobs.push({ ...j, _gateway: up.id })
        }
      } catch {}
    }
  }
  allCronJobs.value = jobs
}

// 增强版 Agent 列表 — 聚合每个 Agent 的所有 session 数据
const enrichedAgents = computed(() => {
  const map = new Map() // agentId+gateway → enriched
  for (const up of allUpstreams.value) {
    if (up.gateway_status !== 'connected') continue
    const sess = up._sessions || []
    const agents = up._agents || []
    // 先注册 agents_list 中的
    for (const ag of agents) {
      const aid = ag.id || ag.agentId
      if (!aid) continue
      const k = aid + '|' + up.id
      if (!map.has(k)) {
        map.set(k, {
          id: aid, gateway: up.id, model: null, provider: null,
          totalTokens: 0, contextTokens: 0, contextPct: 0,
          lastActive: 0, isActive: false, hasError: false,
          sessionBreakdown: { main: 0, isolated: 0, sub: 0, other: 0 },
          channels: [], users: [], sessions: [], configured: !!ag.configured
        })
      }
    }
    // 从 sessions 聚合
    for (const s of sess) {
      const aid = extractAgentFromKey(s.key) || s.agentId || s.agent_id
      if (!aid) continue
      const k = aid + '|' + up.id
      if (!map.has(k)) {
        map.set(k, {
          id: aid, gateway: up.id, model: null, provider: null,
          totalTokens: 0, contextTokens: 0, contextPct: 0,
          lastActive: 0, isActive: false, hasError: false,
          sessionBreakdown: { main: 0, isolated: 0, sub: 0, other: 0 },
          channels: [], users: [], sessions: [], configured: true
        })
      }
      const ag = map.get(k)
      // 添加原始 session 引用
      const kind = extractSessionKind(s.key) || s.kind || 'other'
      ag.sessions.push({ ...s, kind })
      // model/provider
      if (s.model && !ag.model) ag.model = s.model
      if (s.messages && s.messages.length > 0) {
        const lastMsg = s.messages[s.messages.length - 1]
        if (lastMsg && lastMsg.provider && !ag.provider) ag.provider = lastMsg.provider
        if (lastMsg && lastMsg.model && !ag.model) ag.model = lastMsg.model
      }
      // token
      const toks = s.totalTokens || s.total_tokens || 0
      ag.totalTokens += toks
      const ctx = s.contextTokens || s.context_tokens || 0
      if (ctx > ag.contextTokens) ag.contextTokens = ctx
      // active
      const updAt = s.updatedAt || s.updated_at || 0
      const ts = typeof updAt === 'number' && updAt < 1e12 ? updAt * 1000 : updAt
      if (ts > ag.lastActive) ag.lastActive = ts
      const ageMin = (Date.now() - ts) / 60000
      if (ageMin < 30 && toks > 0) ag.isActive = true
      // aborted
      if (s.abortedLastRun) ag.hasError = true
      // session breakdown
      if (kind === 'main' || kind === 'direct') ag.sessionBreakdown.main++
      else if (kind === 'isolated') ag.sessionBreakdown.isolated++
      else if (kind === 'sub' || kind === 'subagent') ag.sessionBreakdown.sub++
      else ag.sessionBreakdown.other++
      // channel
      const ch = s.channel || s.lastChannel
      if (ch && !ag.channels.includes(ch)) ag.channels.push(ch)
      // user
      const user = extractUser(s)
      if (user && !ag.users.includes(user)) ag.users.push(user)
    }
  }
  // 计算 contextPct
  for (const ag of map.values()) {
    ag.contextPct = ag.contextTokens > 0 ? Math.round(ag.totalTokens / ag.contextTokens * 100) : 0
  }
  return Array.from(map.values()).sort((a, b) => b.lastActive - a.lastActive)
})

// 统计概览
const agentStats = computed(() => {
  const agents = enrichedAgents.value
  return {
    total: agents.length,
    active: agents.filter(a => a.isActive).length,
    totalTokens: agents.reduce((s, a) => s + a.totalTokens, 0),
    totalSessions: agents.reduce((s, a) => s + a.sessions.length, 0),
    abortedCount: agents.filter(a => a.hasError).length
  }
})

// Token 饼图分段
const PIE_COLORS = ['#6366f1', '#22c55e', '#06b6d4', '#eab308', '#ef4444', '#8b5cf6', '#f97316', '#ec4899', '#14b8a6', '#64748b']
const tokenPieSegments = computed(() => {
  const agents = enrichedAgents.value.filter(a => a.totalTokens > 0)
  const total = agents.reduce((s, a) => s + a.totalTokens, 0)
  if (total === 0) return []
  const circumference = 2 * Math.PI * 48 // ~301.59
  let offset = 0
  return agents.slice(0, 10).map((ag, i) => {
    const pct = ag.totalTokens / total
    const dashLen = pct * circumference
    const seg = {
      name: agentShortId(ag.id),
      color: PIE_COLORS[i % PIE_COLORS.length],
      dash: dashLen + ' ' + (circumference - dashLen),
      offset: -offset,
      pctLabel: Math.round(pct * 100) + '%'
    }
    offset += dashLen
    return seg
  })
})

// Gateway 运维信息（解析 status_text）
const gatewayOpsInfo = computed(() => {
  const list = []
  for (const up of allUpstreams.value) {
    if (up.gateway_status !== 'connected') continue
    const info = { id: up.id, version: null, mode: null, elevated: false, compactions: 0, queueDepth: 0 }
    const txt = up.status_text || ''
    // 解析版本
    const verM = /v?(\d+\.\d+\.\d+[^\s]*)/.exec(txt)
    if (verM) info.version = verM[1]
    // 模式
    if (/thinking/i.test(txt)) info.mode = 'thinking'
    else if (/direct/i.test(txt)) info.mode = 'direct'
    else if (/streaming/i.test(txt)) info.mode = 'streaming'
    // elevated
    if (/elevated/i.test(txt)) info.elevated = true
    // compactions
    const compM = /[Cc]ompact(?:ion)?s?[:\s]+(\d+)/.exec(txt)
    if (compM) info.compactions = parseInt(compM[1])
    // queue
    const qM = /[Qq]ueue[:\s]+(\d+)/.exec(txt)
    if (qM) info.queueDepth = parseInt(qM[1])
    list.push(info)
  }
  return list
})

// 协作树视图
const collabTree = computed(() => {
  const gwMap = new Map()
  for (const ag of enrichedAgents.value) {
    if (!gwMap.has(ag.gateway)) {
      gwMap.set(ag.gateway, { gateway: ag.gateway, agents: [], cronJobs: [] })
    }
    gwMap.get(ag.gateway).agents.push(ag)
  }
  // 关联 cron jobs
  for (const cj of allCronJobs.value) {
    const gw = gwMap.get(cj._gateway)
    if (gw) gw.cronJobs.push(cj)
  }
  return Array.from(gwMap.values())
})

// 用户归因
const userAttribution = computed(() => {
  const userMap = new Map()
  for (const ag of enrichedAgents.value) {
    for (const s of ag.sessions) {
      const userId = extractUser(s)
      if (!userId) continue
      if (!userMap.has(userId)) {
        userMap.set(userId, {
          id: userId,
          displayName: s.displayName || userId,
          totalTokens: 0,
          contextTokens: 0,
          tokenPct: 0,
          sessionCount: 0,
          lastActive: 0,
          channels: [],
          agentIds: []
        })
      }
      const u = userMap.get(userId)
      u.totalTokens += (s.totalTokens || s.total_tokens || 0)
      const ctx = s.contextTokens || s.context_tokens || 0
      if (ctx > u.contextTokens) u.contextTokens = ctx
      u.sessionCount++
      const ts = s.updatedAt || s.updated_at || 0
      const normalTs = typeof ts === 'number' && ts < 1e12 ? ts * 1000 : ts
      if (normalTs > u.lastActive) u.lastActive = normalTs
      const ch = s.channel || s.lastChannel
      if (ch && !u.channels.includes(ch)) u.channels.push(ch)
      if (!u.agentIds.includes(ag.id)) u.agentIds.push(ag.id)
      if (s.displayName && s.displayName !== userId) u.displayName = s.displayName
    }
  }
  for (const u of userMap.values()) {
    u.tokenPct = u.contextTokens > 0 ? Math.round(u.totalTokens / u.contextTokens * 100) : 0
  }
  return Array.from(userMap.values()).sort((a, b) => b.totalTokens - a.totalTokens)
})

// === Per-upstream AOC computed properties ===
const expandedAgents = computed(() => {
  if (!expandedId.value) return []
  return enrichedAgents.value.filter(ag => ag.gateway === expandedId.value)
})

const expandedAgentStats = computed(() => {
  const agents = expandedAgents.value
  return {
    total: agents.length,
    active: agents.filter(a => a.isActive).length,
    totalTokens: agents.reduce((s, a) => s + a.totalTokens, 0),
    totalSessions: agents.reduce((s, a) => s + a.sessions.length, 0),
    abortedCount: agents.filter(a => a.hasError).length
  }
})

const expandedTokenPieSegments = computed(() => {
  const agents = expandedAgents.value.filter(a => a.totalTokens > 0)
  const total = agents.reduce((s, a) => s + a.totalTokens, 0)
  if (total === 0) return []
  const circumference = 2 * Math.PI * 48
  let offset = 0
  return agents.slice(0, 10).map((ag, i) => {
    const pct = ag.totalTokens / total
    const dashLen = pct * circumference
    const seg = {
      name: agentShortId(ag.id),
      color: PIE_COLORS[i % PIE_COLORS.length],
      dash: dashLen + ' ' + (circumference - dashLen),
      offset: -offset,
      pctLabel: Math.round(pct * 100) + '%'
    }
    offset += dashLen
    return seg
  })
})

const expandedGatewayOpsInfo = computed(() => {
  if (!expandedId.value) return []
  return gatewayOpsInfo.value.filter(gw => gw.id === expandedId.value)
})

const expandedCollabTree = computed(() => {
  if (!expandedId.value) return []
  return collabTree.value.filter(gw => gw.gateway === expandedId.value)
})

const expandedUserAttribution = computed(() => {
  if (!expandedId.value) return []
  const expandedAgentIds = new Set(expandedAgents.value.map(a => a.id))
  // Rebuild user attribution from only the expanded upstream's agents
  const userMap = new Map()
  for (const ag of expandedAgents.value) {
    for (const s of ag.sessions) {
      const userId = extractUser(s)
      if (!userId) continue
      if (!userMap.has(userId)) {
        userMap.set(userId, {
          id: userId,
          displayName: s.displayName || userId,
          totalTokens: 0,
          contextTokens: 0,
          tokenPct: 0,
          sessionCount: 0,
          lastActive: 0,
          channels: [],
          agentIds: []
        })
      }
      const u = userMap.get(userId)
      u.totalTokens += (s.totalTokens || s.total_tokens || 0)
      const ctx = s.contextTokens || s.context_tokens || 0
      if (ctx > u.contextTokens) u.contextTokens = ctx
      u.sessionCount++
      const ts = s.updatedAt || s.updated_at || 0
      const normalTs = typeof ts === 'number' && ts < 1e12 ? ts * 1000 : ts
      if (normalTs > u.lastActive) u.lastActive = normalTs
      const ch = s.channel || s.lastChannel
      if (ch && !u.channels.includes(ch)) u.channels.push(ch)
      if (!u.agentIds.includes(ag.id)) u.agentIds.push(ag.id)
      if (s.displayName && s.displayName !== userId) u.displayName = s.displayName
    }
  }
  for (const u of userMap.values()) {
    u.tokenPct = u.contextTokens > 0 ? Math.round(u.totalTokens / u.contextTokens * 100) : 0
  }
  return Array.from(userMap.values()).sort((a, b) => b.totalTokens - a.totalTokens)
})

// AOC 辅助函数
function extractSessionKind(key) {
  if (!key) return null
  // agent:AGENT_ID:SESSION_TYPE[:...]
  const parts = key.split(':')
  if (parts.length >= 3 && parts[0] === 'agent') {
    const t = parts[2]
    if (t === 'main' || t === 'direct') return 'main'
    if (t === 'isolated') return 'isolated'
    if (t === 'sub' || t === 'subagent') return 'sub'
    return t || 'other'
  }
  return null
}
function extractUser(s) {
  // 从 displayName 或 lastTo 提取
  if (s.displayName) return s.displayName
  if (s.lastTo) {
    // "lanxin:3588352-xxx" → 用户部分
    const parts = s.lastTo.split(':')
    return parts.length > 1 ? parts.slice(1).join(':') : s.lastTo
  }
  return null
}
function agentShortId(id) {
  if (!id) return '?'
  // lanxin-3588352-xxxxxxxxxxxxx → lanxin-3588..xxx
  if (id.length > 24) return id.slice(0, 14) + '…' + id.slice(-6)
  return id
}
function fmtTokensShort(t) {
  if (!t || t === 0) return '0'
  if (t >= 1e6) return (t / 1e6).toFixed(1) + 'M'
  if (t >= 1e3) return (t / 1e3).toFixed(1) + 'K'
  return String(t)
}
function contextBarColor(pct) {
  if (pct >= 90) return '#ef4444'
  if (pct >= 70) return '#eab308'
  if (pct >= 40) return '#06b6d4'
  return '#22c55e'
}
function channelIcon(ch) {
  const s = (inner) => `<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${inner}</svg>`
  const svgs = {
    lanxin: s('<rect x="5" y="2" width="14" height="20" rx="2" ry="2"/><line x1="12" y1="18" x2="12.01" y2="18"/>'),
    telegram: s('<line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>'),
    discord: s('<path d="M18 8a7.5 7.5 0 0 0-12 0"/><circle cx="9" cy="15" r="1"/><circle cx="15" cy="15" r="1"/><path d="M7 21c2-1 3-2 5-2s3 1 5 2"/>'),
    whatsapp: s('<path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z"/>'),
    slack: s('<line x1="4" y1="9" x2="20" y2="9"/><line x1="4" y1="15" x2="20" y2="15"/><line x1="10" y1="3" x2="8" y2="21"/><line x1="16" y1="3" x2="14" y2="21"/>'),
    web: s('<circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>'),
    api: s('<polyline points="13 2 3 14 12 14 11 22"/>')
  }
  return svgs[ch] || s('<path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"/>')
}
function sessionKindIcon(kind) {
  const s = (inner) => `<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${inner}</svg>`
  const svgs = {
    main: s('<path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>'),
    direct: s('<path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>'),
    isolated: s('<rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>'),
    sub: s('<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>'),
    subagent: s('<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>'),
    other: s('<path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/>')
  }
  return svgs[kind] || svgs.other
}
function gatewayColor(gw) {
  let h = 0
  for (let i = 0; i < (gw || '').length; i++) h = (gw || '').charCodeAt(i) + ((h << 5) - h)
  return `hsl(${Math.abs(h) % 360},60%,55%)`
}

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
    // 异步加载所有 cron jobs（不阻塞主加载）
    loadAllCronJobs()
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
  expandedId.value = up.id; activeTab.value = 'sessions'; diagResult.value = null; expandedSessionKey.value = null; sessionMessages.value = []
  if (up.token_configured && up.gateway_status !== 'not_configured') await loadTabData(up.id, 'sessions')
}
async function switchTab(tab, id) { activeTab.value = tab; expandedSessionKey.value = null; sessionMessages.value = []; if (tab === 'agent') { aocView.value = 'dashboard'; await loadTabData(id, 'cron') } else if (tab !== 'diag') await loadTabData(id, tab) }
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
  } catch (e) {
    sessions.value = []; cronJobs.value = []
    // 502 upstream_auth_failed: 提示用户重新配置 Token，不要静默
    const msg = e.message || ''
    if (msg.includes('502')) {
      showToast(`${id}: Gateway Token 无效或连接失败，请检查配置`, 'error')
    }
  } finally { detailLoading.value = false }
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

// Session history
async function toggleSessionHistory(s) {
  const key = s.key || s.sessionId
  if (expandedSessionKey.value === key) {
    expandedSessionKey.value = null
    sessionMessages.value = []
    return
  }
  expandedSessionKey.value = key
  sessionMessages.value = []
  Object.keys(msgExpanded).forEach(k => delete msgExpanded[k])
  if (!expandedId.value) return
  sessionHistoryLoading.value = true
  try {
    const d = await api(`/api/v1/upstreams/${encodeURIComponent(expandedId.value)}/gateway/session-history?sessionKey=${encodeURIComponent(key)}&limit=30`)
    sessionMessages.value = d.messages || []
  } catch(e) { console.error('loadSessionHistory error:', e) }
  finally { sessionHistoryLoading.value = false }
}

function extractContent(content) {
  const full = extractContentFull(content)
  if (full.length > 500) return full.substring(0, 500) + '...'
  return full
}

function extractContentFull(content) {
  if (typeof content === 'string') return content
  if (Array.isArray(content)) {
    return content.map(c => {
      if (c.type === 'text') return c.text || ''
      if (c.type === 'thinking') return '[思考] ' + (c.thinking || '').substring(0, 200) + '...'
      if (c.type === 'tool_use') return '[工具调用] ' + (c.name || '') + ': ' + JSON.stringify(c.input || {}).substring(0, 200)
      if (c.type === 'tool_result') return '[工具结果] ' + (typeof c.content === 'string' ? c.content.substring(0, 200) : JSON.stringify(c.content || '').substring(0, 200))
      return JSON.stringify(c)
    }).join('\n')
  }
  if (content && typeof content === 'object') return JSON.stringify(content)
  return String(content || '')
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
.diag-card-full { grid-column:1 / -1; }
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
.form-group { margin-bottom:12px; }
.form-group label { display:block; font-size:12px; font-weight:600; color:var(--text-secondary,#94a3b8); margin-bottom:4px; }
.form-group select.form-input { padding-right:12px; }
.form-group textarea.form-input { font-family:'JetBrains Mono','Fira Code',monospace; resize:vertical; min-height:60px; line-height:1.5; }
.modal-actions { display:flex; justify-content:flex-end; gap:8px; margin-top:16px; padding-top:12px; border-top:1px solid var(--border-subtle,#334155); }

/* Skill 操作按钮 */
.skill-item { position:relative; }
.skill-actions { display:flex; gap:2px; margin-left:auto; flex-shrink:0; }
.skill-actions .btn { opacity:0.3; transition:opacity .15s; }
.skill-item:hover .skill-actions .btn { opacity:1; }

/* Agent 卡片操作 */
.aoc-card-ops { display:flex; gap:4px; justify-content:flex-end; padding-top:8px; margin-top:8px; border-top:1px solid rgba(51,65,85,.2); }
.aoc-card-ops .btn { opacity:0.4; transition:opacity .15s; }
.aoc-agent-card:hover .aoc-card-ops .btn { opacity:1; }
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

/* ==============================
   Agent Operations Center (AOC)
   ============================== */

/* AOC Section */
.aoc-section { }
.aoc-inline { background:transparent; border:none; border-radius:0; padding:12px 16px; margin-bottom:0; }
.aoc-view-toggle { display:flex; gap:2px; background:rgba(15,23,42,.6); border-radius:8px; padding:2px; }
.aoc-vbtn { padding:4px 10px; border:none; background:transparent; border-radius:6px; font-size:13px; cursor:pointer; transition:all .15s; color:var(--text-tertiary,#64748b); }
.aoc-vbtn:hover { background:rgba(99,102,241,.1); color:var(--text-secondary,#94a3b8); }
.aoc-vbtn.active { background:rgba(99,102,241,.2); color:#a5b4fc; box-shadow:0 1px 3px rgba(0,0,0,.2); }

/* === Dashboard View === */
.aoc-dashboard { animation:fadeIn .3s; }

/* 统计条 */
.aoc-stat-strip { display:grid; grid-template-columns:repeat(auto-fill,minmax(130px,1fr)); gap:10px; margin-bottom:16px; }
.aoc-mini-stat { background:var(--bg-base,#0f172a); border:1px solid var(--border-subtle,#334155); border-radius:10px; padding:14px 16px; text-align:center; transition:border-color .2s; }
.aoc-mini-stat:hover { border-color:rgba(99,102,241,.3); }
.ams-value { font-size:24px; font-weight:700; color:var(--text-primary,#e2e8f0); line-height:1.2; }
.ams-label { font-size:11px; color:var(--text-tertiary,#64748b); margin-top:4px; }
.ams-green { color:#4ade80; }
.ams-cyan { color:#22d3ee; }
.ams-indigo { color:#a5b4fc; }
.ams-red { color:#f87171; }

/* 图表区 */
.aoc-charts-row { display:grid; grid-template-columns:1fr 1fr; gap:14px; margin-bottom:16px; }
@media(max-width:900px) { .aoc-charts-row { grid-template-columns:1fr; } }
.aoc-chart-card { background:var(--bg-base,#0f172a); border:1px solid var(--border-subtle,#334155); border-radius:10px; padding:16px; }
.aoc-chart-title { font-size:13px; font-weight:600; color:var(--text-primary,#e2e8f0); margin-bottom:12px; }

/* 饼图 */
.aoc-pie-wrap { display:flex; align-items:center; gap:20px; }
@media(max-width:600px) { .aoc-pie-wrap { flex-direction:column; } }
.aoc-pie-svg { width:120px; height:120px; flex-shrink:0; }
.aoc-pie-legend { flex:1; display:flex; flex-direction:column; gap:5px; }
.aoc-legend-row { display:flex; align-items:center; gap:8px; font-size:12px; color:var(--text-secondary,#94a3b8); }
.aoc-legend-dot { width:8px; height:8px; border-radius:2px; flex-shrink:0; }
.aoc-legend-name { flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; max-width:120px; }
.aoc-legend-val { font-weight:600; color:var(--text-primary,#e2e8f0); }

/* 条形图 */
.aoc-bars-wrap { display:flex; flex-direction:column; gap:8px; max-height:240px; overflow-y:auto; }
.aoc-bar-row { display:flex; align-items:center; gap:8px; }
.aoc-bar-label { width:90px; font-size:11px; color:var(--text-secondary,#94a3b8); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; text-align:right; flex-shrink:0; }
.aoc-bar-track { flex:1; height:14px; background:rgba(51,65,85,.4); border-radius:7px; overflow:hidden; }
.aoc-bar-fill { height:100%; border-radius:7px; transition:width .6s ease; min-width:2px; }
.aoc-bar-pct { width:40px; font-size:11px; font-weight:600; text-align:right; flex-shrink:0; }

/* 运维条 */
.aoc-ops-strip { display:flex; gap:10px; flex-wrap:wrap; }
.aoc-ops-item { background:var(--bg-base,#0f172a); border:1px solid var(--border-subtle,#334155); border-radius:10px; padding:12px 16px; flex:1; min-width:200px; }
.aoc-ops-head { display:flex; align-items:center; gap:8px; margin-bottom:8px; }
.aoc-ops-gw { font-size:13px; font-weight:600; color:var(--text-primary,#e2e8f0); }
.aoc-ops-ver { font-size:11px; color:#a5b4fc; background:rgba(99,102,241,.1); padding:1px 6px; border-radius:4px; }
.aoc-ops-tags { display:flex; gap:6px; flex-wrap:wrap; }
.aoc-ops-tag { font-size:10px; font-weight:600; padding:2px 8px; border-radius:9999px; text-transform:uppercase; letter-spacing:.03em; }
.aot-direct { background:rgba(34,197,94,.12); color:#4ade80; }
.aot-thinking { background:rgba(99,102,241,.12); color:#a5b4fc; }
.aot-streaming { background:rgba(6,182,212,.12); color:#22d3ee; }
.aot-warn { background:rgba(234,179,8,.12); color:#facc15; }
.aot-info { background:rgba(6,182,212,.12); color:#22d3ee; }

/* === Cards View === */
.aoc-cards-view { animation:fadeIn .3s; }
.aoc-agent-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(320px,1fr)); gap:14px; }
@media(max-width:700px) { .aoc-agent-grid { grid-template-columns:1fr; } }
.aoc-agent-card { background:var(--bg-base,#0f172a); border:1px solid var(--border-subtle,#334155); border-radius:12px; padding:16px; transition:all .2s; position:relative; overflow:hidden; }
.aoc-agent-card:hover { border-color:rgba(99,102,241,.35); transform:translateY(-1px); box-shadow:0 4px 12px rgba(0,0,0,.2); }
.aoc-card-active { border-color:rgba(34,197,94,.3); }
.aoc-card-active::before { content:''; position:absolute; top:0; left:0; right:0; height:2px; background:linear-gradient(90deg,#22c55e,#4ade80); }
.aoc-card-error { border-color:rgba(239,68,68,.3); }
.aoc-card-error::before { content:''; position:absolute; top:0; left:0; right:0; height:2px; background:linear-gradient(90deg,#ef4444,#f87171); }

/* Card top */
.aoc-card-top { display:flex; align-items:center; gap:10px; margin-bottom:12px; }
.aoc-card-avatar { width:36px; height:36px; border-radius:10px; display:flex; align-items:center; justify-content:center; color:#fff; font-weight:700; font-size:15px; flex-shrink:0; position:relative; }
.aoc-status-dot { position:absolute; bottom:-2px; right:-2px; width:10px; height:10px; border-radius:50%; border:2px solid var(--bg-base,#0f172a); }
.asd-active { background:#22c55e; box-shadow:0 0 6px rgba(34,197,94,.6); animation:status-pulse 2s ease-in-out infinite; }
.asd-idle { background:#64748b; }
.asd-error { background:#ef4444; box-shadow:0 0 6px rgba(239,68,68,.6); }
@keyframes status-pulse { 0%,100%{box-shadow:0 0 4px rgba(34,197,94,.3)} 50%{box-shadow:0 0 10px rgba(34,197,94,.7)} }
.aoc-card-ids { flex:1; min-width:0; }
.aoc-card-name { font-size:14px; font-weight:700; color:var(--text-primary,#e2e8f0); white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
.aoc-card-gw { font-size:11px; color:var(--text-tertiary,#64748b); margin-top:1px; }
.aoc-card-status-badge { font-size:11px; font-weight:600; padding:2px 8px; border-radius:9999px; flex-shrink:0; }
.acsb-active { background:rgba(34,197,94,.12); color:#4ade80; }
.acsb-idle { background:rgba(100,116,139,.1); color:#64748b; }
.acsb-error { background:rgba(239,68,68,.12); color:#f87171; }

/* Model */
.aoc-card-model { display:flex; align-items:center; gap:6px; font-size:12px; color:var(--text-secondary,#94a3b8); margin-bottom:10px; padding:6px 10px; background:rgba(99,102,241,.06); border-radius:6px; }
.aoc-provider-tag { font-size:10px; background:rgba(99,102,241,.12); color:#a5b4fc; padding:1px 5px; border-radius:4px; margin-left:auto; }

/* Token bar */
.aoc-card-token { margin-bottom:10px; }
.aoc-token-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:4px; }
.aoc-token-label { font-size:11px; color:var(--text-tertiary,#64748b); font-weight:500; }
.aoc-token-nums { font-size:11px; color:var(--text-secondary,#94a3b8); font-family:'SF Mono',Menlo,monospace; }
.aoc-token-bar { height:6px; background:rgba(51,65,85,.5); border-radius:3px; overflow:hidden; }
.aoc-token-fill { height:100%; border-radius:3px; transition:width .6s ease; }
.aoc-token-pct { font-size:11px; font-weight:700; text-align:right; margin-top:2px; }

/* Session tags */
.aoc-card-sessions { display:flex; flex-wrap:wrap; gap:5px; margin-bottom:10px; }
.aoc-sess-tag { font-size:10px; font-weight:600; padding:2px 7px; border-radius:4px; background:rgba(99,102,241,.1); color:#a5b4fc; }
.aoc-sess-iso { background:rgba(234,179,8,.1); color:#facc15; }
.aoc-sess-sub { background:rgba(6,182,212,.1); color:#22d3ee; }
.aoc-sess-other { background:rgba(100,116,139,.1); color:#94a3b8; }

/* Card footer */
.aoc-card-footer { display:flex; align-items:center; gap:10px; padding-top:8px; border-top:1px solid rgba(51,65,85,.4); font-size:11px; color:var(--text-tertiary,#64748b); }
.aoc-footer-item { display:flex; align-items:center; gap:3px; }
.aoc-channel-icons { display:flex; gap:2px; }
.aoc-ch-icon { font-size:12px; }
.aoc-footer-time { margin-left:auto; }

/* Warn banner */
.aoc-card-warn { display:flex; align-items:center; gap:6px; margin-top:8px; padding:6px 10px; background:rgba(239,68,68,.08); border:1px solid rgba(239,68,68,.2); border-radius:6px; font-size:11px; font-weight:600; color:#f87171; }

/* === Collaboration View === */
.aoc-collab-view { animation:fadeIn .3s; display:flex; flex-direction:column; gap:16px; }
.aoc-collab-gw { background:var(--bg-base,#0f172a); border:1px solid var(--border-subtle,#334155); border-radius:10px; padding:16px; }
.aoc-collab-gw-head { display:flex; align-items:center; gap:8px; margin-bottom:12px; padding-bottom:8px; border-bottom:1px solid rgba(51,65,85,.4); padding-left:10px; border-left:3px solid #6366f1; }
.aoc-collab-gw-name { font-size:14px; font-weight:700; color:var(--text-primary,#e2e8f0); }
.aoc-collab-gw-count { font-size:11px; color:var(--text-tertiary,#64748b); }
.aoc-collab-agent { margin-bottom:12px; padding-left:12px; border-left:2px solid rgba(99,102,241,.15); }
.aoc-collab-agent-head { display:flex; align-items:center; gap:8px; margin-bottom:6px; }
.aoc-collab-avatar { width:24px; height:24px; border-radius:6px; display:flex; align-items:center; justify-content:center; color:#fff; font-weight:700; font-size:11px; flex-shrink:0; }
.aoc-collab-agent-name { font-size:13px; font-weight:600; color:var(--text-primary,#e2e8f0); }
.aoc-collab-model { font-size:11px; color:var(--text-tertiary,#64748b); }
.aoc-collab-sessions { margin-left:16px; display:flex; flex-direction:column; gap:4px; }
.aoc-collab-sess { display:flex; align-items:center; gap:8px; padding:5px 10px; border-radius:6px; font-size:12px; color:var(--text-secondary,#94a3b8); background:rgba(51,65,85,.2); }
.aoc-collab-sess-icon { font-size:13px; flex-shrink:0; }
.aoc-collab-sess-type { font-weight:600; width:56px; flex-shrink:0; }
.acs-main .aoc-collab-sess-type { color:#a5b4fc; }
.acs-direct .aoc-collab-sess-type { color:#a5b4fc; }
.acs-isolated .aoc-collab-sess-type { color:#facc15; }
.acs-sub .aoc-collab-sess-type { color:#22d3ee; }
.acs-subagent .aoc-collab-sess-type { color:#22d3ee; }
.aoc-collab-sess-ch { width:60px; flex-shrink:0; }
.aoc-collab-sess-model { flex:1; color:var(--text-tertiary,#475569); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.aoc-collab-sess-tokens { font-weight:600; width:50px; text-align:right; flex-shrink:0; color:var(--text-primary,#e2e8f0); }
.aoc-collab-sess-time { font-size:11px; width:64px; text-align:right; flex-shrink:0; color:var(--text-tertiary,#64748b); }
.aoc-collab-cron { margin-top:10px; padding-top:8px; border-top:1px dashed rgba(51,65,85,.4); }
.aoc-collab-cron-title { font-size:12px; font-weight:600; color:var(--text-secondary,#94a3b8); margin-bottom:6px; }
.aoc-collab-cron-item { display:flex; align-items:center; gap:8px; padding:4px 10px; font-size:12px; color:var(--text-secondary,#94a3b8); }
.aoc-cron-name { font-weight:500; flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }

/* === Users View === */
.aoc-users-view { animation:fadeIn .3s; }
.aoc-users-grid { display:grid; grid-template-columns:repeat(auto-fill,minmax(280px,1fr)); gap:14px; }
@media(max-width:600px) { .aoc-users-grid { grid-template-columns:1fr; } }
.aoc-user-card { background:var(--bg-base,#0f172a); border:1px solid var(--border-subtle,#334155); border-radius:12px; padding:16px; transition:border-color .2s; }
.aoc-user-card:hover { border-color:rgba(99,102,241,.3); }
.aoc-user-top { display:flex; align-items:center; gap:10px; margin-bottom:14px; }
.aoc-user-avatar { width:36px; height:36px; border-radius:50%; display:flex; align-items:center; justify-content:center; color:#fff; font-weight:700; font-size:15px; flex-shrink:0; }
.aoc-user-info { flex:1; min-width:0; }
.aoc-user-name { font-size:14px; font-weight:600; color:var(--text-primary,#e2e8f0); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.aoc-user-channel { font-size:11px; color:var(--text-tertiary,#64748b); margin-top:1px; }

/* User ring */
.aoc-user-ring-row { display:flex; align-items:center; gap:16px; margin-bottom:12px; }
.aoc-user-ring-svg { width:80px; height:80px; flex-shrink:0; }
.aoc-user-ring-detail { flex:1; display:flex; flex-direction:column; gap:5px; }
.aoc-urd-row { display:flex; justify-content:space-between; font-size:12px; }
.aoc-urd-label { color:var(--text-tertiary,#64748b); }
.aoc-urd-val { color:var(--text-primary,#e2e8f0); font-weight:600; }

/* User agent tags */
.aoc-user-agents { display:flex; flex-wrap:wrap; gap:5px; }
.aoc-user-agent-tag { display:flex; align-items:center; gap:4px; font-size:10px; font-weight:500; padding:2px 8px; border-radius:4px; background:rgba(99,102,241,.08); color:var(--text-secondary,#94a3b8); }
.aoc-ua-dot { width:6px; height:6px; border-radius:50%; flex-shrink:0; }

/* Skills View */
.aoc-skills-view { animation:fadeIn .3s; }
.skills-stat-strip { display:flex; gap:8px; margin-bottom:16px; }
.skills-search { margin-bottom:16px; position:relative; }
.skills-search-input { width:100%; padding:8px 12px; background:var(--bg-base,#0f172a); border:1px solid var(--border-subtle,#334155); border-radius:8px; color:var(--text-primary,#e2e8f0); font-size:13px; outline:none; box-sizing:border-box; transition:border-color .15s; }
.skills-search-input:focus { border-color:#6366f1; }
.skills-search-count { position:absolute; right:12px; top:50%; transform:translateY(-50%); font-size:11px; color:var(--text-tertiary,#64748b); }
.skill-group { background:var(--bg-base,#0f172a); border:1px solid var(--border-subtle,#334155); border-radius:10px; overflow:hidden; margin-bottom:10px; }
.skill-group-header { display:flex; align-items:center; gap:8px; padding:10px 14px; cursor:pointer; user-select:none; transition:background .15s; }
.skill-group-header:hover { background:rgba(99,102,241,.04); }
.skill-cat-tag { font-size:12px; font-weight:600; padding:2px 10px; border-radius:9999px; }
.scat-global { background:rgba(6,182,212,.12); color:#22d3ee; }
.scat-user { background:rgba(34,197,94,.12); color:#4ade80; }
.scat-workspace { background:rgba(245,158,11,.12); color:#fbbf24; }
.skill-group-count { font-size:11px; color:var(--text-tertiary,#64748b); margin-left:auto; }
.skill-group-body { border-top:1px solid var(--border-subtle,#334155); }
.skill-item { display:flex; align-items:flex-start; gap:10px; padding:8px 14px; border-bottom:1px solid rgba(51,65,85,.3); transition:background .15s; }
.skill-item:last-child { border-bottom:none; }
.skill-item:hover { background:rgba(99,102,241,.03); }
.skill-icon { font-size:16px; flex-shrink:0; margin-top:2px; }
.skill-info { flex:1; min-width:0; }
.skill-name { font-size:13px; font-weight:600; color:var(--text-primary,#e2e8f0); }
.skill-desc { font-size:11px; color:var(--text-tertiary,#64748b); margin-top:2px; line-height:1.4; overflow:hidden; text-overflow:ellipsis; display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; }
.skill-badges { display:flex; gap:4px; flex-shrink:0; align-items:center; }
.skill-badge { font-size:10px; font-weight:600; padding:1px 6px; border-radius:4px; }
.sb-ok { background:rgba(34,197,94,.1); color:#4ade80; }
.sb-ws { background:rgba(245,158,11,.1); color:#fbbf24; max-width:100px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.ams-amber { color:#fbbf24; }

/* SVG icon alignment */
.dtab-icon { display:inline-flex; vertical-align:-2px; }
.dtab-icon svg { width:14px; height:14px; }
.aoc-vbtn svg { width:14px; height:14px; vertical-align:-2px; }
.aoc-ch-icon svg { width:12px; height:12px; vertical-align:-1px; }
.aoc-collab-sess-icon { display:inline-flex; align-items:center; }
.aoc-collab-sess-icon svg { width:12px; height:12px; }
.skill-icon svg { width:16px; height:16px; }
.skill-cat-tag svg { width:12px; height:12px; vertical-align:-2px; }
.eye-btn svg { vertical-align:middle; }

/* Session History / Replay */
.session-row { cursor:pointer; transition:background .15s; }
.session-row:hover { background:rgba(99,102,241,.06); }
.session-detail-row { cursor:default !important; }
.session-detail-row:hover { background:transparent !important; }
.session-detail-row td { padding:0 !important; border-bottom:1px solid rgba(51,65,85,.3); }
.session-replay { max-height:400px; overflow-y:auto; padding:12px 16px; background:rgba(15,23,42,.4); border-radius:0 0 8px 8px; animation:slideDown .2s; }
.chat-messages { display:flex; flex-direction:column; gap:10px; }
.chat-msg { padding:8px 12px; border-radius:8px; }
.msg-user { background:rgba(99,102,241,.08); border-left:3px solid #6366f1; }
.msg-assistant { background:rgba(34,197,94,.06); border-left:3px solid #22c55e; }
.msg-system { background:rgba(245,158,11,.06); border-left:3px solid #eab308; font-size:12px; }
.msg-tool { background:rgba(6,182,212,.06); border-left:3px solid #06b6d4; font-size:12px; }
.msg-role { font-size:11px; font-weight:600; text-transform:uppercase; margin-bottom:4px; color:var(--text-tertiary,#64748b); letter-spacing:.03em; }
.msg-content { font-size:13px; line-height:1.5; white-space:pre-wrap; word-break:break-word; color:var(--text-primary,#e2e8f0); }
.msg-meta { font-size:10px; color:var(--text-tertiary,#64748b); margin-top:4px; }
.msg-expand-btn { background:none; border:none; color:#6366f1; cursor:pointer; font-size:11px; padding:2px 0; margin-top:4px; transition:color .15s; }
.msg-expand-btn:hover { color:#a5b4fc; text-decoration:underline; }

/* v29.0: Chat 发送区 */
.chat-actions { display:flex; gap:8px; align-items:center; padding:10px 0 2px; border-top:1px solid rgba(51,65,85,.3); margin-top:10px; }
.chat-input { flex:1; background:rgba(30,41,59,.6); border:1px solid var(--border-subtle,#334155); border-radius:6px; padding:6px 10px; color:var(--text-primary,#e2e8f0); font-size:13px; outline:none; }
.chat-input:focus { border-color:#6366f1; }

/* v29.0: 危险按钮 */
.btn-danger-ghost { color:#ef4444 !important; }
.btn-danger-ghost:hover { background:rgba(239,68,68,.12) !important; }
.btn-warn { background:rgba(234,179,8,.15); color:#eab308; border:1px solid rgba(234,179,8,.3); }
.btn-warn:hover { background:rgba(234,179,8,.25); }

/* v29.0: 文件编辑器 */
.aoc-files-view { padding:0; }
.files-layout { display:flex; min-height:520px; max-height:70vh; }
.files-sidebar { width:220px; border-right:1px solid var(--border-subtle,#334155); padding:10px; flex-shrink:0; overflow-y:auto; }
.files-agent-select { margin-bottom:8px; }
.files-list { display:flex; flex-direction:column; gap:2px; }
.file-item { display:flex; align-items:center; gap:6px; padding:5px 8px; border-radius:4px; cursor:pointer; font-size:13px; color:var(--text-secondary,#94a3b8); transition:background .15s; }
.file-item:hover { background:rgba(99,102,241,.08); }
.file-item.active { background:rgba(99,102,241,.15); color:#a5b4fc; }
.files-editor { flex:1; display:flex; flex-direction:column; overflow:hidden; }
.editor-header { display:flex; align-items:center; gap:8px; padding:8px 14px; border-bottom:1px solid var(--border-subtle,#334155); background:rgba(15,23,42,.3); }
.badge-unsaved { font-size:10px; background:rgba(234,179,8,.2); color:#eab308; padding:2px 8px; border-radius:3px; }
.file-textarea { flex:1; background:rgba(15,23,42,.2); border:none; padding:14px 16px; color:var(--text-primary,#e2e8f0); font-family:'JetBrains Mono','Fira Code','Cascadia Code',monospace; font-size:13px; line-height:1.7; resize:none; outline:none; min-height:460px; tab-size:2; }

/* v29.0: 心跳/设备 */
.aoc-heartbeat-view { padding:12px 0; }
.hb-cards { display:grid; grid-template-columns:repeat(auto-fill,minmax(260px,1fr)); gap:12px; }
.hb-card { background:rgba(30,41,59,.4); border:1px solid var(--border-subtle,#334155); border-radius:8px; padding:14px; }
.hb-card h4 { margin:0 0 10px; font-size:14px; }
.hb-row { display:flex; justify-content:space-between; align-items:center; padding:3px 0; font-size:13px; }
.hb-device { display:flex; justify-content:space-between; align-items:center; padding:4px 0; font-size:13px; border-bottom:1px solid rgba(51,65,85,.2); }
.hb-actions { display:flex; gap:8px; }

</style>