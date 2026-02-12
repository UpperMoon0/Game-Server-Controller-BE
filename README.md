# Game Server Controller

A high-performance game server management system built with Go, featuring REST API and gRPC interfaces for managing game server nodes and instances.

## Features

- **Multi-Node Management**: Register and manage multiple game server nodes
- **Server Lifecycle**: Create, update, delete, start, stop, and restart game servers
- **Resource Scheduling**: Optimal node selection based on resource requirements
- **Real-time Metrics**: Monitor node and server metrics in real-time
- **High Performance**: gRPC for node communication, REST API for UI
- **Container Ready**: Docker support for easy deployment

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Controller Service                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐   │
│  │ REST API    │  │ gRPC Server │  │ Scheduler           │   │
│  │ (Port 8080)│  │(Port 50051) │  │ - Node Manager      │   │
│  └─────────────┘  └─────────────┘  │ - Resource Allocator│   │
│                                    └─────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
                    gRPC Streaming
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│  Node 1     │      │  Node 2     │      │  Node N     │
│ (Game Srv)  │      │ (Game Srv)  │      │ (Game Srv)  │
└─────────────┘      └─────────────┘      └─────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose (optional)
- Redis (for caching and pub/sub)
- SQLite or PostgreSQL

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/your-org/game-server-controller.git
   cd game-server-controller
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Configure the application**
   ```bash
   cp config.yaml.example config.yaml
   # Edit config.yaml with your settings
   ```

4. **Run the controller**
   ```bash
   # Development
   go run ./cmd/controller

   # Production (with Docker)
   docker-compose up -d
   ```

### Docker Deployment

```bash
# Build and start services
docker-compose up -d

# View logs
docker-compose logs -f controller

# Stop services
docker-compose down
```

## API Documentation

### REST API Endpoints

#### Nodes
- `GET /api/v1/nodes` - List all nodes
- `POST /api/v1/nodes` - Register a new node
- `GET /api/v1/nodes/:id` - Get node details
- `PUT /api/v1/nodes/:id` - Update node configuration
- `DELETE /api/v1/nodes/:id` - Unregister node
- `GET /api/v1/nodes/:id/status` - Get node status
- `GET /api/v1/nodes/:id/metrics` - Get node metrics

#### Servers
- `GET /api/v1/servers` - List all servers
- `POST /api/v1/servers` - Create a new server
- `GET /api/v1/servers/:id` - Get server details
- `PUT /api/v1/servers/:id` - Update server configuration
- `DELETE /api/v1/servers/:id` - Delete server
- `POST /api/v1/servers/:id/action` - Perform server action (start/stop/restart)
- `GET /api/v1/servers/:id/logs` - Get server logs
- `GET /api/v1/servers/:id/metrics` - Get server metrics

#### Example: Create a Server

```bash
curl -X POST http://localhost:8080/api/v1/servers \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "node-1",
    "game_type": "minecraft",
    "config": {
      "name": "My Server",
      "version": "1.20.1",
      "max_players": 20,
      "settings": {
        "difficulty": "normal",
        "gamemode": "survival"
      }
    },
    "requirements": {
      "min_cpu_cores": 2,
      "min_memory_mb": 4096,
      "min_storage_mb": 10240
    }
  }'
```

### gRPC API

The gRPC API is primarily used for node-to-controller communication. See the [Proto Definitions](proto/controller.proto) for details.

## Configuration

All configuration is managed through `config.yaml`:

```yaml
# Server Configuration
environment: "development"
rest_host: "0.0.0.0"
rest_port: 8080
grpc_host: "0.0.0.0"
grpc_port: 50051

# Database Configuration
database_type: "sqlite"
database_host: "./data/controller.db"

# Redis Configuration
redis_host: "localhost"
redis_port: 6379

# Node Configuration
default_heartbeat_interval: 30
node_timeout: 120

# Logging Configuration
log_level: "info"
log_format: "json"
log_file_path: "./logs/controller.log"
```

## Project Structure

```
game-server-controller/
├── cmd/
│   └── controller/           # Main entrypoint
├── internal/
│   ├── api/
│   │   ├── rest/             # REST API handlers
│   │   └── grpc/             # gRPC server
│   ├── core/
│   │   ├── models/           # Data models
│   │   └── repository/       # Database operations
│   ├── node/                 # Node management
│   └── scheduler/            # Resource scheduling
├── pkg/
│   ├── config/               # Configuration management
│   └── logger/               # Logging utilities
├── proto/                    # Protocol Buffer definitions
├── migrations/               # Database migrations
├── config.yaml               # Configuration file
├── Dockerfile
└── docker-compose.yml
```

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

### Building

```bash
# Build binary
go build -o controller ./cmd/controller

# Build Docker image
docker build -t game-server-controller:latest .
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

For issues and feature requests, please use the [GitHub Issues](https://github.com/your-org/game-server-controller/issues) page.
