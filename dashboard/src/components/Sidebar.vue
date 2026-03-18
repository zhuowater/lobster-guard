<template>
  <nav class="sidebar" :class="{ collapsed: appState.sidebarCollapsed, 'mobile-open': mobileOpen }">
    <!-- 严格模式横幅 (v11.1) -->
    <div class="strict-banner" v-if="strictMode && !appState.sidebarCollapsed">⚠️ 严格模式已启用</div>
    <div class="sidebar-brand">
      <span class="sidebar-logo" :class="{'strict-logo': strictMode}">🦞</span>
      <div class="sidebar-brand-text">
        <div class="sidebar-brand-title">龙虾卫士</div>
        <div class="sidebar-brand-sub">Lobster Guard</div>
      </div>
    </div>
    <div class="sidebar-nav">
      <!-- IM 安全 -->
      <div class="nav-group-label" v-if="!appState.sidebarCollapsed">IM 安全</div>
      <router-link
        v-for="item in imItems" :key="item.path"
        :to="item.path"
        class="nav-item"
        :class="{ active: $route.path === item.path || ($route.path.startsWith(item.path + '/') && item.path.length > 1) }"
        @click="$emit('closeMobile')"
        :title="appState.sidebarCollapsed ? item.label : ''"
      >
        <span class="nav-icon" v-html="item.svg"></span>
        <span class="nav-label">{{ item.label }}</span>
      </router-link>

      <!-- LLM 安全 (仅启用时显示) -->
      <template v-if="llmEnabled">
        <div class="nav-divider"></div>
        <div class="nav-group-label" v-if="!appState.sidebarCollapsed">LLM 安全</div>
        <router-link
          v-for="item in llmItems" :key="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ active: $route.path === item.path || $route.path.startsWith(item.path + '/') }"
          @click="$emit('closeMobile')"
          :title="appState.sidebarCollapsed ? item.label : ''"
        >
          <span class="nav-icon" v-html="item.svg"></span>
          <span class="nav-label">{{ item.label }}</span>
        </router-link>
      </template>

      <!-- 系统 -->
      <div class="nav-divider"></div>
      <div class="nav-group-label" v-if="!appState.sidebarCollapsed">系统</div>
      <router-link
        v-for="item in systemItems" :key="item.path"
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
      <!-- 严格模式开关 (v11.1) -->
      <div class="strict-toggle" v-if="!appState.sidebarCollapsed">
        <span class="strict-label">🛡️ 严格模式</span>
        <label class="toggle-switch">
          <input type="checkbox" :checked="strictMode" @change="toggleStrictMode">
          <span class="toggle-slider" :class="{'toggle-active': strictMode}"></span>
        </label>
      </div>
      <div class="sidebar-version">{{ appState.version }}</div>
      <div class="sidebar-status">
        <span class="dot dot-sm" :class="dotClass"></span>
        <span class="sidebar-status-text">{{ statusText }}</span>
      </div>
    </div>
    <!-- 严格模式切换 toast -->
    <Transition name="toast-fade">
      <div class="strict-toast" v-if="strictToastVisible">{{ strictToast }}</div>
    </Transition>
    <button class="sidebar-toggle" @click="toggleSidebar" :title="appState.sidebarCollapsed ? '展开' : '折叠'">
      <svg v-if="appState.sidebarCollapsed" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
      <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
    </button>
  </nav>
</template>

<script setup>
import { inject, computed, ref, onMounted } from 'vue'
import { toggleSidebar } from '../stores/app.js'
import { api, apiPost } from '../api.js'

defineProps({ mobileOpen: Boolean })
defineEmits(['closeMobile'])

const appState = inject('appState')

const llmEnabled = ref(false)
const strictMode = ref(false)

async function checkLLMStatus() {
  try { const d = await api('/api/v1/llm/status'); llmEnabled.value = d.enabled === true } catch { llmEnabled.value = false }
}

async function loadStrictMode() {
  try { const d = await api('/api/v1/system/strict-mode'); strictMode.value = d.enabled === true } catch { strictMode.value = false }
}

const strictToast = ref('')
const strictToastVisible = ref(false)

async function toggleStrictMode() {
  const newVal = !strictMode.value
  const msg = newVal ? '确定要启用严格模式吗？\n\n所有规则将切换为"拦截"模式，影子模式规则也将生效。' : '确定要关闭严格模式吗？\n\n所有规则将恢复到之前的状态。'
  if (!confirm(msg)) return
  try {
    const res = await apiPost('/api/v1/system/strict-mode', { enabled: newVal })
    strictMode.value = newVal
    const imCount = res.affected_im_rules || 0
    const llmCount = res.affected_llm_rules || 0
    if (newVal) {
      strictToast.value = `已切换为严格模式，${imCount} 条 IM 规则 + ${llmCount} 条 LLM 规则已设为拦截`
    } else {
      strictToast.value = `已关闭严格模式，${imCount} 条 IM 规则 + ${llmCount} 条 LLM 规则已恢复`
    }
    strictToastVisible.value = true
    setTimeout(() => { strictToastVisible.value = false }, 4000)
  } catch { alert('切换失败') }
}

onMounted(() => { checkLLMStatus(); loadStrictMode() })

// IM 安全导航项
const imItems = [
  { path: '/overview', label: '概览', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>' },
  { path: '/upstream', label: '上游', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><circle cx="6" cy="6" r="1" fill="currentColor" stroke="none"/><circle cx="6" cy="18" r="1" fill="currentColor" stroke="none"/></svg>' },
  { path: '/routes', label: '路由', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="6" y1="3" x2="6" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/></svg>' },
  { path: '/rules', label: '规则', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>' },
  { path: '/audit', label: '审计', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>' },
  { path: '/user-profiles', label: '用户画像', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/><circle cx="12" cy="7" r="1" fill="currentColor" stroke="none"/><path d="M15 11l2 2m0 0l2-2m-2 2V9"/></svg>' },
]

// LLM 安全导航项
const llmItems = [
  { path: '/llm', label: 'LLM 概览', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44 2.5 2.5 0 0 1-2.96-3.08 3 3 0 0 1-.34-5.58 2.5 2.5 0 0 1 1.32-4.24A2.5 2.5 0 0 1 9.5 2"/><path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44 2.5 2.5 0 0 0 2.96-3.08 3 3 0 0 0 .34-5.58 2.5 2.5 0 0 0-1.32-4.24A2.5 2.5 0 0 0 14.5 2"/></svg>' },
  { path: '/llm-rules', label: 'LLM 规则', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>' },
  { path: '/agent', label: 'Agent 行为', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="10" rx="2"/><circle cx="12" cy="5" r="2"/><line x1="12" y1="7" x2="12" y2="11"/><line x1="8" y1="16" x2="8" y2="16.01"/><line x1="16" y1="16" x2="16" y2="16.01"/></svg>' },
  { path: '/sessions', label: '会话回放', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="20" rx="2.18" ry="2.18"/><line x1="7" y1="2" x2="7" y2="22"/><line x1="17" y1="2" x2="17" y2="22"/><line x1="2" y1="12" x2="22" y2="12"/><line x1="2" y1="7" x2="7" y2="7"/><line x1="2" y1="17" x2="7" y2="17"/><line x1="17" y1="7" x2="22" y2="7"/><line x1="17" y1="17" x2="22" y2="17"/></svg>' },
  { path: '/prompts', label: 'Prompt 追踪', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><path d="M9 15l2 2 4-4"/></svg>' },
  { path: '/honeypot', label: 'Agent 蜜罐', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2C9.5 2 7 4 7 7c0 2-1 3-2 4s-2 3-2 5c0 3.3 4 6 9 6s9-2.7 9-6c0-2-1-4-2-5s-2-2-2-4c0-3-2.5-5-5-5z"/><path d="M10 13h4"/><path d="M9 17h6"/></svg>' },
  { path: '/ab-testing', label: 'A/B 测试', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><path d="M12 2v4"/><path d="M8 10h3l1 4h0l1-4h3"/><line x1="6" y1="18" x2="10" y2="18"/><line x1="14" y1="18" x2="18" y2="18"/></svg>' },
  { path: '/behavior', label: '行为画像', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2a4 4 0 0 1 4 4c0 2.5-2 4-2 6h-4c0-2-2-3.5-2-6a4 4 0 0 1 4-4z"/><path d="M10 12h4"/><path d="M10 16h4"/><path d="M11 16v3a1 1 0 0 0 2 0v-3"/><circle cx="7" cy="14" r="1"/><circle cx="17" cy="14" r="1"/><path d="M5 10c-1 0-2 1-2 2s1 2 2 2"/><path d="M19 10c1 0 2 1 2 2s-1 2-2 2"/></svg>' },
  { path: '/attack-chains', label: '攻击链', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>' },
]

// 系统导航项
const systemItems = [
  { path: '/monitor', label: '监控', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>' },
  { path: '/anomaly', label: '异常检测', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12h2l3-7 4 14 4-10 3 3h4"/></svg>' },
  { path: '/reports', label: '报告', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="12" y1="18" x2="12" y2="12"/><polyline points="9 15 12 12 15 15"/></svg>' },
  { path: '/tenants', label: '租户', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>' },
  { path: '/redteam', label: '红队测试', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/><line x1="12" y1="2" x2="12" y2="6"/><line x1="12" y1="18" x2="12" y2="22"/><line x1="2" y1="12" x2="6" y2="12"/><line x1="18" y1="12" x2="22" y2="12"/></svg>' },
  { path: '/leaderboard', label: '排行榜', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9H4.5a2.5 2.5 0 0 1 0-5C7 4 9 7 12 7s5-3 7.5-3a2.5 2.5 0 0 1 0 5H18"/><path d="M18 9v10a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2V9"/><line x1="12" y1="11" x2="12" y2="17"/><line x1="9" y1="14" x2="9" y2="17"/><line x1="15" y1="13" x2="15" y2="17"/></svg>' },
  { path: '/ops', label: '运维', svg: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>' },
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
.sidebar-nav { flex: 1; padding: var(--space-2) 0; overflow-y: auto; overflow-x: hidden; min-height: 0; }
.sidebar-nav::-webkit-scrollbar { width: 4px; }
.sidebar-nav::-webkit-scrollbar-track { background: transparent; }
.sidebar-nav::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.15); border-radius: 4px; }
.sidebar-nav::-webkit-scrollbar-thumb:hover { background: rgba(255,255,255,0.3); }

.nav-group-label {
  padding: var(--space-2) var(--space-4) var(--space-1);
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-tertiary);
  white-space: nowrap;
  overflow: hidden;
}
.nav-divider {
  height: 1px;
  background: var(--border-subtle);
  margin: var(--space-2) var(--space-3);
}

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
.sidebar.collapsed .nav-group-label,
.sidebar.collapsed .nav-divider { display: none; }
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

/* 严格模式 */
.strict-banner {
  background: linear-gradient(90deg, #DC2626, #EF4444);
  color: #fff; text-align: center; font-size: 11px; font-weight: 700;
  padding: 4px 8px; letter-spacing: 0.05em;
}
.strict-logo { filter: hue-rotate(0deg) saturate(3) brightness(1.2); }
.strict-toggle {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 0 var(--space-2) 0; margin-bottom: var(--space-2);
  border-bottom: 1px solid var(--border-subtle);
}
.strict-label { font-size: var(--text-xs); color: var(--text-secondary); white-space: nowrap; }
.toggle-switch { position: relative; display: inline-block; width: 36px; height: 20px; cursor: pointer; }
.toggle-switch input { opacity: 0; width: 0; height: 0; }
.toggle-slider {
  position: absolute; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(255,255,255,0.1); border-radius: 20px; transition: all .3s;
}
.toggle-slider::before {
  content: ''; position: absolute; height: 16px; width: 16px; left: 2px; bottom: 2px;
  background: #fff; border-radius: 50%; transition: all .3s;
}
.toggle-active { background: #EF4444; }
.toggle-active::before { transform: translateX(16px); }

/* 严格模式 toast */
.strict-toast {
  position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%);
  background: var(--bg-surface); border: 1px solid var(--color-primary);
  color: var(--text-primary); padding: 10px 20px; border-radius: var(--radius-md);
  font-size: var(--text-xs); font-weight: 600; box-shadow: var(--shadow-lg);
  z-index: 9999; white-space: nowrap; max-width: 90vw; text-align: center;
}
.toast-fade-enter-active { transition: all .3s ease; }
.toast-fade-leave-active { transition: all .3s ease; }
.toast-fade-enter-from, .toast-fade-leave-to { opacity: 0; transform: translateX(-50%) translateY(10px); }
@media(max-width:768px) {
  .sidebar {
    position: fixed; left: -280px; top: 0; bottom: 0; z-index: 201;
    transition: left .3s; box-shadow: 4px 0 20px rgba(0,0,0,.5);
  }
  .sidebar.mobile-open { left: 0; }
}
</style>
