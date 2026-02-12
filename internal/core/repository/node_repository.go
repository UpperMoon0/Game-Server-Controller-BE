package repository

import (
	"context"
	"database/sql"
	"encoding/json"
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

	gameTypesJSON, err := json.Marshal(node.GameTypes)
	if err != nil {
		return fmt.Errorf("failed to marshal game types: %w", err)
	}

	query := `
		INSERT INTO nodes (
			id, name, hostname, ip_address, port, status, game_types,
			total_cpu_cores, total_memory_mb, total_storage_mb,
			available_cpu_cores, available_memory_mb, available_storage_mb,
			os_version, agent_version, heartbeat_interval, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`

	_, err = r.db.ExecContext(ctx, query,
		node.ID, node.Name, node.Hostname, node.IPAddress, node.Port,
		node.Status, gameTypesJSON,
		node.TotalCPUCores, node.TotalMemoryMB, node.TotalStorageMB,
		node.AvailableCPUCores, node.AvailableMemoryMB, node.AvailableStorageMB,
		node.OSVersion, node.AgentVersion, node.HeartbeatInterval,
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
		SELECT id, name, hostname, ip_address, port, status, game_types,
			total_cpu_cores, total_memory_mb, total_storage_mb,
			available_cpu_cores, available_memory_mb, available_storage_mb,
			os_version, agent_version, heartbeat_interval, last_heartbeat,
			created_at, updated_at
		FROM nodes WHERE id = $1
	`

	var node models.Node
	var gameTypesJSON []byte
	var lastHeartbeat sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&node.ID, &node.Name, &node.Hostname, &node.IPAddress, &node.Port,
		&node.Status, &gameTypesJSON,
		&node.TotalCPUCores, &node.TotalMemoryMB, &node.TotalStorageMB,
		&node.AvailableCPUCores, &node.AvailableMemoryMB, &node.AvailableStorageMB,
		&node.OSVersion, &node.AgentVersion, &node.HeartbeatInterval, &lastHeartbeat,
		&node.CreatedAt, &node.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	if err := json.Unmarshal(gameTypesJSON, &node.GameTypes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal game types: %w", err)
	}

	if lastHeartbeat.Valid {
		node.LastHeartbeat = lastHeartbeat.Time
	}

	return &node, nil
}

// GetByHostname retrieves a node by hostname
func (r *NodeRepository) GetByHostname(ctx context.Context, hostname string) (*models.Node, error) {
	query := `
		SELECT id, name, hostname, ip_address, port, status, game_types,
			total_cpu_cores, total_memory_mb, total_storage_mb,
			available_cpu_cores, available_memory_mb, available_storage_mb,
			os_version, agent_version, heartbeat_interval, last_heartbeat,
			created_at, updated_at
		FROM nodes WHERE hostname = $1
	`

	var node models.Node
	var gameTypesJSON []byte
	var lastHeartbeat sql.NullTime

	err := r.db.QueryRowContext(ctx, query, hostname).Scan(
		&node.ID, &node.Name, &node.Hostname, &node.IPAddress, &node.Port,
		&node.Status, &gameTypesJSON,
		&node.TotalCPUCores, &node.TotalMemoryMB, &node.TotalStorageMB,
		&node.AvailableCPUCores, &node.AvailableMemoryMB, &node.AvailableStorageMB,
		&node.OSVersion, &node.AgentVersion, &node.HeartbeatInterval, &lastHeartbeat,
		&node.CreatedAt, &node.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	if err := json.Unmarshal(gameTypesJSON, &node.GameTypes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal game types: %w", err)
	}

	if lastHeartbeat.Valid {
		node.LastHeartbeat = lastHeartbeat.Time
	}

	return &node, nil
}

// List retrieves all nodes
func (r *NodeRepository) List(ctx context.Context, status *models.NodeStatus) ([]*models.Node, error) {
	var query string
	var args []interface{}

	if status != nil {
		query = `
			SELECT id, name, hostname, ip_address, port, status, game_types,
				total_cpu_cores, total_memory_mb, total_storage_mb,
				available_cpu_cores, available_memory_mb, available_storage_mb,
				os_version, agent_version, heartbeat_interval, last_heartbeat,
				created_at, updated_at
			FROM nodes WHERE status = $1 ORDER BY created_at DESC
		`
		args = []interface{}{*status}
	} else {
		query = `
			SELECT id, name, hostname, ip_address, port, status, game_types,
				total_cpu_cores, total_memory_mb, total_storage_mb,
				available_cpu_cores, available_memory_mb, available_storage_mb,
				os_version, agent_version, heartbeat_interval, last_heartbeat,
				created_at, updated_at
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
		var gameTypesJSON []byte
		var lastHeartbeat sql.NullTime

		if err := rows.Scan(
			&node.ID, &node.Name, &node.Hostname, &node.IPAddress, &node.Port,
			&node.Status, &gameTypesJSON,
			&node.TotalCPUCores, &node.TotalMemoryMB, &node.TotalStorageMB,
			&node.AvailableCPUCores, &node.AvailableMemoryMB, &node.AvailableStorageMB,
			&node.OSVersion, &node.AgentVersion, &node.HeartbeatInterval, &lastHeartbeat,
			&node.CreatedAt, &node.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}

		if err := json.Unmarshal(gameTypesJSON, &node.GameTypes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal game types: %w", err)
		}

		if lastHeartbeat.Valid {
			node.LastHeartbeat = lastHeartbeat.Time
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
			name = $1, status = $2, game_types = $3,
			available_cpu_cores = $4, available_memory_mb = $5, available_storage_mb = $6,
			heartbeat_interval = $7, last_heartbeat = $8, updated_at = $9
		WHERE id = $10
	`

	gameTypesJSON, err := json.Marshal(node.GameTypes)
	if err != nil {
		return fmt.Errorf("failed to marshal game types: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		node.Name, node.Status, gameTypesJSON,
		node.AvailableCPUCores, node.AvailableMemoryMB, node.AvailableStorageMB,
		node.HeartbeatInterval, node.LastHeartbeat, node.UpdatedAt, node.ID,
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
