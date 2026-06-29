package main

import (
	"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "重启服务（先优雅停止，再后台启动）",
		Long:  "重启后台服务：先优雅停止正在运行的实例，再以最新配置后台启动。\n服务未在运行时等价于 start。",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			// 先停（未运行时 stopDaemon 仅打印提示并返回 nil），再以最新配置启动。
			if err := stopDaemon(out); err != nil {
				return err
			}
			return startDaemon(out)
		},
	}
}
