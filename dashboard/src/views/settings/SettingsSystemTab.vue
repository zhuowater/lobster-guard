<template>
  <div>
    <div class="card" style="margin-bottom:20px">
      <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg></span><span class="card-title">系统信息</span><div class="card-actions"><button class="btn btn-ghost btn-sm" @click="refreshHealth">刷新</button></div></div>
      <Skeleton v-if="!appState.health" type="text" />
      <div v-else>
        <div class="status-grid">
          <div class="ring-chart"><svg width="100" height="100" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="none" stroke="rgba(255,255,255,0.06)" stroke-width="8" /><circle cx="50" cy="50" r="40" fill="none" :stroke="ringColor" stroke-width="8" :stroke-dasharray="C" :stroke-dashoffset="ringOffset" stroke-linecap="round" style="transition:stroke-dashoffset .6s" /></svg><span class="ring-label" :style="{ color: ringColor }">{{ pct }}%</span></div>
          <div class="status-info">
            <div class="status-row"><span class="status-key">总体状态</span><span class="status-val" :style="{ color: statusColor }">{{ statusText }}</span></div>
            <div class="status-row"><span class="status-key">版本</span><span class="status-val">{{ health.version }}</span></div>
            <div class="status-row"><span class="status-key">运行时间</span><span class="status-val">{{ formattedUptime }}</span></div>
            <div class="status-row"><span class="status-key">模式</span><span class="status-val">{{ health.mode || '--' }}</span></div>
            <div class="status-row"><span class="status-key">上游</span><span class="status-val">{{ healthyUp }}/{{ totalUp }}</span></div>
            <div class="status-row"><span class="status-key">路由数</span><span class="status-val">{{ health.routes?.total || 0 }}</span></div>
            <div class="status-row"><span class="status-key">审计日志</span><span class="status-val">{{ health.audit?.total || 0 }}</span></div>
            <div class="status-row"><span class="status-key">限流</span><span class="status-val">{{ rlText }}</span></div>
          </div>
        </div>
      </div>
    </div>
    <div class="card">
      <div class="card-header"><span class="card-icon"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg></span><span class="card-title">健康检查详情</span></div>
      <Skeleton v-if="!appState.health || !appState.health.checks" type="text" />
      <div v-else><div v-for="hc in healthCheckList" :key="hc.name" class="status-row"><span class="status-key">{{ hc.icon }} {{ hc.name }}</span><span class="status-val" :style="{ color: hc.color }">{{ hc.val }}</span></div></div>
    </div>
  </div>
</template>

<script setup>
import Skeleton from '../../components/Skeleton.vue'

defineProps({
  appState: Object,
  refreshHealth: Function,
  C: Number,
  health: Object,
  healthyUp: Number,
  totalUp: Number,
  pct: Number,
  ringColor: String,
  ringOffset: Number,
  statusColor: String,
  statusText: String,
  rlText: String,
  formattedUptime: String,
  healthCheckList: Array,
})
</script>

<style scoped>
.status-grid { display: grid; grid-template-columns: 120px 1fr; gap: 20px; align-items: center; }
.ring-chart { position: relative; width: 100px; height: 100px; }
.ring-label { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; font-weight: 700; }
.status-info { display: flex; flex-direction: column; gap: 8px; }
.status-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 0; border-bottom: 1px solid var(--border-subtle); }
.status-row:last-child { border-bottom: none; }
.status-key { color: var(--text-secondary); font-size: var(--text-sm); }
.status-val { color: var(--text-primary); font-size: var(--text-sm); }
@media (max-width: 640px) {
  .status-grid { grid-template-columns: 1fr; }
}
</style>
