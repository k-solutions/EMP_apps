class PublishFeedJob < ApplicationJob
  queue_as :default

  def perform(feed_request)
    publisher = nil
    begin
      publisher = RabbitmqPublisher.new
      publisher.publish(
        routing_key: "rss_commands_worker",
        payload:     { job_id: feed_request.job_id, urls: feed_request.urls }.to_json
      )
      feed_request.update!(status: "processing")
    rescue => e
      Rails.logger.warn "RabbitMQ publishing failed. Dropping back to Fallback Mode: #{e.message}"
      execute_fallback(feed_request)
    ensure
      publisher.close rescue nil if publisher
    end
  end

  private

  def execute_fallback(feed_request)
    # Generate ES256 JWT
    token = JwtService.generate_token(user_id: feed_request.user_id)

    # Invoke Go REST API synchronously
    uri = URI("#{ENV.fetch('RSS_SERVICE_URL', 'http://localhost:8080')}/parse")
    req = Net::HTTP::Post.new(uri)
    req["Authorization"] = "Bearer #{token}"
    req["Content-Type"] = "application/json"
    req.body = { urls: feed_request.urls, job_id: feed_request.job_id }.to_json

    res = Net::HTTP.start(uri.hostname, uri.port, open_timeout: 5, read_timeout: 10) do |http|
      http.request(req)
    end

    if res.code == "200"
      data = JSON.parse(res.body)

      feed_request.update!(job_id: data["job_id"], status: data["status"])

      items = data["items"] || []
      errors = data["errors"] || []

      if items.any?
        items.group_by { |item| item["source_url"] }.each do |url, feed_items|
          source_name = feed_items.first["source"] || "RSS Source"
          feed = Feed.find_or_create_by!(url: url) do |f|
            f.title = source_name
          end

          feed_request.feeds << feed unless feed_request.feeds.include?(feed)

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

      ActionCable.server.broadcast("feed_#{feed_request.user_id}", {
        feed_request_id: feed_request.id,
        status:          data["status"],
        items:           items,
        errors:          errors
      })
    else
      raise "Go RSS parser fallback failed with HTTP #{res.code}"
    end
  rescue => e
    Rails.logger.error "Fallback path execution failed for FeedRequest ##{feed_request.id}: #{e.message}"
    feed_request.update!(status: "failed")
    ActionCable.server.broadcast("feed_#{feed_request.user_id}", {
      feed_request_id: feed_request.id,
      status:          "failed",
      items:           [],
      errors:          [ e.message ]
    })
  end
end
