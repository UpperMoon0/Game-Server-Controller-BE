package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/game-server/controller/internal/core/models"
	"github.com/game-server/controller/internal/core/repository"
	nodepkg "github.com/game-server/controller/internal/node"
	"go.uber.org/zap"
)

// Scheduler handles resource allocation and node lifecycle
type Scheduler struct {
	nodeRepo *repository.NodeRepository
	nodeMgr  *nodepkg.Manager
	logger   *zap.Logger
}

// NewScheduler creates a new scheduler
func NewScheduler(
	nodeRepo *repository.NodeRepository,
	nodeMgr *nodepkg.Manager,
	logger *zap.Logger,
) *Scheduler {
	return &Scheduler{
		nodeRepo: nodeRepo,
		nodeMgr:  nodeMgr,
		logger:   logger,
	}
}

// StartNode starts a node
func (s *Scheduler) StartNode(ctx context.Context, nodeID string) error {
	node, err := s.nodeMgr.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Update status
	node.Status = models.NodeStatusStarting
	if err := s.nodeRepo.Update(ctx, node); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Send start command to node
	cmd := &nodepkg.Command{
		ID:   generateCommandID(),
		Type: nodepkg.CommandTypeStart,
		Payload: map[string]interface{}{
			"node_id": nodeID,
		},
		Response: make(chan *nodepkg.CommandResult, 1),
	}

	if err := s.nodeMgr.SendCommand(nodeID, cmd); err != nil {
		node.Status = models.NodeStatusStopped
		s.nodeRepo.Update(ctx, node)
		return fmt.Errorf("failed to send start command: %w", err)
	}

	return nil
}

// StopNode stops a node
func (s *Scheduler) StopNode(ctx context.Context, nodeID string) error {
	node, err := s.nodeMgr.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Update status
	node.Status = models.NodeStatusStopping
	if err := s.nodeRepo.Update(ctx, node); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Send stop command to node
	cmd := &nodepkg.Command{
		ID:   generateCommandID(),
		Type: nodepkg.CommandTypeStop,
		Payload: map[string]interface{}{
			"node_id": nodeID,
		},
		Response: make(chan *nodepkg.CommandResult, 1),
	}

	if err := s.nodeMgr.SendCommand(nodeID, cmd); err != nil {
		node.Status = models.NodeStatusRunning
		s.nodeRepo.Update(ctx, node)
		return fmt.Errorf("failed to send stop command: %w", err)
	}

	return nil
}

// RestartNode restarts a node
func (s *Scheduler) RestartNode(ctx context.Context, nodeID string) error {
	// Stop then start
	if err := s.StopNode(ctx, nodeID); err != nil {
		return err
	}

	// Wait for stop
	time.Sleep(5 * time.Second)

	return s.StartNode(ctx, nodeID)
}

// GetNodeCounts returns node counts by status
func (s *Scheduler) GetNodeCounts() (map[models.NodeStatus]int, error) {
	ctx := context.Background()
	return s.nodeRepo.CountByStatus(ctx)
}

// Helper functions

func generateCommandID() string {
	return fmt.Sprintf("cmd-%d", time.Now().UnixNano())
}
