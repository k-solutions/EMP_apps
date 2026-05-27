# config/initializers/rabbitmq.rb
begin
  conn = Bunny.new(ENV.fetch("RABBITMQ_URL", "amqp://guest:guest@localhost:5672"))
  conn.start
  conn.close
rescue => e
  warn "========================================================="
  warn "FATAL: RabbitMQ is not available. Exiting."
  warn "Error details: #{e.message}"
  warn "========================================================="
  exit 1
end
