import React from 'react'
import { FeedItem } from './FeedItem'
import { StatusBadge } from '../shared/StatusBadge'

export function FeedList({ items, pendingRequests }) {
  // Sort items by publish_date descending
  const sortedItems = [...items].sort((a, b) => {
    const da = new Date(a.publish_date)
    const db = new Date(b.publish_date)
    return db - da
  })

  return (
    <div className="feed-list-container">
      {pendingRequests && pendingRequests.length > 0 && (
        <div className="mb-4">
          <h6 className="fw-bold text-uppercase text-secondary mb-2 small" style={{ letterSpacing: '0.05em' }}>Active Tasks</h6>
          <div className="card shadow-sm border-0 rounded-3 p-3 bg-light">
            {pendingRequests.map((req) => (
              <div key={req.feed_request_id || req.job_id} className="d-flex justify-content-between align-items-center mb-2">
                <span className="text-secondary small text-truncate me-3" style={{ maxWidth: '70%' }}>
                  Parsing: {req.urls ? req.urls.join(', ') : 'RSS URLs'}
                </span>
                <StatusBadge status={req.status} />
              </div>
            ))}
          </div>
        </div>
      )}

      <h5 className="fw-bold mb-3 text-dark">Feed Stories</h5>
      {sortedItems.length === 0 ? (
        <div className="card shadow-sm border-0 rounded-3 p-5 text-center bg-white text-muted">
          <p className="mb-0">No feeds yet</p>
        </div>
      ) : (
        <div className="row">
          {sortedItems.map((item) => (
            <div key={item.link} className="col-12">
              <FeedItem item={item} />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
