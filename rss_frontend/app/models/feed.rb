class Feed < ApplicationRecord
  has_and_belongs_to_many :feed_requests
  has_many :feed_items, dependent: :destroy

  validates :url, presence: true, uniqueness: true
end
