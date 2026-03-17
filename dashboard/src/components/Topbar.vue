<template>
  <header class="topbar">
    <button class="hamburger" @click="$emit('toggleMobile')">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="18" x2="21" y2="18"/></svg>
    </button>
    <div class="topbar-breadcrumb">
      <span class="topbar-breadcrumb-root">龙虾卫士</span>
      <span class="topbar-breadcrumb-sep">/</span>
      <span class="topbar-breadcrumb-current">{{ currentTitle }}</span>
    </div>
    <div class="topbar-search">
      <svg class="topbar-search-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
      <input type="text" ref="searchInput" placeholder="搜索..." autocomplete="off" />
      <span class="topbar-search-hint">Ctrl+K</span>
    </div>
    <div class="topbar-right">
      <div class="topbar-status">
        <span class="dot dot-sm" :class="dotClass"></span>
        <span class="topbar-status-label">{{ statusLabel }}</span>
      </div>
      <div class="topbar-uptime" v-if="formattedUptime">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
        {{ formattedUptime }}
      </div>
    </div>
  </header>
</template>

<script setup>
import { inject, computed, ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'

defineEmits(['toggleMobile'])
const appState = inject('appState')
const route = useRoute()
const searchInput = ref(null)

const currentTitle = computed(() => route.meta?.title || '概览')
const dotClass = computed(() => appState.connectionStatus === 'connected' ? 'dot-healthy' : 'dot-unhealthy')
const statusLabel = computed(() => {
  const s = appState.connectionStatus
  return s === 'connected' ? '在线' : (s === 'degraded' ? '降级' : '断开')
})

const formattedUptime = computed(() => {
  const raw = appState.uptime
  if (!raw || raw === '--') return ''
  return formatUptime(raw)
})

function formatUptime(raw) {
  // Parse Go duration format: "22m21.534308958s", "3h22m1s", "72h15m3s", etc.
  let totalSeconds = 0

  // Match hours
  const hMatch = raw.match(/([\d.]+)h/)
  if (hMatch) totalSeconds += parseFloat(hMatch[1]) * 3600

  // Match minutes
  const mMatch = raw.match(/([\d.]+)m(?!s)/)
  if (mMatch) totalSeconds += parseFloat(mMatch[1]) * 60

  // Match seconds
  const sMatch = raw.match(/([\d.]+)s/)
  if (sMatch) totalSeconds += parseFloat(sMatch[1])

  if (totalSeconds <= 0) return raw

  const minutes = Math.floor(totalSeconds / 60)
  const hours = Math.floor(totalSeconds / 3600)
  const days = Math.floor(totalSeconds / 86400)

  if (minutes < 1) return '< 1 min'
  if (hours < 1) return minutes + ' min'
  if (days < 1) return hours + 'h ' + (minutes % 60) + 'm'
  if (days < 7) return days + 'd ' + (hours % 24) + 'h'
  return days + 'd'
}

function onKeydown(e) {
  if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
    e.preventDefault()
    searchInput.value?.focus()
  }
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onUnmounted(() => document.removeEventListener('keydown', onKeydown))
</script>

<style scoped>
.topbar {
  height: var(--topbar-h); min-height: var(--topbar-h);
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  display: flex; align-items: center; padding: 0 var(--space-5); gap: var(--space-4);
  z-index: 100;
}
.topbar-breadcrumb { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-sm); white-space: nowrap; }
.topbar-breadcrumb-root { color: var(--text-tertiary); }
.topbar-breadcrumb-sep { color: var(--text-disabled); font-size: var(--text-xs); }
.topbar-breadcrumb-current { color: var(--text-primary); font-weight: 500; }
.topbar-search { flex: 1; max-width: 400px; margin: 0 auto; position: relative; }
.topbar-search-icon {
  position: absolute; left: var(--space-3); top: 50%; transform: translateY(-50%);
  color: var(--text-tertiary); pointer-events: none;
}
.topbar-search input {
  width: 100%; background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary); padding: var(--space-2) var(--space-10) var(--space-2) var(--space-8);
  font-size: var(--text-sm); outline: none; transition: border-color var(--transition-fast);
  font-family: var(--font-sans);
}
.topbar-search input:focus { border-color: var(--color-primary); }
.topbar-search input::placeholder { color: var(--text-tertiary); }
.topbar-search-hint {
  position: absolute; right: var(--space-3); top: 50%; transform: translateY(-50%);
  font-size: var(--text-xs); color: var(--text-disabled); background: var(--bg-overlay);
  padding: 1px 6px; border-radius: var(--radius-sm); pointer-events: none;
  border: 1px solid var(--border-subtle);
}
.topbar-right { display: flex; align-items: center; gap: var(--space-4); white-space: nowrap; }
.topbar-status { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); color: var(--text-secondary); }
.topbar-uptime {
  display: flex; align-items: center; gap: var(--space-1);
  font-family: var(--font-mono); color: var(--text-tertiary); font-size: var(--text-xs);
}
.topbar-uptime svg { color: var(--text-disabled); }
.hamburger {
  display: none; background: none; border: none; color: var(--text-primary);
  cursor: pointer; padding: var(--space-1) var(--space-2);
}
@media(max-width:768px) {
  .hamburger { display: flex; align-items: center; }
  .topbar-search { max-width: 200px; }
}
@media(max-width:480px) { .topbar-search { display: none; } }
</style>
