package scheduler

import (
	"fmt"
	"time"

	"github.com/game-server/controller/internal/core/models"
	nodepkg "github.com/game-server/controller/internal/node"
	"go.uber.org/zap"
)

// Scheduler handles resource allocation and node lifecycle
// Note: Node status is managed by node agents, not stored in DB
type Scheduler struct {
	nodeMgr *nodepkg.Manager
	logger  *zap.Logger
}

// NewScheduler creates a new scheduler
func NewScheduler(
	nodeMgr *nodepkg.Manager,
	logger *zap.Logger,
) *Scheduler {
	return &Scheduler{
		nodeMgr: nodeMgr,
		logger:  logger,
	}
}

// StartNode sends a start command to a node agent
func (s *Scheduler) StartNode(nodeID string) error {
	// Send start command to node agent
	cmd := &nodepkg.Command{
		ID:   generateCommandID(),
		Type: nodepkg.CommandTypeStart,
		Payload: map[string]interface{}{
			"node_id": nodeID,
		},
		Response: make(chan *nodepkg.CommandResult, 1),
	}

	if err := s.nodeMgr.SendCommand(nodeID, cmd); err != nil {
		return fmt.Errorf("failed to send start command: %w", err)
	}

	return nil
}

// StopNode sends a stop command to a node agent
func (s *Scheduler) StopNode(nodeID string) error {
	// Send stop command to node agent
	cmd := &nodepkg.Command{
		ID:   generateCommandID(),
		Type: nodepkg.CommandTypeStop,
		Payload: map[string]interface{}{
			"node_id": nodeID,
		},
		Response: make(chan *nodepkg.CommandResult, 1),
	}

	if err := s.nodeMgr.SendCommand(nodeID, cmd); err != nil {
		return fmt.Errorf("failed to send stop command: %w", err)
	}

	return nil
}

// RestartNode restarts a node
func (s *Scheduler) RestartNode(nodeID string) error {
	// Stop then start
	if err := s.StopNode(nodeID); err != nil {
		return err
	}

	// Wait for stop
	time.Sleep(5 * time.Second)

	return s.StartNode(nodeID)
}

// GetNodeCounts returns node counts by status (fetched from node agents via node manager)
func (s *Scheduler) GetNodeCounts() (map[models.NodeStatus]int, error) {
	metrics, err := s.nodeMgr.GetClusterMetrics()
	if err != nil {
		return nil, err
	}
	
	counts := make(map[models.NodeStatus]int)
	counts[models.NodeStatusRunning] = metrics.OnlineNodes
	counts[models.NodeStatusOffline] = metrics.OfflineNodes
	counts[models.NodeStatusStopped] = metrics.TotalNodes - metrics.OnlineNodes - metrics.OfflineNodes
	
	return counts, nil
}

// Helper functions

func generateCommandID() string {
	return fmt.Sprintf("cmd-%d", time.Now().UnixNano())
}