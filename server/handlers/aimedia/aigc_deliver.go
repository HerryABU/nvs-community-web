package aimedia

// ==================== 交付副本：出图即烧录 → 内容寻址 → 签名 URL ====================
//
// 与 aigc.go 的分工：
//   · aigc.go         —— 「用户点下载」时的按需导出（裁剪/尺寸/豁免），返回二进制、不落盘；
//   · aigc_deliver.go —— 出图成功**立即**生成一份带标识的交付副本并落盘，前端拿到的
//     就是这个文件的 URL。用户在图库里看到的、另存下来的，本身就是合规版本，
//     不会出现「先发一张干净原图、要下载时才补标识」的空档。
//
// 三条硬约束：
//   1. 文件名不可猜：ObjectKey = SHA-256(源图 + 参数 + 合规配置)，64 位 hex，
//      不含用户/任务/时间信息，无法靠枚举或规律推测；
//   2. 光有文件名也取不到：URL 必须带 HMAC 签名（绑 key + 过期时间 + 签发用户），
//      服务端先验签再读盘；存放目录不做静态映射（原图目录同理）；
//   3. 同图不重复存：同一张源图 + 同一套参数 ⇒ 同一个 ObjectKey ⇒ 磁盘只有一份，
//      不同用户命中时只追加一条授权记录（grants），ProduceID 一并复用。

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"nvs-server/aigc"
	"nvs-server/config"
	"nvs-server/handlers/hctx"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
	aiops "nvs-server/models/aiops"
)

const (
	// aigcDeliverTTL 签名有效期。过期不影响文件本身，读列表时服务端会重签。
	aigcDeliverTTL = 180 * 24 * time.Hour
	// aigcFileRoute 取图端点前缀（不含站点前缀，前端 mediaUrl() 负责补）
	aigcFileRoute = "/api/ai/aigc/file/"
)

// DeliveredImage 一张交付文件的对外视图
type DeliveredImage struct {
	URL     string `json:"url"`
	Key     string `json:"key"`
	Labeled bool   `json:"labeled"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	// Note 降级说明（如「角标烧录失败，已交付原图」）
	Note string `json:"note,omitempty"`
}

func aigcDeliverDir() string { return filepath.Join(aigcRoot(), "deliver") }

// deliveryPath 两级分片，避免单目录堆到几十万文件后 readdir 变慢
func deliveryPath(key, ext string) string {
	if len(key) < 4 {
		return filepath.Join(aigcDeliverDir(), key+"."+ext)
	}
	return filepath.Join(aigcDeliverDir(), key[0:2], key[2:4], key+"."+ext)
}

// ---------- 内容寻址键 ----------

// deliveryKey 由「源图 + 输出格式 + 合规配置」决定，同输入必然同键。
//
// 刻意**不**把提示词算进来：自动交付只写法定七字段，提示词属于可选附加信息
// （用户主动导出的那条路径才有）。否则同一张图换个说法就要多存一份，
// 而文件内容几乎一样，「同图不重复存」就形同虚设。
func deliveryKey(sourceHash, srcMime, format string, cfg aiops.AigcConfig) string {
	h := sha256.New()
	io.WriteString(h, "nvs-aigc-delivery-v1\n")
	io.WriteString(h, sourceHash+"\n")
	io.WriteString(h, srcMime+"\n")
	io.WriteString(h, format+"\n")
	io.WriteString(h, fmt.Sprintf("%v|%s|%s|%.4f|%s|%.3f|%.3f|%s|%v|%d\n",
		cfg.Enabled, cfg.LabelText, cfg.Position, cfg.FontSizePct, cfg.TextColor,
		cfg.TextOpacity, cfg.ShadowOpacity, cfg.ContentProducer,
		cfg.ReservedIncludeModel, cfg.MaxOutputPixels))
	return hex.EncodeToString(h.Sum(nil))
}

// rawDeliveryKey 未烧录时的降级副本键（与烧录版分属不同键，绝不互相覆盖）
func rawDeliveryKey(sourceHash, srcMime string) string {
	sum := sha256.Sum256([]byte("nvs-aigc-raw-v1\n" + sourceHash + "\n" + srcMime))
	return hex.EncodeToString(sum[:])
}

// ---------- 签名 ----------

func deliverySig(key string, exp int64, uid uint) string {
	mac := hmac.New(sha256.New, []byte(config.JWTSecret))
	fmt.Fprintf(mac, "aigc-deliver|%s|%d|%d", key, exp, uid)
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// signDeliveryURL 生成取图地址：文件名是内容哈希，凭据在 query 里，不依赖 Cookie。
//
// 为什么不用 Cookie 鉴权：<img src> 发不出 X-Device-Fingerprint 这类自定义头，
// 走 AuthRequired 会直接 401。签名 URL 无状态，有效期可控，还能按签发用户追溯。
func signDeliveryURL(key string, uid uint) string {
	exp := time.Now().Add(aigcDeliverTTL).Unix()
	return fmt.Sprintf("%s%s?e=%d&u=%d&s=%s", aigcFileRoute, key, exp, uid, deliverySig(key, exp, uid))
}

func validDeliverySig(key, expStr, uidStr, sig string) bool {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || exp <= 0 {
		return false
	}
	if time.Now().Unix() > exp {
		return false
	}
	uid64, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil {
		return false
	}
	want := deliverySig(key, exp, uint(uid64))
	return hmac.Equal([]byte(want), []byte(strings.ToLower(strings.TrimSpace(sig))))
}

var objectKeyRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ---------- 落盘 ----------

// writeBlobAtomic 先写临时文件再 rename：中途失败不会留下半截图片被当成成品
func writeBlobAtomic(dst string, data []byte) error {
	if st, err := os.Stat(dst); err == nil && st.Size() == int64(len(data)) {
		return nil // 已存在且大小一致（内容寻址，大小一致基本可信）
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// ---------- 交付主流程 ----------

// DeliverAIImages 出图成功后同步执行：原图落盘（去重）→ 烧录标识 → 交付副本（去重）。
//
// 返回顺序与入参一致；失败的槽位回填原始上游地址（至少前端还能显示）。
// **绝不返回 error**：这一步失败不该把已经扣过费的出图判成失败。
func DeliverAIImages(userID, taskID uint, modelName string, urls []string) []DeliveredImage {
	out := make([]DeliveredImage, len(urls))
	cfg := aiops.LoadAigcConfig()
	for i, raw := range urls {
		d, ok := deliverOne(userID, taskID, modelName, cfg, raw, i)
		if !ok {
			out[i] = DeliveredImage{URL: strings.TrimSpace(raw), Note: "未能生成站内副本"}
			continue
		}
		out[i] = d
	}
	return out
}

func deliverOne(userID, taskID uint, modelName string, cfg aiops.AigcConfig, raw string, index int) (DeliveredImage, bool) {
	data, fromURL, err := readAigcSource(raw)
	if err != nil || len(data) == 0 {
		return DeliveredImage{}, false
	}
	srcCfg, srcFormat, derr := image.DecodeConfig(bytes.NewReader(data))
	if derr != nil {
		return DeliveredImage{}, false
	}
	srcExt := extForImageFormat(srcFormat)
	srcMime := "image/" + srcExt
	sum := sha256.Sum256(data)
	srcHex := hex.EncodeToString(sum[:])

	// 1) 原图落盘（内容寻址，同一张图重复生成只存一份）+ 记 asset
	origPath := filepath.Join(aigcOriginalsDir(), srcHex[:16]+"."+srcExt)
	if werr := writeBlobAtomic(origPath, data); werr != nil {
		log.Printf("[aigc-deliver] 原图写盘失败 task=%d err=%v", taskID, werr)
		return DeliveredImage{}, false
	}
	asset := &aiops.AigcAsset{
		UserID: userID, TaskID: taskID, Index: index,
		Source: aiops.AigcSourceAI, ModelName: modelName,
		FilePath: origPath, Mime: srcMime,
		Width: srcCfg.Width, Height: srcCfg.Height,
		SizeBytes: int64(len(data)), SHA256: srcHex, FromURL: fromURL,
	}
	_ = aiops.CreateAigcAsset(asset)

	// 2) 站长关掉标识：不烧录，但依然走站内交付，
	//    这样「原图目录对外不可访问」这条防线不因为配置切换而破掉
	if !cfg.Enabled {
		key := rawDeliveryKey(srcHex, srcMime)
		if d, ok := reuseDelivery(userID, taskID, key); ok {
			return d, true
		}
		d, _, ok := storeDelivery(userID, taskID, key, "", srcHex, data, srcMime, srcExt,
			srcCfg.Width, srcCfg.Height, false, "")
		return d, ok
	}

	format := "jpeg"
	if strings.EqualFold(srcFormat, "png") {
		format = "png"
	}
	key := deliveryKey(srcHex, srcMime, format, cfg)
	if d, ok := reuseDelivery(userID, taskID, key); ok {
		return d, true
	}

	produceID := newProduceID()
	// 提示词不进文件：自动交付只写法定字段，保证「同图同文件、可跨用户去重」
	res, eerr := aigc.Export(aigc.ExportRequest{
		Source: data, Mime: srcMime, SourceIsAI: true,
		Format: format, ProduceID: produceID, ModelName: modelName,
	}, toPipelineConfig(cfg))
	if eerr != nil {
		// 烧录失败（字体坏了 / 图太大 / 元数据写不进去）：退一步交付原图，
		// 至少用户能看到图；note 里说清楚，别让人误以为「已经合规加标了」。
		log.Printf("[aigc-deliver] 烧录失败 task=%d index=%d err=%v（降级交付原图）", taskID, index, eerr)
		rkey := rawDeliveryKey(srcHex, srcMime)
		if d, ok := reuseDelivery(userID, taskID, rkey); ok {
			return d, true
		}
		d, _, ok := storeDelivery(userID, taskID, rkey, "", srcHex, data, srcMime, srcExt,
			srcCfg.Width, srcCfg.Height, false, "标识烧录失败，已交付原图："+aigcExportErrText(eerr))
		return d, ok
	}

	d, created, ok := storeDelivery(userID, taskID, key, produceID, srcHex, res.Data, res.Mime, res.Ext,
		res.Width, res.Height, res.Labeled, res.Note)
	if ok && created {
		// 内容编号 → 生成侧信息的映射（模型名不进法定字段，只能落业务库）
		fieldsJSON, _ := json.Marshal(res.Fields)
		_ = aiops.CreateAigcExport(&aiops.AigcExport{
			ProduceID: produceID, AssetID: asset.ID, UserID: userID, ModelName: modelName,
			Format: res.Ext, Width: res.Width, Height: res.Height,
			Labeled: res.Labeled, LabelText: cfg.LabelText, LabelFontSize: res.FontSize,
			ContentProducer: cfg.ContentProducer, ReservedCode1: res.Fields.ReservedCode1,
			PromptAttached: res.PromptAttached, AIGCJSON: string(fieldsJSON),
		})
	}
	return d, ok
}

// reuseDelivery 命中已有副本时只追加授权 + 计数
func reuseDelivery(userID, taskID uint, key string) (DeliveredImage, bool) {
	d, err := aiops.GetAigcDeliveryByKey(key)
	if err != nil || d == nil {
		return DeliveredImage{}, false
	}
	if st, serr := os.Stat(d.FilePath); serr != nil || st.Size() == 0 {
		return DeliveredImage{}, false // 记录还在但文件没了：当没命中，重新生成
	}
	aiops.EnsureAigcDeliveryGrant(key, userID, taskID)
	aiops.BumpAigcDeliveryHits(key)
	return DeliveredImage{
		URL: signDeliveryURL(key, userID), Key: key,
		Labeled: d.Labeled, Width: d.Width, Height: d.Height,
	}, true
}

// storeDelivery 写盘 + 落库 + 授权；created 表示这条记录是本次新建的
func storeDelivery(userID, taskID uint, key, produceID, srcHash string,
	data []byte, mime, ext string, w, h int, labeled bool, note string) (DeliveredImage, bool, bool) {

	path := deliveryPath(key, ext)
	if err := writeBlobAtomic(path, data); err != nil {
		log.Printf("[aigc-deliver] 写盘失败 key=%s err=%v", key, err)
		return DeliveredImage{}, false, false
	}
	// 并发下别处可能已插入同一个 key：先查一次，避免唯一索引报错
	if d, err := aiops.GetAigcDeliveryByKey(key); err == nil && d != nil {
		aiops.EnsureAigcDeliveryGrant(key, userID, taskID)
		aiops.BumpAigcDeliveryHits(key)
		return DeliveredImage{
			URL: signDeliveryURL(key, userID), Key: key,
			Labeled: d.Labeled, Width: d.Width, Height: d.Height, Note: note,
		}, false, true
	}
	rec := &aiops.AigcDelivery{
		ObjectKey: key, ProduceID: produceID, SourceHash: srcHash,
		FilePath: path, Mime: mime, Ext: ext,
		Width: w, Height: h, SizeBytes: int64(len(data)), Labeled: labeled,
	}
	if err := aiops.CreateAigcDelivery(rec); err != nil {
		log.Printf("[aigc-deliver] 落库失败 key=%s err=%v", key, err)
		aiops.EnsureAigcDeliveryGrant(key, userID, taskID)
		return DeliveredImage{URL: signDeliveryURL(key, userID), Key: key, Labeled: labeled, Width: w, Height: h, Note: note}, false, true
	}
	aiops.EnsureAigcDeliveryGrant(key, userID, taskID)
	return DeliveredImage{
		URL: signDeliveryURL(key, userID), Key: key,
		Labeled: labeled, Width: w, Height: h, Note: note,
	}, true, true
}

// ---------- 取图端点 ----------

// AiAigcFile GET /api/ai/aigc/file/:key —— 公开路由，仅凭签名准入
//
// 一律回 404（不回 403）：连「这个文件名是否存在」都不告诉未授权者。
func AiAigcFile(c *gin.Context) {
	key := strings.ToLower(strings.TrimSpace(c.Param("key")))
	// 允许带扩展名（<hash>.png），文件名主干仍是 64 位哈希
	if i := strings.IndexByte(key, '.'); i > 0 {
		key = key[:i]
	}
	if !objectKeyRe.MatchString(key) {
		c.Status(404)
		return
	}
	if !validDeliverySig(key, c.Query("e"), c.Query("u"), c.Query("s")) {
		c.Status(404)
		return
	}
	d, err := aiops.GetAigcDeliveryByKey(key)
	if err != nil || d == nil {
		c.Status(404)
		return
	}
	if st, serr := os.Stat(d.FilePath); serr != nil || st.IsDir() {
		c.Status(404)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("ETag", `"`+key+`"`)
	if d.Public {
		c.Header("Cache-Control", "public, max-age=2592000, immutable")
	} else {
		c.Header("Cache-Control", "private, max-age=2592000, immutable")
	}
	c.File(d.FilePath)
}

// AiAigcSign POST /api/ai/aigc/sign —— 用内容键换一份新签名（历史里的 URL 过期时用）
//
// 必须校验归属：否则「猜不到文件名」这条防线会退化成「猜到就能换一张票」。
func AiAigcSign(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		infra.Unauthorized(c, "请先登录")
		return
	}
	var req struct {
		Key  string   `json:"key"`
		Keys []string `json:"keys"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	keys := req.Keys
	if strings.TrimSpace(req.Key) != "" {
		keys = append(keys, req.Key)
	}
	out := make([]DeliveredImage, 0, len(keys))
	for _, raw := range keys {
		key := strings.ToLower(strings.TrimSpace(raw))
		if !objectKeyRe.MatchString(key) {
			continue
		}
		d, err := aiops.GetAigcDeliveryByKey(key)
		if err != nil || d == nil {
			continue
		}
		if !d.Public && !aiops.HasAigcDeliveryGrant(key, uid) && !isAdminID(uid) {
			continue
		}
		out = append(out, DeliveredImage{
			URL: signDeliveryURL(key, uid), Key: key,
			Labeled: d.Labeled, Width: d.Width, Height: d.Height,
		})
	}
	infra.Success(c, gin.H{"items": out})
}

// ---------- result JSON 重签 ----------

var deliveryURLRe = regexp.MustCompile(`/api/ai/aigc/file/[0-9a-f]{64}`)

// resignResult 把任务结果里的取图地址按当前用户重签。
//
// 为什么每次读都签：签名有有效期，而任务结果是永久保存的；库里存的是旧签名，
// 直接返回会在某天集体裂图。重签成本只有一次 HMAC。
func resignResult(result string, uid uint) string {
	if strings.TrimSpace(result) == "" || !strings.Contains(result, aigcFileRoute) {
		return result
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		return result
	}
	resignOne := func(s string) string {
		idx := strings.Index(s, aigcFileRoute)
		if idx < 0 {
			return s
		}
		rest := s[idx+len(aigcFileRoute):]
		if j := strings.IndexAny(rest, "?#"); j >= 0 {
			rest = rest[:j]
		}
		if !objectKeyRe.MatchString(rest) {
			return s
		}
		return signDeliveryURL(rest, uid)
	}
	if arr, ok := m["images"].([]interface{}); ok {
		for i, v := range arr {
			if s, ok := v.(string); ok {
				arr[i] = resignOne(s)
			}
		}
	}
	if arr, ok := m["items"].([]interface{}); ok {
		for _, v := range arr {
			item, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			if s, ok := item["url"].(string); ok {
				item["url"] = resignOne(s)
			}
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return result
	}
	return string(b)
}

// ---------- 公开状态同步 ----------

// resultLabeled 该任务的结果是否**全部**已烧录显式标识。
// 展示层据此决定要不要再叠一层 CSS 角标：图里已经烧了字，再叠一层就重复了。
func resultLabeled(result string) bool {
	if !strings.Contains(result, aigcFileRoute) {
		return false
	}
	var m struct {
		Items []struct {
			Labeled bool `json:"labeled"`
		} `json:"items"`
	}
	if json.Unmarshal([]byte(result), &m) != nil || len(m.Items) == 0 {
		return false
	}
	for _, it := range m.Items {
		if !it.Labeled {
			return false
		}
	}
	return true
}

// SyncDeliveryPublic 发布/取消发布作品时同步交付文件的 public 标记
func SyncDeliveryPublic(result string, public bool) {
	if !strings.Contains(result, aigcFileRoute) {
		return
	}
	for _, m := range deliveryURLRe.FindAllString(result, -1) {
		key := strings.TrimPrefix(m, aigcFileRoute)
		aiops.SetAigcDeliveryPublic(key, public)
	}
}
