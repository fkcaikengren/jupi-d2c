# D2C-manager

D2C 插件图片上传后端的 Go 版本（daemon）。从 Node.js (Hono) 版 `storage_service` 迁移而来，**HTTP 接口保持不变**。

## 架构

- `cmd/d2c-manager` — 进程入口，加载配置、监听 SIGINT/SIGTERM。
- `internal/config` — 环境变量解析与校验。
- `internal/storage` — 写盘逻辑（唯一直接落盘处，换 S3/OSS/R2 只改这里）。
- `internal/auth` — Bearer / PRIVATE-TOKEN 鉴权中间件。
- `internal/queue` — 有界任务队列 + worker 池，支持优雅排空。
- `internal/httpapi` — Gin 路由与处理器（接口形状），含手写 CORS 中间件。
- `internal/daemon` — 组合 server + 池，管理启动/关闭顺序。

上传请求被读入后提交到 worker 池，处理器阻塞等待结果 channel，从而在保持同步响应契约的同时，对磁盘写入做并发限制与背压。关闭顺序：信号 → `server.Shutdown`（停止接收、排空在途请求）→ `pool.Shutdown`（排空队列、等待 worker）。

> CORS 未使用 `gin-contrib/cors`：该库在 Origin 与请求 Host 同源时会提前返回、不写 `Access-Control-Allow-Origin`，破坏“总是返回 `*`”的契约。因此用一个无条件写 `*` 的手写中间件复刻 Hono `cors()` 的行为。

## 启动

```bash
cp .env.example .env   # 把 STORAGE_TOKEN 改成长随机串
go run ./cmd/d2c-manager
# 或编译
go build -o d2c-manager ./cmd/d2c-manager && ./d2c-manager
```

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

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | `3000` | 监听端口 |
| `STORAGE_TOKEN` | （必填） | 客户端 token |
| `UPLOAD_DIR` | `./uploads` | 落盘目录 |
| `PUBLIC_BASE_URL` | `http://localhost:3000` | URL 拼接基址（末尾斜杠会被去掉） |
| `MAX_FILE_SIZE` | `10485760` | 单文件最大字节 |
| `WORKER_COUNT` | `4` | 持久化 worker 数（新增） |
| `QUEUE_SIZE` | `64` | 队列容量（新增） |

## 测试

```bash
go test ./... -race
```
