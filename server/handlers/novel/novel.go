package novel

import (
	"encoding/json"
	"fmt"
	"nvs-server/utils/infra"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nvs-server/config"
	"nvs-server/handlers/auth"
	"nvs-server/handlers/hctx"
	"nvs-server/handlers/publish"
	"nvs-server/handlers/realname"
	"nvs-server/models"
	"nvs-server/moderation"
	"nvs-server/readperm"
	"nvs-server/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	quota "nvs-server/models/quota"
)

// applyPermissionInput 校验并写入作品级阅读权限（#65）。
// perm / price 用指针区分「未传」与「显式清空」；等级必须仍在站长开放集合内，
// paid 等级售价不得低于站长设定的最低价。返回 false 表示已写出响应。
func applyPermissionInput(c *gin.Context, novel *models.Novel, perm *string, price *int64) bool {
	if perm == nil && price == nil {
		return true
	}
	level := novel.ReadPermission
	if perm != nil {
		level = *perm
	}
	p := novel.PaidPrice
	if price != nil {
		p = *price
	}
	if err := readperm.ValidateSelection(level, p); err != nil {
		infra.BadRequest(c, err.Error())
		return false
	}
	novel.ReadPermission = level
	novel.PaidPrice = p
	return true
}

// POST /api/novels
func CreateNovel(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	userRole := hctx.CurrentUserRole(c)

	// 如果用户是 reader，自动升级为 author（首次发布作品时）
	if userRole == "reader" {
		// 生成签名密钥
		signingKey, _ := security.GenerateSigningKey()
		updates := map[string]interface{}{"role": "author"}
		if signingKey != "" {
			updates["signing_key"] = signingKey
		}
		if err := models.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
			infra.InternalError(c, "升级作者角色失败")
			return
		}
		userRole = "author"
		hctx.SetCurrentUserRole(c, "author")
	}

	if userRole != "author" && userRole != "vip_author" && userRole != "admin" {
		infra.Forbidden(c, "只有作者才能发布作品")
		return
	}

	// 实名强制门禁（站长开启 realname_required 时生效）
	if realname.CheckRealNameRequired(c) {
		return
	}
	// 未成年保护门禁（strict 模式下未成年人不可发布作品）
	if auth.CheckMinorPublishRestricted(c) {
		return
	}

	// 每日发文限额预检（§13.4；后台关闭时不生效）
	if !publish.EnforceDailyPublish(c, quota.PublishKindNovel, 0) {
		return
	}

	var input CreateNovelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 标题必填（trim 后判空，避免纯空格标题入库）
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		infra.BadRequest(c, "请输入作品标题")
		return
	}

	// 净化 HTML
	input.Summary = htmlSanitizer.Sanitize(input.Summary)

	status := input.Status
	if status == "" {
		status = "draft"
	}

	tags := input.Tags
	if tags == "" {
		tags = "[]"
	}

	sourceType := input.SourceType
	if sourceType == "" {
		sourceType = "original"
	}
	creationMethod := input.CreationMethod
	if creationMethod == "" {
		creationMethod = "human"
	}

	novel := &models.Novel{
		AuthorID:        userID,
		Title:           input.Title,
		Category:        input.Category,
		Tags:            tags,
		Summary:         input.Summary,
		Mode:            normalizeNovelMode(input.Mode),
		PricePerChapter: input.PricePerChapter,
		Status:          status,
		SourceType:      sourceType,
		CreationMethod:  creationMethod,
		CommentsEnabled: input.CommentsEnabled == nil || *input.CommentsEnabled,
	}
	// #76 同人衍生处理
	switch input.DerivativeType {
	case "site":
		if input.DerivativeOriginID == 0 {
			infra.BadRequest(c, "请选择引用的站内原著")
			return
		}
		origin, oerr := models.GetNovelByID(input.DerivativeOriginID)
		if oerr != nil || origin.Status != "published" {
			infra.BadRequest(c, "引用的原著不存在或未发布")
			return
		}
		novel.DerivativeType = "site"
		novel.DerivativeOriginID = origin.ID
		novel.DerivativeOriginTitle = origin.Title
		if origin.Author != nil {
			novel.DerivativeOriginAuthor = origin.Author.Nickname
		}
	case "external":
		src := strings.TrimSpace(input.DerivativeExternalSource)
		if src == "" {
			infra.BadRequest(c, "站外同人请填写来源标注")
			return
		}
		novel.DerivativeType = "external"
		novel.DerivativeExternalSource = src
	case "":
		// 默认原创
	default:
		infra.BadRequest(c, "无效的同人类型")
		return
	}
	if !applyPermissionInput(c, novel, input.ReadPermission, input.PaidPrice) {
		return
	}

	if err := models.CreateNovel(novel); err != nil {
		infra.InternalError(c, "创建作品失败")
		return
	}
	// 发布流水记账（计入每日发文限额）
	quota.RecordPublish(userID, quota.PublishKindNovel, novel.ID, 0)

	// 保存多分类
	SaveNovelCategories(novel.ID, input.Categories, input.Category)
	// 如果只有单分类，确保向后兼容
	if input.Category == "" && len(input.Categories) > 0 {
		novel.Category = input.Categories[0]
		models.DB.Model(novel).Update("category", novel.Category)
	}
	// 填充返回用的分类名
	novel.CategoryNames = getCategoryNames(input.Categories, input.Category)

	// 创建作者目录
	authorDir := filepath.Join(config.NovelDataDir, "authors", fmt.Sprintf("%d", userID))
	novelDir := filepath.Join(authorDir, fmt.Sprintf("%d", novel.ID))
	os.MkdirAll(novelDir, 0755)

	// 创建 index.json
	indexPath := filepath.Join(novelDir, "index.json")
	indexData := map[string]interface{}{
		"novel_id": novel.ID,
		"title":    novel.Title,
		"chapters": []interface{}{},
	}
	indexBytes, _ := json.MarshalIndent(indexData, "", "  ")
	os.WriteFile(indexPath, indexBytes, 0644)

	// AI 内容审核（作品简介/标题也属于公开内容，与章节发布同等对待）
	go moderation.ReviewOnWrite(moderation.ScopeNovel, novel.ID)

	infra.SuccessMessage(c, "创建成功", novel)
}

// PUT /api/novels/:id
func UpdateNovel(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	userRole := hctx.CurrentUserRole(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		infra.BadRequest(c, "无效的作品 ID")
		return
	}

	novel, err := models.GetNovelByID(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			infra.NotFound(c, "作品不存在")
			return
		}
		infra.InternalError(c, "获取作品失败")
		return
	}

	// 只有作者本人或管理员可以编辑
	if novel.AuthorID != userID && userRole != "admin" {
		infra.Forbidden(c, "无权编辑此作品")
		return
	}

	var input UpdateNovelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		infra.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	if input.Title != "" {
		novel.Title = input.Title
	}
	if input.Category != "" {
		novel.Category = input.Category
	}
	if input.Tags != "" {
		novel.Tags = input.Tags
	}
	if input.Summary != "" {
		novel.Summary = htmlSanitizer.Sanitize(input.Summary)
	}
	if input.CoverURL != "" {
		novel.CoverURL = input.CoverURL
	}
	if input.PricePerChapter >= 0 {
		novel.PricePerChapter = input.PricePerChapter
	}
	if input.Status != "" {
		novel.Status = input.Status
	}
	if input.SourceType != "" {
		novel.SourceType = input.SourceType
	}
	if input.Mode != "" {
		novel.Mode = normalizeNovelMode(input.Mode)
	}
	if input.CreationMethod != "" {
		novel.CreationMethod = input.CreationMethod
	}
	// #76 同人衍生处理（空字符串=不改动；"original"=清除衍生）
	switch input.DerivativeType {
	case "site":
		if input.DerivativeOriginID == 0 {
			infra.BadRequest(c, "请选择引用的站内原著")
			return
		}
		origin, oerr := models.GetNovelByID(input.DerivativeOriginID)
		if oerr != nil || origin.Status != "published" {
			infra.BadRequest(c, "引用的原著不存在或未发布")
			return
		}
		novel.DerivativeType = "site"
		novel.DerivativeOriginID = origin.ID
		novel.DerivativeOriginTitle = origin.Title
		if origin.Author != nil {
			novel.DerivativeOriginAuthor = origin.Author.Nickname
		}
		novel.DerivativeExternalSource = ""
	case "external":
		src := strings.TrimSpace(input.DerivativeExternalSource)
		if src == "" {
			infra.BadRequest(c, "站外同人请填写来源标注")
			return
		}
		novel.DerivativeType = "external"
		novel.DerivativeOriginID = 0
		novel.DerivativeOriginTitle = ""
		novel.DerivativeOriginAuthor = ""
		novel.DerivativeExternalSource = src
	case "original":
		novel.DerivativeType = ""
		novel.DerivativeOriginID = 0
		novel.DerivativeOriginTitle = ""
		novel.DerivativeOriginAuthor = ""
		novel.DerivativeExternalSource = ""
	case "":
		// 未传，不改动
	}
	// 作者隔离开关：true=外人需登录才能查看本作品（对应 3.2 作者自决）
	if input.WallEnabled != nil {
		novel.WallEnabled = *input.WallEnabled
	}
	// 评论区开关：nil=不修改，非 nil 直接覆盖（Save 全字段持久化）
	if input.CommentsEnabled != nil {
		novel.CommentsEnabled = *input.CommentsEnabled
	}
	if !applyPermissionInput(c, novel, input.ReadPermission, input.PaidPrice) {
		return
	}

	if err := models.UpdateNovel(novel); err != nil {
		infra.InternalError(c, "更新作品失败")
		return
	}

	// 更新多分类（只在显式传入时更新）
	if input.Categories != nil || input.Category != "" {
		SaveNovelCategories(novel.ID, input.Categories, input.Category)
		if len(input.Categories) > 0 {
			models.DB.Model(novel).Update("category", input.Categories[0])
		} else if input.Category != "" {
			models.DB.Model(novel).Update("category", input.Category)
		}
		novel.CategoryNames = getCategoryNames(input.Categories, input.Category)
	} else {
		// 加载已有分类
		novel.CategoryNames = loadCategoryNames(novel.ID)
	}

	// 简介/标题改动后重新送检（否则改标题即可绕过首次审核）
	go moderation.ReviewOnWrite(moderation.ScopeNovel, novel.ID)

	infra.SuccessMessage(c, "更新成功", novel)
}

// DELETE /api/novels/:id
func DeleteNovel(c *gin.Context) {
	userID := hctx.CurrentUserID(c)
	userRole := hctx.CurrentUserRole(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		infra.BadRequest(c, "无效的作品 ID")
		return
	}

	novel, err := models.GetNovelByID(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			infra.NotFound(c, "作品不存在")
			return
		}
		infra.InternalError(c, "获取作品失败")
		return
	}

	if novel.AuthorID != userID && userRole != "admin" {
		infra.Forbidden(c, "无权删除此作品")
		return
	}

	// 作者自删：先清除封禁原因标记，避免误显示 176 号令封禁横幅（ban_reason 非空才视为封禁）
	_ = models.DB.Model(&models.Novel{}).Where("id = ?", id).Update("ban_reason", "")
	_ = models.DB.Model(&models.Chapter{}).Where("novel_id = ?", id).Update("ban_reason", "")

	if err := models.DeleteNovel(uint(id)); err != nil {
		infra.InternalError(c, "删除失败")
		return
	}

	// 标记为「用户自删」（区别于管理员封禁：deleted_by='admin'），回收站据此区分
	_ = models.DB.Unscoped().Model(&models.Novel{}).Where("id = ?", id).Update("deleted_by", "user")

	// 合规留存：软删除，保留正文文件（不物理移除），审计可追溯原始内容
	infra.SuccessMessage(c, "删除成功", nil)
}

// novelOwnershipCheck 检查当前用户是否是作品的作者
func novelOwnershipCheck(c *gin.Context, novelID uint) (*models.Novel, bool) {
	userID := hctx.CurrentUserID(c)
	userRole := hctx.CurrentUserRole(c)

	novel, err := models.GetNovelByID(novelID)
	if err != nil {
		return nil, false
	}

	if novel.AuthorID != userID && userRole != "admin" {
		return novel, false
	}

	return novel, true
}

// SaveNovelCategories 保存作品的多分类记录
func SaveNovelCategories(novelID uint, categories []string, fallbackCategory string) {
	// 先删除旧分类
	models.DB.Where("novel_id = ?", novelID).Delete(&models.NovelCategory{})

	// 合并 categories 和 fallbackCategory
	categorySet := make(map[string]bool)
	for _, cat := range categories {
		cat = strings.TrimSpace(cat)
		if cat != "" {
			categorySet[cat] = true
		}
	}
	if fallbackCategory != "" {
		categorySet[strings.TrimSpace(fallbackCategory)] = true
	}

	// 插入新分类
	for cat := range categorySet {
		models.DB.Create(&models.NovelCategory{
			NovelID:  novelID,
			Category: cat,
		})
	}
}

// getCategoryNames 从前端输入获取分类名列表
func getCategoryNames(categories []string, fallbackCategory string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, cat := range categories {
		cat = strings.TrimSpace(cat)
		if cat != "" && !seen[cat] {
			seen[cat] = true
			result = append(result, cat)
		}
	}
	if fallbackCategory != "" {
		fallbackCategory = strings.TrimSpace(fallbackCategory)
		if !seen[fallbackCategory] {
			result = append(result, fallbackCategory)
		}
	}
	if len(result) == 0 {
		result = append(result, "其他")
	}
	return result
}

// loadCategoryNames 从数据库加载作品的分类名列表
func loadCategoryNames(novelID uint) []string {
	var cats []models.NovelCategory
	models.DB.Where("novel_id = ?", novelID).Find(&cats)
	var result []string
	for _, c := range cats {
		result = append(result, c.Category)
	}
	if len(result) == 0 {
		result = append(result, "其他")
	}
	return result
}

// normalizeNovelMode 归一化叙事形态：prose（散文体，默认）/ script（话本气泡体）
func normalizeNovelMode(m string) string {
	if m == "script" {
		return "script"
	}
	return "prose"
}

// normalizeChapterMode 章节叙事形态：”=继承作品（默认）/ prose / script / mixed（散文与话本混排）
func normalizeChapterMode(m string) string {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case "prose":
		return "prose"
	case "script":
		return "script"
	case "mixed":
		return "mixed"
	default:
		return "" // 继承作品形态
	}
}

// effectiveChapterMode 章节生效形态：章节显式指定优先，否则继承作品
func effectiveChapterMode(chapter *models.Chapter, novel *models.Novel) string {
	if chapter != nil && chapter.Mode != "" {
		return chapter.Mode
	}
	return novelMode(novel)
}
