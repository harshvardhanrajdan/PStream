# PStream - A Go-based Streaming Server

PStream is a high-performance streaming server implementation in Go that provides WebSocket-based pub/sub functionality with built-in metrics using Prometheus.

## Features

- WebSocket-based streaming server
- Producer/Consumer pattern implementation
- In-memory message storage
- Prometheus metrics integration
- Concurrent message handling
- Easy to extend and customize

## Prerequisites

- Go 1.21 or higher
- Make (for build automation)

## Installation

Clone the repository:

```bash
git clone https://github.com/yourusername/PStream.git
cd PStream
```

Install dependencies:

```bash
go mod download
```

## Usage

### Starting the Server

To start the server:

```bash
make run
```

Or manually:

```bash
go run main.go
```

The server will start on port 3000 by default.

### Project Structure

- `main.go` - Entry point of the application
- `server.go` - WebSocket server implementation
- `producer.go` - Message producer implementation
- `consumer.go` - Message consumer implementation
- `storage.go` - Message storage interface and implementation
- `types.go` - Common types and interfaces
- `cmd/` - Command-line tools and utilities

### Configuration

The server can be configured using the `Config` struct:

```go
cfg := &Config{
    ListenAddr: ":3000",
    StoreProducerFunc: func() Storer {
        return NewMemoryStore()
    },
}
```

## API

### WebSocket Endpoints

- `/ws/producer` - Endpoint for message producers
- `/ws/consumer` - Endpoint for message consumers

### Metrics

Prometheus metrics are available at `/metrics` endpoint, providing:
- Message throughput
- Connection statistics
- System metrics

## Development

### Running Tests

```bash
make test
```

### Building

```bash
make build
```

