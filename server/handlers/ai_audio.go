package handlers

// ==================== 语音合成 / 音乐（gap-audit #18）====================
// POST /api/ai/speech
//   { text, voice?, kind?(tts|music), model_id? }
// 产物落 data/uploads/yyyy/mm/*.mp3，返回站内 URL（与图片/视频同一套存储与静态服务）。
//
// 三条硬约束（比「能不能生成」重要得多）：
//   1. 文本长度上限（默认 2000 字）：TTS 按字符计费，不限制就是成本黑洞；
//   2. 必须有可用的 audio 模型（`ai_models.kind = audio`），否则直接报「未配置」，
//      不要偷偷回落到 chat 模型去「假装」生成；
//   3. 产物不覆盖、不删除他人文件；文件名随机，URL 不可枚举。

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/security"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

const maxSpeechRunes = 2000

// Speech POST /api/ai/speech
func Speech(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	var req struct {
		Text    string `json:"text"`
		Voice   string `json:"voice"`
		Kind    string `json:"kind"`     // tts（默认）/ music
		ModelID uint   `json:"model_id"` // 0=自动选一个可用的 audio 模型
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		utils.BadRequest(c, "内容不能为空")
		return
	}
	if n := len([]rune(text)); n > maxSpeechRunes {
		utils.BadRequest(c, fmt.Sprintf("内容过长（%d 字，上限 %d 字）", n, maxSpeechRunes))
		return
	}
	kind := "tts"
	if req.Kind == "music" {
		kind = "music"
	}

	m := pickAudioModel(req.ModelID)
	if m == nil {
		utils.BadRequest(c, "未配置语音/音乐模型，请管理员在「AI 模型池」新增 kind=audio 的模型")
		return
	}
	// 内容审核：语音念出来的违规内容和文字一样违规
	clean, hits := security.ApplyBadWords(text, "ai")
	if len(hits) > 0 && clean == "" {
		utils.Forbidden(c, "内容不合规")
		return
	}

	// 计费：audio 按次（PriceCall）；免费额度覆盖时扣免费次数
	if err := chargeAudioCall(uid, m); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	lease, kerr := models.AcquireModelKey(m)
	if kerr != nil {
		utils.BadRequest(c, "模型密钥不可用")
		return
	}
	apiKey := ""
	if lease != nil {
		apiKey = lease.Key
	}
	client := aiclient.NewClient(m.BaseURL, apiKey, 120*time.Second)
	voice := req.Voice
	if voice == "" {
		voice = "alloy"
	}
	data, mime, err := client.Speech(aiclient.SpeechRequest{
		Model: m.ModelID, Input: clean, Voice: voice, ResponseFormat: "mp3",
	})
	if err != nil {
		if lease != nil {
			lease.Fail()
		}
		if kind == "music" {
			utils.InternalError(c, "音乐生成失败："+err.Error())
		} else {
			utils.InternalError(c, "语音合成失败："+err.Error())
		}
		return
	}
	lease.OK()

	url, werr := saveAudioFile(data, aiclient.SpeechExt(mime))
	if werr != nil {
		utils.InternalError(c, "保存音频失败")
		return
	}
	utils.Success(c, gin.H{
		"url": url, "mime": mime, "size": len(data),
		"model": m.Name, "kind": kind, "voice": voice,
	})
}

// pickAudioModel 选可用的 audio 模型（指定 ID 时校验 kind）
func pickAudioModel(id uint) *models.AiModel {
	var m models.AiModel
	if id > 0 {
		if err := models.DB.Where("id = ? AND enabled = ?", id, true).First(&m).Error; err != nil {
			return nil
		}
		if m.Kind != "audio" {
			return nil
		}
		return &m
	}
	if err := models.DB.Where("kind = ? AND enabled = ?", "audio", true).
		Order("priority DESC, id ASC").First(&m).Error; err != nil {
		return nil
	}
	return &m
}

// chargeAudioCall 扣费：免费额度优先，否则扣 PriceCall 代币
func chargeAudioCall(userID uint, m *models.AiModel) error {
	// 免费额度：占用 1 次调用（与文本共用池子，站长可统一控总量）
	if ok, _, _ := models.CheckAiFreeQuota(userID, 0); ok {
		_ = models.RecordAiFreeUsage(userID, 1, 0)
		return nil
	}
	price := m.PriceCall
	if price <= 0 {
		return nil
	}
	if !models.IsTokenEnabled() {
		return fmt.Errorf("免费额度已用完且代币未启用")
	}
	if models.GetTokenBalance(userID) < price {
		return fmt.Errorf("余额不足（本次需要 %d %s）", price, models.GetTokenName())
	}
	if _, err := models.AppendTokenLedger(userID, "consume", -price, "ai_audio", m.ID,
		"语音/音乐生成："+m.Name, ""); err != nil {
		return fmt.Errorf("扣费失败：%v", err)
	}
	return nil
}

// saveAudioFile 落盘到 uploads（随机名，防枚举）
func saveAudioFile(data []byte, ext string) (string, error) {
	if ext == "" {
		ext = ".mp3"
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	name := "tts_" + hex.EncodeToString(b) + ext
	relDir := time.Now().Format("2006/01")
	absDir := filepath.Join(config.UploadDir, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(absDir, name), data, 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("/uploads/%s/%s", relDir, name), nil
}

// AdminSpeechModels GET /api/admin/ai/audio-models —— 供后台提示「有没有可用的语音模型」
func AdminSpeechModels(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var list []models.AiModel
	_ = models.DB.Where("kind = ?", "audio").Order("priority DESC, id ASC").Find(&list).Error
	utils.Success(c, gin.H{
		"list": list, "count": len(list),
		"hint":      "kind=audio 的模型用于语音合成/音乐；未配置时 /api/ai/speech 会返回「未配置」而不是假装生成",
		"max_runes": maxSpeechRunes,
	})
}
