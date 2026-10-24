package prompttpl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nvs-server/models"
)

// TestBuildNovelVars 上下文自动注入：作品信息 / 大纲 / 角色 / 摘要 / 前文 / 作者样本，
// 且客户端提供的「原文」优先于旧章节正文。
func TestBuildNovelVars(t *testing.T) {
	setupTplDB(t)

	author := &models.User{Username: "ctx_author", Email: "ctx@example.com", PasswordHash: "x", Role: models.RoleAuthor}
	if err := models.DB.Create(author).Error; err != nil {
		t.Fatalf("建作者失败: %v", err)
	}
	novel := &models.Novel{AuthorID: author.ID, Title: "星海拾遗", Category: "科幻", Summary: "一个关于记忆的故事"}
	if err := models.DB.Create(novel).Error; err != nil {
		t.Fatalf("建作品失败: %v", err)
	}
	_ = models.UpsertNovelOutline(novel.ID, "三幕：启程 / 迷失 / 归来")
	_ = models.UpsertNovelCharacters(novel.ID, "顾昭：主角，记忆修复师；林夏：搭档，理性")

	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("写章节文件失败: %v", err)
		}
		return p
	}
	ch1 := &models.Chapter{NovelID: novel.ID, ChapterNumber: 1, Title: "启程", ContentPath: write("1.txt", "第一章正文：他走进观测站。"), Status: "published"}
	ch2 := &models.Chapter{NovelID: novel.ID, ChapterNumber: 2, Title: "迷失", ContentPath: write("2.txt", "第二章正文：信号中断了。"), Status: "published"}
	if err := models.DB.Create(ch1).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}
	if err := models.DB.Create(ch2).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}
	_ = models.UpsertChapterSummary(novel.ID, ch1.ID, "主角抵达观测站，发现异常信号")

	res, err := BuildNovelVars(novel.ID, ch2.ID, map[string]string{"原文": "编辑器里正在写的新段落"}, 0)
	if err != nil {
		t.Fatalf("组装失败: %v", err)
	}

	cases := map[string]string{
		"作品名":  "星海拾遗",
		"类型":   "科幻",
		"大纲":   "三幕",
		"角色信息": "顾昭",
		"前文":   "观测站",
		"原文":   "编辑器里正在写的新段落", // 客户端优先
		"章节名":  "迷失",
	}
	for name, want := range cases {
		got := res.Vars[name]
		if !strings.Contains(got, want) {
			t.Fatalf("变量 %s 期望包含 %q，实际 %q", name, want, got)
		}
	}
	if !strings.Contains(res.Vars["前文摘要"], "观测站") {
		t.Fatalf("前文摘要未注入：%q", res.Vars["前文摘要"])
	}
	if strings.TrimSpace(res.Vars["作者样本"]) == "" {
		t.Fatalf("作者样本为空")
	}
	// 客户端变量不应被标记为自动填充
	for _, name := range res.AutoFilled {
		if name == "原文" {
			t.Fatalf("客户端提供的变量不应计入 auto_filled")
		}
	}
	if res.NovelTitle != "星海拾遗" || res.ChapterTitle != "迷失" {
		t.Fatalf("标题信息异常: %+v", res)
	}
}

// TestPlaceholdersAndFilter 占位提取与过滤（只保留模板用到的变量）
func TestPlaceholdersAndFilter(t *testing.T) {
	content := "【{{作品名}}】{{前文}} 与 {{前文}}，以及 {{角色信息}}"
	ph := Placeholders(content)
	if len(ph) != 3 {
		t.Fatalf("占位去重后应为 3 个，得到 %v", ph)
	}
	vars := map[string]string{"作品名": "A", "前文": "B", "角色信息": "C", "大纲": "D"}
	out := FilterVars(content, vars)
	if len(out) != 3 {
		t.Fatalf("过滤后应剩 3 个，得到 %v", out)
	}
	if _, ok := out["大纲"]; ok {
		t.Fatalf("模板未使用的变量不应保留")
	}
}
