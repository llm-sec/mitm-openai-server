package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// 全局标志
	cfgFile string
	dataDir string
	verbose bool

	// rootCmd 代表没有调用子命令时的基础命令
	rootCmd = &cobra.Command{
		Use:   "mitm-openai-server",
		Short: "一个用于记录API请求的假OpenAPI服务器",
		Long: `这是一个简单的假OpenAPI服务器，用于接收和记录传入的请求数据。
服务器符合OpenAPI标准，总是返回固定的成功响应，并将请求详细信息记录到本地文件数据库中。

可以使用不同的子命令来启动服务器、管理请求数据或查看版本信息。`,
		Run: func(cmd *cobra.Command, args []string) {
			// 如果没有指定子命令，默认执行server子命令
			serverCmd.Run(cmd, args)
		},
	}
)

// Execute 添加所有子命令到根命令并设置标志
// 这是由main.main()调用的
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// 持久性标志，对所有子命令都有效
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认是 $HOME/.mitm-openai-server.yaml)")
	rootCmd.PersistentFlags().StringVar(&dataDir, "data", "./data", "请求数据存储目录")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "启用详细输出")
}
