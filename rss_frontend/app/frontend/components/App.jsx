import React, { useState, useEffect, useCallback } from 'react'
import { BrowserRouter, Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom'
import { api } from '../api/client'
import { LoginForm } from './auth/LoginForm'
import { FeedForm } from './feeds/FeedForm'
import { FeedList } from './feeds/FeedList'
import { useFeedChannel } from '../hooks/useFeedChannel'
import { ErrorBanner } from './shared/ErrorBanner'

function Navigation({ userEmail, mode, onLogout }) {
  return (
    <nav className="navbar navbar-expand-lg navbar-dark shadow-sm mb-4" style={{ background: 'linear-gradient(135deg, #4F46E5 0%, #7C3AED 100%)' }}>
      <div className="container">
        <a className="navbar-brand fw-bold d-flex align-items-center" href="/">
          RSS Reader
        </a>
        <div className="d-flex align-items-center ms-auto">
          {mode && (
            <span
              className={`badge me-3 px-3 py-2 rounded-pill ${
                mode === 'full' ? 'bg-success-subtle text-success' : 'bg-warning-subtle text-warning'
              }`}
              data-mode={mode}
              style={{ fontSize: '0.8rem', border: '1px solid currentColor' }}
            >
              {mode === 'full' ? 'Live Mode' : 'Fallback Mode'}
            </span>
          )}
          {userEmail && (
            <>
              <span className="text-white-50 me-3 small d-none d-sm-inline">{userEmail}</span>
              <button onClick={onLogout} className="btn btn-outline-light btn-sm rounded-2 fw-semibold px-3">
                Sign out
              </button>
            </>
          )}
        </div>
      </div>
    </nav>
  )
}

function Dashboard({ feedItems, setFeedItems, mode, setMode }) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [pendingRequests, setPendingRequests] = useState([])

  // ActionCable messaging callback
  const handleBroadcastMessage = useCallback((data) => {
    // data: { feed_request_id, status, items, errors }
    setPendingRequests((prev) => {
      const match = prev.find((r) => r.feed_request_id === data.feed_request_id)
      if (!match) return prev
      
      // If done or failed, we remove or update the status
      return prev.map((req) => {
        if (req.feed_request_id === data.feed_request_id) {
          return { ...req, status: data.status }
        }
        return req
      })
    })

    if (data.status === 'done' || data.status === 'failed') {
      // After a small delay, clean up the pending list
      setTimeout(() => {
        setPendingRequests((prev) => prev.filter((r) => r.feed_request_id !== data.feed_request_id))
      }, 3000)
    }

    if (data.status === 'done' && data.items && data.items.length > 0) {
      setFeedItems((prev) => {
        const existingLinks = new Set(prev.map((item) => item.link))
        const newItems = data.items.filter((item) => !existingLinks.has(item.link))
        return [...newItems, ...prev]
      })
    }

    if (data.errors && data.errors.length > 0) {
      setError(`Some feeds failed to parse: ${data.errors.join(', ')}`)
    }
  }, [setFeedItems])

  // Subscribe to channel
  useFeedChannel(handleBroadcastMessage)

  const handleFeedSubmit = async (urls) => {
    setLoading(true)
    setError('')
    try {
      const res = await api.submitFeeds(urls)
      if (res.mode === 'full') {
        // Async: add to pending
        setPendingRequests((prev) => [
          ...prev,
          {
            feed_request_id: res.feed_request_id,
            job_id: res.job_id,
            urls: urls,
            status: res.status
          }
        ])
      } else {
        // Sync (fallback): items returned inline immediately
        if (res.items && res.items.length > 0) {
          setFeedItems((prev) => {
            const existingLinks = new Set(prev.map((item) => item.link))
            const newItems = res.items.filter((item) => !existingLinks.has(item.link))
            return [...newItems, ...prev]
          })
        }
        // Add a temporary completed request
        const reqId = res.feed_request_id
        setPendingRequests((prev) => [
          ...prev,
          {
            feed_request_id: reqId,
            job_id: res.job_id,
            urls: urls,
            status: 'done'
          }
        ])
        setTimeout(() => {
          setPendingRequests((prev) => prev.filter((r) => r.feed_request_id !== reqId))
        }, 3000)
      }
    } catch (err) {
      setError(err.message || 'Failed to submit feeds')
    } finally {
      setLoading(false)
    }
  }

  // Refresh items on load
  useEffect(() => {
    api.getFeedItems()
      .then((data) => setFeedItems(data.items || []))
      .catch((err) => setError('Failed to load feed items'))

    api.getHealth()
      .then((h) => setMode(h.mode))
      .catch(() => {})
  }, [setFeedItems, setMode])

  return (
    <div className="container py-2">
      {error && <ErrorBanner message={error} onClose={() => setError('')} />}
      <div className="row">
        <div className="col-lg-4 mb-4">
          <FeedForm onSubmit={handleFeedSubmit} loading={loading} />
        </div>
        <div className="col-lg-8">
          <FeedList items={feedItems} pendingRequests={pendingRequests} />
        </div>
      </div>
    </div>
  )
}

function MainApp() {
  const [authenticated, setAuthenticated] = useState(null)
  const [userEmail, setUserEmail] = useState('')
  const [feedItems, setFeedItems] = useState([])
  const [mode, setMode] = useState('full')
  const [flashMessage, setFlashMessage] = useState('')
  const navigate = useNavigate()
  const location = useLocation()

  // Bootstrap check on load
  useEffect(() => {
    api.getFeedItems()
      .then((data) => {
        setFeedItems(data.items || [])
        setAuthenticated(true)
        // Extract a fake or simple user email or read from user if we had user metadata
        setUserEmail('user@example.com')
        // Also fetch current mode
        api.getHealth().then((h) => setMode(h.mode)).catch(() => {})
      })
      .catch(() => {
        setAuthenticated(false)
      })
  }, [])

  const handleLogin = async (email, password) => {
    const data = await api.login(email, password)
    setUserEmail(email)
    setAuthenticated(true)
    setFlashMessage('Signed in successfully')
    
    // Refresh feeds
    const feedData = await api.getFeedItems()
    setFeedItems(feedData.items || [])

    // Refresh mode
    const health = await api.getHealth()
    setMode(health.mode)

    navigate('/feeds')

    setTimeout(() => {
      setFlashMessage('')
    }, 4000)
  }

  const handleLogout = async () => {
    try {
      await api.logout()
    } catch {}
    setAuthenticated(false)
    setUserEmail('')
    setFeedItems([])
    setFlashMessage('')
    navigate('/login')
  }

  if (authenticated === null) {
    return (
      <div className="d-flex align-items-center justify-content-center min-vh-100">
        <div className="spinner-border text-primary" role="status">
          <span className="visually-hidden">Loading...</span>
        </div>
      </div>
    )
  }

  return (
    <div className="min-vh-100 bg-light">
      {authenticated && (
        <Navigation userEmail={userEmail} mode={mode} onLogout={handleLogout} />
      )}
      {flashMessage && (
        <div className="container mt-3">
          <div className="alert alert-success alert-dismissible fade show" role="alert">
            {flashMessage}
            <button type="button" className="btn-close" data-bs-dismiss="alert" aria-label="Close" onClick={() => setFlashMessage('')}></button>
          </div>
        </div>
      )}
      <Routes>
        <Route
          path="/login"
          element={
            authenticated ? <Navigate to="/feeds" replace /> : <LoginForm onLoginSuccess={handleLogin} />
          }
        />
        <Route
          path="/feeds"
          element={
            authenticated ? (
              <Dashboard
                feedItems={feedItems}
                setFeedItems={setFeedItems}
                mode={mode}
                setMode={setMode}
              />
            ) : (
              <Navigate to="/login" replace />
            )
          }
        />
        <Route
          path="*"
          element={<Navigate to={authenticated ? "/feeds" : "/login"} replace />}
        />
      </Routes>
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <MainApp />
    </BrowserRouter>
  )
}
