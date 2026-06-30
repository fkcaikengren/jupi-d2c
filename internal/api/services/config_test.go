package services

import (
	"path/filepath"
	"testing"

	"jupi-d2c/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleConfig(dir string) config.AppConfig {
	return config.AppConfig{
		Port:        5678,
		Token:       "secret",
		UploadDir:   dir,
		DBPath:      filepath.Join(dir, "jupi-d2c.db"),
		MaxFileSize: 1024,
		WorkerCount: 2,
		QueueSize:   8,
	}
}

// Current 读不到磁盘文件时回退到启动快照。
func TestConfigService_CurrentFallsBackToRunning(t *testing.T) {
	running := sampleConfig(t.TempDir())
	s := NewConfigService(filepath.Join(t.TempDir(), "missing.yml"), running)
	assert.Equal(t, running, s.Current())
}

// Current 读得到磁盘文件时返回磁盘内容。
func TestConfigService_CurrentReadsDisk(t *testing.T) {
	running := sampleConfig(t.TempDir())
	path := filepath.Join(t.TempDir(), "config.yml")

	onDisk := running
	onDisk.Port = 4000
	require.NoError(t, config.Save(path, onDisk))

	s := NewConfigService(path, running)
	assert.Equal(t, 4000, s.Current().Port)
}

// RestartRequired：相等不需要重启，任一影响运行的字段变化即需要。
func TestConfigService_RestartRequired(t *testing.T) {
	running := sampleConfig(t.TempDir())
	s := NewConfigService("", running)

	assert.False(t, s.RestartRequired(running))

	changed := running
	changed.Port = 4000
	assert.True(t, s.RestartRequired(changed))
}

// Save 内含校验：非法配置不落盘并返回错误。
func TestConfigService_SaveRejectsInvalid(t *testing.T) {
	running := sampleConfig(t.TempDir())
	path := filepath.Join(t.TempDir(), "config.yml")
	s := NewConfigService(path, running)

	bad := running
	bad.Port = 0
	assert.Error(t, s.Save(bad))
}
