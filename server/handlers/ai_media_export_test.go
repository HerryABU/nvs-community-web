package handlers

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"nvs-server/config"
	"nvs-server/models"
)

// TestAssertPublicHost SSRF 第一道：私网/回环/元数据地址必须拒绝
func TestAssertPublicHost(t *testing.T) {
	bad := []string{"127.0.0.1", "localhost", "10.1.2.3", "192.168.1.1", "172.16.0.1",
		"169.254.169.254", "::1"}
	for _, h := range bad {
		if err := assertPublicHost(h); err == nil {
			t.Fatalf("%s 应被拒绝", h)
		}
	}
	if err := assertPublicHost(""); err == nil {
		t.Fatal("空主机应拒绝")
	}
}

// TestFetchMediaBytesTraversal 站内路径目录穿越必须拒绝（否则导出=任意文件读取）
func TestFetchMediaBytesTraversal(t *testing.T) {
	dir := t.TempDir()
	old := config.UploadDir
	config.UploadDir = dir
	defer func() { config.UploadDir = old }()

	// 正常文件
	rel := filepath.Join("2026", "10")
	if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, rel, "a.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, data, reason := fetchMediaBytes("/uploads/2026/10/a.png")
	if f == nil || string(data) != "png" || reason != "" {
		t.Fatalf("正常站内文件应可读: f=%v reason=%s", f, reason)
	}

	// 穿越
	for _, bad := range []string{"/uploads/../../etc/passwd", "/uploads/../../../windows/win.ini"} {
		if f, _, reason := fetchMediaBytes(bad); f != nil || reason == "" {
			t.Fatalf("穿越路径应被拒绝: %s (reason=%s)", bad, reason)
		}
	}

	// 不支持的协议
	for _, bad := range []string{"data:image/png;base64,xx", "blob:abc", "file:///etc/passwd"} {
		if f, _, reason := fetchMediaBytes(bad); f != nil || reason == "" {
			t.Fatalf("应拒绝 %s", bad)
		}
	}
}

// TestResultURLs 兼容多种 result JSON 形态
func TestResultURLs(t *testing.T) {
	cases := map[string]int{
		`{"images":["a","b"]}`:                      2,
		`{"videos":["v1"]}`:                         1,
		`{"url":"x"}`:                               1,
		`{"images":["a"],"videos":["b"],"url":"c"}`: 3,
		`not json`:                                  0,
		``:                                          0,
	}
	for raw, want := range cases {
		if got := len(resultURLs(raw)); got != want {
			t.Fatalf("%s -> %d, want %d", raw, got, want)
		}
	}
}

// TestZipMediaTasksContainsManifest 打包必须含 items.json 与 manifest.json，且 manifest 有哈希
func TestZipMediaTasksContainsManifest(t *testing.T) {
	dir := t.TempDir()
	old := config.UploadDir
	config.UploadDir = dir
	defer func() { config.UploadDir = old }()
	if err := os.MkdirAll(filepath.Join(dir, "2026", "10"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026", "10", "x.png"), []byte("hello-img"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []models.AiMediaTask{{
		ID: 7, Kind: models.AiKindImage, ModelName: "m", Prompt: "p",
		Status: models.AiMediaSucceeded, Cost: 3,
		Result:      `{"images":["/uploads/2026/10/x.png","/uploads/2026/10/missing.png"]}`,
		Attachments: `[{"url":"/uploads/2026/10/ref.png"}]`,
	}}
	buf, err := zipMediaTasks(list)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["items.json"] || !names["manifest.json"] {
		t.Fatalf("缺少必要条目: %v", names)
	}
	if !names["files/task-7-1.png"] {
		t.Fatalf("应包含已抓到的图片: %v", names)
	}
	// manifest 里应有 sha256 与被跳过原因
	for _, f := range zr.File {
		if f.Name != "manifest.json" {
			continue
		}
		rc, _ := f.Open()
		b := new(bytes.Buffer)
		_, _ = b.ReadFrom(rc)
		_ = rc.Close()
		if !bytes.Contains(b.Bytes(), []byte("sha256")) {
			t.Fatal("manifest 应含 sha256")
		}
		if !bytes.Contains(b.Bytes(), []byte("missing.png")) {
			t.Fatal("缺失文件应记入 manifest 的跳过原因")
		}
	}
}
