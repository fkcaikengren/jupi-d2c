// Package handlers 是 HTTP 传输层：解析 gin 请求、调用 services、按契约回写 JSON。
// 它依赖 services（领域逻辑）与 config/storage（数据形状），自身不含业务规则。
package handlers

import (
	"jupi-d2c/internal/api/services"
	"jupi-d2c/internal/config"
	"jupi-d2c/internal/infra/queue"
)

// Handlers 持有处理请求所需的依赖：启动期配置快照与各 service。
// cfg 用于鉴权外的运行期参数（maxFileSize、UploadDir）；service 封装池与配置读写。
type Handlers struct {
	cfg     config.AppConfig
	uploads *services.UploadService
	configs *services.ConfigService
}

// New 用启动期快照、worker 池与 config.yml 路径装配各 service。
func New(cfg config.AppConfig, pool *queue.Pool, configPath string) *Handlers {
	return &Handlers{
		cfg:     cfg,
		uploads: services.NewUploadService(pool),
		configs: services.NewConfigService(configPath, cfg),
	}
}
