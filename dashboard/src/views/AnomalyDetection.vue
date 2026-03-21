<template>
  <div>
    <div class="page-header">
      <div class="page-header-left">
        <h2><Icon name="bar-chart" :size="18" /> 异常基线检测</h2>
        <p class="page-desc">连续运行 {{ anomalyConfig.min_ready_days || 3 }} 天后自动建立正常行为基线，偏离 >{{ anomalyConfig.warning_threshold || 2 }}σ 自动告警</p>
      </div>
      <div class="page-header-right">
        <button class="btn btn-secondary btn-sm" @click="refreshAll" :disabled="refreshing">{{ refreshing ? '刷新中...' : '刷新' }}</button>
        <button class="btn btn-primary btn-sm" @click="router.push('/settings')"><Icon name="bell" :size="12" /> 告警设置</button>
      </div>
    </div>
    <div class="ov-cards" v-if="loaded">
      <StatCard :iconSvg="svgWave" :value="status.metrics_count||0" label="监控指标" color="blue"/>
      <StatCard :iconSvg="svgCheck" :value="readyDisplay" label="基线状态" :color="baselineStatusColor"/>
      <StatCard :iconSvg="svgAlert" :value="status.alerts_24h||0" label="24h 异常告警" color="red"/>
      <StatCard :iconSvg="svgClock" :value="lastAnomalyDisplay" label="最近异常" :color="lastAnomalyColor"/>
    </div>
    <div class="ov-cards" v-else><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>
    <div class="section-title"><span>基线管理</span><span class="section-badge">{{ readyCount }}/{{ status.metrics_count || 0 }} 就绪</span></div>
    <div class="baseline-grid" v-if="loaded">
      <div :class="cardClass(m)" v-for="m in metricsData" :key="m.metric_name">
        <div class="bc-header">
          <span class="bc-status-dot" :class="statusDotClass(m)"></span>
          <span class="bc-name">{{ metricDisplayName(m.metric_name) }}</span>
          <button class="bc-chart-btn" @click="openTrendModal(m)" title="查看趋势"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></button>
        </div>
        <div class="bc-body" v-if="m.baseline && m.baseline.ready">
          <div class="bc-row"><span class="bc-label">基线均值</span><span class="bc-val">{{ formatNum(m.baseline.hourly_mean[currentHour]) }}</span></div>
          <div class="bc-row"><span class="bc-label">标准差 (σ)</span><span class="bc-val">{{ formatNum(m.baseline.hourly_std[currentHour]) }}</span></div>
          <div class="bc-row"><span class="bc-label">样本数</span><span class="bc-val">{{ m.baseline.sample_count }}h ({{ Math.round(m.baseline.sample_count/24) }}天)</span></div>
          <div class="bc-row">
            <span class="bc-label">当前值</span>
            <span class="bc-val" :class="m.anomaly?'anomaly-val':'normal-val'">{{ formatNum(m.current_value) }} <span class="bc-status-tag" :class="m.anomaly?'tag-anomaly':'tag-normal'">{{ m.anomaly?'⚠️ 异常':'✅ 正常' }}</span></span>
          </div>
          <div class="bc-chart">
            <svg viewBox="0 0 360 80" class="baseline-svg" preserveAspectRatio="none">
              <polygon :points="sigma2BandPoints(m.baseline)" fill="rgba(99,102,241,0.10)" stroke="none"/>
              <polyline :points="meanLinePoints(m.baseline)" fill="none" stroke="#6366F1" stroke-width="1.5" stroke-linejoin="round"/>
              <circle :cx="15*currentHour" :cy="mapY(m.current_value, m.baseline)" :r="4" :fill="m.anomaly?'#EF4444':'#10B981'" stroke="#fff" stroke-width="1.5"/>
            </svg>
            <div class="bc-chart-labels"><span>0h</span><span>6h</span><span>12h</span><span>18h</span><span>23h</span></div>
          </div>
          <div class="bc-threshold">
            <div class="bc-thr-row"><span class="bc-thr-label">告警σ</span><div class="bc-thr-input-wrap"><input type="number" class="bc-thr-input" v-model.number="metricThresholdEdits[m.metric_name].warn" min="0.5" max="20" step="0.1"/></div></div>
            <div class="bc-thr-row"><span class="bc-thr-label">严重σ</span><div class="bc-thr-input-wrap"><input type="number" class="bc-thr-input" v-model.number="metricThresholdEdits[m.metric_name].crit" min="1" max="30" step="0.1"/></div></div>
            <button class="bc-thr-save" @click="saveMetricThreshold(m.metric_name)" :disabled="thresholdSaving[m.metric_name]">{{ thresholdSaving[m.metric_name] ? '...' : '保存' }}</button>
          </div>
        </div>
        <div class="bc-body bc-learning" v-else>
          <div class="bc-learning-icon">📡</div>
          <div class="bc-learning-text">数据收集中...</div>
          <div class="bc-learning-sub">需要至少 {{ anomalyConfig.min_ready_days || 3 }} 天数据建立基线</div>
          <div class="bc-progress-wrap"><div class="bc-progress-bar"><div class="bc-progress-fill" :style="{width: learningProgress(m)+'%'}"></div></div><span class="bc-progress-text">{{ learningProgress(m) }}%</span></div>
        </div>
      </div>
    </div>
    <div v-else class="baseline-grid"><Skeleton type="card" v-for="i in 6" :key="i"/></div>
    <div class="modal-overlay" v-if="trendModal.visible" @click.self="trendModal.visible=false">
      <div class="modal-box modal-lg">
        <div class="modal-header"><h3>{{ trendModal.displayName }} — 24h 趋势</h3><button class="modal-close" @click="trendModal.visible=false">✕</button></div>
        <div class="modal-body">
          <div v-if="trendModal.loading" style="text-align:center;padding:40px;color:var(--text-tertiary)">加载中...</div>
          <div v-else-if="!trendModal.ready" style="text-align:center;padding:40px;color:var(--text-tertiary)">基线尚未就绪</div>
          <TrendChart v-else :data="trendChartData" :lines="trendChartLines" :xLabels="trendChartXLabels" :height="220"/>
          <div class="trend-legend-custom" v-if="trendModal.ready && !trendModal.loading">
            <span class="tlc-item"><span class="tlc-dot" style="background:#6366F1"></span>基线均值</span>
            <span class="tlc-item"><span class="tlc-dot" style="background:#F59E0B"></span>告警 ({{ trendModal.warnT }}σ)</span>
            <span class="tlc-item"><span class="tlc-dot" style="background:#EF4444"></span>严重 ({{ trendModal.critT }}σ)</span>
            <span class="tlc-item"><span class="tlc-dot" style="background:#10B981"></span>当前值</span>
          </div>
        </div>
      </div>
    </div>
    <div class="section-title" style="margin-top:24px"><span>全局检测参数</span><button class="btn btn-sm btn-secondary" @click="showConfig = !showConfig" style="margin-left:auto;font-size:12px">{{ showConfig ? '收起' : '展开配置' }}</button></div>
    <div v-if="showConfig" class="config-panel card">
      <div class="config-grid">
        <div class="config-item"><label>基线窗口 (天)</label><input type="number" v-model.number="anomalyConfig.window_days" min="1" max="90" class="config-input"/></div>
        <div class="config-item"><label>默认告警阈值 (σ)</label><input type="number" v-model.number="anomalyConfig.warning_threshold" min="0.5" max="10" step="0.1" class="config-input"/></div>
        <div class="config-item"><label>默认严重阈值 (σ)</label><input type="number" v-model.number="anomalyConfig.critical_threshold" min="1" max="20" step="0.1" class="config-input"/></div>
        <div class="config-item"><label>最小标准差</label><input type="number" v-model.number="anomalyConfig.min_std_dev" min="0.1" max="100" step="0.1" class="config-input"/></div>
        <div class="config-item"><label>基线就绪最少天数</label><input type="number" v-model.number="anomalyConfig.min_ready_days" min="1" max="30" class="config-input"/></div>
        <div class="config-item"><label>基线更新间隔 (分钟)</label><input type="number" v-model.number="anomalyConfig.baseline_interval_min" min="1" max="1440" class="config-input"/></div>
        <div class="config-item"><label>异常检查间隔 (分钟)</label><input type="number" v-model.number="anomalyConfig.check_interval_min" min="1" max="1440" class="config-input"/></div>
        <div class="config-item"><label>最大告警数</label><input type="number" v-model.number="anomalyConfig.max_alerts" min="10" max="10000" class="config-input"/></div>
      </div>
      <div class="config-actions">
        <button class="btn btn-primary" @click="saveConfig" :disabled="configSaving">{{ configSaving ? '保存中...' : '保存配置' }}</button>
        <button class="btn btn-secondary" @click="resetConfig">恢复默认</button>
      </div>
    </div>
    <div class="section-title" style="margin-top:24px"><span>异常事件</span><span class="section-badge" :class="alerts.length?'badge-red':''">{{ alerts.length }} 条</span></div>
    <div class="filter-bar" v-if="alerts.length">
      <div class="filter-group"><label>指标</label><select v-model="filterMetric" class="filter-select"><option value="">全部</option><option v-for="mn in metricNames" :key="mn" :value="mn">{{ metricDisplayName(mn) }}</option></select></div>
      <div class="filter-group"><label>严重性</label><select v-model="filterSeverity" class="filter-select"><option value="">全部</option><option value="warning">⚠️ warning</option><option value="critical">🔴 critical</option></select></div>
      <div class="filter-group"><label>方向</label><select v-model="filterDirection" class="filter-select"><option value="">全部</option><option value="above">↑ 高于</option><option value="below">↓ 低于</option></select></div>
      <span class="filter-count">{{ filteredAlerts.length }}/{{ alerts.length }}</span>
    </div>
    <div class="card" v-if="loaded">
      <EmptyState v-if="!alerts.length" :iconSvg="svgShieldOk" title="未检测到异常" description="所有指标在正常基线范围内"/>
      <EmptyState v-else-if="!filteredAlerts.length" :iconSvg="svgFilter" title="无匹配结果" description="尝试调整过滤条件"/>
      <div v-else class="table-wrap">
        <table>
          <thead><tr><th>时间</th><th>指标</th><th>期望值</th><th>实际值</th><th>偏离</th><th>方向</th><th>严重度</th><th></th></tr></thead>
          <tbody>
            <template v-for="a in filteredAlerts" :key="a.id">
              <tr :class="'row-'+a.severity" @click="toggleExpand(a.id)" style="cursor:pointer">
                <td>{{ fmtTime(a.timestamp) }}</td>
                <td><a class="metric-link" @click.stop="onAlertMetricClick(a.metric_name)">{{ metricDisplayName(a.metric_name) }}</a></td>
                <td class="mono">{{ formatNum(a.expected) }}</td>
                <td class="mono fw-bold">{{ formatNum(a.actual) }}</td>
                <td><div class="deviation-cell"><div class="deviation-bar-wrap"><div class="deviation-bar" :style="{width: Math.min(100, a.deviation/5*100)+'%', background: a.severity==='critical'?'#EF4444':'#F59E0B'}"></div><div class="deviation-mark-2s"></div><div class="deviation-mark-3s"></div></div><span class="deviation-text">{{ a.deviation.toFixed(1) }}σ</span></div></td>
                <td><span :class="'dir-'+a.direction">{{ a.direction==='above'?'↑ 高于':'↓ 低于' }}</span></td>
                <td><span class="severity-badge" :class="'sev-'+a.severity">{{ a.severity==='critical'?'🔴 严重':'🟡 告警' }}</span></td>
                <td><span class="expand-icon" :class="{rotated: expandedAlerts[a.id]}">▸</span></td>
              </tr>
              <tr v-if="expandedAlerts[a.id]" class="detail-row" :class="'row-'+a.severity">
                <td colspan="8">
                  <div class="alert-detail">
                    <div class="alert-detail-grid">
                      <div class="ad-item"><span class="ad-label">指标</span><span class="ad-val">{{ a.metric_name }}</span></div>
                      <div class="ad-item"><span class="ad-label">基线均值</span><span class="ad-val mono">{{ formatNum(a.expected) }}</span></div>
                      <div class="ad-item"><span class="ad-label">标准差</span><span class="ad-val mono">{{ formatNum(a.std_dev) }}</span></div>
                      <div class="ad-item"><span class="ad-label">实际值</span><span class="ad-val mono fw-bold" :class="a.severity==='critical'?'anomaly-val':'warn-val'">{{ formatNum(a.actual) }}</span></div>
                      <div class="ad-item"><span class="ad-label">偏离</span><span class="ad-val mono">{{ a.deviation.toFixed(2) }}σ</span></div>
                      <div class="ad-item"><span class="ad-label">偏离%</span><span class="ad-val mono">{{ a.expected > 0 ? ((Math.abs(a.actual - a.expected) / a.expected) * 100).toFixed(1) + '%' : '--' }}</span></div>
                    </div>
                    <div class="ad-comparison">
                      <div class="ad-bar-group"><div class="ad-bar-label">基线</div><div class="ad-bar-track"><div class="ad-bar-fill ad-bar-baseline" :style="{width: compWidth(a,'expected')+'%'}"></div></div><span class="ad-bar-val">{{ formatNum(a.expected) }}</span></div>
                      <div class="ad-bar-group"><div class="ad-bar-label">实际</div><div class="ad-bar-track"><div class="ad-bar-fill" :class="a.severity==='critical'?'ad-bar-critical':'ad-bar-warning'" :style="{width: compWidth(a,'actual')+'%'}"></div></div><span class="ad-bar-val" :class="a.severity==='critical'?'anomaly-val':'warn-val'">{{ formatNum(a.actual) }}</span></div>
                    </div>
                    <div class="ad-actions">
                      <button class="btn btn-secondary btn-xs" @click.stop="openTrendModalForMetric(a.metric_name)">📈 趋势</button>
                      <button class="btn btn-secondary btn-xs" @click.stop="router.push('/settings')">🔔 告警设置</button>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>
    <Skeleton v-else type="table"/>
  </div>
</template>
<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { api, apiPut } from '../api.js'
import { showToast } from '../stores/app.js'
import Icon from '../components/Icon.vue'
import StatCard from '../components/StatCard.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'
import TrendChart from '../components/TrendChart.vue'

const router = useRouter()
const svgWave='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M2 12h2l3-7 4 14 4-10 3 3h4"/></svg>'
const svgCheck='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>'
const svgAlert='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgClock='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>'
const svgShieldOk='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>'
const svgFilter='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/></svg>'

const loaded = ref(false), refreshing = ref(false), status = ref({}), baselines = ref({}), alerts = ref([]), metricsData = ref([])
const currentHour = new Date().getUTCHours()
const filterMetric = ref(''), filterSeverity = ref(''), filterDirection = ref('')
const expandedAlerts = reactive({})
const showConfig = ref(false), configSaving = ref(false)
const defaultConfig = {window_days:7,warning_threshold:2.0,critical_threshold:3.0,min_std_dev:1.0,min_ready_days:3,baseline_interval_min:60,check_interval_min:5,max_alerts:100}
const anomalyConfig = ref({...defaultConfig})
const metricThresholdEdits = reactive({})
const thresholdSaving = reactive({})
const trendModal = reactive({visible:false,metricName:'',displayName:'',loading:false,ready:false,points:[],warnT:2,critT:3})
const metricNames = ['im_requests_per_hour','im_blocks_per_hour','llm_calls_per_hour','llm_tokens_per_hour','tool_calls_per_hour','high_risk_tools_per_hour']
const metricLabels = {'im_requests_per_hour':'IM 每小时请求数','im_blocks_per_hour':'IM 每小时拦截数','llm_calls_per_hour':'LLM 每小时调用数','llm_tokens_per_hour':'LLM 每小时 Token 消耗','tool_calls_per_hour':'每小时工具调用数','high_risk_tools_per_hour':'每小时高危工具调用数'}
function metricDisplayName(n){return metricLabels[n]||n}
const metricRouteMap={'im_requests_per_hour':'/audit','im_blocks_per_hour':'/audit','llm_calls_per_hour':'/agent','llm_tokens_per_hour':'/agent','tool_calls_per_hour':'/agent','high_risk_tools_per_hour':'/agent'}
function onAlertMetricClick(mn){const t=metricRouteMap[mn];if(t)router.push(t)}
function toggleExpand(id){expandedAlerts[id]=!expandedAlerts[id]}

const readyCount = computed(()=>metricsData.value.filter(m=>m.baseline&&m.baseline.ready).length)
const readyDisplay = computed(()=>{const t=status.value.metrics_count||0,r=status.value.baselines_ready||0;if(r>=t&&t>0)return'全部就绪';if(r===0)return'学习中';return r+'/'+t+' 就绪'})
const baselineStatusColor = computed(()=>{const t=status.value.metrics_count||0,r=status.value.baselines_ready||0;if(r>=t&&t>0)return'green';if(r===0)return'yellow';return'blue'})
const lastAnomalyDisplay = computed(()=>{if(!alerts.value.length)return'无';const ts=alerts.value[0].timestamp;if(!ts)return'无';const d=new Date(ts);if(isNaN(d.getTime()))return String(ts);const diff=Date.now()-d.getTime();if(diff<60000)return'刚刚';if(diff<3600000)return Math.floor(diff/60000)+'分钟前';if(diff<86400000)return Math.floor(diff/3600000)+'小时前';return Math.floor(diff/86400000)+'天前'})
const lastAnomalyColor = computed(()=>{if(!alerts.value.length)return'green';const diff=Date.now()-new Date(alerts.value[0].timestamp).getTime();if(diff<3600000)return'red';if(diff<86400000)return'yellow';return'green'})
const filteredAlerts = computed(()=>{let l=alerts.value;if(filterMetric.value)l=l.filter(a=>a.metric_name===filterMetric.value);if(filterSeverity.value)l=l.filter(a=>a.severity===filterSeverity.value);if(filterDirection.value)l=l.filter(a=>a.direction===filterDirection.value);return l})

const trendChartData = computed(()=>{if(!trendModal.points||!trendModal.points.length)return[];return trendModal.points.map(p=>({baseline:p.baseline_mean,upper_warn:p.upper_warn,upper_crit:p.upper_crit,current:p.is_current?p.current_value:null}))})
const trendChartLines=[{key:'baseline',color:'#6366F1',label:'基线均值',width:2},{key:'upper_warn',color:'#F59E0B',label:'告警阈值',width:1},{key:'upper_crit',color:'#EF4444',label:'严重阈值',width:1},{key:'current',color:'#10B981',label:'当前值',width:2}]
const trendChartXLabels = computed(()=>{if(!trendModal.points||!trendModal.points.length)return[];return trendModal.points.map(p=>p.hour+':00')})

function cardClass(m){if(m.anomaly)return'baseline-card bc-anomaly-active';if(m.baseline&&m.baseline.ready)return'baseline-card bc-normal';return'baseline-card bc-learning-card'}
function statusDotClass(m){if(m.anomaly)return'dot-red';if(m.baseline&&m.baseline.ready)return'dot-green';return'dot-gray'}
function formatNum(v){if(v==null)return'--';if(typeof v!=='number')return String(v);if(v>=1000)return(v/1000).toFixed(1)+'k';return Number(v.toFixed(1)).toString()}
function fmtTime(ts){if(!ts)return'--';const d=new Date(ts);return isNaN(d.getTime())?String(ts):d.toLocaleString('zh-CN',{hour12:false})}
function learningProgress(m){if(!m.baseline)return 0;const s=m.baseline.sample_count||0;return Math.min(100,Math.round(s/((anomalyConfig.value.min_ready_days||3)*24)*100))}
function compWidth(a,f){const mx=Math.max(a.expected||0,a.actual||0,1);return Math.max(3,(a[f]/mx)*100)}

function getMinMax(bl){let mn=Infinity,mx=-Infinity;for(let h=0;h<24;h++){const lo=(bl.hourly_mean[h]||0)-2*(bl.hourly_std[h]||1);const hi=(bl.hourly_mean[h]||0)+2*(bl.hourly_std[h]||1);if(lo<mn)mn=lo;if(hi>mx)mx=hi}if(mn===mx){mn=0;mx=10}return{min:Math.max(0,mn-(mx-mn)*0.1),max:mx+(mx-mn)*0.1}}
function mapY(val,bl){const{min,max}=getMinMax(bl);return 70-((val-min)/(max-min))*60}
function meanLinePoints(bl){const{min,max}=getMinMax(bl);let pts=[];for(let h=0;h<24;h++){pts.push(h*15+','+(70-((bl.hourly_mean[h]||0)-min)/(max-min)*60))}return pts.join(' ')}
function sigma2BandPoints(bl){const{min,max}=getMinMax(bl);let u=[],l=[];for(let h=0;h<24;h++){const m=bl.hourly_mean[h]||0,s=bl.hourly_std[h]||1;u.push(h*15+','+(70-(m+2*s-min)/(max-min)*60));l.unshift(h*15+','+(70-(Math.max(0,m-2*s)-min)/(max-min)*60))}return[...u,...l].join(' ')}

function initMetricThresholds(apiThresholds){
  for(const mn of metricNames){
    if(!metricThresholdEdits[mn]){
      const fromApi=apiThresholds&&apiThresholds[mn]
      metricThresholdEdits[mn]={warn:fromApi?fromApi.warning_threshold:(anomalyConfig.value.warning_threshold||2.0),crit:fromApi?fromApi.critical_threshold:(anomalyConfig.value.critical_threshold||3.0)}
    }
    if(thresholdSaving[mn]===undefined)thresholdSaving[mn]=false
  }
}

async function saveMetricThreshold(mn){
  thresholdSaving[mn]=true
  try{
    await apiPut('/api/v1/anomaly/metric-thresholds/'+mn,{warning_threshold:metricThresholdEdits[mn].warn,critical_threshold:metricThresholdEdits[mn].crit})
    showToast(metricDisplayName(mn)+' 阈值已保存','success')
  }catch(e){showToast('保存失败: '+e.message,'error')}
  thresholdSaving[mn]=false
}

async function openTrendModal(m){
  trendModal.metricName=m.metric_name
  trendModal.displayName=metricDisplayName(m.metric_name)
  trendModal.visible=true
  trendModal.loading=true
  trendModal.ready=false
  try{
    const d=await api('/api/v1/anomaly/trend/'+m.metric_name)
    trendModal.points=d.points||[]
    trendModal.ready=d.ready||false
    trendModal.warnT=d.warning_threshold||2
    trendModal.critT=d.critical_threshold||3
  }catch{trendModal.ready=false;trendModal.points=[]}
  trendModal.loading=false
}
function openTrendModalForMetric(mn){
  const m=metricsData.value.find(x=>x.metric_name===mn)
  if(m)openTrendModal(m)
  else{trendModal.metricName=mn;trendModal.displayName=metricDisplayName(mn);trendModal.visible=true;trendModal.loading=true;openTrendModal({metric_name:mn})}
}

async function loadConfig(){try{anomalyConfig.value=await api('/api/v1/anomaly/config')}catch{anomalyConfig.value={...defaultConfig}}}
async function saveConfig(){
  configSaving.value=true
  try{const d=await apiPut('/api/v1/anomaly/config',anomalyConfig.value);anomalyConfig.value=d.config||anomalyConfig.value;showToast('配置已保存','success')}
  catch(e){showToast('保存失败: '+e.message,'error')}
  configSaving.value=false
}
function resetConfig(){anomalyConfig.value={...defaultConfig}}

async function loadData(){
  try{status.value=await api('/api/v1/anomaly/status')}catch{status.value={enabled:false}}
  try{const d=await api('/api/v1/anomaly/baselines');baselines.value=d.baselines||{}}catch{baselines.value={}}
  try{const d=await api('/api/v1/anomaly/alerts?limit=50');alerts.value=d.alerts||[]}catch{alerts.value=[]}
  const data=[]
  for(const name of metricNames){
    try{const detail=await api('/api/v1/anomaly/metric/'+name);data.push(detail)}
    catch{data.push({metric_name:name,baseline:baselines.value[name]||null,current_value:0,anomaly:false})}
  }
  metricsData.value=data
  loaded.value=true
}

async function loadThresholds(){
  let t={}
  try{const d=await api('/api/v1/anomaly/metric-thresholds');t=d.thresholds||{}}catch{}
  initMetricThresholds(t)
}

async function refreshAll(){
  refreshing.value=true
  await Promise.all([loadData(),loadConfig(),loadThresholds()])
  refreshing.value=false
  showToast('数据已刷新','success')
}

let timer=null
onMounted(()=>{loadData();loadConfig();loadThresholds();timer=setInterval(loadData,60000)})
onUnmounted(()=>clearInterval(timer))
</script>
<style scoped>
.page-header{margin-bottom:20px;display:flex;justify-content:space-between;align-items:flex-start;flex-wrap:wrap;gap:12px}
.page-header-left{flex:1}
.page-header-right{display:flex;gap:8px;align-items:center}
.page-header h2{margin:0 0 4px 0;font-size:var(--text-lg);color:var(--text-primary);display:flex;align-items:center;gap:8px}
.page-desc{font-size:var(--text-sm);color:var(--text-tertiary);margin:0}
.section-title{font-size:var(--text-base);font-weight:700;color:var(--text-primary);margin-bottom:12px;display:flex;align-items:center;gap:8px}
.section-badge{font-size:11px;padding:2px 8px;border-radius:9999px;background:rgba(99,102,241,0.15);color:#818CF8;font-weight:600}
.badge-red{background:rgba(239,68,68,0.15);color:#EF4444}
.baseline-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(320px,1fr));gap:16px;margin-bottom:20px}
.baseline-card{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);padding:16px;transition:all .2s}
.baseline-card:hover{border-color:var(--color-primary)}
.bc-normal{border-left:3px solid #10B981}
.bc-learning-card{border-left:3px solid #64748B;opacity:0.85}
.bc-anomaly-active{border-left:3px solid #EF4444;border-color:#EF4444;animation:anomaly-pulse 2s ease-in-out infinite}
@keyframes anomaly-pulse{0%,100%{box-shadow:0 0 0 0 rgba(239,68,68,0.2)}50%{box-shadow:0 0 12px 2px rgba(239,68,68,0.3)}}
.bc-header{display:flex;align-items:center;gap:8px;margin-bottom:12px}
.bc-status-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}
.dot-green{background:#10B981}
.dot-red{background:#EF4444;animation:dot-blink 1s ease-in-out infinite}
.dot-gray{background:#64748B}
@keyframes dot-blink{0%,100%{opacity:1}50%{opacity:0.4}}
.bc-name{font-size:var(--text-sm);font-weight:700;color:var(--text-primary);flex:1}
.bc-chart-btn{background:none;border:1px solid var(--border-subtle);border-radius:var(--radius-md);padding:4px 6px;cursor:pointer;color:var(--text-tertiary);transition:all .2s;display:flex;align-items:center}
.bc-chart-btn:hover{color:var(--color-primary);border-color:var(--color-primary)}
.bc-row{display:flex;justify-content:space-between;align-items:center;padding:3px 0;font-size:var(--text-xs)}
.bc-label{color:var(--text-tertiary)}
.bc-val{color:var(--text-primary);font-family:var(--font-mono);font-weight:600}
.bc-sub{color:var(--text-disabled);font-weight:400}
.bc-status-tag{padding:1px 6px;border-radius:9999px;font-size:10px;font-weight:700}
.tag-anomaly{background:rgba(239,68,68,0.15);color:#EF4444}
.tag-normal{background:rgba(16,185,129,0.15);color:#10B981}
.anomaly-val{color:#EF4444!important}
.normal-val{color:#10B981!important}
.warn-val{color:#F59E0B!important}
.bc-chart{margin-top:10px}
.baseline-svg{width:100%;height:60px}
.bc-chart-labels{display:flex;justify-content:space-between;font-size:9px;color:var(--text-disabled);padding:0 2px}
.bc-threshold{display:flex;align-items:center;gap:8px;margin-top:10px;padding-top:10px;border-top:1px solid var(--border-subtle)}
.bc-thr-row{display:flex;align-items:center;gap:4px}
.bc-thr-label{font-size:10px;color:var(--text-tertiary);min-width:36px}
.bc-thr-input-wrap{display:flex;align-items:center}
.bc-thr-input{width:56px;background:var(--bg-primary);border:1px solid var(--border-subtle);border-radius:var(--radius-sm);padding:3px 6px;color:var(--text-primary);font-size:11px;font-family:var(--font-mono);text-align:center}
.bc-thr-input:focus{border-color:var(--color-primary);outline:none}
.bc-thr-save{padding:3px 10px;font-size:11px;background:var(--color-primary);color:#fff;border:none;border-radius:var(--radius-sm);cursor:pointer;margin-left:auto;transition:opacity .2s}
.bc-thr-save:hover{opacity:0.85}
.bc-thr-save:disabled{opacity:0.5;cursor:not-allowed}
.bc-learning{text-align:center;padding:20px 0}
.bc-learning-icon{font-size:2rem;margin-bottom:8px}
.bc-learning-text{font-size:var(--text-sm);font-weight:700;color:var(--text-primary);margin-bottom:4px}
.bc-learning-sub{font-size:var(--text-xs);color:var(--text-tertiary);margin-bottom:12px}
.bc-progress-wrap{display:flex;align-items:center;gap:8px}
.bc-progress-bar{flex:1;height:6px;background:rgba(255,255,255,0.06);border-radius:3px;overflow:hidden}
.bc-progress-fill{height:100%;background:linear-gradient(90deg,#6366F1,#818CF8);border-radius:3px;transition:width .5s}
.bc-progress-text{font-size:11px;color:var(--text-tertiary);font-family:var(--font-mono);min-width:30px}
.metric-link{color:var(--color-primary);cursor:pointer;font-weight:600;text-decoration:none}
.metric-link:hover{text-decoration:underline}
.filter-bar{display:flex;align-items:center;gap:16px;margin-bottom:12px;flex-wrap:wrap}
.filter-group{display:flex;align-items:center;gap:6px}
.filter-group label{font-size:var(--text-xs);color:var(--text-tertiary);font-weight:600}
.filter-select{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-md);padding:4px 8px;color:var(--text-primary);font-size:var(--text-xs)}
.filter-select:focus{border-color:var(--color-primary);outline:none}
.filter-count{font-size:var(--text-xs);color:var(--text-disabled);font-family:var(--font-mono);margin-left:auto}
.mono{font-family:var(--font-mono)}
.fw-bold{font-weight:700}
.row-critical{background:rgba(239,68,68,0.06)}
.row-warning{background:rgba(245,158,11,0.06)}
.deviation-cell{display:flex;align-items:center;gap:8px}
.deviation-bar-wrap{flex:1;height:8px;background:rgba(255,255,255,0.06);border-radius:4px;position:relative;overflow:hidden;min-width:60px}
.deviation-bar{height:100%;border-radius:4px;transition:width .5s}
.deviation-mark-2s{position:absolute;left:40%;top:0;bottom:0;width:2px;background:#F59E0B;opacity:.5}
.deviation-mark-3s{position:absolute;left:60%;top:0;bottom:0;width:2px;background:#EF4444;opacity:.5}
.deviation-text{font-size:var(--text-xs);font-weight:700;font-family:var(--font-mono);min-width:36px}
.dir-above{color:#EF4444;font-weight:600}
.dir-below{color:#3B82F6;font-weight:600}
.severity-badge{display:inline-block;padding:2px 8px;border-radius:9999px;font-size:11px;font-weight:700}
.sev-critical{background:rgba(239,68,68,0.15);color:#EF4444}
.sev-warning{background:rgba(245,158,11,0.15);color:#F59E0B}
.expand-icon{color:var(--text-disabled);font-size:12px;transition:transform .2s;display:inline-block}
.expand-icon.rotated{transform:rotate(90deg)}
.detail-row td{padding:0!important}
.alert-detail{padding:16px 24px;border-top:1px solid var(--border-subtle)}
.alert-detail-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:12px;margin-bottom:16px}
.ad-item{display:flex;flex-direction:column;gap:2px}
.ad-label{font-size:10px;color:var(--text-disabled);text-transform:uppercase;letter-spacing:0.5px}
.ad-val{font-size:var(--text-sm);color:var(--text-primary)}
.ad-comparison{margin-bottom:12px}
.ad-bar-group{display:flex;align-items:center;gap:8px;margin-bottom:6px}
.ad-bar-label{font-size:11px;color:var(--text-tertiary);min-width:32px}
.ad-bar-track{flex:1;height:10px;background:rgba(255,255,255,0.06);border-radius:5px;overflow:hidden}
.ad-bar-fill{height:100%;border-radius:5px;transition:width .5s}
.ad-bar-baseline{background:#6366F1}
.ad-bar-warning{background:#F59E0B}
.ad-bar-critical{background:#EF4444}
.ad-bar-val{font-size:11px;font-family:var(--font-mono);min-width:40px;font-weight:600}
.ad-actions{display:flex;gap:8px}
.btn-xs{padding:3px 8px;font-size:11px;display:flex;align-items:center;gap:4px}
.config-panel{padding:20px;margin-bottom:16px}
.config-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:16px;margin-bottom:16px}
.config-item{display:flex;flex-direction:column;gap:4px}
.config-item label{font-size:var(--text-xs);color:var(--text-tertiary);font-weight:600}
.config-input{background:var(--bg-primary);border:1px solid var(--border-subtle);border-radius:var(--radius-md);padding:6px 10px;color:var(--text-primary);font-size:var(--text-sm);font-family:var(--font-mono);width:100%}
.config-input:focus{border-color:var(--color-primary);outline:none;box-shadow:0 0 0 2px rgba(99,102,241,0.2)}
.config-actions{display:flex;align-items:center;gap:12px}
.btn-sm{padding:4px 10px;font-size:12px}
.btn-secondary{background:var(--bg-surface);border:1px solid var(--border-subtle);color:var(--text-secondary);border-radius:var(--radius-md);cursor:pointer;display:flex;align-items:center;gap:4px}
.btn-secondary:hover{border-color:var(--color-primary);color:var(--text-primary)}
.modal-overlay{position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.6);z-index:1000;display:flex;align-items:center;justify-content:center;padding:20px}
.modal-box{background:var(--bg-surface);border:1px solid var(--border-subtle);border-radius:var(--radius-lg);max-width:800px;width:100%;max-height:80vh;overflow:auto}
.modal-lg{min-width:600px}
.modal-header{display:flex;justify-content:space-between;align-items:center;padding:16px 20px;border-bottom:1px solid var(--border-subtle)}
.modal-header h3{margin:0;font-size:var(--text-base);color:var(--text-primary)}
.modal-close{background:none;border:none;color:var(--text-tertiary);font-size:18px;cursor:pointer;padding:4px 8px}
.modal-close:hover{color:var(--text-primary)}
.modal-body{padding:20px}
.trend-legend-custom{display:flex;gap:16px;justify-content:center;margin-top:12px;flex-wrap:wrap}
.tlc-item{display:flex;align-items:center;gap:4px;font-size:11px;color:var(--text-tertiary)}
.tlc-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}
</style>
