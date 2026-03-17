<template>
  <div class="trend-chart-wrap" ref="chartWrap">
    <div v-if="timeRanges && timeRanges.length" class="trend-range-switch">
      <button v-for="tr in timeRanges" :key="tr.value" class="range-btn" :class="{ active: currentRange === tr.value }" @click="$emit('rangeChange', tr.value)">{{ tr.label }}</button>
    </div>
    <svg :viewBox="`0 0 ${svgW} ${svgH}`" :style="{ width: '100%', height: chartHeight + 'px' }" xmlns="http://www.w3.org/2000/svg" @mousemove="onMouseMove" @mouseleave="onMouseLeave">
      <!-- Grid lines (horizontal dashed) -->
      <line v-for="g in gridLines" :key="'g'+g.y" :x1="padL" :y1="g.y" :x2="svgW - padR" :y2="g.y" stroke="rgba(255,255,255,0.06)" stroke-width="0.5" stroke-dasharray="4,4" />
      <!-- Y labels (right-aligned) -->
      <text v-for="g in gridLines" :key="'yl'+g.y" :x="padL - 6" :y="g.y + 3" fill="#64748B" font-size="9" text-anchor="end" font-family="var(--font-mono, monospace)">{{ g.label }}</text>
      <!-- X labels -->
      <text v-for="xl in xTickLabels" :key="'xl'+xl.x" :x="xl.x" :y="svgH - 2" fill="#64748B" font-size="9" text-anchor="middle" font-family="var(--font-mono, monospace)">{{ xl.text }}</text>
      <!-- Area fills (very subtle) -->
      <polygon v-for="line in polylines" :key="'a'+line.key" :points="line.areaPoints" :fill="line.color" opacity="0.06" />
      <!-- Polylines -->
      <polyline v-for="line in polylines" :key="line.key" :points="line.points" fill="none" :stroke="line.color" :stroke-width="line.width || 1.5" stroke-linejoin="round" :opacity="line.opacity || 0.9" />
      <!-- Hover vertical line -->
      <line v-if="hoverIdx >= 0" :x1="hoverX" :y1="padT" :x2="hoverX" :y2="svgH - padB" stroke="rgba(148,163,184,0.2)" stroke-width="1" stroke-dasharray="3,3" />
      <!-- Hover dots -->
      <circle v-for="dot in hoverDots" :key="'d'+dot.key" :cx="dot.x" :cy="dot.y" r="3.5" :fill="dot.color" stroke="var(--bg-base, #0B0E1A)" stroke-width="1.5" />
    </svg>
    <!-- Tooltip -->
    <div v-if="hoverIdx >= 0" class="trend-tooltip" :style="tooltipStyle">
      <div class="trend-tooltip-title">{{ tooltipTitle }}</div>
      <div v-for="item in tooltipItems" :key="item.key" class="trend-tooltip-item">
        <span class="trend-tooltip-dot" :style="{ background: item.color }"></span>
        <span>{{ item.label }}</span>
        <b>{{ item.value }}</b>
      </div>
    </div>
    <!-- Legend -->
    <div class="trend-legend">
      <span v-for="line in lines" :key="line.key" class="trend-legend-item">
        <span class="trend-legend-color" :style="{ background: line.color }"></span>{{ line.label }}
      </span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  data: { type: Array, default: () => [] },
  lines: { type: Array, default: () => [] },
  width: { type: Number, default: 600 },
  height: { type: Number, default: 160 },
  xLabels: { type: Array, default: () => [] },
  timeRanges: { type: Array, default: () => [] },
  currentRange: { type: String, default: '' },
})

const emit = defineEmits(['rangeChange'])

const chartWrap = ref(null)
const containerW = ref(600)
const padL = 44, padR = 16, padT = 12, padB = 24
const svgW = computed(() => Math.max(200, containerW.value))
const svgH = computed(() => props.height)
const chartHeight = computed(() => props.height)
const graphW = computed(() => svgW.value - padL - padR)
const graphH = computed(() => svgH.value - padT - padB)

const maxVal = computed(() => {
  let m = 0
  for (const d of props.data) {
    for (const l of props.lines) {
      const v = d[l.key] ?? 0
      if (v > m) m = v
    }
  }
  return m || 1
})

function niceMax(v) {
  if (v <= 5) return 5
  if (v <= 10) return 10
  const mag = Math.pow(10, Math.floor(Math.log10(v)))
  const norm = v / mag
  if (norm <= 1.5) return 1.5 * mag
  if (norm <= 2) return 2 * mag
  if (norm <= 3) return 3 * mag
  if (norm <= 5) return 5 * mag
  return 10 * mag
}

const yMax = computed(() => niceMax(maxVal.value))

const gridLines = computed(() => {
  const lines = []
  for (let i = 0; i <= 4; i++) {
    const y = padT + graphH.value * i / 4
    const val = yMax.value * (4 - i) / 4
    lines.push({ y, label: val % 1 === 0 ? String(Math.round(val)) : val.toFixed(1) })
  }
  return lines
})

const xTickLabels = computed(() => {
  const n = props.data.length
  if (n === 0) return []
  const labels = props.xLabels.length ? props.xLabels : props.data.map((_, i) => String(i))
  // Show labels at 6-hour intervals if possible, otherwise every ~8 steps
  const step = Math.max(1, Math.floor(n / 6))
  const result = []
  for (let i = 0; i < n; i += step) {
    const x = padL + (i / Math.max(1, n - 1)) * graphW.value
    result.push({ x, text: labels[i] || '' })
  }
  return result
})

function xFor(i) {
  const n = props.data.length
  return padL + (i / Math.max(1, n - 1)) * graphW.value
}
function yFor(v) {
  return padT + graphH.value - (v / yMax.value) * graphH.value
}

const polylines = computed(() => {
  if (!props.data.length || !props.lines.length) return []
  return props.lines.map(l => {
    const pts = props.data.map((d, i) => {
      const x = xFor(i)
      const y = yFor(d[l.key] ?? 0)
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    const areaStart = `${padL.toFixed(1)},${(padT + graphH.value).toFixed(1)}`
    const areaEnd = `${xFor(props.data.length - 1).toFixed(1)},${(padT + graphH.value).toFixed(1)}`
    return {
      key: l.key,
      color: l.color,
      width: l.width || 1.5,
      opacity: l.opacity || 0.9,
      points: pts.join(' '),
      areaPoints: areaStart + ' ' + pts.join(' ') + ' ' + areaEnd,
    }
  })
})

const hoverIdx = ref(-1)
const hoverClientX = ref(0)
const hoverClientY = ref(0)

const hoverX = computed(() => {
  if (hoverIdx.value < 0) return 0
  return xFor(hoverIdx.value)
})

const hoverDots = computed(() => {
  if (hoverIdx.value < 0) return []
  const d = props.data[hoverIdx.value]
  if (!d) return []
  return props.lines.map(l => ({
    key: l.key,
    x: xFor(hoverIdx.value),
    y: yFor(d[l.key] ?? 0),
    color: l.color,
  }))
})

const tooltipTitle = computed(() => {
  if (hoverIdx.value < 0) return ''
  const labels = props.xLabels.length ? props.xLabels : props.data.map((_, i) => String(i))
  return labels[hoverIdx.value] || ''
})

const tooltipItems = computed(() => {
  if (hoverIdx.value < 0) return []
  const d = props.data[hoverIdx.value]
  if (!d) return []
  return props.lines.map(l => ({
    key: l.key,
    label: l.label,
    color: l.color,
    value: d[l.key] ?? 0,
  }))
})

const tooltipStyle = computed(() => {
  if (!chartWrap.value) return { display: 'none' }
  const rect = chartWrap.value.getBoundingClientRect()
  let left = hoverClientX.value - rect.left + 12
  let top = hoverClientY.value - rect.top - 20
  if (left > rect.width - 140) left = left - 160
  if (top < 0) top = 10
  return { left: left + 'px', top: top + 'px' }
})

function onMouseMove(e) {
  if (!chartWrap.value || !props.data.length) return
  const svgRect = e.currentTarget.getBoundingClientRect()
  const relX = e.clientX - svgRect.left
  const scale = svgW.value / svgRect.width
  const svgX = relX * scale
  const n = props.data.length
  let best = 0, bestDist = Infinity
  for (let i = 0; i < n; i++) {
    const d = Math.abs(xFor(i) - svgX)
    if (d < bestDist) { bestDist = d; best = i }
  }
  hoverIdx.value = best
  hoverClientX.value = e.clientX
  hoverClientY.value = e.clientY
}

function onMouseLeave() {
  hoverIdx.value = -1
}

let resizeObs = null
onMounted(() => {
  if (chartWrap.value) {
    containerW.value = chartWrap.value.clientWidth
    resizeObs = new ResizeObserver(entries => {
      for (const entry of entries) {
        containerW.value = entry.contentRect.width
      }
    })
    resizeObs.observe(chartWrap.value)
  }
})
onUnmounted(() => {
  if (resizeObs) resizeObs.disconnect()
})
</script>

<style scoped>
.trend-chart-wrap { position: relative; width: 100%; }
.trend-range-switch { display: flex; gap: var(--space-1); margin-bottom: var(--space-2); }
.range-btn {
  background: transparent; border: 1px solid var(--border-default);
  border-radius: var(--radius-sm); color: var(--text-tertiary); padding: 2px var(--space-3);
  cursor: pointer; font-size: var(--text-xs); transition: all var(--transition-fast);
  font-family: var(--font-sans);
}
.range-btn.active { background: var(--color-primary-dim); color: var(--color-primary); border-color: var(--color-primary); }
.range-btn:hover { color: var(--color-primary); }
.trend-tooltip {
  position: absolute; background: var(--bg-overlay); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); padding: var(--space-2) var(--space-3); font-size: var(--text-xs); pointer-events: none;
  z-index: 10; min-width: 120px; box-shadow: var(--shadow-md);
}
.trend-tooltip-title { color: var(--text-secondary); margin-bottom: var(--space-1); font-weight: 600; }
.trend-tooltip-item { display: flex; align-items: center; gap: var(--space-2); padding: 1px 0; color: var(--text-primary); }
.trend-tooltip-item b { margin-left: auto; font-family: var(--font-mono); }
.trend-tooltip-dot { width: 8px; height: 8px; border-radius: 2px; flex-shrink: 0; }
.trend-legend { display: flex; gap: var(--space-4); margin-top: var(--space-2); font-size: var(--text-xs); color: var(--text-secondary); }
.trend-legend-item { display: flex; align-items: center; gap: var(--space-1); }
.trend-legend-color { display: inline-block; width: 10px; height: 3px; border-radius: 2px; }
</style>
