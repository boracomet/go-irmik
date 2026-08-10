// Package redisx registers a Redis-backed cache.Store.
//
// Blank-import to enable cache.New with Driver "redis":
//
//	import _ "github.com/boracomet/go-irmik/irmik/cache/redisx"
//
// Or call Register() explicitly. The core irmik/cache package stays free of go-redis.
package redisx

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/boracomet/go-irmik/irmik/cache"
)

var registerOnce sync.Once

func init() {
	Register()
}

// Register wires the "redis" driver into cache.New. Safe to call multiple times.
func Register() {
	registerOnce.Do(func() {
		cache.Register("redis", func(opts cache.Options) (cache.Store, error) {
			return New(opts.RedisURL)
		})
	})
}

type redisStore struct {
	client *redis.Client
}

type redisEntry struct {
	Body        []byte `json:"body"`
	ContentType string `json:"contentType"`
	ExpiresAt   int64  `json:"expiresAt"`
	StaleAt     int64  `json:"staleAt"`
}

// New returns a Redis-backed Store. redisURL may be empty (defaults to redis://localhost:6379/0).
func New(redisURL string) (cache.Store, error) {
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache redis parse url: %w", err)
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache redis ping: %w", err)
	}
	return &redisStore{client: client}, nil
}

func (s *redisStore) Get(ctx context.Context, key string) (cache.Entry, error) {
	data, err := s.client.Get(ctx, redisKey(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return cache.Entry{}, cache.ErrMiss
		}
		return cache.Entry{}, err
	}
	var re redisEntry
	if err := json.Unmarshal(data, &re); err != nil {
		return cache.Entry{}, err
	}
	e := entryFromRedis(re)
	if e.Expired() {
		_ = s.Delete(ctx, key)
		return cache.Entry{}, cache.ErrMiss
	}
	return e, nil
}

func (s *redisStore) Set(ctx context.Context, key string, entry cache.Entry) error {
	re := redisFromEntry(entry)
	data, err := json.Marshal(re)
	if err != nil {
		return err
	}
	var ttl time.Duration
	if !entry.ExpiresAt.IsZero() {
		ttl = time.Until(entry.ExpiresAt)
		if ttl <= 0 {
			return nil // already expired; do not store
		}
	}
	return s.client.Set(ctx, redisKey(key), data, ttl).Err()
}

func (s *redisStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, redisKey(key)).Err()
}

func (s *redisStore) Clear(ctx context.Context) error {
	iter := s.client.Scan(ctx, 0, "irmik:cache:*", 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 100 {
			if err := s.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			keys = keys[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		return s.client.Del(ctx, keys...).Err()
	}
	return nil
}

func (s *redisStore) Close() error {
	return s.client.Close()
}

func redisKey(key string) string {
	return "irmik:cache:" + key
}

func redisFromEntry(e cache.Entry) redisEntry {
	re := redisEntry{
		Body:        e.Body,
		ContentType: e.ContentType,
	}
	if !e.ExpiresAt.IsZero() {
		re.ExpiresAt = e.ExpiresAt.UnixNano()
	}
	if !e.StaleAt.IsZero() {
		re.StaleAt = e.StaleAt.UnixNano()
	}
	return re
}

func entryFromRedis(re redisEntry) cache.Entry {
	e := cache.Entry{
		Body:        re.Body,
		ContentType: re.ContentType,
	}
	if re.ExpiresAt != 0 {
		e.ExpiresAt = time.Unix(0, re.ExpiresAt)
	}
	if re.StaleAt != 0 {
		e.StaleAt = time.Unix(0, re.StaleAt)
	}
	return e
}
