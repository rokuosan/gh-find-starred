package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var (
	// ErrCacheMiss indicates no cache entry exists at the configured location.
	ErrCacheMiss = errors.New("cache miss")
	// ErrCacheExpired indicates the cached entry has exceeded its TTL.
	ErrCacheExpired = errors.New("cache expired")
)

// Store defines generic typed cache persistence operations.
type Store[T any] interface {
	Get() (T, error)
	Set(data T) error
}

// Option configures a FileStore.
type Option[T any] func(*FileStore[T])

// WithClock injects a custom clock function, primarily for deterministic testing.
func WithClock[T any](fn func() time.Time) Option[T] {
	return func(s *FileStore[T]) {
		if fn != nil {
			s.now = fn
		}
	}
}

type entry[T any] struct {
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Data      T         `json:"data"`
}

// FileStore persists JSON-encoded typed cache entries on the local filesystem
// with TTL validation and atomic writes.
type FileStore[T any] struct {
	path string
	ttl  time.Duration
	now  func() time.Time
}

// NewFileStore constructs a FileStore writing to path with the given TTL.
func NewFileStore[T any](path string, ttl time.Duration, opts ...Option[T]) *FileStore[T] {
	s := &FileStore[T]{
		path: path,
		ttl:  ttl,
		now:  time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Get reads and validates the cached value, returning ErrCacheMiss or ErrCacheExpired
// when no valid fresh entry is available.
func (s *FileStore[T]) Get() (T, error) {
	var zero T

	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return zero, ErrCacheMiss
		}
		return zero, fmt.Errorf("open cache file: %w", err)
	}
	defer f.Close()

	var e entry[T]
	if err := json.NewDecoder(f).Decode(&e); err != nil {
		return zero, fmt.Errorf("decode cache file: %w", err)
	}
	if e.ExpiresAt.IsZero() {
		return zero, errors.New("invalid cache file: missing expires_at")
	}
	if !s.now().Before(e.ExpiresAt) {
		return zero, ErrCacheExpired
	}

	return e.Data, nil
}

// Set writes data atomically to disk using a temporary file and rename.
func (s *FileStore[T]) Set(data T) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	now := s.now().UTC()
	e := entry[T]{
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
		Data:      data,
	}

	tmp, err := os.CreateTemp(dir, ".cache-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp cache file: %w", err)
	}
	tmpPath := tmp.Name()

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := json.NewEncoder(tmp).Encode(e); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp cache file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("rename cache file: %w", err)
	}

	cleanup = false
	return nil
}
