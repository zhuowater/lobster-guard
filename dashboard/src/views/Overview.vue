<template>
  <div>
    <!-- 安全驾驶舱 (v11.1) -->
    <div class="cockpit-section" v-if="healthScore">
      <div class="cockpit-header">
        <div class="cockpit-title">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
          安全驾驶舱
        </div>
        <div class="cockpit-controls">
          <button class="gen-report-btn" @click="goReport" title="生成安全报告"><Icon name="bar-chart" :size="14" /> 生成报告</button>
          <select v-model="timeRange" @change="onTimeRangeChange" class="time-range-select" title="数据时间范围">
            <option value="24h">⏱ 24小时</option>
            <option value="7d">⏱ 7天</option>
            <option value="30d">⏱ 30天</option>
          </select>
          <div class="refresh-control">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
            <select v-model="refreshInterval" @change="onRefreshChange" class="refresh-select">
              <option value="30000">30s</option><option value="60000">1m</option><option value="300000">5m</option><option value="0">手动</option>
            </select>
          </div>
        </div>
      </div>
      <div class="cockpit-body">
        <div class="cockpit-left" @click="showScoreDetail=!showScoreDetail" style="cursor:pointer">
          <div class="score-ring-wrap">
            <svg viewBox="0 0 120 120" class="score-ring-svg">
              <circle cx="60" cy="60" r="52" fill="none" :stroke="scoreColor" stroke-width="10" opacity="0.15"/>
              <circle cx="60" cy="60" r="52" fill="none" :stroke="scoreColor" stroke-width="10" stroke-linecap="round" :stroke-dasharray="scoreDash" stroke-dashoffset="0" transform="rotate(-90 60 60)" class="score-ring-progress"/>
            </svg>
            <div class="score-center">
              <div class="score-number" :style="{color:scoreColor}">{{healthScore.score}}</div>
              <div class="score-label" :style="{color:scoreColor}">{{healthScore.level_label}}</div>
            </div>
          </div>
        </div>
        <div class="cockpit-center">
          <div class="score-desc">
            <span class="score-level-badge" :class="'badge-'+healthScore.level">{{healthScore.level_label}}</span>
            <span class="score-text">安全健康分 {{healthScore.score}}/100</span>
            <span class="time-badge" title="健康分固定使用7天评估窗口">7d</span>
          </div>
          <div v-if="showScoreDetail && healthScore.deductions && healthScore.deductions.length" class="deduction-list">
            <div v-for="d in healthScore.deductions" :key="d.name" class="deduction-item">
              <span class="deduction-name">{{d.name}}</span><span class="deduction-points">-{{d.points}}</span><span class="deduction-detail">{{d.detail}}</span>
              <router-link v-if="deductionLink(d.name)" :to="deductionLink(d.name)" class="deduction-jump" title="查看详情" @click.stop>→</router-link>
            </div>
          </div>
          <div v-else-if="showScoreDetail" class="deduction-empty">✅ 未发现安全风险</div>
          <!-- v11.2 异常检测指示器 -->
          <div class="anomaly-indicator" v-if="anomalyStatus">
            <router-link to="/anomaly" class="anomaly-link" v-if="anomalyStatus.alerts_24h > 0">
              ⚠️ 检测到 {{ anomalyStatus.alerts_24h }} 个异常
            </router-link>
            <span class="anomaly-learning" v-else-if="anomalyStatus.baselines_ready < anomalyStatus.metrics_count">
              <Icon name="bar-chart" :size="14" /> 基线学习中 ({{ anomalyStatus.baselines_ready }}/{{ anomalyStatus.metrics_count }} 就绪)
            </span>
            <router-link to="/anomaly" class="anomaly-ok" v-else>
              ✅ 异常检测正常 ({{ anomalyStatus.metrics_count }} 指标)
            </router-link>
          </div>
          <div class="trend-mini" v-if="healthScore.trend && healthScore.trend.length">
            <div class="trend-mini-label">7天趋势</div>
            <svg :viewBox="'0 0 200 50'" class="trend-mini-svg">
              <polyline :points="trendMiniPoints" fill="none" :stroke="scoreColor" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>
              <circle v-for="(p,i) in trendMiniPointsArr" :key="i" :cx="p.x" :cy="p.y" r="3" :fill="scoreColor"/>
            </svg>
            <div class="trend-mini-dates"><span v-for="t in healthScore.trend" :key="t.date">{{t.date.substring(5)}}</span></div>
          </div>
        </div>
        <div class="cockpit-right" v-if="systemHealth">
          <div class="sys-health-title">系统状态</div>
          <div class="sys-metric" v-for="m in sysMetrics" :key="m.label">
            <div class="sys-metric-header"><span class="sys-metric-label">{{m.label}}</span><span class="sys-metric-value" :style="{color:m.color}">{{m.display}}</span></div>
            <div class="sys-metric-bar"><div class="sys-metric-fill" :style="{width:Math.min(m.pct,100)+'%',background:m.color}"></div></div>
          </div>
          <div class="sys-goroutines"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg> Goroutines: {{systemHealth.goroutines}}</div>
        </div>
      </div>
    </div>
    <!-- Stat Cards -->
    <div class="ov-cards" v-if="loaded">
      <StatCard :iconSvg="svgGlobe" :value="stats.total" :label="'总请求'" :badge="timeRange" color="blue" class="stat-clickable" @click="router.push({ path: '/audit', query: { since: timeRange } })"/>
      <StatCard :iconSvg="svgShieldX" :value="stats.blocked" :label="'拦截数'" :badge="timeRange" color="red" class="stat-clickable" @click="router.push({ path: '/audit', query: { since: timeRange } })"/>
      <StatCard :iconSvg="svgAlertTriangle" :value="stats.warned" :label="'告警数'" :badge="timeRange" color="yellow" class="stat-clickable" @click="router.push({ path: '/audit', query: { since: timeRange } })"/>
      <StatCard :iconSvg="svgPercent" :value="stats.rate" :label="'拦截率'" :badge="timeRange" color="green" class="stat-clickable" @click="router.push({ path: '/rules', query: { since: timeRange } })"/>
      <StatCard :iconSvg="svgUserDanger" :value="highRiskUserCount" label="高危用户" badge="30d" color="red" class="stat-clickable" @click="router.push('/user-profiles')"/>
    </div>
    <div class="ov-cards" v-else><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>
    <!-- Trend + Health -->
    <div class="ov-row">
      <div class="card"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span><span class="card-title">请求趋势</span></div>
        <Skeleton v-if="!loaded" type="chart"/><EmptyState v-else-if="!trendData.length" :iconSvg="svgTrend" title="暂无趋势数据" description="系统运行后将自动收集趋势数据"/>
        <TrendChart v-else :data="trendChartData" :lines="trendLines" :xLabels="trendXLabels" :height="170" :timeRanges="[{label:'24h',value:'24h'},{label:'7d',value:'7d'}]" :currentRange="trendRange" @rangeChange="onTrendRangeChange"/></div>
      <div class="card"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span><span class="card-title">健康状态</span></div>
        <Skeleton v-if="!loaded" type="text"/><EmptyState v-else-if="!healthBars.length" :iconSvg="svgHeart" title="无健康数据" description="等待系统上报健康信息"/>
        <div v-else><div class="hb-row" v-for="hb in healthBars" :key="hb.name"><span class="hb-label">{{hb.name}}</span><div class="hb-track"><div class="hb-fill" :style="{width:Math.max(5,hb.pct)+'%',background:hb.color}"></div></div><span class="hb-val" :style="{color:hb.color}">{{hb.val}}</span></div></div></div>
    </div>
    <!-- Pie + Top Rules -->
    <div class="ov-row">
      <div class="card"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.21 15.89A10 10 0 1 1 8 2.83"/><path d="M22 12A10 10 0 0 0 12 2v10z"/></svg></span><span class="card-title">拦截类型分布</span></div>
        <Skeleton v-if="!loaded" type="chart"/><PieChart v-else :data="pieData" :size="180"/></div>
      <div class="card"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg></span><span class="card-title">规则命中 TOP5</span></div>
        <Skeleton v-if="!loaded" type="text"/><EmptyState v-else-if="!topRules.length" :iconSvg="svgTarget" title="规则正在保护中" description="命中数据将在检测到威胁后显示"/>
        <div v-else><TransitionGroup name="list-anim" tag="div"><div class="hbar-row" v-for="(r,i) in topRules" :key="r.name"><span class="hbar-rank">#{{i+1}}</span><span class="hbar-name" :title="r.name">{{r.name}}</span><div class="hbar-track"><div class="hbar-fill hbar-fill-anim" :style="{'--target-w':Math.max(5,r.pct)+'%',background:barColors[i%barColors.length]}">{{r.hits}}</div></div></div></TransitionGroup></div></div>
    </div>
    <!-- Heatmap -->
    <div class="card" style="margin-bottom:20px"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg></span><span class="card-title">7 天攻击频率热力图</span></div>
      <Skeleton v-if="!loaded" type="chart"/><EmptyState v-else-if="!heatmapData.length" :iconSvg="svgGrid" title="暂无热力图数据" description="系统运行 24 小时后将生成攻击频率热力图"/><HeatMap v-else :data="heatmapData" title=""/></div>
    <!-- 安全洞察（v14-v17 功能摘要） -->
    <div class="card" style="margin-bottom:20px" v-if="summaryLoaded">
      <div class="card-header"><span class="card-icon">🔍</span><span class="card-title">安全洞察</span></div>
      <div class="insight-grid">
        <div class="insight-card" @click="router.push('/redteam')">
          <div class="insight-header">🎯 红队测试</div>
          <div class="insight-value" :class="summaryRedteamClass">{{ summaryRedteamRate }}%</div>
          <div class="insight-sub">检测率 · {{ summaryRedteamVulns }} 个漏洞</div>
        </div>
        <div class="insight-card" @click="router.push('/honeypot')">
          <div class="insight-header">🍯 蜜罐</div>
          <div class="insight-value">{{ summary.honeypot?.total_triggers || 0 }}</div>
          <div class="insight-sub">触发 · {{ summary.honeypot?.total_detonated || 0 }} 引爆</div>
        </div>
        <div class="insight-card" @click="router.push('/attack-chains')">
          <div class="insight-header">🔗 攻击链</div>
          <div class="insight-value" :class="(summary.attack_chains?.critical_chains||0) > 0 ? 'danger' : ''">{{ summary.attack_chains?.active_chains || 0 }}</div>
          <div class="insight-sub">活跃链 · {{ summary.attack_chains?.critical_chains || 0 }} 高危</div>
        </div>
        <div class="insight-card" @click="router.push('/leaderboard')">
          <div class="insight-header">🏆 排行榜</div>
          <div class="insight-value">{{ summaryTopTenant }}</div>
          <div class="insight-sub">{{ summaryTopScore }} 分 · TOP1</div>
        </div>
        <div class="insight-card" @click="router.push('/behavior')">
          <div class="insight-header">🧠 行为画像</div>
          <div class="insight-value" :class="(summary.behavior?.high_risk||0) > 0 ? 'warning' : ''">{{ summary.behavior?.anomaly_count || 0 }}</div>
          <div class="insight-sub">行为突变 · {{ summary.behavior?.high_risk || 0 }} 高风险</div>
        </div>
        <div class="insight-card" @click="router.push('/ab-testing')">
          <div class="insight-header">🔬 A/B 测试</div>
          <div class="insight-value">{{ summary.ab_testing?.active_tests || 0 }}</div>
          <div class="insight-sub">进行中 · {{ summary.ab_testing?.total_tests || 0 }} 总计</div>
        </div>
      </div>
    </div>

    <!-- Recent Attacks -->
    <div class="card"><div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg></span><span class="card-title">最近攻击事件</span></div>
      <Skeleton v-if="!loaded" type="table"/><EmptyState v-else-if="!recentAttacks.length" :iconSvg="svgShieldCheck" title="当前环境安全" description="没有检测到攻击事件"/>
      <div v-else class="table-wrap"><table><thead><tr><th>时间</th><th>方向</th><th>发送者</th><th>原因</th></tr></thead>
        <TransitionGroup name="list-anim" tag="tbody"><tr v-for="a in recentAttacks" :key="a.id||a.trace_id||a.timestamp" class="row-block" style="cursor:pointer" @click="$router.push('/audit')"><td>{{fmtTime(a.timestamp||a.time)}}</td><td>{{a.direction==='inbound'?'入站':'出站'}}</td><td><a v-if="a.sender_id" class="user-link" @click.stop="$router.push('/user-profiles/'+encodeURIComponent(a.sender_id))">{{a.sender_id}}</a><span v-else>--</span></td><td>{{a.reason||'--'}}</td></tr></TransitionGroup></table></div></div>
  </div>
</template>
<script setup>
import { ref, computed, inject, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import StatCard from '../components/StatCard.vue'
import TrendChart from '../components/TrendChart.vue'
import Icon from '../components/Icon.vue'
import PieChart from '../components/PieChart.vue'
import HeatMap from '../components/HeatMap.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'
const appState = inject('appState'), router = useRouter()
const barColors = ['#3B82F6','#10B981','#F59E0B','#EF4444','#8B5CF6']
const pieColors = ['#EF4444','#F59E0B','#3B82F6','#10B981','#8B5CF6','#06B6D4','#EC4899','#F97316']
const svgGlobe='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
const svgShieldX='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><line x1="9.5" y1="9.5" x2="14.5" y2="14.5"/><line x1="14.5" y1="9.5" x2="9.5" y2="14.5"/></svg>'
const svgAlertTriangle='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgPercent='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="19" y1="5" x2="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/></svg>'
const svgUserDanger='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="18" y1="8" x2="23" y2="13"/><line x1="23" y1="8" x2="18" y2="13"/></svg>'
const svgTrend='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>'
const svgHeart=svgTrend
const svgTarget='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/></svg>'
const svgGrid='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg>'
const svgShieldCheck='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>'
const loaded=ref(false),stats=ref({total:'--',blocked:'--',warned:'--',rate:'--'}),trendData=ref([]),trendRange=ref('24h'),recentAttacks=ref([]),topRules=ref([]),pieData=ref([]),heatmapData=ref([]),highRiskUserCount=ref(0)
// v18: 安全洞察摘要
const summary=ref({}),summaryLoaded=ref(false)
const summaryRedteamRate=computed(()=>{const r=summary.value.redteam;return r?(r.pass_rate||0).toFixed(1):'--'})
const summaryRedteamVulns=computed(()=>{const r=summary.value.redteam;return r?(r.failed||0):0})
const summaryRedteamClass=computed(()=>{const v=parseFloat(summaryRedteamRate.value);return v>=80?'success':v>=50?'warning':'danger'})
const summaryTopTenant=computed(()=>{const lb=summary.value.leaderboard;return lb&&lb.length?lb[0].tenant_name||lb[0].tenant_id:'--'})
const summaryTopScore=computed(()=>{const lb=summary.value.leaderboard;return lb&&lb.length?lb[0].health_score:'--'})
const healthScore=ref(null),showScoreDetail=ref(false),systemHealth=ref(null),refreshInterval=ref(localStorage.getItem('overview_refresh')||'30000'),anomalyStatus=ref(null)
// v11.4: 全局时间范围选择器
const timeRange=ref(localStorage.getItem('overview_time_range')||'24h')
const scoreColorMap={excellent:'#10B981',good:'#3B82F6',warning:'#F59E0B',danger:'#EF4444',critical:'#DC2626'}
const scoreColor=computed(()=>healthScore.value?(scoreColorMap[healthScore.value.level]||'#6B7280'):'#6B7280')
const scoreDash=computed(()=>{if(!healthScore.value)return'0 327';const c=2*Math.PI*52,p=healthScore.value.score/100;return`${c*p} ${c*(1-p)}`})
const trendMiniPointsArr=computed(()=>{if(!healthScore.value?.trend?.length)return[];const d=healthScore.value.trend,w=200,h=50,pad=10;return d.map((v,i)=>({x:pad+(i/Math.max(d.length-1,1))*(w-2*pad),y:pad+(1-v.score/100)*(h-2*pad)}))})
const trendMiniPoints=computed(()=>trendMiniPointsArr.value.map(p=>`${p.x},${p.y}`).join(' '))
const sysMetrics=computed(()=>{const s=systemHealth.value;if(!s)return[];const r=[];if(s.cpu_percent!=null){const p=s.cpu_percent;r.push({label:'CPU',pct:p,display:p.toFixed(1)+'%',color:p>80?'#EF4444':p>60?'#F59E0B':'#10B981'})}if(s.memory_percent!=null){const p=s.memory_percent;r.push({label:'内存',pct:p,display:(s.memory_used_mb||0).toFixed(0)+' MB',color:p>80?'#EF4444':p>60?'#F59E0B':'#10B981'})}if(s.disk_used_percent!=null){const p=s.disk_used_percent;r.push({label:'磁盘',pct:p,display:p.toFixed(1)+'%',color:p>90?'#EF4444':p>80?'#F59E0B':'#10B981'})}return r})
function fmtTime(ts){if(!ts)return'--';const d=new Date(ts);return isNaN(d.getTime())?String(ts):d.toLocaleString('zh-CN',{hour12:false})}

// v11.3: 分项跳转链接
function deductionLink(name) {
  const map = {
    'IM拦截率': '/audit',
    'IM 拦截率': '/audit',
    'LLM异常率': '/agent',
    'LLM 异常率': '/agent',
    'Canary泄露': '/settings?section=canary',
    'Canary 泄露': '/settings?section=canary',
    '高危用户': '/user-profiles',
    '规则命中': '/rules',
    '规则覆盖': '/rules',
  }
  // Fuzzy match: check if any key is a substring of name
  for (const [k, v] of Object.entries(map)) {
    if (name.includes(k) || k.includes(name)) return v
  }
  return null
}
const healthBars=computed(()=>{const h=appState.health;if(!h||!h.checks)return[];const dims=[{k:'database',n:'数据库',fn:c=>c.latency_ms!=null?Math.min(100,Math.max(0,100-c.latency_ms*2)):50,vfn:c=>c.latency_ms!=null?c.latency_ms.toFixed(1)+'ms':'--'},{k:'upstream',n:'上游',fn:c=>c.total>0?(c.healthy/c.total*100):0,vfn:c=>c.healthy!=null?c.healthy+'/'+c.total:'--'},{k:'disk',n:'磁盘',fn:c=>c.used_percent!=null?(100-c.used_percent):50,vfn:c=>c.used_percent!=null?c.used_percent.toFixed(1)+'%':'--'},{k:'memory',n:'内存',fn:c=>c.alloc_mb!=null?Math.max(0,100-c.alloc_mb/10):50,vfn:c=>c.alloc_mb!=null?c.alloc_mb.toFixed(1)+' MB':'--'},{k:'goroutines',n:'Goroutine',fn:c=>c.count!=null?Math.max(0,100-c.count/10):50,vfn:c=>c.count!=null?String(c.count):''}];const result=[];for(const dm of dims){const c=h.checks[dm.k];if(!c)continue;const pct=dm.fn(c);const color=c.status==='ok'?'var(--color-success)':(c.status==='warning'?'var(--color-warning)':'var(--color-danger)');result.push({name:dm.n,pct,color,val:dm.vfn(c)})}return result})
const trendChartData=computed(()=>trendData.value.map(t=>({total:(t.pass||0)+(t.block||0)+(t.warn||0),block:t.block||0,warn:t.warn||0})))
const trendLines=[{key:'total',color:'#3B82F6',label:'总请求'},{key:'block',color:'#EF4444',label:'拦截'},{key:'warn',color:'#F59E0B',label:'告警'}]
const trendXLabels=computed(()=>trendData.value.map(t=>{const h=t.hour||'';if(trendRange.value==='7d')return h.substring(5,10)+' '+h.substring(11,13)+':00';const hp=h.substring(11,13);return hp?hp+':00':''}))
function goReport(){router.push({path:'/reports',query:{auto:'daily'}})}
function onTimeRangeChange(){localStorage.setItem('overview_time_range',timeRange.value);trendRange.value=timeRange.value==='24h'?'24h':'7d';loadData();loadHealthScore();loadSystemHealth();loadAnomalyStatus()}
function onTrendRangeChange(range){trendRange.value=range;loadTrend()}
async function loadTrend(){try{const d=await api('/api/v1/audit/timeline?hours='+(trendRange.value==='7d'?168:24));trendData.value=d.timeline||[]}catch{trendData.value=[]}}
async function loadHealthScore(){try{healthScore.value=await api('/api/v1/health/score')}catch{}}
async function loadSystemHealth(){try{const d=await api('/healthz');if(d.system)systemHealth.value=d.system}catch{}}
async function loadAnomalyStatus(){try{anomalyStatus.value=await api('/api/v1/anomaly/status')}catch{anomalyStatus.value=null}}
async function loadData(){
  try{const d=await api(`/api/v1/stats?since=${timeRange.value}`);const total=d.total||0;const breakdown=d.breakdown||{};let blocked=0,warned=0;for(const k of Object.keys(breakdown)){if(k.indexOf('block')>=0)blocked+=breakdown[k];if(k.indexOf('warn')>=0)warned+=breakdown[k]};const rate=total>0?(blocked/total*100).toFixed(1):'0.0';stats.value={total,blocked,warned,rate:rate+'%'}}catch{}
  await loadTrend()
  try{const d=await api('/api/v1/audit/logs?action=block&limit=5');recentAttacks.value=d.logs||[]}catch{recentAttacks.value=[]}
  try{const d=await api('/api/v1/rules/hits');let list=Array.isArray(d)?d:(d.hits||[]);list.sort((a,b)=>(b.hits||0)-(a.hits||0));const top=list.slice(0,5);const maxH=top.length?(top[0].hits||1):1;topRules.value=top.map(r=>({...r,pct:(r.hits/maxH)*100}));const groupMap={};for(const r of list){const g=r.group||'other';if(!groupMap[g])groupMap[g]=0;groupMap[g]+=r.hits||0};const groups=Object.entries(groupMap).sort((a,b)=>b[1]-a[1]);pieData.value=groups.map(([label,value],i)=>({label,value,color:pieColors[i%pieColors.length]}))}catch{topRules.value=[];pieData.value=[]}
  try{const d=await api('/api/v1/audit/timeline?hours=168');const tl=d.timeline||[];const matrix=Array.from({length:7},()=>Array(24).fill(0));const now=new Date();for(const t of tl){if(!t.hour)continue;const dt=new Date(t.hour);if(isNaN(dt.getTime()))continue;const diffDays=Math.floor((now-dt)/86400000);const dayIdx=6-Math.min(6,diffDays);const hourIdx=dt.getHours();matrix[dayIdx][hourIdx]+=(t.block||0)+(t.warn||0)};heatmapData.value=matrix}catch{heatmapData.value=[]}
  try{const rs=await api('/api/v1/users/risk-stats');highRiskUserCount.value=(rs.critical_count||0)+(rs.high_count||0)}catch{highRiskUserCount.value=0}
  loaded.value=true
}
function onRefreshChange(){localStorage.setItem('overview_refresh',refreshInterval.value);setupTimer()}
let refreshTimer=null
function setupTimer(){clearInterval(refreshTimer);const ms=parseInt(refreshInterval.value);if(ms>0)refreshTimer=setInterval(()=>{loadData();loadHealthScore();loadSystemHealth();loadAnomalyStatus()},ms)}
async function loadSummary(){try{summary.value=await api('/api/v1/overview/summary');summaryLoaded.value=true}catch{summaryLoaded.value=false}}
onMounted(()=>{trendRange.value=timeRange.value==='24h'?'24h':'7d';loadData();loadHealthScore();loadSystemHealth();loadAnomalyStatus();loadSummary();setupTimer()})
onUnmounted(()=>clearInterval(refreshTimer))
</script>
<style scoped>
.stat-clickable{cursor:pointer!important}.stat-clickable:hover{transform:translateY(-3px)!important;box-shadow:var(--shadow-lg)!important;border-color:var(--color-primary)!important}
.user-link{color:var(--color-primary);cursor:pointer;text-decoration:none;font-weight:500}.user-link:hover{text-decoration:underline}
.hbar-rank{width:24px;font-size:var(--text-xs);color:var(--color-primary);font-weight:700;text-align:center;flex-shrink:0}
.hbar-fill-anim{width:0;animation:hbar-grow .8s ease-out forwards}@keyframes hbar-grow{from{width:0}to{width:var(--target-w)}}
.list-anim-enter-active{animation:list-in .2s ease-out}.list-anim-leave-active{animation:list-out .2s ease-in}.list-anim-move{transition:transform .2s ease}@keyframes list-in{from{opacity:0;transform:translateY(-10px)}to{opacity:1;transform:translateY(0)}}@keyframes list-out{from{opacity:1;transform:translateY(0)}to{opacity:0;transform:translateY(10px)}}
/* 安全驾驶舱 */
.cockpit-section{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:var(--space-4);margin-bottom:20px}
.cockpit-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-3)}
.cockpit-title{display:flex;align-items:center;gap:var(--space-2);font-size:var(--text-base);font-weight:700;color:var(--text-primary)}
.cockpit-body{display:flex;gap:var(--space-4);align-items:flex-start}
.cockpit-left{flex-shrink:0;width:140px}
.cockpit-center{flex:1;min-width:0}
.cockpit-right{flex-shrink:0;width:180px;background:var(--bg-elevated);border-radius:var(--radius-md);padding:var(--space-3)}
.score-ring-wrap{position:relative;width:130px;height:130px}
.score-ring-svg{width:100%;height:100%}
.score-ring-progress{transition:stroke-dasharray .8s ease}
.score-center{position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);text-align:center}
.score-number{font-size:2rem;font-weight:800;line-height:1;font-family:var(--font-mono)}
.score-label{font-size:var(--text-xs);font-weight:600;margin-top:2px}
.score-desc{display:flex;align-items:center;gap:var(--space-2);margin-bottom:var(--space-2)}
.score-level-badge{display:inline-block;padding:2px 10px;border-radius:9999px;font-size:var(--text-xs);font-weight:700;color:#fff}
.badge-excellent{background:#10B981}.badge-good{background:#3B82F6}.badge-warning{background:#F59E0B}.badge-danger{background:#EF4444}.badge-critical{background:#DC2626}
.score-text{font-size:var(--text-sm);color:var(--text-secondary)}
.deduction-list{margin-top:var(--space-2)}
.deduction-item{display:flex;align-items:center;gap:var(--space-2);padding:4px 0;font-size:var(--text-xs);border-bottom:1px solid var(--border-subtle)}
.deduction-name{font-weight:600;color:var(--text-primary);min-width:80px}
.deduction-points{color:#EF4444;font-weight:700;font-family:var(--font-mono);min-width:30px}
.deduction-detail{color:var(--text-tertiary);flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.deduction-jump{color:var(--color-primary);text-decoration:none;font-weight:700;font-size:var(--text-sm);flex-shrink:0;padding:0 4px;opacity:0.7;transition:opacity .2s}
.deduction-jump:hover{opacity:1;text-decoration:none}
.deduction-empty{font-size:var(--text-xs);color:var(--text-tertiary);padding:var(--space-2) 0}
.trend-mini{margin-top:var(--space-2)}
.trend-mini-label{font-size:10px;color:var(--text-tertiary);margin-bottom:2px}
.trend-mini-svg{width:100%;height:50px}
.trend-mini-dates{display:flex;justify-content:space-between;font-size:9px;color:var(--text-disabled)}
.sys-health-title{font-size:var(--text-xs);font-weight:700;color:var(--text-primary);margin-bottom:var(--space-2)}
.sys-metric{margin-bottom:var(--space-2)}
.sys-metric-header{display:flex;justify-content:space-between;font-size:10px;margin-bottom:2px}
.sys-metric-label{color:var(--text-secondary)}
.sys-metric-value{font-weight:700;font-family:var(--font-mono)}
.sys-metric-bar{height:6px;background:rgba(255,255,255,0.06);border-radius:3px;overflow:hidden}
.sys-metric-fill{height:100%;border-radius:3px;transition:width .6s ease}
.sys-goroutines{font-size:10px;color:var(--text-tertiary);display:flex;align-items:center;gap:4px;margin-top:var(--space-2)}
.refresh-control{display:flex;align-items:center;gap:var(--space-1);color:var(--text-tertiary)}
.refresh-select{background:var(--bg-elevated);border:1px solid var(--border-default);border-radius:var(--radius-sm);color:var(--text-primary);font-size:var(--text-xs);padding:2px 6px;cursor:pointer}
/* 异常检测指示器 (v11.2) */
.anomaly-indicator{margin-top:var(--space-2);padding:4px 0}
.anomaly-link{color:#EF4444;font-size:var(--text-xs);font-weight:700;text-decoration:none;cursor:pointer;display:inline-flex;align-items:center;gap:4px;padding:3px 10px;background:rgba(239,68,68,0.1);border-radius:9999px;transition:all .2s}
.anomaly-link:hover{background:rgba(239,68,68,0.2);text-decoration:none}
.anomaly-learning{color:var(--text-tertiary);font-size:var(--text-xs)}
.anomaly-ok{color:var(--text-tertiary);font-size:var(--text-xs);text-decoration:none}
.anomaly-ok:hover{color:var(--text-secondary)}
/* v11.4: 全局时间选择器 + 时间标注 */
.cockpit-controls{display:flex;align-items:center;gap:var(--space-2)}
.gen-report-btn{background:var(--bg-elevated);border:1px solid var(--color-primary);border-radius:var(--radius-sm);color:var(--color-primary);font-size:var(--text-xs);font-weight:600;padding:3px 8px;cursor:pointer;transition:all .2s;white-space:nowrap}
.gen-report-btn:hover{background:var(--color-primary);color:#fff}
.time-range-select{background:var(--bg-elevated);border:1px solid var(--color-primary);border-radius:var(--radius-sm);color:var(--color-primary);font-size:var(--text-xs);font-weight:600;padding:3px 8px;cursor:pointer;transition:all .2s}
.time-range-select:hover{background:var(--color-primary);color:#fff}
.time-badge{display:inline-block;padding:1px 6px;border-radius:9999px;font-size:10px;font-weight:600;color:var(--text-tertiary);background:rgba(107,114,128,0.15);margin-left:4px;vertical-align:middle;line-height:1.4}
@media(max-width:768px){.cockpit-body{flex-direction:column}.cockpit-left,.cockpit-right{width:100%}}
/* v18: 安全洞察 */
.insight-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:12px}
.insight-card{background:var(--bg-elevated);border:1px solid var(--border-subtle);border-radius:var(--radius-md);padding:16px;cursor:pointer;transition:all var(--transition-fast)}
.insight-card:hover{border-color:var(--color-primary);box-shadow:0 0 12px rgba(99,102,241,0.15)}
.insight-header{font-size:12px;color:var(--text-tertiary);margin-bottom:8px}
.insight-value{font-size:28px;font-weight:700;color:var(--text-primary)}
.insight-sub{font-size:11px;color:var(--text-tertiary);margin-top:4px}
.insight-value.danger{color:var(--color-danger)}.insight-value.warning{color:var(--color-warning)}.insight-value.success{color:var(--color-success)}
@media(max-width:900px){.insight-grid{grid-template-columns:repeat(2,1fr)}}
</style>
