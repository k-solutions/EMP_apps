import React from 'react'

export function ErrorBanner({ message, onClose }) {
  if (!message) return null
  return (
    <div className="alert alert-danger alert-dismissible fade show error-banner" role="alert">
      <strong>Error:</strong> {message}
      {onClose && (
        <button
          type="button"
          className="btn-close"
          data-bs-dismiss="alert"
          aria-label="Close"
          onClick={onClose}
        ></button>
      )}
    </div>
  )
}
