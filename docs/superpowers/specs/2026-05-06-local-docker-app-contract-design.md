# Onespace Local Docker App Contract Design

日期：2026-05-06

状态：待用户评审

## 1. 背景

Onespace 当前已经具备单用户 daemon、CLI、静态 Web UI、Docker Compose dev runner、Go/Java 服务构建运行、日志、任务和健康检查能力。下一步产品定义要从“能部署一个服务”推进到“能准确描述服务运行契约，并解释容器里每项配置从哪里来”。

本阶段不扩展到 Kubernetes、k3d、kind、Terraform 或高保真本地 K8s。底层只支持本机 Docker 容器运行。Docker Compose 可以继续作为实现细节，但产品概念对用户暴露为 local Docker app contract，而不是多 runtime 抽象。

## 2. 目标

- 扩展 `onespace.yaml`，让用户描述服务运行所需的环境变量、环境文件、配置文件、敏感文件、volume、端口、健康检查和启动依赖。
- 新增 Config Composer，合成服务最终运行配置，并记录每个配置项的来源。
- 新增 Config Inspector，通过 API、CLI 和 Web UI 展示服务配置来源。
- 确保 secret 在 API、CLI、UI、日志和 job result 中默认脱敏。
- 将配置合成结果渲染进现有 Docker Compose dev runner。
- 保持当前 Go/Java dev runner、build/restart/deploy/debug/logs/jobs/health 行为兼容。

## 3. 非目标

- 不做文件监听。
- 不做自动 rebuild 或自动 restart。
- 不做通用测试命令编排。
- 不做 Build/Test Timeline。
- 不做 Kubernetes、k3d、kind 或本地高保真 K8s。
- 不做 Terraform variable export 或 drift check。
- 不做生产 secret 管理、云 secret backend 或凭据托管。
- 不做完整 Postman 替代品。
- 不在本阶段实现 Agent Workbench。Agent 后续会消费本阶段提供的结构化配置、日志和 deploy 结果。

## 4. 产品边界

Onespace 继续定位为本地开发控制面：

```text
onespace.yaml
  ↓
Config Composer
  ↓
Docker Compose dev runner
  ↓
deploy / debug / logs / health
  ↓
Config Inspector / Web UI / CLI / future agent tools
```

本阶段的核心交付不是“更多容器编排能力”，而是让用户和后续 agent 能回答这些问题：

- 容器最终拿到了哪些 env？
- 某个 env 来自 `onespace.yaml`、`.env`、`.env.local` 还是系统生成？
- 某个配置文件是否挂进了容器，目标路径是什么？
- 哪些配置被标记为 secret，是否已经脱敏？
- 当前服务依赖哪些 addon 或服务，启动顺序在 Compose 中是否可表达？

## 5. `onespace.yaml` 扩展

当前服务配置保留：

```yaml
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
```

新增字段：

```yaml
services:
  user-api:
    env:
      APP_ENV: local
      LOG_LEVEL: debug

    envFrom:
      - file: .env
      - file: .env.local
        optional: true

    files:
      - source: config/local.yaml
        target: /etc/user-api/config.yaml
        mode: "0444"
      - source: certs/dev-ca.pem
        target: /etc/ssl/dev-ca.pem
        mode: "0444"

    secrets:
      - name: DB_PASSWORD
        fromFile: .secrets/db_password
      - name: SERVICE_TOKEN
        fromFile: .secrets/service_token

    secretFiles:
      - source: .secrets/client.key
        target: /etc/user-api/client.key
        mode: "0400"

    volumes:
      - source: onespace-user-api-cache
        target: /workspace/.cache
      - source: ./tmp
        target: /workspace/tmp

    dependsOn:
      - postgres
      - redis
```

### 路径规则

- `source`、`envFrom.file`、`secrets.fromFile` 和 `secretFiles.source` 的相对路径按 workspace root 解析。
- `repoPath` 仍然必须位于 `allowedRepoRoots` 下。
- 配置文件和 secret 文件不要求位于 repoPath 下，因为它们通常属于 workspace。
- 相对 host bind mount 路径按 workspace root 解析。
- 命名 volume 不含 `/`、`.` 前缀或路径分隔符时按 Docker named volume 处理。

## 6. 配置合成规则

Config Composer 生成两类输出：

1. Runtime materialization：供 Compose 生成使用。
2. Inspector view：供 API、CLI、UI 和后续 agent 使用。

环境变量优先级：

```text
language/runtime generated env
< onespace.yaml env
< envFrom files in listed order
< service generated dependency env
```

第一版不实现 CLI `--env`、用户本机 override 或 agent session override。后续如果需要，再在优先级尾部追加。

`envFrom.file` 支持简单 `.env` 格式：

```text
KEY=value
QUOTED="value"
SINGLE='value'
# comment
EMPTY=
```

第一版不展开 shell 表达式，不执行命令，不支持 `export KEY=value` 以外的复杂 shell 语义。解析失败时 daemon 启动失败；`optional: true` 的文件不存在时跳过并在 inspector 中显示 skipped source。

## 7. Secret 规则

Secret 分两类：

- secret env：`secrets[].name + fromFile` 注入到容器环境变量。
- secret file：`secretFiles[].source + target` 以只读文件形式挂载到容器。

脱敏规则：

- Inspector 中 `secret env` 的 `value` 永远显示为 `******`。
- Inspector 中 `secret file` 显示 source path、target、mode 和 `secret: true`，不显示文件内容。
- Compose 生成时可以把 secret env 值放入容器环境，因为本阶段只面向本机 Docker；但不会写入 job result、API response 或 Web UI 明文。
- 读取 secret 文件失败时 daemon 启动失败，避免服务以缺失 secret 的状态运行。

## 8. Inspector API

新增：

```text
GET /api/services/{service}/config
```

响应：

```json
{
  "service": "user-api",
  "env": [
    {
      "name": "APP_ENV",
      "value": "local",
      "source": "onespace.yaml env",
      "secret": false
    },
    {
      "name": "DB_PASSWORD",
      "value": "******",
      "source": ".secrets/db_password",
      "secret": true
    }
  ],
  "files": [
    {
      "source": "/workspace/config/local.yaml",
      "target": "/etc/user-api/config.yaml",
      "mode": "0444",
      "secret": false
    }
  ],
  "volumes": [
    {
      "source": "onespace-user-api-cache",
      "target": "/workspace/.cache",
      "type": "volume"
    }
  ],
  "dependsOn": ["postgres", "redis"]
}
```

## 9. CLI

新增：

```bash
onespace config <service>
onespace config <service> --json
```

文本输出重点是可扫读：

```text
SERVICE user-api

ENV
APP_ENV       local       onespace.yaml env
DB_PASSWORD   ******      .secrets/db_password secret

FILES
/etc/user-api/config.yaml   config/local.yaml   0444
```

`--json` 输出与 API 响应一致。

## 10. Web UI

Web UI 在服务行增加 `Config` 操作，点击后在 Activity 面板展示：

- env 表格。
- files 表格。
- volumes 表格。
- dependsOn 列表。

第一版不做独立复杂页面，不做配置编辑，只读即可。

## 11. Compose 渲染

Compose 生成逻辑继续写入 `generated/docker-compose.yml`。服务定义新增：

- `environment`：包含 composer 合成后的 runtime env 和 secret env。
- `volumes`：包含 repo mount、config files、secret files、用户声明 volumes。
- `depends_on`：包含用户声明依赖。

文件挂载默认 readonly。用户声明 volume 默认为读写。secret file 默认 readonly。

## 12. 验收标准

- 现有示例 workspace 在不添加新字段时仍可加载、测试和生成 Compose。
- 新示例 workspace 能包含 `.env`、`.env.local`、config file、secret file、volume 和 dependsOn。
- `GET /api/services/user-api/config` 返回配置来源，secret value 脱敏。
- `onespace config user-api --json` 返回同样结构。
- Web UI 可以查看 Config Inspector。
- `generated/docker-compose.yml` 包含 env、file mounts、secret mounts、volumes 和 depends_on。
- `go test ./...` 通过。
- 不引入 K8s、kind、k3d、Terraform、文件监听、自动 rebuild/restart 或测试 timeline 概念。
