import { renderHook } from '@testing-library/react'
import { useFeedChannel } from './useFeedChannel'
import consumer from '../cable'

jest.mock('../cable', () => {
  const mockSubscription = {
    unsubscribe: jest.fn()
  }
  return {
    subscriptions: {
      create: jest.fn(() => mockSubscription)
    }
  }
})

describe('useFeedChannel', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('subscribes on mount and unsubscribes on unmount', () => {
    const mockOnMessage = jest.fn()
    const { unmount } = renderHook(() => useFeedChannel(mockOnMessage))

    expect(consumer.subscriptions.create).toHaveBeenCalledWith('FeedChannel', expect.any(Object))

    const subscription = consumer.subscriptions.create.mock.results[0].value

    unmount()
    expect(subscription.unsubscribe).toHaveBeenCalled()
  })

  it('calls onMessage when message is received', () => {
    const mockOnMessage = jest.fn()
    renderHook(() => useFeedChannel(mockOnMessage))

    const createArgs = consumer.subscriptions.create.mock.calls[0]
    const handlers = createArgs[1]

    const data = { feed_request_id: 1, status: 'done' }
    handlers.received(data)

    expect(mockOnMessage).toHaveBeenCalledWith(data)
  })
})
