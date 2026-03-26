<template>
  <div class="audit-page">
    <div class="audit-stats-grid" v-if="statsLoaded">
      <StatCard :iconSvg="svgFileText" :value="auditStats.total||0" label="日志总量" color="blue" />
      <StatCard :iconSvg="svgShieldX" :value="auditStats.blocked||0" label="拦截数" color="red" :badge="'今日 +'+(auditStats.today_blocked||0)" />
      <StatCard :iconSvg="svgAlertTriangle" :value="auditStats.warned||0" label="告警数" color="yellow" />
      <StatCard :iconSvg="svgPercent" :value="blockRate" label="拦截率" color="green" />
    </div>
    <div class="audit-stats-grid" v-else><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/><Skeleton type="card"/></div>

    <div class="card">
      <div class="card-header">
        <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg></span>
        <span class="card-title">审计日志</span>
        <div class="card-actions">
          <button class="btn btn-ghost btn-sm" @click="refreshAll" :disabled="loading"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg> 刷新</button>
        </div>
      </div>

      <div style="margin-bottom:var(--space-4)">
        <div class="section-label"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg> 请求趋势</div>
        <TrendChart v-if="timelineData.length" :data="timelineChartData" :lines="timelineLines" :xLabels="timelineXLabels" :height="140" :timeRanges="[{label:'24h',value:'24h'},{label:'7d',value:'7d'}]" :currentRange="timelineRange" @rangeChange="onTimelineRangeChange"/>
        <div v-else style="color:var(--text-tertiary);font-size:var(--text-sm);text-align:center;padding:var(--space-3)">暂无趋势数据</div>
      </div>

      <div class="audit-filters">
        <div class="filter-row">
          <div class="search-box">
            <svg class="search-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
            <input v-model="filters.q" placeholder="搜索内容、原因..." @keyup.enter="applyFilters" class="search-input"/>
            <button v-if="filters.q" class="search-clear" @click="filters.q='';applyFilters()">✕</button>
          </div>
          <div class="time-presets">
            <button v-for="tp in timePresets" :key="tp.value" class="btn btn-sm" :class="activeTimePreset===tp.value?'btn-active':'btn-ghost'" @click="setTimePreset(tp.value)">{{ tp.label }}</button>
          </div>
        </div>
        <div class="filter-row">
          <select v-model="filters.direction" @change="applyFilters" class="filter-select"><option value="">全部方向</option><option value="inbound">🔽 入站</option><option value="outbound">🔼 出站</option></select>
          <select v-model="filters.action" @change="applyFilters" class="filter-select"><option value="">全部动作</option><option value="block">🔴 阻断</option><option value="warn">🟡 告警</option><option value="log">⚪ 记录</option><option value="pass">🟢 放行</option><option value="allow">🟢 允许</option></select>
          <input v-model="filters.sender_id" placeholder="发送者 ID" class="filter-input" @keyup.enter="applyFilters"/>
          <input v-model="filters.trace_id" placeholder="Trace ID" class="filter-input filter-input-mono" @keyup.enter="applyFilters"/>
          <button class="btn btn-sm" @click="applyFilters" :disabled="loading">筛选</button>
          <button v-if="hasActiveFilters" class="btn btn-ghost btn-sm" @click="clearAllFilters">清除</button>
        </div>
        <div class="filter-row" v-if="activeTimePreset==='custom'">
          <div class="date-range-picker">
            <span class="date-label">开始</span><input type="datetime-local" v-model="filters.start_time" class="date-input"/>
            <span class="date-label">至</span><input type="datetime-local" v-model="filters.end_time" class="date-input"/>
            <button class="btn btn-ghost btn-sm" @click="applyFilters">应用</button>
          </div>
        </div>
        <div class="filter-tags" v-if="activeFilterTags.length">
          <span v-for="tag in activeFilterTags" :key="tag.key" class="filter-tag">{{ tag.label }}: {{ tag.value }} <span class="filter-tag-x" @click="removeFilter(tag.key)">✕</span></span>
        </div>
      </div>

      <DataTable :columns="columns" :data="logs" :loading="loading" :page-size="20" :page-sizes="[20,50,100,200]" empty-text="暂无日志记录" empty-desc="调整筛选条件或等待新请求" :expandable="true" :row-class="rowClass" row-key="id">
        <template #toolbar>
          <button class="btn btn-ghost btn-sm" @click="exportAudit('csv')"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg> CSV</button>
          <button class="btn btn-ghost btn-sm" @click="exportAudit('json')">JSON</button>
        </template>
        <template #cell-timestamp="{row}"><span class="time-cell" :title="fullTime(row.timestamp||row.time||row.created_at)">{{ relativeTime(row.timestamp||row.time||row.created_at) }}</span></template>
        <template #cell-direction="{value}"><span class="tag" :class="value==='inbound'?'tag-inbound':'tag-outbound'">{{ value==='inbound'?'入站':value==='outbound'?'出站':(value||'--') }}</span></template>
        <template #cell-action="{value}"><span class="tag" :class="actionTagClass(value)">{{ ({block:'阻断',warn:'告警',pass:'放行',allow:'允许',log:'记录'})[value]||value||'--' }}</span></template>
        <template #cell-sender_id="{row}"><a v-if="row.sender_id" class="link-primary" @click.stop="$router.push('/user-profiles/'+encodeURIComponent(row.sender_id))">{{ row.sender_id }}</a><span v-else class="text-muted">--</span></template>
        <template #cell-trace_id="{row}">
          <span v-if="row.trace_id" class="trace-cell"><a class="trace-link" @click.stop="$router.push('/sessions/'+encodeURIComponent(row.trace_id))" :title="row.trace_id">{{ row.trace_id.substring(0,8) }}…</a><span class="trace-filter" @click.stop="filters.trace_id=row.trace_id;applyFilters()" title="筛选">🔍</span></span>
          <span v-else class="text-muted">--</span>
        </template>
        <template #cell-content_preview="{row}"><span class="content-cell" :title="row.content_preview">{{ (row.content_preview||'--').substring(0,80) }}{{ (row.content_preview||'').length>80?'…':'' }}</span></template>
        <template #cell-latency="{row}"><span :class="latencyClass(row)">{{ row.latency!=null?row.latency+'ms':(row.latency_ms!=null?row.latency_ms+'ms':'--') }}</span></template>
        <template #expand="{row}">
          <div class="expand-detail">
            <div class="detail-grid">
              <div class="detail-section">
                <h4 class="detail-title">基本信息</h4>
                <div class="detail-row"><span class="detail-label">时间</span><span>{{ fullTime(row.timestamp||row.time) }}</span></div>
                <div class="detail-row"><span class="detail-label">方向</span><span class="tag" :class="row.direction==='inbound'?'tag-inbound':'tag-outbound'">{{ row.direction==='inbound'?'入站':'出站' }}</span></div>
                <div class="detail-row"><span class="detail-label">动作</span><span class="tag" :class="actionTagClass(row.action)">{{ ({block:'阻断',warn:'告警',pass:'放行',allow:'允许',log:'记录'})[row.action]||row.action }}</span></div>
                <div class="detail-row"><span class="detail-label">延迟</span><span :class="latencyClass(row)">{{ row.latency||row.latency_ms||'--' }}ms</span></div>
              </div>
              <div class="detail-section">
                <h4 class="detail-title">关联信息</h4>
                <div class="detail-row"><span class="detail-label">发送者</span><a v-if="row.sender_id" class="link-primary" @click.stop="$router.push('/user-profiles/'+encodeURIComponent(row.sender_id))">{{ row.sender_id }} ↗</a><span v-else class="text-muted">--</span></div>
                <div class="detail-row"><span class="detail-label">App ID</span><span class="mono">{{ row.app_id||'--' }}</span></div>
                <div class="detail-row"><span class="detail-label">Trace ID</span><a v-if="row.trace_id" class="link-primary mono" @click.stop="$router.push('/sessions/'+encodeURIComponent(row.trace_id))">{{ row.trace_id }} ▶</a><span v-else class="text-muted">--</span></div>
                <div class="detail-row"><span class="detail-label">上游</span><span class="mono">{{ row.upstream_id||'--' }}</span></div>
              </div>
            </div>
            <div class="detail-section" v-if="row.reason"><h4 class="detail-title">匹配规则 / 原因</h4><div class="detail-reason">{{ row.reason }} <a class="link-primary" style="margin-left:8px" @click.stop="$router.push('/rules')">查看规则→</a></div></div>
            <div class="detail-section" v-if="row.content_preview"><h4 class="detail-title">请求内容</h4><div class="detail-content"><JsonHighlight :content="row.content_preview"/></div></div>
          </div>
        </template>
      </DataTable>

      <div class="audit-actions">
        <div class="action-group">
          <button class="btn btn-ghost btn-sm" @click="toggleStats" :class="{'btn-active':showStatsPanel}"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg> 详细统计</button>
          <button class="btn btn-ghost btn-sm" @click="toggleArchives" :class="{'btn-active':showArchivePanel}"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="21 8 21 21 3 21 3 8"/><rect x="1" y="3" width="22" height="5"/><line x1="10" y1="12" x2="14" y2="12"/></svg> 归档管理</button>
        </div>
        <div class="action-group">
          <button class="btn btn-ghost btn-sm" @click="confirmArchive">手动归档</button>
          <button class="btn btn-danger btn-sm" @click="confirmCleanup">清理过期</button>
        </div>
      </div>

      <div v-if="showStatsPanel" class="panel-section">
        <div class="panel-header"><span>📊 详细统计</span><button class="btn btn-ghost btn-sm" @click="showStatsPanel=false">收起</button></div>
        <div class="stats-detail-grid">
          <div class="stats-item"><span class="stats-item-label">日志总量</span><span class="stats-item-value">{{ detailStats.total??'--' }}</span></div>
          <div class="stats-item"><span class="stats-item-label">最早记录</span><span class="stats-item-value">{{ detailStats.earliest?fullTime(detailStats.earliest):'--' }}</span></div>
          <div class="stats-item"><span class="stats-item-label">最晚记录</span><span class="stats-item-value">{{ detailStats.latest?fullTime(detailStats.latest):'--' }}</span></div>
          <div class="stats-item"><span class="stats-item-label">磁盘占用</span><span class="stats-item-value">{{ formatSize(detailStats.disk_bytes) }}</span></div>
        </div>
      </div>

      <div v-if="showArchivePanel" class="panel-section">
        <div class="panel-header"><span>📦 归档 ({{ archives.length }})</span><button class="btn btn-ghost btn-sm" @click="showArchivePanel=false">收起</button></div>
        <div v-if="archives.length" class="table-wrap">
          <table><thead><tr><th>文件名</th><th>大小</th><th>时间</th><th>操作</th></tr></thead>
          <tbody><tr v-for="a in archives" :key="a.name">
            <td class="mono" style="font-size:var(--text-xs)">{{ a.name }}</td><td>{{ formatSize(a.size) }}</td><td>{{ fullTime(a.mod_time) }}</td>
            <td><div style="display:flex;gap:var(--space-1)"><a :href="'/api/v1/audit/archives/'+encodeURIComponent(a.name)" class="btn btn-ghost btn-sm" style="text-decoration:none" target="_blank">下载</a><button class="btn btn-danger btn-sm" @click="confirmDeleteArchive(a.name)">删除</button></div></td>
          </tr></tbody></table>
        </div>
        <EmptyState v-else title="暂无归档" description="归档过期日志后将在此显示"/>
      </div>
    </div>
    <ConfirmModal :visible="confirmVisible" :title="confirmTitle" :message="confirmMessage" :type="confirmType" @confirm="doConfirm" @cancel="confirmVisible=false"/>
  </div>
</template>
<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, apiPost, apiDelete, downloadFile } from '../api.js'
import { showToast } from '../stores/app.js'
import DataTable from '../components/DataTable.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import StatCard from '../components/StatCard.vue'
import TrendChart from '../components/TrendChart.vue'
import JsonHighlight from '../components/JsonHighlight.vue'
import Skeleton from '../components/Skeleton.vue'
import EmptyState from '../components/EmptyState.vue'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const logs = ref([])
const timelineData = ref([])
const timelineRange = ref('24h')
const statsLoaded = ref(false)
const auditStats = ref({})
const detailStats = ref({})
const archives = ref([])
const selectedIds = ref([])
const showStatsPanel = ref(false)
const showArchivePanel = ref(false)
const activeTimePreset = ref('')
const confirmVisible = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmType = ref('warning')
let confirmAction = null

const filters = reactive({ direction:'', action:'', sender_id:'', app_id:'', trace_id:'', q:'', start_time:'', end_time:'' })

const timePresets = [{ label:'全部', value:'' },{ label:'今天', value:'today' },{ label:'7天', value:'7d' },{ label:'30天', value:'30d' },{ label:'自定义', value:'custom' }]

const svgFileText = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>'
const svgShieldX = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><line x1="9" y1="9" x2="15" y2="15"/><line x1="15" y1="9" x2="9" y2="15"/></svg>'
const svgAlertTriangle = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
const svgPercent = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="5" x2="5" y2="19"/><circle cx="6.5" cy="6.5" r="2.5"/><circle cx="17.5" cy="17.5" r="2.5"/></svg>'

const columns = [
  { key:'timestamp', label:'时间', sortable:true, width:'150px' },
  { key:'direction', label:'方向', sortable:true, width:'70px' },
  { key:'sender_id', label:'发送者', sortable:true },
  { key:'action', label:'动作', sortable:true, width:'80px' },
  { key:'trace_id', label:'Trace ID', sortable:false, width:'120px' },
  { key:'content_preview', label:'内容', sortable:false },
  { key:'reason', label:'原因', sortable:true },
  { key:'latency', label:'延迟', sortable:true, width:'70px' },
]

const blockRate = computed(() => { const t=auditStats.value.total||0,b=auditStats.value.blocked||0; return t===0?'0%':((b/t)*100).toFixed(1)+'%' })
const hasActiveFilters = computed(() => !!(filters.direction||filters.action||filters.sender_id||filters.trace_id||filters.q||filters.start_time||filters.end_time))
const activeFilterTags = computed(() => {
  const tags = []
  if (filters.direction) tags.push({ key:'direction', label:'方向', value:filters.direction==='inbound'?'入站':'出站' })
  if (filters.action) tags.push({ key:'action', label:'动作', value:filters.action })
  if (filters.sender_id) tags.push({ key:'sender_id', label:'发送者', value:filters.sender_id })
  if (filters.trace_id) tags.push({ key:'trace_id', label:'Trace', value:filters.trace_id.substring(0,12) })
  if (filters.q) tags.push({ key:'q', label:'搜索', value:filters.q })
  if (activeTimePreset.value && activeTimePreset.value!=='custom') tags.push({ key:'time', label:'时间', value:activeTimePreset.value })
  return tags
})

const timelineChartData = computed(() => timelineData.value.map(t => ({ pass:t.pass||0, block:t.block||0, warn:t.warn||0 })))
const timelineLines = [{ key:'pass', color:'#10B981', label:'放行' },{ key:'block', color:'#EF4444', label:'阻断' },{ key:'warn', color:'#F59E0B', label:'告警' }]
const timelineXLabels = computed(() => timelineData.value.map(t => { const h=t.hour||''; if (timelineRange.value==='7d') return h.substring(5,10); const hp=h.substring(11,13); return hp?hp+':00':'' }))

function fullTime(ts) { if (!ts) return '--'; const d=new Date(ts); return isNaN(d.getTime())?String(ts):d.toLocaleString('zh-CN',{hour12:false}) }
function relativeTime(ts) {
  if (!ts) return '--'; const d=new Date(ts); if (isNaN(d.getTime())) return String(ts)
  const sec=Math.floor((Date.now()-d.getTime())/1000); if (sec<0) return fullTime(ts)
  if (sec<60) return sec+'秒前'; const min=Math.floor(sec/60); if (min<60) return min+'分钟前'
  const hr=Math.floor(min/60); if (hr<24) return hr+'小时前'; const day=Math.floor(hr/24); if (day<7) return day+'天前'; return fullTime(ts)
}
function actionTagClass(a) { a=(a||'').toLowerCase(); return a==='block'?'tag-block':a==='warn'?'tag-warn':a==='log'?'tag-log':(a==='pass'||a==='allow')?'tag-pass':'tag-log' }
function rowClass(row) { const a=(row.action||'').toLowerCase(); return a==='block'?'row-block':a==='warn'?'row-warn':'' }
function latencyClass(row) { const ms=row.latency||row.latency_ms||0; return ms>1000?'latency-high':ms>300?'latency-mid':'' }
function formatSize(bytes) { if (!bytes) return '--'; const kb=Math.round(bytes/1024); return kb>1024?(kb/1024).toFixed(1)+' MB':kb+' KB' }

function setTimePreset(val) {
  activeTimePreset.value=val; if (val==='custom') return
  if (val==='') { filters.start_time=''; filters.end_time=''; applyFilters(); return }
  const now=new Date(); let from
  if (val==='today') from=new Date(now.getFullYear(),now.getMonth(),now.getDate())
  else if (val==='7d') from=new Date(now.getTime()-7*24*3600000)
  else if (val==='30d') from=new Date(now.getTime()-30*24*3600000)
  filters.start_time=from?from.toISOString().slice(0,16):''; filters.end_time=''; applyFilters()
}
function removeFilter(key) {
  if (key==='time') { activeTimePreset.value=''; filters.start_time=''; filters.end_time='' }
  else filters[key]=''
  applyFilters()
}
function clearAllFilters() { filters.direction='';filters.action='';filters.sender_id='';filters.trace_id='';filters.q='';filters.start_time='';filters.end_time='';activeTimePreset.value='';applyFilters() }

function syncFiltersToURL() {
  const q={}
  if (filters.direction) q.direction=filters.direction; if (filters.action) q.action=filters.action
  if (filters.sender_id) q.sender_id=filters.sender_id; if (filters.trace_id) q.trace_id=filters.trace_id
  if (filters.q) q.q=filters.q; if (filters.start_time) q.from=filters.start_time; if (filters.end_time) q.to=filters.end_time
  if (activeTimePreset.value) q.preset=activeTimePreset.value
  router.replace({ query:q }).catch(()=>{})
}
function loadFiltersFromURL() {
  const q=route.query
  if (q.direction) filters.direction=q.direction; if (q.action) filters.action=q.action
  if (q.sender_id) filters.sender_id=q.sender_id; if (q.trace_id) filters.trace_id=q.trace_id
  if (q.q) filters.q=q.q; if (q.from) filters.start_time=q.from; if (q.to) filters.end_time=q.to
  if (q.preset) activeTimePreset.value=q.preset
  if (q.since) { const m={'1h':1,'24h':24,'7d':168,'30d':720}; const h=m[q.since]; if(h){filters.start_time=new Date(Date.now()-h*3600000).toISOString().slice(0,16);timelineRange.value=h>24?'7d':'24h'} }
}
function applyFilters() { syncFiltersToURL(); loadLogs() }
function onTimelineRangeChange(range) { timelineRange.value=range; loadTimeline() }

async function loadLogs() {
  loading.value=true
  const p=[]; if (filters.direction) p.push('direction='+encodeURIComponent(filters.direction))
  if (filters.action) p.push('action='+encodeURIComponent(filters.action))
  if (filters.sender_id) p.push('sender_id='+encodeURIComponent(filters.sender_id))
  if (filters.app_id) p.push('app_id='+encodeURIComponent(filters.app_id))
  if (filters.trace_id) p.push('trace_id='+encodeURIComponent(filters.trace_id))
  if (filters.q) p.push('q='+encodeURIComponent(filters.q))
  if (filters.start_time) p.push('from='+encodeURIComponent(new Date(filters.start_time).toISOString()))
  if (filters.end_time) p.push('to='+encodeURIComponent(new Date(filters.end_time).toISOString()))
  p.push('limit=200')
  try { const d=await api('/api/v1/audit/logs?'+p.join('&')); logs.value=d.logs||[] } catch { logs.value=[] }
  loading.value=false
}

async function loadTimeline() {
  try { const hours=timelineRange.value==='7d'?168:24; const d=await api('/api/v1/audit/timeline?hours='+hours); timelineData.value=d.timeline||[] } catch { timelineData.value=[] }
}

async function loadAuditStats() {
  try {
    const d=await api('/api/v1/audit/stats'); detailStats.value=d
    auditStats.value.total=d.total||0
    // Compute blocked/warned from logs or stats
    let blocked=0, warned=0, todayBlocked=0
    const allLogs=logs.value
    for (const l of allLogs) { const a=(l.action||'').toLowerCase(); if(a==='block'){blocked++;} if(a==='warn') warned++ }
    auditStats.value.blocked=blocked; auditStats.value.warned=warned; auditStats.value.today_blocked=todayBlocked
    statsLoaded.value=true
  } catch (e) { showToast('获取统计失败: '+e.message,'error'); statsLoaded.value=true }
}

async function exportAudit(fmt) {
  showToast('正在导出 '+fmt.toUpperCase()+'...','success')
  const p=['format='+fmt,'limit=10000']
  if (filters.direction) p.push('direction='+encodeURIComponent(filters.direction))
  if (filters.action) p.push('action='+encodeURIComponent(filters.action))
  if (filters.sender_id) p.push('sender_id='+encodeURIComponent(filters.sender_id))
  if (filters.q) p.push('q='+encodeURIComponent(filters.q))
  if (filters.start_time) p.push('from='+encodeURIComponent(new Date(filters.start_time).toISOString()))
  if (filters.end_time) p.push('to='+encodeURIComponent(new Date(filters.end_time).toISOString()))
  const url=location.origin+'/api/v1/audit/export?'+p.join('&')
  try { await downloadFile(url,'audit_logs.'+fmt); showToast('导出 '+fmt.toUpperCase()+' 成功','success') } catch(e) { showToast('导出失败: '+e.message,'error') }
}

async function loadArchives() {
  try { const d=await api('/api/v1/audit/archives'); archives.value=d.archives||[] } catch(e) { showToast('加载归档失败: '+e.message,'error') }
}

function toggleStats() { showStatsPanel.value=!showStatsPanel.value; if (showStatsPanel.value) loadAuditStats() }
function toggleArchives() { showArchivePanel.value=!showArchivePanel.value; if (showArchivePanel.value) loadArchives() }

function confirmCleanup() {
  confirmTitle.value='清理过期日志'; confirmMessage.value='确认清理过期日志？此操作将删除超过保留天数的日志记录，不可撤销。'
  confirmType.value='danger'; confirmAction=async()=>{
    try { const d=await apiPost('/api/v1/audit/cleanup',{}); showToast('清理完成：删除 '+d.deleted+' 条','success'); loadLogs(); loadAuditStats() } catch(e) { showToast('清理失败: '+e.message,'error') }
  }; confirmVisible.value=true
}

function confirmArchive() {
  confirmTitle.value='手动归档'; confirmMessage.value='确认手动归档过期日志？归档后日志将被压缩保存。'
  confirmType.value='warning'; confirmAction=async()=>{
    try { const d=await apiPost('/api/v1/audit/archive',{})
      if (d.status==='no_data') { showToast('没有需要归档的日志','success'); return }
      showToast('归档完成：已归档 '+d.deleted+' 条','success'); loadLogs(); loadAuditStats(); loadArchives()
    } catch(e) { showToast('归档失败: '+e.message,'error') }
  }; confirmVisible.value=true
}

function confirmDeleteArchive(name) {
  confirmTitle.value='删除归档'; confirmMessage.value='确认删除归档文件 '+name+'？此操作不可撤销。'
  confirmType.value='danger'; confirmAction=async()=>{
    try { await apiDelete('/api/v1/audit/archives/'+encodeURIComponent(name)); showToast('归档已删除','success'); loadArchives() } catch(e) { showToast('删除失败: '+e.message,'error') }
  }; confirmVisible.value=true
}

function batchExport(fmt) { exportAudit(fmt) }
function confirmBatchDelete() {
  confirmTitle.value='批量删除'; confirmMessage.value='确认删除选中的 '+selectedIds.value.length+' 条日志？此操作不可撤销。'
  confirmType.value='danger'; confirmAction=async()=>{
    showToast('批量删除功能暂未实现','error'); selectedIds.value=[]
  }; confirmVisible.value=true
}

function doConfirm() { confirmVisible.value=false; if (confirmAction) confirmAction() }

function refreshAll() { loadLogs(); loadTimeline(); loadAuditStats() }

let refreshTimer=null
onMounted(() => { loadFiltersFromURL(); loadLogs(); loadTimeline(); loadAuditStats(); refreshTimer=setInterval(()=>{loadLogs();loadTimeline()},30000) })
onUnmounted(() => clearInterval(refreshTimer))
</script>
<style scoped>
.audit-page { display:flex; flex-direction:column; gap:var(--space-4); }
.audit-stats-grid { display:grid; grid-template-columns:repeat(4,1fr); gap:var(--space-3); }
@media (max-width:900px) { .audit-stats-grid { grid-template-columns:repeat(2,1fr); } }
.section-label { font-size:var(--text-sm); color:var(--text-secondary); margin-bottom:var(--space-2); font-weight:500; display:flex; align-items:center; gap:6px; }

/* Filters */
.audit-filters { margin-bottom:var(--space-3); }
.filter-row { display:flex; align-items:center; gap:var(--space-2); flex-wrap:wrap; margin-bottom:var(--space-2); }
.search-box { position:relative; flex:1; min-width:200px; max-width:400px; }
.search-icon { position:absolute; left:10px; top:50%; transform:translateY(-50%); color:var(--text-tertiary); pointer-events:none; }
.search-input { width:100%; padding:var(--space-2) var(--space-3) var(--space-2) 32px; background:var(--bg-elevated); border:1px solid var(--border-default); border-radius:var(--radius-md); color:var(--text-primary); font-size:var(--text-sm); outline:none; font-family:var(--font-sans); transition:border-color var(--transition-fast); }
.search-input:focus { border-color:var(--color-primary); }
.search-clear { position:absolute; right:8px; top:50%; transform:translateY(-50%); background:none; border:none; color:var(--text-tertiary); cursor:pointer; font-size:var(--text-sm); padding:2px 4px; }
.search-clear:hover { color:var(--text-primary); }
.time-presets { display:flex; gap:var(--space-1); }
.filter-select, .filter-input { background:var(--bg-elevated); border:1px solid var(--border-default); border-radius:var(--radius-md); color:var(--text-primary); padding:var(--space-2) var(--space-3); font-size:var(--text-sm); outline:none; font-family:var(--font-sans); transition:border-color var(--transition-fast); }
.filter-select:focus, .filter-input:focus { border-color:var(--color-primary); }
.filter-select option { background:var(--bg-elevated); }
.filter-input-mono { font-family:var(--font-mono); font-size:var(--text-xs); }
.date-range-picker { display:flex; align-items:center; gap:var(--space-2); flex-wrap:wrap; background:var(--bg-elevated); border-radius:var(--radius-md); padding:var(--space-1) var(--space-2); border:1px solid var(--border-subtle); }
.date-label { font-size:var(--text-xs); color:var(--text-tertiary); white-space:nowrap; }
.date-input { background:var(--bg-base); border:1px solid var(--border-default); border-radius:var(--radius-sm); color:var(--text-primary); padding:var(--space-1) var(--space-2); font-size:var(--text-xs); outline:none; color-scheme:dark; font-family:var(--font-sans); }
.date-input:focus { border-color:var(--color-primary); }

/* Filter tags */
.filter-tags { display:flex; gap:var(--space-1); flex-wrap:wrap; margin-top:var(--space-1); }
.filter-tag { display:inline-flex; align-items:center; gap:4px; padding:2px 8px; background:var(--color-primary-dim, rgba(99,102,241,0.15)); color:var(--color-primary); border-radius:9999px; font-size:var(--text-xs); font-weight:500; }
.filter-tag-x { cursor:pointer; opacity:0.6; font-size:10px; }
.filter-tag-x:hover { opacity:1; }

/* Active button */
.btn-active { background:var(--color-primary-dim, rgba(99,102,241,0.15)) !important; color:var(--color-primary) !important; border-color:var(--color-primary) !important; }

/* Tags */
.tag { display:inline-block; padding:1px 8px; border-radius:9999px; font-size:var(--text-xs); font-weight:600; white-space:nowrap; line-height:1.6; }
.tag-block { background:rgba(239,68,68,0.15); color:#EF4444; }
.tag-warn { background:rgba(245,158,11,0.15); color:#F59E0B; }
.tag-log { background:rgba(107,114,128,0.15); color:#9CA3AF; }
.tag-pass { background:rgba(16,185,129,0.15); color:#10B981; }
.tag-inbound { background:rgba(59,130,246,0.15); color:#3B82F6; }
.tag-outbound { background:rgba(139,92,246,0.15); color:#8B5CF6; }

/* Row highlights */
:deep(.row-block) { background:rgba(239,68,68,0.04) !important; }
:deep(.row-block:hover) { background:rgba(239,68,68,0.08) !important; }
:deep(.row-warn) { background:rgba(245,158,11,0.04) !important; }
:deep(.row-warn:hover) { background:rgba(245,158,11,0.08) !important; }

/* Links */
.link-primary { color:var(--color-primary); cursor:pointer; text-decoration:none; font-weight:500; }
.link-primary:hover { text-decoration:underline; }
.text-muted { color:var(--text-tertiary); font-size:var(--text-xs); }
.mono { font-family:var(--font-mono); font-size:var(--text-xs); }

/* Time cell */
.time-cell { font-size:var(--text-xs); color:var(--text-secondary); cursor:default; }

/* Trace */
.trace-cell { display:inline-flex; align-items:center; gap:4px; font-size:var(--text-xs); }
.trace-link { font-family:var(--font-mono); color:var(--color-primary); cursor:pointer; text-decoration:none; }
.trace-link:hover { text-decoration:underline; }
.trace-filter { cursor:pointer; opacity:0.4; font-size:10px; }
.trace-filter:hover { opacity:1; }

/* Content */
.content-cell { max-width:300px; overflow:hidden; text-overflow:ellipsis; display:inline-block; font-size:var(--text-xs); }

/* Latency */
.latency-high { color:#EF4444; font-weight:600; }
.latency-mid { color:#F59E0B; }

/* Expand detail */
.expand-detail { font-size:var(--text-sm); line-height:1.8; background:var(--bg-elevated); padding:var(--space-4); border-radius:var(--radius-md); }
.detail-grid { display:grid; grid-template-columns:1fr 1fr; gap:var(--space-4); margin-bottom:var(--space-3); }
@media (max-width:700px) { .detail-grid { grid-template-columns:1fr; } }
.detail-section { margin-bottom:var(--space-2); }
.detail-title { font-size:var(--text-xs); font-weight:600; color:var(--color-primary); text-transform:uppercase; letter-spacing:0.05em; margin-bottom:var(--space-2); padding-bottom:var(--space-1); border-bottom:1px solid var(--border-subtle); }
.detail-row { display:flex; gap:var(--space-2); padding:var(--space-1) 0; align-items:center; }
.detail-label { font-size:var(--text-xs); color:var(--text-tertiary); min-width:60px; flex-shrink:0; }
.detail-reason { background:var(--bg-base); padding:var(--space-2) var(--space-3); border-radius:var(--radius-sm); border-left:3px solid var(--color-warning); font-size:var(--text-sm); }
.detail-content { background:var(--bg-base); padding:var(--space-3); border-radius:var(--radius-sm); overflow-x:auto; max-height:300px; overflow-y:auto; }

/* Bottom actions */
.audit-actions { margin-top:var(--space-3); display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:var(--space-2); }
.action-group { display:flex; gap:var(--space-2); }

/* Panels */
.panel-section { margin-top:var(--space-3); padding:var(--space-3); background:var(--bg-elevated); border-radius:var(--radius-md); border:1px solid var(--border-subtle); }
.panel-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:var(--space-3); font-size:var(--text-sm); font-weight:600; }
.stats-detail-grid { display:grid; grid-template-columns:repeat(4,1fr); gap:var(--space-3); }
@media (max-width:700px) { .stats-detail-grid { grid-template-columns:repeat(2,1fr); } }
.stats-item { display:flex; flex-direction:column; gap:var(--space-1); }
.stats-item-label { font-size:var(--text-xs); color:var(--text-tertiary); }
.stats-item-value { font-size:var(--text-sm); font-weight:600; color:var(--text-primary); font-family:var(--font-mono); }

/* Batch bar */
.batch-bar { display:flex; align-items:center; gap:var(--space-2); padding:var(--space-2) var(--space-3); background:var(--color-primary-dim, rgba(99,102,241,0.1)); border-radius:var(--radius-md); margin-bottom:var(--space-2); }
.batch-count { font-size:var(--text-sm); font-weight:600; color:var(--color-primary); }
</style>
