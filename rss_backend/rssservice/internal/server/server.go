package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/emarchant/rssreader"
	"github.com/emarchant/rssservice/internal/auth"
	"github.com/emarchant/rssservice/internal/cache"
	"github.com/emarchant/rssservice/internal/docs"
	"github.com/emarchant/rssservice/internal/jobstore"
	"github.com/emarchant/rssservice/internal/rabbitmq"
	"github.com/emarchant/rssservice/internal/server/handler"
	"github.com/emarchant/rssservice/internal/server/middleware"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

func NewServer(
	port int,
	readTimeout, writeTimeout, idleTimeout time.Duration,
	reader rssreader.RssReader,
	store jobstore.JobStore,
	publisher *rabbitmq.Publisher,
	urlCache *cache.URLCache,
	validator *auth.JWTValidator,
	isFallback bool,
	logger *slog.Logger,
) *Server {
	r := chi.NewRouter()

	// Logging middleware
	r.Use(middleware.Logging(logger))
	r.Use(chiMiddleware.Recoverer)

	// Health check (unauthenticated)
	r.Get("/health", handler.Health(func() string {
		if isFallback {
			return "fallback"
		}
		return "full"
	}()))

	// OpenAPI spec raw handler (unauthenticated)
	r.Get("/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		data, err := docs.Assets.ReadFile("openapi.yaml")
		if err != nil {
			logger.Error("failed to read openapi.yaml", "err", err)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("spec not found"))
			return
		}
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write(data)
	})

	// Swagger UI asset file server (unauthenticated)
	fs := http.FileServer(http.FS(docs.Assets))
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		data, err := docs.Assets.ReadFile("swagger/index.html")
		if err != nil {
			logger.Error("failed to read index.html", "err", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
	r.Get("/docs/*", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/swagger" + r.URL.Path[len("/docs"):]
		fs.ServeHTTP(w, r)
	})

	// AsyncAPI spec raw handler (unauthenticated)
	r.Get("/docs/asyncapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		data, err := docs.Assets.ReadFile("asyncapi.yaml")
		if err != nil {
			logger.Error("failed to read asyncapi.yaml", "err", err)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("spec not found"))
			return
		}
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write(data)
	})

	// AsyncAPI interactive UI (unauthenticated)
	r.Get("/async-docs", func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>AsyncAPI Contract Documentation</title>
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <script src="https://unpkg.com/@asyncapi/web-component@1.0.0-next.47/lib/asyncapi-web-component.js" defer></script>
  <style>
    body {
      margin: 0;
      padding: 0;
      background-color: #0f172a;
      color: #f1f5f9;
      font-family: 'Inter', system-ui, -apple-system, sans-serif;
    }
    .header {
      background: linear-gradient(135deg, #1e1b4b 0%, #0f172a 100%);
      padding: 24px 40px;
      border-bottom: 1px solid #1e293b;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }
    .header h1 {
      margin: 0;
      font-size: 24px;
      background: linear-gradient(to right, #818cf8, #c084fc);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }
    .badge {
      background-color: #312e81;
      color: #a5b4fc;
      padding: 6px 12px;
      border-radius: 9999px;
      font-size: 12px;
      font-weight: 600;
      border: 1px solid #4338ca;
    }
    .container {
      padding: 20px 40px;
    }
    asyncapi-component {
      --asyncapi-theme-primary: #818cf8;
      --asyncapi-theme-background: #0f172a;
      --asyncapi-theme-text: #f1f5f9;
    }
  </style>
</head>
<body>
  <div class="header">
    <h1>RSS Service — Message Bus Contract</h1>
    <span class="badge">AsyncAPI 3.0.0</span>
  </div>
  <div class="container">
    <asyncapi-component
      schema-url="/docs/asyncapi.yaml"
      css-import="https://unpkg.com/@asyncapi/web-component@1.0.0-next.47/styles/default.min.css">
    </asyncapi-component>
  </div>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})

	// Authenticated routes
	r.Group(func(sub chi.Router) {
		sub.Use(middleware.Auth(validator))

		sub.Post("/parse", handler.Parse(reader, store, publisher, urlCache, isFallback))
		sub.Get("/jobs/{id}", handler.Jobs(store))
	})

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      r,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	return &Server{
		httpServer: httpServer,
		logger:     logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server gracefully")
	return s.httpServer.Shutdown(ctx)
}
