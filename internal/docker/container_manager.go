package docker

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
)

// UpdateResult holds the result of a container update operation
type UpdateResult struct {
	Updated      bool   // Whether the container was actually updated
	Skipped      bool   // Whether the update was skipped (already up-to-date)
	Message      string // Human-readable message
	ContainerID  string // New container ID if updated
	OldImage     string // Previous image
	NewImage     string // New image (same as old if skipped)
	OldDigest    string // Previous image digest
	NewDigest    string // New image digest
}

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
	// Pull the latest image first
	cm.logger.Info("Pulling latest node agent image", zap.String("image", cfg.Image))
	reader, err := cm.client.ImagePull(ctx, cfg.Image, image.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image %s: %w", cfg.Image, err)
	}
	// Must read all data from the reader to ensure the pull is complete
	// Just closing the reader doesn't wait for the pull to finish
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		reader.Close()
		return "", fmt.Errorf("failed to complete image pull: %w", err)
	}
	reader.Close()
	cm.logger.Info("Successfully pulled image", zap.String("image", cfg.Image))

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

	// Add game types as environment variable if provided
	if len(cfg.GameTypes) > 0 {
		gameTypesStr := cfg.GameTypes[0]
		for i := 1; i < len(cfg.GameTypes); i++ {
			gameTypesStr += "," + cfg.GameTypes[i]
		}
		envVars = append(envVars, fmt.Sprintf("GAME_TYPES=%s", gameTypesStr))
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
		// Add host.docker.internal mapping for Linux compatibility
		// This allows node containers to reach the controller on the host
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
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
	if err := cm.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Clean up container on start failure
		_ = cm.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
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
	if err := cm.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
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
	containers, err := cm.client.ContainerList(ctx, container.ListOptions{
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
	containers, err := cm.client.ContainerList(ctx, container.ListOptions{
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

// UpdateNodeContainer updates a node container to the latest image while preserving data
// Returns UpdateResult indicating whether the update was performed or skipped
func (cm *ContainerManager) UpdateNodeContainer(ctx context.Context, nodeID string, latestImage string) (*UpdateResult, error) {
	// Find the existing container
	containerID, err := cm.findContainerByNodeID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if containerID == "" {
		return nil, fmt.Errorf("container not found for node: %s", nodeID)
	}

	// Inspect the existing container to get its configuration
	info, err := cm.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	currentImage := info.Config.Image

	cm.logger.Info("Checking for node container image update",
		zap.String("node_id", nodeID),
		zap.String("current_image", currentImage),
		zap.String("latest_image", latestImage),
		zap.String("container_id", containerID))

	// Get the digest of the currently running image
	currentDigest, err := cm.getImageDigest(ctx, currentImage)
	if err != nil {
		cm.logger.Warn("Failed to get current image digest, proceeding with update", zap.Error(err))
		currentDigest = ""
	}

	// Pull the latest image
	cm.logger.Info("Pulling latest image", zap.String("image", latestImage))
	reader, err := cm.client.ImagePull(ctx, latestImage, image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", latestImage, err)
	}
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		reader.Close()
		return nil, fmt.Errorf("failed to complete image pull: %w", err)
	}
	reader.Close()
	cm.logger.Info("Successfully pulled latest image", zap.String("image", latestImage))

	// Get the digest of the newly pulled image
	latestDigest, err := cm.getImageDigest(ctx, latestImage)
	if err != nil {
		cm.logger.Warn("Failed to get latest image digest", zap.Error(err))
		latestDigest = ""
	}

	// Compare digests - skip update if they match
	if currentDigest != "" && latestDigest != "" && currentDigest == latestDigest {
		cm.logger.Info("Image is already up-to-date, skipping update",
			zap.String("node_id", nodeID),
			zap.String("digest", currentDigest))

		return &UpdateResult{
			Updated:   false,
			Skipped:   true,
			Message:   "Container is already running the latest image",
			OldImage:  currentImage,
			NewImage:  latestImage,
			OldDigest: currentDigest,
			NewDigest: latestDigest,
		}, nil
	}

	cm.logger.Info("Image update required",
		zap.String("node_id", nodeID),
		zap.String("old_digest", currentDigest),
		zap.String("new_digest", latestDigest))

	// Stop the container
	timeout := 30
	if err := cm.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return nil, fmt.Errorf("failed to stop container: %w", err)
	}
	cm.logger.Info("Stopped container for update", zap.String("container_id", containerID))

	// Get the volume binds from the old container
	var binds []string
	for _, mount := range info.Mounts {
		if mount.Type == "volume" {
			binds = append(binds, fmt.Sprintf("%s:%s", mount.Name, mount.Destination))
		}
	}

	// Get environment variables from old container
	envVars := info.Config.Env

	// Get labels from old container
	labels := info.Config.Labels

	// Get network settings
	var networkName string
	for name := range info.NetworkSettings.Networks {
		networkName = name
		break
	}

	// Get port bindings
	var hostPort string
	if ports, ok := info.NetworkSettings.Ports["50051/tcp"]; ok && len(ports) > 0 {
		hostPort = ports[0].HostPort
	}

	// Remove the old container
	if err := cm.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	}); err != nil {
		return nil, fmt.Errorf("failed to remove old container: %w", err)
	}
	cm.logger.Info("Removed old container", zap.String("container_id", containerID))

	// Create new container with the same configuration but new image
	containerName := info.Name
	if containerName[0] == '/' {
		containerName = containerName[1:]
	}

	// Container configuration
	containerConfig := &container.Config{
		Image:       latestImage,
		Env:         envVars,
		Labels:      labels,
		ExposedPorts: nat.PortSet{
			"50051/tcp": struct{}{},
		},
	}

	// Host configuration
	hostConfig := &container.HostConfig{
		Binds: binds,
		PortBindings: nat.PortMap{
			"50051/tcp": []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: hostPort},
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Resources: info.HostConfig.Resources,
		// Add host.docker.internal mapping for Linux compatibility
		// This allows node containers to reach the controller on the host
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
	}

	// Network configuration
	networkConfig := &network.NetworkingConfig{}
	if networkName != "" {
		networkConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			networkName: {},
		}
	}

	// Create new container
	resp, err := cm.client.ContainerCreate(ctx,
		containerConfig,
		hostConfig,
		networkConfig,
		nil,
		containerName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new container: %w", err)
	}

	// Start the new container
	if err := cm.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Clean up container on start failure
		_ = cm.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to start new container: %w", err)
	}

	cm.logger.Info("Node container updated successfully",
		zap.String("node_id", nodeID),
		zap.String("old_container_id", containerID),
		zap.String("new_container_id", resp.ID),
		zap.String("new_image", latestImage))

	return &UpdateResult{
		Updated:     true,
		Skipped:     false,
		Message:     "Container updated successfully",
		ContainerID: resp.ID,
		OldImage:    currentImage,
		NewImage:    latestImage,
		OldDigest:   currentDigest,
		NewDigest:   latestDigest,
	}, nil
}

// getImageDigest gets the digest of an image
func (cm *ContainerManager) getImageDigest(ctx context.Context, imageName string) (string, error) {
	// Inspect the image to get its digest
	imgInspect, _, err := cm.client.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return "", fmt.Errorf("failed to inspect image %s: %w", imageName, err)
	}

	// The digest is in the RepoDigests field for pulled images
	// Format: repo@sha256:digest
	if len(imgInspect.RepoDigests) > 0 {
		// Extract just the digest part
		for _, repoDigest := range imgInspect.RepoDigests {
			if idx := len(imageName); idx < len(repoDigest) && repoDigest[:idx] == imageName {
				return repoDigest[idx+1:], nil // +1 to skip the @ symbol
			}
			// Try without registry prefix
			if idx := findDigestInRepoDigest(repoDigest, imageName); idx != "" {
				return idx, nil
			}
		}
		// Return the digest from the first RepoDigest if image name doesn't match
		for _, repoDigest := range imgInspect.RepoDigests {
			if atIdx := findAtSymbol(repoDigest); atIdx >= 0 {
				return repoDigest[atIdx+1:], nil
			}
		}
	}

	// Fallback to the image ID (which is a digest of the manifest)
	if imgInspect.ID != "" {
		// Image ID format: sha256:digest
		if len(imgInspect.ID) > 7 && imgInspect.ID[:7] == "sha256:" {
			return imgInspect.ID, nil
		}
	}

	return "", fmt.Errorf("could not find digest for image %s", imageName)
}

// findDigestInRepoDigest extracts digest from repo digest string
func findDigestInRepoDigest(repoDigest, imageName string) string {
	// repoDigest format: registry/repo@sha256:digest or repo@sha256:digest
	// imageName could be: repo:tag or repo or registry/repo:tag
	
	// Strip tag from imageName if present
	baseImageName := imageName
	for i := 0; i < len(imageName); i++ {
		if imageName[i] == ':' && i > 0 {
			// Check if this is not a registry port (has / before :)
			hasSlash := false
			for j := 0; j < i; j++ {
				if imageName[j] == '/' {
					hasSlash = true
					break
				}
			}
			if hasSlash || i < len(imageName)-1 {
				// This is a tag, not a port
				baseImageName = imageName[:i]
				break
			}
		}
	}
	
	atIdx := findAtSymbol(repoDigest)
	if atIdx >= 0 {
		// Check if the repo part matches
		repoPart := repoDigest[:atIdx]
		if repoPart == baseImageName || repoPart == imageName {
			return repoDigest[atIdx+1:]
		}
	}
	return ""
}

// findAtSymbol finds the @ symbol in a repo digest string
func findAtSymbol(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '@' {
			return i
		}
	}
	return -1
}

// RestartNodeContainer restarts a node container
func (cm *ContainerManager) RestartNodeContainer(ctx context.Context, nodeID string) error {
	containerID, err := cm.findContainerByNodeID(ctx, nodeID)
	if err != nil {
		return err
	}
	if containerID == "" {
		return fmt.Errorf("container not found for node: %s", nodeID)
	}

	timeout := 30
	if err := cm.client.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}

	cm.logger.Info("Node container restarted",
		zap.String("node_id", nodeID),
		zap.String("container_id", containerID))

	return nil
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
