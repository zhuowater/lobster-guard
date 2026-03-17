<template>
  <div class="pie-chart-wrap">
    <div class="pie-chart-container">
      <!-- SVG Donut -->
      <div class="pie-svg-box" :style="{ width: size + 'px', height: size + 'px' }" @mouseleave="hoverIdx = -1">
        <svg :viewBox="`0 0 ${size} ${size}`" :width="size" :height="size">
          <circle v-for="(seg, i) in segments" :key="i"
            :cx="center" :cy="center" :r="radius"
            fill="none"
            :stroke="hoverIdx >= 0 && hoverIdx !== i ? seg.color + '88' : seg.color"
            :stroke-width="hoverIdx === i ? strokeW + 4 : strokeW"
            :stroke-dasharray="seg.dashArray"
            :stroke-dashoffset="seg.dashOffset"
            stroke-linecap="butt"
            style="transition: stroke-width .2s, stroke .2s; cursor: pointer"
            @mouseenter="hoverIdx = i"
            @mousemove="onMouseMove($event, i)"
          />
        </svg>
        <!-- Center label -->
        <div class="pie-center">
          <div class="pie-center-num">{{ total }}</div>
          <div class="pie-center-label">总计</div>
        </div>
      </div>
      <!-- Legend -->
      <div class="pie-legend">
        <div v-for="(item, i) in legendItems" :key="i" class="pie-legend-item"
          :class="{ dimmed: hoverIdx >= 0 && hoverIdx !== i }"
          @mouseenter="hoverIdx = i" @mouseleave="hoverIdx = -1">
          <span class="pie-legend-dot" :style="{ background: item.color }"></span>
          <span class="pie-legend-label">{{ item.label }}</span>
          <span class="pie-legend-value">{{ item.value }}</span>
          <span class="pie-legend-pct">({{ item.pct }}%)</span>
        </div>
      </div>
    </div>
    <!-- Tooltip -->
    <div v-if="hoverIdx >= 0 && tooltipVisible" class="pie-tooltip" :style="tooltipStyle">
      <div class="pie-tooltip-label">{{ legendItems[hoverIdx]?.label }}</div>
      <div class="pie-tooltip-val">
        <b>{{ legendItems[hoverIdx]?.value }}</b>
        <span style="color:var(--text-secondary);margin-left:4px">({{ legendItems[hoverIdx]?.pct }}%)</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  data: { type: Array, default: () => [] }, // [{label, value, color}]
  size: { type: Number, default: 180 },
})

const strokeW = 28
const hoverIdx = ref(-1)
const tooltipVisible = ref(false)
const tooltipX = ref(0)
const tooltipY = ref(0)

const center = computed(() => props.size / 2)
const radius = computed(() => (props.size - strokeW - 8) / 2)
const circumference = computed(() => 2 * Math.PI * radius.value)
const total = computed(() => props.data.reduce((s, d) => s + (d.value || 0), 0))

const segments = computed(() => {
  const t = total.value || 1
  const segs = []
  let offset = 0
  for (const d of props.data) {
    const ratio = d.value / t
    const dashLen = ratio * circumference.value
    segs.push({
      color: d.color || '#666',
      dashArray: `${dashLen} ${circumference.value - dashLen}`,
      dashOffset: -offset,
    })
    offset += dashLen
  }
  return segs
})

const legendItems = computed(() => {
  const t = total.value || 1
  return props.data.map(d => ({
    label: d.label,
    value: d.value || 0,
    color: d.color || '#666',
    pct: ((d.value || 0) / t * 100).toFixed(1),
  }))
})

const tooltipStyle = computed(() => ({
  left: tooltipX.value + 'px',
  top: tooltipY.value + 'px',
}))

function onMouseMove(e, i) {
  hoverIdx.value = i
  tooltipVisible.value = true
  const rect = e.currentTarget.closest('.pie-chart-wrap').getBoundingClientRect()
  tooltipX.value = e.clientX - rect.left + 12
  tooltipY.value = e.clientY - rect.top - 10
}
</script>

<style scoped>
.pie-chart-wrap { position: relative; }
.pie-chart-container { display: flex; align-items: center; gap: 20px; flex-wrap: wrap; }
.pie-svg-box { position: relative; flex-shrink: 0; }
.pie-svg-box svg { transform: rotate(-90deg); }
.pie-center {
  position: absolute; top: 50%; left: 50%; transform: translate(-50%,-50%);
  text-align: center; pointer-events: none;
}
.pie-center-num { font-size: 1.6rem; font-weight: 800; font-family: monospace; color: var(--color-primary); }
.pie-center-label { font-size: .68rem; color: var(--text-secondary); }
.pie-legend { display: flex; flex-direction: column; gap: 6px; min-width: 120px; }
.pie-legend-item {
  display: flex; align-items: center; gap: 6px; font-size: .78rem;
  cursor: pointer; transition: opacity .2s; padding: 2px 4px; border-radius: 4px;
}
.pie-legend-item:hover { background: var(--bg-elevated); }
.pie-legend-item.dimmed { opacity: .4; }
.pie-legend-dot { width: 10px; height: 10px; border-radius: 3px; flex-shrink: 0; }
.pie-legend-label { color: var(--text); flex: 1; }
.pie-legend-value { font-weight: 700; font-family: monospace; color: var(--text); }
.pie-legend-pct { font-size: .7rem; color: var(--text-secondary); }
.pie-tooltip {
  position: absolute; background: var(--bg-overlay); border: 1px solid var(--border-strong);
  border-radius: 6px; padding: 8px 12px; font-size: .78rem; pointer-events: none;
  z-index: 10; box-shadow: 0 4px 16px rgba(0,0,0,.5);
}
.pie-tooltip-label { color: var(--text-secondary); margin-bottom: 2px; }
.pie-tooltip-val { color: var(--text); }
</style>
