require 'rails_helper'

RSpec.describe "Feed items display", type: :feature, js: true do
  fixtures :users, :feed_requests, :feed_items

  before { sign_in_as users(:alice) }

  scenario "user sees their cached feed items" do
    visit "/feeds"

    expect(page).to have_css(".feed-item", count: 2)
    expect(page).to have_text("Breaking News")
    expect(page).to have_text("Other Story")
  end

  scenario "items are displayed with correct fields" do
    visit "/feeds"

    within(".feed-item", text: "Breaking News") do
      expect(page).to have_text("BBC News")
      expect(page).to have_text("2026-05-23")
      expect(page).to have_text("Something happened today.")
      expect(page).to have_link("Read more", href: "https://bbc.co.uk/news/1")
    end
  end

  scenario "items from other users are not visible" do
    click_button "Sign out"
    sign_in_as users(:bob)
    visit "/feeds"

    expect(page).to have_css(".feed-item", count: 0)
    expect(page).to have_text("No feeds yet")
  end

  scenario "items are sorted newest first" do
    visit "/feeds"

    titles = all(".feed-item h4").map(&:text)
    expect(titles).to eq([ "Breaking News", "Other Story" ])
  end
end
