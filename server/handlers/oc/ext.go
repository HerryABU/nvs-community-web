package ochandler

// ==================== OC 扩展接口：自定义表情 / 角色卡导入导出（gap-audit #22 #24）====================

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"nvs-server/middleware"
	"nvs-server/oc"
	"nvs-server/scriptmode"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ─────────────── 表情包 ───────────────

// ListEmotes GET /api/oc/emotes —— 我的表情 + 内置键
func ListEmotes(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	list, err := oc.ListMyEmotes(uid)
	if err != nil {
		utils.InternalError(c, "读取失败")
		return
	}
	utils.Success(c, gin.H{
		"list":         list,
		"builtin_keys": scriptmode.EmoteKeys(),
		"custom_keys":  scriptmode.CustomEmoteKeys(),
		"note":         "表情键全局唯一（气泡由服务端渲染，没有「当前用户」上下文），冲突时请换个名字",
	})
}

// CreateEmote POST /api/oc/emotes { key, url, note }
func CreateEmote(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	var in struct {
		Key  string `json:"key"`
		URL  string `json:"url"`
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	e, err := oc.CreateEmote(uid, in.Key, in.URL, in.Note)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	scriptmode.RegisterCustomEmotes(oc.CustomEmoteMap())
	utils.SuccessMessage(c, "已添加，可在气泡里用 ["+e.Key+"]", e)
}

// DeleteEmote DELETE /api/oc/emotes/:id
func DeleteEmote(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := oc.DeleteEmote(uid, uint(id)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	scriptmode.RegisterCustomEmotes(oc.CustomEmoteMap())
	utils.SuccessMessage(c, "已删除", nil)
}

// ─────────────── 角色卡导入导出 ───────────────

// GetCharacter 已有（handler.go）。这里只加 export / import。

// ExportCard GET /api/oc/characters/:cid/export —— 下载酒馆 V2 JSON
func ExportCard(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("cid"), 10, 64)
	ch, err := oc.GetCharacter(uint(id), uid, isAdmin(c))
	if err != nil {
		utils.NotFound(c, "角色不存在")
		return
	}
	card, err := oc.ExportCard(ch)
	if err != nil {
		utils.InternalError(c, "导出失败")
		return
	}
	name := strings.ReplaceAll(ch.Name, "/", "_")
	c.Header("Content-Disposition", `attachment; filename="`+name+`.json"`)
	c.JSON(http.StatusOK, card)
}

// ImportCard POST /api/oc/characters/import —— 上传 JSON（multipart: file 或 JSON body）
// 只创建角色卡与（可选）世界书，不自动加入任何群。
func ImportCard(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	raw, err := readCardBody(c)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	parsed, err := oc.ParseCard(raw)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	// 世界书先建（角色要引用它的 ID）
	var bookID uint
	if parsed.Book != nil && len(parsed.Book.Entries) > 0 {
		title := parsed.Book.Name
		if strings.TrimSpace(title) == "" {
			title = parsed.Name + "·世界书"
		}
		book, berr := oc.CreateWorldBook(uid, 0, title, "从角色卡导入", "private")
		if berr == nil && book != nil {
			bookID = book.ID
			for _, e := range parsed.Book.Entries {
				_, _ = oc.SaveEntryFull(uid, book.ID, 0, oc.EntryInput{
					Kind: "world", Key: strings.Join(e.Keys, ","), Content: e.Content,
					SecondaryKeys: strings.Join(e.SecondaryKeys, ","),
					Selective:     e.Selective, Constant: e.Constant,
					InsertionOrder: e.InsertionOrder, Priority: e.Priority,
					Position: e.Position, Depth: e.Depth, Probability: e.Probability,
					Enabled: boolPtr(e.Enabled), CaseSensitive: e.CaseSensitive, Group: e.Group,
				})
			}
		}
	}
	greetings := ""
	if len(parsed.Greetings) > 0 {
		// 简易 JSON 数组（元素含引号时转义）
		parts := make([]string, 0, len(parsed.Greetings))
		for _, g := range parsed.Greetings {
			parts = append(parts, `"`+strings.ReplaceAll(g, `"`, `\"`)+`"`)
		}
		greetings = "[" + strings.Join(parts, ",") + "]"
	}
	ch, cerr := oc.CreateCharacter(uid, oc.CharacterInput{
		Name: parsed.Name, Persona: parsed.Persona, Appearance: parsed.Appearance,
		Speech: parsed.Speech, Scenario: parsed.Scenario, FirstMessage: parsed.FirstMes,
		MesExample: parsed.MesExample, SystemPrompt: parsed.SystemPrompt,
		PostHistory: parsed.PostHistory, CreatorNotes: parsed.CreatorNotes,
		Greetings: greetings, Tags: parsed.Tags, BookID: bookID,
		Visibility: "private",
	})
	if cerr != nil {
		utils.BadRequest(c, "导入失败："+cerr.Error())
		return
	}
	utils.SuccessMessage(c, "导入成功", gin.H{
		"character": ch, "book_id": bookID,
		"entries": func() int {
			if parsed.Book != nil {
				return len(parsed.Book.Entries)
			}
			return 0
		}(),
		"raw_kept": parsed.Raw != "",
	})
}

func readCardBody(c *gin.Context) ([]byte, error) {
	// multipart 上传优先
	if fh, err := c.FormFile("file"); err == nil && fh != nil {
		f, oerr := fh.Open()
		if oerr != nil {
			return nil, oerr
		}
		defer f.Close()
		return io.ReadAll(io.LimitReader(f, 2<<20)) // 2MB 上限
	}
	// 直接 JSON body
	b, err := io.ReadAll(io.LimitReader(c.Request.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil, errEmptyCard
	}
	return b, nil
}

var errEmptyCard = &cardErr{"请上传角色卡 JSON（酒馆 V2/V1）"}

type cardErr struct{ msg string }

func (e *cardErr) Error() string { return e.msg }

// ExportBook GET /api/oc/books/:bid/export —— 世界书导出为酒馆 lorebook
func ExportBook(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	id, _ := strconv.ParseUint(c.Param("bid"), 10, 64)
	book, err := oc.GetWorldBook(uint(id), uid, isAdmin(c))
	if err != nil {
		utils.NotFound(c, "世界书不存在")
		return
	}
	entries, _ := oc.ListEntries(book.ID)
	c.Header("Content-Disposition", `attachment; filename="lorebook-`+strconv.Itoa(int(book.ID))+`.json"`)
	c.JSON(http.StatusOK, oc.ExportLorebookJSON(book, entries))
}

// ImportBook POST /api/oc/books/import —— 导入 lorebook
func ImportBook(c *gin.Context) {
	uid := middleware.UserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	raw, err := readCardBody(c)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	parsed, err := oc.ParseLorebookJSON(raw)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	title := parsed.Name
	if strings.TrimSpace(title) == "" {
		title = "导入的世界书"
	}
	book, err := oc.CreateWorldBook(uid, 0, title, "从 Lorebook 导入", "private")
	if err != nil {
		utils.BadRequest(c, "创建世界书失败")
		return
	}
	n := 0
	for _, e := range parsed.Entries {
		if _, serr := oc.SaveEntryFull(uid, book.ID, 0, oc.EntryInput{
			Kind: "world", Key: strings.Join(e.Keys, ","), Content: e.Content,
			SecondaryKeys: strings.Join(e.SecondaryKeys, ","),
			Selective:     e.Selective, Constant: e.Constant,
			InsertionOrder: e.InsertionOrder, Priority: e.Priority,
			Position: e.Position, Depth: e.Depth, Probability: e.Probability,
			Enabled: boolPtr(e.Enabled), CaseSensitive: e.CaseSensitive, Group: e.Group,
		}); serr == nil {
			n++
		}
	}
	utils.SuccessMessage(c, "导入成功", gin.H{"book": book, "entries": n})
}

// boolPtr 小工具（EntryInput.Enabled 用指针区分「未设置」与「显式 false」）
func boolPtr(v bool) *bool { return &v }
