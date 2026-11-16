package feed

// ==================== 微文图片上限（后台可调，默认 20） ====================

import (
	"strconv"
	"strings"

	"nvs-server/models"
)

const (
	ConfigMaxMicroImages   = "feed_micro_max_images"
	DefaultMaxMicroImages  = 20
	AbsoluteMaxMicroImages = 50
)

// MaxMicroImages 当前允许的单篇微文图片上限（缺省 20，后台可配 1~50）
func MaxMicroImages() int {
	v := strings.TrimSpace(models.GetPlatformConfig(ConfigMaxMicroImages))
	if v == "" {
		return DefaultMaxMicroImages
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return DefaultMaxMicroImages
	}
	if n > AbsoluteMaxMicroImages {
		return AbsoluteMaxMicroImages
	}
	return n
}

// SetMaxMicroImages 后台设置图片上限（范围 1~50）
func SetMaxMicroImages(n int) error {
	if n < 1 {
		n = 1
	}
	if n > AbsoluteMaxMicroImages {
		n = AbsoluteMaxMicroImages
	}
	return models.SetPlatformConfig(ConfigMaxMicroImages, strconv.Itoa(n))
}
