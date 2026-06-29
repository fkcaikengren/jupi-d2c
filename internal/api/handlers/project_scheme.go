package handlers

import (
	"errors"
	"net/http"

	"jupi-d2c/internal/api/services"

	"github.com/gin-gonic/gin"
)

// ListProjectSchemes 按更新时间倒序分页返回项目适配方案列表（公开，不含 scheme 大字段）。
func (h *Handlers) ListProjectSchemes(c *gin.Context) {
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), defaultPageSize)
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	items, total, err := h.schemes.List(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":    items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// GetProjectScheme 按项目绝对路径返回完整方案（含 scheme markdown）。
// path 由 query 参数传入（而非 path 段），避免绝对路径中的 / 干扰路由匹配。
func (h *Handlers) GetProjectScheme(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少参数 path"})
		return
	}

	ps, err := h.schemes.Get(path)
	if errors.Is(err, services.ErrSchemeNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "path": path})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ps)
}
