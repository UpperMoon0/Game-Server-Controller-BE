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
// Note: Status and LastHeartbeat are fetched from node agents via gRPC, not stored in DB
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
			id, name, port, game_type, version, initialized,
			agent_version, heartbeat_interval, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		node.ID, node.Name, node.Port, node.GameType, node.Version, node.Initialized,
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
		SELECT id, name, port, game_type, version, initialized,
			agent_version, heartbeat_interval,
			created_at, updated_at, started_at
		FROM nodes WHERE id = $1
	`

	var node models.Node
	var agentVersion sql.NullString
	var version sql.NullString
	var startedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&node.ID, &node.Name, &node.Port, &node.GameType, &version, &node.Initialized,
		&agentVersion, &node.HeartbeatInterval,
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
	if startedAt.Valid {
		node.StartedAt = startedAt
	}

	// Status is fetched from node agent via gRPC, default to stopped
	node.Status = models.NodeStatusStopped

	return &node, nil
}

// GetByName retrieves a node by name
func (r *NodeRepository) GetByName(ctx context.Context, name string) (*models.Node, error) {
	query := `
		SELECT id, name, port, game_type, version, initialized,
			agent_version, heartbeat_interval,
			created_at, updated_at, started_at
		FROM nodes WHERE name = $1
	`

	var node models.Node
	var agentVersion sql.NullString
	var version sql.NullString
	var startedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&node.ID, &node.Name, &node.Port, &node.GameType, &version, &node.Initialized,
		&agentVersion, &node.HeartbeatInterval,
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
	if startedAt.Valid {
		node.StartedAt = startedAt
	}

	// Status is fetched from node agent via gRPC, default to stopped
	node.Status = models.NodeStatusStopped

	return &node, nil
}

// List retrieves all nodes
func (r *NodeRepository) List(ctx context.Context, _ *models.NodeStatus) ([]*models.Node, error) {
	// Note: status filtering is now done in-memory after fetching from node agents
	query := `
		SELECT id, name, port, game_type, version, initialized,
			agent_version, heartbeat_interval,
			created_at, updated_at, started_at
		FROM nodes ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*models.Node
	for rows.Next() {
		var node models.Node
		var agentVersion sql.NullString
		var version sql.NullString
		var startedAt sql.NullTime

		if err := rows.Scan(
			&node.ID, &node.Name, &node.Port, &node.GameType, &version, &node.Initialized,
			&agentVersion, &node.HeartbeatInterval,
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
		if startedAt.Valid {
			node.StartedAt = startedAt
		}

		// Status is fetched from node agent via gRPC, default to stopped
		node.Status = models.NodeStatusStopped

		nodes = append(nodes, &node)
	}

	return nodes, nil
}

// Update updates a node in the database
func (r *NodeRepository) Update(ctx context.Context, node *models.Node) error {
	node.UpdatedAt = time.Now()

	query := `
		UPDATE nodes SET
			name = $1, port = $2, game_type = $3, version = $4, initialized = $5,
			heartbeat_interval = $6, updated_at = $7, started_at = $8
		WHERE id = $9
	`

	_, err := r.db.ExecContext(ctx, query,
		node.Name, node.Port, node.GameType, node.Version, node.Initialized,
		node.HeartbeatInterval, node.UpdatedAt, node.StartedAt,
		node.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update node: %w", err)
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