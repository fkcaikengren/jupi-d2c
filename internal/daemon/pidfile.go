package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// DefaultPIDFile 是未显式指定时的 PID 文件路径（cwd）。
const DefaultPIDFile = "./d2c-manager.pid"

// WritePIDFile 原子写入 PID（同目录临时文件 + rename），避免并发 start 读到半截内容。
func WritePIDFile(path string, pid int) error {
	tmp, err := os.CreateTemp("", "d2c-pid-*")
	if err != nil {
		return fmt.Errorf("创建临时 PID 文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后为 no-op

	if _, err := tmp.WriteString(strconv.Itoa(pid)); err != nil {
		tmp.Close()
		return fmt.Errorf("写入 PID 失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时 PID 文件失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("替换 PID 文件失败: %w", err)
	}
	return nil
}

// ReadPIDFile 读取 PID 文件中的进程号。文件不存在返回 (0, os.ErrNotExist)，
// 调用方可据此判定“未运行”。
func ReadPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("PID 文件内容非法 %q: %w", string(data), err)
	}
	return pid, nil
}

// RemovePIDFile 删除 PID 文件；文件本就不存在不视为错误。
func RemovePIDFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ProcessAlive 用信号 0 探测进程是否存活（不真正发信号）。
// 进程存在但属于他人时返回 EPERM，仍视为存活。
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return err == syscall.EPERM
}

// RunningPID 综合 PID 文件 + 存活探测：返回正在运行的进程号，未运行返回 0。
// PID 文件存在但进程已死（陈旧 PID 文件）也返回 0。
func RunningPID(pidFile string) (int, error) {
	pid, err := ReadPIDFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if !ProcessAlive(pid) {
		return 0, nil
	}
	return pid, nil
}
