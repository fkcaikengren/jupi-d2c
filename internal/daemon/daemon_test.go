package daemon

import (
	"context"
	"encoding/json"
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

// TestDaemon_ServesAPIAndUploads 验证单端口上同时提供上传 API、配置 API 与健康检查。
func TestDaemon_ServesAPIAndUploads(t *testing.T) {
	cfg := testCfg(t)
	cfgPath := filepath.Join(t.TempDir(), "config.yml")
	d, err := New(cfg, cfgPath)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- d.Run(ctx) }()
	time.Sleep(150 * time.Millisecond)

	port := d.Addr().(*net.TCPAddr).Port
	base := fmt.Sprintf("http://127.0.0.1:%d", port)

	// /health 公开。
	resp, err := http.Get(base + "/health")
	require.NoError(t, err)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// /api/config 需要 token，且 maxFileSize 等字段可读到。
	req, _ := http.NewRequest(http.MethodGet, base+"/api/config", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	var body struct {
		Config struct {
			MaxFileSize int64 `json:"maxFileSize"`
		} `json:"config"`
	}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&body))
	assert.Equal(t, int64(1024), body.Config.MaxFileSize)

	cancel()
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not shut down in time")
	}
}

func TestDaemon_PortCollision(t *testing.T) {
	// 先占用一个端口（监听全部接口）制造冲突。
	// 必须用 ":0" 而不是 "127.0.0.1:0"：在 macOS/BSD 上 0.0.0.0:port 与
	// 127.0.0.1:port 不冲突；daemon 监听 :port (0.0.0.0)，所以占用也得是 0.0.0.0。
	occupied, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port

	cfg := testCfg(t)
	cfg.Port = port
	_, err = New(cfg, filepath.Join(t.TempDir(), "config.yml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "绑定监听器")
}
