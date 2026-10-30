package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(host, port string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port),
		DB:   0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{
		client: client,
	}
}

func (c *RedisCache) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if err := json.Unmarshal(data, dst); err != nil {
		_ = c.client.Del(ctx, key)
		return false, err
	}

	return true, nil
}

func (c *RedisCache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if err := c.client.Set(ctx, key, string(data), ttl).Err(); err != nil {
		return err
	}

	return nil
}

func (c *RedisCache) DelJSON(ctx context.Context, key ...string) error {
	if len(key) == 0 {
		return nil
	}

	if err := c.client.Del(ctx, key...).Err(); err != nil {
		return err
	}

	return nil
}
