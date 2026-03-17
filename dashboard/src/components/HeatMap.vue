<template>
  <div class="heatmap-wrap">
    <div class="heatmap-title">{{ title || '7 天攻击频率热力图' }}</div>
    <div class="heatmap-grid-box">
      <!-- Hour labels (left) -->
      <div class="heatmap-y-labels">
        <div v-for="h in 24" :key="h" class="heatmap-y-label">{{ String(h - 1).padStart(2, '0') }}</div>
      </div>
      <!-- Grid -->
      <div class="heatmap-grid">
        <!-- Day headers -->
        <div class="heatmap-header">
          <div v-for="day in dayLabels" :key="day" class="heatmap-day-label">{{ day }}</div>
        </div>
        <!-- Cells: 24 rows × 7 cols -->
        <div class="heatmap-body">
          <div v-for="hour in 24" :key="hour" class="heatmap-row">
            <div v-for="day in 7" :key="day" class="heatmap-cell"
              :style="{ background: cellColor(hour - 1, day - 1) }"
              @mouseenter="onHover($event, hour - 1, day - 1)"
              @mouseleave="hoverInfo = null">
            </div>
          </div>
        </div>
      </div>
    </div>
    <!-- Color scale -->
    <div class="heatmap-scale">
      <span class="heatmap-scale-label">少</span>
      <div class="heatmap-scale-bar"></div>
      <span class="heatmap-scale-label">多</span>
    </div>
    <!-- Tooltip -->
    <div v-if="hoverInfo" class="heatmap-tooltip" :style="hoverInfo.style">
      <div>{{ hoverInfo.dayName }} {{ String(hoverInfo.hour).padStart(2, '0') }}:00</div>
      <div style="font-weight:700;color:var(--neon-blue)">{{ hoverInfo.count }} 次攻击</div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  data: { type: Array, default: () => [] }, // 7×24 matrix or flat array[168]
  title: { type: String, default: '' },
})

const dayNames = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']

const dayLabels = computed(() => {
  // Show last 7 days with day names
  const labels = []
  const now = new Date()
  for (let d = 6; d >= 0; d--) {
    const date = new Date(now.getTime() - d * 86400000)
    const wd = date.getDay()
    labels.push(dayNames[wd === 0 ? 6 : wd - 1])
  }
  return labels
})

// Get cell value: data can be a flat array[168] (day0h0..day0h23..day6h23) or 2D
function getVal(hour, day) {
  if (!props.data || !props.data.length) return 0
  if (Array.isArray(props.data[0])) {
    // 2D: data[day][hour]
    return props.data[day]?.[hour] ?? 0
  }
  // Flat: index = day * 24 + hour
  return props.data[day * 24 + hour] ?? 0
}

const maxCount = computed(() => {
  let m = 0
  for (let d = 0; d < 7; d++) {
    for (let h = 0; h < 24; h++) {
      const v = getVal(h, d)
      if (v > m) m = v
    }
  }
  return m || 1
})

function cellColor(hour, day) {
  const v = getVal(hour, day)
  if (v === 0) return 'rgba(0,40,80,0.5)'
  const ratio = Math.min(1, v / maxCount.value)
  // Deep blue → cyan → yellow → red
  if (ratio < 0.25) {
    const t = ratio / 0.25
    return interpolate([0, 40, 80], [0, 180, 220], t, 0.5 + t * 0.3)
  } else if (ratio < 0.5) {
    const t = (ratio - 0.25) / 0.25
    return interpolate([0, 180, 220], [100, 220, 100], t, 0.7)
  } else if (ratio < 0.75) {
    const t = (ratio - 0.5) / 0.25
    return interpolate([100, 220, 100], [255, 200, 0], t, 0.8)
  } else {
    const t = (ratio - 0.75) / 0.25
    return interpolate([255, 200, 0], [255, 60, 60], t, 0.9)
  }
}

function interpolate(c1, c2, t, a) {
  const r = Math.round(c1[0] + (c2[0] - c1[0]) * t)
  const g = Math.round(c1[1] + (c2[1] - c1[1]) * t)
  const b = Math.round(c1[2] + (c2[2] - c1[2]) * t)
  return `rgba(${r},${g},${b},${a})`
}

const hoverInfo = ref(null)

function onHover(e, hour, day) {
  const rect = e.currentTarget.closest('.heatmap-wrap').getBoundingClientRect()
  hoverInfo.value = {
    hour,
    dayName: dayLabels.value[day] || '',
    count: getVal(hour, day),
    style: {
      left: (e.clientX - rect.left + 12) + 'px',
      top: (e.clientY - rect.top - 40) + 'px',
    }
  }
}
</script>

<style scoped>
.heatmap-wrap { position: relative; }
.heatmap-title { font-size: .82rem; color: var(--text-dim); margin-bottom: 8px; font-weight: 600; }
.heatmap-grid-box { display: flex; gap: 4px; }
.heatmap-y-labels { display: flex; flex-direction: column; gap: 1px; padding-top: 22px; }
.heatmap-y-label { height: 14px; line-height: 14px; font-size: .6rem; color: var(--text-dim); text-align: right; padding-right: 4px; }
.heatmap-grid { flex: 1; min-width: 0; }
.heatmap-header { display: grid; grid-template-columns: repeat(7, 1fr); gap: 1px; margin-bottom: 2px; }
.heatmap-day-label { text-align: center; font-size: .65rem; color: var(--text-dim); font-weight: 600; }
.heatmap-body { display: flex; flex-direction: column; gap: 1px; }
.heatmap-row { display: grid; grid-template-columns: repeat(7, 1fr); gap: 1px; }
.heatmap-cell {
  height: 14px; border-radius: 2px; cursor: pointer;
  transition: transform .15s, box-shadow .15s;
}
.heatmap-cell:hover { transform: scale(1.3); z-index: 2; box-shadow: 0 0 6px rgba(0,212,255,.5); }
.heatmap-scale { display: flex; align-items: center; gap: 6px; margin-top: 8px; justify-content: center; }
.heatmap-scale-label { font-size: .65rem; color: var(--text-dim); }
.heatmap-scale-bar {
  width: 120px; height: 10px; border-radius: 5px;
  background: linear-gradient(to right, rgba(0,40,80,0.5), rgba(0,180,220,0.7), rgba(100,220,100,0.8), rgba(255,200,0,0.85), rgba(255,60,60,0.9));
}
.heatmap-tooltip {
  position: absolute; background: rgba(10,14,39,.95); border: 1px solid rgba(0,212,255,.3);
  border-radius: 6px; padding: 6px 10px; font-size: .75rem; pointer-events: none;
  z-index: 10; box-shadow: 0 4px 16px rgba(0,0,0,.5); color: var(--text); white-space: nowrap;
}
</style>
