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
                        <button v-for="t in tabs" :key="t.key" class="dtab" :class="{ active: activeTab === t.key }" @click="switchTab(t.key, up.id)" v-show="t.key !== 'agent' || up.gateway_status === 'connected'">{{ t.icon }} {{ t.label }}</button>
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
                      <!-- Agent Tab (AOC per-upstream) -->
                      <div v-if="activeTab === 'agent'" class="dtab-body aoc-section aoc-inline">
                        <div class="section-header">
                          <h3 class="section-title">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
                            Agent 运营中心 <span class="badge-count">{{ expandedAgents.length }}</span>
                          </h3>
                          <div class="aoc-view-toggle">
                            <button class="aoc-vbtn" :class="{ active: aocView === 'dashboard' }" @click="aocView='dashboard'" title="仪表盘">📊</button>
                            <button class="aoc-vbtn" :class="{ active: aocView === 'cards' }" @click="aocView='cards'" title="详情卡片">🃏</button>
                            <button class="aoc-vbtn" :class="{ active: aocView === 'collab' }" @click="aocView='collab'" title="协作视图">🌳</button>
                            <button class="aoc-vbtn" :class="{ active: aocView === 'users' }" @click="aocView='users'" title="用户归因">👥</button>
                            <button class="aoc-vbtn" :class="{ active: aocView === 'skills' }" @click="switchToSkills" title="Skill 目录">🧩</button>
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
                                    <span v-for="ch in ag.channels" :key="ch" class="aoc-ch-icon" :title="ch">{{ channelIcon(ch) }}</span>
                                  </span>
                                </div>
                                <div class="aoc-footer-item" v-if="ag.users.length > 0" :title="'用户: ' + ag.users.join(', ')">
                                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
                                  <span>{{ ag.users.length }}</span>
                                </div>
                                <div class="aoc-footer-item aoc-footer-time" :title="'最后活跃: ' + new Date(ag.lastActive).toLocaleString()">{{ fmtTime(ag.lastActive) }}</div>
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
                                  <span class="aoc-collab-sess-icon">{{ sessionKindIcon(sess.kind) }}</span>
                                  <span class="aoc-collab-sess-type">{{ sess.kind }}</span>
                                  <span class="aoc-collab-sess-ch">{{ sess.channel || '—' }}</span>
                                  <span class="aoc-collab-sess-model">{{ sess.model || '—' }}</span>
                                  <span class="aoc-collab-sess-tokens">{{ fmtTokensShort(sess.totalTokens || 0) }}</span>
                                  <span class="aoc-collab-sess-time">{{ fmtTime(sess.updatedAt) }}</span>
                                </div>
                              </div>
                            </div>
                            <div v-if="gw.cronJobs && gw.cronJobs.length > 0" class="aoc-collab-cron">
                              <div class="aoc-collab-cron-title">⏰ 定时任务</div>
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
                          </div>
                          <div v-if="skillsLoading" class="skel-lines"><div class="skel-line" v-for="i in 6" :key="i"></div></div>
                          <template v-else>
                            <div v-for="group in groupedSkills" :key="group.category" class="skill-group">
                              <div class="skill-group-header" @click="group.expanded = !group.expanded">
                                <span class="skill-cat-tag" :class="'scat-' + group.catKey">{{ group.label }}</span>
                                <span class="skill-group-count">{{ group.skills.length }}</span>
                                <svg :class="{ 'expand-chevron': true, open: group.expanded }" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
                              </div>
                              <div v-if="group.expanded" class="skill-group-body">
                                <div v-for="sk in group.skills" :key="sk.name + sk.category" class="skill-item">
                                  <div class="skill-icon">🧩</div>
                                  <div class="skill-info">
                                    <div class="skill-name">{{ sk.name }}</div>
                                    <div class="skill-desc" v-if="sk.description">{{ sk.description }}</div>
                                  </div>
                                  <div class="skill-badges">
                                    <span v-if="sk.has_skill_md" class="skill-badge sb-ok">SKILL.md</span>
                                    <span v-if="sk.workspace" class="skill-badge sb-ws" :title="sk.workspace">{{ sk.workspace.substring(0, 12) }}...</span>
                                  </div>
                                </div>
                              </div>
                            </div>
                            <div v-if="filteredSkills.length === 0 && !skillsLoading" class="dtab-empty">{{ skillSearch ? '无匹配 skill' : '暂无 skill 数据' }}</div>
                          </template>
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
  { key: 'agent', icon: '👥', label: 'Agent' },
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
      const label = catKey === 'global' ? '🌐 全局 Skills' : catKey === 'user' ? '👤 用户 Skills' : '📁 Workspace Skills'
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
  const map = { lanxin: '📱', telegram: '✈️', discord: '🎮', whatsapp: '📞', slack: '💼', web: '🌐', api: '⚡' }
  return map[ch] || '💬'
}
function sessionKindIcon(kind) {
  const map = { main: '🏠', direct: '🏠', isolated: '🔒', sub: '🔗', subagent: '🔗', other: '📄' }
  return map[kind] || '📄'
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
  expandedId.value = up.id; activeTab.value = 'sessions'; diagResult.value = null
  if (up.token_configured && up.gateway_status !== 'not_configured') await loadTabData(up.id, 'sessions')
}
async function switchTab(tab, id) { activeTab.value = tab; if (tab === 'agent') { aocView.value = 'dashboard'; await loadTabData(id, 'cron') } else if (tab !== 'diag') await loadTabData(id, tab) }
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

</style>