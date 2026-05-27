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

    PublishFeedJob.perform_later(feed_request)

    render json: {
      feed_request_id: feed_request.id,
      job_id: job_id,
      status: "pending",
      mode: "full"
    }, status: :accepted
  end
end
