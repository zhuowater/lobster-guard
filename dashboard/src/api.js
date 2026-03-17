/** @module api - API 调用封装 */

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
 * 通用 API 请求
 * @param {string} path - API 路径
 * @param {RequestInit} [opts] - fetch 选项
 * @returns {Promise<any>}
 */
export async function api(path, opts = {}) {
  opts.headers = authHeaders()
  const res = await fetch(location.origin + path, opts)
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
  return api(path, { method: 'DELETE', body: JSON.stringify(body) })
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
