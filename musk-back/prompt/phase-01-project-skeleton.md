You are implementing a production-grade Go AI Agent project.

Project name:

autonomous-agent

Core stack:

- Go
- CloudWeGo Eino
- OpenAI-compatible LLM
- HTTP API
- YAML configuration
- Structured logging
- Docker Compose

Phase 1 goal:

Build the full project skeleton and a minimal runnable agent.

Requirements:

1. Initialize a Go module.
2. Integrate CloudWeGo Eino.
3. Create cmd/server/main.go.
4. Create internal/agent/runtime.
5. Create a Model Provider abstraction.
6. Support an OpenAI-compatible API.
7. Configure these environment variables:
   - LLM_BASE_URL
   - LLM_API_KEY
   - LLM_MODEL
8. Implement the simplest agent:
   User Input → Eino Agent → LLM → Response.
9. Provide an HTTP API:

POST /api/v1/agent/run

Request:

{
  "message": "hello"
}

Response:

{
  "request_id": "...",
  "answer": "..."
}

10. Add a health check:

GET /health

11. Add structured logging.
12. Add graceful shutdown.
13. Add configs/agent.yaml.
14. Create README.md.
15. Create a Makefile.

Engineering requirements:

- Idiomatic Go
- Pass context.Context through the whole call chain
- Abstract every dependency behind an interface
- Do not hardcode configuration
- Business code must not depend on the HTTP handler directly
- Decouple Agent Runtime from the API

Tests:

1. go test ./...
2. go vet ./...
3. go build ./...

When finished, report:

- Project layout
- How to start the server
- API example
- Test results

Do not implement the next phase.
