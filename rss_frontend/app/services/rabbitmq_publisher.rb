class RabbitmqPublisher
  EXCHANGE = "rss_commands".freeze

  def initialize
    @conn    = Bunny.new(ENV.fetch("RABBITMQ_URL", "amqp://guest:guest@localhost:5672"))
    @conn.start
    @channel = @conn.create_channel
    @exchange = @channel.direct(EXCHANGE, durable: true)
  end

  def publish(routing_key:, payload:)
    @exchange.publish(
      payload,
      routing_key: routing_key,
      persistent:  true   # persistent messages for durability
    )
  end

  def close
    @conn.close
  rescue => e
    Rails.logger.warn("Failed to close RabbitMQ connection: #{e.message}")
  end
end
