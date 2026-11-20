package mediacover

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkVideoAndAudio(t *testing.T) {
	bin, err := ffmpegPath()
	if err != nil {
		t.Skip("未安装 ffmpeg，跳过")
	}
	dir := t.TempDir()
	srcVideo := filepath.Join(dir, "src.mp4")
	outVideo := filepath.Join(dir, "marked.mp4")
	srcAudio := filepath.Join(dir, "src.wav")
	outAudio := filepath.Join(dir, "marked.wav")

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("ffmpeg 失败 %v: %s", e, out)
		}
	}
	run("-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc=size=320x180:rate=25", "-t", "2", "-c:v", "libx264", srcVideo)
	run("-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", srcAudio)

	labeled, err := MarkVideo(srcVideo, outVideo, "AI生成")
	if err != nil {
		t.Fatalf("MarkVideo: %v", err)
	}
	if FindFont() != "" && !labeled {
		t.Fatal("有中文字体时应烧录可见角标")
	}
	if st, err := os.Stat(outVideo); err != nil || st.Size() == 0 {
		t.Fatalf("标记后的视频为空: %v", err)
	}

	meta := exec.Command(bin, "-hide_banner", "-i", outVideo)
	out, _ := meta.CombinedOutput()
	if !strings.Contains(string(out), "AI生成") {
		t.Fatalf("视频 metadata 没有 AIGC 标识:\n%s", out)
	}

	if _, err := MarkAudio(srcAudio, outAudio, "AI生成"); err != nil {
		t.Fatalf("MarkAudio: %v", err)
	}
	out, _ = exec.Command(bin, "-hide_banner", "-i", outAudio).CombinedOutput()
	if !strings.Contains(string(out), "AI生成") {
		t.Fatalf("音频 metadata 没有 AIGC 标识:\n%s", out)
	}
}
