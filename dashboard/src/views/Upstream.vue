<template>
  <div class="card">
    <div class="card-header">
      <span class="card-icon">🔗</span>
      <span class="card-title">上游管理</span>
      <div class="card-actions">
        <button class="btn btn-sm" @click="loadUpstreams">🔄 刷新</button>
      </div>
    </div>
    <DataTable
      :columns="columns"
      :data="upstreams"
      :loading="loading"
      empty-text="暂无上游节点"
      empty-icon="🔗"
      :expandable="true"
    >
      <template #empty-hint>请配置 upstream 或等待服务注册</template>
      <template #cell-healthy="{ row }">
        <span class="dot" :class="row.healthy ? 'dot-healthy' : 'dot-unhealthy'"></span>
        {{ row.healthy ? '健康' : '异常' }}
      </template>
      <template #cell-last_heartbeat="{ row }">
        {{ fmtTime(row.last_heartbeat) }}
      </template>
      <template #expand="{ row }">
        <div style="display:flex;gap:20px;flex-wrap:wrap;font-size:.82rem">
          <div><b style="color:var(--neon-blue)">ID:</b> {{ row.id }}</div>
          <div><b style="color:var(--neon-blue)">地址:</b> {{ row.address || row.addr || row.host }}:{{ row.port }}</div>
          <div><b style="color:var(--neon-blue)">静态:</b> {{ row.static ? '是' : '否' }}</div>
          <div v-if="row.tags"><b style="color:var(--neon-blue)">Tags:</b> {{ JSON.stringify(row.tags) }}</div>
          <div v-if="row.load"><b style="color:var(--neon-blue)">负载:</b> {{ JSON.stringify(row.load) }}</div>
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
