package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_RequiresToken(t *testing.T) {
	os.Clearenv()
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STORAGE_TOKEN")
}

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("STORAGE_TOKEN", "secret")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 3000, cfg.Port)
	assert.Equal(t, "secret", cfg.Token)
	assert.Equal(t, "./uploads", cfg.UploadDir)
	assert.Equal(t, "http://localhost:3000", cfg.PublicBaseURL)
	assert.Equal(t, int64(10*1024*1024), cfg.MaxFileSize)
	assert.Equal(t, 4, cfg.WorkerCount)
	assert.Equal(t, 64, cfg.QueueSize)
}

func TestLoad_TrimsTrailingSlash(t *testing.T) {
	os.Clearenv()
	os.Setenv("STORAGE_TOKEN", "secret")
	os.Setenv("PUBLIC_BASE_URL", "https://cdn.example.com///")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com", cfg.PublicBaseURL)
}

func TestLoad_RejectsBadNumber(t *testing.T) {
	os.Clearenv()
	os.Setenv("STORAGE_TOKEN", "secret")
	os.Setenv("MAX_FILE_SIZE", "-5")
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MAX_FILE_SIZE")
}
