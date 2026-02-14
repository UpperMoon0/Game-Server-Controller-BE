package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/game-server/controller/internal/core/models"
	"github.com/game-server/controller/internal/core/repository"
	"github.com/game-server/controller/internal/docker"
	"github.com/game-server/controller/pkg/config"
	"go.uber.org/zap"
)

// Manager handles node lifecycle and communication
type Manager struct {
	nodeRepo      *repository.NodeRepository
	volumeMgr     *docker.VolumeManager
	containerMgr  *docker.ContainerManager
	cfg           *config.Config
	logger        *zap.Logger
	
	// In-memory state
	nodes        map[string]*NodeState
	mu           sync.RWMutex
	streams      map[string]chan *StreamEvent
	streamsMu    sync.RWMutex
}

// NodeState represents the in-memory state of a node
type NodeState struct {
	Node          *models.Node
	Connected     bool
	LastHeartbeat time.Time
	CommandQueue  chan *Command
	Metrics       *models.NodeMetrics
}

// Command represents a command to be sent to a node
type Command struct {
	ID        string
	Type      CommandType
	Payload   interface{}
	Response  chan *CommandResult
}

// CommandType represents the type of command
type CommandType string

const (
	CommandTypeStart   CommandType = "start"
	CommandTypeStop    CommandType = "stop"
	CommandTypeRestart CommandType = "restart"
)

// CommandResult represents the result of a command
type CommandResult struct {
	Success bool
	Message string
	Error   error
}

// StreamEvent represents an event from a node stream
type StreamEvent struct {
	NodeID    string
	Type      models.EventType
	Payload   interface{}
	Timestamp time.Time
}

// NewManager creates a new node manager
func NewManager(
	nodeRepo *repository.NodeRepository,
	volumeMgr *docker.VolumeManager,
	containerMgr *docker.ContainerManager,
	cfg *config.Config,
	logger *zap.Logger,
) *Manager {
	return &Manager{
		nodeRepo:     nodeRepo,
		volumeMgr:    volumeMgr,
		containerMgr: containerMgr,
		cfg:          cfg,
		logger:       logger,
		nodes:        make(map[string]*NodeState),
		streams:      make(map[string]chan *StreamEvent),
	}
}

// RegisterNode registers a new node with the manager (called when node agent connects via gRPC)
func (m *Manager) RegisterNode(ctx context.Context, node *models.Node) error {
	m.mu.Lock()

	// Check if node already exists in memory
	existing, exists := m.nodes[node.ID]
	if exists {
		// Update existing node state
		existing.Node = node
		existing.Connected = true
		existing.LastHeartbeat = time.Now()
		m.mu.Unlock()

		m.logger.Info("Node reconnected",
			zap.String("node_id", node.ID),
			zap.String("name", node.Name))
		return nil
	}

	// Create new node state in memory
	state := &NodeState{
		Node:          node,
		Connected:     true,
		LastHeartbeat: time.Now(),
		CommandQueue:  make(chan *Command, 100),
	}

	m.nodes[node.ID] = state
	m.mu.Unlock()

	m.logger.Info("Node registered",
		zap.String("node_id", node.ID),
		zap.String("name", node.Name),
		zap.String("game_type", node.GameType))

	// Check if node exists in database, create if not
	existingNode, err := m.nodeRepo.GetByID(ctx, node.ID)
	if err != nil {
		return fmt.Errorf("failed to check node existence: %w", err)
	}
	if existingNode == nil {
		if err := m.nodeRepo.Create(ctx, node); err != nil {
			return fmt.Errorf("failed to create node in database: %w", err)
		}
	} else {
		// Update status to running in database
		node.Status = models.NodeStatusRunning
		if err := m.nodeRepo.Update(ctx, node); err != nil {
			m.logger.Error("Failed to update node status", zap.Error(err))
		}
	}

	return nil
}

// CreateNode creates a new node in the database only (called via REST API)
func (m *Manager) CreateNode(ctx context.Context, node *models.Node) error {
	// Only create in database - in-memory state is for connected agents only
	if err := m.nodeRepo.Create(ctx, node); err != nil {
		return fmt.Errorf("failed to create node in database: %w", err)
	}

	m.logger.Info("Node created in database",
		zap.String("node_id", node.ID),
		zap.String("name", node.Name),
		zap.String("game_type", node.GameType))

	return nil
}

// UnregisterNode removes a node from the manager
func (m *Manager) UnregisterNode(ctx context.Context, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// Close command queue
	close(state.CommandQueue)

	// Remove from registry
	delete(m.nodes, nodeID)

	// Update node status in database
	state.Node.Status = models.NodeStatusOffline
	if err := m.nodeRepo.Update(ctx, state.Node); err != nil {
		m.logger.Error("Failed to update node status", zap.Error(err))
	}

	m.logger.Info("Node unregistered", zap.String("node_id", nodeID))

	return nil
}

// DeleteNode permanently deletes a node
func (m *Manager) DeleteNode(ctx context.Context, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.nodes[nodeID]
	if !exists {
		// Check if node exists in database
		node, err := m.nodeRepo.GetByID(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("failed to check node existence: %w", err)
		}
		if node == nil {
			return fmt.Errorf("node not found: %s", nodeID)
		}
		// Node exists in DB but not in memory, proceed with deletion
	} else {
		// Close command queue if node is in memory
		close(state.CommandQueue)
		// Remove from registry
		delete(m.nodes, nodeID)
	}

	// Delete node from database
	if err := m.nodeRepo.Delete(ctx, nodeID); err != nil {
		return fmt.Errorf("failed to delete node from database: %w", err)
	}

	// Remove Docker container for this node
	if m.containerMgr != nil {
		if err := m.containerMgr.RemoveNodeContainer(ctx, nodeID); err != nil {
			m.logger.Warn("Failed to remove node container",
				zap.Error(err),
				zap.String("node_id", nodeID))
			// Don't fail the deletion, just log the warning
		}
	}

	// Delete Docker volumes for this node
	if m.volumeMgr != nil {
		if err := m.volumeMgr.DeleteNodeVolumes(ctx, nodeID); err != nil {
			m.logger.Warn("Failed to delete node volumes",
				zap.Error(err),
				zap.String("node_id", nodeID))
			// Don't fail the deletion, just log the warning
		}
	}

	m.logger.Info("Node deleted permanently",
		zap.String("node_id", nodeID))

	return nil
}

// CreateNodeContainer creates a new node container
func (m *Manager) CreateNodeContainer(ctx context.Context, cfg *docker.NodeContainerConfig) (string, error) {
	if m.containerMgr == nil {
		return "", fmt.Errorf("container manager not initialized")
	}

	containerID, err := m.containerMgr.CreateNodeContainer(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create node container: %w", err)
	}

	m.logger.Info("Node container created",
		zap.String("container_id", containerID),
		zap.String("node_id", cfg.NodeID))

	return containerID, nil
}

// GetNodeContainerInfo returns information about a node container
func (m *Manager) GetNodeContainerInfo(ctx context.Context, nodeID string) (*docker.ContainerInfo, error) {
	if m.containerMgr == nil {
		return nil, fmt.Errorf("container manager not initialized")
	}

	return m.containerMgr.GetNodeContainerInfo(ctx, nodeID)
}

// GetNode retrieves a node by ID
func (m *Manager) GetNode(nodeID string) (*models.Node, error) {
	m.mu.RLock()
	state, exists := m.nodes[nodeID]
	m.mu.RUnlock()

	if exists {
		return state.Node, nil
	}

	// If not in memory, check database
	ctx := context.Background()
	node, err := m.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	if node == nil {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}
	return node, nil
}

// ListNodes retrieves all nodes (from memory and database)
func (m *Manager) ListNodes() ([]*models.Node, error) {
	// Get all nodes from database
	ctx := context.Background()
	dbNodes, err := m.nodeRepo.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes from database: %w", err)
	}

	// Merge with in-memory state (for real-time status)
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, node := range dbNodes {
		if state, exists := m.nodes[node.ID]; exists {
			// Update with real-time status from memory
			node.Status = state.Node.Status
		}
	}

	return dbNodes, nil
}

// UpdateNodeStatus updates the status of a node
func (m *Manager) UpdateNodeStatus(nodeID string, status models.NodeStatus) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	state.Node.Status = status
	state.LastHeartbeat = time.Now()

	return nil
}

// Update updates a node
func (m *Manager) Update(node *models.Node) error {
	m.mu.Lock()
	if state, exists := m.nodes[node.ID]; exists {
		state.Node = node
		state.LastHeartbeat = time.Now()
	}
	m.mu.Unlock()

	// Update in database
	ctx := context.Background()
	if err := m.nodeRepo.Update(ctx, node); err != nil {
		return fmt.Errorf("failed to update node in database: %w", err)
	}

	return nil
}

// UpdateNodeMetrics updates the metrics of a node
func (m *Manager) UpdateNodeMetrics(nodeID string, metrics *models.NodeMetrics) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	state.Metrics = metrics
	state.LastHeartbeat = time.Now()

	return nil
}

// SendCommand sends a command to a node
func (m *Manager) SendCommand(nodeID string, cmd *Command) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	select {
	case state.CommandQueue <- cmd:
		return nil
	default:
		return fmt.Errorf("command queue full for node: %s", nodeID)
	}
}

// HandleNodeEvent handles an event from a node
func (m *Manager) HandleNodeEvent(event *StreamEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.nodes[event.NodeID]
	if !exists {
		m.logger.Warn("Received event from unknown node",
			zap.String("node_id", event.NodeID),
			zap.String("event_type", string(event.Type)))
		return
	}

	state.LastHeartbeat = time.Now()

	// Broadcast event to subscribers
	m.streamsMu.RLock()
	defer m.streamsMu.RUnlock()

	for _, ch := range m.streams {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}

	m.logger.Debug("Node event received",
		zap.String("node_id", event.NodeID),
		zap.String("event_type", string(event.Type)))
}

// SubscribeToEvents creates a new subscription to node events
func (m *Manager) SubscribeToEvents(nodeID string) <-chan *StreamEvent {
	ch := make(chan *StreamEvent, 100)

	m.streamsMu.Lock()
	m.streams[fmt.Sprintf("%s-%d", nodeID, time.Now().UnixNano())] = ch
	m.streamsMu.Unlock()

	return ch
}

// UnsubscribeFromEvents removes a subscription
func (m *Manager) UnsubscribeFromEvents(ch <-chan *StreamEvent) {
	m.streamsMu.Lock()
	defer m.streamsMu.Unlock()

	for key, channel := range m.streams {
		if channel == ch {
			delete(m.streams, key)
			close(channel)
			break
		}
	}
}

// GetNodeMetrics retrieves the latest metrics for a node
func (m *Manager) GetNodeMetrics(nodeID string) (*models.NodeMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	return state.Metrics, nil
}

// GetClusterMetrics retrieves aggregated metrics for all nodes
func (m *Manager) GetClusterMetrics() (*ClusterMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := &ClusterMetrics{
		TotalNodes:     len(m.nodes),
		OnlineNodes:    0,
		OfflineNodes:   0,
	}

	for _, state := range m.nodes {
		if state.Node.Status == models.NodeStatusRunning {
			metrics.OnlineNodes++
		} else {
			metrics.OfflineNodes++
		}
	}

	return metrics, nil
}

// ClusterMetrics represents aggregated cluster metrics
type ClusterMetrics struct {
	TotalNodes   int
	OnlineNodes  int
	OfflineNodes int
}

// StartHealthCheck starts periodic health checks for all nodes
func (m *Manager) StartHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.GetNodeTimeout())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkNodeHealth()
		}
	}
}

// checkNodeHealth checks the health of all nodes
func (m *Manager) checkNodeHealth() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	timeout := m.cfg.GetNodeTimeout()
	now := time.Now()

	for _, state := range m.nodes {
		if !state.Connected {
			continue
		}

		if now.Sub(state.LastHeartbeat) > timeout {
			state.Node.Status = models.NodeStatusError
			m.logger.Warn("Node heartbeat timeout",
				zap.String("node_id", state.Node.ID),
				zap.String("name", state.Node.Name))
		}
	}
}
