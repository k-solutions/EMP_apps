import React from 'react'
import { render, screen } from '@testing-library/react'
import { FeedItem } from './FeedItem'

describe('FeedItem', () => {
  const item = {
    title: 'Test Story Title',
    source: 'BBC News',
    source_url: 'https://bbc.com/feed',
    link: 'https://bbc.com/story-123',
    publish_date: '2026-05-24T12:34:56.000Z',
    description: 'This is a test description of the story.'
  }

  it('renders all required fields', () => {
    render(<FeedItem item={item} />)
    expect(screen.getByText('Test Story Title')).toBeInTheDocument()
    expect(screen.getByText('BBC News')).toBeInTheDocument()
    expect(screen.getByText('This is a test description of the story.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /read more/i })).toHaveAttribute('href', 'https://bbc.com/story-123')
  })

  it('formats publish_date as YYYY-MM-DD', () => {
    render(<FeedItem item={item} />)
    expect(screen.getByText('2026-05-24')).toBeInTheDocument()
  })

  it('truncates description if it is longer than 250 characters', () => {
    const longDescription = 'a'.repeat(300)
    const longItem = { ...item, description: longDescription }
    render(<FeedItem item={longItem} />)

    const truncatedText = 'a'.repeat(250) + '...'
    expect(screen.getByText(truncatedText)).toBeInTheDocument()
  })
})
