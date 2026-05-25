package jobstore

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrJobNotFound = errors.New("job not found")

type RssItem struct {
	Title       string `json:"title"`
	Source      string `json:"source"`
	SourceURL   string `json:"source_url"`
	Link        string `json:"link"`
	PublishDate string `json:"publish_date"` // YYYY-MM-DD
	Description string `json:"description"`
}

type URLErr struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type Job struct {
	ID     string    `json:"job_id"`
	Status string    `json:"status"` // pending | processing | done | failed
	Items  []RssItem `json:"items,omitempty"`
	Errors []URLErr  `json:"errors,omitempty"`
	Error  string    `json:"error,omitempty"`
}

type JobStore interface {
	Create(ctx context.Context, job *Job) error
	Get(ctx context.Context, id string) (*Job, error)
	Update(ctx context.Context, job *Job) error
}

// RedisJobStore implements JobStore using Redis.
type RedisJobStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisJobStore(client *redis.Client, ttl time.Duration) *RedisJobStore {
	return &RedisJobStore{
		client: client,
		ttl:    ttl,
	}
}

func (s *RedisJobStore) Create(ctx context.Context, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, "job:"+job.ID, data, s.ttl).Err()
}

func (s *RedisJobStore) Get(ctx context.Context, id string) (*Job, error) {
	data, err := s.client.Get(ctx, "job:"+id).Result()
	if err == redis.Nil {
		return nil, ErrJobNotFound
	} else if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *RedisJobStore) Update(ctx context.Context, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, "job:"+job.ID, data, s.ttl).Err()
}

// InMemoryJobStore implements JobStore in memory as a fallback.
type InMemoryJobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

func NewInMemoryJobStore() *InMemoryJobStore {
	return &InMemoryJobStore{
		jobs: make(map[string]*Job),
	}
}

func (s *InMemoryJobStore) Create(ctx context.Context, job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Deep copy the job to avoid side effects
	var copied Job
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &copied); err != nil {
		return err
	}
	s.jobs[job.ID] = &copied
	return nil
}

func (s *InMemoryJobStore) Get(ctx context.Context, id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	// Deep copy to prevent concurrent modification of returned job
	var copied Job
	data, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &copied); err != nil {
		return nil, err
	}
	return &copied, nil
}

func (s *InMemoryJobStore) Update(ctx context.Context, job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[job.ID]; !ok {
		return ErrJobNotFound
	}
	var copied Job
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &copied); err != nil {
		return err
	}
	s.jobs[job.ID] = &copied
	return nil
}
