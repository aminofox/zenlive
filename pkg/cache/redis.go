package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements Cache using Redis
type RedisCache struct {
	client     *redis.Client
	keyPrefix  string
	defaultTTL time.Duration

	// Stats
	hits   int64
	misses int64
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(client *redis.Client, keyPrefix string, defaultTTL time.Duration) *RedisCache {
	if defaultTTL == 0 {
		defaultTTL = 5 * time.Minute
	}

	return &RedisCache{
		client:     client,
		keyPrefix:  keyPrefix,
		defaultTTL: defaultTTL,
	}
}

// Get retrieves a value from Redis
func (rc *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	fullKey := rc.getKey(key)

	data, err := rc.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			rc.misses++
			return nil, errors.New("key not found")
		}
		return nil, err
	}

	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	rc.hits++
	return value, nil
}

// GetString retrieves a string value from Redis
func (rc *RedisCache) GetString(ctx context.Context, key string) (string, error) {
	fullKey := rc.getKey(key)

	value, err := rc.client.Get(ctx, fullKey).Result()
	if err != nil {
		if err == redis.Nil {
			rc.misses++
			return "", errors.New("key not found")
		}
		return "", err
	}

	rc.hits++
	return value, nil
}

// GetBytes retrieves raw bytes from Redis
func (rc *RedisCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	fullKey := rc.getKey(key)

	data, err := rc.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			rc.misses++
			return nil, errors.New("key not found")
		}
		return nil, err
	}

	rc.hits++
	return data, nil
}

// Set stores a value in Redis
func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	fullKey := rc.getKey(key)

	if ttl == 0 {
		ttl = rc.defaultTTL
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return rc.client.Set(ctx, fullKey, data, ttl).Err()
}

// SetString stores a string value in Redis
func (rc *RedisCache) SetString(ctx context.Context, key string, value string, ttl time.Duration) error {
	fullKey := rc.getKey(key)

	if ttl == 0 {
		ttl = rc.defaultTTL
	}

	return rc.client.Set(ctx, fullKey, value, ttl).Err()
}

// SetBytes stores raw bytes in Redis
func (rc *RedisCache) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	fullKey := rc.getKey(key)

	if ttl == 0 {
		ttl = rc.defaultTTL
	}

	return rc.client.Set(ctx, fullKey, value, ttl).Err()
}

// Delete removes a value from Redis
func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	fullKey := rc.getKey(key)
	return rc.client.Del(ctx, fullKey).Err()
}

// Exists checks if a key exists in Redis
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := rc.getKey(key)

	count, err := rc.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Clear clears all keys with the prefix
func (rc *RedisCache) Clear(ctx context.Context) error {
	pattern := rc.keyPrefix + "*"

	iter := rc.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		rc.client.Del(ctx, iter.Val())
	}

	return iter.Err()
}

// Keys returns all keys with the prefix
func (rc *RedisCache) Keys(ctx context.Context) ([]string, error) {
	pattern := rc.keyPrefix + "*"

	keys := make([]string, 0)
	iter := rc.client.Scan(ctx, 0, pattern, 100).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		// Remove prefix
		if len(key) > len(rc.keyPrefix) {
			keys = append(keys, key[len(rc.keyPrefix):])
		}
	}

	return keys, iter.Err()
}

// Stats returns cache statistics
func (rc *RedisCache) Stats(ctx context.Context) (CacheStats, error) {
	// Get number of keys
	pattern := rc.keyPrefix + "*"
	count := 0

	iter := rc.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		count++
	}

	if err := iter.Err(); err != nil {
		return CacheStats{}, err
	}

	stats := CacheStats{
		Hits:   rc.hits,
		Misses: rc.misses,
		Size:   count,
	}

	total := rc.hits + rc.misses
	if total > 0 {
		stats.HitRate = float64(rc.hits) / float64(total)
	}

	return stats, nil
}

// Increment increments a counter
func (rc *RedisCache) Increment(ctx context.Context, key string) (int64, error) {
	fullKey := rc.getKey(key)
	return rc.client.Incr(ctx, fullKey).Result()
}

// Decrement decrements a counter
func (rc *RedisCache) Decrement(ctx context.Context, key string) (int64, error) {
	fullKey := rc.getKey(key)
	return rc.client.Decr(ctx, fullKey).Result()
}

// SetNX sets a value only if it doesn't exist
func (rc *RedisCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	fullKey := rc.getKey(key)

	if ttl == 0 {
		ttl = rc.defaultTTL
	}

	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}

	return rc.client.SetNX(ctx, fullKey, data, ttl).Result()
}

// Expire sets a new expiration time for a key
func (rc *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := rc.getKey(key)
	return rc.client.Expire(ctx, fullKey, ttl).Err()
}

// TTL returns the remaining time to live for a key
func (rc *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := rc.getKey(key)
	return rc.client.TTL(ctx, fullKey).Result()
}

// HSet sets a hash field
func (rc *RedisCache) HSet(ctx context.Context, key, field string, value interface{}) error {
	fullKey := rc.getKey(key)

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return rc.client.HSet(ctx, fullKey, field, data).Err()
}

// HGet retrieves a hash field
func (rc *RedisCache) HGet(ctx context.Context, key, field string) (interface{}, error) {
	fullKey := rc.getKey(key)

	data, err := rc.client.HGet(ctx, fullKey, field).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, errors.New("field not found")
		}
		return nil, err
	}

	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	return value, nil
}

// HGetAll retrieves all hash fields
func (rc *RedisCache) HGetAll(ctx context.Context, key string) (map[string]interface{}, error) {
	fullKey := rc.getKey(key)

	data, err := rc.client.HGetAll(ctx, fullKey).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for field, value := range data {
		var v interface{}
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			result[field] = value // Store as string if unmarshal fails
		} else {
			result[field] = v
		}
	}

	return result, nil
}

// LPush prepends values to a list
func (rc *RedisCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	fullKey := rc.getKey(key)

	// Convert values to JSON
	jsonValues := make([]interface{}, len(values))
	for i, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		jsonValues[i] = data
	}

	return rc.client.LPush(ctx, fullKey, jsonValues...).Err()
}

// RPush appends values to a list
func (rc *RedisCache) RPush(ctx context.Context, key string, values ...interface{}) error {
	fullKey := rc.getKey(key)

	// Convert values to JSON
	jsonValues := make([]interface{}, len(values))
	for i, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		jsonValues[i] = data
	}

	return rc.client.RPush(ctx, fullKey, jsonValues...).Err()
}

// LRange retrieves a range of list elements
func (rc *RedisCache) LRange(ctx context.Context, key string, start, stop int64) ([]interface{}, error) {
	fullKey := rc.getKey(key)

	data, err := rc.client.LRange(ctx, fullKey, start, stop).Result()
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(data))
	for i, item := range data {
		var value interface{}
		if err := json.Unmarshal([]byte(item), &value); err != nil {
			result[i] = item // Store as string if unmarshal fails
		} else {
			result[i] = value
		}
	}

	return result, nil
}

// SAdd adds members to a set
func (rc *RedisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	fullKey := rc.getKey(key)

	// Convert members to JSON
	jsonMembers := make([]interface{}, len(members))
	for i, member := range members {
		data, err := json.Marshal(member)
		if err != nil {
			return err
		}
		jsonMembers[i] = data
	}

	return rc.client.SAdd(ctx, fullKey, jsonMembers...).Err()
}

// SMembers retrieves all members of a set
func (rc *RedisCache) SMembers(ctx context.Context, key string) ([]interface{}, error) {
	fullKey := rc.getKey(key)

	data, err := rc.client.SMembers(ctx, fullKey).Result()
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(data))
	for i, item := range data {
		var value interface{}
		if err := json.Unmarshal([]byte(item), &value); err != nil {
			result[i] = item // Store as string if unmarshal fails
		} else {
			result[i] = value
		}
	}

	return result, nil
}

// getKey returns the full key with prefix
func (rc *RedisCache) getKey(key string) string {
	return rc.keyPrefix + key
}

// Pipeline creates a new Redis pipeline for batch operations
func (rc *RedisCache) Pipeline(ctx context.Context) redis.Pipeliner {
	return rc.client.Pipeline()
}

// Transaction creates a new Redis transaction
func (rc *RedisCache) Transaction(ctx context.Context, fn func(pipe redis.Pipeliner) error) error {
	_, err := rc.client.TxPipelined(ctx, fn)
	return err
}
