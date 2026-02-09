package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/abcdlsj/otter/internal/config"
)

// Glob 工具提供强大的文件模式匹配功能
type Glob struct{}

func (Glob) Name() string { return "glob" }
func (Glob) Desc() string {
	return "Find files matching glob patterns. Supports multiple patterns, exclusions, and size filtering"
}

func (Glob) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"patterns": map[string]any{
				"type":        "array",
				"description": "Glob patterns to match (e.g., '*.go', '**/*.md'). Supports ** for recursive matching",
				"items": map[string]any{
					"type": "string",
				},
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Root directory to search from (default: current directory)",
			},
			"exclude": map[string]any{
				"type":        "array",
				"description": "Patterns to exclude (e.g., '*_test.go', 'vendor/**')",
				"items": map[string]any{
					"type": "string",
				},
			},
			"max_results": map[string]any{
				"type":        "number",
				"description": "Maximum number of results to return (default: 100, max: 500)",
			},
			"sort_by": map[string]any{
				"type":        "string",
				"description": "Sort results by: 'name' (default), 'path', 'size', 'time'",
				"enum":        []string{"name", "path", "size", "time"},
			},
		},
		"required": []string{"patterns"},
	}
}

func (g Glob) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Patterns   []string `json:"patterns"`
		Path       string   `json:"path"`
		Exclude    []string `json:"exclude"`
		MaxResults int      `json:"max_results"`
		SortBy     string   `json:"sort_by"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}

	// 默认值设置
	if args.Path == "" {
		args.Path = "."
	}
	if args.MaxResults <= 0 {
		args.MaxResults = 100
	}
	if args.MaxResults > 500 {
		args.MaxResults = 500
	}
	if args.SortBy == "" {
		args.SortBy = "name"
	}

	// 清理路径
	args.Path = filepath.Clean(args.Path)

	// 检查读取权限
	cfg := &config.C
	if !cfg.CheckReadPermission(args.Path) {
		return "", fmt.Errorf("permission denied: cannot read %s", args.Path)
	}

	// 收集匹配的文件
	var matches []fileMatch
	seen := make(map[string]bool)

	for _, pattern := range args.Patterns {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// 处理 ** 递归匹配
		if strings.Contains(pattern, "**") {
			err := g.globRecursive(ctx, cfg, args.Path, pattern, args.Exclude, &matches, seen)
			if err != nil {
				return "", err
			}
		} else {
			// 标准 glob 匹配
			err := g.globStandard(ctx, cfg, args.Path, pattern, args.Exclude, &matches, seen)
			if err != nil {
				return "", err
			}
		}
	}

	if len(matches) == 0 {
		return "No files found matching the pattern(s)", nil
	}

	// 排序
	g.sortMatches(matches, args.SortBy)

	// 限制结果数量
	if len(matches) > args.MaxResults {
		matches = matches[:args.MaxResults]
	}

	// 格式化输出
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d file(s) in %s\n\n", len(matches), args.Path))

	for _, m := range matches {
		info, err := os.Stat(m.fullPath)
		if err != nil {
			result.WriteString(fmt.Sprintf("  %s\n", m.relPath))
			continue
		}
		sizeStr := formatFileSize(info.Size())
		result.WriteString(fmt.Sprintf("  %s (%s)\n", m.relPath, sizeStr))
	}

	if len(seen) > args.MaxResults {
		result.WriteString(fmt.Sprintf("\n... (%d more matches not shown)", len(seen)-args.MaxResults))
	}

	return result.String(), nil
}

// fileMatch 表示一个匹配的文件
type fileMatch struct {
	fullPath string
	relPath  string
	info     os.FileInfo
}

// globStandard 处理标准 glob 模式（非递归 **）
func (g Glob) globStandard(ctx context.Context, cfg *config.Config, root, pattern string, exclude []string, matches *[]fileMatch, seen map[string]bool) error {
	// 如果模式以 / 开头，从根目录开始
	searchPath := root
	if filepath.IsAbs(pattern) {
		searchPath = "/"
		pattern = pattern[1:]
	}

	// 构建完整搜索路径
	fullPattern := filepath.Join(searchPath, pattern)

	// 使用 filepath.Glob
	files, err := filepath.Glob(fullPattern)
	if err != nil {
		return fmt.Errorf("invalid pattern '%s': %w", pattern, err)
	}

	for _, f := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 检查是否在排除列表中
		if g.isExcluded(f, exclude) {
			continue
		}

		// 检查权限
		if !cfg.CheckReadPermission(f) {
			continue
		}

		// 去重
		if seen[f] {
			continue
		}
		seen[f] = true

		// 获取文件信息
		info, err := os.Stat(f)
		if err != nil {
			continue
		}

		// 跳过目录
		if info.IsDir() {
			continue
		}

		// 计算相对路径
		relPath, err := filepath.Rel(root, f)
		if err != nil {
			relPath = f
		}

		*matches = append(*matches, fileMatch{
			fullPath: f,
			relPath:  relPath,
			info:     info,
		})
	}

	return nil
}

// globRecursive 处理包含 ** 的递归模式
func (g Glob) globRecursive(ctx context.Context, cfg *config.Config, root, pattern string, exclude []string, matches *[]fileMatch, seen map[string]bool) error {
	// 解析 ** 模式
	// 例如: "**/*.go" -> 递归查找所有 .go 文件
	// 例如: "src/**/test_*.go" -> 在 src 下递归查找 test_*.go

	// 分离基础路径和文件模式
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		// 多个 ** 或没有 **，作为标准 glob 处理
		return g.globStandard(ctx, cfg, root, pattern, exclude, matches, seen)
	}

	basePattern := strings.TrimSuffix(parts[0], "/")
	filePattern := strings.TrimPrefix(parts[1], "/")

	searchRoot := root
	if basePattern != "" {
		searchRoot = filepath.Join(root, basePattern)
	}

	// 确保搜索根目录存在
	info, err := os.Stat(searchRoot)
	if err != nil || !info.IsDir() {
		return nil // 静默处理，不报错
	}

	// 递归遍历
	return filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 跳过隐藏目录和常见忽略目录
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			if name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == "target" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查文件是否匹配模式
		matched, err := filepath.Match(filePattern, d.Name())
		if err != nil || !matched {
			return nil
		}

		// 检查排除列表
		if g.isExcluded(path, exclude) {
			return nil
		}

		// 检查权限
		if !cfg.CheckReadPermission(path) {
			return nil
		}

		// 去重
		if seen[path] {
			return nil
		}
		seen[path] = true

		// 获取文件信息
		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			relPath = path
		}

		*matches = append(*matches, fileMatch{
			fullPath: path,
			relPath:  relPath,
			info:     fileInfo,
		})

		return nil
	})
}

// isExcluded 检查路径是否匹配排除模式
func (g Glob) isExcluded(path string, exclude []string) bool {
	for _, pattern := range exclude {
		// 支持 ** 的排除模式
		if strings.Contains(pattern, "**") {
			// 简化处理：**/pattern 匹配任何子目录中的 pattern
			suffix := strings.TrimPrefix(pattern, "**/")
			if suffix != pattern {
				// 检查路径的任何部分是否匹配后缀
				if matched, _ := filepath.Match(suffix, filepath.Base(path)); matched {
					return true
				}
				// 检查完整路径是否包含该模式
				if strings.Contains(path, suffix) {
					return true
				}
			}
		} else {
			// 标准匹配
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return true
			}
			// 也检查完整路径匹配
			if matched, _ := filepath.Match(pattern, path); matched {
				return true
			}
		}
	}
	return false
}

// sortMatches 对匹配结果进行排序
func (g Glob) sortMatches(matches []fileMatch, sortBy string) {
	switch sortBy {
	case "path":
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].relPath < matches[j].relPath
		})
	case "size":
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].info.Size() > matches[j].info.Size() // 降序
		})
	case "time":
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].info.ModTime().After(matches[j].info.ModTime()) // 降序
		})
	default: // "name"
		sort.Slice(matches, func(i, j int) bool {
			return filepath.Base(matches[i].relPath) < filepath.Base(matches[j].relPath)
		})
	}
}
