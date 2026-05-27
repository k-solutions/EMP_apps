require 'rails_helper'

RSpec.describe FeedRequest, type: :model do
  describe 'associations' do
    it { should belong_to(:user) }
    it { should have_many(:feed_items).dependent(:destroy) }
  end

  describe 'validations' do
    it { should validate_presence_of(:urls) }
    it { should validate_inclusion_of(:status).in_array(%w[pending processing done failed]) }
  end
end
