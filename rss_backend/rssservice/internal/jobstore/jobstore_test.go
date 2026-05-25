package jobstore

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestInMemoryJobStore(t *testing.T) {
	store := NewInMemoryJobStore()
	ctx := context.Background()

	// 1. Get non-existent
	_, err := store.Get(ctx, "non-existent")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}

	// 2. Create job
	job := &Job{
		ID:     "job-1",
		Status: "pending",
		Items:  []RssItem{{Title: "Item 1", Link: "https://example.com"}},
	}
	err = store.Create(ctx, job)
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// 3. Get job
	retrieved, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if retrieved.ID != "job-1" || retrieved.Status != "pending" || len(retrieved.Items) != 1 {
		t.Errorf("job details mismatch: %+v", retrieved)
	}

	// 4. Update job
	retrieved.Status = "processing"
	err = store.Update(ctx, retrieved)
	if err != nil {
		t.Fatalf("failed to update job: %v", err)
	}

	updated, err := store.Get(ctx, "job-1")
	if err != nil {
		t.Fatalf("failed to get updated job: %v", err)
	}
	if updated.Status != "processing" {
		t.Errorf("expected status = processing, got %s", updated.Status)
	}
}

func TestRedisJobStore(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("skipping Redis job store test: local Redis not running")
	}
	defer rdb.Close()

	store := NewRedisJobStore(rdb, 10*time.Second)

	// Clean up key
	rdb.Del(ctx, "job:job-redis-1")
	defer rdb.Del(ctx, "job:job-redis-1")

	// 1. Get non-existent
	_, err := store.Get(ctx, "job-redis-1")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}

	// 2. Create job
	job := &Job{
		ID:     "job-redis-1",
		Status: "pending",
		Items:  []RssItem{{Title: "Item Redis", Link: "https://example.com/redis"}},
	}
	err = store.Create(ctx, job)
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// 3. Get job
	retrieved, err := store.Get(ctx, "job-redis-1")
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if retrieved.ID != "job-redis-1" || retrieved.Status != "pending" || len(retrieved.Items) != 1 {
		t.Errorf("job details mismatch: %+v", retrieved)
	}

	// 4. Update job
	retrieved.Status = "done"
	err = store.Update(ctx, retrieved)
	if err != nil {
		t.Fatalf("failed to update job: %v", err)
	}

	updated, err := store.Get(ctx, "job-redis-1")
	if err != nil {
		t.Fatalf("failed to get updated job: %v", err)
	}
	if updated.Status != "done" {
		t.Errorf("expected status = done, got %s", updated.Status)
	}
}
