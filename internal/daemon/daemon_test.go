package daemon

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"d2c-manager/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCfg(t *testing.T) config.AppConfig {
	t.Helper()
	return config.AppConfig{
		Port:          0, // ":0" → 内核分配空闲端口
		AdminPort:     0,
		Token:         "secret",
		UploadDir:     t.TempDir(),
		PublicBaseURL: "http://localhost",
		MaxFileSize:   1024,
		WorkerCount:   1,
		QueueSize:     4,
	}
}

func TestDaemon_StartAndGracefulShutdown(t *testing.T) {
	d, err := New(testCfg(t), filepath.Join(t.TempDir(), "config.yml"))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()

	time.Sleep(150 * time.Millisecond) // 让 server 起来
	cancel()                           // 模拟收到关闭信号

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not shut down in time")
	}
}

func TestDaemon_New_CreatesUploadDir(t *testing.T) {
	dir := t.TempDir() + "/nested/uploads"
	cfg := testCfg(t)
	cfg.UploadDir = dir
	_, err := New(cfg, filepath.Join(t.TempDir(), "config.yml"))
	require.NoError(t, err)
	assert.DirExists(t, dir)
}

func TestDaemon_BothListenersServe(t *testing.T) {
	d, err := New(testCfg(t), filepath.Join(t.TempDir(), "config.yml"))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(150 * time.Millisecond)

	// 公开 server 的 /health。
	publicURL := fmt.Sprintf("http://127.0.0.1:%d/health", d.PublicAddr().(*net.TCPAddr).Port)
	resp, err := http.Get(publicURL)
	require.NoError(t, err)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// 面板 server 的 /api/health（公开端点，验证 listener 暴露正确）。
	adminURL := fmt.Sprintf("http://127.0.0.1:%d/api/health", d.AdminAddr().(*net.TCPAddr).Port)
	resp2, err := http.Get(adminURL)
	require.NoError(t, err)
	io.Copy(io.Discard, resp2.Body)
	resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	cancel()
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not shut down in time")
	}
}

func TestDaemon_AdminPortCollision(t *testing.T) {
	// 先占用一个端口（监听全部接口），作为 AdminPort 制造冲突。
	// 必须用 ":0" 而不是 "127.0.0.1:0"：在 macOS/BSD 上 0.0.0.0:port 与
	// 127.0.0.1:port 不冲突；daemon 监听 :port (0.0.0.0)，所以占用也得是 0.0.0.0。
	occupied, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port

	cfg := testCfg(t)
	cfg.AdminPort = port
	_, err = New(cfg, filepath.Join(t.TempDir(), "config.yml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "面板监听器")
}
