package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/emarchant/rssservice/internal/jobstore"
	"github.com/emarchant/rssservice/internal/server/middleware"
	"github.com/go-chi/chi/v5"
)

func Jobs(store jobstore.JobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := chi.URLParam(r, "id")
		if jobID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad request: missing job ID"}`))
			return
		}

		middleware.SetLogJobID(r.Context(), jobID)

		job, err := store.Get(r.Context(), jobID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			if errors.Is(err, jobstore.ErrJobNotFound) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"job not found"}`))
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"failed to retrieve job"}`))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(job)
	}
}
