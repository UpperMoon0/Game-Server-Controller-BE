package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

// VolumeManager manages Docker volumes for game server nodes
type VolumeManager struct {
	client *client.Client
	logger *zap.Logger
}

// VolumeConfig holds configuration for volume naming
type VolumeConfig struct {
	Prefix string // e.g., "game-server-node-"
}

// NewVolumeManager creates a new volume manager
func NewVolumeManager(logger *zap.Logger) (*VolumeManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &VolumeManager{
		client: cli,
		logger: logger,
	}, nil
}

// GetNodeVolumeNames returns the expected volume names for a node
func (vm *VolumeManager) GetNodeVolumeNames(nodeID string) []string {
	// Volume names follow the pattern: game-server-node-{type}
	// For node-specific volumes, we use the node ID
	return []string{
		fmt.Sprintf("game-server-node-%s-servers", nodeID),
		fmt.Sprintf("game-server-node-%s-backups", nodeID),
		fmt.Sprintf("game-server-node-%s-logs", nodeID),
	}
}

// DeleteNodeVolumes deletes all volumes associated with a node
func (vm *VolumeManager) DeleteNodeVolumes(ctx context.Context, nodeID string) error {
	volumeNames := vm.GetNodeVolumeNames(nodeID)
	
	var errors []string
	for _, volumeName := range volumeNames {
		if err := vm.deleteVolume(ctx, volumeName); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", volumeName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to delete some volumes: %s", strings.Join(errors, ", "))
	}

	vm.logger.Info("Deleted all volumes for node", zap.String("node_id", nodeID))
	return nil
}

// deleteVolume deletes a single volume by name
func (vm *VolumeManager) deleteVolume(ctx context.Context, volumeName string) error {
	// Check if volume exists
	volumes, err := vm.client.VolumeList(ctx, volume.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "name",
			Value: volumeName,
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to list volumes: %w", err)
	}

	// Find the exact volume
	var found bool
	for _, v := range volumes.Volumes {
		if v.Name == volumeName {
			found = true
			break
		}
	}

	if !found {
		vm.logger.Debug("Volume not found, skipping deletion", zap.String("volume", volumeName))
		return nil
	}

	// Remove the volume
	if err := vm.client.VolumeRemove(ctx, volumeName, true); err != nil {
		return fmt.Errorf("failed to remove volume %s: %w", volumeName, err)
	}

	vm.logger.Info("Deleted volume", zap.String("volume", volumeName))
	return nil
}

// ListNodeVolumes lists all volumes for a node
func (vm *VolumeManager) ListNodeVolumes(ctx context.Context, nodeID string) ([]*volume.Volume, error) {
	volumeNames := vm.GetNodeVolumeNames(nodeID)
	
	volumes, err := vm.client.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	var nodeVolumes []*volume.Volume
	for _, v := range volumes.Volumes {
		for _, name := range volumeNames {
			if v.Name == name {
				nodeVolumes = append(nodeVolumes, v)
				break
			}
		}
	}

	return nodeVolumes, nil
}

// GetVolumeUsage returns the total size of volumes for a node
func (vm *VolumeManager) GetVolumeUsage(ctx context.Context, nodeID string) (int64, error) {
	volumes, err := vm.ListNodeVolumes(ctx, nodeID)
	if err != nil {
		return 0, err
	}

	var totalSize int64
	for _, v := range volumes {
		if v.UsageData != nil {
			totalSize += v.UsageData.Size
		}
	}

	return totalSize, nil
}

// Close closes the Docker client
func (vm *VolumeManager) Close() error {
	return vm.client.Close()
}
