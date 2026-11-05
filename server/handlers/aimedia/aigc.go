package aimedia

// ==================== AIGC 标识：出口管道 + 合规配置 ====================
//
// 设计要点（对应需求文档）：
//   · 原图原封不动落盘，不带任何标识（FR-1）——落盘在出图成功后异步做，不拖慢出图；
//   · 用户下载走 POST /api/ai/aigc/export，**服务端**合成 + 烧角标 + 写 XMP 后返回二进制，
//     前端不碰像素、不碰元数据（FR-2/FR-3/FR-4）；
//   · 导出不落盘、不返回 URL，避免产生「无标识的公开副本」绕过义务（FR-7.1）；
//   · 豁免只免显式角标，隐式字段无条件写（FR-3.5/FR-6.3），并留独立日志（FR-6.4）。

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"nvs-server/utils/infra"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nvs-server/aigc"
	"nvs-server/config"
	"nvs-server/handlers/hctx"
	"nvs-server/handlers/web"
	"nvs-server/models"

	"github.com/gin-gonic/gin"
	aiops "nvs-server/models/aiops"
)

// aigcRoot 资源根目录：与 uploads 同级（默认 ./data/aigc）
func aigcRoot() string {
	return filepath.Join(filepath.Dir(config.UploadDir), "aigc")
}

func aigcOriginalsDir() string { return filepath.Join(aigcRoot(), "originals") }

// toPipelineConfig models 配置 → 管道配置
func toPipelineConfig(m aiops.AigcConfig) aigc.Config {
	return aigc.Config{
		Enabled:              m.Enabled,
		LabelText:            m.LabelText,
		Position:             m.Position,
		FontSizePct:          m.FontSizePct,
		TextColor:            m.TextColor,
		TextOpacity:          m.TextOpacity,
		ShadowOpacity:        m.ShadowOpacity,
		ContentProducer:      m.ContentProducer,
		ReservedIncludeModel: m.ReservedIncludeModel,
		MaxOutputPixels:      m.MaxOutputPixels,
	}
}

// InitAigcFont 启动时挂载站长自定义字体（没有就用内置子集）
func InitAigcFont() {
	p := filepath.Join(aigcRoot(), "label.ttf")
	if st, err := os.Stat(p); err == nil && st.Size() > 1024 {
		aigc.SetCustomFontPath(p)
		return
	}
	aigc.SetCustomFontPath("")
}

// ---------- 原图落盘 ----------

// readAigcSource 把一张图的「地址」变成字节：支持 data URL / 站内 /uploads / http(s)
func readAigcSource(raw string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", fmt.Errorf("空地址")
	}
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		// data:image/png;base64,xxxx
		i := strings.Index(raw, ",")
		if i < 0 {
			return nil, "", fmt.Errorf("data URL 格式错误")
		}
		meta, payload := raw[:i], raw[i+1:]
		if !strings.Contains(meta, "base64") {
			return nil, "", fmt.Errorf("data URL 不是 base64")
		}
		b, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, "", err
		}
		return b, "", nil
	}
	_, data, reason := fetchMediaBytes(raw)
	if len(data) == 0 {
		return nil, "", fmt.Errorf("%s", reason)
	}
	return data, raw, nil
}

// SaveAigcOriginalsAsync 出图成功后异步把原图落盘 + 记字节信息。
// 失败只记日志：不能因为落盘失败把已经扣过费的出图判成失败。
func SaveAigcOriginalsAsync(userID, taskID uint, modelName string, urls []string) {
	if len(urls) == 0 {
		return
	}
	go func() {
		dir := aigcOriginalsDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
		for idx, raw := range urls {
			data, fromURL, err := readAigcSource(raw)
			if err != nil || len(data) == 0 {
				continue
			}
			cfgImg, format, derr := image.DecodeConfig(bytes.NewReader(data))
			if derr != nil {
				continue
			}
			sum := sha256.Sum256(data)
			hexSum := hex.EncodeToString(sum[:])
			ext := extForImageFormat(format)
			name := hexSum[:16] + "." + ext
			full := filepath.Join(dir, name)
			if _, statErr := os.Stat(full); statErr != nil {
				if werr := os.WriteFile(full, data, 0o644); werr != nil {
					continue
				}
			}
			_ = aiops.CreateAigcAsset(&aiops.AigcAsset{
				UserID:    userID,
				TaskID:    taskID,
				Index:     idx,
				Source:    aiops.AigcSourceAI,
				ModelName: modelName,
				FilePath:  full,
				Mime:      "image/" + ext,
				Width:     cfgImg.Width,
				Height:    cfgImg.Height,
				SizeBytes: int64(len(data)),
				SHA256:    hexSum,
				FromURL:   fromURL,
			})
		}
	}()
}

func extForImageFormat(f string) string {
	switch strings.ToLower(f) {
	case "jpeg":
		return "jpg"
	case "png":
		return "png"
	case "webp":
		return "webp"
	case "gif":
		return "gif"
	default:
		return "bin"
	}
}

// AiAigcOptions GET /api/ai/aigc/options —— 前端据此决定显示哪些选项
//
// 只放"该不该显示开关"这类信息，不放 ContentProducer 等后台配置内容。
func AiAigcOptions(c *gin.Context) {
	cfg := aiops.LoadAigcConfig()
	infra.Success(c, gin.H{
		"enabled":               cfg.Enabled,
		"waiver_enabled":        cfg.WaiverEnabled,
		"waiver_agreement":      cfg.WaiverAgreement,
		"waiver_confirm_text":   cfg.WaiverConfirmText,
		"prompt_attach_enabled": cfg.PromptAttachEnabled,
		"label_text":            cfg.LabelText,
		"max_output_pixels":     cfg.MaxOutputPixels,
	})
}

// ---------- 出口管道 ----------

// AiAigcExport POST /api/ai/aigc/export
// 入参只有参数（裁剪/尺寸/格式/豁免/提示词），图片由服务端从原图重新合成。
func AiAigcExport(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var req struct {
		AssetID uint `json:"asset_id"`
		TaskID  uint `json:"task_id"`
		// ImageIndex 同一次出图有多张时，导出第几张（0 起）
		ImageIndex int `json:"image_index"`
		// 裁剪区域（像素，相对原图）
		CropX int `json:"crop_x"`
		CropY int `json:"crop_y"`
		CropW int `json:"crop_w"`
		CropH int `json:"crop_h"`
		// 输出尺寸
		TargetW int    `json:"target_w"`
		TargetH int    `json:"target_h"`
		Format  string `json:"format"`
		// 显式标识豁免
		Waive      bool   `json:"waive"`
		WaiverText string `json:"waiver_text"`
		// 提示词附加（非标识）
		IncludePrompt bool   `json:"include_prompt"`
		Prompt        string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}

	asset, err := resolveAigcAsset(uid, req.AssetID, req.TaskID, req.ImageIndex)
	if err != nil {
		infra.NotFound(c, "原图不存在或无权访问")
		return
	}
	data, rerr := os.ReadFile(asset.FilePath)
	if rerr != nil {
		infra.BadRequest(c, "原图已不在磁盘上，请重新生成")
		return
	}

	cfg := aiops.LoadAigcConfig()
	if cfg.Enabled && strings.TrimSpace(cfg.ContentProducer) == "" {
		infra.BadRequest(c, "站长还没填「生成合成服务提供者」，隐式标识无法合规写入")
		return
	}

	produceID := newProduceID()
	waived := false
	if req.Waive {
		reason := ""
		switch {
		case !cfg.WaiverEnabled:
			reason = "站长未开启显式标识豁免"
		case !sameConfirmText(req.WaiverText, cfg.WaiverConfirmText):
			reason = "确认语句不匹配（需逐字输入）"
		default:
			waived = true
		}
		if reason != "" {
			logAigcWaiver(c, uid, produceID, cfg, req.WaiverText, "denied", reason)
			infra.BadRequest(c, reason)
			return
		}
	}

	prompt := ""
	if req.IncludePrompt && cfg.PromptAttachEnabled {
		prompt = strings.TrimSpace(req.Prompt)
		if len([]rune(prompt)) > 2000 {
			prompt = string([]rune(prompt)[:2000])
		}
	}

	res, eerr := aigc.Export(aigc.ExportRequest{
		Source:     data,
		Mime:       asset.Mime,
		SourceIsAI: asset.Source == aiops.AigcSourceAI,
		CropX:      req.CropX, CropY: req.CropY, CropW: req.CropW, CropH: req.CropH,
		TargetW: req.TargetW, TargetH: req.TargetH,
		Format:     req.Format,
		WaiveLabel: waived,
		Prompt:     prompt,
		ProduceID:  produceID,
		ModelName:  asset.ModelName,
	}, toPipelineConfig(cfg))
	if eerr != nil {
		if req.Waive {
			logAigcWaiver(c, uid, produceID, cfg, req.WaiverText, "denied", "导出失败："+eerr.Error())
		}
		infra.BadRequest(c, aigcExportErrText(eerr))
		return
	}

	// 落导出记录：ProduceID → 模型名 的映射就在这里（模型名绝不进法定字段）
	fieldsJSON, _ := json.Marshal(res.Fields)
	_ = aiops.CreateAigcExport(&aiops.AigcExport{
		ProduceID:       produceID,
		AssetID:         asset.ID,
		UserID:          uid,
		ModelName:       asset.ModelName,
		Format:          res.Ext,
		Width:           res.Width,
		Height:          res.Height,
		Waived:          waived,
		Labeled:         res.Labeled,
		LabelText:       cfg.LabelText,
		LabelFontSize:   res.FontSize,
		ContentProducer: cfg.ContentProducer,
		ReservedCode1:   res.Fields.ReservedCode1,
		PromptAttached:  res.PromptAttached,
		AIGCJSON:        string(fieldsJSON),
	})
	if req.Waive {
		logAigcWaiver(c, uid, produceID, cfg, req.WaiverText, "granted", "")
	}

	filename := produceID + "." + res.Ext
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("X-AIGC-ProduceID", produceID)
	if res.Labeled {
		c.Header("X-AIGC-Labeled", "1")
	} else {
		c.Header("X-AIGC-Labeled", "0")
	}
	if res.Note != "" {
		// 中文不能直接进 header，用 ASCII 码让前端映射文案
		c.Header("X-AIGC-Note", "webp_fallback_jpeg")
	}
	c.Header("Cache-Control", "no-store")
	c.Data(200, res.Mime, res.Data)
}

func aigcExportErrText(err error) string {
	switch err {
	case aigc.ErrTooLarge:
		return "输出尺寸超出本站上限，请调小"
	case aigc.ErrBadCrop:
		return "裁剪区域超出原图范围"
	case aigc.ErrFontUnavailable:
		return "标识字体不可用（打包异常），请联系站长"
	}
	return "导出失败：" + err.Error()
}

// resolveAigcAsset 按 asset_id 或 task_id 取原图，并校验归属
func resolveAigcAsset(uid, assetID, taskID uint, index int) (*aiops.AigcAsset, error) {
	var a *aiops.AigcAsset
	var err error
	if assetID > 0 {
		a, err = aiops.GetAigcAsset(assetID)
	} else if taskID > 0 {
		a, err = aiops.GetAigcAssetByTaskIndex(taskID, index)
	} else {
		return nil, fmt.Errorf("缺少 asset_id 或 task_id")
	}
	if err != nil || a == nil {
		return nil, fmt.Errorf("not found")
	}
	if a.UserID != uid && !isAdminID(uid) {
		return nil, fmt.Errorf("forbidden")
	}
	return a, nil
}

// isAdminID 按用户 ID 判管理员（导出时允许站长查看任意原图）
func isAdminID(uid uint) bool {
	u, err := models.GetUserByID(uid)
	return err == nil && u != nil && u.Role == "admin"
}

func sameConfirmText(got, want string) bool {
	norm := func(s string) string {
		s = strings.ReplaceAll(s, "\u3000", " ")
		s = strings.ReplaceAll(s, "\r", "")
		s = strings.ReplaceAll(s, "\n", "")
		return strings.TrimSpace(s)
	}
	w := norm(want)
	if w == "" {
		return false
	}
	return norm(got) == w
}

func logAigcWaiver(c *gin.Context, uid uint, produceID string, cfg aiops.AigcConfig, input, result, reason string) {
	sum := sha256.Sum256([]byte(input))
	ver := sha256.Sum256([]byte(cfg.WaiverAgreement))
	name := ""
	if u, err := models.GetUserByID(uid); err == nil && u != nil {
		name = u.Username
	}
	_ = aiops.CreateAigcWaiverLog(&aiops.AigcWaiverLog{
		UserID:           uid,
		UserName:         name,
		ProduceID:        produceID,
		RequestTime:      time.Now(),
		AgreementVersion: "sha256:" + hex.EncodeToString(ver[:])[:8],
		InputText:        input,
		InputHash:        hex.EncodeToString(sum[:]),
		ResponseResult:   result,
		DenyReason:       reason,
		IP:               c.ClientIP(),
		UA:               truncateStr(c.GetHeader("User-Agent"), 250),
	})
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// newProduceID 内容编号：UUIDv4 形状（不引第三方库，标准库就够）
func newProduceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// ---------- 后台：配置 / 核验 / 豁免日志 ----------

// AdminGetAigcConfig GET /api/admin/aigc/config
func AdminGetAigcConfig(c *gin.Context) {
	infra.Success(c, gin.H{
		"config": aiops.LoadAigcConfig(),
		"font": gin.H{
			"custom": aiops.LoadAigcConfig() != aiops.DefaultAigcConfig() &&
				fileExists(filepath.Join(aigcRoot(), "label.ttf")),
			"builtin_note": "内置 Noto Sans SC 子集（SIL OFL 1.1），仅含角标常用字",
		},
	})
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Size() > 1024
}

// AdminSaveAigcConfig PUT /api/admin/aigc/config
func AdminSaveAigcConfig(c *gin.Context) {
	var cfg aiops.AigcConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if cfg.Enabled && strings.TrimSpace(cfg.ContentProducer) == "" {
		infra.BadRequest(c, "「生成合成服务提供者」必填：这是国标要求的主体信息")
		return
	}
	if err := aiops.SaveAigcConfig(cfg); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	infra.SuccessMessage(c, "已保存", aiops.LoadAigcConfig())
}

// AdminListAigcExports GET /api/admin/aigc/exports?page=&size=
func AdminListAigcExports(c *gin.Context) {
	page := web.AtoiDefault(c.Query("page"), 1)
	size := web.AtoiDefault(c.Query("size"), 20)
	list, total := aiops.ListAigcExports(page, size)
	infra.Success(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

// AdminListAigcWaivers GET /api/admin/aigc/waivers?page=&size=
func AdminListAigcWaivers(c *gin.Context) {
	page := web.AtoiDefault(c.Query("page"), 1)
	size := web.AtoiDefault(c.Query("size"), 20)
	list, total := aiops.ListAigcWaiverLogs(page, size)
	infra.Success(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

// AdminAigcVerify GET /api/admin/aigc/verify/:produce_id —— 用 ProduceID 回查模型名
func AdminAigcVerify(c *gin.Context) {
	pid := strings.TrimSpace(c.Param("produce_id"))
	if pid == "" {
		infra.BadRequest(c, "缺少 ProduceID")
		return
	}
	e, err := aiops.GetAigcExportByProduceID(pid)
	if err != nil {
		infra.NotFound(c, "没有这条导出记录")
		return
	}
	infra.Success(c, gin.H{
		"produce_id": e.ProduceID,
		"model_name": e.ModelName,
		"asset_id":   e.AssetID,
		"format":     e.Format,
		"size":       fmt.Sprintf("%dx%d", e.Width, e.Height),
		"waived":     e.Waived,
		"labeled":    e.Labeled,
		"label_text": e.LabelText,
		"label_font": e.LabelFontSize,
		"aigc_json":  e.AIGCJSON,
		"created_at": e.CreatedAt,
	})
}

// AdminUploadAigcFont POST /api/admin/aigc/font（multipart: file，仅 .ttf）
func AdminUploadAigcFont(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		infra.BadRequest(c, "请选择字体文件（.ttf）")
		return
	}
	if fh.Size > 20<<20 {
		infra.BadRequest(c, "字体文件过大（上限 20MB）")
		return
	}
	name := strings.ToLower(filepath.Base(fh.Filename))
	if !strings.HasSuffix(name, ".ttf") {
		infra.BadRequest(c, "只支持 .ttf 字体")
		return
	}
	dir := aigcRoot()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		infra.InternalError(c, "无法创建目录")
		return
	}
	dst := filepath.Join(dir, "label.ttf")
	if err := c.SaveUploadedFile(fh, dst); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	// 立刻生效：重新加载字体缓存，并做一次渲染自检（坏字体当场拒掉）
	aigc.SetCustomFontPath(dst)
	if _, err := aigc.FaceForSize(24); err != nil {
		_ = os.Remove(dst)
		aigc.SetCustomFontPath("")
		infra.BadRequest(c, "字体解析失败，已回退内置字体")
		return
	}
	infra.SuccessMessage(c, "字体已更新", gin.H{"path": dst})
}
