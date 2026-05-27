class CreateFeedItems < ActiveRecord::Migration[8.1]
  def change
    create_table :feed_items do |t|
      t.bigint :feed_request_id, null: false
      t.string :title
      t.string :source
      t.string :source_url
      t.string :link
      t.date :publish_date
      t.text :description
      t.timestamps
    end

    add_index :feed_items, :feed_request_id
    add_index :feed_items, [ :feed_request_id, :link ], unique: true
    add_foreign_key :feed_items, :feed_requests
  end
end
