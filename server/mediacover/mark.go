package mediacover

// 音视频 AIGC 标识：仍严守许可证红线，只按路径调用外部 ffmpeg，
// 不内联、不打包、不静态链接任何 libav* / ffmpeg 二进制。
//
// 视频：drawtext 把「AI生成」烧进右下角，同时写 metadata comment；
// 音频：没有画面可烧，写 metadata（comment / description），供播放器与导出读取。
// 找不到中文字体时视频降级为「只写 metadata」，不会让已扣费的生成任务失败。

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FindFont 找可用的中文字体；环境变量优先，便于容器部署自带字体。
func FindFont() string {
	for _, key := range []string{"NVS_AIGC_FONT", "AIGC_FONT_PATH"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			if st, err := os.Stat(v); err == nil && !st.IsDir() {
				return v
			}
		}
	}
	for _, p := range []string{
		`C:\Windows\Fonts\simhei.ttf`,
		`C:\Windows\Fonts\msyh.ttc`,
		`/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc`,
		`/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc`,
		`/System/Library/Fonts/PingFang.ttc`,
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func escapeFilterText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `’`)
	s = strings.ReplaceAll(s, `:`, `\:`)
	return s
}

// ffmpeg 滤镜里的 Windows 路径要写成 C\:/Windows/Fonts/xxx.ttf
func filterFontPath(p string) string {
	p = filepath.ToSlash(p)
	if len(p) > 1 && p[1] == ':' {
		p = p[:1] + `\:` + p[2:]
	}
	return p
}

func runMark(input, output, vf, label string, audioOnly bool) error {
	bin, err := ffmpegPath()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", input}
	if vf != "" {
		args = append(args, "-vf", vf)
		if audioOnly {
			args = append(args, "-c:a", "copy")
		}
	} else {
		args = append(args, "-c", "copy")
	}
	args = append(args,
		"-map_metadata", "0",
		"-metadata", "comment="+label,
		"-metadata", "description="+label,
		output,
	)
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return errors.New("ffmpeg 写入 AIGC 标识失败: " + firstLine(msg))
	}
	return nil
}

// MarkVideo 烧录可见角标 + 写元数据。没有中文字体时只写元数据（labeled=false 由调用方判断）。
func MarkVideo(input, output, label string) (labeled bool, err error) {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "AI生成"
	}
	font := FindFont()
	if font == "" {
		err = runMark(input, output, "", label, false)
		return false, err
	}
	vf := "drawtext=fontfile='" + filterFontPath(font) +
		"':text='" + escapeFilterText(label) +
		"':fontsize=28:fontcolor=white:box=1:boxcolor=black@0.55:x=w-text_w-16:y=h-text_h-16"
	err = runMark(input, output, vf, label, false)
	return err == nil, err
}

// MarkAudio 音频没有画面，写元数据即可（导出/播放器/取证都能读到）。
func MarkAudio(input, output, label string) (labeled bool, err error) {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "AI生成"
	}
	err = runMark(input, output, "", label, true)
	return err == nil, err
}
