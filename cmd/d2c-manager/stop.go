package main

import (
	"fmt"
	"syscall"
	"time"

	"d2c-manager/internal/daemon"

	"github.com/spf13/cobra"
)

// stopTimeout 是发出 SIGTERM 后等待进程优雅退出的上限。
// 略大于 daemon 内部的 shutdownTimeout(30s)，给排空请求/队列留足余量。
const stopTimeout = 35 * time.Second

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "stop",
		Aliases: []string{"pause"},
		Short:   "停止服务（优雅关闭）",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pid, err := daemon.RunningPID(pidFile)
			if err != nil {
				return fmt.Errorf("检查运行状态失败: %w", err)
			}
			if pid == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "服务未在运行")
				_ = daemon.RemovePIDFile(pidFile) // 清理可能存在的陈旧 PID 文件
				return nil
			}

			// SIGTERM 触发 daemon 的优雅关闭（排空在途请求与队列）。
			if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
				return fmt.Errorf("发送停止信号失败 (pid %d): %w", pid, err)
			}

			// 轮询等待进程真正退出。
			deadline := time.Now().Add(stopTimeout)
			for daemon.ProcessAlive(pid) {
				if time.Now().After(deadline) {
					return fmt.Errorf("等待停止超时 (pid %d)，进程仍在运行", pid)
				}
				time.Sleep(200 * time.Millisecond)
			}

			if err := daemon.RemovePIDFile(pidFile); err != nil {
				return fmt.Errorf("删除 PID 文件失败: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "已停止 (pid %d)\n", pid)
			return nil
		},
	}
}
