package mediacover

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// 说明：这些测试依赖外部 ffmpeg 命令（不内联、不打包，发不发布由部署者决定）。
// 没有 ffmpeg 的机器上直接 skip，不能让 CI 因为环境差异变红。
func makeSampleVideo(t *testing.T, dir string) string {
	t.Helper()
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("机器上没有 ffmpeg，跳过抽帧测试")
	}
	out := filepath.Join(dir, "sample.mp4")
	cmd := exec.Command(bin, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=2:size=320x240:rate=10",
		"-pix_fmt", "yuv420p", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("无法生成测试视频（%v）：%s", err, string(b))
	}
	return out
}

func TestExtractFrameFromGeneratedVideo(t *testing.T) {
	dir := t.TempDir()
	src := makeSampleVideo(t, dir)

	frame, err := ExtractFrame(src, 10, 320)
	if err != nil {
		t.Fatalf("抽帧失败: %v", err)
	}
	if len(frame.JPEG) < 512 {
		t.Fatalf("产出的帧太小，可能不是有效 JPEG：%d bytes", len(frame.JPEG))
	}
	// JPEG 魔法字节 FFD8...FFD9
	if !bytes.HasPrefix(frame.JPEG, []byte{0xFF, 0xD8}) || !bytes.HasSuffix(frame.JPEG, []byte{0xFF, 0xD9}) {
		t.Fatal("产出的不是完整 JPEG")
	}
	if frame.Width != 320 {
		t.Fatalf("缩放宽应当是 320，实际 %d", frame.Width)
	}
	if frame.Height <= 0 {
		t.Fatalf("高度解析失败: %d", frame.Height)
	}
}

func TestDurationFromGeneratedVideo(t *testing.T) {
	dir := t.TempDir()
	src := makeSampleVideo(t, dir)
	if d := Duration(src); d < 1 || d > 3 {
		t.Fatalf("2 秒视频应解析出 1~3 秒，实际 %d", d)
	}
}

func TestExtractFrameRejectsMissingFile(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("机器上没有 ffmpeg")
	}
	if _, err := ExtractFrame(filepath.Join(t.TempDir(), "nope.mp4"), 10, 320); err == nil {
		t.Fatal("不存在的输入应当报错")
	}
}

func TestAvailableMirrorsLookPath(t *testing.T) {
	_, err := exec.LookPath("ffmpeg")
	want := err == nil
	if got := Available(); got != want && os.Getenv("NVS_FFMPEG_PATH") == "" && os.Getenv("FFMPEG_PATH") == "" {
		t.Fatalf("Available()=%v 与 PATH 探测结果 %v 不一致", got, want)
	}
}
