class Api::V1::SessionsController < Devise::SessionsController
  include ActionController::Cookies
  respond_to :json
  skip_before_action :verify_authenticity_token, only: [ :create, :destroy ]

  # POST /api/v1/users/sign_in
  def create
    self.resource = warden.authenticate!(auth_options)
    sign_in(resource_name, resource)

    render json: { success: true, user: { email: resource.email } }, status: :ok
  rescue => e
    render json: { error: "Invalid email or password" }, status: :unauthorized
  end

  # DELETE /api/v1/users/sign_out
  def destroy
    Devise.sign_out_all_scopes ? sign_out : sign_out(resource_name)
    render json: { success: true }, status: :ok
  end

  protected

  def auth_options
    { scope: resource_name, recall: "#{controller_path}#new" }
  end
end
