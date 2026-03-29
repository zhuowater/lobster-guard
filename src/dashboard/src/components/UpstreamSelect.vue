<template>
  <div class="us-wrap">
    <select
      :value="modelValue"
      @input="$emit('update:modelValue', $event.target.value)"
      class="us-select"
      :class="{ 'us-error': error }"
    >
      <option value="" disabled>{{ placeholder }}</option>
      <option
        v-for="up in sortedUpstreams"
        :key="up.id"
        :value="up.id"
        :disabled="!up.healthy && onlyHealthy"
      >
        {{ up.id }} — {{ up.address || up.addr || up.host }}:{{ up.port }}
        {{ up.healthy ? '✓' : '✗' }}
        ({{ up.user_count || 0 }} 用户)
      </option>
    </select>
    <div v-if="loading" class="us-loading">
      <span class="us-spinner"></span>
    </div>
    <div v-if="selectedUpstream" class="us-preview">
      <span class="us-dot" :class="selectedUpstream.healthy ? 'us-dot-ok' : 'us-dot-err'"></span>
      <span class="us-preview-text">
        {{ selectedUpstream.id }} · {{ selectedUpstream.address || selectedUpstream.addr }}:{{ selectedUpstream.port }}
      </span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { api } from '../api.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: '选择上游...' },
  onlyHealthy: { type: Boolean, default: false },
  error: { type: Boolean, default: false },
})

defineEmits(['update:modelValue'])

const upstreams = ref([])
const loading = ref(false)

const sortedUpstreams = computed(() => {
  return [...upstreams.value].sort((a, b) => {
    if (a.healthy && !b.healthy) return -1
    if (!a.healthy && b.healthy) return 1
    return (a.id || '').localeCompare(b.id || '')
  })
})

const selectedUpstream = computed(() => {
  if (!props.modelValue) return null
  return upstreams.value.find(u => u.id === props.modelValue)
})

async function loadUpstreams() {
  loading.value = true
  try {
    const d = await api('/api/v1/upstreams')
    upstreams.value = d.upstreams || []
  } catch {
    upstreams.value = []
  }
  loading.value = false
}

onMounted(loadUpstreams)

defineExpose({ reload: loadUpstreams })
</script>

<style scoped>
.us-wrap { position: relative; }
.us-select {
  width: 100%;
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary);
  padding: var(--space-2) var(--space-3);
  font-size: var(--text-sm); outline: none;
  font-family: var(--font-sans);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  cursor: pointer;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' fill='none' stroke='%238B95A8' stroke-width='2'%3E%3Cpolyline points='2 4 6 8 10 4'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 12px center;
  padding-right: 32px;
}
.us-select:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-dim);
}
.us-select.us-error {
  border-color: var(--color-danger);
  box-shadow: 0 0 0 3px var(--color-danger-dim);
}
.us-select option {
  background: var(--bg-elevated); color: var(--text-primary);
  padding: var(--space-2);
}
.us-select option:disabled { color: var(--text-disabled); }

.us-loading {
  position: absolute; right: 36px; top: 50%; transform: translateY(-50%);
}
.us-spinner {
  display: inline-block; width: 12px; height: 12px;
  border: 2px solid var(--color-primary-dim); border-top-color: var(--color-primary);
  border-radius: 50%; animation: spn .6s linear infinite;
}

.us-preview {
  display: flex; align-items: center; gap: var(--space-2);
  margin-top: var(--space-1);
  font-size: var(--text-xs); color: var(--text-tertiary);
}
.us-dot {
  width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0;
}
.us-dot-ok { background: var(--color-success); box-shadow: 0 0 4px var(--color-success); }
.us-dot-err { background: var(--color-danger); box-shadow: 0 0 4px var(--color-danger); }
.us-preview-text { font-family: var(--font-mono); }
</style>
