require 'rails_helper'

RSpec.describe "POST /api/v1/feeds", type: :request do
  fixtures :users

  let(:headers) { { "Content-Type" => "application/json" } }
  let(:json) { JSON.parse(response.body) }

  context "when authenticated" do
    before do
      sign_in users(:alice)
      # Stub RabbitmqPublisher to avoid real AMQP connection in request spec
      @mock_publisher = instance_double(RabbitmqPublisher)
      allow(RabbitmqPublisher).to receive(:new).and_return(@mock_publisher)
      allow(@mock_publisher).to receive(:publish).and_return(true)
      allow(@mock_publisher).to receive(:close).and_return(true)
    end

    it "returns 202 with job_id and status processing" do
      post "/api/v1/feeds",
        params: { urls: ["https://feeds.bbci.co.uk/news/rss.xml"] }.to_json,
        headers: headers

      expect(response).to have_http_status(:accepted)
      expect(json["status"]).to eq("processing")
      expect(json["job_id"]).to be_present
      expect(json["mode"]).to eq("full")
    end

    it "creates a FeedRequest record" do
      expect {
        post "/api/v1/feeds",
          params: { urls: ["https://example.com/rss"] }.to_json,
          headers: headers
      }.to change(FeedRequest, :count).by(1)

      expect(FeedRequest.last.status).to eq("processing")
    end

    it "returns 422 when urls is empty" do
      post "/api/v1/feeds",
        params: { urls: [] }.to_json,
        headers: headers

      expect(response).to have_http_status(:unprocessable_entity)
    end

    it "returns 422 when urls is missing" do
      post "/api/v1/feeds",
        params: {}.to_json,
        headers: headers

      expect(response).to have_http_status(:unprocessable_entity)
    end
  end

  context "when unauthenticated" do
    it "returns 401" do
      post "/api/v1/feeds",
        params: { urls: ["https://example.com/rss"] }.to_json,
        headers: headers

      expect(response).to have_http_status(:unauthorized)
    end
  end
end
