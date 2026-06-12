require 'rails_helper'

RSpec.describe Feed, type: :model do
  describe 'associations' do
    it { should have_and_belong_to_many(:feed_requests) }
    it { should have_many(:feed_items).dependent(:destroy) }
  end

  describe 'validations' do
    it { should validate_presence_of(:url) }

    context 'uniqueness' do
      it 'validates uniqueness of url' do
        Feed.create!(url: 'https://example.com/feed1')
        duplicate = Feed.new(url: 'https://example.com/feed1')
        expect(duplicate).not_to be_valid
        expect(duplicate.errors[:url]).to include('has already been taken')
      end
    end
  end
end
