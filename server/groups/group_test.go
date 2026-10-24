package groups

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupGroupTestDB 独立内存库（唯一名，避免与 models 包测试互相污染）
func setupGroupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// ⚠️ 内存库必须唯一命名：`file:groups_test?mode=memory&cache=shared` 是进程内同名共享库，
	// 同包多个测试 / `-count=N` / 同一测试重跑都会复用同一份数据，
	// 表现为「单独跑通过、全量跑或重复跑偶发失败」（本轮 TestUnreadAndMarkRead 就栽在这）。
	dsn := fmt.Sprintf("file:groups_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("主迁移失败: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("群组迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	t.Cleanup(func() {
		models.DB = old
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func mkUser(t *testing.T, username, role string) *models.User {
	t.Helper()
	u := &models.User{Username: username, Email: username + "@example.com", PasswordHash: "x", Role: role}
	if err := models.DB.Create(u).Error; err != nil {
		t.Fatalf("建用户失败: %v", err)
	}
	return u
}

// TestCreateGroupQuotaAndNumber 建群：群号生成 + 角色额度 + 未成年拦截
func TestCreateGroupQuotaAndNumber(t *testing.T) {
	setupGroupTestDB(t)
	reader := mkUser(t, "g_reader", models.RoleReader)
	author := mkUser(t, "g_author", models.RoleAuthor)
	minor := mkUser(t, "g_minor", models.RoleReader)
	models.DB.Model(&models.User{}).Where("id = ?", minor.ID).Update("minor_protected", true)
	minor.MinorProtected = true

	g, err := CreateGroup(reader.ID, reader, GroupInput{Name: "读书会"})
	if err != nil {
		t.Fatalf("建群失败: %v", err)
	}
	if !regexp.MustCompile(`^\d{7}$`).MatchString(g.GroupNo) {
		t.Fatalf("群号应为 7 位数字，得到 %q", g.GroupNo)
	}
	if g.Status != GroupStatusActive || g.MemberCount != 1 {
		t.Fatalf("初始状态异常: %+v", g)
	}
	// 读者默认额度 1 个
	if _, err := CreateGroup(reader.ID, reader, GroupInput{Name: "第二个群"}); err == nil {
		t.Fatalf("读者第 2 个群应被额度拦截")
	}
	// 作者默认额度 3 个
	if _, err := CreateGroup(author.ID, author, GroupInput{Name: "作者群"}); err != nil {
		t.Fatalf("作者建群应成功: %v", err)
	}
	// 未成年禁止建群
	if _, err := CreateGroup(minor.ID, minor, GroupInput{Name: "未成年群"}); err == nil {
		t.Fatalf("未成年人建群应被拒绝")
	}
	// 按人覆盖：给读者放开到 5
	if err := SetUserGroupQuota(reader.ID, 5, "测试放开"); err != nil {
		t.Fatalf("设置按人额度失败: %v", err)
	}
	if _, err := CreateGroup(reader.ID, reader, GroupInput{Name: "第二个群"}); err != nil {
		t.Fatalf("放开后应可建群: %v", err)
	}
}

// TestJoinModesAndVisibility 加入方式与内容可见性
func TestJoinModesAndVisibility(t *testing.T) {
	setupGroupTestDB(t)
	owner := mkUser(t, "j_owner", models.RoleAuthor)
	m1 := mkUser(t, "j_u1", models.RoleReader)
	m2 := mkUser(t, "j_u2", models.RoleReader)
	m3 := mkUser(t, "j_u3", models.RoleReader)

	openG, err := CreateGroup(owner.ID, owner, GroupInput{Name: "开放群", JoinMode: GroupJoinOpen})
	if err != nil {
		t.Fatalf("建群失败: %v", err)
	}
	if st, err := JoinGroup(openG, m1, ""); err != nil || st != GroupMemberActive {
		t.Fatalf("open 群应直接加入: %v %s", err, st)
	}

	applyG, err := CreateGroup(owner.ID, owner, GroupInput{Name: "申请群", JoinMode: GroupJoinApply,
		Visibility: GroupVisibilityPrivate})
	if err != nil {
		t.Fatalf("建群失败: %v", err)
	}
	if st, err := JoinGroup(applyG, m2, "想加入"); err != nil || st != GroupMemberPending {
		t.Fatalf("apply 群应进入待审: %v %s", err, st)
	}
	// 待审成员不能读私密内容
	if CanReadGroupContent(applyG, GroupRoleMember, GroupMemberPending) {
		t.Fatalf("待审成员不应读取私密群内容")
	}
	if CanReadGroupContent(applyG, "", "") {
		t.Fatalf("非成员不应读取私密群内容")
	}

	inviteG, err := CreateGroup(owner.ID, owner, GroupInput{Name: "邀请群", JoinMode: GroupJoinInvite})
	if err != nil {
		t.Fatalf("建群失败: %v", err)
	}
	if _, err := JoinGroup(inviteG, m3, "WRONGCODE"); err == nil {
		t.Fatalf("错误邀请码应被拒绝")
	}
	if st, err := JoinGroup(inviteG, m3, inviteG.InviteCode); err != nil || st != GroupMemberActive {
		t.Fatalf("正确邀请码应加入成功: %v %s", err, st)
	}
}

// TestPostReviewFlow 先审后发：pending → 群管通过 → 可回帖
func TestPostReviewFlow(t *testing.T) {
	setupGroupTestDB(t)
	owner := mkUser(t, "p_owner", models.RoleAuthor)
	member := mkUser(t, "p_member", models.RoleReader)
	g, err := CreateGroup(owner.ID, owner, GroupInput{Name: "审核群", Visibility: GroupVisibilitySemi})
	if err != nil {
		t.Fatalf("建群失败: %v", err)
	}
	if _, err := JoinGroup(g, member, ""); err != nil {
		t.Fatalf("加入失败: %v", err)
	}
	if !g.ReviewFirst {
		t.Fatalf("半公开群应默认先审后发")
	}
	p, err := CreateGroupPost(g, member, "第一篇", "内容内容", false)
	if err != nil {
		t.Fatalf("发帖失败: %v", err)
	}
	if p.Status != GroupPostPending {
		t.Fatalf("先审后发群的新帖应为 pending，得到 %s", p.Status)
	}
	// 未过审不能回帖
	if _, err := CreateGroupReply(g, p.ID, member, "抢沙发"); err == nil {
		t.Fatalf("待审帖不应允许回帖")
	}
	// 群主审核通过
	if err := ReviewGroupPost(p.ID, owner.ID, models.RoleAuthor, true); err != nil {
		t.Fatalf("审核失败: %v", err)
	}
	if _, err := CreateGroupReply(g, p.ID, owner, "欢迎"); err != nil {
		t.Fatalf("过审后应可回帖: %v", err)
	}
	// 审核拒绝 → 不可见
	p2, _ := CreateGroupPost(g, member, "第二篇", "内容内容", false)
	if err := ReviewGroupPost(p2.ID, owner.ID, models.RoleAuthor, false); err != nil {
		t.Fatalf("拒绝失败: %v", err)
	}
	after, _ := GetGroupPost(p2.ID)
	if after.Status != GroupPostRemoved {
		t.Fatalf("拒绝后应为 removed，得到 %s", after.Status)
	}
}

// TestInviteMemberPermission 拉人：作者可邀请；关闭成员邀请后普通成员不可邀请
func TestInviteMemberPermission(t *testing.T) {
	setupGroupTestDB(t)
	owner := mkUser(t, "i_owner", models.RoleAuthor)
	m1 := mkUser(t, "i_m1", models.RoleReader)
	m2 := mkUser(t, "i_m2", models.RoleReader)
	m3 := mkUser(t, "i_m3", models.RoleReader)
	g, err := CreateGroup(owner.ID, owner, GroupInput{Name: "拉人群"})
	if err != nil {
		t.Fatalf("建群失败: %v", err)
	}
	if _, err := InviteMember(g, owner.ID, models.RoleAuthor, owner.Username, m1.Username); err != nil {
		t.Fatalf("群主邀请失败: %v", err)
	}
	// 允许成员邀请时，普通成员可拉人
	g, _ = GetGroup(g.ID)
	if _, err := InviteMember(g, m1.ID, models.RoleReader, m1.Username, m2.Username); err != nil {
		t.Fatalf("成员邀请应成功: %v", err)
	}
	// 关闭成员邀请
	models.DB.Model(&Group{}).Where("id = ?", g.ID).Update("allow_member_invite", false)
	g, _ = GetGroup(g.ID)
	if g.AllowMemberInvite {
		t.Fatalf("关闭成员邀请未生效")
	}
	if _, err := InviteMember(g, m2.ID, models.RoleReader, m2.Username, m3.Username); err == nil {
		t.Fatalf("关闭成员邀请后普通成员不应能拉人")
	}
}

// TestCheckArchiveSafety ZIP 炸弹检测
func TestCheckArchiveSafety(t *testing.T) {
	dir := t.TempDir()

	// 高危：高压缩比（全零数据）
	bomb := filepath.Join(dir, "bomb.zip")
	f, err := os.Create(bomb)
	if err != nil {
		t.Fatalf("建文件失败: %v", err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("zeros.bin")
	chunk := make([]byte, 1<<20)
	for i := 0; i < 5; i++ {
		if _, err := w.Write(chunk); err != nil {
			t.Fatalf("写压缩内容失败: %v", err)
		}
	}
	_ = zw.Close()
	_ = f.Close()
	if err := CheckArchiveSafety(bomb); err == nil {
		t.Fatalf("高压缩比压缩包应被拒绝")
	}

	// 正常：小文本
	ok := filepath.Join(dir, "ok.zip")
	f2, _ := os.Create(ok)
	zw2 := zip.NewWriter(f2)
	w2, _ := zw2.Create("a.txt")
	_, _ = w2.Write([]byte("hello nvs"))
	_ = zw2.Close()
	_ = f2.Close()
	if err := CheckArchiveSafety(ok); err != nil {
		t.Fatalf("普通压缩包不应被拒绝: %v", err)
	}
}
