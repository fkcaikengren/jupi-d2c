package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// defaultConfigPath 是未显式指定 CONFIG_FILE 时的配置文件路径（cwd）。
const defaultConfigPath = "./config.yml"

// AppConfig 是启动期解析并校验后的配置。
// 解析优先级：config.yml > 环境变量(/.env) > 硬编码默认值。
type AppConfig struct {
	Port          int
	Token         string
	UploadDir     string
	PublicBaseURL string
	MaxFileSize   int64
	WorkerCount   int
	QueueSize     int
}

// envSeeder 把环境变量读成 viper 的“默认值”。环境变量只作为 bootstrap 默认，
// 优先级低于 config.yml。present-but-invalid 的数字仍是硬错误（记录首个错误）。
type envSeeder struct {
	err error
}

func (s *envSeeder) str(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func (s *envSeeder) int64(name string, fallback int64) int64 {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		if s.err == nil {
			s.err = fmt.Errorf("环境变量 %s 不是合法正数: %s", name, raw)
		}
		return fallback
	}
	return n
}

// ResolvePath 解析配置文件路径：显式参数 > CONFIG_FILE 环境变量 > 默认 ./config.yml。
func ResolvePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if p := os.Getenv("CONFIG_FILE"); p != "" {
		return p
	}
	return defaultConfigPath
}

// Load 解析默认路径（CONFIG_FILE / ./config.yml）的配置，并返回最终路径，
// 以便 daemon / 面板 API 知道往哪里写回。
func Load() (AppConfig, string, error) {
	path := ResolvePath("")
	cfg, err := LoadFromPath(path)
	return cfg, path, err
}

// LoadFromPath 从指定 config.yml 路径解析配置；文件不存在不是错误（首次启动）。
func LoadFromPath(path string) (AppConfig, error) {
	_ = godotenv.Load() // 仅把 .env 注入进程环境；不存在时静默忽略

	s := &envSeeder{}
	v := viper.New()
	v.SetConfigType("yaml")
	if path != "" {
		v.SetConfigFile(path)
	}

	// 用环境变量(或硬编码默认)播种 viper 默认值——这是最低优先级，会被 config.yml 覆盖。
	v.SetDefault("port", s.int64("PORT", 3000))
	v.SetDefault("token", s.str("STORAGE_TOKEN", ""))
	v.SetDefault("upload_dir", s.str("UPLOAD_DIR", "./uploads"))
	v.SetDefault("public_base_url", s.str("PUBLIC_BASE_URL", "http://localhost:3000"))
	v.SetDefault("max_file_size", s.int64("MAX_FILE_SIZE", 10*1024*1024))
	v.SetDefault("worker_count", s.int64("WORKER_COUNT", 4))
	v.SetDefault("queue_size", s.int64("QUEUE_SIZE", 64))
	if s.err != nil {
		return AppConfig{}, s.err
	}

	if path != "" {
		if err := v.ReadInConfig(); err != nil {
			var nf viper.ConfigFileNotFoundError
			if !errors.As(err, &nf) && !os.IsNotExist(err) {
				return AppConfig{}, fmt.Errorf("读取配置文件失败: %w", err)
			}
			// 文件不存在：使用 env/默认值继续。
		}
	}

	cfg := AppConfig{
		Port:          v.GetInt("port"),
		Token:         v.GetString("token"),
		UploadDir:     v.GetString("upload_dir"),
		PublicBaseURL: strings.TrimRight(v.GetString("public_base_url"), "/"),
		MaxFileSize:   v.GetInt64("max_file_size"),
		WorkerCount:   v.GetInt("worker_count"),
		QueueSize:     v.GetInt("queue_size"),
	}
	if err := Validate(cfg); err != nil {
		return AppConfig{}, err
	}
	return cfg, nil
}

// Validate 校验配置取值，供 Load 与面板 PUT 共用，确保永不写入非法配置。
func Validate(c AppConfig) error {
	if c.Token == "" {
		return errors.New("缺少 STORAGE_TOKEN / token 配置")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("PORT 超出范围 (1-65535): %d", c.Port)
	}
	if c.MaxFileSize <= 0 {
		return fmt.Errorf("MAX_FILE_SIZE 必须为正数: %d", c.MaxFileSize)
	}
	if c.WorkerCount < 1 {
		return fmt.Errorf("WORKER_COUNT 必须 >= 1: %d", c.WorkerCount)
	}
	if c.QueueSize < 1 {
		return fmt.Errorf("QUEUE_SIZE 必须 >= 1: %d", c.QueueSize)
	}
	if c.UploadDir == "" {
		return errors.New("UPLOAD_DIR 不能为空")
	}
	return nil
}

// yamlConfig 是 config.yml 的落盘形状（snake_case，与 viper key 一致）。
type yamlConfig struct {
	Port          int    `yaml:"port"`
	Token         string `yaml:"token"`
	UploadDir     string `yaml:"upload_dir"`
	PublicBaseURL string `yaml:"public_base_url"`
	MaxFileSize   int64  `yaml:"max_file_size"`
	WorkerCount   int    `yaml:"worker_count"`
	QueueSize     int    `yaml:"queue_size"`
}

// Save 校验并原子写入 config.yml（同目录临时文件 + rename）。token 会被写入，
// 因为 config.yml 是配置的最终来源。
func Save(path string, c AppConfig) error {
	if err := Validate(c); err != nil {
		return err
	}
	data, err := yaml.Marshal(yamlConfig{
		Port:          c.Port,
		Token:         c.Token,
		UploadDir:     c.UploadDir,
		PublicBaseURL: c.PublicBaseURL,
		MaxFileSize:   c.MaxFileSize,
		WorkerCount:   c.WorkerCount,
		QueueSize:     c.QueueSize,
	})
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.yml.tmp")
	if err != nil {
		return fmt.Errorf("创建临时配置文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后为 no-op

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("写入临时配置文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("同步临时配置文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时配置文件失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("替换配置文件失败: %w", err)
	}
	return nil
}
