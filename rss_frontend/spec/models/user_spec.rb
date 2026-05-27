require 'rails_helper'

RSpec.describe User, type: :model do
  describe 'associations' do
    it { should have_many(:feed_requests).dependent(:destroy) }
  end
end
