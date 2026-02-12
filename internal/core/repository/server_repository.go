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

// ServerRepository handles database operations for servers
type ServerRepository struct {
	db     *Database
	logger *zap.Logger
}

// NewServerRepository creates a new server repository
func NewServerRepository(db *Database, logger *zap.Logger) *ServerRepository {
	return &ServerRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new server in the database
func (r *ServerRepository) Create(ctx context.Context, server *models.Server) error {
	server.ID = uuid.New().String()
	server.InstanceID = uuid.New().String()
	server.CreatedAt = time.Now()
	server.UpdatedAt = time.Now()

	settingsJSON, err := json.Marshal(server.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	envVarsJSON, err := json.Marshal(server.EnvVars)
	if err != nil {
		return fmt.Errorf("failed to marshal env vars: %w", err)
	}

	query := `
		INSERT INTO servers (
			id, name, node_id, game_type, instance_id, status,
			version, settings, env_vars, max_players, world_name, online_mode,
			port, query_port, rcon_port, ip_address, player_count,
			cpu_usage, memory_usage, uptime_seconds,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`

	_, err = r.db.ExecContext(ctx, query,
		server.ID, server.Name, server.NodeID, server.GameType, server.InstanceID, server.Status,
		server.Version, settingsJSON, envVarsJSON, server.MaxPlayers, server.WorldName, server.OnlineMode,
		server.Port, server.QueryPort, server.RCONPort, server.IPAddress, server.PlayerCount,
		server.CPUUsage, server.MemoryUsage, server.UptimeSeconds,
		server.CreatedAt, server.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	r.logger.Info("Server created",
		zap.String("server_id", server.ID),
		zap.String("name", server.Name),
		zap.String("node_id", server.NodeID))

	return nil
}

// GetByID retrieves a server by ID
func (r *ServerRepository) GetByID(ctx context.Context, id string) (*models.Server, error) {
	query := `
		SELECT id, name, node_id, game_type, instance_id, status,
			version, settings, env_vars, max_players, world_name, online_mode,
			port, query_port, rcon_port, ip_address, player_count,
			cpu_usage, memory_usage, uptime_seconds,
			created_at, updated_at, started_at
		FROM servers WHERE id = $1
	`

	var server models.Server
	var settingsJSON, envVarsJSON []byte
	var startedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&server.ID, &server.Name, &server.NodeID, &server.GameType, &server.InstanceID, &server.Status,
		&server.Version, &settingsJSON, &envVarsJSON, &server.MaxPlayers, &server.WorldName, &server.OnlineMode,
		&server.Port, &server.QueryPort, &server.RCONPort, &server.IPAddress, &server.PlayerCount,
		&server.CPUUsage, &server.MemoryUsage, &server.UptimeSeconds,
		&server.CreatedAt, &server.UpdatedAt, &startedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	if err := json.Unmarshal(settingsJSON, &server.Settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if err := json.Unmarshal(envVarsJSON, &server.EnvVars); err != nil {
		return nil, fmt.Errorf("failed to unmarshal env vars: %w", err)
	}

	if startedAt.Valid {
		server.StartedAt = startedAt
	}

	return &server, nil
}

// List retrieves all servers with optional filters
func (r *ServerRepository) List(ctx context.Context, filters *models.ServerFilters) ([]*models.Server, error) {
	query := `
		SELECT id, name, node_id, game_type, instance_id, status,
			version, settings, env_vars, max_players, world_name, online_mode,
			port, query_port, rcon_port, ip_address, player_count,
			cpu_usage, memory_usage, uptime_seconds,
			created_at, updated_at, started_at
		FROM servers WHERE 1=1
	`

	var args []interface{}
	argNum := 1

	if filters.NodeID != "" {
		query += fmt.Sprintf(" AND node_id = $%d", argNum)
		args = append(args, filters.NodeID)
		argNum++
	}

	if filters.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, filters.Status)
		argNum++
	}

	if filters.GameType != "" {
		query += fmt.Sprintf(" AND game_type = $%d", argNum)
		args = append(args, filters.GameType)
		argNum++
	}

	if filters.HasPlayer != nil {
		if *filters.HasPlayer {
			query += fmt.Sprintf(" AND player_count > $%d", argNum)
		} else {
			query += fmt.Sprintf(" AND player_count = $%d", argNum)
		}
		args = append(args, 0)
		argNum++
	}

	query += " ORDER BY created_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filters.Limit)
		argNum++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filters.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}
	defer rows.Close()

	var servers []*models.Server
	for rows.Next() {
		var server models.Server
		var settingsJSON, envVarsJSON []byte
		var startedAt sql.NullTime

		if err := rows.Scan(
			&server.ID, &server.Name, &server.NodeID, &server.GameType, &server.InstanceID, &server.Status,
			&server.Version, &settingsJSON, &envVarsJSON, &server.MaxPlayers, &server.WorldName, &server.OnlineMode,
			&server.Port, &server.QueryPort, &server.RCONPort, &server.IPAddress, &server.PlayerCount,
			&server.CPUUsage, &server.MemoryUsage, &server.UptimeSeconds,
			&server.CreatedAt, &server.UpdatedAt, &startedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan server: %w", err)
		}

		if err := json.Unmarshal(settingsJSON, &server.Settings); err != nil {
			return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
		}

		if err := json.Unmarshal(envVarsJSON, &server.EnvVars); err != nil {
			return nil, fmt.Errorf("failed to unmarshal env vars: %w", err)
		}

		if startedAt.Valid {
			server.StartedAt = startedAt
		}

		servers = append(servers, &server)
	}

	return servers, nil
}

// Update updates a server in the database
func (r *ServerRepository) Update(ctx context.Context, server *models.Server) error {
	server.UpdatedAt = time.Now()

	settingsJSON, err := json.Marshal(server.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	envVarsJSON, err := json.Marshal(server.EnvVars)
	if err != nil {
		return fmt.Errorf("failed to marshal env vars: %w", err)
	}

	query := `
		UPDATE servers SET
			name = $1, status = $2, version = $3, settings = $4, env_vars = $5,
			max_players = $6, world_name = $7, online_mode = $8,
			player_count = $9, cpu_usage = $10, memory_usage = $11,
			uptime_seconds = $12, updated_at = $13, started_at = $14
		WHERE id = $15
	`

	var startedAt interface{}
	if server.StartedAt.Valid {
		startedAt = server.StartedAt.Time
	} else {
		startedAt = nil
	}

	_, err = r.db.ExecContext(ctx, query,
		server.Name, server.Status, server.Version, settingsJSON, envVarsJSON,
		server.MaxPlayers, server.WorldName, server.OnlineMode,
		server.PlayerCount, server.CPUUsage, server.MemoryUsage,
		server.UptimeSeconds, server.UpdatedAt, startedAt, server.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update server: %w", err)
	}

	return nil
}

// UpdateStatus updates the status of a server
func (r *ServerRepository) UpdateStatus(ctx context.Context, id string, status models.ServerStatus) error {
	query := `UPDATE servers SET status = $1, updated_at = $2 WHERE id = $3`

	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	return nil
}

// Delete deletes a server from the database
func (r *ServerRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM servers WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	return nil
}

// CountByNode counts servers by node ID
func (r *ServerRepository) CountByNode(ctx context.Context, nodeID string) (int, error) {
	query := `SELECT COUNT(*) FROM servers WHERE node_id = $1`

	var count int
	err := r.db.QueryRowContext(ctx, query, nodeID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count servers: %w", err)
	}

	return count, nil
}

// CountByStatus counts servers by status
func (r *ServerRepository) CountByStatus(ctx context.Context) (map[models.ServerStatus]int, error) {
	query := `SELECT status, COUNT(*) FROM servers GROUP BY status`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count servers: %w", err)
	}
	defer rows.Close()

	result := make(map[models.ServerStatus]int)
	for rows.Next() {
		var status models.ServerStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan count: %w", err)
		}
		result[status] = count
	}

	return result, nil
}
