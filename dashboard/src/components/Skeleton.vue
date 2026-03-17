<template>
  <div class="skeleton" :class="'skeleton--' + type">
    <template v-if="type === 'card'">
      <div class="sk-card">
        <div class="sk-line sk-line-sm" style="width:40%"></div>
        <div class="sk-line sk-line-xl" style="width:60%;margin-top:12px"></div>
        <div class="sk-line sk-line-xs" style="width:30%;margin-top:8px"></div>
      </div>
    </template>
    <template v-else-if="type === 'table'">
      <div class="sk-table">
        <div class="sk-table-header">
          <div class="sk-line sk-line-sm" style="width:15%"></div>
          <div class="sk-line sk-line-sm" style="width:10%"></div>
          <div class="sk-line sk-line-sm" style="width:12%"></div>
          <div class="sk-line sk-line-sm" style="width:20%"></div>
          <div class="sk-line sk-line-sm" style="width:25%"></div>
        </div>
        <div class="sk-table-row" v-for="i in 5" :key="i">
          <div class="sk-line sk-line-sm" :style="{ width: (40 + Math.sin(i) * 20) + '%' }"></div>
          <div class="sk-line sk-line-sm" style="width:8%"></div>
          <div class="sk-line sk-line-sm" :style="{ width: (10 + i * 3) + '%' }"></div>
          <div class="sk-line sk-line-sm" :style="{ width: (15 + i * 5) + '%' }"></div>
          <div class="sk-line sk-line-sm" :style="{ width: (20 + Math.cos(i) * 10) + '%' }"></div>
        </div>
      </div>
    </template>
    <template v-else-if="type === 'chart'">
      <div class="sk-chart">
        <div class="sk-chart-y">
          <div class="sk-line sk-line-xs" style="width:100%" v-for="i in 4" :key="i"></div>
        </div>
        <div class="sk-chart-area">
          <div class="sk-line" style="width:100%;height:100%;border-radius:var(--radius-md)"></div>
        </div>
      </div>
    </template>
    <template v-else>
      <div class="sk-text">
        <div class="sk-line sk-line-sm" style="width:90%"></div>
        <div class="sk-line sk-line-sm" style="width:75%"></div>
        <div class="sk-line sk-line-sm" style="width:60%"></div>
      </div>
    </template>
  </div>
</template>

<script setup>
defineProps({
  type: { type: String, default: 'text', validator: v => ['card', 'table', 'chart', 'text'].includes(v) },
})
</script>

<style scoped>
.skeleton { padding: var(--space-2); }

.sk-line {
  background: linear-gradient(90deg, var(--bg-elevated) 25%, rgba(255,255,255,0.06) 50%, var(--bg-elevated) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
  border-radius: var(--radius-sm);
  height: 12px;
}
.sk-line-xs { height: 8px; }
.sk-line-sm { height: 12px; }
.sk-line-xl { height: 28px; }

@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

/* Card skeleton */
.sk-card {
  padding: var(--space-4);
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
}

/* Table skeleton */
.sk-table { display: flex; flex-direction: column; gap: var(--space-2); }
.sk-table-header {
  display: flex; gap: var(--space-3); padding: var(--space-2) 0;
  border-bottom: 1px solid var(--border-subtle);
}
.sk-table-row {
  display: flex; gap: var(--space-3); padding: var(--space-2) 0;
  border-bottom: 1px solid var(--border-subtle);
}

/* Chart skeleton */
.sk-chart { display: flex; gap: var(--space-2); height: 140px; }
.sk-chart-y {
  width: 32px; display: flex; flex-direction: column; justify-content: space-between;
  padding: var(--space-1) 0;
}
.sk-chart-area { flex: 1; }

/* Text skeleton */
.sk-text { display: flex; flex-direction: column; gap: var(--space-3); padding: var(--space-3) 0; }
</style>
