class FeedItem < ApplicationRecord
  belongs_to :feed

  validates :link, presence: true, uniqueness: { scope: :feed_id }

  scope :sorted, -> { order(publish_date: :desc) }
end
