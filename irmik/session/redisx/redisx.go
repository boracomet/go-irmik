// Package redisx registers a Redis-backed session.Store.
//
// Blank-import to enable session.New / NewManager with Driver "redis":
//
//	import _ "github.com/boracomet/go-irmik/irmik/session/redisx"
//
// Or call Register() explicitly. The core irmik/session package stays free of go-redis.
package redisx

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/boracomet/go-irmik/irmik/session"
)

var registerOnce sync.Once

func init() {
	Register()
}

// Register wires the "redis" driver into session.New. Safe to call multiple times.
func Register() {
	registerOnce.Do(func() {
		session.Register("redis", func(opts session.Options) (session.Store, error) {
			return New(opts.RedisURL)
		})
	})
}

type redisStore struct {
	client *redis.Client
}

// New returns a Redis-backed session Store.
func New(redisURL string) (session.Store, error) {
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("session redis parse url: %w", err)
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("session redis ping: %w", err)
	}
	return &redisStore{client: client}, nil
}

type redisPayload struct {
	Values    map[string]any `json:"values"`
	Flash     map[string]any `json:"flash,omitempty"`
	ExpiresAt int64          `json:"expiresAt"`
}

func (s *redisStore) Get(ctx context.Context, id string) (session.Data, error) {
	raw, err := s.client.Get(ctx, redisKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return session.Data{}, session.ErrNotFound
		}
		return session.Data{}, err
	}
	var p redisPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return session.Data{}, err
	}
	d := session.Data{Values: p.Values, Flash: p.Flash}
	if p.ExpiresAt != 0 {
		d.ExpiresAt = time.Unix(0, p.ExpiresAt)
		if time.Now().After(d.ExpiresAt) {
			_ = s.Delete(ctx, id)
			return session.Data{}, session.ErrNotFound
		}
	}
	if d.Values == nil {
		d.Values = make(map[string]any)
	}
	return d, nil
}

func (s *redisStore) Save(ctx context.Context, id string, data session.Data) error {
	p := redisPayload{Values: data.Values, Flash: data.Flash}
	var ttl time.Duration
	if !data.ExpiresAt.IsZero() {
		p.ExpiresAt = data.ExpiresAt.UnixNano()
		ttl = time.Until(data.ExpiresAt)
		if ttl <= 0 {
			return s.Delete(ctx, id)
		}
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, redisKey(id), raw, ttl).Err()
}

func (s *redisStore) Delete(ctx context.Context, id string) error {
	return s.client.Del(ctx, redisKey(id)).Err()
}

func (s *redisStore) Close() error {
	return s.client.Close()
}

func redisKey(id string) string {
	return "irmik:session:" + id
}
