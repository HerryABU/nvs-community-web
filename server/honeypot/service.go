package honeypot

// ==================== 服务层：策略读取 / 事件落库 / 封禁 / 导出 ====================

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"nvs-server/models"

	"github.com/gin-gonic/gin"
)

// ─── 策略读写 ───

var (
	setMu    sync.RWMutex
	cached   Setting
	cacheAt  time.Time
	cacheTTL = 15 * time.Second
)

// offSetting 关闭态默认值（DB 未就绪时回落）
func offSetting() Setting {
	return Setting{ID: 1, Level: LevelOff, RecordUA: true, RecordReferer: true, RecordQuery: true,
		BodySnippetLimit: 512, BanMinutes: 60, RetentionDays: 90}
}

// Current 读取当前策略（15 秒进程缓存，避免每请求打库）
func Current() Setting {
	// 未完成建站 / DB 尚未初始化时，诱饵必须静默失效（不能 panic，否则裸奔期直接被扫崩）
	if models.DB == nil {
		return offSetting()
	}
	setMu.RLock()
	if time.Since(cacheAt) < cacheTTL && cached.ID > 0 {
		s := cached
		setMu.RUnlock()
		return s
	}
	setMu.RUnlock()

	var s Setting
	err := models.DB.Where("id = 1").First(&s).Error
	if err != nil {
		s = offSetting()
	}
	setMu.Lock()
	cached, cacheAt = s, time.Now()
	setMu.Unlock()
	return s
}

// SaveSetting 保存策略（站长/管理员）
func SaveSetting(in Setting) (*Setting, error) {
	in.Level = NormalizeLevel(in.Level)
	if in.Enabled && in.Level == LevelOff {
		in.Level = LevelLow // 开启即给最低等级，避免「开了但没诱饵」
	}
	if !in.Enabled {
		in.Level = LevelOff
	}
	if in.BodySnippetLimit <= 0 {
		in.BodySnippetLimit = 512
	}
	if in.BodySnippetLimit > 4096 {
		in.BodySnippetLimit = 4096
	}
	if in.BanMinutes <= 0 {
		in.BanMinutes = 60
	}
	if in.RetentionDays <= 0 {
		in.RetentionDays = 90
	}
	if in.DelayMs < 0 {
		in.DelayMs = 0
	}
	in.UpdatedAt = time.Now()

	var old Setting
	if err := models.DB.Where("id = 1").First(&old).Error; err != nil {
		in.ID = 1
		if err := models.DB.Create(&in).Error; err != nil {
			return nil, err
		}
	} else {
		in.ID = old.ID
		if err := models.DB.Model(&Setting{}).Where("id = ?", old.ID).Updates(map[string]interface{}{
			"enabled": in.Enabled, "level": in.Level,
			"record_ua": in.RecordUA, "record_referer": in.RecordReferer,
			"record_xff": in.RecordXFF, "record_query": in.RecordQuery,
			"record_headers": in.RecordHeaders, "record_body_snippet": in.RecordBodySnippet,
			"body_snippet_limit": in.BodySnippetLimit, "header_allowlist": in.HeaderAllowlist,
			"auto_ban": in.AutoBan, "ban_minutes": in.BanMinutes, "delay_ms": in.DelayMs,
			"retention_days": in.RetentionDays, "updated_at": in.UpdatedAt,
		}).Error; err != nil {
			return nil, err
		}
	}
	setMu.Lock()
	cached, cacheAt = in, time.Now()
	setMu.Unlock()
	return &in, nil
}

// ResetCache 测试/手动刷新用
func ResetCache() {
	setMu.Lock()
	cached, cacheAt = Setting{}, time.Time{}
	setMu.Unlock()
}

// ─── 日志字段策略 ───

// defaultHeaderAllowlist 默认只记这些「有用但不敏感」的头
const defaultHeaderAllowlist = "X-Forwarded-For,X-Real-IP,X-Requested-With,Sec-Fetch-Site,Sec-Fetch-Mode,Sec-Fetch-Dest,CF-Connecting-IP,Accept-Language"

// sensitiveHeaders 永不过白名单
var sensitiveHeaders = map[string]bool{
	"cookie": true, "authorization": true, "x-api-key": true,
	"set-cookie": true, "proxy-authorization": true,
}

// LogFields 按策略筛出的日志字段（关掉的字段留空，不落库）
type LogFields struct {
	UA          string
	Referer     string
	XFF         string
	Query       string
	Headers     string
	BodySnippet string
}

// BuildLogFields 依策略裁剪字段（管理员勾选哪项才记哪项）
func BuildLogFields(set Setting, c *gin.Context) LogFields {
	var f LogFields
	req := c.Request
	if set.RecordUA {
		f.UA = truncate(req.UserAgent(), 512)
	}
	if set.RecordReferer {
		f.Referer = truncate(req.Referer(), 512)
	}
	if set.RecordXFF {
		f.XFF = truncate(req.Header.Get("X-Forwarded-For"), 256)
	}
	if set.RecordQuery {
		f.Query = truncate(req.URL.RawQuery, 512)
	}
	if set.RecordHeaders {
		allow := set.HeaderAllowlist
		if strings.TrimSpace(allow) == "" {
			allow = defaultHeaderAllowlist
		}
		keep := map[string]bool{}
		for _, k := range strings.Split(allow, ",") {
			k = strings.ToLower(strings.TrimSpace(k))
			if k != "" && !sensitiveHeaders[k] {
				keep[k] = true
			}
		}
		out := map[string]string{}
		for k, v := range req.Header {
			lk := strings.ToLower(k)
			if !keep[lk] || sensitiveHeaders[lk] {
				continue
			}
			out[lk] = truncate(strings.Join(v, ","), 256)
		}
		if b, err := json.Marshal(out); err == nil {
			f.Headers = string(b)
		}
	}
	if set.RecordBodySnippet {
		f.BodySnippet = readBodySnippet(req, set.BodySnippetLimit)
	}
	return f
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// readBodySnippet 读一小段 body 并复位（不破坏后续 handler）
func readBodySnippet(req *http.Request, limit int) string {
	if req.Body == nil {
		return ""
	}
	if limit <= 0 {
		limit = 512
	}
	buf := make([]byte, limit)
	n, _ := io.ReadFull(io.LimitReader(req.Body, int64(limit)), buf)
	// 重新拼回 body：已读部分 + 剩余
	rest := req.Body
	req.Body = io.NopCloser(io.MultiReader(strings.NewReader(string(buf[:n])), rest))
	return sanitizeSnippet(string(buf[:n]))
}

// sanitizeSnippet 洗白 body 片段：去掉疑似凭据（api key / token / bearer / 密码）
func sanitizeSnippet(s string) string {
	lower := strings.ToLower(s)
	for _, kw := range []string{"password", "passwd", "token", "secret", "sk-", "bearer", "api_key", "apikey"} {
		if strings.Contains(lower, kw) {
			return "[已省略：疑似包含凭据]"
		}
	}
	return s
}

// ─── 封禁（进程内黑名单；重启清空，v2 落库） ───

var (
	banMu    sync.Mutex
	banUntil = map[string]time.Time{}
)

// Ban 封禁 IP 若干分钟
func Ban(ip string, minutes int) {
	if ip == "" || minutes <= 0 {
		return
	}
	banMu.Lock()
	banUntil[ip] = time.Now().Add(time.Duration(minutes) * time.Minute)
	banMu.Unlock()
}

// IsBanned 是否在封禁期
func IsBanned(ip string) bool {
	banMu.Lock()
	defer banMu.Unlock()
	until, ok := banUntil[ip]
	if !ok {
		return false
	}
	if time.Now().After(until) {
		delete(banUntil, ip)
		return false
	}
	return true
}

// Unban 解除封禁
func Unban(ip string) {
	banMu.Lock()
	delete(banUntil, ip)
	banMu.Unlock()
}

// BannedList 当前封禁列表
func BannedList() map[string]time.Time {
	banMu.Lock()
	defer banMu.Unlock()
	out := make(map[string]time.Time, len(banUntil))
	now := time.Now()
	for k, v := range banUntil {
		if now.Before(v) {
			out[k] = v
		}
	}
	return out
}

// ─── 事件记录 ───

// Record 记录一次诱饵命中，并按等级处置
func Record(set Setting, c *gin.Context, decoy Decoy) (*Event, error) {
	if !set.Enabled || set.Level == LevelOff {
		return nil, nil
	}
	ip := c.ClientIP()
	fields := BuildLogFields(set, c)
	now := time.Now()

	sess, err := ensureSession(ip, fields.UA, fields.Referer, fields.XFF, now)
	if err != nil {
		return nil, err
	}

	action := "record"
	if IsBanned(ip) {
		action = "banned_skip"
	} else if set.AutoBan && (decoy.Risk >= 60 || set.Level == LevelHigh) {
		Ban(ip, set.BanMinutes)
		action = "ban"
	} else if set.DelayMs > 0 {
		action = "delay"
	}

	ev := &Event{
		SessionID: sess.ID, Kind: EventDecoyHit, Decoy: decoy.ID,
		Method: c.Request.Method, Path: truncate(c.Request.URL.Path, 512),
		Query: fields.Query, UA: fields.UA, Referer: fields.Referer, XFF: fields.XFF,
		Headers: fields.Headers, BodySnippet: fields.BodySnippet,
		Risk: decoy.Risk, Action: action, CreatedAt: now,
	}
	if err := models.DB.Create(ev).Error; err != nil {
		return nil, err
	}
	_ = models.DB.Model(&Session{}).Where("id = ?", sess.ID).Updates(map[string]interface{}{
		"event_count": models.DB.Raw("event_count + 1"),
		"last_seen":   now,
		"max_risk":    maxInt(sess.MaxRisk, decoy.Risk),
		"banned":      action == "ban",
	}).Error
	return ev, nil
}

func ensureSession(ip, ua, referer, xff string, now time.Time) (*Session, error) {
	var s Session
	err := models.DB.Where("src_ip = ? AND ua_norm = ?", ip, truncate(ua, 256)).First(&s).Error
	if err == nil {
		return &s, nil
	}
	s = Session{
		HID: randHex(8), SrcIP: ip, UANorm: truncate(ua, 256),
		Referer: truncate(referer, 512), XFF: truncate(xff, 256),
		FirstSeen: now, LastSeen: now,
	}
	if cerr := models.DB.Create(&s).Error; cerr != nil {
		return nil, cerr
	}
	return &s, nil
}

// updateSessionStats 会话计数与风险汇总（探测路径也走这里）
func updateSessionStats(sess *Session, risk int, now time.Time) error {
	return models.DB.Model(&Session{}).Where("id = ?", sess.ID).Updates(map[string]interface{}{
		"event_count": models.DB.Raw("event_count + 1"),
		"last_seen":   now,
		"max_risk":    maxInt(sess.MaxRisk, risk),
	}).Error
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ─── 查询 / 导出 / 清理 ───

// ListSessions 会话列表（按最近活跃倒序）
func ListSessions(page, pageSize int, keyword string, minRisk int) ([]Session, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	q := models.DB.Model(&Session{})
	if kw := strings.TrimSpace(keyword); kw != "" {
		q = q.Where("src_ip LIKE ? OR hid LIKE ? OR ua_norm LIKE ?", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
	}
	if minRisk > 0 {
		q = q.Where("max_risk >= ?", minRisk)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Session
	if err := q.Order("last_seen DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// SessionDetail 会话详情（含事件时间轴）
func SessionDetail(id uint) (*Session, []Event, error) {
	var s Session
	if err := models.DB.Where("id = ?", id).First(&s).Error; err != nil {
		return nil, nil, errors.New("会话不存在")
	}
	var events []Event
	_ = models.DB.Where("session_id = ?", id).Order("created_at ASC").Limit(500).Find(&events).Error
	return &s, events, nil
}

// Overview 概览统计（后台面板顶部卡片）
type Overview struct {
	Setting      Setting `json:"setting"`
	SessionCount int64   `json:"session_count"`
	EventCount   int64   `json:"event_count"`
	HighRisk     int64   `json:"high_risk"`
	Last24h      int64   `json:"last_24h"`
	BannedCount  int     `json:"banned_count"`
}

// GetOverview 概览
func GetOverview() Overview {
	var o Overview
	o.Setting = Current()
	models.DB.Model(&Session{}).Count(&o.SessionCount)
	models.DB.Model(&Event{}).Count(&o.EventCount)
	models.DB.Model(&Session{}).Where("max_risk >= ?", 60).Count(&o.HighRisk)
	models.DB.Model(&Event{}).Where("created_at >= ?", time.Now().Add(-24*time.Hour)).Count(&o.Last24h)
	o.BannedCount = len(BannedList())
	return o
}

// Export 导出取证数据（JSON；管理员操作本身也要留痕）
func Export(sessionID uint, operatorID uint) (map[string]interface{}, error) {
	out := map[string]interface{}{"exported_at": time.Now(), "operator_id": operatorID}
	if sessionID > 0 {
		s, events, err := SessionDetail(sessionID)
		if err != nil {
			return nil, err
		}
		out["session"] = s
		out["events"] = events
	} else {
		var sessions []Session
		_ = models.DB.Order("last_seen DESC").Limit(1000).Find(&sessions).Error
		var events []Event
		_ = models.DB.Order("created_at DESC").Limit(5000).Find(&events).Error
		out["sessions"] = sessions
		out["events"] = events
	}
	out["setting"] = Current()
	return out, nil
}

// LogExport 记录「管理员导出取证数据」这一动作（导出需留痕）
func LogExport(sessionID, operatorID uint) {
	now := time.Now()
	ev := &Event{
		SessionID: sessionID, Kind: EventExport, Decoy: "export",
		Method: "EXPORT", Path: "/admin/honeypot/export",
		BodySnippet: "操作人：" + strconv.FormatUint(uint64(operatorID), 10),
		Risk:        0, Action: "export", CreatedAt: now,
	}
	_ = models.DB.Create(ev).Error
}

// Cleanup 按留存天数清理（append-only 的例外：TTL 归档）
func Cleanup() int64 {
	set := Current()
	days := set.RetentionDays
	if days <= 0 {
		days = 90
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	res := models.DB.Where("created_at < ?", cutoff).Delete(&Event{})
	return res.RowsAffected
}
