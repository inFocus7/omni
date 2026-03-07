package cache

import (
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	data   map[string]Entry[T]
	mu     sync.RWMutex
	ttl    time.Duration
	logger *zerolog.Logger
}

var _ Cache = (*SimpleCache[any])(nil)

func NewSimpleCache[T any](ttl time.Duration) *SimpleCache[T] {
	logger := log.With().Fields(map[string]interface{}{
		"component": "cache",
	}).Logger()

	return &SimpleCache[T]{
		data:   make(map[string]Entry[T]),
		ttl:    ttl,
		logger: &logger,
	}
}

func (c *SimpleCache[T]) Get(key string) (T, error) {
	var zero T
	if c.data == nil {
		c.logger.Error().Msg("cache is nil")
		return zero, CacheMissError
	}

	c.mu.RLock()
	value, ok := c.data[key]
	c.mu.RUnlock()

	loggerFields := map[string]interface{}{
		"key": key,
	}

	if !ok {
		c.logger.Debug().Fields(loggerFields).Msg("cache miss")
		return zero, CacheMissError
	}

	if time.Since(value.Created) > c.ttl {
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		c.logger.Debug().Fields(loggerFields).Msg("cache expired")
		return zero, ExpiredEntryError
	}

	c.logger.Debug().Fields(loggerFields).Msg("cache hit")
	return value.Value, nil
}

func (c *SimpleCache[T]) Set(key string, value T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = Entry[T]{
		Value:   value,
		Created: time.Now(),
	}
	c.logger.Debug().Fields(map[string]interface{}{
		"key": key,
	}).Msg("cache set")
	return nil
}
