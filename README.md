# Onespace

Onespace is a single-user local Docker development control plane for running, rebuilding, debugging, configuring, and observing Go/Java microservices and image-based local services on one VM.

Primary workflows:

- Web UI for service status, logs, jobs, and manual deploy/debug actions.
- CLI for developers, scripts, and coding agents.
- Docker Compose dev runners for container-side build/run/debug.
- Direct container services for image-first tools such as mock BMCs, local databases, or protocol simulators.
- `onespace.yaml` as a local app contract for env, env files, config files, secret files, volumes, ports, health checks, and dependencies.
- Config Inspector through API, CLI, and Web UI so humans and agents can see where runtime configuration came from.

Non-goals:

- No multi-user control plane, RBAC, credential management, or production deployment workflow.
- No Kubernetes, k3d, kind, Terraform, or high-fidelity local Kubernetes runtime.
- No file watching, automatic rebuild/restart loop, or build/test timeline.

Start with:

```bash
go run ./cmd/onespace-server serve --config examples/workspaces/smoke-go/onespace.yaml
go run ./cmd/onespace deploy user-api --wait --json
go run ./cmd/onespace config user-api --json
```

Example `onespace.yaml` contract fields:

```yaml
services:
  user-api:
    env:
      APP_ENV: local
    envFrom:
      - file: .env
      - file: .env.local
        optional: true
    files:
      - source: config/local.yaml
        target: /etc/user-api/config.yaml
        mode: "0444"
    secrets:
      - name: DB_PASSWORD
        fromFile: .secrets/db_password.example
    volumes:
      - source: onespace-user-api-cache
        target: /workspace/.cache
    dependsOn:
      - redis

  bmc-a:
    kind: container
    image: metal-forge/mock-ipmi:dev
    ports:
      - name: ipmi
        container: 623
        host: 6230
        protocol: udp
```

See [docs/user-guide.md](docs/user-guide.md) for installation, quick start, workspace configuration, CLI, Web UI, API, and troubleshooting details.
