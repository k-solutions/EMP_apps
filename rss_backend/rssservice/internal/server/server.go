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
