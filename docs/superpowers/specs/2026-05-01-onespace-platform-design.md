# Onespace 轻量化自动化部署平台设计

日期：2026-05-01

状态：待用户评审

## 1. 背景

Onespace 是面向单个开发者、单台虚拟机的轻量化微服务开发部署平台。它解决的核心问题是：开发者或编码 agent 在多个独立仓库中修改 Go / Java 微服务代码后，可以快速在同一台 VM 上完成容器内构建、重启、调试和健康检查。

平台不替代 CI/CD，不做多机调度，不做 Kubernetes 抽象。它更接近一个本地开发控制面：用 Web UI 给人观察和操作，用 CLI 给 Codex、Claude Code、opencode 等 agent 调用，用 Docker Compose 和 Docker Engine 承担底层容器编排。

## 2. 设计目标

- 单用户使用，部署在一台开发 VM 上。
- 支持每个微服务一个独立 Git 仓库。
- 不管理 Git 凭据，不负责 clone 授权；要求本地 repoPath 已存在且 remote 信息已配置。
- 支持保守的 `git pull --ff-only` 拉取更新。
- 支持 Go 和 Java 项目。
- 所有服务构建、运行、调试都在容器内完成。
- 支持 Web UI 操作和观察服务状态、任务、日志、调试端口。
- 提供 CLI，供开发者、脚本和编码 agent 调用。
- CLI 支持 `--json` 输出，便于 agent 解析失败阶段、日志引用和健康检查结果。
- 底层优先复用 Docker Compose，避免重新实现容器编排系统。
- 支持 dirty working tree 的 build / deploy，因为 agent 修改完代码后通常还没有提交。

## 3. 非目标

- 不做多用户、租户隔离、RBAC、审计系统。
- 不管理 Git 账号、SSH key、token、clone 授权。
- 不自动 stash、merge、rebase 或解决 Git 冲突。
- 不做生产发布、蓝绿、灰度、回滚、镜像仓库治理。
- 不做多 VM 调度。
- 不在第一版实现 MCP；CLI 稳定后再包装成 MCP tools。
- 不提供任意 shell 执行 API。

## 4. 总体架构

```text
开发者 / Web UI / CLI / Agent
        ↓
onespace-server 常驻 daemon
        ↓
Workspace 管理 / Git 状态 / Job runner / Runtime adapter
        ↓
Docker Compose + Docker Engine
        ↓
每服务一个 dev runner 容器
        ↓
容器内 build / run / debug
```

核心组件：

- `onespace-server`：常驻 daemon，提供 HTTP API、SSE/WebSocket 事件、Web UI 静态资源、任务队列、状态存储和 Docker Compose 适配。
- `onespace` CLI：调用 daemon API，支持人类可读输出和 `--json` 机器可读输出。
- Web UI：由 daemon 提供，展示 workspace、服务、Git 状态、任务、日志、运行状态和调试信息。
- SQLite：保存 workspace 注册信息、服务状态、任务历史、日志索引、端口分配结果。
- Docker Compose adapter：生成和执行 workspace 级 Compose 文件。
- Dev runner 容器：每个服务一个常驻开发容器，源码目录以 volume 方式挂载进去。

## 5. 部署模型

默认监听地址：

```text
127.0.0.1:17890
```

推荐访问方式：

- 本机浏览器访问 Web UI。
- 远程开发时通过 SSH tunnel 访问 Web UI。
- CLI 默认访问 `http://127.0.0.1:17890`。
- Codex、Claude Code、opencode 通过 CLI 触发构建和部署。

单用户模式下仍保留 token，但它是本机防误调用机制，不是复杂认证体系。

示例 CLI 配置：

```yaml
server: http://127.0.0.1:17890
token: local-dev-token
workspace: order-system-dev
output: text
```

## 6. Workspace 模型

Workspace 是一组本地仓库、服务配置、依赖组件和生成产物的集合。

推荐目录：

```text
/data/workspaces/order-system-dev/
  onespace.yaml
  repos/
    user-api/
    order-api/
    payment-api/
  generated/
    docker-compose.yml
  state/
    logs/
```

平台不要求仓库必须放在 `repos/`，但建议通过 `allowedRepoRoots` 限制可被平台引用的本地路径。

## 7. Workspace 配置

示例：

```yaml
version: 1
name: order-system-dev

allowedRepoRoots:
  - /data/workspaces/order-system-dev/repos

server:
  bind: 127.0.0.1:17890

runtime:
  type: compose
  projectName: order-system-dev
  network: order-system-dev

ports:
  appRange: "18080-18199"
  debugRange: "40000-40199"

services:
  user-api:
    language: go
    repoPath: /data/workspaces/order-system-dev/repos/user-api
    workdir: /workspace
    image: onespace/go-dev:1.23
    main: ./cmd/user-api
    ports:
      - name: http
        container: 8080
        host: 18081
    health:
      type: http
      url: http://127.0.0.1:18081/health
      timeoutSeconds: 30
    build:
      command: go build -o /workspace/.onespace/bin/app ./cmd/user-api
    run:
      command: /workspace/.onespace/bin/app
    debug:
      port: 40001
      buildCommand: go build -gcflags="all=-N -l" -o /workspace/.onespace/bin/app ./cmd/user-api
      command: dlv exec /workspace/.onespace/bin/app --headless --listen=:40001 --api-version=2 --accept-multiclient --continue

  order-api:
    language: java
    repoPath: /data/workspaces/order-system-dev/repos/order-api
    workdir: /workspace
    image: onespace/java-dev:21-maven
    ports:
      - name: http
        container: 8080
        host: 18082
    health:
      type: http
      url: http://127.0.0.1:18082/actuator/health
      timeoutSeconds: 60
    build:
      command: mvn package -DskipTests
    run:
      command: java -jar target/order-api.jar
    debug:
      port: 40002
      command: java -agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:40002 -jar target/order-api.jar

addons:
  postgres:
    image: postgres:16
    ports:
      - "15432:5432"
    env:
      POSTGRES_DB: app
      POSTGRES_USER: app
      POSTGRES_PASSWORD: app

  redis:
    image: redis:7
    ports:
      - "16379:6379"
```

## 8. 服务生命周期

每个服务使用一个常驻 dev runner 容器。repoPath 挂载到容器的 `/workspace`，构建缓存以 Docker volume 或宿主机目录挂载。

基础流程：

```text
ensure container
→ exec build command
→ stop previous service process
→ start run command
→ stream logs
→ wait health check
→ record job result
```

`deploy` 是面向 CLI 和 agent 的高级动作：

```text
deploy = build + restart + health check + structured result
```

`debug` 是特殊运行模式：

```text
debug = debug build + stop normal process + start debug process + return attach info
```

同一个服务在任一时刻只允许一个会改变运行状态的 job 执行，例如 `build`、`restart`、`deploy`、`debug`。只读动作如 `status`、`logs`、`health` 可以并发执行。

## 9. 容器内进程管理

MVP 推荐在 dev runner 容器内放一个轻量 supervisor。daemon 不直接长期持有业务进程，而是通过 supervisor 控制服务进程的启动、停止、重启和日志流。

推荐职责分工：

- daemon：任务编排、状态记录、调用 Docker API / Compose、下发 build/run/debug 命令。
- supervisor：容器内业务进程管理、退出码记录、stdout/stderr 转发。
- Docker：容器生命周期、网络、端口、volume。

如果第一版为了实现更快，也可以先用 `docker exec` 启动前台命令并由 daemon 记录进程状态；但正式设计以 supervisor 为默认目标，因为日志和进程控制更稳定。

## 10. Go Profile

Go dev runner 镜像包含：

- Go toolchain。
- Delve。
- 可选文件监听工具，例如 `air`、`reflex` 或 `watchexec`。
- 基础调试与证书工具。

默认挂载：

```text
repoPath      -> /workspace
go-build-cache -> /root/.cache/go-build
go-mod-cache   -> /go/pkg/mod
```

普通构建：

```bash
go build -o /workspace/.onespace/bin/app ./cmd/user-api
```

调试构建：

```bash
go build -gcflags="all=-N -l" -o /workspace/.onespace/bin/app ./cmd/user-api
```

调试启动：

```bash
dlv exec /workspace/.onespace/bin/app \
  --headless \
  --listen=:40001 \
  --api-version=2 \
  --accept-multiclient \
  --continue
```

IDE attach 信息由平台返回：

```json
{
  "service": "user-api",
  "mode": "debug",
  "debugger": "delve",
  "address": "127.0.0.1:40001"
}
```

## 11. Java Profile

Java dev runner 镜像按构建工具拆分：

- `onespace/java-dev:17-maven`
- `onespace/java-dev:21-maven`
- `onespace/java-dev:17-gradle`
- `onespace/java-dev:21-gradle`

默认挂载：

```text
repoPath      -> /workspace
maven-cache   -> /root/.m2
gradle-cache  -> /root/.gradle
```

Maven 构建：

```bash
mvn package -DskipTests
```

Gradle 构建：

```bash
gradle build -x test
```

Java 调试启动：

```bash
java -agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:40002 \
  -jar target/order-api.jar
```

Spring Boot 项目也可以选择 `mvn spring-boot:run` 或 `gradle bootRun` 作为 dev-fast 模式，但 MVP 默认使用 package 后 `java -jar`，行为更接近独立运行。

## 12. Git 策略

平台不拥有代码，只编排本地代码。

Git 能力范围：

- 读取当前分支。
- 读取 remote 信息。
- 读取 HEAD commit。
- 判断工作区是否 dirty。
- 判断 ahead / behind。
- 执行保守 pull。

唯一默认 pull 命令：

```bash
git -C <repoPath> pull --ff-only
```

pull 拒绝条件：

- repoPath 不存在。
- repoPath 不是 Git repo。
- 当前没有 tracking branch。
- 工作区 dirty。
- 本地和远端 diverged。
- 当前处于 detached HEAD。
- 认证失败。
- pull 发生冲突。

build / deploy 不要求 clean working tree。这样 agent 可以修改代码后立即触发构建。

## 13. CLI 设计

CLI 是 agent 集成的第一入口。所有命令默认调用 daemon API，不重复实现业务逻辑。

MVP 命令：

```bash
onespace status
onespace status user-api

onespace pull user-api
onespace pull --all

onespace build user-api
onespace up user-api
onespace restart user-api
onespace deploy user-api --wait
onespace deploy user-api --wait --json

onespace debug user-api --wait
onespace health user-api
onespace logs user-api --tail 200

onespace workspace list
onespace workspace use order-system-dev
```

Agent 推荐调用：

```bash
onespace deploy user-api --wait --json
```

成功响应：

```json
{
  "service": "user-api",
  "status": "success",
  "jobId": "job_20260501_0001",
  "commit": "a1b2c3d",
  "dirty": true,
  "container": "running",
  "health": "passing",
  "url": "http://127.0.0.1:18081"
}
```

失败响应：

```json
{
  "service": "user-api",
  "status": "failed",
  "jobId": "job_20260501_0002",
  "stage": "build",
  "exitCode": 2,
  "logRef": "job_20260501_0002"
}
```

失败时 CLI 返回非零 exit code，便于 agent 判断下一步是否读取日志或继续修复。

## 14. Web UI 设计

Web UI 面向人工观察和手动操作，不替代 CLI。

MVP 页面：

- Workspace 总览。
- 服务列表。
- 服务详情。
- 任务历史。
- 实时日志。
- 调试信息。

服务列表展示：

```text
service      language  branch          git       runtime   health   debug
user-api     Go        feature/login   dirty     running   passing  :40001
order-api    Java      develop         behind 3  failed    failing  :40002
```

服务详情展示：

- repoPath。
- remote。
- branch。
- HEAD commit。
- dirty / ahead / behind。
- 最近一次 build / deploy 结果。
- 当前容器状态。
- 当前业务进程状态。
- 端口映射。
- health check 结果。
- debug attach 信息。
- 操作按钮：pull、build、restart、deploy、debug、stop。

日志页支持：

- 按服务查看运行日志。
- 按 job 查看构建日志。
- tail 最新 N 行。
- 实时流式查看。

## 15. HTTP API 草案

MVP API：

```text
GET  /api/workspaces
GET  /api/workspaces/current
POST /api/workspaces/use

GET  /api/services
GET  /api/services/{service}
GET  /api/services/{service}/logs
GET  /api/services/{service}/health

POST /api/services/{service}/pull
POST /api/services/{service}/build
POST /api/services/{service}/up
POST /api/services/{service}/restart
POST /api/services/{service}/deploy
POST /api/services/{service}/debug
POST /api/services/{service}/stop

GET  /api/jobs
GET  /api/jobs/{jobId}
GET  /api/jobs/{jobId}/logs

GET  /api/events
```

`/api/events` 可以先用 Server-Sent Events，后续再切 WebSocket。SSE 对日志和任务状态推送已经足够轻。

## 16. Job 模型

Job 字段：

```text
id
type: pull | build | up | restart | deploy | debug | stop
workspace
service
status: queued | running | success | failed | canceled
stage
startedAt
finishedAt
exitCode
logRef
result
```

Job stage 示例：

```text
validate
git-status
pull
ensure-container
build
stop-process
start-process
health-check
done
```

Job 结果需要同时服务 UI 和 CLI。CLI 的 `--json` 输出来自同一份 result。

## 17. 日志策略

日志分三类：

- daemon 日志：平台自身运行日志。
- job 日志：pull、build、deploy、debug 等任务输出。
- service 日志：业务进程 stdout/stderr。

MVP 存储方式：

- SQLite 保存日志索引和 job 元信息。
- 文件系统保存日志内容。
- 日志按 workspace、service、job 分目录。

示例：

```text
state/logs/
  daemon.log
  jobs/
    job_20260501_0001.log
  services/
    user-api.log
```

日志 API 支持 `tail`，CLI 支持：

```bash
onespace logs user-api --tail 200
onespace logs user-api --job job_20260501_0001 --tail 200
```

## 18. Compose 生成策略

平台生成 workspace 级 Compose 文件：

```text
generated/docker-compose.yml
```

生成内容包括：

- 每个服务的 dev runner 容器。
- addon 容器，例如 Postgres、Redis。
- 网络。
- volume。
- 端口映射。
- 必要环境变量。

生成文件归平台管理。用户可以阅读和排查，但不建议手动编辑。需要自定义时通过 `onespace.yaml` 或服务内 `.onespace/service.yaml` 扩展。

## 19. 服务内可选配置

每个服务仓库可以提供：

```text
.onespace/service.yaml
```

用途：

- 覆盖 build command。
- 覆盖 run command。
- 覆盖 debug command。
- 定义默认 health check。
- 定义服务暴露端口。

workspace 配置优先级高于服务内配置。这样开发者可以在 workspace 层针对当前环境做覆盖。

优先级：

```text
workspace onespace.yaml
> service .onespace/service.yaml
> language profile 默认值
```

## 20. 安全边界

第一版安全策略：

- 默认仅监听 `127.0.0.1`。
- 支持 token header。
- repoPath 必须位于 allowedRepoRoots 下。
- 不提供任意 shell API。
- Git 只执行固定的受控命令。
- Docker 操作只针对当前 workspace 的 Compose project。
- debug 端口默认只建议通过 localhost 或 SSH tunnel 使用。
- 如果用户显式配置 `0.0.0.0` 监听，daemon 启动时给出高风险提示，并要求配置 token。

## 21. 错误处理

所有改变状态的 API 都返回 jobId。失败结果必须包含：

- service。
- jobId。
- stage。
- exitCode。
- logRef。
- userMessage。

常见错误：

- 配置错误：repoPath 不存在、端口冲突、未知语言。
- Git 错误：dirty 时 pull、diverged、认证失败、没有 tracking branch。
- 构建错误：编译失败、依赖下载失败。
- 运行错误：进程启动失败、端口占用。
- 健康检查错误：超时、HTTP 非成功状态、连接失败。
- Docker 错误：镜像不存在、Compose up 失败、Docker daemon 不可用。

错误处理原则：

- 平台不自动修复代码或 Git 冲突。
- 平台返回足够结构化信息，让人或 agent 能继续处理。
- build / deploy 失败不清理日志。
- restart 失败时保留失败进程日志。

## 22. Agent 集成

第一阶段以 CLI 为 agent 接口。

推荐 agent 工作流：

```text
agent 修改服务代码
→ onespace deploy <service> --wait --json
→ 成功：继续接口测试或通知用户
→ 失败：读取 job logs
→ agent 修复代码
→ 再次 deploy
```

后续 MCP 可以包装为：

```text
onespace_status(service)
onespace_deploy(service, wait)
onespace_logs(service, tail, jobId)
onespace_health(service)
onespace_debug(service)
```

MCP 不直接执行新逻辑，只调用 daemon API。

## 23. MVP 范围

MVP 包含：

- 单用户常驻 daemon。
- Web UI。
- CLI。
- workspace manifest 加载。
- 本地 repo 状态扫描。
- 保守 `git pull --ff-only`。
- Go profile。
- Java Maven profile。
- Docker Compose 生成和执行。
- dev runner 容器。
- 容器内 build / run / debug。
- deploy job。
- health check。
- 实时日志。
- job history。
- CLI `--json` 输出。

MVP 不包含：

- 多用户权限。
- MCP server。
- Gradle 深度优化。
- Kubernetes。
- 镜像仓库发布。
- 生产发布策略。
- 自动 Git merge / rebase / stash。
- 分布式任务调度。

## 24. 后续演进

第一阶段：本机开发闭环

- daemon、CLI、Web UI。
- Go + Java Maven。
- Compose runtime。
- dev runner 容器。
- deploy / debug / logs / health。

第二阶段：语言和项目模板增强

- Java Gradle 完善。
- Spring Boot devtools 友好模式。
- Go hot reload 模板。
- 常用 addon 模板：MySQL、Kafka、RabbitMQ、MinIO、Nacos。

第三阶段：Agent 体验增强

- MCP server。
- 更稳定的结构化错误码。
- agent 专用日志摘要 API。
- job cancellation。

第四阶段：团队或共享 VM 能力

- 多用户认证。
- workspace 权限。
- 操作审计。
- 端口配额。

共享 VM 能力不是当前单用户 MVP 的一部分。

## 25. 关键决策

- 使用常驻 daemon，而不是纯 CLI 临时执行。
- Web UI、CLI、未来 MCP 共用 daemon API。
- CLI 是 agent 第一接口。
- 不管理 Git 凭据和 clone。
- pull 使用 `git pull --ff-only`。
- build / deploy 支持 dirty working tree。
- 服务构建在容器内完成。
- 每个服务使用 dev runner 容器。
- 底层使用 Docker Compose。
- 初期面向单用户 VM。

## 26. 成功标准

MVP 完成后，一个典型开发流程应该满足：

1. 开发者在 VM 上准备好多个本地服务仓库。
2. 开发者配置一个 workspace manifest。
3. daemon 启动后，Web UI 能展示所有服务的 Git、构建、运行、健康状态。
4. 开发者或 agent 修改 Go / Java 服务代码。
5. 执行 `onespace deploy user-api --wait --json`。
6. 平台在容器内构建服务并重启进程。
7. 平台完成健康检查并返回结构化结果。
8. 构建失败时，agent 可以通过 `onespace logs` 获取日志并继续修复。
9. 需要调试时，执行 `onespace debug user-api --wait --json` 后可以用 IDE attach 到 Delve 或 JDWP。
