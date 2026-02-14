package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/game-server/controller/internal/core/models"
	"github.com/game-server/controller/internal/docker"
	"github.com/game-server/controller/internal/node"
	"github.com/game-server/controller/internal/scheduler"
	"github.com/game-server/controller/pkg/config"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NodeHandler handles REST API requests for nodes
type NodeHandler struct {
	nodeRepo     *node.Manager
	scheduler    *scheduler.Scheduler
	containerMgr *docker.ContainerManager
	cfg          *config.Config
	logger       *zap.Logger
}

// NewNodeHandler creates a new node handler
func NewNodeHandler(
	nodeRepo *node.Manager,
	scheduler *scheduler.Scheduler,
	containerMgr *docker.ContainerManager,
	cfg *config.Config,
	logger *zap.Logger,
) *NodeHandler {
	return &NodeHandler{
		nodeRepo:     nodeRepo,
		scheduler:    scheduler,
		containerMgr: containerMgr,
		cfg:          cfg,
		logger:       logger,
	}
}

// RegisterRoutes registers the node routes
func (h *NodeHandler) RegisterRoutes(router *gin.RouterGroup) {
	nodes := router.Group("/nodes")
	{
		nodes.GET("", h.ListNodes)
		nodes.POST("", h.CreateNode)
		nodes.GET("/:id", h.GetNode)
		nodes.PUT("/:id", h.UpdateNode)
		nodes.DELETE("/:id", h.DeleteNode)
		nodes.GET("/:id/status", h.GetNodeStatus)
		nodes.GET("/:id/metrics", h.GetNodeMetrics)
		nodes.POST("/:id/action", h.NodeAction)
	}
}

// ListNodes returns a list of all nodes
func (h *NodeHandler) ListNodes(c *gin.Context) {
	status := c.Query("status")
	var nodeStatus *models.NodeStatus
	if status != "" {
		s := models.NodeStatus(status)
		nodeStatus = &s
	}

	nodes, err := h.nodeRepo.ListNodes()
	if err != nil {
		h.logger.Error("Failed to list nodes", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list nodes",
			"message": err.Error(),
		})
		return
	}

	// Filter by status if provided
	if nodeStatus != nil {
		filtered := make([]*models.Node, 0)
		for _, n := range nodes {
			if n.Status == *nodeStatus {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes":      nodes,
		"total":      len(nodes),
		"online":     countNodesByStatus(nodes, models.NodeStatusOnline),
		"offline":    countNodesByStatus(nodes, models.NodeStatusOffline),
	})
}

// GetNode returns a single node by ID
func (h *NodeHandler) GetNode(c *gin.Context) {
	id := c.Param("id")

	node, err := h.nodeRepo.GetNode(id)
	if err != nil {
		h.logger.Error("Failed to get node", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Node not found",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, node)
}

// CreateNode creates a new node by starting a node agent container
func (h *NodeHandler) CreateNode(c *gin.Context) {
	var req models.CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	// Check if container manager is available
	if h.containerMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Container manager not available",
			"message": "Docker is not configured or unavailable",
		})
		return
	}

	// Set default port if not provided
	port := req.Port
	if port == 0 {
		port = 8080
	}

	// Generate node ID
	nodeID := generateNodeID()

	// Create node container config
	containerCfg := &docker.NodeContainerConfig{
		NodeID:         nodeID,
		NodeName:       req.Name,
		Image:          h.cfg.NodeAgentImage,
		ControllerAddr: h.cfg.GetGRPCAddress(),
		GameTypes:      []string{req.GameType},
		NetworkName:    h.cfg.NodeNetworkName,
	}

	// Start the node agent container (this will create volumes automatically)
	ctx := c.Request.Context()
	containerID, err := h.containerMgr.CreateNodeContainer(ctx, containerCfg)
	if err != nil {
		h.logger.Error("Failed to create node container", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create node container",
			"message": err.Error(),
		})
		return
	}

	h.logger.Info("Node container started",
		zap.String("node_id", nodeID),
		zap.String("container_id", containerID),
		zap.String("name", req.Name))

	// Return the node info - the node agent will register itself via gRPC
	c.JSON(http.StatusCreated, gin.H{
		"node": gin.H{
			"id":                nodeID,
			"name":              req.Name,
			"port":              port,
			"status":            "pending",
			"game_type":         req.GameType,
			"container_id":      containerID,
		},
		"message": "Node container started, waiting for registration",
	})
}

// UpdateNode updates a node
func (h *NodeHandler) UpdateNode(c *gin.Context) {
	id := c.Param("id")

	var req models.UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	node, err := h.nodeRepo.GetNode(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Node not found",
			"message": err.Error(),
		})
		return
	}

	// Update fields
	if req.Name != nil {
		node.Name = *req.Name
	}
	if req.Port != nil {
		node.Port = *req.Port
	}
	if req.GameType != nil {
		node.GameType = *req.GameType
	}
	if req.HeartbeatInterval != nil {
		node.HeartbeatInterval = *req.HeartbeatInterval
	}
	if req.Status != nil {
		node.Status = *req.Status
	}

	if err := h.nodeRepo.Update(node); err != nil {
		h.logger.Error("Failed to update node", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update node",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"node":     node,
		"message": "Node updated successfully",
	})
}

// DeleteNode deletes a node by removing its container and volumes first
func (h *NodeHandler) DeleteNode(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()

	// First, remove the container if container manager is available
	if h.containerMgr != nil {
		if err := h.containerMgr.RemoveNodeContainer(ctx, id); err != nil {
			h.logger.Warn("Failed to remove node container (may not exist)",
				zap.Error(err),
				zap.String("node_id", id))
			// Continue with deletion even if container removal fails
		}
	}

	// Delete the node (this will also delete volumes via node manager)
	if err := h.nodeRepo.DeleteNode(ctx, id); err != nil {
		h.logger.Error("Failed to delete node", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete node",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// GetNodeStatus returns the current status of a node
func (h *NodeHandler) GetNodeStatus(c *gin.Context) {
	id := c.Param("id")

	metrics, err := h.nodeRepo.GetNodeMetrics(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Node not found",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"node_id":   id,
		"status":    "online",
		"metrics":   metrics,
	})
}

// GetNodeMetrics returns the metrics of a node
func (h *NodeHandler) GetNodeMetrics(c *gin.Context) {
	id := c.Param("id")

	metrics, err := h.nodeRepo.GetNodeMetrics(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Node not found",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"node_id": id,
		"metrics": metrics,
	})
}

// NodeAction performs an action on a node
func (h *NodeHandler) NodeAction(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	switch req.Action {
	case "maintenance":
		// Set node to maintenance mode
		node, err := h.nodeRepo.GetNode(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Node not found",
				"message": err.Error(),
			})
			return
		}
		node.Status = models.NodeStatusMaintenance
		if err := h.nodeRepo.Update(node); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to set maintenance mode",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Node set to maintenance mode",
		})

	case "refresh":
		// Refresh node connection
		c.JSON(http.StatusOK, gin.H{
			"message": "Node refresh requested",
		})

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid action",
			"message": "Unknown action: " + req.Action,
		})
	}
}

// Helper function to count nodes by status
func countNodesByStatus(nodes []*models.Node, status models.NodeStatus) int {
	count := 0
	for _, n := range nodes {
		if n.Status == status {
			count++
		}
	}
	return count
}

// generateNodeID generates a unique node ID
func generateNodeID() string {
	return uuid.New().String()
}
