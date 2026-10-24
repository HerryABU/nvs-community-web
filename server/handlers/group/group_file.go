package grouphandler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"nvs-server/groups"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 群文件 ====================
// 存储位置：data/groups/<group_id>/（**不在** /uploads 静态目录下，只能经鉴权路由下载）
// 安全：扩展名白名单 + 50MB 单文件 + 512MB 单群 + ZIP 炸弹检测 + Content-Disposition 净化。
// 权限：仅正常成员可上传/下载；删除限上传者本人或群管。

const groupFileDir = "./data/groups"

// GroupFileUpload POST /api/groups/:id/files （multipart: file）
func GroupFileUpload(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	if ctx.Status != groups.GroupMemberActive {
		utils.Forbidden(c, "只有群成员可以上传文件")
		return
	}
	if ctx.Group.Status != groups.GroupStatusActive {
		utils.BadRequest(c, "该群组当前不可上传文件")
		return
	}

	var file multipart.File
	var header *multipart.FileHeader
	var err error
	if file, header, err = c.Request.FormFile("file"); err != nil {
		utils.BadRequest(c, "请选择要上传的文件")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !groups.GroupFileExtAllowed(ext) {
		utils.BadRequest(c, "不支持的文件类型（允许：图片/PDF/Office/文本/压缩包/音视频）")
		return
	}
	if header.Size > groups.MaxGroupFileSize {
		utils.BadRequest(c, fmt.Sprintf("文件不能超过 %d MB", groups.MaxGroupFileSize>>20))
		return
	}
	used := groups.GroupFileUsedBytes(ctx.Group.ID)
	if used+header.Size > groups.MaxGroupTotalSize {
		utils.BadRequest(c, fmt.Sprintf("本群文件空间已用 %.1f MB，上限 %d MB",
			float64(used)/1024/1024, groups.MaxGroupTotalSize>>20))
		return
	}

	// 落盘：data/groups/<gid>/<随机名>.<ext>
	dir := filepath.Join(groupFileDir, strconv.FormatUint(uint64(ctx.Group.ID), 10))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		utils.InternalError(c, "创建目录失败")
		return
	}
	rb := make([]byte, 8)
	if _, err := rand.Read(rb); err != nil {
		utils.InternalError(c, "生成文件名失败")
		return
	}
	name := hex.EncodeToString(rb) + ext
	abs := filepath.Join(dir, name)
	dst, err := os.Create(abs)
	if err != nil {
		utils.InternalError(c, "保存文件失败")
		return
	}
	h := sha256.New()
	written, cpErr := io.Copy(io.MultiWriter(dst, h), io.LimitReader(file, groups.MaxGroupFileSize+1))
	_ = dst.Close()
	if cpErr != nil {
		_ = os.Remove(abs)
		utils.InternalError(c, "写入文件失败")
		return
	}
	if written > groups.MaxGroupFileSize {
		_ = os.Remove(abs)
		utils.BadRequest(c, "文件过大")
		return
	}

	// 压缩包：ZIP 炸弹检测（只读中央目录）
	if ext == ".zip" {
		if zerr := groups.CheckArchiveSafety(abs); zerr != nil {
			_ = os.Remove(abs)
			utils.BadRequest(c, zerr.Error())
			return
		}
	}

	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	rec, cerr := groups.CreateGroupFile(ctx.Group.ID, uid,
		groups.SanitizeGroupFileName(header.Filename), written, mime,
		hex.EncodeToString(h.Sum(nil)), filepath.ToSlash(abs))
	if cerr != nil {
		_ = os.Remove(abs)
		utils.InternalError(c, cerr.Error())
		return
	}
	utils.SuccessMessage(c, "上传成功", rec)
}

// GroupFileList GET /api/groups/:id/files
func GroupFileList(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	if ctx.Status != groups.GroupMemberActive && !ctx.IsAdmin {
		utils.Forbidden(c, "仅群成员可查看群文件")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "30"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 30
	}
	list, total, err := groups.ListGroupFiles(ctx.Group.ID, page, size)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{
		"list": list, "total": total, "page": page, "size": size,
		"used": groups.GroupFileUsedBytes(ctx.Group.ID), "quota": groups.MaxGroupTotalSize,
	})
}

// GroupFileDownload GET /api/groups/:id/files/:fid/download
func GroupFileDownload(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	fid, _ := strconv.ParseUint(c.Param("fid"), 10, 64)
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	if ctx.Status != groups.GroupMemberActive && !ctx.IsAdmin {
		utils.Forbidden(c, "仅群成员可下载群文件")
		return
	}
	f, err := groups.GetGroupFile(ctx.Group.ID, uint(fid))
	if err != nil {
		utils.NotFound(c, "文件不存在")
		return
	}
	// 存储路径必须落在 data/groups 内（防路径穿越）
	abs, aerr := filepath.Abs(filepath.Clean(f.StoragePath))
	if aerr != nil {
		utils.Forbidden(c, "非法文件路径")
		return
	}
	root, _ := filepath.Abs(groupFileDir)
	if !strings.HasPrefix(filepath.ToSlash(abs), filepath.ToSlash(root)+"/") {
		utils.Forbidden(c, "非法文件路径")
		return
	}
	if _, serr := os.Stat(abs); serr != nil {
		utils.NotFound(c, "文件已丢失（可能已被清理）")
		return
	}
	// 头注入防护：文件名编码 + nosniff
	disposition := fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(f.Name))
	c.Header("Content-Disposition", disposition)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", "application/octet-stream")
	c.File(abs)
}

// GroupFileDelete DELETE /api/groups/:id/files/:fid
func GroupFileDelete(c *gin.Context) {
	gid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	fid, _ := strconv.ParseUint(c.Param("fid"), 10, 64)
	uid, ok := requireGroupLogin(c)
	if !ok {
		return
	}
	ctx, ok := loadGroupCtx(c, uint(gid))
	if !ok {
		return
	}
	isManager := ctx.Role == groups.GroupRoleOwner || ctx.Role == groups.GroupRoleAdmin || ctx.IsAdmin
	if err := groups.DeleteGroupFile(ctx.Group.ID, uint(fid), uid, isManager); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已删除", nil)
}
