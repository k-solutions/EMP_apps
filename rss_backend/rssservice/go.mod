module github.com/emarchant/rssservice

go 1.26.2

replace github.com/emarchant/rssreader => ../rss_reader

require (
	github.com/emarchant/rssreader v0.0.0-00010101000000-000000000000
	github.com/go-chi/chi/v5 v5.3.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/oklog/ulid/v2 v2.1.1
	github.com/redis/go-redis/v9 v9.19.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/rabbitmq/amqp091-go v1.11.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
)
