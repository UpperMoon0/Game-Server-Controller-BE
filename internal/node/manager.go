package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/game-server/controller/internal/core/models"
	"github.com/game-server/controller/internal/core/repository"
	"github.com/game-server/controller/pkg/config"
	"go.uber.org/zap"
)

// Manager handles node lifecycle and communication
type Manager struct {
	nodeRepo     *repository.NodeRepository
	serverRepo   *repository.ServerRepository
	metricsRepo  *repository.MetricsRepository
	cfg          *config.Config
	logger       *zap.Logger
	
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
	CommandTypeCreateServer  CommandType = "create_server"
	CommandTypeUpdateServer  CommandType = "update_server"
	CommandTypeDeleteServer  CommandType = "delete_server"
	CommandTypeStartServer   CommandType = "start_server"
	CommandTypeStopServer    CommandType = "stop_server"
	CommandTypeRestartServer CommandType = "restart_server"
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
	serverRepo *repository.ServerRepository,
	metricsRepo *repository.MetricsRepository,
	cfg *config.Config,
	logger *zap.Logger,
) *Manager {
	return &Manager{
		nodeRepo:    nodeRepo,
		serverRepo:  serverRepo,
		metricsRepo: metricsRepo,
		cfg:         cfg,
		logger:      logger,
		nodes:       make(map[string]*NodeState),
		streams:     make(map[string]chan *StreamEvent),
	}
}

// RegisterNode registers a new node with the manager
func (m *Manager) RegisterNode(ctx context.Context, node *models.Node) error {
	m.mu.Lock()
	
	// Check if node already exists
	existing, exists := m.nodes[node.ID]
	if exists {
		// Update existing node state
		existing.Node = node
		existing.Connected = true
		existing.LastHeartbeat = time.Now()
		m.mu.Unlock()
		
		m.logger.Info("Node reconnected",
			zap.String("node_id", node.ID),
			zap.String("hostname", node.Hostname))
		return nil
	}

	// Create new node state
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
		zap.String("hostname", node.Hostname),
		zap.Strings("game_types", node.GameTypes))

	// Update database
	if err := m.nodeRepo.Create(ctx, node); err != nil {
		return fmt.Errorf("failed to create node in database: %w", err)
	}

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

// GetNode retrieves a node by ID
func (m *Manager) GetNode(nodeID string) (*models.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	return state.Node, nil
}

// ListNodes retrieves all registered nodes
func (m *Manager) ListNodes() ([]*models.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*models.Node, 0, len(m.nodes))
	for _, state := range m.nodes {
		nodes = append(nodes, state.Node)
	}

	return nodes, nil
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
	defer m.mu.Unlock()

	state, exists := m.nodes[node.ID]
	if !exists {
		return fmt.Errorf("node not found: %s", node.ID)
	}

	state.Node = node
	state.LastHeartbeat = time.Now()

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

	// Store metrics in Redis
	ctx := context.Background()
	if err := m.metricsRepo.StoreNodeMetrics(ctx, metrics); err != nil {
		m.logger.Error("Failed to store metrics", zap.Error(err))
	}

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
		TotalCPUCores:  0,
		UsedCPUCores:   0,
		TotalMemoryMB:  0,
		UsedMemoryMB:   0,
	}

	for _, state := range m.nodes {
		if state.Node.Status == models.NodeStatusOnline {
			metrics.OnlineNodes++
		} else {
			metrics.OfflineNodes++
		}

		metrics.TotalCPUCores += int64(state.Node.TotalCPUCores)
		metrics.TotalMemoryMB += state.Node.TotalMemoryMB

		if state.Metrics != nil {
			metrics.UsedCPUCores += int64(state.Metrics.CPUUsagePercent * float64(state.Node.TotalCPUCores) / 100)
			metrics.UsedMemoryMB += int64(state.Metrics.MemoryUsagePercent * float64(state.Node.TotalMemoryMB) / 100)
		}
	}

	return metrics, nil
}

// ClusterMetrics represents aggregated cluster metrics
type ClusterMetrics struct {
	TotalNodes    int
	OnlineNodes   int
	OfflineNodes  int
	TotalCPUCores int64
	UsedCPUCores  int64
	TotalMemoryMB int64
	UsedMemoryMB  int64
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
			state.Node.Status = models.NodeStatusUnhealthy
			m.logger.Warn("Node heartbeat timeout",
				zap.String("node_id", state.Node.ID),
				zap.String("hostname", state.Node.Hostname))
		}
	}
}
