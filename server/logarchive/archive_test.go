package logarchive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// dateNameRe 日归档命名：<date>.7z
var dateNameRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.7z$`)

func setup(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"requests/2026-11-01", "requests/2026-11-02", "database/2026-11-01"} {
		dir := filepath.Join(root, "log", d)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "x.log"), []byte("hello "+d), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRun_MonthlyTarGz(t *testing.T) {
	root := setup(t)
	out, err := Run(Options{LogDir: filepath.Join(root, "log"), ArchiveDir: filepath.Join(root, "archive")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// 周/月归档（tar.gz）必须生成（不依赖 7z）
	var monthly, weekly string
	for _, o := range out {
		base := filepath.Base(o)
		if strings.Contains(o, "weekly") && strings.HasSuffix(base, ".tar.gz") {
			weekly = o
		}
		if strings.Contains(o, "monthly") && strings.HasSuffix(base, ".tar.gz") {
			monthly = o
		}
	}
	if weekly == "" {
		t.Fatalf("未生成周归档, outputs=%v", out)
	}
	if monthly == "" {
		t.Fatalf("未生成月归档, outputs=%v", out)
	}
	// 校验包内按日期目录组织
	f, err := os.Open(monthly)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}
	joined := strings.Join(names, ",")
	// 新层级：月 tar.gz 内是「周 tar.gz」（文件名形如 2026W45.tar.gz，按周/日期分类）
	if !strings.Contains(joined, "2026W") {
		t.Fatalf("月归档内未按日期/周分类: %v", names)
	}
	// 日归档：同一天的所有类型打在一个 7z 里（daily/<date>.7z）
	var daily string
	for _, o := range out {
		if strings.Contains(o, "daily") && strings.HasSuffix(filepath.Base(o), ".7z") {
			daily = o
		}
	}
	if daily == "" {
		t.Fatalf("未生成日 7z 归档, outputs=%v", out)
	}
	if !dateNameRe.MatchString(filepath.Base(daily)) {
		t.Fatalf("日归档命名应为 <date>.7z，实际 %s", filepath.Base(daily))
	}
}

func TestRun_Idempotent(t *testing.T) {
	root := setup(t)
	opt := Options{LogDir: filepath.Join(root, "log"), ArchiveDir: filepath.Join(root, "archive")}
	if _, err := Run(opt); err != nil {
		t.Fatal(err)
	}
	out1, err := Run(opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(out1) != 0 {
		t.Fatalf("幂等失败：第二次仍产出 %v", out1)
	}
}
