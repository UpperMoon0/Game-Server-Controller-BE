package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/game-server/controller/internal/core/models"
	"github.com/game-server/controller/internal/node"
	"github.com/game-server/controller/internal/scheduler"
	"go.uber.org/zap"
)

// NodeHandler handles REST API requests for nodes
type NodeHandler struct {
	nodeRepo   *node.Manager
	scheduler  *scheduler.Scheduler
	logger     *zap.Logger
}

// NewNodeHandler creates a new node handler
func NewNodeHandler(nodeRepo *node.Manager, scheduler *scheduler.Scheduler, logger *zap.Logger) *NodeHandler {
	return &NodeHandler{
		nodeRepo:  nodeRepo,
		scheduler: scheduler,
		logger:    logger,
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

// CreateNode creates a new node
func (h *NodeHandler) CreateNode(c *gin.Context) {
	var req models.CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	// Set default port if not provided
	port := req.Port
	if port == 0 {
		port = 8080
	}

	node := &models.Node{
		Name:              req.Name,
		Port:              port,
		Status:            models.NodeStatusOffline,
		GameType:          req.GameType,
		HeartbeatInterval: 30,
	}

	ctx := c.Request.Context()
	if err := h.nodeRepo.RegisterNode(ctx, node); err != nil {
		h.logger.Error("Failed to create node", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create node",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"node":     node,
		"message": "Node created successfully",
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

// DeleteNode deletes a node
func (h *NodeHandler) DeleteNode(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
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
