package prompttpl

import (
	"strings"
	"testing"

	"nvs-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTplDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:prompttpl_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatalf("主迁移失败: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("模板表迁移失败: %v", err)
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

// TestRender 渲染：必填缺失、默认值回落、长度截断、变量未使用
func TestRender(t *testing.T) {
	vars := []TemplateVar{
		{Name: "前文", Label: "前文", Required: true, MaxLen: 10},
		{Name: "类型", Label: "类型", Default: "科幻"},
	}
	content := "【{{类型}}】{{前文}}|{{未知变量}}"

	// 缺必填：保留占位并报告
	res := Render(content, vars, map[string]string{})
	// 未声明的占位同样报告（便于站长发现拼写错误）
	if len(res.Missing) != 2 || res.Missing[0] != "前文" || res.Missing[1] != "未知变量" {
		t.Fatalf("应报告前文与未知变量，得到 %+v", res.Missing)
	}
	if !strings.Contains(res.Text, "【科幻】") {
		t.Fatalf("默认值未回落：%s", res.Text)
	}
	if !strings.Contains(res.Text, "{{前文}}") {
		t.Fatalf("缺失变量应保留占位：%s", res.Text)
	}

	// 超长截断
	long := strings.Repeat("字", 50)
	res2 := Render(content, vars, map[string]string{"前文": long, "多余": "x"})
	if strings.Contains(res2.Text, long) {
		t.Fatalf("变量未按 max_len 截断")
	}
	if len([]rune(res2.Text)) > len([]rune(content))+20 {
		t.Fatalf("渲染结果异常长：%d", len([]rune(res2.Text)))
	}
	if len(res2.Unused) != 1 || res2.Unused[0] != "多余" {
		t.Fatalf("未使用变量应被报告：%+v", res2.Unused)
	}

	// 模板指令不执行（纯文本替换）
	res3 := Render("{{a}}", []TemplateVar{{Name: "a"}}, map[string]string{"a": "{{b}}${x}<script>"})
	if res3.Text != "{{b}}${x}<script>" {
		t.Fatalf("应原样输出（不做二次解析/不执行）：%q", res3.Text)
	}
}

// TestCRUDAndVersions 创建 / 更新（版本快照）/ 回滚 / 内置保护
func TestCRUDAndVersions(t *testing.T) {
	setupTplDB(t)

	tpl, err := Create(Input{
		Key: "t-demo", Title: "演示模板", Scene: "novel_chapter", Category: "测试",
		Content: "第一版 {{a}}", Vars: []TemplateVar{{Name: "a", Label: "甲"}},
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if tpl.Version != 1 || !tpl.Enabled {
		t.Fatalf("初始版本/状态异常: %+v", tpl)
	}
	if _, err := Create(Input{Key: "t-demo", Title: "重复", Content: "x"}); err == nil {
		t.Fatalf("重复 key 应被拒绝")
	}

	upd, err := Update(tpl.ID, Input{
		Key: "t-demo", Title: "演示模板2", Content: "第二版 {{a}}{{b}}",
		Vars: []TemplateVar{{Name: "a"}, {Name: "b"}},
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if upd.Version != 2 || upd.Content != "第二版 {{a}}{{b}}" {
		t.Fatalf("更新后版本异常: %+v", upd)
	}

	vers, err := ListVersions(tpl.ID)
	if err != nil || len(vers) != 1 {
		t.Fatalf("应有 1 条历史快照，得到 %d（%v）", len(vers), err)
	}
	if vers[0].Content != "第一版 {{a}}" {
		t.Fatalf("快照内容不对: %q", vers[0].Content)
	}

	// 回滚到 v1
	back, err := Restore(tpl.ID, 1)
	if err != nil {
		t.Fatalf("回滚失败: %v", err)
	}
	if back.Content != "第一版 {{a}}" || back.Version != 3 {
		t.Fatalf("回滚结果异常: %+v", back)
	}

	// 内置模板：不可删除
	InitSeeds()
	var builtin Template
	if err := models.DB.Where("key = ?", "tl-continue").First(&builtin).Error; err != nil {
		t.Fatalf("种子模板未写入: %v", err)
	}
	if err := Delete(builtin.ID); err == nil {
		t.Fatalf("内置模板应禁止删除")
	}
	// 幂等：重复 InitSeeds 不产生重复
	var n int64
	models.DB.Model(&Template{}).Where("key = ?", "tl-continue").Count(&n)
	InitSeeds()
	models.DB.Model(&Template{}).Where("key = ?", "tl-continue").Count(&n)
	if n != 1 {
		t.Fatalf("种子应幂等，得到 %d 条", n)
	}
}

// TestSeedsAreGenreLiterature 默认框架应覆盖类型文学场景，且不写入爽文模板
func TestSeedsAreGenreLiterature(t *testing.T) {
	setupTplDB(t)
	InitSeeds()
	list, err := List("", "", false)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(list) < 8 {
		t.Fatalf("默认模板过少：%d", len(list))
	}
	has := map[string]bool{}
	for _, x := range list {
		has[x.Key] = true
		if strings.Contains(x.Content, "爽点") || strings.Contains(x.Title, "爽文") {
			t.Fatalf("默认框架不应内置爽文模板：%s", x.Title)
		}
	}
	for _, key := range []string{"tl-continue", "tl-hard-sf", "tl-mystery", "tl-alt-history", "tl-experimental", "tl-summary"} {
		if !has[key] {
			t.Fatalf("缺少默认模板 %s", key)
		}
	}
}
