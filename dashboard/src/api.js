/** @module api - API 调用封装（v14.0: 自动注入 tenant 参数） */
import { getCurrentTenant } from './stores/app.js'

const TOKEN_KEY = 'lobster_guard_token'

/** 获取保存的 Token */
export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

/** 保存 Token */
export function saveToken(token) {
  localStorage.setItem(TOKEN_KEY, token)
}

/** 清除 Token */
export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
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
 * 排除不需要 tenant 的路径（healthz、tenants管理、系统配置等）
 */
function injectTenantParam(path) {
  // 这些路径不注入 tenant（全局路由 or 租户管理本身）
  const skipPaths = ['/healthz', '/api/v1/tenants', '/api/v1/system/', '/api/v1/config', '/api/v1/demo/', '/api/v1/notifications', '/api/v1/llm/status', '/api/v1/llm/rules']
  for (const sp of skipPaths) {
    if (path.startsWith(sp)) return path
  }

  const tenant = getCurrentTenant()
  if (!tenant || tenant === 'default') return path // default 不注入，后端会默认
  const sep = path.includes('?') ? '&' : '?'
  return path + sep + 'tenant=' + encodeURIComponent(tenant)
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
  if (!res.ok) throw new Error('HTTP ' + res.status)
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
  const blob = await res.blob()
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = filename
  a.click()
  URL.revokeObjectURL(a.href)
}
