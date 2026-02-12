package models

import (
	"database/sql"
	"time"
)

// ServerStatus represents the current status of a game server
type ServerStatus string

const (
	ServerStatusInstalling ServerStatus = "installing"
	ServerStatusStopped    ServerStatus = "stopped"
	ServerStatusRunning    ServerStatus = "running"
	ServerStatusError      ServerStatus = "error"
	ServerStatusUpdating   ServerStatus = "updating"
	ServerStatusStarting   ServerStatus = "starting"
	ServerStatusStopping   ServerStatus = "stopping"
	ServerStatusBackingUp  ServerStatus = "backing_up"
)

// Server represents a game server instance
type Server struct {
	ID            string         `json:"id" db:"id"`
	Name          string         `json:"name" db:"name"`
	NodeID        string         `json:"node_id" db:"node_id"`
	GameType      string         `json:"game_type" db:"game_type"`
	InstanceID    string         `json:"instance_id" db:"instance_id"`
	Status        ServerStatus   `json:"status" db:"status"`
	
	// Configuration
	Version       string         `json:"version" db:"version"`
	Settings      map[string]string `json:"settings" db:"-"`
	EnvVars       map[string]string `json:"env_vars" db:"-"`
	MaxPlayers    int            `json:"max_players" db:"max_players"`
	WorldName     string         `json:"world_name" db:"world_name"`
	OnlineMode    bool           `json:"online_mode" db:"online_mode"`
	
	// Network
	Port          int            `json:"port" db:"port"`
	QueryPort     int            `json:"query_port" db:"query_port"`
	RCONPort      int            `json:"rcon_port" db:"rcon_port"`
	IPAddress     string         `json:"ip_address" db:"ip_address"`
	
	// Metrics
	PlayerCount   int            `json:"player_count" db:"player_count"`
	CPUUsage      float64        `json:"cpu_usage" db:"cpu_usage"`
	MemoryUsage   int64          `json:"memory_usage" db:"memory_usage"`
	UptimeSeconds int64          `json:"uptime_seconds" db:"uptime_seconds"`
	
	// Timestamps
	CreatedAt     time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at" db:"updated_at"`
	StartedAt     sql.NullTime   `json:"started_at" db:"started_at"`
}

// ServerConfig represents the configuration for a game server
type ServerConfig struct {
	Name          string            `json:"name" binding:"required"`
	Version       string            `json:"version" binding:"required"`
	Settings      map[string]string  `json:"settings"`
	EnvVars       map[string]string  `json:"env_vars"`
	MaxPlayers    int               `json:"max_players" binding:"min=0"`
	WorldName     string            `json:"world_name"`
	OnlineMode    bool              `json:"online_mode"`
	QueryType     string            `json:"query_type"`
	AutoStart     bool              `json:"auto_start"`
	AutoRestart   bool              `json:"auto_restart"`
	RestartDelay  int               `json:"restart_delay_seconds" binding:"min=0"`
	BackupEnabled bool              `json:"backup_enabled"`
	BackupSchedule string           `json:"backup_schedule"`
}

// ResourceRequirements represents the resource requirements for a server
type ResourceRequirements struct {
	MinCPUCores       int   `json:"min_cpu_cores" binding:"min=1"`
	MinMemoryMB       int64 `json:"min_memory_mb" binding:"min=256"`
	MinStorageMB      int64 `json:"min_storage_mb" binding:"min=1024"`
	MaxCPUCores       int   `json:"max_cpu_cores"`
	MaxMemoryMB       int64 `json:"max_memory_mb"`
	MaxPlayers        int   `json:"max_players" binding:"min=0"`
	NetworkBandwidthMbps int `json:"network_bandwidth_mbps"`
}

// ServerMetrics represents real-time metrics for a server
type ServerMetrics struct {
	ServerID       string    `json:"server_id"`
	PlayerCount    int       `json:"player_count"`
	OnlinePlayers  []string  `json:"online_players"`
	CPUUsage       float64   `json:"cpu_usage_percent"`
	MemoryUsage    int64     `json:"memory_usage_mb"`
	TPS            float64   `json:"ticks_per_second"`
	MSPT           float64   `json:"ms_per_tick"`
	NetworkIn      int64     `json:"network_bytes_in"`
	NetworkOut     int64     `json:"network_bytes_out"`
	UptimeSeconds  int64     `json:"uptime_seconds"`
	AveragePing    float64   `json:"average_ping_ms"`
	Timestamp      time.Time `json:"timestamp"`
}

// CreateServerRequest represents a request to create a new server
type CreateServerRequest struct {
	NodeID      string              `json:"node_id" binding:"required"`
	GameType    string              `json:"game_type" binding:"required"`
	Config      ServerConfig        `json:"config" binding:"required"`
	Requirements ResourceRequirements `json:"requirements"`
}

// UpdateServerRequest represents a request to update server configuration
type UpdateServerRequest struct {
	Config   *ServerConfig `json:"config"`
	Restart  bool          `json:"restart"`
}

// ServerAction represents an action to perform on a server
type ServerAction string

const (
	ServerActionStart    ServerAction = "start"
	ServerActionStop     ServerAction = "stop"
	ServerActionRestart  ServerAction = "restart"
	ServerActionReinstall ServerAction = "reinstall"
	ServerActionBackup   ServerAction = "backup"
)

// ServerLog represents a log entry from a server
type ServerLog struct {
	ID         string    `json:"id"`
	ServerID   string    `json:"server_id"`
	Level      string    `json:"level"`
	Message    string    `json:"message"`
	Source     string    `json:"source"`
	Timestamp  time.Time `json:"timestamp"`
	LineNumber int       `json:"line_number"`
}

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
	LogLevelFatal   LogLevel = "fatal"
)

// ServerFilters represents filters for listing servers
type ServerFilters struct {
	NodeID    string        `query:"node_id"`
	Status    ServerStatus  `query:"status"`
	GameType  string        `query:"game_type"`
	HasPlayer *bool         `query:"has_player"`
	Limit     int           `query:"limit" binding:"min=1,max=100"`
	Offset    int           `query:"offset" binding:"min=0"`
}

// CreateServerResponse represents the response after creating a server
type CreateServerResponse struct {
	Success     bool       `json:"success"`
	ServerID     string     `json:"server_id,omitempty"`
	Message      string     `json:"message,omitempty"`
	ServerInfo   *ServerInfo `json:"server_info,omitempty"`
}

// ServerInfo represents basic information about a created server
type ServerInfo struct {
	ServerID    string `json:"server_id"`
	NodeID      string `json:"node_id"`
	Port        int    `json:"port"`
	QueryPort   int    `json:"query_port"`
	RCONPort    int    `json:"rcon_port"`
	IPAddress   string `json:"ip_address"`
}
