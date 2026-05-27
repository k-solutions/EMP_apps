import React, { useState } from 'react'

export function FeedForm({ onSubmit, loading }) {
  const [urls, setUrls] = useState([''])

  const handleUrlChange = (index, value) => {
    const newUrls = [...urls]
    newUrls[index] = value
    setUrls(newUrls)
  }

  const addField = () => {
    setUrls([...urls, ''])
  }

  const removeField = (index) => {
    if (urls.length > 1) {
      setUrls(urls.filter((_, i) => i !== index))
    }
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    const activeUrls = urls.map(u => u.trim()).filter(Boolean)
    if (activeUrls.length > 0) {
      onSubmit(activeUrls)
    }
  }

  const isSubmitDisabled = loading || urls.map(u => u.trim()).filter(Boolean).length === 0

  return (
    <form onSubmit={handleSubmit} className="mb-4">
      <div className="card shadow-sm border-0 rounded-3 p-4 bg-white">
        <h5 className="fw-bold mb-3 text-dark">Submit RSS Feeds</h5>

        {urls.map((url, index) => (
          <div key={index} className="input-group mb-3">
            <label htmlFor={`feed-url-input-${index + 1}`} className="input-group-text bg-light border-0 text-secondary small">Feed URL {index + 1}</label>
            <input
              type="url"
              id={`feed-url-input-${index + 1}`}
              className="form-control border-light-subtle shadow-none"
              placeholder="https://example.com/rss.xml"
              value={url}
              onChange={(e) => handleUrlChange(index, e.target.value)}
              required
            />
            {urls.length > 1 && (
              <button
                type="button"
                className="btn btn-outline-danger border-light-subtle"
                onClick={() => removeField(index)}
              >
                Remove
              </button>
            )}
          </div>
        ))}

        <div className="d-flex justify-content-between align-items-center mt-2">
          <button
            type="button"
            className="btn btn-outline-secondary btn-sm rounded-2 fw-semibold px-3"
            onClick={addField}
            disabled={loading}
          >
            Add another URL
          </button>

          <button
            type="submit"
            className="btn btn-primary btn-md rounded-2 fw-semibold px-4"
            disabled={isSubmitDisabled}
          >
            {loading ? (
              <span className="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
            ) : null}
            Parse Feeds
          </button>
        </div>
      </div>
    </form>
  )
}
