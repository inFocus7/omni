package cache

import (
	"errors"
	"sync"
	"time"
)

const DefaultTTL = 30 * time.Minute

var ExpiredEntryError = errors.New("entry expired")
var CacheMissError = errors.New("cache miss")

type Cache interface {
	Get(key string) (interface{}, error)
	Set(key string, value interface{}) error
}

type Entry[T any] struct {
	Value   T
	Created time.Time
}

type SimpleCache[T any] struct {
	data map[string]Entry[T]
	mu   sync.RWMutex
	ttl  time.Duration
}

var _ Cache = (*SimpleCache[any])(nil)

func NewSimpleCache[T any](ttl time.Duration) *SimpleCache[T] {
	return &SimpleCache[T]{
		data: make(map[string]Entry[T]),
		ttl:  ttl,
	}
}

func (c *SimpleCache[T]) Get(key string) (T, error) {
	var zero T
	if c.data == nil {
		return zero, CacheMissError
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if value, ok := c.data[key]; ok {
		// check that the entry is not expired
		if time.Since(value.Created) > c.ttl {
			return zero, ExpiredEntryError
		}

		return value.Value, nil
	}

	return zero, CacheMissError
}

func (c *SimpleCache[T]) Set(key string, value T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = Entry[T]{
		Value:   value,
		Created: time.Now(),
	}
	return nil
}
