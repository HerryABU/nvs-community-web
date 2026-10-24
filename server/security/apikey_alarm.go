package security

// ==================== API Key 审计告警（gap-audit #15 告警口径）====================
// requirements-audit 里这条是 🟡：「有 LastUsedAt 与使用上限；HMAC 指纹与告警未做」。
// 补齐三件：
//   1. **审计用 HMAC 指纹**：不存明文、不存可逆密文的前提下，能回答
//      「这把 key 是不是就是当时那把」——审计/对账/工单里经常需要这个。
//   2. **多 IP 提醒**：同一把 key 在 24h 内从多个 IP 使用（共享/泄露的典型特征）。
//   3. **异常解密告警**：解密失败（机器密钥轮换、密文被改）必须出告警，不能只在日志里打个 error。
//
// 注意：多 IP 只是**提醒**，不是封禁依据——移动网络切换基站、公司出口 IP 池都会触发。
// 所以这里只产出告警（带 ip 列表），是否处置由人决定。

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"nvs-server/models"
)

// 告警类型
const (
	AlarmMultiIP      = "multi_ip"      // 同 key 多 IP
	AlarmDecryptFail  = "decrypt_fail"  // 密钥解密失败
	AlarmBannedUse    = "banned_use"    // 已封禁 key 仍被使用
	AlarmHMACMismatch = "hmac_mismatch" // 指纹不匹配（密文被替换）
)

// apiKeyAuditSecret 审计 HMAC 密钥：优先用环境变量，否则用进程内稳定派生值。
// 说明：它的作用只是「同一把 key 在同一部署里指纹一致」，不是加密强度保证。
func apiKeyAuditSecret() []byte {
	if v := strings.TrimSpace(envOr("NVS_KEY_AUDIT_SECRET", "")); v != "" {
		return []byte(v)
	}
	// 没有配置时退化：用 JWT secret + 固定标签派生（同一部署内稳定，跨部署不通用）
	base := envOr("JWT_SECRET", "nvs-default-audit-secret")
	return []byte("nvs-apikey-audit::" + base)
}

// APIKeyHMAC 生成审计指纹：HMAC-SHA256(secret, prefix + ":" + plain)[:32]
// 用途：审计日志/工单里引用指纹，既能对上号，又不泄露 key。
func APIKeyHMAC(prefix, plain string) string {
	mac := hmac.New(sha256.New, apiKeyAuditSecret())
	mac.Write([]byte(prefix + ":" + strings.TrimSpace(plain)))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// VerifyAPIKeyHMAC 校验指纹（审计复核用）
func VerifyAPIKeyHMAC(prefix, plain, want string) bool {
	return hmac.Equal([]byte(APIKeyHMAC(prefix, plain)), []byte(strings.TrimSpace(want)))
}

// ─────────────── 多 IP 跟踪 ───────────────

type ipTrack struct {
	ips      map[string]time.Time
	lastSeen time.Time
}

var (
	ipMu     sync.Mutex
	ipTracks = map[uint]*ipTrack{} // keyID → 最近 IP
)

// TrackKeyIP 记录一把 key 的最近使用 IP；超过阈值时写一条告警（同 key 1 小时内不重复告警）
func TrackKeyIP(keyID, userID uint, ip string) {
	ip = strings.TrimSpace(ip)
	if keyID == 0 || ip == "" {
		return
	}
	now := time.Now()
	ipMu.Lock()
	t, ok := ipTracks[keyID]
	if !ok {
		t = &ipTrack{ips: map[string]time.Time{}}
		ipTracks[keyID] = t
	}
	t.ips[ip] = now
	t.lastSeen = now
	// 清理 24h 前的 IP
	for k, at := range t.ips {
		if now.Sub(at) > 24*time.Hour {
			delete(t.ips, k)
		}
	}
	distinct := len(t.ips)
	list := make([]string, 0, distinct)
	for k := range t.ips {
		list = append(list, k)
	}
	// 内存表上限保护（避免长期运行无限增长）
	if len(ipTracks) > 5000 {
		for k, v := range ipTracks {
			if now.Sub(v.lastSeen) > 24*time.Hour {
				delete(ipTracks, k)
			}
		}
	}
	ipMu.Unlock()

	if distinct >= 3 {
		raiseAlarm(AlarmMultiIP, keyID, userID, 60,
			fmt.Sprintf("同一把 Key 24 小时内从 %d 个 IP 使用：%s",
				distinct, strings.Join(trimList(list, 6), ", ")))
	}
}

// ─────────────── 告警写入 ───────────────

// raiseAlarm 写告警（dedupeMinutes 内同 kind+key 只写一条，避免刷屏）
func raiseAlarm(kind string, keyID, userID uint, dedupeMinutes int, detail string) {
	if models.DB == nil {
		return
	}
	since := time.Now().Add(-time.Duration(dedupeMinutes) * time.Minute)
	var n int64
	_ = models.DB.Model(&models.ApiKeyAlarm{}).
		Where("kind = ? AND key_id = ? AND created_at >= ?", kind, keyID, since).Count(&n).Error
	if n > 0 {
		return
	}
	_ = models.DB.Create(&models.ApiKeyAlarm{
		Kind: kind, KeyID: keyID, UserID: userID,
		Detail: truncate(detail, 512), CreatedAt: time.Now(),
	}).Error
}

// ReportDecryptFailure 上报解密失败（由解密调用点捕获后调用）
func ReportDecryptFailure(keyID, userID uint, reason string) {
	raiseAlarm(AlarmDecryptFail, keyID, userID, 10, "密钥解密失败："+reason)
}

// ReportBannedKeyUse 封禁 Key 仍被使用（说明客户端没换 key，或有人在试旧 key）
func ReportBannedKeyUse(keyID, userID uint, ip string) {
	raiseAlarm(AlarmBannedUse, keyID, userID, 30, "已封禁的 Key 仍被使用，来源 IP "+ip)
}

// ReportHMACMismatch 指纹不匹配（密文被替换/密钥轮换）
func ReportHMACMismatch(keyID, userID uint, detail string) {
	raiseAlarm(AlarmHMACMismatch, keyID, userID, 10, detail)
}

// ─────────────── 查询 ───────────────

// ListKeyAlarms 告警列表
func ListKeyAlarms(page, size int, kind string, acked *bool) ([]models.ApiKeyAlarm, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	q := models.DB.Model(&models.ApiKeyAlarm{})
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if acked != nil {
		q = q.Where("acked = ?", *acked)
	}
	var total int64
	q.Count(&total)
	var list []models.ApiKeyAlarm
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// AckKeyAlarm 标记已处理
func AckKeyAlarm(id, operatorID uint, note string) error {
	now := time.Now()
	return models.DB.Model(&models.ApiKeyAlarm{}).Where("id = ?", id).Updates(map[string]interface{}{
		"acked": true, "acked_by": operatorID, "acked_at": now, "note": truncate(note, 256),
	}).Error
}

func trimList(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	return list[:n]
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// ResetIPTracking 测试用
func ResetIPTracking() {
	ipMu.Lock()
	ipTracks = map[uint]*ipTrack{}
	ipMu.Unlock()
}
