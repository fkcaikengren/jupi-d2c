package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"d2c-manager/internal/api/public"
	"d2c-manager/internal/api/ui"
	"d2c-manager/internal/config"
	"d2c-manager/internal/infra/queue"
	"d2c-manager/internal/infra/storage"
)

const shutdownTimeout = 30 * time.Second

// Daemon 组合公开 HTTP server、本地面板 server 与 worker 池，
// 并管理它们的启动/关闭顺序。
type Daemon struct {
	cfg          config.AppConfig
	configPath   string
	pool         *queue.Pool
	publicServer *http.Server
	uiServer     *http.Server
	publicLn     net.Listener
	uiLn         net.Listener
}

// New 校验环境、确保上传目录存在、提前绑定两个监听器，并接线两个 server + 池。
// 监听器在此处（而非 Run）绑定：端口冲突快速失败，且实际地址在并发启动前即确定，
// 测试可安全读取（Port:0 时由内核分配）。configPath 传给面板 API 用于读写 config.yml。
func New(cfg config.AppConfig, configPath string) (*Daemon, error) {
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}

	publicLn, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port)) // 0.0.0.0:PORT 对外
	if err != nil {
		return nil, fmt.Errorf("绑定公开监听器 :%d 失败: %w", cfg.Port, err)
	}
	uiLn, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.AdminPort)) // 0.0.0.0:AdminPort 控制面板
	if err != nil {
		publicLn.Close()
		return nil, fmt.Errorf("绑定面板监听器 :%d 失败: %w", cfg.AdminPort, err)
	}

	pool := queue.NewPool(cfg.WorkerCount, cfg.QueueSize, storage.SaveBytes)
	return &Daemon{
		cfg:          cfg,
		configPath:   configPath,
		pool:         pool,
		publicServer: &http.Server{Handler: public.NewRouter(cfg, pool)},
		uiServer:     &http.Server{Handler: ui.NewRouter(cfg, configPath)},
		publicLn:     publicLn,
		uiLn:         uiLn,
	}, nil
}

// Run 启动池与两个 HTTP server，阻塞直到 ctx 取消或某个 server 出错，然后优雅关闭。
func (d *Daemon) Run(ctx context.Context) error {
	d.pool.Start()

	errCh := make(chan error, 2)
	go func() {
		log.Printf("[d2c-manager] public listening on %s", d.publicLn.Addr())
		log.Printf("[d2c-manager] upload dir: %s", d.cfg.UploadDir)
		log.Printf("[d2c-manager] public base: %s", d.cfg.PublicBaseURL)
		if err := d.publicServer.Serve(d.publicLn); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		log.Printf("[d2c-manager] panel  listening on %s", d.uiLn.Addr())
		if err := d.uiServer.Serve(d.uiLn); err != nil && err != http.ErrServerClosed {
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

// PublicAddr / AdminAddr 暴露实际绑定地址（New 之后即可用）。
func (d *Daemon) PublicAddr() net.Addr { return d.publicLn.Addr() }
func (d *Daemon) AdminAddr() net.Addr  { return d.uiLn.Addr() }

// gracefulStop 先停两个 HTTP server（排空在途请求、停止接收），再停池
// （排空队列、等 worker）。这个顺序保证池关闭时不会有 handler 还在 Submit。
func (d *Daemon) gracefulStop() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := d.publicServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[d2c-manager] public shutdown error: %v", err)
	}
	if err := d.uiServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[d2c-manager] panel shutdown error: %v", err)
	}
	if err := d.pool.Shutdown(shutdownCtx); err != nil {
		log.Printf("[d2c-manager] pool shutdown error: %v", err)
		return err
	}
	log.Printf("[d2c-manager] stopped cleanly")
	return nil
}