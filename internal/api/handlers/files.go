package handlers

import (
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ===== 文件树 DTO（camelCase，与 web/src/api.ts 契约一致）=====

// fileNode 是上传目录树的一个节点：目录（type=dir，带 children）或文件（type=file，带元信息）。
type fileNode struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"` // "dir" | "file"
	Path     string      `json:"path"` // 相对 UploadDir 的路径，以 / 分隔；根为 ""
	Children []*fileNode `json:"children,omitempty"`

	Size        int64  `json:"size,omitempty"`
	ModTime     string `json:"modTime,omitempty"`     // RFC3339 (UTC)
	URL         string `json:"url,omitempty"`         // /uploads/<path>
	ContentType string `json:"contentType,omitempty"` // 按扩展名推断，供前端选图标
}

type filesResponse struct {
	Root       *fileNode `json:"root"`
	UploadDir  string    `json:"uploadDir"` // 解析后的绝对路径，供面板展示「当前上传目录」
	TotalFiles int       `json:"totalFiles"`
	TotalSize  int64     `json:"totalSize"`
}

// ListFiles 递归遍历上传目录，返回嵌套文件树及汇总。目录不存在/为空都返回空根（不报错）。
func (h *Handlers) ListFiles(c *gin.Context) {
	base, err := filepath.Abs(h.cfg.UploadDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		return
	}

	root := &fileNode{Name: filepath.Base(base), Type: "dir", Path: "", Children: []*fileNode{}}
	totalFiles, totalSize := 0, int64(0)

	// walk 递归读取 dir（绝对路径），rel 是其相对 base 的路径（以 / 分隔，根为 ""）。
	var walk func(dir, rel string) ([]*fileNode, error)
	walk = func(dir, rel string) ([]*fileNode, error) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			// 根不存在视为空目录；子目录读失败则跳过该层。
			return []*fileNode{}, nil
		}
		nodes := make([]*fileNode, 0, len(entries))
		for _, e := range entries {
			name := e.Name()
			childRel := name
			if rel != "" {
				childRel = path.Join(rel, name)
			}
			if e.IsDir() {
				children, _ := walk(filepath.Join(dir, name), childRel)
				dirNode := &fileNode{
					Name:     name,
					Type:     "dir",
					Path:     childRel,
					Children: children,
				}
				// 目录也带修改时间，用于与文件统一按时间降序排序。
				if info, err := e.Info(); err == nil {
					dirNode.ModTime = info.ModTime().UTC().Format(time.RFC3339)
				}
				nodes = append(nodes, dirNode)
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			totalFiles++
			totalSize += info.Size()
			nodes = append(nodes, &fileNode{
				Name:        name,
				Type:        "file",
				Path:        childRel,
				Size:        info.Size(),
				ModTime:     info.ModTime().UTC().Format(time.RFC3339),
				URL:         "/uploads/" + childRel,
				ContentType: contentTypeByName(name),
			})
		}
		// 按修改时间降序：最近修改的（文件或目录）排在最上面。
		// ModTime 为 RFC3339 UTC 定宽字符串，字典序降序即时间倒序。
		sort.SliceStable(nodes, func(i, j int) bool {
			return nodes[i].ModTime > nodes[j].ModTime
		})
		return nodes, nil
	}

	root.Children, _ = walk(base, "")
	c.JSON(http.StatusOK, filesResponse{Root: root, UploadDir: base, TotalFiles: totalFiles, TotalSize: totalSize})
}

// CleanupFiles 删除上传目录中修改时间早于 now-hours 的旧文件（即「xx 小时前的文件」），
// 并清理因此变空的子目录。hours 由 query 参数 hours 指定（小时，可为小数，>0；默认 1）。
// 返回 { deleted, freedBytes, hours }。
func (h *Handlers) CleanupFiles(c *gin.Context) {
	hours, err := strconv.ParseFloat(c.DefaultQuery("hours", "1"), 64)
	if err != nil || hours <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数 hours 非法，应为正数"})
		return
	}
	base, err := filepath.Abs(h.cfg.UploadDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		return
	}
	cutoff := time.Now().Add(-time.Duration(hours * float64(time.Hour)))

	deleted, freed := 0, int64(0)
	// 第一遍：删除修改时间早于 cutoff 的文件（仅文件，目录留待第二遍按空清理）。
	_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			if os.Remove(p) == nil {
				deleted++
				freed += info.Size()
			}
		}
		return nil
	})
	// 第二遍：自底向上删除变空的子目录（保留上传根目录本身）。
	removeEmptyDirs(base)

	log.Printf("[cleanup] 🧹 清理 %.4g 小时前的文件：删除 %d 个，释放 %d 字节", hours, deleted, freed)
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "freedBytes": freed, "hours": hours})
}

// removeEmptyDirs 自底向上删除 dir 下所有空子目录（不删除 dir 自身）。
func removeEmptyDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		removeEmptyDirs(sub)
		if rest, err := os.ReadDir(sub); err == nil && len(rest) == 0 {
			os.Remove(sub)
		}
	}
}

// contentTypeByName 按扩展名推断 MIME，仅用于前端选图标/判断是否图片；无法推断返回 ""。
func contentTypeByName(name string) string {
	ct := mime.TypeByExtension(filepath.Ext(name))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i] // 去掉 "; charset=utf-8" 之类后缀
	}
	return strings.TrimSpace(ct)
}
