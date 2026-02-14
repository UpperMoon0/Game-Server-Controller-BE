package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/game-server/controller/internal/core/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NodeRepository handles database operations for nodes
type NodeRepository struct {
	db     *Database
	logger *zap.Logger
}

// NewNodeRepository creates a new node repository
func NewNodeRepository(db *Database, logger *zap.Logger) *NodeRepository {
	return &NodeRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new node in the database
func (r *NodeRepository) Create(ctx context.Context, node *models.Node) error {
	node.ID = uuid.New().String()
	node.CreatedAt = time.Now()
	node.UpdatedAt = time.Now()

	query := `
		INSERT INTO nodes (
			id, name, port, status, game_type, version,
			player_count, cpu_usage, memory_usage, uptime_seconds,
			agent_version, heartbeat_interval, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.db.ExecContext(ctx, query,
		node.ID, node.Name, node.Port, node.Status, node.GameType, node.Version,
		node.PlayerCount, node.CPUUsage, node.MemoryUsage, node.UptimeSeconds,
		node.AgentVersion, node.HeartbeatInterval,
		node.CreatedAt, node.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	r.logger.Info("Node created",
		zap.String("node_id", node.ID),
		zap.String("name", node.Name))

	return nil
}

// GetByID retrieves a node by ID
func (r *NodeRepository) GetByID(ctx context.Context, id string) (*models.Node, error) {
	query := `
		SELECT id, name, port, status, game_type, version,
			player_count, cpu_usage, memory_usage, uptime_seconds,
			agent_version, heartbeat_interval, last_heartbeat,
			created_at, updated_at, started_at
		FROM nodes WHERE id = $1
	`

	var node models.Node
	var lastHeartbeat sql.NullTime
	var agentVersion sql.NullString
	var version sql.NullString
	var startedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&node.ID, &node.Name, &node.Port, &node.Status, &node.GameType, &version,
		&node.PlayerCount, &node.CPUUsage, &node.MemoryUsage, &node.UptimeSeconds,
		&agentVersion, &node.HeartbeatInterval, &lastHeartbeat,
		&node.CreatedAt, &node.UpdatedAt, &startedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	if agentVersion.Valid {
		node.AgentVersion = agentVersion.String
	}
	if version.Valid {
		node.Version = version.String
	}
	if lastHeartbeat.Valid {
		node.LastHeartbeat = lastHeartbeat.Time
	}
	if startedAt.Valid {
		node.StartedAt = startedAt
	}

	return &node, nil
}

// GetByName retrieves a node by name
func (r *NodeRepository) GetByName(ctx context.Context, name string) (*models.Node, error) {
	query := `
		SELECT id, name, port, status, game_type, version,
			player_count, cpu_usage, memory_usage, uptime_seconds,
			agent_version, heartbeat_interval, last_heartbeat,
			created_at, updated_at, started_at
		FROM nodes WHERE name = $1
	`

	var node models.Node
	var lastHeartbeat sql.NullTime
	var agentVersion sql.NullString
	var version sql.NullString
	var startedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&node.ID, &node.Name, &node.Port, &node.Status, &node.GameType, &version,
		&node.PlayerCount, &node.CPUUsage, &node.MemoryUsage, &node.UptimeSeconds,
		&agentVersion, &node.HeartbeatInterval, &lastHeartbeat,
		&node.CreatedAt, &node.UpdatedAt, &startedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	if agentVersion.Valid {
		node.AgentVersion = agentVersion.String
	}
	if version.Valid {
		node.Version = version.String
	}
	if lastHeartbeat.Valid {
		node.LastHeartbeat = lastHeartbeat.Time
	}
	if startedAt.Valid {
		node.StartedAt = startedAt
	}

	return &node, nil
}

// List retrieves all nodes
func (r *NodeRepository) List(ctx context.Context, status *models.NodeStatus) ([]*models.Node, error) {
	var query string
	var args []interface{}

	if status != nil {
		query = `
			SELECT id, name, port, status, game_type, version,
				player_count, cpu_usage, memory_usage, uptime_seconds,
				agent_version, heartbeat_interval, last_heartbeat,
				created_at, updated_at, started_at
			FROM nodes WHERE status = $1 ORDER BY created_at DESC
		`
		args = []interface{}{*status}
	} else {
		query = `
			SELECT id, name, port, status, game_type, version,
				player_count, cpu_usage, memory_usage, uptime_seconds,
				agent_version, heartbeat_interval, last_heartbeat,
				created_at, updated_at, started_at
			FROM nodes ORDER BY created_at DESC
		`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*models.Node
	for rows.Next() {
		var node models.Node
		var lastHeartbeat sql.NullTime
		var agentVersion sql.NullString
		var version sql.NullString
		var startedAt sql.NullTime

		if err := rows.Scan(
			&node.ID, &node.Name, &node.Port, &node.Status, &node.GameType, &version,
			&node.PlayerCount, &node.CPUUsage, &node.MemoryUsage, &node.UptimeSeconds,
			&agentVersion, &node.HeartbeatInterval, &lastHeartbeat,
			&node.CreatedAt, &node.UpdatedAt, &startedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}

		if agentVersion.Valid {
			node.AgentVersion = agentVersion.String
		}
		if version.Valid {
			node.Version = version.String
		}
		if lastHeartbeat.Valid {
			node.LastHeartbeat = lastHeartbeat.Time
		}
		if startedAt.Valid {
			node.StartedAt = startedAt
		}

		nodes = append(nodes, &node)
	}

	return nodes, nil
}

// Update updates a node in the database
func (r *NodeRepository) Update(ctx context.Context, node *models.Node) error {
	node.UpdatedAt = time.Now()

	query := `
		UPDATE nodes SET
			name = $1, port = $2, status = $3, game_type = $4, version = $5,
			player_count = $6, cpu_usage = $7, memory_usage = $8, uptime_seconds = $9,
			heartbeat_interval = $10, last_heartbeat = $11, updated_at = $12, started_at = $13
		WHERE id = $14
	`

	_, err := r.db.ExecContext(ctx, query,
		node.Name, node.Port, node.Status, node.GameType, node.Version,
		node.PlayerCount, node.CPUUsage, node.MemoryUsage, node.UptimeSeconds,
		node.HeartbeatInterval, node.LastHeartbeat, node.UpdatedAt, node.StartedAt,
		node.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}

	return nil
}

// UpdateHeartbeat updates the last heartbeat time
func (r *NodeRepository) UpdateHeartbeat(ctx context.Context, id string, heartbeat time.Time) error {
	query := `UPDATE nodes SET last_heartbeat = $1, updated_at = $2 WHERE id = $3`

	_, err := r.db.ExecContext(ctx, query, heartbeat, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}

	return nil
}

// UpdateMetrics updates the runtime metrics for a node
func (r *NodeRepository) UpdateMetrics(ctx context.Context, id string, playerCount int, cpuUsage float64, memoryUsage int64, uptimeSeconds int64) error {
	query := `UPDATE nodes SET player_count = $1, cpu_usage = $2, memory_usage = $3, uptime_seconds = $4, updated_at = $5 WHERE id = $6`

	_, err := r.db.ExecContext(ctx, query, playerCount, cpuUsage, memoryUsage, uptimeSeconds, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update metrics: %w", err)
	}

	return nil
}

// Delete deletes a node from the database
func (r *NodeRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM nodes WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	return nil
}

// CountByStatus counts nodes by status
func (r *NodeRepository) CountByStatus(ctx context.Context) (map[models.NodeStatus]int, error) {
	query := `SELECT status, COUNT(*) FROM nodes GROUP BY status`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count nodes: %w", err)
	}
	defer rows.Close()

	result := make(map[models.NodeStatus]int)
	for rows.Next() {
		var status models.NodeStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan count: %w", err)
		}
		result[status] = count
	}

	return result, nil
}
