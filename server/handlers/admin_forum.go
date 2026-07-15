package handlers

import (
	"strconv"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GET /api/admin/forums — 列出所有论坛（管理员）
func AdminListForums(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	forums, err := models.ListAllForums("")
	if err != nil {
		utils.InternalError(c, "获取论坛列表失败")
		return
	}
	utils.Success(c, forums)
}

// POST /api/admin/forums — 创建论坛（管理员）
func AdminCreateForum(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Zone        string `json:"zone"`
		ParentID    *uint  `json:"parent_id"`
		SortOrder   int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写论坛名称")
		return
	}
	if req.Type == "" {
		req.Type = "general"
	}

	forum := &models.Forum{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Zone:        req.Zone,
		ParentID:    req.ParentID,
		SortOrder:   req.SortOrder,
	}
	if err := models.CreateForum(forum); err != nil {
		utils.InternalError(c, "创建论坛失败")
		return
	}
	utils.Success(c, forum)
}

// PUT /api/admin/forums/:id — 更新论坛（管理员）
func AdminUpdateForum(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "无效的论坛ID")
		return
	}

	forum, err := models.GetForumByID(uint(id))
	if err != nil {
		utils.NotFound(c, "论坛不存在")
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Type        *string `json:"type"`
		Zone        *string `json:"zone"`
		ParentID    *uint   `json:"parent_id"`
		SortOrder   *int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "无效的请求数据")
		return
	}

	if req.Name != nil {
		forum.Name = *req.Name
	}
	if req.Description != nil {
		forum.Description = *req.Description
	}
	if req.Type != nil {
		forum.Type = *req.Type
	}
	if req.Zone != nil {
		forum.Zone = *req.Zone
	}
	if req.ParentID != nil {
		forum.ParentID = req.ParentID
	}
	if req.SortOrder != nil {
		forum.SortOrder = *req.SortOrder
	}

	if err := models.UpdateForum(forum); err != nil {
		utils.InternalError(c, "更新论坛失败")
		return
	}
	utils.Success(c, forum)
}

// DELETE /api/admin/forums/:id — 删除论坛（管理员）
func AdminDeleteForum(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "无效的论坛ID")
		return
	}

	if err := models.DeleteForum(uint(id)); err != nil {
		utils.InternalError(c, "删除论坛失败")
		return
	}
	utils.Success(c, gin.H{"message": "论坛已删除"})
}
