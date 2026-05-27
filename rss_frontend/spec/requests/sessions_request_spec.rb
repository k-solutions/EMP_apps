require 'rails_helper'

RSpec.describe "User Sessions API", type: :request do
  fixtures :users

  let(:headers) { { "Content-Type" => "application/json" } }
  let(:json) { JSON.parse(response.body) }

  describe "POST /api/v1/users/sign_in" do
    it "logs in successfully with valid credentials" do
      post "/api/v1/users/sign_in.json",
        params: { user: { email: users(:alice).email, password: "password" } }.to_json,
        headers: headers

      expect(response).to have_http_status(:ok)
      expect(json["success"]).to be(true)
      expect(json["user"]["email"]).to eq(users(:alice).email)
    end

    it "returns 401 unauthorized with invalid credentials" do
      post "/api/v1/users/sign_in.json",
        params: { user: { email: users(:alice).email, password: "wrong_password" } }.to_json,
        headers: headers

      expect(response).to have_http_status(:unauthorized)
      expect(json["error"]).to be_present
    end
  end

  describe "DELETE /api/v1/users/sign_out" do
    before do
      sign_in users(:alice)
    end

    it "logs out successfully" do
      delete "/api/v1/users/sign_out.json", headers: headers

      expect(response).to have_http_status(:ok)
      expect(json["success"]).to be(true)
    end
  end
end
