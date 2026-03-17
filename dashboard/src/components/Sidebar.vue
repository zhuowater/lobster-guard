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
      >
        <span class="nav-icon">{{ item.icon }}</span>
        <span class="nav-label">{{ item.label }}</span>
      </router-link>
    </div>
    <div class="sidebar-footer">
      <div class="sidebar-version">{{ appState.version }}</div>
      <div class="sidebar-status">
        <span class="dot dot-sm" :class="dotClass"></span>
        <span>{{ statusText }}</span>
      </div>
    </div>
    <button class="sidebar-toggle" @click="toggleSidebar" title="折叠/展开">≡</button>
  </nav>
</template>

<script setup>
import { inject, computed } from 'vue'
import { toggleSidebar } from '../stores/app.js'

defineProps({ mobileOpen: Boolean })
defineEmits(['closeMobile'])

const appState = inject('appState')

const navItems = [
  { path: '/overview', icon: '📊', label: '概览' },
  { path: '/upstream', icon: '🔗', label: '上游' },
  { path: '/routes', icon: '🗺️', label: '路由' },
  { path: '/rules', icon: '🛡️', label: '规则' },
  { path: '/audit', icon: '📋', label: '审计' },
  { path: '/monitor', icon: '⚡', label: '监控' },
  { path: '/settings', icon: '⚙️', label: '设置' },
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
  background: linear-gradient(180deg, #0d1137, #151a3a);
  border-right: 1px solid rgba(0,212,255,.12);
  display: flex; flex-direction: column; transition: width .3s, min-width .3s;
  z-index: 200; position: relative; overflow: hidden;
}
.sidebar.collapsed { width: var(--sidebar-collapsed); min-width: var(--sidebar-collapsed); }
.sidebar-brand {
  padding: 16px 16px 12px; display: flex; align-items: center; gap: 10px;
  border-bottom: 1px solid rgba(0,212,255,.08); min-height: 60px;
  overflow: hidden; white-space: nowrap;
}
.sidebar-logo { font-size: 2rem; flex-shrink: 0; animation: lf 3s ease-in-out infinite; }
@keyframes lf { 0%,100% { transform: translateY(0) rotate(0); } 25% { transform: translateY(-3px) rotate(-4deg); } 75% { transform: translateY(2px) rotate(3deg); } }
.sidebar-brand-text { display: flex; flex-direction: column; overflow: hidden; transition: opacity .3s; }
.sidebar.collapsed .sidebar-brand-text { opacity: 0; width: 0; }
.sidebar-brand-title {
  font-size: 1rem; font-weight: 700;
  background: linear-gradient(90deg, var(--neon-blue), var(--neon-green));
  -webkit-background-clip: text; -webkit-text-fill-color: transparent; background-clip: text;
  white-space: nowrap;
}
.sidebar-brand-sub { font-size: .65rem; color: var(--text-dim); white-space: nowrap; }
.sidebar-nav { flex: 1; padding: 8px 0; overflow-y: auto; overflow-x: hidden; }
.nav-item {
  display: flex; align-items: center; gap: 12px; padding: 10px 16px; margin: 2px 8px;
  border-radius: 8px; cursor: pointer; color: var(--text-dim); font-size: .88rem;
  transition: all .2s; position: relative; white-space: nowrap; overflow: hidden; text-decoration: none;
}
.nav-item:hover { background: rgba(0,212,255,.06); color: var(--text); }
.nav-item.active { background: rgba(0,212,255,.1); color: var(--neon-blue); }
.nav-item.active::before {
  content: ''; position: absolute; left: 0; top: 4px; bottom: 4px; width: 3px;
  background: var(--neon-blue); border-radius: 0 3px 3px 0;
}
.nav-icon { font-size: 1.15rem; flex-shrink: 0; width: 24px; text-align: center; }
.nav-label { transition: opacity .3s; overflow: hidden; }
.sidebar.collapsed .nav-label { opacity: 0; width: 0; }
.sidebar.collapsed .nav-item { justify-content: center; padding: 10px 0; margin: 2px 4px; }
.sidebar-footer {
  padding: 12px 16px; border-top: 1px solid rgba(0,212,255,.08);
  display: flex; flex-direction: column; gap: 6px; overflow: hidden;
}
.sidebar-version { font-size: .7rem; color: var(--text-dim); font-family: monospace; white-space: nowrap; transition: opacity .3s; }
.sidebar-status { display: flex; align-items: center; gap: 6px; font-size: .75rem; color: var(--text-dim); white-space: nowrap; }
.sidebar.collapsed .sidebar-version, .sidebar.collapsed .sidebar-status span:not(.dot) { opacity: 0; width: 0; overflow: hidden; }
.sidebar-toggle {
  display: flex; align-items: center; justify-content: center; padding: 8px;
  margin: 0 8px 8px; border-radius: 6px; cursor: pointer; color: var(--text-dim);
  font-size: 1.1rem; transition: all .2s; border: 1px solid rgba(0,212,255,.1); background: transparent;
}
.sidebar-toggle:hover { background: rgba(0,212,255,.1); color: var(--neon-blue); }

@media(max-width:768px) {
  .sidebar {
    position: fixed; left: -280px; top: 0; bottom: 0; z-index: 201;
    transition: left .3s; box-shadow: 4px 0 20px rgba(0,0,0,.5);
  }
  .sidebar.mobile-open { left: 0; }
}
</style>
