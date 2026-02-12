package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/game-server/controller/internal/core/models"
	"github.com/game-server/controller/internal/core/repository"
	"github.com/game-server/controller/internal/node"
	"go.uber.org/zap"
)

// Scheduler handles resource allocation and server lifecycle
type Scheduler struct {
	nodeRepo    *repository.NodeRepository
	serverRepo  *repository.ServerRepository
	nodeMgr     *node.Manager
	logger      *zap.Logger
}

// NewScheduler creates a new scheduler
func NewScheduler(
	nodeRepo *repository.NodeRepository,
	serverRepo *repository.ServerRepository,
	nodeMgr *node.Manager,
	logger *zap.Logger,
) *Scheduler {
	return &Scheduler{
		nodeRepo:   nodeRepo,
		serverRepo: serverRepo,
		nodeMgr:    nodeMgr,
		logger:     logger,
	}
}

// CreateServer creates a new server on the optimal node
func (s *Scheduler) CreateServer(ctx context.Context, req *models.CreateServerRequest) (*models.CreateServerResponse, error) {
	// Find optimal node for the server
	node, err := s.FindOptimalNode(req.GameType, &req.Requirements)
	if err != nil {
		return nil, fmt.Errorf("failed to find optimal node: %w", err)
	}

	// Allocate resources
	if err := s.AllocateResources(node.ID, &req.Requirements); err != nil {
		return nil, fmt.Errorf("failed to allocate resources: %w", err)
	}

	// Create server configuration
	server := &models.Server{
		Name:          req.Config.Name,
		NodeID:        node.ID,
		GameType:      req.GameType,
		Status:        models.ServerStatusInstalling,
		Version:       req.Config.Version,
		Settings:      req.Config.Settings,
		EnvVars:       req.Config.EnvVars,
		MaxPlayers:    req.Config.MaxPlayers,
		WorldName:     req.Config.WorldName,
		OnlineMode:    req.Config.OnlineMode,
		Port:          0, // Will be assigned by node
		QueryPort:     0,
		RCONPort:      0,
		IPAddress:     node.IPAddress,
		PlayerCount:   0,
		CPUUsage:      0,
		MemoryUsage:   0,
		UptimeSeconds: 0,
	}

	// Create server in database
	if err := s.serverRepo.Create(ctx, server); err != nil {
		s.ReleaseResources(node.ID, &req.Requirements)
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	// Send create command to node
	cmd := &node.Command{
		ID:   generateCommandID(),
		Type: node.CommandTypeCreateServer,
		Payload: map[string]interface{}{
			"server_id":     server.ID,
			"game_type":    req.GameType,
			"config":        req.Config,
			"requirements":  req.Requirements,
		},
		Response: make(chan *node.CommandResult, 1),
	}

	if err := s.nodeMgr.SendCommand(node.ID, cmd); err != nil {
		s.serverRepo.Delete(ctx, server.ID)
		s.ReleaseResources(node.ID, &req.Requirements)
		return nil, fmt.Errorf("failed to send create command: %w", err)
	}

	// Wait for result (with timeout)
	select {
	case result := <-cmd.Response:
		if !result.Success {
			s.serverRepo.Delete(ctx, server.ID)
			s.ReleaseResources(node.ID, &req.Requirements)
			return nil, fmt.Errorf("failed to create server on node: %s", result.Message)
		}
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("timeout waiting for server creation")
	}

	s.logger.Info("Server created",
		zap.String("server_id", server.ID),
		zap.String("node_id", node.ID),
		zap.String("game_type", req.GameType))

	return &models.CreateServerResponse{
		Success:   true,
		ServerID:  server.ID,
		Message:   "Server created successfully",
		ServerInfo: &models.ServerInfo{
			ServerID:  server.ID,
			NodeID:    node.ID,
			Port:      server.Port,
			IPAddress: server.IPAddress,
		},
	}, nil
}

// UpdateServer updates server configuration
func (s *Scheduler) UpdateServer(ctx context.Context, serverID string, req models.UpdateServerRequest) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Update server fields
	if req.Config != nil {
		server.Name = req.Config.Name
		server.Version = req.Config.Version
		server.Settings = req.Config.Settings
		server.EnvVars = req.Config.EnvVars
		server.MaxPlayers = req.Config.MaxPlayers
		server.WorldName = req.Config.WorldName
		server.OnlineMode = req.Config.OnlineMode
	}

	// Save to database
	if err := s.serverRepo.Update(ctx, server); err != nil {
		return fmt.Errorf("failed to update server: %w", err)
	}

	// Send update command to node if server is running
	if server.Status == models.ServerStatusRunning && req.Restart {
		return s.RestartServer(ctx, serverID)
	}

	return nil
}

// DeleteServer deletes a server
func (s *Scheduler) DeleteServer(ctx context.Context, serverID string, backup bool) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Send delete command to node
	cmd := &node.Command{
		ID:   generateCommandID(),
		Type: node.CommandTypeDeleteServer,
		Payload: map[string]interface{}{
			"server_id":         serverID,
			"backup_before_delete": backup,
		},
		Response: make(chan *node.CommandResult, 1),
	}

	if err := s.nodeMgr.SendCommand(server.NodeID, cmd); err != nil {
		s.logger.Error("Failed to send delete command", zap.Error(err))
	}

	// Delete from database
	if err := s.serverRepo.Delete(ctx, serverID); err != nil {
		s.logger.Error("Failed to delete server from database", zap.Error(err))
	}

	// Release resources
	requirements := &models.ResourceRequirements{
		MinCPUCores:  1,
		MinMemoryMB:  1024,
		MinStorageMB: 1024,
	}
	s.ReleaseResources(server.NodeID, requirements)

	s.logger.Info("Server deleted", zap.String("server_id", serverID))

	return nil
}

// StartServer starts a server
func (s *Scheduler) StartServer(ctx context.Context, serverID string) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Update status
	if err := s.serverRepo.UpdateStatus(ctx, serverID, models.ServerStatusStarting); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Send start command to node
	cmd := &node.Command{
		ID:   generateCommandID(),
		Type: node.CommandTypeStartServer,
		Payload: map[string]interface{}{
			"server_id": serverID,
		},
		Response: make(chan *node.CommandResult, 1),
	}

	if err := s.nodeMgr.SendCommand(server.NodeID, cmd); err != nil {
		s.serverRepo.UpdateStatus(ctx, serverID, models.ServerStatusStopped)
		return fmt.Errorf("failed to send start command: %w", err)
	}

	return nil
}

// StopServer stops a server
func (s *Scheduler) StopServer(ctx context.Context, serverID string) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Update status
	if err := s.serverRepo.UpdateStatus(ctx, serverID, models.ServerStatusStopping); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Send stop command to node
	cmd := &node.Command{
		ID:   generateCommandID(),
		Type: node.CommandTypeStopServer,
		Payload: map[string]interface{}{
			"server_id": serverID,
		},
		Response: make(chan *node.CommandResult, 1),
	}

	if err := s.nodeMgr.SendCommand(server.NodeID, cmd); err != nil {
		s.serverRepo.UpdateStatus(ctx, serverID, models.ServerStatusRunning)
		return fmt.Errorf("failed to send stop command: %w", err)
	}

	return nil
}

// RestartServer restarts a server
func (s *Scheduler) RestartServer(ctx context.Context, serverID string) error {
	// Stop then start
	if err := s.StopServer(ctx, serverID); err != nil {
		return err
	}

	// Wait for stop
	time.Sleep(5 * time.Second)

	return s.StartServer(ctx, serverID)
}

// ReinstallServer reinstalls a server
func (s *Scheduler) ReinstallServer(ctx context.Context, serverID string) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Update status
	if err := s.serverRepo.UpdateStatus(ctx, serverID, models.ServerStatusInstalling); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Send reinstall command
	cmd := &node.Command{
		ID:   generateCommandID(),
		Type: node.CommandTypeDeleteServer,
		Payload: map[string]interface{}{
			"server_id":     serverID,
			"reinstall":     true,
			"backup_first": true,
		},
		Response: make(chan *node.CommandResult, 1),
	}

	if err := s.nodeMgr.SendCommand(server.NodeID, cmd); err != nil {
		return fmt.Errorf("failed to send reinstall command: %w", err)
	}

	return nil
}

// BackupServer backs up a server
func (s *Scheduler) BackupServer(ctx context.Context, serverID string) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Update status
	if err := s.serverRepo.UpdateStatus(ctx, serverID, models.ServerStatusBackingUp); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Send backup command (placeholder)

	return nil
}

// FindOptimalNode finds the best node for a server
func (s *Scheduler) FindOptimalNode(gameType string, requirements *models.ResourceRequirements) (*models.Node, error) {
	nodes, err := s.nodeMgr.ListNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Filter by game type
	filtered := make([]*models.Node, 0)
	for _, n := range nodes {
		if n.Status != models.NodeStatusOnline {
			continue
		}
		if !containsGameType(n.GameTypes, gameType) {
			continue
		}
		if n.AvailableCPUCores < requirements.MinCPUCores {
			continue
		}
		if n.AvailableMemoryMB < requirements.MinMemoryMB {
			continue
		}
		filtered = append(filtered, n)
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no suitable node found for game type: %s", gameType)
	}

	// Select node with best resource utilization
	bestNode := filtered[0]
	bestScore := calculateNodeScore(bestNode, requirements)

	for _, n := range filtered[1:] {
		score := calculateNodeScore(n, requirements)
		if score < bestScore {
			bestScore = score
			bestNode = n
		}
	}

	return bestNode, nil
}

// AllocateResources allocates resources on a node
func (s *Scheduler) AllocateResources(nodeID string, requirements *models.ResourceRequirements) error {
	node, err := s.nodeMgr.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Update available resources
	node.AvailableCPUCores -= requirements.MinCPUCores
	node.AvailableMemoryMB -= requirements.MinMemoryMB
	node.AvailableStorageMB -= requirements.MinStorageMB

	// Update node in database
	ctx := context.Background()
	if err := s.nodeRepo.Update(ctx, node); err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}

	return nil
}

// ReleaseResources releases resources on a node
func (s *Scheduler) ReleaseResources(nodeID string, requirements *models.ResourceRequirements) {
	node, err := s.nodeMgr.GetNode(nodeID)
	if err != nil {
		s.logger.Error("Failed to release resources", zap.Error(err))
		return
	}

	// Update available resources
	node.AvailableCPUCores += requirements.MinCPUCores
	node.AvailableMemoryMB += requirements.MinMemoryMB
	node.AvailableStorageMB += requirements.MinStorageMB

	// Update node in database
	ctx := context.Background()
	if err := s.nodeRepo.Update(ctx, node); err != nil {
		s.logger.Error("Failed to release resources", zap.Error(err))
	}
}

// GetServer retrieves a server by ID
func (s *Scheduler) GetServer(serverID string) (*models.Server, error) {
	// Implementation placeholder
	return nil, fmt.Errorf("not implemented")
}

// ListServers lists all servers
func (s *Scheduler) ListServers(filters *models.ServerFilters) ([]*models.Server, error) {
	// Implementation placeholder
	return nil, fmt.Errorf("not implemented")
}

// GetServerLogs gets server logs
func (s *Scheduler) GetServerLogs(serverID string, tail int) ([]string, error) {
	// Implementation placeholder
	return nil, fmt.Errorf("not implemented")
}

// GetServerMetrics gets server metrics
func (s *Scheduler) GetServerMetrics(serverID string) (*models.ServerMetrics, error) {
	// Implementation placeholder
	return nil, fmt.Errorf("not implemented")
}

// GetServerCounts returns server counts by status
func (s *Scheduler) GetServerCounts() (map[models.ServerStatus]int, error) {
	// Implementation placeholder
	return make(map[models.ServerStatus]int), nil
}

// Helper functions

func generateCommandID() string {
	return fmt.Sprintf("cmd-%d", time.Now().UnixNano())
}

func containsGameType(gameTypes []string, gameType string) bool {
	for _, gt := range gameTypes {
		if gt == gameType {
			return true
		}
	}
	return false
}

func calculateNodeScore(node *models.Node, requirements *models.ResourceRequirements) float64 {
	// Lower score is better
	cpuScore := float64(node.AvailableCPUCores - requirements.MinCPUCores)
	memoryScore := float64(node.AvailableMemoryMB - requirements.MinMemoryMB)
	storageScore := float64(node.AvailableStorageMB - requirements.MinStorageMB)

	return cpuScore + memoryScore + storageScore
}
