package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jupi-d2c/internal/api/services"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// baseURL 由请求拼出 scheme://host，用于生成可分享的 AST 访问地址。
// scheme 优先取反代注入的 X-Forwarded-Proto，否则按是否 TLS 推断。
func baseURL(c *gin.Context) string {
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + c.Request.Host
}

// saveDesignInput 是 POST /api/design 的请求体：ast 容忍任意 JSON（对象或字符串）。
type saveDesignInput struct {
	Tag string          `json:"tag"`
	AST json.RawMessage `json:"ast"`
}

// SaveDesign 持久化一次生成的 AST（一个 design），需 Bearer token。
// 返回 { data: { id, tag, createdAt, astUrl } }，astUrl 为公开可访问的 AST 地址。
func (h *Handlers) SaveDesign(c *gin.Context) {
	var in saveDesignInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON", "message": err.Error()})
		return
	}
	if len(in.AST) == 0 || string(in.AST) == "null" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ast 不能为空"})
		return
	}

	createdAt := time.Now().UnixMilli()
	tag := in.Tag
	if tag == "" {
		// 无 tag 时回落到服务端时间串，保证列表里始终有可读标识。
		tag = time.UnixMilli(createdAt).Format("2006-01-02_15-04-05")
	}

	saved, err := h.designs.Save(tag, string(in.AST), createdAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		return
	}

	astURL := baseURL(c) + "/api/ast/" + saved.ID
	log.Printf("[design] ✓ 保存 design id=%s tag=%q -> %s", saved.ID, saved.Tag, astURL)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":        saved.ID,
		"tag":       saved.Tag,
		"createdAt": saved.CreatedAt,
		"astUrl":    astURL,
	}})
}

// ListDesigns 按生成时间倒序分页返回 design 列表（公开，不含 ast 大字段）。
// 可选 query 参数 tag（可重复）按 tag 过滤，只返回命中其中任一 tag 的 design。
func (h *Handlers) ListDesigns(c *gin.Context) {
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), defaultPageSize)
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	tags := c.QueryArray("tag")

	items, total, err := h.designs.List(page, pageSize, tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		return
	}

	base := baseURL(c)
	out := make([]gin.H, 0, len(items))
	for _, d := range items {
		out = append(out, gin.H{
			"id":          d.ID,
			"tag":         d.Tag,
			"createdAt":   d.CreatedAt,
			"astUrl":      base + "/api/ast/" + d.ID,
			"referDomUrl": base + "/api/ast/" + d.ID + "/refer-dom",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"items":    out,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// GetAST 公开返回某个 design 的 AST JSON 原文；URL 本身即凭据，不鉴权。
func (h *Handlers) GetAST(c *gin.Context) {
	ast, err := h.designs.GetAST(c.Param("id"))
	if errors.Is(err, services.ErrDesignNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "id": c.Param("id")})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(ast))
}

// ListTags 公开返回所有去重 tag（升序），供前端筛选下拉使用。
func (h *Handlers) ListTags(c *gin.Context) {
	tags, err := h.designs.Tags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// CleanupDesigns 删除生成时间早于 now-hours 的 design（即「xx 小时前」），需 Bearer token。
// hours 由 query 参数指定（小时，可为小数，>0；默认 1）。返回 { deleted, hours }。
func (h *Handlers) CleanupDesigns(c *gin.Context) {
	hours, err := strconv.ParseFloat(c.DefaultQuery("hours", "1"), 64)
	if err != nil || hours <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数 hours 非法，应为正数"})
		return
	}
	cutoff := time.Now().Add(-time.Duration(hours * float64(time.Hour))).UnixMilli()

	deleted, err := h.designs.DeleteOlderThan(cutoff)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		return
	}
	log.Printf("[design] 🧹 清理 %.4g 小时前的 design：删除 %d 条", hours, deleted)
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "hours": hours})
}

// parsePositiveInt 解析正整数 query 参数，非法或非正时回落到 def。
func parsePositiveInt(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

// GetReferDom 公开返回某个 design 的 refer_dom（参考 DOM）及状态信息；URL 即凭据。
func (h *Handlers) GetReferDom(c *gin.Context) {
	id := c.Param("id")
	referDom, status, errorsStr, err := h.designs.GetReferDom(id)
	if errors.Is(err, services.ErrDesignNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found", "id": id})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"referDom": referDom,
		"status":   status,
		"errors":   errorsStr,
	})
}

// GenerateReferDom 分析指定 design 的 AST 并生成参考 HTML DOM 结构。
// 内部逻辑（AI 调用、重试、验证、持久化）已委托给 services.AIService。
func (h *Handlers) GenerateReferDom(c *gin.Context) {
	id := c.Param("id")

	result, err := h.ai.GenerateReferDom(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrDesignNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found", "id": id})
			return
		}
		if errors.Is(err, services.ErrAIConfigNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "AI 未配置，请先在 AI 聊天页面中配置 AI 参数"})
			return
		}
		errStr := err.Error()
		if strings.Contains(errStr, "AI 服务调用失败") || strings.Contains(errStr, "AI 服务不可用") {
			c.JSON(http.StatusBadGateway, gin.H{"error": "AI 服务调用失败", "message": errStr})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "message": errStr})
		return
	}

	if result.Status == "warning" {
		c.JSON(http.StatusOK, gin.H{
			"code": "REFER_DOM_WARNING",
			"data": gin.H{
				"id":       id,
				"referDom": result.ReferDom,
				"warnings": result.Warnings,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"id":       id,
		"referDom": result.ReferDom,
	}})
}
