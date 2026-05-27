class CreateFeedRequests < ActiveRecord::Migration[8.1]
  def change
    create_table :feed_requests do |t|
      t.bigint :user_id, null: false
      t.string :job_id
      t.text :urls, array: true, default: []
      t.string :status, null: false, default: "pending"
      t.string :mode, null: false, default: "full"
      t.timestamps
    end

    add_index :feed_requests, :user_id
    add_index :feed_requests, :job_id
    add_foreign_key :feed_requests, :users
  end
end
