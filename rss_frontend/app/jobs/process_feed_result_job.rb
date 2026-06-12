class ProcessFeedResultJob
  include Sneakers::Worker

  from_queue "rss_results_rails",
    exchange:      "rss_results",
    exchange_type: :direct,
    routing_key:   "rss_results_rails",
    durable:       true,
    ack:           :manual

  def work(payload)
    data   = JSON.parse(payload)
    job_id = data["job_id"]

    request = FeedRequest.find_by(job_id: job_id)
    unless request
      Rails.logger.warn "unknown_job_id: #{job_id}"
      return :ack
    end

    # Idempotency guard — prevents double processing if message is delivered twice
    return :ack if request.status == "done" || request.status == "failed"

    # Optimistic lock — prevents duplicate processing
    updated = FeedRequest.where(job_id: job_id, status: [ "pending", "processing" ])
                         .update_all(status: data["status"])
    return :ack if updated == 0

    items  = data["items"]  || []
    errors = data["errors"] || []

    if items.any?
      items.group_by { |item| item["source_url"] }.each do |url, feed_items|
        source_name = feed_items.first["source"] || "RSS Source"
        feed = Feed.find_or_create_by!(url: url) do |f|
          f.title = source_name
        end

        request.feeds << feed unless request.feeds.include?(feed)

        feed_items_data = feed_items.map do |item|
          pub_date = item["publish_date"] || item["Date"]
          {
            feed_id:         feed.id,
            title:           item["title"],
            source:          item["source"],
            source_url:      item["source_url"],
            link:            item["link"],
            publish_date:    pub_date,
            description:     item["description"],
            created_at:      Time.current,
            updated_at:      Time.current
          }
        end
        FeedItem.insert_all(feed_items_data, unique_by: [ :feed_id, :link ])
      end
    end

    ActionCable.server.broadcast("feed_#{request.user_id}", {
      feed_request_id: request.id,
      status:          data["status"],
      items:           items,
      errors:          errors
    })

    :ack
  rescue => e
    Rails.logger.error "process_feed_result_error for job_id #{job_id}: #{e.message}"
    :reject # requeue for retry
  end
end
