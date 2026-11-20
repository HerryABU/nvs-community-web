package feed

// 视频封面自动生成（抽帧）。
//
// 关键约束：ffmpeg 只做「外部命令调用」，不内联、不打包、不静态链接（LGPL/GPL 与本项目 AGPL 的
// 分发义务冲突）。仓库里没有也不允许放 ffmpeg 二进制；机器上没装就自动降级，
// 由前端 canvas 抽帧或占位图兜底，绝不影响发布。

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nvs-server/config"
	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/hctx"
	"nvs-server/mediacover"
	"nvs-server/models/feed"
	"nvs-server/utils/infra"

	"github.com/gin-gonic/gin"
)

var coverBusy sync.Map // postID -> struct{}：同一条内容别并发抽帧

// localMediaPath 把站内 /uploads/... 地址换成磁盘路径；外链一律不处理（避免 SSRF 与「帮别人抓图」）
func localMediaPath(raw string) (string, bool) {
	u := strings.TrimSpace(raw)
	if u == "" || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return "", false
	}
	i := strings.IndexAny(u, "?#")
	if i >= 0 {
		u = u[:i]
	}
	if !strings.HasPrefix(u, "/uploads/") {
		return "", false
	}
	rel := strings.TrimPrefix(u, "/uploads/")
	rel = filepath.FromSlash(rel)
	if strings.Contains(rel, "..") {
		return "", false
	}
	return filepath.Join(config.UploadDir, rel), true
}

// coverDir 封面与其它上传物同域：data/uploads/yyyy/mm/<16hex>.jpg
func coverDir() (string, string) {
	rel := time.Now().Format("2006/01")
	return filepath.Join(config.UploadDir, rel), rel
}

// AutoCoverAsync 发布后异步抽帧当封面；失败只记日志。
//
// 只补空封面：作者自己选了封面就不动，避免「刚设好又被覆盖」。
func AutoCoverAsync(postID uint, videoURL string) {
	if postID == 0 || videoURL == "" {
		return
	}
	if _, loaded := coverBusy.LoadOrStore(postID, struct{}{}); loaded {
		return
	}
	go func() {
		defer coverBusy.Delete(postID)
		if _, err := buildCover(postID, videoURL); err != nil {
			if err == mediacover.ErrUnavailable {
				return // 没装 ffmpeg 是常态，静默降级
			}
			log.Printf("[feed-cover] post=%d 抽帧失败: %v", postID, err)
		}
	}()
}

func buildCover(postID uint, videoURL string) (string, error) {
	src, ok := localMediaPath(videoURL)
	if !ok {
		return "", nil // 外链视频不处理
	}
	if st, err := os.Stat(src); err != nil || st.IsDir() {
		return "", nil
	}
	frame, err := mediacover.ExtractFrame(src, 10, 720)
	if err != nil {
		return "", err
	}
	dir, relDir := coverDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	rb := make([]byte, 8)
	if _, err := rand.Read(rb); err != nil {
		return "", err
	}
	name := hex.EncodeToString(rb) + ".jpg"
	abs := filepath.Join(dir, name)
	// 先写临时文件再改名，避免列表刚好读到半截图
	tmp := abs + ".tmp"
	if err := os.WriteFile(tmp, frame.JPEG, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, abs); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	url := "/uploads/" + relDir + "/" + name
	if err := feed.SetCover(postID, url); err != nil {
		return "", err
	}
	if sec := mediacover.Duration(src); sec > 0 {
		_ = feed.SetDuration(postID, sec)
	}
	return url, nil
}

// RegenerateCover POST /api/feed/:id/cover —— 手动重新抽帧（历史数据补齐、或原封面不满意）
func RegenerateCover(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	id := parseUint(c.Param("id"))
	if id == 0 {
		infra.BadRequest(c, "参数错误")
		return
	}
	p, err := feed.GetPostByID(id)
	if err != nil {
		infra.NotFound(c, "内容不存在")
		return
	}
	if p.UserID != uid && !admincommon.IsSuperAdmin(c) {
		infra.Forbidden(c, "无权操作")
		return
	}
	if p.Kind != feed.KindVideo || p.VideoURL == "" {
		infra.BadRequest(c, "只有视频才需要抽帧封面")
		return
	}
	// 手动重生要能覆盖旧封面：先把 cover 清空再走同一套逻辑
	_ = feed.ForceSetCover(p.ID, "")
	url, err := buildCover(p.ID, p.VideoURL)
	if err != nil {
		if err == mediacover.ErrUnavailable {
			infra.BadRequest(c, "服务端未安装 ffmpeg，无法抽帧（可用 NVS_FFMPEG_PATH 指定，或在前端上传封面）")
			return
		}
		infra.BadRequest(c, "抽帧失败："+err.Error())
		return
	}
	infra.Success(c, gin.H{"cover_url": url, "duration": p.Duration})
}
