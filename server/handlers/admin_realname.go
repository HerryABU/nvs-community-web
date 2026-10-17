// admin_realname.go — 管理端实名记录查询 + 实名核验提供商配置（F10 / F21.1 / 第三方接口）。
// 姓名解密后仅在管理员响应中返回；列表默认脱敏，详情可查看明文（取证需要）。
package handlers

import (
	"strconv"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/realname"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GET /api/admin/realname — 实名用户列表（分页 + 关键词 + minor_only 未成年筛选）
// query: page, page_size, keyword(匹配用户名/昵称/身份证脱敏串), minor_only=true
func AdminListRealName(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	minorOnly := c.Query("minor_only") == "true"

	q := models.DB.Model(&models.User{}).Where("real_name_verified = ?", true)
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR nickname LIKE ? OR id_card_masked LIKE ?", like, like, like)
	}
	if minorOnly {
		q = q.Where("minor_protected = ?", true)
	}

	var total int64
	q.Count(&total)

	// 全局未成年总数（不受筛选影响，供统计卡展示）
	var minorTotal int64
	models.DB.Model(&models.User{}).Where("real_name_verified = ? AND minor_protected = ?", true, true).Count(&minorTotal)

	var users []models.User
	if err := q.Order("real_name_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		utils.InternalError(c, "查询失败")
		return
	}

	rows := make([]gin.H, 0, len(users))
	for i := range users {
		u := &users[i]
		name := ""
		if dec, err := utils.DecryptField(u.RealName); err == nil {
			name = dec
		}
		age := 0
		if u.BirthDate != nil {
			age = utils.AgeAt(*u.BirthDate, time.Now())
		}
		rows = append(rows, gin.H{
			"user_id":         u.ID,
			"username":        u.Username,
			"nickname":        u.Nickname,
			"role":            u.Role,
			"phone":           u.Phone,
			"real_name":       name,
			"id_card_masked":  u.IDCardMasked,
			"minor_protected": u.MinorProtected,
			"age":             age,
			"real_name_at":    u.RealNameAt,
		})
	}

	utils.Success(c, gin.H{
		"list":        rows,
		"total":       total,
		"minor_total": minorTotal,
		"page":        page,
		"page_size":   pageSize,
	})
}

// ============ 实名核验提供商配置（管理后台「实名与未成年保护」页） ============

// maskRealNameSecrets 把配置中的敏感字段置空并记录是否已配置
func maskRealNameSecrets(c realname.Config) (realname.Config, map[string]bool) {
	has := map[string]bool{}
	if c.National.Token != "" {
		has["national_token"] = true
		c.National.Token = ""
	}
	if c.Tencent.SecretKey != "" {
		has["tencent_secret_key"] = true
		c.Tencent.SecretKey = ""
	}
	if c.Alibaba.AppCode != "" {
		has["alibaba_app_code"] = true
		c.Alibaba.AppCode = ""
	}
	if c.Huawei.SK != "" {
		has["huawei_sk"] = true
		c.Huawei.SK = ""
	}
	if c.Netease.SecretKey != "" {
		has["netease_secret_key"] = true
		c.Netease.SecretKey = ""
	}
	return c, has
}

// GET /api/admin/realname-provider — 当前提供商配置（敏感字段脱敏）
func AdminGetRealNameProvider(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg := models.GetRealNameProviderConfig()
	masked, has := maskRealNameSecrets(cfg)
	utils.Success(c, gin.H{
		"config":     masked,
		"has_secret": has,
		"providers":  realname.Providers(),
	})
}

// PUT /api/admin/realname-provider — 保存提供商配置
// 敏感字段留空 = 沿用已保存值（避免回显明文又被空白覆盖）
func AdminSetRealNameProvider(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req realname.Config
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	req.Provider = strings.TrimSpace(strings.ToLower(req.Provider))
	if req.Provider != "" {
		if _, ok := realname.Get(req.Provider); !ok {
			utils.BadRequest(c, "未知的提供商: "+req.Provider)
			return
		}
	}
	// 留空的敏感字段沿用旧值
	old := models.GetRealNameProviderConfig()
	if req.National.Token == "" {
		req.National.Token = old.National.Token
	}
	if req.Tencent.SecretKey == "" {
		req.Tencent.SecretKey = old.Tencent.SecretKey
	}
	if req.Alibaba.AppCode == "" {
		req.Alibaba.AppCode = old.Alibaba.AppCode
	}
	if req.Huawei.SK == "" {
		req.Huawei.SK = old.Huawei.SK
	}
	if req.Netease.SecretKey == "" {
		req.Netease.SecretKey = old.Netease.SecretKey
	}
	if err := models.SetRealNameProviderConfig(req); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "提供商配置已保存", nil)
}

// POST /api/admin/realname-provider/test — 提供商连通性测试
// 用「测试姓名 + 必然非法的身份证号」探测链路：各厂商对非法输入均不收费，
// 能区分「链路/签名故障」与「核验不一致」两种状态。
// body: { real_name?, id_card? } 均可选，缺省用内置测试值
func AdminTestRealNameProvider(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		RealName string `json:"real_name"`
		IDCard   string `json:"id_card"`
	}
	_ = c.ShouldBindJSON(&req)
	name := strings.TrimSpace(req.RealName)
	if name == "" {
		name = "张三四"
	}
	idCard := strings.ToUpper(strings.TrimSpace(req.IDCard))
	if idCard == "" {
		idCard = "110101190001010000" // 校验位非法的测试号
	}
	cfg := models.GetRealNameProviderConfig()
	if cfg.Provider == "" {
		utils.BadRequest(c, "尚未选择提供商")
		return
	}
	res, err := realname.Verify(c.Request.Context(), cfg, name, idCard)
	if err != nil {
		utils.Success(c, gin.H{
			"ok":      false,
			"message": "链路测试失败：" + err.Error(),
			"hint":    "请检查凭据是否正确、服务是否开通、网络是否可达",
		})
		return
	}
	utils.Success(c, gin.H{
		"ok":      true,
		"matched": res.Matched,
		"code":    res.Code,
		"message": "链路通畅（提供商返回：" + res.Message + "）",
	})
}
