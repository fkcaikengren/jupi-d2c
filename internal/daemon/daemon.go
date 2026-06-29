package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"jupi-d2c/internal/api"
	"jupi-d2c/internal/config"
	"jupi-d2c/internal/infra/database"
	"jupi-d2c/internal/infra/queue"
	"jupi-d2c/internal/infra/storage"
)

const shutdownTimeout = 30 * time.Second

// Daemon 组合单一 HTTP server 与 worker 池，并管理它们的启动/关闭顺序。
type Daemon struct {
	cfg        config.AppConfig
	configPath string
	pool       *queue.Pool
	db         *sql.DB
	server     *http.Server
	ln         net.Listener
}

// New 校验环境、确保上传目录存在、提前绑定监听器，并接线 server + 池。
// 监听器在此处（而非 Run）绑定：端口冲突快速失败，且实际地址在启动前即确定，
// 测试可安全读取（Port:0 时由内核分配）。configPath 传给配置 API 用于读写 config.yml。
func New(cfg config.AppConfig, configPath string) (*Daemon, error) {
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port)) // 0.0.0.0:PORT
	if err != nil {
		return nil, fmt.Errorf("绑定监听器 :%d 失败: %w", cfg.Port, err)
	}

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		ln.Close()
		return nil, err
	}

	pool := queue.NewPool(cfg.WorkerCount, cfg.QueueSize, storage.SaveBytes)
	return &Daemon{
		cfg:        cfg,
		configPath: configPath,
		pool:       pool,
		db:         db,
		server:     &http.Server{Handler: api.NewRouter(cfg, pool, configPath, db)},
		ln:         ln,
	}, nil
}

// Run 启动池与 HTTP server，阻塞直到 ctx 取消或 server 出错，然后优雅关闭。
func (d *Daemon) Run(ctx context.Context) error {
	d.pool.Start()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("[jupi-d2c] listening on %s", d.ln.Addr())
		log.Printf("[jupi-d2c] upload dir: %s", d.cfg.UploadDir)
		if err := d.server.Serve(d.ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		d.gracefulStop()
		return err
	case <-ctx.Done():
		log.Printf("[jupi-d2c] shutdown signal received")
		return d.gracefulStop()
	}
}

// Addr 暴露实际绑定地址（New 之后即可用）。
func (d *Daemon) Addr() net.Addr { return d.ln.Addr() }

// gracefulStop 先停 HTTP server（排空在途请求、停止接收），再停池
// （排空队列、等 worker）。这个顺序保证池关闭时不会有 handler 还在 Submit。
func (d *Daemon) gracefulStop() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := d.server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[jupi-d2c] server shutdown error: %v", err)
	}
	if err := d.pool.Shutdown(shutdownCtx); err != nil {
		log.Printf("[jupi-d2c] pool shutdown error: %v", err)
		return err
	}
	// 池停妥后再关数据库：此时已无 handler 会再访问 db。
	if err := d.db.Close(); err != nil {
		log.Printf("[jupi-d2c] db close error: %v", err)
	}
	log.Printf("[jupi-d2c] stopped cleanly")
	return nil
}
