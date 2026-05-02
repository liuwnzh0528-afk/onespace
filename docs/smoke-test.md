# Smoke Test

Build dev image:

```bash
docker build -t onespace/go-dev:1.23 -f deploy/images/go-dev/Dockerfile .
```

Start daemon:

```bash
go run ./cmd/onespace-server serve --config examples/workspaces/smoke-go/onespace.yaml
```

Deploy service:

```bash
go run ./cmd/onespace deploy user-api --wait --json
```

Check health:

```bash
curl -fsS http://127.0.0.1:18081/health
```
