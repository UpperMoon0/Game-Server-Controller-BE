package config

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the controller service
type Config struct {
	mu sync.RWMutex // For thread-safe access to mutable fields

	// Config file path for saving changes
	configPath string

	// Server Configuration
	RESTHost    string `mapstructure:"REST_HOST" yaml:"rest_host"`
	RESTPort    int    `mapstructure:"REST_PORT" yaml:"rest_port"`
	GRPCHost    string `mapstructure:"GRPC_HOST" yaml:"grpc_host"`
	GRPCPort    int    `mapstructure:"GRPC_PORT" yaml:"grpc_port"`
	Environment string `mapstructure:"ENVIRONMENT" yaml:"environment"`

	// Advertise Address (for Docker containers to connect)
	// If empty, uses GRPCHost. For Docker, typically use "host.docker.internal" or host IP
	// This field can be updated at runtime
	GRPCAdvertiseHost string `mapstructure:"GRPC_ADVERTISE_HOST" yaml:"grpc_advertise_host"`

	// Database Configuration (PostgreSQL only)
	DBUrl           string `mapstructure:"DB_URL" yaml:"db_url"`
	DatabaseName    string `mapstructure:"DATABASE_NAME" yaml:"database_name"`
	DatabaseUser    string `mapstructure:"DATABASE_USER" yaml:"database_user"`
	DatabasePassword string `mapstructure:"DATABASE_PASSWORD" yaml:"database_password"`
	DatabaseSSLMode string `mapstructure:"DATABASE_SSL_MODE" yaml:"database_ssl_mode"`

	// Node Agent Configuration
	NodeAgentImage  string `mapstructure:"NODE_AGENT_IMAGE" yaml:"node_agent_image"`
	NodeNetworkName string `mapstructure:"NODE_NETWORK_NAME" yaml:"node_network_name"`

	// Node Configuration
	DefaultHeartbeatInterval int `mapstructure:"DEFAULT_HEARTBEAT_INTERVAL" yaml:"default_heartbeat_interval"`
	NodeTimeout              int `mapstructure:"NODE_TIMEOUT" yaml:"node_timeout"`

	// Metrics Configuration
	MetricsEnabled       bool   `mapstructure:"METRICS_ENABLED" yaml:"metrics_enabled"`
	MetricsInterval      int    `mapstructure:"METRICS_INTERVAL" yaml:"metrics_interval"`
	MetricsRetentionDays  int   `mapstructure:"METRICS_RETENTION_DAYS" yaml:"metrics_retention_days"`

	// Logging Configuration
	LogLevel    string `mapstructure:"LOG_LEVEL" yaml:"log_level"`
	LogFormat   string `mapstructure:"LOG_FORMAT" yaml:"log_format"`
	LogFilePath string `mapstructure:"LOG_FILE_PATH" yaml:"log_file_path"`

	// Clustering
	ClusterEnabled    bool   `mapstructure:"CLUSTER_ENABLED" yaml:"cluster_enabled"`
	ClusterNodeID    string `mapstructure:"CLUSTER_NODE_ID" yaml:"cluster_node_id"`
	ClusterAddress   string `mapstructure:"CLUSTER_ADDRESS" yaml:"cluster_address"`
}

// Load reads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("REST_HOST", "0.0.0.0")
	v.SetDefault("REST_PORT", 8080)
	v.SetDefault("GRPC_HOST", "0.0.0.0")
	v.SetDefault("GRPC_PORT", 50051)
	v.SetDefault("ENVIRONMENT", "development")
	v.SetDefault("DB_URL", "localhost:5432")
	v.SetDefault("DATABASE_NAME", "game_server")
	v.SetDefault("DATABASE_SSL_MODE", "disable")
	v.SetDefault("NODE_AGENT_IMAGE", "nstut/game-server-node:latest")
	v.SetDefault("NODE_NETWORK_NAME", "nstut-network")
	v.SetDefault("DEFAULT_HEARTBEAT_INTERVAL", 30)
	v.SetDefault("NODE_TIMEOUT", 120)
	v.SetDefault("METRICS_ENABLED", true)
	v.SetDefault("METRICS_INTERVAL", 5)
	v.SetDefault("METRICS_RETENTION_DAYS", 30)
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "json")
	v.SetDefault("CLUSTER_ENABLED", false)

	// Set config file
	var resolvedConfigPath string
	if configPath != "" {
		v.SetConfigFile(configPath)
		resolvedConfigPath = configPath
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/game-server-controller")
	}

	// Environment variables
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Get the actual config file path used
	if resolvedConfigPath == "" {
		resolvedConfigPath = v.ConfigFileUsed()
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Store the config file path for saving later
	config.configPath = resolvedConfigPath

	return &config, nil
}

// GetRESTAddress returns the REST server address
func (c *Config) GetRESTAddress() string {
	return fmt.Sprintf("%s:%d", c.RESTHost, c.RESTPort)
}

// GetGRPCAddress returns the gRPC server address (for binding)
func (c *Config) GetGRPCAddress() string {
	return fmt.Sprintf("%s:%d", c.GRPCHost, c.GRPCPort)
}

// GetGRPCAdvertiseAddress returns the gRPC address that node agents should connect to
// This is different from GetGRPCAddress() because 0.0.0.0 is not reachable from containers
func (c *Config) GetGRPCAdvertiseAddress() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	host := c.GRPCAdvertiseHost
	if host == "" {
		// If not set, try to detect the appropriate address
		// For Docker, use "host.docker.internal" which resolves to the host
		host = "host.docker.internal"
	}
	return fmt.Sprintf("%s:%d", host, c.GRPCPort)
}

// GetGRPCAdvertiseHost returns the advertise host (without port)
func (c *Config) GetGRPCAdvertiseHost() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.GRPCAdvertiseHost == "" {
		return "host.docker.internal"
	}
	return c.GRPCAdvertiseHost
}

// SetGRPCAdvertiseHost sets the advertise host at runtime and persists to config file
func (c *Config) SetGRPCAdvertiseHost(host string) error {
	c.mu.Lock()
	c.GRPCAdvertiseHost = host
	c.mu.Unlock()
	
	// Save to config file
	return c.Save()
}

// Save writes the current configuration to the config file
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.configPath == "" {
		return fmt.Errorf("no config file path set")
	}

	// Read existing config file to preserve comments and structure
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		// If file doesn't exist, create new one
		if os.IsNotExist(err) {
			return c.writeNewConfig()
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse as YAML map to preserve structure
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Update the grpc_advertise_host field
	rawConfig["grpc_advertise_host"] = c.GRPCAdvertiseHost

	// Write back to file
	outData, err := yaml.Marshal(rawConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(c.configPath, outData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// writeNewConfig creates a new config file with current settings
func (c *Config) writeNewConfig() error {
	outData, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(c.configPath, outData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetDatabaseDSN returns the PostgreSQL connection string
func (c *Config) GetDatabaseDSN() string {
	host, port := c.parseHostPort()
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, c.DatabaseUser, c.DatabasePassword, c.DatabaseName, c.DatabaseSSLMode)
}

// parseHostPort parses DB_URL which can be "host:port" or just "host"
func (c *Config) parseHostPort() (host, port string) {
	parts := strings.Split(c.DBUrl, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return c.DBUrl, "5432" // default PostgreSQL port
}

// GetHeartbeatInterval returns the heartbeat interval as a duration
func (c *Config) GetHeartbeatInterval() time.Duration {
	return time.Duration(c.DefaultHeartbeatInterval) * time.Second
}

// GetNodeTimeout returns the node timeout as a duration
func (c *Config) GetNodeTimeout() time.Duration {
	return time.Duration(c.NodeTimeout) * time.Second
}

// GetMetricsInterval returns the metrics interval as a duration
func (c *Config) GetMetricsInterval() time.Duration {
	return time.Duration(c.MetricsInterval) * time.Second
}
