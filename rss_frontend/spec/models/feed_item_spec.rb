require 'rails_helper'

RSpec.describe FeedItem, type: :model do
  describe 'associations' do
    it { should belong_to(:feed_request) }
  end

  describe 'validations' do
    it { should validate_presence_of(:link) }

    # Uniqueness check scoped to feed_request_id
    context 'uniqueness' do
      let(:user) { User.create!(email: 'test@example.com', password: 'password') }
      let(:feed_request) { FeedRequest.create!(user: user, urls: [ 'http://example.com' ], status: 'pending') }

      it 'validates uniqueness of link scoped to feed_request_id' do
        FeedItem.create!(feed_request: feed_request, link: 'https://example.com/item1')
        duplicate = FeedItem.new(feed_request: feed_request, link: 'https://example.com/item1')
        expect(duplicate).not_to be_valid
        expect(duplicate.errors[:link]).to include('has already been taken')
      end
    end
  end
end
