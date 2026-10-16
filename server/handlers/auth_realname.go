// auth_realname.go — 实名制申请/状态查询 + 国家实名检测 API 代理（F10 / F21.1）。
// 合规要点（公安部令第176号 + 未成年人网络保护条例）：
//   - 姓名以 AES-256-GCM 加密落库；身份证号仅存 sha256 哈希（查重）与脱敏串（展示），绝不存明文；
//   - 国家实名核验为「代理」模式：姓名+身份证号仅随请求转发至国家 API，服务器不落库、不记日志明文。
package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// POST /api/auth/realname/apply — 提交实名（需登录）
// body: { real_name, id_card }
func ApplyRealName(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	var req struct {
		RealName string `json:"real_name" binding:"required"`
		IDCard   string `json:"id_card" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写姓名与身份证号")
		return
	}
	name := strings.TrimSpace(req.RealName)
	idCard := strings.ToUpper(strings.TrimSpace(req.IDCard))
	if utils.RuneLen(name) < 2 || utils.RuneLen(name) > 32 {
		utils.BadRequest(c, "姓名长度应为 2-32 个字符")
		return
	}
	birth, err := utils.ValidateIDCard(idCard)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// 一号一实名：身份证哈希已被他人使用则拒绝（防买卖实名/一号多号）
	idHash := sha256Hex("idcard|" + idCard)
	var dup int64
	models.DB.Model(&models.User{}).Where("id_card_hash = ? AND id != ?", idHash, userID).Count(&dup)
	if dup > 0 {
		utils.BadRequest(c, "该身份证号已完成实名，无法重复绑定")
		return
	}

	encrypted, err := utils.EncryptField(name)
	if err != nil {
		utils.InternalError(c, "实名信息加密失败")
		return
	}
	age := utils.AgeAt(birth, time.Now())
	now := time.Now()
	updates := map[string]interface{}{
		"real_name":          encrypted,
		"id_card_hash":       idHash,
		"id_card_masked":     utils.MaskIDCard(idCard),
		"real_name_verified": true,
		"real_name_at":       &now,
		"birth_date":         &birth,
		"minor_protected":    age < 18,
	}
	if err := models.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		utils.InternalError(c, "实名信息保存失败")
		return
	}

	recordUserAudit(c, userID, models.AuditChangeProfile, "完成实名认证", "success")
	utils.SuccessMessage(c, "实名认证成功", gin.H{
		"id_card_masked":  utils.MaskIDCard(idCard),
		"minor_protected": age < 18,
	})
}

// GET /api/auth/realname — 查询我的实名状态（需登录）
func GetMyRealName(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	user, err := models.GetUserByID(userID)
	if err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	utils.Success(c, gin.H{
		"real_name_verified": user.RealNameVerified,
		"id_card_masked":     user.IDCardMasked,
		"birth_date":         user.BirthDate,
		"minor_protected":    user.MinorProtected,
	})
}

// NationalRealNameResp 国家实名检测 API 的标准化返回
type NationalRealNameResp struct {
	Matched        bool   `json:"matched"`           // 姓名与身份证是否一致
	Status         string `json:"status"`            // 原始状态串（透传，便于排查）
	Message        string `json:"message"`           // 提示信息
	MinorProtected bool   `json:"minor_protected"`   // 核验通过时按证件推算的未成年标记
}

// POST /api/auth/realname/national-check — 国家实名检测 API 代理（需登录）
// 两种模式：
//   - 普通模式（advanced=false）：核验通过即完成实名落库（姓名加密、身份证仅哈希+脱敏）；
//   - 高级模式（advanced=true）：仅验证年龄，服务器不存储姓名与身份证号，
//     只按证件推算出生日期与未成年标记落库（未成年人网络保护「年龄验证」场景）。
// body: { real_name, id_card, advanced? }
func NationalRealNameCheck(c *gin.Context) {
	userID := currentUserID(c)
	if userID == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	cfg := models.GetNationalRealNameConfig()
	if !cfg.Enabled || cfg.URL == "" {
		utils.BadRequest(c, "站长尚未开启国家实名检测")
		return
	}
	var req struct {
		RealName  string `json:"real_name" binding:"required"`
		IDCard    string `json:"id_card" binding:"required"`
		Advanced  bool   `json:"advanced"` // 高级模式：只验龄，不落身份信息
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写姓名与身份证号")
		return
	}
	name := strings.TrimSpace(req.RealName)
	idCard := strings.ToUpper(strings.TrimSpace(req.IDCard))
	if _, err := utils.ValidateIDCard(idCard); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"real_name": name,
		"id_card":   idCard,
		"token":     cfg.Token,
	})
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, cfg.URL, bytes.NewReader(payload))
	if err != nil {
		utils.InternalError(c, "实名检测请求构造失败")
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		utils.InternalError(c, "实名检测服务暂时不可用")
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		utils.InternalError(c, "实名检测响应读取失败")
		return
	}

	// 兼容多种常见响应形态：{"code":0,...} / {"status":"match"} / {"result":true}
	var raw map[string]interface{}
	_ = json.Unmarshal(body, &raw)
	out := NationalRealNameResp{Message: "核验完成"}
	if v, ok := raw["code"].(float64); ok {
		out.Matched = v == 0
		out.Status = "code=" + itoa(int(v))
	}
	if v, ok := raw["status"].(string); ok {
		low := strings.ToLower(v)
		out.Matched = low == "match" || low == "consistent" || low == "一致"
		out.Status = v
	}
	if v, ok := raw["result"].(bool); ok {
		out.Matched = v
	}
	if v, ok := raw["message"].(string); ok && v != "" {
		out.Message = v
	}

	// 核验通过则落库。高级模式只验龄：仅按证件推算出生日期与未成年标记，
	// 不存姓名/身份证哈希/脱敏串，real_name_verified 保持原值。
	if out.Matched {
		birth, verr := utils.ValidateIDCard(idCard)
		if verr == nil {
			ageMinor := utils.AgeAt(birth, time.Now()) < 18
			out.MinorProtected = ageMinor
			if req.Advanced {
				_ = models.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
					"birth_date":      &birth,
					"minor_protected": ageMinor,
				}).Error
				recordUserAudit(c, userID, models.AuditChangeProfile, "国家实名核验通过（高级模式：仅验证年龄，不存储身份信息）", "success")
			} else if encrypted, eerr := utils.EncryptField(name); eerr == nil {
				idHash := sha256Hex("idcard|" + idCard)
				var dup int64
				models.DB.Model(&models.User{}).Where("id_card_hash = ? AND id != ?", idHash, userID).Count(&dup)
				if dup == 0 {
					now := time.Now()
					_ = models.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
						"real_name":          encrypted,
						"id_card_hash":       idHash,
						"id_card_masked":     utils.MaskIDCard(idCard),
						"real_name_verified": true,
						"real_name_at":       &now,
						"birth_date":         &birth,
						"minor_protected":    ageMinor,
					}).Error
					recordUserAudit(c, userID, models.AuditChangeProfile, "国家实名核验通过并完成实名", "success")
				}
			}
		}
	}

	utils.Success(c, out)
}
