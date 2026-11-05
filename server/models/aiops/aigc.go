package aiops

import (
	"encoding/json"
	"nvs-server/models"
	"strings"
	"time"
)

// ==================== AIGC 内容标识（GB 45438—2025 / TC260 标识办法）====================
//
// 三张表各管一件事，互不混用：
//   · AigcAsset     —— 原图（**不带任何标识**，不是对外交付物）
//   · AigcExport    —— 每次导出生成一个 ProduceID，并留下 ProduceID → 模型名 的映射
//   · AigcWaiverLog —— 显式标识豁免的知情记录（独立表，**不复用隔离墙表**）
//
// 这么做是因为国标要求「隐式标识只保留一份」且「模型名不属于法定字段」：
// 模型名只能进业务库，靠 ProduceID 回查；法定七字段一个字都不许多。

const aigcConfigKey = "aigc_label_config"

// AigcConfig 后台可配置项（NFR-4）
type AigcConfig struct {
	// Enabled 总开关。关掉后出口管道不再加角标/写字段（仅限私有部署自担责任）
	Enabled bool `json:"enabled"`
	// LabelText 显式角标文案；国标要求同时含「人工智能/AI」与「生成/合成」要素
	LabelText string `json:"label_text"`
	// Position 角标位置：bottom-right / bottom-left / top-right / top-left
	Position string `json:"position"`
	// FontSizePct 字号占图片最短边的比例（国标要求 ≥5%）
	FontSizePct float64 `json:"font_size_pct"`
	// TextColor 文字颜色 #RRGGBB
	TextColor string `json:"text_color"`
	// TextOpacity 文字不透明度 0~1
	TextOpacity float64 `json:"text_opacity"`
	// ShadowOpacity 描边/阴影不透明度（压在浅色图上否则看不清）
	ShadowOpacity float64 `json:"shadow_opacity"`
	// ContentProducer 生成合成服务提供者名称或 TC260 27 位编码（必填，不得填 API 厂商）
	ContentProducer string `json:"content_producer"`
	// ReservedIncludeModel 是否把模型标识随文件写进 ReservedCode1 复合格式（h=…;m=…）
	ReservedIncludeModel bool `json:"reserved_include_model"`
	// WaiverEnabled 是否允许用户申请「不含显式标识」的版本（默认关闭）
	WaiverEnabled bool `json:"waiver_enabled"`
	// WaiverAgreement 豁免协议核心条款（弹窗展示）
	WaiverAgreement string `json:"waiver_agreement"`
	// WaiverConfirmText 用户必须手动逐字输入的确认语句
	WaiverConfirmText string `json:"waiver_confirm_text"`
	// PromptAttachEnabled 是否允许把提示词作为附加信息写进文件（非标识）
	PromptAttachEnabled bool `json:"prompt_attach_enabled"`
	// MaxOutputPixels 单次导出的像素上限（防一次请求把内存打爆）
	MaxOutputPixels int `json:"max_output_pixels"`
}

// DefaultAigcConfig 默认配置：默认就具备合规能力（角标 + 隐式字段），豁免默认关闭
func DefaultAigcConfig() AigcConfig {
	producer := strings.TrimSpace(models.GetSiteName())
	if producer == "" {
		producer = "本站"
	}
	return AigcConfig{
		Enabled:              true,
		LabelText:            "AI生成",
		Position:             "bottom-right",
		FontSizePct:          0.05,
		TextColor:            "#FFFFFF",
		TextOpacity:          0.85,
		ShadowOpacity:        0.30,
		ContentProducer:      producer,
		ReservedIncludeModel: false,
		WaiverEnabled:        false,
		WaiverAgreement: "本版本不含显式标识。您知悉：依据《人工智能生成合成内容标识办法》第十条，" +
			"您将该内容对外发布时，应当自行添加显式标识；因未标识造成的后果由发布者承担。",
		WaiverConfirmText:   "我确认申请不含显式标识版本，并知悉发布时须自行标识",
		PromptAttachEnabled: false,
		MaxOutputPixels:     40_000_000,
	}
}

// LoadAigcConfig 读取配置（坏值回退默认）
func LoadAigcConfig() AigcConfig {
	c := DefaultAigcConfig()
	if raw := strings.TrimSpace(models.GetPlatformConfig(aigcConfigKey)); raw != "" {
		var got AigcConfig
		if err := json.Unmarshal([]byte(raw), &got); err == nil {
			c = got
		}
	}
	c.normalize()
	return c
}

// SaveAigcConfig 保存配置
func SaveAigcConfig(c AigcConfig) error {
	c.normalize()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return models.SetPlatformConfig(aigcConfigKey, string(b))
}

// normalize 把越界值拉回安全区间：角标字号低于 5% 就不合规了，直接抬上来
func (c *AigcConfig) normalize() {
	c.LabelText = strings.TrimSpace(c.LabelText)
	if c.LabelText == "" {
		c.LabelText = "AI生成"
	}
	if len([]rune(c.LabelText)) > 24 {
		c.LabelText = string([]rune(c.LabelText)[:24])
	}
	switch c.Position {
	case "bottom-right", "bottom-left", "top-right", "top-left":
	default:
		c.Position = "bottom-right"
	}
	// 国标：文字高度 ≥ 最短边 5%。上限 15% 只是别把图糊掉
	if c.FontSizePct < 0.05 {
		c.FontSizePct = 0.05
	}
	if c.FontSizePct > 0.15 {
		c.FontSizePct = 0.15
	}
	if !validHexColor(c.TextColor) {
		c.TextColor = "#FFFFFF"
	}
	c.TextOpacity = clamp01(c.TextOpacity)
	c.ShadowOpacity = clamp01(c.ShadowOpacity)
	if c.TextOpacity < 0.5 {
		c.TextOpacity = 0.5
	}
	c.ContentProducer = strings.TrimSpace(c.ContentProducer)
	if c.MaxOutputPixels <= 0 {
		c.MaxOutputPixels = 40_000_000
	}
	if c.MaxOutputPixels > 120_000_000 {
		c.MaxOutputPixels = 120_000_000
	}
	c.WaiverAgreement = strings.TrimSpace(c.WaiverAgreement)
	c.WaiverConfirmText = strings.TrimSpace(c.WaiverConfirmText)
	if c.WaiverEnabled && c.WaiverConfirmText == "" {
		c.WaiverConfirmText = DefaultAigcConfig().WaiverConfirmText
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func validHexColor(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// ---------- 原图 ----------

const (
	AigcSourceAI   = "ai_generated"
	AigcSourceUser = "user_upload"
)

// AigcAsset 原图。**不含任何标识**，只作为出口管道的输入。
type AigcAsset struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"not null;index" json:"user_id"`
	TaskID uint `gorm:"not null;default:0;index" json:"task_id"`
	// Source ai_generated / user_upload —— 决定出口管道是否写标识（FR-7.2）
	Source string `gorm:"size:16;not null;default:ai_generated;index" json:"source"`
	// Index 同一次出图里的第几张（多图导出时按它取，避免总是取到最新一张）
	Index int `gorm:"not null;default:0" json:"index"`
	// ModelName 仅用于业务库映射，**永远不进法定字段**
	ModelName string `gorm:"size:64;default:''" json:"model_name"`
	FilePath  string `gorm:"size:512;not null" json:"file_path"`
	Mime      string `gorm:"size:64;default:''" json:"mime"`
	Width     int    `gorm:"not null;default:0" json:"width"`
	Height    int    `gorm:"not null;default:0" json:"height"`
	SizeBytes int64  `gorm:"not null;default:0" json:"size_bytes"`
	SHA256    string `gorm:"size:64;default:''" json:"sha256"`
	// FromURL 原始上游地址（远程图），便于排障；不放给用户
	FromURL   string    `gorm:"size:512;default:''" json:"-"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (AigcAsset) TableName() string { return "aigc_assets" }

// ---------- 导出记录（ProduceID → 模型名 的映射） ----------

// AigcExport 一次导出 = 一个 ProduceID。国标要求每条内容有唯一编号，
// 且模型名不属于法定字段 —— 所以映射只能落在这里。
type AigcExport struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ProduceID string `gorm:"size:64;not null;uniqueIndex" json:"produce_id"`
	AssetID   uint   `gorm:"not null;index" json:"asset_id"`
	UserID    uint   `gorm:"not null;index" json:"user_id"`
	ModelName string `gorm:"size:64;default:''" json:"model_name"`
	Format    string `gorm:"size:8;default:jpeg" json:"format"`
	Width     int    `gorm:"not null;default:0" json:"width"`
	Height    int    `gorm:"not null;default:0" json:"height"`
	// Waived 本次是否走了显式标识豁免（隐式字段始终写入）
	Waived  bool `gorm:"not null;default:false" json:"waived"`
	Labeled bool `gorm:"not null;default:false" json:"labeled"`
	// LabelText / FontSize 便于后台核验「角标是否合规」
	LabelText       string `gorm:"size:64;default:''" json:"label_text"`
	LabelFontSize   int    `gorm:"not null;default:0" json:"label_font_size"`
	ContentProducer string `gorm:"size:128;default:''" json:"content_producer"`
	ReservedCode1   string `gorm:"size:128;default:''" json:"reserved_code1"`
	PromptAttached  bool   `gorm:"not null;default:false" json:"prompt_attached"`
	// AIGCJSON 实际写进文件的那份字段（核验时不用再解码图片）
	AIGCJSON  string    `gorm:"type:text" json:"aigc_json"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (AigcExport) TableName() string { return "aigc_exports" }

// ---------- 豁免日志（独立表，不与隔离墙混用） ----------

// AigcWaiverLog 显式标识豁免的知情记录：原文 + 哈希 + 协议版本 + 结果，留存 ≥180 天。
type AigcWaiverLog struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	UserID   uint   `gorm:"not null;index" json:"user_id"`
	UserName string `gorm:"size:64;default:''" json:"user_name"`
	// ProduceID 对应这次导出的内容编号（导出成功才有值）
	ProduceID string `gorm:"size:64;index" json:"produce_id"`
	// RequestTime 用户提交时间（与 CreatedAt 分开：便于按用户口径统计）
	RequestTime      time.Time `json:"request_time"`
	AgreementVersion string    `gorm:"size:32;default:''" json:"agreement_version"`
	InputText        string    `gorm:"type:text" json:"input_text"` // 原文，不做清洗
	InputHash        string    `gorm:"size:64;default:''" json:"input_hash"`
	// ResponseResult granted / denied（输入不符或站长关闭豁免时为 denied）
	ResponseResult string    `gorm:"size:16;default:''" json:"response_result"`
	DenyReason     string    `gorm:"size:128;default:''" json:"deny_reason"`
	IP             string    `gorm:"size:64;default:''" json:"ip"`
	UA             string    `gorm:"size:256;default:''" json:"ua"`
	CreatedAt      time.Time `gorm:"index" json:"created_at"`
}

func (AigcWaiverLog) TableName() string { return "aigc_waiver_logs" }

// ---------- CRUD ----------

// CreateAigcAsset 记录一张原图
func CreateAigcAsset(a *AigcAsset) error {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	return models.DB.Create(a).Error
}

// GetAigcAsset 取原图（校验归属由调用方负责）
func GetAigcAsset(id uint) (*AigcAsset, error) {
	var a AigcAsset
	if err := models.DB.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAigcAssetByTaskIndex 按「任务 + 第几张」取原图；同图去重（内容寻址）时可能只有一条，
// 所以取不到就回落到该任务下 id 最小的那条，别让用户点了第二张却导不出。
func GetAigcAssetByTaskIndex(taskID uint, index int) (*AigcAsset, error) {
	var a AigcAsset
	if err := models.DB.Where("task_id = ? AND `index` = ?", taskID, index).First(&a).Error; err == nil {
		return &a, nil
	}
	if err := models.DB.Where("task_id = ?", taskID).Order("id ASC").First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// CreateAigcExport 落一条导出记录。
//
// ⚠️ GORM 的 Create 会**跳过零值**，让列默认值生效；而这三个 bool 的「正确值」经常是 false
// （用户上传图不标角标、没走豁免、没带提示词）。所以创建后必须用 map 回填一次，
// 否则数据库里会记成 true——审计时看起来「全都标了」，实际没有。
func CreateAigcExport(e *AigcExport) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	if err := models.DB.Create(e).Error; err != nil {
		return err
	}
	return models.DB.Model(&AigcExport{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"labeled":         e.Labeled,
		"waived":          e.Waived,
		"prompt_attached": e.PromptAttached,
	}).Error
}

// GetAigcExportByProduceID 用 ProduceID 回查（验收 #11）
func GetAigcExportByProduceID(pid string) (*AigcExport, error) {
	var e AigcExport
	if err := models.DB.Where("produce_id = ?", pid).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ListAigcExports 后台核验列表
func ListAigcExports(page, size int) ([]AigcExport, int64) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	var total int64
	models.DB.Model(&AigcExport{}).Count(&total)
	var list []AigcExport
	models.DB.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list)
	return list, total
}

// CreateAigcWaiverLog 落豁免日志
func CreateAigcWaiverLog(l *AigcWaiverLog) error {
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	if l.RequestTime.IsZero() {
		l.RequestTime = l.CreatedAt
	}
	return models.DB.Create(l).Error
}

// ListAigcWaiverLogs 后台豁免日志列表
func ListAigcWaiverLogs(page, size int) ([]AigcWaiverLog, int64) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	var total int64
	models.DB.Model(&AigcWaiverLog{}).Count(&total)
	var list []AigcWaiverLog
	models.DB.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list)
	return list, total
}
