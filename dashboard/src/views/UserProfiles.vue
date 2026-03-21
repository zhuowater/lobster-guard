<template>
  <div>
    <!-- 统计卡片 -->
    <div class="ov-cards" v-if="loaded">
      <StatCard :iconSvg="svgUsers" :value="stats.total_users" label="用户总数" color="blue" />
      <StatCard :iconSvg="svgAlert" :value="stats.critical_count + stats.high_count" label="高危用户" color="red" />
      <StatCard :iconSvg="svgScore" :value="stats.avg_score" label="平均风险分" color="yellow" />
      <StatCard :iconSvg="svgBell" :value="stats.alerts_24h" label="24h 告警" color="purple" />
    </div>
    <div class="ov-cards" v-else><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /><Skeleton type="card" /></div>

    <!-- 搜索过滤 -->
    <div class="filter-bar card" style="margin-bottom:16px">
      <div class="filter-bar-inner">
        <div class="search-box">
          <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input v-model="searchText" class="search-input" placeholder="搜索用户ID或名称..." />
          <button v-if="searchText" class="search-clear" @click="searchText=''">&times;</button>
        </div>
        <div class="filter-selects">
          <select v-model="filterRisk" class="filter-select"><option value="">全部风险</option><option value="critical">极危</option><option value="high">高危</option><option value="medium">中危</option><option value="low">低危</option></select>
          <select v-model="sortBy" class="filter-select"><option value="risk">按风险排序</option><option value="requests">按请求量</option><option value="recent">按最近活跃</option><option value="block_rate">按拦截率</option></select>
        </div>
        <span class="filter-count">{{ filteredUsers.length }}/{{ users.length }}</span>
      </div>
    </div>

    <!-- TOP10 + Pie -->
    <div class="ov-row">
      <div class="card" style="flex:2">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg></span>
          <span class="card-title">风险用户列表</span>
        </div>
        <Skeleton v-if="!loaded" type="table" />
        <EmptyState v-else-if="!filteredUsers.length" :iconSvg="svgShieldCheck" title="暂无用户数据" description="注入演示数据后将显示用户画像" />
        <div v-else class="table-wrap">
          <table>
            <thead>
              <tr><th style="width:40px">#</th><th>用户</th><th>风险分</th><th>等级</th><th>请求数</th><th>拦截率</th><th>注入尝试</th><th>高危工具</th><th>最后活跃</th><th>趋势</th></tr>
            </thead>
            <tbody>
              <tr v-for="(u, i) in pagedUsers" :key="u.user_id" class="row-clickable" @click="goToDetail(u.user_id)">
                <td class="rank-cell">{{ (currentPage - 1) * pageSize + i + 1 }}</td>
                <td class="user-cell">
                  <span class="user-avatar" :class="'avatar-' + u.risk_level">{{ u.user_id.charAt(0).toUpperCase() }}</span>
                  <span>{{ u.display_name || u.user_id }}</span>
                </td>
                <td>
                  <div class="score-bar"><div class="score-fill" :style="{ width: u.risk_score + '%', background: riskColor(u.risk_level) }"></div><span class="score-num">{{ u.risk_score }}</span></div>
                </td>
                <td><span class="risk-badge" :class="'risk-' + u.risk_level">{{ riskLabel(u.risk_level) }}</span></td>
                <td>{{ u.total_requests }}</td>
                <td :class="{ 'text-danger': u.block_rate > 0.3 }">{{ (u.block_rate * 100).toFixed(1) }}%</td>
                <td>{{ u.injection_attempts }}</td>
                <td>{{ u.high_risk_tools }}</td>
                <td>{{ fmtTimeAgo(u.last_seen) }}</td>
                <td><span class="trend-arrow" :class="'trend-' + u.risk_trend">{{ trendIcon(u.risk_trend) }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
        <!-- 分页 -->
        <div v-if="filteredUsers.length > pageSize" class="dt-pagination">
          <div class="dt-page-info">共 {{ filteredUsers.length }} 条，第 {{ currentPage }}/{{ totalPages }} 页</div>
          <div class="dt-page-controls">
            <button class="btn btn-ghost btn-sm" :disabled="currentPage<=1" @click="currentPage--">上一页</button>
            <span class="dt-page-num">{{ currentPage }}</span>
            <button class="btn btn-ghost btn-sm" :disabled="currentPage>=totalPages" @click="currentPage++">下一页</button>
          </div>
        </div>
      </div>
      <div class="card" style="flex:1">
        <div class="card-header">
          <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.21 15.89A10 10 0 1 1 8 2.83"/><path d="M22 12A10 10 0 0 0 12 2v10z"/></svg></span>
          <span class="card-title">风险等级分布</span>
        </div>
        <Skeleton v-if="!loaded" type="chart" />
        <PieChart v-else :data="pieData" :size="180" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api.js'
import StatCard from '../components/StatCard.vue'
import PieChart from '../components/PieChart.vue'
import EmptyState from '../components/EmptyState.vue'
import Skeleton from '../components/Skeleton.vue'

const router = useRouter()

const svgUsers='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>'
const svgAlert='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgScore='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="5" x2="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/></svg>'
const svgBell='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>'
const svgShieldCheck='<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><polyline points="9 12 11 14 15 10"/></svg>'

const loaded=ref(false),users=ref([]),stats=ref({total_users:0,critical_count:0,high_count:0,medium_count:0,low_count:0,avg_score:0,alerts_24h:0}),pieData=ref([])
const searchText=ref(''),filterRisk=ref(''),sortBy=ref('risk'),currentPage=ref(1),pageSize=20
const riskOrder={critical:0,high:1,medium:2,low:3}

function riskColor(l){return{critical:'#EF4444',high:'#F59E0B',medium:'#3B82F6',low:'#6B7280'}[l]||'#6B7280'}
function riskLabel(l){return{critical:'极危',high:'高危',medium:'中危',low:'低危'}[l]||l}
function trendIcon(t){return t==='rising'?'↑':t==='falling'?'↓':'→'}
function fmtTimeAgo(ts){if(!ts)return'--';const d=new Date(ts);if(isNaN(d.getTime()))return'--';const diff=Date.now()-d.getTime();if(diff<60000)return'刚刚';if(diff<3600000)return Math.floor(diff/60000)+'分钟前';if(diff<86400000)return Math.floor(diff/3600000)+'小时前';return Math.floor(diff/86400000)+'天前'}
function goToDetail(userId){router.push('/user-profiles/'+encodeURIComponent(userId))}

const filteredUsers=computed(()=>{
  let list=[...users.value]
  if(searchText.value){const q=searchText.value.toLowerCase();list=list.filter(u=>(u.user_id||'').toLowerCase().includes(q)||(u.display_name||'').toLowerCase().includes(q))}
  if(filterRisk.value)list=list.filter(u=>u.risk_level===filterRisk.value)
  if(sortBy.value==='risk')list.sort((a,b)=>(riskOrder[a.risk_level]??9)-(riskOrder[b.risk_level]??9)||(b.risk_score||0)-(a.risk_score||0))
  else if(sortBy.value==='requests')list.sort((a,b)=>(b.total_requests||0)-(a.total_requests||0))
  else if(sortBy.value==='recent')list.sort((a,b)=>new Date(b.last_seen||0)-new Date(a.last_seen||0))
  else if(sortBy.value==='block_rate')list.sort((a,b)=>(b.block_rate||0)-(a.block_rate||0))
  return list
})
const totalPages=computed(()=>Math.max(1,Math.ceil(filteredUsers.value.length/pageSize)))
const pagedUsers=computed(()=>{const s=(currentPage.value-1)*pageSize;return filteredUsers.value.slice(s,s+pageSize)})
watch([searchText,filterRisk,sortBy],()=>{currentPage.value=1})

async function loadData(){
  try{
    const [topRes,statsRes]=await Promise.all([api('/api/v1/users/risk-top?limit=50'),api('/api/v1/users/risk-stats')])
    users.value=topRes.users||[];stats.value=statsRes
    pieData.value=[{label:'极危',value:statsRes.critical_count||0,color:'#EF4444'},{label:'高危',value:statsRes.high_count||0,color:'#F59E0B'},{label:'中危',value:statsRes.medium_count||0,color:'#3B82F6'},{label:'低危',value:statsRes.low_count||0,color:'#6B7280'}].filter(d=>d.value>0)
  }catch(e){console.error('Failed to load user profiles:',e)}
  loaded.value=true
}

let timer=null
onMounted(()=>{loadData();timer=setInterval(loadData,30000)})
onUnmounted(()=>clearInterval(timer))
</script>

<style scoped>
.filter-bar{padding:12px 16px}.filter-bar-inner{display:flex;gap:12px;align-items:center;flex-wrap:wrap}
.search-box{position:relative;flex:1;min-width:200px}
.search-icon{position:absolute;left:10px;top:50%;transform:translateY(-50%);color:var(--text-tertiary);pointer-events:none}
.search-input{width:100%;padding:8px 30px 8px 32px;background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-sm)}
.search-input:focus{outline:none;border-color:var(--color-primary)}
.search-clear{position:absolute;right:8px;top:50%;transform:translateY(-50%);background:none;border:none;color:var(--text-tertiary);font-size:1.1rem;cursor:pointer;padding:0 4px}
.search-clear:hover{color:var(--text-primary)}
.filter-selects{display:flex;gap:8px;flex-wrap:wrap}
.filter-select{padding:6px 10px;background:var(--bg-base);border:1px solid var(--border-subtle);border-radius:var(--radius-md);color:var(--text-primary);font-size:var(--text-xs);cursor:pointer}
.filter-select:focus{outline:none;border-color:var(--color-primary)}
.filter-count{font-size:var(--text-xs);color:var(--text-tertiary);white-space:nowrap}

.row-clickable{cursor:pointer;transition:background .15s}.row-clickable:hover{background:rgba(99,102,241,.06)!important}
.rank-cell{font-weight:700;color:var(--color-primary);text-align:center}
.user-cell{display:flex;align-items:center;gap:8px}
.user-avatar{width:28px;height:28px;border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:700;color:#fff;flex-shrink:0}
.avatar-critical{background:#EF4444}.avatar-high{background:#F59E0B}.avatar-medium{background:#3B82F6}.avatar-low{background:#6B7280}
.score-bar{position:relative;width:80px;height:20px;background:var(--bg-elevated);border-radius:10px;overflow:hidden;display:inline-flex;align-items:center}
.score-fill{position:absolute;left:0;top:0;bottom:0;border-radius:10px;transition:width .5s ease}
.score-num{position:relative;z-index:1;width:100%;text-align:center;font-size:11px;font-weight:700;color:var(--text-primary)}
.risk-badge{display:inline-block;padding:2px 8px;border-radius:9999px;font-size:var(--text-xs);font-weight:600;text-transform:uppercase}
.risk-low{background:rgba(107,114,128,.15);color:#6B7280}.risk-medium{background:rgba(59,130,246,.15);color:#3B82F6}
.risk-high{background:rgba(245,158,11,.15);color:#F59E0B}.risk-critical{background:rgba(239,68,68,.15);color:#EF4444}
.text-danger{color:#EF4444!important;font-weight:600}
.trend-arrow{font-size:16px;font-weight:700}.trend-rising{color:#EF4444}.trend-stable{color:#6B7280}.trend-falling{color:#10B981}

.dt-pagination{display:flex;justify-content:space-between;align-items:center;margin-top:var(--space-3);padding-top:var(--space-2);border-top:1px solid var(--border-subtle);font-size:var(--text-sm);color:var(--text-secondary)}
.dt-page-controls{display:flex;gap:var(--space-2);align-items:center}
.dt-page-num{font-weight:600;color:var(--color-primary);background:var(--color-primary-dim);padding:var(--space-1) var(--space-2);border-radius:var(--radius-sm);font-size:var(--text-xs);min-width:24px;text-align:center}
.dt-page-info{font-size:var(--text-xs);color:var(--text-tertiary)}
</style>