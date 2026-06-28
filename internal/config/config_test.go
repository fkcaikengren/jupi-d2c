package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// missingPath 返回一个临时目录下尚不存在的 config.yml 路径。
func missingPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "config.yml")
}

func TestLoad_RequiresToken(t *testing.T) {
	os.Clearenv()
	_, err := LoadFromPath(missingPath(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STORAGE_TOKEN")
}

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("STORAGE_TOKEN", "secret")
	cfg, err := LoadFromPath(missingPath(t))
	require.NoError(t, err)
	assert.Equal(t, 3000, cfg.Port)
	assert.Equal(t, 3001, cfg.AdminPort)
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
	cfg, err := LoadFromPath(missingPath(t))
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com", cfg.PublicBaseURL)
}

func TestLoad_RejectsBadNumber(t *testing.T) {
	os.Clearenv()
	os.Setenv("STORAGE_TOKEN", "secret")
	os.Setenv("MAX_FILE_SIZE", "-5")
	_, err := LoadFromPath(missingPath(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MAX_FILE_SIZE")
}

// config.yml 的值覆盖环境变量（yml 是最终来源）。
func TestLoad_Precedence_ConfigOverEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("STORAGE_TOKEN", "env-token")
	os.Setenv("PORT", "3000")

	path := missingPath(t)
	require.NoError(t, Save(path, AppConfig{
		Port: 4000, AdminPort: 4001, Token: "file-token",
		UploadDir: "./uploads", PublicBaseURL: "http://localhost:4000",
		MaxFileSize: 2048, WorkerCount: 2, QueueSize: 8,
	}))

	cfg, err := LoadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, 4000, cfg.Port, "yml 应覆盖 env PORT")
	assert.Equal(t, "file-token", cfg.Token, "yml 应覆盖 env STORAGE_TOKEN")
}

// 没有 yml key 时，环境变量覆盖硬编码默认值。
func TestLoad_Precedence_EnvOverDefault(t *testing.T) {
	os.Clearenv()
	os.Setenv("STORAGE_TOKEN", "secret")
	os.Setenv("WORKER_COUNT", "9")
	cfg, err := LoadFromPath(missingPath(t))
	require.NoError(t, err)
	assert.Equal(t, 9, cfg.WorkerCount)
}

func TestValidate_Errors(t *testing.T) {
	base := AppConfig{
		Port: 3000, AdminPort: 3001, Token: "secret",
		UploadDir: "./uploads", PublicBaseURL: "http://localhost",
		MaxFileSize: 1024, WorkerCount: 1, QueueSize: 1,
	}
	require.NoError(t, Validate(base))

	cases := map[string]func(c *AppConfig){
		"空 token":      func(c *AppConfig) { c.Token = "" },
		"port 越界":      func(c *AppConfig) { c.Port = 0 },
		"adminPort 越界": func(c *AppConfig) { c.AdminPort = 70000 },
		"端口相同":         func(c *AppConfig) { c.AdminPort = c.Port },
		"maxFileSize":  func(c *AppConfig) { c.MaxFileSize = 0 },
		"workerCount":  func(c *AppConfig) { c.WorkerCount = 0 },
		"queueSize":    func(c *AppConfig) { c.QueueSize = 0 },
		"空 uploadDir":  func(c *AppConfig) { c.UploadDir = "" },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			c := base
			mut(&c)
			assert.Error(t, Validate(c))
		})
	}
}

func TestSave_RoundTrip(t *testing.T) {
	os.Clearenv()
	path := missingPath(t)
	want := AppConfig{
		Port: 8080, AdminPort: 8081, Token: "rt-token",
		UploadDir: "./data", PublicBaseURL: "https://cdn.example.com",
		MaxFileSize: 5 * 1024 * 1024, WorkerCount: 3, QueueSize: 32,
	}
	require.NoError(t, Save(path, want))
	assert.FileExists(t, path)

	got, err := LoadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestSave_NoLeftoverTempFiles(t *testing.T) {
	os.Clearenv()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	require.NoError(t, Save(path, AppConfig{
		Port: 3000, AdminPort: 3001, Token: "secret",
		UploadDir: "./uploads", PublicBaseURL: "http://localhost",
		MaxFileSize: 1024, WorkerCount: 1, QueueSize: 1,
	}))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "应只剩 config.yml，无残留 .tmp")
}

func TestSave_RejectsInvalid(t *testing.T) {
	os.Clearenv()
	path := missingPath(t)
	err := Save(path, AppConfig{Port: 0}) // 非法
	require.Error(t, err)
	assert.NoFileExists(t, path, "校验失败不应写文件")
}
