package models

import (
	"testing"

	"nvs-server/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupFloorPasswordTestDB 独立内存库（唯一名，避免与同包其他测试共享污染）
func setupFloorPasswordTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:floorpwd_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := AutoMigrateAll(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// seedFloor 造一栋楼：顶层主贴 + 两条楼内回复
func seedFloor(t *testing.T, authorID uint, withPassword bool) (*Comment, []Comment) {
	t.Helper()
	hash := ""
	if withPassword {
		h, err := utils.HashPassword("secret123")
		if err != nil {
			t.Fatalf("哈希密码失败: %v", err)
		}
		hash = h
	}
	floor := &Comment{
		UserID: authorID, NovelID: 1, Content: "楼主正文", FloorNo: 1,
		HasPassword: withPassword, PasswordHash: hash,
	}
	if err := DB.Create(floor).Error; err != nil {
		t.Fatalf("造楼失败: %v", err)
	}
	replies := []Comment{
		{UserID: authorID + 1, NovelID: 1, Content: "楼内回复甲", ParentID: &floor.ID, RootID: &floor.ID},
		{UserID: authorID + 2, NovelID: 1, Content: "楼内回复乙", ParentID: &floor.ID, RootID: &floor.ID},
	}
	for i := range replies {
		if err := DB.Create(&replies[i]).Error; err != nil {
			t.Fatalf("造回复失败: %v", err)
		}
	}
	return floor, replies
}

// TestFloorAccessUnlocked 解锁判定：无密码/作者/staff/密码对错
func TestFloorAccessUnlocked(t *testing.T) {
	setupFloorPasswordTestDB(t)
	floor, _ := seedFloor(t, 100, true)

	cases := []struct {
		name     string
		viewer   uint
		isStaff  bool
		password string
		want     bool
	}{
		{"楼作者本人", 100, false, "", true},
		{"管理员", 999, true, "", true},
		{"陌生人无密码", 200, false, "", false},
		{"陌生人错密码", 200, false, "wrong", false},
		{"陌生人正确密码", 200, false, "secret123", true},
		{"游客（未登录）", 0, false, "secret123", true},
	}
	for _, c := range cases {
		if got := FloorAccessUnlocked(floor, c.viewer, c.isStaff, map[uint]string{floor.ID: c.password}); got != c.want {
			t.Errorf("%s：got %v want %v", c.name, got, c.want)
		}
	}

	// 未设密码的楼：任何人直接放行
	plain, _ := seedFloor(t, 300, false)
	if !FloorAccessUnlocked(plain, 9999, false, nil) {
		t.Error("未设密码的楼应对任何人放行")
	}
}

// TestRedactLockedComments 脱敏：加密楼 content 置空 + Locked 标记 + 楼内回复剔除
func TestRedactLockedComments(t *testing.T) {
	setupFloorPasswordTestDB(t)
	floor, _ := seedFloor(t, 100, true)
	plainFloor, _ := seedFloor(t, 300, false)

	var list []Comment
	DB.Preload("User").Where("novel_id = ?", 1).Find(&list)

	// 陌生人无密码：加密楼脱敏、楼内回复剔除；普通楼原样
	out := RedactLockedComments(list, 200, false, nil)
	var lockedCnt, plainCnt, replyCnt int
	for _, c := range out {
		if c.ID == floor.ID {
			lockedCnt++
			if !c.Locked || c.Content != "" {
				t.Errorf("加密楼应 Locked 且 content 置空，got locked=%v content=%q", c.Locked, c.Content)
			}
		}
		if c.ID == plainFloor.ID {
			plainCnt++
			if c.Locked || c.Content != "楼主正文" {
				t.Errorf("普通楼不应脱敏，got locked=%v content=%q", c.Locked, c.Content)
			}
		}
		if c.ParentID != nil {
			replyCnt++
		}
	}
	if lockedCnt != 1 {
		t.Errorf("加密楼应保留占位行，got %d", lockedCnt)
	}
	if replyCnt != 2 {
		t.Errorf("只剩普通楼的 2 条回复应保留，实际 %d", replyCnt)
	}

	// 带对密码：加密楼完整返回
	out2 := RedactLockedComments(list, 200, false, map[uint]string{floor.ID: "secret123"})
	for _, c := range out2 {
		if c.ID == floor.ID && (c.Locked || c.Content != "楼主正文") {
			t.Errorf("解锁后加密楼应完整返回，got locked=%v content=%q", c.Locked, c.Content)
		}
	}

	// 管理员：不脱敏
	out3 := RedactLockedComments(list, 999, true, nil)
	for _, c := range out3 {
		if c.ID == floor.ID && c.Content != "楼主正文" {
			t.Errorf("管理员应看到完整内容，got %q", c.Content)
		}
	}
}

// TestSetFloorPasswordAndDetail 设密/解密 + 单楼整读
func TestSetFloorPasswordAndDetail(t *testing.T) {
	setupFloorPasswordTestDB(t)
	floor, _ := seedFloor(t, 100, false)

	// 楼内回复不可设密（仅顶层）
	reply := &Comment{UserID: 100, NovelID: 1, Content: "x", ParentID: &floor.ID, RootID: &floor.ID}
	DB.Create(reply)
	if err := SetFloorPassword(reply.ID, "hash", true); err != nil {
		t.Fatalf("SetFloorPassword 返回错误: %v", err)
	}
	var r Comment
	DB.First(&r, reply.ID)
	if r.HasPassword {
		t.Error("楼内回复不应被设密（parent_id IS NULL 条件应拦住）")
	}

	// 楼设密
	hash, _ := utils.HashPassword("newpass99")
	if err := SetFloorPassword(floor.ID, hash, true); err != nil {
		t.Fatalf("设密失败: %v", err)
	}
	var f Comment
	DB.First(&f, floor.ID)
	if !f.HasPassword || f.PasswordHash == "" {
		t.Fatal("设密后 has_password 应为 true 且 hash 非空")
	}

	// 单楼整读：楼 + 全部回复
	got, replies, err := GetFloorWithReplies(floor.ID)
	if err != nil {
		t.Fatalf("GetFloorWithReplies 失败: %v", err)
	}
	if got.Content != "楼主正文" || len(replies) != 3 {
		t.Errorf("单楼整读异常：content=%q replies=%d", got.Content, len(replies))
	}

	// 解密（空 hash + false）
	if err := SetFloorPassword(floor.ID, "", false); err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	DB.First(&f, floor.ID)
	if f.HasPassword {
		t.Error("解密后 has_password 应为 false")
	}
}

// TestSMSConfigFromDB 短信配置：DB 优先 + 未落库回退 env 默认（log/空）
func TestSMSConfigFromDB(t *testing.T) {
	setupFloorPasswordTestDB(t)

	// 未保存过：回退 config 包默认（测试环境 SMSGateway 为 log）
	gateway, url, smsToken, _, _, fromDB := GetSMSConfigFromDB()
	if fromDB {
		t.Error("未保存过时 fromDB 应为 false")
	}
	if gateway != "log" || url != "" {
		t.Errorf("回退值异常：gateway=%q url=%q", gateway, url)
	}

	// 管理端保存 http 网关
	_ = SetPlatformConfig(SMSGatewayConfigKey, "http")
	_ = SetPlatformConfig(SMSHTTPURLConfigKey, "https://sms.example.com/send")
	_ = SetPlatformConfig(SMSHTTPTokenConfigKey, "tok-123")
	gateway, url, smsToken, _, _, fromDB = GetSMSConfigFromDB()
	if !fromDB || gateway != "http" || url != "https://sms.example.com/send" || smsToken != "tok-123" {
		t.Errorf("DB 值未优先生效：fromDB=%v gateway=%q url=%q token=%q", fromDB, gateway, url, smsToken)
	}

	// 主动清空 URL（DB 空串以 DB 为准，不回填 env）
	_ = SetPlatformConfig(SMSHTTPURLConfigKey, "")
	_, url, _, _, _, _ = GetSMSConfigFromDB()
	if url != "" {
		t.Errorf("主动清空后 URL 应为空，got %q", url)
	}

	// 非法网关值归一为 log
	_ = SetPlatformConfig(SMSGatewayConfigKey, "bogus")
	gateway, _, _, _, _, _ = GetSMSConfigFromDB()
	if gateway != "log" {
		t.Errorf("非法网关应归一为 log，got %q", gateway)
	}
}

// TestWeChatConfigFromDB 微信配置：DB 优先 + 未落库回退 env
func TestWeChatConfigFromDB(t *testing.T) {
	setupFloorPasswordTestDB(t)

	// 未保存：回退 env（测试环境通常为空）
	appID, _, _, fromDB := GetWeChatConfigFromDB()
	if fromDB {
		t.Error("未保存过时 fromDB 应为 false")
	}
	if appID != "" {
		t.Errorf("测试环境 env 应为空，got %q", appID)
	}
	if GetWeChatLoginAvailable() {
		t.Error("无配置时微信扫码应不可用")
	}

	// 保存三件套 → 可用
	_ = SetPlatformConfig(WeChatAppIDConfigKey, "wx-appid")
	_ = SetPlatformConfig(WeChatAppSecretConfigKey, "wx-secret")
	_ = SetPlatformConfig(WeChatCallbackConfigKey, "https://site.example.com/api/auth/wechat/callback")
	appID, secret, callback, fromDB := GetWeChatConfigFromDB()
	if !fromDB || appID != "wx-appid" || secret != "wx-secret" || callback == "" {
		t.Errorf("DB 值未优先生效：fromDB=%v appID=%q secret=%q callback=%q", fromDB, appID, secret, callback)
	}
	if !GetWeChatLoginAvailable() {
		t.Error("三件套齐全时应可用")
	}

	// 清空 AppID → 不可用（即使 env 有值也不回填）
	_ = SetPlatformConfig(WeChatAppIDConfigKey, "")
	if GetWeChatLoginAvailable() {
		t.Error("清空 AppID 后应不可用")
	}
}
