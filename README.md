# jupi-d2c

D2C 插件图片上传后端的 Go 版本（daemon）。从 Node.js (Hono) 版 `storage_service` 迁移而来，**HTTP 接口保持不变**。

## 安装

### Homebrew（推荐）

```bash
brew tap fkcaikengren/jupi-d2c
brew install jupi-d2c
```

后续升级：

```bash
brew upgrade jupi-d2c
```

> 发布的二进制已内嵌前端 Web UI，安装后即可直接 `jupi-d2c` 运行。

### 从源码构建

需要 Go（见 `go.mod`）与 Node 22 / pnpm（构建内嵌前端）：

```bash
cd web && pnpm install && pnpm build && cd ..
go build -o jupi-d2c ./cmd/jupi-d2c
```

查看版本：`jupi-d2c --version`。

## 发布（维护者）

发布完全由 [GoReleaser](https://goreleaser.com) + GitHub Actions 驱动：推送一个 `vX.Y.Z` tag 即触发 `.github/workflows/release.yml`，自动构建前端、交叉编译 macOS/Linux（amd64 + arm64）二进制、创建 GitHub Release，并把更新后的 Homebrew Cask 提交到 [`fkcaikengren/homebrew-jupi-d2c`](https://github.com/fkcaikengren/homebrew-jupi-d2c)。

```bash
git tag v0.1.0
git push origin v0.1.0
```

**前置条件**：在 `jupi-d2c` 仓库 Settings → Secrets 添加 `HOMEBREW_TAP_GITHUB_TOKEN`——一个对 `homebrew-jupi-d2c` 有写权限的 GitHub PAT（仅需 `repo`/`contents` 权限）。默认 `GITHUB_TOKEN` 仅能写本仓库，无法跨仓库推送 formula。

本地校验配置：`goreleaser check`；本地试跑（不发布）：`goreleaser release --snapshot --clean`。

## 架构

代码按职责分三层放：

- `cmd/jupi-d2c` — 进程入口与 CLI（cobra）：裸命令前台运行，`start`/`stop`/`status` 以守护进程控制后台实例；加载配置、监听 SIGINT/SIGTERM。
- `internal/config` — 基于 `gopkg.in/yaml.v3` 的配置解析、校验与落盘；`config.yml` 为唯一来源（无环境变量 / `.env` 兜底）。
- `internal/api` — 单一 HTTP 服务（gin），所有路由挂在一个引擎上。
  - `router.go` — 引擎工厂 + 路由装配（`NewEngine` / `NewRouter`）。
  - `api/handlers` — HTTP 传输层：health / upload / config 处理器，解析请求、调用 service、按契约回写 JSON。
  - `api/services` — 领域逻辑：upload（落盘编排）、config（读写并校验 `config.yml`）。
  - `api/middleware` — 鉴权（`Bearer` / `PRIVATE-TOKEN`，恒定时间比较）、CORS 等横切中间件。
  - `api/webui` — `go:embed` 内嵌前端构建产物并提供 SPA 兜底（`NoRoute` 托管 `/`）。
- `internal/infra` — 可替换的 IO/异步后端。
  - `infra/queue` — 有界任务队列 + worker 池，支持优雅排空。
  - `infra/storage` — 写盘逻辑（唯一直接落盘处，换 S3/OSS/R2 只改这里）。
  - `infra/database` — 占位；未来扩展数据库服务时加在这里。
- `internal/daemon` — 组合「HTTP server + 池」，管理启动/关闭顺序。
- `web/` — 前端子项目（React 18 + Tailwind v4 + Vite，pnpm + Node 22）。

### 网关层：单一服务、单端口

作为本地桌面/CLI 工具，所有路由共用一个端口 `0.0.0.0:PORT`（默认 5678），一个 gin 引擎：

```
   /health            健康检查（公开）
   /api/upload        上传（Bearer token）
   /api/config        读/改配置（Bearer token）
   /uploads/*         上传目录映射为静态资源（公开，URL 即凭据）
   /*                 内嵌前端 SPA（控制面板）
            │
        daemon core
```

`/api/config` 因能改写配置（含 `token`、端口）需要 Bearer token；`/health`、`/uploads/*` 与 UI 资源公开访问。访问范围由部署者自行通过防火墙/NAT 控制。

上传请求被读入后提交到 worker 池，处理器阻塞等待结果 channel，从而在保持同步响应契约的同时，对磁盘写入做并发限制与背压。关闭顺序：信号 → `server.Shutdown`（停止接收、排空在途请求）→ `pool.Shutdown`（排空队列、等待 worker）。

> CORS 未使用 `gin-contrib/cors`：该库在 Origin 与请求 Host 同源时会提前返回、不写 `Access-Control-Allow-Origin`，破坏“总是返回 `*`”的契约。因此用一个无条件写 `*` 的手写中间件复刻 Hono `cors()` 的行为。

## 启动

```bash
# 先构建前端（产物内嵌进二进制；未构建时管理页显示占位提示）
cd web && pnpm install && pnpm build && cd ..

go run ./cmd/jupi-d2c
# 或编译为单一二进制
go build -o jupi-d2c ./cmd/jupi-d2c && ./jupi-d2c
```

无需任何环境变量或 `.env`：首次启动会在配置缺省落点自动生成 `config.yml` 并把随机 token 打到 stderr（详见「配置」）。

启动后：上传 API、文件访问与控制面板都在 `http://localhost:5678`（控制面板在 `/`）。

> `go build` 不依赖前端产物即可通过：`internal/api/webui/dist/.gitkeep` 作为 `go:embed` 锚点，配合 `//go:embed all:dist` 的 `all:` 前缀（裸 `//go:embed dist` 会跳过 `.` 开头文件、空目录时编译失败）。只有跑过 `pnpm build` 才会内嵌真实 UI。

### 守护进程控制

裸命令在前台运行（stdout 实时打印 HTTP 活动）；以下子命令以守护进程方式管理后台实例：

```bash
./jupi-d2c start    # 后台拉起；日志写入 jupi-d2c.log，PID 写入 jupi-d2c.pid
./jupi-d2c status   # 查看运行状态
./jupi-d2c stop     # 优雅关闭
```

通用 flag：`--config <path>`（配置文件路径）、`--pid-file <path>`；`--log-file <path>` 仅 `start` 可用。

`--pid-file` / `--log-file` 默认与配置文件同目录、与 `config.yml` 相邻：开发（存在 `./config.yml`）落进程工作目录；生产落 `~/.jupi-d2c/`（即 `~/.jupi-d2c/jupi-d2c.pid`、`~/.jupi-d2c/jupi-d2c.log`）。显式指定 flag 时以 flag 为准。

## 控制面板（Web UI）

`web/` 是独立前端子项目（React 18 + Tailwind CSS v4 + Vite，Node 22 / pnpm）。

```bash
cd web
pnpm install      # 依赖 esbuild 构建脚本已在 pnpm-workspace.yaml 放行
pnpm dev          # 开发服务器，/api 代理到 127.0.0.1:5678
pnpm build        # 产物输出到 ../internal/api/webui/dist，供 go:embed 打包
```

首次访问 UI 时，浏览器会要求输入访问 `token`（只存在 localStorage，不会上传）；之后所有 `/api/config` 请求自动带 `Authorization: Bearer <token>`。在 UI 上可查看/修改全部配置并保存到 `config.yml`。**修改在下次重启 daemon 后生效**——保存后若与运行中的实例不一致，页面会提示「需重启」。`token` 只写不回显。右上角「更换 token」可清除 localStorage 回到登录页。

## API

与 Node 版完全一致：

### `POST /api/upload`
鉴权（任选其一，恒定时间比较；`<token>` 即 `config.yml` 中的值）：`Authorization: Bearer <token>`、`Authorization: <token>` 或 `PRIVATE-TOKEN: <token>`。
请求体接受原始二进制或 `multipart/form-data`（字段名 `file`）。
可选 `tag`（multipart 表单字段或 query 参数）将文件归档到 `/uploads/<tag>/` 子目录。
响应：`{ "data": { "filename", "url", "size", "contentType" } }`。
错误：`401` 缺 auth / `403` token 不匹配 / `400` 空体或字段缺失 / `413` 超限 / `503` 队列不可用 / `500` 内部错误。

### `GET /uploads/:filename`
公开访问已上传文件。

### `GET /health`
`{ "ok": true, "time": "<ISO8601>", "maxFileSize": <int> }`。

## 配置

配置的**唯一来源是 `config.yml`**，无环境变量 / `.env` 兜底（管理 UI 也直接写入它）。

上手有两种方式，任选其一：

```bash
cp config.example.yml config.yml   # 复制示例后填好 token
# 或：直接运行二进制，首次启动自动生成默认 config.yml 并打印随机 token
```

路径解析优先级（显式优于约定）：

1. 显式 `--config <path>` flag；
2. `./config.yml`（存在时，相对进程工作目录）；
3. `~/.jupi-d2c/config.yml`（默认落点）。

> `config.yml` 含 token，已加入 `.gitignore`；纳入版本控制的是不含 token 的 `config.example.yml`。

| yml 键 | 默认 | 说明 |
|---|---|---|
| `port` | `5678` | 服务监听端口（上传 API / 配置面板 / 文件访问共用） |
| `token` | （必填） | 客户端 token，留空启动报错；自动生成时填入随机值 |
| `upload_dir` | `./uploads` | 落盘目录；相对路径锚定到配置文件所在目录 |
| `max_file_size` | `10485760` | 单文件最大字节 |
| `worker_count` | `4` | 持久化 worker 数 |
| `queue_size` | `64` | 队列容量 |

## 测试

```bash
go test ./... -race
```