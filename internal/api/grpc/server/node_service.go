package server

import (
	"context"
	"time"

	"github.com/game-server/controller/internal/api/grpc/proto"
	"github.com/game-server/controller/internal/core/models"
	"github.com/game-server/controller/internal/node"
	"go.uber.org/zap"
)

// nodeServiceServer implements the NodeServiceServer interface
type nodeServiceServer struct {
	proto.UnimplementedNodeServiceServer
	manager *node.Manager
	logger  *zap.Logger
}

// NewNodeServiceServer creates a new NodeServiceServer
func NewNodeServiceServer(manager *node.Manager, logger *zap.Logger) proto.NodeServiceServer {
	return &nodeServiceServer{
		manager: manager,
		logger:  logger,
	}
}

// RegisterNode handles node registration
func (s *nodeServiceServer) RegisterNode(ctx context.Context, req *proto.RegisterNodeRequest) (*proto.RegisterNodeResponse, error) {
	s.logger.Info("Registering node",
		zap.String("nodeId", req.NodeId),
		zap.String("hostname", req.Hostname),
		zap.String("ipAddress", req.IpAddress),
	)

	// Convert proto request to models.Node
	nodeModel := &models.Node{
		ID:          req.NodeId,
		Name:        req.Hostname,
		IPAddress:   req.IpAddress,
		GameType:    req.GameType,
		Initialized: req.Initialized,
		Status:      models.NodeStatusRunning,
	}

	if req.Resources != nil {
		nodeModel.TotalCPUCores = req.Resources.TotalCpuCores
		nodeModel.TotalMemoryMB = req.Resources.TotalMemoryMb
		nodeModel.TotalStorageMB = req.Resources.TotalStorageMb
		nodeModel.AvailableCPUCores = req.Resources.AvailableCpuCores
		nodeModel.AvailableMemoryMB = req.Resources.AvailableMemoryMb
		nodeModel.AvailableStorageMB = req.Resources.AvailableStorageMb
	}

	// Register the node with the manager
	err := s.manager.RegisterNode(ctx, nodeModel)
	if err != nil {
		s.logger.Error("Failed to register node",
			zap.String("nodeId", req.NodeId),
			zap.Error(err),
		)
		return nil, err
	}

	// Return success response
	response := &proto.RegisterNodeResponse{
		ControllerId:             "controller-1",
		HeartbeatIntervalSeconds: 30,
	}

	s.logger.Info("Node registered successfully", zap.String("nodeId", req.NodeId))
	return response, nil
}

// StreamEvents handles bidirectional streaming for events and commands
func (s *nodeServiceServer) StreamEvents(stream proto.NodeService_StreamEventsServer) error {
	var nodeID string

	// Receive events from the node
	for {
		event, err := stream.Recv()
		if err != nil {
			s.logger.Debug("Stream ended", zap.Error(err))
			return err
		}

		nodeID = event.NodeId

		// Process the event
		s.logger.Debug("Received event from node",
			zap.String("nodeId", event.NodeId),
			zap.String("type", event.Type.String()),
		)

		// Handle command results
		if event.Type == proto.EventType_EVENT_TYPE_COMMAND_RESULT && event.CommandResult != nil {
			s.logger.Info("Received command result",
				zap.String("nodeId", nodeID),
				zap.String("commandId", event.CommandResult.CommandId),
				zap.Bool("success", event.CommandResult.Success),
				zap.String("message", event.CommandResult.Message))

			// Handle the command result through the manager
			s.manager.HandleCommandResult(nodeID, event.CommandResult.CommandId, event.CommandResult.Success, event.CommandResult.Message)
			continue
		}

		// Convert proto event to internal event
		internalEvent := &node.StreamEvent{
			NodeID:    event.NodeId,
			Type:      convertEventType(event.Type),
			Timestamp: time.Unix(event.Timestamp, 0),
		}

		// Handle the event through the manager
		s.manager.HandleNodeEvent(internalEvent)

		// Check for pending commands for this node
		cmd, err := s.manager.GetPendingCommand(nodeID)
		if err != nil || cmd == nil {
			continue
		}

		// Convert command to proto
		protoCmd := convertCommandToProto(cmd)
		if protoCmd != nil {
			s.logger.Info("Sending command to node",
				zap.String("nodeId", nodeID),
				zap.String("commandId", cmd.ID),
				zap.String("type", cmd.Type.String()))
			
			if err := stream.Send(protoCmd); err != nil {
				s.logger.Error("Failed to send command to node",
					zap.String("nodeId", nodeID),
					zap.Error(err),
				)
				return err
			}
		}
	}
}

// UpdateNodeStatus handles node status updates
func (s *nodeServiceServer) UpdateNodeStatus(ctx context.Context, req *proto.UpdateNodeStatusRequest) (*proto.UpdateNodeStatusResponse, error) {
	s.logger.Debug("Updating node status",
		zap.String("nodeId", req.NodeId),
	)

	if req.Status != nil {
		status := models.NodeStatusRunning
		if req.Status.Health != proto.NodeHealth_NODE_HEALTH_HEALTHY {
			status = models.NodeStatusError
		}
		err := s.manager.UpdateNodeStatus(req.NodeId, status)
		if err != nil {
			s.logger.Error("Failed to update node status",
				zap.String("nodeId", req.NodeId),
				zap.Error(err),
			)
			return nil, err
		}
	}

	return &proto.UpdateNodeStatusResponse{
		Success: true,
		Message: "Status updated successfully",
	}, nil
}

// GetNodeConfig handles getting node configuration
func (s *nodeServiceServer) GetNodeConfig(ctx context.Context, req *proto.GetNodeConfigRequest) (*proto.GetNodeConfigResponse, error) {
	s.logger.Debug("Getting node config",
		zap.String("nodeId", req.NodeId),
	)

	// Return default config
	config := &proto.ControllerConfig{
		ControllerAddress:      "localhost",
		GrpcPort:               50051,
		RestPort:               8080,
		MetricsIntervalSeconds: 60,
		AutoUpdateEnabled:      false,
	}

	return &proto.GetNodeConfigResponse{
		Config: config,
	}, nil
}

// Helper functions

func convertEventType(t proto.EventType) models.EventType {
	switch t {
	case proto.EventType_EVENT_TYPE_NODE_ONLINE:
		return models.EventTypeNodeOnline
	case proto.EventType_EVENT_TYPE_NODE_OFFLINE:
		return models.EventTypeNodeOffline
	case proto.EventType_EVENT_TYPE_HEARTBEAT:
		return models.EventTypeHeartbeat
	case proto.EventType_EVENT_TYPE_SERVER_STARTED:
		return models.EventTypeServerStarted
	case proto.EventType_EVENT_TYPE_SERVER_STOPPED:
		return models.EventTypeServerStopped
	default:
		return models.EventTypeHeartbeat
	}
}

func convertCommandToProto(cmd *node.Command) *proto.ControllerCommand {
	if cmd == nil {
		return nil
	}

	protoCmd := &proto.ControllerCommand{
		CommandId: cmd.ID,
		Type:      convertCommandType(cmd.Type),
		Timestamp: time.Now().Unix(),
	}

	switch cmd.Type {
	case node.CommandTypeInitialize:
		if gameType, ok := cmd.Payload.(string); ok {
			protoCmd.InitializeNode = &proto.InitializeNodeCmd{
				GameType: gameType,
			}
		}
	case node.CommandTypeStart:
		if serverID, ok := cmd.Payload.(string); ok {
			protoCmd.StartServer = &proto.StartServerCmd{
				ServerId: serverID,
			}
		}
	case node.CommandTypeStop:
		if serverID, ok := cmd.Payload.(string); ok {
			protoCmd.StopServer = &proto.StopServerCmd{
				ServerId: serverID,
			}
		}
	}

	return protoCmd
}

func convertCommandType(t node.CommandType) proto.CommandType {
	switch t {
	case node.CommandTypeInitialize:
		return proto.CommandType_COMMAND_TYPE_INITIALIZE_NODE
	case node.CommandTypeStart:
		return proto.CommandType_COMMAND_TYPE_START_SERVER
	case node.CommandTypeStop:
		return proto.CommandType_COMMAND_TYPE_STOP_SERVER
	case node.CommandTypeRestart:
		return proto.CommandType_COMMAND_TYPE_RESTART_SERVER
	default:
		return proto.CommandType_COMMAND_TYPE_UNSPECIFIED
	}
}
