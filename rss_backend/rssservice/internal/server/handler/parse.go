package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/emarchant/rssreader"
	"github.com/emarchant/rssservice/internal/cache"
	"github.com/emarchant/rssservice/internal/jobstore"
	"github.com/emarchant/rssservice/internal/rabbitmq"
	"github.com/emarchant/rssservice/internal/server/middleware"
	"github.com/oklog/ulid/v2"
)

type ParseRequest struct {
	URLs []string `json:"urls"`
}

type ParseResponse struct {
	JobID string `json:"job_id"`
}

type FallbackResponse struct {
	JobID  string             `json:"job_id"`
	Status string             `json:"status"`
	Items  []jobstore.RssItem `json:"items,omitempty"`
	Errors []jobstore.URLErr  `json:"errors,omitempty"`
	Error  string             `json:"error,omitempty"`
}

func Parse(
	reader rssreader.RssReader,
	store jobstore.JobStore,
	publisher *rabbitmq.Publisher,
	urlCache *cache.URLCache,
	isFallback bool,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ParseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.URLs) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad request: invalid JSON or empty urls list"}`))
			return
		}

		jobID := ulid.Make().String()
		middleware.SetLogJobID(r.Context(), jobID)

		// 1. Try Asynchronous Mode first if not in hard fallback
		if !isFallback && publisher != nil {
			job := &jobstore.Job{
				ID:     jobID,
				Status: "pending",
			}
			if err := store.Create(r.Context(), job); err == nil {
				// Try publishing to RabbitMQ command queue
				err = publisher.PublishCommand(r.Context(), jobID, req.URLs)
				if err == nil {
					// Async path succeeded!
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusAccepted)
					_ = json.NewEncoder(w).Encode(ParseResponse{JobID: jobID})
					return
				}
				// If publishing fails (broker exception/down), catch it and execute dynamic synchronous fallback
			}
		}

		// 2. Synchronous Fallback Mode (checks Redis Cache first)
		var finalItems []jobstore.RssItem
		var finalErrors []jobstore.URLErr
		var urlsToFetch []string

		// Check cache if active
		if urlCache != nil {
			for _, url := range req.URLs {
				cachedItems, err := urlCache.Get(r.Context(), url)
				if err == nil {
					finalItems = append(finalItems, cachedItems...)
				} else {
					urlsToFetch = append(urlsToFetch, url)
				}
			}
		} else {
			urlsToFetch = req.URLs
		}

		// Parse cache misses
		if len(urlsToFetch) > 0 {
			parsedItems, parseErr := reader.Parse(r.Context(), urlsToFetch)

			fetchedItemsByURL := make(map[string][]jobstore.RssItem)
			for _, item := range parsedItems {
				rssItem := jobstore.RssItem{
					Title:       item.Title,
					Source:      item.Source,
					SourceURL:   item.SourceURL,
					Link:        item.Link,
					PublishDate: item.PublishDate.String(),
					Description: item.Description,
				}
				finalItems = append(finalItems, rssItem)
				fetchedItemsByURL[item.SourceURL] = append(fetchedItemsByURL[item.SourceURL], rssItem)
			}

			var urlErrs []jobstore.URLErr

			if parseErr != nil {
				var pErr rssreader.ParseError
				if errors.As(parseErr, &pErr) {
					for _, ue := range pErr.Errors() {
						urlErr := jobstore.URLErr{
							URL:   ue.URL(),
							Error: ue.Error(),
						}
						urlErrs = append(urlErrs, urlErr)
						finalErrors = append(finalErrors, urlErr)
					}
				} else {
					for _, u := range urlsToFetch {
						urlErr := jobstore.URLErr{
							URL:   u,
							Error: parseErr.Error(),
						}
						urlErrs = append(urlErrs, urlErr)
						finalErrors = append(finalErrors, urlErr)
					}
				}
			}

			// Hydrate Cache
			if urlCache != nil {
				for _, u := range urlsToFetch {
					hasFailed := false
					for _, ue := range urlErrs {
						if ue.URL == u {
							hasFailed = true
							break
						}
					}
					if !hasFailed {
						itemsForURL := fetchedItemsByURL[u]
						if itemsForURL == nil {
							itemsForURL = []jobstore.RssItem{}
						}
						_ = urlCache.Set(r.Context(), u, itemsForURL)
					}
				}
			}
		}

		allFailed := len(finalItems) == 0 && len(finalErrors) >= len(req.URLs) && len(req.URLs) > 0
		status := "done"
		if allFailed {
			status = "failed"
		}

		resp := FallbackResponse{
			JobID:  jobID,
			Status: status,
		}
		if allFailed {
			resp.Error = "all URLs failed"
			resp.Errors = finalErrors
		} else {
			resp.Items = finalItems
			resp.Errors = finalErrors
		}

		// Save completed fallback job in the store so it can still be polled if clients check /jobs/{id}
		_ = store.Create(r.Context(), &jobstore.Job{
			ID:     jobID,
			Status: status,
			Items:  resp.Items,
			Errors: resp.Errors,
			Error:  resp.Error,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
