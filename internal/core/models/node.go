package models

import (
	"database/sql"
	"time"
)

// NodeStatus represents the current status of a node
type NodeStatus string

const (
	NodeStatusOnline      NodeStatus = "online"
	NodeStatusOffline     NodeStatus = "offline"
	NodeStatusMaintenance NodeStatus = "maintenance"
	NodeStatusUnknown     NodeStatus = "unknown"
	NodeStatusUnhealthy   NodeStatus = "unhealthy"
)

// Node represents a game server node in the system
type Node struct {
	ID               string         `json:"id" db:"id"`
	Name             string         `json:"name" db:"name"`
	Hostname         string         `json:"hostname" db:"hostname"`
	IPAddress        string         `json:"ip_address" db:"ip_address"`
	Port             int            `json:"port" db:"port"`
	Status           NodeStatus     `json:"status" db:"status"`
	GameTypes        []string       `json:"game_types" db:"game_types"`
	TotalCPUCores    int            `json:"total_cpu_cores" db:"total_cpu_cores"`
	TotalMemoryMB    int64          `json:"total_memory_mb" db:"total_memory_mb"`
	TotalStorageMB   int64          `json:"total_storage_mb" db:"total_storage_mb"`
	AvailableCPUCores int           `json:"available_cpu_cores" db:"available_cpu_cores"`
	AvailableMemoryMB int64         `json:"available_memory_mb" db:"available_memory_mb"`
	AvailableStorageMB int64        `json:"available_storage_mb" db:"available_storage_mb"`
	OSVersion        string         `json:"os_version" db:"os_version"`
	AgentVersion     string         `json:"agent_version" db:"agent_version"`
	HeartbeatInterval int           `json:"heartbeat_interval" db:"heartbeat_interval"`
	LastHeartbeat     time.Time     `json:"last_heartbeat" db:"last_heartbeat"`
	CreatedAt        time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at" db:"updated_at"`
}

// NodeMetrics represents real-time metrics for a node
type NodeMetrics struct {
	NodeID           string    `json:"node_id"`
	CPUUsagePercent  float64   `json:"cpu_usage_percent"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	StorageUsagePercent float64 `json:"storage_usage_percent"`
	NetworkInBytes   int64     `json:"network_in_bytes"`
	NetworkOutBytes  int64     `json:"network_out_bytes"`
	ActiveConnections int32    `json:"active_connections"`
	LoadAverage      float64   `json:"load_average"`
	Timestamp        time.Time `json:"timestamp"`
}

// NodeHealth represents the health status of a node
type NodeHealth string

const (
	NodeHealthHealthy   NodeHealth = "healthy"
	NodeHealthDegraded   NodeHealth = "degraded"
	NodeHealthUnhealthy  NodeHealth = "unhealthy"
	NodeHealthCritical   NodeHealth = "critical"
)

// CreateNodeRequest represents a request to create a new node
type CreateNodeRequest struct {
	Name              string   `json:"name" binding:"required"`
	Hostname          string   `json:"hostname" binding:"required"`
	IPAddress         string   `json:"ip_address" binding:"required"`
	Port              int      `json:"port" binding:"required,min=1,max=65535"`
	GameType          string   `json:"game_type" binding:"required"`
	TotalCPUCores     int      `json:"total_cpu_cores" binding:"required,min=1"`
	TotalMemoryMB     int64    `json:"total_memory_mb" binding:"required,min=1024"`
	TotalStorageMB    int64    `json:"total_storage_mb" binding:"required,min=1024"`
	OSVersion         string   `json:"os_version"`
}

// UpdateNodeRequest represents a request to update node configuration
type UpdateNodeRequest struct {
	Name              *string    `json:"name"`
	GameTypes         []string   `json:"game_types"`
	HeartbeatInterval *int       `json:"heartbeat_interval"`
	Status            *NodeStatus `json:"status"`
}

// NodeEvent represents an event from a node
type NodeEvent struct {
	ID        string          `json:"id"`
	NodeID    string          `json:"node_id"`
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      sql.NullString  `json:"data"`
}

// EventType represents the type of node event
type EventType string

const (
	EventTypeNodeOnline         EventType = "node_online"
	EventTypeNodeOffline        EventType = "node_offline"
	EventTypeNodeStatusUpdate   EventType = "node_status_update"
	EventTypeServerCreated      EventType = "server_created"
	EventTypeServerStarted      EventType = "server_started"
	EventTypeServerStopped      EventType = "server_stopped"
	EventTypeServerError        EventType = "server_error"
	EventTypeMetricsUpdate      EventType = "metrics_update"
	EventTypeLog                EventType = "log"
	EventTypeHeartbeat          EventType = "heartbeat"
)
