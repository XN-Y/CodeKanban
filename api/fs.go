package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"code-kanban/api/h"
)

const fsTag = "fs-文件系统"

// DirEntry 目录条目
type DirEntry struct {
	Name string `json:"name" doc:"目录名称"`
	Path string `json:"path" doc:"完整路径"`
}

// ListDirsResponse 目录列表响应
type ListDirsResponse struct {
	Dirs        []DirEntry `json:"dirs" doc:"目录列表"`
	ParentPath  string     `json:"parentPath,omitempty" doc:"父目录路径"`
	CurrentPath string     `json:"currentPath" doc:"当前路径"`
}

// HomeResponse HOME目录响应
type HomeResponse struct {
	Path string `json:"path" doc:"HOME目录路径"`
}

func registerFSRoutes(group *huma.Group) {
	// Get home directory
	huma.Get(group, "/fs/home", func(ctx context.Context, _ *struct{}) (*h.ItemResponse[HomeResponse], error) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to get home directory", err)
		}

		resp := h.NewItemResponse(HomeResponse{
			Path: homeDir,
		})
		resp.Status = http.StatusOK
		return resp, nil
	}, func(op *huma.Operation) {
		op.OperationID = "fs-home"
		op.Summary = "获取HOME目录"
		op.Description = "返回当前用户的HOME目录路径"
		op.Tags = []string{fsTag}
	})

	// List directories
	huma.Get(group, "/fs/list-dirs", func(ctx context.Context, input *struct {
		Path string `query:"path" doc:"目录路径"`
	}) (*h.ItemResponse[ListDirsResponse], error) {
		path := input.Path

		// 如果路径为空，返回错误（需要提供路径）
		if path == "" {
			return nil, huma.Error400BadRequest("path is required")
		}

		// 规范化路径
		path = filepath.Clean(path)

		// 检查路径是否存在
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, huma.Error404NotFound("directory not found")
			}
			return nil, huma.Error500InternalServerError("failed to access directory", err)
		}

		if !info.IsDir() {
			return nil, huma.Error400BadRequest("path is not a directory")
		}

		// 读取子目录
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to read directory", err)
		}

		var dirs []DirEntry
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			// 跳过隐藏目录（以.开头）
			if strings.HasPrefix(name, ".") {
				continue
			}

			fullPath := filepath.Join(path, name)
			dirs = append(dirs, DirEntry{
				Name: name,
				Path: fullPath,
			})
		}

		// 按名称排序
		sort.Slice(dirs, func(i, j int) bool {
			return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
		})

		// 获取父目录
		parentPath := filepath.Dir(path)
		// Windows: C:\ 的父目录还是 C:\，需要特殊处理
		if runtime.GOOS == "windows" {
			// 如果是驱动器根目录（如 C:\），parentPath 设为空表示到顶了
			if len(path) == 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
				parentPath = ""
			}
		} else {
			// Unix: / 的父目录还是 /
			if parentPath == path {
				parentPath = ""
			}
		}

		resp := h.NewItemResponse(ListDirsResponse{
			Dirs:        dirs,
			ParentPath:  parentPath,
			CurrentPath: path,
		})
		resp.Status = http.StatusOK
		return resp, nil
	}, func(op *huma.Operation) {
		op.OperationID = "fs-list-dirs"
		op.Summary = "列出目录"
		op.Description = "列出指定目录下的子目录"
		op.Tags = []string{fsTag}
	})
}
