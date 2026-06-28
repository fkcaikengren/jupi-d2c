package daemon

import (
	"context"
	"testing"
	"time"

	"d2c-manager/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemon_StartAndGracefulShutdown(t *testing.T) {
	cfg := config.AppConfig{
		Port:          0, // ":0" → 内核分配空闲端口
		Token:         "secret",
		UploadDir:     t.TempDir(),
		PublicBaseURL: "http://localhost",
		MaxFileSize:   1024,
		WorkerCount:   1,
		QueueSize:     4,
	}
	d, err := New(cfg)
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
	cfg := config.AppConfig{
		Port: 0, Token: "secret", UploadDir: dir,
		PublicBaseURL: "http://localhost", MaxFileSize: 1024,
		WorkerCount: 1, QueueSize: 1,
	}
	_, err := New(cfg)
	require.NoError(t, err)
	assert.DirExists(t, dir)
}
