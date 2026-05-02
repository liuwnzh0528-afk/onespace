# Onespace

Onespace is a single-user development control plane for running, rebuilding, debugging, and observing local Go and Java microservices on one VM.

Primary workflows:

- Web UI for service status, logs, jobs, and manual deploy/debug actions.
- CLI for developers, scripts, and coding agents.
- Docker Compose dev runners for container-side build/run/debug.

Start with:

```bash
go run ./cmd/onespace-server serve --config examples/workspaces/smoke-go/onespace.yaml
go run ./cmd/onespace deploy user-api --wait --json
```
