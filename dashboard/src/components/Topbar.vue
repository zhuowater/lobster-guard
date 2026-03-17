<template>
  <header class="topbar">
    <button class="hamburger" @click="$emit('toggleMobile')">☰</button>
    <div class="topbar-breadcrumb">
      <span class="topbar-breadcrumb-root">🦞 龙虾卫士</span>
      <span class="topbar-breadcrumb-sep">›</span>
      <span class="topbar-breadcrumb-current">{{ currentTitle }}</span>
    </div>
    <div class="topbar-search">
      <input type="text" ref="searchInput" placeholder="搜索..." autocomplete="off" />
      <span class="topbar-search-hint">Ctrl+K</span>
    </div>
    <div class="topbar-right">
      <div class="topbar-status">
        <span class="dot dot-sm" :class="dotClass"></span>
        <span>{{ statusLabel }}</span>
      </div>
      <div class="topbar-uptime">{{ appState.uptime }}</div>
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
  background: linear-gradient(135deg, rgba(13,17,55,.95), rgba(26,31,58,.95));
  border-bottom: 1px solid rgba(0,212,255,.1);
  display: flex; align-items: center; padding: 0 20px; gap: 16px;
  backdrop-filter: blur(20px); z-index: 100;
}
.topbar-breadcrumb { display: flex; align-items: center; gap: 6px; font-size: .85rem; white-space: nowrap; }
.topbar-breadcrumb-root { color: var(--text-dim); }
.topbar-breadcrumb-sep { color: var(--text-dim); font-size: .7rem; }
.topbar-breadcrumb-current { color: var(--neon-blue); font-weight: 600; }
.topbar-search { flex: 1; max-width: 400px; margin: 0 auto; position: relative; }
.topbar-search input {
  width: 100%; background: rgba(0,0,0,.3); border: 1px solid rgba(0,212,255,.2);
  border-radius: 8px; color: var(--text); padding: 7px 36px 7px 12px; font-size: .82rem;
  outline: none; transition: border-color .3s;
}
.topbar-search input:focus { border-color: var(--neon-blue); box-shadow: 0 0 12px rgba(0,212,255,.15); }
.topbar-search input::placeholder { color: var(--text-dim); }
.topbar-search-hint {
  position: absolute; right: 10px; top: 50%; transform: translateY(-50%);
  font-size: .65rem; color: var(--text-dim); background: rgba(0,0,0,.3);
  padding: 1px 6px; border-radius: 3px; pointer-events: none;
}
.topbar-right { display: flex; align-items: center; gap: 12px; white-space: nowrap; }
.topbar-status { display: flex; align-items: center; gap: 6px; font-size: .78rem; color: var(--text-dim); }
.topbar-uptime { font-family: monospace; color: var(--neon-green); font-size: .78rem; }
.hamburger { display: none; background: none; border: none; color: var(--text); font-size: 1.3rem; cursor: pointer; padding: 4px 8px; }
@media(max-width:768px) {
  .hamburger { display: block; }
  .topbar-search { max-width: 200px; }
}
@media(max-width:480px) { .topbar-search { display: none; } }
</style>
