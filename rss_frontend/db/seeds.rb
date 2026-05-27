# This file ensures the existence of records required to run the application in every environment.
# The code here is idempotent so that it can be executed safely at any point.

# 1. Create a default dev user
user = User.find_or_initialize_by(email: "user@example.com")
if user.new_record?
  user.password = "password"
  user.save!
  puts "Created default user: user@example.com / password"
else
  puts "Default user user@example.com already exists"
end

# 2. Create sample FeedRequest
feed_request = FeedRequest.find_or_create_by!(
  user: user,
  job_id: "01KSFT7Y2VGH5SXRKNBJ03JK5N",
  urls: ["https://feeds.bbci.co.uk/news/rss.xml"]
) do |req|
  req.status = "done"
end
puts "Created sample FeedRequest: #{feed_request.job_id}"

# 3. Create sample FeedItems
feed_items_data = [
  {
    feed_request: feed_request,
    title: "Google DeepMind releases new agentic AI coder",
    source: "BBC Tech",
    source_url: "https://feeds.bbci.co.uk/news/rss.xml",
    link: "https://example.com/deepmind-agentic-ai",
    publish_date: "2026-05-26",
    description: "Google DeepMind team makes a massive leap forward in Advanced Agentic Coding by releasing Antigravity, a fully agentic AI developer that simplifies complex microservice stacks."
  },
  {
    feed_request: feed_request,
    title: "RabbitMQ and Sneakers power high-throughput architectures",
    source: "BBC Tech",
    source_url: "https://feeds.bbci.co.uk/news/rss.xml",
    link: "https://example.com/rabbitmq-sneakers-throughput",
    publish_date: "2026-05-25",
    description: "Developers adopt RabbitMQ direct/topic exchanges and Sneakers workers for lightning-fast, non-blocking asynchronous event handling in modern web applications."
  },
  {
    feed_request: feed_request,
    title: "Simplicity in software engineering leads to massive success",
    source: "BBC News",
    source_url: "https://feeds.bbci.co.uk/news/rss.xml",
    link: "https://example.com/simplicity-software-engineering",
    publish_date: "2026-05-24",
    description: "A recent survey of cloud applications highlights that eliminating redundant layers, such as Sidekiq or extra Redis streams, leads to higher maintainability and fewer bugs."
  }
]

feed_items_data.each do |item_data|
  FeedItem.find_or_create_by!(
    feed_request: item_data[:feed_request],
    link: item_data[:link]
  ) do |item|
    item.title = item_data[:title]
    item.source = item_data[:source]
    item.source_url = item_data[:source_url]
    item.publish_date = item_data[:publish_date]
    item.description = item_data[:description]
  end
end
puts "Seed data loaded successfully! 🚀"
