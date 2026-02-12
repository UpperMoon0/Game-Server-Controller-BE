package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/game-server/controller/internal/core/models"
	"github.com/game-server/controller/pkg/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Redis wraps the Redis client with additional functionality
type Redis struct {
	*redis.Client
	logger *zap.Logger
}

// NewRedis creates a new Redis client
func NewRedis(cfg *config.Config) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddress(),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Redis{
		Client: client,
		logger: zap.NewNop(),
	}, nil
}

// NewRedisWithLogger creates a Redis client with a logger
func NewRedisWithLogger(client *redis.Client, logger *zap.Logger) *Redis {
	return &Redis{
		Client: client,
		logger: logger,
	}
}

// Close closes the Redis connection
func (r *Redis) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

// Metrics operations

// StoreNodeMetrics stores node metrics in Redis
func (r *Redis) StoreNodeMetrics(ctx context.Context, metrics *models.NodeMetrics) error {
	key := fmt.Sprintf("node:metrics:%s", metrics.NodeID)
	
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Store with TTL (10 seconds for real-time data)
	if err := r.Client.Set(ctx, key, data, 10*time.Second).Err(); err != nil {
		return fmt.Errorf("failed to store metrics: %w", err)
	}

	return nil
}

// GetNodeMetrics retrieves the latest node metrics
func (r *Redis) GetNodeMetrics(ctx context.Context, nodeID string) (*models.NodeMetrics, error) {
	key := fmt.Sprintf("node:metrics:%s", nodeID)

	data, err := r.Client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	var metrics models.NodeMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	return &metrics, nil
}

// StoreServerMetrics stores server metrics in Redis
func (r *Redis) StoreServerMetrics(ctx context.Context, metrics *models.ServerMetrics) error {
	key := fmt.Sprintf("server:metrics:%s", metrics.ServerID)
	
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Store with TTL (10 seconds for real-time data)
	if err := r.Client.Set(ctx, key, data, 10*time.Second).Err(); err != nil {
		return fmt.Errorf("failed to store metrics: %w", err)
	}

	return nil
}

// GetServerMetrics retrieves the latest server metrics
func (r *Redis) GetServerMetrics(ctx context.Context, serverID string) (*models.ServerMetrics, error) {
	key := fmt.Sprintf("server:metrics:%s", serverID)

	data, err := r.Client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	var metrics models.ServerMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	return &metrics, nil
}

// Pub/Sub operations

// PublishEvent publishes an event to a channel
func (r *Redis) PublishEvent(ctx context.Context, channel string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := r.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

// SubscribeToEvents subscribes to events on a channel
func (r *Redis) SubscribeToEvents(ctx context.Context, channel string) *redis.PubSub {
	return r.Subscribe(ctx, channel)
}

// Cache operations

// CacheNode caches node data
func (r *Redis) CacheNode(ctx context.Context, node *models.Node, ttl time.Duration) error {
	key := fmt.Sprintf("node:%s", node.ID)
	
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}

	if err := r.Client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to cache node: %w", err)
	}

	return nil
}

// GetCachedNode retrieves cached node data
func (r *Redis) GetCachedNode(ctx context.Context, id string) (*models.Node, error) {
	key := fmt.Sprintf("node:%s", id)

	data, err := r.Client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cached node: %w", err)
	}

	var node models.Node
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node: %w", err)
	}

	return &node, nil
}

// InvalidateNodeCache removes node from cache
func (r *Redis) InvalidateNodeCache(ctx context.Context, id string) error {
	key := fmt.Sprintf("node:%s", id)
	
	if err := r.Client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to invalidate cache: %w", err)
	}

	return nil
}

// Rate limiting

// AcquireRateLimit acquires a rate limit slot
func (r *Redis) AcquireRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	count, err := r.Client.Incr(ctx, fmt.Sprintf("ratelimit:%s", key)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to increment rate limit: %w", err)
	}

	if count == 1 {
		r.Client.Expire(ctx, fmt.Sprintf("ratelimit:%s", key), window)
	}

	if count > int64(limit) {
		return false, nil
	}

	return true, nil
}
