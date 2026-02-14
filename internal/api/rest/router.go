package rest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/game-server/controller/internal/api/rest/handlers"
	"github.com/game-server/controller/internal/core/repository"
	"github.com/game-server/controller/internal/docker"
	"github.com/game-server/controller/internal/node"
	"github.com/game-server/controller/internal/scheduler"
	"github.com/game-server/controller/pkg/config"
	"go.uber.org/zap"
)

// Server represents the REST API server
type Server struct {
	router       *gin.Engine
	httpServer   *http.Server
	cfg          *config.Config
	nodeRepo     *node.Manager
	serverRepo   *repository.ServerRepository
	scheduler    *scheduler.Scheduler
	containerMgr *docker.ContainerManager
	logger       *zap.Logger
}

// NewServer creates a new REST API server
func NewServer(
	cfg *config.Config,
	nodeRepo *node.Manager,
	serverRepo *repository.ServerRepository,
	scheduler *scheduler.Scheduler,
	containerMgr *docker.ContainerManager,
	logger *zap.Logger,
) *Server {
	// Set Gin mode based on environment
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(LoggerMiddleware(logger))
	router.Use(CORSMiddleware())

	return &Server{
		router:       router,
		cfg:          cfg,
		nodeRepo:     nodeRepo,
		serverRepo:   serverRepo,
		scheduler:    scheduler,
		containerMgr: containerMgr,
		logger:       logger,
	}
}

// Start starts the REST API server
func (s *Server) Start() error {
	// Register routes
	s.registerRoutes()

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         s.cfg.GetRESTAddress(),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		s.logger.Info("Starting REST API server", zap.String("address", s.cfg.GetRESTAddress()))
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Fatal("REST API server failed", zap.Error(err))
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the REST API server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down REST API server...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown REST server: %w", err)
	}

	s.logger.Info("REST API server stopped")
	return nil
}

// registerRoutes registers all API routes
func (s *Server) registerRoutes() {
	// Health check endpoints
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/ready", s.readinessCheck)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// Register node handler
		nodeHandler := handlers.NewNodeHandler(s.nodeRepo, s.scheduler, s.containerMgr, s.cfg, s.logger)
		nodeHandler.RegisterRoutes(v1)

		// Register server handler
		serverHandler := handlers.NewServerHandler(s.nodeRepo, s.scheduler, s.logger)
		serverHandler.RegisterRoutes(v1)

		// Metrics endpoint
		v1.GET("/metrics", s.getClusterMetrics)

		// Game types endpoint
		v1.GET("/game-types", s.getGameTypes)
	}
}

// Health check endpoints

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) readinessCheck(c *gin.Context) {
	// Check if database is accessible
	_, err := s.serverRepo.CountByStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "not ready",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ready",
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) getClusterMetrics(c *gin.Context) {
	// Get cluster metrics from node manager
	clusterMetrics, err := s.nodeRepo.GetClusterMetrics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get cluster metrics",
			"message": err.Error(),
		})
		return
	}

	// Get server counts
	serverCounts, err := s.scheduler.GetServerCounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get server counts",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes":   clusterMetrics,
		"servers": serverCounts,
		"timestamp": time.Now().UTC(),
	})
}

// getGameTypes returns the list of supported game types
func (s *Server) getGameTypes(c *gin.Context) {
	// Only Minecraft is supported for now
	gameTypes := []gin.H{
		{
			"id":          "minecraft",
			"name":        "Minecraft",
			"description": "Minecraft Java Edition server",
			"default_port": 25565,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"game_types": gameTypes,
	})
}

// LoggerMiddleware returns a gin middleware for logging
func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		logger.Info("HTTP Request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}

// CORSMiddleware returns a gin middleware for CORS
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RunServer starts the REST API server (standalone function for testing)
func RunServer(cfg *config.Config, logger *zap.Logger) error {
	server := NewServer(nil, nil, nil, nil, nil, logger)
	
	if err := server.Start(); err != nil {
		return err
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(ctx)
}
