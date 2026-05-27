import React from 'react'

const STATUS_CLASS = {
  pending:    "bg-warning text-dark",
  processing: "bg-info text-dark",
  done:       "bg-success text-white",
  failed:     "bg-danger text-white",
}

export function StatusBadge({ status }) {
  return (
    <span
      className={`badge ${STATUS_CLASS[status] ?? "bg-secondary text-white"}`}
      data-status={status}
    >
      {status}
    </span>
  )
}
