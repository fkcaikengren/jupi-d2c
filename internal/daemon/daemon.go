package daemon

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"d2c-manager/internal/config"
	"d2c-manager/internal/httpapi"
	"d2c-manager/internal/queue"
	"d2c-manager/internal/storage"
)

const shutdownTimeout = 30 * time.Second

// Daemon 组合 HTTP server 与 worker 池，并管理它们的启动/关闭顺序。
type Daemon struct {
	cfg    config.AppConfig
	pool   *queue.Pool
	server *http.Server
}

// New 校验环境、确保上传目录存在，并接线 server + 池。
func New(cfg config.AppConfig) (*Daemon, error) {
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}
	pool := queue.NewPool(cfg.WorkerCount, cfg.QueueSize, storage.SaveBytes)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: httpapi.NewRouter(cfg, pool),
	}
	return &Daemon{cfg: cfg, pool: pool, server: server}, nil
}

// Run 启动池与 HTTP server，阻塞直到 ctx 取消或 server 出错，然后优雅关闭。
func (d *Daemon) Run(ctx context.Context) error {
	d.pool.Start()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("[d2c-manager] listening on :%d", d.cfg.Port)
		log.Printf("[d2c-manager] upload dir: %s", d.cfg.UploadDir)
		log.Printf("[d2c-manager] public base: %s", d.cfg.PublicBaseURL)
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		d.gracefulStop()
		return err
	case <-ctx.Done():
		log.Printf("[d2c-manager] shutdown signal received")
		return d.gracefulStop()
	}
}

// gracefulStop 先停 HTTP（排空在途请求、停止接收），再停池（排空队列、等 worker）。
// 这个顺序保证池关闭时不会有 handler 还在 Submit。
func (d *Daemon) gracefulStop() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := d.server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[d2c-manager] http shutdown error: %v", err)
	}
	if err := d.pool.Shutdown(shutdownCtx); err != nil {
		log.Printf("[d2c-manager] pool shutdown error: %v", err)
		return err
	}
	log.Printf("[d2c-manager] stopped cleanly")
	return nil
}
