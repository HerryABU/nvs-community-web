package models

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupSearchLogTestDB 独立命名内存库，避免与其它测试共享 file::memory: 互相污染
func setupSearchLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:searchlog_test?mode=memory&cache=shared"), &gorm.Config{})
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

func TestTokenize(t *testing.T) {
	// 英文整词 + 小写化 + 停用词过滤
	got := Tokenize("Hello the World")
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Errorf("英文切分词结果异常: %v", got)
	}

	// CJK 短段（≤4）整段成词
	got = Tokenize("修真")
	if len(got) != 1 || got[0] != "修真" {
		t.Errorf("短中文段应整段成词，实际 %v", got)
	}
	got = Tokenize("三体")
	if len(got) != 1 || got[0] != "三体" {
		t.Errorf("短中文段应整段成词，实际 %v", got)
	}

	// CJK 长段二元切分；含虚字的二元组丢弃
	got = Tokenize("星球大战星球")
	join := map[string]int{}
	for _, w := range got {
		join[w]++
	}
	if join["星球"] != 2 || join["大战"] != 1 {
		t.Errorf("长中文段二元切分异常: %v", got)
	}

	// 虚字短段整段丢弃
	if got = Tokenize("我的"); len(got) != 0 {
		t.Errorf("虚字短段应被丢弃，实际 %v", got)
	}

	// 中英混合
	got = Tokenize("推荐好看的web小说")
	if len(got) == 0 {
		t.Fatalf("混合文本应切出词: %v", got)
	}

	// 空/纯空白
	if got = Tokenize("   "); got != nil {
		t.Errorf("空白输入应返回 nil，实际 %v", got)
	}

	// 超长截断不 panic
	long := ""
	for i := 0; i < MaxTokenizeRunes+500; i++ {
		long += "测"
	}
	if got = Tokenize(long); len(got) == 0 {
		t.Errorf("超长输入应截断后仍可切词")
	}
}

func TestRecordSearchQueryTruncate(t *testing.T) {
	setupSearchLogTestDB(t)

	// 超长 query 截断到 MaxSearchQueryLen 后异步落库
	long := ""
	for i := 0; i < MaxSearchQueryLen+100; i++ {
		long += "词"
	}
	RecordSearchQuery(long, 7, "unified", 3)
	RecordSearchQuery("   ", 7, "unified", 0) // 空白应被忽略

	// 异步 worker 最多 500ms flush；轮询等待落库
	var count int64
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		DB.Model(&SearchLog{}).Count(&count)
		if count >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if count < 1 {
		t.Fatalf("搜索埋点未落库（count=%d）", count)
	}
	var log SearchLog
	if err := DB.Order("id DESC").First(&log).Error; err != nil {
		t.Fatalf("读取搜索日志失败: %v", err)
	}
	if len([]rune(log.Query)) != MaxSearchQueryLen {
		t.Errorf("超长 query 应截断到 %d，实际 %d", MaxSearchQueryLen, len([]rune(log.Query)))
	}
	if log.UserID != 7 || log.Source != "unified" || log.ResultCount != 3 {
		t.Errorf("埋点字段异常: %+v", log)
	}
}

func TestAggregateSearchWordsAndQueries(t *testing.T) {
	db := setupSearchLogTestDB(t)

	// 直接落库（绕过异步 worker，保证确定性）
	rows := []SearchLog{
		{Query: "修真小说", UserID: 1, Source: "unified", CreatedAt: time.Now()},
		{Query: "修真小说", UserID: 2, Source: "novels", CreatedAt: time.Now()},
		{Query: "好看的修真", UserID: 0, Source: "unified", CreatedAt: time.Now()},
		{Query: "hello world", UserID: 0, Source: "unified", CreatedAt: time.Now()},
		{Query: "hello world", UserID: 0, Source: "unified", CreatedAt: time.Now()},
		{Query: "科幻小说", UserID: 0, Source: "unified", CreatedAt: time.Now()},
		{Query: "科幻小说", UserID: 0, Source: "unified", CreatedAt: time.Now()},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("造数据失败: %v", err)
	}

	words, err := AggregateSearchWords(7, 50)
	if err != nil {
		t.Fatalf("分词聚合失败: %v", err)
	}
	byWord := map[string]int{}
	for _, w := range words {
		byWord[w.Word] = w.Count
	}
	// 短段（≤4 字）整段成词：「修真小说」「科幻小说」各出现 2 次
	if byWord["修真小说"] != 2 {
		t.Errorf("修真小说 应计 2（短段整取），实际 %d", byWord["修真小说"])
	}
	if byWord["科幻小说"] != 2 {
		t.Errorf("科幻小说 应计 2（短段整取），实际 %d", byWord["科幻小说"])
	}
	// 英文整词
	if byWord["hello"] != 2 || byWord["world"] != 2 {
		t.Errorf("hello/world 应各 2，实际 %d/%d", byWord["hello"], byWord["world"])
	}
	// 长段二元切分路径：「好看的修真」→ 好看/修真（看的、的修含虚字被丢弃），
	// 二者各出现 1 次，低于 ≥2 阈值被滤掉
	if _, ok := byWord["修真"]; ok {
		t.Errorf("修真 仅 1 次应被 ≥2 阈值滤除，实际 %d", byWord["修真"])
	}
	if _, ok := byWord["看的"]; ok {
		t.Errorf("含虚字的二元组 看的 不应出现")
	}
	// 停用词不入库
	if _, ok := byWord["the"]; ok {
		t.Errorf("停用词 the 不应出现")
	}

	queries, err := AggregateSearchQueries(7, 50)
	if err != nil {
		t.Fatalf("短语聚合失败: %v", err)
	}
	byQuery := map[string]int{}
	for _, q := range queries {
		byQuery[q.Word] = q.Count
	}
	if byQuery["修真小说"] != 2 {
		t.Errorf("原始短语「修真小说」应计 2，实际 %d", byQuery["修真小说"])
	}
	if byQuery["hello world"] != 2 {
		t.Errorf("原始短语「hello world」应计 2，实际 %d", byQuery["hello world"])
	}
	// 单次出现的短语也计入（≥1）
	if byQuery["好看的修真"] != 1 {
		t.Errorf("单次短语应计入，实际 %d", byQuery["好看的修真"])
	}

	total, err := CountSearchLogs(7)
	if err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if total != int64(len(rows)) {
		t.Errorf("搜索总次数应为 %d，实际 %d", len(rows), total)
	}

	// 窗口外数据不计入
	db.Create(&SearchLog{Query: "窗外词汇", CreatedAt: time.Now().AddDate(0, 0, -30)})
	words, _ = AggregateSearchWords(7, 50)
	for _, w := range words {
		if w.Word == "窗外" || w.Word == "词汇" {
			t.Errorf("窗口外数据不应计入: %v", words)
		}
	}

	// limit 收敛
	limited, _ := AggregateSearchQueries(7, 2)
	if len(limited) > 2 {
		t.Errorf("limit=2 应最多 2 项，实际 %d", len(limited))
	}
}

func TestSearchLogCleanup(t *testing.T) {
	db := setupSearchLogTestDB(t)
	db.Create(&SearchLog{Query: "甲", CreatedAt: time.Now()})
	db.Create(&SearchLog{Query: "乙", CreatedAt: time.Now().AddDate(0, 0, -100)})

	// 保留期清理：只删 90 天前的
	removed, err := SearchLogRetentionCleanup(90)
	if err != nil {
		t.Fatalf("保留期清理失败: %v", err)
	}
	if removed != 1 {
		t.Errorf("应清理 1 条旧记录，实际 %d", removed)
	}
	var left int64
	db.Model(&SearchLog{}).Count(&left)
	if left != 1 {
		t.Errorf("应剩余 1 条，实际 %d", left)
	}

	// 全量清空
	if err := SearchLogCleanup(); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	db.Model(&SearchLog{}).Count(&left)
	if left != 0 {
		t.Errorf("清空后应为 0，实际 %d", left)
	}
}

func TestAggregateTagHotWords(t *testing.T) {
	db := setupSearchLogTestDB(t)

	// 1) 博客结构化标签
	tagA := BlogTag{Name: "修真"}
	tagB := BlogTag{Name: "科幻"}
	db.Create(&tagA)
	db.Create(&tagB)
	blog1 := AuthorBlog{AuthorID: 1, Title: "博客一", ContentPath: "x", CreatedAt: time.Now()}
	blog2 := AuthorBlog{AuthorID: 1, Title: "博客二", ContentPath: "x", CreatedAt: time.Now()}
	db.Create(&blog1)
	db.Create(&blog2)
	db.Create(&BlogTagLink{BlogID: blog1.ID, TagID: tagA.ID})
	db.Create(&BlogTagLink{BlogID: blog2.ID, TagID: tagA.ID})
	db.Create(&BlogTagLink{BlogID: blog2.ID, TagID: tagB.ID})

	// 2) 作品 JSON 标签
	db.Create(&Novel{AuthorID: 1, Title: "作品一", Tags: `["修真","末日"]`, CreatedAt: time.Now()})
	db.Create(&Novel{AuthorID: 1, Title: "作品二", Tags: `["修真"]`, CreatedAt: time.Now()})

	// 窗口外博客不计入
	oldBlog := AuthorBlog{AuthorID: 1, Title: "旧博客", ContentPath: "x", CreatedAt: time.Now().AddDate(0, 0, -30)}
	db.Create(&oldBlog)
	db.Create(&BlogTagLink{BlogID: oldBlog.ID, TagID: tagB.ID})

	words, err := AggregateTagHotWords(7, 50)
	if err != nil {
		t.Fatalf("标签热词聚合失败: %v", err)
	}
	byTag := map[string]int{}
	for _, w := range words {
		byTag[w.Word] = w.Count
	}
	// 修真：博客 2 + 作品 2 = 4
	if byTag["修真"] != 4 {
		t.Errorf("修真 应计 4（博客2+作品2），实际 %d", byTag["修真"])
	}
	// 科幻：仅窗口内博客 1（旧博客不计）
	if byTag["科幻"] != 1 {
		t.Errorf("科幻 应计 1，实际 %d", byTag["科幻"])
	}
	// 末日：作品 JSON 标签 1
	if byTag["末日"] != 1 {
		t.Errorf("末日 应计 1，实际 %d", byTag["末日"])
	}
	// 降序校验
	for i := 1; i < len(words); i++ {
		if words[i-1].Count < words[i].Count {
			t.Errorf("结果应按计数降序: %v", words)
			break
		}
	}
}

func TestParseNovelTags(t *testing.T) {
	// 合法 JSON 数组
	got := parseNovelTags(`["修真","末日","  "]`)
	if len(got) != 2 || got[0] != "修真" || got[1] != "末日" {
		t.Errorf("JSON 标签解析异常: %v", got)
	}
	// 空值
	if got = parseNovelTags(""); got != nil {
		t.Errorf("空值应返回 nil，实际 %v", got)
	}
	if got = parseNovelTags("[]"); len(got) != 0 {
		t.Errorf("空数组应返回空，实际 %v", got)
	}
	// 非法 JSON 退化为分隔符切分
	got = parseNovelTags("修真,末日；科幻")
	if len(got) != 3 {
		t.Errorf("非法 JSON 退化切分异常: %v", got)
	}
}
