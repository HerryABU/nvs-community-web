package aimedia

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"nvs-server/aigc"
	"nvs-server/config"
	"nvs-server/models"
	aiops "nvs-server/models/aiops"
)

// setupDeliverDB 独立内存库（不与其它测试共享，避免串数据）
func setupDeliverDB(t *testing.T) {
	t.Helper()
	prev := models.DB
	dsn := fmt.Sprintf("file:aigc_deliver_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&aiops.AigcAsset{}, &aiops.AigcExport{}, &aiops.AigcDelivery{},
		&aiops.AigcDeliveryGrant{}, &models.PlatformConfig{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	models.DB = db
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
		models.DB = prev
	})
}

// setupDeliverDirs 把 uploads/aigc 都指到临时目录，测试不碰真实数据目录
func setupDeliverDirs(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	prevUpload := config.UploadDir
	prevSecret := config.JWTSecret
	config.UploadDir = filepath.Join(root, "uploads")
	config.JWTSecret = "test-secret-for-aigc-delivery"
	if err := os.MkdirAll(config.UploadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		config.UploadDir = prevUpload
		config.JWTSecret = prevSecret
	})
}

// makeTestPNG 造一张纯色 PNG（内容确定，便于断言「同图同键」）
func makeTestPNG(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// 内容寻址键：同输入必须同键（否则「同图不重复存」直接失效），
// 参数或合规配置变化必须换键（否则旧图会被当成新图复用，角标文案改了却不生效）。
func TestDeliveryKeyDedup(t *testing.T) {
	setupDeliverDB(t) // DefaultAigcConfig 要读站点名（走 platform_configs）
	cfg := aiops.DefaultAigcConfig()
	cfg.ContentProducer = "星海文学"

	k1 := deliveryKey("abc123", "image/png", "png", cfg)
	if k2 := deliveryKey("abc123", "image/png", "png", cfg); k2 != k1 {
		t.Fatal("同输入必须同键，否则「同图不重复存」直接失效")
	}
	if k3 := deliveryKey("abc123", "image/png", "jpeg", cfg); k3 == k1 {
		t.Fatal("输出格式不同必须换键（否则扩展名与内容会错配）")
	}
	if len(k1) != 64 {
		t.Fatalf("键应为 64 位 hex，得到 %d 位", len(k1))
	}

	cfg2 := cfg
	cfg2.LabelText = "AI合成"
	if deliveryKey("abc123", "image/png", "png", cfg2) == k1 {
		t.Fatal("角标文案变了必须换键")
	}
	cfg3 := cfg
	cfg3.Enabled = false
	if deliveryKey("abc123", "image/png", "png", cfg3) == k1 {
		t.Fatal("总开关变了必须换键")
	}
	// 提示词不进键：自动交付只写法定字段，同图换词必须复用同一份文件
	cfg4 := cfg
	cfg4.PromptAttachEnabled = true
	if deliveryKey("abc123", "image/png", "png", cfg4) != k1 {
		t.Fatal("附加提示词开关不应改变自动交付的内容键（它只影响手动导出）")
	}
}

// 签名：篡改任何一个字段都必须失效，过期也必须失效
func TestDeliverySigValid(t *testing.T) {
	setupDeliverDirs(t)
	key := strings.Repeat("a", 64)
	url := signDeliveryURL(key, 7)
	if !strings.HasPrefix(url, aigcFileRoute+key+"?") {
		t.Fatalf("URL 形态不对：%s", url)
	}
	exp := fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix())
	sig := deliverySig(key, time.Now().Add(time.Hour).Unix(), 7)
	if !validDeliverySig(key, exp, "7", sig) {
		t.Fatal("正常签名应通过")
	}
	if validDeliverySig(key, exp, "8", sig) {
		t.Fatal("换 uid 应失效")
	}
	if validDeliverySig(strings.Repeat("b", 64), exp, "7", sig) {
		t.Fatal("换 key 应失效")
	}
	if validDeliverySig(key, exp, "7", "deadbeef") {
		t.Fatal("伪造签名应失效")
	}
	past := time.Now().Add(-time.Minute)
	if validDeliverySig(key, fmt.Sprintf("%d", past.Unix()), "7", deliverySig(key, past.Unix(), 7)) {
		t.Fatal("过期签名应失效")
	}
}

// 历史结果重签：文件名不变，只换凭据；非交付地址原样保留
func TestResignResult(t *testing.T) {
	setupDeliverDirs(t)
	key := strings.Repeat("c", 64)
	old := `{"images":["` + aigcFileRoute + key + `?e=1&u=0&s=old","https://up.example/x.png"],"items":[{"url":"` + aigcFileRoute + key + `?e=1&u=0&s=old","labeled":true}]}`
	got := resignResult(old, 42)
	if strings.Contains(got, "s=old") {
		t.Fatalf("旧签名应被替换：%s", got)
	}
	if !strings.Contains(got, aigcFileRoute+key+"?") {
		t.Fatalf("文件名（内容键）必须保持不变：%s", got)
	}
	if !strings.Contains(got, "https://up.example/x.png") {
		t.Fatal("非交付地址不应被改写")
	}
	if !strings.Contains(got, `"labeled":true`) {
		t.Fatal("其余字段应原样保留")
	}
	// 无交付地址的结果必须原样返回（不要多此一举重排 JSON）
	plain := `{"video":"http://v/x.mp4"}`
	if resignResult(plain, 1) != plain {
		t.Fatal("不含交付地址时应原样返回")
	}
}

// 展示层判断：全部烧录过才不再叠角标
func TestResultLabeled(t *testing.T) {
	key := strings.Repeat("d", 64)
	all := `{"images":["` + aigcFileRoute + key + `"],"items":[{"labeled":true},{"labeled":true}]}`
	if !resultLabeled(all) {
		t.Fatal("全部 labeled 应返回 true")
	}
	mixed := `{"items":[{"labeled":true},{"labeled":false}]}`
	if resultLabeled(mixed) {
		t.Fatal("部分未烧录不应算已标识")
	}
	// 老数据（改造前只有上游地址）不能误判为已标识
	if resultLabeled(`{"images":["https://up.example/x.png"]}`) {
		t.Fatal("无交付信息应返回 false")
	}
}

// 端到端：出图 → 落盘 → 烧录 → 签名 URL → 同图去重
func TestDeliverAIImagesRoundTrip(t *testing.T) {
	setupDeliverDB(t)
	setupDeliverDirs(t)

	// 站内原图（模拟前端上传/上游落盘后的地址）
	src := makeTestPNG(t, 64, 48, color.RGBA{10, 120, 200, 255})
	rel := filepath.Join("2026", "11", "src.png")
	full := filepath.Join(config.UploadDir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, src, 0o644); err != nil {
		t.Fatal(err)
	}

	got := DeliverAIImages(11, 5, "Mock-Image", []string{"/uploads/2026/11/src.png"})
	if len(got) != 1 {
		t.Fatalf("应返回 1 条，得到 %d", len(got))
	}
	d := got[0]
	if !objectKeyRe.MatchString(d.Key) {
		t.Fatalf("内容键应为 64 位 hex：%q", d.Key)
	}
	if !strings.HasPrefix(d.URL, aigcFileRoute+d.Key) {
		t.Fatalf("URL 形态不对：%s", d.URL)
	}
	if !d.Labeled {
		t.Fatalf("应烧录显式标识：%+v", d)
	}

	rec, err := aiops.GetAigcDeliveryByKey(d.Key)
	if err != nil || rec == nil {
		t.Fatalf("交付记录未落库：%v", err)
	}
	data, rerr := os.ReadFile(rec.FilePath)
	if rerr != nil {
		t.Fatalf("交付文件不在磁盘上：%v", rerr)
	}
	if !bytes.Equal(data, src) {
		if f := aigc.ExtractXMP(data, rec.Mime); !aigc.HasAIGC(f) {
			t.Fatal("交付文件应带 AIGC 隐式标识")
		}
	} else {
		t.Fatal("交付文件不应与原图逐字节相同（说明角标/元数据没写进去）")
	}

	// 同一张图再来一次（另一个用户）：同一内容键、磁盘只多一条授权记录，不新增文件
	before := countFiles(t, aigcDeliverDir())
	again := DeliverAIImages(22, 6, "Mock-Image", []string{"/uploads/2026/11/src.png"})
	if len(again) != 1 || again[0].Key != d.Key {
		t.Fatalf("同图应复用同一内容键：%q vs %q", again[0].Key, d.Key)
	}
	if after := countFiles(t, aigcDeliverDir()); after != before {
		t.Fatalf("同图不应重复存储：文件数 %d → %d", before, after)
	}
	if !aiops.HasAigcDeliveryGrant(d.Key, 22) {
		t.Fatal("第二个用户应获得授权记录")
	}
	rec2, _ := aiops.GetAigcDeliveryByKey(d.Key)
	if rec2.Hits < 1 {
		t.Fatalf("命中次数应累加：%d", rec2.Hits)
	}
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}
