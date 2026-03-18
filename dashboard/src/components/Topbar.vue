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
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api.js'

defineEmits(['toggleMobile'])
const appState = inject('appState')
const route = useRoute()
const router = useRouter()
const searchInput = ref(null)

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
  else if (n.type === 'high_risk_tool') router.push('/agent')
  notifOpen.value = false
}
async function loadNotifications() { try { const d = await api('/api/v1/notifications'); notifications.value = d.notifications || [] } catch { notifications.value = [] } }
function fmtTime(ts) { if (!ts) return ''; const d = new Date(ts); return isNaN(d.getTime()) ? '' : d.toLocaleString('zh-CN', { hour12: false }) }
function onClickOutside(e) { if (notifWrap.value && !notifWrap.value.contains(e.target)) notifOpen.value = false }
let notifTimer = null

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

onMounted(() => { document.addEventListener('keydown', onKeydown); document.addEventListener('click', onClickOutside); loadNotifications(); notifTimer = setInterval(loadNotifications, 60000) })
onUnmounted(() => { document.removeEventListener('keydown', onKeydown); document.removeEventListener('click', onClickOutside); clearInterval(notifTimer) })
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
</style>
