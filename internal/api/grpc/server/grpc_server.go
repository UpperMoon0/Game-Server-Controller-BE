package server

import (
	"context"
	"fmt"
	"net"
	"time"

	pb "github.com/nstut/game-server-proto/gen"
	"github.com/game-server/controller/internal/node"
	"github.com/game-server/controller/pkg/config"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// GRPCServer represents the gRPC server
type GRPCServer struct {
	grpcServer *grpc.Server
	cfg        *config.Config
	nodeMgr    *node.Manager
	logger     *zap.Logger
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(
	cfg *config.Config,
	nodeMgr *node.Manager,
	logger *zap.Logger,
) (*GRPCServer, error) {
	var opts []grpc.ServerOption

	// Configure server options
	opts = append(opts,
		grpc.MaxRecvMsgSize(10*1024*1024), // 10MB
		grpc.MaxSendMsgSize(10*1024*1024), // 10MB
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     5 * time.Minute,
			MaxConnectionAge:     30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  1 * time.Minute,
			Timeout:               20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second, // Allow pings as frequent as 5s
			PermitWithoutStream: true,             // Allow pings without active streams
		}),
	)

	// Add TLS if configured (optional)
	// if cfg.TLSCert != "" && cfg.TLSKey != "" {
	// 	creds, err := credentials.NewServerTLSFromFile(cfg.TLSCert, cfg.TLSKey)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to load TLS certificates: %w", err)
	// 	}
	// 	opts = append(opts, grpc.Creds(creds))
	// }

	return &GRPCServer{
		grpcServer: grpc.NewServer(opts...),
		cfg:        cfg,
		nodeMgr:    nodeMgr,
		logger:     logger,
	}, nil
}

// Start starts the gRPC server
func (s *GRPCServer) Start() error {
	// Register services
	pb.RegisterNodeServiceServer(s.grpcServer, NewNodeServiceServer(s.nodeMgr, s.logger))

	// Enable reflection for development
	if s.cfg.Environment != "production" {
		reflection.Register(s.grpcServer)
	}

	// Create listener
	lis, err := net.Listen("tcp", s.cfg.GetGRPCAddress())
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.logger.Info("Starting gRPC server", zap.String("address", s.cfg.GetGRPCAddress()))

	// Start serving
	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the gRPC server
func (s *GRPCServer) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down gRPC server...")

	// Graceful shutdown with timeout
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-ctx.Done():
		s.grpcServer.Stop()
		return ctx.Err()
	case <-done:
		s.logger.Info("gRPC server stopped")
		return nil
	}
}

// GetGRPCServer returns the underlying gRPC server (for testing)
func (s *GRPCServer) GetGRPCServer() *grpc.Server {
	return s.grpcServer
}
