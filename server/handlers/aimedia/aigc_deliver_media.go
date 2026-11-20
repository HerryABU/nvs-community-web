package aimedia

// 视频/音频 AIGC 交付副本。图片那条链路（DeliverAIImages）已经证明：
// 出结果时就同步烧标识 + 内容寻址落盘，用户看到/导出的就是合规版本。
// 音视频这里沿用同一张 aigc_deliveries 表和签名 URL，只是烧录改成外部 ffmpeg 命令。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"nvs-server/config"
	"nvs-server/mediacover"
	aiops "nvs-server/models/aiops"
)

const (
	aigcMediaVideo = "video"
	aigcMediaAudio = "audio"
)

// DeliverAIMedia 生成带 AIGC 标识的交付副本，返回签名 URL 与是否成功烧录显式角标。
// 失败时退回原地址（至少已扣费的任务不会失败），绝不把 ffmpeg 依赖变成硬故障。
func DeliverAIMedia(userID, taskID uint, raw, kind string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw, false
	}
	cfg := aiops.LoadAigcConfig()
	if !cfg.Enabled {
		return raw, false
	}
	data, srcMime, err := readAigcSource(raw)
	if err != nil || len(data) == 0 {
		return raw, false
	}
	sum := sha256.Sum256(data)
	srcHex := hex.EncodeToString(sum[:])
	ext := mediaExt(kind, data, srcMime)

	key := mediaDeliveryKey(srcHex, srcMime, ext, cfg)
	if d, e := aiops.GetAigcDeliveryByKey(key); e == nil && d != nil {
		aiops.EnsureAigcDeliveryGrant(key, userID, taskID)
		aiops.BumpAigcDeliveryHits(key)
		return signDeliveryURL(key, userID), d.Labeled
	}

	dir, err := os.MkdirTemp("", "nvs-aigc-media-*")
	if err != nil {
		return raw, false
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "src."+ext)
	out := filepath.Join(dir, "marked."+ext)
	if err := os.WriteFile(in, data, 0o644); err != nil {
		return raw, false
	}

	label := cfg.LabelText
	payload := data
	labeled := false
	if kind == aigcMediaVideo {
		labeled, err = mediacover.MarkVideo(in, out, label)
	} else {
		labeled, err = mediacover.MarkAudio(in, out, label)
		labeled = err == nil // 音频只有元数据标识，没有可见角标
	}
	if err == nil {
		if b, e := os.ReadFile(out); e == nil && len(b) > 0 {
			payload = b
		}
	} else {
		log.Printf("[aigc-media] 标识烧录失败，交付原文件: %v", err)
		labeled = false
	}

	mime := "application/octet-stream"
	if kind == aigcMediaVideo {
		mime = "video/" + ext
	} else if e := ext; e == "wav" {
		mime = "audio/wav"
	} else {
		mime = "audio/mpeg"
	}
	d, _, ok := storeDelivery(userID, taskID, key, newProduceID(), srcHex, payload, mime, ext, 0, 0, labeled, "")
	if !ok {
		return raw, false
	}
	return d.URL, labeled
}

func mediaDeliveryKey(srcHash, srcMime, ext string, cfg aiops.AigcConfig) string {
	h := sha256.New()
	fmt.Fprintf(h, "nvs-aigc-media-v1\n%s\n%s\n%s\n%s\n%s\n",
		srcHash, srcMime, ext, cfg.LabelText, cfg.ContentProducer)
	return hex.EncodeToString(h.Sum(nil))
}

func mediaExt(kind string, data []byte, srcMime string) string {
	if kind == aigcMediaVideo {
		return "mp4"
	}
	lower := strings.ToLower(srcMime)
	if strings.Contains(lower, "wav") || (len(data) > 4 && string(data[:4]) == "RIFF") {
		return "wav"
	}
	if strings.Contains(lower, "mp4") || strings.Contains(lower, "m4a") || (len(data) > 8 && string(data[4:8]) == "ftyp") {
		return "m4a"
	}
	return "mp3"
}

// aigcMediaDir 供导出/清理逻辑复用（目录本身随交付表落盘，不做静态映射）
func aigcMediaDir() string { return filepath.Join(config.UploadDir, ".aigc-media") }
