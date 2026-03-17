<template>
  <div class="ov-card">
    <div class="ov-card-icon" :class="color">{{ icon }}</div>
    <div class="ov-card-body">
      <div class="ov-card-value" :class="color">{{ value }}</div>
      <div class="ov-card-label">{{ label }}</div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  icon: { type: String, default: '📊' },
  value: { type: [String, Number], default: '--' },
  label: { type: String, default: '' },
  color: { type: String, default: 'blue' },
})
</script>
