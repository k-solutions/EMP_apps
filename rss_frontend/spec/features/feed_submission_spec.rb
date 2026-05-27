require 'rails_helper'

RSpec.describe "Feed submission (RabbitMQ + ActionCable)",
               type: :feature, js: true do
  fixtures :users

  before do
    # Stub RabbitmqPublisher to avoid real AMQP connection in feature spec
    @mock_publisher = instance_double(RabbitmqPublisher)
    allow(RabbitmqPublisher).to receive(:new).and_return(@mock_publisher)
    allow(@mock_publisher).to receive(:publish).and_return(true)
    allow(@mock_publisher).to receive(:close).and_return(true)
    sign_in_as users(:alice)
  end

  scenario "user submits a URL and sees parsed items via ActionCable" do
    visit "/feeds"
    expect(page).to have_text("Submit RSS Feeds")

    page.execute_script("const el = document.getElementById('feed-url-input-1'); const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set; setter.call(el, 'https://feeds.bbci.co.uk/news/rss.xml'); el.dispatchEvent(new Event('input', { bubbles: true }));")
    click_button "Parse Feeds"

    # Optimistic UI — processing badge appears immediately
    expect(page).to have_css("[data-status='processing']")

    feed_request = FeedRequest.last
    expect(feed_request.status).to eq("processing")

    # Simulate worker broadcasting result via ActionCable
    ActionCable.server.broadcast(
      "feed_#{users(:alice).id}",
      {
        feed_request_id: feed_request.id,
        status: "done",
        items: [
          {
            "title" => "Cable Story",
            "link" => "https://example.com/cable-1",
            "source" => "BBC News",
            "source_url" => "https://feeds.bbci.co.uk/news/rss.xml",
            "publish_date" => "2026-05-24",
            "description" => "Broadcasted over ActionCable"
          }
        ],
        errors: []
      }
    )

    # Wait for the item to render on page
    expect(page).to have_css(".feed-item", count: 1)
    expect(page).to have_text("Cable Story")
    expect(page).to have_text("Broadcasted over ActionCable")
  end
end
