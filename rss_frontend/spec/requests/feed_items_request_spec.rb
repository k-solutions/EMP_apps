require 'rails_helper'

RSpec.describe "GET /api/v1/feed_items", type: :request do
  fixtures :users, :feed_requests, :feed_items

  let(:json) { JSON.parse(response.body) }

  context "when authenticated as alice" do
    before { sign_in users(:alice) }

    it "returns alice's feed items" do
      get "/api/v1/feed_items"

      expect(response).to have_http_status(:ok)
      expect(json["items"].length).to eq(2)
    end

    it "returns items sorted by publish_date descending" do
      get "/api/v1/feed_items"

      dates = json["items"].map { |i| Date.parse(i["publish_date"]) }
      expect(dates).to eq(dates.sort.reverse)
    end

    it "returns correct item fields" do
      get "/api/v1/feed_items"

      item = json["items"].first
      expect(item.keys).to match_array(
        %w[title source source_url link publish_date description]
      )
    end
  end

  context "when authenticated as bob (no items)" do
    before { sign_in users(:bob) }

    it "returns an empty items array" do
      get "/api/v1/feed_items"

      expect(response).to have_http_status(:ok)
      expect(json["items"]).to eq([])
    end
  end

  context "when unauthenticated" do
    it "returns 401" do
      get "/api/v1/feed_items"
      expect(response).to have_http_status(:unauthorized)
    end
  end
end
