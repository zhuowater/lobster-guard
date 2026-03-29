/** @module api - API 调用封装（v14.1: JWT 认证 + 401 自动跳转 + tenant 注入） */
import { getCurrentTenant, getAuthToken, logoutUser, showToast } from './stores/app.js'

const TOKEN_KEY = 'lobster_guard_token'
const AUTH_TOKEN_KEY = 'lg_auth_token'

/** 获取保存的 Token（优先 JWT，降级旧 token） */
export function getToken() {
  return localStorage.getItem(AUTH_TOKEN_KEY) || localStorage.getItem(TOKEN_KEY) || ''
}

/** 保存 Token（兼容旧接口） */
export function saveToken(token) {
  // 同时保存到两个 key（兼容旧代码读取 lobster_guard_token）
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(AUTH_TOKEN_KEY, token)
}

/** 清除 Token */
export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(AUTH_TOKEN_KEY)
}

/** 构造认证头 */
function authHeaders() {
  const h = { 'Content-Type': 'application/json' }
  const t = getToken()
  if (t) h['Authorization'] = 'Bearer ' + t
  return h
}

/**
 * v14.0: 为 URL 自动添加 tenant 参数
 * 排除不需要 tenant 的路径
 */
function injectTenantParam(path) {
  const skipPaths = [
    '/healthz',
    '/api/v1/tenants',        // 租户管理本身是全局的
    '/api/v1/system/',         // 系统设置是全局的
    '/api/v1/config',          // 配置是全局的
    '/api/v1/demo/',           // Demo seed/clear 是全局的
    '/api/v1/auth/',           // 认证是全局的
    '/api/v1/layouts',         // 布局是用户级的，不是租户级
    '/api/v1/bigscreen/',      // 大屏聚合是全局的
    '/api/v1/op-audit',        // 操作审计是全局的
  ]
  for (const sp of skipPaths) {
    if (path.startsWith(sp)) return path
  }

  const tenant = getCurrentTenant()
  if (!tenant || tenant === 'default') return path
  const sep = path.includes('?') ? '&' : '?'
  return path + sep + 'tenant=' + encodeURIComponent(tenant)
}

/**
 * v14.1: 处理 401 响应 — 自动跳转登录页
 */
function handle401() {
  logoutUser()
  clearToken()
  // 如果不在登录页，跳转
  if (!window.location.hash.includes('/login')) {
    window.location.hash = '#/login'
  }
}

/**
 * 通用 API 请求
 * @param {string} path - API 路径
 * @param {RequestInit} [opts] - fetch 选项
 * @returns {Promise<any>}
 */
export async function api(path, opts = {}) {
  opts.headers = authHeaders()
  const url = location.origin + injectTenantParam(path)
  const res = await fetch(url, opts)
  if (res.status === 401) {
    handle401()
    showToast('登录已过期，请重新登录', 'error')
    throw new Error('Unauthorized')
  }
  if (!res.ok) {
    let message = 'HTTP ' + res.status
    try {
      const data = await res.clone().json()
      message = data?.error || data?.message || message
    } catch {
      try {
        const raw = await res.text()
        if (raw) message = raw
      } catch {}
    }
    showToast(message, 'error')
    throw new Error(message)
  }
  return res.json()
}

/**
 * POST 请求
 * @param {string} path
 * @param {object} body
 */
export async function apiPost(path, body) {
  return api(path, { method: 'POST', body: JSON.stringify(body) })
}

/**
 * PUT 请求
 * @param {string} path
 * @param {object} body
 */
export async function apiPut(path, body) {
  return api(path, { method: 'PUT', body: JSON.stringify(body) })
}

/**
 * PATCH 请求
 * @param {string} path
 * @param {object} body
 */
export async function apiPatch(path, body) {
  return api(path, { method: 'PATCH', body: JSON.stringify(body) })
}

/**
 * DELETE 请求（带 body）
 * @param {string} path
 * @param {object} body
 */
export async function apiDelete(path, body) {
  return api(path, { method: 'DELETE', body: body ? JSON.stringify(body) : undefined })
}

/**
 * 下载文件（blob）
 * @param {string} url - 完整 URL
 */
export async function downloadFile(url, filename) {
  const headers = {}
  const t = getToken()
  if (t) headers['Authorization'] = 'Bearer ' + t
  const res = await fetch(url, { headers })
  if (res.status === 401) {
    handle401()
    showToast('登录已过期，请重新登录', 'error')
    throw new Error('Unauthorized')
  }
  if (!res.ok) {
    const message = '下载失败：HTTP ' + res.status
    showToast(message, 'error')
    throw new Error(message)
  }
  const blob = await res.blob()
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = filename
  a.click()
  URL.revokeObjectURL(a.href)
}
