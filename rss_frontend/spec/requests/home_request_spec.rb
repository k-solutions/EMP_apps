require 'rails_helper'

RSpec.describe "GET /", type: :request do
  it "returns 200 OK and mounts the React application layout shell" do
    get "/"
    expect(response).to have_http_status(:ok)
    expect(response.body).to include("application")
  end
end
