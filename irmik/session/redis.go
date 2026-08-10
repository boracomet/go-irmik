package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	client *redis.Client
}

// NewRedis returns a Redis-backed session Store.
func NewRedis(redisURL string) (Store, error) {
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

func (s *redisStore) Get(ctx context.Context, id string) (Data, error) {
	raw, err := s.client.Get(ctx, redisKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return Data{}, ErrNotFound
		}
		return Data{}, err
	}
	var p redisPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Data{}, err
	}
	d := Data{Values: p.Values, Flash: p.Flash}
	if p.ExpiresAt != 0 {
		d.ExpiresAt = time.Unix(0, p.ExpiresAt)
		if time.Now().After(d.ExpiresAt) {
			_ = s.Delete(ctx, id)
			return Data{}, ErrNotFound
		}
	}
	if d.Values == nil {
		d.Values = make(map[string]any)
	}
	return d, nil
}

func (s *redisStore) Save(ctx context.Context, id string, data Data) error {
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
