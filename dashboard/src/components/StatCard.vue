<template>
  <div class="stat-card" :class="'stat-card--' + color" @mouseenter="hovered = true" @mouseleave="hovered = false">
    <div class="stat-card-top">
      <span class="stat-card-icon" v-html="iconSvg"></span>
      <span class="stat-card-label">{{ label }}</span>
    </div>
    <div class="stat-card-value">{{ displayText }}</div>
    <div v-if="change" class="stat-card-change" :class="changeUp ? 'change-up' : 'change-down'">
      <svg v-if="changeUp" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="18 15 12 9 6 15"/></svg>
      <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="6 9 12 15 18 9"/></svg>
      {{ change }}
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'

const props = defineProps({
  icon: { type: String, default: '' },
  iconSvg: { type: String, default: '' },
  value: { type: [String, Number], default: '--' },
  label: { type: String, default: '' },
  color: { type: String, default: 'blue' },
  change: { type: String, default: '' },
  changeUp: { type: Boolean, default: true },
})

const hovered = ref(false)
const animatedValue = ref(0)
const isAnimating = ref(false)

// Detect if value has a suffix (like % or ms)
const suffix = computed(() => {
  const v = String(props.value)
  const m = v.match(/[^\d.]+$/)
  return m ? m[0] : ''
})

const isNumeric = computed(() => {
  const v = String(props.value)
  const numPart = v.replace(/[^\d.]/g, '')
  return numPart !== '' && !isNaN(parseFloat(numPart)) && v !== '--'
})

const targetNum = computed(() => {
  if (!isNumeric.value) return 0
  return parseFloat(String(props.value).replace(/[^\d.]/g, '')) || 0
})

const isPercent = computed(() => suffix.value.includes('%'))

const displayText = computed(() => {
  if (!isNumeric.value) return props.value
  if (isPercent.value) {
    return animatedValue.value.toFixed(1) + suffix.value
  }
  if (suffix.value) {
    // For ms or other suffixed values
    if (String(props.value).includes('.')) {
      return animatedValue.value.toFixed(1) + suffix.value
    }
    return Math.round(animatedValue.value) + suffix.value
  }
  return Math.round(animatedValue.value)
})

watch(() => props.value, (newVal) => {
  if (!isNumeric.value) return
  const end = targetNum.value
  const start = animatedValue.value
  const duration = 800
  const startTime = performance.now()

  function animate(now) {
    const progress = Math.min((now - startTime) / duration, 1)
    const eased = 1 - Math.pow(1 - progress, 3) // ease-out cubic
    animatedValue.value = start + (end - start) * eased
    if (progress < 1) {
      requestAnimationFrame(animate)
    }
  }
  requestAnimationFrame(animate)
}, { immediate: true })
</script>

<style scoped>
.stat-card {
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-4) var(--space-5);
  position: relative;
  overflow: hidden;
  transition: transform var(--transition-fast), box-shadow var(--transition-fast), border-color var(--transition-fast);
  cursor: default;
}
.stat-card::before {
  content: ''; position: absolute; left: 0; top: 0; bottom: 0; width: 3px;
}
.stat-card--blue::before { background: var(--color-primary); }
.stat-card--red::before { background: var(--color-danger); }
.stat-card--yellow::before { background: var(--color-warning); }
.stat-card--green::before { background: var(--color-success); }

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
  border-color: var(--border-default);
}

.stat-card-top {
  display: flex; align-items: center; gap: var(--space-2);
  margin-bottom: var(--space-3);
}
.stat-card-icon {
  display: flex; align-items: center; justify-content: center;
  width: 20px; height: 20px;
}
.stat-card--blue .stat-card-icon { color: var(--color-primary); }
.stat-card--red .stat-card-icon { color: var(--color-danger); }
.stat-card--yellow .stat-card-icon { color: var(--color-warning); }
.stat-card--green .stat-card-icon { color: var(--color-success); }

.stat-card-label {
  font-size: var(--text-sm); color: var(--text-secondary); font-weight: 500;
}
.stat-card-value {
  font-size: var(--text-2xl); font-weight: 700;
  font-variant-numeric: tabular-nums;
  font-family: var(--font-mono);
  color: var(--text-primary);
  line-height: 1.2;
}
.stat-card-change {
  display: flex; align-items: center; gap: 2px;
  font-size: var(--text-xs); margin-top: var(--space-2);
  font-weight: 500; font-family: var(--font-mono);
}
.change-up { color: var(--color-success); }
.change-down { color: var(--color-danger); }
</style>
