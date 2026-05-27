class RemoveModeFromFeedRequests < ActiveRecord::Migration[8.1]
  def change
    remove_column :feed_requests, :mode, :string
  end
end
