require 'rails_helper'

RSpec.describe FeedChannel, type: :channel do
  fixtures :users

  it "subscribes to stream" do
    stub_connection current_user: users(:alice)
    subscribe
    expect(subscription).to be_confirmed
    expect(subscription).to have_stream_from("feed_#{users(:alice).id}")
  end

  it "rejects when unauthenticated" do
    stub_connection current_user: nil
    subscribe
    expect(subscription).to be_rejected
  end
end
