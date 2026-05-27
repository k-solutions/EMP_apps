import React from 'react'
import { render, screen } from '@testing-library/react'
import { FeedList } from './FeedList'

describe('FeedList', () => {
  const mockItems = [
    {
      title: 'Article Old',
      source: 'BBC News',
      source_url: 'https://bbc.com/feed',
      link: 'https://bbc.com/1',
      publish_date: '2026-05-20',
      description: 'Older story'
    },
    {
      title: 'Article New',
      source: 'BBC News',
      source_url: 'https://bbc.com/feed',
      link: 'https://bbc.com/2',
      publish_date: '2026-05-24',
      description: 'Newer story'
    }
  ]

  it('renders "No feeds yet" when items are empty', () => {
    render(<FeedList items={[]} pendingRequests={[]} />)
    expect(screen.getByText(/no feeds yet/i)).toBeInTheDocument()
  })

  it('renders items sorted by publish_date descending', () => {
    render(<FeedList items={mockItems} pendingRequests={[]} />)
    const titles = screen.getAllByRole('heading', { level: 4 }).map(el => el.textContent)
    expect(titles).toEqual(['Article New', 'Article Old'])
  })

  it('renders active tasks / pending requests with status badges', () => {
    const mockPending = [
      {
        feed_request_id: 1,
        urls: ['https://test.com/rss'],
        status: 'pending'
      }
    ]
    render(<FeedList items={[]} pendingRequests={mockPending} />)
    expect(screen.getByText(/active tasks/i)).toBeInTheDocument()
    expect(screen.getByText(/parsing: https:\/\/test.com\/rss/i)).toBeInTheDocument()
    expect(screen.getByText('pending')).toBeInTheDocument()
  })
})
