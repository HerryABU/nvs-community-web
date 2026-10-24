package honeypot

// ==================== 指纹采集（M3） ====================
// 验证页 DOMContentLoaded 即采集（不等提交）：攻击者关掉页面也已经拿到数据。
// 采集项与真实风控靠拢但**只做标记不硬拒**（硬拒会立刻暴露这是蜜罐）。
// 合规：仅管理员可读；随事件按留存天数一起清理；不做跨站追踪。

import (
	"encoding/json"
	"time"
)

// Fingerprint 浏览器/环境指纹
type Fingerprint struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	SessionID uint   `gorm:"not null;index" json:"session_id"`
	Canvas    string `gorm:"size:64;default:''" json:"canvas"`
	WebGL     string `gorm:"size:64;default:''" json:"webgl"`
	Fonts     string `gorm:"size:64;default:''" json:"fonts"`
	TZ        string `gorm:"size:64;default:''" json:"tz"`
	Lang      string `gorm:"size:64;default:''" json:"lang"`
	Screen    string `gorm:"size:32;default:''" json:"screen"`
	DPR       string `gorm:"size:16;default:''" json:"dpr"`
	Cores     string `gorm:"size:16;default:''" json:"cores"`
	Memory    string `gorm:"size:16;default:''" json:"memory"`
	Hints     string `gorm:"size:256;default:''" json:"client_hints"`
	Headless  string `gorm:"size:256;default:''" json:"headless_flags"`
	// TLS 指纹（反代透传）：缺失记 missing，不伪造；来源记 source
	JA3       string `gorm:"size:64;default:'';index" json:"ja3"`
	JA4       string `gorm:"size:64;default:''" json:"ja4"`
	TLSSource string `gorm:"size:24;default:''" json:"tls_source"`
	TLSVer    string `gorm:"size:16;default:''" json:"tls_version"`
	// WebRTC ICE 候选里的 host 地址（内网 IP / mDNS 名），仅作关联线索
	WebRTCIPs string    `gorm:"size:256;default:''" json:"webrtc_ips"`
	Raw       string    `gorm:"type:text" json:"raw"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (Fingerprint) TableName() string { return "honeypot_fingerprints" }

// FingerprintInput 上报载荷（前端 JS 采集）
type FingerprintInput struct {
	Canvas   string `json:"canvas"`
	WebGL    string `json:"webgl"`
	Fonts    string `json:"fonts"`
	TZ       string `json:"tz"`
	Lang     string `json:"lang"`
	Screen   string `json:"screen"`
	DPR      string `json:"dpr"`
	Cores    string `json:"cores"`
	Memory   string `json:"memory"`
	Hints    string `json:"hints"`
	Headless string `json:"headless"`
	Raw      string `json:"raw"`
	WebRTC   string `json:"webrtc"` // 前端 RTCPeerConnection 采集（逗号分隔）
	// TLS 由服务端注入（不信任前端上报）
	JA3       string `json:"-"`
	JA4       string `json:"-"`
	TLSSource string `json:"-"`
	TLSVer    string `json:"-"`
}

// SaveFingerprint 落库（同会话同指纹去重）
func SaveFingerprint(sessionID uint, in FingerprintInput) error {
	var n int64
	db().Model(&Fingerprint{}).
		Where("session_id = ? AND canvas = ? AND webgl = ?", sessionID, in.Canvas, in.WebGL).
		Count(&n)
	if n > 0 {
		return nil
	}
	if len(in.Raw) > 4000 {
		in.Raw = in.Raw[:4000]
	}
	return db().Create(&Fingerprint{
		SessionID: sessionID,
		Canvas:    truncate(in.Canvas, 64), WebGL: truncate(in.WebGL, 64),
		Fonts: truncate(in.Fonts, 64), TZ: truncate(in.TZ, 64), Lang: truncate(in.Lang, 64),
		Screen: truncate(in.Screen, 32), DPR: truncate(in.DPR, 16),
		Cores: truncate(in.Cores, 16), Memory: truncate(in.Memory, 16),
		Hints: truncate(in.Hints, 256), Headless: truncate(in.Headless, 256),
		JA3: truncate(in.JA3, 64), JA4: truncate(in.JA4, 64),
		TLSSource: truncate(in.TLSSource, 24), TLSVer: truncate(in.TLSVer, 16),
		WebRTCIPs: truncate(in.WebRTC, 256),
		Raw:       in.Raw, CreatedAt: time.Now(),
	}).Error
}

// ListFingerprints 会话的指纹
func ListFingerprints(sessionID uint) ([]Fingerprint, error) {
	var list []Fingerprint
	if err := db().Where("session_id = ?", sessionID).
		Order("id DESC").Limit(20).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// fingerprintJSON 生成可导出的指纹摘要（取证包用）
func fingerprintJSON(fp Fingerprint) string {
	b, err := json.Marshal(fp)
	if err != nil {
		return "{}"
	}
	return string(b)
}
