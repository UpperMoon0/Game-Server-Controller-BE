package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the controller service
type Config struct {
	// Server Configuration
	RESTHost    string `mapstructure:"REST_HOST"`
	RESTPort    int    `mapstructure:"REST_PORT"`
	GRPCHost    string `mapstructure:"GRPC_HOST"`
	GRPCPort    int    `mapstructure:"GRPC_PORT"`
	Environment string `mapstructure:"ENVIRONMENT"`
	
	// Database Configuration
	DatabaseType     string `mapstructure:"DATABASE_TYPE"`
	DatabaseHost     string `mapstructure:"DATABASE_HOST"`
	DatabasePort     int    `mapstructure:"DATABASE_PORT"`
	DatabaseName     string `mapstructure:"DATABASE_NAME"`
	DatabaseUser     string `mapstructure:"DATABASE_USER"`
	DatabasePassword string `mapstructure:"DATABASE_PASSWORD"`
	DatabaseSSLMode  string `mapstructure:"DATABASE_SSL_MODE"`
	
	// Redis Configuration
	RedisHost     string `mapstructure:"REDIS_HOST"`
	RedisPort     int    `mapstructure:"REDIS_PORT"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD"`
	RedisDB       int    `mapstructure:"REDIS_DB"`
	
	// Node Configuration
	DefaultHeartbeatInterval int `mapstructure:"DEFAULT_HEARTBEAT_INTERVAL"`
	NodeTimeout              int `mapstructure:"NODE_TIMEOUT"`
	
	// Metrics Configuration
	MetricsEnabled       bool   `mapstructure:"METRICS_ENABLED"`
	MetricsInterval      int    `mapstructure:"METRICS_INTERVAL"`
	MetricsRetentionDays  int    `mapstructure:"METRICS_RETENTION_DAYS"`
	
	// Logging Configuration
	LogLevel    string `mapstructure:"LOG_LEVEL"`
	LogFormat   string `mapstructure:"LOG_FORMAT"`
	LogFilePath string `mapstructure:"LOG_FILE_PATH"`
	
	// Clustering
	ClusterEnabled    bool   `mapstructure:"CLUSTER_ENABLED"`
	ClusterNodeID    string `mapstructure:"CLUSTER_NODE_ID"`
	ClusterAddress   string `mapstructure:"CLUSTER_ADDRESS"`
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
	v.SetDefault("DATABASE_TYPE", "sqlite")
	v.SetDefault("DATABASE_HOST", "localhost")
	v.SetDefault("DATABASE_PORT", 5432)
	v.SetDefault("DATABASE_SSL_MODE", "disable")
	v.SetDefault("REDIS_HOST", "localhost")
	v.SetDefault("REDIS_PORT", 6379)
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("DEFAULT_HEARTBEAT_INTERVAL", 30)
	v.SetDefault("NODE_TIMEOUT", 120)
	v.SetDefault("METRICS_ENABLED", true)
	v.SetDefault("METRICS_INTERVAL", 5)
	v.SetDefault("METRICS_RETENTION_DAYS", 30)
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "json")
	v.SetDefault("CLUSTER_ENABLED", false)

	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
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

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// GetRESTAddress returns the REST server address
func (c *Config) GetRESTAddress() string {
	return fmt.Sprintf("%s:%d", c.RESTHost, c.RESTPort)
}

// GetGRPCAddress returns the gRPC server address
func (c *Config) GetGRPCAddress() string {
	return fmt.Sprintf("%s:%d", c.GRPCHost, c.GRPCPort)
}

// GetDatabaseDSN returns the database connection string
func (c *Config) GetDatabaseDSN() string {
	if c.DatabaseType == "sqlite" {
		return c.DatabaseHost // This is actually the file path for sqlite
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DatabaseHost, c.DatabasePort, c.DatabaseUser, c.DatabasePassword, c.DatabaseName, c.DatabaseSSLMode)
}

// GetRedisAddress returns the Redis address
func (c *Config) GetRedisAddress() string {
	return fmt.Sprintf("%s:%d", c.RedisHost, c.RedisPort)
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
