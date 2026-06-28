# D2C-manager

D2C 插件图片上传后端的 Go 版本（daemon）。从 Node.js (Hono) 版 `storage_service` 迁移而来，**HTTP 接口保持不变**。

## 架构

代码按职责分三层放：

- `cmd/d2c-manager` — 进程入口，加载配置、监听 SIGINT/SIGTERM。
- `internal/config` — 基于 Viper 的配置解析、校验与落盘（config.yml）。
- `internal/api` — 所有 HTTP 入口（对外 + 本地面板）。
  - `api/public` — 对外 Gin 路由与处理器（POST /api/upload、GET /uploads/*、/health）。
  - `api/ui` — 本地控制面板：配置 REST API + 托管内嵌前端。`/api/config` 因能改写配置（含 token 与端口）需要 Bearer token；UI 资源与 `/api/health` 公开访问。
  - `api/ui/webui` — `go:embed` 内嵌前端构建产物并提供 SPA 兜底。
  - `api/middleware` — 鉴权（Bearer / PRIVATE-TOKEN）、CORS 等横切中间件。
- `internal/infra` — 可替换的 IO/异步后端。
  - `infra/queue` — 有界任务队列 + worker 池，支持优雅排空。
  - `infra/storage` — 写盘逻辑（唯一直接落盘处，换 S3/OSS/R2 只改这里）。
  - `infra/database` — 占位；未来扩展数据库服务时加在这里。
- `internal/daemon` — 组合「对外 server + 面板 server + 池」，管理启动/关闭顺序。
- `web/` — 前端子项目（React 18 + Tailwind v4 + Vite，pnpm + Node 22）。

### 网关层：两个监听器

```
   对外 HTTP API               本地控制面板
   (0.0.0.0:PORT)              (0.0.0.0:ADMIN_PORT)
          │                            │
          └──────────────┬─────────────┘
                     daemon core
```

对外上传 API 监听 `0.0.0.0:PORT`，行为与旧版一致。本地控制面板监听 `0.0.0.0:ADMIN_PORT`（默认 3001），公开访问：UI 资源与 `/api/health` 任何人可用；`/api/config`（GET/PUT）需要 Bearer token——它能改写 `STORAGE_TOKEN`、端口等关键配置，所以必须持有 token 才允许。访问范围由部署者自行通过防火墙/NAT 控制。

上传请求被读入后提交到 worker 池，处理器阻塞等待结果 channel，从而在保持同步响应契约的同时，对磁盘写入做并发限制与背压。关闭顺序：信号 → 两个 `server.Shutdown`（停止接收、排空在途请求）→ `pool.Shutdown`（排空队列、等待 worker）。

> CORS 未使用 `gin-contrib/cors`：该库在 Origin 与请求 Host 同源时会提前返回、不写 `Access-Control-Allow-Origin`，破坏“总是返回 `*`”的契约。因此用一个无条件写 `*` 的手写中间件复刻 Hono `cors()` 的行为。

## 启动

```bash
cp .env.example .env   # 把 STORAGE_TOKEN 改成长随机串

# 先构建前端（产物内嵌进二进制；未构建时管理页显示占位提示）
cd web && pnpm install && pnpm build && cd ..

go run ./cmd/d2c-manager
# 或编译为单一二进制
go build -o d2c-manager ./cmd/d2c-manager && ./d2c-manager
```

启动后：对外 API 在 `http://localhost:3000`，本地控制面板在 `http://localhost:3001`。

> `go build` 不依赖前端产物即可通过（`internal/api/ui/webui/dist/.gitkeep` 作为 `go:embed` 锚点）；只有跑过 `pnpm build` 才会内嵌真实 UI。

## 控制面板（Web UI）

`web/` 是独立前端子项目（React 18 + Tailwind CSS v4 + Vite，Node 22 / pnpm）。

```bash
cd web
pnpm install      # 依赖 esbuild 构建脚本已在 pnpm-workspace.yaml 放行
pnpm dev          # 开发服务器，/api 代理到 127.0.0.1:3001
pnpm build        # 产物输出到 ../internal/api/ui/webui/dist，供 go:embed 打包
```

首次访问 UI 时，浏览器会要求输入 `STORAGE_TOKEN`（只存在 localStorage，不会上传）；之后所有 `/api/config` 请求自动带 `Authorization: Bearer <token>`。在 UI 上可查看/修改全部配置并保存到 `config.yml`。**修改在下次重启 daemon 后生效**——保存后若与运行中的实例不一致，页面会提示「需重启」。`STORAGE_TOKEN` 只写不回显；`ADMIN_PORT` 只读（避免把自己锁在外）。右上角「更换 token」可清除 localStorage 回到登录页。

## API

与 Node 版完全一致：

### `POST /api/upload`
鉴权（任选其一）：`Authorization: Bearer <STORAGE_TOKEN>` 或 `PRIVATE-TOKEN: <STORAGE_TOKEN>`。
请求体接受原始二进制或 `multipart/form-data`（字段名 `file`）。
响应：`{ "data": { "url", "filename", "size", "contentType" } }`。
错误：`401` 缺 auth / `403` token 不匹配 / `400` 空体或字段缺失 / `413` 超限 / `500` 内部错误。

### `GET /uploads/:filename`
公开访问已上传文件。

### `GET /health`
`{ "ok": true, "time": "<ISO8601>", "maxFileSize": <int> }`。

## 配置

配置由 Viper 统一管理，**优先级：`config.yml` > 环境变量(/.env) > 硬编码默认值**。
`config.yml` 是最终来源（管理 UI 写入它），环境变量仅作为首次启动的 bootstrap 默认值。
路径默认 `./config.yml`，可用 `CONFIG_FILE` 覆盖。`config.yml` 含 token，已加入 `.gitignore`。

| 变量 / yml 键 | 默认 | 说明 |
|---|---|---|
| `PORT` / `port` | `3000` | 对外 API 监听端口 |
| `ADMIN_PORT` / `admin_port` | `3001` | 本地控制面板端口（公开监听 `0.0.0.0`，由部署者控制访问范围） |
| `STORAGE_TOKEN` / `token` | （必填） | 客户端 token |
| `UPLOAD_DIR` / `upload_dir` | `./uploads` | 落盘目录 |
| `PUBLIC_BASE_URL` / `public_base_url` | `http://localhost:3000` | URL 拼接基址（末尾斜杠会被去掉） |
| `MAX_FILE_SIZE` / `max_file_size` | `10485760` | 单文件最大字节 |
| `WORKER_COUNT` / `worker_count` | `4` | 持久化 worker 数 |
| `QUEUE_SIZE` / `queue_size` | `64` | 队列容量 |

## 测试

```bash
go test ./... -race
```