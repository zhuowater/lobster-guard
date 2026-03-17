<template>
  <nav class="sidebar" :class="{ collapsed: appState.sidebarCollapsed, 'mobile-open': mobileOpen }">
    <div class="sidebar-brand">
      <span class="sidebar-logo">🦞</span>
      <div class="sidebar-brand-text">
        <div class="sidebar-brand-title">龙虾卫士</div>
        <div class="sidebar-brand-sub">Lobster Guard</div>
      </div>
    </div>
    <div class="sidebar-nav">
      <router-link
        v-for="item in navItems" :key="item.path"
        :to="item.path"
        class="nav-item"
        :class="{ active: $route.path === item.path }"
        @click="$emit('closeMobile')"
        :title="appState.sidebarCollapsed ? item.label : ''"
      >
        <span class="nav-icon" v-html="item.svg"></span>
        <span class="nav-label">{{ item.label }}</span>
      </router-link>
    </div>
    <div class="sidebar-footer">
      <div class="sidebar-version">{{ appState.version }}</div>
      <div class="sidebar-status">
        <span class="dot dot-sm" :class="dotClass"></span>
        <span class="sidebar-status-text">{{ statusText }}</span>
      </div>
    </div>
    <button class="sidebar-toggle" @click="toggleSidebar" :title="appState.sidebarCollapsed ? '展开' : '折叠'">
      <svg v-if="appState.sidebarCollapsed" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
      <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
    </button>
  </nav>
</template>

<script setup>
import { inject, computed } from 'vue'
import { toggleSidebar } from '../stores/app.js'

defineProps({ mobileOpen: Boolean })
defineEmits(['closeMobile'])

const appState = inject('appState')

const navItems = [
  { path: '/overview', label: '概览', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>' },
  { path: '/upstream', label: '上游', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><circle cx="6" cy="6" r="1" fill="currentColor" stroke="none"/><circle cx="6" cy="18" r="1" fill="currentColor" stroke="none"/></svg>' },
  { path: '/routes', label: '路由', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></svg>' },
  { path: '/rules', label: '规则', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>' },
  { path: '/audit', label: '审计', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>' },
  { path: '/monitor', label: '监控', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>' },
  { path: '/settings', label: '设置', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>' },
]

const dotClass = computed(() => appState.connectionStatus === 'connected' ? 'dot-healthy' : 'dot-unhealthy')
const statusText = computed(() => {
  const s = appState.connectionStatus
  return s === 'connected' ? '在线' : (s === 'degraded' ? '降级' : '断开')
})
</script>

<style scoped>
.sidebar {
  width: var(--sidebar-w); min-width: var(--sidebar-w); height: 100vh;
  background: var(--bg-surface);
  border-right: 1px solid var(--border-subtle);
  display: flex; flex-direction: column; transition: width var(--transition-normal), min-width var(--transition-normal);
  z-index: 200; position: relative; overflow: hidden;
}
.sidebar.collapsed { width: var(--sidebar-collapsed); min-width: var(--sidebar-collapsed); }
.sidebar-brand {
  padding: var(--space-4) var(--space-4) var(--space-3); display: flex; align-items: center; gap: var(--space-3);
  border-bottom: 1px solid var(--border-subtle); min-height: 60px;
  overflow: hidden; white-space: nowrap;
  background: var(--gradient-surface);
}
.sidebar-logo { font-size: 1.75rem; flex-shrink: 0; }
.sidebar-brand-text { display: flex; flex-direction: column; overflow: hidden; transition: opacity var(--transition-normal), width var(--transition-normal); }
.sidebar.collapsed .sidebar-brand-text { opacity: 0; width: 0; }
.sidebar-brand-title {
  font-size: var(--text-base); font-weight: 700;
  color: var(--text-primary);
  white-space: nowrap;
}
.sidebar-brand-sub { font-size: var(--text-xs); color: var(--text-tertiary); white-space: nowrap; }
.sidebar-nav { flex: 1; padding: var(--space-2) 0; overflow-y: auto; overflow-x: hidden; }
.nav-item {
  display: flex; align-items: center; gap: var(--space-3); padding: var(--space-2) var(--space-4); margin: var(--space-1) var(--space-2);
  border-radius: var(--radius-md); cursor: pointer; color: var(--text-secondary); font-size: var(--text-sm);
  transition: all var(--transition-fast); position: relative; white-space: nowrap; overflow: hidden; text-decoration: none;
}
.nav-item:hover { background: var(--bg-elevated); color: var(--text-primary); }
.nav-item.active { background: var(--color-primary-dim); color: var(--color-primary); }
.nav-item.active::before {
  content: ''; position: absolute; left: 0; top: var(--space-1); bottom: var(--space-1); width: 3px;
  background: var(--color-primary); border-radius: 0 3px 3px 0;
}
.nav-icon { flex-shrink: 0; width: 20px; height: 20px; display: flex; align-items: center; justify-content: center; }
.nav-label { transition: opacity var(--transition-normal); overflow: hidden; }
.sidebar.collapsed .nav-label { opacity: 0; width: 0; }
.sidebar.collapsed .nav-item { justify-content: center; padding: var(--space-2) 0; margin: var(--space-1) var(--space-1); }
.sidebar-footer {
  padding: var(--space-3) var(--space-4); border-top: 1px solid var(--border-subtle);
  display: flex; flex-direction: column; gap: var(--space-1); overflow: hidden;
}
.sidebar-version { font-size: var(--text-xs); color: var(--text-tertiary); font-family: var(--font-mono); white-space: nowrap; transition: opacity var(--transition-normal); }
.sidebar-status { display: flex; align-items: center; gap: var(--space-2); font-size: var(--text-xs); color: var(--text-secondary); white-space: nowrap; }
.sidebar.collapsed .sidebar-version,
.sidebar.collapsed .sidebar-status-text { opacity: 0; width: 0; overflow: hidden; }
.sidebar-toggle {
  display: flex; align-items: center; justify-content: center; padding: var(--space-2);
  margin: 0 var(--space-2) var(--space-2); border-radius: var(--radius-md); cursor: pointer; color: var(--text-tertiary);
  transition: all var(--transition-fast); border: 1px solid var(--border-subtle); background: transparent;
}
.sidebar-toggle:hover { background: var(--bg-elevated); color: var(--text-primary); }

@media(max-width:768px) {
  .sidebar {
    position: fixed; left: -280px; top: 0; bottom: 0; z-index: 201;
    transition: left .3s; box-shadow: 4px 0 20px rgba(0,0,0,.5);
  }
  .sidebar.mobile-open { left: 0; }
}
</style>
