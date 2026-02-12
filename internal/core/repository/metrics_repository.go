package repository

import (
	"context"

	"github.com/game-server/controller/internal/core/models"
	"go.uber.org/zap"
)

// MetricsRepository handles metrics operations
type MetricsRepository struct {
	redis *Redis
	logger *zap.Logger
}

// NewMetricsRepository creates a new metrics repository
func NewMetricsRepository(redis *Redis, logger *zap.Logger) *MetricsRepository {
	return &MetricsRepository{
		redis:  redis,
		logger: logger,
	}
}

// StoreNodeMetrics stores node metrics
func (r *MetricsRepository) StoreNodeMetrics(ctx context.Context, metrics *models.NodeMetrics) error {
	return r.redis.StoreNodeMetrics(ctx, metrics)
}

// GetNodeMetrics retrieves node metrics
func (r *MetricsRepository) GetNodeMetrics(ctx context.Context, nodeID string) (*models.NodeMetrics, error) {
	return r.redis.GetNodeMetrics(ctx, nodeID)
}

// StoreServerMetrics stores server metrics
func (r *MetricsRepository) StoreServerMetrics(ctx context.Context, metrics *models.ServerMetrics) error {
	return r.redis.StoreServerMetrics(ctx, metrics)
}

// GetServerMetrics retrieves server metrics
func (r *MetricsRepository) GetServerMetrics(ctx context.Context, serverID string) (*models.ServerMetrics, error) {
	return r.redis.GetServerMetrics(ctx, serverID)
}
