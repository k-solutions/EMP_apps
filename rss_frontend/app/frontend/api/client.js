const defaultHeaders = {
  'Content-Type': 'application/json',
  'Accept': 'application/json'
}

export async function request(path, options = {}) {
  const url = path.startsWith('/api') ? path : `/api/v1${path}`
  const response = await fetch(url, {
    ...options,
    headers: {
      ...defaultHeaders,
      ...options.headers
    },
    credentials: 'include' // crucial for cookie session propagation
  })

  let body = null
  const contentType = response.headers.get('content-type')
  if (contentType && contentType.includes('application/json')) {
    body = await response.json()
  } else {
    body = await response.text()
  }

  if (!response.ok) {
    const errorMsg = (body && typeof body === 'object')
      ? (body.error || body.message || (body.errors && body.errors.join(', ')))
      : body
    throw new Error(errorMsg || 'Request failed')
  }

  return body
}

export const api = {
  login: (email, password) => request('/users/sign_in', {
    method: 'POST',
    body: JSON.stringify({ user: { email, password } })
  }),
  logout: () => request('/users/sign_out', {
    method: 'DELETE'
  }),
  submitFeeds: (urls) => request('/feeds', {
    method: 'POST',
    body: JSON.stringify({ urls })
  }),
  getFeedItems: () => request('/feed_items', {
    method: 'GET'
  }),
  getHealth: () => request('/health', {
    method: 'GET'
  })
}
