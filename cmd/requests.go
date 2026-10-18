package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/llm-sec/mitm-openai-server/pkg/storage"
	"github.com/spf13/cobra"
)

var (
	requestsListLimit  int
	requestsListOffset int
	requestsDeleteAll  bool
)

// requestsCmd 管理已记录的请求数据
var requestsCmd = &cobra.Command{
	Use:   "requests",
	Short: "管理已记录的请求数据",
	Long: `列出、查看和删除服务器记录的API请求。

示例:
  mitm-openai-server requests list
  mitm-openai-server requests get <request-id>
  mitm-openai-server requests delete <request-id>
  mitm-openai-server requests delete --all`,
}

// requestsListCmd 列出所有已记录的请求
var requestsListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有已记录的请求",
	Run: func(cmd *cobra.Command, args []string) {
		store := openRequestStorage()
		defer store.Close()

		requests, err := store.GetAllRequests(requestsListLimit, requestsListOffset)
		if err != nil {
			log.Fatalf("获取请求列表失败: %v", err)
		}

		if len(requests) == 0 {
			fmt.Println("暂无请求记录")
			return
		}

		fmt.Printf("共 %d 条请求记录:\n", len(requests))
		for _, req := range requests {
			statusCode := "-"
			if req.Response != nil {
				statusCode = fmt.Sprintf("%d", req.Response.StatusCode)
			}
			fmt.Printf("  %s  [user=%d] %s %s -> %s\n",
				req.ID, req.UserID, req.Method, req.Path, statusCode)
		}
	},
}

// requestsGetCmd 查看特定请求的详情
var requestsGetCmd = &cobra.Command{
	Use:   "get <request-id>",
	Short: "查看特定请求的详情",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		store := openRequestStorage()
		defer store.Close()

		id := args[0]
		req, err := findRequestByID(store, id)
		if err != nil {
			log.Fatalf("%v", err)
		}

		data, err := json.MarshalIndent(req, "", "  ")
		if err != nil {
			log.Fatalf("序列化请求数据失败: %v", err)
		}
		fmt.Println(string(data))
	},
}

// requestsDeleteCmd 删除已记录的请求
var requestsDeleteCmd = &cobra.Command{
	Use:   "delete <request-id>",
	Short: "删除特定请求",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		store := openRequestStorage()
		defer store.Close()

		if requestsDeleteAll {
			if err := store.DeleteAllRequests(); err != nil {
				log.Fatalf("删除所有请求失败: %v", err)
			}
			fmt.Println("已删除所有请求记录")
			return
		}

		if len(args) != 1 {
			log.Fatalf("请指定要删除的请求ID，或使用 --all 删除所有请求")
		}

		id := args[0]
		req, err := findRequestByID(store, id)
		if err != nil {
			log.Fatalf("%v", err)
		}

		// 使用请求所属的用户ID进行删除，满足用户隔离要求
		if err := store.DeleteRequest(req.UserID, id); err != nil {
			log.Fatalf("删除请求失败: %v", err)
		}
		fmt.Printf("已删除请求 %s\n", id)
	},
}

func init() {
	rootCmd.AddCommand(requestsCmd)
	requestsCmd.AddCommand(requestsListCmd)
	requestsCmd.AddCommand(requestsGetCmd)
	requestsCmd.AddCommand(requestsDeleteCmd)

	requestsListCmd.Flags().IntVar(&requestsListLimit, "limit", 100, "每页返回的最大记录数")
	requestsListCmd.Flags().IntVar(&requestsListOffset, "offset", 0, "分页起始位置")
	requestsDeleteCmd.Flags().BoolVar(&requestsDeleteAll, "all", false, "删除所有请求")
}

// openRequestStorage 打开SQLite存储，用于CLI管理请求数据
func openRequestStorage() *storage.SQLiteStorage {
	dir := dataDir
	if dir == "" {
		dir = "./data"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("获取数据目录绝对路径失败: %v", err)
	}

	store, err := storage.NewSQLiteStorage(absDir)
	if err != nil {
		log.Fatalf("打开存储失败: %v", err)
	}
	return store
}

// findRequestByID 在所有请求中查找指定ID的请求（管理员视图）
func findRequestByID(store *storage.SQLiteStorage, id string) (*storage.Request, error) {
	requests, err := store.GetAllRequests(1000000, 0)
	if err != nil {
		return nil, fmt.Errorf("查询请求失败: %v", err)
	}
	for _, req := range requests {
		if req.ID == id {
			return req, nil
		}
	}
	return nil, fmt.Errorf("未找到ID为 %s 的请求", id)
}
