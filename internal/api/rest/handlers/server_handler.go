package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/game-server/controller/internal/core/models"
	"github.com/game-server/controller/internal/node"
	"github.com/game-server/controller/internal/scheduler"
	"go.uber.org/zap"
)

// ServerHandler handles REST API requests for servers
type ServerHandler struct {
	serverRepo *node.Manager
	scheduler  *scheduler.Scheduler
	logger     *zap.Logger
}

// NewServerHandler creates a new server handler
func NewServerHandler(serverRepo *node.Manager, scheduler *scheduler.Scheduler, logger *zap.Logger) *ServerHandler {
	return &ServerHandler{
		serverRepo: serverRepo,
		scheduler:  scheduler,
		logger:     logger,
	}
}

// RegisterRoutes registers the server routes
func (h *ServerHandler) RegisterRoutes(router *gin.RouterGroup) {
	servers := router.Group("/servers")
	{
		servers.GET("", h.ListServers)
		servers.POST("", h.CreateServer)
		servers.GET("/:id", h.GetServer)
		servers.PUT("/:id", h.UpdateServer)
		servers.DELETE("/:id", h.DeleteServer)
		servers.POST("/:id/action", h.ServerAction)
		servers.GET("/:id/status", h.GetServerStatus)
		servers.GET("/:id/logs", h.GetServerLogs)
		servers.GET("/:id/metrics", h.GetServerMetrics)
	}
}

// ListServers returns a list of all servers
func (h *ServerHandler) ListServers(c *gin.Context) {
	var filters models.ServerFilters
	if err := c.ShouldBindQuery(&filters); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid query parameters",
			"message": err.Error(),
		})
		return
	}

	// Set default values
	if filters.Limit == 0 {
		filters.Limit = 50
	}

	// Parse query parameters manually
	if nodeID := c.Query("node_id"); nodeID != "" {
		filters.NodeID = nodeID
	}
	if status := c.Query("status"); status != "" {
		filters.Status = models.ServerStatus(status)
	}
	if gameType := c.Query("game_type"); gameType != "" {
		filters.GameType = gameType
	}

	// Get servers from scheduler (which manages server lifecycle)
	servers, err := h.scheduler.ListServers(&filters)
	if err != nil {
		h.logger.Error("Failed to list servers", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list servers",
			"message": err.Error(),
		})
		return
	}

	// Count by status
	running := 0
	stopped := 0
	installing := 0
	errorCount := 0
	for _, s := range servers {
		switch s.Status {
		case models.ServerStatusRunning:
			running++
		case models.ServerStatusStopped:
			stopped++
		case models.ServerStatusInstalling:
			installing++
		case models.ServerStatusError:
			errorCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"servers":     servers,
		"total":       len(servers),
		"running":     running,
		"stopped":     stopped,
		"installing":  installing,
		"error":       errorCount,
	})
}

// GetServer returns a single server by ID
func (h *ServerHandler) GetServer(c *gin.Context) {
	id := c.Param("id")

	server, err := h.scheduler.GetServer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Server not found",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, server)
}

// CreateServer creates a new server
func (h *ServerHandler) CreateServer(c *gin.Context) {
	var req models.CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	result, err := h.scheduler.CreateServer(ctx, &req)
	if err != nil {
		h.logger.Error("Failed to create server", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create server",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"server_id":   result.ServerID,
		"server_info": result.ServerInfo,
		"message":     result.Message,
	})
}

// UpdateServer updates a server
func (h *ServerHandler) UpdateServer(c *gin.Context) {
	id := c.Param("id")

	var req models.UpdateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	if err := h.scheduler.UpdateServer(ctx, id, req); err != nil {
		h.logger.Error("Failed to update server", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update server",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Server updated successfully",
	})
}

// DeleteServer deletes a server
func (h *ServerHandler) DeleteServer(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Backup bool `json:"backup"`
	}
	c.ShouldBindJSON(&req)

	ctx := c.Request.Context()
	if err := h.scheduler.DeleteServer(ctx, id, req.Backup); err != nil {
		h.logger.Error("Failed to delete server", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete server",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ServerAction performs an action on a server
func (h *ServerHandler) ServerAction(c *gin.Context) {
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

	ctx := c.Request.Context()

	switch req.Action {
	case "start":
		if err := h.scheduler.StartServer(ctx, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to start server",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Server starting...",
		})

	case "stop":
		if err := h.scheduler.StopServer(ctx, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to stop server",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Server stopping...",
		})

	case "restart":
		if err := h.scheduler.RestartServer(ctx, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to restart server",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Server restarting...",
		})

	case "reinstall":
		if err := h.scheduler.ReinstallServer(ctx, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to reinstall server",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Server reinstalling...",
		})

	case "backup":
		if err := h.scheduler.BackupServer(ctx, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to backup server",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Server backup started...",
		})

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid action",
			"message": "Unknown action: " + req.Action,
		})
	}
}

// GetServerStatus returns the current status of a server
func (h *ServerHandler) GetServerStatus(c *gin.Context) {
	id := c.Param("id")

	server, err := h.scheduler.GetServer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Server not found",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_id":  id,
		"status":    server.Status,
		"player_count": server.PlayerCount,
		"uptime":    server.UptimeSeconds,
	})
}

// GetServerLogs returns the logs of a server
func (h *ServerHandler) GetServerLogs(c *gin.Context) {
	id := c.Param("id")

	tail := 100
	if tailStr := c.Query("tail"); tailStr != "" {
		// Parse tail parameter
	}

	logs, err := h.scheduler.GetServerLogs(id, tail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get logs",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_id": id,
		"logs":      logs,
		"tail":      tail,
	})
}

// GetServerMetrics returns the metrics of a server
func (h *ServerHandler) GetServerMetrics(c *gin.Context) {
	id := c.Param("id")

	metrics, err := h.scheduler.GetServerMetrics(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Server not found",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_id": id,
		"metrics":   metrics,
	})
}
