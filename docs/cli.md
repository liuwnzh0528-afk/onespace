# Onespace CLI

Agent deploy:

```bash
onespace deploy user-api --wait --json
```

Read failed job logs:

```bash
onespace logs user-api --job job_20260501_0002 --tail 200
```

## Commands

| Command | Description |
|---------|-------------|
| `status` | List services and their status |
| `pull <service>` | Pull latest changes (ff-only) |
| `build <service>` | Build a service |
| `restart <service>` | Restart a service |
| `deploy <service>` | Build and deploy a service |
| `debug <service>` | Start service in debug mode |
| `health <service>` | Check service health |
| `logs <service>` | Read service logs |
| `version` | Print version |

## Flags

- `--wait` Wait for operation to complete (default: true)
- `--json` Output result as JSON
- `--job <id>` Specify job ID for logs
- `--tail <n>` Number of log lines (default: 200)

## Environment

- `ONESPACE_URL` Daemon URL (default: `http://127.0.0.1:18080`)
