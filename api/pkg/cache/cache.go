package cache

import (
	"context"
	"time"
)

// Cache defines the interface for cache operations
type Cache interface {
	// Get retrieves a value from the cache and unmarshals it into dest
	Get(ctx context.Context, key string, dest any) error

	// Set marshals and stores a value in the cache with expiration
	Set(ctx context.Context, key string, value any, expiration time.Duration) error

	// Delete removes a value from the cache
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache
	Exists(ctx context.Context, key string) (bool, error)

	// Close closes the cache connection
	Close() error

	// Ping checks if the cache is accessible
	Ping(ctx context.Context) error
}

// ErrCacheMiss is returned when a key is not found in the cache
type ErrCacheMiss struct {
	Key string
}

func (e ErrCacheMiss) Error() string {
	return "cache miss for key: " + e.Key
}
