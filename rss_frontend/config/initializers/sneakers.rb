require 'sneakers'

Sneakers.configure(
  amqp: ENV.fetch("RABBITMQ_URL", "amqp://guest:guest@localhost:5672"),
  vhost: "/",
  heartbeat: 30,
  workers: 2,
  threads: 2,
  log: "log/sneakers.log"
)
