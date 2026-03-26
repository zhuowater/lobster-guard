<template>
  <header class="topbar">
    <button class="hamburger" @click="$emit('toggleMobile')">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="18" x2="21" y2="18"/></svg>
    </button>
    <div class="topbar-breadcrumb">
      <router-link to="/overview" class="topbar-breadcrumb-root topbar-breadcrumb-link">龙虾卫士</router-link>
      <span class="topbar-breadcrumb-sep">/</span>
      <router-link :to="tabFirstRoute" class="topbar-breadcrumb-tab topbar-breadcrumb-link">{{ currentTabLabel }}</router-link>
      <span class="topbar-breadcrumb-sep">/</span>
      <span class="topbar-breadcrumb-current">{{ currentPageLabel }}</span>
    </div>
    <!-- v15.0: 顶部 Tab 导航 -->
    <div class="topnav-tabs" v-if="navStore.mode === 'classic'">
      <button
        v-for="(cfg, key) in navStore.tabs" :key="key"
        class="topnav-tab"
        :class="{ 'topnav-tab-active': navStore.activeTab === key }"
        @click="onTabClick(key)"
      >
        <Icon :name="cfg.icon" :size="14" class="topnav-tab-icon" />
        <span class="topnav-tab-label">{{ cfg.label }}</span>
      </button>
    </div>
    <div class="topbar-search">
      <svg class="topbar-search-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
      <input type="text" ref="searchInput" placeholder="搜索..." autocomplete="off" />
      <span class="topbar-search-hint">Ctrl+K</span>
    </div>
    <div class="topbar-right">
      <!-- v14.0: 租户切换器 -->
      <div class="tenant-switcher" v-if="tenants.length > 1">
        <select class="tenant-select" :value="currentTenantId" @change="onTenantChange">
          <option v-for="t in tenants" :key="t.id" :value="t.id">{{ t.name }}</option>
        </select>
      </div>
      <!-- 通知中心 (v11.1) -->
      <div class="notif-wrap" ref="notifWrap">
        <button class="notif-btn" @click="toggleNotif" :title="'通知 (' + unreadCount + ' 未读)'">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
          <span class="notif-badge" v-if="unreadCount > 0">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
        </button>
        <div class="notif-panel" v-if="notifOpen">
          <div class="notif-panel-header">
            <span>通知中心</span>
            <button class="notif-mark-read" @click="markAllRead" v-if="unreadCount > 0">全部已读</button>
          </div>
          <div class="notif-list" v-if="notifications.length">
            <div v-for="n in notifications" :key="n.id" class="notif-item" :class="{'notif-unread': !isRead(n.id)}" @click="onNotifClick(n)">
              <span class="notif-severity" :class="'sev-'+n.severity">●</span>
              <div class="notif-content">
                <div class="notif-summary">{{ n.summary }}</div>
                <div class="notif-detail" v-if="n.detail">{{ n.detail }}</div>
                <div class="notif-time">{{ fmtTime(n.timestamp) }} · {{ n.type_label }}</div>
              </div>
            </div>
          </div>
          <div class="notif-empty" v-else>✅ 暂无通知</div>
        </div>
      </div>
      <!-- v15.0: 模式切换开关 -->
      <div class="mode-toggle" @click="toggleMode" :title="navStore.mode === 'classic' ? '切换到叙事模式' : '切换到经典模式'">
        <span class="mode-toggle-label" :class="{ 'mode-active': navStore.mode === 'narrative' }"><Icon name="eye" :size="12" /></span>
        <div class="mode-toggle-track" :class="{ 'mode-track-classic': navStore.mode === 'classic' }">
          <div class="mode-toggle-thumb" :class="{ 'mode-thumb-right': navStore.mode === 'classic' }"></div>
        </div>
        <span class="mode-toggle-label" :class="{ 'mode-active': navStore.mode === 'classic' }"><Icon name="grid" :size="12" /></span>
      </div>
      <div class="topbar-status">
        <span class="dot dot-sm" :class="dotClass"></span>
        <span class="topbar-status-label">{{ statusLabel }}</span>
      </div>
      <div class="topbar-uptime" v-if="formattedUptime">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
        {{ formattedUptime }}
      </div>
      <!-- v14.1: 用户信息 + 登出 -->
      <div class="user-menu" v-if="authUser" ref="userMenuWrap">
        <button class="user-btn" @click="userMenuOpen = !userMenuOpen">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
          <span class="user-name">{{ authUser.display_name || authUser.username }}</span>
          <span class="user-role-badge" :class="'role-' + authUser.role">{{ authUser.role }}</span>
        </button>
        <div class="user-dropdown" v-if="userMenuOpen">
          <div class="user-dropdown-info">
            <div class="user-dropdown-name">{{ authUser.display_name || authUser.username }}</div>
            <div class="user-dropdown-role">{{ roleLabel(authUser.role) }}</div>
          </div>
          <div class="user-dropdown-divider"></div>
          <button class="user-dropdown-item" @click="doLogout">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
            退出登录
          </button>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup>
import { inject, computed, ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, clearToken } from '../api.js'
import { currentTenant, setTenant, updateTenantList, currentUser, logoutUser } from '../stores/app.js'
import { navStore } from '../stores/navigation.js'
import Icon from './Icon.vue'

defineEmits(['toggleMobile'])
const appState = inject('appState')
const route = useRoute()
const router = useRouter()
const searchInput = ref(null)

// v15.0: Tab 导航 & 模式切换
function onTabClick(tab) {
  navStore.setTab(tab)
  // 导航到该 Tab 的第一个路由
  const firstRoute = navStore.tabs[tab]?.routes[0]
  if (firstRoute) {
    router.push({ name: firstRoute })
  }
}

function toggleMode() {
  const newMode = navStore.mode === 'classic' ? 'narrative' : 'classic'
  navStore.setMode(newMode)
}

// v14.0: 租户切换
const tenants = ref([])
const currentTenantId = computed(() => currentTenant.value)

async function loadTenants() {
  try {
    const d = await api('/api/v1/tenants')
    tenants.value = d.tenants || []
    updateTenantList(d.tenants || [])
  } catch {
    tenants.value = [{ id: 'default', name: '默认租户' }]
  }
}

function onTenantChange(e) {
  setTenant(e.target.value)
  // Reload current page to refresh data with new tenant
  router.go(0)
}

// v14.1: 用户菜单
const userMenuOpen = ref(false)
const userMenuWrap = ref(null)
const authUser = ref(null)

async function loadAuthUser() {
  try {
    const d = await api('/api/v1/auth/me')
    authUser.value = d
    if (d.username) {
      currentUser.value = d
    }
  } catch {
    authUser.value = null
  }
}

function roleLabel(role) {
  const map = { admin: '管理员', operator: '运维', viewer: '只读' }
  return map[role] || role
}

async function doLogout() {
  try { await api('/api/v1/auth/logout', { method: 'POST' }) } catch {}
  logoutUser()
  clearToken()
  userMenuOpen.value = false
  router.push('/login')
}

function onUserMenuClickOutside(e) {
  if (userMenuWrap.value && !userMenuWrap.value.contains(e.target)) {
    userMenuOpen.value = false
  }
}

// v11.1: 通知中心
const notifOpen = ref(false)
const notifWrap = ref(null)
const notifications = ref([])
const READ_KEY = 'lobster_notif_read'

function getReadIds() { try { return JSON.parse(localStorage.getItem(READ_KEY) || '[]') } catch { return [] } }
function isRead(id) { return getReadIds().includes(id) }
const unreadCount = computed(() => { const read = getReadIds(); return notifications.value.filter(n => !read.includes(n.id)).length })

function toggleNotif() { notifOpen.value = !notifOpen.value; if (notifOpen.value) loadNotifications() }
function markAllRead() { const ids = notifications.value.map(n => n.id); localStorage.setItem(READ_KEY, JSON.stringify(ids)); notifOpen.value = false }
function onNotifClick(n) {
  const read = getReadIds(); if (!read.includes(n.id)) { read.push(n.id); localStorage.setItem(READ_KEY, JSON.stringify(read)) }
  if (n.type === 'blocked') router.push('/audit')
  else if (n.type === 'canary_leak') router.push('/agent')
  else if (n.type === 'budget_exceeded') router.push('/agent')
  else if (n.type === 'high_risk_tool') router.push('/sessions')
  else if (n.type === 'anomaly') router.push('/anomaly')
  else if (n.type === 'report_ready') router.push('/reports')
  else if (n.type === 'session_risk') router.push('/sessions')
  else if (n.type === 'prompt_changed') router.push('/prompts')
  notifOpen.value = false
}
async function loadNotifications() { try { const d = await api('/api/v1/notifications'); notifications.value = d.notifications || [] } catch { notifications.value = [] } }
function fmtTime(ts) { if (!ts) return ''; const d = new Date(ts); return isNaN(d.getTime()) ? '' : d.toLocaleString('zh-CN', { hour12: false }) }
function onClickOutside(e) { if (notifWrap.value && !notifWrap.value.contains(e.target)) notifOpen.value = false }
let notifTimer = null

const currentTitle = computed(() => route.meta?.title || '概览')

// v18.0: 三级面包屑 — 龙虾卫士 > Tab名 > 页面名
const allNavItems = {
  'overview':          { label: '概览（驾驶舱）' },
  'custom-dashboard':  { label: '自定义大屏' },
  'anomaly':           { label: '异常检测' },
  'monitor':           { label: '监控指标' },
  'audit':             { label: '审计日志' },
  'sessions':          { label: '会话回放' },
  'session-detail':    { label: '会话详情' },
  'attack-chains':     { label: '攻击链分析' },
  'user-profiles':     { label: '用户画像' },
  'user-detail':       { label: '用户详情' },
  'behavior':          { label: '行为画像' },
  'honeypot':          { label: 'Agent 蜜罐' },
  'singularity':       { label: '奇点蜜罐' },
  'prompts':           { label: 'Prompt 追踪' },
  'taint':             { label: '污染追踪' },
  'redteam':           { label: 'Red Team' },
  'semantic':          { label: '语义检测' },
  'rules':             { label: '入站规则' },
  'llm-rules':         { label: 'LLM 规则' },
  'llm-targets':       { label: 'LLM 目标' },
  'tools':             { label: '工具策略' },
  'evolution':         { label: '自进化' },
  'cache':             { label: '响应缓存' },
  'gateway':           { label: 'API 网关' },
  'routes':            { label: '路由策略' },
  'envelopes':         { label: '执行信封' },
  'events':            { label: '事件总线' },
  'ab-testing':        { label: 'A/B 测试' },
  'upstream':          { label: '上游管理' },
  'plan-compiler':     { label: '执行计划' },
  'agent':             { label: 'Agent 行为' },
  'path-policy':       { label: '路径治理' },
  'counterfactual':    { label: '反事实验证' },
  'capability':        { label: '能力标签' },
  'deviations':        { label: '偏差检测' },
  'ifc':               { label: '信息流控制' },
  'reports':           { label: '报告中心' },
  'leaderboard':       { label: '排行榜' },
  'tenants':           { label: '租户管理' },
  'users':             { label: '用户管理' },
  'llm':               { label: 'LLM 概览' },
  'ops':               { label: '运维工具' },
  'settings':          { label: '设置' },
  'gateway-monitor':   { label: 'Gateway 监控' },
}

const currentTabKey = computed(() => navStore.getTabForRoute(route.name))
const currentTabLabel = computed(() => {
  const tab = navStore.tabs[currentTabKey.value]
  return tab ? tab.label : '安全总览'
})
const tabFirstRoute = computed(() => {
  const tab = navStore.tabs[currentTabKey.value]
  if (!tab) return '/overview'
  const firstName = tab.routes ? tab.routes[0] : (tab.groups ? tab.groups[0].routes[0] : 'overview')
  return { name: firstName }
})
const currentPageLabel = computed(() => {
  const name = route.name
  if (allNavItems[name]) return allNavItems[name].label
  return route.meta?.title || '概览'
})
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

onMounted(() => { document.addEventListener('keydown', onKeydown); document.addEventListener('click', onClickOutside); document.addEventListener('click', onUserMenuClickOutside); loadNotifications(); notifTimer = setInterval(loadNotifications, 60000); loadTenants(); loadAuthUser() })
onUnmounted(() => { document.removeEventListener('keydown', onKeydown); document.removeEventListener('click', onClickOutside); document.removeEventListener('click', onUserMenuClickOutside); clearInterval(notifTimer) })
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
.topbar-breadcrumb-tab { color: var(--text-secondary); }
.topbar-breadcrumb-link { text-decoration: none; transition: color var(--transition-fast); cursor: pointer; }
.topbar-breadcrumb-link:hover { color: var(--color-primary); }
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
/* 通知中心 */
.notif-wrap { position: relative; }
.notif-btn {
  position: relative; background: none; border: none; color: var(--text-secondary);
  cursor: pointer; padding: 4px 6px; border-radius: var(--radius-sm);
  transition: all var(--transition-fast);
}
.notif-btn:hover { background: var(--bg-elevated); color: var(--text-primary); }
.notif-badge {
  position: absolute; top: -2px; right: -4px; background: #EF4444; color: #fff;
  font-size: 10px; font-weight: 700; min-width: 16px; height: 16px;
  border-radius: 8px; display: flex; align-items: center; justify-content: center;
  padding: 0 4px; line-height: 1;
}
.notif-panel {
  position: absolute; top: 100%; right: 0; margin-top: 8px;
  width: 380px; max-height: 480px; overflow-y: auto;
  background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); box-shadow: var(--shadow-lg);
  z-index: 300;
}
.notif-panel-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--border-subtle);
  font-weight: 700; font-size: var(--text-sm); color: var(--text-primary);
}
.notif-mark-read {
  background: none; border: none; color: var(--color-primary);
  font-size: var(--text-xs); cursor: pointer; font-weight: 600;
}
.notif-mark-read:hover { text-decoration: underline; }
.notif-list { max-height: 400px; overflow-y: auto; }
.notif-item {
  display: flex; gap: var(--space-2); padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle); cursor: pointer;
  transition: background var(--transition-fast);
}
.notif-item:hover { background: var(--bg-elevated); }
.notif-unread { background: rgba(99, 102, 241, 0.05); }
.notif-severity { flex-shrink: 0; margin-top: 3px; font-size: 10px; }
.sev-critical { color: #DC2626; }
.sev-high { color: #EF4444; }
.sev-medium { color: #F59E0B; }
.sev-low { color: #10B981; }
.notif-content { flex: 1; min-width: 0; }
.notif-summary { font-size: var(--text-xs); font-weight: 600; color: var(--text-primary); }
.notif-detail { font-size: 11px; color: var(--text-tertiary); margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.notif-time { font-size: 10px; color: var(--text-disabled); margin-top: 2px; }
.notif-empty { padding: var(--space-6); text-align: center; font-size: var(--text-sm); color: var(--text-tertiary); }
/* v14.0: 租户切换器 */
.tenant-switcher { position: relative; }
.tenant-select {
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); color: var(--text-primary); padding: 4px 24px 4px 8px;
  font-size: var(--text-xs); font-family: var(--font-sans); cursor: pointer;
  outline: none; appearance: none; -webkit-appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%23999' stroke-width='2'%3E%3Cpolyline points='6 9 12 15 18 9'/%3E%3C/svg%3E");
  background-repeat: no-repeat; background-position: right 6px center;
  transition: border-color var(--transition-fast);
  max-width: 160px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.tenant-select:hover { border-color: var(--color-primary); }
.tenant-select:focus { border-color: var(--color-primary); }
/* v14.1: 用户菜单 */
.user-menu { position: relative; }
.user-btn {
  display: flex; align-items: center; gap: 6px;
  background: none; border: 1px solid transparent; color: var(--text-secondary);
  cursor: pointer; padding: 4px 8px; border-radius: var(--radius-md);
  font-size: var(--text-xs); font-family: var(--font-sans);
  transition: all var(--transition-fast);
}
.user-btn:hover { background: var(--bg-elevated); color: var(--text-primary); border-color: var(--border-default); }
.user-name { font-weight: 500; max-width: 80px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.user-role-badge {
  font-size: 10px; font-weight: 700; padding: 1px 5px; border-radius: 4px;
  text-transform: uppercase; letter-spacing: 0.3px;
}
.role-admin { background: rgba(99, 102, 241, 0.15); color: #818cf8; }
.role-operator { background: rgba(16, 185, 129, 0.15); color: #34d399; }
.role-viewer { background: rgba(156, 163, 175, 0.15); color: #9ca3af; }
.user-dropdown {
  position: absolute; top: 100%; right: 0; margin-top: 6px;
  min-width: 180px; background: var(--bg-surface); border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg); box-shadow: var(--shadow-lg); z-index: 300;
  overflow: hidden;
}
.user-dropdown-info { padding: 12px 14px; }
.user-dropdown-name { font-size: var(--text-sm); font-weight: 600; color: var(--text-primary); }
.user-dropdown-role { font-size: var(--text-xs); color: var(--text-tertiary); margin-top: 2px; }
.user-dropdown-divider { height: 1px; background: var(--border-subtle); }
.user-dropdown-item {
  display: flex; align-items: center; gap: 8px; width: 100%;
  background: none; border: none; color: var(--text-secondary);
  cursor: pointer; padding: 10px 14px; font-size: var(--text-sm);
  font-family: var(--font-sans); transition: all var(--transition-fast);
}
.user-dropdown-item:hover { background: var(--bg-elevated); color: #f87171; }

/* v15.0: TopNav Tabs */
.topnav-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 36px;
  flex-shrink: 0;
}

.topnav-tab {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 12px;
  height: 32px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 13px;
  font-family: var(--font-sans);
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
  position: relative;
  border-bottom: 2px solid transparent;
}

.topnav-tab:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.04);
}

.topnav-tab-active {
  background: rgba(99, 102, 241, 0.15);
  color: var(--color-primary);
  border-bottom: 2px solid var(--color-primary);
}

.topnav-tab-icon {
  font-size: 14px;
  line-height: 1;
}

.topnav-tab-label {
  font-weight: 500;
}

/* v15.0: 模式切换开关 */
.mode-toggle {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 12px;
  transition: all 0.2s ease;
  user-select: none;
}

.mode-toggle:hover {
  background: rgba(255, 255, 255, 0.04);
}

.mode-toggle-label {
  font-size: 12px;
  line-height: 1;
  opacity: 0.4;
  transition: opacity 0.2s ease;
}

.mode-toggle-label.mode-active {
  opacity: 1;
}

.mode-toggle-track {
  width: 24px;
  height: 14px;
  border-radius: 7px;
  background: rgba(139, 92, 246, 0.3);
  position: relative;
  transition: background 0.2s ease;
}

.mode-track-classic {
  background: rgba(99, 102, 241, 0.3);
}

.mode-toggle-thumb {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #a78bfa;
  position: absolute;
  top: 2px;
  left: 2px;
  transition: all 0.2s ease;
}

.mode-thumb-right {
  left: 12px;
  background: #818cf8;
}

@media(max-width:768px) {
  .topnav-tabs { display: none; }
  .mode-toggle { display: none; }
}
</style>
