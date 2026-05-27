import { useEffect } from 'react'
import consumer from '../cable'

export function useFeedChannel(onMessage) {
  useEffect(() => {
    const sub = consumer.subscriptions.create("FeedChannel", {
      received(data) {
        if (onMessage) {
          onMessage(data)
        }
      }
    })
    return () => {
      sub.unsubscribe()
    }
  }, [onMessage])
}
