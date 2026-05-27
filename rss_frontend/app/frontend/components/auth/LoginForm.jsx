import React, { useState } from 'react'
import { ErrorBanner } from '../shared/ErrorBanner'

export function LoginForm({ onLoginSuccess }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      await onLoginSuccess(email, password)
    } catch (err) {
      setError(err.message || 'Invalid email or password')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="d-flex align-items-center justify-content-center min-vh-100 bg-light">
      <div className="card shadow-lg border-0 rounded-4" style={{ maxWidth: '400px', width: '100%', overflow: 'hidden' }}>
        <div className="bg-primary text-white text-center py-4 px-3" style={{ background: 'linear-gradient(135deg, #4F46E5 0%, #7C3AED 100%)' }}>
          <h2 className="fw-bold mb-0">Welcome Back</h2>
          <p className="text-white-50 small mb-0 mt-1">Sign in to your RSS Reader account</p>
        </div>
        <div className="card-body p-4">
          {error && <ErrorBanner message={error} onClose={() => setError('')} />}

          <form onSubmit={handleSubmit}>
            <div className="mb-3">
              <label htmlFor="email" className="form-label text-secondary fw-semibold small">Email Address</label>
              <input
                type="email"
                id="email"
                className="form-control form-control-lg rounded-3 border-light-subtle shadow-sm fs-6"
                placeholder="name@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoComplete="email"
              />
            </div>

            <div className="mb-4">
              <label htmlFor="password" className="form-label text-secondary fw-semibold small">Password</label>
              <input
                type="password"
                id="password"
                className="form-control form-control-lg rounded-3 border-light-subtle shadow-sm fs-6"
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                autoComplete="current-password"
              />
            </div>

            <button
              type="submit"
              className="btn btn-primary btn-lg w-100 rounded-3 shadowfw-semibold py-2 transition-all"
              style={{
                background: 'linear-gradient(135deg, #4F46E5 0%, #7C3AED 100%)',
                border: 'none',
              }}
              disabled={loading}
            >
              {loading ? (
                <span className="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
              ) : null}
              Sign in
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
