class FeedRequest < ApplicationRecord
  STATUSES = %w[pending processing done failed].freeze

  belongs_to :user
  has_many :feed_items, dependent: :destroy

  validates :status, inclusion: { in: STATUSES }
  validates :urls,   presence: true
end
