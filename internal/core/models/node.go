package models

import (
	"database/sql"
	"time"
)

// NodeStatus represents the current status of a node
type NodeStatus string

const (
	NodeStatusInstalling  NodeStatus = "installing"
	NodeStatusStopped     NodeStatus = "stopped"
	NodeStatusRunning     NodeStatus = "running"
	NodeStatusError       NodeStatus = "error"
	NodeStatusUpdating    NodeStatus = "updating"
	NodeStatusStarting    NodeStatus = "starting"
	NodeStatusStopping    NodeStatus = "stopping"
	NodeStatusOffline     NodeStatus = "offline"
	NodeStatusMaintenance NodeStatus = "maintenance"
)

// Node represents a game server node instance
// Note: Status is fetched from node agents via gRPC, not stored in DB
type Node struct {
	ID        string `json:"id" db:"id"`
	Name      string `json:"name" db:"name"`
	GameType  string `json:"game_type" db:"game_type"`
	Version   string `json:"version" db:"version"`
	Port      int    `json:"port" db:"port"`
	
	// Initialization state (stored in DB, managed by node agent)
	Initialized bool `json:"initialized" db:"initialized"`
	
	// Agent Connection
	AgentVersion      string `json:"agent_version" db:"agent_version"`
	HeartbeatInterval int    `json:"heartbeat_interval" db:"heartbeat_interval"`
	
	// Timestamps
	CreatedAt time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt time.Time     `json:"updated_at" db:"updated_at"`
	StartedAt sql.NullTime  `json:"started_at" db:"started_at"`
	
	// Runtime state (fetched from node agent, not stored in DB)
	Status        NodeStatus `json:"status" db:"-"`
	LastHeartbeat time.Time   `json:"last_heartbeat" db:"-"`
}

// NodeMetrics represents real-time metrics for a node (fetched from node agent, not stored in DB)
type NodeMetrics struct {
	NodeID             string    `json:"node_id"`
	PlayerCount        int       `json:"player_count"`
	CPUUsagePercent    float64   `json:"cpu_usage_percent"`
	MemoryUsagePercent float64   `json:"memory_usage_percent"`
	MemoryUsageMB      int64     `json:"memory_usage_mb"`
	UptimeSeconds      int64     `json:"uptime_seconds"`
	Timestamp          time.Time `json:"timestamp"`
}

// CreateNodeRequest represents a request to create a new node
type CreateNodeRequest struct {
	Name     string `json:"name" binding:"required"`
	GameType string `json:"game_type" binding:"required"`
	Version  string `json:"version"`
	Port     int    `json:"port" binding:"omitempty,min=1,max=65535"`
}

// UpdateNodeRequest represents a request to update node configuration
type UpdateNodeRequest struct {
	Name              *string     `json:"name"`
	GameType          *string     `json:"game_type"`
	Version           *string     `json:"version"`
	Port              *int        `json:"port"`
	Status            *NodeStatus `json:"status"`
	HeartbeatInterval *int        `json:"heartbeat_interval"`
}

// NodeAction represents an action to perform on a node
type NodeAction string

const (
	NodeActionStart     NodeAction = "start"
	NodeActionStop      NodeAction = "stop"
	NodeActionRestart   NodeAction = "restart"
	NodeActionReinstall NodeAction = "reinstall"
)

// NodeFilters represents filters for listing nodes
type NodeFilters struct {
	Status   NodeStatus `query:"status"`
	GameType string     `query:"game_type"`
	Limit    int        `query:"limit" binding:"omitempty,min=1,max=100"`
	Offset   int        `query:"offset" binding:"omitempty,min=0"`
}

// NodeEvent represents an event from a node
type NodeEvent struct {
	ID        string         `json:"id"`
	NodeID    string         `json:"node_id"`
	Type      EventType      `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Data      sql.NullString `json:"data"`
}

// EventType represents the type of node event
type EventType string

const (
	EventTypeNodeOnline       EventType = "node_online"
	EventTypeNodeOffline      EventType = "node_offline"
	EventTypeNodeStatusUpdate EventType = "node_status_update"
	EventTypeNodeStarted      EventType = "node_started"
	EventTypeNodeStopped      EventType = "node_stopped"
	EventTypeNodeError        EventType = "node_error"
	EventTypeMetricsUpdate    EventType = "metrics_update"
	EventTypeHeartbeat        EventType = "heartbeat"
)
