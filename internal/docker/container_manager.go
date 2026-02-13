package docker

import (
	"context"
	"fmt"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
)

// ContainerManager manages Docker containers for game server nodes
type ContainerManager struct {
	client     *client.Client
	volumeMgr  *VolumeManager
	logger     *zap.Logger
}

// NodeContainerConfig holds configuration for creating a node container
type NodeContainerConfig struct {
	NodeID           string
	NodeName         string
	Image            string
	ControllerAddr   string
	MaxServers       int
	TotalCPUCores    int
	TotalMemoryMB    int64
	TotalStorageMB   int64
	GameTypes        []string
	NetworkName      string
}

// NewContainerManager creates a new container manager
func NewContainerManager(volumeMgr *VolumeManager, logger *zap.Logger) (*ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &ContainerManager{
		client:    cli,
		volumeMgr: volumeMgr,
		logger:    logger,
	}, nil
}

// CreateNodeContainer creates a new node container with volumes
func (cm *ContainerManager) CreateNodeContainer(ctx context.Context, cfg *NodeContainerConfig) (string, error) {
	// Create volumes first
	volumeNames := cm.volumeMgr.GetNodeVolumeNames(cfg.NodeID)
	if err := cm.createVolumes(ctx, volumeNames); err != nil {
		return "", fmt.Errorf("failed to create volumes: %w", err)
	}

	// Build environment variables
	envVars := []string{
		fmt.Sprintf("NODE_ID=%s", cfg.NodeID),
		fmt.Sprintf("NODE_NAME=%s", cfg.NodeName),
		fmt.Sprintf("CONTROLLER_ADDRESS=%s", cfg.ControllerAddr),
		"GRPC_ADDRESS=0.0.0.0:50051",
		"HEARTBEAT_INTERVAL=30",
		"NODE_TIMEOUT=120",
		"SERVER_DIRECTORY=/app/servers",
		"BACKUP_DIRECTORY=/app/backups",
		fmt.Sprintf("MAX_SERVERS=%d", cfg.MaxServers),
		"DEFAULT_MAX_PLAYERS=32",
		fmt.Sprintf("TOTAL_CPU_CORES=%d", cfg.TotalCPUCores),
		fmt.Sprintf("TOTAL_MEMORY_MB=%d", cfg.TotalMemoryMB),
		fmt.Sprintf("TOTAL_STORAGE_MB=%d", cfg.TotalStorageMB),
		"LOG_LEVEL=info",
		"LOG_FORMAT=json",
		"LOG_FILE_PATH=/app/logs/node.log",
		"ENVIRONMENT=production",
	}

	// Container name
	containerName := fmt.Sprintf("game-server-node-%s", cfg.NodeID)

	// Build volume binds
	binds := []string{
		fmt.Sprintf("%s:/app/servers", volumeNames[0]),
		fmt.Sprintf("%s:/app/backups", volumeNames[1]),
		fmt.Sprintf("%s:/app/logs", volumeNames[2]),
	}

	// Container configuration
	containerConfig := &container.Config{
		Image: cfg.Image,
		Env:   envVars,
		ExposedPorts: nat.PortSet{
			"50051/tcp": struct{}{},
		},
		Labels: map[string]string{
			"game-server.node-id":   cfg.NodeID,
			"game-server.node-name": cfg.NodeName,
			"game-server.managed":   "true",
		},
	}

	// Host configuration
	hostConfig := &container.HostConfig{
		Binds: binds,
		PortBindings: nat.PortMap{
			"50051/tcp": []nat.PortBinding{
				{HostIP: "0.0.0.0"}, // Docker will assign random port
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Resources: container.Resources{
			NanoCPUs: int64(cfg.TotalCPUCores) * 1e9,
			Memory:   cfg.TotalMemoryMB * 1024 * 1024,
		},
	}

	// Network configuration
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			cfg.NetworkName: {},
		},
	}

	// Create container
	resp, err := cm.client.ContainerCreate(ctx,
		containerConfig,
		hostConfig,
		networkConfig,
		nil,
		containerName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := cm.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		// Clean up container on start failure
		cm.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	cm.logger.Info("Node container created and started",
		zap.String("container_id", resp.ID),
		zap.String("node_id", cfg.NodeID),
		zap.String("container_name", containerName))

	return resp.ID, nil
}

// createVolumes creates the volumes for a node
func (cm *ContainerManager) createVolumes(ctx context.Context, volumeNames []string) error {
	for _, name := range volumeNames {
		_, err := cm.client.VolumeCreate(ctx, volume.CreateOptions{
			Name: name,
		})
		if err != nil {
			return fmt.Errorf("failed to create volume %s: %w", name, err)
		}
		cm.logger.Debug("Created volume", zap.String("volume", name))
	}
	return nil
}

// StopNodeContainer stops a node container
func (cm *ContainerManager) StopNodeContainer(ctx context.Context, nodeID string) error {
	containerID, err := cm.findContainerByNodeID(ctx, nodeID)
	if err != nil {
		return err
	}
	if containerID == "" {
		return fmt.Errorf("container not found for node: %s", nodeID)
	}

	timeout := 30
	if err := cm.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	cm.logger.Info("Node container stopped",
		zap.String("node_id", nodeID),
		zap.String("container_id", containerID))

	return nil
}

// RemoveNodeContainer removes a node container
func (cm *ContainerManager) RemoveNodeContainer(ctx context.Context, nodeID string) error {
	containerID, err := cm.findContainerByNodeID(ctx, nodeID)
	if err != nil {
		return err
	}
	if containerID == "" {
		cm.logger.Debug("No container found for node, already removed", zap.String("node_id", nodeID))
		return nil
	}

	// Remove container
	if err := cm.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	cm.logger.Info("Node container removed",
		zap.String("node_id", nodeID),
		zap.String("container_id", containerID))

	return nil
}

// GetNodeContainerInfo returns information about a node container
func (cm *ContainerManager) GetNodeContainerInfo(ctx context.Context, nodeID string) (*ContainerInfo, error) {
	containerID, err := cm.findContainerByNodeID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if containerID == "" {
		return nil, nil
	}

	info, err := cm.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Get the host port
	var hostPort int
	if ports, ok := info.NetworkSettings.Ports["50051/tcp"]; ok && len(ports) > 0 {
		hostPort, _ = strconv.Atoi(ports[0].HostPort)
	}

	return &ContainerInfo{
		ID:         info.ID,
		Name:       info.Name,
		Status:     info.State.Status,
		HostPort:   hostPort,
		IPAddress:  info.NetworkSettings.IPAddress,
		Created:    info.Created,
		Image:      info.Config.Image,
	}, nil
}

// ListNodeContainers lists all node containers
func (cm *ContainerManager) ListNodeContainers(ctx context.Context) ([]*ContainerInfo, error) {
	containers, err := cm.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "label",
			Value: "game-server.managed=true",
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []*ContainerInfo
	for _, c := range containers {
		info := &ContainerInfo{
			ID:      c.ID,
			Name:    c.Names[0],
			Status:  c.State,
			Image:   c.Image,
			NodeID:  c.Labels["game-server.node-id"],
		}
		result = append(result, info)
	}

	return result, nil
}

// findContainerByNodeID finds a container by node ID label
func (cm *ContainerManager) findContainerByNodeID(ctx context.Context, nodeID string) (string, error) {
	containers, err := cm.client.ContainerList(ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "label",
			Value: fmt.Sprintf("game-server.node-id=%s", nodeID),
		}),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return "", nil
	}

	return containers[0].ID, nil
}

// Close closes the Docker client
func (cm *ContainerManager) Close() error {
	return cm.client.Close()
}

// ContainerInfo holds information about a container
type ContainerInfo struct {
	ID        string
	Name      string
	Status    string
	HostPort  int
	IPAddress string
	Created   string
	Image     string
	NodeID    string
}
