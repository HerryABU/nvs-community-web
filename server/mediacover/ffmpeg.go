// Package mediacover 用 ffmpeg 从视频里抽一帧当封面。
//
// 为什么不用别的方式：
//   - 纯 Go 解 H.264 要么带 cgo（单 exe 发不出去），要么引入体积巨大的解码器；
//   - 让前端 canvas 抽帧只能覆盖「网页端手动上传」这一条路径，接口直传/历史数据仍然没封面；
//   - ffmpeg 是可选依赖：机器上没有就返回 ErrUnavailable，调用方降级（前端 canvas / 占位图），
//     绝不能让「没装 ffmpeg」把发布流程搞挂。
//
// ⚠️ 许可证红线：**严禁内联 / 打包 / 静态链接 ffmpeg**（含 ffmpeg.exe、libav*、cgo 绑定）。
// ffmpeg 是 LGPL/GPL 项目，本项目是 AGPL-3.0，把它的二进制塞进发行包或嵌进 exe 会把
// 许可证义务（源码提供、可替换库、GPL 传染）带到整个发布物上，也会让单 exe 体积翻几十 MB。
// 这里只做「按路径调用外部命令」：由部署者自行安装 ffmpeg 并保证 PATH 可找到，
// 或用 NVS_FFMPEG_PATH / FFMPEG_PATH 指到自己的可执行文件 —— 进程边界即许可边界。
// 仓库里不得出现任何 ffmpeg 二进制或 av* 库文件。
//
// 取帧策略：thumbnail 滤镜会分析开头 N 帧、挑最具代表性的一帧，
// 比固定 -ss 0（很多视频首帧是黑场/台标）稳得多。
package mediacover

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrUnavailable 机器上没有 ffmpeg（或不在 PATH 里）
var ErrUnavailable = errors.New("ffmpeg 不可用")

// Frame 抽帧结果
type Frame struct {
	JPEG   []byte
	Width  int
	Height int
}

func ffmpegPath() (string, error) {
	// 部署者显式指定优先（不随包分发，只是「指个路」）
	for _, key := range []string{"NVS_FFMPEG_PATH", "FFMPEG_PATH"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			if st, err := os.Stat(v); err == nil && !st.IsDir() {
				return v, nil
			}
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}
	// Windows 上常见的 winget / scoop 安装位置兜底（服务以自启动方式跑时 PATH 可能更短）
	for _, p := range []string{
		`C:\ffmpeg\bin\ffmpeg.exe`,
		`C:\Program Files\ffmpeg\bin\ffmpeg.exe`,
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", ErrUnavailable
}

// Available 供健康检查/后台提示用
func Available() bool {
	_, err := ffmpegPath()
	return err == nil
}

// ExtractFrame 从 input 里挑一帧（分析开头 maxFrames 帧）输出 JPEG。
// 失败时返回错误；调用方自行决定是否降级。
func ExtractFrame(input string, maxFrames int, targetWidth int) (Frame, error) {
	bin, err := ffmpegPath()
	if err != nil {
		return Frame{}, err
	}
	if maxFrames <= 0 || maxFrames > 300 {
		maxFrames = 10
	}
	if targetWidth <= 0 || targetWidth > 1920 {
		targetWidth = 720
	}
	dir, err := os.MkdirTemp("", "nvs-cover-*")
	if err != nil {
		return Frame{}, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "frame.jpg")

	// thumbnail=N 会缓存前 N 帧挑最合适的一帧；scale=-2 保比例，宽取偶数
	vf := "thumbnail=" + strconv.Itoa(maxFrames) + ",scale=" + strconv.Itoa(targetWidth) + ":-2"

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin,
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", input,
		"-vf", vf,
		"-frames:v", "1",
		"-q:v", "3",
		out,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return Frame{}, errors.New("ffmpeg 抽帧失败: " + firstLine(msg))
	}
	data, err := os.ReadFile(out)
	if err != nil || len(data) == 0 {
		return Frame{}, errors.New("ffmpeg 没有产出帧（视频可能损坏或格式不支持）")
	}
	cfg, _, derr := image.DecodeConfig(bytes.NewReader(data))
	if derr != nil {
		return Frame{JPEG: data}, nil
	}
	return Frame{JPEG: data, Width: cfg.Width, Height: cfg.Height}, nil
}

// Duration 视频时长（秒），取不到返回 0（不视为错误）
func Duration(input string) int {
	bin, err := ffmpegPath()
	if err != nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// -i 不带输出：ffmpeg 会把时长打到 stderr，读 Duration: 行即可，省一个 ffprobe 依赖
	cmd := exec.CommandContext(ctx, bin, "-hide_banner", "-i", input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run()
	s := stderr.String()
	i := strings.Index(s, "Duration:")
	if i < 0 {
		return 0
	}
	seg := strings.TrimSpace(s[i+len("Duration:"):])
	if j := strings.IndexByte(seg, ','); j > 0 {
		seg = seg[:j]
	}
	parts := strings.Split(strings.TrimSpace(seg), ":")
	if len(parts) != 3 {
		return 0
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	sec, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0
	}
	return h*3600 + m*60 + int(sec)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i > 0 {
		return s[:i]
	}
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
