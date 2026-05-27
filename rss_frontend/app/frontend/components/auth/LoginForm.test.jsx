import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { LoginForm } from './LoginForm'

describe('LoginForm', () => {
  it('renders email and password inputs and submit button', () => {
    render(<LoginForm onLoginSuccess={jest.fn()} />)
    expect(screen.getByLabelText(/email address/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('submits email and password on form submission', async () => {
    const mockOnLoginSuccess = jest.fn()
    render(<LoginForm onLoginSuccess={mockOnLoginSuccess} />)

    fireEvent.change(screen.getByLabelText(/email address/i), { target: { value: 'test@example.com' } })
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'password123' } })
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(mockOnLoginSuccess).toHaveBeenCalledWith('test@example.com', 'password123')
    })
  })

  it('shows error banner when authentication fails', async () => {
    const mockOnLoginSuccess = jest.fn().mockRejectedValue(new Error('Invalid credentials'))
    render(<LoginForm onLoginSuccess={mockOnLoginSuccess} />)

    fireEvent.change(screen.getByLabelText(/email address/i), { target: { value: 'wrong@example.com' } })
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'wrong' } })
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText(/invalid credentials/i)).toBeInTheDocument()
    })
  })
})
