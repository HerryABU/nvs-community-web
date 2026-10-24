package handlers

// ==================== AI 出图：附件上传 + 历史导出（gap-audit #17）====================
// 两件事：
//   1. **附件上传**：参考图（混图/换脸）以前只以 data URL 传给上游、**不落库**，
//      于是历史里看不到「当时用了什么图」，也无法复用。现在附件单独存 URL 列表。
//   2. **历史导出**：把一段时间的出图/视频任务打成 zip（items.json + files/ + manifest 哈希）。
//
// 安全要点（导出最容易被忽略的地方）：
//   - 站内 /uploads 的图直接读盘（路径必须落在 UploadDir 内，防目录穿越）；
//   - 远程 http(s) 图走 **SSRF 白名单抓取**：解析 IP 后拒绝私网/回环/链路本地/组播，
//     禁止重定向到私网，限 10MB / 15 秒；
//   - 抓不到就跳过（manifest 记录 skip 原因），不让单张坏图毁掉整包导出。

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

const (
	maxAttachBytes   = 10 << 20  // 单附件 10MB
	maxExportFiles   = 120       // 单次导出最多下载多少张图
	maxExportBytes   = 200 << 20 // 整包上限（防一次导出打爆磁盘/内存）
	maxRemoteImgSize = 10 << 20
)

// AttachMedia POST /api/ai/media/attach —— 上传参考图/素材（multipart: file）
// 返回 { url, name, size }；前端再把它作为 attach_urls 提交给出图接口。
func AttachMedia(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "请选择文件")
		return
	}
	if fh.Size > maxAttachBytes {
		utils.BadRequest(c, "附件不能超过 10MB")
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowed := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true}
	if !allowed[ext] {
		utils.BadRequest(c, "只支持 png/jpg/jpeg/webp/gif")
		return
	}
	f, err := fh.Open()
	if err != nil {
		utils.InternalError(c, "读取文件失败")
		return
	}
	defer f.Close()

	relDir := filepath.Join("ai-refs", time.Now().Format("2006/01"))
	absDir := filepath.Join(config.UploadDir, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		utils.InternalError(c, "创建目录失败")
		return
	}
	name := randHexName() + ext
	dst, err := os.Create(filepath.Join(absDir, name))
	if err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	defer dst.Close()
	written, err := io.Copy(dst, io.LimitReader(f, maxAttachBytes+1))
	if err != nil || written > maxAttachBytes {
		_ = os.Remove(filepath.Join(absDir, name))
		utils.BadRequest(c, "文件过大或写入失败")
		return
	}
	utils.Success(c, gin.H{
		"url":  "/uploads/" + filepath.ToSlash(relDir) + "/" + name,
		"name": fh.Filename,
		"size": written,
	})
}

// ExportMediaTask GET /api/ai/media/:id/export?files=1
// 单任务导出：默认 JSON；files=1 时打 zip（含图片文件）
func ExportMediaTask(c *gin.Context) {
	uid := currentUserID(c)
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if uid == 0 || id == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	task, err := models.GetAiMediaTask(uint(id), uid)
	if err != nil || task == nil {
		utils.NotFound(c, "任务不存在")
		return
	}
	if c.Query("files") != "1" {
		c.Header("Content-Disposition",
			fmt.Sprintf(`attachment; filename="ai-media-%d.json"`, task.ID))
		c.JSON(http.StatusOK, buildExportItem(*task, nil))
		return
	}
	buf, err := zipMediaTasks([]models.AiMediaTask{*task})
	if err != nil {
		utils.InternalError(c, "打包失败")
		return
	}
	writeZip(c, buf, fmt.Sprintf("ai-media-%d.zip", task.ID))
}

// ExportMediaHistory GET /api/ai/media/export.zip?kind=image&month=YYYY-MM&limit=200
// 历史导出：items.json（全部任务）+ files/（图片）+ manifest.json（每文件 SHA256）
func ExportMediaHistory(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	kind := strings.TrimSpace(c.Query("kind"))
	if kind != "" && kind != models.AiKindImage && kind != models.AiKindVideo {
		utils.BadRequest(c, "kind 需为 image 或 video")
		return
	}
	var since, until time.Time
	if m := strings.TrimSpace(c.Query("month")); m != "" {
		t, perr := time.ParseInLocation("2006-01", m, time.Local)
		if perr != nil {
			utils.BadRequest(c, "month 格式应为 YYYY-MM")
			return
		}
		since = t
		until = t.AddDate(0, 1, 0)
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	list, err := models.ListAiMediaForExport(uid, kind, since, until, limit)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	if len(list) == 0 {
		utils.BadRequest(c, "该范围内没有可导出的任务")
		return
	}
	buf, err := zipMediaTasks(list)
	if err != nil {
		utils.InternalError(c, "打包失败")
		return
	}
	name := "ai-media-history"
	if kind != "" {
		name += "-" + kind
	}
	if m := strings.TrimSpace(c.Query("month")); m != "" {
		name += "-" + m
	}
	writeZip(c, buf, name+".zip")
}

// ─────────────── 打包 ───────────────

type exportItem struct {
	ID          uint                     `json:"id"`
	Kind        string                   `json:"kind"`
	Model       string                   `json:"model"`
	Prompt      string                   `json:"prompt"`
	Params      string                   `json:"params"`
	Status      string                   `json:"status"`
	Result      string                   `json:"result"`
	Cost        int64                    `json:"cost"`
	Error       string                   `json:"error,omitempty"`
	CreatedAt   string                   `json:"created_at"`
	Attachments []models.MediaAttachment `json:"attachments,omitempty"`
	Files       []string                 `json:"files,omitempty"`
}

func buildExportItem(t models.AiMediaTask, files []string) exportItem {
	return exportItem{
		ID: t.ID, Kind: t.Kind, Model: t.ModelName, Prompt: t.Prompt,
		Params: t.Params, Status: t.Status, Result: t.Result, Cost: t.Cost,
		Error: t.Error, CreatedAt: t.CreatedAt.Format(time.RFC3339),
		Attachments: models.ParseAttachments(t.Attachments), Files: files,
	}
}

// zipMediaTasks 组包：items.json + files/ + manifest.json（含每个文件的 SHA256）
func zipMediaTasks(list []models.AiMediaTask) ([]byte, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)

	manifest := make([]map[string]interface{}, 0, len(list)*2)
	totalBytes := 0
	fileCount := 0

	items := make([]exportItem, 0, len(list))
	for _, t := range list {
		urls := resultURLs(t.Result)
		files := make([]string, 0, len(urls))
		for i, u := range urls {
			keep, data, reason := fetchMediaBytes(u)
			if keep != nil {
				if totalBytes+len(data) > maxExportBytes || fileCount >= maxExportFiles {
					manifest = append(manifest, map[string]interface{}{
						"task_id": t.ID, "url": u, "skipped": "超出单次导出上限（文件数或体积）",
					})
					continue
				}
				name := fmt.Sprintf("files/task-%d-%d%s", t.ID, i+1, keep.ext)
				if err := addZipFile(zw, name, data, &manifest); err == nil {
					files = append(files, name)
					totalBytes += len(data)
					fileCount++
				}
				continue
			}
			manifest = append(manifest, map[string]interface{}{
				"task_id": t.ID, "url": u, "skipped": reason,
			})
		}
		items = append(items, buildExportItem(t, files))
	}

	// items.json
	itemsJSON, _ := json.MarshalIndent(map[string]interface{}{
		"exported_at": time.Now().Format(time.RFC3339),
		"count":       len(items),
		"items":       items,
	}, "", "  ")
	if w, err := zw.Create("items.json"); err == nil {
		_, _ = w.Write(itemsJSON)
		sum := sha256.Sum256(itemsJSON)
		manifest = append(manifest, map[string]interface{}{
			"file": "items.json", "bytes": len(itemsJSON), "sha256": hex.EncodeToString(sum[:]),
		})
	}
	mj, _ := json.MarshalIndent(map[string]interface{}{
		"note":  "本清单记录包内每个文件的 SHA256；被跳过的图片附原因（站内路径非法/远程拒绝/超限）",
		"files": manifest,
	}, "", "  ")
	if w, err := zw.Create("manifest.json"); err == nil {
		_, _ = w.Write(mj)
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func addZipFile(zw *zip.Writer, name string, data []byte, manifest *[]map[string]interface{}) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	*manifest = append(*manifest, map[string]interface{}{
		"file": name, "bytes": len(data), "sha256": hex.EncodeToString(sum[:]),
	})
	return nil
}

// resultURLs 从任务 result JSON 里取图片/视频地址（兼容 {images:[...]} / {videos:[...]} / {url:...}）
func resultURLs(result string) []string {
	if strings.TrimSpace(result) == "" {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		return nil
	}
	out := []string{}
	for _, k := range []string{"images", "videos", "urls"} {
		if raw, ok := m[k]; ok {
			var arr []string
			if json.Unmarshal(raw, &arr) == nil {
				out = append(out, arr...)
			}
		}
	}
	if raw, ok := m["url"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// fetchedFile 抓取结果
type fetchedFile struct {
	ext string
}

// fetchMediaBytes 取一张图的字节：
//   - 站内 /uploads/…：读盘（必须落在 UploadDir 内）
//   - http(s)：SSRF 白名单抓取
//   - 其它（data:/blob:/空）：跳过
func fetchMediaBytes(raw string) (*fetchedFile, []byte, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil, "空地址"
	}
	if strings.HasPrefix(raw, "/uploads/") {
		rel := strings.TrimPrefix(path.Clean(raw), "/uploads/")
		root := filepath.Join(config.UploadDir)
		abs := filepath.Join(root, filepath.FromSlash(rel))
		// 防目录穿越：解析后必须仍在 root 内
		if relAbs, err := filepath.Abs(abs); err != nil ||
			!strings.HasPrefix(relAbs, mustAbs(root)+string(os.PathSeparator)) {
			return nil, nil, "站内路径非法（越界）"
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, nil, "站内文件不存在"
		}
		return &fetchedFile{ext: safeExt(abs)}, data, ""
	}
	low := strings.ToLower(raw)
	if strings.HasPrefix(low, "data:") || strings.HasPrefix(low, "blob:") {
		return nil, nil, "内联数据不导出（体积过大）"
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, nil, "不支持的协议"
	}
	if err := assertPublicHost(u.Hostname()); err != nil {
		return nil, nil, "拒绝访问内网地址"
	}
	data, err := httpGetLimited(raw)
	if err != nil {
		return nil, nil, "远程抓取失败：" + truncateErr(err)
	}
	return &fetchedFile{ext: safeExt(u.Path)}, data, ""
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

func safeExt(p string) string {
	e := strings.ToLower(filepath.Ext(p))
	switch e {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".mp4", ".webm":
		return e
	}
	return ".bin"
}

func truncateErr(err error) string {
	s := err.Error()
	if len(s) > 80 {
		return s[:80]
	}
	return s
}

// assertPublicHost 拒绝私网/回环/链路本地/组播等地址（SSRF 第一道）
func assertPublicHost(host string) error {
	if host == "" {
		return fmt.Errorf("空主机")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("私网地址")
		}
		// 云厂商元数据段（169.254.0.0/16 已被 IsLinkLocalUnicast 覆盖，这里补 ::ffff: 映射）
		if v4 := ip.To4(); v4 != nil && v4[0] == 169 && v4[1] == 254 {
			return fmt.Errorf("元数据地址")
		}
	}
	return nil
}

// httpGetLimited 抓取（禁跳转到私网、限 10MB、15 秒）
func httpGetLimited(rawURL string) ([]byte, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("重定向过多")
			}
			return assertPublicHost(req.URL.Hostname())
		},
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "nvs-export/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteImgSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxRemoteImgSize {
		return nil, fmt.Errorf("超过 10MB")
	}
	return data, nil
}

func writeZip(c *gin.Context, data []byte, filename string) {
	sum := sha256.Sum256(data)
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Content-Type", "application/zip")
	c.Header("X-Bundle-SHA256", hex.EncodeToString(sum[:]))
	c.Data(http.StatusOK, "application/zip", data)
}

// randHexName 随机文件名（8 字节 hex）
func randHexName() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
