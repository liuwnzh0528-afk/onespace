# Onespace 用户指南

Onespace 是一个单用户、单机 VM 使用的微服务开发控制面。当前实现面向本地或远程开发环境，使用常驻 daemon 管理 workspace，通过 Docker Compose 创建每个服务的 dev runner 容器，并在容器内执行 build、run、debug、health check。CLI 和 Web UI 都调用 daemon API。

当前版本支持：

- Go 服务。
- Java Maven 服务。
- 每个服务一个本地 Git 仓库。
- `git pull --ff-only`。
- 容器内 build、deploy、restart、debug、stop。
- HTTP health check。
- `onespace.yaml` 运行配置契约：env、envFrom、files、secrets、volumes、dependsOn。
- Config Inspector：通过 API、CLI 和 Web UI 查看配置来源，secret 默认脱敏。
- job 历史、service 日志、job 日志接口。
- Web UI 的服务列表、Deploy、Debug、Logs、Config 操作。
- 供 agent 调用的 CLI JSON 输出。

当前版本不做：

- 凭据管理、clone、stash、merge、rebase 或冲突处理。
- 多用户、鉴权、RBAC、审计。
- 多 VM 调度或 Kubernetes 编排。
- Kubernetes、k3d、kind、Terraform、本地高保真 K8s。
- 文件监听、自动 rebuild/restart、Build/Test Timeline。
- 任意 shell 执行 API。
- MCP 封装。

## 前置条件

安装并确认以下工具可用：

```bash
go version
docker version
docker compose version
git --version
```

要求：

- Go 1.22 或更新版本。
- Docker Engine。
- Docker Compose v2。
- Git。
- 每个服务仓库必须已经存在于本机，并且仓库 remote 信息已经配置好。Onespace 只做状态读取和 `git pull --ff-only`。

## 快速开始

以下命令使用仓库内置 Go 示例。

1. 构建 CLI 和 daemon：

```bash
go build ./cmd/onespace-server
go build ./cmd/onespace
```

2. 构建 dev runner 镜像：

```bash
docker build -t onespace/go-dev:1.23 -f deploy/images/go-dev/Dockerfile .
docker build -t onespace/java-dev:21-maven -f deploy/images/java-dev-maven/Dockerfile .
```

3. 初始化示例服务仓库：

```bash
sh examples/workspaces/init-smoke-repos.sh
```

这个脚本只用于示例 workspace。它会把 `examples/workspaces/smoke-go/repos/user-api` 和 `examples/workspaces/smoke-java/repos/order-api` 初始化成本地 Git repo，并创建初始提交。

4. 启动 daemon：

```bash
./onespace-server serve --config examples/workspaces/smoke-go/onespace.yaml
```

daemon 默认监听 `127.0.0.1:18080`。保持该进程运行。

5. 在另一个终端部署 Go 示例服务：

```bash
./onespace deploy user-api --wait --json
curl -fsS http://127.0.0.1:18081/health
```

预期 CLI 返回类似：

```json
{"service":"user-api","status":"success","jobId":"job_deploy_user-api_...","stage":"done","dirty":false,"health":"passing","url":"http://127.0.0.1:18081/health"}
```

6. 查看 Go 示例服务的运行配置来源：

```bash
./onespace config user-api --json
```

预期返回包含 `APP_ENV`、`LOG_LEVEL`、`DB_PASSWORD`、配置文件挂载和 volume。Secret 值会显示为 `******`，不会在 CLI、API 或 Web UI 中明文展示。

7. 启动 Go debug 模式：

```bash
./onespace debug user-api --wait --json
```

预期返回：

```json
{"service":"user-api","status":"success","stage":"debug","debug":{"debugger":"dlv","address":"127.0.0.1:40001"}}
```

8. 验证 Java 示例：

先停止当前 daemon，然后启动 Java workspace：

```bash
./onespace-server serve --config examples/workspaces/smoke-java/onespace.yaml
```

另一个终端执行：

```bash
./onespace deploy order-api --wait --json
curl -fsS http://127.0.0.1:18082/health
./onespace debug order-api --wait --json
```

Java debug 返回 `debugger:"jdwp"`，地址为 `127.0.0.1:40002`。

## Workspace 配置

daemon 启动时只加载一个 workspace：

```bash
onespace-server serve --config /path/to/onespace.yaml
```

`onespace.yaml` 所在目录就是 workspace root。Onespace 会在该目录下生成：

```text
generated/docker-compose.yml
state/onespace.db
state/logs/
```

示例结构：

```text
my-workspace/
  onespace.yaml
  repos/
    user-api/
    order-api/
  generated/
  state/
```

示例配置：

```yaml
version: 1
name: order-system-dev

allowedRepoRoots:
  - repos

server:
  bind: 127.0.0.1:18080

runtime:
  type: docker-compose
  projectName: order-system-dev
  network: order-system-dev-default

ports:
  debugRange: "40000-40999"

services:
  user-api:
    language: go
    repoPath: repos/user-api
    main: ./cmd/user-api
    ports:
      - name: http
        container: 8080
        host: 18081
    health:
      type: http
      url: http://127.0.0.1:18081/health
      timeoutSeconds: 30

  order-api:
    language: java-maven
    repoPath: repos/order-api
    ports:
      - name: http
        container: 8080
        host: 18082
    health:
      type: http
      url: http://127.0.0.1:18082/health
      timeoutSeconds: 30
```

### 路径规则

- `allowedRepoRoots` 可以是绝对路径，也可以是相对 workspace root 的路径。
- `repoPath` 可以是绝对路径，也可以是相对 workspace root 的路径。
- `repoPath` 必须位于任意一个 `allowedRepoRoots` 下，否则 daemon 启动失败。

### 运行配置契约

服务可以声明环境变量、环境文件、配置文件、secret 文件、volume 和启动依赖：

```yaml
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
```

`onespace config <service> --json` 可以查看最终配置及来源。Secret 值在 CLI、API 和 Web UI 中显示为 `******`。

字段说明：

| 字段 | 作用 | 路径规则 |
| --- | --- | --- |
| `env` | 直接声明普通环境变量 | 不涉及路径 |
| `envFrom.file` | 从 `.env` 风格文件批量读取环境变量 | 相对 workspace root |
| `envFrom.optional` | 文件不存在时跳过并记录 warning | 不涉及路径 |
| `files` | 把普通配置文件只读挂载到容器 | `source` 相对 workspace root |
| `secrets` | 从文件读取 secret 并注入为环境变量 | `fromFile` 相对 workspace root |
| `secretFiles` | 把敏感文件只读挂载到容器 | `source` 相对 workspace root |
| `volumes` | 声明 Docker named volume 或 host bind mount | `./`、`../` 开头按 workspace root 解析 |
| `dependsOn` | 映射到 Compose `depends_on` | 名称应对应服务或 addon |

环境变量优先级：

```text
generated runtime env
< onespace.yaml env
< envFrom files in listed order
< secrets
```

`.env` 文件支持简单格式：

```text
KEY=value
QUOTED="value"
SINGLE='value'
export EXPORTED=yes
# comment
EMPTY=
```

不会执行 shell 表达式，也不会展开命令。非 optional 的 env 文件或 secret 文件缺失会导致 daemon 启动失败，避免容器在配置缺失时运行。

### 服务语言

当前支持：

| language | 默认镜像 | 默认 build | 默认 run | 默认 debug |
| --- | --- | --- | --- | --- |
| `go` | `onespace/go-dev:1.23` | `go build -o /workspace/.onespace/bin/app <main>` | `/workspace/.onespace/bin/app` | Delve |
| `java-maven` | `onespace/java-dev:21-maven` | `mvn package -DskipTests` | `java -jar target/*.jar` | JDWP |

Go 服务的 `main` 为空时默认使用 `.`。

可以用 `build.command`、`run.command`、`debug.buildCommand`、`debug.command` 覆盖默认命令。

### 端口

服务端口配置在 `services.<name>.ports`：

```yaml
ports:
  - name: http
    container: 8080
    host: 18081
```

debug 端口配置在 `services.<name>.debug.port`：

```yaml
debug:
  port: 40001
```

如果未配置 debug port，loader 会按服务名排序，从 `ports.debugRange` 的起始端口开始分配；未配置 `ports.debugRange` 时默认从 `40000` 开始。

## Daemon

启动：

```bash
onespace-server serve --config /path/to/onespace.yaml
```

查看版本：

```bash
onespace-server version
```

daemon 启动时会：

1. 读取并校验 workspace 配置。
2. 生成 `generated/docker-compose.yml`。
3. 打开 `state/onespace.db`。
4. 初始化 job store、log store、Git service、Compose runtime。
5. 启动 HTTP API 和 Web UI。

默认监听地址是 `127.0.0.1:18080`。可以通过 `server.bind` 修改。

远程访问建议使用 SSH tunnel，例如：

```bash
ssh -L 18080:127.0.0.1:18080 user@vm
```

## Web UI

如果 daemon 能找到 `web/static`，浏览器访问：

```text
http://127.0.0.1:18080/
```

当前 Web UI 提供：

- 服务列表。
- 手动刷新。
- Deploy。
- Debug。
- Logs。
- Config。
- Activity 面板显示操作结果、日志内容或配置来源。

当前 Web UI 是轻量 MVP：服务列表只展示 daemon 返回的基础服务信息，Git branch、runtime container 等扩展字段尚未由 API 填充。

## CLI

CLI 默认连接：

```text
http://127.0.0.1:18080
```

可以用环境变量覆盖：

```bash
ONESPACE_URL=http://127.0.0.1:18080 onespace status
```

### `status`

列出当前 workspace 服务：

```bash
onespace status
```

输出包括服务名、语言和 health 字段。当前 API 的服务列表未实时执行 health check，因此 health 可能为空。

### `pull <service>`

对服务仓库执行保守拉取：

```bash
onespace pull user-api
```

行为：

- 读取 Git 状态。
- 如果 working tree dirty，拒绝 pull。
- 如果 detached HEAD，拒绝 pull。
- 如果没有 tracking branch，拒绝 pull。
- 执行 `git pull --ff-only`。

说明：Onespace 会忽略 `.onespace/` 运行产物，避免自身 build/run 文件把仓库标记为 dirty。

### `build <service>`

在 dev runner 容器内执行 build 命令：

```bash
onespace build user-api
```

行为：

1. 确保 Compose 容器存在。
2. 启动目标服务容器。
3. 在容器内执行 `build.command`。
4. 返回 job result。

### `restart <service>`

重启服务进程，不重新 build：

```bash
onespace restart user-api
```

行为：

1. 通过 `onespace-supervisor stop` 停止旧进程。
2. 通过 `onespace-supervisor start <run.command>` 启动新进程。
3. 检查 supervisor status。

### `deploy <service>`

构建、重启并执行 health check：

```bash
onespace deploy user-api --wait --json
```

行为：

1. 读取 Git status，返回 commit 和 dirty 状态。
2. 确保容器存在并启动。
3. 在容器内执行 build。
4. 停止旧进程。
5. 启动 run 命令。
6. 在 `health.timeoutSeconds` 窗口内重试 HTTP health check。
7. 记录 job。

适合 agent 调用的 JSON 输出：

```bash
onespace deploy user-api --wait --json
```

失败时也会输出结构化结果，关键字段包括：

- `service`
- `status`
- `jobId`
- `stage`
- `exitCode`
- `health`
- `debug`

### `debug <service>`

构建 debug 版本并启动调试进程：

```bash
onespace debug user-api --wait --json
onespace debug order-api --wait --json
```

Go 返回 Delve 地址：

```json
{"debug":{"debugger":"dlv","address":"127.0.0.1:40001"}}
```

Java Maven 返回 JDWP 地址：

```json
{"debug":{"debugger":"jdwp","address":"127.0.0.1:40002"}}
```

连接方式取决于 IDE：

- Go：连接 Delve remote target。
- Java：连接 remote JVM debug target。

### `health <service>`

执行服务配置中的 health check：

```bash
onespace health user-api
```

输出：

```text
passing
```

如果服务没有 health 配置，返回 `unknown`。

### `config <service>`

查看服务最终运行配置及来源：

```bash
onespace config user-api
onespace config user-api --json
```

文本输出适合人工扫读，JSON 输出适合 agent 和脚本解析。响应包含：

- `env`：最终环境变量、展示值、来源、是否 secret。
- `files`：普通配置文件和 secret file 的 source、target、mode、是否 secret。
- `volumes`：Docker named volume 或 bind mount。
- `dependsOn`：服务或 addon 依赖。
- `warnings`：例如 optional env file 不存在。

Secret env 的真实值只进入 Docker runtime，Config Inspector 中始终显示为 `******`。

### `logs <service>`

读取服务日志：

```bash
onespace logs user-api --tail 200
```

日志来源：

1. daemon state 中的 service log。
2. 如果 daemon service log 为空，则回退读取服务仓库内 `.onespace/service.log`。

### `logs --job <jobId>`

读取 job 日志：

```bash
onespace logs --job job_deploy_user-api_1777735020868374000 --tail 200
```

当前实现提供 job log API 和文件存储能力，但主要 runtime 输出仍写入 service log。

### `stop <service>`

停止服务进程：

```bash
onespace stop user-api
```

只停止容器内业务进程，不删除 Compose 容器。

### `version`

查看 CLI 版本：

```bash
onespace version
```

## HTTP API

CLI 和 Web UI 使用同一组 daemon API。默认 base URL：

```text
http://127.0.0.1:18080
```

### Services

```http
GET /api/services
GET /api/services/{service}
GET /api/services/{service}/config
GET /api/services/{service}/health
GET /api/services/{service}/logs?tail=200
```

### Operations

```http
POST /api/services/{service}/pull
POST /api/services/{service}/build
POST /api/services/{service}/restart
POST /api/services/{service}/deploy
POST /api/services/{service}/debug
POST /api/services/{service}/stop
```

### Jobs

```http
GET /api/jobs?limit=50
GET /api/jobs/{jobId}
GET /api/jobs/{jobId}/logs?tail=200
```

job metadata 存储在 workspace 的 `state/onespace.db`。

### Events

```http
GET /api/events
```

当前 API 提供 SSE endpoint，deploy 操作会 publish job event。Web UI 当前未依赖 SSE。

## 容器运行模型

daemon 为整个 workspace 生成一个 Compose 文件：

```text
<workspace>/generated/docker-compose.yml
```

每个服务对应一个 dev runner 容器：

- service repo 挂载到容器 `/workspace`。
- Config Composer 会把 `env`、`envFrom`、`secrets` 合成为 Compose `environment`。
- `files` 和 `secretFiles` 以只读 bind mount 进入容器。
- `volumes` 会映射为 named volume 或 host bind mount。
- `dependsOn` 会映射为 Compose `depends_on`。
- 容器默认命令是 `sleep infinity`。
- build、run、debug 都通过 `docker compose exec` 在容器内执行。
- 业务进程由 `onespace-supervisor` 管理。
- 业务日志写入容器内 `/workspace/.onespace/service.log`，也就是宿主机服务 repo 的 `.onespace/service.log`。

清理容器：

```bash
docker compose -f <workspace>/generated/docker-compose.yml down --remove-orphans
```

## Git 行为

Onespace 不管理凭据，也不 clone 仓库。服务 repo 必须提前准备好。

`deploy` 会读取 Git status，用于返回 commit 和 dirty 状态；dirty 不会阻止 deploy，因为 agent 修改代码后通常需要直接 build/run 验证。

`pull` 会拒绝以下情况：

- working tree dirty。
- detached HEAD。
- 没有 tracking branch。
- `git pull --ff-only` 失败。

## Agent 调用建议

Codex、Claude Code、opencode 或脚本优先使用 CLI，而不是直接调用 Docker：

```bash
onespace deploy user-api --wait --json
onespace config user-api --json
```

建议 agent 处理流程：

1. 修改代码。
2. 运行单元测试或局部测试。
3. 调用 `onespace deploy <service> --wait --json`。
4. 如果失败，读取 JSON 的 `stage` 和 `jobId`。
5. 调用 `onespace config <service> --json` 确认 env、文件挂载和 secret 来源。
6. 调用 `onespace logs <service> --tail 200` 查看服务日志。
7. 修复后再次 deploy。

常见失败阶段：

| stage | 含义 |
| --- | --- |
| `validate` | 服务名不存在或配置错误 |
| `git-status` | repoPath 不是 Git repo 或 Git 状态读取失败 |
| `ensure-container` | Compose 文件或 Docker/Compose 调用失败 |
| `build` | 容器内 build 命令失败 |
| `start-process` | supervisor 启动业务进程失败 |
| `done` | deploy 完成 |
| `debug` | debug 启动完成 |

## 常见操作

### 使用自定义 workspace

1. 创建 workspace 目录。
2. 把服务仓库放到 `repos/` 或其他受控目录。
3. 编写 `onespace.yaml`。
4. 构建需要的 dev runner 镜像。
5. 启动 daemon。
6. 用 CLI deploy 服务。

示例：

```bash
mkdir -p /data/workspaces/order-system-dev/repos
cd /data/workspaces/order-system-dev
onespace-server serve --config onespace.yaml
```

### 修改服务代码后重新部署

```bash
onespace deploy user-api --wait --json
```

### 进入 debug 模式

```bash
onespace debug user-api --wait --json
```

然后用 IDE 连接返回的 debug address。

### 查看最近 job

```bash
curl -fsS 'http://127.0.0.1:18080/api/jobs?limit=10'
```

### 查看服务日志

```bash
onespace logs user-api --tail 200
```

### 查看服务配置来源

```bash
onespace config user-api
onespace config user-api --json
curl -fsS http://127.0.0.1:18080/api/services/user-api/config
```

### 停止服务进程

```bash
onespace stop user-api
```

### 停止并删除 dev runner 容器

```bash
docker compose -f generated/docker-compose.yml down --remove-orphans
```

需要在 workspace root 下执行，或把路径换成实际 workspace 的 `generated/docker-compose.yml`。

## 排障

### daemon 启动失败：`repoPath is not under any allowedRepoRoot`

检查 `repoPath` 是否在 `allowedRepoRoots` 下。相对路径按 `onespace.yaml` 所在目录解析。

### daemon 启动失败：env file 或 secret file 不存在

检查 `envFrom.file`、`secrets.fromFile`、`files.source` 和 `secretFiles.source` 是否相对 workspace root 存在。可选 env 文件需要显式设置：

```yaml
envFrom:
  - file: .env.local
    optional: true
```

Secret 文件不支持 optional。缺失时应先补齐本地 secret 文件，再启动 daemon。

### deploy 失败在 `git-status`

检查服务目录是否是 Git repo：

```bash
git -C <repoPath> status
```

示例 workspace 可执行：

```bash
sh examples/workspaces/init-smoke-repos.sh
```

### deploy 失败在 `ensure-container`

检查 Docker 和 Compose：

```bash
docker version
docker compose version
```

确认 daemon 已生成：

```bash
ls <workspace>/generated/docker-compose.yml
```

### deploy 失败在 `build`

直接查看服务日志和容器环境：

```bash
onespace logs <service> --tail 200
docker compose -f <workspace>/generated/docker-compose.yml exec -T <service> sh -c 'pwd; env; command -v go || true; command -v java || true'
```

### health 一直 failing

检查：

- 服务是否监听配置的 container port。
- host port 是否被其他进程占用。
- `health.url` 是否使用正确 host port。
- 应用启动是否需要更长时间；调整 `health.timeoutSeconds`。

### debug 返回地址但 IDE 连不上

检查：

- `debug.port` 是否已映射到宿主机。
- Go 服务容器内是否有 `/go/bin/dlv`。
- Java 服务是否用 JDWP 参数启动。
- 远程 VM 场景下是否已建立 SSH tunnel。

### pull 被拒绝

`pull` 是保守操作。先检查：

```bash
git -C <repoPath> status
git -C <repoPath> branch -vv
```

如果工作区有未提交业务修改，Onespace 不会自动 stash 或 merge。

## 当前限制

- 一次 daemon 只加载一个 workspace。
- 无鉴权，只建议监听 `127.0.0.1` 并通过 SSH tunnel 访问。
- Web UI 只提供基础操作面板。
- Config Inspector 是只读视图，不提供配置编辑。
- CLI 的 `--wait` 当前操作是同步执行；参数保留用于后续异步 job 模型。
- job result 中的 `result` 字段在 API JSON 中是 base64 编码的原始 JSON bytes。
- job log API 已存在，但当前主要进程输出通过 service log 暴露。
- Compose 容器生命周期需要手动 `docker compose down` 清理。
