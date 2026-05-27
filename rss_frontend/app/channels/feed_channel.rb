class FeedChannel < ApplicationCable::Channel
  def subscribed
    if current_user
      stream_from "feed_#{current_user.id}"
    else
      reject
    end
  end

  def unsubscribed
    # Any cleanup needed when channel is unsubscribed
  end
end
