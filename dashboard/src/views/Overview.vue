<template>
  <div class="overview-page">
    <!-- 顶部工具栏 -->
    <div class="overview-toolbar">
      <div class="toolbar-left">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
        <span class="toolbar-title">安全驾驶舱</span>
        <span class="last-refresh" :title="'上次刷新: ' + lastRefreshTime">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
          {{ lastRefreshDisplay }}
        </span>
      </div>
      <div class="toolbar-right">
        <div class="time-range-group">
          <button v-for="tr in timeRangeOptions" :key="tr.value" class="tr-btn" :class="{ active: timeRange === tr.value }" @click="setTimeRange(tr.value)">{{ tr.label }}</button>
        </div>
        <button class="toolbar-btn refresh-btn" @click="manualRefresh" :class="{ spinning: refreshing }" title="手动刷新">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
        </button>
        <div class="refresh-interval-wrap">
          <select v-model="refreshInterval" @change="onRefreshChange" class="refresh-select" title="自动刷新间隔">
            <option value="30000">30s</option><option value="60000">1m</option><option value="300000">5m</option><option value="0">手动</option>
          </select>
        </div>
      </div>
    </div>

    <!-- 安全驾驶舱：健康评分 + 系统状态 -->
    <div class="cockpit-section" v-if="healthScore">
      <div class="cockpit-body">
        <div class="cockpit-left" @click="showScoreDetail = !showScoreDetail" title="点击查看评分详情">
          <div class="score-ring-wrap">
            <svg viewBox="0 0 120 120" class="score-ring-svg">
              <defs>
                <linearGradient id="scoreGrad" x1="0%" y1="0%" x2="100%" y2="100%">
                  <stop offset="0%" :stop-color="scoreColor" stop-opacity="1"/>
                  <stop offset="100%" :stop-color="scoreColorEnd" stop-opacity="0.7"/>
                </linearGradient>
              </defs>
              <circle cx="60" cy="60" r="52" fill="none" :stroke="scoreColor" stroke-width="10" opacity="0.12"/>
              <circle cx="60" cy="60" r="52" fill="none" stroke="url(#scoreGrad)" stroke-width="10" stroke-linecap="round" :stroke-dasharray="scoreDash" stroke-dashoffset="0" transform="rotate(-90 60 60)" class="score-ring-progress"/>
            </svg>
            <div class="score-center">
              <div class="score-number" :style="{ color: scoreColor }">{{ animatedScore }}</div>
              <div class="score-label" :style="{ color: scoreColor }">{{ healthScore.level_label }}</div>
            </div>
          </div>
          <div class="score-expand-hint">{{ showScoreDetail ? '收起详情' : '点击展开' }}</div>
        </div>
        <div class="cockpit-center">
          <div class="score-desc">
            <span class="score-level-badge" :class="'badge-' + healthScore.level">{{ healthScore.level_label }}</span>
            <span class="score-text">安全健康分 {{ healthScore.score }}/100</span>
            <span class="time-badge" title="健康分固定使用7天评估窗口">7d</span>
          </div>
          <Transition name="slide-down">
            <div v-if="showScoreDetail && healthScore.deductions && healthScore.deductions.length" class="deduction-list">
              <div v-for="d in healthScore.deductions" :key="d.name" class="deduction-item">
                <span class="deduction-name">{{ d.name }}</span>
                <span class="deduction-points">-{{ d.points }}</span>
                <span class="deduction-detail">{{ d.detail }}</span>
                <router-link v-if="deductionLink(d.name)" :to="deductionLink(d.name)" class="deduction-jump" title="查看详情" @click.stop>→</router-link>
              </div>
            </div>
            <div v-else-if="showScoreDetail" class="deduction-empty">✅ 未发现安全风险</div>
          </Transition>
          <div class="anomaly-indicator" v-if="anomalyStatus">
            <router-link to="/anomaly" class="anomaly-link" v-if="anomalyStatus.alerts_24h > 0">⚠️ 检测到 {{ anomalyStatus.alerts_24h }} 个异常</router-link>
            <span class="anomaly-learning" v-else-if="anomalyStatus.baselines_ready < anomalyStatus.metrics_count"><Icon name="bar-chart" :size="14" /> 基线学习中 ({{ anomalyStatus.baselines_ready }}/{{ anomalyStatus.metrics_count }} 就绪)</span>
            <router-link to="/anomaly" class="anomaly-ok" v-else>✅ 异常检测正常 ({{ anomalyStatus.metrics_count }} 指标)</router-link>
          </div>
          <div class="trend-mini" v-if="healthScore.trend && healthScore.trend.length">
            <div class="trend-mini-label">7天趋势</div>
            <svg :viewBox="'0 0 200 50'" class="trend-mini-svg">
              <polyline :points="trendMiniPoints" fill="none" :stroke="scoreColor" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>
              <circle v-for="(p, i) in trendMiniPointsArr" :key="i" :cx="p.x" :cy="p.y" r="3" :fill="scoreColor"/>
            </svg>
            <div class="trend-mini-dates"><span v-for="t in healthScore.trend" :key="t.date">{{ t.date.substring(5) }}</span></div>
          </div>
        </div>
        <div class="cockpit-right" v-if="systemHealth">
          <div class="sys-health-title">系统状态</div>
          <div class="sys-metric" v-for="m in sysMetrics" :key="m.label">
            <div class="sys-metric-header"><span class="sys-metric-label">{{ m.label }}</span><span class="sys-metric-value" :style="{ color: m.color }">{{ m.display }}</span></div>
            <div class="sys-metric-bar"><div class="sys-metric-fill" :style="{ width: Math.min(m.pct, 100) + '%', background: m.color }"></div></div>
          </div>
          <div class="sys-goroutines"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg> Goroutines: {{ systemHealth.goroutines }}</div>
        </div>
      </div>
    </div>

    <!-- 指标卡片 -->
    <div class="ov-cards" v-if="loaded">
      <StatCard :iconSvg="svgGlobe" :value="stats.total" label="总请求" :badge="timeRange" color="blue" :change="stats.totalChange" :changeUp="stats.totalUp" class="stat-clickable" :class="{ 'stat-flash': flashCards }" @click="router.push({ path: '/audit', query: { since: timeRange } })"/>
      <StatCard :iconSvg="svgShieldX" :value="stats.blocked" label="拦截数" :badge="timeRange" color="indigo" :change="stats.blockedChange" :changeUp="false" class="stat-clickable" :class="{ 'stat-flash': flashCards }" @click="router.push({ path: '/audit', query: { action: 'block', since: timeRange } })"/>
      <StatCard :iconSvg="svgAlertTriangle" :value="stats.warned" label="告警数" :badge="timeRange" color="yellow" :change="stats.warnedChange" :changeUp="false" class="stat-clickable" :class="{ 'stat-flash': flashCards }" @click="router.push({ path: '/audit', query: { action: 'warn', since: timeRange } })"/>
      <StatCard :iconSvg="svgPercent" :value="stats.rate" label="拦截率" :badge="timeRange" color="indigo" class="stat-clickable" :class="{ 'stat-flash': flashCards }" @click="router.push({ path: '/rules' })"/>
      <StatCard :iconSvg="svgUserDanger" :value="highRiskUserCount" label="高危用户" badge="30d" color="red" class="stat-clickable" :class="{ 'stat-flash': flashCards }" @click="router.push('/user-profiles')"/>
      <StatCard :iconSvg="svgGlobe" :value="sourceCategoryCount" label="来源分类" badge="LLM" color="green" class="stat-clickable" :class="{ 'stat-flash': flashCards }" @click="router.push({ path: '/sessions' })"/>
      <StatCard :iconSvg="svgIFC" :value="ifcStats ? ifcStats.total_violations : '--'" label="IFC 违规" badge="all" color="purple" class="stat-clickable" :class="{ 'stat-flash': flashCards }" @click="router.push('/ifc')"/>
      <StatCard :iconSvg="svgDeviation" :value="deviationStats ? deviationStats.total_deviations : '--'" label="计划偏差" badge="all" color="orange" class="stat-clickable" :class="{ 'stat-flash': flashCards }" @click="router.push('/deviations')"/>
      <StatCard :iconSvg="svgCapDeny" :value="capabilityStats ? capabilityStats.deny_count : '--'" label="能力拒绝" badge="all" color="orange" class="stat-clickable" :class="{ 'stat-flash': flashCards }" @click="router.push('/capability')"/>
    </div>
    <div class="ov-cards" v-else><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>

    <!-- 快捷操作 + 最近告警 -->
    <div class="quick-row">
      <div class="card quick-card">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg></span><span class="card-title">快捷操作</span></div>
        <div class="qa-grid">
          <button class="qa-btn" @click="hotReloadRules" :disabled="actionLoading.reload">
            <div class="qa-icon-wrap qa-reload"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg></div>
            <div class="qa-text"><div class="qa-label">热更新规则</div><div class="qa-desc">重新加载所有规则</div></div>
          </button>
          <button class="qa-btn" @click="clearCache" :disabled="actionLoading.cache">
            <div class="qa-icon-wrap qa-cache"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg></div>
            <div class="qa-text"><div class="qa-label">清理缓存</div><div class="qa-desc">清除 LLM 响应缓存</div></div>
          </button>
          <button class="qa-btn" @click="goReport">
            <div class="qa-icon-wrap qa-report"><Icon name="bar-chart" :size="18" /></div>
            <div class="qa-text"><div class="qa-label">生成报告</div><div class="qa-desc">创建安全分析报告</div></div>
          </button>
          <button class="qa-btn" @click="router.push('/rules')">
            <div class="qa-icon-wrap qa-rules"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg></div>
            <div class="qa-text"><div class="qa-label">规则管理</div><div class="qa-desc">查看和编辑规则</div></div>
          </button>
        </div>
      </div>
      <div class="card alerts-card">
        <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg></span><span class="card-title">最近告警</span><router-link to="/audit?action=block" class="card-more">查看全部 →</router-link></div>
        <Skeleton v-if="!loaded" type="table"/>
        <EmptyState v-else-if="!recentAttacks.length" :iconSvg="svgShieldCheck" title="当前环境安全" description="没有检测到攻击事件"/>
        <div v-else class="alert-list">
          <div v-for="a in recentAttacks" :key="a.id || a.trace_id || a.timestamp" class="alert-item" @click="$router.push('/audit')">
            <div class="alert-time">{{ fmtTimeShort(a.timestamp || a.time) }}</div>
            <div class="alert-body">
              <span class="alert-dir" :class="a.direction === 'inbound' ? 'dir-in' : 'dir-out'">{{ a.direction === 'inbound' ? '入站' : '出站' }}</span>
              <span class="alert-reason">{{ a.reason || '--' }}</span>
            </div>
            <div class="alert-sender">
              <a v-if="a.sender_id" class="user-link" @click.stop="$router.push('/user-profiles/' + encodeURIComponent(a.sender_id))">{{ a.sender_id }}</a>
              <span v-else>--</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 趋势图 + 健康状态 -->
    <div class="ov-row">
      <div class="card"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span><span class="card-title">请求趋势</span></div>
        <Skeleton v-if="!loaded" type="chart"/><EmptyState v-else-if="!trendData.length" :iconSvg="svgTrend" title="暂无趋势数据" description="系统运行后将自动收集趋势数据"/>
        <TrendChart v-else :data="trendChartData" :lines="trendLines" :xLabels="trendXLabels" :height="170" :timeRanges="[{label:'24h',value:'24h'},{label:'7d',value:'7d'}]" :currentRange="trendRange" @rangeChange="onTrendRangeChange"/></div>
      <div class="card"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span><span class="card-title">健康状态</span></div>
        <Skeleton v-if="!loaded" type="text"/><EmptyState v-else-if="!healthBars.length" :iconSvg="svgHeart" title="无健康数据" description="等待系统上报健康信息"/>
        <div v-else><div class="hb-row" v-for="hb in healthBars" :key="hb.name"><span class="hb-label">{{hb.name}}</span><div class="hb-track"><div class="hb-fill" :style="{width:Math.max(5,hb.pct)+'%',background:hb.color}"></div></div><span class="hb-val" :style="{color:hb.color}">{{hb.val}}</span></div></div></div>
    </div>

    <!-- 饼图 + 规则命中 -->
    <div class="ov-row">
      <div class="card"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.21 15.89A10 10 0 1 1 8 2.83"/><path d="M22 12A10 10 0 0 0 12 2v10z"/></svg></span><span class="card-title">拦截类型分布</span></div>
        <Skeleton v-if="!loaded" type="chart"/><PieChart v-else :data="pieData" :size="180"/></div>
      <div class="card"><div class="card-header"><span class="card-icon"><Icon name="globe" :size="16" /></span><span class="card-title">来源分类分布</span><router-link to="/agent" class="card-more">查看工具审计 →</router-link></div>
        <Skeleton v-if="!loaded" type="text"/><EmptyState v-else-if="!sourceCategoryRows.length" :iconSvg="svgGlobe" title="暂无来源分类数据" description="LLM 工具调用产生来源分类后，这里会显示 public_web / internal_api / external_api 等分布"/>
        <div v-else><TransitionGroup name="list-anim" tag="div"><div class="hbar-row hbar-row-clickable" v-for="(r,i) in sourceCategoryRows" :key="r.category" @click="goToSourceCategory(r.category)"><span class="hbar-rank">#{{i+1}}</span><span class="hbar-name" :title="r.category">{{r.category}}</span><div class="hbar-track"><div class="hbar-fill hbar-fill-anim" :style="{'--target-w':Math.max(5,r.pct)+'%',background:sourceBarColors[i%sourceBarColors.length]}">{{r.count}}</div></div></div></TransitionGroup></div></div>
    </div>

    <div class="ov-row">
      <div class="card"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg></span><span class="card-title">规则命中 TOP5</span></div>
        <Skeleton v-if="!loaded" type="text"/><EmptyState v-else-if="!topRules.length" :iconSvg="svgTarget" title="规则正在保护中" description="命中数据将在检测到威胁后显示"/>
        <div v-else><TransitionGroup name="list-anim" tag="div"><div class="hbar-row" v-for="(r,i) in topRules" :key="r.name"><span class="hbar-rank">#{{i+1}}</span><span class="hbar-name" :title="r.name">{{r.name}}</span><div class="hbar-track"><div class="hbar-fill hbar-fill-anim" :style="{'--target-w':Math.max(5,r.pct)+'%',background:barColors[i%barColors.length]}">{{r.hits}}</div></div></div></TransitionGroup></div></div>
      <div class="card"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg></span><span class="card-title">最近攻击事件</span></div>
        <Skeleton v-if="!loaded" type="table"/><EmptyState v-else-if="!recentAttacks.length" :iconSvg="svgShieldCheck" title="当前环境安全" description="没有检测到攻击事件"/>
        <div v-else class="table-wrap"><table><thead><tr><th>时间</th><th>方向</th><th>发送者</th><th>原因</th></tr></thead>
          <TransitionGroup name="list-anim" tag="tbody"><tr v-for="a in recentAttacks" :key="a.id||a.trace_id||a.timestamp" class="row-block" style="cursor:pointer" @click="$router.push('/audit')"><td>{{fmtTime(a.timestamp||a.time)}}</td><td>{{a.direction==='inbound'?'入站':'出站'}}</td><td><a v-if="a.sender_id" class="user-link" @click.stop="$router.push('/user-profiles/'+encodeURIComponent(a.sender_id))">{{a.sender_id}}</a><span v-else>--</span></td><td>{{a.reason||'--'}}</td></tr></TransitionGroup></table></div></div>
    </div>

    <!-- 热力图 -->
    <div class="card" style="margin-bottom:20px"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg></span><span class="card-title">7 天攻击频率热力图</span></div>
      <Skeleton v-if="!loaded" type="chart"/><EmptyState v-else-if="!heatmapData.length" :iconSvg="svgGrid" title="暂无热力图数据" description="系统运行 24 小时后将生成攻击频率热力图"/><HeatMap v-else :data="heatmapData" title=""/></div>

    <!-- 安全洞察 -->
    <div class="card" style="margin-bottom:20px" v-if="summaryLoaded">
      <div class="card-header"><span class="card-icon"><Icon name="search" :size="16" /></span><span class="card-title">安全洞察</span></div>
      <div class="insight-grid">
        <div class="insight-card" @click="router.push('/redteam')"><div class="insight-header"><Icon name="crosshair" :size="14" /> 红队测试</div><div class="insight-value" :class="summaryRedteamClass">{{ summaryRedteamRate }}%</div><div class="insight-sub">检测率 · {{ summaryRedteamVulns }} 个漏洞</div></div>
        <div class="insight-card" @click="router.push('/honeypot')"><div class="insight-header">🍯 蜜罐</div><div class="insight-value">{{ summary.honeypot?.total_triggers || 0 }}</div><div class="insight-sub">触发 · {{ summary.honeypot?.total_detonated || 0 }} 引爆</div></div>
        <div class="insight-card" @click="router.push('/attack-chains')"><div class="insight-header">🔗 攻击链</div><div class="insight-value" :class="(summary.attack_chains?.critical_chains||0) > 0 ? 'danger' : ''">{{ summary.attack_chains?.active_chains || 0 }}</div><div class="insight-sub">活跃链 · {{ summary.attack_chains?.critical_chains || 0 }} 高危</div></div>
        <div class="insight-card" @click="router.push('/leaderboard')"><div class="insight-header"><Icon name="trophy" :size="14" /> 排行榜</div><div class="insight-value">{{ summaryTopTenant }}</div><div class="insight-sub">{{ summaryTopScore }} 分 · TOP1</div></div>
        <div class="insight-card" @click="router.push('/behavior')"><div class="insight-header"><Icon name="behavior" :size="14" /> 行为画像</div><div class="insight-value" :class="(summary.behavior?.high_risk||0) > 0 ? 'warning' : ''">{{ summary.behavior?.anomaly_count || 0 }}</div><div class="insight-sub">行为突变 · {{ summary.behavior?.high_risk || 0 }} 高风险</div></div>
        <div class="insight-card" @click="router.push('/ab-testing')"><div class="insight-header"><Icon name="split" :size="14" /> A/B 测试</div><div class="insight-value">{{ summary.ab_testing?.active_tests || 0 }}</div><div class="insight-sub">进行中 · {{ summary.ab_testing?.total_tests || 0 }} 总计</div></div>
        <div class="insight-card governance-insight" v-if="governanceSummary" @click="router.push('/ifc')">
          <div class="insight-header"><Icon name="shield" :size="14" /> 治理引擎</div>
          <div class="governance-engines">
            <div class="gov-engine-row"><span class="gov-engine-name">IFC</span><span class="gov-engine-status" :class="governanceSummary.ifc.enabled ? 'enabled' : 'disabled'">{{ governanceSummary.ifc.enabled ? '启用' : '停用' }}</span><span class="gov-engine-count">{{ governanceSummary.ifc.violations }} 违规</span></div>
            <div class="gov-engine-row"><span class="gov-engine-name">偏差检测</span><span class="gov-engine-status" :class="governanceSummary.deviations.enabled ? 'enabled' : 'disabled'">{{ governanceSummary.deviations.enabled ? '启用' : '停用' }}</span><span class="gov-engine-count">{{ governanceSummary.deviations.detected }} 偏差</span></div>
            <div class="gov-engine-row"><span class="gov-engine-name">Capability</span><span class="gov-engine-status" :class="governanceSummary.capability.enabled ? 'enabled' : 'disabled'">{{ governanceSummary.capability.enabled ? '启用' : '停用' }}</span><span class="gov-engine-count">{{ governanceSummary.capability.denials }} 拒绝</span></div>
          </div>
        </div>
      </div>
    </div>

    <ConfirmModal :visible="cfmVisible" :title="cfmTitle" :message="cfmMsg" :type="cfmType" @confirm="doCfmAction" @cancel="cfmVisible = false" />
  </div>
</template>
<script setup>
import { ref, computed, inject, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api, apiPost } from '../api.js'
import StatCard from '../components/StatCard.vue'
import TrendChart from '../components/TrendChart.vue'
import Icon from '../components/Icon.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const cfmVisible = ref(false), cfmTitle = ref(''), cfmMsg = ref(''), cfmType = ref('danger')
let cfmAction = null
function doCfmAction() { cfmVisible.value = false; if (cfmAction) cfmAction() }
import PieChart from '../components/PieChart.vue'
import HeatMap from '../components/HeatMap.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'

const appState = inject('appState')
const showToast = inject('showToast', () => {})
const router = useRouter()

// ====== 常量 ======
const barColors = ['#6366F1', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6']
const sourceBarColors = ['#10B981', '#14B8A6', '#3B82F6', '#8B5CF6', '#F59E0B', '#EF4444']
const pieColors = ['#EF4444', '#F59E0B', '#6366F1', '#10B981', '#8B5CF6', '#06B6D4', '#EC4899', '#F97316']
const timeRangeOptions = [
  { label: '1h', value: '1h' },
  { label: '6h', value: '6h' },
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
]

// ====== SVG Icons ======
const svgGlobe = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
const svgShieldX = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><line x1="9.5" y1="9.5" x2="14.5" y2="14.5"/><line x1="14.5" y1="9.5" x2="9.5" y2="14.5"/></svg>'
const svgAlertTriangle = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgPercent = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" y1="5" x2="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/></svg>'
const svgUserDanger = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="18" y1="8" x2="23" y2="13"/><line x1="23" y1="8" x2="18" y2="13"/></svg>'
const svgTrend = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgHeart = svgTrend
const svgTarget = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg>'
const svgGrid = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg>'
const svgShieldCheck = '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>'
const svgIFC = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="M8 11l4-4 4 4"/><path d="M12 7v10"/></svg>'
const svgDeviation = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgCapDeny = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/><line x1="12" y1="15" x2="12" y2="19"/></svg>'

// ====== 响应式状态 ======
const loaded = ref(false)
const refreshing = ref(false)
const flashCards = ref(false)
const stats = ref({ total: '--', blocked: '--', warned: '--', rate: '--', totalChange: '', blockedChange: '', warnedChange: '', totalUp: true })
const prevStats = ref({ total: 0, blocked: 0, warned: 0 })
const trendData = ref([])
const trendRange = ref('24h')
const recentAttacks = ref([])
const topRules = ref([])
const sourceCategoryRows = ref([])
const pieData = ref([])
const heatmapData = ref([])
const highRiskUserCount = ref(0)
const summary = ref({})
const summaryLoaded = ref(false)
const healthScore = ref(null)
const showScoreDetail = ref(false)
const systemHealth = ref(null)
const anomalyStatus = ref(null)
const ifcStats = ref(null)
const deviationStats = ref(null)
const capabilityStats = ref(null)
const governanceSummary = ref(null)
const refreshInterval = ref(localStorage.getItem('overview_refresh') || '30000')
const timeRange = ref(localStorage.getItem('overview_time_range') || '24h')
const lastRefreshTs = ref(Date.now())
const lastRefreshDisplay = ref('刚刚')
const actionLoading = ref({ reload: false, cache: false })

// ====== 健康分动画 ======
const animatedScore = ref(0)
watch(() => healthScore.value?.score, (newVal) => {
  if (newVal == null) return
  const start = animatedScore.value
  const end = newVal
  const duration = 1000
  const startTime = performance.now()
  function tick(now) {
    const p = Math.min((now - startTime) / duration, 1)
    const eased = 1 - Math.pow(1 - p, 3)
    animatedScore.value = Math.round(start + (end - start) * eased)
    if (p < 1) requestAnimationFrame(tick)
  }
  requestAnimationFrame(tick)
}, { immediate: true })

// ====== "上次刷新"定时器 ======
const lastRefreshTime = computed(() => new Date(lastRefreshTs.value).toLocaleTimeString('zh-CN', { hour12: false }))
let refreshDisplayTimer = null
function updateRefreshDisplay() {
  const diff = Math.floor((Date.now() - lastRefreshTs.value) / 1000)
  if (diff < 5) lastRefreshDisplay.value = '刚刚'
  else if (diff < 60) lastRefreshDisplay.value = diff + '秒前'
  else lastRefreshDisplay.value = Math.floor(diff / 60) + '分钟前'
}

// ====== 健康分颜色 ======
const scoreColorMap = { excellent: '#10B981', good: '#3B82F6', warning: '#F59E0B', danger: '#EF4444', critical: '#DC2626' }
const scoreColor = computed(() => healthScore.value ? (scoreColorMap[healthScore.value.level] || '#6B7280') : '#6B7280')
const scoreColorEnd = computed(() => {
  const base = scoreColor.value
  return base === '#10B981' ? '#059669' : base === '#3B82F6' ? '#2563EB' : base === '#F59E0B' ? '#D97706' : base === '#EF4444' ? '#DC2626' : '#4B5563'
})
const scoreDash = computed(() => {
  if (!healthScore.value) return '0 327'
  const c = 2 * Math.PI * 52
  const p = healthScore.value.score / 100
  return `${c * p} ${c * (1 - p)}`
})
const trendMiniPointsArr = computed(() => {
  if (!healthScore.value?.trend?.length) return []
  const d = healthScore.value.trend, w = 200, h = 50, pad = 10
  return d.map((v, i) => ({ x: pad + (i / Math.max(d.length - 1, 1)) * (w - 2 * pad), y: pad + (1 - v.score / 100) * (h - 2 * pad) }))
})
const trendMiniPoints = computed(() => trendMiniPointsArr.value.map(p => `${p.x},${p.y}`).join(' '))

// ====== 系统指标 ======
const sysMetrics = computed(() => {
  const s = systemHealth.value
  if (!s) return []
  const r = []
  if (s.cpu_percent != null) { const p = s.cpu_percent; r.push({ label: 'CPU', pct: p, display: p.toFixed(1) + '%', color: p > 80 ? '#EF4444' : p > 60 ? '#F59E0B' : '#10B981' }) }
  if (s.memory_percent != null) { const p = s.memory_percent; r.push({ label: '内存', pct: p, display: (s.memory_used_mb || 0).toFixed(0) + ' MB', color: p > 80 ? '#EF4444' : p > 60 ? '#F59E0B' : '#10B981' }) }
  if (s.disk_used_percent != null) { const p = s.disk_used_percent; r.push({ label: '磁盘', pct: p, display: p.toFixed(1) + '%', color: p > 90 ? '#EF4444' : p > 80 ? '#F59E0B' : '#10B981' }) }
  return r
})

// ====== 安全洞察 ======
const summaryRedteamRate = computed(() => { const r = summary.value.redteam; return r ? (r.pass_rate || 0).toFixed(1) : '--' })
const summaryRedteamVulns = computed(() => { const r = summary.value.redteam; return r ? (r.failed || 0) : 0 })
const summaryRedteamClass = computed(() => { const v = parseFloat(summaryRedteamRate.value); return v >= 80 ? 'success' : v >= 50 ? 'warning' : 'danger' })
const summaryTopTenant = computed(() => { const lb = summary.value.leaderboard; return lb && lb.length ? lb[0].tenant_name || lb[0].tenant_id : '--' })
const summaryTopScore = computed(() => { const lb = summary.value.leaderboard; return lb && lb.length ? lb[0].health_score : '--' })
const sourceCategoryCount = computed(() => sourceCategoryRows.value.length)

// ====== 辅助函数 ======
function fmtTime(ts) { if (!ts) return '--'; const d = new Date(ts); return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false }) }
function fmtTimeShort(ts) { if (!ts) return '--'; const d = new Date(ts); return isNaN(d.getTime()) ? String(ts) : d.toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit' }) }


function goToSourceCategory(category) {
  router.push({ path: '/sessions', query: { source_category: category } })
}
function deductionLink(name) {
  const map = { 'IM拦截率': '/audit', 'IM 拦截率': '/audit', 'LLM异常率': '/agent', 'LLM 异常率': '/agent', 'Canary泄露': '/settings?section=canary', 'Canary 泄露': '/settings?section=canary', '高危用户': '/user-profiles', '规则命中': '/rules', '规则覆盖': '/rules', 'IFC 违规率': '/ifc', 'IFC违规率': '/ifc', 'Plan 偏差': '/deviations', 'Plan偏差': '/deviations' }
  for (const [k, v] of Object.entries(map)) { if (name.includes(k) || k.includes(name)) return v }
  return null
}

// ====== 健康状态条 ======
const healthBars = computed(() => {
  const h = appState.health
  if (!h || !h.checks) return []
  const dims = [
    { k: 'database', n: '数据库', fn: c => c.latency_ms != null ? Math.min(100, Math.max(0, 100 - c.latency_ms * 2)) : 50, vfn: c => c.latency_ms != null ? c.latency_ms.toFixed(1) + 'ms' : '--' },
    { k: 'upstream', n: '上游', fn: c => c.total > 0 ? (c.healthy / c.total * 100) : 0, vfn: c => c.healthy != null ? c.healthy + '/' + c.total : '--' },
    { k: 'disk', n: '磁盘', fn: c => c.used_percent != null ? (100 - c.used_percent) : 50, vfn: c => c.used_percent != null ? c.used_percent.toFixed(1) + '%' : '--' },
    { k: 'memory', n: '内存', fn: c => c.alloc_mb != null ? Math.max(0, 100 - c.alloc_mb / 10) : 50, vfn: c => c.alloc_mb != null ? c.alloc_mb.toFixed(1) + ' MB' : '--' },
    { k: 'goroutines', n: 'Goroutine', fn: c => c.count != null ? Math.max(0, 100 - c.count / 10) : 50, vfn: c => c.count != null ? String(c.count) : '' }
  ]
  const result = []
  for (const dm of dims) {
    const c = h.checks[dm.k]; if (!c) continue
    const pct = dm.fn(c)
    const color = c.status === 'ok' ? 'var(--color-success)' : (c.status === 'warning' ? 'var(--color-warning)' : 'var(--color-danger)')
    result.push({ name: dm.n, pct, color, val: dm.vfn(c) })
  }
  return result
})

// ====== 趋势图数据 ======
const trendChartData = computed(() => trendData.value.map(t => ({ total: (t.pass || 0) + (t.block || 0) + (t.warn || 0), block: t.block || 0, warn: t.warn || 0 })))
const trendLines = [{ key: 'total', color: '#6366F1', label: '总请求' }, { key: 'block', color: '#EF4444', label: '拦截' }, { key: 'warn', color: '#F59E0B', label: '告警' }]
const trendXLabels = computed(() => trendData.value.map(t => { const h = t.hour || ''; if (trendRange.value === '7d') return h.substring(5, 10) + ' ' + h.substring(11, 13) + ':00'; const hp = h.substring(11, 13); return hp ? hp + ':00' : '' }))

// ====== 时间范围切换 ======
function setTimeRange(val) {
  timeRange.value = val
  localStorage.setItem('overview_time_range', val)
  trendRange.value = (val === '24h' || val === '1h' || val === '6h') ? '24h' : '7d'
  refreshAllData()
}
function onTrendRangeChange(range) { trendRange.value = range; loadTrend() }

// ====== 数据加载 ======
async function loadTrend() {
  try { const d = await api('/api/v1/audit/timeline?hours=' + (trendRange.value === '7d' ? 168 : 24)); trendData.value = d.timeline || [] } catch { trendData.value = [] }
}
async function loadHealthScore() { try { healthScore.value = await api('/api/v1/health/score') } catch {} }
async function loadSystemHealth() { try { const d = await api('/healthz'); if (d.system) systemHealth.value = d.system } catch {} }
async function loadAnomalyStatus() { try { anomalyStatus.value = await api('/api/v1/anomaly/status') } catch { anomalyStatus.value = null } }
async function loadGovernanceStats() {
  try { ifcStats.value = await api('/api/v1/ifc/stats') } catch { ifcStats.value = null }
  try { deviationStats.value = await api('/api/v1/deviations/stats') } catch { deviationStats.value = null }
  try { capabilityStats.value = await api('/api/v1/capabilities/stats') } catch { capabilityStats.value = null }
  // Build governance summary from individual stats + config
  try {
    const ifcConfig = await api('/api/v1/ifc/config').catch(() => null)
    const devConfig = await api('/api/v1/deviations/config').catch(() => null)
    const capMappings = await api('/api/v1/capabilities/mappings').catch(() => null)
    governanceSummary.value = {
      ifc: { enabled: ifcConfig?.enabled ?? false, violations: ifcStats.value?.total_violations || 0, blocked: ifcStats.value?.total_blocked || 0 },
      deviations: { enabled: devConfig?.enabled ?? false, detected: deviationStats.value?.total_deviations || 0, critical: deviationStats.value?.critical_count || 0 },
      capability: { enabled: true, denials: capabilityStats.value?.deny_count || 0, evaluations: capabilityStats.value?.total_evaluations || 0 }
    }
  } catch { governanceSummary.value = null }
}

async function loadData() {
  try {
    const d = await api('/api/v1/stats?since=' + timeRange.value)
    const total = d.total || 0
    const breakdown = d.breakdown || {}
    let blocked = 0, warned = 0
    for (const k of Object.keys(breakdown)) { if (k.indexOf('block') >= 0) blocked += breakdown[k]; if (k.indexOf('warn') >= 0) warned += breakdown[k] }
    const rate = total > 0 ? (blocked / total * 100).toFixed(1) : '0.0'
    // 计算变化百分比
    const pTotal = prevStats.value.total
    const pBlocked = prevStats.value.blocked
    const pWarned = prevStats.value.warned
    const totalChange = pTotal > 0 ? (((total - pTotal) / pTotal) * 100).toFixed(1) + '%' : ''
    const blockedChange = pBlocked > 0 ? (((blocked - pBlocked) / pBlocked) * 100).toFixed(1) + '%' : ''
    const warnedChange = pWarned > 0 ? (((warned - pWarned) / pWarned) * 100).toFixed(1) + '%' : ''
    // 检测是否有新事件 -> 闪烁提示
    if (loaded.value && (total !== prevStats.value.total || blocked !== prevStats.value.blocked)) {
      flashCards.value = true
      setTimeout(() => { flashCards.value = false }, 1500)
    }
    prevStats.value = { total, blocked, warned }
    stats.value = { total, blocked, warned, rate: rate + '%', totalChange, blockedChange, warnedChange, totalUp: total >= pTotal }
  } catch {}
  await loadTrend()
  try { const d = await api('/api/v1/audit/logs?action=block&limit=5'); recentAttacks.value = d.logs || [] } catch { recentAttacks.value = [] }
  try {
    const d = await api('/api/v1/rules/hits'); let list = Array.isArray(d) ? d : (d.hits || [])
    list.sort((a, b) => (b.hits || 0) - (a.hits || 0)); const top = list.slice(0, 5)
    const maxH = top.length ? (top[0].hits || 1) : 1
    topRules.value = top.map(r => ({ ...r, pct: (r.hits / maxH) * 100 }))
    const groupMap = {}
    for (const r of list) { const g = r.group || 'other'; if (!groupMap[g]) groupMap[g] = 0; groupMap[g] += r.hits || 0 }
    const groups = Object.entries(groupMap).sort((a, b) => b[1] - a[1])
    pieData.value = groups.map(([label, value], i) => ({ label, value, color: pieColors[i % pieColors.length] }))
  } catch { topRules.value = []; pieData.value = [] }
  try {
    const d = await api('/api/v1/audit/timeline?hours=168'); const tl = d.timeline || []
    const matrix = Array.from({ length: 7 }, () => Array(24).fill(0)); const now = new Date()
    for (const t of tl) { if (!t.hour) continue; const dt = new Date(t.hour); if (isNaN(dt.getTime())) continue; const diffDays = Math.floor((now - dt) / 86400000); const dayIdx = 6 - Math.min(6, diffDays); const hourIdx = dt.getHours(); matrix[dayIdx][hourIdx] += (t.block || 0) + (t.warn || 0) }
    heatmapData.value = matrix
  } catch { heatmapData.value = [] }
  try {
    const toolStats = await api('/api/v1/llm/tools/stats')
    const bySource = Array.isArray(toolStats.by_source_category) ? toolStats.by_source_category : []
    const maxSource = bySource.length ? (bySource[0].count || 1) : 1
    sourceCategoryRows.value = bySource.map(item => ({
      category: item.category,
      count: item.count,
      pct: ((item.count || 0) / maxSource) * 100,
    }))
  } catch { sourceCategoryRows.value = [] }
  try { const rs = await api('/api/v1/users/risk-stats'); highRiskUserCount.value = (rs.critical_count || 0) + (rs.high_count || 0) } catch { highRiskUserCount.value = 0 }
  loaded.value = true
}

async function loadSummary() { try { summary.value = await api('/api/v1/overview/summary'); summaryLoaded.value = true } catch { summaryLoaded.value = false } }

async function refreshAllData() {
  refreshing.value = true
  await Promise.all([loadData(), loadHealthScore(), loadSystemHealth(), loadAnomalyStatus(), loadGovernanceStats()])
  lastRefreshTs.value = Date.now()
  updateRefreshDisplay()
  refreshing.value = false
}

async function manualRefresh() {
  if (refreshing.value) return
  await refreshAllData()
  showToast('数据已刷新', 'success')
}

// ====== 快捷操作 ======
function goReport() { router.push({ path: '/reports', query: { auto: 'daily' } }) }

async function hotReloadRules() {
  actionLoading.value.reload = true
  try {
    await apiPost('/api/v1/rules/reload')
    showToast('规则已热更新', 'success')
  } catch (e) {
    showToast('规则更新失败: ' + e.message, 'error')
  }
  actionLoading.value.reload = false
}

function clearCache() {
  cfmTitle.value = '清理缓存'; cfmMsg.value = '确认清除所有 LLM 响应缓存？此操作不可恢复。'; cfmType.value = 'warning'
  cfmAction = async () => {
    actionLoading.value.cache = true
    try { await api('/api/v1/cache/entries', { method: 'DELETE' }); showToast('缓存已清理', 'success') }
    catch (e) { showToast('缓存清理失败: ' + e.message, 'error') }
    actionLoading.value.cache = false
  }; cfmVisible.value = true
}

// ====== 自动刷新 ======
function onRefreshChange() { localStorage.setItem('overview_refresh', refreshInterval.value); setupTimer() }
let refreshTimer = null
function setupTimer() {
  clearInterval(refreshTimer)
  const ms = parseInt(refreshInterval.value)
  if (ms > 0) refreshTimer = setInterval(() => { refreshAllData() }, ms)
}

onMounted(() => {
  trendRange.value = (timeRange.value === '24h' || timeRange.value === '1h' || timeRange.value === '6h') ? '24h' : '7d'
  refreshAllData()
  loadSummary()
  setupTimer()
  refreshDisplayTimer = setInterval(updateRefreshDisplay, 5000)
})
onUnmounted(() => { clearInterval(refreshTimer); clearInterval(refreshDisplayTimer) })
</script>
<style scoped>
/* ====== 顶部工具栏 ====== */
.overview-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--space-4); padding: var(--space-3) var(--space-4); background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); }
.toolbar-left { display: flex; align-items: center; gap: var(--space-2); color: var(--text-primary); }
.toolbar-title { font-size: var(--text-base); font-weight: 700; }
.last-refresh { display: flex; align-items: center; gap: 4px; font-size: var(--text-xs); color: var(--text-tertiary); margin-left: var(--space-3); padding: 2px 8px; background: rgba(99,102,241,0.08); border-radius: 9999px; }
.toolbar-right { display: flex; align-items: center; gap: var(--space-2); }
.time-range-group { display: flex; background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); overflow: hidden; }
.tr-btn { background: transparent; border: none; color: var(--text-tertiary); font-size: var(--text-xs); font-weight: 600; padding: 5px 12px; cursor: pointer; transition: all .2s; }
.tr-btn:hover { color: var(--text-primary); background: rgba(99,102,241,0.06); }
.tr-btn.active { color: #fff; background: #6366F1; }
.toolbar-btn { display: flex; align-items: center; justify-content: center; width: 32px; height: 32px; border-radius: var(--radius-md); border: 1px solid var(--border-subtle); background: var(--bg-elevated); color: var(--text-secondary); cursor: pointer; transition: all .2s; }
.toolbar-btn:hover { border-color: #6366F1; color: #6366F1; }
.refresh-btn.spinning svg { animation: spin-refresh .8s linear infinite; }
@keyframes spin-refresh { from { transform: rotate(0deg) } to { transform: rotate(360deg) } }
.refresh-interval-wrap { position: relative; }
.refresh-select { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); color: var(--text-primary); font-size: var(--text-xs); padding: 5px 8px; cursor: pointer; }

/* ====== 闪烁动画 ====== */
.stat-flash { animation: card-flash 1.5s ease; }
@keyframes card-flash { 0%, 100% { box-shadow: none } 25% { box-shadow: 0 0 16px rgba(99,102,241,0.4) } 50% { box-shadow: 0 0 8px rgba(99,102,241,0.2) } 75% { box-shadow: 0 0 16px rgba(99,102,241,0.3) } }

/* ====== StatCard 可点击样式 ====== */
.stat-clickable { cursor: pointer !important; }
.stat-clickable:hover { transform: translateY(-3px) !important; box-shadow: var(--shadow-lg) !important; border-color: #6366F1 !important; }

/* ====== 快捷操作 + 最近告警行 ====== */
.quick-row { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); margin-bottom: var(--space-5); }
.qa-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-3); }
.qa-btn { display: flex; align-items: center; gap: var(--space-3); background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: var(--space-3); cursor: pointer; transition: all .2s; text-align: left; }
.qa-btn:hover { border-color: #6366F1; background: rgba(99,102,241,0.04); transform: translateY(-1px); }
.qa-btn:disabled { opacity: 0.5; cursor: not-allowed; transform: none; }
.qa-icon-wrap { display: flex; align-items: center; justify-content: center; width: 36px; height: 36px; border-radius: var(--radius-md); flex-shrink: 0; }
.qa-reload { background: rgba(99,102,241,0.1); color: #6366F1; }
.qa-cache { background: rgba(239,68,68,0.1); color: #EF4444; }
.qa-report { background: rgba(16,185,129,0.1); color: #10B981; }
.qa-rules { background: rgba(245,158,11,0.1); color: #F59E0B; }
.qa-label { font-size: var(--text-sm); font-weight: 600; color: var(--text-primary); }
.qa-desc { font-size: 10px; color: var(--text-tertiary); margin-top: 2px; }

/* ====== 告警列表 ====== */
.card-more { margin-left: auto; font-size: var(--text-xs); color: #6366F1; text-decoration: none; font-weight: 500; }
.card-more:hover { text-decoration: underline; }
.alert-list { max-height: 240px; overflow-y: auto; }
.alert-item { display: flex; align-items: center; gap: var(--space-3); padding: 8px 0; border-bottom: 1px solid var(--border-subtle); cursor: pointer; transition: background .15s; }
.alert-item:hover { background: rgba(99,102,241,0.04); }
.alert-item:last-child { border-bottom: none; }
.alert-time { font-size: var(--text-xs); color: var(--text-tertiary); font-family: var(--font-mono); min-width: 50px; }
.alert-body { flex: 1; display: flex; align-items: center; gap: var(--space-2); min-width: 0; }
.alert-dir { font-size: 10px; padding: 1px 6px; border-radius: 9999px; font-weight: 600; flex-shrink: 0; }
.dir-in { background: rgba(239,68,68,0.12); color: #EF4444; }
.dir-out { background: rgba(59,130,246,0.12); color: #3B82F6; }
.alert-reason { font-size: var(--text-xs); color: var(--text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.alert-sender { font-size: var(--text-xs); color: var(--text-tertiary); flex-shrink: 0; }
.user-link { color: #6366F1; cursor: pointer; text-decoration: none; font-weight: 500; }
.user-link:hover { text-decoration: underline; }

/* ====== 驾驶舱 ====== */
.cockpit-section { background: var(--bg-surface); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); padding: var(--space-4); margin-bottom: 20px; }
.cockpit-body { display: flex; gap: var(--space-4); align-items: flex-start; }
.cockpit-left { flex-shrink: 0; width: 140px; cursor: pointer; text-align: center; }
.cockpit-center { flex: 1; min-width: 0; }
.cockpit-right { flex-shrink: 0; width: 180px; background: var(--bg-elevated); border-radius: var(--radius-md); padding: var(--space-3); }
.score-ring-wrap { position: relative; width: 130px; height: 130px; margin: 0 auto; }
.score-ring-svg { width: 100%; height: 100%; }
.score-ring-progress { transition: stroke-dasharray .8s ease; }
.score-center { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); text-align: center; }
.score-number { font-size: 2rem; font-weight: 800; line-height: 1; font-family: var(--font-mono); }
.score-label { font-size: var(--text-xs); font-weight: 600; margin-top: 2px; }
.score-expand-hint { font-size: 10px; color: var(--text-disabled); margin-top: 4px; transition: color .2s; }
.cockpit-left:hover .score-expand-hint { color: #6366F1; }
.score-desc { display: flex; align-items: center; gap: var(--space-2); margin-bottom: var(--space-2); }
.score-level-badge { display: inline-block; padding: 2px 10px; border-radius: 9999px; font-size: var(--text-xs); font-weight: 700; color: #fff; }
.badge-excellent { background: #10B981; } .badge-good { background: #3B82F6; } .badge-warning { background: #F59E0B; } .badge-danger { background: #EF4444; } .badge-critical { background: #DC2626; }
.score-text { font-size: var(--text-sm); color: var(--text-secondary); }
.time-badge { display: inline-block; padding: 1px 6px; border-radius: 9999px; font-size: 10px; font-weight: 600; color: var(--text-tertiary); background: rgba(107,114,128,0.15); margin-left: 4px; vertical-align: middle; line-height: 1.4; }

/* ====== 展开动画 ====== */
.slide-down-enter-active, .slide-down-leave-active { transition: all .3s ease; overflow: hidden; }
.slide-down-enter-from, .slide-down-leave-to { opacity: 0; max-height: 0; }
.slide-down-enter-to, .slide-down-leave-from { opacity: 1; max-height: 300px; }

/* ====== 扣分详情 ====== */
.deduction-list { margin-top: var(--space-2); }
.deduction-item { display: flex; align-items: center; gap: var(--space-2); padding: 4px 0; font-size: var(--text-xs); border-bottom: 1px solid var(--border-subtle); }
.deduction-name { font-weight: 600; color: var(--text-primary); min-width: 80px; }
.deduction-points { color: #EF4444; font-weight: 700; font-family: var(--font-mono); min-width: 30px; }
.deduction-detail { color: var(--text-tertiary); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.deduction-jump { color: #6366F1; text-decoration: none; font-weight: 700; font-size: var(--text-sm); flex-shrink: 0; padding: 0 4px; opacity: 0.7; transition: opacity .2s; }
.deduction-jump:hover { opacity: 1; }
.deduction-empty { font-size: var(--text-xs); color: var(--text-tertiary); padding: var(--space-2) 0; }

/* ====== 异常指示器 ====== */
.anomaly-indicator { margin-top: var(--space-2); padding: 4px 0; }
.anomaly-link { color: #EF4444; font-size: var(--text-xs); font-weight: 700; text-decoration: none; cursor: pointer; display: inline-flex; align-items: center; gap: 4px; padding: 3px 10px; background: rgba(239,68,68,0.1); border-radius: 9999px; transition: all .2s; }
.anomaly-link:hover { background: rgba(239,68,68,0.2); }
.anomaly-learning { color: var(--text-tertiary); font-size: var(--text-xs); }
.anomaly-ok { color: var(--text-tertiary); font-size: var(--text-xs); text-decoration: none; }
.anomaly-ok:hover { color: var(--text-secondary); }

/* ====== 趋势迷你图 ====== */
.trend-mini { margin-top: var(--space-2); }
.trend-mini-label { font-size: 10px; color: var(--text-tertiary); margin-bottom: 2px; }
.trend-mini-svg { width: 100%; height: 50px; }
.trend-mini-dates { display: flex; justify-content: space-between; font-size: 9px; color: var(--text-disabled); }

/* ====== 系统状态 ====== */
.sys-health-title { font-size: var(--text-xs); font-weight: 700; color: var(--text-primary); margin-bottom: var(--space-2); }
.sys-metric { margin-bottom: var(--space-2); }
.sys-metric-header { display: flex; justify-content: space-between; font-size: 10px; margin-bottom: 2px; }
.sys-metric-label { color: var(--text-secondary); }
.sys-metric-value { font-weight: 700; font-family: var(--font-mono); }
.sys-metric-bar { height: 6px; background: rgba(255,255,255,0.06); border-radius: 3px; overflow: hidden; }
.sys-metric-fill { height: 100%; border-radius: 3px; transition: width .6s ease; }
.sys-goroutines { font-size: 10px; color: var(--text-tertiary); display: flex; align-items: center; gap: 4px; margin-top: var(--space-2); }

/* ====== 横向条形图 ====== */
.hbar-rank { width: 24px; font-size: var(--text-xs); color: #6366F1; font-weight: 700; text-align: center; flex-shrink: 0; }
.hbar-fill-anim { width: 0; animation: hbar-grow .8s ease-out forwards; }
@keyframes hbar-grow { from { width: 0 } to { width: var(--target-w) } }

/* ====== 列表动画 ====== */
.list-anim-enter-active { animation: list-in .2s ease-out; } .list-anim-leave-active { animation: list-out .2s ease-in; } .list-anim-move { transition: transform .2s ease; }
@keyframes list-in { from { opacity: 0; transform: translateY(-10px) } to { opacity: 1; transform: translateY(0) } }
@keyframes list-out { from { opacity: 1; transform: translateY(0) } to { opacity: 0; transform: translateY(10px) } }

/* ====== 安全洞察 ====== */
.insight-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
.insight-card { background: var(--bg-elevated); border: 1px solid var(--border-subtle); border-radius: var(--radius-md); padding: 16px; cursor: pointer; transition: all var(--transition-fast); }
.insight-card:hover { border-color: #6366F1; box-shadow: 0 0 12px rgba(99,102,241,0.15); }
.insight-header { font-size: 12px; color: var(--text-tertiary); margin-bottom: 8px; }
.insight-value { font-size: 28px; font-weight: 700; color: var(--text-primary); }
.insight-sub { font-size: 11px; color: var(--text-tertiary); margin-top: 4px; }
.insight-value.danger { color: var(--color-danger); } .insight-value.warning { color: var(--color-warning); } .insight-value.success { color: var(--color-success); }

/* ====== 治理引擎洞察卡片 ====== */
.governance-insight { grid-column: span 1; }
.governance-engines { display: flex; flex-direction: column; gap: 6px; margin-top: 4px; }
.gov-engine-row { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.gov-engine-name { font-weight: 600; color: var(--text-primary); min-width: 60px; }
.gov-engine-status { display: inline-block; padding: 1px 6px; border-radius: 9999px; font-size: 10px; font-weight: 600; }
.gov-engine-status.enabled { background: rgba(16,185,129,0.15); color: #10B981; }
.gov-engine-status.disabled { background: rgba(107,114,128,0.15); color: var(--text-tertiary); }
.gov-engine-count { color: var(--text-tertiary); margin-left: auto; font-family: var(--font-mono); font-size: 11px; }

/* ====== 响应式 ====== */
@media (max-width: 1024px) {
  .quick-row { grid-template-columns: 1fr; }
  .insight-grid { grid-template-columns: repeat(2, 1fr); }
}
@media (max-width: 768px) {
  .cockpit-body { flex-direction: column; }
  .cockpit-left, .cockpit-right { width: 100%; }
  .overview-toolbar { flex-direction: column; gap: var(--space-2); }
  .toolbar-right { width: 100%; justify-content: flex-end; flex-wrap: wrap; }
  .qa-grid { grid-template-columns: 1fr; }
  .insight-grid { grid-template-columns: 1fr; }
}
</style>
