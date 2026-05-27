import React from 'react'

export function FeedItem({ item }) {
  const { title, source, source_url, link, publish_date, description } = item

  const truncateText = (text, maxLength = 250) => {
    if (!text) return ''
    if (text.length <= maxLength) return text
    return text.substring(0, maxLength) + '...'
  }

  const formatDate = (dateStr) => {
    if (!dateStr) return ''
    try {
      const date = new Date(dateStr)
      if (isNaN(date.getTime())) return dateStr
      return date.toISOString().split('T')[0]
    } catch {
      return dateStr
    }
  }

  return (
    <div className="card shadow-sm border-0 rounded-3 mb-3 feed-item bg-white overflow-hidden">
      <div className="card-body p-4">
        <div className="d-flex justify-content-between align-items-start mb-2">
          <span className="badge bg-light text-secondary border border-light-subtle fw-semibold mb-2" style={{ fontSize: '0.75rem' }}>
            {source}
          </span>
          <span className="text-muted small" data-publish-date={publish_date}>
            {formatDate(publish_date)}
          </span>
        </div>
        <h4 className="card-title fw-bold text-dark mb-2" style={{ fontSize: '1.15rem' }}>
          {title}
        </h4>
        <p className="card-text text-secondary mb-3" style={{ fontSize: '0.9rem', lineHeight: '1.5' }}>
          {truncateText(description)}
        </p>
        <div className="d-flex align-items-center justify-content-between mt-3 pt-3 border-top border-light-subtle">
          <a
            href={link}
            target="_blank"
            rel="noopener noreferrer"
            className="btn btn-link text-decoration-none p-0 fw-semibold text-primary d-inline-flex align-items-center"
            style={{ fontSize: '0.9rem' }}
          >
            Read more
          </a>
          {source_url && (
            <span className="text-secondary small text-truncate" style={{ maxWidth: '180px' }}>
              {source_url}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}
