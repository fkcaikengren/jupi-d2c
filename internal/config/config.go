package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// AppConfig 是启动期从环境变量解析并校验后的配置。
type AppConfig struct {
	Port          int
	Token         string
	UploadDir     string
	PublicBaseURL string
	MaxFileSize   int64
	WorkerCount   int
	QueueSize     int
}

func readEnv(name, fallback string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		v = fallback
	}
	if v == "" {
		return "", fmt.Errorf("缺少环境变量 %s", name)
	}
	return v, nil
}

func readEnvNumber(name string, fallback int64) (int64, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("环境变量 %s 不是合法正数: %s", name, raw)
	}
	return n, nil
}

// Load 从 cwd 下的 .env 读取（已存在的环境变量优先），再做校验。
func Load() (AppConfig, error) {
	_ = godotenv.Load() // .env 不存在时静默忽略

	token, err := readEnv("STORAGE_TOKEN", "")
	if err != nil {
		return AppConfig{}, err
	}
	uploadDir, err := readEnv("UPLOAD_DIR", "./uploads")
	if err != nil {
		return AppConfig{}, err
	}
	publicBaseURL, err := readEnv("PUBLIC_BASE_URL", "http://localhost:3000")
	if err != nil {
		return AppConfig{}, err
	}
	port, err := readEnvNumber("PORT", 3000)
	if err != nil {
		return AppConfig{}, err
	}
	maxFileSize, err := readEnvNumber("MAX_FILE_SIZE", 10*1024*1024)
	if err != nil {
		return AppConfig{}, err
	}
	workerCount, err := readEnvNumber("WORKER_COUNT", 4)
	if err != nil {
		return AppConfig{}, err
	}
	queueSize, err := readEnvNumber("QUEUE_SIZE", 64)
	if err != nil {
		return AppConfig{}, err
	}

	return AppConfig{
		Port:          int(port),
		Token:         token,
		UploadDir:     uploadDir,
		PublicBaseURL: strings.TrimRight(publicBaseURL, "/"),
		MaxFileSize:   maxFileSize,
		WorkerCount:   int(workerCount),
		QueueSize:     int(queueSize),
	}, nil
}
