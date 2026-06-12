require 'rails_helper'

RSpec.describe FeedItem, type: :model do
  describe 'associations' do
    it { should belong_to(:feed) }
  end

  describe 'validations' do
    it { should validate_presence_of(:link) }

    # Uniqueness check scoped to feed_id
    context 'uniqueness' do
      let(:feed) { Feed.create!(url: 'http://example.com', title: 'Example Feed') }

      it 'validates uniqueness of link scoped to feed_id' do
        FeedItem.create!(feed: feed, link: 'https://example.com/item1')
        duplicate = FeedItem.new(feed: feed, link: 'https://example.com/item1')
        expect(duplicate).not_to be_valid
        expect(duplicate.errors[:link]).to include('has already been taken')
      end
    end
  end
end
