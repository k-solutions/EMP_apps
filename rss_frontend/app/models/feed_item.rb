class FeedItem < ApplicationRecord
  belongs_to :feed_request

  validates :link, presence: true, uniqueness: { scope: :feed_request_id }

  scope :sorted, -> { order(publish_date: :desc) }
end
