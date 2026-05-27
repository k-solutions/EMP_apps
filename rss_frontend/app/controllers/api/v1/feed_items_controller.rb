class Api::V1::FeedItemsController < Api::BaseController
  # GET /api/v1/feed_items
  def index
    items = FeedItem
      .joins(:feed_request)
      .where(feed_requests: { user_id: current_user.id })
      .order(publish_date: :desc)
      .map do |item|
        {
          title: item.title,
          source: item.source,
          source_url: item.source_url,
          link: item.link,
          publish_date: item.publish_date&.to_s,
          description: item.description
        }
      end

    render json: { items: items }
  end
end
