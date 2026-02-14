package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/game-server/controller/internal/api/grpc/server"
	"github.com/game-server/controller/internal/api/rest"
	"github.com/game-server/controller/internal/core/repository"
	"github.com/game-server/controller/internal/docker"
	"github.com/game-server/controller/internal/node"
	"github.com/game-server/controller/internal/scheduler"
	"github.com/game-server/controller/pkg/config"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	configPath := "config.yaml"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	var log *zap.Logger
	if cfg.LogFormat == "json" {
		log, err = zap.NewProduction()
	} else {
		log, err = zap.NewDevelopment()
	}
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Game Server Controller",
		zap.String("environment", cfg.Environment),
		zap.String("rest_address", cfg.GetRESTAddress()),
		zap.String("grpc_address", cfg.GetGRPCAddress()))

	// Initialize database
	db, err := repository.NewDatabase(cfg)
	if err != nil {
		log.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// Initialize Docker volume manager
	volumeMgr, err := docker.NewVolumeManager(log)
	if err != nil {
		log.Warn("Failed to initialize Docker volume manager, volume cleanup will be disabled", zap.Error(err))
		// Continue without volume manager - deletion will still work, just won't clean up volumes
	}

	// Initialize Docker container manager
	var containerMgr *docker.ContainerManager
	if volumeMgr != nil {
		containerMgr, err = docker.NewContainerManager(volumeMgr, log)
		if err != nil {
			log.Warn("Failed to initialize Docker container manager, dynamic node creation will be disabled", zap.Error(err))
			// Continue without container manager - nodes can still register manually
		}
	}

	if volumeMgr != nil {
		defer volumeMgr.Close()
	}
	if containerMgr != nil {
		defer containerMgr.Close()
	}

	// Initialize repositories
	nodeRepo := repository.NewNodeRepository(db, log)
	serverRepo := repository.NewServerRepository(db, log)

	// Initialize node manager
	nodeMgr := node.NewManager(nodeRepo, serverRepo, volumeMgr, containerMgr, cfg, log)

	// Initialize scheduler
	sched := scheduler.NewScheduler(nodeRepo, serverRepo, nodeMgr, log)

	// Initialize gRPC server
	grpcServer, err := server.NewGRPCServer(cfg, nodeMgr, sched, log)
	if err != nil {
		log.Fatal("Failed to create gRPC server", zap.Error(err))
	}

	// Initialize REST API server
	restServer := rest.NewServer(cfg, nodeMgr, serverRepo, sched, containerMgr, log)

	// Start gRPC server
	go func() {
		if err := grpcServer.Start(); err != nil {
			log.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	// Start REST server
	go func() {
		if err := restServer.Start(); err != nil {
			log.Fatal("REST server failed", zap.Error(err))
		}
	}()

	log.Info("Server is ready",
		zap.String("rest", cfg.GetRESTAddress()),
		zap.String("grpc", cfg.GetGRPCAddress()))

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down servers...")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown servers
	if err := restServer.Shutdown(ctx); err != nil {
		log.Error("REST server shutdown error", zap.Error(err))
	}

	if err := grpcServer.Shutdown(ctx); err != nil {
		log.Error("gRPC server shutdown error", zap.Error(err))
	}

	log.Info("Servers stopped")
}

// StartGameServerController is the main entry point
func StartGameServerController() error {
	main()
	return nil
}
