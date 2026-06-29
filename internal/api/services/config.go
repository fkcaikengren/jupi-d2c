// Package services 放置与传输层（gin/HTTP）无关的领域逻辑：
// 配置的读取/落盘/重启判定、上传任务的提交。handlers 层依赖本包，本包不反向依赖。
package services

import "jupi-d2c/internal/config"

// ConfigService 负责 config.yml 的读取、落盘与"是否需要重启"的判定。
// running 是 daemon 启动期的配置快照，用于和磁盘配置比对。
type ConfigService struct {
	path    string
	running config.AppConfig
}

// NewConfigService 绑定 config.yml 路径与启动期快照。
func NewConfigService(path string, running config.AppConfig) *ConfigService {
	return &ConfigService{path: path, running: running}
}

// Current 返回磁盘上的当前配置；读不到或校验失败时回退到启动快照，
// 保证面板永远能展示一份可用配置。
func (s *ConfigService) Current() config.AppConfig {
	c, err := config.LoadFromPath(s.path)
	if err != nil {
		return s.running
	}
	return c
}

// Save 校验并原子写回 config.yml（config.Save 内含 Validate，非法配置不落盘）。
func (s *ConfigService) Save(next config.AppConfig) error {
	return config.Save(s.path, next)
}

// RestartRequired 判定给定配置与运行实例是否有任一影响运行的字段不同。
// 任一不同即意味着 config.yml 已与运行实例脱节，需重启 daemon 才能生效。
func (s *ConfigService) RestartRequired(c config.AppConfig) bool {
	r := s.running
	return c.Port != r.Port ||
		c.Token != r.Token ||
		c.UploadDir != r.UploadDir ||
		c.DBPath != r.DBPath ||
		c.MaxFileSize != r.MaxFileSize ||
		c.WorkerCount != r.WorkerCount ||
		c.QueueSize != r.QueueSize
}
