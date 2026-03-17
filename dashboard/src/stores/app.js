/** @module stores/app - 全局状态管理（reactive + provide/inject） */
import { reactive } from 'vue'

/** 全局应用状态 */
export const appState = reactive({
  /** @type {object|null} 健康检查数据 */
  health: null,
  /** @type {string} 连接状态: connected | disconnected | degraded */
  connectionStatus: 'disconnected',
  /** @type {string} 版本号 */
  version: '--',
  /** @type {string} 运行时间 */
  uptime: '--',
  /** @type {boolean} 侧边栏折叠 */
  sidebarCollapsed: localStorage.getItem('sidebar_collapsed') === '1',
  /** @type {object|null} toast 消息 */
  toast: null,
})

/** 更新健康数据 */
export function updateHealth(data) {
  appState.health = data
  appState.version = data.version ? 'v' + data.version : '--'
  appState.uptime = data.uptime || '--'
  const status = data.status
  appState.connectionStatus = status === 'healthy' ? 'connected' : (status === 'degraded' ? 'degraded' : 'disconnected')
}

/** 设置连接断开 */
export function setDisconnected() {
  appState.connectionStatus = 'disconnected'
}

/** 切换侧边栏 */
export function toggleSidebar() {
  appState.sidebarCollapsed = !appState.sidebarCollapsed
  localStorage.setItem('sidebar_collapsed', appState.sidebarCollapsed ? '1' : '0')
}

/** 显示 toast */
let _toastTimer = null
export function showToast(message, type = '') {
  appState.toast = { message, type }
  clearTimeout(_toastTimer)
  _toastTimer = setTimeout(() => { appState.toast = null }, 3000)
}

/** Vue 插件：provide appState */
export const appStorePlugin = {
  install(app) {
    app.provide('appState', appState)
    app.provide('showToast', showToast)
  }
}
