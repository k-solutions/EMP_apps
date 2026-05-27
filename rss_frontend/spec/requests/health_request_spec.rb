require 'rails_helper'

RSpec.describe "GET /api/v1/health", type: :request do
  let(:json) { JSON.parse(response.body) }

  it "returns 200 OK with health status and active mode" do
    get "/api/v1/health"

    expect(response).to have_http_status(:ok)
    expect(json["status"]).to eq("ok")
    expect(json["mode"]).to eq("full")
  end
end
