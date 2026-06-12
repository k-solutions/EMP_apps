class CreateFeedsAndAssociateFeedItems < ActiveRecord::Migration[8.1]
  def change
    create_table :feeds do |t|
      t.string :url, null: false
      t.string :title
      t.timestamps
    end
    add_index :feeds, :url, unique: true

    create_join_table :feed_requests, :feeds do |t|
      t.index [ :feed_request_id, :feed_id ], unique: true
      t.index [ :feed_id, :feed_request_id ]
    end

    # Modify feed_items
    remove_foreign_key :feed_items, :feed_requests
    remove_index :feed_items, column: [ :feed_request_id, :link ]
    remove_index :feed_items, column: :feed_request_id
    remove_column :feed_items, :feed_request_id, :bigint

    add_reference :feed_items, :feed, null: false, foreign_key: true
    add_index :feed_items, [ :feed_id, :link ], unique: true
  end
end
