package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nvs-server/models"

	"github.com/gin-gonic/gin"
)

// TestSetBirthDate 自报出生日期：成功置位 + 年龄推算
func TestSetBirthDate(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "minor_user", "reader")

	// 2012 年出生 → 当前应未成年
	c, w := asUser("reader", 1)
	setJSONBody(c, map[string]string{"birth_date": "2012-05-20"})
	SetBirthDate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("自报出生日期应成功，实际 %d: %s", w.Code, w.Body.String())
	}

	var user models.User
	if err := models.DB.First(&user, 1).Error; err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if user.BirthDate == nil {
		t.Fatal("出生日期应已写入")
	}
	if !user.MinorProtected {
		t.Error("2012 年出生应标记为未成年")
	}
}

// TestSetBirthDateRejectsFuture 未来日期拒绝
func TestSetBirthDateRejectsFuture(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "future_user", "reader")

	future := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
	c, w := asUser("reader", 1)
	setJSONBody(c, map[string]string{"birth_date": future})
	SetBirthDate(c)
	if w.Code == http.StatusOK {
		t.Fatalf("未来出生日期应被拒绝，实际 %d", w.Code)
	}
}

// TestSetBirthDateBlockedAfterRealName 已实名用户不可自报覆盖证件出生日期
func TestSetBirthDateBlockedAfterRealName(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "verified_user", "reader")
	if err := models.DB.Model(&models.User{}).Where("id = ?", 1).
		Updates(map[string]interface{}{"real_name_verified": true}).Error; err != nil {
		t.Fatalf("预置实名态失败: %v", err)
	}

	c, w := asUser("reader", 1)
	setJSONBody(c, map[string]string{"birth_date": "2015-01-01"})
	SetBirthDate(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("已实名用户自报应返回 403，实际 %d: %s", w.Code, w.Body.String())
	}
}

// TestMinorStatusNeedAsk minor_mode 开启且未填报时应提示询问
func TestMinorStatusNeedAsk(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "ask_user", "reader")

	// 默认 off：不需要询问
	c, w := asUser("reader", 1)
	c.Request = httptest.NewRequest("GET", "/", nil)
	GetMinorStatus(c)
	var resp struct {
		Data struct {
			NeedAsk       bool   `json:"need_ask"`
			BirthDateSet  bool   `json:"birth_date_set"`
			MinorMode     string `json:"minor_mode"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Data.NeedAsk {
		t.Error("minor_mode=off 时不应要求询问")
	}

	// 开启 soft 且未填报：应要求询问
	if err := models.SetPlatformConfig(models.MinorModeKey, string(models.MinorModeSoft)); err != nil {
		t.Fatalf("设置 minor_mode 失败: %v", err)
	}
	c2, w2 := asUser("reader", 1)
	c2.Request = httptest.NewRequest("GET", "/", nil)
	GetMinorStatus(c2)
	resp.Data.NeedAsk = false
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.Data.NeedAsk {
		t.Error("minor_mode=soft 且未填报出生日期时应要求询问")
	}
}

// TestMinorPublishRestricted strict 模式下拦截未成年人发布
func TestMinorPublishRestricted(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "strict_minor", "reader")
	seedTestUser(t, 2, "strict_adult", "reader")
	seedTestUser(t, 3, "strict_admin", "admin")
	birth := time.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := models.DB.Model(&models.User{}).Where("id = ?", 1).
		Updates(map[string]interface{}{"birth_date": &birth, "minor_protected": true}).Error; err != nil {
		t.Fatalf("预置未成年态失败: %v", err)
	}
	if err := models.SetPlatformConfig(models.MinorModeKey, string(models.MinorModeStrict)); err != nil {
		t.Fatalf("设置 strict 失败: %v", err)
	}

	// 未成年人 → 拦截
	c, _ := asUser("reader", 1)
	if !checkMinorPublishRestricted(c) {
		t.Error("strict 模式下未成年人发布应被拦截")
	}
	// 成年人 → 放行
	c2, _ := asUser("reader", 2)
	if checkMinorPublishRestricted(c2) {
		t.Error("strict 模式下成年人不应被拦截")
	}
	// 管理员 → 豁免
	c3, _ := asUser("admin", 3)
	if checkMinorPublishRestricted(c3) {
		t.Error("管理员应豁免未成年门禁")
	}

	// soft 模式 → 一律放行
	if err := models.SetPlatformConfig(models.MinorModeKey, string(models.MinorModeSoft)); err != nil {
		t.Fatalf("设置 soft 失败: %v", err)
	}
	c4, _ := asUser("reader", 1)
	if checkMinorPublishRestricted(c4) {
		t.Error("soft 模式下不应拦截发布")
	}
}

// TestNationalCheckAdvancedMode 高级模式：核验通过只写出生日期，不落身份信息
func TestNationalCheckAdvancedMode(t *testing.T) {
	setupHandlerTestDB(t)
	seedTestUser(t, 1, "adv_user", "reader")

	// mock 国家实名 API：永远返回一致
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok"}`))
	}))
	defer srv.Close()

	models.SetPlatformConfig(models.NationalRealNameKey, `{"enabled":true,"url":"`+srv.URL+`","token":"t"}`)
	cfg := models.GetNationalRealNameConfig()
	if !cfg.Enabled || cfg.URL != srv.URL {
		t.Fatalf("国家实名 API 配置未生效: %+v", cfg)
	}

	// 2010 年出生身份证：地址码 110101 + 出生日期 20100101 + 顺序码 019（17 位，校验位由辅助函数补齐）
	idCard := "11010120100101019"
	idCard += idCardCheckDigitForTest(idCard)

	c, w := asUser("reader", 1)
	setJSONBody(c, map[string]interface{}{
		"real_name": "测试",
		"id_card":   idCard,
		"advanced":  true,
	})
	NationalRealNameCheck(c)
	if w.Code != http.StatusOK {
		t.Fatalf("高级模式核验应成功，实际 %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Matched        bool `json:"matched"`
			MinorProtected bool `json:"minor_protected"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if !resp.Data.Matched || !resp.Data.MinorProtected {
		t.Fatalf("2010 年出生应核验通过且为未成年: %+v", resp.Data)
	}

	var user models.User
	if err := models.DB.First(&user, 1).Error; err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if user.BirthDate == nil {
		t.Error("高级模式应写入出生日期")
	}
	if user.RealNameVerified {
		t.Error("高级模式不应标记实名认证")
	}
	if user.IDCardHash != "" || user.IDCardMasked != "" {
		t.Error("高级模式不应存储身份证哈希/脱敏串")
	}
	if user.RealName != "" {
		t.Error("高级模式不应存储姓名")
	}
	if !user.MinorProtected {
		t.Error("高级模式应置未成年标记")
	}
}

// idCardCheckDigitForTest 计算 GB 11643-1999 校验位（仅供测试造号）
func idCardCheckDigitForTest(id17 string) string {
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	sum := 0
	for i, ch := range id17 {
		sum += int(ch-'0') * weights[i]
	}
	return "10X98765432"[sum%11 : sum%11+1]
}

// 确保 gin 测试模式只初始化一次
func init() { gin.SetMode(gin.TestMode) }
