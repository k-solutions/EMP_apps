import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { FeedForm } from './FeedForm'

describe('FeedForm', () => {
  it('renders initial input field and parse button', () => {
    render(<FeedForm onSubmit={jest.fn()} loading={false} />)
    expect(screen.getByPlaceholderText('https://example.com/rss.xml')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /parse feeds/i })).toBeInTheDocument()
  })

  it('can add and remove additional URL inputs', () => {
    render(<FeedForm onSubmit={jest.fn()} loading={false} />)

    // Initial state: 1 input, no remove button
    expect(screen.getAllByPlaceholderText('https://example.com/rss.xml')).toHaveLength(1)
    expect(screen.queryByRole('button', { name: /remove/i })).not.toBeInTheDocument()

    // Click add button
    fireEvent.click(screen.getByRole('button', { name: /add another url/i }))
    expect(screen.getAllByPlaceholderText('https://example.com/rss.xml')).toHaveLength(2)

    // Remove buttons should be visible now
    const removeButtons = screen.getAllByRole('button', { name: /remove/i })
    expect(removeButtons).toHaveLength(2)

    // Remove the second input
    fireEvent.click(removeButtons[1])
    expect(screen.getAllByPlaceholderText('https://example.com/rss.xml')).toHaveLength(1)
  })

  it('calls onSubmit with list of URLs on form submission', () => {
    const mockOnSubmit = jest.fn()
    render(<FeedForm onSubmit={mockOnSubmit} loading={false} />)

    const input = screen.getByPlaceholderText('https://example.com/rss.xml')
    fireEvent.change(input, { target: { value: 'https://testfeed.com/rss' } })

    fireEvent.click(screen.getByRole('button', { name: /parse feeds/i }))
    expect(mockOnSubmit).toHaveBeenCalledWith(['https://testfeed.com/rss'])
  })

  it('disables submit button when inputs are empty or loading', () => {
    const { rerender } = render(<FeedForm onSubmit={jest.fn()} loading={false} />)
    // Empty input, button should be disabled
    expect(screen.getByRole('button', { name: /parse feeds/i })).toBeDisabled()

    // With input filled
    fireEvent.change(screen.getByPlaceholderText('https://example.com/rss.xml'), { target: { value: 'https://testfeed.com/rss' } })
    expect(screen.getByRole('button', { name: /parse feeds/i })).not.toBeDisabled()

    // Loading = true
    rerender(<FeedForm onSubmit={jest.fn()} loading={true} />)
    expect(screen.getByRole('button', { name: /parse feeds/i })).toBeDisabled()
  })
})
