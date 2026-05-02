# MVP Verification

Run unit tests:

```bash
go test ./...
```

Build binaries:

```bash
go build ./cmd/onespace-server
go build ./cmd/onespace
```

Build dev images:

```bash
docker build -t onespace/go-dev:1.23 -f deploy/images/go-dev/Dockerfile .
docker build -t onespace/java-dev:21-maven -f deploy/images/java-dev-maven/Dockerfile .
```

Run smoke test:

```bash
go run ./cmd/onespace-server serve --config examples/workspaces/smoke-go/onespace.yaml
go run ./cmd/onespace deploy user-api --wait --json
curl -fsS http://127.0.0.1:18081/health
```
