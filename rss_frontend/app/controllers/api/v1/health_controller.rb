class Api::V1::HealthController < Api::BaseController
  skip_before_action :authenticate_user!

  # GET /api/v1/health
  def index
    render json: {
      status: "ok",
      mode: "full"
    }
  end
end
