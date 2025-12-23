# MangaHub - Network Programming Project

A comprehensive manga management system demonstrating multiple network protocols: HTTP REST API, TCP, UDP, WebSocket, and gRPC.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Running the Servers](#running-the-servers)
- [CLI Commands](#cli-commands)
- [API Testing](#api-testing)
- [Troubleshooting](#troubleshooting)

## Overview

MangaHub consists of 5 server components and a unified CLI application:

| Component | Port | Protocol | Description |
|-----------|------|----------|-------------|
| API Server | 8080 | HTTP/REST | User authentication and manga management |
| TCP Server | 9090 | TCP | Real-time progress synchronization |
| UDP Server | 9091 | UDP | Broadcast notifications for new releases |
| gRPC Server | 9092 | gRPC | Internal manga operations via Protocol Buffers |
| WebSocket Server | 8080 | WebSocket | Real-time chat functionality |
| CLI App | - | - | Unified command-line interface |

## Prerequisites

### Required Software

1. **Go** (version 1.21 or later)
   ```bash
   go version
   ```

2. **Protocol Buffers Compiler (protoc)**
   ```bash
   # macOS
   brew install protobuf
   
   # Windows (using Chocolatey)
   choco install protoc
   
   # Verify installation
   protoc --version
   ```

3. **Go Protobuf Plugins**
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```

4. **Add Go bin to PATH**
   ```bash
   # Windows (PowerShell)
   $env:Path += ";$env:USERPROFILE\go\bin"
   
   # macOS/Linux (zsh)
   echo 'export PATH=$PATH:~/go/bin' >> ~/.zshrc
   source ~/.zshrc
   
   # macOS/Linux (bash)
   echo 'export PATH=$PATH:~/go/bin' >> ~/.bash_profile
   source ~/.bash_profile
   ```

## Installation

1. **Navigate to project directory**
   ```bash
   cd /path/to/mangahub
   ```

2. **Install Go dependencies**
   ```bash
   go mod tidy
   ```

3. **Generate Protocol Buffer code**
   ```bash
   cd proto
   protoc --go_out=. --go_opt=paths=source_relative \
          --go-grpc_out=. --go-grpc_opt=paths=source_relative \
          manga.proto
   cd ..
   ```

## Quick Start

### Build the CLI Application

```bash
# Windows
go build -o mangahub.exe ./cmd/cli

# macOS/Linux
go build -o mangahub ./cmd/cli
```

### Initialize Configuration

```bash
# Windows
.\mangahub.exe init

# macOS/Linux
./mangahub init
```

This creates:
- Configuration file at `~/.mangahub/config.yaml`
- Log directory at `~/.mangahub/logs/`
- Database file at `~/.mangahub/data.db`

## Running the Servers

### Start All Servers (Recommended)

```bash
go run cmd/server/main.go
```

This starts all 5 servers in a single process:
- HTTP API Server on port 8080
- TCP Sync Server on port 9090
- UDP Notification Server on port 9091
- gRPC Service on port 9092
- WebSocket Chat on port 8080

### Run Individual Servers

If needed, you can run servers separately in different terminals:

**Terminal 1 - API Server:**
```bash
go run cmd/api-server/main.go
```

**Terminal 2 - TCP Server:**
```bash
go run cmd/tcp-server/main.go
```

**Terminal 3 - UDP Server:**
```bash
go run cmd/udp-server/main.go
```

**Terminal 4 - gRPC Server:**
```bash
go run cmd/grpc-server/main.go
```

## CLI Commands

### General Commands

```bash
# Show version
./mangahub version

# Show help
./mangahub help
```

### Authentication Commands

```bash
# Register new account
./mangahub auth register --username <username> --email <email>

# Login
./mangahub auth login --username <username>

# Check login status
./mangahub auth status

# Logout
./mangahub auth logout
```

**Example:**
```bash
./mangahub auth register --username john --email john@example.com
./mangahub auth login --username john
```

### Manga Management Commands

```bash
# Search manga
./mangahub manga search <query>

# Search manga using gRPC
./mangahub manga search <query> --use-grpc

# Get manga details
./mangahub manga info <manga-id>

# Get manga details using gRPC
./mangahub manga info <manga-id> --use-grpc

# List all manga
./mangahub manga list
```

**Examples:**
```bash
./mangahub manga search "One Piece"
./mangahub manga info one-piece
./mangahub manga search "Naruto" --use-grpc
```

### Library Management Commands

```bash
# View your library
./mangahub library list

# Filter library by status
./mangahub library list --status reading

# Add manga to library
./mangahub library add --manga-id <id> --status <status>

# Remove manga from library
./mangahub library remove <manga-id>
```

**Examples:**
```bash
./mangahub library add --manga-id one-piece --status reading
./mangahub library list --status completed
./mangahub library remove naruto
```

**Available status values:**
- `reading`
- `completed`
- `plan-to-read`
- `on-hold`
- `dropped`

### Progress Management Commands

```bash
# Update reading progress
./mangahub progress update --manga-id <id> --chapter <number>

# View progress history
./mangahub progress history
```

**Example:**
```bash
./mangahub progress update --manga-id one-piece --chapter 1000
```

### TCP Synchronization Commands

```bash
# Connect to TCP sync server (test connection)
./mangahub sync connect

# Monitor real-time progress updates
./mangahub sync monitor

# Check sync status
./mangahub sync status
```

**Example:**
```bash
# Terminal 1: Monitor updates
./mangahub sync monitor

# Terminal 2: Update progress (will broadcast to Terminal 1)
./mangahub progress update --manga-id one-piece --chapter 50
```

### UDP Notification Commands

```bash
# Subscribe to notifications
./mangahub notify subscribe

# Test UDP connection
./mangahub notify test

# Send chapter notification (admin)
./mangahub notify send --manga-id <id> --chapter <number>
```

**Example:**
```bash
# Terminal 1: Subscribe to notifications
./mangahub notify subscribe

# Terminal 2: Send notification (will appear in Terminal 1)
./mangahub notify send --manga-id one-piece --chapter 1100
```

### WebSocket Chat Commands

```bash
# Join chat room
./mangahub chat join
```

**Example:**
```bash
# After joining, simply type messages and press Enter
./mangahub chat join
> Hello everyone!
> Type /quit to exit
```

### gRPC Operations Commands

```bash
# Get manga via gRPC
./mangahub grpc get --manga-id <id>

# Search manga via gRPC
./mangahub grpc search --query <text>

# Update progress via gRPC
./mangahub grpc update --manga-id <id> --chapter <number>
```

**Examples:**
```bash
./mangahub grpc get --manga-id one-piece
./mangahub grpc search --query "Demon Slayer"
./mangahub grpc update --manga-id naruto --chapter 700
```

### Server Management Commands

```bash
# Check server status
./mangahub server status

# Ping all servers
./mangahub server ping
```

### Configuration Commands

```bash
# Show current configuration
./mangahub config show
```

### Statistics Commands

```bash
# View reading statistics
./mangahub stats overview
```

### Export Commands

```bash
# Export library to JSON
./mangahub export library --output library.json
```

## API Testing

### Using cURL

**Register User:**
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com","password":"testpass123"}'
```

**Login:**
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass123"}'
```

**Get Manga List:**
```bash
curl http://localhost:8080/api/manga
```

**Search Manga:**
```bash
curl "http://localhost:8080/api/manga?query=One"
```

**Get Manga by ID:**
```bash
curl http://localhost:8080/api/manga/one-piece
```

**Get Library (requires authentication):**
```bash
curl http://localhost:8080/api/library \
  -H "Authorization: Bearer <your-token>"
```

**Update Progress:**
```bash
curl -X PUT http://localhost:8080/api/progress \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/json" \
  -d '{"manga_id":"one-piece","chapter":100}'
```

### Using netcat (TCP/UDP Testing)

**Test TCP Server:**
```bash
# Connect
nc localhost 9090

# Send auth message
{"user_id":"user123"}
```

**Test UDP Server:**
```bash
# Send ping
echo '{"type":"ping"}' | nc -u localhost 9091

# Register for notifications
echo '{"type":"register","user_id":"user123"}' | nc -u localhost 9091
```

## Troubleshooting

### Port Already in Use

```bash
# Windows
netstat -ano | findstr :8080
taskkill /PID <PID> /F

# macOS/Linux
lsof -i :8080
kill -9 <PID>
```

### Database Locked

Ensure only one process accesses the database at a time. Close all database connections (DBeaver, etc.) before running servers.

### Protobuf Generation Errors

```bash
# Verify protoc is installed
protoc --version

# Verify plugins are in PATH
# Windows
where protoc-gen-go
where protoc-gen-go-grpc

# macOS/Linux
which protoc-gen-go
which protoc-gen-go-grpc

# Regenerate protobuf files
cd proto
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       manga.proto
cd ..
```

### Connection Refused

Ensure all servers are running before testing. Check server logs for errors:

```bash
# Check API server
curl http://localhost:8080/health

# Check TCP server
nc -v localhost 9090

# Check UDP server
echo '{"type":"ping"}' | nc -u localhost 9091
```

### Go Module Issues

```bash
# Clean module cache
go clean -modcache

# Reinstall dependencies
go mod tidy
```

### JWT Token Expired

Tokens expire after 24 hours. Re-login using:
```bash
./mangahub auth login --username <username>
```

## Environment Variables

| Variable | Default | Description |
|---------|---------|-------------|
| `PORT` | `8080` | HTTP API server port |
| `DB_PATH` | `./data/mangahub.db` | SQLite database path |
| `JWT_SECRET` | Auto-generated | JWT signing secret |
| `TCP_PORT` | `9090` | TCP server port |
| `UDP_PORT` | `9091` | UDP server port |
| `GRPC_PORT` | `9092` | gRPC server port |

**Example:**
```bash
# Windows (PowerShell)
$env:DB_PATH="C:\custom\path\db.sqlite"
$env:PORT="8000"
go run cmd/server/main.go

# macOS/Linux
export DB_PATH=/custom/path/db.sqlite
export PORT=8000
go run cmd/server/main.go
```

## Common Workflows

### Complete User Workflow

```bash
# 1. Start server
go run cmd/server/main.go

# 2. Open new terminal and initialize CLI
./mangahub init

# 3. Register account
./mangahub auth register --username alice --email alice@example.com

# 4. Login
./mangahub auth login --username alice

# 5. Search manga
./mangahub manga search "One Piece"

# 6. Add to library
./mangahub library add --manga-id one-piece --status reading

# 7. Update progress
./mangahub progress update --manga-id one-piece --chapter 10

# 8. View library
./mangahub library list
```

### Real-time Sync Workflow

```bash
# Terminal 1: Monitor sync
./mangahub sync monitor

# Terminal 2: Update progress
./mangahub progress update --manga-id naruto --chapter 50

# You'll see the update appear in Terminal 1 in real-time
```

### Notification Workflow

```bash
# Terminal 1: Subscribe to notifications
./mangahub notify subscribe

# Terminal 2: Send notification
./mangahub notify send --manga-id demon-slayer --chapter 205

# Terminal 1 will receive the notification
```

## Project Structure

```
mangahub/
├── cmd/
│   ├── api-server/      # HTTP REST API server
│   ├── tcp-server/      # TCP sync server
│   ├── udp-server/      # UDP notification server
│   ├── grpc-server/     # gRPC server
│   ├── server/          # Unified server (all-in-one)
│   └── cli/             # CLI application
├── internal/
│   ├── auth/            # JWT authentication
│   ├── grpc/            # gRPC implementation
│   ├── manga/           # Manga business logic
│   ├── tcp/             # TCP server logic
│   ├── udp/             # UDP server logic
│   ├── user/            # User management
│   └── websocket/       # WebSocket chat
├── pkg/
│   ├── database/        # Database initialization
│   ├── models/          # Data models
│   └── proto/           # Generated protobuf code
├── proto/
│   └── manga.proto      # Protocol buffer definitions
├── data/                # Database and data files
├── go.mod               # Go module dependencies
└── go.sum               # Go module checksums
```