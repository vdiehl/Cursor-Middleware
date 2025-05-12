# Cursor-Middleware
Cursor Middleware to support local vllms, e.g. LM Studio, using Cursor tools

## Project Overview
This is a Go middleware project with a simple API handler. To run, use `go run main.go`.

## Features
- Basic HTTP proxy handling
- Tool call translation logic
- Integration-ready middleware structure

## Getting Started
1. **Install Dependencies**: Ensure you have Go 1.20+ installed
2. **Build Project**: `go mod tidy && go build -o cursor-middleware`
3. **Run Server**: `./cursor-middleware` (Defaults to :80)

## Usage
```bash
# Example API request (replace with actual endpoints)
curl -X POST http://localhost:80/api/v1/endpoint \
  -H "Content-Type: application/json" \
  -d '{"key":"value"}'
```

## Contributing
- Fork the repository
- Create a feature branch
- Add tests for new functionality
- Follow Go fmt standards
- Submit a pull request

## License
[See LICENSE file for details](LICENSE)

## API Endpoints
```api
GET /health
  Checks if the middleware is running

POST /api/v1/proxy
  Proxy endpoint for forwarding requests to local vLLM servers
```

## Development Setup
- **Environment Variables**: 
  - `PORT`: Custom port (default: 8080)
  - `LOG_LEVEL`: debug/info/warn/error

## Testing
```bash
# Run tests
GO111MODULE=on go test ./...
```

## Architecture
This middleware acts as a proxy between local vLLM servers and client applications. It supports:
- Tool call translation between different API formats
- Customizable request/response handling
- Easy integration with existing services
