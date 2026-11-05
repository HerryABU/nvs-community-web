package geo

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nvs-server/handlers/admincommon"
	"nvs-server/ipgeo"
	"nvs-server/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	theme "nvs-server/models/theme"
)

// GetMapSetting GET /api/map-setting —— 公开：前端据此决定用哪套地图
//
// 额外带上「看的人在哪」：中国省级明细只对境内访客展示，
// 境外访客只给全球视野（省份/城市粒度的境内读者分布不对外）。
func GetMapSetting(c *gin.Context) {
	data := theme.PublicMapSetting()
	reg := ipgeo.Lookup(c.ClientIP())
	// 内网 / 保留地址（本机调试、内网反代）按境内处理，否则站长自测会看不到中国视野
	cn := reg.CountryCD == "CN" || reg.IsReserved()
	data["viewer"] = gin.H{
		"cn":      cn,
		"country": reg.Country,
		"code":    reg.CountryCD,
	}
	utils.Success(c, data)
}

// AdminGetMapSetting GET /api/admin/map-setting —— 含完整配置（key 明文给站长自己核对）
func AdminGetMapSetting(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	utils.Success(c, theme.LoadMapSetting())
}

// AdminSaveMapSetting PUT /api/admin/map-setting
func AdminSaveMapSetting(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var in theme.MapSetting
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数不合法")
		return
	}
	if err := theme.SaveMapSetting(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存", theme.LoadMapSetting())
}

// AdminTestMapSetting POST /api/admin/map-setting/test
//
// 站长填完 tk 最想知道的是「它到底能不能用」。这里由服务端带 Referer 去拉一张
// 天地图瓦片，把真实 HTTP 结果翻译成人话 —— 尤其是「白名单没配」这种情况，
// 浏览器里只会白屏，站长无从判断。
//
// 只请求固定的天地图瓦片地址（不接受任意 URL），因此没有 SSRF 面。
func AdminTestMapSetting(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var in struct {
		Tk      string `json:"tk"`
		Referer string `json:"referer"`
	}
	_ = c.ShouldBindJSON(&in)
	tk := strings.TrimSpace(in.Tk)
	if tk == "" {
		tk = strings.TrimSpace(theme.LoadMapSetting().TiandituKey)
	}
	if tk == "" {
		utils.BadRequest(c, "还没有填 tk：请先到 tianditu.gov.cn 申请「浏览器端」密钥")
		return
	}
	referer := strings.TrimSpace(in.Referer)
	if referer == "" {
		scheme := "http"
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		referer = scheme + "://" + c.Request.Host
	}

	// z=4 的 (row=6, col=12) 覆盖中国中东部；底图用矢量 vec 层
	tile := fmt.Sprintf(
		"https://t0.tianditu.gov.cn/vec_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0"+
			"&LAYER=vec&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles"+
			"&TILEMATRIX=4&TILEROW=6&TILECOL=12&tk=%s", url.QueryEscape(tk))

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, tile, nil)
	if err != nil {
		utils.BadRequest(c, "tk 含非法字符")
		return
	}
	req.Header.Set("Referer", referer)
	// 用常见浏览器 UA：天地图前面有 WAF，识别到脚本 UA 会直接返回 418「疑似攻击行为」，
	// 那样测出来的结果和站长在浏览器里的真实体验对不上。
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "+
		"(KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		utils.Success(c, gin.H{
			"ok": false, "stage": "network",
			"reason": "无法连接天地图服务",
			"hint":   "服务器出网可能被限制，或本地 DNS 解析不到 t0.tianditu.gov.cn：" + err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	cost := time.Since(start).Milliseconds()

	out := gin.H{
		"ok":           false,
		"stage":        "http",
		"status":       resp.StatusCode,
		"content_type": ct,
		"bytes":        len(body),
		"ms":           cost,
		"referer":      referer,
		"tile_url":     tile,
	}
	// 非图片响应里通常就写着原因（天地图会返回一段说明页），抽一行给站长看
	if !strings.HasPrefix(ct, "image/") {
		if s := plainSnippet(body, 160); s != "" {
			out["body_hint"] = s
		}
	}
	switch {
	case resp.StatusCode == http.StatusOK && strings.HasPrefix(ct, "image/") && len(body) > 500:
		out["ok"] = true
		out["reason"] = "密钥可用，已成功取到一张地图瓦片"
		out["hint"] = "保存后会立即生效；若前台仍是旧底图，刷新一次页面即可"
	case resp.StatusCode == http.StatusOK && strings.HasPrefix(ct, "image/"):
		out["ok"] = true
		out["reason"] = "取到了瓦片，但内容偏小（可能返回的是提示图）"
		out["hint"] = "建议到前台地图卡片上肉眼确认一次底图是否正常"
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTeapot:
		// 实测：tk 无效时天地图返回 418 + 一段说明页，而不是 401
		bh, _ := out["body_hint"].(string)
		if strings.Contains(bh, "疑似攻击") || strings.Contains(bh, "拦截") {
			out["reason"] = "服务端请求被天地图 WAF 拦截（HTTP 418），这不代表密钥无效"
			out["hint"] = "以浏览器里的两张缩略图为准：图能显示就说明 tk 和 Referer 白名单都正常"
			break
		}
		out["reason"] = fmt.Sprintf("tk 未获授权（HTTP %d）：天地图不认这个密钥，或该应用没开通此图层", resp.StatusCode)
		out["hint"] = "核对三点：① 应用类型必须是「浏览器端」；② Referer 白名单含 " + referer + "；③ tk 完整复制、无多余空格"
	case resp.StatusCode == http.StatusTooManyRequests:
		out["reason"] = "超出调用配额（HTTP 429）"
		out["hint"] = "天地图按应用限制日调用量，可稍后再试或提升配额"
	default:
		out["reason"] = fmt.Sprintf("返回了非瓦片内容（HTTP %d, %s）", resp.StatusCode, ct)
		out["hint"] = "多半是 tk 类别不对（服务端 key 不能用于前端底图），或该应用未开通此图层"
	}
	utils.Success(c, out)
}

// plainSnippet 从 HTML/JSON 里抽一小段可读文本（去标签、压空白、截断），
// 仅用于把上游错误说明转述给站长，不做任何渲染。
func plainSnippet(b []byte, limit int) string {
	s := string(b)
	// 去掉 style/script 段：天地图的说明页前面挂了一大坨 CSS，抽出来会淹没正文
	for _, tag := range []string{"style", "script"} {
		for {
			lo := strings.Index(strings.ToLower(s), "<"+tag)
			if lo < 0 {
				break
			}
			hi := strings.Index(strings.ToLower(s)[lo:], "</"+tag+">")
			if hi < 0 {
				break
			}
			s = s[:lo] + s[lo+hi+len(tag)+3:]
		}
	}
	if i := strings.Index(strings.ToLower(s), "<body"); i > 0 {
		s = s[i:]
	}
	var sb strings.Builder
	skip := false
	for _, r := range s {
		switch {
		case r == '<':
			skip = true
		case r == '>':
			skip = false
		case !skip:
			if r == '\n' || r == '\r' || r == '\t' {
				sb.WriteByte(' ')
			} else {
				sb.WriteRune(r)
			}
		}
	}
	out := strings.Join(strings.Fields(sb.String()), " ")
	if len([]rune(out)) > limit {
		out = string([]rune(out)[:limit]) + "…"
	}
	return out
}
