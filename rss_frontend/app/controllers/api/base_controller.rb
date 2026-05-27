class Api::BaseController < ActionController::API
  include ActionController::Cookies
  include ActionController::MimeResponds

  # Force json format and authenticate user
  before_action :ensure_json_request
  before_action :authenticate_user!

  rescue_from ActiveRecord::RecordInvalid, with: :render_unprocessable_entity
  rescue_from StandardError, with: :render_internal_error

  private

  def ensure_json_request
    # Force JSON parsing
    request.format = :json
  end

  def render_unprocessable_entity(exception)
    render json: { errors: exception.record.errors.full_messages }, status: :unprocessable_entity
  end

  def render_internal_error(exception)
    Rails.logger.error("API error: #{exception.message}\n#{exception.backtrace.join("\n")}")
    render json: { error: "Internal Server Error", message: exception.message }, status: :internal_server_error
  end
end
