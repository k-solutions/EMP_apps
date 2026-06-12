class FeedRequest < ApplicationRecord
  STATUSES = %w[pending processing done failed].freeze

  belongs_to :user
  has_and_belongs_to_many :feeds
  has_many :feed_items, through: :feeds

  validates :status, inclusion: { in: STATUSES }
  validates :urls,   presence: true
end
