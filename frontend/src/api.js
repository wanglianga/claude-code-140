// API 封装:token 存 localStorage
const TOKEN_KEY = 'pv_token'
const USER_KEY = 'pv_user'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}
export function getUser() {
  try {
    return JSON.parse(localStorage.getItem(USER_KEY) || 'null')
  } catch {
    return null
  }
}
export function saveAuth(token, user) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}
export function clearAuth() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

export async function api(path, { method = 'GET', body } = {}) {
  const headers = { 'Content-Type': 'application/json' }
  const tok = getToken()
  if (tok) headers.Authorization = 'Bearer ' + tok
  const res = await fetch(path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })
  const text = await res.text()
  let data = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = { error: text }
  }
  if (!res.ok) {
    throw new Error(data?.error || `请求失败(${res.status})`)
  }
  return data
}

export const get = (p) => api(p)
export const post = (p, body) => api(p, { method: 'POST', body })
export const put = (p, body) => api(p, { method: 'PUT', body })
