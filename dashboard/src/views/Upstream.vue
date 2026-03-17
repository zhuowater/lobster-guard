<template>
  <div class="card">
    <div class="card-header">
      <span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><circle cx="6" cy="6" r="1" fill="currentColor" stroke="none"/><circle cx="6" cy="18" r="1" fill="currentColor" stroke="none"/></svg></span>
      <span class="card-title">上游管理</span>
      <div class="card-actions">
        <button class="btn btn-sm" @click="loadUpstreams">刷新</button>
      </div>
    </div>
    <DataTable
      :columns="columns"
      :data="upstreams"
      :loading="loading"
      empty-text="暂无上游节点"
      empty-desc="请配置 upstream 或等待服务注册"
     
      :expandable="true"
    >
      <template #empty-hint>请配置 upstream 或等待服务注册</template>
      <template #cell-healthy="{ row }">
        <div class="health-badge" :class="row.healthy ? 'health-ok' : 'health-err'">
          <span class="dot dot-sm" :class="row.healthy ? 'dot-healthy' : 'dot-unhealthy'"></span>
          {{ row.healthy ? '健康' : '异常' }}
        </div>
      </template>
      <template #cell-last_heartbeat="{ row }">
        {{ fmtTime(row.last_heartbeat) }}
      </template>
      <template #expand="{ row }">
        <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:12px 24px;font-size:.82rem">
          <div><span style="color:var(--text-tertiary);font-size:var(--text-xs);display:block;margin-bottom:2px">ID</span><span style="font-weight:500">{{ row.id }}</span></div>
          <div><span style="color:var(--text-tertiary);font-size:var(--text-xs);display:block;margin-bottom:2px">地址</span><span style="font-weight:500;font-family:var(--font-mono)">{{ row.address || row.addr || row.host }}:{{ row.port }}</span></div>
          <div><span style="color:var(--text-tertiary);font-size:var(--text-xs);display:block;margin-bottom:2px">静态</span><span style="font-weight:500">{{ row.static ? '是' : '否' }}</span></div>
          <div><span style="color:var(--text-tertiary);font-size:var(--text-xs);display:block;margin-bottom:2px">用户数</span><span style="font-weight:500;color:var(--color-primary)">{{ row.user_count || 0 }}</span></div>
          <div v-if="row.tags"><span style="color:var(--text-tertiary);font-size:var(--text-xs);display:block;margin-bottom:2px">Tags</span><span style="font-weight:500">{{ JSON.stringify(row.tags) }}</span></div>
          <div v-if="row.load"><span style="color:var(--text-tertiary);font-size:var(--text-xs);display:block;margin-bottom:2px">负载</span><span style="font-weight:500">{{ JSON.stringify(row.load) }}</span></div>
          <div><span style="color:var(--text-tertiary);font-size:var(--text-xs);display:block;margin-bottom:2px">最后心跳</span><span style="font-weight:500">{{ fmtTime(row.last_heartbeat) }}</span></div>
        </div>
      </template>
    </DataTable>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api.js'
import DataTable from '../components/DataTable.vue'

const loading = ref(false)
const upstreams = ref([])

const columns = [
  { key: 'id', label: 'ID', sortable: true },
  { key: 'address', label: '地址', sortable: false },
  { key: 'port', label: '端口', sortable: true },
  { key: 'healthy', label: '状态', sortable: true },
  { key: 'user_count', label: '用户数', sortable: true },
  { key: 'last_heartbeat', label: '最后心跳', sortable: true },
]

function fmtTime(ts) {
  if (!ts) return '--'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? String(ts) : d.toLocaleString('zh-CN', { hour12: false })
}

async function loadUpstreams() {
  loading.value = true
  try {
    const d = await api('/api/v1/upstreams')
    upstreams.value = d.upstreams || []
  } catch { upstreams.value = [] }
  loading.value = false
}

onMounted(loadUpstreams)
</script>

<style scoped>
.health-badge {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 2px 10px; border-radius: var(--radius-sm); font-size: var(--text-xs); font-weight: 500;
}
.health-ok { background: var(--color-success-dim); color: var(--color-success); }
.health-err { background: var(--color-danger-dim); color: var(--color-danger); }
</style>
