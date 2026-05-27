class Api::V1::FeedsController < Api::BaseController
  # POST /api/v1/feeds
  def create
    urls = params[:urls]

    if urls.blank? || !urls.is_a?(Array)
      return render json: { error: "Urls parameter must be a non-empty array" }, status: :unprocessable_entity
    end

    job_id = SecureRandom.alphanumeric(26).upcase

    feed_request = current_user.feed_requests.create!(
      job_id: job_id,
      urls: urls,
      status: "pending"
    )

    publisher = RabbitmqPublisher.new
    begin
      publisher.publish(
        routing_key: "rss.commands.#{job_id}",
        payload:     { job_id: job_id, urls: urls }.to_json
      )
      feed_request.update!(status: "processing")

      render json: {
        feed_request_id: feed_request.id,
        job_id: job_id,
        status: "processing",
        mode: "full"
      }, status: :accepted
    rescue => e
      feed_request.destroy
      render json: { error: "Failed to publish command: #{e.message}" }, status: :internal_server_error
    ensure
      publisher.close
    end
  end
end
