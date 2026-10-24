const TOKEN_KEY = 'watchparty.token'
const USER_KEY = 'watchparty.user'
const VIEWER_PREFIX = 'watchparty.viewer.'
const ACTIVE_PREFIX = 'watchparty.active.'

export const auth = {
  getToken: () => localStorage.getItem(TOKEN_KEY),
  setToken: (token) => localStorage.setItem(TOKEN_KEY, token),
  getUser: () => {
    try {
      return JSON.parse(localStorage.getItem(USER_KEY) || 'null')
    } catch {
      return null
    }
  },
  setUser: (user) => localStorage.setItem(USER_KEY, JSON.stringify(user)),
  clear: () => {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
  },
  // Per-session tokens issued when a viewer redeems a one-time invite link.
  getViewerToken: (sessionId) => (sessionId ? localStorage.getItem(VIEWER_PREFIX + sessionId) : null),
  setViewerToken: (sessionId, token) => localStorage.setItem(VIEWER_PREFIX + sessionId, token),
  clearViewerToken: (sessionId) => localStorage.removeItem(VIEWER_PREFIX + sessionId),
  // Session the streamer had open, so a reload keeps playing the same video.
  getActiveSession: (userId) => (userId ? localStorage.getItem(ACTIVE_PREFIX + userId) : null),
  setActiveSession: (userId, sessionId) =>
    localStorage.setItem(ACTIVE_PREFIX + userId, sessionId),
  clearActiveSession: (userId) => localStorage.removeItem(ACTIVE_PREFIX + userId),
}

async function request(path, { method = 'GET', body, headers = {}, token } = {}) {
  const bearer = token ?? auth.getToken()
  const isForm = body instanceof FormData
  const res = await fetch(path, {
    method,
    headers: {
      ...(body && !isForm ? { 'Content-Type': 'application/json' } : {}),
      ...(bearer ? { Authorization: `Bearer ${bearer}` } : {}),
      ...headers,
    },
    body: isForm ? body : body ? JSON.stringify(body) : undefined,
  })

  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`)
  return data
}

function uploadWithProgress(path, file, onProgress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const token = auth.getToken()

    xhr.open('POST', path)
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) onProgress(Math.round((e.loaded / e.total) * 100))
    }
    xhr.onload = () => {
      let data = {}
      try {
        data = JSON.parse(xhr.responseText)
      } catch {
        /* empty body */
      }
      if (xhr.status >= 200 && xhr.status < 300) resolve(data)
      else reject(new Error(data.error || `HTTP ${xhr.status}`))
    }
    xhr.onerror = () => reject(new Error('upload failed'))
    const formData = new FormData()
    formData.append('video', file)
    xhr.send(formData)
  })
}

export const api = {
  login: async (password) => {
    const data = await request('/api/auth/login', { method: 'POST', body: { password } })
    auth.setToken(data.token)
    auth.setUser(data.user)
    return data
  },

  logout: async () => {
    await request('/api/auth/logout', { method: 'POST' }).catch(() => {})
    auth.clear()
  },

  me: () => request('/api/me'),
  sessions: () => request('/api/sessions'),
  session: (id, token) => request(`/api/session/${id}`, { token }),
  updateSessionTitle: (id, title) =>
    request(`/api/session/${id}`, { method: 'PATCH', body: { title } }),
  deleteSession: (id) => request(`/api/session/${id}`, { method: 'DELETE' }),
  createInviteLink: (id) => request(`/api/session/${id}/invite`, { method: 'POST' }),
  currentInviteLink: (id) => request(`/api/session/${id}/invite`),
  exchangeInvite: (token) => request('/api/invite/exchange', { method: 'POST', body: { token } }),
  upload: (file, onProgress) => uploadWithProgress('/api/upload', file, onProgress),
}
