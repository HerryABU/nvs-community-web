package aimedia

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/config"
	"nvs-server/mediacover"
	"nvs-server/models"
	aiops "nvs-server/models/aiops"
)

func setupMediaDeliverTest(t *testing.T) {
	t.Helper()
	name := fmtTempName()
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrateAll(db); err != nil {
		t.Fatal(err)
	}
	models.DB = db
	config.UploadDir = filepath.Join(t.TempDir(), "uploads")
	cfg := aiops.LoadAigcConfig()
	cfg.Enabled = true
	cfg.LabelText = "AI生成"
	if err := aiops.SaveAigcConfig(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
}

func fmtTempName() string {
	return "file:aigc_media_test_" + time.Now().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
}

func TestDeliverAIMediaMarksAudio(t *testing.T) {
	if !mediacover.Available() {
		t.Skip("未安装 ffmpeg，跳过")
	}
	setupMediaDeliverTest(t)
	dir := filepath.Join(config.UploadDir, "2026", "11")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "src.wav")
	cmd := exec.Command(binFFmpeg(), "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1", src)
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("生成测试音频失败: %v %s", e, out)
	}

	url, labeled := DeliverAIMedia(7, 3, "/uploads/2026/11/src.wav", aigcMediaAudio)
	if !strings.HasPrefix(url, aigcFileRoute) {
		t.Fatalf("应返回签名交付地址，实际 %q", url)
	}
	if !labeled {
		t.Fatal("音频元数据标识应成功")
	}
	key := strings.Split(strings.TrimPrefix(url, aigcFileRoute), "?")[0]
	d, err := aiops.GetAigcDeliveryByKey(key)
	if err != nil || d == nil || !d.Labeled || d.Ext != "wav" {
		t.Fatalf("交付记录不正确: %#v %v", d, err)
	}
	out, _ := exec.Command(binFFmpeg(), "-hide_banner", "-i", d.FilePath).CombinedOutput()
	if !strings.Contains(string(out), "AI生成") {
		t.Fatalf("交付文件缺少 AIGC 元数据:\n%s", out)
	}

	// 视频：除元数据外，有中文字体时还要烧可见角标
	vsrc := filepath.Join(dir, "src.mp4")
	cmd = exec.Command(binFFmpeg(), "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=160x90:rate=10", "-t", "1", "-c:v", "libx264", vsrc)
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("生成测试视频失败: %v %s", e, out)
	}
	vurl, vlabeled := DeliverAIMedia(7, 4, "/uploads/2026/11/src.mp4", aigcMediaVideo)
	if !strings.HasPrefix(vurl, aigcFileRoute) || !vlabeled {
		t.Fatalf("视频交付失败: url=%q labeled=%v", vurl, vlabeled)
	}
	vkey := strings.Split(strings.TrimPrefix(vurl, aigcFileRoute), "?")[0]
	vd, err := aiops.GetAigcDeliveryByKey(vkey)
	if err != nil || vd == nil || vd.Ext != "mp4" || !vd.Labeled {
		t.Fatalf("视频交付记录不正确: %#v %v", vd, err)
	}
}

func binFFmpeg() string {
	// 测试里只找 PATH；自定义路径的部署由 mediacover 自己的测试覆盖
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "ffmpeg"
	}
	return p
}
